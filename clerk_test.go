package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	clerk "github.com/clerk/clerk-sdk-go/v2"
)

// =====================================================================
// Test scaffolding
// =====================================================================

// withClerkEnv installs the standard "kill switch on + valid secret" env
// block for one subtest and registers a cleanup hook that restores prior
// state. Each test must own its env mutations to keep `go test -race`
// honest.
func withClerkEnv(t *testing.T, extra map[string]string) {
	t.Helper()
	defaults := map[string]string{
		"CLERK_ENABLED":         "1",
		"CLERK_SECRET_KEY":      "sk_test_clerk_secret_xyz",
		"CLERK_PUBLISHABLE_KEY": "pk_test_clerk_pub",
	}
	for k, v := range extra {
		defaults[k] = v
	}
	for k, v := range defaults {
		t.Setenv(k, v)
	}
}

// stubVerifier installs a clerkVerifierFn replacement that returns the
// supplied claims/err and counts invocations. Restores the production
// verifier when the test ends.
func stubVerifier(t *testing.T, claims *clerk.SessionClaims, err error) *int32 {
	t.Helper()
	prev := clerkVerifierFn
	var calls int32
	clerkVerifierFn = func(ctx context.Context, token string) (*clerk.SessionClaims, error) {
		atomic.AddInt32(&calls, 1)
		return claims, err
	}
	t.Cleanup(func() { clerkVerifierFn = prev; currentClerkSecret = "" })
	return &calls
}

// stubUserFetcher installs a clerkUserFetcherFn replacement and counts
// invocations.
func stubUserFetcher(t *testing.T, user *clerk.User, err error) *int32 {
	t.Helper()
	prev := clerkUserFetcherFn
	var calls int32
	clerkUserFetcherFn = func(ctx context.Context, id string) (*clerk.User, error) {
		atomic.AddInt32(&calls, 1)
		return user, err
	}
	t.Cleanup(func() { clerkUserFetcherFn = prev; currentClerkSecret = "" })
	return &calls
}

// makeClaims builds a minimal *clerk.SessionClaims with only the fields the
// wrapper inspects. The SDK's RegisteredClaims.Subject is set; everything
// else is left zero.
func makeClaims(subject string) *clerk.SessionClaims {
	c := &clerk.SessionClaims{}
	c.Subject = subject
	return c
}

// =====================================================================
// Kill switch
// =====================================================================

func TestVerifyClerkRequest_KillSwitchOff(t *testing.T) {
	t.Setenv("CLERK_ENABLED", "0")
	t.Setenv("CLERK_SECRET_KEY", "irrelevant")
	verifierCalls := stubVerifier(t, makeClaims("user_X"), nil)

	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	r.Header.Set("Authorization", "Bearer some-jwt")

	sess, err := VerifyClerkRequest(r, func(string, string, string) (*Session, error) {
		t.Fatal("resolver should not be called when kill switch is off")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("kill-switch-off must not return error, got: %v", err)
	}
	if sess != nil {
		t.Errorf("kill-switch-off must return nil session, got: %+v", sess)
	}
	if atomic.LoadInt32(verifierCalls) != 0 {
		t.Errorf("verifier should not be called when kill switch is off; calls=%d", *verifierCalls)
	}
}

func TestGetClerkUserByID_KillSwitchOffReturnsSentinel(t *testing.T) {
	t.Setenv("CLERK_ENABLED", "0")
	t.Setenv("CLERK_SECRET_KEY", "irrelevant")
	stubUserFetcher(t, nil, nil)

	_, err := GetClerkUserByID(context.Background(), "user_123")
	if !errors.Is(err, ErrClerkDisabled) {
		t.Fatalf("expected ErrClerkDisabled, got: %v", err)
	}
}

// =====================================================================
// No token on request
// =====================================================================

func TestVerifyClerkRequest_NoTokenReturnsNilNil(t *testing.T) {
	withClerkEnv(t, nil)
	verifierCalls := stubVerifier(t, nil, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	// No Authorization header, no __session cookie.

	sess, err := VerifyClerkRequest(r, func(string, string, string) (*Session, error) {
		t.Fatal("resolver should not be called when no token is present")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("no-token must not return error, got: %v", err)
	}
	if sess != nil {
		t.Errorf("no-token must return nil session, got: %+v", sess)
	}
	if atomic.LoadInt32(verifierCalls) != 0 {
		t.Errorf("verifier should not run on no-token fast path; calls=%d", *verifierCalls)
	}
}

func TestVerifyClerkRequest_BearerHeaderTrumpsCookie(t *testing.T) {
	withClerkEnv(t, nil)
	var capturedToken string
	prev := clerkVerifierFn
	clerkVerifierFn = func(ctx context.Context, token string) (*clerk.SessionClaims, error) {
		capturedToken = token
		return makeClaims("user_X"), nil
	}
	t.Cleanup(func() { clerkVerifierFn = prev; currentClerkSecret = "" })

	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	r.Header.Set("Authorization", "Bearer header-token-xyz")
	r.AddCookie(&http.Cookie{Name: "__session", Value: "cookie-token-abc"})

	resolverCalled := int32(0)
	_, err := VerifyClerkRequest(r, func(clerkUserID, email, phone string) (*Session, error) {
		atomic.AddInt32(&resolverCalled, 1)
		return &Session{UserID: "local-1"}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedToken != "header-token-xyz" {
		t.Errorf("Bearer header MUST win over cookie; got token %q", capturedToken)
	}
	if atomic.LoadInt32(&resolverCalled) != 1 {
		t.Errorf("resolver should be called exactly once; calls=%d", resolverCalled)
	}
}

func TestVerifyClerkRequest_CookieFallback(t *testing.T) {
	withClerkEnv(t, nil)
	var capturedToken string
	prev := clerkVerifierFn
	clerkVerifierFn = func(ctx context.Context, token string) (*clerk.SessionClaims, error) {
		capturedToken = token
		return makeClaims("user_X"), nil
	}
	t.Cleanup(func() { clerkVerifierFn = prev; currentClerkSecret = "" })

	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	r.AddCookie(&http.Cookie{Name: "__session", Value: "cookie-only-token"})

	_, err := VerifyClerkRequest(r, func(string, string, string) (*Session, error) {
		return &Session{UserID: "local-1"}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedToken != "cookie-only-token" {
		t.Errorf("expected cookie token, got %q", capturedToken)
	}
}

// =====================================================================
// Configuration error: kill switch ON but no secret key
// =====================================================================

func TestVerifyClerkRequest_MissingSecretWhenEnabledIsConfigError(t *testing.T) {
	t.Setenv("CLERK_ENABLED", "1")
	t.Setenv("CLERK_SECRET_KEY", "")
	verifierCalls := stubVerifier(t, makeClaims("user_X"), nil)

	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	r.Header.Set("Authorization", "Bearer abc.def.ghi")

	_, err := VerifyClerkRequest(r, func(string, string, string) (*Session, error) {
		t.Fatal("resolver should not be called on config error")
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected configuration error for missing CLERK_SECRET_KEY, got nil")
	}
	if !strings.Contains(err.Error(), "CLERK_SECRET_KEY") {
		t.Errorf("error must name the missing var, got: %v", err)
	}
	if atomic.LoadInt32(verifierCalls) != 0 {
		t.Errorf("verifier should not be called when configuration fails; calls=%d", *verifierCalls)
	}
}

// =====================================================================
// SDK rejects token (invalid / expired)
// =====================================================================

func TestVerifyClerkRequest_InvalidTokenReturnsNilNotError(t *testing.T) {
	withClerkEnv(t, nil)
	stubVerifier(t, nil, errors.New("invalid signing algorithm"))

	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	r.Header.Set("Authorization", "Bearer fake.jwt.token")

	resolverCalled := false
	sess, err := VerifyClerkRequest(r, func(string, string, string) (*Session, error) {
		resolverCalled = true
		return nil, nil
	})
	if err != nil {
		t.Fatalf("invalid-token must NOT return an error (legacy path takes over); got: %v", err)
	}
	if sess != nil {
		t.Errorf("invalid-token must return nil session, got: %+v", sess)
	}
	if resolverCalled {
		t.Error("resolver must NOT be called when SDK rejects the token")
	}
}

func TestVerifyClerkRequest_ClaimsWithoutSubjectReturnsNil(t *testing.T) {
	withClerkEnv(t, nil)
	// SDK returns claims but the JWT was missing `sub` somehow — defensive branch.
	stubVerifier(t, &clerk.SessionClaims{}, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	r.Header.Set("Authorization", "Bearer x")

	resolverCalled := false
	sess, err := VerifyClerkRequest(r, func(string, string, string) (*Session, error) {
		resolverCalled = true
		return &Session{UserID: "local-1"}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess != nil {
		t.Errorf("subject-less claims must produce nil session, got: %+v", sess)
	}
	if resolverCalled {
		t.Error("resolver must NOT be called when claims have no subject")
	}
}

// =====================================================================
// Valid token + no local user (webhook hasn't landed)
// =====================================================================

func TestVerifyClerkRequest_NoLocalUserReturnsNilNil(t *testing.T) {
	withClerkEnv(t, nil)
	stubVerifier(t, makeClaims("user_clerk_999"), nil)

	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	r.Header.Set("Authorization", "Bearer valid.jwt")

	sess, err := VerifyClerkRequest(r, func(clerkUserID, email, phone string) (*Session, error) {
		if clerkUserID != "user_clerk_999" {
			t.Errorf("resolver should receive sub claim; got %q", clerkUserID)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("missing-local-user must NOT return error, got: %v", err)
	}
	if sess != nil {
		t.Errorf("missing-local-user must return nil session, got: %+v", sess)
	}
}

func TestVerifyClerkRequest_NilResolverReturnsNilNil(t *testing.T) {
	withClerkEnv(t, nil)
	stubVerifier(t, makeClaims("user_X"), nil)

	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	r.Header.Set("Authorization", "Bearer valid.jwt")

	sess, err := VerifyClerkRequest(r, nil)
	if err != nil {
		t.Fatalf("nil resolver must not error, got: %v", err)
	}
	if sess != nil {
		t.Errorf("nil resolver must return nil session, got: %+v", sess)
	}
}

func TestVerifyClerkRequest_ResolverErrorPropagates(t *testing.T) {
	withClerkEnv(t, nil)
	stubVerifier(t, makeClaims("user_X"), nil)

	dbErr := errors.New("db connection refused")
	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	r.Header.Set("Authorization", "Bearer valid.jwt")

	sess, err := VerifyClerkRequest(r, func(string, string, string) (*Session, error) {
		return nil, dbErr
	})
	if !errors.Is(err, dbErr) {
		t.Fatalf("resolver error must propagate; got: %v", err)
	}
	if sess != nil {
		t.Errorf("resolver error must not return a session, got: %+v", sess)
	}
}

// =====================================================================
// Valid token + valid local user — happy path
// =====================================================================

func TestVerifyClerkRequest_HappyPath(t *testing.T) {
	withClerkEnv(t, nil)
	verifierCalls := stubVerifier(t, makeClaims("user_clerk_1"), nil)

	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	r.Header.Set("Authorization", "Bearer valid.jwt.token")

	expected := &Session{
		UserID:      "local-user-1",
		PhoneNumber: "+15551234567",
		AuthMethod:  "google",
		CreatedAt:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	resolverGot := struct {
		clerkUserID string
		called      int32
	}{}
	sess, err := VerifyClerkRequest(r, func(clerkUserID, email, phone string) (*Session, error) {
		atomic.AddInt32(&resolverGot.called, 1)
		resolverGot.clerkUserID = clerkUserID
		return expected, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session, got nil")
	}
	if sess != expected {
		t.Errorf("session not the one resolver returned: got %+v want %+v", sess, expected)
	}
	if resolverGot.clerkUserID != "user_clerk_1" {
		t.Errorf("resolver got clerkUserID=%q want user_clerk_1", resolverGot.clerkUserID)
	}
	if atomic.LoadInt32(verifierCalls) != 1 {
		t.Errorf("verifier should be called exactly once; calls=%d", *verifierCalls)
	}
	if atomic.LoadInt32(&resolverGot.called) != 1 {
		t.Errorf("resolver should be called exactly once; calls=%d", resolverGot.called)
	}
}

// =====================================================================
// GetClerkUserByID
// =====================================================================

func TestGetClerkUserByID_HappyPath(t *testing.T) {
	withClerkEnv(t, nil)

	primaryEmailID := "idn_email_primary"
	primaryPhoneID := "idn_phone_primary"
	first := "Tamika"
	last := "Jones"
	stubUserFetcher(t, &clerk.User{
		ID:                    "user_clerk_42",
		FirstName:             &first,
		LastName:              &last,
		PrimaryEmailAddressID: &primaryEmailID,
		PrimaryPhoneNumberID:  &primaryPhoneID,
		EmailAddresses: []*clerk.EmailAddress{
			{ID: "idn_email_other", EmailAddress: "alt@example.com"},
			{ID: primaryEmailID, EmailAddress: "tamika@example.com"},
		},
		PhoneNumbers: []*clerk.PhoneNumber{
			{ID: primaryPhoneID, PhoneNumber: "+15551234567"},
		},
		Banned: false,
	}, nil)

	user, err := GetClerkUserByID(context.Background(), "user_clerk_42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != "user_clerk_42" {
		t.Errorf("ID: got %q", user.ID)
	}
	if user.FirstName != "Tamika" || user.LastName != "Jones" {
		t.Errorf("name: got %q %q", user.FirstName, user.LastName)
	}
	if user.PrimaryEmail != "tamika@example.com" {
		t.Errorf("PrimaryEmail: got %q", user.PrimaryEmail)
	}
	if user.PrimaryPhone != "+15551234567" {
		t.Errorf("PrimaryPhone: got %q", user.PrimaryPhone)
	}
	if user.BannedAt {
		t.Error("BannedAt should be false")
	}
	if user.Raw == nil {
		t.Error("Raw should be populated for callers that need fields we didn't project")
	}
}

func TestGetClerkUserByID_404ReturnsNilNil(t *testing.T) {
	withClerkEnv(t, nil)
	notFoundErr := &clerk.APIErrorResponse{HTTPStatusCode: http.StatusNotFound}
	stubUserFetcher(t, nil, notFoundErr)

	user, err := GetClerkUserByID(context.Background(), "user_does_not_exist")
	if err != nil {
		t.Fatalf("404 must NOT produce an error, got: %v", err)
	}
	if user != nil {
		t.Errorf("404 must return nil user, got: %+v", user)
	}
}

func TestGetClerkUserByID_5xxReturnsError(t *testing.T) {
	withClerkEnv(t, nil)
	upstream := &clerk.APIErrorResponse{HTTPStatusCode: http.StatusInternalServerError}
	stubUserFetcher(t, nil, upstream)

	_, err := GetClerkUserByID(context.Background(), "user_x")
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
	var apiErr *clerk.APIErrorResponse
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *clerk.APIErrorResponse, got %T: %v", err, err)
	}
	if apiErr.HTTPStatusCode != http.StatusInternalServerError {
		t.Errorf("HTTPStatusCode: got %d", apiErr.HTTPStatusCode)
	}
}

func TestGetClerkUserByID_RequiresClerkUserID(t *testing.T) {
	withClerkEnv(t, nil)
	stubUserFetcher(t, nil, nil)

	_, err := GetClerkUserByID(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty clerkUserID")
	}
}

// =====================================================================
// extractClerkToken — direct unit coverage for the fast-path predicate
// =====================================================================

func TestExtractClerkToken_AuthHeaderHappyPath(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer my-jwt-1.2.3")

	tok, err := extractClerkToken(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "my-jwt-1.2.3" {
		t.Errorf("got %q", tok)
	}
}

func TestExtractClerkToken_EmptyBearerSkipped(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer ")

	_, err := extractClerkToken(r)
	if !errors.Is(err, errNoClerkToken) {
		t.Fatalf("empty bearer should be no-token; got %v", err)
	}
}

func TestExtractClerkToken_NoHeaderNoCookieIsNoToken(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := extractClerkToken(r)
	if !errors.Is(err, errNoClerkToken) {
		t.Fatalf("expected errNoClerkToken, got: %v", err)
	}
}
