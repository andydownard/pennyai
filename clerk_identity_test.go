package main

// clerk_identity_test.go — upsertClerkUser and its helpers, called directly.
// The exchange-level behaviour is in handlers_clerk_test.go.

import (
	"database/sql"
	"errors"
	"testing"
)

func countRows(t *testing.T, s *Server, query string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("counting (%s): %v", query, err)
	}
	return n
}

func TestUpsertClerkUser_RequiresClerkID(t *testing.T) {
	server := newTestServer(t)
	if _, _, err := upsertClerkUser(server.db, ClerkIdentity{}); err == nil {
		t.Fatal("want an error for an empty clerk id")
	}
	if n := countRows(t, server, "SELECT COUNT(*) FROM users"); n != 0 {
		t.Errorf("users rows: got %d want 0", n)
	}
}

func TestUpsertClerkUser_MintsThenRepeats(t *testing.T) {
	server := newTestServer(t)
	id := ClerkIdentity{ClerkUserID: "user_u1", Email: " Pat@Example.com ", Name: "Pat"}

	u1, by1, err := upsertClerkUser(server.db, id)
	if err != nil || by1 != "" {
		t.Fatalf("first: err=%v mappedBy=%q want \"\"", err, by1)
	}
	u2, by2, err := upsertClerkUser(server.db, id)
	if err != nil || by2 != "clerk" {
		t.Fatalf("second: err=%v mappedBy=%q want clerk", err, by2)
	}
	if u1.ID != u2.ID {
		t.Errorf("same sub must give the same user: %s vs %s", u1.ID, u2.ID)
	}
	if u1.Email != "pat@example.com" || u1.Name != "Pat" || u1.HouseholdID == 0 {
		t.Errorf("user: %+v", u1)
	}
	for _, q := range []string{
		"SELECT COUNT(*) FROM users",
		"SELECT COUNT(*) FROM households",
		"SELECT COUNT(*) FROM household_members",
		"SELECT COUNT(*) FROM auth_methods WHERE provider = 'clerk'",
	} {
		if n := countRows(t, server, q); n != 1 {
			t.Errorf("%s: got %d want 1", q, n)
		}
	}
	if n := countRows(t, server, "SELECT login_count FROM users WHERE id = ?", u1.ID); n != 2 {
		t.Errorf("login_count: got %d want 2", n)
	}
}

func TestUpsertClerkUser_NameIsKeptEmailFollowsClerk(t *testing.T) {
	server := newTestServer(t)
	u, _, _ := upsertClerkUser(server.db, ClerkIdentity{ClerkUserID: "user_n", Email: "old@example.com", Name: "First"})
	if _, _, err := upsertClerkUser(server.db, ClerkIdentity{ClerkUserID: "user_n", Email: "new@example.com", Name: "Second"}); err != nil {
		t.Fatalf("repeat: %v", err)
	}
	var email, name string
	server.db.QueryRow("SELECT email, name FROM users WHERE id = ?", u.ID).Scan(&email, &name)
	if email != "new@example.com" {
		t.Errorf("email follows the Rally account: got %q", email)
	}
	if name != "First" {
		t.Errorf("a name already set is kept: got %q", name)
	}
}

func TestUpsertClerkUser_NeverAdoptsOrStoresAPhone(t *testing.T) {
	// A legacy row holds a phone. The token carries the same phone, verified.
	// Phones are not an identity signal any more: the legacy row stays as it
	// was, a new row is minted, and that row holds no phone.
	server := newTestServer(t)
	legacy, _ := upsertUser(server.db, "5558880000")

	u, by, err := upsertClerkUser(server.db, ClerkIdentity{ClerkUserID: "user_ad", Email: "ad@example.com"})
	if err != nil || by != "" {
		t.Fatalf("err=%v mappedBy=%q want a fresh mint", err, by)
	}
	if u.ID == legacy.ID {
		t.Fatalf("must not adopt the phone-only row %s", legacy.ID)
	}
	var clerk sql.NullString
	server.db.QueryRow("SELECT clerk_user_id FROM users WHERE id = ?", legacy.ID).Scan(&clerk)
	if clerk.Valid {
		t.Errorf("legacy row was bound to %q", clerk.String)
	}
	var phone sql.NullString
	server.db.QueryRow("SELECT phone FROM users WHERE id = ?", u.ID).Scan(&phone)
	if phone.Valid && phone.String != "" {
		t.Errorf("minted row stores phone %q", phone.String)
	}
	if n := countRows(t, server, "SELECT COUNT(*) FROM users"); n != 2 {
		t.Errorf("users rows: got %d want 2", n)
	}
}

func TestUpsertClerkUser_ConflictIsReportedNotOverwritten(t *testing.T) {
	// A row bound to Clerk user A is never rebound: B gets its own row.
	server := newTestServer(t)
	a, _, _ := upsertClerkUser(server.db, ClerkIdentity{ClerkUserID: "user_a", Email: "same@example.com"})
	b, _, err := upsertClerkUser(server.db, ClerkIdentity{ClerkUserID: "user_b", Email: "same@example.com"})
	if errors.Is(err, errClerkAccountConflict) || err != nil {
		t.Fatalf("B should be minted cleanly, got %v", err)
	}
	if a.ID == b.ID {
		t.Errorf("A=%+v B=%+v", a, b)
	}
	var clerk string
	server.db.QueryRow("SELECT clerk_user_id FROM users WHERE id = ?", a.ID).Scan(&clerk)
	if clerk != "user_a" {
		t.Errorf("A's binding changed to %q", clerk)
	}
}
