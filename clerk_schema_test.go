// clerk_schema_test.go — STEP 1 of the Clerk satellite-daughter work.
//
// users.clerk_user_id is the join key between a Clerk identity (the `sub`
// claim on a Clerk session JWT) and pennyai's local users row. It is:
//
//   - additive (ALTER TABLE ADD COLUMN via addColumnIfNotExists — never a
//     table recreate, per CLAUDE.md Database Rules)
//   - nullable with no default, so every pre-existing row stays valid
//   - protected by a PARTIAL unique index
//     (WHERE clerk_user_id IS NOT NULL) rather than a column UNIQUE
//     constraint, so unlimited NULLs coexist and the migration cannot fail on
//     an existing database
//
// The partial index is the real backstop against two local users claiming one
// Clerk identity.
//
// The shape assertions here are textual — the same approach
// schema_init_test.go takes — while the behavioural assertions run against
// real PostgreSQL built by initDB.
package main

import (
	"os"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------
// Shape: schema.go declares the column additively and the partial index
// -----------------------------------------------------------------------

func TestSchema_ClerkUserID_IsAdditiveColumnWithPartialUniqueIndex(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatalf("read schema.go: %v", err)
	}
	body := string(src)

	// The column must be declared inside the users ALTER loop, which runs
	// through addColumnIfNotExists — never as part of a CREATE TABLE
	// recreate.
	if !strings.Contains(body, `{"clerk_user_id", "TEXT"}`) {
		t.Fatalf(`schema.go users ALTER loop missing {"clerk_user_id", "TEXT"} — the column must be added via addColumnIfNotExists, not a table recreate`)
	}

	// Nullable, no default: a NOT NULL or DEFAULT would break existing rows
	// and would make "no Clerk identity" indistinguishable from one.
	if strings.Contains(body, `{"clerk_user_id", "TEXT NOT NULL`) || strings.Contains(body, `{"clerk_user_id", "TEXT DEFAULT`) {
		t.Errorf("clerk_user_id must be nullable with no default so pre-existing rows stay valid")
	}

	idx := strings.Index(body, "CREATE UNIQUE INDEX IF NOT EXISTS idx_users_clerk_user_id ON users(clerk_user_id) WHERE clerk_user_id IS NOT NULL")
	if idx < 0 {
		t.Fatalf("schema.go missing the PARTIAL unique index on users(clerk_user_id) — without WHERE clerk_user_id IS NOT NULL the second NULL row would collide on some engines and the migration could fail")
	}

	usersAlter := strings.Index(body, `{"clerk_user_id", "TEXT"}`)
	if idx < usersAlter {
		t.Errorf("the unique index at %d is declared before the column at %d — index creation would fail on a fresh database", idx, usersAlter)
	}

	// Never a column-level UNIQUE constraint: that is a table-shape change.
	if strings.Contains(body, "clerk_user_id TEXT UNIQUE") {
		t.Errorf("clerk_user_id must not use a column UNIQUE constraint — use the partial index so unlimited NULLs coexist")
	}
}

// -----------------------------------------------------------------------
// Behaviour (PostgreSQL): positive, negative, idempotency, isolation
// -----------------------------------------------------------------------

// 1. positive — a user row carries a Clerk identity and reads back.
func TestSchema_ClerkUserID_StoresAndReadsBack(t *testing.T) {
	_, db := openTestDB(t)

	if _, err := db.Exec("INSERT INTO users (id, clerk_user_id) VALUES (?, ?)", "u-a", "user_a"); err != nil {
		t.Fatalf("inserting user with clerk_user_id: %v", err)
	}

	var got string
	if err := db.QueryRow("SELECT clerk_user_id FROM users WHERE id = ?", "u-a").Scan(&got); err != nil {
		t.Fatalf("reading clerk_user_id back: %v", err)
	}
	if got != "user_a" {
		t.Errorf("clerk_user_id: got %q want %q", got, "user_a")
	}
}

// 2. negative — two local users cannot claim one Clerk identity.
func TestSchema_ClerkUserID_DuplicateRejected(t *testing.T) {
	_, db := openTestDB(t)

	if _, err := db.Exec("INSERT INTO users (id, clerk_user_id) VALUES (?, ?)", "u-a", "user_a"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err := db.Exec("INSERT INTO users (id, clerk_user_id) VALUES (?, ?)", "u-b", "user_a")
	if err == nil {
		t.Fatal("a second users row with the same clerk_user_id was accepted — one Clerk identity must map to at most one local user")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Errorf("expected a unique-constraint violation, got: %v", err)
	}

	var n int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE clerk_user_id = ?", "user_a").Scan(&n)
	if n != 1 {
		t.Errorf("rows holding user_a: got %d want 1", n)
	}
}

// 3. idempotency — re-running schema init is a no-op and leaves rows alone.
func TestSchema_ClerkUserID_InitIsIdempotent(t *testing.T) {
	rawDB, db := openTestDB(t)

	if _, err := db.Exec("INSERT INTO users (id, phone, clerk_user_id) VALUES (?, ?, ?)", "u-keep", "5550001111", "user_keep"); err != nil {
		t.Fatalf("seeding pre-existing row: %v", err)
	}

	// Second init on the same database: must not error, must not drop data.
	if err := initDB(rawDB); err != nil {
		t.Fatalf("second schema init returned error: %v", err)
	}

	var clerkID, phone string
	if err := db.QueryRow("SELECT clerk_user_id, phone FROM users WHERE id = ?", "u-keep").Scan(&clerkID, &phone); err != nil {
		t.Fatalf("pre-existing row did not survive re-init: %v", err)
	}
	if clerkID != "user_keep" || phone != "5550001111" {
		t.Errorf("row mutated by re-init: clerk_user_id=%q phone=%q", clerkID, phone)
	}

	// The uniqueness guard still holds after a second init (index not dropped
	// and not duplicated into a plain index).
	if _, err := db.Exec("INSERT INTO users (id, clerk_user_id) VALUES (?, ?)", "u-dup", "user_keep"); err == nil {
		t.Error("uniqueness guard lost after re-init — duplicate clerk_user_id accepted")
	}
}

// 4. isolation — the index is partial: many NULLs coexist, and distinct
//    Clerk identities coexist. Legacy rows (every row today) are untouched.
func TestSchema_ClerkUserID_NullsAndDistinctIdentitiesCoexist(t *testing.T) {
	_, db := openTestDB(t)

	for _, id := range []string{"legacy-1", "legacy-2", "legacy-3"} {
		if _, err := db.Exec("INSERT INTO users (id) VALUES (?)", id); err != nil {
			t.Fatalf("inserting legacy row %s with NULL clerk_user_id: %v", id, err)
		}
	}

	if _, err := db.Exec("INSERT INTO users (id, clerk_user_id) VALUES (?, ?)", "u-a", "user_a"); err != nil {
		t.Fatalf("inserting user_a: %v", err)
	}
	if _, err := db.Exec("INSERT INTO users (id, clerk_user_id) VALUES (?, ?)", "u-b", "user_b"); err != nil {
		t.Fatalf("distinct clerk_user_id rejected — the index must be on the value, not the column's presence: %v", err)
	}

	var nulls int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE clerk_user_id IS NULL").Scan(&nulls)
	if nulls != 3 {
		t.Errorf("rows with NULL clerk_user_id: got %d want 3 (partial index must not restrict NULLs)", nulls)
	}

	var bound int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE clerk_user_id IS NOT NULL").Scan(&bound)
	if bound != 2 {
		t.Errorf("rows bound to a Clerk identity: got %d want 2", bound)
	}
}
