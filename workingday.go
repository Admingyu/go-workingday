// Package workingday determines workdays using the legacy festival JSON format.
package workingday

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	// festivalURL is the legacy online source used for an explicit data refresh.
	festivalURL = "http://pc.suishenyun.net/peacock/api/h5/festival"
	// defaultHTTPTimeout prevents an online refresh from waiting indefinitely.
	defaultHTTPTimeout = 15 * time.Second
)

// defaultCalendarJSON is the bundled offline snapshot used by LoadCalendar.
//
//go:embed data/festival.json
var defaultCalendarJSON []byte

// dayType is one date override from the legacy festival JSON format.
// Status 0 means rest and status 1 means work.
type dayType struct {
	Date   int `json:"date"`
	Status int `json:"status"`
}

// holidaysType groups date overrides by the region keys used by the legacy API.
type holidaysType struct {
	Cn []dayType `json:"cn"`
	Hk []dayType `json:"hk"`
	Ma []dayType `json:"ma"`
	Tw []dayType `json:"tw"`
}

// calendarBody mirrors the relevant fields returned by the legacy festival API.
type calendarBody struct {
	NationalHoliday interface{}   `json:"national_holiday"`
	Holidays        *holidaysType `json:"holidays"`
}

// Calendar is an immutable parsed legacy workday calendar.
// A Calendar can be reused concurrently after loading.
type Calendar struct {
	data calendarBody
}

// LoadCalendar parses and returns the bundled offline festival JSON.
// It never performs a network request.
func LoadCalendar() (*Calendar, error) {
	calendar, err := ParseCalendar(bytes.NewReader(defaultCalendarJSON))
	if err != nil {
		return nil, fmt.Errorf("load bundled calendar: %w", err)
	}
	return calendar, nil
}

// LoadCalendarFromFile imports a caller-provided offline festival JSON file.
func LoadCalendarFromFile(path string) (*Calendar, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open calendar file %q: %w", path, err)
	}
	defer file.Close()

	calendar, err := ParseCalendar(file)
	if err != nil {
		return nil, fmt.Errorf("parse calendar file %q: %w", path, err)
	}
	return calendar, nil
}

// LoadCalendarOnline explicitly downloads and parses the latest legacy JSON
// into memory. It does not modify the bundled snapshot. Normal workday queries
// should use the bundled offline calendar or LoadCalendarFromFile.
func LoadCalendarOnline(ctx context.Context) (*Calendar, error) {
	if ctx == nil {
		return nil, fmt.Errorf("load online calendar: context must not be nil")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, festivalURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create calendar request: %w", err)
	}
	response, err := (&http.Client{Timeout: defaultHTTPTimeout}).Do(request)
	if err != nil {
		return nil, fmt.Errorf("download calendar: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download calendar: unexpected HTTP status %s", response.Status)
	}
	calendar, err := ParseCalendar(response.Body)
	if err != nil {
		return nil, fmt.Errorf("parse downloaded calendar: %w", err)
	}
	return calendar, nil
}

// ParseCalendar imports calendar data in the legacy festival API JSON format.
// All four region arrays must exist and every entry must contain a valid date
// and a status of either 0 or 1.
func ParseCalendar(reader io.Reader) (*Calendar, error) {
	if reader == nil {
		return nil, fmt.Errorf("parse calendar: reader must not be nil")
	}

	var data calendarBody
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("decode festival JSON: %w", err)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode festival JSON: multiple JSON values are not allowed")
		}
		return nil, fmt.Errorf("decode festival JSON trailing data: %w", err)
	}
	if data.Holidays == nil {
		return nil, fmt.Errorf("decode festival JSON: holidays is required")
	}

	regions := map[string][]dayType{
		"CN": data.Holidays.Cn,
		"HK": data.Holidays.Hk,
		"MA": data.Holidays.Ma,
		"TW": data.Holidays.Tw,
	}
	for region, days := range regions {
		if len(days) == 0 {
			return nil, fmt.Errorf("decode festival JSON: holidays.%s must not be empty", region)
		}
		seen := make(map[int]int, len(days))
		for index, day := range days {
			if _, err := time.Parse("20060102", fmt.Sprintf("%08d", day.Date)); err != nil {
				return nil, fmt.Errorf("decode festival JSON: holidays.%s[%d] has invalid date %d", region, index, day.Date)
			}
			if day.Status != 0 && day.Status != 1 {
				return nil, fmt.Errorf("decode festival JSON: holidays.%s[%d] has invalid status %d", region, index, day.Status)
			}
			if status, exists := seen[day.Date]; exists && status != day.Status {
				return nil, fmt.Errorf("decode festival JSON: holidays.%s has conflicting statuses for %d", region, day.Date)
			}
			seen[day.Date] = day.Status
		}
	}

	return &Calendar{data: data}, nil
}

// IsWorkDay reports whether date is a workday in region and returns NORMAL,
// WORK, or REST to explain the result.
func (calendar *Calendar) IsWorkDay(date time.Time, region string) (bool, string, error) {
	holidays, err := calendar.regionHolidays(region)
	if err != nil {
		return false, "", err
	}

	dateNumber := date.Year()*10000 + int(date.Month())*100 + date.Day()
	for _, holiday := range holidays {
		if holiday.Date != dateNumber {
			continue
		}
		if holiday.Status == 0 {
			return false, "REST", nil
		}
		return true, "WORK", nil
	}

	weekday := date.Weekday()
	return weekday != time.Saturday && weekday != time.Sunday, "NORMAL", nil
}

// LastThirdWorkDay returns the third workday counted backward from the end of
// date's month for mainland China.
func (calendar *Calendar) LastThirdWorkDay(date time.Time) (time.Time, error) {
	return calendar.NthWorkdayFromLast(date, 3, "CN")
}

// NthWorkdayFromLast returns the nth workday counted backward from the end of
// date's month. It returns an error instead of crossing into the previous month.
func (calendar *Calendar) NthWorkdayFromLast(date time.Time, n int, region string) (time.Time, error) {
	if n <= 0 {
		return time.Time{}, fmt.Errorf("nth workday from last: n must be greater than zero")
	}
	if _, err := calendar.regionHolidays(region); err != nil {
		return time.Time{}, err
	}

	location := date.Location()
	monthStart := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, location)
	day := monthStart.AddDate(0, 1, 0).Add(-time.Second)
	workdayCount := 0
	for !day.Before(monthStart) {
		isWorkday, _, err := calendar.IsWorkDay(day, region)
		if err != nil {
			return time.Time{}, err
		}
		if isWorkday {
			workdayCount++
			if workdayCount == n {
				return day, nil
			}
		}
		day = day.AddDate(0, 0, -1)
	}

	return time.Time{}, fmt.Errorf("nth workday from last: month %s has fewer than %d workdays", monthStart.Format("2006-01"), n)
}

// regionHolidays returns the date overrides for a supported region.
func (calendar *Calendar) regionHolidays(region string) ([]dayType, error) {
	if calendar == nil || calendar.data.Holidays == nil {
		return nil, fmt.Errorf("calendar must not be nil")
	}
	switch region {
	case "CN":
		return calendar.data.Holidays.Cn, nil
	case "HK":
		return calendar.data.Holidays.Hk, nil
	case "MA":
		return calendar.data.Holidays.Ma, nil
	case "TW":
		return calendar.data.Holidays.Tw, nil
	default:
		return nil, fmt.Errorf("unsupported region %q: expected CN, HK, MA, or TW", region)
	}
}

// FillCalendar preserves the v1 API and now returns bundled offline data.
// New code should use LoadCalendar so parsing errors can be handled explicitly.
func FillCalendar() calendarBody {
	calendar, err := LoadCalendar()
	if err != nil {
		panic(err)
	}
	return calendar.data
}

// IsWorkDay preserves the v1 API and queries the bundled offline calendar.
func IsWorkDay(date time.Time, region string) (bool, string) {
	calendar, err := LoadCalendar()
	if err != nil {
		panic(err)
	}
	isWorkday, status, err := calendar.IsWorkDay(date, region)
	if err != nil {
		panic(err)
	}
	return isWorkday, status
}

// LastThirdWorkDay preserves the v1 API for mainland China.
func LastThirdWorkDay(date time.Time) time.Time {
	return NthWorkdayFromLast(date, 3, "CN")
}

// NthWorkdayFromLast preserves the v1 API and queries the bundled offline calendar.
func NthWorkdayFromLast(date time.Time, n int, region string) time.Time {
	calendar, err := LoadCalendar()
	if err != nil {
		panic(err)
	}
	workday, err := calendar.NthWorkdayFromLast(date, n, region)
	if err != nil {
		panic(err)
	}
	return workday
}

// GetRegionHolidays preserves the v1 API and returns a copy of the bundled
// offline date overrides for region.
func GetRegionHolidays(region string) []dayType {
	calendar, err := LoadCalendar()
	if err != nil {
		panic(err)
	}
	holidays, err := calendar.regionHolidays(region)
	if err != nil {
		panic(err)
	}
	return append([]dayType(nil), holidays...)
}
