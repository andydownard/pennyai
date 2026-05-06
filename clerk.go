package main

// clerk.go — Go port of timpager's lib/clerk.js.
//
// Wave 1 / PR 3 of the Clerk + FirstPromoter integration. See
// timpager/plans/clerk-firstpromoter-phase-1-findings.md section 8d step 13.
//
// Public surface:
//   - VerifyClerkRequest(r, resolveLocalUser) → (*Session, error)
//       Verifies a Clerk session JWT carried on the request and returns the
//       same Session shape pennyai's existing SessionStore returns, so route
//       handlers don't need to care which path verified them. Returns
//       (nil, nil) when:
//         * CLERK_ENABLED != "1" (kill switch off)
//         * No Authorization Bearer header AND no __session cookie
//         * Clerk SDK rejects the token (invalid / expired / wrong audience)
//         * The Clerk session is valid but resolveLocalUser returns nil
//           (the local users row hasn't been created yet by the webhook)
//
//   - GetClerkUserByID(ctx, clerkUserID) → (*ClerkUser, error)
//       Wraps clerk.user.Get. Returns (nil, nil) on 404, otherwise an error.
//       Returns ErrClerkDisabled when the kill switch is off — strict caller.
//
//   - var ErrClerkDisabled (sentinel)
//
// Configuration (read at CALL TIME, not module load — per timpager
// regression #21):
//   - CLERK_ENABLED         — "1" enables the SDK chain. Anything else is OFF.
//   - CLERK_SECRET_KEY      — REQUIRED when enabled. No fallback chain.
//   - CLERK_PUBLISHABLE_KEY — passed forward for parity with the Node wrapper;
//                             not used by the Go SDK's verify path itself.
//
// Cookie:
//   The Clerk SDK reads its session token from the `__session` cookie by
//   default — Clerk's published convention. We do NOT change that; the
//   request object is forwarded with all headers intact and verification
//   reads __session itself. The legacy `tp_session` cookie remains owned by
//   session.go and is unaffected.
//
// Test seam:
//   `clerkVerifierFn` and `clerkUserFetcherFn` are package-level function
//   variables that production code points at the real SDK. Tests overwrite
//   them with stubs to exercise every branch (kill switch, no token, valid
//   token + resolver returns user, valid token + resolver returns nil,
//   invalid token, SDK error, getClerkUserByID 404 vs. other error).
//   Production code MUST NOT touch these from outside this file.

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	clerk "github.com/clerk/clerk-sdk-go/v2"
	clerkjwt "github.com/clerk/clerk-sdk-go/v2/jwt"
	clerkuser "github.com/clerk/clerk-sdk-go/v2/user"
)

// -------- Constants --------

const (
	clerkCookieName     = "__session"
	clerkBearerPrefix   = "Bearer "
	clerkBearerMinChars = len(clerkBearerPrefix) + 1 // "Bearer x" is the shortest possible
)

// -------- Errors --------

// ErrClerkDisabled is returned by strict callers (GetClerkUserByID) when
// CLERK_ENABLED != "1". VerifyClerkRequest does NOT return this — it
// returns (nil, nil) so the legacy session path can run during the cutover
// window.
var ErrClerkDisabled = errors.New("clerk disabled (CLERK_ENABLED != 1)")

// errNoClerkToken is sentinel-only: returned by extractClerkToken to signal
// "no Authorization Bearer header AND no __session cookie." Callers map it
// to (nil, nil) so the absence of a Clerk token is not a verification
// failure.
var errNoClerkToken = errors.New("no clerk token on request")

// -------- Public types --------

// ClerkUser is the trimmed projection of a Clerk SDK user we care about.
// Wrapping the SDK's *clerk.User keeps callers from depending on the SDK
// type directly — easier to swap providers later. Callers can still reach
// Raw if they need a field we haven't projected.
type ClerkUser struct {
	ID            string
	PrimaryEmail  string
	PrimaryPhone  string
	FirstName     string
	LastName      string
	BannedAt      bool // true when the SDK reports the user as banned
	Raw           *clerk.User
}

// LocalUserResolver is the seam between this generic Clerk wrapper and
// pennyai's user table. The verifier hands you a verified Clerk identity
// (Clerk user ID + claims-derived email/phone, all best-effort) and you
// return the local Session — or nil if the local row doesn't exist yet.
type LocalUserResolver func(clerkUserID, email, phone string) (*Session, error)

// -------- Internal: kill switch + key handling --------

// clerkIsEnabled hot-reads the kill switch on every call. Tests mutate
// os.Setenv in place; production ops can flip the flag without restart.
func clerkIsEnabled() bool {
	return os.Getenv("CLERK_ENABLED") == "1"
}

// requireClerkSecret returns the secret or a clear, operator-friendly error.
// Per timpager regression #21, security secrets do not get fallback chains.
func requireClerkSecret() (string, error) {
	key := os.Getenv("CLERK_SECRET_KEY")
	if key == "" {
		return "", errors.New("CLERK_SECRET_KEY is required when CLERK_ENABLED=1 (no fallback)")
	}
	return key, nil
}

// -------- Internal: SDK wrapping (test seams) --------

// clerkVerifierFn verifies a Clerk session JWT and returns the session
// claims on success. Production wires this to the real Clerk SDK in init().
// Tests overwrite it to skip network calls + JWKS fetches. The signature
// mirrors clerkjwt.Verify(ctx, *clerkjwt.VerifyParams) but lets us pass
// just the raw JWT — that's all VerifyClerkRequest knows.
var clerkVerifierFn = func(ctx context.Context, token string) (*clerk.SessionClaims, error) {
	return clerkjwt.Verify(ctx, &clerkjwt.VerifyParams{Token: token})
}

// clerkUserFetcherFn looks up a Clerk user by ID. Production wires this to
// clerkuser.Get; tests stub it.
var clerkUserFetcherFn = func(ctx context.Context, id string) (*clerk.User, error) {
	return clerkuser.Get(ctx, id)
}

// configureClerkSDK installs CLERK_SECRET_KEY into the SDK's package-level
// state. The SDK uses a global secret key (clerk.SetKey) and global Backend.
// Calling this is a no-op when already configured with the same key, so
// it's safe to call on every request — but we only call it once per
// process per VerifyClerkRequest entry to avoid lock contention in
// clerk.SetKey under load.
//
// Returns the secret on success (so callers can compare for "key changed?"
// reconfiguration in long-running tests) or an error if the secret isn't
// configured.
func configureClerkSDK() (string, error) {
	secret, err := requireClerkSecret()
	if err != nil {
		return "", err
	}
	if currentClerkSecret != secret {
		clerk.SetKey(secret)
		currentClerkSecret = secret
	}
	return secret, nil
}

// currentClerkSecret tracks the last secret we pushed into the SDK so we
// don't re-call clerk.SetKey on every request. Reset by tests via
// _resetClerkState.
var currentClerkSecret string

// _resetClerkState clears the test seam overrides + the cached secret.
// Test-only; production code MUST NOT call this.
func _resetClerkState() {
	clerkVerifierFn = func(ctx context.Context, token string) (*clerk.SessionClaims, error) {
		return clerkjwt.Verify(ctx, &clerkjwt.VerifyParams{Token: token})
	}
	clerkUserFetcherFn = func(ctx context.Context, id string) (*clerk.User, error) {
		return clerkuser.Get(ctx, id)
	}
	currentClerkSecret = ""
}

// -------- Internal: token extraction --------

// extractClerkToken pulls the JWT off the request. Authorization header
// wins over cookie when both are present (matches the Node wrapper's
// behavior). Returns errNoClerkToken when nothing is present so callers
// can short-circuit without paying SDK overhead.
func extractClerkToken(r *http.Request) (string, error) {
	if r == nil || r.Header == nil {
		return "", errNoClerkToken
	}
	auth := r.Header.Get("Authorization")
	if len(auth) >= clerkBearerMinChars && strings.HasPrefix(auth, clerkBearerPrefix) {
		token := strings.TrimSpace(auth[len(clerkBearerPrefix):])
		if token != "" {
			return token, nil
		}
	}
	if c, err := r.Cookie(clerkCookieName); err == nil && c.Value != "" {
		return c.Value, nil
	}
	return "", errNoClerkToken
}

// -------- Public surface --------

// VerifyClerkRequest verifies a Clerk session JWT on the incoming request.
// Returns the same Session shape pennyai's existing SessionStore returns so
// callers don't care which path verified them.
//
// Returns (nil, nil) — NOT an error — when:
//   - The kill switch is off
//   - No Authorization Bearer header AND no __session cookie
//   - The Clerk SDK rejects the token (invalid / expired / wrong audience)
//   - The Clerk session is valid but resolveLocalUser returns nil
//
// Returns an error when:
//   - The kill switch is ON but CLERK_SECRET_KEY is unset (configuration error)
//   - The local-user resolver returns an error (DB failure, etc.)
//
// resolveLocalUser is required when callers want a Session back. If you
// pass nil, this function returns (nil, nil) on every valid token —
// because there's no way to map Clerk → local without the seam.
func VerifyClerkRequest(r *http.Request, resolveLocalUser LocalUserResolver) (*Session, error) {
	// 1) Kill switch — short-circuit BEFORE any env-var or SDK access.
	if !clerkIsEnabled() {
		return nil, nil
	}

	// 2) Fast-path: no Clerk-shaped token on the request → nil, no SDK overhead.
	token, err := extractClerkToken(r)
	if err != nil {
		// Sentinel: anonymous request. Not an error.
		return nil, nil
	}

	// 3) Configure the SDK. A missing secret key when the kill switch is ON
	//    is a configuration error and gets surfaced to the caller — that's
	//    NOT the same as "user has no token."
	if _, err := configureClerkSDK(); err != nil {
		return nil, err
	}

	// 4) Verify via SDK. Anything other than a clean nil error means the
	//    token was missing/invalid/expired/etc. Log via stdlib and return
	//    nil so the legacy session path can run.
	ctx := r.Context()
	claims, err := clerkVerifierFn(ctx, token)
	if err != nil {
		// Verification failure is a benign per-request signal during the
		// cutover (most requests still carry tp_session). Don't propagate
		// the error — let the legacy fallthrough decide.
		return nil, nil
	}
	if claims == nil || claims.Subject == "" {
		// Defensive: SDK returned no claims or no subject. Treat as anonymous.
		return nil, nil
	}

	// 5) No resolver → we can't mint a Session. Return nil so the caller
	//    treats this request as anonymous. Production wiring always passes
	//    a resolver; this is a defensive branch for misuse.
	if resolveLocalUser == nil {
		return nil, nil
	}

	// 6) Map Clerk → local user. The Clerk Go SDK's SessionClaims doesn't
	//    carry email/phone in the JWT (those live on the User resource).
	//    If the caller needs them they can fetch via GetClerkUserByID — for
	//    most pennyai routes the local users row lookup by clerk_user_id is
	//    enough. We pass empty strings here for parity with the Node
	//    wrapper's signature; the resolver can ignore them or do a
	//    user-fetch if needed.
	session, err := resolveLocalUser(claims.Subject, "", "")
	if err != nil {
		return nil, err
	}
	if session == nil {
		// Valid Clerk session but no local row yet. Webhook hasn't landed.
		// Return nil so the caller treats this request as anonymous; do
		// NOT propagate as an error.
		return nil, nil
	}
	return session, nil
}

// GetClerkUserByID looks up a Clerk user by their Clerk user ID. Returns
// (nil, nil) on 404 so callers can branch cleanly on "exists vs. not."
//
// Returns ErrClerkDisabled when CLERK_ENABLED != "1" — strict caller.
// Unlike VerifyClerkRequest, there is no graceful nil fallback; if you
// call this you really wanted a Clerk lookup.
func GetClerkUserByID(ctx context.Context, clerkUserID string) (*ClerkUser, error) {
	if !clerkIsEnabled() {
		return nil, ErrClerkDisabled
	}
	if clerkUserID == "" {
		return nil, errors.New("GetClerkUserByID: clerkUserID is required")
	}
	if _, err := configureClerkSDK(); err != nil {
		return nil, err
	}

	user, err := clerkUserFetcherFn(ctx, clerkUserID)
	if err != nil {
		// The Clerk SDK surfaces 404 via clerk.APIErrorResponse with a
		// status code field; callers commonly want nil/nil on miss.
		if isClerkNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if user == nil {
		return nil, nil
	}

	return shapeClerkUser(user), nil
}

// shapeClerkUser projects the SDK's *clerk.User to our ClerkUser type. It
// lifts the primary email + phone out of the nested arrays the SDK uses
// (each User can have N email addresses; one of them is "primary" via
// PrimaryEmailAddressID). Defensive on every dereference.
func shapeClerkUser(u *clerk.User) *ClerkUser {
	if u == nil {
		return nil
	}
	out := &ClerkUser{ID: u.ID, Raw: u}
	if u.FirstName != nil {
		out.FirstName = *u.FirstName
	}
	if u.LastName != nil {
		out.LastName = *u.LastName
	}
	if u.PrimaryEmailAddressID != nil {
		for _, e := range u.EmailAddresses {
			if e == nil {
				continue
			}
			if e.ID == *u.PrimaryEmailAddressID {
				out.PrimaryEmail = e.EmailAddress
				break
			}
		}
	}
	if u.PrimaryPhoneNumberID != nil {
		for _, p := range u.PhoneNumbers {
			if p == nil {
				continue
			}
			if p.ID == *u.PrimaryPhoneNumberID {
				out.PrimaryPhone = p.PhoneNumber
				break
			}
		}
	}
	if u.Banned {
		out.BannedAt = true
	}
	return out
}

// isClerkNotFound reports whether the SDK error represents a 404. The Clerk
// SDK wraps API errors in *clerk.APIErrorResponse with a HTTPStatusCode
// field. Callers commonly want nil/nil on miss; this helper isolates the
// error-shape inspection.
func isClerkNotFound(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *clerk.APIErrorResponse
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatusCode == http.StatusNotFound
	}
	return false
}

