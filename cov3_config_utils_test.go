package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSplitCSV(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{",,,", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b ,, c ,", []string{"a", "b", "c"}},
		{"user_1,user_1", []string{"user_1", "user_1"}},
	}
	for _, tc := range cases {
		if got := splitCSV(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitCSV(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
}

func TestPluralS(t *testing.T) {
	for n, want := range map[int]string{0: "s", 1: "", 2: "s", -1: "s", 100: "s"} {
		if got := pluralS(n); got != want {
			t.Errorf("pluralS(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestLoadDotEnv(t *testing.T) {
	for _, k := range []string{"COV3_PLAIN", "COV3_EXPORTED", "COV3_QUOTED_EQ", "COV3_PRESET", "COV3_SPACED", "COV3_COMMENTED"} {
		t.Setenv(k, "") // registers restore; empty counts as unset for loadDotEnv
	}
	t.Setenv("COV3_PRESET", "from-env")

	path := filepath.Join(t.TempDir(), ".env")
	contents := strings.Join([]string{
		"# a comment",
		"",
		"COV3_PLAIN=plain",
		"export COV3_EXPORTED=exported",
		"COV3_QUOTED_EQ=a=b=c",
		"COV3_PRESET=from-file",
		"  COV3_SPACED  =  spaced value  ",
		"#COV3_COMMENTED=nope",
		"no-equals-sign-line",
	}, "\n")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	loadDotEnv(path)

	want := map[string]string{
		"COV3_PLAIN":     "plain",
		"COV3_EXPORTED":  "exported",
		"COV3_QUOTED_EQ": "a=b=c",
		"COV3_PRESET":    "from-env", // the environment wins over the file
		"COV3_SPACED":    "spaced value",
		"COV3_COMMENTED": "",
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}

	// Idempotent: loading twice changes nothing.
	loadDotEnv(path)
	if os.Getenv("COV3_PLAIN") != "plain" || os.Getenv("COV3_PRESET") != "from-env" {
		t.Error("second load changed values")
	}
	// A missing file is fine.
	loadDotEnv(filepath.Join(t.TempDir(), "absent.env"))
}

func TestLoadConfig_DefaultsAndOverrides(t *testing.T) {
	t.Chdir(t.TempDir()) // no .env in the way
	managed := []string{
		"PORT", "DATABASE_URL", "NPS_INTERVAL_DAYS", "ANTHROPIC_API_KEY", "LOGODEV_TOKEN", "LOGODEV_SECRET",
		"GOOGLE_PLACES_API_KEY", "WEBHOOK_BASE_URL", "APP_BASE_URL", "EMAIL_WEBHOOK_SECRET",
		"RAILWAY_ENVIRONMENT_NAME", "TEST_MODE", "SENDGRID_API_KEY", "BLUECART_API_KEY", "GH_PAT", "GH_REPO",
		"STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET", "WATT_API_KEY", "ADMIN_CLERK_USER_IDS", "GEMINI_API_KEY", "OPENAI_API_KEY",
	}
	for _, k := range managed {
		t.Setenv(k, "")
	}

	def := loadConfig()
	if def.Port != "8080" {
		t.Errorf("default port = %q", def.Port)
	}
	if !strings.HasPrefix(def.DatabaseURL, "postgres://") {
		t.Errorf("default database url = %q", def.DatabaseURL)
	}
	if def.NPSIntervalDays != 90 {
		t.Errorf("default nps interval = %d", def.NPSIntervalDays)
	}
	if def.SecureCookies || def.TestMode {
		t.Errorf("SecureCookies=%v TestMode=%v, want both false", def.SecureCookies, def.TestMode)
	}
	if def.AppBaseURL != defaultAppBaseURL {
		t.Errorf("AppBaseURL = %q", def.AppBaseURL)
	}
	if len(def.AdminClerkUserIDs) != 0 {
		t.Errorf("admins = %v, want none", def.AdminClerkUserIDs)
	}

	t.Setenv("PORT", "9999")
	t.Setenv("DATABASE_URL", "postgres://example/db")
	t.Setenv("NPS_INTERVAL_DAYS", "30")
	t.Setenv("ANTHROPIC_API_KEY", "ak")
	t.Setenv("STRIPE_SECRET_KEY", "sk")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "wh")
	t.Setenv("EMAIL_WEBHOOK_SECRET", "es")
	t.Setenv("RAILWAY_ENVIRONMENT_NAME", "production")
	t.Setenv("TEST_MODE", "true")
	t.Setenv("APP_BASE_URL", "https://example.test/")
	t.Setenv("ADMIN_CLERK_USER_IDS", " user_a , user_b ,")
	cfg := loadConfig()
	if cfg.Port != "9999" || cfg.DatabaseURL != "postgres://example/db" || cfg.NPSIntervalDays != 30 {
		t.Errorf("overrides not applied: %+v", cfg)
	}
	if cfg.AnthropicAPIKey != "ak" || cfg.StripeSecretKey != "sk" || cfg.StripeWebhookSecret != "wh" || cfg.EmailWebhookSecret != "es" {
		t.Errorf("secrets not read: %+v", cfg)
	}
	if !cfg.SecureCookies || !cfg.TestMode {
		t.Errorf("SecureCookies=%v TestMode=%v, want both true", cfg.SecureCookies, cfg.TestMode)
	}
	if cfg.AppBaseURL != "https://example.test" {
		t.Errorf("AppBaseURL = %q (trailing slash should be trimmed)", cfg.AppBaseURL)
	}
	if !reflect.DeepEqual(cfg.AdminClerkUserIDs, []string{"user_a", "user_b"}) {
		t.Errorf("admins = %v", cfg.AdminClerkUserIDs)
	}

	for _, bad := range []string{"0", "-5", "abc"} {
		t.Setenv("NPS_INTERVAL_DAYS", bad)
		if got := loadConfig().NPSIntervalDays; got != 90 {
			t.Errorf("NPS_INTERVAL_DAYS=%q gave %d, want default 90", bad, got)
		}
	}
	t.Setenv("TEST_MODE", "1")
	if loadConfig().TestMode {
		t.Error("TEST_MODE only enables on the exact string \"true\"")
	}
}

func TestLoadConfig_ReadsDotEnvButEnvironmentWins(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("PORT", "")
	t.Setenv("WATT_API_KEY", "real-env")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PORT=7777\nWATT_API_KEY=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := loadConfig()
	if cfg.Port != "7777" {
		t.Errorf("port = %q, want 7777 from .env", cfg.Port)
	}
	if cfg.WattAPIKey != "real-env" {
		t.Errorf("WattAPIKey = %q, environment must win over .env", cfg.WattAPIKey)
	}
}

func TestServeWithOGTags(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Missing index.html: 404.
	rr := httptest.NewRecorder()
	serveWithOGTags(rr, httptest.NewRequest("GET", "/r/ABC", nil))
	if rr.Code != http.StatusNotFound {
		t.Errorf("missing index: got %d, want 404", rr.Code)
	}

	page := `<html><head>` +
		`<meta property="og:title" content="Groflo — Your AI Shopping Assistant" />` +
		`<meta property="og:description" content="Penny helps your household save on groceries with smart suggestions and shared lists." />` +
		`</head><body>app</body></html>`
	if err := os.MkdirAll(filepath.Join(dir, "static"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "static", "index.html"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}

	rr = httptest.NewRecorder()
	serveWithOGTags(rr, httptest.NewRequest("GET", "/r/ABC", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("referral: got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type = %q", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Join me on Groflo!") || strings.Contains(body, "Your AI Shopping Assistant") {
		t.Errorf("referral og tags not injected: %s", body)
	}
	if !strings.Contains(body, "<body>app</body>") {
		t.Error("page body lost")
	}

	rr = httptest.NewRecorder()
	serveWithOGTags(rr, httptest.NewRequest("GET", "/join/XYZ", nil))
	if !strings.Contains(rr.Body.String(), "You&#39;re invited to a Groflo household!") {
		t.Errorf("invite og title missing or not HTML-escaped: %s", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "Join me on Groflo!") {
		t.Error("invite page got referral copy")
	}

	// The code in the URL is never reflected into the page.
	rr = httptest.NewRecorder()
	serveWithOGTags(rr, httptest.NewRequest("GET", "/join/%22%3E%3Cscript%3Ealert(1)%3C/script%3E", nil))
	if strings.Contains(rr.Body.String(), "<script>alert") {
		t.Error("URL content reflected into page")
	}
}
