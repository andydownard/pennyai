package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// buildLearningContext constructs a structured string of Penny's learned
// preferences for a given user, suitable for injection into Penny's system prompt.
// Returns "" if no learnings exist or "Learn from my choices" is off.
func (s *Server) buildLearningContext(userID string) string {
	return s.learningContext(userID, true)
}

// rebuildLearningContext computes the context from the learnings tables,
// ignoring penny_context_cache. The background job that refreshes the cache
// must use this: reading through the cache would return the old value forever.
func (s *Server) rebuildLearningContext(userID string) string {
	return s.learningContext(userID, false)
}

// encodeContextCache and decodeContextCache store the context string as a
// JSON string. context_json is JSONB on PostgreSQL, which rejects plain text;
// decode also accepts a plain string written before this change.
func encodeContextCache(ctx string) string {
	b, _ := json.Marshal(ctx)
	return string(b)
}

func decodeContextCache(raw string) string {
	var out string
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		return out
	}
	return raw
}

func (s *Server) learningContext(userID string, useCache bool) string {
	// Single user lookup for the three settings used below: the autotune gate
	// (learn from my choices), the Penny slider, and the household id. Collapsed
	// from three separate "SELECT ... FROM users WHERE id = ?" round trips.
	var autotune, slider, hhID int
	s.db.QueryRow(`SELECT COALESCE(penny_autotune, 0), COALESCE(penny_slider, 15), COALESCE(household_id, 0)
		FROM users WHERE id = ?`, userID).Scan(&autotune, &slider, &hhID)
	if autotune == 0 {
		return ""
	}

	// Fast path: the background learning job precomputes this exact context
	// string into penny_context_cache. On a hit, skip the ~7-query rebuild.
	// On a miss (or read error), fall back to the rebuild below, unchanged.
	if useCache {
		var cached string
		if err := s.db.QueryRow(
			"SELECT context_json FROM penny_context_cache WHERE user_id = ?", userID,
		).Scan(&cached); err == nil {
			if ctx := decodeContextCache(cached); ctx != "" && ctx != "{}" {
				return ctx
			}
		}
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

	// Penny slider position (fetched in the combined user lookup above)
	if slider <= 8 {
		parts = append(parts, "- Strongly prefers savings over convenience — suggest budget options")
	} else if slider >= 22 {
		parts = append(parts, "- Strongly prefers convenience over savings — suggest premium/quality options")
	}

	// Household context (household id fetched in the combined user lookup above)
	if hhID > 0 {
		var memberCount int
		s.db.QueryRow("SELECT COUNT(*) FROM household_members WHERE household_id = ?", hhID).Scan(&memberCount)
		if memberCount > 1 {
			parts = append(parts, fmt.Sprintf("- Part of a %d-person household with a shared list", memberCount))
		}
	}

	// Pending repurchase nudges — items due for restock
	nudges := s.getRepurchaseNudges(userID)
	if len(nudges) > 0 {
		var nudgeItems []string
		for _, n := range nudges {
			nudgeItems = append(nudgeItems, fmt.Sprintf("%s (%d days overdue)", n.ItemName, n.DaysOverdue))
		}
		parts = append(parts, "- Items due for restock: "+strings.Join(nudgeItems, ", ")+" — gently remind the user if relevant")
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
// Uses collect-then-write pattern (no rows cursor held open during writes).
func (s *Server) getRepurchaseNudges(userID string) []RepurchaseNudge {
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
		lastAdded, err := parseDBTime(lastAddedStr)
		if err != nil {
			continue
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
func (s *Server) getHouseholdAllergens(userID string) []string {
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
func (s *Server) checkHouseholdDuplicates(userID string, addedItems []ListItem) []DuplicateWarning {
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

// processSwapPropagations detects organic brand adoption within households.
// When user A accepts a swap for brand X, and later user B in the same household
// adds brand X items manually (not via swap), that's organic propagation.
// Uses collect-then-write pattern (no rows cursor held open during writes).
func (s *Server) processSwapPropagations() {
	// Step 1: Collect all accepted swaps with household context
	type acceptedSwap struct {
		dealID     int
		userID     string
		hhID       int
		offerBrand string
		acceptedAt string
	}
	var swaps []acceptedSwap

	rows, err := s.db.Query(`
		SELECT se.deal_id, se.user_id, COALESCE(u.household_id, 0),
		       bd.offer_brand, se.created_at
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		JOIN users u ON u.id = se.user_id
		WHERE se.action = 'accepted' AND COALESCE(u.household_id, 0) > 0
	`)
	if err != nil {
		log.Printf("[propagation] failed to query accepted swaps: %v", err)
		return
	}
	for rows.Next() {
		var sw acceptedSwap
		if err := rows.Scan(&sw.dealID, &sw.userID, &sw.hhID, &sw.offerBrand, &sw.acceptedAt); err != nil {
			continue
		}
		swaps = append(swaps, sw)
	}
	rows.Close()

	if len(swaps) == 0 {
		return
	}

	// Step 2: For each accepted swap, check if other household members
	// added items with the same brand (case-insensitive) via manual/sms/email sources
	type propagation struct {
		dealID     int
		sourceUID  string
		adopterUID string
		hhID       int
		itemName  string
		brand     string
	}
	var props []propagation

	for _, sw := range swaps {
		// Find household members who added items with this brand after the swap
		memberRows, err := s.db.Query(`
			SELECT hm.user_id, li.name, li.brand
			FROM household_members hm
			JOIN list_items li ON li.user_id = hm.user_id
			WHERE hm.household_id = ?
			  AND hm.user_id != ?
			  AND LOWER(li.brand) = LOWER(?)
			  AND li.source IN ('manual', 'sms', 'email', 'app')
			  AND li.created_at > ?
		`, sw.hhID, sw.userID, sw.offerBrand, sw.acceptedAt)
		if err != nil {
			continue
		}
		for memberRows.Next() {
			var adopterUID string
			var itemName, brand string
			if err := memberRows.Scan(&adopterUID, &itemName, &brand); err != nil {
				continue
			}
			props = append(props, propagation{
				dealID:    sw.dealID,
				sourceUID: sw.userID,
				adopterUID: adopterUID,
				hhID:      sw.hhID,
				itemName:  itemName,
				brand:     brand,
			})
		}
		memberRows.Close()
	}

	if len(props) == 0 {
		return
	}

	// Step 3: Write propagation records (idempotent via UNIQUE constraint)
	inserted := 0
	for _, p := range props {
		result, err := s.db.Exec(`
			INSERT INTO swap_propagations (deal_id, source_user_id, adopter_user_id, household_id, item_name, brand)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (deal_id, adopter_user_id) DO NOTHING
		`, p.dealID, p.sourceUID, p.adopterUID, p.hhID, p.itemName, p.brand)
		if err == nil {
			if n, _ := result.RowsAffected(); n > 0 {
				inserted++
			}
		}
	}

	if inserted > 0 {
		log.Printf("[propagation] detected %d new organic swap propagations from %d candidates", inserted, len(props))
	}
}

// handleSwapPropagations returns recent organic swap propagation records.
// GET /api/admin/swap-propagations — admin only.
func (s *Server) handleSwapPropagations(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT sp.deal_id, bd.offer_brand, bd.offer_product,
		       COALESCE(su.name, ''), COALESCE(au.name, ''),
		       sp.household_id, sp.item_name, sp.created_at
		FROM swap_propagations sp
		JOIN brand_deals bd ON bd.id = sp.deal_id
		JOIN users su ON su.id = sp.source_user_id
		JOIN users au ON au.id = sp.adopter_user_id
		ORDER BY sp.created_at DESC
		LIMIT 100
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load propagations"})
		return
	}

	type propRow struct {
		DealID          int    `json:"deal_id"`
		OfferBrand      string `json:"offer_brand"`
		OfferProduct    string `json:"offer_product"`
		SourceUserName  string `json:"source_user_name"`
		AdopterUserName string `json:"adopter_user_name"`
		HouseholdID     int    `json:"household_id"`
		ItemName        string `json:"item_name"`
		CreatedAt       string `json:"created_at"`
	}
	var results []propRow
	for rows.Next() {
		var p propRow
		if err := rows.Scan(&p.DealID, &p.OfferBrand, &p.OfferProduct, &p.SourceUserName, &p.AdopterUserName, &p.HouseholdID, &p.ItemName, &p.CreatedAt); err != nil {
			continue
		}
		results = append(results, p)
	}
	rows.Close()

	if results == nil {
		results = []propRow{}
	}

	writeJSON(w, http.StatusOK, results)
}
