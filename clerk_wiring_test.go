package main

// clerk_wiring_test.go — STEP 7: the three places Rally sign-in touches
// existing surfaces.
//
//  1. requireSession gains a Bearer fallback. The cookie path is unchanged and
//     still wins; the Clerk path only runs when there is no usable cookie.
//  2. /api/config gains an additive clerk_enabled field.
//  3. The CSP widens for Clerk's Frontend API — and ONLY when both
//     CLERK_ENABLED=1 and CLERK_FRONTEND_API_ORIGIN are set. With the flag off
//     the header must be byte-identical to today's.
//
// The dark build must be indistinguishable from today's: with CLERK_ENABLED
// unset, a valid Bearer token still gets 401 and the CSP strings are unchanged.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------
// requireSession — Bearer fallback
// -----------------------------------------------------------------------

// clerkSessionHarness creates a user bound to a Clerk identity so the
// lookup-only resolver can find them, without going through the exchange.
func clerkSessionHarness(t *testing.T) (*Server, string, string) {
	t.Helper()
	server := newTestServer(t, &MockTwilioClient{})
	user, err := upsertUser(server.db, "5551230000")
	if err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	if _, err := server.db.Exec("UPDATE users SET clerk_user_id = ? WHERE id = ?", "user_bearer_1", user.ID); err != nil {
		t.Fatalf("binding clerk identity: %v", err)
	}
	return server, user.ID, "user_bearer_1"
}

func clerkGetList(server *Server, bearer string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/list", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	server.handleList(rr, req)
	return rr
}

func TestRequireSession_BearerOnlyAuthenticates(t *testing.T) {
	server, userID, sub := clerkSessionHarness(t)
	priv := newTestKeypair(t)
	withClerkSatelliteEnv(t, priv, nil)
	createListItem(t, server.db, userID, "bearer milk", "Dairy")

	rr := clerkGetList(server, mintValidClerkJWT(t, priv, sub))
	if rr.Code != http.StatusOK {
		t.Fatalf("a valid Bearer with no cookie must authenticate: got %d (%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "bearer milk") {
		t.Error("the request did not resolve to the right user's list")
	}
}

func TestRequireSession_InvalidBearerIs401(t *testing.T) {
	server, _, _ := clerkSessionHarness(t)
	priv := newTestKeypair(t)
	withClerkSatelliteEnv(t, priv, nil)

	o := defaultClerkTokenOpts()
	o.azp = "https://attacker.example"
	if rr := clerkGetList(server, mintClerkTestJWT(t, priv, o)); rr.Code != http.StatusUnauthorized {
		t.Fatalf("an unverified Bearer must be 401: got %d", rr.Code)
	}
	if rr := clerkGetList(server, "garbage"); rr.Code != http.StatusUnauthorized {
		t.Fatalf("a malformed Bearer must be 401: got %d", rr.Code)
	}
}

func TestRequireSession_ClerkCookieIsIgnored(t *testing.T) {
	server, _, sub := clerkSessionHarness(t)
	priv := newTestKeypair(t)
	jwks := withClerkSatelliteEnv(t, priv, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/list", nil)
	req.AddCookie(&http.Cookie{Name: "__session", Value: mintValidClerkJWT(t, priv, sub)})
	rr := httptest.NewRecorder()
	server.handleList(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("a __session cookie must never authenticate here: got %d", rr.Code)
	}
	if jwks.Lookups() != 0 {
		t.Errorf("the JWKS must not be consulted for a cookie; lookups=%d", jwks.Lookups())
	}
}

func TestRequireSession_DarkBuildIgnoresBearerEntirely(t *testing.T) {
	// With the kill switch off the build must behave exactly as it does today:
	// a perfectly valid Clerk token is simply not a credential.
	server, _, sub := clerkSessionHarness(t)
	priv := newTestKeypair(t)
	jwks := withClerkSatelliteEnv(t, priv, map[string]string{"CLERK_ENABLED": "0"})

	rr := clerkGetList(server, mintValidClerkJWT(t, priv, sub))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("with CLERK_ENABLED off a Bearer must be ignored: got %d", rr.Code)
	}
	if jwks.Lookups() != 0 {
		t.Errorf("no verification work when disabled; lookups=%d", jwks.Lookups())
	}
}

func TestRequireSession_CookieWinsOverGarbageBearer(t *testing.T) {
	// The legacy path must never be degraded by the presence of a bad Bearer.
	server := newTestServer(t, &MockTwilioClient{})
	priv := newTestKeypair(t)
	jwks := withClerkSatelliteEnv(t, priv, nil)
	cookie, userID := loginUser(t, server, "5559990000")
	createListItem(t, server.db, userID, "cookie bread", "Bakery")

	rr := clerkGetList(server, "not-a-jwt-at-all", cookie)
	if rr.Code != http.StatusOK {
		t.Fatalf("a valid cookie must win regardless of the Authorization header: got %d (%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "cookie bread") {
		t.Error("the cookie user's list was not returned")
	}
	if jwks.Lookups() != 0 {
		t.Errorf("a usable cookie must short-circuit before any Clerk work; lookups=%d", jwks.Lookups())
	}
}

func TestRequireSession_BearerIgnoredForUnboundClerkUser(t *testing.T) {
	// A verified Clerk user who has never completed an exchange has no local
	// row, so they are anonymous — not a 500.
	server := newTestServer(t, &MockTwilioClient{})
	priv := newTestKeypair(t)
	withClerkSatelliteEnv(t, priv, nil)

	if rr := clerkGetList(server, mintValidClerkJWT(t, priv, "user_never_exchanged")); rr.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want 401", rr.Code)
	}
}

func TestRequireSession_PreservesLegacy401Messages(t *testing.T) {
	// The Bearer fallback must not change what an unauthenticated legacy request
	// is told: "Not logged in" with no cookie, "Session expired" with a stale one.
	server := newTestServer(t, &MockTwilioClient{})
	t.Setenv("CLERK_ENABLED", "")

	noCookie := clerkGetList(server, "")
	if noCookie.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d", noCookie.Code)
	}
	if msg := parseJSON(noCookie)["error"]; msg != "Not logged in" {
		t.Errorf("no-cookie message: got %v want \"Not logged in\"", msg)
	}

	stale := clerkGetList(server, "", &http.Cookie{Name: "session", Value: "expired-or-forged"})
	if stale.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d", stale.Code)
	}
	if msg := parseJSON(stale)["error"]; msg != "Session expired" {
		t.Errorf("stale-cookie message: got %v want \"Session expired\"", msg)
	}
}

// -----------------------------------------------------------------------
// /api/config — additive only
// -----------------------------------------------------------------------

func TestAPIConfig_ClerkEnabledIsAdditive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.GoogleClientID = "google-client-id-123"

	t.Run("backward compatible with the flag off", func(t *testing.T) {
		t.Setenv("CLERK_ENABLED", "")
		rr := doRequest(server.handleAPIConfig, http.MethodGet, "/api/config", nil, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d", rr.Code)
		}
		body := parseJSON(rr)
		// Existing contract: same key, same type, same value.
		gcid, ok := body["google_client_id"].(string)
		if !ok || gcid != "google-client-id-123" {
			t.Errorf("google_client_id must stay a string with the same value; got %#v", body["google_client_id"])
		}
		enabled, ok := body["clerk_enabled"].(bool)
		if !ok {
			t.Fatalf("clerk_enabled must be a boolean; got %#v", body["clerk_enabled"])
		}
		if enabled {
			t.Error("clerk_enabled must default to false")
		}
	})

	t.Run("reports true when enabled", func(t *testing.T) {
		t.Setenv("CLERK_ENABLED", "1")
		body := parseJSON(doRequest(server.handleAPIConfig, http.MethodGet, "/api/config", nil, nil))
		if body["clerk_enabled"] != true {
			t.Errorf("clerk_enabled: got %#v want true", body["clerk_enabled"])
		}
		if body["google_client_id"] != "google-client-id-123" {
			t.Error("enabling Rally sign-in must not disturb the Google client id")
		}
	})

	t.Run("still rejects the wrong method", func(t *testing.T) {
		rr := doRequest(server.handleAPIConfig, http.MethodPost, "/api/config", nil, nil)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status: got %d want 405", rr.Code)
		}
	})
}

// -----------------------------------------------------------------------
// CSP — byte-identical when off, widened only when fully configured
// -----------------------------------------------------------------------

// These two strings are copied VERBATIM from server.go as of this change. If a
// future edit alters a directive, this test fails and the author has to decide
// deliberately rather than drift.
const (
	wantEmbedCSP = "default-src 'self'; script-src 'self' https://accounts.google.com https://apis.google.com https://www.googletagmanager.com; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' https://accounts.google.com https://www.google-analytics.com https://www.googletagmanager.com; frame-src https://accounts.google.com; frame-ancestors 'self' https://vibeflourishing.com https://vibeflourishing-production.up.railway.app https://deck.groflo.io https://deck.thrift.ly https://localhost:5173"
	wantAppCSP   = "default-src 'self'; script-src 'self' https://accounts.google.com https://apis.google.com https://www.googletagmanager.com; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' https://accounts.google.com https://*.ingest.us.sentry.io https://us.i.posthog.com https://www.google-analytics.com https://www.googletagmanager.com https://penny.rallytasks.com; frame-src https://accounts.google.com; frame-ancestors 'self' https://penny.rallytasks.com; font-src 'self'"
)

func TestCSPHeader_UnchangedWhenClerkIsOff(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
	}{
		{"flag unset", map[string]string{"CLERK_ENABLED": "", "CLERK_FRONTEND_API_ORIGIN": ""}},
		{"flag on but no FAPI origin", map[string]string{"CLERK_ENABLED": "1", "CLERK_FRONTEND_API_ORIGIN": ""}},
		{"FAPI origin set but flag off", map[string]string{"CLERK_ENABLED": "0", "CLERK_FRONTEND_API_ORIGIN": "https://clerk.groflo.io"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			if got := cspHeader("/embed/list"); got != wantEmbedCSP {
				t.Errorf("embed CSP changed:\n got: %s\nwant: %s", got, wantEmbedCSP)
			}
			if got := cspHeader("/"); got != wantAppCSP {
				t.Errorf("app CSP changed:\n got: %s\nwant: %s", got, wantAppCSP)
			}
		})
	}
}

func TestCSPHeader_WidensOnlyWhenFullyConfigured(t *testing.T) {
	const fapi = "https://clerk.groflo.io"
	t.Setenv("CLERK_ENABLED", "1")
	t.Setenv("CLERK_FRONTEND_API_ORIGIN", fapi)

	for _, path := range []string{"/", "/embed/list"} {
		t.Run(path, func(t *testing.T) {
			got := cspHeader(path)

			// Every directive that needs the Clerk origin has it.
			for _, directive := range []string{"script-src", "connect-src", "frame-src"} {
				section := cspDirective(got, directive)
				if section == "" {
					t.Fatalf("CSP has no %s directive: %s", directive, got)
				}
				if !strings.Contains(section, fapi) {
					t.Errorf("%s must include the Clerk Frontend API origin; got %q", directive, section)
				}
			}

			// clerk-js uses a blob worker.
			if !strings.Contains(got, "worker-src 'self' blob:") {
				t.Errorf("CSP must allow a blob worker for clerk-js; got %s", got)
			}

			// Nothing pre-existing was dropped. Widening is strictly additive.
			baseline := wantAppCSP
			if strings.HasPrefix(path, "/embed") {
				baseline = wantEmbedCSP
			}
			for _, directive := range strings.Split(baseline, ";") {
				directive = strings.TrimSpace(directive)
				if directive == "" {
					continue
				}
				name := strings.SplitN(directive, " ", 2)[0]
				for _, source := range strings.Fields(directive)[1:] {
					if !strings.Contains(cspDirective(got, name), source) {
						t.Errorf("widening dropped %s from %s", source, name)
					}
				}
			}
		})
	}
}

// cspDirective returns the named directive's text (without its name).
func cspDirective(csp, name string) string {
	for _, d := range strings.Split(csp, ";") {
		d = strings.TrimSpace(d)
		if d == name || strings.HasPrefix(d, name+" ") {
			return strings.TrimSpace(strings.TrimPrefix(d, name))
		}
	}
	return ""
}

func TestCSPHeader_NoCredentialsHeaderOnAppPaths(t *testing.T) {
	// Access-Control-Allow-Credentials must stay absent: Rally sign-in uses a
	// Bearer header, not a cross-origin cookie, so nothing here needs it.
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleAPIConfig, http.MethodGet, "/api/config", nil, nil)
	if v := rr.Header().Get("Access-Control-Allow-Credentials"); v != "" {
		t.Errorf("Access-Control-Allow-Credentials should not be set; got %q", v)
	}
}
