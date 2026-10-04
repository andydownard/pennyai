package main

import (
	"fmt"
	"time"
)

// dbTimeLayouts are the shapes a timestamp read from the database as a string
// can take. Which one arrives depends on the driver and on the SQL:
//
//   - a TIMESTAMPTZ column scanned straight into a string arrives as RFC 3339
//   - the same column cast to TEXT (what pgcompat does inside COALESCE(x, ''))
//     arrives as "2006-01-02 15:04:05.999999+00"
//   - SQLite (the default test database) stores "2006-01-02 15:04:05"
var dbTimeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999Z07",
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
}

// parseDBTime parses a timestamp string read from the database. Every caller
// that compares a stored time against now must use this; parsing with a single
// SQLite layout fails on PostgreSQL, and the callers treat a failed parse as
// "no timestamp", which silently switches off dedup windows and cooling
// periods.
func parseDBTime(s string) (time.Time, error) {
	for _, layout := range dbTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized database timestamp %q", s)
}
