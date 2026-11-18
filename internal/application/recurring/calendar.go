package recurring

import (
	"fmt"
	"time"

	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
)

type calendar struct {
	zone         *time.Location
	hour, minute int
	weekly       bool
	weekday      time.Weekday
}

func scheduleCalendar(schedule domainexecution.Schedule) (calendar, error) {
	zone, err := time.LoadLocation(schedule.TimeZone)
	if err != nil || schedule.TimeZone == "" || schedule.TimeZone == "Local" {
		return calendar{}, fmt.Errorf("select a valid IANA time zone")
	}
	clock, err := time.Parse("15:04", schedule.LocalTime)
	if err != nil || clock.Format("15:04") != schedule.LocalTime {
		return calendar{}, fmt.Errorf("enter a local time in HH:MM")
	}
	if schedule.Frequency != "daily" && schedule.Frequency != "weekly" {
		return calendar{}, fmt.Errorf("select daily or weekly frequency")
	}
	if schedule.Weekday < 0 || schedule.Weekday > 6 {
		return calendar{}, fmt.Errorf("select a valid weekday")
	}
	return calendar{zone: zone, hour: clock.Hour(), minute: clock.Minute(),
		weekly: schedule.Frequency == "weekly", weekday: time.Weekday(schedule.Weekday)}, nil
}

func (c calendar) occurrence(day time.Time) time.Time {
	if c.weekly && day.Weekday() != c.weekday {
		return time.Time{}
	}
	wall := day.Add(time.Duration(c.hour)*time.Hour + time.Duration(c.minute)*time.Minute)
	var first time.Time
	for _, sample := range []time.Time{wall.Add(-36 * time.Hour), wall, wall.Add(36 * time.Hour)} {
		_, offset := sample.In(c.zone).Zone()
		instant := wall.Add(-time.Duration(offset) * time.Second)
		local := instant.In(c.zone)
		if local.Year() == day.Year() && local.YearDay() == day.YearDay() &&
			local.Hour() == c.hour && local.Minute() == c.minute && (first.IsZero() || instant.Before(first)) {
			first = instant
		}
	}
	return first
}

func (c calendar) adjacent(now time.Time, step int) time.Time {
	year, month, dayOfMonth := now.In(c.zone).Date()
	day := time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
	for tries := 0; tries < 21; tries++ {
		instant := c.occurrence(day)
		if !instant.IsZero() && instant.After(now) == (step > 0) {
			return instant
		}
		day = day.AddDate(0, 0, step)
	}
	return time.Time{}
}
