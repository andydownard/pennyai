package main

import (
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

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

	server := &Server{
		config:   cfg,
		db:       db,
		sessions: NewSessionStore(db),
		twilio: &RealTwilioClient{
			AccountSID:    cfg.TwilioAccountSID,
			AuthToken:     cfg.TwilioAuthToken,
			VerifyService: cfg.TwilioVerifyService,
		},
		parseItems:   parseItemsWithClaude,
		parseCommand: parseCommandWithClaude,
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
	http.HandleFunc("/api/config", server.handleAPIConfig)
	http.HandleFunc("/api/list", server.handleList)
	http.HandleFunc("/api/list/", server.handleList)
	http.HandleFunc("/api/list/clear", server.handleListClear)
	http.HandleFunc("/api/list/restore", server.handleListRestore)
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
	http.HandleFunc("/api/nps/check", server.handleNPSCheck)
	http.HandleFunc("/api/nps", server.handleNPS)
	http.HandleFunc("/api/penny/quote", server.handlePennyQuote)
	http.HandleFunc("/api/admin/dashboard", server.handleAdminDashboard)
	http.HandleFunc("/api/admin/analytics", server.handleAdminAnalytics)
	http.HandleFunc("/api/brands", server.handleBrands)
	http.HandleFunc("/api/logo", server.handleLogoProxy)
	http.HandleFunc("/api/trips", server.handleTrips)
	http.HandleFunc("/privacy", handlePrivacyPolicy)
	http.HandleFunc("/terms", handleTerms)

	// Serve the React frontend. The custom handler supports SPA routing:
	// known files are served directly, everything else gets index.html.
	// /r/{code} referral links and /join/:code inject dynamic OG tags for social previews.
	fs := http.FileServer(http.Dir("static"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Return 404 JSON for unknown /api/ paths (don't fall through to SPA)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
			return
		}
		// Use path.Clean to prevent path traversal
		cleanPath := path.Clean(r.URL.Path)
		if _, err := os.Stat("static" + cleanPath); err == nil && !strings.Contains(cleanPath, "..") {
			fs.ServeHTTP(w, r)
			return
		}
		// B2: Inject dynamic OG tags for referral and invite links
		if strings.HasPrefix(r.URL.Path, "/r/") || strings.HasPrefix(r.URL.Path, "/join/") {
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
			server.otpIPLimiter.Range(func(k, v any) bool {
				times := v.([]time.Time)
				recent := 0
				for _, t := range times {
					if time.Since(t) < 10*time.Minute {
						recent++
					}
				}
				if recent == 0 {
					server.otpIPLimiter.Delete(k)
				}
				return true
			})
			server.emailLoginIPLimiter.Range(func(k, v any) bool {
				times := v.([]time.Time)
				recent := 0
				for _, t := range times {
					if time.Since(t) < 10*time.Minute {
						recent++
					}
				}
				if recent == 0 {
					server.emailLoginIPLimiter.Delete(k)
				}
				return true
			})
			server.parseLimiter.Range(func(k, v any) bool {
				bucket := v.(*rateBucket)
				bucket.mu.Lock()
				recent := 0
				for _, t := range bucket.times {
					if time.Since(t) < time.Hour {
						recent++
					}
				}
				bucket.mu.Unlock()
				if recent == 0 {
					server.parseLimiter.Delete(k)
				}
				return true
			})
		}
	}()

	log.Printf("thriftly is running on http://localhost:%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}
