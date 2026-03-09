package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleLogoProxy(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	domain := r.URL.Query().Get("domain")
	brand := r.URL.Query().Get("brand")

	// If brand name provided, resolve to domain via brands table
	if brand != "" && domain == "" {
		var d string
		if err := s.db.QueryRow("SELECT domain FROM brands WHERE LOWER(name) = LOWER(?)", brand).Scan(&d); err == nil {
			domain = d
		} else {
			// Guess domain from brand name
			domain = strings.ToLower(brand)
			domain = strings.Map(func(r rune) rune {
				if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
					return r
				}
				return -1
			}, domain) + ".com"
		}
	}

	if domain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing domain or brand parameter"})
		return
	}
	// Sanitize: only allow alphanumeric, dots, hyphens
	for _, c := range domain {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-') {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid domain"})
			return
		}
	}

	cacheKey := strings.ToLower(domain)

	// Layer 1: in-memory cache
	if cached, ok := s.logoCache.Load(cacheKey); ok {
		cl := cached.(*cachedLogo)
		w.Header().Set("Content-Type", cl.contentType)
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", logoCacheMaxAge))
		w.Write(cl.data)
		return
	}

	// Layer 2: SQLite BLOB
	var logoData []byte
	var contentType string
	err := s.db.QueryRow(
		"SELECT bl.logo_data, bl.content_type FROM brand_logos bl JOIN brands b ON b.id = bl.brand_id WHERE LOWER(b.domain) = LOWER(?)",
		domain,
	).Scan(&logoData, &contentType)
	if err == nil && len(logoData) > 0 {
		s.logoCache.Store(cacheKey, &cachedLogo{data: logoData, contentType: contentType})
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", logoCacheMaxAge))
		w.Write(logoData)
		return
	}

	// Layer 3: fetch from external APIs
	var brandID int
	var brandName string
	s.db.QueryRow("SELECT id, name FROM brands WHERE LOWER(domain) = LOWER(?)", domain).Scan(&brandID, &brandName)

	logoData, contentType = fetchBrandLogo(domain, s.config.LogoDevSecret, s.config.LogoDevToken)
	if logoData != nil {
		// Store in SQLite if brand exists (async)
		if brandID > 0 {
			go func(id int, data []byte, ct string) {
				s.db.Exec(
					"INSERT OR REPLACE INTO brand_logos (brand_id, logo_data, content_type, fetched_at, source) VALUES (?, ?, ?, CURRENT_TIMESTAMP, 'logodev')",
					id, data, ct,
				)
			}(brandID, logoData, contentType)
		}
		s.logoCache.Store(cacheKey, &cachedLogo{data: logoData, contentType: contentType})
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", logoCacheMaxAge))
		w.Write(logoData)
		return
	}

	// Layer 4: SVG placeholder
	name := brandName
	if name == "" {
		name = strings.TrimSuffix(domain, ".com")
	}
	s.servePlaceholderLogo(w, name)
}

// fetchBrandLogo fetches a brand logo from Logo.dev.
//
// Logo.dev issues two credentials:
//   - Public token  (pk_...): validated by Referer header — only works in browsers
//     where the referring page matches a registered domain. Server-side Go HTTP
//     clients send no Referer, so Logo.dev rejects these requests with non-200.
//   - Secret key    (sk_...): bypasses Referer validation, intended for server-side
//     use. This is what we must use here.
//
// We accept both and prefer the secret key (sk_...) for the actual API call.
// The public token is accepted as a fallback in case the secret is unset but the
// token alone is configured (e.g. a plan that doesn't issue a secret key).
func fetchBrandLogo(domain, secretKey, publicToken string) ([]byte, string) {
	// Prefer secret key for server-side use (bypasses Referer validation).
	// Fall back to public token with Referer header as a best-effort.
	token := secretKey
	useReferer := false
	if token == "" {
		token = publicToken
		useReferer = true
	}
	if token == "" {
		return nil, ""
	}
	client := &http.Client{Timeout: 5 * time.Second}
	reqURL := "https://img.logo.dev/" + url.QueryEscape(domain) + "?token=" + url.QueryEscape(token) + "&size=128&format=png"
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, ""
	}
	if useReferer {
		req.Header.Set("Referer", "https://app.thrift.ly/")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		io.Copy(io.Discard, resp.Body) // drain for connection reuse
		return nil, ""
	}
	data, err := io.ReadAll(resp.Body)
	if err == nil && len(data) > 200 {
		ct := resp.Header.Get("Content-Type")
		if ct == "" {
			ct = "image/png"
		}
		return data, ct
	}
	return nil, ""
}

// servePlaceholderLogo returns an SVG with the brand's initials in a deterministic color.
func (s *Server) servePlaceholderLogo(w http.ResponseWriter, name string) {
	initials := logoInitials(name)
	color := logoColor(name)
	// Escape initials for safe SVG embedding
	safeInitials := strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;", "\"", "&quot;").Replace(initials)
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128">
  <rect width="128" height="128" rx="16" fill="%s"/>
  <text x="64" y="64" dominant-baseline="central" text-anchor="middle"
        font-family="system-ui,sans-serif" font-weight="600" font-size="48" fill="white">%s</text>
</svg>`, color, safeInitials)
	w.Header().Set("Content-Type", "image/svg+xml")
	// Short cache for placeholders so real logos replace them on next fetch
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(svg))
}

func logoInitials(name string) string {
	words := strings.Fields(name)
	if len(words) == 0 {
		return "?"
	}
	if len(words) == 1 {
		r := []rune(words[0])
		return strings.ToUpper(string(r[0:1]))
	}
	return strings.ToUpper(string([]rune(words[0])[0:1]) + string([]rune(words[1])[0:1]))
}

func logoColor(name string) string {
	colors := []string{"#4F46E5", "#7C3AED", "#DB2777", "#DC2626", "#EA580C", "#CA8A04", "#16A34A", "#0891B2"}
	h := 0
	for _, c := range name {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return colors[h%len(colors)]
}

// warmLogoCache pre-fetches logos for brands missing from the brand_logos table.
// Runs in the background after server starts.
func (s *Server) warmLogoCache() {
	// First, load all existing logos into memory
	rows, err := s.db.Query("SELECT b.domain, bl.logo_data, bl.content_type FROM brand_logos bl JOIN brands b ON b.id = bl.brand_id")
	if err == nil {
		count := 0
		for rows.Next() {
			var domain, ct string
			var data []byte
			if rows.Scan(&domain, &data, &ct) == nil {
				s.logoCache.Store(strings.ToLower(domain), &cachedLogo{data: data, contentType: ct})
				count++
			}
		}
		rows.Close() // close explicitly before next query to avoid holding cursor
		if count > 0 {
			log.Printf("[logos] loaded %d logos from database", count)
		}
	}

	// Collect missing logos first, then close the cursor before any HTTP I/O
	type missingBrand struct {
		id     int
		name   string
		domain string
	}
	var missing []missingBrand
	rows2, err := s.db.Query(`
		SELECT b.id, b.name, b.domain FROM brands b
		LEFT JOIN brand_logos bl ON bl.brand_id = b.id
		WHERE bl.brand_id IS NULL AND b.domain != ''
	`)
	if err != nil {
		return
	}
	for rows2.Next() {
		var mb missingBrand
		if rows2.Scan(&mb.id, &mb.name, &mb.domain) == nil {
			missing = append(missing, mb)
		}
	}
	rows2.Close()

	for _, mb := range missing {
		data, ct := fetchBrandLogo(mb.domain, s.config.LogoDevSecret, s.config.LogoDevToken)
		if data != nil {
			s.db.Exec(
				"INSERT OR REPLACE INTO brand_logos (brand_id, logo_data, content_type, fetched_at, source) VALUES (?, ?, ?, CURRENT_TIMESTAMP, 'logodev')",
				mb.id, data, ct,
			)
			s.logoCache.Store(strings.ToLower(mb.domain), &cachedLogo{data: data, contentType: ct})
			log.Printf("[logos] fetched logo for %s (%s)", mb.name, mb.domain)
		}
		time.Sleep(500 * time.Millisecond) // rate limit
	}
	log.Printf("[logos] warm-up complete")
}


// checkDealsForItems finds the first active, un-suppressed, in-budget deal that matches
// any of the added items. Slider/autotune settings gate whether deals are shown.
//
//	slider 0-33 = always show, 34-66 = show if discount > $1, 67-100 = never show
//	autotune ON: Penny decides based on accept/shown ratio (< 5% = stop showing)
func (s *Server) checkDealsForItems(userID int, addedItems []ListItem) *PendingSwap {
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
		log.Printf("[swaps] failed to load user %d settings: %v", userID, err)
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
		       COALESCE(brand_story, '')
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
		ID                  int
		CompetitorProduct   string
		OfferBrand          string
		OfferProduct        string
		DiscountCents       int
		BudgetRemaining     int
		OfferPriceCents     int
		CompetitorPriceCents int
		OfferSize           string
		CompetitorSize      string
		OfferUnitPrice      string
		CompetitorUnitPrice string
		WhyBulletsJSON      string
		AllergenNotes       string
		DealType            string
		QuantityOffered     int
		TrialProductName    string
		TrialSize           string
		CampaignTag         string
		EndsAt              string
		CategoryID          int
		ProductDescription  string
		KeyBenefitsJSON     string
		BrandStory          string
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
			&d.ProductDescription, &d.KeyBenefitsJSON, &d.BrandStory); err != nil {
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

			// Manual slider 34-66: only show if discount > $1
			if autotune == 0 && slider >= 34 && deal.DiscountCents <= 100 {
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
					log.Printf("[swaps] prediction filter: skipping deal %d for user %d (predicted acceptance %.1f%%, confidence %.2f)",
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

			// Calculate savings percent when both prices are available
			savingsPercent := 0
			if competitorPriceCents > 0 && offerPriceCents > 0 {
				savingsPercent = int(float64(competitorPriceCents-offerPriceCents) / float64(competitorPriceCents) * 100)
			}

			return &PendingSwap{
				DealID:             deal.ID,
				ItemID:             item.ID,
				OfferBrand:         deal.OfferBrand,
				OfferProduct:       deal.OfferProduct,
				DiscountCents:      deal.DiscountCents,
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
			}
		}
	}

	return nil
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

func (s *Server) handleBrands(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	rows, err := s.db.Query("SELECT name, domain FROM brands")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal error"})
		return
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var name, domain string
		if err := rows.Scan(&name, &domain); err != nil {
			continue
		}
		result[strings.ToLower(name)] = domain
	}
	if err := rows.Err(); err != nil {
		log.Printf("[brands] row iteration error: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(result)
}


func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	tail := strings.TrimPrefix(r.URL.Path, "/api/list")
	tail = strings.Trim(tail, "/")

	if tail == "parse" && r.Method == http.MethodPost {
		s.handleParseAndAdd(w, r)
		return
	}

	if tail == "" {
		switch r.Method {
		case http.MethodGet:
			s.handleListGet(w, r)
		case http.MethodPost:
			s.handleListAdd(w, r)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		}
		return
	}

	parts := strings.Split(tail, "/")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		return
	}

	if len(parts) == 2 && parts[1] == "toggle" && r.Method == http.MethodPatch {
		s.handleListToggle(w, r, id)
		return
	}

	if len(parts) == 2 && parts[1] == "urgent" && r.Method == http.MethodPatch {
		s.handleListUrgent(w, r, id)
		return
	}

	if len(parts) == 2 && parts[1] == "claim" && r.Method == http.MethodPatch {
		s.handleListClaim(w, r, id)
		return
	}

	if len(parts) == 2 && parts[1] == "history" && r.Method == http.MethodGet {
		s.handleItemHistory(w, r, id)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodPatch {
		s.handleListEdit(w, r, id)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodDelete {
		s.handleListDelete(w, r, id)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
}

// handleParseAndAdd handles POST /api/list/parse
// Request:  { "text": "milk eggs sourdough chicken", "image": "data:image/jpeg;base64,..." }
// Response: ParseResponse with items, added count, and updated count
func (s *Server) handleParseAndAdd(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024) // 10 MB limit

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Per-user rate limit: 30 parse requests per hour
	if checkRateLimitBucket(&s.parseLimiter, session.UserID, time.Hour, 30) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many requests — please wait a few minutes"})
		return
	}

	var req struct {
		Text            string   `json:"text"`
		Image           string   `json:"image"`
		Images          []string `json:"images"`
		SwapDealID      int      `json:"swap_deal_id"`
		SwapItemID      int      `json:"swap_item_id"`
		IsReceipt       bool     `json:"is_receipt"`
		DeclineReason   string   `json:"decline_reason"`    // optional: tried_before, dietary, brand_loyal, price, other
		TimeToActionMs  int      `json:"time_to_action_ms"` // ms from swap card render to accept/decline tap
		WhyTapped       bool     `json:"why_tapped"`        // true if user tapped "Why?" before deciding
	}
	// Decode directly — don't use decodeJSON which would override our 10MB limit with 1MB
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := "Invalid request body"
		if err.Error() == "http: request body too large" {
			msg = "Request too large"
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	req.Image = strings.TrimSpace(req.Image)
	// Merge: support both single image and images array
	images := req.Images
	if len(images) == 0 && req.Image != "" {
		images = []string{req.Image}
	}
	// Cap images to prevent memory exhaustion
	if len(images) > 5 {
		images = images[:5]
	}
	// =========================================================================
	// SWAP ACCEPT/DECLINE — short-circuit before Claude call
	// =========================================================================
	if req.SwapDealID > 0 {
		hhIDs, _ := getHouseholdUserIDs(s.db, session.UserID)
		whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID, hhIDs)
		resp := ParseResponse{Items: []ListItem{}, Skipped: []SkippedItem{}}

		if req.Text == "accept_swap" {
			// 1-swap-per-user-per-campaign: prevent accepting the same deal more than once (B14)
			var priorAccepts int
			s.db.QueryRow(
				"SELECT COUNT(*) FROM swap_events WHERE user_id = ? AND deal_id = ? AND action = 'accepted'",
				session.UserID, req.SwapDealID,
			).Scan(&priorAccepts)
			if priorAccepts > 0 {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "Swap already accepted"})
				return
			}

			// Wrap entire accept in a transaction — budget check + mutations are atomic (B11 fix)
			tx, err := s.db.Begin()
			if err != nil {
				log.Printf("[swaps] failed to begin tx: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process swap"})
				return
			}
			defer tx.Rollback()

			// Verify deal is active and read deal data — inside tx for atomicity
			var offerBrand, offerProduct, status, dealType, trialProductName, trialSize string
			var discountCents, budgetTotal, budgetSpent, quantityOffered int
			err = tx.QueryRow(
				`SELECT offer_brand, offer_product, discount_cents, total_budget_cents, budget_spent_cents, status,
				        deal_type, quantity_offered, trial_product_name, trial_size
				 FROM brand_deals WHERE id = ?`,
				req.SwapDealID,
			).Scan(&offerBrand, &offerProduct, &discountCents, &budgetTotal, &budgetSpent, &status,
				&dealType, &quantityOffered, &trialProductName, &trialSize)
			if err != nil || status != "active" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Deal no longer available"})
				return
			}
			if budgetTotal > 0 && budgetSpent >= budgetTotal {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Deal budget exhausted"})
				return
			}

			// Atomic budget increment — conditional UPDATE prevents overspend under concurrency
			// Only increments if budget_spent_cents + discount would not exceed the total
			if budgetTotal > 0 {
				result, err := tx.Exec(
					"UPDATE brand_deals SET budget_spent_cents = budget_spent_cents + ? WHERE id = ? AND (total_budget_cents = 0 OR budget_spent_cents + ? <= total_budget_cents)",
					discountCents, req.SwapDealID, discountCents,
				)
				if err != nil {
					log.Printf("[swaps] failed to update deal %d budget: %v", req.SwapDealID, err)
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process swap"})
					return
				}
				rows, _ := result.RowsAffected()
				if rows == 0 {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Deal budget exhausted"})
					return
				}
			} else {
				// No budget cap — just increment
				if _, err := tx.Exec("UPDATE brand_deals SET budget_spent_cents = budget_spent_cents + ? WHERE id = ?",
					discountCents, req.SwapDealID); err != nil {
					log.Printf("[swaps] failed to update deal %d budget: %v", req.SwapDealID, err)
				}
			}

			// Apply deal to list — behavior varies by deal_type
			if req.SwapItemID > 0 {
				switch dealType {
				case "bogo":
					// BOGO: keep original item but set quantity to quantity_offered (typically 2)
					qty := quantityOffered
					if qty < 2 {
						qty = 2
					}
					updateArgs := append([]interface{}{offerProduct, offerBrand, qty, req.SwapItemID}, whereArgs...)
					_, err = tx.Exec(
						"UPDATE list_items SET name = ?, brand = ?, quantity = ? WHERE id = ? AND "+whereClause,
						updateArgs...,
					)
					if err != nil {
						log.Printf("[swaps] bogo accept update error: %v", err)
					}
				case "trial":
					// Trial: keep original item unchanged, add trial product as new item
					trialResult, trialErr := tx.Exec(
						"INSERT INTO list_items (user_id, name, brand, size, quantity, department, source, swap_savings_cents) VALUES (?, ?, ?, ?, 1, 'Other', 'swap', ?)",
						session.UserID, trialProductName, offerBrand, trialSize, discountCents,
					)
					if trialErr != nil {
						log.Printf("[swaps] trial accept insert error: %v", trialErr)
					}
					_ = trialResult
				default:
					// sku_swap / seasonal: replace item on list
					updateArgs := append([]interface{}{offerProduct, offerBrand, req.SwapItemID}, whereArgs...)
					_, err = tx.Exec(
						"UPDATE list_items SET name = ?, brand = ? WHERE id = ? AND "+whereClause,
						updateArgs...,
					)
					if err != nil {
						log.Printf("[swaps] accept update error: %v", err)
					}
				}

				// Record savings on the swapped list item so household members see "Saved $X via Penny"
				if dealType != "trial" {
					savingsArgs := append([]interface{}{discountCents, req.SwapItemID}, whereArgs...)
					if _, err := tx.Exec(
						"UPDATE list_items SET swap_savings_cents = ? WHERE id = ? AND "+whereClause,
						savingsArgs...,
					); err != nil {
						log.Printf("[swaps] failed to set swap_savings_cents on item %d: %v", req.SwapItemID, err)
					}
				}
			}

			// Log accepted event
			whyTappedInt := 0
			if req.WhyTapped {
				whyTappedInt = 1
			}
			result, err := tx.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action, time_to_action_ms, why_tapped) VALUES (?, ?, ?, 'accepted', ?, ?)",
				session.UserID, req.SwapDealID, req.SwapItemID, req.TimeToActionMs, whyTappedInt)
			if err != nil {
				log.Printf("[swaps] failed to insert swap event: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process swap"})
				return
			}
			swapEventID, _ := result.LastInsertId()

			// Insert rebate ledger entry
			if _, err := tx.Exec(
				"INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES (?, ?, ?, ?, 'pending')",
				session.UserID, swapEventID, req.SwapDealID, discountCents,
			); err != nil {
				log.Printf("[swaps] failed to insert rebate: %v", err)
			}

			// Update user counters and rebate balance
			if _, err := tx.Exec("UPDATE users SET deal_accept_count = deal_accept_count + 1, swap_credits_cents = swap_credits_cents + ? WHERE id = ?",
				discountCents, session.UserID); err != nil {
				log.Printf("[swaps] failed to update user credits for user %d: %v", session.UserID, err)
			}

			// Auto-pause deal if budget now exhausted (re-read inside tx)
			var newBudgetSpent int
			tx.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", req.SwapDealID).Scan(&newBudgetSpent)
			if budgetTotal > 0 && newBudgetSpent >= budgetTotal {
				if _, err := tx.Exec("UPDATE brand_deals SET status = 'paused' WHERE id = ?", req.SwapDealID); err != nil {
					log.Printf("[swaps] failed to auto-pause deal %d: %v", req.SwapDealID, err)
				}
				log.Printf("[swaps] deal %d auto-paused: budget exhausted", req.SwapDealID)
			}

			if err := tx.Commit(); err != nil {
				log.Printf("[swaps] failed to commit accept tx: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process swap"})
				return
			}

			// Read updated balance for cumulative total in response
			var newBalance int
			s.db.QueryRow("SELECT swap_credits_cents FROM users WHERE id = ?", session.UserID).Scan(&newBalance)

			// Record learning signal
			go s.recordSignal(session.UserID, "swap_accepted", offerBrand, fmt.Sprintf("%d", discountCents), map[string]interface{}{
				"deal_id": req.SwapDealID, "deal_type": dealType, "discount_cents": discountCents,
			})

			resp.Action = "swap_accepted"
			resp.SwapCreditsCents = newBalance
			switch dealType {
			case "bogo":
				resp.Message = fmt.Sprintf("BOGO! Get %d for the price of 1. You save $%.2f. ($%.2f total saved!)", quantityOffered, float64(discountCents)/100, float64(newBalance)/100)
			case "trial":
				resp.Message = fmt.Sprintf("Free trial added! %s (%s). You save $%.2f. ($%.2f total saved!)", trialProductName, trialSize, float64(discountCents)/100, float64(newBalance)/100)
			default:
				resp.Message = fmt.Sprintf("Swapped! You save $%.2f. ($%.2f total saved!)", float64(discountCents)/100, float64(newBalance)/100)
			}
			if req.SwapItemID > 0 {
				if item, err := getItemByID(s.db, req.SwapItemID, session.UserID); err == nil {
					resp.Updated = []ListItem{item}
				}
			}
		} else {
			// decline_swap
			declineWhyTapped := 0
			if req.WhyTapped {
				declineWhyTapped = 1
			}
			s.db.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action, decline_reason, time_to_action_ms, why_tapped) VALUES (?, ?, ?, 'declined', ?, ?, ?)",
				session.UserID, req.SwapDealID, req.SwapItemID, req.DeclineReason, req.TimeToActionMs, declineWhyTapped)
			s.db.Exec(`INSERT INTO swap_suppressions (user_id, deal_id, decline_count) VALUES (?, ?, 1)
				ON CONFLICT(user_id, deal_id) DO UPDATE SET decline_count = decline_count + 1`,
				session.UserID, req.SwapDealID)

			go s.recordSignal(session.UserID, "swap_declined", "", req.DeclineReason, map[string]interface{}{
				"deal_id": req.SwapDealID, "reason": req.DeclineReason,
			})

			resp.Action = "swap_declined"
			resp.Message = "No problem, keeping your original choice."
		}

		writeJSON(w, http.StatusCreated, resp)
		return
	}

	// =========================================================================
	// RECEIPT PHOTO — cross-reference receipt against list
	// =========================================================================
	if req.IsReceipt && len(images) > 0 {
		if s.config.AnthropicAPIKey == "" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI parsing not configured"})
			return
		}

		// Rate limit: 3/day, 10/week
		if checkReceiptRateLimit(session.UserID) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "You've reached the receipt scanning limit. Try again tomorrow."})
			return
		}

		// SHA-256 dedup check
		hasher := sha256.New()
		for _, img := range images {
			hasher.Write([]byte(img))
		}
		imageHash := hex.EncodeToString(hasher.Sum(nil))

		var existingReceiptID int
		if err := s.db.QueryRow("SELECT id FROM receipts WHERE user_id = ? AND image_hash = ?",
			session.UserID, imageHash).Scan(&existingReceiptID); err == nil {
			writeJSON(w, http.StatusOK, ParseResponse{
				Items:   []ListItem{},
				Skipped: []SkippedItem{},
				Action:  "receipt_duplicate",
				Message: "Looks like I've already processed this receipt!",
			})
			return
		}

		// Load current list items
		hhIDs, _ := getHouseholdUserIDs(s.db, session.UserID)
		whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID, hhIDs)
		rows, err := s.db.Query(
			"SELECT "+listItemColumns+" FROM "+listItemJoins+" WHERE li."+whereClause+" AND li.checked = 0 ORDER BY li.name",
			whereArgs...,
		)
		var currentItems []ListItem
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				item, err := scanListItem(rows)
				if err == nil {
					currentItems = append(currentItems, item)
				}
			}
		}

		// Parse receipt
		receiptParser := s.parseReceipt
		if receiptParser == nil {
			receiptParser = parseReceiptWithClaude
		}
		matchedNames, err := receiptParser(s.config.AnthropicAPIKey, images, currentItems)
		if err != nil {
			log.Printf("[receipt] parse error for user %d: %v", session.UserID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Couldn't read the receipt. Try again?"})
			return
		}

		// Check off matched items
		var checkedItems []ListItem
		for _, matchName := range matchedNames {
			for _, item := range currentItems {
				if strings.EqualFold(item.Name, matchName) && !item.Checked {
					s.db.Exec("UPDATE list_items SET checked = 1 WHERE id = ?", item.ID)
					recordModification(s.db, item.ID, session.UserID, "checked", "receipt scan", "app")
					item.Checked = true
					checkedItems = append(checkedItems, item)
					break
				}
			}
		}

		// Insert receipt record
		result, insertErr := s.db.Exec("INSERT INTO receipts (user_id, image_hash, items_found, items_matched) VALUES (?, ?, ?, ?)",
			session.UserID, imageHash, len(matchedNames), len(checkedItems))

		// Enhanced receipt processing (structured extraction + swap verification) — async
		if insertErr == nil {
			receiptID64, _ := result.LastInsertId()
			receiptID := int(receiptID64)
			go s.processReceiptEnhanced(session.UserID, receiptID, matchedNames, images)
		}

		resp := ParseResponse{
			Items:   []ListItem{},
			Skipped: []SkippedItem{},
			Checked: checkedItems,
			Action:  "receipt_processed",
		}
		if len(checkedItems) > 0 {
			resp.Message = fmt.Sprintf("Found %d items on your receipt — checked them off!", len(checkedItems))
		} else {
			resp.Message = "I couldn't match any receipt items to your list. You might have already checked them off!"
		}
		writeJSON(w, http.StatusCreated, resp)
		return
	}

	if req.Text == "" && len(images) == 0 {
		log.Printf("[parse] empty text and images")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Text or image is required"})
		return
	}
	if s.config.AnthropicAPIKey == "" {
		log.Printf("[parse] no API key configured")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI parsing not configured"})
		return
	}

	log.Printf("Parsing grocery text for user %d: len=%d (images: %d)", session.UserID, len(req.Text), len(images))

	// Cache household IDs once for this request to avoid repeated queries
	hhIDs, _ := getHouseholdUserIDs(s.db, session.UserID)

	// For text-only requests, use the command parser which supports natural language operations
	if len(images) == 0 && req.Text != "" {
		// Get current list items so the LLM can reference them
		whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID, hhIDs)
		rows, err := s.db.Query(
			"SELECT "+listItemColumns+" FROM "+listItemJoins+" WHERE li."+whereClause+" ORDER BY li.name",
			whereArgs...,
		)
		var currentItems []ListItem
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				item, err := scanListItem(rows)
				if err == nil {
					currentItems = append(currentItems, item)
				}
			}
		}

		// Get user name for personalized messages
		var userName string
		s.db.QueryRow("SELECT COALESCE(name,'') FROM users WHERE id = ?", session.UserID).Scan(&userName)
		firstName := strings.Split(strings.TrimSpace(userName), " ")[0]

		// Build learning context for Penny's system prompt
		learningCtx := s.buildLearningContext(session.UserID)

		cmd, err := s.parseCommand(s.config.AnthropicAPIKey, req.Text, currentItems, firstName, learningCtx)
		if err != nil {
			log.Printf("[parse] command error for user %d: %v", session.UserID, err)
			dbError(w, err, "I couldn't understand that. Try again?")
			return
		}

		resp := ParseResponse{
			Action:  cmd.Action,
			Message: cmd.Message,
			Items:   []ListItem{},
			Skipped: []SkippedItem{},
		}

		switch cmd.Action {
		case "remove":
			for _, p := range cmd.Items {
				if p.Name == "" {
					continue
				}
				// Find matching item on the list (case-insensitive)
				for _, existing := range currentItems {
					if strings.EqualFold(existing.Name, p.Name) || strings.EqualFold(existing.Brand+" "+existing.Name, p.Name) {
						// ON DELETE CASCADE handles item_modifications cleanup
						delArgs := append([]interface{}{existing.ID}, whereArgs...)
						if _, err := s.db.Exec("DELETE FROM list_items WHERE id = ? AND "+whereClause, delArgs...); err != nil {
							log.Printf("[parse] remove item error: %v", err)
						}
						resp.Removed = append(resp.Removed, existing)
						break
					}
				}
			}
		case "check":
			for _, p := range cmd.Items {
				if p.Name == "" {
					continue
				}
				for _, existing := range currentItems {
					if strings.EqualFold(existing.Name, p.Name) && !existing.Checked {
						if _, err := s.db.Exec("UPDATE list_items SET checked = 1 WHERE id = ?", existing.ID); err != nil {
							log.Printf("[parse] check item error: %v", err)
						}
						existing.Checked = true
						resp.Checked = append(resp.Checked, existing)
						break
					}
				}
			}
		case "uncheck":
			for _, p := range cmd.Items {
				if p.Name == "" {
					continue
				}
				for _, existing := range currentItems {
					if strings.EqualFold(existing.Name, p.Name) && existing.Checked {
						if _, err := s.db.Exec("UPDATE list_items SET checked = 0 WHERE id = ?", existing.ID); err != nil {
							log.Printf("[parse] uncheck item error: %v", err)
						}
						existing.Checked = false
						resp.Checked = append(resp.Checked, existing)
						break
					}
				}
			}
		case "update":
			for _, p := range cmd.Items {
				if p.Name == "" {
					continue
				}
				for _, existing := range currentItems {
					if strings.EqualFold(existing.Name, p.Name) {
						var setClauses []string
						var setArgs []interface{}
						if p.NewName != "" && !strings.EqualFold(p.NewName, existing.Name) {
							setClauses = append(setClauses, "name = ?")
							setArgs = append(setArgs, p.NewName)
						}
						if p.Quantity > 0 {
							setClauses = append(setClauses, "quantity = ?")
							setArgs = append(setArgs, p.Quantity)
						}
						if p.Size != "" {
							setClauses = append(setClauses, "size = ?")
							setArgs = append(setArgs, p.Size)
						}
						if p.Brand != "" {
							setClauses = append(setClauses, "brand = ?")
							setArgs = append(setArgs, p.Brand)
						}
						if p.Department != "" {
							setClauses = append(setClauses, "department = ?")
							setArgs = append(setArgs, p.Department)
						}
						if p.Notes != "" {
							setClauses = append(setClauses, "notes = ?")
							setArgs = append(setArgs, p.Notes)
						}
						if len(setClauses) > 0 {
							query := "UPDATE list_items SET " + strings.Join(setClauses, ", ") + " WHERE id = ? AND " + whereClause
							setArgs = append(setArgs, existing.ID)
							setArgs = append(setArgs, whereArgs...)
							if _, err := s.db.Exec(query, setArgs...); err != nil {
								log.Printf("[parse] update item error: %v", err)
							}
							updated, err := getItemByID(s.db, existing.ID, session.UserID)
							if err == nil {
								resp.Updated = append(resp.Updated, updated)
							}
						}
						break
					}
				}
			}
		case "save":
			name := cmd.SaveName
			if name == "" {
				name = "Saved List"
			}
			itemsToSave, err := collectListItems(s.db, session.UserID)
			if err == nil && len(itemsToSave) > 0 {
				tx, err := s.db.Begin()
				if err == nil {
					result, err := tx.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, ?)", session.UserID, name)
					if err == nil {
						savedListID, _ := result.LastInsertId()
						insertSavedListItems(tx, savedListID, itemsToSave)
						tx.Commit()
					} else {
						tx.Rollback()
					}
				}
			}
		case "clear":
			// Don't auto-delete — let the frontend confirm what to clear.
			// Just pass the action through so the UI can present options.
			resp.Action = "clear"
		case "restore":
			var stashJSON string
			if err := s.db.QueryRow("SELECT COALESCE(last_cleared_items,'[]') FROM users WHERE id = ?", session.UserID).Scan(&stashJSON); err == nil && stashJSON != "[]" {
				var stashed []ParsedItem
				if json.Unmarshal([]byte(stashJSON), &stashed) == nil && len(stashed) > 0 {
					added, _ := addParsedItems(s.db, session.UserID, stashed, "app")
					resp.Items = added
					resp.Added = len(added)
					s.db.Exec("UPDATE users SET last_cleared_items = '[]' WHERE id = ?", session.UserID)
					resp.Message = cmd.Message
				} else {
					resp.Message = "I don't have anything to restore right now."
				}
			} else {
				resp.Message = "I don't have anything to restore right now."
			}
		case "share":
			resp.Action = "share"
			resp.Message = cmd.Message
		case "load_saved_list":
			resp.Action = "load_saved_list"
			// Query user's saved lists (household-scoped)
			slWhereClause, slWhereArgs := householdItemsWhereClause(s.db, session.UserID, hhIDs)
			slRows, slErr := s.db.Query(
				"SELECT sl.id, sl.name, COUNT(sli.id) FROM saved_lists sl LEFT JOIN saved_list_items sli ON sli.saved_list_id = sl.id WHERE sl."+slWhereClause+" GROUP BY sl.id ORDER BY sl.name",
				slWhereArgs...,
			)
			if slErr == nil {
				defer slRows.Close()
				for slRows.Next() {
					var sl SavedListSummary
					if err := slRows.Scan(&sl.ID, &sl.Name, &sl.ItemCount); err == nil {
						resp.SavedLists = append(resp.SavedLists, sl)
					}
				}
			}
			if len(resp.SavedLists) == 0 {
				resp.Message = "You don't have any saved lists yet. Tell me to save your current list and give it a name!"
			} else if cmd.SavedListName != "" {
				// Try to match a specific list by name (case-insensitive)
				targetName := strings.ToLower(strings.TrimSpace(cmd.SavedListName))
				var matchedList *SavedListSummary
				for i := range resp.SavedLists {
					if strings.ToLower(resp.SavedLists[i].Name) == targetName {
						matchedList = &resp.SavedLists[i]
						break
					}
				}
				if matchedList == nil {
					// Fuzzy match — check if name contains the search term
					for i := range resp.SavedLists {
						if strings.Contains(strings.ToLower(resp.SavedLists[i].Name), targetName) {
							matchedList = &resp.SavedLists[i]
							break
						}
					}
				}
				if matchedList != nil {
					// Auto-load the matched list using upsertListItem for dupe prevention
					loadRows, loadErr := s.db.Query(
						"SELECT name, brand, department, quantity, size, notes FROM saved_list_items WHERE saved_list_id = ?",
						matchedList.ID,
					)
					if loadErr == nil {
						var savedItems []itemData
						for loadRows.Next() {
							var si itemData
							if err := loadRows.Scan(&si.Name, &si.Brand, &si.Department, &si.Quantity, &si.Size, &si.Notes); err == nil {
								savedItems = append(savedItems, si)
							}
						}
						loadRows.Close()
						added := 0
						for _, si := range savedItems {
							_, _, err := upsertListItem(s.db, session.UserID, si.Name, si.Brand, si.Department, si.Size, si.Quantity, "app", "")
							if err == nil {
								added++
							}
						}
						resp.Added = added
						resp.Message = cmd.Message
					}
				} else {
					resp.Message = fmt.Sprintf("I couldn't find a list called %q. Here are your saved lists — pick one!", cmd.SavedListName)
				}
			} else {
				resp.Message = cmd.Message
			}
		case "delete_saved_list":
			resp.Action = "delete_saved_list"
			if cmd.SavedListName == "" {
				resp.Message = "Which saved list would you like me to delete? Just tell me the name."
			} else {
				targetName := strings.ToLower(strings.TrimSpace(cmd.SavedListName))
				slWhereClause, slWhereArgs := householdItemsWhereClause(s.db, session.UserID, hhIDs)
				var matchedID int
				var matchedName string
				rows, err := s.db.Query(
					"SELECT id, name FROM saved_lists WHERE "+slWhereClause,
					slWhereArgs...,
				)
				if err == nil {
					defer rows.Close()
					for rows.Next() {
						var id int
						var name string
						if err := rows.Scan(&id, &name); err == nil {
							if strings.ToLower(name) == targetName || strings.Contains(strings.ToLower(name), targetName) {
								matchedID = id
								matchedName = name
								break
							}
						}
					}
				}
				if matchedID > 0 {
					s.db.Exec("DELETE FROM saved_lists WHERE id = ?", matchedID)
					resp.Message = fmt.Sprintf("Deleted your %q list!", matchedName)
				} else {
					resp.Message = fmt.Sprintf("I couldn't find a saved list called %q.", cmd.SavedListName)
				}
			}
		case "done_shopping":
			resp.Action = "done_shopping"
			resp.Message = cmd.Message
			if resp.Message == "" {
				resp.Message = "Nice trip! Want to snap a photo of your receipt? I'll check off what you got."
			}
		case "search":
			resp.Action = "search"
			resp.Message = cmd.Message
			// Search may also include items to add (e.g., recipe ingredients)
			if len(cmd.Items) > 0 {
				added, skippedItems := addParsedItems(s.db, session.UserID, cmd.Items, "app")
				resp.Items = added
				resp.Skipped = skippedItems
				resp.Added = len(added)
			}
		case "update_preference":
			for _, item := range cmd.Items {
				if item.Brand != "" && item.Name != "" {
					category := strings.ToLower(strings.TrimSpace(item.Name))
					recordBrandChoice(s.db, session.UserID, category, item.Brand, "command")
					// Force streak to 3 for explicit intent
					s.db.Exec("UPDATE brand_preferences SET streak = 3 WHERE user_id = ? AND product_category = ?",
						session.UserID, category)
					// Also update existing unchecked items matching this category
					updateArgs := append(append([]interface{}{item.Brand}, whereArgs...), "%"+category+"%")
					s.db.Exec("UPDATE list_items SET brand = ? WHERE "+whereClause+" AND checked = 0 AND LOWER(name) LIKE ?",
						updateArgs...)
				}
			}
			resp.Message = cmd.Message
		default:
			// Default "add" action — use the existing addParsedItems flow
			added, skippedItems := addParsedItems(s.db, session.UserID, cmd.Items, "app")
			resp.Items = added
			resp.Skipped = skippedItems
			resp.Added = len(added)

			// Resolve brand alternatives using preferences
			var pennyBrandMode string
			s.db.QueryRow("SELECT COALESCE(penny_brand_mode,'ask') FROM users WHERE id = ?", session.UserID).Scan(&pennyBrandMode)
			// A/B test: brand_defaults variant gets global popularity defaults
			abVariant, _ := getVariant(s.db, "onboarding_v2", session.UserID)
			useDefaults := abVariant == "brand_defaults"
			disambiguations := resolveBrandAlternatives(s.db, session.UserID, pennyBrandMode, added, cmd.Items, useDefaults)
			if len(disambiguations) > 0 {
				resp.BrandDisambiguations = disambiguations
			}

			// Check for matching swap deals on newly added items
			if swap := s.checkDealsForItems(session.UserID, added); swap != nil {
				resp.PendingSwap = swap
			}

			// Check household duplicate items
			if dupes := s.checkHouseholdDuplicates(session.UserID, added); len(dupes) > 0 {
				resp.DuplicateWarnings = dupes
			}
		}

		if resp.Items == nil {
			resp.Items = []ListItem{}
		}
		if resp.Skipped == nil {
			resp.Skipped = []SkippedItem{}
		}
		writeJSON(w, http.StatusCreated, resp)
		return
	}

	// Image-based requests: use the existing item parser
	parsed, err := s.cachedParseItems(s.config.AnthropicAPIKey, req.Text, images)
	if err != nil {
		log.Printf("[parse] error for user %d: %v", session.UserID, err)
		dbError(w, err, "We couldn't understand that right now. Please try again.")
		return
	}

	added, skippedItems := addParsedItems(s.db, session.UserID, parsed, "app")

	// Resolve brand alternatives for image-based parsing too
	var imgBrandMode string
	s.db.QueryRow("SELECT COALESCE(penny_brand_mode,'ask') FROM users WHERE id = ?", session.UserID).Scan(&imgBrandMode)
	imgABVariant, _ := getVariant(s.db, "onboarding_v2", session.UserID)
	imgDisambiguations := resolveBrandAlternatives(s.db, session.UserID, imgBrandMode, added, parsed, imgABVariant == "brand_defaults")

	resp := ParseResponse{
		Items:   added,
		Skipped: skippedItems,
		Added:   len(added),
	}
	if len(imgDisambiguations) > 0 {
		resp.BrandDisambiguations = imgDisambiguations
	}

	// Check for matching swap deals on newly added items
	if swap := s.checkDealsForItems(session.UserID, added); swap != nil {
		resp.PendingSwap = swap
	}

	// Check household duplicate items
	if dupes := s.checkHouseholdDuplicates(session.UserID, added); len(dupes) > 0 {
		resp.DuplicateWarnings = dupes
	}

	if resp.Items == nil {
		resp.Items = []ListItem{}
	}
	if resp.Skipped == nil {
		resp.Skipped = []SkippedItem{}
	}

	// Record learning signals for added items (async — never block the response)
	if len(added) > 0 {
		go func(uid int, items []ListItem) {
			for _, item := range items {
				s.recordSignal(uid, "item_added", item.Department, item.Brand, map[string]interface{}{
					"name": item.Name, "source": item.Source,
				})
			}
		}(session.UserID, added)
	}

	writeJSON(w, http.StatusCreated, resp)
}

// handleListGet handles GET /api/list
func (s *Server) handleListGet(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	rows, err := s.db.Query(
		"SELECT "+listItemColumns+" FROM "+listItemJoins+" WHERE li."+whereClause+" ORDER BY li.name",
		whereArgs...,
	)
	if err != nil {
		dbError(w, err, "We couldn't load your list. Please try again.")
		return
	}
	defer rows.Close()

	items := []ListItem{}
	for rows.Next() {
		item, err := scanListItem(rows)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[list] row iteration error: %v", err)
	}

	writeJSON(w, http.StatusOK, items)
}

// handleListSwaps returns pending (un-actioned) swap offers for the user's list items.
// A swap is pending if it was offered but not yet accepted or declined.
func (s *Server) handleListSwaps(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT se.item_id, se.deal_id,
		       bd.offer_brand, bd.offer_product, bd.discount_cents,
		       bd.offer_price_cents, bd.competitor_price_cents,
		       bd.offer_size, bd.competitor_size,
		       bd.offer_unit_price, bd.competitor_unit_price,
		       bd.why_bullets, bd.allergen_notes,
		       bd.deal_type, bd.quantity_offered, bd.trial_product_name, bd.trial_size, bd.campaign_tag,
		       COALESCE(bd.ends_at,'')
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE se.user_id = ? AND se.action = 'offered'
		  AND bd.status = 'active'
		  AND NOT EXISTS (
		    SELECT 1 FROM swap_events se2
		    WHERE se2.user_id = se.user_id AND se2.deal_id = se.deal_id AND se2.item_id = se.item_id
		      AND se2.action IN ('accepted', 'declined')
		      AND se2.id > se.id
		  )
		  AND se.item_id IN (SELECT id FROM list_items WHERE checked = 0)
		ORDER BY se.created_at DESC
	`, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load swaps"})
		return
	}
	defer rows.Close()

	swaps := []PendingSwap{}
	for rows.Next() {
		var ps PendingSwap
		var whyJSON, allergen string
		if err := rows.Scan(&ps.ItemID, &ps.DealID,
			&ps.OfferBrand, &ps.OfferProduct, &ps.DiscountCents,
			&ps.OfferPriceCents, &ps.OriginalPriceCents,
			&ps.OfferSize, &ps.OriginalSize,
			&ps.OfferUnitPrice, &ps.OriginalUnitPrice,
			&whyJSON, &allergen,
			&ps.DealType, &ps.QuantityOffered, &ps.TrialProductName, &ps.TrialSize, &ps.CampaignTag,
			&ps.EndsAt,
		); err != nil {
			log.Printf("[swaps] scan error: %v", err)
			continue
		}
		if whyJSON != "" && whyJSON != "[]" {
			json.Unmarshal([]byte(whyJSON), &ps.WhyBullets)
		}
		ps.AllergenWarning = allergen
		if ps.OriginalPriceCents > 0 && ps.OfferPriceCents > 0 {
			ps.SavingsPercent = int(float64(ps.OriginalPriceCents-ps.OfferPriceCents) / float64(ps.OriginalPriceCents) * 100)
		}
		swaps = append(swaps, ps)
	}

	writeJSON(w, http.StatusOK, swaps)
}

// handleListAdd handles POST /api/list
// Request: { "name": "Milk", "department": "Dairy & Eggs" }
func (s *Server) handleListAdd(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Name       string `json:"name"`
		Department string `json:"department"`
		Quantity   int    `json:"quantity"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Name is required"})
		return
	}
	if len(req.Name) > 200 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Item name is too long"})
		return
	}
	if req.Department == "" {
		req.Department = "Other"
	}
	if req.Quantity < 1 {
		req.Quantity = 1
	}

	item, _, err := upsertListItem(s.db, session.UserID, req.Name, "", req.Department, "", req.Quantity, "app", "")
	if err != nil {
		dbError(w, err, "We couldn't save your item right now. Please try again.")
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

// handleListToggle handles PATCH /api/list/{id}/toggle
func (s *Server) handleListToggle(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	toggleArgs := append([]interface{}{id}, whereArgs...)
	_, err := s.db.Exec(
		"UPDATE list_items SET checked = NOT checked WHERE id = ? AND "+whereClause,
		toggleArgs...,
	)
	if err != nil {
		dbError(w, err, "We couldn't save your item right now. Please try again.")
		return
	}

	item, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}

	action := "checked"
	if !item.Checked {
		action = "unchecked"
	}
	recordModification(s.db, id, session.UserID, action, "", "app")

	writeJSON(w, http.StatusOK, item)
}

// handleListUrgent handles PATCH /api/list/{id}/urgent
func (s *Server) handleListUrgent(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	urgentArgs := append([]interface{}{id}, whereArgs...)
	_, err := s.db.Exec(
		"UPDATE list_items SET urgent = NOT urgent WHERE id = ? AND "+whereClause,
		urgentArgs...,
	)
	if err != nil {
		dbError(w, err, "We couldn't save your item right now. Please try again.")
		return
	}

	item, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}

	action := "marked urgent"
	if !item.Urgent {
		action = "unmarked urgent"
	}
	recordModification(s.db, id, session.UserID, action, "", "app")

	writeJSON(w, http.StatusOK, item)
}

// handleListClaim handles PATCH /api/list/{id}/claim
func (s *Server) handleListClaim(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Check item exists in household
	item, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}

	if item.ClaimedByID != nil && *item.ClaimedByID == session.UserID {
		// Unclaim
		_, err = s.db.Exec("UPDATE list_items SET claimed_by = NULL, claimed_at = NULL WHERE id = ?", id)
		if err != nil {
			dbError(w, err, "We couldn't save your item right now. Please try again.")
			return
		}
		recordModification(s.db, id, session.UserID, "unclaimed", "", "app")
	} else {
		// Claim (or override)
		_, err = s.db.Exec("UPDATE list_items SET claimed_by = ?, claimed_at = CURRENT_TIMESTAMP WHERE id = ?", session.UserID, id)
		if err != nil {
			dbError(w, err, "We couldn't save your item right now. Please try again.")
			return
		}
		recordModification(s.db, id, session.UserID, "claimed", "", "app")
	}

	updated, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// handleItemHistory handles GET /api/list/{id}/history
func (s *Server) handleItemHistory(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Verify item belongs to household
	_, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}

	rows, err := s.db.Query(
		"SELECT m.action, m.detail, m.source, m.created_at, COALESCE(u.name, u.phone_number, '') FROM item_modifications m LEFT JOIN users u ON u.id = m.user_id WHERE m.item_id = ? ORDER BY m.created_at DESC",
		id,
	)
	if err != nil {
		dbError(w, err, "We couldn't load item history. Please try again.")
		return
	}
	defer rows.Close()

	type modification struct {
		Action    string `json:"action"`
		Detail    string `json:"detail"`
		Source    string `json:"source"`
		CreatedAt string `json:"created_at"`
		UserName  string `json:"user_name"`
	}
	var mods []modification
	for rows.Next() {
		var m modification
		var detail, source, userName sql.NullString
		if err := rows.Scan(&m.Action, &detail, &source, &m.CreatedAt, &userName); err != nil {
			continue
		}
		m.Detail = detail.String
		m.Source = source.String
		m.UserName = userName.String
		mods = append(mods, m)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[history] row iteration error: %v", err)
	}
	if mods == nil {
		mods = []modification{}
	}

	writeJSON(w, http.StatusOK, mods)
}

// handleListEdit handles PATCH /api/list/{id}
// Request: { "name": "...", "brand": "...", "department": "...", "quantity": 1, "size": "..." } (all optional)
func (s *Server) handleListEdit(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Name       *string `json:"name"`
		Brand      *string `json:"brand"`
		Department *string `json:"department"`
		Quantity   *int    `json:"quantity"`
		Size       *string `json:"size"`
		Notes      *string `json:"notes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)

	// Build a single UPDATE with only the fields that were provided
	var setClauses []string
	var setArgs []interface{}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Name cannot be empty"})
			return
		}
		setClauses = append(setClauses, "name = ?")
		setArgs = append(setArgs, name)
	}
	if req.Brand != nil {
		setClauses = append(setClauses, "brand = ?")
		setArgs = append(setArgs, *req.Brand)
	}
	if req.Department != nil {
		setClauses = append(setClauses, "department = ?")
		setArgs = append(setArgs, *req.Department)
	}
	if req.Quantity != nil {
		qty := *req.Quantity
		if qty < 1 {
			qty = 1
		}
		setClauses = append(setClauses, "quantity = ?")
		setArgs = append(setArgs, qty)
	}
	if req.Size != nil {
		setClauses = append(setClauses, "size = ?")
		setArgs = append(setArgs, *req.Size)
	}
	if req.Notes != nil {
		setClauses = append(setClauses, "notes = ?")
		setArgs = append(setArgs, *req.Notes)
	}

	if len(setClauses) > 0 {
		query := "UPDATE list_items SET " + strings.Join(setClauses, ", ") + " WHERE id = ? AND " + whereClause
		args := append(setArgs, id)
		args = append(args, whereArgs...)
		if _, err := s.db.Exec(query, args...); err != nil {
			dbError(w, err, "We couldn't save your item right now. Please try again.")
			return
		}
		recordModification(s.db, id, session.UserID, "updated", strings.Join(setClauses, ", "), "app")
	}

	item, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// handleListDelete handles DELETE /api/list/{id}
func (s *Server) handleListDelete(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	// ON DELETE CASCADE handles item_modifications cleanup
	args := append([]interface{}{id}, whereArgs...)
	result, err := s.db.Exec(
		"DELETE FROM list_items WHERE id = ? AND "+whereClause,
		args...,
	)
	if err != nil {
		dbError(w, err, "We couldn't remove that item. Please try again.")
		return
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleListClear handles POST /api/list/clear
func (s *Server) handleListClear(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)

	// Stash checked items before deleting so Penny can restore them
	rows, err := s.db.Query("SELECT name, brand, department, quantity, size, notes FROM list_items WHERE checked = 1 AND "+whereClause, whereArgs...)
	if err == nil {
		type stashedItem struct {
			Name       string `json:"name"`
			Brand      string `json:"brand"`
			Department string `json:"department"`
			Quantity   int    `json:"quantity"`
			Size       string `json:"size"`
			Notes      string `json:"notes"`
		}
		var stashed []stashedItem
		for rows.Next() {
			var si stashedItem
			if rows.Scan(&si.Name, &si.Brand, &si.Department, &si.Quantity, &si.Size, &si.Notes) == nil {
				stashed = append(stashed, si)
			}
		}
		rows.Close()
		if stashJSON, e := json.Marshal(stashed); e == nil {
			s.db.Exec("UPDATE users SET last_cleared_items = ? WHERE id = ?", string(stashJSON), session.UserID)
		}
	}

	// If ?all=true, clear entire list (used by Penny chat "clear list" command)
	clearAll := r.URL.Query().Get("all") == "true"
	if clearAll {
		_, err = s.db.Exec("DELETE FROM list_items WHERE "+whereClause, whereArgs...)
	} else {
		_, err = s.db.Exec("DELETE FROM list_items WHERE checked = 1 AND "+whereClause, whereArgs...)
	}
	if err != nil {
		dbError(w, err, "We couldn't clear your list. Please try again.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleListRestore handles POST /api/list/restore — bulk re-insert items (used by undo)
func (s *Server) handleListRestore(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var items []struct {
		Name       string `json:"name"`
		Brand      string `json:"brand"`
		Department string `json:"department"`
		Quantity   int    `json:"quantity"`
		Size       string `json:"size"`
		Notes      string `json:"notes"`
		Checked    bool   `json:"checked"`
		Urgent     bool   `json:"urgent"`
	}
	if !decodeJSON(w, r, &items) {
		return
	}

	// Cap restore size to prevent abuse
	if len(items) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Too many items to restore."})
		return
	}

	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		qty := item.Quantity
		if qty < 1 {
			qty = 1
		}
		dept := item.Department
		if dept == "" {
			dept = "Other"
		}
		checked := 0
		if item.Checked {
			checked = 1
		}
		urgent := 0
		if item.Urgent {
			urgent = 1
		}
		_, err := s.db.Exec(
			`INSERT INTO list_items (user_id, name, brand, department, quantity, size, notes, checked, urgent, source)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'app')`,
			session.UserID, strings.TrimSpace(item.Name), strings.TrimSpace(item.Brand),
			dept, qty, strings.TrimSpace(item.Size), strings.TrimSpace(item.Notes), checked, urgent,
		)
		if err != nil {
			dbError(w, err, "We couldn't restore your list. Please try again.")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// =============================================================================

func (s *Server) handleBrandResolve(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		ItemID          int    `json:"item_id"`
		Brand           string `json:"brand"`
		ProductCategory string `json:"product_category"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.ItemID == 0 || req.Brand == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "item_id and brand are required"})
		return
	}

	// Update the item's brand — scoped to household ownership
	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	updateArgs := append([]interface{}{req.Brand, req.ItemID}, whereArgs...)
	result, err := s.db.Exec("UPDATE list_items SET brand = ? WHERE id = ? AND "+whereClause, updateArgs...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update item"})
		return
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}

	// Record the brand choice for preference learning
	category := req.ProductCategory
	if category == "" {
		var itemName string
		s.db.QueryRow("SELECT name FROM list_items WHERE id = ?", req.ItemID).Scan(&itemName)
		category = itemName
	}
	recordBrandChoice(s.db, session.UserID, category, req.Brand, "disambiguation")
	recordModification(s.db, req.ItemID, session.UserID, "updated", "brand resolved to "+req.Brand, "app")

	// Return the updated item
	item, err := getItemByID(s.db, req.ItemID, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load item"})
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// handleShareList handles POST /api/list/share — creates a shared link for the user's list.
func (s *Server) handleShareList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Generate 16-byte hex code
	codeBytes := make([]byte, 16)
	if _, err := rand.Read(codeBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate share code"})
		return
	}
	code := hex.EncodeToString(codeBytes)

	// Check if user has a household
	var householdID sql.NullInt64
	s.db.QueryRow("SELECT household_id FROM users WHERE id = ?", session.UserID).Scan(&householdID)

	// Generate an invite code if user has a household
	var inviteCode string
	if householdID.Valid {
		inviteBytes := make([]byte, 16)
		if _, err := rand.Read(inviteBytes); err == nil {
			inviteCode = hex.EncodeToString(inviteBytes)
			expiresAt := time.Now().Add(inviteExpiry)
			s.db.Exec(
				"INSERT INTO invites (household_id, code, created_by, expires_at) VALUES (?, ?, ?, ?)",
				householdID.Int64, inviteCode, session.UserID, expiresAt,
			)
		}
	}

	// Insert shared link with 7-day expiry
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err := s.db.Exec(
		"INSERT INTO shared_links (code, user_id, household_id, invite_code, expires_at) VALUES (?, ?, ?, ?, ?)",
		code, session.UserID, householdID, inviteCode, expiresAt,
	)
	if err != nil {
		dbError(w, err, "Failed to create share link")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"url":        "/shared/" + code,
		"code":       code,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

// handleSharedView handles GET /api/shared/{code} — returns read-only list data (no auth required).
func (s *Server) handleSharedView(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	// Extract code from URL path
	code := strings.TrimPrefix(r.URL.Path, "/api/shared/")
	if code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing share code"})
		return
	}

	// Look up the shared link
	var userID int
	var householdID sql.NullInt64
	var inviteCode sql.NullString
	var expiresAt string
	var viewCount int
	err := s.db.QueryRow(
		"SELECT user_id, household_id, invite_code, expires_at, view_count FROM shared_links WHERE code = ?",
		code,
	).Scan(&userID, &householdID, &inviteCode, &expiresAt, &viewCount)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Share link not found"})
		return
	}

	// Check expiry
	expiry, err := time.Parse("2006-01-02 15:04:05", expiresAt)
	if err != nil {
		// Try RFC3339 format
		expiry, err = time.Parse(time.RFC3339, expiresAt)
	}
	if err != nil || time.Now().After(expiry) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "This share link has expired"})
		return
	}

	// Increment view count
	s.db.Exec("UPDATE shared_links SET view_count = view_count + 1 WHERE code = ?", code)

	// Get sharer's name
	var sharerName string
	s.db.QueryRow("SELECT COALESCE(name, '') FROM users WHERE id = ?", userID).Scan(&sharerName)

	// Get list items (name, quantity, checked only — no IDs or sensitive data)
	whereClause, whereArgs := householdItemsWhereClause(s.db, userID)
	rows, err := s.db.Query(
		"SELECT name, COALESCE(quantity, 1), checked FROM list_items WHERE "+whereClause+" ORDER BY name",
		whereArgs...,
	)
	if err != nil {
		dbError(w, err, "Failed to load shared list")
		return
	}
	defer rows.Close()

	type sharedItem struct {
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
		Checked  bool   `json:"checked"`
	}
	items := []sharedItem{}
	for rows.Next() {
		var item sharedItem
		if rows.Scan(&item.Name, &item.Quantity, &item.Checked) == nil {
			items = append(items, item)
		}
	}

	resp := map[string]interface{}{
		"items":       items,
		"sharer_name": sharerName,
		"expires_at":  expiry.Format(time.RFC3339),
	}
	if inviteCode.Valid && inviteCode.String != "" {
		resp["invite_code"] = inviteCode.String
	}

	writeJSON(w, http.StatusOK, resp)
}

// =============================================================================
