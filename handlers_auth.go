package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// handleRetiredTexting answers every retired texting endpoint with 410 Gone.
func handleRetiredTexting(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{
		"error": "Texting is no longer part of Groflo.",
	})
}

// handleTestLogin handles POST /auth/test-login. TEST MODE ONLY: it mints a
// session with no credentials, for end-to-end tests. The route is registered
// only when TestMode is on, and the handler refuses again on its own so a
// future registration mistake cannot expose it.
//
// Request (optional): {"name":"Pat"}. Every call mints a new user; any other
// field, including a phone number, is ignored.
func (s *Server) handleTestLogin(w http.ResponseWriter, r *http.Request) {
	if !s.config.TestMode {
		writeNotFound(w, "Not found")
		return
	}
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeBadRequest(w, "Invalid request body")
		return
	}
	name := strings.TrimSpace(req.Name)

	user, err := createPhonelessTestUser(s.db, name)
	if err != nil {
		dbError(w, err, "Failed to sign in")
		return
	}

	token, err := s.sessions.CreateSession(user.ID, "", "test")
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
		"can_submit_feedback":  s.isAdmin(session),
	}

	// Include login_count and name for onboarding flow
	var loginCount int
	var userName string
	s.db.QueryRow(
		"SELECT login_count, name FROM users WHERE id = ?",
		session.UserID,
	).Scan(&loginCount, &userName)
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

// listItemColumns is the shared SELECT column list for list_items queries.
const listItemColumns = "li.id, li.name, li.brand, li.department, li.quantity, li.size, li.notes, li.source, li.checked, li.urgent, li.created_at, COALESCE(u.name,''), li.claimed_by, COALESCE(cu.name,''), li.swap_savings_cents, li.emoji"

// listItemJoins is the shared FROM/JOIN clause for list_items queries.
const listItemJoins = "list_items li JOIN users u ON u.id = li.user_id LEFT JOIN users cu ON cu.id = li.claimed_by"
