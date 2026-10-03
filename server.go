package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// =============================================================================
// CONSTANTS
// =============================================================================

const (
	sessionDuration          = 30 * 24 * time.Hour
	inviteExpiry             = 7 * 24 * time.Hour
	referralPointsPerInvite  = 10
	referralRedeemThreshold  = 100
	referralRedeemCents      = 1000
	npsMinLogins             = 10
	logoCacheMaxAge          = 604800
	pennyQuoteRateLimit      = 10 * time.Second
	defaultSMSBufferDuration = 10 * time.Second
	maxMediaBytes            = 5 * 1024 * 1024
	maxImageBytes            = 5 * 1024 * 1024
	maxParseBodyBytes        = 10 * 1024 * 1024
)

// loadDotEnv reads a .env file and sets any variables not already in the environment.
// Supports lines like KEY=value and export KEY=value. Skips comments and blanks.
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // no .env file is fine
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Only set if not already in environment (env takes precedence)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func loadConfig() Config {
	loadDotEnv(".env")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://localhost:5432/pennyai?sslmode=disable"
	}
	npsInterval := 90
	if v := os.Getenv("NPS_INTERVAL_DAYS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			npsInterval = parsed
		}
	}

	// Parse FEEDBACK_PHONES: comma-separated phone numbers (digits only, no +1 prefix)
	var feedbackPhones []string
	if fp := os.Getenv("FEEDBACK_PHONES"); fp != "" {
		for _, p := range strings.Split(fp, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				feedbackPhones = append(feedbackPhones, p)
			}
		}
	}

	return Config{
		TwilioAccountSID:    os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:     os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioVerifyService: os.Getenv("TWILIO_VERIFY_SERVICE_SID"),
		TwilioPhoneNumber:   os.Getenv("TWILIO_PHONE_NUMBER"),
		AnthropicAPIKey:     os.Getenv("ANTHROPIC_API_KEY"),
		AdminPhone:          os.Getenv("ADMIN_PHONE"),
		LogoDevToken:        os.Getenv("LOGODEV_TOKEN"),
		LogoDevSecret:       os.Getenv("LOGODEV_SECRET"),
		GooglePlacesAPIKey:  os.Getenv("GOOGLE_PLACES_API_KEY"),
		NPSIntervalDays:     npsInterval,
		Port:                port,
		DatabaseURL:         databaseURL,
		WebhookBaseURL:      os.Getenv("WEBHOOK_BASE_URL"), // e.g. "https://app.groflo.io"
		SecureCookies:       os.Getenv("RAILWAY_ENVIRONMENT_NAME") != "",
		TestMode:            os.Getenv("TEST_MODE") == "true",
		GoogleClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		SendGridAPIKey:      os.Getenv("SENDGRID_API_KEY"),
		BlueCartAPIKey:      os.Getenv("BLUECART_API_KEY"),
		FeedbackPhones:      feedbackPhones,
		GitHubPAT:           os.Getenv("GH_PAT"),
		GitHubRepo:          os.Getenv("GH_REPO"),
		StripeSecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		WattAPIKey:          os.Getenv("WATT_API_KEY"),
		TimpagerURL:         os.Getenv("TIMPAGER_URL"),
		TimpagerAPIKey:      os.Getenv("TIMPAGER_API_KEY"),
		GeminiAPIKey:        os.Getenv("GEMINI_API_KEY"),
		OpenAIAPIKey:        os.Getenv("OPENAI_API_KEY"),
	}
}

// =============================================================================
// MAIN
// =============================================================================

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher by delegating to the underlying ResponseWriter.
// Required for SSE streaming through the logging middleware.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Content-Security-Policy, as two literal baselines. Extracted from the request
// handler so the Rally sign-in widening is testable — and so the test can assert
// byte-for-byte equality with these strings while the feature is off.
const (
	// Embed routes are framed by approved partner origins.
	cspEmbedBase = "default-src 'self'; script-src 'self' https://accounts.google.com https://apis.google.com https://www.googletagmanager.com; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' https://accounts.google.com https://www.google-analytics.com https://www.googletagmanager.com; frame-src https://accounts.google.com; frame-ancestors 'self' https://vibeflourishing.com https://vibeflourishing-production.up.railway.app https://deck.groflo.io https://deck.thrift.ly https://localhost:5173"
	// Every other route.
	cspAppBase = "default-src 'self'; script-src 'self' https://accounts.google.com https://apis.google.com https://www.googletagmanager.com; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' https://accounts.google.com https://*.ingest.us.sentry.io https://us.i.posthog.com https://www.google-analytics.com https://www.googletagmanager.com https://penny.rallytasks.com; frame-src https://accounts.google.com; frame-ancestors 'self' https://penny.rallytasks.com; font-src 'self'"
)

// cspHeader returns the Content-Security-Policy for a path.
//
// Output is byte-identical to the historical literals unless BOTH
// CLERK_ENABLED=1 and CLERK_FRONTEND_API_ORIGIN are set — so this ships dark
// with the rest of Rally sign-in. When both are set, the Clerk Frontend API
// origin is added to script-src, connect-src and frame-src, and worker-src is
// allowed a blob (clerk-js runs a blob worker). Widening is strictly additive:
// no existing source is ever removed.
func cspHeader(path string) string {
	base := cspAppBase
	if strings.HasPrefix(path, "/embed") {
		base = cspEmbedBase
	}

	origin := strings.TrimSpace(os.Getenv("CLERK_FRONTEND_API_ORIGIN"))
	if !clerkIsEnabled() || origin == "" {
		return base
	}

	out := make([]string, 0, 10)
	for _, directive := range strings.Split(base, "; ") {
		name := strings.SplitN(directive, " ", 2)[0]
		switch name {
		case "script-src", "connect-src", "frame-src":
			directive += " " + origin
		}
		out = append(out, directive)
	}
	out = append(out, "worker-src 'self' blob:")
	return strings.Join(out, "; ")
}

func main() {
	cfg := loadConfig()

	// pennyai is a Clerk SATELLITE: it verifies session JWTs against the Rally
	// primary's public JWKS and never calls Clerk's Backend API. Holding a Clerk
	// secret here is itself the misconfiguration, so refuse to boot on it —
	// whether or not CLERK_ENABLED is set. Webhooks and secrets live in timpager.
	if forbidden := clerkForbiddenSecrets(); len(forbidden) > 0 {
		log.Fatalf("Refusing to start: %s is set. pennyai is a Clerk satellite and must never hold a Clerk secret; unset these and restart.", strings.Join(forbidden, ", "))
	}

	// Sentry error monitoring — gracefully skipped if DSN not set
	if dsn := os.Getenv("SENTRY_DSN"); dsn != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              dsn,
			Environment:      os.Getenv("RAILWAY_ENVIRONMENT_NAME"),
			TracesSampleRate: 0.1,
		}); err != nil {
			log.Printf("[sentry] init failed: %v", err)
		} else {
			log.Println("[sentry] initialized")
			defer sentry.Flush(2 * time.Second)
		}
	}

	if !cfg.TestMode && (cfg.TwilioAccountSID == "" || cfg.TwilioAuthToken == "" || cfg.TwilioVerifyService == "") {
		log.Fatal(`
Missing Twilio credentials. Set these environment variables:

  export TWILIO_ACCOUNT_SID=ACxxxxxxxx
  export TWILIO_AUTH_TOKEN=xxxxxxxx
  export TWILIO_VERIFY_SERVICE_SID=VAxxxxxxxx

Get these from https://console.twilio.com (see README for full setup).
`)
	}

	// Open PostgreSQL database
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Wrap with PGDB for SQLite→Postgres SQL conversion
	pgdb := WrapPG(db)

	if err := initDB(pgdb.DB); err != nil {
		log.Fatalf("Schema initialization error: %v", err)
	}

	server := &Server{
		config:   cfg,
		db:       pgdb,
		sessions: NewSessionStore(pgdb.DB),
		twilio: &RealTwilioClient{
			AccountSID:    cfg.TwilioAccountSID,
			AuthToken:     cfg.TwilioAuthToken,
			VerifyService: cfg.TwilioVerifyService,
		},
		// AI parse functions use multi-provider fallback (ai_providers.go).
		// Primary: Anthropic, fallback: Gemini, then OpenAI.
		// Complexity tiers: text→Simple, commands→Medium, images/receipts→Hard.
		parseItems:   parseItemsWithClaude,   // Simple/Hard (auto-detected by image presence)
		parseCommand: parseCommandWithClaude,  // Medium (natural language understanding)
		parseReceipt: parseReceiptWithClaude,  // Hard (image parsing + fraud detection)
		smsBuffer:    newMessageBuffer(defaultSMSBufferDuration),
	}

	// Clean expired sessions every hour
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			server.sessions.CleanExpired()
		}
	}()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Check DB connectivity
		dbOK := true
		if err := server.db.Ping(); err != nil {
			dbOK = false
		}

		// Verify frontend assets exist (prevents deploying broken frontend)
		frontendOK := true
		if data, err := os.ReadFile("static/index.html"); err == nil {
			re := regexp.MustCompile(`(?:src|href)="(/assets/[^"]+)"`)
			for _, m := range re.FindAllStringSubmatch(string(data), -1) {
				if _, err := os.Stat("static" + m[1]); os.IsNotExist(err) {
					frontendOK = false
					break
				}
			}
		} else {
			frontendOK = false
		}

		if !frontendOK || !dbOK {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"degraded","frontend_ok":%v,"db_ok":%v}`, frontendOK, dbOK)
			return
		}
		w.Write([]byte(`{"status":"ok","frontend_ok":true,"db_ok":true,"version":"2026-03-15a"}`))
	})
	http.HandleFunc("/auth/send-code", server.handleSendCode)
	http.HandleFunc("/auth/verify-code", server.handleVerifyCode)
	http.HandleFunc("/auth/google", server.handleGoogleAuth)
	http.HandleFunc("/auth/email/register", server.handleEmailRegister)
	http.HandleFunc("/auth/email/login", server.handleEmailLogin)
	http.HandleFunc("/auth/email/forgot", server.handleEmailForgot)
	http.HandleFunc("/auth/email/reset", server.handleEmailReset)
	http.HandleFunc("/auth/email/magic-link", server.handleMagicLink)
	http.HandleFunc("/auth/email/magic-verify", server.handleMagicLinkVerify)
	// Rally sign-in (Clerk satellite). Dark behind CLERK_ENABLED: answers 404
	// until the flag is on. The four legacy auth routes above are untouched.
	http.HandleFunc("/auth/clerk/exchange", server.handleClerkExchange)
	http.HandleFunc("/auth/logout", server.handleLogout)
	http.HandleFunc("/auth/me", server.handleMe)
	http.HandleFunc("/api/me/sms-consent", server.handleSmsConsent)
	http.HandleFunc("/api/auth/link", server.handleAuthLink)
	http.HandleFunc("/api/config", server.handleAPIConfig)
	http.HandleFunc("/api/list/stream", server.handleListStream)
	http.HandleFunc("/api/list", server.handleList)
	http.HandleFunc("/api/list/", server.handleList)
	http.HandleFunc("/api/list/department-order", server.handleDepartmentOrder)
	http.HandleFunc("/api/list/clear", server.handleListClear)
	http.HandleFunc("/api/list/restore", server.handleListRestore)
	http.HandleFunc("/api/list/share", server.handleShareList)
	http.HandleFunc("/api/list/swaps", server.handleListSwaps)
	http.HandleFunc("/api/shared/", server.handleSharedView)
	http.HandleFunc("/api/list/brand-resolve", server.handleBrandResolve)
	http.HandleFunc("/api/saved-lists", server.handleSavedLists)
	http.HandleFunc("/api/saved-lists/", server.handleSavedLists)
	http.HandleFunc("/api/places/autocomplete", server.handlePlacesAutocomplete)
	http.HandleFunc("/api/places/detail", server.handlePlacesDetail)
	http.HandleFunc("/api/settings", server.handleSettings)
	http.HandleFunc("/api/settings/invite", server.handleInvite)
	http.HandleFunc("/api/settings/leave-household", server.handleLeaveHousehold)
	http.HandleFunc("/api/penny.vcf", server.handleVCard)
	http.HandleFunc("/api/join", server.handleJoin)
	http.HandleFunc("/api/email", server.handleEmailWebhook)
	http.HandleFunc("/api/sms", server.handleSMSWebhook)
	http.HandleFunc("/api/referral", server.handleReferral)
	http.HandleFunc("/api/referral/redeem", server.handleReferralRedeem)
	http.HandleFunc("/api/referral/claim", server.handleReferralClaim)
	http.HandleFunc("/api/rebates", server.handleRebates)
	http.HandleFunc("/api/earnings", server.handleEarnings)
	http.HandleFunc("/api/earnings/", server.handleEarnings)
	http.HandleFunc("/api/nps/dismiss", server.handleNPSDismiss)
	http.HandleFunc("/api/nps/check", server.handleNPSCheck)
	http.HandleFunc("/api/nps", server.handleNPS)
	http.HandleFunc("/api/penny/quote", server.handlePennyQuote)
	http.HandleFunc("/api/penny/nudges", server.handleRepurchaseNudges)
	http.HandleFunc("/api/admin/dashboard", server.handleAdminDashboard)
	http.HandleFunc("/api/admin/analytics", server.handleAdminAnalytics)
	http.HandleFunc("/api/admin/deals", server.handleAdminDeals)
	http.HandleFunc("/api/admin/deals/", func(w http.ResponseWriter, r *http.Request) {
		// Route /api/admin/deals/{id}/review to the review handler
		if strings.HasSuffix(r.URL.Path, "/review") {
			server.handleAdminDealReview(w, r)
			return
		}
		server.handleAdminDeals(w, r)
	})
	http.HandleFunc("/api/admin/payouts", server.handleAdminPayouts)
	http.HandleFunc("/api/admin/business-metrics", server.handleBusinessMetrics)
	http.HandleFunc("/api/admin/churn-risk", server.handleChurnRisk)
	http.HandleFunc("/api/admin/cohort-analysis", server.handleCohortAnalysis)
	http.HandleFunc("/api/admin/marketplace-health", server.handleMarketplaceHealth)
	http.HandleFunc("/api/admin/segment-analysis", server.handleSegmentAnalysis)
	http.HandleFunc("/api/admin/category-insights", server.handleCategoryInsights)
	http.HandleFunc("/api/admin/swap-predictions", server.handleSwapPredictions)
	http.HandleFunc("/api/admin/swap-propagations", server.handleSwapPropagations)
	http.HandleFunc("/api/admin/store-stats", server.handleAdminStoreStats)
	http.HandleFunc("/api/admin/category-alerts", server.handleCategoryAlerts)
	http.HandleFunc("/api/admin/backup", server.handleAdminBackup)
	http.HandleFunc("/api/wrapped", server.handleWrapped)
	http.HandleFunc("/api/milestones/check", server.handleMilestoneCheck)
	http.HandleFunc("/api/milestones/ack", server.handleMilestoneAck)
	http.HandleFunc("/api/penny/insights", server.handlePennyInsights)
	http.HandleFunc("/api/preferences", server.handleDietaryPreferences)
	http.HandleFunc("/api/preferences/reset", server.handlePreferenceReset)
	http.HandleFunc("/api/household/dietary", server.handleHouseholdDietary)
	http.HandleFunc("/api/stores", server.handleStores)
	http.HandleFunc("/api/stores/", server.handleStoreSubroute)
	http.HandleFunc("/api/price-compare", server.handlePriceComparison)
	http.HandleFunc("/api/admin/regional-swaps", server.handleRegionalSwapPerformance)
	http.HandleFunc("/api/feedback", server.handleFeedbackRoutes)
	http.HandleFunc("/api/admin/feedback", server.handleAdminFeedback)
	http.HandleFunc("/api/admin/feedback/", server.handleAdminFeedbackByID)

	// --- Watt Data (CPG brand manager outreach) ---
	http.HandleFunc("/api/admin/watt/trait-search", server.handleWattTraitSearch)
	http.HandleFunc("/api/admin/watt/brand-search", server.handleWattBrandSearch)
	http.HandleFunc("/api/admin/watt/resolve", server.handleWattEntityResolve)

	// --- A/B Testing ---
	http.HandleFunc("GET /api/ab/variant", handleGetVariant(server))
	http.HandleFunc("POST /api/ab/event", handleTrackABEvent(server))
	http.HandleFunc("GET /api/admin/ab-tests", handleListABTests(server))
	http.HandleFunc("POST /api/admin/ab-tests", handleCreateABTest(server))
	http.HandleFunc("GET /api/admin/ab-tests/{name}/results", handleABTestResults(server))
	http.HandleFunc("POST /api/admin/ab-tests/{name}/resolve", handleResolveABTest(server))

	// --- Brand Portal (M4) ---
	http.HandleFunc("/brand/register", server.handleBrandRegister)
	http.HandleFunc("/brand/login", server.handleBrandLogin)
	http.HandleFunc("/brand/me", server.handleBrandMe)
	http.HandleFunc("/brand/logout", server.handleBrandLogout)
	http.HandleFunc("/brand/dashboard", server.handleBrandDashboard)
	http.HandleFunc("/brand/deals", server.handleBrandDeals)
	http.HandleFunc("/brand/deals/", server.handleBrandDeals)
	http.HandleFunc("/brand/category-intel/", server.handleCategoryIntel)
	http.HandleFunc("/brand/deal-performance/", server.handleBrandDealPerformance)
	http.HandleFunc("/brand/optimize/", server.handleOptimizeDeal)
	http.HandleFunc("/brand/pricing-tiers", server.handlePricingTiers)
	http.HandleFunc("/brand/transactions", server.handleBrandTransactions)
	http.HandleFunc("/brand/deposit", server.handleBrandDeposit)
	http.HandleFunc("/brand/export", server.handleBrandExport)
	http.HandleFunc("/brand/stripe-webhook", server.handleStripeWebhook)
	http.HandleFunc("/brand/unsubscribe", server.handleBrandUnsubscribe)
	http.HandleFunc("/brand/competitive-intel", server.handleCompetitiveIntel)
	http.HandleFunc("/brand/category-alerts", server.handleCategoryAlerts)
	http.HandleFunc("/brand/category-report/", server.handleCategoryReport)
	http.HandleFunc("/brand/subscription", server.handleBrandSubscription)
	http.HandleFunc("/brand/feedback", server.handleBrandFeedback)
	http.HandleFunc("/brand/logo", server.handleBrandLogoUpload)
	http.HandleFunc("/brand/logo/", server.handleBrandLogoServe)

	http.HandleFunc("/api/avatar", server.handleAvatar)
	http.HandleFunc("/api/avatar/", server.handleAvatar)
	http.HandleFunc("/api/categories", server.handleCategories)
	http.HandleFunc("/api/brands", server.handleBrands)
	http.HandleFunc("/api/logo", server.handleLogoProxy)
	http.HandleFunc("/api/trips", server.handleTrips)
	http.HandleFunc("/api/bug-report", server.handleBugReport)

	// --- Fully Free Trial Campaigns ---
	http.HandleFunc("/api/receipt/verify", server.handleFullyFreeReceiptVerify)
	http.HandleFunc("/api/rebates/pending", server.handleFullyFreeRebates)

	// --- Receipt Reading (M7) ---
	http.HandleFunc("/api/receipts", server.handleReceipts)
	http.HandleFunc("/api/receipts/", server.handleReceiptRouting)
	http.HandleFunc("/api/receipts/claims", server.handleSwapClaims)

	http.HandleFunc("/privacy", handlePrivacyPolicy)
	http.HandleFunc("/terms", handleTerms)

	// --- Embeddable View (for vibeflourishing.com pitch deck) ---
	http.HandleFunc("/embed", func(w http.ResponseWriter, r *http.Request) {
		// Serve the consumer SPA at /embed — the React app handles embed mode
		// Security headers for embed are handled in the outer middleware (conditional X-Frame-Options)
		http.ServeFile(w, r, "static/index.html")
	})
	http.HandleFunc("/embed/", func(w http.ResponseWriter, r *http.Request) {
		// Serve sub-paths under /embed (e.g., /embed/list, /embed/penny)
		http.ServeFile(w, r, "static/index.html")
	})

	// Serve the React frontends. Two separate SPAs:
	// - Consumer app (static/) at app.groflo.io
	// - Brand portal (static-brand/) at brands.groflo.io
	// Known files are served directly, everything else gets index.html for SPA routing.
	// /r/{code} referral links and /join/:code inject dynamic OG tags for social previews.
	consumerFS := http.FileServer(http.Dir("static"))
	brandFS := http.FileServer(http.Dir("static-brand"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Return 404 JSON for unknown /api/ paths (don't fall through to SPA)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeNotFound(w, "Not found")
			return
		}

		// penny.rallytasks.com serves the consumer app directly (no redirect)
		// app.groflo.io continues to work as-is

		// Brand portal: serve from static-brand/ for brand(s).* subdomain
		host := r.Host
		if strings.HasPrefix(host, "brand.") || strings.HasPrefix(host, "brands.") || strings.HasPrefix(host, "brands-") || strings.HasPrefix(host, "brand-") {
			// Skip /brand/ API routes (handled by registered handlers)
			if strings.HasPrefix(r.URL.Path, "/brand/") {
				writeNotFound(w, "Not found")
				return
			}
			cleanPath := path.Clean(r.URL.Path)
			if _, err := os.Stat("static-brand" + cleanPath); err == nil && !strings.Contains(cleanPath, "..") {
				if strings.HasPrefix(cleanPath, "/assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				brandFS.ServeHTTP(w, r)
				return
			}
			http.ServeFile(w, r, "static-brand/index.html")
			return
		}

		// Consumer app: serve from static/
		cleanPath := path.Clean(r.URL.Path)
		if _, err := os.Stat("static" + cleanPath); err == nil && !strings.Contains(cleanPath, "..") {
			// Hashed assets (Vite fingerprinted filenames) can be cached aggressively
			if strings.HasPrefix(cleanPath, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else if cleanPath == "/sw.js" {
				// Service worker must not be cached — browser checks for updates on each page load
				w.Header().Set("Cache-Control", "no-cache")
			}
			consumerFS.ServeHTTP(w, r)
			return
		}
		// B2: Inject dynamic OG tags for referral and invite links
		if strings.HasPrefix(r.URL.Path, "/r/") || strings.HasPrefix(r.URL.Path, "/join/") || strings.HasPrefix(r.URL.Path, "/shared/") {
			serveWithOGTags(w, r)
			return
		}
		http.ServeFile(w, r, "static/index.html")
	})

	// Wrap with panic recovery and request/response logging
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				log.Printf("[panic] %s %s: %v", r.Method, r.URL.Path, rv)
				sentry.CurrentHub().Recover(rv)
				sentry.Flush(2 * time.Second)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()

		// 301 redirect: legacy thrift.ly hosts → groflo.io. Catches existing
		// SMS share links, email signatures, bookmarks, and search results
		// that point at the old domain. Required for ≥12 months per panel R5
		// (household continuity). Strip the port if present (Railway adds one).
		host := r.Host
		if i := strings.Index(host, ":"); i >= 0 {
			host = host[:i]
		}
		var redirectHost string
		switch host {
		case "thrift.ly", "www.thrift.ly":
			redirectHost = "groflo.io"
		case "app.thrift.ly":
			redirectHost = "app.groflo.io"
		case "brand.thrift.ly", "brands.thrift.ly":
			redirectHost = "brands.groflo.io"
		case "deck.thrift.ly":
			redirectHost = "deck.groflo.io"
		}
		if redirectHost != "" {
			target := "https://" + redirectHost + r.URL.RequestURI()
			log.Printf("[redirect] %s%s → %s", r.Host, r.URL.RequestURI(), target)
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}

		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Embed routes: allow framing by approved origins; all other routes: DENY
		if strings.HasPrefix(r.URL.Path, "/embed") {
			w.Header().Set("Content-Security-Policy", cspHeader(r.URL.Path))
			// Dynamic CORS origin for approved embed hosts
			origin := r.Header.Get("Origin")
			allowedOrigins := map[string]bool{"https://vibeflourishing.com": true, "https://deck.groflo.io": true, "https://deck.thrift.ly": true}
			if allowedOrigins[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "https://vibeflourishing.com")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			// Handle CORS preflight
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		} else {
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Content-Security-Policy", cspHeader(r.URL.Path))
		}
		if server.config.SecureCookies {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		log.Printf("[req] %s %s (Content-Length: %d)", r.Method, r.URL.Path, r.ContentLength)
		rec := &statusRecorder{ResponseWriter: w, statusCode: 200}
		http.DefaultServeMux.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("[res] %s %s → %d", r.Method, r.URL.Path, rec.statusCode)
		}
	})

	go server.warmLogoCache()

	// Prune stale rate-limiter entries every 5 minutes
	go func() {
		for range time.Tick(5 * time.Minute) {
			server.otpLimiter.Range(func(k, v any) bool {
				if time.Since(v.(time.Time)) > 2*time.Minute {
					server.otpLimiter.Delete(k)
				}
				return true
			})
			pruneTimeLimiter(&server.otpIPLimiter, 10*time.Minute)
			pruneTimeLimiter(&server.emailLoginIPLimiter, 10*time.Minute)
			pruneTimeLimiter(&server.passwordResetIPLimiter, 15*time.Minute)
			pruneTimeLimiter(&server.clerkExchangeIPLimiter, clerkExchangeWindow)
			pruneBucketLimiter(&server.parseLimiter, time.Hour)
			pruneBucketLimiter(&server.earningsUploadLimiter, time.Hour)
			pruneBucketLimiter(&server.npsSubmitLimiter, time.Hour)
		}
	}()

	// Weekly brand email digest (checks hourly, sends every 7 days per brand)
	go server.runBrandDigestLoop()

	// Learning loop: derives insights from raw signals every 6 hours
	go server.runLearningLoop()

	// Daily metrics aggregation: deal performance, category insights, business metrics
	go server.runDailyMetricsLoop()

	// F8 Phase 2: Daily digest SMS for opted-in users (checks hourly, 20h dedup window)
	go server.runDailyDigestLoop()

	// Keep-alive: self-ping every 4 minutes to prevent Railway from sleeping the container
	go func() {
		time.Sleep(30 * time.Second) // Wait for server to be ready
		for {
			time.Sleep(4 * time.Minute)
			resp, err := http.Get("http://localhost:" + cfg.Port + "/health")
			if err != nil {
				log.Printf("[keepalive] self-ping failed: %v", err)
			} else {
				resp.Body.Close()
				if resp.StatusCode != 200 {
					log.Printf("[keepalive] self-ping returned %d", resp.StatusCode)
				}
			}
		}
	}()

	log.Printf("Groflo is running on http://localhost:%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}
