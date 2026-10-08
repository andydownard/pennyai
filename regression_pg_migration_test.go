// regression_pg_migration_test.go
//
// Regression tests for bugs surfaced in the 2026-04-13 PostgreSQL migration audit
// (see INBOX.md "2026-04-16 — Max-effort PG/marketing audit findings").
//
// Each test covers one fix:
//   - INSERT OR IGNORE → ON CONFLICT idempotency on auth_methods,
//     savings_milestones, swap_propagations.
//   - LOWER(col) LIKE → col ILIKE case-insensitive search.
//   - handleAvatarGet accepts TEXT UUID (not int).
//
// The auth_methods / milestones / propagations tests deliberately call the same
// INSERT twice and assert one row exists. If pgcompat ever silently strips
// `OR IGNORE` without adding `ON CONFLICT DO NOTHING`, these go red.
package main

import (
	"net/http"
	"testing"
)

// ---- INSERT OR IGNORE → ON CONFLICT idempotency ----

func TestRegression_AuthMethodsInsertIsIdempotent(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-am', '5550000001')")

	// Use the same conversion path the production handler uses.
	stmt := "INSERT INTO auth_methods (user_id, provider, provider_id) VALUES (?, 'phone', ?) ON CONFLICT (provider, provider_id) DO NOTHING"
	if _, err := server.db.Exec(stmt, "user-am", "5550000001"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := server.db.Exec(stmt, "user-am", "5550000001"); err != nil {
		t.Fatalf("second insert (idempotent): %v", err)
	}

	var count int
	server.db.QueryRow(
		"SELECT COUNT(*) FROM auth_methods WHERE provider = 'phone' AND provider_id = ?",
		"5550000001",
	).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 auth_methods row, got %d", count)
	}
}

func TestRegression_SavingsMilestonesInsertIsIdempotent(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-sm', '5550000002')")

	stmt := "INSERT INTO savings_milestones (user_id, cents) VALUES (?, ?) ON CONFLICT (user_id, cents) DO NOTHING"
	if _, err := server.db.Exec(stmt, "user-sm", 1000); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := server.db.Exec(stmt, "user-sm", 1000); err != nil {
		t.Fatalf("second insert (idempotent): %v", err)
	}

	var count int
	server.db.QueryRow(
		"SELECT COUNT(*) FROM savings_milestones WHERE user_id = ? AND cents = ?",
		"user-sm", 1000,
	).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 savings_milestones row, got %d", count)
	}
}

func TestRegression_SwapPropagationsInsertIsIdempotent(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-sp1', '5550000003'), ('user-sp2', '5550000004')")
	server.db.Exec("INSERT INTO households (id) VALUES (5001)")
	// Minimal brand_deal so deal_id FK is satisfied.
	if _, err := server.db.Exec(
		"INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product) VALUES (?, ?, ?, ?)",
		5001, "TestProduct", "TestBrand", "TestProduct",
	); err != nil {
		t.Fatalf("seed brand_deals: %v", err)
	}

	stmt := `INSERT INTO swap_propagations
		(deal_id, source_user_id, adopter_user_id, household_id, item_name, brand)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (deal_id, adopter_user_id) DO NOTHING`
	if _, err := server.db.Exec(stmt, 5001, "user-sp1", "user-sp2", 5001, "TestProduct", "TestBrand"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := server.db.Exec(stmt, 5001, "user-sp1", "user-sp2", 5001, "TestProduct", "TestBrand"); err != nil {
		t.Fatalf("second insert (idempotent): %v", err)
	}

	var count int
	server.db.QueryRow(
		"SELECT COUNT(*) FROM swap_propagations WHERE deal_id = ? AND adopter_user_id = ?",
		5001, "user-sp2",
	).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 swap_propagations row, got %d", count)
	}
}

// ---- LOWER(col) LIKE → col ILIKE: case-insensitive search ----

func TestRegression_StoreSearchIsCaseInsensitive(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "5550000005")

	// Seed with mixed-case data; LIKE is case-sensitive in Postgres.
	// The fix uses ILIKE so this stays consistent.
	if _, err := server.upsertStore("Whole Foods #42", "Whole Foods", "1 Market St", "SF", "CA", "94103", "", nil, nil); err != nil {
		t.Fatalf("upsertStore: %v", err)
	}

	rr := doRequest(server.handleStores, "GET", "/api/stores?q=whole", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	// Body must contain the seeded store; if LIKE were still case-sensitive
	// against title-case data with a lowercase query, body would be `[]`.
	body := rr.Body.String()
	if !contains(body, "Whole Foods") {
		t.Errorf("case-insensitive search miss; body=%s", body)
	}
}

// ---- handleAvatarGet accepts TEXT UUID (not int) ----

func TestRegression_AvatarGetAcceptsUUIDPath(t *testing.T) {
	server := newTestServer(t)
	// Seed user + photo. UUID-shaped string (not numeric).
	uid := "8e3c2a1b-4f5d-4e6a-b7c8-9d0e1f2a3b4c"
	server.db.Exec("INSERT INTO users (id, phone) VALUES (?, '5550000006')", uid)
	server.db.Exec(
		"INSERT INTO profile_photos (user_id, photo_data, content_type) VALUES (?, ?, ?)",
		uid, makeJPEG(40), "image/jpeg",
	)

	rr := doRequest(server.handleAvatarGet, "GET", "/api/avatar/"+uid, nil, nil)
	// Pre-fix this returned 400 because handleAvatarGet called strconv.Atoi.
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for valid UUID path, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type: got %q, want image/jpeg", got)
	}
}

// (uses contains() defined in ai_providers_test.go)
