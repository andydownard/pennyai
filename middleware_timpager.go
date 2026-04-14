package main

import (
	"bytes"
	"database/sql"
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
			if userID == "" {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to map timpager user"})
				return Session{}, false
			}

			return Session{
				UserID: userID,
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
func (s *Server) resolveOrCreateTimpagerUser(phone, displayName string) string {
	// Try to find existing user
	var userID string
	err := s.db.QueryRow(`SELECT id FROM users WHERE phone = ?`, phone).Scan(&userID)
	if err == nil {
		return userID
	}

	// Create new user
	err = s.db.QueryRow(
		`INSERT INTO users (phone, display_name, auth_method, created_at) VALUES (?, ?, 'timpager', datetime('now')) RETURNING id`,
		phone, displayName,
	).Scan(&userID)
	if err != nil {
		log.Printf("[timpager-auth] failed to create user for phone %s: %v", phone, err)
		return ""
	}

	log.Printf("[timpager-auth] created Penny user %s for timpager phone %s", userID, phone)
	return userID
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

// pushPayoutToTimpager queues a user payout (e.g., swap rebate cash-out) onto
// timpager's Stripe Connect platform. STUB MODE for Phase 4 prep — the
// timpager endpoint POST /api/marketplaces/penny/payouts does not yet exist.
// This helper logs the intended payout and (optionally) writes to a local
// outbox table for later replay. When the timpager endpoint ships, flip
// stubMode to false and the helper will start posting for real.
//
// Idempotency key format: groflo_payout_{distributionID}.
//
// SAFETY (per timpager CLAUDE.md): NEVER retry on 409 — duplicate is success.
// NEVER call from inside a wallet UPDATE transaction; call AFTER commit.
func (s *Server) pushPayoutToTimpager(grofloUserID string, amountCents int, distributionID string) {
	if amountCents <= 0 {
		log.Printf("[timpager-payout] refusing non-positive amount: %d", amountCents)
		return
	}
	if distributionID == "" {
		log.Printf("[timpager-payout] refusing empty distribution_id")
		return
	}
	idempotencyKey := fmt.Sprintf("groflo_payout_%s", distributionID)

	// STUB MODE — log only, do not yet POST. Flip to false in Phase 4.
	const stubMode = true
	if stubMode {
		log.Printf("[timpager-payout][STUB] would push payout user=%s amount=%dc distribution=%s key=%s",
			grofloUserID, amountCents, distributionID, idempotencyKey)
		return
	}

	if s.config.TimpagerURL == "" || s.config.TimpagerAPIKey == "" {
		log.Printf("[timpager-payout] not configured (missing TIMPAGER_URL/KEY) — skipping payout %s", distributionID)
		return
	}

	// Resolve timpager_user_id from local cache (set during Phase 3 backfill)
	var timpagerUserID sql.NullString
	s.db.QueryRow(
		`SELECT timpager_user_id FROM users WHERE id = ?`, grofloUserID,
	).Scan(&timpagerUserID)
	if !timpagerUserID.Valid || timpagerUserID.String == "" {
		log.Printf("[timpager-payout] user %s has no timpager_user_id — falling back to direct Stripe path", grofloUserID)
		return
	}

	go func() {
		body := map[string]interface{}{
			"user_id":         timpagerUserID.String,
			"amount_cents":    amountCents,
			"currency":        "USD",
			"distribution_id": distributionID,
			"idempotency_key": idempotencyKey,
		}
		jsonBody, _ := json.Marshal(body)

		for attempt := 0; attempt < 3; attempt++ {
			req, err := http.NewRequest("POST",
				s.config.TimpagerURL+"/api/marketplaces/penny/payouts",
				bytes.NewReader(jsonBody),
			)
			if err != nil {
				log.Printf("[timpager-payout] new request failed: %v", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+s.config.TimpagerAPIKey)

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("[timpager-payout] attempt %d failed: %v", attempt+1, err)
				time.Sleep(time.Duration(1<<attempt) * time.Second)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == 201 || resp.StatusCode == 409 {
				// 201 = created, 409 = already queued (idempotent success). NEVER retry on 409.
				log.Printf("[timpager-payout] pushed user=%s %dc distribution=%s status=%d",
					grofloUserID, amountCents, distributionID, resp.StatusCode)
				return
			}
			log.Printf("[timpager-payout] attempt %d got status %d", attempt+1, resp.StatusCode)
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
		log.Printf("[timpager-payout] failed after 3 attempts: user=%s %dc distribution=%s",
			grofloUserID, amountCents, distributionID)
	}()
}

