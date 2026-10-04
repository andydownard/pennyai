package main

import (
	"testing"
	"time"
)

func TestParseDBTime_AcceptsEveryShapeTheDatabaseReturns(t *testing.T) {
	want := time.Date(2026, 10, 3, 21, 48, 44, 0, time.UTC)
	for _, in := range []string{
		"2026-10-03 21:48:44",           // SQLite
		"2026-10-03T21:48:44Z",          // TIMESTAMPTZ scanned to string
		"2026-10-03T21:48:44.000000Z",   // with fractional seconds
		"2026-10-03 21:48:44+00",        // TIMESTAMPTZ::TEXT
		"2026-10-03 21:48:44.000000+00", // TIMESTAMPTZ::TEXT with micros
		"2026-10-03 15:48:44-06",        // non-UTC session time zone
		"2026-10-03 21:48:44+00:00",
		"2026-10-03T21:48:44",
	} {
		got, err := parseDBTime(in)
		if err != nil {
			t.Errorf("parseDBTime(%q) failed: %v", in, err)
			continue
		}
		if !got.Equal(want) {
			t.Errorf("parseDBTime(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseDBTime_RejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "yesterday", "2026-10-03"} {
		if _, err := parseDBTime(in); err == nil {
			t.Errorf("parseDBTime(%q) should fail", in)
		}
	}
}

func TestConvertSQL_CoalesceWithEmptyStringCastsToText(t *testing.T) {
	cases := map[string]string{
		"SELECT COALESCE(ends_at,'') FROM d":              "SELECT COALESCE((ends_at)::TEXT,'') FROM d",
		"SELECT COALESCE(u.updated_at, '') FROM u":        "SELECT COALESCE((u.updated_at)::TEXT, '') FROM u",
		"SELECT COALESCE(MAX(created_at), '') FROM u":     "SELECT COALESCE((MAX(created_at))::TEXT, '') FROM u",
		"SELECT coalesce(a, '') , COALESCE(b,'x') FROM u": "SELECT coalesce((a)::TEXT, '') , COALESCE(b,'x') FROM u",
		"SELECT COALESCE(n, 0), COALESCE(j, '[]') FROM u": "SELECT COALESCE(n, 0), COALESCE(j, '[]') FROM u",
		"SELECT COALESCE(a, b, '') FROM u":                "SELECT COALESCE(a, b, '') FROM u",
		"SELECT COALESCE(NULLIF(a, ''), '') FROM u":       "SELECT COALESCE((NULLIF(a, ''))::TEXT, '') FROM u",
		"SELECT COALESCE(COALESCE(a,''), '') FROM u":      "SELECT COALESCE((COALESCE((a)::TEXT,''))::TEXT, '') FROM u",
		"SELECT 'COALESCE(a, '''')' FROM u":               "SELECT 'COALESCE(a, '''')' FROM u",
		"UPDATE u SET x = COALESCE(?, '') WHERE id = ?":   "UPDATE u SET x = COALESCE(($1)::TEXT, '') WHERE id = $2",
	}
	for in, want := range cases {
		if got := convertSQL(in); got != want {
			t.Errorf("convertSQL(%q)\n  got  %q\n  want %q", in, got, want)
		}
	}
}

func TestConvertSQL_DateFunction(t *testing.T) {
	const utcNow = "(NOW() AT TIME ZONE 'UTC')"
	cases := map[string]string{
		// One argument: SQLite returns 'YYYY-MM-DD' text in UTC.
		"SELECT date(created_at) FROM t":                "SELECT TO_CHAR((created_at) AT TIME ZONE 'UTC', 'YYYY-MM-DD') FROM t",
		"SELECT 1 FROM t WHERE DATE(bt.created_at) = ?": "SELECT 1 FROM t WHERE TO_CHAR((bt.created_at) AT TIME ZONE 'UTC', 'YYYY-MM-DD') = $1",
		"SELECT date('now')":                            "SELECT TO_CHAR(" + utcNow + ", 'YYYY-MM-DD')",
		// 'now' with a modifier, compared against TEXT date columns.
		"DELETE FROM s WHERE trip_date < date('now', '-2 days')": "DELETE FROM s WHERE trip_date < TO_CHAR(" + utcNow + " + INTERVAL '-2 days', 'YYYY-MM-DD')",
		"SELECT 1 FROM m WHERE date >= DATE('now', ?)":           "SELECT 1 FROM m WHERE date >= TO_CHAR(" + utcNow + " + CAST($1 AS INTERVAL), 'YYYY-MM-DD')",
		// A parameter base with a modifier, compared against a timestamp.
		"SELECT 1 FROM t WHERE created_at >= DATE(?, '-6 days') AND DATE(created_at) <= ?": "SELECT 1 FROM t WHERE created_at >= (CAST($1 AS DATE) + INTERVAL '-6 days') AND TO_CHAR((created_at) AT TIME ZONE 'UTC', 'YYYY-MM-DD') <= $2",
		// Not the date() function.
		"SELECT trip_date, update_date(x) FROM t": "SELECT trip_date, update_date(x) FROM t",
		"SELECT datetime('now')":                  "SELECT NOW()",
		"SELECT 'date(x)' FROM t":                 "SELECT 'date(x)' FROM t",
	}
	for in, want := range cases {
		if got := convertSQL(in); got != want {
			t.Errorf("convertSQL(%q)\n  got  %q\n  want %q", in, got, want)
		}
	}
}
