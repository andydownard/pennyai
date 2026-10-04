// schema_init_test.go
//
// Regression test for the 2026-04-18 staging crash. initDB assumed another service's
// shared Postgres already contained a `users` table, so the first ALTER TABLE
// users in a fresh pennyai-owned Postgres raised "relation \"users\" does not
// exist" and schema init aborted before any table was created.
//
// The fix (see schema.go) is for pennyai to own its own users and sessions
// tables via CREATE TABLE IF NOT EXISTS, before any ALTER. This test is
// textual because schema.go runs against Postgres only — the rest of the
// Go test suite runs on SQLite via test_helpers_test.go. A Postgres
// integration check lives in staging deploy verification (DEPLOYMENT.md).
package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestSchema_InitDB_CreatesUsersAndSessionsBeforeAnyAlter(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatalf("read schema.go: %v", err)
	}
	body := string(src)

	usersCreate := strings.Index(body, "CREATE TABLE IF NOT EXISTS users")
	if usersCreate < 0 {
		t.Fatalf("schema.go missing CREATE TABLE IF NOT EXISTS users — PR 1 regression")
	}

	sessionsCreate := strings.Index(body, "CREATE TABLE IF NOT EXISTS sessions")
	if sessionsCreate < 0 {
		t.Fatalf("schema.go missing CREATE TABLE IF NOT EXISTS sessions — PR 1 regression")
	}

	firstUsersAlter := strings.Index(body, `addColumnIfNotExists(db, "users"`)
	if firstUsersAlter < 0 {
		t.Fatalf("schema.go missing addColumnIfNotExists for users — test stale?")
	}
	if firstUsersAlter < usersCreate {
		t.Fatalf("ALTER users at %d appears before CREATE TABLE users at %d — staging will crash", firstUsersAlter, usersCreate)
	}

	listItemsCreate := strings.Index(body, "CREATE TABLE IF NOT EXISTS list_items")
	if listItemsCreate < 0 {
		t.Fatalf("schema.go missing CREATE TABLE IF NOT EXISTS list_items — test stale?")
	}
	if listItemsCreate < usersCreate {
		t.Fatalf("list_items CREATE at %d appears before users CREATE at %d — FK will fail", listItemsCreate, usersCreate)
	}
	if listItemsCreate < sessionsCreate {
		t.Fatalf("list_items CREATE at %d appears before sessions CREATE at %d — shared-DB ordering regressed", listItemsCreate, sessionsCreate)
	}
}

func TestSchema_InitDB_DeclaresSwapAndModIndexes(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatalf("read schema.go: %v", err)
	}
	body := string(src)

	// initDB is PostgreSQL-only (SERIAL, TIMESTAMPTZ); the rest of the suite
	// runs on SQLite via initTestDB, which does not build indexes. So this
	// mirrors the file's other index assertions and checks schema.go's source
	// for the CREATE INDEX statements. Both must live inside the index block
	// (after all CREATE TABLE statements).
	indexHeader := strings.Index(body, "INDEXES (placed after all table creation)")
	if indexHeader < 0 {
		t.Fatalf("schema.go missing the index block header — test stale?")
	}

	swapDealIdx := strings.Index(body, "CREATE INDEX IF NOT EXISTS idx_swap_events_deal_id ON swap_events(deal_id)")
	if swapDealIdx < 0 {
		t.Fatalf("schema.go missing idx_swap_events_deal_id on swap_events(deal_id)")
	}
	if swapDealIdx < indexHeader {
		t.Fatalf("idx_swap_events_deal_id at %d appears before the index block at %d", swapDealIdx, indexHeader)
	}

	modUserIdx := strings.Index(body, "CREATE INDEX IF NOT EXISTS idx_item_modifications_user_id ON item_modifications(user_id)")
	if modUserIdx < 0 {
		t.Fatalf("schema.go missing idx_item_modifications_user_id on item_modifications(user_id)")
	}
	if modUserIdx < indexHeader {
		t.Fatalf("idx_item_modifications_user_id at %d appears before the index block at %d", modUserIdx, indexHeader)
	}
}

func TestSchema_InitDB_DeclaresUsersAsOwnedByPennyai(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatalf("read schema.go: %v", err)
	}
	body := string(src)

	if strings.Contains(body, "SHARED USERS TABLE") {
		t.Fatalf("schema.go still claims the users table is shared — pennyai owns its own users/sessions")
	}
	if !strings.Contains(body, "USERS TABLE — owned by pennyai") {
		t.Fatalf("schema.go should declare USERS TABLE — owned by pennyai as the section header")
	}
}

// TestSchema_UniqueIndexesPresentInTestSchema is the drift guard for the money
// paths.
//
// The Go suite runs against SQLite :memory: with a hand-copied schema in
// test_helpers_test.go. That copy had drifted: production has carried
// idx_referrals_unique since schema.go's index block, but the test schema did
// not, so the referral idempotency tests passed against code that double-credits
// in production. A unique index absent from the test schema is a test suite that
// cannot see the bug it is supposed to be guarding.
//
// Every UNIQUE index declared in schema.go must therefore also be declared in
// test_helpers_test.go. Non-unique indexes are only a performance concern and are
// deliberately not required.
func TestSchema_UniqueIndexesPresentInTestSchema(t *testing.T) {
	schemaSrc, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatalf("read schema.go: %v", err)
	}
	testSrc, err := os.ReadFile("test_helpers_test.go")
	if err != nil {
		t.Fatalf("read test_helpers_test.go: %v", err)
	}

	// Compare index NAMES, not statements: schema.go writes Postgres DDL and the
	// test schema writes SQLite DDL, so the statements differ in whitespace and
	// sometimes in type syntax. The name is the contract.
	//
	// Both sides are matched with the same regex so that a real CREATE UNIQUE
	// INDEX statement is required on each. A substring search would not do —
	// a passing mention in a comment, or a renamed index that merely has the old
	// name as a prefix, would satisfy it and the guard would never fire.
	re := regexp.MustCompile(`(?i)CREATE\s+UNIQUE\s+INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)
	declared := func(src []byte) map[string]bool {
		out := map[string]bool{}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			out[strings.ToLower(m[1])] = true
		}
		return out
	}

	prod := declared(schemaSrc)
	if len(prod) == 0 {
		t.Fatal("found no UNIQUE indexes in schema.go — test stale?")
	}
	test := declared(testSrc)

	for name := range prod {
		if !test[name] {
			t.Errorf("schema.go declares UNIQUE index %s but test_helpers_test.go does not — "+
				"the SQLite test schema cannot catch the duplicate this index prevents. "+
				"Add the equivalent CREATE UNIQUE INDEX to initTestDB.", name)
		}
	}
}
