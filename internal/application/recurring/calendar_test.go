package recurring

import (
	"testing"
	"time"

	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
)

func utc(value string) time.Time {
	result, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return result
}

func TestCalendarHandlesDaylightSavingTransitions(t *testing.T) {
	for _, fixture := range []struct{ clock, after, next string }{
		{"02:30", "2026-03-07T08:00:00Z", "2026-03-09T06:30:00Z"},
		{"01:30", "2026-11-01T04:00:00Z", "2026-11-01T05:30:00Z"},
		{"01:30", "2026-11-01T05:30:00Z", "2026-11-02T06:30:00Z"},
	} {
		rule, err := scheduleCalendar(domainexecution.Schedule{TimeZone: "America/New_York", LocalTime: fixture.clock, Frequency: "daily"})
		if err != nil {
			t.Fatal(err)
		}
		if got := rule.adjacent(utc(fixture.after), 1); !got.Equal(utc(fixture.next)) {
			t.Errorf("after %s got %s, want %s", fixture.after, got, fixture.next)
		}
	}
}

func TestWeeklyCalendarFindsLatestAndUpcomingTimes(t *testing.T) {
	rule, err := scheduleCalendar(domainexecution.Schedule{TimeZone: "America/Sao_Paulo",
		LocalTime: "09:00", Frequency: "weekly", Weekday: int(time.Monday)})
	if err != nil {
		t.Fatal(err)
	}
	now := utc("2026-11-18T10:00:00Z")
	if next := rule.adjacent(now, 1); !next.Equal(utc("2026-11-23T12:00:00Z")) {
		t.Fatalf("unexpected next occurrence %s", next)
	}
	if latest := rule.adjacent(now, -1); !latest.Equal(utc("2026-11-16T12:00:00Z")) {
		t.Fatalf("unexpected latest occurrence %s", latest)
	}
}
