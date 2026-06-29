// schema_init_test.go
//
// Regression test for the 2026-04-18 staging crash. initDB assumed timpager's
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

	if strings.Contains(body, "SHARED USERS TABLE — owned by timpager") {
		t.Fatalf("schema.go still claims users table is owned by timpager — CLAUDE.md says pennyai owns its own users/sessions since 2026-04-18")
	}
	if !strings.Contains(body, "USERS TABLE — owned by pennyai") {
		t.Fatalf("schema.go should declare USERS TABLE — owned by pennyai as the section header")
	}
}
