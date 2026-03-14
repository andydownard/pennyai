package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"net/mail"
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
	BrandUserID int
	CreatedAt   time.Time
}

func (s *Server) createBrandSession(brandUserID int) (string, error) {
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

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
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

	brandUserID, _ := result.LastInsertId()

	// Initialize balance
	s.db.Exec("INSERT INTO brand_balance (brand_user_id, balance_cents) VALUES (?, 0)", brandUserID)

	// Create session
	token, err := s.createBrandSession(int(brandUserID))
	if err != nil {
		dbError(w, err, "Failed to create session")
		return
	}

	setBrandSessionCookie(w, s.config, token)
	log.Printf("[brand-auth] brand user %d registered (%s, %s)", brandUserID, req.Email, req.CompanyName)

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

	var brandUserID int
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
	log.Printf("[brand-auth] brand user %d logged in (%s)", brandUserID, req.Email)

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

	var email, companyName, badge string
	var verified int
	var createdAt string
	err := s.db.QueryRow(
		"SELECT email, company_name, verified, badge, created_at FROM brand_users WHERE id = ?",
		bs.BrandUserID,
	).Scan(&email, &companyName, &verified, &badge, &createdAt)
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
