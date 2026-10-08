package main

// clerk.go — Rally sign-in verifier. pennyai is first-party on the Rally
// apex and verifies the Rally Clerk instance's session tokens.
//
// Three rules shape every line below.
//
//  1. NO CLERK SECRET. pennyai verifies session JWTs against the issuer's
//     PUBLIC JWKS and never touches Clerk's Backend API. The server refuses to
//     boot if CLERK_SECRET_KEY or CLERK_WEBHOOK_SIGNING_SECRET is present
//     (see clerkForbiddenSecrets, wired into main()).
//
//  2. COOKIE-BLIND. Clerk's published convention is a `__session` cookie on the
//     apex domain. pennyai never reads it. Only an `Authorization: Bearer`
//     header counts, so a cookie minted for some other Rally app cannot
//     authenticate anyone here, and a cookie bleed across the apex is a
//     non-event for us.
//
//  3. FAIL CLOSED ON TRUST. A Clerk token that does not verify never
//     authenticates anyone. Anything that would require trusting an unverified
//     input — a missing issuer, an empty azp allowlist, an unreachable JWKS —
//     is an error, never a pass.
//
// Why the SDK's jwt.Verify is necessary but not sufficient. With VerifyParams.JWK
// supplied it does no network and needs no secret, and it checks the signature,
// exp/nbf with leeway, header alg == JWK alg, and azp via AuthorizedPartyHandler.
// It does NOT require `exp` to be present, does NOT require `sub`, and its
// issuer check is only a PATTERN ("https://clerk." prefix, or contains
// ".clerk.accounts") applied when IsSatellite is false. So this file adds:
//
//   - an RS256-and-kid shape pre-check, before any key lookup, which defeats
//     `alg: none` and HS256 confusion without paying for a JWKS round trip
//   - JWK pinning: kty RSA, use "sig" (or unset), alg RS256
//   - exact issuer equality against CLERK_JWT_ISSUER
//   - `exp` present and `sub` present
//
// Configuration, all read at CALL TIME (regression #21 — a flag flip must not
// need a restart, and tests use t.Setenv):
//
//	CLERK_ENABLED             "1" enables the path. Anything else is OFF.
//	CLERK_JWT_ISSUER          Primary issuer, matched EXACTLY. Required.
//	CLERK_AUTHORIZED_PARTIES  CSV azp allowlist, exact origins. Required.
//
// Test seam: clerkJWKS is a package-level interface value. Tests swap it for an
// in-process stub so the suite is entirely networkless. Production code MUST
// NOT reassign it.

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	clerk "github.com/clerk/clerk-sdk-go/v2"
	clerkjwt "github.com/clerk/clerk-sdk-go/v2/jwt"
)

// -------- Constants --------

const (
	clerkBearerPrefix   = "Bearer "
	clerkBearerMinChars = len(clerkBearerPrefix) + 1 // "Bearer x" is the shortest possible

	// clerkClockLeeway absorbs clock skew between the primary instance and this
	// server. Deliberately small: it is the window in which an expired token is
	// still honoured.
	clerkClockLeeway = 10 * time.Second

	// clerkJWKSTimeout bounds the one outbound request this file ever makes.
	clerkJWKSTimeout = 5 * time.Second

	// clerkJWKSRefetchWindow throttles refetches for unknown key IDs so a caller
	// sending random kids cannot drive a fetch storm at the primary.
	clerkJWKSRefetchWindow = 60 * time.Second

	// clerkJWKSFailureWindow is a short negative cache after a failed fetch, so
	// a JWKS outage does not make every Bearer request pay the full timeout.
	clerkJWKSFailureWindow = 10 * time.Second

	clerkJWKSPath = "/.well-known/jwks.json"
)

// -------- Errors --------

// ErrClerkDisabled is returned by strict callers when CLERK_ENABLED != "1".
// VerifyClerkRequest does NOT return it — that path returns (nil, nil) so the
// legacy session path runs untouched.
var ErrClerkDisabled = errors.New("clerk disabled (CLERK_ENABLED != 1)")

// errNoClerkToken signals "no Authorization Bearer header." Callers map it to
// (nil, nil): the absence of a Clerk token is not a verification failure.
var errNoClerkToken = errors.New("no clerk token on request")

// -------- Public types --------

// ClerkIdentity is a VERIFIED Clerk identity. Every field came out of a token
// whose signature, issuer, authorized party and lifetime have been checked.
//
// Email and Name are only populated when the primary
// instance is configured to add those custom claims to its session token;
// Clerk's default session JWT carries none of them. Downstream code must treat them as
// best-effort and work when they are empty.
type ClerkIdentity struct {
	ClerkUserID     string // `sub`
	SessionID       string // `sid`
	AuthorizedParty string // `azp`
	Email           string // optional custom claim
	Name            string // optional custom claim
	Claims          *clerk.SessionClaims
}

// LocalUserResolver maps a verified Clerk identity to a local pennyai session.
// Returns nil when no local users row carries that Clerk identity yet — the
// request is then anonymous, not an error.
type LocalUserResolver func(id ClerkIdentity) (*Session, error)

// clerkCustomClaims is the optional custom-claim schema. Absent claims stay
// empty strings.
type clerkCustomClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// -------- Configuration --------

type clerkSatelliteConfig struct {
	Issuer            string
	AuthorizedParties []string
}

// clerkIsEnabled hot-reads the kill switch on every call.
func clerkIsEnabled() bool {
	return os.Getenv("CLERK_ENABLED") == "1"
}

// clerkConfig reads the verifier configuration at call time. Both values are
// mandatory: without an exact issuer a token from any Clerk instance would
// pass the SDK's pattern check, and without an azp allowlist any app on the
// primary instance could mint tokens this server would honour. Security
// configuration gets no fallback chain (regression #21).
func clerkConfig() (clerkSatelliteConfig, error) {
	return clerkConfigFrom(os.Getenv)
}

// clerkConfigFrom is clerkConfig over an injectable environment, so boot
// validation can be unit-tested.
func clerkConfigFrom(getenv func(string) string) (clerkSatelliteConfig, error) {
	issuer := strings.TrimSpace(getenv("CLERK_JWT_ISSUER"))
	if issuer == "" {
		return clerkSatelliteConfig{}, errors.New("CLERK_JWT_ISSUER is required when CLERK_ENABLED=1 (no fallback)")
	}

	var parties []string
	for _, p := range strings.Split(getenv("CLERK_AUTHORIZED_PARTIES"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			parties = append(parties, p)
		}
	}
	if len(parties) == 0 {
		return clerkSatelliteConfig{}, errors.New("CLERK_AUTHORIZED_PARTIES is required when CLERK_ENABLED=1 (azp allowlist, no fallback)")
	}

	return clerkSatelliteConfig{Issuer: issuer, AuthorizedParties: parties}, nil
}

// clerkForbiddenSecrets returns the names of Clerk secrets that are set but
// must never exist on this service. Pure, so main() can refuse to boot on it
// (log.Fatalf is not unit-testable; this function is).
//
// Note the check is independent of CLERK_ENABLED: holding the secret is itself
// the misconfiguration, whether or not the feature is on.
func clerkForbiddenSecrets() []string {
	var found []string
	for _, name := range []string{"CLERK_SECRET_KEY", "CLERK_WEBHOOK_SIGNING_SECRET"} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			found = append(found, name)
		}
	}
	return found
}

// -------- JWKS source (test seam) --------

// clerkJWKSSource resolves a key ID to a verification key for an issuer.
type clerkJWKSSource interface {
	keyForKID(ctx context.Context, issuer, kid string) (*clerk.JSONWebKey, error)
}

// clerkJWKS is the production source. Tests replace it with an in-process stub.
var clerkJWKS clerkJWKSSource = newHTTPJWKSSource()

// httpJWKSSource fetches and caches the primary instance's PUBLIC JWKS. No
// secret, no Backend API: a plain GET of issuer + /.well-known/jwks.json.
//
// The SDK's own jwks.Client goes through clerk.Backend and therefore needs
// CLERK_SECRET_KEY, which this service must never hold — hence our own fetch.
type httpJWKSSource struct {
	mu          sync.Mutex
	keys        map[string]map[string]*clerk.JSONWebKey // issuer -> kid -> key
	lastRefetch map[string]time.Time                    // issuer -> last unknown-kid refetch
	lastFailure map[string]time.Time                    // issuer -> last failed fetch
	httpClient  *http.Client
}

func newHTTPJWKSSource() *httpJWKSSource {
	return &httpJWKSSource{
		keys:        map[string]map[string]*clerk.JSONWebKey{},
		lastRefetch: map[string]time.Time{},
		lastFailure: map[string]time.Time{},
		httpClient:  &http.Client{Timeout: clerkJWKSTimeout},
	}
}

func (s *httpJWKSSource) keyForKID(ctx context.Context, issuer, kid string) (*clerk.JSONWebKey, error) {
	if issuer == "" {
		return nil, errors.New("clerk jwks: issuer is required")
	}
	if kid == "" {
		return nil, errors.New("clerk jwks: kid is required")
	}

	s.mu.Lock()
	cached, haveKeySet := s.keys[issuer]
	if k, ok := cached[kid]; ok {
		s.mu.Unlock()
		return k, nil
	}
	// Short negative cache: during a JWKS outage, do not make every request pay
	// the full timeout.
	if failedAt, ok := s.lastFailure[issuer]; ok && time.Since(failedAt) < clerkJWKSFailureWindow {
		s.mu.Unlock()
		return nil, fmt.Errorf("clerk jwks: recent fetch failure for issuer, not retrying yet (kid %q)", kid)
	}
	// Unknown kid on a key set we already hold. Keys rotate, so one refetch is
	// warranted — but at most one per window per issuer, so a caller sending
	// random kids cannot drive a fetch storm at the primary.
	if haveKeySet {
		if last, ok := s.lastRefetch[issuer]; ok && time.Since(last) < clerkJWKSRefetchWindow {
			s.mu.Unlock()
			return nil, fmt.Errorf("clerk jwks: no key for kid %q (refetch throttled)", kid)
		}
		s.lastRefetch[issuer] = time.Now()
	}
	s.mu.Unlock()

	fetched, err := s.fetch(ctx, issuer)
	if err != nil {
		// Fail closed. A known kid already in cache is still served above; we
		// never treat a fetch failure as "assume valid."
		s.mu.Lock()
		s.lastFailure[issuer] = time.Now()
		s.mu.Unlock()
		return nil, err
	}

	s.mu.Lock()
	s.keys[issuer] = fetched
	delete(s.lastFailure, issuer)
	k, ok := fetched[kid]
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("clerk jwks: no key for kid %q", kid)
	}
	return k, nil
}

// fetch GETs the public JWKS and keeps only keys suitable for verifying an
// RS256 signature. Filtering at load time means the verifier can never be
// handed an encryption key or a key of the wrong family.
func (s *httpJWKSSource) fetch(ctx context.Context, issuer string) (map[string]*clerk.JSONWebKey, error) {
	url := strings.TrimSuffix(issuer, "/") + clerkJWKSPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("clerk jwks: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clerk jwks: fetching: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clerk jwks: unexpected status %d", resp.StatusCode)
	}

	// Bound the read: a hostile or broken endpoint must not exhaust memory.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("clerk jwks: reading body: %w", err)
	}

	var set clerk.JSONWebKeySet
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("clerk jwks: decoding: %w", err)
	}

	out := map[string]*clerk.JSONWebKey{}
	for _, k := range set.Keys {
		if k == nil || k.KeyID == "" {
			continue
		}
		if err := clerkKeyIsUsable(k); err != nil {
			log.Printf("[clerk-auth] skipping unusable JWKS key kid=%s: %v", k.KeyID, err)
			continue
		}
		out[k.KeyID] = k
	}
	if len(out) == 0 {
		return nil, errors.New("clerk jwks: no usable RS256 signing keys in key set")
	}
	return out, nil
}

// clerkKeyIsUsable pins the key to RS256 signature verification with RSA key
// material. Anything else is refused rather than attempted.
func clerkKeyIsUsable(k *clerk.JSONWebKey) error {
	if k == nil {
		return errors.New("nil key")
	}
	if k.Algorithm != "" && k.Algorithm != "RS256" {
		return fmt.Errorf("alg %q is not RS256", k.Algorithm)
	}
	if k.Use != "" && k.Use != "sig" {
		return fmt.Errorf("use %q is not sig", k.Use)
	}
	if k.Key == nil {
		return errors.New("no key material")
	}
	if _, ok := k.Key.(*rsa.PublicKey); !ok {
		return fmt.Errorf("key material is %T, not an RSA public key", k.Key)
	}
	return nil
}

// -------- Token extraction --------

// extractClerkToken pulls the JWT off the Authorization header. The `__session`
// cookie is deliberately NOT consulted — see rule 2 at the top of this file.
func extractClerkToken(r *http.Request) (string, error) {
	if r == nil || r.Header == nil {
		return "", errNoClerkToken
	}
	auth := r.Header.Get("Authorization")
	if len(auth) >= clerkBearerMinChars && strings.HasPrefix(auth, clerkBearerPrefix) {
		if token := strings.TrimSpace(auth[len(clerkBearerPrefix):]); token != "" {
			return token, nil
		}
	}
	return "", errNoClerkToken
}

// -------- Verification --------

// clerkTokenHeader is the subset of a JWS header we pre-check.
type clerkTokenHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

// clerkPrecheckHeader parses and validates the token's header without
// verifying anything cryptographic. Running first means `alg: none` and an
// HS256 confusion attempt are refused before we spend a key lookup on them.
func clerkPrecheckHeader(token string) (clerkTokenHeader, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return clerkTokenHeader{}, fmt.Errorf("malformed token: expected 3 segments, got %d", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return clerkTokenHeader{}, fmt.Errorf("malformed token header: %w", err)
	}
	var h clerkTokenHeader
	if err := json.Unmarshal(raw, &h); err != nil {
		return clerkTokenHeader{}, fmt.Errorf("malformed token header: %w", err)
	}
	if h.Algorithm != "RS256" {
		return clerkTokenHeader{}, fmt.Errorf("unsupported signing algorithm %q: only RS256 is accepted", h.Algorithm)
	}
	if h.KeyID == "" {
		return clerkTokenHeader{}, errors.New("token header has no kid")
	}
	return h, nil
}

// VerifyClerkToken verifies a Clerk session JWT and returns the verified
// identity. Never logs or returns the token itself.
//
// Order matters: shape → key → signature/lifetime/azp (SDK) → claims the SDK
// does not enforce (exact issuer, exp present, sub present).
func VerifyClerkToken(ctx context.Context, token string) (*ClerkIdentity, error) {
	cfg, err := clerkConfig()
	if err != nil {
		return nil, err
	}

	header, err := clerkPrecheckHeader(token)
	if err != nil {
		return nil, err
	}

	key, err := clerkJWKS.keyForKID(ctx, cfg.Issuer, header.KeyID)
	if err != nil {
		return nil, err
	}
	// Defensive: the seam is swappable, so pin the key here too rather than
	// trusting whatever the source handed back.
	if err := clerkKeyIsUsable(key); err != nil {
		return nil, fmt.Errorf("clerk jwks: unusable key for kid %q: %w", header.KeyID, err)
	}

	allowed := make(map[string]bool, len(cfg.AuthorizedParties))
	for _, p := range cfg.AuthorizedParties {
		allowed[p] = true
	}

	claims, err := clerkjwt.Verify(ctx, &clerkjwt.VerifyParams{
		Token:  token,
		JWK:    key,
		Leeway: clerkClockLeeway,
		// IsSatellite false keeps the SDK's (pattern-only) issuer check on as a
		// first filter. The exact match below is what actually binds us to one
		// instance.
		IsSatellite: false,
		AuthorizedPartyHandler: func(azp string) bool {
			// An absent azp is never acceptable: it is what pins a token to the
			// origin it was minted for.
			return azp != "" && allowed[azp]
		},
		CustomClaimsConstructor: func(context.Context) any { return &clerkCustomClaims{} },
	})
	if err != nil {
		return nil, err
	}
	if claims == nil {
		return nil, errors.New("clerk verify returned no claims")
	}

	// Post-checks the SDK does not perform.
	if claims.Issuer != cfg.Issuer {
		// Reached when a token's issuer satisfies the SDK's PATTERN but is not
		// our primary instance.
		return nil, fmt.Errorf("invalid issuer: token is not from the configured Rally instance")
	}
	if claims.Expiry == nil {
		return nil, errors.New("token has no exp claim")
	}
	if claims.Subject == "" {
		return nil, errors.New("token has no sub claim")
	}

	id := &ClerkIdentity{
		ClerkUserID:     claims.Subject,
		SessionID:       claims.SessionID,
		AuthorizedParty: claims.AuthorizedParty,
		Claims:          claims,
	}
	if custom, ok := claims.Custom.(*clerkCustomClaims); ok && custom != nil {
		id.Email = custom.Email
		id.Name = custom.Name
	}
	return id, nil
}

// VerifyClerkRequest verifies a Clerk session JWT carried on the request and
// returns the same Session shape SessionStore returns, so route handlers do
// not care which path authenticated them.
//
// Returns (nil, nil) — NOT an error — when:
//   - the kill switch is off
//   - there is no Authorization Bearer header (a `__session` cookie alone is
//     always nil: we are cookie-blind)
//   - the token does not verify
//   - resolveLocalUser is nil, or returns no local row yet
//
// Returns an error only when the resolver fails (a DB problem, which the caller
// should surface as a 500 rather than silently downgrade to anonymous).
func VerifyClerkRequest(r *http.Request, resolveLocalUser LocalUserResolver) (*Session, error) {
	if !clerkIsEnabled() {
		return nil, nil
	}

	token, err := extractClerkToken(r)
	if err != nil {
		return nil, nil // anonymous request, not a failure
	}

	id, err := VerifyClerkToken(r.Context(), token)
	if err != nil {
		// Reason class only. Never the token, the email, or the phone.
		log.Printf("[clerk-auth] bearer token not accepted on %s %s (%T)", r.Method, r.URL.Path, err)
		return nil, nil
	}

	if resolveLocalUser == nil {
		return nil, nil
	}
	session, err := resolveLocalUser(*id)
	if err != nil {
		return nil, err
	}
	if session == nil {
		// Verified Clerk user with no local row yet: anonymous, not an error.
		// The exchange endpoint is what creates the row.
		return nil, nil
	}
	return session, nil
}

// _resetClerkState restores the production JWKS source. Test-only; production
// code MUST NOT call it.
func _resetClerkState() {
	clerkJWKS = newHTTPJWKSSource()
}
