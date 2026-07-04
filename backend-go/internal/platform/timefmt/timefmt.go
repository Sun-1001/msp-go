package timefmt

import "time"

const (
	dateLayout           = "2006-01-02"
	dateTimeMicrosLayout = "2006-01-02T15:04:05.999999"
)

// Date formats a calendar date using the API response date layout.
func Date(value time.Time) string {
	return value.Format(dateLayout)
}

// DateTimeMicros formats a timestamp using the API response microsecond layout.
func DateTimeMicros(value time.Time) string {
	return value.Format(dateTimeMicrosLayout)
}

// OptionalDateTimeMicros formats a timestamp pointer or returns nil when absent.
func OptionalDateTimeMicros(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := DateTimeMicros(*value)
	return &formatted
}
