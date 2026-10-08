package main

import (
	"encoding/json"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
)


// checkDealsForItems finds the first active, un-suppressed, in-budget deal that matches
// any of the added items. Slider/autotune settings gate whether deals are shown.
//
//	slider 0-33 = always show, 34-66 = show if discount > $1, 67-100 = never show
//	autotune ON: Penny decides based on accept/shown ratio (< 5% = stop showing)
func (s *Server) checkDealsForItems(userID string, addedItems []ListItem) *PendingSwap {
	if len(addedItems) == 0 {
		return nil
	}

	// Load user slider and autotune settings
	var slider, autotune, acceptCount, shownCount int
	err := s.db.QueryRow(
		"SELECT penny_slider, penny_autotune, deal_accept_count, deal_shown_count FROM users WHERE id = ?",
		userID,
	).Scan(&slider, &autotune, &acceptCount, &shownCount)
	if err != nil {
		log.Printf("[swaps] failed to load user %s settings: %v", userID, err)
		return nil
	}

	// Slider gate
	if autotune == 1 {
		// Autotune: stop showing if acceptance rate < 5% after 20+ offers
		if shownCount >= 20 {
			ratio := float64(acceptCount) / float64(shownCount)
			if ratio < 0.05 {
				return nil
			}
		}
	} else {
		// Manual slider: 67-100 = never show deals
		if slider >= 67 {
			return nil
		}
	}

	// Session-level frequency cap: max 3 pending (offered but not acted on) swaps at a time.
	// This prevents overwhelming users with too many swap suggestions during a single list-building session.
	const maxPendingSwaps = 3
	var pendingCount int
	s.db.QueryRow(`
		SELECT COUNT(*) FROM swap_events se
		WHERE se.user_id = ? AND se.action = 'offered'
		  AND NOT EXISTS (
		    SELECT 1 FROM swap_events se2
		    WHERE se2.user_id = se.user_id AND se2.deal_id = se.deal_id AND se2.item_id = se.item_id
		      AND se2.action IN ('accepted', 'declined')
		      AND se2.id > se.id
		  )
		  AND se.item_id IN (SELECT id FROM list_items WHERE checked = 0)
	`, userID).Scan(&pendingCount)
	if pendingCount >= maxPendingSwaps {
		return nil
	}

	// Load active deals with price comparison & why data
	rows, err := s.db.Query(`
		SELECT id, competitor_product, offer_brand, offer_product, discount_cents,
		       total_budget_cents, budget_spent_cents,
		       offer_price_cents, competitor_price_cents,
		       offer_size, competitor_size,
		       offer_unit_price, competitor_unit_price,
		       why_bullets, allergen_notes,
		       deal_type, quantity_offered, trial_product_name, trial_size, campaign_tag,
		       COALESCE(ends_at,''),
		       COALESCE(category_id, 0),
		       COALESCE(product_description, ''),
		       COALESCE(key_benefits, '[]'),
		       COALESCE(brand_story, ''),
		       max_rebate_cents, platform_fee_cents, require_verified_identity,
		       receipt_grace_days, upc
		FROM brand_deals
		WHERE status = 'active'
		  AND (starts_at <= datetime('now') OR starts_at IS NULL)
		  AND (ends_at IS NULL OR ends_at >= datetime('now'))
	`)
	if err != nil {
		log.Printf("[swaps] failed to load deals: %v", err)
		return nil
	}
	defer rows.Close()

	type activeDeal struct {
		ID                      int
		CompetitorProduct       string
		OfferBrand              string
		OfferProduct            string
		DiscountCents           int
		BudgetRemaining         int
		OfferPriceCents         int
		CompetitorPriceCents    int
		OfferSize               string
		CompetitorSize          string
		OfferUnitPrice          string
		CompetitorUnitPrice     string
		WhyBulletsJSON          string
		AllergenNotes           string
		DealType                string
		QuantityOffered         int
		TrialProductName        string
		TrialSize               string
		CampaignTag             string
		EndsAt                  string
		CategoryID              int
		ProductDescription      string
		KeyBenefitsJSON         string
		BrandStory              string
		MaxRebateCents          int
		PlatformFeeCents        int
		RequireVerifiedIdentity int
		ReceiptGraceDays        int
		UPC                     string
	}
	var deals []activeDeal
	for rows.Next() {
		var d activeDeal
		var budgetTotal, budgetSpent int
		if err := rows.Scan(&d.ID, &d.CompetitorProduct, &d.OfferBrand, &d.OfferProduct,
			&d.DiscountCents, &budgetTotal, &budgetSpent,
			&d.OfferPriceCents, &d.CompetitorPriceCents,
			&d.OfferSize, &d.CompetitorSize,
			&d.OfferUnitPrice, &d.CompetitorUnitPrice,
			&d.WhyBulletsJSON, &d.AllergenNotes,
			&d.DealType, &d.QuantityOffered, &d.TrialProductName, &d.TrialSize, &d.CampaignTag,
			&d.EndsAt, &d.CategoryID,
			&d.ProductDescription, &d.KeyBenefitsJSON, &d.BrandStory,
			&d.MaxRebateCents, &d.PlatformFeeCents, &d.RequireVerifiedIdentity,
			&d.ReceiptGraceDays, &d.UPC); err != nil {
			continue
		}
		d.BudgetRemaining = budgetTotal - budgetSpent
		if d.BudgetRemaining > 0 || budgetTotal == 0 { // 0 budget = unlimited
			deals = append(deals, d)
		}
	}

	// Load household allergens for safety filtering (union of all members' allergens)
	userAllergens := s.getHouseholdAllergens(userID)

	for _, item := range addedItems {
		itemKey := strings.ToLower(item.Brand + " " + item.Name)

		// Dynamic auction: collect all eligible deals for this item, then pick the best
		type auctionCandidate struct {
			deal     activeDeal
			bidScore float64 // effective bid = discount_cents × predicted_acceptance
		}
		var candidates []auctionCandidate

		for _, deal := range deals {
			// Case-insensitive fuzzy match: check if competitor product appears in item
			competitor := strings.ToLower(deal.CompetitorProduct)
			if !strings.Contains(itemKey, competitor) && !strings.Contains(strings.ToLower(item.Name), competitor) {
				continue
			}

			// SAFETY: Allergen hard-filter — never suggest products containing user's allergens
			if len(userAllergens) > 0 {
				dealText := strings.ToLower(deal.AllergenNotes + " " + deal.OfferProduct + " " + deal.OfferBrand)
				allergenMatch := false
				for _, allergen := range userAllergens {
					if strings.Contains(dealText, strings.ToLower(allergen)) {
						allergenMatch = true
						break
					}
				}
				if allergenMatch {
					continue
				}
			}

			// Fully free fraud gates
			if deal.DealType == "fully_free" {
				// 72h cooling period: accounts < 72h old cannot accept fully_free deals
				var createdAt string
				s.db.QueryRow("SELECT created_at FROM users WHERE id = ?", userID).Scan(&createdAt)
				if t, err := parseDBTime(createdAt); err == nil {
					if time.Since(t) < 72*time.Hour {
						continue
					}
				}
				// Verified identity required
				if deal.RequireVerifiedIdentity == 1 {
					// A verified identity is a Rally (Clerk) sign-in.
					var clerkID string
					s.db.QueryRow("SELECT COALESCE(clerk_user_id,'') FROM users WHERE id = ?", userID).Scan(&clerkID)
					if clerkID == "" {
						continue
					}
				}
				// Household-level dedup: max 1 fully_free redemption per deal per household
				hhID := getUserHouseholdID(s.db, userID)
				if hhID > 0 {
					var hhAccepts int
					s.db.QueryRow(`SELECT COUNT(*) FROM swap_events se
						JOIN users u ON u.id = se.user_id
						WHERE se.deal_id = ? AND se.action = 'accepted' AND u.household_id = ?`,
						deal.ID, hhID).Scan(&hhAccepts)
					if hhAccepts > 0 {
						continue
					}
				}
			}

			// Manual slider 34-66: only show if discount > $1
			if autotune == 0 && slider >= 34 && deal.DiscountCents <= 100 && deal.DealType != "fully_free" {
				continue
			}

			// Check suppression (3+ declines = permanent suppress)
			var declineCount int
			s.db.QueryRow("SELECT decline_count FROM swap_suppressions WHERE user_id = ? AND deal_id = ?",
				userID, deal.ID).Scan(&declineCount)
			if declineCount >= 3 {
				continue
			}

			// 1-swap-per-user-per-campaign: don't re-offer a deal already accepted
			var priorAccepts int
			s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE user_id = ? AND deal_id = ? AND action = 'accepted'",
				userID, deal.ID).Scan(&priorAccepts)
			if priorAccepts > 0 {
				continue
			}

			// Prediction filter: suppress deals where confidence is high and acceptance rate is very low
			predRate := 0.5 // neutral default
			if autotune == 1 {
				pRate, predConf := s.getUserSwapPrediction(userID, deal.CategoryID, deal.OfferBrand)
				predRate = pRate
				if predConf >= 0.5 && predRate < 0.10 {
					log.Printf("[swaps] prediction filter: skipping deal %d for user %s (predicted acceptance %.1f%%, confidence %.2f)",
						deal.ID, userID, predRate*100, predConf)
					continue
				}
			}

			// Compute auction bid score: discount × predicted acceptance rate
			bidScore := float64(deal.DiscountCents) * predRate
			candidates = append(candidates, auctionCandidate{deal: deal, bidScore: bidScore})
		}

		if len(candidates) == 0 {
			continue
		}

		// Sort by bid score descending — highest effective bid wins
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].bidScore > candidates[j].bidScore
		})

		// Winner is the first candidate
		winner := candidates[0]
		deal := winner.deal

		// Log auction results: winner gets 'offered', losers get 'outbid'
		if len(candidates) > 1 {
			log.Printf("[auction] %d candidates for item %q — winner: deal %d (bid %.1f), %d outbid",
				len(candidates), item.Name, deal.ID, winner.bidScore, len(candidates)-1)
			for _, loser := range candidates[1:] {
				s.db.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, ?, 'outbid')",
					userID, loser.deal.ID, item.ID)
			}
		}

		{
			// Found a match via auction! Log the offer and return it
			s.db.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, ?, 'offered')",
				userID, deal.ID, item.ID)
			s.db.Exec("UPDATE users SET deal_shown_count = deal_shown_count + 1 WHERE id = ?", userID)

			// Parse why_bullets JSON array
			var whyBullets []string
			if deal.WhyBulletsJSON != "" && deal.WhyBulletsJSON != "[]" {
				if err := json.Unmarshal([]byte(deal.WhyBulletsJSON), &whyBullets); err != nil {
					log.Printf("[swaps] failed to parse why_bullets for deal %d: %v", deal.ID, err)
				}
			}

			// Enrich with real BlueCart/Walmart pricing if available
			offerPriceCents := deal.OfferPriceCents
			competitorPriceCents := deal.CompetitorPriceCents
			offerSize := deal.OfferSize
			competitorSize := deal.CompetitorSize
			offerUnitPrice := deal.OfferUnitPrice
			competitorUnitPrice := deal.CompetitorUnitPrice

			if s.config.BlueCartAPIKey != "" {
				zip := s.getUserZip(userID)
				if zip != "" {
					category := item.Department
					if offerPrice, err := s.fetchBrandPrice(s.config.BlueCartAPIKey, deal.OfferBrand, category, zip); err == nil {
						offerPriceCents = offerPrice.PriceCents
						if offerPrice.UnitPrice != "" {
							offerUnitPrice = offerPrice.UnitPrice
						}
						if offerPrice.Size != "" {
							offerSize = offerPrice.Size
						}
					}
					if compPrice, err := s.fetchBrandPrice(s.config.BlueCartAPIKey, deal.CompetitorProduct, category, zip); err == nil {
						competitorPriceCents = compPrice.PriceCents
						if compPrice.UnitPrice != "" {
							competitorUnitPrice = compPrice.UnitPrice
						}
						if compPrice.Size != "" {
							competitorSize = compPrice.Size
						}
					}
				}
			}

			// Store-aware pricing: override with actual store prices if available
			storeName := ""
			userStoreID := s.getUserActiveStoreID(userID)
			if userStoreID > 0 {
				if storePrice, found := s.getStorePrice(userStoreID, deal.OfferProduct, deal.OfferBrand); found {
					offerPriceCents = storePrice
				}
				if storePrice, found := s.getStorePrice(userStoreID, deal.CompetitorProduct, ""); found {
					competitorPriceCents = storePrice
				}
				// Get store name for display
				s.db.QueryRow("SELECT name FROM stores WHERE id = ?", userStoreID).Scan(&storeName)
			}

			// Calculate savings percent — prefer per-oz when sizes differ (honest metric)
			savingsPercent := 0
			if offerUnitPrice != "" && competitorUnitPrice != "" && offerSize != competitorSize {
				// Parse unit prices (e.g., "$0.10/oz") for per-oz comparison
				offerUP := parseUnitPriceCents(offerUnitPrice)
				compUP := parseUnitPriceCents(competitorUnitPrice)
				if compUP > 0 && offerUP > 0 {
					savingsPercent = int(float64(compUP-offerUP) / float64(compUP) * 100)
				}
			} else if competitorPriceCents > 0 && offerPriceCents > 0 {
				savingsPercent = int(float64(competitorPriceCents-offerPriceCents) / float64(competitorPriceCents) * 100)
			}

			return &PendingSwap{
				DealID:            deal.ID,
				ItemID:            item.ID,
				OfferBrand:        deal.OfferBrand,
				OfferProduct:      deal.OfferProduct,
				CompetitorProduct: deal.CompetitorProduct,
				DiscountCents:     deal.DiscountCents,
				OfferPriceCents:    offerPriceCents,
				OriginalPriceCents: competitorPriceCents,
				OfferSize:          offerSize,
				OriginalSize:       competitorSize,
				OfferUnitPrice:     offerUnitPrice,
				OriginalUnitPrice:  competitorUnitPrice,
				SavingsPercent:     savingsPercent,
				WhyBullets:         whyBullets,
				AllergenWarning:    deal.AllergenNotes,
				DealType:           deal.DealType,
				QuantityOffered:    deal.QuantityOffered,
				TrialProductName:   deal.TrialProductName,
				TrialSize:          deal.TrialSize,
				CampaignTag:        deal.CampaignTag,
				EndsAt:             deal.EndsAt,
				StoreName:          storeName,
				ProductDescription: deal.ProductDescription,
				BrandStory:         deal.BrandStory,
				KeyBenefits:        parseKeyBenefits(deal.KeyBenefitsJSON),
				MaxRebateCents:     deal.MaxRebateCents,
				UPC:                deal.UPC,
			}
		}
	}

	return nil
}

// parseUnitPriceCents extracts cents from a unit price string like "$0.10/oz" → 10.
func parseUnitPriceCents(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "$")
	if idx := strings.Index(s, "/"); idx > 0 {
		s = s[:idx]
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int(f*100 + 0.5) // round to avoid float truncation (e.g., 0.29*100=28.99)
}

// parseKeyBenefits parses a JSON array of strings, returning nil on empty/invalid.
func parseKeyBenefits(jsonStr string) []string {
	if jsonStr == "" || jsonStr == "[]" {
		return nil
	}
	var benefits []string
	if err := json.Unmarshal([]byte(jsonStr), &benefits); err != nil {
		return nil
	}
	if len(benefits) == 0 {
		return nil
	}
	return benefits
}

