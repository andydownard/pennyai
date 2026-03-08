package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
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

	// 7. User-level: repurchase cycles (how often users re-add the same items)
	type cycleRow struct {
		uid   int
		item  string
		count int
		days  float64 // average days between additions
	}
	var cycleData []cycleRow
	cyRows, err := s.db.Query(`
		SELECT im.user_id, LOWER(li.name) as item_name,
			COUNT(*) as add_count,
			CASE WHEN COUNT(*) > 1
				THEN (julianday(MAX(im.created_at)) - julianday(MIN(im.created_at))) / (COUNT(*) - 1)
				ELSE 0 END as avg_days
		FROM item_modifications im
		JOIN list_items li ON li.id = im.item_id
		WHERE im.action = 'added'
			AND im.created_at >= datetime('now', '-90 days')
		GROUP BY im.user_id, LOWER(li.name)
		HAVING add_count >= 2 AND avg_days > 0
		ORDER BY im.user_id, add_count DESC
	`)
	if err == nil {
		for cyRows.Next() {
			var r cycleRow
			cyRows.Scan(&r.uid, &r.item, &r.count, &r.days)
			cycleData = append(cycleData, r)
		}
		cyRows.Close()
	}
	// Store top 5 repurchase items per user
	cycleUserSeen := map[int]int{}
	for _, r := range cycleData {
		if cycleUserSeen[r.uid] >= 5 {
			continue
		}
		cycleUserSeen[r.uid]++
		writes = append(writes, learning{
			"user", r.uid,
			"repurchase_cycle:" + r.item,
			fmt.Sprintf("%.0f days", r.days),
			clampConfidence(float64(r.count)/5.0, 1.0),
		})
	}

	// 8. User-level: shopping role detection for household members
	// Determines each member's role: primary_shopper, list_builder, or occasional
	// based on items added and items checked off.
	type roleRow struct {
		uid        int
		hhID       int
		totalItems int
		checkedOff int
	}
	var roleData []roleRow
	roleRows, err := s.db.Query(`
		SELECT u.id, COALESCE(u.household_id, 0) as hh_id,
			COUNT(DISTINCT li.id) as total_items,
			COUNT(DISTINCT CASE WHEN li.checked = 1 THEN li.id END) as checked_off
		FROM users u
		JOIN household_members hm ON hm.user_id = u.id
		JOIN list_items li ON li.user_id = u.id
		GROUP BY u.id
	`)
	if err == nil {
		for roleRows.Next() {
			var r roleRow
			roleRows.Scan(&r.uid, &r.hhID, &r.totalItems, &r.checkedOff)
			roleData = append(roleData, r)
		}
		roleRows.Close()
	}

	// Group by household to compute median total_items per household
	hhMembers := map[int][]roleRow{}
	for _, r := range roleData {
		if r.hhID > 0 {
			hhMembers[r.hhID] = append(hhMembers[r.hhID], r)
		}
	}

	for _, members := range hhMembers {
		// Compute median total_items for this household
		totals := make([]int, len(members))
		for i, m := range members {
			totals[i] = m.totalItems
		}
		sort.Ints(totals)
		median := totals[len(totals)/2]

		for _, m := range members {
			var role string
			if m.totalItems <= 5 {
				role = "occasional"
			} else if m.totalItems >= median {
				checkRatio := 0.0
				if m.totalItems > 0 {
					checkRatio = float64(m.checkedOff) / float64(m.totalItems)
				}
				if checkRatio > 0.5 {
					role = "primary_shopper"
				} else if checkRatio < 0.3 {
					role = "list_builder"
				} else {
					role = "list_builder"
				}
			} else {
				// Below median but more than 5 items
				checkRatio := 0.0
				if m.totalItems > 0 {
					checkRatio = float64(m.checkedOff) / float64(m.totalItems)
				}
				if checkRatio < 0.3 {
					role = "list_builder"
				} else {
					role = "occasional"
				}
			}
			// Confidence based on data volume: more items = higher confidence, caps at 1.0
			conf := clampConfidence(float64(m.totalItems)/20.0, 1.0)
			writes = append(writes, learning{"user", m.uid, "shopping_role", role, conf})
		}
	}

	// Apply all collected writes
	for _, w := range writes {
		s.updateLearning(w.scope, w.scopeID, w.key, w.value, w.confidence)
	}

	// Update penny_context_cache for all active users
	userIDs := map[int]bool{}
	for _, w := range writes {
		if w.scope == "user" {
			userIDs[w.scopeID] = true
		}
	}
	for uid := range userIDs {
		ctx := s.buildLearningContext(uid)
		if ctx != "" {
			s.db.Exec(`INSERT INTO penny_context_cache (user_id, context_json, computed_at)
				VALUES (?, ?, datetime('now'))
				ON CONFLICT(user_id) DO UPDATE SET context_json = ?, computed_at = datetime('now')`,
				uid, ctx, ctx)
		}
	}

	elapsed := time.Since(start)
	log.Printf("[learning] processed %d learnings, updated %d context caches in %v", len(writes), len(userIDs), elapsed)
}

func clampConfidence(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// buildLearningContext constructs a structured string of Penny's learned
// preferences for a given user, suitable for injection into Penny's system prompt.
// Returns "" if no learnings exist or "Learn from my choices" is off.
func (s *Server) buildLearningContext(userID int) string {
	// Check if user has autotune enabled (learn from my choices)
	var autotune int
	s.db.QueryRow("SELECT COALESCE(penny_autotune, 0) FROM users WHERE id = ?", userID).Scan(&autotune)
	if autotune == 0 {
		return ""
	}

	var parts []string

	// User-level learnings from penny_learnings
	rows, err := s.db.Query(`SELECT key, value, confidence FROM penny_learnings
		WHERE scope = 'user' AND scope_id = ? AND confidence >= 0.3
		ORDER BY confidence DESC`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var k, v string
			var conf float64
			rows.Scan(&k, &v, &conf)
			switch {
			case k == "shopping_frequency":
				parts = append(parts, fmt.Sprintf("- Shopping frequency: %s (confidence %.0f%%)", v, conf*100))
			case k == "price_sensitivity":
				parts = append(parts, fmt.Sprintf("- Price sensitivity: %s", v))
			case k == "preferred_shopping_day":
				parts = append(parts, fmt.Sprintf("- Usually shops on: %s", v))
			case k == "items_per_month":
				parts = append(parts, fmt.Sprintf("- Adds ~%s items/month", v))
			case k == "avg_accepted_discount_cents":
				parts = append(parts, fmt.Sprintf("- Typical accepted discount: $%.2f", parseFloat(v)/100))
			case k == "shopping_role":
				roleDesc := v
				switch v {
				case "primary_shopper":
					roleDesc = "primary shopper (adds and checks off most items)"
				case "list_builder":
					roleDesc = "list builder (adds items but rarely checks them off)"
				case "occasional":
					roleDesc = "occasional contributor"
				}
				parts = append(parts, fmt.Sprintf("- Household role: %s", roleDesc))
			case strings.HasPrefix(k, "repurchase_cycle:"):
				item := strings.TrimPrefix(k, "repurchase_cycle:")
				parts = append(parts, fmt.Sprintf("- Buys %s every %s", item, v))
			}
		}
	}

	// Brand preferences (top 5 by pick count)
	bpRows, err := s.db.Query(`SELECT product_category, brand, pick_count
		FROM brand_preferences WHERE user_id = ?
		ORDER BY pick_count DESC LIMIT 5`, userID)
	if err == nil {
		defer bpRows.Close()
		var brandPrefs []string
		for bpRows.Next() {
			var cat, brand string
			var picks int
			bpRows.Scan(&cat, &brand, &picks)
			if picks >= 2 {
				brandPrefs = append(brandPrefs, fmt.Sprintf("%s for %s", brand, cat))
			}
		}
		if len(brandPrefs) > 0 {
			parts = append(parts, "- Preferred brands: "+strings.Join(brandPrefs, ", "))
		}
	}

	// Dietary profile if it exists
	var allergens, diets string
	s.db.QueryRow("SELECT COALESCE(allergens, '[]'), COALESCE(diets, '[]') FROM user_dietary_profiles WHERE user_id = ?", userID).Scan(&allergens, &diets)
	if allergens != "[]" && allergens != "" {
		parts = append(parts, fmt.Sprintf("- Allergens: %s — NEVER suggest products containing these", allergens))
	}
	if diets != "[]" && diets != "" {
		parts = append(parts, fmt.Sprintf("- Dietary preferences: %s", diets))
	}

	// Penny slider position
	var slider int
	s.db.QueryRow("SELECT COALESCE(penny_slider, 15) FROM users WHERE id = ?", userID).Scan(&slider)
	if slider <= 8 {
		parts = append(parts, "- Strongly prefers savings over convenience — suggest budget options")
	} else if slider >= 22 {
		parts = append(parts, "- Strongly prefers convenience over savings — suggest premium/quality options")
	}

	// Household context
	var hhID int
	s.db.QueryRow("SELECT COALESCE(household_id, 0) FROM users WHERE id = ?", userID).Scan(&hhID)
	if hhID > 0 {
		var memberCount int
		s.db.QueryRow("SELECT COUNT(*) FROM household_members WHERE household_id = ?", hhID).Scan(&memberCount)
		if memberCount > 1 {
			parts = append(parts, fmt.Sprintf("- Part of a %d-person household with a shared list", memberCount))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return "What Penny knows about this user (use this context to personalize suggestions — never reference these facts directly unless relevant):\n" + strings.Join(parts, "\n")
}

// RepurchaseNudge represents a single "time to restock" nudge for an item
// whose repurchase cycle has elapsed since the user last added it.
type RepurchaseNudge struct {
	ItemName    string `json:"item_name"`
	CycleDays   int    `json:"cycle_days"`
	DaysOverdue int    `json:"days_overdue"`
	LastAdded   string `json:"last_added"`
}

// getRepurchaseNudges queries penny_learnings for repurchase_cycle insights
// and checks if any items are "due" (last added + cycle days <= now).
// Uses collect-then-write pattern for SQLite (MaxOpenConns=1).
func (s *Server) getRepurchaseNudges(userID int) []RepurchaseNudge {
	// Step 1: Collect all repurchase_cycle learnings for this user
	type cycleInfo struct {
		itemName string
		avgDays  int
	}
	var cycles []cycleInfo

	rows, err := s.db.Query(`SELECT key, value FROM penny_learnings
		WHERE scope = 'user' AND scope_id = ? AND key LIKE 'repurchase_cycle:%'`,
		userID)
	if err != nil {
		return nil
	}
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		itemName := strings.TrimPrefix(k, "repurchase_cycle:")
		if itemName == "" {
			continue
		}
		var days float64
		fmt.Sscanf(v, "%f", &days)
		if days <= 0 {
			continue
		}
		cycles = append(cycles, cycleInfo{itemName: itemName, avgDays: int(days)})
	}
	rows.Close()

	if len(cycles) == 0 {
		return nil
	}

	// Step 2: For each cycle item, find the most recent time it was added
	var nudges []RepurchaseNudge
	now := time.Now()

	for _, c := range cycles {
		var lastAddedStr string
		err := s.db.QueryRow(`
			SELECT MAX(im.created_at)
			FROM item_modifications im
			JOIN list_items li ON li.id = im.item_id
			WHERE im.user_id = ? AND im.action = 'added' AND LOWER(li.name) = LOWER(?)
		`, userID, c.itemName).Scan(&lastAddedStr)
		if err != nil || lastAddedStr == "" {
			continue
		}

		// Parse the last added timestamp
		lastAdded, err := time.Parse("2006-01-02 15:04:05", lastAddedStr)
		if err != nil {
			// Try alternate format
			lastAdded, err = time.Parse("2006-01-02T15:04:05Z", lastAddedStr)
			if err != nil {
				continue
			}
		}

		// Check if the item is due: current_date - last_added_date >= avg_days
		daysSinceAdded := int(now.Sub(lastAdded).Hours() / 24)
		daysOverdue := daysSinceAdded - c.avgDays
		if daysOverdue >= 0 {
			// Also check the item isn't currently on the list (unchecked)
			var onList int
			s.db.QueryRow(`SELECT COUNT(*) FROM list_items
				WHERE user_id = ? AND LOWER(name) = LOWER(?) AND checked = 0`,
				userID, c.itemName).Scan(&onList)
			if onList > 0 {
				continue // already on the list, no need to nudge
			}

			nudges = append(nudges, RepurchaseNudge{
				ItemName:    c.itemName,
				CycleDays:   c.avgDays,
				DaysOverdue: daysOverdue,
				LastAdded:   lastAdded.Format("2006-01-02"),
			})
		}
	}

	return nudges
}

// handleRepurchaseNudges returns pending repurchase nudges for the logged-in user.
// GET /api/penny/nudges
func (s *Server) handleRepurchaseNudges(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	nudges := s.getRepurchaseNudges(session.UserID)
	if nudges == nil {
		nudges = []RepurchaseNudge{} // return empty array, not null
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"nudges": nudges,
	})
}

// parseFloat is a simple helper for converting string to float64.
func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// getHouseholdAllergens returns the union of all allergens across all members
// of the user's household. Used for safety filtering — no swap should suggest
// a product containing any household member's allergen.
func (s *Server) getHouseholdAllergens(userID int) []string {
	var hhID int
	s.db.QueryRow("SELECT COALESCE(household_id, 0) FROM users WHERE id = ?", userID).Scan(&hhID)
	if hhID == 0 {
		// No household — just return this user's allergens
		var allergensJSON string
		if err := s.db.QueryRow("SELECT COALESCE(allergens, '[]') FROM user_dietary_profiles WHERE user_id = ?", userID).Scan(&allergensJSON); err == nil {
			var allergens []string
			json.Unmarshal([]byte(allergensJSON), &allergens)
			return allergens
		}
		return nil
	}

	// Union all household members' allergens
	allergenSet := map[string]bool{}
	rows, err := s.db.Query(`
		SELECT COALESCE(dp.allergens, '[]')
		FROM household_members hm
		LEFT JOIN user_dietary_profiles dp ON dp.user_id = hm.user_id
		WHERE hm.household_id = ?
	`, hhID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var aJSON string
		rows.Scan(&aJSON)
		var memberAllergens []string
		json.Unmarshal([]byte(aJSON), &memberAllergens)
		for _, a := range memberAllergens {
			allergenSet[strings.ToLower(a)] = true
		}
	}

	result := make([]string, 0, len(allergenSet))
	for a := range allergenSet {
		result = append(result, a)
	}
	return result
}

// handleHouseholdDietary returns the merged dietary profile for the user's household.
// GET /api/household/dietary
func (s *Server) handleHouseholdDietary(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var hhID int
	s.db.QueryRow("SELECT COALESCE(household_id, 0) FROM users WHERE id = ?", session.UserID).Scan(&hhID)
	if hhID == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"household":    false,
			"all_allergens": []string{},
			"all_diets":    []string{},
			"members":      []interface{}{},
		})
		return
	}

	// Collect each member's dietary profile
	type memberDiet struct {
		Name      string   `json:"name"`
		Allergens []string `json:"allergens"`
		Diets     []string `json:"diets"`
	}
	var members []memberDiet
	allergenSet := map[string]bool{}
	dietSet := map[string]bool{}

	rows, err := s.db.Query(`
		SELECT COALESCE(u.name, 'Member'), COALESCE(dp.allergens, '[]'), COALESCE(dp.diets, '[]')
		FROM household_members hm
		JOIN users u ON u.id = hm.user_id
		LEFT JOIN user_dietary_profiles dp ON dp.user_id = hm.user_id
		WHERE hm.household_id = ?
	`, hhID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name, aJSON, dJSON string
			rows.Scan(&name, &aJSON, &dJSON)
			var allergens, diets []string
			json.Unmarshal([]byte(aJSON), &allergens)
			json.Unmarshal([]byte(dJSON), &diets)
			firstName := strings.Split(strings.TrimSpace(name), " ")[0]
			members = append(members, memberDiet{Name: firstName, Allergens: allergens, Diets: diets})
			for _, a := range allergens {
				allergenSet[a] = true
			}
			for _, d := range diets {
				dietSet[d] = true
			}
		}
	}

	allAllergens := make([]string, 0, len(allergenSet))
	for a := range allergenSet {
		allAllergens = append(allAllergens, a)
	}
	allDiets := make([]string, 0, len(dietSet))
	for d := range dietSet {
		allDiets = append(allDiets, d)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"household":     true,
		"all_allergens": allAllergens,
		"all_diets":     allDiets,
		"members":       members,
	})
}

// checkHouseholdDuplicates checks if another household member added the same
// items within the last 4 hours. Returns warnings for the frontend.
func (s *Server) checkHouseholdDuplicates(userID int, addedItems []ListItem) []DuplicateWarning {
	var hhID int
	s.db.QueryRow("SELECT COALESCE(household_id, 0) FROM users WHERE id = ?", userID).Scan(&hhID)
	if hhID == 0 {
		return nil
	}

	var warnings []DuplicateWarning
	for _, item := range addedItems {
		var otherName string
		var hoursAgo int
		err := s.db.QueryRow(`
			SELECT COALESCE(u.name, 'Someone'),
				CAST((julianday('now') - julianday(im.created_at)) * 24 AS INTEGER) as hours_ago
			FROM item_modifications im
			JOIN list_items li ON li.id = im.item_id
			JOIN users u ON u.id = im.user_id
			JOIN household_members hm ON hm.user_id = im.user_id
			WHERE hm.household_id = ?
				AND im.user_id != ?
				AND im.action = 'added'
				AND LOWER(li.name) = LOWER(?)
				AND im.created_at >= datetime('now', '-4 hours')
			ORDER BY im.created_at DESC LIMIT 1
		`, hhID, userID, item.Name).Scan(&otherName, &hoursAgo)
		if err == nil {
			firstName := strings.Split(strings.TrimSpace(otherName), " ")[0]
			warnings = append(warnings, DuplicateWarning{
				ItemName: item.Name,
				AddedBy:  firstName,
				HoursAgo: hoursAgo,
			})
		}
	}
	return warnings
}
