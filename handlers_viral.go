package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"
)

// handleWrapped returns a "Thriftly Wrapped" monthly savings summary card.
// GET /api/wrapped?month=2026-03 (defaults to current month)
func (s *Server) handleWrapped(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Determine month range
	now := time.Now()
	year, month := now.Year(), now.Month()
	if m := r.URL.Query().Get("month"); m != "" {
		if t, err := time.Parse("2006-01", m); err == nil {
			year, month = t.Year(), t.Month()
		}
	}

	startOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	endOfMonth := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	monthLabel := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).Format("January 2006")

	// Total savings this month
	var totalSavingsCents int
	s.db.QueryRow(`
		SELECT COALESCE(SUM(amount_cents), 0) FROM swap_rebates
		WHERE user_id = ? AND created_at >= ? AND created_at < ?`,
		session.UserID, startOfMonth, endOfMonth,
	).Scan(&totalSavingsCents)

	// Swaps accepted this month
	var swapsCount int
	s.db.QueryRow(`
		SELECT COUNT(*) FROM swap_events
		WHERE user_id = ? AND action = 'accepted' AND created_at >= ? AND created_at < ?`,
		session.UserID, startOfMonth, endOfMonth,
	).Scan(&swapsCount)

	// Items added this month
	var itemsAdded int
	s.db.QueryRow(`
		SELECT COUNT(*) FROM item_modifications
		WHERE user_id = ? AND action = 'added' AND created_at >= ? AND created_at < ?`,
		session.UserID, startOfMonth, endOfMonth,
	).Scan(&itemsAdded)

	// Top swap brand (most accepted swaps)
	var topSwapBrand string
	s.db.QueryRow(`
		SELECT bd.offer_brand FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE se.user_id = ? AND se.action = 'accepted' AND se.created_at >= ? AND se.created_at < ?
		GROUP BY bd.offer_brand
		ORDER BY COUNT(*) DESC LIMIT 1`,
		session.UserID, startOfMonth, endOfMonth,
	).Scan(&topSwapBrand)

	// Top department (most items added)
	var topCategory string
	s.db.QueryRow(`
		SELECT COALESCE(li.department, 'Grocery') FROM item_modifications im
		JOIN list_items li ON li.id = im.item_id
		WHERE im.user_id = ? AND im.action = 'added' AND im.created_at >= ? AND im.created_at < ?
		GROUP BY li.department
		ORDER BY COUNT(*) DESC LIMIT 1`,
		session.UserID, startOfMonth, endOfMonth,
	).Scan(&topCategory)

	// Streak: consecutive days with any activity in this month
	streakDays := 0
	dayRows, err := s.db.Query(`
		SELECT DISTINCT date(created_at) as d FROM item_modifications
		WHERE user_id = ? AND created_at >= ? AND created_at < ?
		ORDER BY d DESC`,
		session.UserID, startOfMonth, endOfMonth,
	)
	if err == nil {
		defer dayRows.Close()
		var prevDate string
		for dayRows.Next() {
			var d string
			dayRows.Scan(&d)
			if prevDate == "" {
				streakDays = 1
				prevDate = d
				continue
			}
			// Check if consecutive (previous day - 1)
			prev, _ := time.Parse("2006-01-02", prevDate)
			curr, _ := time.Parse("2006-01-02", d)
			if prev.Sub(curr).Hours() <= 25 { // ~1 day tolerance
				streakDays++
				prevDate = d
			} else {
				break
			}
		}
	}

	// Build share text
	savingsStr := fmt.Sprintf("$%d.%02d", totalSavingsCents/100, totalSavingsCents%100)
	shareText := fmt.Sprintf("I saved %s with Thriftly in %s!", savingsStr, monthLabel)
	if swapsCount > 0 {
		shareText += fmt.Sprintf(" %d smart swaps 🔄", swapsCount)
	}
	if streakDays >= 7 {
		shareText += fmt.Sprintf(" %d-day streak 🔥", streakDays)
	}

	// User's name for the card
	var userName string
	s.db.QueryRow("SELECT COALESCE(name, '') FROM users WHERE id = ?", session.UserID).Scan(&userName)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"month":               monthLabel,
		"total_savings_cents": totalSavingsCents,
		"swaps_count":         swapsCount,
		"items_added":         itemsAdded,
		"top_swap_brand":      topSwapBrand,
		"top_category":        topCategory,
		"streak_days":         streakDays,
		"share_text":          shareText,
		"user_name":           userName,
	})
}

// handleAdminBackup handles POST /api/admin/backup.
// Creates a VACUUM INTO clone of the database for reporting/disaster recovery.
func (s *Server) handleAdminBackup(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	reportPath := s.config.DBPath + ".reporting"
	if rp := s.config.ReportingDBPath; rp != "" {
		reportPath = rp
	}

	start := time.Now()

	// WAL checkpoint first to ensure all data is flushed
	if _, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		log.Printf("[backup] WAL checkpoint failed: %v", err)
		// Continue anyway — VACUUM INTO will still work
	}

	// VACUUM INTO creates a complete, consistent copy of the database
	if _, err := s.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", reportPath)); err != nil {
		log.Printf("[backup] VACUUM INTO failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Backup failed: " + err.Error()})
		return
	}

	elapsed := time.Since(start)
	log.Printf("[backup] VACUUM INTO %s completed in %v", reportPath, elapsed)

	// Reopen reporting DB for admin queries
	if s.reportDB != nil {
		s.reportDB.Close()
	}
	rdb, err := openReportDB(reportPath)
	if err != nil {
		log.Printf("[backup] failed to open reporting DB: %v", err)
	} else {
		s.reportDB = rdb
		log.Printf("[backup] reporting DB opened at %s", reportPath)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"path":        reportPath,
		"elapsed_ms":  elapsed.Milliseconds(),
		"report_db":   s.reportDB != nil,
	})
}

// openReportDB opens a read-only SQLite connection for the reporting clone.
func openReportDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2) // read-only, low concurrency
	db.Exec("PRAGMA query_only = ON")
	return db, nil
}

// adminDB returns the reporting DB if available, otherwise the primary DB.
// This lets admin analytics run against the clone without impacting production.
func (s *Server) adminDB() *sql.DB {
	if s.reportDB != nil {
		return s.reportDB
	}
	return s.db
}

// Savings milestone thresholds in cents
var milestoneThresholds = []int{1000, 2500, 5000, 10000, 25000, 50000} // $10, $25, $50, $100, $250, $500

// handleMilestoneCheck returns the highest unacknowledged savings milestone.
// GET /api/milestones/check
func (s *Server) handleMilestoneCheck(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Get total lifetime savings
	var totalCents int
	s.db.QueryRow(`SELECT COALESCE(SUM(amount_cents), 0) FROM swap_rebates WHERE user_id = ?`,
		session.UserID).Scan(&totalCents)

	// Find highest reached but unacked milestone
	var unacked struct {
		cents int
		found bool
	}
	for i := len(milestoneThresholds) - 1; i >= 0; i-- {
		threshold := milestoneThresholds[i]
		if totalCents < threshold {
			continue
		}
		// Insert if not exists (reached but never recorded)
		s.db.Exec(`INSERT OR IGNORE INTO savings_milestones (user_id, cents) VALUES (?, ?)`,
			session.UserID, threshold)

		// Check if unacked
		var ackedAt sql.NullString
		err := s.db.QueryRow(`SELECT acked_at FROM savings_milestones WHERE user_id = ? AND cents = ?`,
			session.UserID, threshold).Scan(&ackedAt)
		if err == nil && !ackedAt.Valid {
			unacked.cents = threshold
			unacked.found = true
			break
		}
	}

	if !unacked.found {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"milestone": nil,
		})
		return
	}

	dollars := fmt.Sprintf("$%d", unacked.cents/100)
	if unacked.cents%100 != 0 {
		dollars = fmt.Sprintf("$%d.%02d", unacked.cents/100, unacked.cents%100)
	}

	var userName string
	s.db.QueryRow("SELECT COALESCE(name, '') FROM users WHERE id = ?", session.UserID).Scan(&userName)

	shareText := fmt.Sprintf("I just hit %s in savings on Thriftly! Brands pay me to try their products.", dollars)
	if userName != "" {
		shareText = fmt.Sprintf("%s just hit %s in savings on Thriftly! Brands pay you to try their products.", userName, dollars)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"milestone": map[string]interface{}{
			"cents":      unacked.cents,
			"dollars":    dollars,
			"share_text": shareText + "\n\nGet Thriftly: https://app.thrift.ly",
			"total_cents": totalCents,
		},
	})
}

// handleMilestoneAck marks a savings milestone as acknowledged.
// POST /api/milestones/ack  { "cents": 1000 }
func (s *Server) handleMilestoneAck(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var body struct {
		Cents int `json:"cents"`
	}
	if !decodeJSON(w, r, &body) || body.Cents <= 0 {
		if body.Cents <= 0 {
			writeBadRequest(w, "Invalid milestone")
		}
		return
	}

	// Ensure the milestone record exists and mark acked
	s.db.Exec(`INSERT OR IGNORE INTO savings_milestones (user_id, cents) VALUES (?, ?)`,
		session.UserID, body.Cents)
	s.db.Exec(`UPDATE savings_milestones SET acked_at = datetime('now') WHERE user_id = ? AND cents = ?`,
		session.UserID, body.Cents)

	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}
