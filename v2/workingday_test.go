package v2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testCalendar = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//go-workingday test//CN
X-WR-CALDESC:2024~2024年测试数据
BEGIN:VEVENT
DTSTART;VALUE=DATE:20240101
DTEND;VALUE=DATE:20240102
SUMMARY:元旦 假期 第1天/共1天
END:VEVENT
BEGIN:VEVENT
DTSTART:20240106T090000
DTEND:20240106T180000
SUMMARY:元旦 补班 第1天/共1天
END:VEVENT
BEGIN:VEVENT
DTSTART;VALUE=DATE:20240429
DTEND;VALUE=DATE:20240430
SUMMARY:劳动节 放假
END:VEVENT
BEGIN:VEVENT
DTSTART:20240428T090000
DTEND:20240428T180000
SUMMARY:劳动节 补班
END:VEVENT
END:VCALENDAR`

// TestLoadCalendarUsesBundledData verifies that the default v2 path works
// entirely from the embedded ICS snapshot.
func TestLoadCalendarUsesBundledData(t *testing.T) {
	calendar, err := LoadCalendar(RegionCN)
	if err != nil {
		t.Fatalf("LoadCalendar() error = %v", err)
	}

	gotWork, gotStatus, err := calendar.IsWorkDay(testDate(2026, time.January, 4))
	if err != nil {
		t.Fatalf("IsWorkDay() error = %v", err)
	}
	if !gotWork || gotStatus != DayStatusWork {
		t.Fatalf("IsWorkDay() = (%v, %q), want (true, %q)", gotWork, gotStatus, DayStatusWork)
	}
	if gotYears := calendar.Years(); len(gotYears) != 4 || gotYears[0] != 2023 || gotYears[3] != 2026 {
		t.Fatalf("Years() = %v, want [2023 2024 2025 2026]", gotYears)
	}
}

// TestLoadCalendarFromFile verifies caller-provided offline ICS import.
func TestLoadCalendarFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "holidayCal.ics")
	if err := os.WriteFile(path, []byte(testCalendar), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	calendar, err := LoadCalendarFromFile(path, RegionCN)
	if err != nil {
		t.Fatalf("LoadCalendarFromFile() error = %v", err)
	}
	gotWork, gotStatus, err := calendar.IsWorkDay(testDate(2024, time.January, 6))
	if err != nil {
		t.Fatalf("IsWorkDay() error = %v", err)
	}
	if !gotWork || gotStatus != DayStatusWork {
		t.Fatalf("IsWorkDay() = (%v, %q), want (true, %q)", gotWork, gotStatus, DayStatusWork)
	}

	if _, err := LoadCalendarFromFile(filepath.Join(t.TempDir(), "missing.ics"), RegionCN); err == nil {
		t.Fatal("LoadCalendarFromFile() error = nil for missing file, want non-nil")
	}
}

// TestParseCalendarAndIsWorkDay verifies holiday, compensated-workday, weekday,
// weekend, coverage, and year-reporting behavior using an offline fixture.
func TestParseCalendarAndIsWorkDay(t *testing.T) {
	calendar, err := ParseCalendar(strings.NewReader(testCalendar), RegionCN)
	if err != nil {
		t.Fatalf("ParseCalendar() error = %v", err)
	}

	tests := []struct {
		name       string
		date       time.Time
		wantWork   bool
		wantStatus DayStatus
	}{
		{name: "holiday on weekday", date: testDate(2024, time.January, 1), wantWork: false, wantStatus: DayStatusRest},
		{name: "compensated workday on weekend", date: testDate(2024, time.January, 6), wantWork: true, wantStatus: DayStatusWork},
		{name: "normal weekday", date: testDate(2024, time.January, 3), wantWork: true, wantStatus: DayStatusNormal},
		{name: "normal weekend", date: testDate(2024, time.January, 7), wantWork: false, wantStatus: DayStatusNormal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotWork, gotStatus, err := calendar.IsWorkDay(test.date)
			if err != nil {
				t.Fatalf("IsWorkDay() error = %v", err)
			}
			if gotWork != test.wantWork || gotStatus != test.wantStatus {
				t.Fatalf("IsWorkDay() = (%v, %q), want (%v, %q)", gotWork, gotStatus, test.wantWork, test.wantStatus)
			}
		})
	}

	if _, _, err := calendar.IsWorkDay(testDate(2025, time.January, 1)); err == nil {
		t.Fatal("IsWorkDay() error = nil for uncovered year, want non-nil")
	}
	if got := calendar.Years(); len(got) != 1 || got[0] != 2024 {
		t.Fatalf("Years() = %v, want [2024]", got)
	}
}

// TestNthWorkdayFromLast verifies that explicit overrides take precedence while
// counting backward and that invalid requests fail rather than crossing months.
func TestNthWorkdayFromLast(t *testing.T) {
	calendar, err := ParseCalendar(strings.NewReader(testCalendar), RegionCN)
	if err != nil {
		t.Fatalf("ParseCalendar() error = %v", err)
	}

	got, err := calendar.NthWorkdayFromLast(testDate(2024, time.April, 10), 3)
	if err != nil {
		t.Fatalf("NthWorkdayFromLast() error = %v", err)
	}
	want := testDate(2024, time.April, 26)
	if !got.Equal(want) {
		t.Fatalf("NthWorkdayFromLast() = %s, want %s", got, want)
	}

	got, err = calendar.LastThirdWorkday(testDate(2024, time.April, 10))
	if err != nil {
		t.Fatalf("LastThirdWorkday() error = %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("LastThirdWorkday() = %s, want %s", got, want)
	}

	if _, err := calendar.NthWorkdayFromLast(testDate(2024, time.April, 10), 0); err == nil {
		t.Fatal("NthWorkdayFromLast() error = nil for n=0, want non-nil")
	}
	if _, err := calendar.NthWorkdayFromLast(testDate(2024, time.April, 10), 31); err == nil {
		t.Fatal("NthWorkdayFromLast() error = nil for n larger than the month, want non-nil")
	}
}

// TestParseCalendarRejectsInvalidData verifies that malformed or ambiguous feed
// data is surfaced instead of being silently treated as a normal weekday.
func TestParseCalendarRejectsInvalidData(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		region string
	}{
		{name: "unsupported region", data: testCalendar, region: "HK"},
		{name: "malformed calendar", data: "not an iCalendar", region: RegionCN},
		{name: "missing coverage", data: "BEGIN:VCALENDAR\nVERSION:2.0\nEND:VCALENDAR", region: RegionCN},
		{name: "invalid coverage", data: strings.Replace(testCalendar, "2024~2024年", "2025~2024年", 1), region: RegionCN},
		{name: "empty coverage year", data: strings.Replace(testCalendar, "2024~2024年", "2024~2025年", 1), region: RegionCN},
		{name: "missing summary", data: calendarWithEvent("DTSTART;VALUE=DATE:20240101"), region: RegionCN},
		{name: "unknown summary", data: calendarWithEvent("DTSTART;VALUE=DATE:20240101\nSUMMARY:普通纪念日"), region: RegionCN},
		{name: "invalid date", data: calendarWithEvent("DTSTART;VALUE=DATE:20241301\nSUMMARY:假期"), region: RegionCN},
		{
			name: "conflicting duplicate",
			data: `BEGIN:VCALENDAR
VERSION:2.0
X-WR-CALDESC:2024~2024年测试数据
BEGIN:VEVENT
DTSTART;VALUE=DATE:20240101
SUMMARY:元旦 假期
END:VEVENT
BEGIN:VEVENT
DTSTART:20240101T090000
SUMMARY:元旦 补班
END:VEVENT
END:VCALENDAR`,
			region: RegionCN,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseCalendar(strings.NewReader(test.data), test.region); err == nil {
				t.Fatal("ParseCalendar() error = nil, want non-nil")
			}
		})
	}
}

// TestNilReceivers verifies that programmer errors return explicit failures
// instead of causing nil-pointer panics.
func TestNilReceivers(t *testing.T) {
	var calendar *Calendar
	if _, _, err := calendar.IsWorkDay(testDate(2024, time.January, 1)); err == nil {
		t.Fatal("IsWorkDay() error = nil for nil calendar, want non-nil")
	}
	if _, err := calendar.NthWorkdayFromLast(testDate(2024, time.January, 1), 1); err == nil {
		t.Fatal("NthWorkdayFromLast() error = nil for nil calendar, want non-nil")
	}
	if got := calendar.Years(); got != nil {
		t.Fatalf("Years() = %v for nil calendar, want nil", got)
	}
}

// testDate creates a midnight UTC value for concise calendar assertions.
func testDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// calendarWithEvent wraps event properties in the minimum valid iCalendar envelope.
func calendarWithEvent(properties string) string {
	return "BEGIN:VCALENDAR\nVERSION:2.0\nX-WR-CALDESC:2024~2024年测试数据\nBEGIN:VEVENT\n" + properties + "\nEND:VEVENT\nEND:VCALENDAR"
}
