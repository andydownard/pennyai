package main

import (
	"encoding/json"
	"html"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func setSessionCookie(w http.ResponseWriter, cfg Config, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 30, // 30 days
	})
}

// getClientIP extracts the client IP from the request.
// Only trusts X-Forwarded-For when running behind a reverse proxy (Railway),
// detected by the presence of RAILWAY_ENVIRONMENT_NAME. This prevents attackers
// from spoofing their IP to bypass rate limiting.
func getClientIP(r *http.Request) string {
	// Only trust X-Forwarded-For when behind Railway's proxy
	if os.Getenv("RAILWAY_ENVIRONMENT_NAME") != "" {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Use the first (leftmost) IP — the client IP added by Railway's proxy
			clientIP := strings.TrimSpace(strings.Split(xff, ",")[0])
			if clientIP != "" {
				return clientIP
			}
		}
	}
	// Fallback: use RemoteAddr directly (strip port)
	clientIP := r.RemoteAddr
	if idx := strings.LastIndex(clientIP, ":"); idx > 0 && !strings.Contains(clientIP[idx:], "]") {
		clientIP = clientIP[:idx]
	}
	return strings.TrimSpace(clientIP)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[warn] failed to encode JSON response: %v", err)
	}
}

func writeBadRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
}

func writeConflict(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusConflict, map[string]string{"error": msg})
}

// pluralS returns "s" when count != 1, for simple English pluralization.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func writeNotFound(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": msg})
}

// requireMethod checks that the request uses the expected HTTP method.
// Returns true if the method matches; otherwise writes 405 and returns false.
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return false
	}
	return true
}

// dbError logs an error and writes a JSON 500 response. Replaces 15+ identical blocks.
func dbError(w http.ResponseWriter, err error, msg string) {
	log.Printf("%s: %v", msg, err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": msg})
}

// decodeJSON reads a JSON request body into v. Returns false (and writes 400) on failure.
func decodeJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "Content-Type must be application/json"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1*1024*1024) // 1 MB limit for JSON endpoints
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		msg := "Invalid request body"
		if err.Error() == "http: request body too large" {
			msg = "Request too large"
		}
		writeBadRequest(w, msg)
		return false
	}
	return true
}

// checkRateLimit checks a sync.Map-based sliding window rate limiter.
// Values are stored as []time.Time. Returns true if the request should be blocked.
func checkRateLimit(store *sync.Map, key string, window time.Duration, maxCount int) bool {
	now := time.Now()
	if raw, ok := store.Load(key); ok {
		times := raw.([]time.Time)
		recent := make([]time.Time, 0, len(times))
		for _, t := range times {
			if now.Sub(t) < window {
				recent = append(recent, t)
			}
		}
		if len(recent) >= maxCount {
			store.Store(key, recent)
			return true
		}
		store.Store(key, append(recent, now))
	} else {
		store.Store(key, []time.Time{now})
	}
	return false
}

// checkRateLimitBucket checks a rateBucket-based rate limiter with mutex.
// Returns true if the request should be blocked.
func checkRateLimitBucket(store *sync.Map, key any, window time.Duration, maxCount int) bool {
	now := time.Now()
	val, _ := store.LoadOrStore(key, &rateBucket{})
	bucket := val.(*rateBucket)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	recent := make([]time.Time, 0, len(bucket.times))
	for _, t := range bucket.times {
		if now.Sub(t) < window {
			recent = append(recent, t)
		}
	}
	if len(recent) >= maxCount {
		bucket.times = recent
		return true
	}
	bucket.times = append(recent, now)
	return false
}

// pruneTimeLimiter removes stale entries from a sync.Map-based []time.Time rate limiter.
func pruneTimeLimiter(store *sync.Map, maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	store.Range(func(key, val any) bool {
		times := val.([]time.Time)
		recent := 0
		for _, t := range times {
			if t.After(cutoff) {
				recent++
			}
		}
		if recent == 0 {
			store.Delete(key)
		}
		return true
	})
}

// pruneBucketLimiter removes stale entries from a sync.Map-based *rateBucket rate limiter.
func pruneBucketLimiter(store *sync.Map, maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	store.Range(func(key, val any) bool {
		rb := val.(*rateBucket)
		rb.mu.Lock()
		recent := 0
		for _, t := range rb.times {
			if t.After(cutoff) {
				recent++
			}
		}
		rb.mu.Unlock()
		if recent == 0 {
			store.Delete(key)
		}
		return true
	})
}

// serveWithOGTags serves index.html with dynamic Open Graph meta tags injected
// for referral (/r/:code) and invite (/join/:code) links. This gives rich
// previews when links are shared in iMessage, WhatsApp, Slack, etc.
func serveWithOGTags(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	page := string(data)

	var ogTitle, ogDescription string
	if strings.HasPrefix(r.URL.Path, "/r/") {
		ogTitle = "Join me on Groflo!"
		ogDescription = "I'm using Penny, an AI shopping assistant that helps me save on groceries. Join my household and we can share lists and save together."
	} else {
		ogTitle = "You're invited to a Groflo household!"
		ogDescription = "Someone wants to share their grocery list with you. Join their household on Groflo and let Penny help you both save."
	}

	// Replace the default OG tags with dynamic ones (HTML-escaped to prevent injection)
	page = strings.Replace(page,
		`<meta property="og:title" content="Groflo — Your AI Shopping Assistant" />`,
		`<meta property="og:title" content="`+html.EscapeString(ogTitle)+`" />`, 1)
	page = strings.Replace(page,
		`<meta property="og:description" content="Penny helps your household save on groceries with smart suggestions and shared lists." />`,
		`<meta property="og:description" content="`+html.EscapeString(ogDescription)+`" />`, 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(page))
}

// requireSavedListAccess checks that listID exists and belongs to a member of
// the caller's household. Returns false (and writes an error response) on failure.
func (s *Server) requireSavedListAccess(w http.ResponseWriter, userID string, listID int) bool {
	var ownerID string
	err := s.db.QueryRow("SELECT user_id FROM saved_lists WHERE id = ?", listID).Scan(&ownerID)
	if err != nil {
		writeNotFound(w, "Saved list not found")
		return false
	}
	householdIDs, _ := getHouseholdUserIDs(s.db, userID)
	for _, uid := range householdIDs {
		if uid == ownerID {
			return true
		}
	}
	writeNotFound(w, "Saved list not found")
	return false
}


// jsonArrayText makes s safe for a JSONB array column (why_bullets,
// key_benefits). PostgreSQL rejects an empty or non-JSON string where SQLite
// stored it as text. A JSON array passes through, blank becomes "[]", and any
// other text is kept as a one-element array so nothing a brand typed is lost.
func jsonArrayText(s string) string {
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(s), &arr); err == nil && arr != nil {
		return s
	}
	if strings.TrimSpace(s) == "" {
		return "[]"
	}
	wrapped, _ := json.Marshal([]string{s})
	return string(wrapped)
}
