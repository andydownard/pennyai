package main

import (
	"log"
	"net/http"
)

// processSwapPredictions computes per-user acceptance rates by category and brand.
// Uses the collect-then-write pattern to avoid SQLite lock contention.
func (s *Server) processSwapPredictions() {
	// Collect: aggregate swap_events joined with brand_deals for category info
	rows, err := s.db.Query(`
		SELECT se.user_id, COALESCE(bd.category_id, 0), bd.offer_brand, se.action,
		       COALESCE(se.time_to_action_ms, 0)
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE se.action IN ('offered', 'accepted', 'declined')
	`)
	if err != nil {
		log.Printf("[predictions] failed to query swap_events: %v", err)
		return
	}

	type predKey struct {
		userID     string
		categoryID int
		brand      string
	}
	type predStats struct {
		offers          int
		accepts         int
		totalTimeMs     int64
		actionCount     int // offers with time_to_action > 0 (for avg calculation)
	}
	stats := map[predKey]*predStats{}

	for rows.Next() {
		var uid string
		var catID, timeMs int
		var brand, action string
		if err := rows.Scan(&uid, &catID, &brand, &action, &timeMs); err != nil {
			continue
		}
		// Per-category prediction (brand="")
		catKey := predKey{uid, catID, ""}
		if stats[catKey] == nil {
			stats[catKey] = &predStats{}
		}
		// Per-brand prediction (categoryID=0)
		brandKey := predKey{uid, 0, brand}
		if stats[brandKey] == nil {
			stats[brandKey] = &predStats{}
		}

		switch action {
		case "offered":
			stats[catKey].offers++
			stats[brandKey].offers++
		case "accepted":
			stats[catKey].accepts++
			stats[brandKey].accepts++
			if timeMs > 0 {
				stats[catKey].totalTimeMs += int64(timeMs)
				stats[catKey].actionCount++
				stats[brandKey].totalTimeMs += int64(timeMs)
				stats[brandKey].actionCount++
			}
		case "declined":
			if timeMs > 0 {
				stats[catKey].totalTimeMs += int64(timeMs)
				stats[catKey].actionCount++
				stats[brandKey].totalTimeMs += int64(timeMs)
				stats[brandKey].actionCount++
			}
		}
	}
	rows.Close()

	if len(stats) == 0 {
		return
	}

	// Write: upsert predictions
	type prediction struct {
		userID     string
		categoryID int
		brand      string
		rate       float64
		offers     int
		accepts    int
		avgTimeMs  int
		confidence float64
	}
	var preds []prediction
	for k, s := range stats {
		if s.offers < 1 {
			continue
		}
		rate := float64(s.accepts) / float64(s.offers)
		avgTime := 0
		if s.actionCount > 0 {
			avgTime = int(s.totalTimeMs / int64(s.actionCount))
		}
		// Confidence scales with sample size: 5 offers = 0.5, 10+ = 1.0
		conf := clampConfidence(float64(s.offers)/10.0, 1.0)
		preds = append(preds, prediction{k.userID, k.categoryID, k.brand, rate, s.offers, s.accepts, avgTime, conf})
	}

	for _, p := range preds {
		s.db.Exec(`INSERT INTO user_swap_predictions (user_id, category_id, brand, acceptance_rate, total_offers, total_accepts, avg_time_to_action_ms, confidence, computed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
			ON CONFLICT(user_id, category_id, brand)
			DO UPDATE SET acceptance_rate = ?, total_offers = ?, total_accepts = ?, avg_time_to_action_ms = ?, confidence = ?, computed_at = datetime('now')`,
			p.userID, p.categoryID, p.brand, p.rate, p.offers, p.accepts, p.avgTimeMs, p.confidence,
			p.rate, p.offers, p.accepts, p.avgTimeMs, p.confidence)
	}

	log.Printf("[predictions] computed %d swap predictions for %d user-category-brand combos", len(preds), len(stats))
}

// getUserSwapPrediction returns the predicted acceptance rate for a user+deal combo.
// Returns (acceptanceRate, confidence). If no prediction exists, returns (0.5, 0) — neutral.
func (s *Server) getUserSwapPrediction(userID string, categoryID int, brand string) (float64, float64) {
	// Try category-specific prediction first (more targeted)
	var catRate, catConf float64
	catErr := s.db.QueryRow(
		"SELECT acceptance_rate, confidence FROM user_swap_predictions WHERE user_id = ? AND category_id = ? AND brand = ''",
		userID, categoryID,
	).Scan(&catRate, &catConf)

	// Try brand-specific prediction
	var brandRate, brandConf float64
	brandErr := s.db.QueryRow(
		"SELECT acceptance_rate, confidence FROM user_swap_predictions WHERE user_id = ? AND category_id = 0 AND brand = ?",
		userID, brand,
	).Scan(&brandRate, &brandConf)

	// Blend: weighted average by confidence. If neither exists, return neutral.
	if catErr != nil && brandErr != nil {
		return 0.5, 0 // No data — neutral prediction, zero confidence
	}
	if catErr != nil {
		return brandRate, brandConf
	}
	if brandErr != nil {
		return catRate, catConf
	}
	// Both exist: blend by confidence weight
	totalConf := catConf + brandConf
	if totalConf == 0 {
		return 0.5, 0
	}
	blendedRate := (catRate*catConf + brandRate*brandConf) / totalConf
	blendedConf := clampConfidence(totalConf/2, 1.0)
	return blendedRate, blendedConf
}

// handleSwapPredictions returns the admin view of user swap predictions.
// GET /api/admin/swap-predictions — admin only.
func (s *Server) handleSwapPredictions(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT user_id, category_id, brand, acceptance_rate, total_offers, total_accepts,
		       avg_time_to_action_ms, confidence, computed_at
		FROM user_swap_predictions
		WHERE confidence >= 0.3
		ORDER BY user_id, acceptance_rate DESC
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load predictions"})
		return
	}

	type predRow struct {
		UserID     int     `json:"user_id"`
		CategoryID int     `json:"category_id"`
		Brand      string  `json:"brand"`
		Rate       float64 `json:"acceptance_rate"`
		Offers     int     `json:"total_offers"`
		Accepts    int     `json:"total_accepts"`
		AvgTimeMs  int     `json:"avg_time_to_action_ms"`
		Confidence float64 `json:"confidence"`
		ComputedAt string  `json:"computed_at"`
	}
	var results []predRow
	for rows.Next() {
		var p predRow
		if err := rows.Scan(&p.UserID, &p.CategoryID, &p.Brand, &p.Rate, &p.Offers, &p.Accepts, &p.AvgTimeMs, &p.Confidence, &p.ComputedAt); err != nil {
			continue
		}
		results = append(results, p)
	}
	rows.Close()

	writeJSON(w, http.StatusOK, results)
}
