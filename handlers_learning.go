package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// recordSignal writes a behavioral signal to user_signals for learning.
// Call this from any handler where a user action teaches Penny something.
func (s *Server) recordSignal(userID int, signal, category, value string, ctx map[string]interface{}) {
	ctxJSON, _ := json.Marshal(ctx)
	s.db.Exec(`INSERT INTO user_signals (user_id, signal, category, value, context)
		VALUES (?, ?, ?, ?, ?)`, userID, signal, category, value, string(ctxJSON))
}

// updateLearning upserts a derived insight into penny_learnings.
// scope: "user", "household", "brand", "store", "global"
// confidence: 0.0 to 1.0 — increases with more confirming signals
func (s *Server) updateLearning(scope string, scopeID int, key, value string, confidence float64) {
	s.db.Exec(`INSERT INTO penny_learnings (scope, scope_id, key, value, confidence, updated_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(scope, scope_id, key)
		DO UPDATE SET value = ?, confidence = ?, updated_at = datetime('now')`,
		scope, scopeID, key, value, confidence, value, confidence)
}

// handlePennyInsights returns Penny's accumulated learnings for a user.
// GET /api/penny/insights — returns user-level and household-level insights.
func (s *Server) handlePennyInsights(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Gather user-level learnings
	userLearnings := map[string]interface{}{}
	rows, err := s.db.Query(`SELECT key, value, confidence FROM penny_learnings
		WHERE scope = 'user' AND scope_id = ?`, session.UserID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var k, v string
			var conf float64
			rows.Scan(&k, &v, &conf)
			userLearnings[k] = map[string]interface{}{"value": v, "confidence": conf}
		}
	}

	// Gather household-level learnings
	householdLearnings := map[string]interface{}{}
	var householdID int
	s.db.QueryRow("SELECT COALESCE(household_id, 0) FROM users WHERE id = ?", session.UserID).Scan(&householdID)
	if householdID > 0 {
		rows2, err := s.db.Query(`SELECT key, value, confidence FROM penny_learnings
			WHERE scope = 'household' AND scope_id = ?`, householdID)
		if err == nil {
			defer rows2.Close()
			for rows2.Next() {
				var k, v string
				var conf float64
				rows2.Scan(&k, &v, &conf)
				householdLearnings[k] = map[string]interface{}{"value": v, "confidence": conf}
			}
		}
	}

	// Shopping patterns from recent signals
	var shopDays int
	s.db.QueryRow(`SELECT COUNT(DISTINCT date(created_at)) FROM user_signals
		WHERE user_id = ? AND created_at >= datetime('now', '-30 days')`,
		session.UserID).Scan(&shopDays)

	var topCategories []string
	catRows, err := s.db.Query(`SELECT category, COUNT(*) as c FROM user_signals
		WHERE user_id = ? AND signal = 'item_added' AND created_at >= datetime('now', '-30 days')
		GROUP BY category ORDER BY c DESC LIMIT 5`,
		session.UserID)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var cat string
			var c int
			catRows.Scan(&cat, &c)
			if cat != "" {
				topCategories = append(topCategories, cat)
			}
		}
	}

	// Brand affinities from brand_preferences
	brandAffinities := []map[string]interface{}{}
	bpRows, err := s.db.Query(`SELECT product_category, brand, pick_count, streak
		FROM brand_preferences WHERE user_id = ?
		ORDER BY pick_count DESC LIMIT 10`,
		session.UserID)
	if err == nil {
		defer bpRows.Close()
		for bpRows.Next() {
			var cat, brand string
			var picks, streak int
			bpRows.Scan(&cat, &brand, &picks, &streak)
			brandAffinities = append(brandAffinities, map[string]interface{}{
				"category": cat, "brand": brand, "picks": picks, "streak": streak,
			})
		}
	}

	// Swap behavior stats
	var swapsOffered, swapsAccepted, swapsDeclined int
	s.db.QueryRow(`SELECT COUNT(*) FROM swap_events WHERE user_id = ? AND action = 'offered'`,
		session.UserID).Scan(&swapsOffered)
	s.db.QueryRow(`SELECT COUNT(*) FROM swap_events WHERE user_id = ? AND action = 'accepted'`,
		session.UserID).Scan(&swapsAccepted)
	s.db.QueryRow(`SELECT COUNT(*) FROM swap_events WHERE user_id = ? AND action = 'declined'`,
		session.UserID).Scan(&swapsDeclined)

	swapRate := 0
	if swapsOffered > 0 {
		swapRate = swapsAccepted * 100 / swapsOffered
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_learnings":      userLearnings,
		"household_learnings": householdLearnings,
		"patterns": map[string]interface{}{
			"active_days_30d":  shopDays,
			"top_categories":   topCategories,
			"brand_affinities": brandAffinities,
		},
		"swap_behavior": map[string]interface{}{
			"offered":     swapsOffered,
			"accepted":    swapsAccepted,
			"declined":    swapsDeclined,
			"accept_rate": swapRate,
		},
	})
}

// runLearningLoop is a background goroutine that periodically derives
// insights from raw signals and updates penny_learnings.
// Runs every 6 hours. Processes user shopping patterns, brand affinities,
// household dynamics, and store price trends.
func (s *Server) runLearningLoop() {
	// Initial run after 5 minutes (let server warm up)
	time.Sleep(5 * time.Minute)
	s.processLearnings()

	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		s.processLearnings()
	}
}

// processLearnings derives insights from raw data and updates penny_learnings.
// Uses a collect-then-write pattern to avoid SQLite locking issues (single-conn in-memory DB).
func (s *Server) processLearnings() {
	start := time.Now()

	// Collect all pending writes, then apply them
	type learning struct {
		scope    string
		scopeID  int
		key      string
		value    string
		confidence float64
	}
	var writes []learning

	// 1. User-level: shopping frequency pattern
	type freqRow struct{ uid, activeDays, itemsAdded int }
	var freqData []freqRow
	rows, err := s.db.Query(`
		SELECT u.id,
			COUNT(DISTINCT date(im.created_at)) as active_days,
			COUNT(DISTINCT CASE WHEN im.action = 'added' THEN im.id END) as items_added
		FROM users u
		JOIN item_modifications im ON im.user_id = u.id
		WHERE im.created_at >= datetime('now', '-30 days')
		GROUP BY u.id
	`)
	if err == nil {
		for rows.Next() {
			var r freqRow
			rows.Scan(&r.uid, &r.activeDays, &r.itemsAdded)
			freqData = append(freqData, r)
		}
		rows.Close()
	}
	for _, r := range freqData {
		freq := "occasional"
		if r.activeDays >= 20 {
			freq = "daily"
		} else if r.activeDays >= 10 {
			freq = "frequent"
		} else if r.activeDays >= 4 {
			freq = "weekly"
		}
		writes = append(writes,
			learning{"user", r.uid, "shopping_frequency", freq, clampConfidence(float64(r.activeDays)/30.0, 1.0)},
			learning{"user", r.uid, "items_per_month", fmt.Sprintf("%d", r.itemsAdded), 0.8},
		)
	}

	// 2. User-level: price sensitivity from swap behavior
	type psRow struct{ uid, accepts, declines int; avgDiscount float64 }
	var psData []psRow
	psRows, err := s.db.Query(`
		SELECT se.user_id,
			COUNT(CASE WHEN se.action = 'accepted' THEN 1 END) as accepts,
			COUNT(CASE WHEN se.action = 'declined' THEN 1 END) as declines,
			AVG(CASE WHEN se.action = 'accepted' THEN bd.discount_cents END) as avg_accepted_discount
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE se.created_at >= datetime('now', '-60 days')
		GROUP BY se.user_id
		HAVING accepts + declines >= 3
	`)
	if err == nil {
		for psRows.Next() {
			var r psRow
			psRows.Scan(&r.uid, &r.accepts, &r.declines, &r.avgDiscount)
			psData = append(psData, r)
		}
		psRows.Close()
	}
	for _, r := range psData {
		total := r.accepts + r.declines
		rate := float64(r.accepts) / float64(total)
		sensitivity := "moderate"
		if rate > 0.6 {
			sensitivity = "high"
		} else if rate < 0.2 {
			sensitivity = "low"
		}
		conf := clampConfidence(float64(total)/20.0, 1.0)
		writes = append(writes, learning{"user", r.uid, "price_sensitivity", sensitivity, conf})
		if r.avgDiscount > 0 {
			writes = append(writes, learning{"user", r.uid, "avg_accepted_discount_cents", fmt.Sprintf("%.0f", r.avgDiscount), conf})
		}
	}

	// 3. User-level: preferred shopping day
	type dayRow struct{ uid, c int; day string }
	var dayData []dayRow
	dRows, err := s.db.Query(`
		SELECT user_id,
			CASE cast(strftime('%w', created_at) as integer)
				WHEN 0 THEN 'Sunday' WHEN 1 THEN 'Monday' WHEN 2 THEN 'Tuesday'
				WHEN 3 THEN 'Wednesday' WHEN 4 THEN 'Thursday' WHEN 5 THEN 'Friday'
				WHEN 6 THEN 'Saturday' END as day_name,
			COUNT(*) as c
		FROM item_modifications
		WHERE created_at >= datetime('now', '-30 days') AND action = 'added'
		GROUP BY user_id, day_name
		ORDER BY user_id, c DESC
	`)
	if err == nil {
		for dRows.Next() {
			var r dayRow
			dRows.Scan(&r.uid, &r.day, &r.c)
			dayData = append(dayData, r)
		}
		dRows.Close()
	}
	seen := map[int]bool{}
	for _, r := range dayData {
		if !seen[r.uid] {
			writes = append(writes, learning{"user", r.uid, "preferred_shopping_day", r.day, clampConfidence(float64(r.c)/8.0, 1.0)})
			seen[r.uid] = true
		}
	}

	// 4. Household-level: coordination score
	type hhRow struct{ hhID, members, claims, activeMembers int }
	var hhData []hhRow
	hhRows, err := s.db.Query(`
		SELECT hm.household_id,
			COUNT(DISTINCT hm.user_id) as members,
			COUNT(DISTINCT CASE WHEN im.action = 'claimed' THEN im.id END) as claims,
			COUNT(DISTINCT im.user_id) as active_members
		FROM household_members hm
		LEFT JOIN item_modifications im ON im.user_id = hm.user_id
			AND im.created_at >= datetime('now', '-30 days')
		GROUP BY hm.household_id
	`)
	if err == nil {
		for hhRows.Next() {
			var r hhRow
			hhRows.Scan(&r.hhID, &r.members, &r.claims, &r.activeMembers)
			hhData = append(hhData, r)
		}
		hhRows.Close()
	}
	for _, r := range hhData {
		if r.members > 0 {
			coordScore := "low"
			if r.activeMembers == r.members && r.claims > 0 {
				coordScore = "high"
			} else if r.activeMembers > 1 {
				coordScore = "medium"
			}
			writes = append(writes,
				learning{"household", r.hhID, "coordination_score", coordScore, clampConfidence(float64(r.activeMembers)/float64(r.members), 1.0)},
				learning{"household", r.hhID, "active_members", fmt.Sprintf("%d/%d", r.activeMembers, r.members), 0.9},
			)
		}
	}

	// 5. Brand-level: performance metrics
	type brandRow struct{ brand string; offered, accepted, declined int }
	var brandData []brandRow
	bRows, err := s.db.Query(`
		SELECT bd.offer_brand,
			COUNT(CASE WHEN se.action = 'offered' THEN 1 END) as offered,
			COUNT(CASE WHEN se.action = 'accepted' THEN 1 END) as accepted,
			COUNT(CASE WHEN se.action = 'declined' THEN 1 END) as declined
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE se.created_at >= datetime('now', '-30 days')
		GROUP BY bd.offer_brand
	`)
	if err == nil {
		for bRows.Next() {
			var r brandRow
			bRows.Scan(&r.brand, &r.offered, &r.accepted, &r.declined)
			brandData = append(brandData, r)
		}
		bRows.Close()
	}
	for _, r := range brandData {
		if r.offered == 0 {
			continue
		}
		rate := r.accepted * 100 / r.offered
		conf := clampConfidence(float64(r.offered)/50.0, 1.0)
		writes = append(writes, learning{"brand", 0, "accept_rate:" + strings.ToLower(r.brand), fmt.Sprintf("%d", rate), conf})
		perf := "average"
		if rate > 25 {
			perf = "strong"
		} else if rate < 10 {
			perf = "weak"
		}
		writes = append(writes, learning{"brand", 0, "performance:" + strings.ToLower(r.brand), perf, conf})
	}

	// 6. Store-level: average prices from receipts
	type storeRow struct{ store string; count int; avgTotal float64 }
	var storeData []storeRow
	sRows, err := s.db.Query(`
		SELECT store_name, COUNT(*) as receipt_count,
			AVG(total_cents) as avg_total
		FROM receipts
		WHERE store_name != '' AND created_at >= datetime('now', '-60 days')
		GROUP BY store_name
		HAVING receipt_count >= 2
	`)
	if err == nil {
		for sRows.Next() {
			var r storeRow
			sRows.Scan(&r.store, &r.count, &r.avgTotal)
			storeData = append(storeData, r)
		}
		sRows.Close()
	}
	for _, r := range storeData {
		priceLevel := "moderate"
		if r.avgTotal > 8000 {
			priceLevel = "premium"
		} else if r.avgTotal < 4000 {
			priceLevel = "budget"
		}
		writes = append(writes, learning{"store", 0, "price_level:" + strings.ToLower(r.store), priceLevel, clampConfidence(float64(r.count)/10.0, 1.0)})
	}

	// Apply all collected writes
	for _, w := range writes {
		s.updateLearning(w.scope, w.scopeID, w.key, w.value, w.confidence)
	}

	elapsed := time.Since(start)
	log.Printf("[learning] processed %d learnings in %v", len(writes), elapsed)
}

func clampConfidence(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
