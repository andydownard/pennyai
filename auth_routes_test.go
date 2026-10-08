package main

// auth_routes_test.go — retired sign-in routes, the test-only login route, boot
// decisions, admin access, and base URL selection.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newAuthMux(server *Server) *http.ServeMux {
	mux := http.NewServeMux()
	registerAuthRoutes(mux, server) // the same function main uses
	return mux
}

func TestRetiredAuth_AllTenPathsAnswer410(t *testing.T) {
	server := newTestServer(t)
	mux := newAuthMux(server)
	if len(retiredAuthPaths) != 10 {
		t.Fatalf("expected 10 retired paths, have %d", len(retiredAuthPaths))
	}
	for _, p := range []string{
		"/auth/send-code", "/auth/verify-code", "/auth/google",
		"/auth/email/register", "/auth/email/login", "/auth/email/forgot",
		"/auth/email/reset", "/auth/email/magic-link", "/auth/email/magic-verify",
		"/api/auth/link",
	} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(method+" "+p, func(t *testing.T) {
				rr := doRequestMux(mux, method, p, map[string]string{"phone_number": "5551234567"}, nil)
				if rr.Code != http.StatusGone {
					t.Fatalf("status: got %d want 410", rr.Code)
				}
				want := "This sign-in method has been retired. Use Continue with Rally."
				if got := parseJSON(rr)["error"]; got != want {
					t.Errorf("error: got %v", got)
				}
			})
		}
	}
	// A retired call must write nothing.
	if n := countRows(t, server, "SELECT COUNT(*) FROM users"); n != 0 {
		t.Errorf("users rows: got %d want 0", n)
	}
}

func TestRetiredTexting_PathsAnswer410(t *testing.T) {
	server := newTestServer(t)
	mux := newAuthMux(server) // registerAuthRoutes: the function main uses
	if len(retiredTextingPaths) != 3 {
		t.Fatalf("expected 3 retired texting paths, have %d", len(retiredTextingPaths))
	}
	for _, p := range []string{"/api/sms", "/api/me/sms-consent", "/api/penny.vcf"} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
			t.Run(method+" "+p, func(t *testing.T) {
				rr := doRequestMux(mux, method, p, map[string]string{"From": "+15551234567", "Body": "milk"}, nil)
				if rr.Code != http.StatusGone {
					t.Fatalf("status: got %d want 410", rr.Code)
				}
				if got := parseJSON(rr)["error"]; got != "Texting is no longer part of Groflo." {
					t.Errorf("error: got %v", got)
				}
			})
		}
	}
	// A retired call must write nothing: no user, no list item.
	if n := countRows(t, server, "SELECT COUNT(*) FROM users"); n != 0 {
		t.Errorf("users rows: got %d want 0", n)
	}
	if n := countRows(t, server, "SELECT COUNT(*) FROM list_items"); n != 0 {
		t.Errorf("list_items rows: got %d want 0", n)
	}
}

func TestRetiredAuth_LiveRoutesStillRegistered(t *testing.T) {
	server := newTestServer(t)
	mux := newAuthMux(server)
	for _, p := range []string{"/auth/me", "/auth/logout", "/api/config", "/auth/clerk/exchange"} {
		_, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, p, nil))
		if pattern == "" {
			t.Errorf("%s is not registered", p)
		}
	}
}

func TestLoginTestMode_Works(t *testing.T) {
	server := newTestServer(t)
	server.config.TestMode = true
	mux := newAuthMux(server)

	t.Run("mints a named user and a session", func(t *testing.T) {
		rr := doRequestMux(mux, http.MethodPost, "/auth/test-login", map[string]string{"name": "Pat"}, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d (%s)", rr.Code, rr.Body.String())
		}
		body := parseJSON(rr)
		cookie := getSessionCookie(rr)
		if body["success"] != true || cookie == nil || !cookie.HttpOnly {
			t.Fatalf("body=%v cookie=%+v", body, cookie)
		}
		var name string
		server.db.QueryRow("SELECT name FROM users WHERE id = ?", body["user_id"]).Scan(&name)
		if name != "Pat" {
			t.Errorf("name=%q", name)
		}
		sess, ok := server.sessions.GetSession(cookie.Value)
		if !ok || sess.UserID != body["user_id"] {
			t.Errorf("session: %+v ok=%v", sess, ok)
		}
	})

	t.Run("a phone_number field is ignored", func(t *testing.T) {
		users := countRows(t, server, "SELECT COUNT(*) FROM users")
		for _, phone := range []string{"212-555-0101", "123"} {
			rr := doRequestMux(mux, http.MethodPost, "/auth/test-login", map[string]string{"phone_number": phone}, nil)
			if rr.Code != http.StatusOK {
				t.Fatalf("phone %q: status: got %d (%s)", phone, rr.Code, rr.Body.String())
			}
			if n := countRows(t, server, "SELECT COUNT(*) FROM users WHERE id = ? AND phone IS NULL", parseJSON(rr)["user_id"]); n != 1 {
				t.Errorf("phone %q: user must hold no phone", phone)
			}
		}
		if n := countRows(t, server, "SELECT COUNT(*) FROM users WHERE phone IS NOT NULL"); n != 0 {
			t.Errorf("rows holding a phone: got %d want 0", n)
		}
		if n := countRows(t, server, "SELECT COUNT(*) FROM users"); n != users+2 {
			t.Errorf("users: %d -> %d, want +2 (each call mints a fresh user)", users, n)
		}
	})

	t.Run("without phone mints a phone-less user with a household", func(t *testing.T) {
		before := countRows(t, server, "SELECT COUNT(*) FROM households")
		rr := doRequestMux(mux, http.MethodPost, "/auth/test-login", nil, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d (%s)", rr.Code, rr.Body.String())
		}
		id := parseJSON(rr)["user_id"]
		if n := countRows(t, server, "SELECT COUNT(*) FROM users WHERE id = ? AND phone IS NULL", id); n != 1 {
			t.Errorf("phone-less user rows: got %d want 1", n)
		}
		if n := countRows(t, server, "SELECT COUNT(*) FROM households"); n != before+1 {
			t.Errorf("households: got %d want %d", n, before+1)
		}
		if n := countRows(t, server, "SELECT COUNT(*) FROM household_members WHERE user_id = ?", id); n != 1 {
			t.Errorf("household_members: got %d want 1", n)
		}
	})

}

func TestLoginTestMode_RefusedOtherwise(t *testing.T) {
	server := newTestServer(t) // TestMode false

	t.Run("route is not registered", func(t *testing.T) {
		mux := newAuthMux(server)
		_, pattern := mux.Handler(httptest.NewRequest(http.MethodPost, "/auth/test-login", nil))
		if pattern != "" {
			t.Errorf("route must not exist when TestMode is off; matched %q", pattern)
		}
	})

	t.Run("handler refuses by itself", func(t *testing.T) {
		rr := doRequest(server.handleTestLogin, http.MethodPost, "/auth/test-login", map[string]string{"phone_number": "2125550101"}, nil)
		if rr.Code != http.StatusNotFound {
			t.Errorf("status: got %d want 404", rr.Code)
		}
		if getSessionCookie(rr) != nil {
			t.Error("no cookie may be set")
		}
		if n := countRows(t, server, "SELECT COUNT(*) FROM users"); n != 0 {
			t.Errorf("users: got %d want 0", n)
		}
		if n := countRows(t, server, "SELECT COUNT(*) FROM sessions"); n != 0 {
			t.Errorf("sessions: got %d want 0", n)
		}
	})
}

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestBootProblems(t *testing.T) {
	healthy := Config{}
	goodClerk := map[string]string{
		"CLERK_ENABLED": "1", "CLERK_JWT_ISSUER": "https://clerk.example.com", "CLERK_AUTHORIZED_PARTIES": "https://app.example.com",
	}
	with := func(base map[string]string, kv ...string) map[string]string {
		out := map[string]string{}
		for k, v := range base {
			out[k] = v
		}
		for i := 0; i < len(kv); i += 2 {
			out[kv[i]] = kv[i+1]
		}
		return out
	}
	has := func(list []string, sub string) bool {
		for _, s := range list {
			if strings.Contains(s, sub) {
				return true
			}
		}
		return false
	}

	t.Run("healthy config has no problems", func(t *testing.T) {
		f, w := bootProblems(healthy, envMap(goodClerk))
		if len(f) != 0 || len(w) != 0 {
			t.Errorf("fatal=%v warnings=%v", f, w)
		}
	})
	t.Run("clerk enabled without issuer is fatal", func(t *testing.T) {
		f, _ := bootProblems(healthy, envMap(with(goodClerk, "CLERK_JWT_ISSUER", "")))
		if !has(f, "CLERK_JWT_ISSUER") {
			t.Errorf("fatal=%v", f)
		}
	})
	t.Run("clerk enabled without azp is fatal", func(t *testing.T) {
		f, _ := bootProblems(healthy, envMap(with(goodClerk, "CLERK_AUTHORIZED_PARTIES", " , ")))
		if !has(f, "CLERK_AUTHORIZED_PARTIES") {
			t.Errorf("fatal=%v", f)
		}
	})
	t.Run("clerk secrets are fatal", func(t *testing.T) {
		for _, name := range []string{"CLERK_SECRET_KEY", "CLERK_WEBHOOK_SIGNING_SECRET"} {
			f, _ := bootProblems(healthy, envMap(with(goodClerk, name, "sk_x")))
			if !has(f, name) {
				t.Errorf("%s: fatal=%v", name, f)
			}
		}
	})
	t.Run("clerk off outside test mode warns loudly", func(t *testing.T) {
		f, w := bootProblems(healthy, envMap(nil))
		if len(f) != 0 || !has(w, "nobody can sign in") {
			t.Errorf("fatal=%v warnings=%v", f, w)
		}
	})
	t.Run("clerk off in test mode is quiet", func(t *testing.T) {
		cfg := healthy
		cfg.TestMode = true
		f, w := bootProblems(cfg, envMap(nil))
		if len(f) != 0 || has(w, "nobody can sign in") {
			t.Errorf("fatal=%v warnings=%v", f, w)
		}
	})
	t.Run("test mode on a deployed service is fatal", func(t *testing.T) {
		cfg := healthy
		cfg.TestMode = true
		f, _ := bootProblems(cfg, envMap(map[string]string{"RAILWAY_ENVIRONMENT_NAME": "production"}))
		if !has(f, "TEST_MODE") {
			t.Errorf("fatal=%v", f)
		}
	})
	t.Run("webhook url with no email secret warns", func(t *testing.T) {
		f, w := bootProblems(Config{WebhookBaseURL: "https://x.example"}, envMap(goodClerk))
		if len(f) != 0 || !has(w, "EMAIL_WEBHOOK_SECRET") {
			t.Errorf("fatal=%v warnings=%v", f, w)
		}
	})
	t.Run("webhook url with an email secret is quiet", func(t *testing.T) {
		f, w := bootProblems(Config{WebhookBaseURL: "https://x.example", EmailWebhookSecret: "s"}, envMap(goodClerk))
		if len(f) != 0 || len(w) != 0 {
			t.Errorf("fatal=%v warnings=%v", f, w)
		}
	})
}

func TestIsAdmin(t *testing.T) {
	server := newTestServer(t)
	server.config.AdminClerkUserIDs = []string{"user_owner", "user_other"}

	// A phone-less owner, bound to a Clerk id on the allowlist.
	server.db.Exec("INSERT INTO users (id, clerk_user_id) VALUES ('owner-1', 'user_owner')")
	// A Clerk user not on the list.
	server.db.Exec("INSERT INTO users (id, clerk_user_id) VALUES ('rando-1', 'user_rando')")
	// A user with no Clerk id at all.
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('plain-1', '5550001111')")

	cases := []struct {
		name string
		s    Session
		want bool
	}{
		{"old session phone equal to the former admin phone", Session{UserID: "plain-1", PhoneNumber: testAdminPhone}, false},
		{"clerk id on allowlist, no phone", Session{UserID: "owner-1"}, true},
		{"clerk id not on allowlist", Session{UserID: "rando-1"}, false},
		{"no clerk id", Session{UserID: "plain-1", PhoneNumber: "5550001111"}, false},
		{"unknown user", Session{UserID: "ghost"}, false},
		{"empty session", Session{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := server.isAdmin(c.s); got != c.want {
				t.Errorf("isAdmin = %v want %v", got, c.want)
			}
		})
	}

	t.Run("allowlist is not consulted when empty", func(t *testing.T) {
		server.config.AdminClerkUserIDs = nil
		if server.isAdmin(Session{UserID: "owner-1"}) {
			t.Error("no allowlist, no admin")
		}
	})
}

func TestIsAdmin_RequireAdminAndMe(t *testing.T) {
	server := newTestServer(t)
	server.config.AdminClerkUserIDs = []string{"user_owner"}
	server.db.Exec("INSERT INTO users (id, clerk_user_id) VALUES ('owner-1', 'user_owner')")
	server.db.Exec("INSERT INTO users (id, clerk_user_id) VALUES ('rando-1', 'user_rando')")

	cookieFor := func(id string) *http.Cookie {
		tok, err := server.sessions.CreateSession(id, "", "clerk")
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		return &http.Cookie{Name: "session", Value: tok}
	}

	rr := doRequest(server.handleAdminDashboard, http.MethodGet, "/api/admin/dashboard", nil, []*http.Cookie{cookieFor("rando-1")})
	if rr.Code != http.StatusForbidden {
		t.Errorf("non-admin: got %d want 403", rr.Code)
	}
	if me := parseJSON(doRequest(server.handleMe, http.MethodGet, "/auth/me", nil, []*http.Cookie{cookieFor("rando-1")})); me["is_admin"] != false {
		t.Errorf("non-admin is_admin: %v", me["is_admin"])
	}
	if me := parseJSON(doRequest(server.handleMe, http.MethodGet, "/auth/me", nil, []*http.Cookie{cookieFor("owner-1")})); me["is_admin"] != true {
		t.Errorf("allowlisted owner is_admin: %v", me["is_admin"])
	}
	rr = doRequest(server.handleAdminDashboard, http.MethodGet, "/api/admin/dashboard", nil, []*http.Cookie{cookieFor("owner-1")})
	if rr.Code == http.StatusForbidden || rr.Code == http.StatusUnauthorized {
		t.Errorf("allowlisted owner must pass requireAdmin; got %d", rr.Code)
	}
}

func TestAppBaseURL(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"default", nil, "https://groflo.rallytasks.com"},
		{"app base url wins", map[string]string{"APP_BASE_URL": "https://a.example/", "WEBHOOK_BASE_URL": "https://w.example"}, "https://a.example"},
		{"falls back to webhook base url", map[string]string{"WEBHOOK_BASE_URL": "https://w.example//"}, "https://w.example"},
		{"blank app base url falls through", map[string]string{"APP_BASE_URL": "  ", "WEBHOOK_BASE_URL": "https://w.example"}, "https://w.example"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := appBaseURL(envMap(c.env)); got != c.want {
				t.Errorf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestRetiredAuth_AnyMethodAndNoRedirect(t *testing.T) {
	server := newTestServer(t)
	mux := newAuthMux(server)
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rr := doRequestMux(mux, method, "/auth/email/magic-verify?token=abc", nil, nil)
		if rr.Code != http.StatusGone {
			t.Errorf("%s magic-verify: got %d want 410", method, rr.Code)
		}
		if loc := rr.Header().Get("Location"); loc != "" {
			t.Errorf("%s magic-verify must not redirect; Location=%q", method, loc)
		}
	}
}

func TestLoginTestMode_MeReportsNoPhone(t *testing.T) {
	server := newTestServer(t)
	server.config.TestMode = true
	mux := newAuthMux(server)

	rr := doRequestMux(mux, http.MethodPost, "/auth/test-login", map[string]string{"phone_number": "2125550101", "name": "Pat"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d (%s)", rr.Code, rr.Body.String())
	}
	cookie := getSessionCookie(rr)
	me := parseJSON(doRequestMux(mux, http.MethodGet, "/auth/me", nil, []*http.Cookie{cookie}))
	for _, k := range []string{"needs_sms_consent", "phone_number"} {
		if _, ok := me[k]; ok {
			t.Errorf("/auth/me must not carry %q: %v", k, me)
		}
	}
	if me["name"] != "Pat" {
		t.Errorf("name: %v", me["name"])
	}
}

func TestMe_Contract(t *testing.T) {
	server := newTestServer(t)
	mux := newAuthMux(server)

	t.Run("no session is 401", func(t *testing.T) {
		if rr := doRequestMux(mux, http.MethodGet, "/auth/me", nil, nil); rr.Code != http.StatusUnauthorized {
			t.Errorf("got %d want 401", rr.Code)
		}
	})
	t.Run("invalid session is 401", func(t *testing.T) {
		rr := doRequestMux(mux, http.MethodGet, "/auth/me", nil, []*http.Cookie{{Name: "session", Value: "forged"}})
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("got %d want 401", rr.Code)
		}
	})
	t.Run("signed-in body", func(t *testing.T) {
		cookie, _ := loginUser(t, server, "5551234567")
		server.db.Exec("UPDATE users SET name = 'Pat' WHERE phone = '5551234567'")
		rr := doRequestMux(mux, http.MethodGet, "/auth/me", nil, []*http.Cookie{cookie})
		if rr.Code != http.StatusOK {
			t.Fatalf("got %d", rr.Code)
		}
		me := parseJSON(rr)
		for _, k := range []string{"phone_number", "needs_sms_consent"} {
			if _, ok := me[k]; ok {
				t.Errorf("/auth/me must not carry %q: %v", k, me)
			}
		}
		for _, k := range []string{"logged_in", "user_id", "name", "login_count", "is_admin", "can_submit_feedback", "auth_methods"} {
			if _, ok := me[k]; !ok {
				t.Errorf("missing key %q in %v", k, me)
			}
		}
		if me["logged_in"] != true || me["name"] != "Pat" {
			t.Errorf("body: %v", me)
		}
	})
	t.Run("phone-less user has no phone_number key", func(t *testing.T) {
		server.db.Exec("INSERT INTO users (id) VALUES ('nophone')")
		tok, _ := server.sessions.CreateSession("nophone", "", "clerk")
		me := parseJSON(doRequestMux(mux, http.MethodGet, "/auth/me", nil, []*http.Cookie{{Name: "session", Value: tok}}))
		if _, ok := me["phone_number"]; ok {
			t.Errorf("phone_number must be absent: %v", me)
		}
	})
}

func TestLogout_ClearsCookieAndRowAndIsRepeatable(t *testing.T) {
	server := newTestServer(t)
	mux := newAuthMux(server)
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequestMux(mux, http.MethodPost, "/auth/logout", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d", rr.Code)
	}
	cleared := getSessionCookie(rr)
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("cookie must be cleared with Max-Age < 0: %+v", cleared)
	}
	if n := countRows(t, server, "SELECT COUNT(*) FROM sessions WHERE token = ?", cookie.Value); n != 0 {
		t.Errorf("session row must be gone, got %d", n)
	}
	if rr := doRequestMux(mux, http.MethodGet, "/auth/me", nil, []*http.Cookie{cookie}); rr.Code != http.StatusUnauthorized {
		t.Errorf("me after logout: %d want 401", rr.Code)
	}
	if rr := doRequestMux(mux, http.MethodPost, "/auth/logout", nil, []*http.Cookie{cookie}); rr.Code != http.StatusOK {
		t.Errorf("second logout: got %d want 200", rr.Code)
	}
}

func TestSettingsPatch_PhoneCannotBeSet(t *testing.T) {
	server := newTestServer(t)
	cookie, userID := loginUser(t, server, "5551234567")

	for _, target := range []string{"5559998888", testAdminPhone, "+1" + testAdminPhone, ""} {
		doRequest(server.handleSettings, http.MethodPatch, "/api/settings", map[string]string{"phone": target, "name": "Pat"}, []*http.Cookie{cookie})
		var phone, name string
		server.db.QueryRow("SELECT phone, name FROM users WHERE id = ?", userID).Scan(&phone, &name)
		if phone != "5551234567" {
			t.Errorf("phone changed to %q after PATCH with %q", phone, target)
		}
		if name != "Pat" {
			t.Errorf("other fields must still update; name=%q", name)
		}
	}
	// And the admin gate did not move.
	if server.isAdmin(Session{UserID: userID, PhoneNumber: "5551234567"}) {
		t.Error("user must not have become admin")
	}
	var holders int
	server.db.QueryRow("SELECT COUNT(*) FROM users WHERE phone = ?", testAdminPhone).Scan(&holders)
	if holders != 0 {
		t.Errorf("nobody may hold the admin phone; got %d", holders)
	}
}
