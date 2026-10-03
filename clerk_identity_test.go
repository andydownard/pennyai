package main

// clerk_identity_test.go — STEP 5: the timpager identity-resolve client.
//
// timpager is the AUTHORITY on Rally identity. pennyai never mints a user for a
// Clerk sign-in on its own: users.id must equal the rally_user_id so one person
// is one canonical user across every Rally app. This client is therefore
// read-only and fail-closed — if the authority cannot be reached or rejects the
// token, nothing is created.
//
// Header discipline matters and is asserted here: the SERVICE key travels as
// X-API-Key and the END-USER Clerk JWT as Authorization: Bearer. timpager's
// routes/identity-v1.js treats an Authorization-only request as a legacy
// body-only resolve, and when both are present it verifies the Bearer itself
// and lets its own verified `sub` override the body. Swapping the two would
// silently downgrade to an unauthenticated lookup.

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func testResolveConfig(stub *stubTimpager, apiKey string) Config {
	return Config{TimpagerURL: strings.TrimSuffix(stub.URL, "/"), TimpagerAPIKey: apiKey}
}

// -----------------------------------------------------------------------
// positive
// -----------------------------------------------------------------------

func TestResolveRallyIdentity_Positive(t *testing.T) {
	stub := newStubTimpagerResolve(t, resolveOKBody("rally-user-1", "user_clerk_1", "Tamika@Example.COM ", "+15551234567"), http.StatusOK)

	id := ClerkIdentity{ClerkUserID: "user_clerk_1", Email: "claim@example.com", Phone: "+15559998888"}
	got, err := resolveRallyIdentity(context.Background(), testResolveConfig(stub, "svc_key_abc"), nil, "the.jwt.value", id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !got.Found {
		t.Error("Found should be true when rally_user_id is present")
	}
	if got.RallyUserID != "rally-user-1" {
		t.Errorf("RallyUserID: got %q want rally-user-1", got.RallyUserID)
	}
	if got.ClerkUserID != "user_clerk_1" {
		t.Errorf("ClerkUserID: got %q", got.ClerkUserID)
	}
	// Phone is normalized to the 10-digit form users.phone stores.
	if got.Phone != "5551234567" {
		t.Errorf("Phone: got %q want 5551234567 (must match users.phone storage)", got.Phone)
	}
	// Email is lower-cased and trimmed so email matching is case-insensitive.
	if got.Email != "tamika@example.com" {
		t.Errorf("Email: got %q want tamika@example.com", got.Email)
	}

	reqs := stub.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests: got %d want 1", len(reqs))
	}
	r := reqs[0]
	if r.Method != http.MethodPost {
		t.Errorf("method: got %s want POST", r.Method)
	}
	if r.Path != "/api/v1/identity/resolve" {
		t.Errorf("path: got %q want /api/v1/identity/resolve", r.Path)
	}
	if r.APIKeyHeader != "svc_key_abc" {
		t.Errorf("X-API-Key: got %q want the service key", r.APIKeyHeader)
	}
	if r.Authorization != "Bearer the.jwt.value" {
		t.Errorf("Authorization: got %q want the end-user Clerk JWT as a Bearer", r.Authorization)
	}
	if !strings.HasPrefix(r.ContentType, "application/json") {
		t.Errorf("Content-Type: got %q", r.ContentType)
	}
	if r.Body["clerk_user_id"] != "user_clerk_1" {
		t.Errorf("body clerk_user_id: got %v", r.Body["clerk_user_id"])
	}
	if r.Body["email"] != "claim@example.com" {
		t.Errorf("body email: got %v (the optional custom claims are forwarded)", r.Body["email"])
	}
	if r.Body["phone"] != "+15559998888" {
		t.Errorf("body phone: got %v", r.Body["phone"])
	}
}

func TestResolveRallyIdentity_OmitsEmptyClaimFields(t *testing.T) {
	// Clerk's default session token carries no email/phone. The body must not
	// claim an empty one: timpager matches clerk → phone → email and an empty
	// string could widen the search.
	stub := newStubTimpagerResolve(t, resolveOKBody("rally-2", "user_clerk_2", "", ""), http.StatusOK)

	_, err := resolveRallyIdentity(context.Background(), testResolveConfig(stub, "k"), nil, "jwt", ClerkIdentity{ClerkUserID: "user_clerk_2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body := stub.Requests()[0].Body
	if _, ok := body["phone"]; ok {
		t.Errorf("an empty phone must be omitted from the body, got %#v", body)
	}
	if _, ok := body["email"]; ok {
		t.Errorf("an empty email must be omitted from the body, got %#v", body)
	}
	if body["clerk_user_id"] != "user_clerk_2" {
		t.Errorf("clerk_user_id must always be sent, got %#v", body)
	}
}

func TestResolveRallyIdentity_TrailingSlashOnURLHandled(t *testing.T) {
	stub := newStubTimpagerResolve(t, resolveOKBody("rally-3", "u3", "", ""), http.StatusOK)
	cfg := Config{TimpagerURL: stub.URL + "/", TimpagerAPIKey: "k"}

	if _, err := resolveRallyIdentity(context.Background(), cfg, nil, "jwt", ClerkIdentity{ClerkUserID: "u3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p := stub.Requests()[0].Path; p != "/api/v1/identity/resolve" {
		t.Errorf("path: got %q — a trailing slash on TIMPAGER_URL must not double up", p)
	}
}

// -----------------------------------------------------------------------
// negative
// -----------------------------------------------------------------------

func TestResolveRallyIdentity_Negative(t *testing.T) {
	t.Run("no rally user is not an error", func(t *testing.T) {
		stub := newStubTimpagerResolve(t, resolveOKBody("", "user_new", "", ""), http.StatusOK)
		got, err := resolveRallyIdentity(context.Background(), testResolveConfig(stub, "k"), nil, "jwt", ClerkIdentity{ClerkUserID: "user_new"})
		if err != nil {
			t.Fatalf("rally_user_id null is a normal answer, not an error: %v", err)
		}
		if got.Found {
			t.Error("Found must be false when rally_user_id is null")
		}
		if got.RallyUserID != "" {
			t.Errorf("RallyUserID: got %q want empty", got.RallyUserID)
		}
	})

	t.Run("401 from the authority", func(t *testing.T) {
		stub := newStubTimpagerResolve(t, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		_, err := resolveRallyIdentity(context.Background(), testResolveConfig(stub, "k"), nil, "jwt", ClerkIdentity{ClerkUserID: "u"})
		if err == nil {
			t.Fatal("a 401 must be an error — the authority rejected the token and nothing may be minted")
		}
		if !errorsIsRejected(err) {
			t.Errorf("a 401 must be classified as a rejection, got: %v", err)
		}
		if stub.Calls() != 1 {
			t.Errorf("a 401 must not be retried; calls=%d", stub.Calls())
		}
	})

	t.Run("403 from the authority", func(t *testing.T) {
		stub := newStubTimpagerResolve(t, `{"error":"forbidden"}`, http.StatusForbidden)
		_, err := resolveRallyIdentity(context.Background(), testResolveConfig(stub, "k"), nil, "jwt", ClerkIdentity{ClerkUserID: "u"})
		if err == nil {
			t.Fatal("a 403 must be an error")
		}
		if !errorsIsRejected(err) {
			t.Errorf("a 403 must be classified as a rejection, got: %v", err)
		}
	})

	t.Run("429 rate limited", func(t *testing.T) {
		stub := newStubTimpagerResolve(t, `{"error":"rate limited"}`, http.StatusTooManyRequests)
		_, err := resolveRallyIdentity(context.Background(), testResolveConfig(stub, "k"), nil, "jwt", ClerkIdentity{ClerkUserID: "u"})
		if err == nil {
			t.Fatal("a 429 must be an error")
		}
		if errorsIsRejected(err) {
			t.Error("a 429 is an availability problem, not a credential rejection")
		}
		if stub.Calls() != 1 {
			t.Errorf("a 429 must not be retried into the limiter; calls=%d", stub.Calls())
		}
	})

	t.Run("5xx is retried once then fails", func(t *testing.T) {
		stub := newStubTimpagerResolve(t, `{"error":"boom"}`, http.StatusBadGateway)
		_, err := resolveRallyIdentity(context.Background(), testResolveConfig(stub, "k"), nil, "jwt", ClerkIdentity{ClerkUserID: "u"})
		if err == nil {
			t.Fatal("two 5xx responses must surface an error")
		}
		if errorsIsRejected(err) {
			t.Error("a 5xx is an availability problem, not a credential rejection")
		}
		if stub.Calls() != 2 {
			t.Errorf("a 5xx deserves exactly one retry; calls=%d want 2", stub.Calls())
		}
	})

	t.Run("5xx then success", func(t *testing.T) {
		stub := newStubTimpagerResolve(t, `{"error":"boom"}`, http.StatusInternalServerError)
		// First attempt 500s; the single retry gets a good answer.
		stub.SetFollowUp(resolveOKBody("rally-retry", "u", "", ""), http.StatusOK)
		got, err := resolveRallyIdentity(context.Background(), testResolveConfig(stub, "k"), nil, "jwt", ClerkIdentity{ClerkUserID: "u"})
		if err != nil {
			t.Fatalf("the retry should have succeeded: %v", err)
		}
		if got.RallyUserID != "rally-retry" {
			t.Errorf("RallyUserID: got %q", got.RallyUserID)
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		stub := newStubTimpagerResolve(t, "{not json", http.StatusOK)
		if _, err := resolveRallyIdentity(context.Background(), testResolveConfig(stub, "k"), nil, "jwt", ClerkIdentity{ClerkUserID: "u"}); err == nil {
			t.Fatal("an undecodable response must be an error, never an empty success")
		}
	})

	t.Run("unreachable authority", func(t *testing.T) {
		stub := newStubTimpagerResolve(t, resolveOKBody("r", "u", "", ""), http.StatusOK)
		cfg := testResolveConfig(stub, "k")
		stub.Close()
		_, err := resolveRallyIdentity(context.Background(), cfg, nil, "jwt", ClerkIdentity{ClerkUserID: "u"})
		if err == nil {
			t.Fatal("an unreachable authority must be an error — fail closed, mint nothing")
		}
		if errorsIsRejected(err) {
			t.Error("a network failure is not a credential rejection")
		}
	})

	t.Run("empty clerk user id", func(t *testing.T) {
		stub := newStubTimpagerResolve(t, resolveOKBody("r", "u", "", ""), http.StatusOK)
		if _, err := resolveRallyIdentity(context.Background(), testResolveConfig(stub, "k"), nil, "jwt", ClerkIdentity{}); err == nil {
			t.Fatal("resolving without a Clerk user id must be refused")
		}
		if stub.Calls() != 0 {
			t.Errorf("no HTTP call should be made; calls=%d", stub.Calls())
		}
	})
}

// -----------------------------------------------------------------------
// idempotency — the client is read-only; repeating it changes nothing
// -----------------------------------------------------------------------

func TestResolveRallyIdentity_Idempotent(t *testing.T) {
	stub := newStubTimpagerResolve(t, resolveOKBody("rally-same", "user_same", "a@b.com", "5551112222"), http.StatusOK)
	cfg := testResolveConfig(stub, "k")
	id := ClerkIdentity{ClerkUserID: "user_same"}

	first, err := resolveRallyIdentity(context.Background(), cfg, nil, "jwt", id)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	second, err := resolveRallyIdentity(context.Background(), cfg, nil, "jwt", id)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if first != second {
		t.Errorf("identical calls must yield identical results: %+v vs %+v", first, second)
	}

	reqs := stub.Requests()
	if len(reqs) != 2 {
		t.Fatalf("calls: got %d want 2", len(reqs))
	}
	if reqs[0].RawBody != reqs[1].RawBody {
		t.Errorf("identical calls must send identical bodies:\n%s\n%s", reqs[0].RawBody, reqs[1].RawBody)
	}
	if reqs[0].APIKeyHeader != reqs[1].APIKeyHeader || reqs[0].Authorization != reqs[1].Authorization {
		t.Error("identical calls must send identical headers")
	}
}

// -----------------------------------------------------------------------
// auth — configuration is mandatory, and the service key never leaks into
// the Authorization header
// -----------------------------------------------------------------------

func TestResolveRallyIdentity_RequiresAuthorityConfig(t *testing.T) {
	id := ClerkIdentity{ClerkUserID: "u"}

	t.Run("missing api key", func(t *testing.T) {
		stub := newStubTimpagerResolve(t, resolveOKBody("r", "u", "", ""), http.StatusOK)
		_, err := resolveRallyIdentity(context.Background(), Config{TimpagerURL: stub.URL}, nil, "jwt", id)
		if err == nil {
			t.Fatal("a missing TIMPAGER_API_KEY must be an error before any HTTP call — never mint without the authority")
		}
		if stub.Calls() != 0 {
			t.Errorf("no request may be sent without the service key; calls=%d", stub.Calls())
		}
		if !strings.Contains(err.Error(), "timpager not configured") {
			t.Errorf("error should name the misconfiguration, got: %v", err)
		}
	})

	t.Run("missing url", func(t *testing.T) {
		if _, err := resolveRallyIdentity(context.Background(), Config{TimpagerAPIKey: "k"}, nil, "jwt", id); err == nil {
			t.Fatal("a missing TIMPAGER_URL must be an error")
		}
	})

	t.Run("missing clerk jwt", func(t *testing.T) {
		stub := newStubTimpagerResolve(t, resolveOKBody("r", "u", "", ""), http.StatusOK)
		_, err := resolveRallyIdentity(context.Background(), testResolveConfig(stub, "k"), nil, "", id)
		if err == nil {
			t.Fatal("resolving without the end-user Clerk JWT must be refused — timpager would fall back to an unauthenticated body-only lookup")
		}
		if stub.Calls() != 0 {
			t.Errorf("no request may be sent without the Clerk JWT; calls=%d", stub.Calls())
		}
	})

	t.Run("service key never travels as a Bearer", func(t *testing.T) {
		stub := newStubTimpagerResolve(t, resolveOKBody("r", "u", "", ""), http.StatusOK)
		if _, err := resolveRallyIdentity(context.Background(), testResolveConfig(stub, "svc_secret_key"), nil, "end.user.jwt", id); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := stub.Requests()[0]
		if strings.Contains(r.Authorization, "svc_secret_key") {
			t.Error("the service key must never appear in the Authorization header — timpager would then treat this as a legacy body-only resolve")
		}
		if r.Authorization != "Bearer end.user.jwt" {
			t.Errorf("Authorization must carry only the end-user Clerk JWT; got %q", r.Authorization)
		}
		if strings.Contains(r.RawBody, "svc_secret_key") {
			t.Error("the service key must never appear in the request body")
		}
	})
}

// errorsIsRejected reports whether the error is the authority refusing the
// credential (401/403) as opposed to an availability problem.
func errorsIsRejected(err error) bool {
	return err != nil && strings.Contains(err.Error(), "authority rejected token")
}
