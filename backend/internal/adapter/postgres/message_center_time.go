package postgres

import "time"

const messageCenterTimeZone = "Asia/Shanghai"

var messageCenterLocation = time.FixedZone(messageCenterTimeZone, 8*60*60)

// messageCenterWallTime interprets a PostgreSQL timestamp without time zone as Beijing time.
func messageCenterWallTime(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	return time.Date(
		value.Year(), value.Month(), value.Day(),
		value.Hour(), value.Minute(), value.Second(), value.Nanosecond(),
		messageCenterLocation,
	)
}

func messageCenterInstant(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	return value.In(messageCenterLocation)
}
