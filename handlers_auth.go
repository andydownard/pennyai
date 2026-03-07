package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) handleGoogleAuth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Credential string `json:"credential"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Credential == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing credential"})
		return
	}

	// Verify Google ID token via tokeninfo endpoint
	resp, err := http.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(req.Credential))
	if err != nil {
		log.Printf("[google-auth] token verification request failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to verify Google token"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Printf("[google-auth] token invalid: %s", string(body))
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid Google token"})
		return
	}

	var tokenInfo struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Name          string `json:"name"`
		Aud           string `json:"aud"`
	}
	if err := json.Unmarshal(body, &tokenInfo); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to parse token info"})
		return
	}

	// Verify audience matches our client ID
	if tokenInfo.Aud != s.config.GoogleClientID {
		log.Printf("[google-auth] audience mismatch: got %s, expected %s", tokenInfo.Aud, s.config.GoogleClientID)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid token audience"})
		return
	}

	if tokenInfo.EmailVerified != "true" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Email not verified"})
		return
	}

	user, err := resolveOrCreateUser(s.db, "google", tokenInfo.Sub, tokenInfo.Email, tokenInfo.Name)
	if err != nil {
		dbError(w, err, "Failed to create account")
		return
	}

	// Update email on user record if not set
	if tokenInfo.Email != "" {
		s.db.Exec("UPDATE users SET email = ? WHERE id = ? AND (email IS NULL OR email = '')", tokenInfo.Email, user.ID)
	}

	token, err := s.sessions.CreateSession(user.ID, user.PhoneNumber, "google")
	if err != nil {
		dbError(w, err, "Failed to create session")
		return
	}

	setSessionCookie(w, s.config, token)
	log.Printf("[google-auth] user %d logged in via Google (%s)", user.ID, tokenInfo.Email)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"user_id": user.ID,
	})
}

// handleEmailRegister handles POST /auth/email/register
func (s *Server) handleEmailRegister(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
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

	// Check if email auth method already exists
	var exists int
	s.db.QueryRow("SELECT COUNT(*) FROM auth_methods WHERE provider = 'email' AND provider_id = ?", req.Email).Scan(&exists)
	if exists > 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "An account with this email already exists. Try signing in."})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		dbError(w, err, "Failed to create account")
		return
	}

	user, err := resolveOrCreateUser(s.db, "email", req.Email, req.Email, strings.TrimSpace(req.Name))
	if err != nil {
		dbError(w, err, "Failed to create account")
		return
	}

	// Set password hash and email
	s.db.Exec("UPDATE auth_methods SET password_hash = ? WHERE user_id = ? AND provider = 'email' AND provider_id = ?", string(hash), user.ID, req.Email)
	s.db.Exec("UPDATE users SET email = ? WHERE id = ? AND (email IS NULL OR email = '')", req.Email, user.ID)

	token, err := s.sessions.CreateSession(user.ID, user.PhoneNumber, "email")
	if err != nil {
		dbError(w, err, "Failed to create session")
		return
	}

	setSessionCookie(w, s.config, token)
	log.Printf("[email-auth] user %d registered via email (%s)", user.ID, req.Email)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"user_id": user.ID,
	})
}

// handleEmailLogin handles POST /auth/email/login
func (s *Server) handleEmailLogin(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	// IP rate limit: max 10 per IP per 10 minutes
	clientIP := getClientIP(r)
	now := time.Now()
	window := 10 * time.Minute
	if raw, ok := s.emailLoginIPLimiter.Load(clientIP); ok {
		times := raw.([]time.Time)
		recent := make([]time.Time, 0, len(times))
		for _, t := range times {
			if now.Sub(t) < window {
				recent = append(recent, t)
			}
		}
		if len(recent) >= 10 {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many login attempts. Please try again later."})
			return
		}
		s.emailLoginIPLimiter.Store(clientIP, append(recent, now))
	} else {
		s.emailLoginIPLimiter.Store(clientIP, []time.Time{now})
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	// Look up auth method
	var userID int
	var passwordHash sql.NullString
	err := s.db.QueryRow(
		"SELECT user_id, password_hash FROM auth_methods WHERE provider = 'email' AND provider_id = ?",
		req.Email,
	).Scan(&userID, &passwordHash)

	if err != nil {
		// Timing attack prevention: always do a bcrypt compare even on miss
		bcrypt.CompareHashAndPassword([]byte("$2a$10$dummyhashfortimingattttttttttttttttttttttttttttttttttttt"), []byte(req.Password))
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid email or password"})
		return
	}

	if !passwordHash.Valid || passwordHash.String == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid email or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash.String), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid email or password"})
		return
	}

	// Fetch user phone for session
	var phone sql.NullString
	s.db.QueryRow("SELECT phone_number FROM users WHERE id = ?", userID).Scan(&phone)

	token, err := s.sessions.CreateSession(userID, phone.String, "email")
	if err != nil {
		dbError(w, err, "Failed to create session")
		return
	}

	// Update login tracking
	s.db.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP, login_count = login_count + 1 WHERE id = ?", userID)

	setSessionCookie(w, s.config, token)
	log.Printf("[email-auth] user %d logged in via email (%s)", userID, req.Email)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"user_id": userID,
	})
}

// handleEmailForgot handles POST /auth/email/forgot
func (s *Server) handleEmailForgot(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	// Always return success to prevent email enumeration
	successResp := map[string]bool{"success": true}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" {
		writeJSON(w, http.StatusOK, successResp)
		return
	}

	// Look up user by email auth method
	var userID int
	err := s.db.QueryRow(
		"SELECT user_id FROM auth_methods WHERE provider = 'email' AND provider_id = ?",
		req.Email,
	).Scan(&userID)
	if err != nil {
		// No account — still return success
		writeJSON(w, http.StatusOK, successResp)
		return
	}

	// Generate reset token
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		writeJSON(w, http.StatusOK, successResp)
		return
	}
	resetToken := hex.EncodeToString(b)

	// Store token (1 hour expiry)
	_, err = s.db.Exec(
		"INSERT INTO password_resets (user_id, token, expires_at) VALUES (?, ?, ?)",
		userID, resetToken, time.Now().UTC().Add(1*time.Hour),
	)
	if err != nil {
		log.Printf("[email-auth] failed to store reset token: %v", err)
		writeJSON(w, http.StatusOK, successResp)
		return
	}

	// Send reset email via SendGrid
	if s.config.SendGridAPIKey != "" {
		go s.sendPasswordResetEmail(req.Email, resetToken)
	} else {
		log.Printf("[email-auth] SENDGRID_API_KEY not set, reset token for %s: %s", req.Email, resetToken)
	}

	writeJSON(w, http.StatusOK, successResp)
}

// handleMagicLink handles POST /auth/email/magic-link
// Sends a passwordless login link to the user's email.
func (s *Server) handleMagicLink(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	// Always return success to prevent email enumeration
	successResp := map[string]bool{"success": true}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Please enter your email"})
		return
	}

	// Look up user by email auth method
	var userID int
	err := s.db.QueryRow(
		"SELECT user_id FROM auth_methods WHERE provider = 'email' AND provider_id = ?",
		req.Email,
	).Scan(&userID)
	if err != nil {
		// No account — still return success to prevent enumeration
		writeJSON(w, http.StatusOK, successResp)
		return
	}

	// Generate magic link token
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		writeJSON(w, http.StatusOK, successResp)
		return
	}
	magicToken := hex.EncodeToString(b)

	// Store token (15 minute expiry for magic links)
	_, err = s.db.Exec(
		"INSERT INTO password_resets (user_id, token, expires_at) VALUES (?, ?, ?)",
		userID, "magic:"+magicToken, time.Now().UTC().Add(15*time.Minute),
	)
	if err != nil {
		log.Printf("[email-auth] failed to store magic link token: %v", err)
		writeJSON(w, http.StatusOK, successResp)
		return
	}

	// Send magic link email
	if s.config.SendGridAPIKey != "" {
		go s.sendMagicLinkEmail(req.Email, magicToken)
	} else {
		log.Printf("[email-auth] SENDGRID_API_KEY not set, magic link token for %s: %s", req.Email, magicToken)
	}

	writeJSON(w, http.StatusOK, successResp)
}

// handleMagicLinkVerify handles GET /auth/email/magic-verify?token=...
// Validates the magic link token, creates a session, and redirects to home.
func (s *Server) handleMagicLinkVerify(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	magicToken := r.URL.Query().Get("token")
	if magicToken == "" {
		http.Redirect(w, r, "/login/email?error=invalid-link", http.StatusFound)
		return
	}

	// Look up token (stored with "magic:" prefix)
	var userID int
	var used int
	var expiresAt time.Time
	err := s.db.QueryRow(
		"SELECT user_id, used, expires_at FROM password_resets WHERE token = ?",
		"magic:"+magicToken,
	).Scan(&userID, &used, &expiresAt)
	if err != nil || used != 0 || time.Now().UTC().After(expiresAt) {
		http.Redirect(w, r, "/login/email?error=expired-link", http.StatusFound)
		return
	}

	// Mark token as used
	s.db.Exec("UPDATE password_resets SET used = 1 WHERE token = ?", "magic:"+magicToken)

	// Fetch user phone for session
	var phone sql.NullString
	s.db.QueryRow("SELECT phone_number FROM users WHERE id = ?", userID).Scan(&phone)

	token, err := s.sessions.CreateSession(userID, phone.String, "email")
	if err != nil {
		http.Redirect(w, r, "/login/email?error=session-failed", http.StatusFound)
		return
	}

	// Update login tracking
	s.db.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP, login_count = login_count + 1 WHERE id = ?", userID)

	setSessionCookie(w, s.config, token)
	log.Printf("[email-auth] user %d logged in via magic link", userID)

	// Redirect to home
	http.Redirect(w, r, "/", http.StatusFound)
}

// sendTransactionalEmail sends an HTML email via SendGrid. subject and htmlBody
// are caller-provided; the from address is always noreply@thrift.ly.
func (s *Server) sendTransactionalEmail(to, subject, htmlBody string) error {
	payload := map[string]interface{}{
		"personalizations": []map[string]interface{}{
			{
				"to":      []map[string]string{{"email": to}},
				"subject": subject,
			},
		},
		"from": map[string]string{
			"email": "noreply@thrift.ly",
			"name":  "thriftly",
		},
		"content": []map[string]string{
			{"type": "text/html", "value": htmlBody},
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.sendgrid.com/v3/mail/send", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("creating SendGrid request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.config.SendGridAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("SendGrid request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SendGrid %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// sendMagicLinkEmail sends a magic login link email via SendGrid.
func (s *Server) sendMagicLinkEmail(email, token string) {
	baseURL := s.config.WebhookBaseURL
	if baseURL == "" {
		baseURL = "http://localhost:" + s.config.Port
	}
	loginLink := baseURL + "/auth/email/magic-verify?token=" + token
	htmlBody := fmt.Sprintf(`<p>Hi there,</p><p>Click the link below to sign in to thriftly:</p><p><a href="%s">Sign in to thriftly</a></p><p>This link expires in 15 minutes. If you didn't request this, you can safely ignore this email.</p><p>— Penny</p>`, loginLink)
	if err := s.sendTransactionalEmail(email, "Sign in to thriftly", htmlBody); err != nil {
		log.Printf("[email-auth] magic link email to %s failed: %v", email, err)
	}
}

// sendPasswordResetEmail sends a password reset email via SendGrid.
func (s *Server) sendPasswordResetEmail(email, token string) {
	baseURL := s.config.WebhookBaseURL
	if baseURL == "" {
		baseURL = "http://localhost:" + s.config.Port
	}
	resetLink := baseURL + "/reset-password?token=" + token
	htmlBody := fmt.Sprintf(`<p>Hi there,</p><p>We received a request to reset your password. Click the link below to set a new password:</p><p><a href="%s">Reset Password</a></p><p>This link expires in 1 hour. If you didn't request this, you can safely ignore this email.</p><p>— thriftly</p>`, resetLink)
	if err := s.sendTransactionalEmail(email, "Reset your thriftly password", htmlBody); err != nil {
		log.Printf("[email-auth] password reset email to %s failed: %v", email, err)
	} else {
		log.Printf("[email-auth] password reset email sent to %s", email)
	}
}

// handleEmailReset handles POST /auth/email/reset
func (s *Server) handleEmailReset(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing reset token"})
		return
	}
	if len(req.Password) < 8 || len(req.Password) > 72 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Password must be 8-72 characters"})
		return
	}

	// Look up and validate token
	var userID int
	var used int
	var expiresAt time.Time
	err := s.db.QueryRow(
		"SELECT user_id, used, expires_at FROM password_resets WHERE token = ?",
		req.Token,
	).Scan(&userID, &used, &expiresAt)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid or expired reset link"})
		return
	}
	if used != 0 || time.Now().UTC().After(expiresAt) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid or expired reset link"})
		return
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		dbError(w, err, "Failed to reset password")
		return
	}

	// Update password in auth_methods
	_, err = s.db.Exec(
		"UPDATE auth_methods SET password_hash = ? WHERE user_id = ? AND provider = 'email'",
		string(hash), userID,
	)
	if err != nil {
		dbError(w, err, "Failed to reset password")
		return
	}

	// Mark token as used
	s.db.Exec("UPDATE password_resets SET used = 1 WHERE token = ?", req.Token)

	// Delete all user sessions (force re-login)
	s.sessions.DeleteUserSessions(userID)

	log.Printf("[email-auth] user %d reset password", userID)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleAPIConfig handles GET /api/config (public, no auth)
func (s *Server) handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"google_client_id": s.config.GoogleClientID,
	})
}

// handleSendCode handles POST /auth/send-code
// Request:  { "phone_number": "5551234567" }
// Response: { "success": true }
func (s *Server) handleSendCode(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		PhoneNumber string `json:"phone_number"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	e164, ok := normalizePhone(req.PhoneNumber)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Please enter a valid 10-digit US phone number",
		})
		return
	}

	// Per-IP rate limit: max 5 OTP requests per IP per 10 minutes
	clientIP := getClientIP(r)
	now := time.Now()
	window := 10 * time.Minute
	if raw, ok := s.otpIPLimiter.Load(clientIP); ok {
		times := raw.([]time.Time)
		// Keep only times within the window
		recent := make([]time.Time, 0, len(times))
		for _, t := range times {
			if now.Sub(t) < window {
				recent = append(recent, t)
			}
		}
		if len(recent) >= 5 {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "Too many requests. Please try again later.",
			})
			return
		}
		s.otpIPLimiter.Store(clientIP, append(recent, now))
	} else {
		s.otpIPLimiter.Store(clientIP, []time.Time{now})
	}

	// Rate limit: 1 OTP per phone per 60 seconds
	if last, ok := s.otpLimiter.Load(e164); ok {
		if time.Since(last.(time.Time)) < 60*time.Second {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "Please wait 60 seconds before requesting another code.",
			})
			return
		}
	}
	s.otpLimiter.Store(e164, now)

	log.Printf("Sending verification code to %s", maskPhone(e164))

	// In test mode, skip Twilio entirely — the test will use code "000000"
	if s.config.TestMode {
		log.Printf("[TEST MODE] Skipping Twilio send for %s", e164)
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	if err := s.twilio.SendVerification(e164); err != nil {
		dbError(w, err, "We couldn't send a verification code right now. Please try again in a moment.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleVerifyCode handles POST /auth/verify-code
// Request:  { "phone_number": "5551234567", "code": "123456" }
// Response: { "success": true, "user_id": 1, "phone_number": "5551234567" }
//
// On success: upserts the user in the database, creates a session, sets cookie.
func (s *Server) handleVerifyCode(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		PhoneNumber string `json:"phone_number"`
		Code        string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	e164, ok := normalizePhone(req.PhoneNumber)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid phone number",
		})
		return
	}

	log.Printf("Checking verification code for %s", e164)

	// In test mode, accept "000000" without calling Twilio
	var approved bool
	var err error
	if s.config.TestMode && req.Code == "000000" {
		log.Printf("[TEST MODE] Auto-approving code for %s", e164)
		approved = true
	} else {
		approved, err = s.twilio.CheckVerification(e164, req.Code)
		if err != nil {
			dbError(w, err, "We couldn't verify your code. Please try again.")
			return
		}
	}

	if !approved {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Invalid code",
		})
		return
	}

	// Phone verified — upsert the user in the database.
	// If this phone has logged in before, we get the existing user.
	// If it's new, a row is created. Either way, we have a user ID.
	digits := strings.TrimPrefix(e164, "+1")

	user, err := upsertUser(s.db, digits)
	if err != nil {
		dbError(w, err, "We couldn't set up your account. Please try again.")
		return
	}

	token, err := s.sessions.CreateSession(user.ID, digits, "phone")
	if err != nil {
		dbError(w, err, "We couldn't log you in. Please try again.")
		return
	}

	setSessionCookie(w, s.config, token)

	log.Printf("User %d (%s) logged in", user.ID, digits)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"user_id":      user.ID,
		"phone_number": digits,
		"is_admin":     s.config.AdminPhone != "" && digits == s.config.AdminPhone,
	})
}

// handleLogout handles POST /auth/logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	if cookie, err := r.Cookie("session"); err == nil {
		s.sessions.DeleteSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.config.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleMe handles GET /auth/me
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	cookie, err := r.Cookie("session")
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Not logged in",
		})
		return
	}

	session, ok := s.sessions.GetSession(cookie.Value)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Session expired",
		})
		return
	}

	resp := map[string]interface{}{
		"logged_in":    true,
		"user_id":      session.UserID,
		"auth_method":  session.AuthMethod,
		"is_admin":     s.config.AdminPhone != "" && session.PhoneNumber != "" && session.PhoneNumber == s.config.AdminPhone,
	}
	if session.PhoneNumber != "" {
		resp["phone_number"] = session.PhoneNumber
	}
	writeJSON(w, http.StatusOK, resp)
}

// listItemColumns is the shared SELECT column list for list_items queries.
const listItemColumns = "li.id, li.name, li.brand, li.department, li.quantity, li.size, li.notes, li.source, li.checked, li.urgent, li.created_at, COALESCE(u.name,''), COALESCE(u.phone_number,''), li.claimed_by, COALESCE(cu.name,'')"

// listItemJoins is the shared FROM/JOIN clause for list_items queries.
const listItemJoins = "list_items li JOIN users u ON u.id = li.user_id LEFT JOIN users cu ON cu.id = li.claimed_by"

// scanListItem scans a row into a ListItem. The row must match listItemColumns.
