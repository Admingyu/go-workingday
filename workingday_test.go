package workingday

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testFestivalJSON = `{
  "national_holiday": {},
  "holidays": {
    "cn": [{"date": 20240101, "status": 0}, {"date": 20240106, "status": 1}],
    "hk": [{"date": 20240101, "status": 0}],
    "ma": [{"date": 20240101, "status": 0}],
    "tw": [{"date": 20240101, "status": 0}]
  }
}`

// TestLoadCalendarUsesBundledData verifies that the default v1 path works
// entirely from the embedded JSON snapshot.
func TestLoadCalendarUsesBundledData(t *testing.T) {
	calendar, err := LoadCalendar()
	if err != nil {
		t.Fatalf("LoadCalendar() error = %v", err)
	}

	tests := []struct {
		date       time.Time
		wantWork   bool
		wantStatus string
	}{
		{date: testLegacyDate(2026, time.January, 1), wantWork: false, wantStatus: "REST"},
		{date: testLegacyDate(2026, time.January, 4), wantWork: true, wantStatus: "WORK"},
	}
	for _, test := range tests {
		gotWork, gotStatus, err := calendar.IsWorkDay(test.date, "CN")
		if err != nil {
			t.Fatalf("IsWorkDay(%s) error = %v", test.date.Format("2006-01-02"), err)
		}
		if gotWork != test.wantWork || gotStatus != test.wantStatus {
			t.Fatalf("IsWorkDay(%s) = (%v, %q), want (%v, %q)", test.date.Format("2006-01-02"), gotWork, gotStatus, test.wantWork, test.wantStatus)
		}
	}
}

// TestLoadCalendarFromFile verifies caller-provided offline JSON import.
func TestLoadCalendarFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "festival.json")
	if err := os.WriteFile(path, []byte(testFestivalJSON), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	calendar, err := LoadCalendarFromFile(path)
	if err != nil {
		t.Fatalf("LoadCalendarFromFile() error = %v", err)
	}
	gotWork, gotStatus, err := calendar.IsWorkDay(testLegacyDate(2024, time.January, 6), "CN")
	if err != nil {
		t.Fatalf("IsWorkDay() error = %v", err)
	}
	if !gotWork || gotStatus != "WORK" {
		t.Fatalf("IsWorkDay() = (%v, %q), want (true, %q)", gotWork, gotStatus, "WORK")
	}

	if _, err := LoadCalendarFromFile(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("LoadCalendarFromFile() error = nil for missing file, want non-nil")
	}
}

// TestParseCalendarRejectsInvalidJSON verifies strict validation of the legacy
// offline format.
func TestParseCalendarRejectsInvalidJSON(t *testing.T) {
	tests := []string{
		`{"holidays": null}`,
		strings.Replace(testFestivalJSON, `"tw": [{"date": 20240101, "status": 0}]`, `"tw": []`, 1),
		strings.Replace(testFestivalJSON, `"status": 1`, `"status": 2`, 1),
		strings.Replace(testFestivalJSON, `20240106`, `20241306`, 1),
		testFestivalJSON + `{}`,
	}
	for _, data := range tests {
		if _, err := ParseCalendar(strings.NewReader(data)); err == nil {
			t.Fatalf("ParseCalendar() error = nil for %s, want non-nil", data)
		}
	}
}

// TestLegacyIsWorkDayUsesBundledData verifies that the original API remains
// compatible while no longer requiring a network request.
func TestLegacyIsWorkDayUsesBundledData(t *testing.T) {
	gotWork, gotStatus := IsWorkDay(testLegacyDate(2026, time.January, 4), "CN")
	if !gotWork || gotStatus != "WORK" {
		t.Fatalf("IsWorkDay() = (%v, %q), want (true, %q)", gotWork, gotStatus, "WORK")
	}
}

// testLegacyDate creates a midnight UTC value for concise v1 assertions.
func testLegacyDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
