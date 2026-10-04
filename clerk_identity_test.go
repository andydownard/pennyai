package main

// clerk_identity_test.go — upsertClerkUser and its helpers, called directly.
// The exchange-level behaviour is in handlers_clerk_test.go.

import (
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

func TestClerkVerifiedPhone(t *testing.T) {
	cases := []struct {
		name string
		id   ClerkIdentity
		want string
	}{
		{"verified e164", ClerkIdentity{Phone: "+15551234567", PhoneVerified: true}, "5551234567"},
		{"verified formatted", ClerkIdentity{Phone: "(555) 123-4567", PhoneVerified: true}, "5551234567"},
		{"verified with leading 1", ClerkIdentity{Phone: "15551234567", PhoneVerified: true}, "5551234567"},
		{"unverified", ClerkIdentity{Phone: "+15551234567"}, ""},
		{"verified malformed", ClerkIdentity{Phone: "555", PhoneVerified: true}, ""},
		{"verified empty", ClerkIdentity{PhoneVerified: true}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clerkVerifiedPhone(c.id); got != c.want {
				t.Errorf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestUpsertClerkUser_RequiresClerkID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	if _, _, err := upsertClerkUser(server.db, ClerkIdentity{}); err == nil {
		t.Fatal("want an error for an empty clerk id")
	}
	if n := countRows(t, server, "SELECT COUNT(*) FROM users"); n != 0 {
		t.Errorf("users rows: got %d want 0", n)
	}
}

func TestUpsertClerkUser_MintsThenRepeats(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
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
	server := newTestServer(t, &MockTwilioClient{})
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

func TestUpsertClerkUser_AdoptsPhoneOnlyRow(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	texted, _ := upsertUser(server.db, "5558880000")

	u, by, err := upsertClerkUser(server.db, ClerkIdentity{ClerkUserID: "user_ad", Phone: "+15558880000", PhoneVerified: true})
	if err != nil || by != "phone" {
		t.Fatalf("err=%v mappedBy=%q want phone", err, by)
	}
	if u.ID != texted.ID {
		t.Errorf("must adopt %s, got %s", texted.ID, u.ID)
	}
	if n := countRows(t, server, "SELECT COUNT(*) FROM users"); n != 1 {
		t.Errorf("users rows: got %d want 1", n)
	}
}

func TestUpsertClerkUser_ConflictIsReportedNotOverwritten(t *testing.T) {
	// A row bound to Clerk user A is never rebound: B gets its own row.
	server := newTestServer(t, &MockTwilioClient{})
	a, _, _ := upsertClerkUser(server.db, ClerkIdentity{ClerkUserID: "user_a", Phone: "5557770000", PhoneVerified: true})
	b, _, err := upsertClerkUser(server.db, ClerkIdentity{ClerkUserID: "user_b", Phone: "5557770000", PhoneVerified: true})
	if errors.Is(err, errClerkAccountConflict) || err != nil {
		t.Fatalf("B should be minted cleanly, got %v", err)
	}
	if a.ID == b.ID || b.PhoneNumber != "" {
		t.Errorf("A=%+v B=%+v", a, b)
	}
	var clerk string
	server.db.QueryRow("SELECT clerk_user_id FROM users WHERE id = ?", a.ID).Scan(&clerk)
	if clerk != "user_a" {
		t.Errorf("A's binding changed to %q", clerk)
	}
}
