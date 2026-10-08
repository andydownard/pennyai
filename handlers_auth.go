package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// sendTransactionalEmail sends an HTML email via SendGrid. subject and htmlBody
// are caller-provided; the from address is always noreply@groflo.io.
func (s *Server) sendTransactionalEmail(to, subject, htmlBody string) error {
	payload := map[string]interface{}{
		"personalizations": []map[string]interface{}{
			{
				"to":      []map[string]string{{"email": to}},
				"subject": subject,
			},
		},
		"from": map[string]string{
			"email": "noreply@groflo.io",
			"name":  "Groflo",
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

// handleRetiredAuth answers every retired sign-in endpoint with 410 Gone.
func handleRetiredAuth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{
		"error": "This sign-in method has been retired. Use Continue with Rally.",
	})
}

// handleTestLogin handles POST /auth/test-login. TEST MODE ONLY: it mints a
// session with no credentials, for end-to-end tests. The route is registered
// only when TestMode is on, and the handler refuses again on its own so a
// future registration mistake cannot expose it.
//
// Request (both optional): {"phone_number":"2125550101","name":"Pat"}
func (s *Server) handleTestLogin(w http.ResponseWriter, r *http.Request) {
	if !s.config.TestMode {
		writeNotFound(w, "Not found")
		return
	}
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		PhoneNumber string `json:"phone_number"`
		Name        string `json:"name"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeBadRequest(w, "Invalid request body")
		return
	}
	name := strings.TrimSpace(req.Name)

	var user User
	if req.PhoneNumber != "" {
		e164, ok := normalizePhone(req.PhoneNumber)
		if !ok {
			writeBadRequest(w, "Enter a 10-digit phone number")
			return
		}
		u, err := upsertUser(s.db, strings.TrimPrefix(e164, "+1"))
		if err != nil {
			dbError(w, err, "Failed to sign in")
			return
		}
		user = u
		// Record SMS consent so /auth/me reports needs_sms_consent=false and the
		// consent modal does not cover the app in browser tests. Idempotent.
		ua := r.Header.Get("User-Agent")
		if len(ua) > 500 {
			ua = ua[:500]
		}
		if _, err := s.db.Exec(
			"UPDATE users SET sms_consent_at = CURRENT_TIMESTAMP, sms_consent_ip = ?, sms_consent_user_agent = ? WHERE id = ? AND sms_consent_at IS NULL",
			getClientIP(r), ua, user.ID,
		); err != nil {
			dbError(w, err, "Failed to sign in")
			return
		}
		if name != "" {
			if _, err := s.db.Exec("UPDATE users SET name = ? WHERE id = ? AND name = ''", name, user.ID); err != nil {
				dbError(w, err, "Failed to sign in")
				return
			}
		}
	} else {
		u, err := createPhonelessTestUser(s.db, name)
		if err != nil {
			dbError(w, err, "Failed to sign in")
			return
		}
		user = u
	}

	token, err := s.sessions.CreateSession(user.ID, user.PhoneNumber, "test")
	if err != nil {
		dbError(w, err, "Failed to create session")
		return
	}
	setSessionCookie(w, s.config, token)
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "user_id": user.ID})
}

// createPhonelessTestUser mints a user with no phone and a solo household.
func createPhonelessTestUser(db *PGDB, name string) (User, error) {
	tx, err := db.Begin()
	if err != nil {
		return User{}, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()
	id := uuid.New().String()
	if _, err := tx.Exec("INSERT INTO users (id, phone, name) VALUES (?, NULL, ?)", id, name); err != nil {
		return User{}, fmt.Errorf("creating test user: %w", err)
	}
	hID, err := createSoloHouseholdTx(tx, id)
	if err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("committing: %w", err)
	}
	return User{ID: id, Name: name, HouseholdID: hID}, nil
}

// handleAPIConfig handles GET /api/config (public, no auth)
func (s *Server) handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"clerk_enabled": clerkIsEnabled(),
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
		"is_admin":             s.isAdmin(session),
		"can_submit_feedback":  s.isFeedbackUser(session),
	}
	if session.PhoneNumber != "" {
		resp["phone_number"] = session.PhoneNumber
	}

	// Include login_count and name for onboarding flow
	var loginCount int
	var userName string
	var smsConsentAt sql.NullString
	var phoneNumber sql.NullString
	s.db.QueryRow(
		"SELECT login_count, name, sms_consent_at, phone FROM users WHERE id = ?",
		session.UserID,
	).Scan(&loginCount, &userName, &smsConsentAt, &phoneNumber)
	resp["login_count"] = loginCount
	if userName != "" {
		resp["name"] = userName
	}

	// needs_sms_consent: existing user has a phone number on file but no
	// recorded consent timestamp. Triggers the rebrand re-consent modal so
	// Twilio A2P 10DLC has a provable consent record per user. Set once a
	// user explicitly re-confirms via POST /api/me/sms-consent.
	resp["needs_sms_consent"] = phoneNumber.Valid && phoneNumber.String != "" && !smsConsentAt.Valid

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

// handleSmsConsent handles POST /api/me/sms-consent
// Records the authenticated user's SMS consent decision under the new Groflo
// brand. Required for users whose accounts pre-date the rebrand and have a
// NULL sms_consent_at column. Twilio A2P 10DLC vetting requires a provable
// consent record per user.
//
// Request:  { "opted_in": true } or { "opted_in": false }
// Response: { "success": true, "needs_sms_consent": false }
//
// opted_in=true:  records timestamp + IP + UA, future SMS will fire
// opted_in=false: clears any prior consent timestamp; future outbound SMS
//                 to this user will be skipped by canSendSMS()
func (s *Server) handleSmsConsent(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		OptedIn bool `json:"opted_in"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	clientIP := getClientIP(r)
	userAgent := r.Header.Get("User-Agent")
	if len(userAgent) > 500 {
		userAgent = userAgent[:500]
	}

	if req.OptedIn {
		if _, err := s.db.Exec(
			`UPDATE users SET sms_consent_at = CURRENT_TIMESTAMP,
			   sms_consent_ip = ?, sms_consent_user_agent = ?
			 WHERE id = ?`,
			clientIP, userAgent, session.UserID,
		); err != nil {
			dbError(w, err, "Failed to record consent")
			return
		}
		log.Printf("[sms-consent] user %s opted in", session.UserID)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success":           true,
			"needs_sms_consent": false,
		})
		return
	}

	// opted_in == false: clear any prior consent. Outbound SMS will stop.
	if _, err := s.db.Exec(
		`UPDATE users SET sms_consent_at = NULL WHERE id = ?`, session.UserID,
	); err != nil {
		dbError(w, err, "Failed to record opt-out")
		return
	}
	log.Printf("[sms-consent] user %s opted out (pennySMS will be skipped)", session.UserID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":           true,
		"needs_sms_consent": true,
	})
}

// canSendSMS returns true if the user has a recorded SMS consent timestamp.
// Wire this into every outbound SMS code path for Twilio A2P 10DLC compliance.
// NULL consent = consent not yet captured (legacy users pre-rebrand) or user
// explicitly opted out. Either way, do not send.
func (s *Server) canSendSMS(userID string) bool {
	var consentAt sql.NullString
	err := s.db.QueryRow(
		`SELECT sms_consent_at FROM users WHERE id = ?`, userID,
	).Scan(&consentAt)
	if err != nil {
		log.Printf("[sms-consent] canSendSMS lookup failed for user %s: %v", userID, err)
		return false
	}
	return consentAt.Valid
}

// listItemColumns is the shared SELECT column list for list_items queries.
const listItemColumns = "li.id, li.name, li.brand, li.department, li.quantity, li.size, li.notes, li.source, li.checked, li.urgent, li.created_at, COALESCE(u.name,''), COALESCE(u.phone,''), li.claimed_by, COALESCE(cu.name,''), li.swap_savings_cents, li.emoji"

// listItemJoins is the shared FROM/JOIN clause for list_items queries.
const listItemJoins = "list_items li JOIN users u ON u.id = li.user_id LEFT JOIN users cu ON cu.id = li.claimed_by"
