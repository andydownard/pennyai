package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// DAILY METRICS AGGREGATION
// =============================================================================

// processDailyMetrics computes deal_performance_daily, category_swap_insights,
// and business_metrics_daily for the given date string (YYYY-MM-DD format).
// Uses the collect-then-write pattern: read all data into slices, close rows,
// then write. This is CRITICAL for SQLite with MaxOpenConns=1.
func (s *Server) processDailyMetrics(dateStr string) {
	start := time.Now()

	// =========================================================================
	// 1. Deal performance daily — aggregate swap_events grouped by deal_id
	// =========================================================================
	type dealPerf struct {
		dealID      int
		impressions int
		accepts     int
		declines    int
	}
	var dealPerfData []dealPerf

	rows, err := s.db.Query(`
		SELECT deal_id,
			COUNT(CASE WHEN action = 'offered' THEN 1 END) as impressions,
			COUNT(CASE WHEN action = 'accepted' THEN 1 END) as accepts,
			COUNT(CASE WHEN action = 'declined' THEN 1 END) as declines
		FROM swap_events
		WHERE DATE(created_at) = ?
		GROUP BY deal_id
	`, dateStr)
	if err != nil {
		log.Printf("[metrics] failed to query swap_events for %s: %v", dateStr, err)
	} else {
		for rows.Next() {
			var dp dealPerf
			if err := rows.Scan(&dp.dealID, &dp.impressions, &dp.accepts, &dp.declines); err != nil {
				log.Printf("[metrics] scan deal perf: %v", err)
				continue
			}
			dealPerfData = append(dealPerfData, dp)
		}
		rows.Close()
	}

	// Collect revenue per deal from brand_transactions (spend events for that date)
	type dealRevenue struct {
		dealID      int
		revenueCents int
	}
	revenueMap := map[int]int{}
	revRows, err := s.db.Query(`
		SELECT bd.id, COALESCE(SUM(bt.amount_cents), 0)
		FROM brand_deals bd
		JOIN brand_transactions bt ON bt.brand_user_id = bd.brand_user_id
		WHERE DATE(bt.created_at) = ? AND bt.type = 'spend'
		GROUP BY bd.id
	`, dateStr)
	if err != nil {
		log.Printf("[metrics] failed to query brand_transactions for revenue: %v", err)
	} else {
		for revRows.Next() {
			var dealID, cents int
			if revRows.Scan(&dealID, &cents) == nil {
				revenueMap[dealID] = cents
			}
		}
		revRows.Close()
	}

	// Collect verified purchase counts from swap_claims
	verifiedMap := map[int]int{}
	verRows, err := s.db.Query(`
		SELECT deal_id, COUNT(*)
		FROM swap_claims
		WHERE status = 'verified' AND DATE(verified_at) = ?
		GROUP BY deal_id
	`, dateStr)
	if err != nil {
		log.Printf("[metrics] failed to query swap_claims: %v", err)
	} else {
		for verRows.Next() {
			var dealID, cnt int
			if verRows.Scan(&dealID, &cnt) == nil {
				verifiedMap[dealID] = cnt
			}
		}
		verRows.Close()
	}

	// Write deal_performance_daily rows
	for _, dp := range dealPerfData {
		_, err := s.db.Exec(`
			INSERT INTO deal_performance_daily (deal_id, date, impressions, accepts, declines, revenue_cents, verified_count)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(deal_id, date) DO UPDATE SET
				impressions = excluded.impressions,
				accepts = excluded.accepts,
				declines = excluded.declines,
				revenue_cents = excluded.revenue_cents,
				verified_count = excluded.verified_count
		`, dp.dealID, dateStr, dp.impressions, dp.accepts, dp.declines, revenueMap[dp.dealID], verifiedMap[dp.dealID])
		if err != nil {
			log.Printf("[metrics] failed to upsert deal_performance_daily for deal %d on %s: %v", dp.dealID, dateStr, err)
		}
	}

	// =========================================================================
	// 2. Category swap insights — aggregate across deals by competitor_product
	// =========================================================================
	type catInsight struct {
		category  string
		offered   int
		accepted  int
		avgDisc   float64
	}
	var catData []catInsight

	catRows, err := s.db.Query(`
		SELECT LOWER(bd.competitor_product) as category,
			COUNT(CASE WHEN se.action = 'offered' THEN 1 END) as offered,
			COUNT(CASE WHEN se.action = 'accepted' THEN 1 END) as accepted,
			AVG(bd.discount_cents) as avg_discount
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE bd.status != 'deleted'
		GROUP BY LOWER(bd.competitor_product)
	`)
	if err != nil {
		log.Printf("[metrics] failed to query category insights: %v", err)
	} else {
		for catRows.Next() {
			var ci catInsight
			if err := catRows.Scan(&ci.category, &ci.offered, &ci.accepted, &ci.avgDisc); err != nil {
				log.Printf("[metrics] scan category insight: %v", err)
				continue
			}
			catData = append(catData, ci)
		}
		catRows.Close()
	}

	// Write category_swap_insights
	for _, ci := range catData {
		acceptRate := 0.0
		if ci.offered > 0 {
			acceptRate = float64(ci.accepted) / float64(ci.offered) * 100.0
		}
		totalSwaps := ci.offered
		_, err := s.db.Exec(`
			INSERT INTO category_swap_insights (category, avg_acceptance_rate, avg_discount_pct, total_swaps, last_computed)
			VALUES (?, ?, ?, ?, datetime('now'))
			ON CONFLICT(category) DO UPDATE SET
				avg_acceptance_rate = excluded.avg_acceptance_rate,
				avg_discount_pct = excluded.avg_discount_pct,
				total_swaps = excluded.total_swaps,
				last_computed = datetime('now')
		`, ci.category, acceptRate, ci.avgDisc, totalSwaps)
		if err != nil {
			log.Printf("[metrics] failed to upsert category_swap_insights for %s: %v", ci.category, err)
		}
	}

	// =========================================================================
	// 3. Business metrics daily — aggregate DAU/WAU/MAU, new users, swap counts
	// =========================================================================

	// DAU: distinct sessions active on dateStr
	var dau int
	s.db.QueryRow(`
		SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE DATE(created_at) <= ? AND DATE(expires_at) >= ?
	`, dateStr, dateStr).Scan(&dau)

	// WAU: distinct sessions active in the 7 days ending on dateStr
	var wau int
	s.db.QueryRow(`
		SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE created_at >= DATE(?, '-6 days') AND DATE(created_at) <= ?
	`, dateStr, dateStr).Scan(&wau)

	// MAU: distinct sessions active in the 30 days ending on dateStr
	var mau int
	s.db.QueryRow(`
		SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE created_at >= DATE(?, '-29 days') AND DATE(created_at) <= ?
	`, dateStr, dateStr).Scan(&mau)

	// New users created on dateStr
	var newUsers int
	s.db.QueryRow("SELECT COUNT(*) FROM users WHERE DATE(created_at) = ?", dateStr).Scan(&newUsers)

	// Swap counts for dateStr
	var swapsOffered, swapsAccepted, swapsDeclined int
	s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE action = 'offered' AND DATE(created_at) = ?", dateStr).Scan(&swapsOffered)
	s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE action = 'accepted' AND DATE(created_at) = ?", dateStr).Scan(&swapsAccepted)
	s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE action = 'declined' AND DATE(created_at) = ?", dateStr).Scan(&swapsDeclined)

	acceptanceRate := 0.0
	if swapsOffered > 0 {
		acceptanceRate = float64(swapsAccepted) / float64(swapsOffered) * 100.0
	}

	// Gross revenue from brand_transactions on dateStr
	var grossRevenueCents int
	s.db.QueryRow(`
		SELECT COALESCE(SUM(amount_cents), 0) FROM brand_transactions
		WHERE type = 'spend' AND DATE(created_at) = ?
	`, dateStr).Scan(&grossRevenueCents)

	// Active deals and brands
	var activeDeals, activeBrands int
	s.db.QueryRow("SELECT COUNT(*) FROM brand_deals WHERE status = 'active'").Scan(&activeDeals)
	s.db.QueryRow("SELECT COUNT(DISTINCT offer_brand) FROM brand_deals WHERE status = 'active'").Scan(&activeBrands)

	// Write business_metrics_daily
	_, err = s.db.Exec(`
		INSERT INTO business_metrics_daily (date, dau, wau, mau, new_users,
			swaps_offered, swaps_accepted, swaps_declined, acceptance_rate,
			gross_revenue_cents, active_deals, active_brands, computed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(date) DO UPDATE SET
			dau = excluded.dau,
			wau = excluded.wau,
			mau = excluded.mau,
			new_users = excluded.new_users,
			swaps_offered = excluded.swaps_offered,
			swaps_accepted = excluded.swaps_accepted,
			swaps_declined = excluded.swaps_declined,
			acceptance_rate = excluded.acceptance_rate,
			gross_revenue_cents = excluded.gross_revenue_cents,
			active_deals = excluded.active_deals,
			active_brands = excluded.active_brands,
			computed_at = datetime('now')
	`, dateStr, dau, wau, mau, newUsers,
		swapsOffered, swapsAccepted, swapsDeclined, acceptanceRate,
		grossRevenueCents, activeDeals, activeBrands)
	if err != nil {
		log.Printf("[metrics] failed to upsert business_metrics_daily for %s: %v", dateStr, err)
	}

	elapsed := time.Since(start)
	log.Printf("[metrics] processed daily metrics for %s in %v (deals=%d, categories=%d)",
		dateStr, elapsed, len(dealPerfData), len(catData))
}

// runDailyMetricsLoop runs processDailyMetrics after an initial 10-minute delay,
// then every 24 hours. Computes metrics for yesterday's date.
func (s *Server) runDailyMetricsLoop() {
	// Initial delay: let the server warm up
	time.Sleep(10 * time.Minute)

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	s.processDailyMetrics(yesterday)
	s.processChurnSignals()
	s.processUserCohorts()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		yesterday = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		s.processDailyMetrics(yesterday)
		s.processChurnSignals()
		s.processUserCohorts()
	}
}

// =============================================================================
// ADMIN BUSINESS METRICS HANDLER
// =============================================================================

// handleBusinessMetrics handles GET /api/admin/business-metrics
// Returns the last 30 days (or ?days=N) of business_metrics_daily rows.
// Requires admin auth.
func (s *Server) handleBusinessMetrics(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}

	modifier := fmt.Sprintf("-%d days", days)

	rows, err := s.db.Query(`
		SELECT date, dau, wau, mau, new_users,
			swaps_offered, swaps_accepted, swaps_declined, acceptance_rate,
			gross_revenue_cents, active_deals, active_brands, computed_at
		FROM business_metrics_daily
		WHERE date >= DATE('now', ?)
		ORDER BY date DESC
	`, modifier)
	if err != nil {
		dbError(w, err, "Failed to query business metrics")
		return
	}

	type metricsRow struct {
		Date              string  `json:"date"`
		DAU               int     `json:"dau"`
		WAU               int     `json:"wau"`
		MAU               int     `json:"mau"`
		NewUsers          int     `json:"new_users"`
		SwapsOffered      int     `json:"swaps_offered"`
		SwapsAccepted     int     `json:"swaps_accepted"`
		SwapsDeclined     int     `json:"swaps_declined"`
		AcceptanceRate    float64 `json:"acceptance_rate"`
		GrossRevenueCents int     `json:"gross_revenue_cents"`
		ActiveDeals       int     `json:"active_deals"`
		ActiveBrands      int     `json:"active_brands"`
		ComputedAt        string  `json:"computed_at"`
	}

	var results []metricsRow
	for rows.Next() {
		var m metricsRow
		if err := rows.Scan(&m.Date, &m.DAU, &m.WAU, &m.MAU, &m.NewUsers,
			&m.SwapsOffered, &m.SwapsAccepted, &m.SwapsDeclined, &m.AcceptanceRate,
			&m.GrossRevenueCents, &m.ActiveDeals, &m.ActiveBrands, &m.ComputedAt); err != nil {
			log.Printf("[metrics] scan business_metrics_daily: %v", err)
			continue
		}
		results = append(results, m)
	}
	rows.Close()

	if results == nil {
		results = []metricsRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"days":    days,
		"metrics": results,
	})
}

// =============================================================================
// BRAND DEAL PERFORMANCE HANDLER
// =============================================================================

// handleBrandDealPerformance handles GET /brand/deal-performance/{id}
// Returns deal_performance_daily rows for the given deal_id (last 30 days).
// Requires brand session auth, and the deal must belong to the authenticated brand.
func (s *Server) handleBrandDealPerformance(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	// Extract deal ID from path: /brand/deal-performance/{id}
	idStr := strings.TrimPrefix(r.URL.Path, "/brand/deal-performance/")
	dealID, err := strconv.Atoi(idStr)
	if err != nil || dealID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid deal ID"})
		return
	}

	// Verify the deal belongs to this brand user
	var ownerID int
	err = s.db.QueryRow("SELECT COALESCE(brand_user_id, 0) FROM brand_deals WHERE id = ?", dealID).Scan(&ownerID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Deal not found"})
		return
	}
	if ownerID != bs.BrandUserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Not your deal"})
		return
	}

	rows, err := s.db.Query(`
		SELECT date, impressions, accepts, declines, revenue_cents, verified_count
		FROM deal_performance_daily
		WHERE deal_id = ? AND date >= DATE('now', '-30 days')
		ORDER BY date DESC
	`, dealID)
	if err != nil {
		dbError(w, err, "Failed to query deal performance")
		return
	}

	type perfRow struct {
		Date          string `json:"date"`
		Impressions   int    `json:"impressions"`
		Accepts       int    `json:"accepts"`
		Declines      int    `json:"declines"`
		RevenueCents  int    `json:"revenue_cents"`
		VerifiedCount int    `json:"verified_count"`
	}

	var results []perfRow
	for rows.Next() {
		var p perfRow
		if err := rows.Scan(&p.Date, &p.Impressions, &p.Accepts, &p.Declines, &p.RevenueCents, &p.VerifiedCount); err != nil {
			log.Printf("[metrics] scan deal_performance_daily: %v", err)
			continue
		}
		results = append(results, p)
	}
	rows.Close()

	if results == nil {
		results = []perfRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deal_id":     dealID,
		"performance": results,
	})
}

// =============================================================================
// CHURN PREDICTION SIGNALS
// =============================================================================

// processChurnSignals computes churn risk signals for all users and upserts
// them into the churn_signals table. Uses collect-then-write pattern.
func (s *Server) processChurnSignals() {
	start := time.Now()

	// --- Signal 1: days_inactive (days since last session activity) ---
	type churnSignal struct {
		userID      int
		signalType  string
		signalValue float64
		riskScore   float64
	}
	var signals []churnSignal

	rows, err := s.db.Query(`
		SELECT u.id,
			COALESCE(
				JULIANDAY('now') - JULIANDAY(MAX(s.created_at)),
				999
			) as days_inactive
		FROM users u
		LEFT JOIN sessions s ON s.user_id = u.id
		GROUP BY u.id
	`)
	if err != nil {
		log.Printf("[churn] failed to query days_inactive: %v", err)
	} else {
		for rows.Next() {
			var userID int
			var daysInactive float64
			if err := rows.Scan(&userID, &daysInactive); err != nil {
				log.Printf("[churn] scan days_inactive: %v", err)
				continue
			}
			risk := daysInactive / 30.0
			if risk > 1.0 {
				risk = 1.0
			}
			signals = append(signals, churnSignal{
				userID:      userID,
				signalType:  "days_inactive",
				signalValue: daysInactive,
				riskScore:   risk,
			})
		}
		rows.Close()
	}

	// --- Signal 2: decline_streak (consecutive declines without an accept) ---
	rows, err = s.db.Query(`
		SELECT user_id, action FROM swap_events
		ORDER BY user_id, created_at DESC
	`)
	if err != nil {
		log.Printf("[churn] failed to query decline_streak: %v", err)
	} else {
		streakMap := map[int]int{}
		currentUser := -1
		counting := true
		for rows.Next() {
			var userID int
			var action string
			if err := rows.Scan(&userID, &action); err != nil {
				continue
			}
			if userID != currentUser {
				currentUser = userID
				counting = true
				streakMap[userID] = 0
			}
			if !counting {
				continue
			}
			if action == "declined" {
				streakMap[userID]++
			} else if action == "accepted" {
				counting = false
			}
		}
		rows.Close()
		for userID, streak := range streakMap {
			risk := float64(streak) / 5.0
			if risk > 1.0 {
				risk = 1.0
			}
			signals = append(signals, churnSignal{
				userID:      userID,
				signalType:  "decline_streak",
				signalValue: float64(streak),
				riskScore:   risk,
			})
		}
	}

	// --- Signal 3: swap_fatigue (decline ratio if > 10 total offers) ---
	rows, err = s.db.Query(`
		SELECT user_id,
			COUNT(CASE WHEN action = 'offered' THEN 1 END) as total_offers,
			COUNT(CASE WHEN action = 'declined' THEN 1 END) as total_declines
		FROM swap_events
		GROUP BY user_id
		HAVING total_offers > 10
	`)
	if err != nil {
		log.Printf("[churn] failed to query swap_fatigue: %v", err)
	} else {
		for rows.Next() {
			var userID, totalOffers, totalDeclines int
			if err := rows.Scan(&userID, &totalOffers, &totalDeclines); err != nil {
				continue
			}
			ratio := float64(totalDeclines) / float64(totalOffers)
			risk := 0.0
			if ratio > 0.8 {
				risk = ratio
				if risk > 1.0 {
					risk = 1.0
				}
			}
			signals = append(signals, churnSignal{
				userID:      userID,
				signalType:  "swap_fatigue",
				signalValue: ratio,
				riskScore:   risk,
			})
		}
		rows.Close()
	}

	// --- Signal 4: list_abandonment (days since last item added) ---
	rows, err = s.db.Query(`
		SELECT u.id,
			COALESCE(
				JULIANDAY('now') - JULIANDAY(MAX(li.created_at)),
				999
			) as days_since_add
		FROM users u
		LEFT JOIN list_items li ON li.user_id = u.id
		GROUP BY u.id
	`)
	if err != nil {
		log.Printf("[churn] failed to query list_abandonment: %v", err)
	} else {
		for rows.Next() {
			var userID int
			var daysSinceAdd float64
			if err := rows.Scan(&userID, &daysSinceAdd); err != nil {
				continue
			}
			risk := daysSinceAdd / 14.0
			if risk > 1.0 {
				risk = 1.0
			}
			signals = append(signals, churnSignal{
				userID:      userID,
				signalType:  "list_abandonment",
				signalValue: daysSinceAdd,
				riskScore:   risk,
			})
		}
		rows.Close()
	}

	// --- Write all signals ---
	for _, sig := range signals {
		_, err := s.db.Exec(`
			INSERT INTO churn_signals (user_id, signal_type, signal_value, risk_score, computed_at)
			VALUES (?, ?, ?, ?, datetime('now'))
			ON CONFLICT(user_id, signal_type) DO UPDATE SET
				signal_value = excluded.signal_value,
				risk_score = excluded.risk_score,
				computed_at = datetime('now')
		`, sig.userID, sig.signalType, sig.signalValue, sig.riskScore)
		if err != nil {
			log.Printf("[churn] failed to upsert churn_signal for user %d type %s: %v", sig.userID, sig.signalType, err)
		}
	}

	elapsed := time.Since(start)
	log.Printf("[churn] processed %d churn signals in %v", len(signals), elapsed)
}

// =============================================================================
// USER COHORT RETENTION
// =============================================================================

// processUserCohorts computes cohort data for all users and upserts into
// user_cohorts. Uses collect-then-write pattern.
func (s *Server) processUserCohorts() {
	start := time.Now()

	type cohortRow struct {
		userID          int
		cohortWeek      string
		signupSource    string
		firstSwapAt     sql.NullString
		totalSwaps      int
		totalSavings    int
		lastActive      sql.NullString
		isChurned       int
	}
	var cohorts []cohortRow

	// Gather user data with cohort week derived from created_at
	rows, err := s.db.Query(`
		SELECT
			u.id,
			CAST(strftime('%Y', u.created_at) AS TEXT) || '-W' ||
				CASE WHEN LENGTH(CAST(strftime('%W', u.created_at) AS TEXT)) = 1
					THEN '0' || CAST(strftime('%W', u.created_at) AS TEXT)
					ELSE CAST(strftime('%W', u.created_at) AS TEXT)
				END as cohort_week,
			COALESCE(am.provider, '') as signup_source
		FROM users u
		LEFT JOIN auth_methods am ON am.user_id = u.id
		GROUP BY u.id
	`)
	if err != nil {
		log.Printf("[cohorts] failed to query users: %v", err)
		return
	}
	type userBase struct {
		userID       int
		cohortWeek   string
		signupSource string
	}
	var users []userBase
	for rows.Next() {
		var ub userBase
		if err := rows.Scan(&ub.userID, &ub.cohortWeek, &ub.signupSource); err != nil {
			log.Printf("[cohorts] scan user base: %v", err)
			continue
		}
		users = append(users, ub)
	}
	rows.Close()

	// Gather first swap and total swaps per user
	type swapStats struct {
		firstSwapAt sql.NullString
		totalSwaps  int
	}
	swapMap := map[int]swapStats{}
	rows, err = s.db.Query(`
		SELECT user_id, MIN(created_at) as first_swap, COUNT(*) as total
		FROM swap_events
		WHERE action = 'accepted'
		GROUP BY user_id
	`)
	if err != nil {
		log.Printf("[cohorts] failed to query swap stats: %v", err)
	} else {
		for rows.Next() {
			var userID int
			var ss swapStats
			if err := rows.Scan(&userID, &ss.firstSwapAt, &ss.totalSwaps); err != nil {
				continue
			}
			swapMap[userID] = ss
		}
		rows.Close()
	}

	// Gather total savings per user from swap_rebates
	savingsMap := map[int]int{}
	rows, err = s.db.Query(`
		SELECT user_id, COALESCE(SUM(amount_cents), 0) as total_savings
		FROM swap_rebates
		GROUP BY user_id
	`)
	if err != nil {
		log.Printf("[cohorts] failed to query savings: %v", err)
	} else {
		for rows.Next() {
			var userID, savings int
			if err := rows.Scan(&userID, &savings); err != nil {
				continue
			}
			savingsMap[userID] = savings
		}
		rows.Close()
	}

	// Gather last session activity per user
	lastActiveMap := map[int]sql.NullString{}
	rows, err = s.db.Query(`
		SELECT user_id, MAX(created_at) as last_active
		FROM sessions
		GROUP BY user_id
	`)
	if err != nil {
		log.Printf("[cohorts] failed to query last active: %v", err)
	} else {
		for rows.Next() {
			var userID int
			var la sql.NullString
			if err := rows.Scan(&userID, &la); err != nil {
				continue
			}
			lastActiveMap[userID] = la
		}
		rows.Close()
	}

	// Assemble cohort rows
	for _, ub := range users {
		cr := cohortRow{
			userID:       ub.userID,
			cohortWeek:   ub.cohortWeek,
			signupSource: ub.signupSource,
			totalSwaps:   swapMap[ub.userID].totalSwaps,
			firstSwapAt:  swapMap[ub.userID].firstSwapAt,
			totalSavings: savingsMap[ub.userID],
			lastActive:   lastActiveMap[ub.userID],
		}
		// Churned if no activity in 30 days
		if cr.lastActive.Valid {
			lastActiveTime, parseErr := time.Parse("2006-01-02 15:04:05", cr.lastActive.String)
			if parseErr == nil && time.Since(lastActiveTime).Hours() > 30*24 {
				cr.isChurned = 1
			}
		} else {
			cr.isChurned = 1 // No sessions at all = churned
		}
		cohorts = append(cohorts, cr)
	}

	// Write all cohort rows
	for _, cr := range cohorts {
		_, err := s.db.Exec(`
			INSERT INTO user_cohorts (user_id, cohort_week, signup_source, first_swap_at, total_swaps, total_savings_cents, last_active, is_churned)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(user_id) DO UPDATE SET
				cohort_week = excluded.cohort_week,
				signup_source = excluded.signup_source,
				first_swap_at = excluded.first_swap_at,
				total_swaps = excluded.total_swaps,
				total_savings_cents = excluded.total_savings_cents,
				last_active = excluded.last_active,
				is_churned = excluded.is_churned
		`, cr.userID, cr.cohortWeek, cr.signupSource, cr.firstSwapAt, cr.totalSwaps, cr.totalSavings, cr.lastActive, cr.isChurned)
		if err != nil {
			log.Printf("[cohorts] failed to upsert user_cohort for user %d: %v", cr.userID, err)
		}
	}

	elapsed := time.Since(start)
	log.Printf("[cohorts] processed %d user cohorts in %v", len(cohorts), elapsed)
}

// =============================================================================
// CHURN RISK HANDLER
// =============================================================================

// handleChurnRisk handles GET /api/admin/churn-risk
// Returns users with risk_score > 0.5 on any signal, grouped by user.
// Requires admin auth.
func (s *Server) handleChurnRisk(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	type signalInfo struct {
		SignalType  string  `json:"signal_type"`
		SignalValue float64 `json:"signal_value"`
		RiskScore   float64 `json:"risk_score"`
		ComputedAt  string  `json:"computed_at"`
	}
	type userRisk struct {
		UserID         int          `json:"user_id"`
		Name           string       `json:"name"`
		Phone          string       `json:"phone"`
		Email          string       `json:"email"`
		CompositeRisk  float64      `json:"composite_risk"`
		Signals        []signalInfo `json:"signals"`
	}

	rows, err := s.db.Query(`
		SELECT cs.user_id, COALESCE(u.name, ''), COALESCE(u.phone_number, ''), COALESCE(u.email, ''),
			cs.signal_type, cs.signal_value, cs.risk_score, cs.computed_at
		FROM churn_signals cs
		JOIN users u ON u.id = cs.user_id
		WHERE cs.risk_score > 0.5
		ORDER BY cs.user_id, cs.risk_score DESC
	`)
	if err != nil {
		dbError(w, err, "Failed to query churn signals")
		return
	}

	userMap := map[int]*userRisk{}
	var order []int
	for rows.Next() {
		var userID int
		var name, phone, email string
		var si signalInfo
		if err := rows.Scan(&userID, &name, &phone, &email, &si.SignalType, &si.SignalValue, &si.RiskScore, &si.ComputedAt); err != nil {
			log.Printf("[churn-risk] scan: %v", err)
			continue
		}
		ur, exists := userMap[userID]
		if !exists {
			ur = &userRisk{
				UserID: userID,
				Name:   name,
				Phone:  phone,
				Email:  email,
			}
			userMap[userID] = ur
			order = append(order, userID)
		}
		ur.Signals = append(ur.Signals, si)
	}
	rows.Close()

	// Compute composite risk: average of all signal risk scores per user
	var results []userRisk
	for _, uid := range order {
		ur := userMap[uid]
		total := 0.0
		for _, sig := range ur.Signals {
			total += sig.RiskScore
		}
		ur.CompositeRisk = total / float64(len(ur.Signals))
		results = append(results, *ur)
	}

	if results == nil {
		results = []userRisk{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"at_risk_users": results,
		"count":         len(results),
	})
}

// =============================================================================
// COHORT ANALYSIS HANDLER
// =============================================================================

// handleCohortAnalysis handles GET /api/admin/cohort-analysis
// Returns cohort retention data grouped by signup week.
// Requires admin auth.
func (s *Server) handleCohortAnalysis(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	type cohortSummary struct {
		CohortWeek    string  `json:"cohort_week"`
		UserCount     int     `json:"user_count"`
		RetainedCount int     `json:"retained_count"`
		AvgSwaps      float64 `json:"avg_swaps"`
		AvgSavings    float64 `json:"avg_savings_cents"`
	}

	rows, err := s.db.Query(`
		SELECT
			cohort_week,
			COUNT(*) as user_count,
			SUM(CASE WHEN last_active >= datetime('now', '-7 days') THEN 1 ELSE 0 END) as retained_count,
			AVG(total_swaps) as avg_swaps,
			AVG(total_savings_cents) as avg_savings
		FROM user_cohorts
		GROUP BY cohort_week
		ORDER BY cohort_week DESC
	`)
	if err != nil {
		dbError(w, err, "Failed to query cohort analysis")
		return
	}

	var results []cohortSummary
	for rows.Next() {
		var cs cohortSummary
		if err := rows.Scan(&cs.CohortWeek, &cs.UserCount, &cs.RetainedCount, &cs.AvgSwaps, &cs.AvgSavings); err != nil {
			log.Printf("[cohort-analysis] scan: %v", err)
			continue
		}
		results = append(results, cs)
	}
	rows.Close()

	if results == nil {
		results = []cohortSummary{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cohorts": results,
	})
}
