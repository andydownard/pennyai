package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type cachedSession struct {
	session   Session
	expiresAt time.Time
}

const sessionCacheTTL = 5 * time.Minute

type SessionStore struct {
	db    *PGDB
	mu    sync.RWMutex
	cache map[string]cachedSession
}

// NewSessionStore takes the PGDB wrapper, never the raw connection: its queries
// use `?` placeholders, which only the wrapper converts for PostgreSQL.
func NewSessionStore(db *PGDB) *SessionStore {
	return &SessionStore{db: db, cache: make(map[string]cachedSession)}
}

// CreateSession generates a random 64-char hex token, stores the session
// in the sessions table, and returns the token.
// Sessions expire after 30 days.
func (s *SessionStore) CreateSession(userID string, phone string, authMethod string) (string, error) {
	b := make([]byte, 32) // 32 bytes = 256 bits of randomness
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)

	now := time.Now().UTC()
	expiresAt := now.Add(sessionDuration)

	_, err := s.db.Exec(
		"INSERT INTO sessions (token, phone, user_id, role, created_at, last_used, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		token, phone, userID, "user", now, now, expiresAt,
	)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *SessionStore) GetSession(token string) (Session, bool) {
	// Check in-memory cache first
	s.mu.RLock()
	if cached, ok := s.cache[token]; ok && time.Now().Before(cached.expiresAt) {
		s.mu.RUnlock()
		return cached.session, true
	}
	s.mu.RUnlock()

	var session Session
	var phone sql.NullString
	var userID sql.NullString
	var displayName sql.NullString
	var email sql.NullString
	// Join with users to get display_name and email.
	err := s.db.QueryRow(
		`SELECT s.user_id, s.phone, s.role, s.created_at, u.display_name, u.email
		 FROM sessions s
		 LEFT JOIN users u ON s.user_id = u.id
		 WHERE s.token = ? AND s.expires_at > ?`,
		token, time.Now().UTC(),
	).Scan(&userID, &phone, &session.AuthMethod, &session.CreatedAt, &displayName, &email)
	if err != nil {
		return Session{}, false
	}
	session.PhoneNumber = phone.String
	session.UserID = userID.String
	_ = displayName
	_ = email
	if session.AuthMethod == "" {
		session.AuthMethod = "phone"
	}

	// Update last_used timestamp (fire-and-forget)
	go func() {
		_, _ = s.db.Exec("UPDATE sessions SET last_used = ? WHERE token = ?", time.Now().UTC(), token)
	}()

	// Cache the result
	s.mu.Lock()
	s.cache[token] = cachedSession{session: session, expiresAt: time.Now().Add(sessionCacheTTL)}
	// Evict expired entries if cache grows large
	if len(s.cache) > 256 {
		now := time.Now()
		for k, v := range s.cache {
			if now.After(v.expiresAt) {
				delete(s.cache, k)
			}
		}
	}
	s.mu.Unlock()

	return session, true
}

// DeleteUserSessions removes all sessions for a user (e.g. after password reset).
func (s *SessionStore) DeleteUserSessions(userID string) {
	// Clear cache entries for this user
	s.mu.Lock()
	for k, v := range s.cache {
		if v.session.UserID == userID {
			delete(s.cache, k)
		}
	}
	s.mu.Unlock()
	if _, err := s.db.Exec("DELETE FROM sessions WHERE user_id = ?", userID); err != nil {
		log.Printf("[warn] failed to delete sessions for user %s: %v", userID, err)
	}
}

func (s *SessionStore) DeleteSession(token string) {
	s.mu.Lock()
	delete(s.cache, token)
	s.mu.Unlock()
	if _, err := s.db.Exec("DELETE FROM sessions WHERE token = ?", token); err != nil {
		log.Printf("[warn] failed to delete session: %v", err)
	}
}

// CleanExpired removes expired sessions. Called periodically.
func (s *SessionStore) CleanExpired() {
	if _, err := s.db.Exec("DELETE FROM sessions WHERE expires_at <= ?", time.Now().UTC()); err != nil {
		log.Printf("[warn] failed to clean expired sessions: %v", err)
	}
}

// =============================================================================
// TWILIO SMS CLIENT
//
// Interface + real implementation. Tests use a mock (in test_helpers_test.go).
// Sign-in does not use Twilio; it only sends and receives SMS.
// =============================================================================

type TwilioClient interface {
	SendSMS(from, to, body string) error
}

type RealTwilioClient struct {
	AccountSID string
	AuthToken  string
}

// doRequest performs a Twilio API POST with Basic Auth and checks the status.
func (t *RealTwilioClient) doRequest(apiURL string, data url.Values, expectStatus int) ([]byte, error) {
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth(t.AccountSID, t.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := twilioHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling Twilio: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != expectStatus {
		return body, fmt.Errorf("twilio returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// SendSMS sends an SMS message via the Twilio Messages API.
func (t *RealTwilioClient) SendSMS(from, to, body string) error {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.AccountSID)
	data := url.Values{"From": {from}, "To": {to}, "Body": {body}}
	_, err := t.doRequest(apiURL, data, http.StatusCreated)
	return err
}

// =============================================================================
// PHONE NUMBER NORMALIZATION
//
// The frontend sends raw 10-digit numbers. Twilio needs E.164 ("+15551234567").
// We also handle common edge cases: leading country code "1", formatting
// characters like parens/dashes/dots/spaces, and accidental letters.
// =============================================================================

// normalizePhone strips non-digits, drops a leading country code 1,
// and returns E.164 format. Returns ("", false) if the result isn't 10 digits.
func normalizePhone(input string) (string, bool) {
	// Keep only digits — drop parens, dashes, dots, spaces, letters, everything
	var digits strings.Builder
	for _, r := range input {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()

	// If someone typed "15551234567" or pasted "+1 (555) 123-4567" (which
	// becomes "15551234567" after stripping), drop the leading country code.
	if len(d) == 11 && d[0] == '1' {
		d = d[1:]
	}

	// Must be exactly 10 digits after cleanup
	if len(d) != 10 {
		return "", false
	}

	return "+1" + d, true
}

