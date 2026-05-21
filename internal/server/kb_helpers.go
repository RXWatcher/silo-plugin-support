package server

import "time"

func timeNow() time.Time { return time.Now() }

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
