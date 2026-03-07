package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
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

// getClientIP extracts the client IP from the request, handling X-Forwarded-For.
func getClientIP(r *http.Request) string {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	if idx := strings.LastIndex(clientIP, ":"); idx > 0 && !strings.Contains(clientIP[idx:], "]") {
		clientIP = clientIP[:idx]
	}
	return strings.TrimSpace(strings.Split(clientIP, ",")[0])
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[warn] failed to encode JSON response: %v", err)
	}
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return false
	}
	return true
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
	html := string(data)

	var ogTitle, ogDescription string
	if strings.HasPrefix(r.URL.Path, "/r/") {
		ogTitle = "Join me on Thriftly!"
		ogDescription = "I'm using Penny, an AI shopping assistant that helps me save on groceries. Join my household and we can share lists and save together."
	} else {
		ogTitle = "You're invited to a Thriftly household!"
		ogDescription = "Someone wants to share their grocery list with you. Join their household on Thriftly and let Penny help you both save."
	}

	// Replace the default OG tags with dynamic ones
	html = strings.Replace(html,
		`<meta property="og:title" content="Thriftly — Your AI Shopping Assistant" />`,
		`<meta property="og:title" content="`+ogTitle+`" />`, 1)
	html = strings.Replace(html,
		`<meta property="og:description" content="Penny helps your household save on groceries with smart suggestions and shared lists." />`,
		`<meta property="og:description" content="`+ogDescription+`" />`, 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// requireSavedListAccess checks that listID exists and belongs to a member of
// the caller's household. Returns false (and writes an error response) on failure.
func (s *Server) requireSavedListAccess(w http.ResponseWriter, userID, listID int) bool {
	var ownerID int
	err := s.db.QueryRow("SELECT user_id FROM saved_lists WHERE id = ?", listID).Scan(&ownerID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Saved list not found"})
		return false
	}
	householdIDs, _ := getHouseholdUserIDs(s.db, userID)
	for _, uid := range householdIDs {
		if uid == ownerID {
			return true
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "Saved list not found"})
	return false
}

