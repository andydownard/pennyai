package main

import (
	"database/sql"
	"testing"
)

// pgSchemaCounts reports table, column and index counts for the current schema.
func pgSchemaCounts(t *testing.T, db *sql.DB) (tables, columns, indexes int) {
	t.Helper()
	q := func(query string) int {
		var n int
		if err := db.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	tables = q(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema()`)
	columns = q(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema()`)
	indexes = q(`SELECT COUNT(*) FROM pg_indexes WHERE schemaname = current_schema()`)
	return
}

// A server restart re-runs initDB against the live database. It must succeed
// every time and must not add, drop or duplicate anything.
func TestPostgres_InitDB_RunsRepeatedlyWithStableSchema(t *testing.T) {
	raw, _ := openTestDB(t)

	if _, err := raw.Exec("INSERT INTO users (id, phone) VALUES (?, ?)", "u-keep", "5550009999"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t0, c0, i0 := pgSchemaCounts(t, raw)
	for run := 1; run <= 3; run++ {
		if err := initDB(raw); err != nil {
			t.Fatalf("initDB run %d: %v", run, err)
		}
		tn, cn, in := pgSchemaCounts(t, raw)
		if tn != t0 || cn != c0 || in != i0 {
			t.Fatalf("run %d changed schema: tables %d->%d columns %d->%d indexes %d->%d", run, t0, tn, c0, cn, i0, in)
		}
	}
	var n int
	if err := raw.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", "u-keep").Scan(&n); err != nil || n != 1 {
		t.Fatalf("seeded row lost across re-init: n=%d err=%v", n, err)
	}
}
