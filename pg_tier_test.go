package main

// Regression tests pinning two bugs that made every sign-in fail in production
// while an earlier SQLite-backed suite stayed green (see pg_testdb_test.go).

import (
	"testing"
)

func pgCount(t *testing.T, db *PGDB, query string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	return n
}

// users.id is a TEXT UUID. The wrapper used to append RETURNING id, fail to
// scan the UUID into an int64, and then run the INSERT a second time.
func TestPG_InsertIntoTextIDTable_RunsExactlyOnce(t *testing.T) {
	_, db := openTestDB(t)

	res, err := db.Exec("INSERT INTO users (id, phone) VALUES (?, ?)", "11111111-1111-1111-1111-111111111111", "2125550001")
	if err != nil {
		t.Fatalf("plain INSERT into a TEXT-id table must succeed, got: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Errorf("RowsAffected = %d, want 1", n)
	}
	if n := pgCount(t, db, "SELECT COUNT(*) FROM users WHERE phone = ?", "2125550001"); n != 1 {
		t.Errorf("rows inserted = %d, want exactly 1", n)
	}
}

// A statement that errors inside a Postgres transaction aborts the whole
// transaction. The wrapper must not probe with a doomed RETURNING id.
func TestPG_InsertInTx_TableWithoutIDColumn_DoesNotAbortTx(t *testing.T) {
	_, db := openTestDB(t)
	db.Exec("INSERT INTO users (id, phone) VALUES (?, ?)", "u-tx", "2125550002")

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		"INSERT INTO sessions (token, phone, user_id, role, expires_at) VALUES (?, ?, ?, 'user', datetime('now', '+1 days'))",
		"tok-tx", "2125550002", "u-tx",
	); err != nil {
		t.Fatalf("INSERT into sessions (no id column) failed: %v", err)
	}
	if _, err := tx.Exec("UPDATE users SET name = ? WHERE id = ?", "after", "u-tx"); err != nil {
		t.Fatalf("transaction was aborted by the preceding INSERT: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if n := pgCount(t, db, "SELECT COUNT(*) FROM sessions WHERE token = ?", "tok-tx"); n != 1 {
		t.Errorf("sessions rows = %d, want 1", n)
	}
}

func TestPG_InsertIntoSerialTable_ReturnsLastInsertID(t *testing.T) {
	_, db := openTestDB(t)
	db.Exec("INSERT INTO users (id, phone) VALUES (?, ?)", "u-serial", "2125550003")

	res, err := db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", "u-serial", "milk", "Dairy")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	if id <= 0 {
		t.Errorf("LastInsertId = %d, want a generated id", id)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Errorf("RowsAffected = %d, want 1", n)
	}
	if n := pgCount(t, db, "SELECT COUNT(*) FROM list_items WHERE id = ?", id); n != 1 {
		t.Errorf("row with returned id not found")
	}
}

// Idempotency guards across the codebase read RowsAffected after INSERT OR
// IGNORE. A conflict must report 0 and must not insert.
func TestPG_InsertOrIgnore_ConflictReportsZeroRows(t *testing.T) {
	_, db := openTestDB(t)

	first, err := db.Exec("INSERT OR IGNORE INTO users (id, phone) VALUES (?, ?)", "u-a", "2125550004")
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := first.RowsAffected(); n != 1 {
		t.Errorf("first insert RowsAffected = %d, want 1", n)
	}
	second, err := db.Exec("INSERT OR IGNORE INTO users (id, phone) VALUES (?, ?)", "u-b", "2125550004")
	if err != nil {
		t.Fatalf("conflicting INSERT OR IGNORE must not error: %v", err)
	}
	if n, _ := second.RowsAffected(); n != 0 {
		t.Errorf("conflicting insert RowsAffected = %d, want 0", n)
	}
	if n := pgCount(t, db, "SELECT COUNT(*) FROM users WHERE phone = ?", "2125550004"); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

// The session store used to hold the raw connection and send `?` placeholders
// straight to Postgres: a syntax error on every sign-in.
func TestPG_SessionStore_CreateGetDelete(t *testing.T) {
	s := newTestServer(t)
	s.db.Exec("INSERT INTO users (id, phone) VALUES (?, ?)", "u-sess", "2125550005")

	token, err := s.sessions.CreateSession("u-sess", "2125550005", "clerk")
	if err != nil {
		t.Fatalf("CreateSession failed on Postgres: %v", err)
	}
	// Fresh store: bypass the in-memory cache so the read hits the database.
	fresh := NewSessionStore(s.db)
	got, ok := fresh.GetSession(token)
	if !ok {
		t.Fatal("GetSession did not find the session just created")
	}
	if got.UserID != "u-sess" {
		t.Errorf("session user = %q, want u-sess", got.UserID)
	}
	fresh.DeleteSession(token)
	if _, ok := NewSessionStore(s.db).GetSession(token); ok {
		t.Error("session still readable after DeleteSession")
	}
}
