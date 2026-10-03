package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

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
			// Explicit request, not client.Post: this endpoint needs the
			// Authorization header. The helper already refuses to run without
			// TimpagerAPIKey, but it never actually sent the key, so every event
			// timpager received was unauthenticated. Mirrors pushPayoutToTimpager.
			req, err := http.NewRequest("POST",
				s.config.TimpagerURL+"/api/marketplaces/penny/revenue-events",
				bytes.NewReader(jsonBody),
			)
			if err != nil {
				log.Printf("[timpager-revenue] new request failed: %v", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+s.config.TimpagerAPIKey)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
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

