package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// timpagerVerifyResponse is the response from timpager's GET /api/auth/verify
type timpagerVerifyResponse struct {
	Valid       bool   `json:"valid"`
	UserID      string `json:"user_id"`
	Phone       string `json:"phone"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

// timpagerVerifyCache caches verify responses for 60 seconds
var timpagerVerifyCache = struct {
	sync.Mutex
	entries map[string]timpagerCacheEntry
}{entries: make(map[string]timpagerCacheEntry)}

type timpagerCacheEntry struct {
	response timpagerVerifyResponse
	expiry   time.Time
}

// verifyTimpagerSession calls timpager's /api/auth/verify to validate a session token.
// Caches responses for 60 seconds to avoid hammering timpager.
func (s *Server) verifyTimpagerSession(token string) (*timpagerVerifyResponse, error) {
	if s.config.TimpagerURL == "" {
		return nil, fmt.Errorf("TIMPAGER_URL not configured")
	}

	// Check cache
	timpagerVerifyCache.Lock()
	if entry, ok := timpagerVerifyCache.entries[token]; ok && time.Now().Before(entry.expiry) {
		timpagerVerifyCache.Unlock()
		return &entry.response, nil
	}
	timpagerVerifyCache.Unlock()

	// Call timpager
	req, err := http.NewRequest("GET", s.config.TimpagerURL+"/api/auth/verify", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("timpager verify request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("timpager verify returned %d", resp.StatusCode)
	}

	var verifyResp timpagerVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
		return nil, fmt.Errorf("timpager verify decode failed: %w", err)
	}

	if !verifyResp.Valid {
		return nil, fmt.Errorf("timpager session invalid")
	}

	// Cache for 60 seconds
	timpagerVerifyCache.Lock()
	timpagerVerifyCache.entries[token] = timpagerCacheEntry{
		response: verifyResp,
		expiry:   time.Now().Add(60 * time.Second),
	}
	timpagerVerifyCache.Unlock()

	return &verifyResp, nil
}

// requireSessionWithTimpager extends requireSession to also check X-Timpager-Session header.
// If the header is present, validates against timpager and maps to a Penny user.
// If absent, falls through to normal cookie-based auth.
func (s *Server) requireSessionWithTimpager(w http.ResponseWriter, r *http.Request) (Session, bool) {
	// Check for X-Timpager-Session header first
	timpagerToken := r.Header.Get("X-Timpager-Session")
	if timpagerToken != "" {
		verifyResp, err := s.verifyTimpagerSession(timpagerToken)
		if err != nil {
			log.Printf("[timpager-auth] verify failed: %v", err)
			// Fall through to cookie auth
		} else {
			// Map timpager user to Penny user
			phone := verifyResp.Phone
			if phone == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Timpager user has no phone"})
				return Session{}, false
			}

			// Find or create Penny user by phone
			userID := s.resolveOrCreateTimpagerUser(phone, verifyResp.DisplayName)
			if userID == 0 {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to map timpager user"})
				return Session{}, false
			}

			return Session{
				UserID:      userID,
				PhoneNumber: phone,
				AuthMethod:  "timpager",
				CreatedAt:   time.Now(),
			}, true
		}
	}

	// Fall through to normal cookie-based auth
	return s.requireSession(w, r)
}

// resolveOrCreateTimpagerUser finds or creates a Penny user by phone number.
func (s *Server) resolveOrCreateTimpagerUser(phone, displayName string) int {
	// Try to find existing user
	var userID int
	err := s.db.QueryRow(`SELECT id FROM users WHERE phone = ?`, phone).Scan(&userID)
	if err == nil {
		return userID
	}

	// Create new user
	result, err := s.db.Exec(
		`INSERT INTO users (phone, display_name, auth_method, created_at) VALUES (?, ?, 'timpager', datetime('now'))`,
		phone, displayName,
	)
	if err != nil {
		log.Printf("[timpager-auth] failed to create user for phone %s: %v", phone, err)
		return 0
	}

	id, _ := result.LastInsertId()
	log.Printf("[timpager-auth] created Penny user %d for timpager phone %s", id, phone)
	return int(id)
}

// pushRevenueEvent sends a revenue event to timpager (fire-and-forget).
func (s *Server) pushRevenueEvent(eventType string, amountCents int, sourceRef, idempotencyKey string) {
	if s.config.TimpagerURL == "" || s.config.TimpagerAPIKey == "" {
		return
	}

	go func() {
		body := map[string]interface{}{
			"event_type":      eventType,
			"amount_cents":    amountCents,
			"currency":        "USD",
			"source_ref":      sourceRef,
			"idempotency_key": idempotencyKey,
		}
		jsonBody, _ := json.Marshal(body)

		for attempt := 0; attempt < 3; attempt++ {
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Post(
				s.config.TimpagerURL+"/api/marketplaces/penny/revenue-events",
				"application/json",
				bytes.NewReader(jsonBody),
			)
			if err != nil {
				log.Printf("[timpager-revenue] attempt %d failed: %v", attempt+1, err)
				time.Sleep(time.Duration(1<<attempt) * time.Second) // exponential backoff
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == 201 || resp.StatusCode == 409 {
				// 201 = created, 409 = duplicate (idempotent success)
				log.Printf("[timpager-revenue] pushed %s event: %dc (status %d)", eventType, amountCents, resp.StatusCode)
				return
			}
			log.Printf("[timpager-revenue] attempt %d got status %d", attempt+1, resp.StatusCode)
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
		log.Printf("[timpager-revenue] failed after 3 attempts: %s %dc", eventType, amountCents)
	}()
}

