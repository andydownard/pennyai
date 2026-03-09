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

	var req GoogleAuthRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Credential == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing credential"})
		return
	}

	// Verify Google ID token via tokeninfo endpoint
	tokenInfoURL := "https://oauth2.googleapis.com/tokeninfo"
	if s.googleTokenInfoURL != "" {
		tokenInfoURL = s.googleTokenInfoURL
	}
	resp, err := http.Get(tokenInfoURL + "?id_token=" + url.QueryEscape(req.Credential))
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

	var tokenInfo GoogleTokenInfo
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

	// Auto-populate profile photo from Google if user doesn't already have one
	if tokenInfo.Picture != "" {
		s.maybeStoreGooglePhoto(user.ID, tokenInfo.Picture)
	}

	token, err := s.sessions.CreateSession(user.ID, user.PhoneNumber, "google")
	if err != nil {
		dbError(w, err, "Failed to create session")
		return
	}

	setSessionCookie(w, s.config, token)
	log.Printf("[google-auth] user %d logged in via Google (%s)", user.ID, tokenInfo.Email)

	authResp := map[string]interface{}{
		"success": true,
		"user_id": user.ID,
	}
	if user.Merged {
		authResp["account_merged"] = true
		authResp["merge_message"] = "Welcome back! I linked your Google account to your existing Thriftly account."
	}
	writeJSON(w, http.StatusOK, authResp)
}

// maybeStoreGooglePhoto fetches and stores a Google profile photo if the user
// doesn't already have one. Errors are logged but never fail the auth flow.
func (s *Server) maybeStoreGooglePhoto(userID int, pictureURL string) {
	// Validate HTTPS URL
	parsed, err := url.Parse(pictureURL)
	if err != nil || parsed.Scheme != "https" {
		log.Printf("[google-auth] ignoring invalid picture URL for user %d: %s", userID, pictureURL)
		return
	}

	// Check if user already has a profile photo — don't overwrite
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM profile_photos WHERE user_id = ?", userID).Scan(&count); err != nil {
		log.Printf("[google-auth] failed to check existing photo for user %d: %v", userID, err)
		return
	}
	if count > 0 {
		log.Printf("[google-auth] user %d already has a profile photo, skipping Google photo", userID)
		return
	}

	// Fetch the image with a short timeout
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(pictureURL)
	if err != nil {
		log.Printf("[google-auth] failed to fetch Google photo for user %d: %v", userID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[google-auth] Google photo returned %d for user %d", resp.StatusCode, userID)
		return
	}

	// Read up to 512KB + 1 byte to detect oversized images
	const maxGooglePhotoBytes = 512 * 1024
	photoData, err := io.ReadAll(io.LimitReader(resp.Body, maxGooglePhotoBytes+1))
	if err != nil {
		log.Printf("[google-auth] failed to read Google photo for user %d: %v", userID, err)
		return
	}
	if len(photoData) > maxGooglePhotoBytes {
		log.Printf("[google-auth] Google photo too large for user %d (%d bytes)", userID, len(photoData))
		return
	}
	if len(photoData) == 0 {
		log.Printf("[google-auth] Google photo empty for user %d", userID)
		return
	}

	// Validate magic bytes: JPEG (FF D8 FF) or PNG (89 50 4E 47)
	var contentType string
	if len(photoData) >= 3 && photoData[0] == 0xFF && photoData[1] == 0xD8 && photoData[2] == 0xFF {
		contentType = "image/jpeg"
	} else if len(photoData) >= 4 && photoData[0] == 0x89 && photoData[1] == 0x50 && photoData[2] == 0x4E && photoData[3] == 0x47 {
		contentType = "image/png"
	} else {
		log.Printf("[google-auth] Google photo for user %d is not JPEG or PNG", userID)
		return
	}

	// Store in profile_photos
	_, err = s.db.Exec(
		`INSERT INTO profile_photos (user_id, photo_data, content_type, updated_at)
		 VALUES (?, ?, ?, datetime('now'))
		 ON CONFLICT(user_id) DO UPDATE SET photo_data = excluded.photo_data, content_type = excluded.content_type, updated_at = datetime('now')`,
		userID, photoData, contentType,
	)
	if err != nil {
		log.Printf("[google-auth] failed to store Google photo for user %d: %v", userID, err)
		return
	}
	log.Printf("[google-auth] stored Google profile photo for user %d (%d bytes, %s)", userID, len(photoData), contentType)
}

// handleEmailRegister handles POST /auth/email/register
func (s *Server) handleEmailRegister(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req EmailRegisterRequest
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

	resp := map[string]interface{}{
		"success": true,
		"user_id": user.ID,
	}
	if user.Merged {
		resp["account_merged"] = true
		resp["merge_message"] = "Welcome back! I linked your email to your existing Thriftly account."
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleEmailLogin handles POST /auth/email/login
func (s *Server) handleEmailLogin(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	// IP rate limit: max 10 per IP per 10 minutes
	clientIP := getClientIP(r)
	if checkRateLimit(&s.emailLoginIPLimiter, clientIP, 10*time.Minute, 10) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many login attempts. Please try again later."})
		return
	}

	var req EmailLoginRequest
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

	// Rate limit: max 5 password reset requests per IP per 15 minutes
	clientIP := getClientIP(r)
	if checkRateLimit(&s.passwordResetIPLimiter, "forgot:"+clientIP, 15*time.Minute, 5) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many requests. Please try again later."})
		return
	}

	var req EmailOnlyRequest
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
		log.Printf("[email-auth] SENDGRID_API_KEY not set, skipping password reset email for %s", req.Email)
	}

	writeJSON(w, http.StatusOK, successResp)
}

// handleMagicLink handles POST /auth/email/magic-link
// Sends a passwordless login link to the user's email.
func (s *Server) handleMagicLink(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	// Rate limit: max 5 magic link requests per IP per 15 minutes
	clientIP := getClientIP(r)
	if checkRateLimit(&s.passwordResetIPLimiter, "magic:"+clientIP, 15*time.Minute, 5) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many requests. Please try again later."})
		return
	}

	var req EmailOnlyRequest
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
		log.Printf("[email-auth] SENDGRID_API_KEY not set, skipping magic link email for %s", req.Email)
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

	var req PasswordResetRequest
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

	var req PhoneRequest
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

	// Skip all OTP rate limiting in test mode to allow E2E tests to run freely
	clientIP := getClientIP(r)
	if !s.config.TestMode {
		// Per-IP rate limit: max 5 OTP requests per IP per 10 minutes
		if checkRateLimit(&s.otpIPLimiter, clientIP, 10*time.Minute, 5) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "Too many requests. Please try again later.",
			})
			return
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
		s.otpLimiter.Store(e164, time.Now())
	}

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

	var req VerifyCodeRequest
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
		"logged_in":            true,
		"user_id":              session.UserID,
		"auth_method":          session.AuthMethod,
		"is_admin":             s.config.AdminPhone != "" && session.PhoneNumber != "" && session.PhoneNumber == s.config.AdminPhone,
		"can_submit_feedback":  s.isFeedbackUser(session),
	}
	if session.PhoneNumber != "" {
		resp["phone_number"] = session.PhoneNumber
	}

	// Include login_count and name for onboarding flow
	var loginCount int
	var userName string
	s.db.QueryRow("SELECT login_count, name FROM users WHERE id = ?", session.UserID).Scan(&loginCount, &userName)
	resp["login_count"] = loginCount
	if userName != "" {
		resp["name"] = userName
	}

	// Include auth_methods array
	authMethods := []string{}
	rows, err := s.db.Query("SELECT DISTINCT provider FROM auth_methods WHERE user_id = ? ORDER BY provider", session.UserID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var provider string
			if rows.Scan(&provider) == nil {
				authMethods = append(authMethods, provider)
			}
		}
	}
	resp["auth_methods"] = authMethods

	writeJSON(w, http.StatusOK, resp)
}

// handleAuthLink handles POST /api/auth/link
// Links an additional auth method (email or phone) to the current user.
func (s *Server) handleAuthLink(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req AuthLinkRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	switch req.Provider {
	case "email":
		s.linkEmail(w, session.UserID, req.Email)
	case "phone":
		s.linkPhone(w, r, session.UserID, req.Phone, req.Code)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid provider. Must be 'email' or 'phone'."})
	}
}

// linkEmail links an email auth method to an existing user.
func (s *Server) linkEmail(w http.ResponseWriter, userID int, email string) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Email is required"})
		return
	}
	if _, err := mail.ParseAddress(email); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid email address"})
		return
	}

	// Check if this email is already linked to a DIFFERENT user
	var existingUserID int
	err := s.db.QueryRow(
		"SELECT user_id FROM auth_methods WHERE provider = 'email' AND provider_id = ?",
		email,
	).Scan(&existingUserID)
	if err == nil && existingUserID != userID {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "This email is already linked to another account"})
		return
	}

	// Idempotent: if already linked to this user, return success
	if err == nil && existingUserID == userID {
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
		return
	}

	// Insert auth method
	_, err = s.db.Exec(
		"INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id) VALUES (?, 'email', ?)",
		userID, email,
	)
	if err != nil {
		dbError(w, err, "Failed to link email")
		return
	}

	// Update users.email
	s.db.Exec("UPDATE users SET email = ? WHERE id = ? AND (email IS NULL OR email = '')", email, userID)

	log.Printf("[auth-link] user %d linked email %s", userID, email)
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// linkPhone links a phone auth method to an existing user after verifying the SMS code.
func (s *Server) linkPhone(w http.ResponseWriter, r *http.Request, userID int, phone, code string) {
	e164, ok := normalizePhone(phone)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid phone number"})
		return
	}

	if code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Verification code is required"})
		return
	}

	// Verify the code
	var approved bool
	var err error
	if s.config.TestMode && code == "000000" {
		approved = true
	} else {
		approved, err = s.twilio.CheckVerification(e164, code)
		if err != nil {
			dbError(w, err, "Failed to verify code")
			return
		}
	}

	if !approved {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid verification code"})
		return
	}

	// Strip +1 prefix for storage (matches existing pattern)
	digits := strings.TrimPrefix(e164, "+1")

	// Check if this phone is already linked to a DIFFERENT user
	var existingUserID int
	err = s.db.QueryRow(
		"SELECT user_id FROM auth_methods WHERE provider = 'phone' AND provider_id = ?",
		digits,
	).Scan(&existingUserID)
	if err == nil && existingUserID != userID {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "This phone number is already linked to another account"})
		return
	}

	// Idempotent: if already linked to this user, return success
	if err == nil && existingUserID == userID {
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
		return
	}

	// Insert auth method
	_, err = s.db.Exec(
		"INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id) VALUES (?, 'phone', ?)",
		userID, digits,
	)
	if err != nil {
		dbError(w, err, "Failed to link phone")
		return
	}

	// Update users.phone_number
	s.db.Exec("UPDATE users SET phone_number = ? WHERE id = ? AND (phone_number IS NULL OR phone_number = '')", digits, userID)

	log.Printf("[auth-link] user %d linked phone %s", userID, maskPhone(e164))
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// listItemColumns is the shared SELECT column list for list_items queries.
const listItemColumns = "li.id, li.name, li.brand, li.department, li.quantity, li.size, li.notes, li.source, li.checked, li.urgent, li.created_at, COALESCE(u.name,''), COALESCE(u.phone_number,''), li.claimed_by, COALESCE(cu.name,''), li.swap_savings_cents"

// listItemJoins is the shared FROM/JOIN clause for list_items queries.
const listItemJoins = "list_items li JOIN users u ON u.id = li.user_id LEFT JOIN users cu ON cu.id = li.claimed_by"

// scanListItem scans a row into a ListItem. The row must match listItemColumns.
