package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	_ "modernc.org/sqlite"
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
	npsMinLogins             = 3
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
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "thriftly.db"
	}
	npsInterval := 90
	if v := os.Getenv("NPS_INTERVAL_DAYS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			npsInterval = parsed
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
		DBPath:              dbPath,
		WebhookBaseURL:      os.Getenv("WEBHOOK_BASE_URL"), // e.g. "https://app.thrift.ly"
		SecureCookies:       os.Getenv("RAILWAY_ENVIRONMENT_NAME") != "",
		TestMode:            os.Getenv("TEST_MODE") == "true",
		GoogleClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		SendGridAPIKey:      os.Getenv("SENDGRID_API_KEY"),
		BlueCartAPIKey:      os.Getenv("BLUECART_API_KEY"),
		ReportingDBPath:     os.Getenv("REPORTING_DB_PATH"),
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

func main() {
	cfg := loadConfig()

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

	// Open the database (creates the file if it doesn't exist)
	db, err := initDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()

	// Open reporting DB clone if it exists (for admin analytics isolation)
	var reportDB *sql.DB
	if cfg.ReportingDBPath != "" {
		if _, err := os.Stat(cfg.ReportingDBPath); err == nil {
			rdb, err := openReportDB(cfg.ReportingDBPath)
			if err != nil {
				log.Printf("[startup] failed to open reporting DB at %s: %v", cfg.ReportingDBPath, err)
			} else {
				reportDB = rdb
				log.Printf("[startup] reporting DB opened at %s", cfg.ReportingDBPath)
			}
		}
	}

	server := &Server{
		config:   cfg,
		db:       db,
		reportDB: reportDB,
		sessions: NewSessionStore(db),
		twilio: &RealTwilioClient{
			AccountSID:    cfg.TwilioAccountSID,
			AuthToken:     cfg.TwilioAuthToken,
			VerifyService: cfg.TwilioVerifyService,
		},
		parseItems:   parseItemsWithClaude,
		parseCommand: parseCommandWithClaude,
		parseReceipt: parseReceiptWithClaude,
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
		w.Write([]byte(`{"status":"ok"}`))
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
	http.HandleFunc("/auth/logout", server.handleLogout)
	http.HandleFunc("/auth/me", server.handleMe)
	http.HandleFunc("/api/auth/link", server.handleAuthLink)
	http.HandleFunc("/api/config", server.handleAPIConfig)
	http.HandleFunc("/api/list", server.handleList)
	http.HandleFunc("/api/list/", server.handleList)
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
	http.HandleFunc("/api/nps/check", server.handleNPSCheck)
	http.HandleFunc("/api/nps", server.handleNPS)
	http.HandleFunc("/api/penny/quote", server.handlePennyQuote)
	http.HandleFunc("/api/penny/nudges", server.handleRepurchaseNudges)
	http.HandleFunc("/api/admin/dashboard", server.handleAdminDashboard)
	http.HandleFunc("/api/admin/analytics", server.handleAdminAnalytics)
	http.HandleFunc("/api/admin/deals", server.handleAdminDeals)
	http.HandleFunc("/api/admin/deals/", server.handleAdminDeals)
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
	http.HandleFunc("/api/stores/", server.handleStorePrices)

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

	http.HandleFunc("/api/avatar", server.handleAvatar)
	http.HandleFunc("/api/avatar/", server.handleAvatar)
	http.HandleFunc("/api/categories", server.handleCategories)
	http.HandleFunc("/api/brands", server.handleBrands)
	http.HandleFunc("/api/logo", server.handleLogoProxy)
	http.HandleFunc("/api/trips", server.handleTrips)
	http.HandleFunc("/api/bug-report", server.handleBugReport)

	// --- Receipt Reading (M7) ---
	http.HandleFunc("/api/receipts", server.handleReceipts)
	http.HandleFunc("/api/receipts/", server.handleReceiptRouting)
	http.HandleFunc("/api/receipts/claims", server.handleSwapClaims)

	http.HandleFunc("/privacy", handlePrivacyPolicy)
	http.HandleFunc("/terms", handleTerms)

	// Serve the React frontends. Two separate SPAs:
	// - Consumer app (static/) at app.thrift.ly
	// - Brand portal (static-brand/) at brands.thrift.ly
	// Known files are served directly, everything else gets index.html for SPA routing.
	// /r/{code} referral links and /join/:code inject dynamic OG tags for social previews.
	consumerFS := http.FileServer(http.Dir("static"))
	brandFS := http.FileServer(http.Dir("static-brand"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Return 404 JSON for unknown /api/ paths (don't fall through to SPA)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
			return
		}

		// Brand portal: serve from static-brand/ for brands.* subdomain
		host := r.Host
		if strings.HasPrefix(host, "brands.") || strings.HasPrefix(host, "brands-") {
			// Skip /brand/ API routes (handled by registered handlers)
			if strings.HasPrefix(r.URL.Path, "/brand/") {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
				return
			}
			cleanPath := path.Clean(r.URL.Path)
			if _, err := os.Stat("static-brand" + cleanPath); err == nil && !strings.Contains(cleanPath, "..") {
				brandFS.ServeHTTP(w, r)
				return
			}
			http.ServeFile(w, r, "static-brand/index.html")
			return
		}

		// Consumer app: serve from static/
		cleanPath := path.Clean(r.URL.Path)
		if _, err := os.Stat("static" + cleanPath); err == nil && !strings.Contains(cleanPath, "..") {
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
		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
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
			pruneBucketLimiter(&server.parseLimiter, time.Hour)
			pruneBucketLimiter(&server.earningsUploadLimiter, time.Hour)
		}
	}()

	// Weekly brand email digest (checks hourly, sends every 7 days per brand)
	go server.runBrandDigestLoop()

	// Learning loop: derives insights from raw signals every 6 hours
	go server.runLearningLoop()

	// Daily metrics aggregation: deal performance, category insights, business metrics
	go server.runDailyMetricsLoop()

	log.Printf("thriftly is running on http://localhost:%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}
