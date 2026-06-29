package main

import (
	"database/sql"
	"fmt"
	"log"
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
		category         string
		offered          int
		accepted         int
		avgDisc          float64
		topDeclineReason string
	}
	var catData []catInsight

	catRows, err := s.db.Query(`
		SELECT LOWER(bd.competitor_product) as category,
			COUNT(CASE WHEN se.action = 'offered' THEN 1 END) as offered,
			COUNT(CASE WHEN se.action = 'accepted' THEN 1 END) as accepted,
			COALESCE(AVG(bd.discount_cents), 0) as avg_discount
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

	// Collect top decline reason per category
	type catDecline struct {
		category string
		reason   string
	}
	var catDeclines []catDecline
	declineRows, err := s.db.Query(`
		SELECT LOWER(bd.competitor_product) as category, se.decline_reason, COUNT(*) as cnt
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE se.action = 'declined' AND se.decline_reason != '' AND bd.status != 'deleted'
		GROUP BY LOWER(bd.competitor_product), se.decline_reason
		ORDER BY LOWER(bd.competitor_product), cnt DESC
	`)
	if err != nil {
		log.Printf("[metrics] failed to query category decline reasons: %v", err)
	} else {
		for declineRows.Next() {
			var cd catDecline
			var cnt int
			if err := declineRows.Scan(&cd.category, &cd.reason, &cnt); err != nil {
				log.Printf("[metrics] scan category decline: %v", err)
				continue
			}
			catDeclines = append(catDeclines, cd)
		}
		declineRows.Close()
	}
	// Build map: category -> top decline reason (first per category due to ORDER BY cnt DESC)
	topDeclineMap := map[string]string{}
	for _, cd := range catDeclines {
		if _, exists := topDeclineMap[cd.category]; !exists {
			topDeclineMap[cd.category] = cd.reason
		}
	}

	// Write category_swap_insights
	for _, ci := range catData {
		acceptRate := 0.0
		if ci.offered > 0 {
			acceptRate = float64(ci.accepted) / float64(ci.offered) * 100.0
		}
		totalSwaps := ci.offered
		topReason := topDeclineMap[ci.category]
		_, err := s.db.Exec(`
			INSERT INTO category_swap_insights (category, avg_acceptance_rate, avg_discount_pct, total_swaps, top_decline_reason, last_computed)
			VALUES (?, ?, ?, ?, ?, datetime('now'))
			ON CONFLICT(category) DO UPDATE SET
				avg_acceptance_rate = excluded.avg_acceptance_rate,
				avg_discount_pct = excluded.avg_discount_pct,
				total_swaps = excluded.total_swaps,
				top_decline_reason = excluded.top_decline_reason,
				last_computed = datetime('now')
		`, ci.category, acceptRate, ci.avgDisc, totalSwaps, topReason)
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

	// Swap counts for dateStr — single query
	var swapsOffered, swapsAccepted, swapsDeclined int
	s.db.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN action='offered' THEN 1 END), 0),
		       COALESCE(SUM(CASE WHEN action='accepted' THEN 1 END), 0),
		       COALESCE(SUM(CASE WHEN action='declined' THEN 1 END), 0)
		FROM swap_events WHERE DATE(created_at) = ?`, dateStr).Scan(&swapsOffered, &swapsAccepted, &swapsDeclined)

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
	s.processMarketplaceHealth()
	s.processSegmentAnalysis()
	s.processCategoryAlerts()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		yesterday = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		s.processDailyMetrics(yesterday)
		s.processChurnSignals()
		s.processUserCohorts()
		s.processMarketplaceHealth()
		s.processSegmentAnalysis()
		s.processCategoryAlerts()
	}
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
		userID      string
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
			var userID string
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
		streakMap := map[string]int{}
		currentUser := ""
		counting := true
		for rows.Next() {
			var userID string
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
			var userID string
			var totalOffers, totalDeclines int
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
			var userID string
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
			log.Printf("[churn] failed to upsert churn_signal for user %s type %s: %v", sig.userID, sig.signalType, err)
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
		userID          string
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
		userID       string
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
	swapMap := map[string]swapStats{}
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
			var userID string
			var ss swapStats
			if err := rows.Scan(&userID, &ss.firstSwapAt, &ss.totalSwaps); err != nil {
				continue
			}
			swapMap[userID] = ss
		}
		rows.Close()
	}

	// Gather total savings per user from swap_rebates
	savingsMap := map[string]int{}
	rows, err = s.db.Query(`
		SELECT user_id, COALESCE(SUM(amount_cents), 0) as total_savings
		FROM swap_rebates
		GROUP BY user_id
	`)
	if err != nil {
		log.Printf("[cohorts] failed to query savings: %v", err)
	} else {
		for rows.Next() {
			var userID string
			var savings int
			if err := rows.Scan(&userID, &savings); err != nil {
				continue
			}
			savingsMap[userID] = savings
		}
		rows.Close()
	}

	// Gather last session activity per user
	lastActiveMap := map[string]sql.NullString{}
	rows, err = s.db.Query(`
		SELECT user_id, MAX(created_at) as last_active
		FROM sessions
		GROUP BY user_id
	`)
	if err != nil {
		log.Printf("[cohorts] failed to query last active: %v", err)
	} else {
		for rows.Next() {
			var userID string
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
			log.Printf("[cohorts] failed to upsert user_cohort for user %s: %v", cr.userID, err)
		}
	}

	elapsed := time.Since(start)
	log.Printf("[cohorts] processed %d user cohorts in %v", len(cohorts), elapsed)
}

// =============================================================================
// MARKETPLACE HEALTH INDEX
// =============================================================================

// processMarketplaceHealth computes a daily health score for the marketplace.
// Supply (deal coverage), demand (user engagement), balance, and concentration.
func (s *Server) processMarketplaceHealth() {
	today := time.Now().Format("2006-01-02")

	// Supply score: how many active deals exist relative to target (50)
	var activeDeals int
	s.db.QueryRow("SELECT COUNT(*) FROM brand_deals WHERE status = 'active'").Scan(&activeDeals)
	supplyScore := clampConfidence(float64(activeDeals)/50.0, 1.0) * 100

	// Demand score: DAU relative to target (100)
	var dau int
	s.db.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE created_at >= datetime('now', '-1 day')`).Scan(&dau)
	demandScore := clampConfidence(float64(dau)/100.0, 1.0) * 100

	// Balance score: ratio of supply to demand (ideal = 1.0)
	balanceScore := 50.0 // default if both are 0
	if activeDeals > 0 && dau > 0 {
		ratio := float64(activeDeals) / float64(dau)
		if ratio > 1.0 {
			ratio = 1.0 / ratio
		}
		balanceScore = ratio * 100
	}

	// Revenue concentration: HHI (Herfindahl-Hirschman Index)
	// 0 = perfectly diverse, 1 = single brand
	type brandSpend struct {
		brand string
		spent int
	}
	var brandSpends []brandSpend
	var totalSpent int
	bRows, err := s.db.Query(`
		SELECT bd.offer_brand, SUM(bd.budget_spent_cents) as spent
		FROM brand_deals bd
		WHERE bd.budget_spent_cents > 0
		GROUP BY bd.offer_brand
	`)
	if err == nil {
		for bRows.Next() {
			var bs brandSpend
			bRows.Scan(&bs.brand, &bs.spent)
			brandSpends = append(brandSpends, bs)
			totalSpent += bs.spent
		}
		bRows.Close()
	}
	concentration := 0.0
	if totalSpent > 0 {
		for _, bs := range brandSpends {
			share := float64(bs.spent) / float64(totalSpent)
			concentration += share * share
		}
	}

	// Composite health index
	healthIndex := (supplyScore + demandScore + balanceScore + (1-concentration)*100) / 4

	s.db.Exec(`INSERT INTO marketplace_health (date, supply_score, demand_score, balance_score, revenue_concentration, health_index, computed_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(date) DO UPDATE SET supply_score=?, demand_score=?, balance_score=?, revenue_concentration=?, health_index=?, computed_at=datetime('now')`,
		today, supplyScore, demandScore, balanceScore, concentration, healthIndex,
		supplyScore, demandScore, balanceScore, concentration, healthIndex)

	log.Printf("[metrics] marketplace health: supply=%.0f demand=%.0f balance=%.0f concentration=%.2f health=%.0f",
		supplyScore, demandScore, balanceScore, concentration, healthIndex)
}

// =============================================================================
// SEGMENT ANALYSIS
// =============================================================================

// processSegmentAnalysis groups users by price_sensitivity from penny_learnings
// and computes acceptance rates per segment. Upserts into category_swap_insights
// with category = "segment:<name>". Uses collect-then-write pattern.
func (s *Server) processSegmentAnalysis() {
	start := time.Now()

	// Step 1: Collect user->segment mapping from penny_learnings
	type userSegment struct {
		userID  string
		segment string
	}
	var userSegments []userSegment

	rows, err := s.db.Query(`
		SELECT scope_id, value
		FROM penny_learnings
		WHERE scope = 'user' AND key = 'price_sensitivity' AND value != ''
	`)
	if err != nil {
		log.Printf("[segment] failed to query penny_learnings: %v", err)
		return
	}
	segmentUsers := map[string][]string{} // segment -> list of user IDs
	for rows.Next() {
		var us userSegment
		if err := rows.Scan(&us.userID, &us.segment); err != nil {
			log.Printf("[segment] scan user segment: %v", err)
			continue
		}
		userSegments = append(userSegments, us)
		segmentUsers[us.segment] = append(segmentUsers[us.segment], us.userID)
	}
	rows.Close()

	if len(segmentUsers) == 0 {
		log.Printf("[segment] no user segments found, skipping")
		return
	}

	// Step 2: For each user, collect offer/accept counts and avg discount
	type userSwapStats struct {
		offers       int
		accepts      int
		totalDisc    float64
		acceptCount  int // for averaging discount
	}
	userStats := map[string]*userSwapStats{}

	rows, err = s.db.Query(`
		SELECT se.user_id,
			COUNT(CASE WHEN se.action = 'offered' THEN 1 END) as offers,
			COUNT(CASE WHEN se.action = 'accepted' THEN 1 END) as accepts,
			COALESCE(AVG(CASE WHEN se.action = 'accepted' THEN bd.discount_cents END), 0) as avg_disc
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		GROUP BY se.user_id
	`)
	if err != nil {
		log.Printf("[segment] failed to query swap stats: %v", err)
		return
	}
	for rows.Next() {
		var userID string
		var offers, accepts int
		var avgDisc float64
		if err := rows.Scan(&userID, &offers, &accepts, &avgDisc); err != nil {
			log.Printf("[segment] scan swap stats: %v", err)
			continue
		}
		userStats[userID] = &userSwapStats{
			offers:  offers,
			accepts: accepts,
			totalDisc:   avgDisc,
			acceptCount: accepts,
		}
	}
	rows.Close()

	// Step 3: Aggregate per segment
	type segmentResult struct {
		segment         string
		totalOffers     int
		totalAccepts    int
		avgDiscountCents float64
	}
	var results []segmentResult

	for segment, uids := range segmentUsers {
		var totalOffers, totalAccepts int
		var discSum float64
		var discCount int
		for _, uid := range uids {
			if stats, ok := userStats[uid]; ok {
				totalOffers += stats.offers
				totalAccepts += stats.accepts
				if stats.acceptCount > 0 {
					discSum += stats.totalDisc
					discCount++
				}
			}
		}
		avgDisc := 0.0
		if discCount > 0 {
			avgDisc = discSum / float64(discCount)
		}
		results = append(results, segmentResult{
			segment:         segment,
			totalOffers:     totalOffers,
			totalAccepts:    totalAccepts,
			avgDiscountCents: avgDisc,
		})
	}

	// Step 4: Write to category_swap_insights with category = "segment:<name>"
	for _, sr := range results {
		acceptRate := 0.0
		if sr.totalOffers > 0 {
			acceptRate = float64(sr.totalAccepts) / float64(sr.totalOffers) * 100.0
		}
		cat := "segment:" + sr.segment
		_, err := s.db.Exec(`
			INSERT INTO category_swap_insights (category, avg_acceptance_rate, avg_discount_pct, total_swaps, last_computed)
			VALUES (?, ?, ?, ?, datetime('now'))
			ON CONFLICT(category) DO UPDATE SET
				avg_acceptance_rate = excluded.avg_acceptance_rate,
				avg_discount_pct = excluded.avg_discount_pct,
				total_swaps = excluded.total_swaps,
				last_computed = datetime('now')
		`, cat, acceptRate, sr.avgDiscountCents, sr.totalOffers)
		if err != nil {
			log.Printf("[segment] failed to upsert segment insight for %s: %v", cat, err)
		}
	}

	elapsed := time.Since(start)
	log.Printf("[segment] processed %d segments (%d users) in %v", len(results), len(userSegments), elapsed)
}

// =============================================================================
// PREDICTIVE CATEGORY ALERTS
// =============================================================================

// processCategoryAlerts detects disruption signals in swap data:
// 1. Rising stars: brands with rapidly increasing acceptance rates
// 2. Declining incumbents: brands losing acceptance share
// 3. Category heating: categories with sharply increasing swap volume
// Writes alerts to category_alerts table for brand portal and admin consumption.
func (s *Server) processCategoryAlerts() {
	// Step 1: Collect per-brand acceptance rates for recent vs prior periods
	// Recent = last 7 days, Prior = 8-30 days ago
	type brandMetric struct {
		brand       string
		categoryID  int
		recentRate  float64
		recentCount int
		priorRate   float64
		priorCount  int
	}

	rows, err := s.db.Query(`
		SELECT bd.offer_brand, COALESCE(bd.category_id, 0),
			COALESCE(SUM(CASE WHEN se.created_at >= datetime('now', '-7 days') AND se.action = 'offered' THEN 1 ELSE 0 END), 0) as recent_offered,
			COALESCE(SUM(CASE WHEN se.created_at >= datetime('now', '-7 days') AND se.action = 'accepted' THEN 1 ELSE 0 END), 0) as recent_accepted,
			COALESCE(SUM(CASE WHEN se.created_at < datetime('now', '-7 days') AND se.created_at >= datetime('now', '-30 days') AND se.action = 'offered' THEN 1 ELSE 0 END), 0) as prior_offered,
			COALESCE(SUM(CASE WHEN se.created_at < datetime('now', '-7 days') AND se.created_at >= datetime('now', '-30 days') AND se.action = 'accepted' THEN 1 ELSE 0 END), 0) as prior_accepted
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE se.created_at >= datetime('now', '-30 days')
		GROUP BY bd.offer_brand, bd.category_id
		HAVING recent_offered + prior_offered >= 5
	`)
	if err != nil {
		log.Printf("[alerts] failed to query brand metrics: %v", err)
		return
	}

	var metrics []brandMetric
	for rows.Next() {
		var m brandMetric
		var recentOff, recentAcc, priorOff, priorAcc int
		if err := rows.Scan(&m.brand, &m.categoryID, &recentOff, &recentAcc, &priorOff, &priorAcc); err != nil {
			continue
		}
		m.recentCount = recentOff
		m.priorCount = priorOff
		if recentOff > 0 {
			m.recentRate = float64(recentAcc) / float64(recentOff)
		}
		if priorOff > 0 {
			m.priorRate = float64(priorAcc) / float64(priorOff)
		}
		metrics = append(metrics, m)
	}
	rows.Close()

	// Step 2: Detect alerts
	type alert struct {
		alertType  string // "rising_star", "declining", "category_heating"
		brand      string
		categoryID int
		severity   string // "high", "medium", "low"
		message    string
		dataJSON   string
	}
	var alerts []alert

	for _, m := range metrics {
		rateDelta := m.recentRate - m.priorRate

		// Rising star: acceptance rate increased by 20%+ with decent volume
		if rateDelta >= 0.20 && m.recentCount >= 5 && m.priorCount >= 3 {
			severity := "medium"
			if rateDelta >= 0.40 {
				severity = "high"
			}
			alerts = append(alerts, alert{
				alertType:  "rising_star",
				brand:      m.brand,
				categoryID: m.categoryID,
				severity:   severity,
				message:    fmt.Sprintf("%s acceptance rate surged %.0f%% → %.0f%% (Δ+%.0f%%)", m.brand, m.priorRate*100, m.recentRate*100, rateDelta*100),
				dataJSON:   fmt.Sprintf(`{"prior_rate":%.3f,"recent_rate":%.3f,"delta":%.3f,"recent_count":%d}`, m.priorRate, m.recentRate, rateDelta, m.recentCount),
			})
		}

		// Declining: acceptance rate dropped 20%+ with decent volume
		if rateDelta <= -0.20 && m.priorCount >= 5 {
			severity := "medium"
			if rateDelta <= -0.40 {
				severity = "high"
			}
			alerts = append(alerts, alert{
				alertType:  "declining",
				brand:      m.brand,
				categoryID: m.categoryID,
				severity:   severity,
				message:    fmt.Sprintf("%s acceptance rate dropped %.0f%% → %.0f%% (Δ%.0f%%)", m.brand, m.priorRate*100, m.recentRate*100, rateDelta*100),
				dataJSON:   fmt.Sprintf(`{"prior_rate":%.3f,"recent_rate":%.3f,"delta":%.3f,"prior_count":%d}`, m.priorRate, m.recentRate, rateDelta, m.priorCount),
			})
		}
	}

	// Step 3: Check for category heating (volume surge)
	catRows, err := s.db.Query(`
		SELECT COALESCE(bd.category_id, 0),
			COALESCE(SUM(CASE WHEN se.created_at >= datetime('now', '-7 days') THEN 1 ELSE 0 END), 0) as recent_vol,
			COALESCE(SUM(CASE WHEN se.created_at < datetime('now', '-7 days') AND se.created_at >= datetime('now', '-30 days') THEN 1 ELSE 0 END), 0) as prior_vol
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE se.created_at >= datetime('now', '-30 days')
		GROUP BY bd.category_id
		HAVING recent_vol >= 10
	`)
	if err == nil {
		for catRows.Next() {
			var catID, recentVol, priorVol int
			catRows.Scan(&catID, &recentVol, &priorVol)
			if priorVol > 0 {
				// Normalize prior to 7-day equivalent (it covers 23 days)
				priorWeekly := float64(priorVol) * 7.0 / 23.0
				if priorWeekly > 0 {
					growthRatio := float64(recentVol) / priorWeekly
					if growthRatio >= 2.0 {
						severity := "medium"
						if growthRatio >= 3.0 {
							severity = "high"
						}
						alerts = append(alerts, alert{
							alertType:  "category_heating",
							brand:      "",
							categoryID: catID,
							severity:   severity,
							message:    fmt.Sprintf("Category swap volume surged %.0fx (weekly: %.0f → %d)", growthRatio, priorWeekly, recentVol),
							dataJSON:   fmt.Sprintf(`{"prior_weekly":%.1f,"recent_weekly":%d,"growth_ratio":%.2f}`, priorWeekly, recentVol, growthRatio),
						})
					}
				}
			}
		}
		catRows.Close()
	}

	// Step 4: Write alerts
	if len(alerts) == 0 {
		return
	}

	for _, a := range alerts {
		if _, err := s.db.Exec(`INSERT INTO category_alerts (alert_type, brand, category_id, severity, message, data_json)
			VALUES (?, ?, ?, ?, ?, ?)`,
			a.alertType, a.brand, a.categoryID, a.severity, a.message, a.dataJSON); err != nil {
			log.Printf("[alerts] failed to insert alert: %v", err)
		}
	}

	log.Printf("[alerts] generated %d category alerts", len(alerts))
}
