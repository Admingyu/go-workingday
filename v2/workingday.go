// Package v2 provides workday calculations backed by an iCalendar feed.
//
// A Calendar is loaded once and can then answer any number of queries without
// performing more network requests. The current data source contains official
// holiday and compensated-workday arrangements for mainland China.
package v2

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
)

const (
	// RegionCN is the supported mainland China region code.
	RegionCN = "CN"
	// defaultCalendarURL is the complete holiday and compensated-workday feed.
	defaultCalendarURL = "https://cdn.jsdelivr.net/gh/lanceliao/china-holiday-calender/holidayCal.ics"
	// defaultHTTPTimeout prevents a load from waiting forever when the source is unavailable.
	defaultHTTPTimeout = 15 * time.Second
	// dateLayout is the compact date representation used by iCalendar and the day index.
	dateLayout = "20060102"
)

// coveragePattern extracts the inclusive supported-year range from the source's
// X-WR-CALDESC value, for example "2023~2026年中国放假...".
var coveragePattern = regexp.MustCompile(`(\d{4})~(\d{4})年`)

// defaultCalendarICS is the bundled offline calendar used by LoadCalendar.
//
//go:embed data/holidayCal.ics
var defaultCalendarICS []byte

// DayStatus describes why a date is or is not a workday.
type DayStatus string

const (
	// DayStatusNormal means no holiday adjustment applies, so the weekday decides the result.
	DayStatusNormal DayStatus = "NORMAL"
	// DayStatusWork means a weekend has explicitly been changed to a compensated workday.
	DayStatusWork DayStatus = "WORK"
	// DayStatusRest means a date is explicitly included in a public holiday break.
	DayStatusRest DayStatus = "REST"
)

// Calendar stores parsed holiday overrides and the years covered by its source.
// Both maps are immutable after parsing, so a Calendar is safe for concurrent reads.
type Calendar struct {
	// region identifies the workday rules represented by this calendar.
	region string
	// days maps YYYYMMDD values to explicit holiday or compensated-workday states.
	days map[string]DayStatus
	// years records every year for which the source contains arrangement data.
	years map[int]struct{}
}

// LoadCalendar parses the bundled offline iCalendar data for region.
// It never performs a network request.
//
// Example:
//
//	calendar, err := v2.LoadCalendar(v2.RegionCN)
//	if err != nil {
//		log.Fatal(err)
//	}
func LoadCalendar(region string) (*Calendar, error) {
	calendar, err := ParseCalendar(bytes.NewReader(defaultCalendarICS), region)
	if err != nil {
		return nil, fmt.Errorf("load bundled %s calendar: %w", region, err)
	}
	return calendar, nil
}

// LoadCalendarFromFile imports a caller-provided offline iCalendar file.
func LoadCalendarFromFile(path string, region string) (*Calendar, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open calendar file %q: %w", path, err)
	}
	defer file.Close()

	calendar, err := ParseCalendar(file, region)
	if err != nil {
		return nil, fmt.Errorf("parse calendar file %q: %w", path, err)
	}
	return calendar, nil
}

// LoadCalendarOnline explicitly downloads and parses the latest calendar for
// region into memory. It does not modify the bundled snapshot. Normal queries
// should use the bundled offline calendar or LoadCalendarFromFile.
func LoadCalendarOnline(ctx context.Context, region string) (*Calendar, error) {
	if ctx == nil {
		return nil, fmt.Errorf("load online calendar: context must not be nil")
	}
	if err := validateRegion(region); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, defaultCalendarURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create calendar request: %w", err)
	}

	client := &http.Client{Timeout: defaultHTTPTimeout}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download %s calendar: %w", region, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s calendar: unexpected HTTP status %s", region, response.Status)
	}

	calendar, err := ParseCalendar(response.Body, region)
	if err != nil {
		return nil, fmt.Errorf("parse downloaded %s calendar: %w", region, err)
	}
	return calendar, nil
}

// ParseCalendar parses an iCalendar stream into workday rules for region.
// X-WR-CALDESC must declare an inclusive YYYY~YYYY coverage range, and each
// event must be identifiable as either a holiday ("假期" or "放假") or a
// compensated workday ("补班"). Unexpected input is returned as an error so
// upstream data changes cannot silently produce incorrect workday results.
func ParseCalendar(reader io.Reader, region string) (*Calendar, error) {
	if reader == nil {
		return nil, fmt.Errorf("parse calendar: reader must not be nil")
	}
	if err := validateRegion(region); err != nil {
		return nil, err
	}

	parsed, err := ics.ParseCalendar(reader)
	if err != nil {
		return nil, fmt.Errorf("parse iCalendar data: %w", err)
	}

	calendar := &Calendar{
		region: region,
		days:   make(map[string]DayStatus),
		years:  make(map[int]struct{}),
	}
	for _, property := range parsed.CalendarProperties {
		if property.IANAToken != "X-WR-CALDESC" {
			continue
		}

		matches := coveragePattern.FindStringSubmatch(property.Value)
		if len(matches) != 3 {
			return nil, fmt.Errorf("parse calendar: X-WR-CALDESC must contain a YYYY~YYYY年 coverage range")
		}
		firstYear, firstYearErr := strconv.Atoi(matches[1])
		lastYear, lastYearErr := strconv.Atoi(matches[2])
		if firstYearErr != nil || lastYearErr != nil || firstYear > lastYear {
			return nil, fmt.Errorf("parse calendar: invalid coverage range %q", matches[0])
		}
		for year := firstYear; year <= lastYear; year++ {
			calendar.years[year] = struct{}{}
		}
		break
	}
	if len(calendar.years) == 0 {
		return nil, fmt.Errorf("parse calendar: X-WR-CALDESC with a YYYY~YYYY年 coverage range is required")
	}

	eventYears := make(map[int]struct{})
	for eventIndex, event := range parsed.Events() {
		startProperty := event.GetProperty(ics.ComponentPropertyDtStart)
		if startProperty == nil || len(startProperty.Value) < len(dateLayout) {
			return nil, fmt.Errorf("event %d: DTSTART must begin with a YYYYMMDD date", eventIndex+1)
		}

		dateKey := startProperty.Value[:len(dateLayout)]
		date, err := time.Parse(dateLayout, dateKey)
		if err != nil {
			return nil, fmt.Errorf("event %d: invalid DTSTART %q: %w", eventIndex+1, startProperty.Value, err)
		}

		summaryProperty := event.GetProperty(ics.ComponentPropertySummary)
		if summaryProperty == nil || strings.TrimSpace(summaryProperty.Value) == "" {
			return nil, fmt.Errorf("event %d on %s: SUMMARY is required", eventIndex+1, date.Format("2006-01-02"))
		}

		var status DayStatus
		switch summary := summaryProperty.Value; {
		case strings.Contains(summary, "补班"):
			status = DayStatusWork
		case strings.Contains(summary, "假期"), strings.Contains(summary, "放假"):
			status = DayStatusRest
		default:
			return nil, fmt.Errorf("event %d on %s: unsupported SUMMARY %q", eventIndex+1, date.Format("2006-01-02"), summary)
		}

		if existingStatus, exists := calendar.days[dateKey]; exists && existingStatus != status {
			return nil, fmt.Errorf("event %d on %s: conflicting statuses %s and %s", eventIndex+1, date.Format("2006-01-02"), existingStatus, status)
		}
		calendar.days[dateKey] = status
		eventYears[date.Year()] = struct{}{}
	}

	if len(calendar.days) == 0 {
		return nil, fmt.Errorf("parse calendar: no holiday or compensated-workday events found")
	}
	for year := range calendar.years {
		if _, exists := eventYears[year]; !exists {
			return nil, fmt.Errorf("parse calendar: coverage year %d contains no events", year)
		}
	}
	return calendar, nil
}

// IsWorkDay reports whether date is a workday and explains the applicable status.
// An explicit holiday or compensated workday takes precedence over the normal
// Monday-to-Friday rule.
func (calendar *Calendar) IsWorkDay(date time.Time) (bool, DayStatus, error) {
	if calendar == nil {
		return false, "", fmt.Errorf("is workday: calendar must not be nil")
	}
	if _, covered := calendar.years[date.Year()]; !covered {
		return false, "", fmt.Errorf("is workday: year %d is not covered by the %s calendar", date.Year(), calendar.region)
	}

	if status, adjusted := calendar.days[date.Format(dateLayout)]; adjusted {
		return status == DayStatusWork, status, nil
	}

	weekday := date.Weekday()
	return weekday != time.Saturday && weekday != time.Sunday, DayStatusNormal, nil
}

// LastThirdWorkday returns the third workday counted backward from the end of
// date's month.
func (calendar *Calendar) LastThirdWorkday(date time.Time) (time.Time, error) {
	return calendar.NthWorkdayFromLast(date, 3)
}

// NthWorkdayFromLast returns the nth workday counted backward from the end of
// date's month. It returns an error instead of spilling into the previous month
// when n exceeds the number of workdays in the requested month.
func (calendar *Calendar) NthWorkdayFromLast(date time.Time, n int) (time.Time, error) {
	if calendar == nil {
		return time.Time{}, fmt.Errorf("nth workday from last: calendar must not be nil")
	}
	if n <= 0 {
		return time.Time{}, fmt.Errorf("nth workday from last: n must be greater than zero")
	}
	if _, covered := calendar.years[date.Year()]; !covered {
		return time.Time{}, fmt.Errorf("nth workday from last: year %d is not covered by the %s calendar", date.Year(), calendar.region)
	}

	location := date.Location()
	monthStart := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, location)
	day := monthStart.AddDate(0, 1, -1)
	workdayCount := 0
	for !day.Before(monthStart) {
		isWorkday, _, err := calendar.IsWorkDay(day)
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

// Years returns the sorted years for which this calendar has arrangement data.
// The returned slice is independent of the calendar and may be modified by the caller.
func (calendar *Calendar) Years() []int {
	if calendar == nil {
		return nil
	}

	years := make([]int, 0, len(calendar.years))
	for year := range calendar.years {
		years = append(years, year)
	}
	sort.Ints(years)
	return years
}

// validateRegion rejects region codes for which v2 has no authoritative
// holiday and compensated-workday source.
func validateRegion(region string) error {
	if region != RegionCN {
		return fmt.Errorf("unsupported region %q: v2 currently supports only %s", region, RegionCN)
	}
	return nil
}
