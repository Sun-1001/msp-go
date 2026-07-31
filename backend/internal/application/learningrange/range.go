// Package learningrange owns the platform learning-statistics calendar contract.
package learningrange

import (
	"errors"
	"time"
)

// Kind identifies a supported learning-statistics range.
type Kind string

const (
	Week     Kind = "week"
	Month    Kind = "month"
	Semester Kind = "semester"
	All      Kind = "all"

	DayInterval  = "day"
	WeekInterval = "week"
)

var (
	// ErrInvalidKind indicates that a range query is unsupported.
	ErrInvalidKind = errors.New("invalid learning range")
	platformZone   = time.FixedZone("Asia/Shanghai", 8*60*60)
)

// Window is one resolved range with a snapshot-safe upper bound.
type Window struct {
	Kind       Kind
	Today      time.Time
	Start      time.Time
	End        time.Time
	SnapshotAt time.Time
	Days       int
	Interval   string
}

// Parse validates a caller-provided range.
func Parse(value string) (Kind, error) {
	switch Kind(value) {
	case Week, Month, Semester, All:
		return Kind(value), nil
	default:
		return "", ErrInvalidKind
	}
}

// ParseOrDefault validates a range or returns fallback for empty/unknown legacy input.
func ParseOrDefault(value string, fallback Kind) Kind {
	kind, err := Parse(value)
	if err != nil {
		return fallback
	}
	return kind
}

// Resolve calculates a range in the platform's fixed Asia/Shanghai calendar.
func Resolve(now time.Time, kind Kind) Window {
	snapshotAt := now.In(platformZone)
	today := time.Date(snapshotAt.Year(), snapshotAt.Month(), snapshotAt.Day(), 0, 0, 0, 0, platformZone)
	start := today.AddDate(0, 0, -mondayIndex(today))
	days := int(today.Sub(start).Hours()/24) + 1
	interval := DayInterval

	switch kind {
	case Month:
		start = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, platformZone)
		days = int(today.Sub(start).Hours()/24) + 1
	case Semester:
		interval = WeekInterval
		if today.Month() >= time.September {
			start = time.Date(today.Year(), time.September, 1, 0, 0, 0, 0, platformZone)
		} else if today.Month() == time.January {
			start = time.Date(today.Year()-1, time.September, 1, 0, 0, 0, 0, platformZone)
		} else {
			start = time.Date(today.Year(), time.February, 1, 0, 0, 0, 0, platformZone)
		}
		days = int(today.Sub(start).Hours()/24) + 1
	case All:
		interval = WeekInterval
		days = 365
		start = today.AddDate(0, 0, -364)
	default:
		kind = Week
	}

	return Window{
		Kind:       kind,
		Today:      today,
		Start:      start,
		End:        snapshotAt,
		SnapshotAt: snapshotAt,
		Days:       days,
		Interval:   interval,
	}
}

// InPlatformZone converts an instant to the platform calendar zone.
func InPlatformZone(value time.Time) time.Time {
	return value.In(platformZone)
}

// StartOfPlatformDayUTC returns the UTC storage boundary for one Asia/Shanghai calendar day.
func StartOfPlatformDayUTC(value time.Time) time.Time {
	local := InPlatformZone(value)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, platformZone).UTC()
}

func mondayIndex(value time.Time) int {
	return (int(value.Weekday()) + 6) % 7
}
