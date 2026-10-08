package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"
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
