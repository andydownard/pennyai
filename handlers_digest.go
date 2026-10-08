package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// =============================================================================
// F8 PHASE 2: DAILY DIGEST SMS
// =============================================================================

// runDailyDigestLoop runs processDailyDigest once every hour. The digest
// function itself enforces a 20-hour dedup window per user, so running hourly
// is safe and allows flexible delivery windows.
func (s *Server) runDailyDigestLoop() {
	// Initial delay: let the server warm up before first run
	time.Sleep(5 * time.Minute)

	// Run once on startup (after warm-up), then every hour
	s.processDailyDigest()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		s.processDailyDigest()
	}
}

// processDailyDigest sends a daily SMS summary of list changes to opted-in users.
//
// Safety invariants (per CLAUDE.md Notification & Communication Safety):
// - Only sends to users with daily_digest_enabled = 1 AND a verified phone number
// - Rate limited: max 1 SMS per user per 24h via daily_digest_last_sent check (20h dedup window)
// - Skips users with no list changes since last digest
// - Updates daily_digest_last_sent BEFORE send (dedup-first pattern: miss > duplicate)
func (s *Server) processDailyDigest() {
	if s.config.TwilioPhoneNumber == "" {
		log.Printf("[digest] skipping — TWILIO_PHONE_NUMBER not configured")
		return
	}

	// Find all opted-in users with a phone number whose last digest was >20h ago (or never sent)
	rows, err := s.db.Query(`
		SELECT u.id, COALESCE(u.name, ''), u.phone, COALESCE(u.daily_digest_last_sent, '')
		FROM users u
		WHERE u.daily_digest_enabled = 1
		  AND u.phone IS NOT NULL
		  AND u.phone != ''
	`)
	if err != nil {
		log.Printf("[digest] query error: %v", err)
		return
	}
	defer rows.Close()

	type digestUser struct {
		ID        string
		Name      string
		Phone     string
		LastSent  string
	}
	var users []digestUser
	for rows.Next() {
		var u digestUser
		if err := rows.Scan(&u.ID, &u.Name, &u.Phone, &u.LastSent); err != nil {
			log.Printf("[digest] scan error: %v", err)
			continue
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[digest] row iteration error: %v", err)
	}

	now := time.Now().UTC()
	sent := 0
	skipped := 0

	for _, u := range users {
		// Dedup: skip if last sent within 20 hours
		if u.LastSent != "" {
			lastSent, err := parseDBTime(u.LastSent)
			if err == nil && now.Sub(lastSent) < 20*time.Hour {
				skipped++
				continue
			}
		}

		// Get list items added/modified since last sent (or last 24h)
		sinceTime := now.Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
		if u.LastSent != "" {
			sinceTime = u.LastSent
		}

		items := s.getDigestItems(u.ID, sinceTime)
		if len(items) == 0 {
			continue // No changes, skip
		}

		// Build the SMS message
		msg := buildDigestMessage(s.config.AppBaseURL, u.Name, items)

		// SAFETY: Update dedup flag BEFORE sending SMS. If SMS fails, user misses
		// one digest (recoverable). If we sent first and UPDATE failed, user gets
		// duplicate SMS next hour (trust-destroying, per CLAUDE.md hard rule).
		res, err := s.db.Exec(
			"UPDATE users SET daily_digest_last_sent = ? WHERE id = ? AND (daily_digest_last_sent IS NULL OR daily_digest_last_sent < ?)",
			now.Format("2006-01-02 15:04:05"), u.ID, now.Add(-20*time.Hour).Format("2006-01-02 15:04:05"),
		)
		if err != nil {
			log.Printf("[digest] failed to update last_sent for user %s: %v", u.ID, err)
			continue // Don't send if we can't set the dedup guard
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			skipped++ // Another process already claimed this user
			continue
		}

		// Twilio A2P 10DLC consent gate. Daily digest is recurring marketing
		// per CTIA classification — must have an explicit consent record per
		// recipient. Skip silently if user has not (re-)consented under the
		// new Groflo brand. The dedup flag stays set so we won't try again
		// until the next 20-hour cycle.
		if !s.canSendSMS(u.ID) {
			log.Printf("[digest] skipping user %s: no SMS consent on file", u.ID)
			continue
		}

		// Send SMS (dedup guard is already set — safe from duplicates)
		if err := s.twilio.SendSMS(s.config.TwilioPhoneNumber, u.Phone, msg); err != nil {
			log.Printf("[digest] SMS error for user %s (dedup flag set, will retry next cycle): %v", u.ID, err)
			continue
		}

		sent++
	}

	if sent > 0 || skipped > 0 {
		log.Printf("[digest] sent=%d skipped=%d total_eligible=%d", sent, skipped, len(users))
	}
}

// getDigestItems returns list item names added or modified since the given time
// for the user's household (or just the user if no household).
func (s *Server) getDigestItems(userID string, since string) []string {
	hid := getUserHouseholdID(s.db, userID)

	var rows *sql.Rows
	var err error
	if hid != 0 {
		// Household: get items from all household members
		rows, err = s.db.Query(`
			SELECT li.name FROM list_items li
			JOIN users u ON u.id = li.user_id
			WHERE u.household_id = ? AND li.created_at >= ? AND li.checked = 0
			GROUP BY li.name
			ORDER BY MAX(li.created_at) DESC
			LIMIT 20
		`, hid, since)
	} else {
		// Solo user
		rows, err = s.db.Query(`
			SELECT li.name FROM list_items li
			WHERE li.user_id = ? AND li.created_at >= ? AND li.checked = 0
			GROUP BY li.name
			ORDER BY MAX(li.created_at) DESC
			LIMIT 20
		`, userID, since)
	}
	if err != nil {
		log.Printf("[digest] items query error for user %s: %v", userID, err)
		return nil
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			items = append(items, name)
		}
	}
	return items
}

// buildDigestMessage constructs the SMS body for the daily digest.
func buildDigestMessage(appURL, userName string, items []string) string {
	firstName := strings.TrimSpace(strings.SplitN(userName, " ", 2)[0])
	if firstName == "" {
		firstName = "Hey"
	} else {
		firstName = "Hey " + firstName
	}

	count := len(items)
	// Show up to 5 item names, then "and X more"
	shown := items
	moreCount := 0
	if len(shown) > 5 {
		moreCount = len(shown) - 5
		shown = shown[:5]
	}

	var itemList string
	if moreCount > 0 {
		itemList = strings.Join(shown, ", ") + fmt.Sprintf(" and %d more", moreCount)
	} else {
		itemList = strings.Join(shown, ", ")
	}

	noun := "items were"
	if count == 1 {
		noun = "item was"
	}

	return fmt.Sprintf("%s, %d %s added to your list today: %s. Open: %s",
		firstName, count, noun, itemList, appURL)
}
