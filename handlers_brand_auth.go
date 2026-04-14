package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// brandLoginIPLimiter rate-limits brand login attempts per IP.
var brandLoginIPLimiter sync.Map

// =============================================================================
// BRAND AUTH HELPERS
// =============================================================================

// BrandSession represents an authenticated brand portal session.
type BrandSession struct {
	BrandUserID string
	CreatedAt   time.Time
}

func (s *Server) createBrandSession(brandUserID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	_, err := s.db.Exec(
		"INSERT INTO brand_sessions (token, brand_user_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
		token, brandUserID, time.Now().UTC(), time.Now().UTC().Add(sessionDuration),
	)
	return token, err
}

func (s *Server) getBrandSession(token string) (BrandSession, bool) {
	var bs BrandSession
	err := s.db.QueryRow(
		"SELECT brand_user_id, created_at FROM brand_sessions WHERE token = ? AND expires_at > ?",
		token, time.Now().UTC(),
	).Scan(&bs.BrandUserID, &bs.CreatedAt)
	if err != nil {
		return BrandSession{}, false
	}
	return bs, true
}

func setBrandSessionCookie(w http.ResponseWriter, cfg Config, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "brand_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 30,
	})
}

func (s *Server) requireBrandSession(w http.ResponseWriter, r *http.Request) (BrandSession, bool) {
	cookie, err := r.Cookie("brand_session")
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not logged in"})
		return BrandSession{}, false
	}
	bs, ok := s.getBrandSession(cookie.Value)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Session expired"})
		return BrandSession{}, false
	}
	return bs, true
}

// =============================================================================
// BRAND REGISTRATION
// =============================================================================

func (s *Server) handleBrandRegister(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		CompanyName string `json:"company_name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid email address"})
		return
	}
	if len(req.Password) < 8 || len(req.Password) > 72 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Password must be 8-72 characters"})
		return
	}
	req.CompanyName = strings.TrimSpace(req.CompanyName)
	if req.CompanyName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Company name is required"})
		return
	}

	// Check if email already registered
	var exists int
	s.db.QueryRow("SELECT COUNT(*) FROM brand_users WHERE email = ?", req.Email).Scan(&exists)
	if exists > 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "An account with this email already exists"})
		return
	}

	cost := bcrypt.DefaultCost
	if s.config.BcryptCost > 0 {
		cost = s.config.BcryptCost
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), cost)
	if err != nil {
		dbError(w, err, "Failed to create account")
		return
	}

	// Generate verify token
	vb := make([]byte, 16)
	rand.Read(vb)
	verifyToken := hex.EncodeToString(vb)

	// Check if this is a "Founding Brand" (first 10)
	var brandCount int
	s.db.QueryRow("SELECT COUNT(*) FROM brand_users").Scan(&brandCount)
	badge := ""
	if brandCount < 10 {
		badge = "founding"
	}

	result, err := s.db.Exec(
		`INSERT INTO brand_users (email, password_hash, company_name, verified, verify_token, badge)
		 VALUES (?, ?, ?, 1, ?, ?)`,
		req.Email, string(hash), req.CompanyName, verifyToken, badge,
	)
	if err != nil {
		dbError(w, err, "Failed to create account")
		return
	}

	brandUserIDInt, _ := result.LastInsertId()
	brandUserID := strconv.FormatInt(brandUserIDInt, 10)

	// Initialize balance
	s.db.Exec("INSERT INTO brand_balance (brand_user_id, balance_cents) VALUES (?, 0)", brandUserID)

	// Create session
	token, err := s.createBrandSession(brandUserID)
	if err != nil {
		dbError(w, err, "Failed to create session")
		return
	}

	setBrandSessionCookie(w, s.config, token)
	log.Printf("[brand-auth] brand user %s registered (%s, %s)", brandUserID, req.Email, req.CompanyName)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"brand_user_id": brandUserID,
		"badge":         badge,
	})
}

// =============================================================================
// BRAND LOGIN
// =============================================================================

func (s *Server) handleBrandLogin(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	clientIP := getClientIP(r)
	if checkRateLimit(&brandLoginIPLimiter, clientIP, 10*time.Minute, 10) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many login attempts"})
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	var brandUserID string
	var passwordHash string
	err := s.db.QueryRow(
		"SELECT id, password_hash FROM brand_users WHERE email = ?",
		req.Email,
	).Scan(&brandUserID, &passwordHash)

	if err != nil {
		bcrypt.CompareHashAndPassword([]byte("$2a$10$dummyhashfortimingattttttttttttttttttttttttttttttttttttt"), []byte(req.Password))
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid email or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid email or password"})
		return
	}

	token, err := s.createBrandSession(brandUserID)
	if err != nil {
		dbError(w, err, "Failed to create session")
		return
	}

	setBrandSessionCookie(w, s.config, token)
	log.Printf("[brand-auth] brand user %s logged in (%s)", brandUserID, req.Email)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"brand_user_id": brandUserID,
	})
}

// =============================================================================
// BRAND ME / LOGOUT
// =============================================================================

func (s *Server) handleBrandMe(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	var email, companyName, badge, logoData string
	var verified int
	var createdAt string
	err := s.db.QueryRow(
		"SELECT email, company_name, verified, badge, created_at, COALESCE(logo_data,'') FROM brand_users WHERE id = ?",
		bs.BrandUserID,
	).Scan(&email, &companyName, &verified, &badge, &createdAt, &logoData)
	if err != nil {
		dbError(w, err, "Failed to load profile")
		return
	}

	var balanceCents int
	s.db.QueryRow("SELECT balance_cents FROM brand_balance WHERE brand_user_id = ?", bs.BrandUserID).Scan(&balanceCents)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"brand_user_id":       bs.BrandUserID,
		"email":               email,
		"company_name":        companyName,
		"verified":            verified == 1,
		"badge":               badge,
		"balance_cents":       balanceCents,
		"created_at":          createdAt,
		"has_logo":            logoData != "",
		"can_submit_feedback": true, // all brand users can submit feedback
	})
}

func (s *Server) handleBrandLogout(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	cookie, err := r.Cookie("brand_session")
	if err == nil {
		s.db.Exec("DELETE FROM brand_sessions WHERE token = ?", cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "brand_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"success": "true"})
}

// handleBrandLogoUpload handles POST /brand/logo — upload a brand logo (base64 image).
func (s *Server) handleBrandLogoUpload(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Image string `json:"image"` // data:image/png;base64,... or raw base64
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Image == "" {
		writeBadRequest(w, "Missing image data")
		return
	}

	// Parse data URI or raw base64
	mime := "image/png"
	data := req.Image
	if strings.HasPrefix(data, "data:") {
		// data:image/png;base64,iVBOR...
		parts := strings.SplitN(data, ",", 2)
		if len(parts) != 2 {
			writeBadRequest(w, "Invalid image data")
			return
		}
		header := parts[0] // data:image/png;base64
		data = parts[1]
		if idx := strings.Index(header, ":"); idx >= 0 {
			mimeEnd := strings.Index(header[idx+1:], ";")
			if mimeEnd > 0 {
				mime = header[idx+1 : idx+1+mimeEnd]
			}
		}
	}

	// Validate it's actually base64 and not too large (max 500KB decoded)
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		writeBadRequest(w, "Invalid base64 image data")
		return
	}
	if len(decoded) > 500*1024 {
		writeBadRequest(w, "Image too large (max 500KB)")
		return
	}

	// Only allow image types
	if !strings.HasPrefix(mime, "image/") {
		writeBadRequest(w, "Only image files are allowed")
		return
	}

	_, err = s.db.Exec("UPDATE brand_users SET logo_data = ?, logo_mime = ? WHERE id = ?", data, mime, bs.BrandUserID)
	if err != nil {
		dbError(w, err, "Failed to save logo")
		return
	}

	log.Printf("[brand] logo uploaded for brand_user %s (%d bytes, %s)", bs.BrandUserID, len(decoded), mime)
	writeJSON(w, http.StatusOK, map[string]string{"success": "true"})
}

// handleBrandLogoServe handles GET /brand/logo/ — serve a brand's logo image.
// Public endpoint (no auth required) so logos can be displayed in consumer-facing contexts.
func (s *Server) handleBrandLogoServe(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	// Extract brand_user_id from path: /brand/logo/123
	brandUserID := strings.TrimPrefix(r.URL.Path, "/brand/logo/")
	if brandUserID == "" {
		http.NotFound(w, r)
		return
	}

	var logoData, logoMime string
	err := s.db.QueryRow("SELECT COALESCE(logo_data,''), COALESCE(logo_mime,'') FROM brand_users WHERE id = ?", brandUserID).Scan(&logoData, &logoMime)
	if err != nil || logoData == "" {
		http.NotFound(w, r)
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(logoData)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", logoMime)
	w.Header().Set("Cache-Control", "public, max-age=86400") // cache 24h
	w.Header().Set("Content-Length", strconv.Itoa(len(decoded)))
	w.Write(decoded)
}
