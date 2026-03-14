package main

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// upsertStore finds or creates a store by name+address. Returns the store ID.
func (s *Server) upsertStore(name, chain, address, city, state, zip, placeID string, lat, lng *float64) (int, error) {
	var id int
	err := s.db.QueryRow("SELECT id FROM stores WHERE name = ? AND address = ?", name, address).Scan(&id)
	if err == nil {
		return id, nil
	}

	var latVal, lngVal sql.NullFloat64
	if lat != nil {
		latVal = sql.NullFloat64{Float64: *lat, Valid: true}
	}
	if lng != nil {
		lngVal = sql.NullFloat64{Float64: *lng, Valid: true}
	}

	result, err := s.db.Exec(
		`INSERT INTO stores (name, chain, address, city, state, zip, place_id, latitude, longitude)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		name, chain, address, city, state, zip, placeID, latVal, lngVal,
	)
	if err != nil {
		return 0, err
	}
	insertID, _ := result.LastInsertId()
	return int(insertID), nil
}

// recordStorePrice records a price observation for a product at a store.
func (s *Server) recordStorePrice(storeID int, productName, brand string, priceCents, categoryID int, source string) {
	if storeID == 0 || productName == "" || priceCents == 0 {
		return
	}
	if _, err := s.db.Exec(
		`INSERT INTO store_prices (store_id, product_name, brand, price_cents, category_id, source)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		storeID, productName, brand, priceCents, categoryID, source,
	); err != nil {
		log.Printf("[stores] failed to record price: %v", err)
	}
}

// handleStores handles GET /api/stores — returns stores, optionally filtered by chain or search.
func (s *Server) handleStores(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireSession(w, r); !ok {
		return
	}

	query := r.URL.Query().Get("q")
	chain := r.URL.Query().Get("chain")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
		limit = l
	}

	type storeResult struct {
		ID      int     `json:"id"`
		Name    string  `json:"name"`
		Chain   string  `json:"chain"`
		Address string  `json:"address"`
		City    string  `json:"city"`
		State   string  `json:"state"`
		Zip     string  `json:"zip"`
		PlaceID string  `json:"place_id,omitempty"`
		Lat     *float64 `json:"latitude,omitempty"`
		Lng     *float64 `json:"longitude,omitempty"`
	}

	var rows *sql.Rows
	var err error

	if query != "" {
		searchTerm := "%" + strings.ToLower(query) + "%"
		rows, err = s.db.Query(
			`SELECT id, name, chain, address, city, state, zip, place_id, latitude, longitude
			 FROM stores WHERE LOWER(name) LIKE ? OR LOWER(chain) LIKE ? OR LOWER(address) LIKE ?
			 ORDER BY name LIMIT ?`,
			searchTerm, searchTerm, searchTerm, limit,
		)
	} else if chain != "" {
		rows, err = s.db.Query(
			`SELECT id, name, chain, address, city, state, zip, place_id, latitude, longitude
			 FROM stores WHERE LOWER(chain) = ? ORDER BY name LIMIT ?`,
			strings.ToLower(chain), limit,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, chain, address, city, state, zip, place_id, latitude, longitude
			 FROM stores ORDER BY name LIMIT ?`, limit,
		)
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load stores"})
		return
	}

	var stores []storeResult
	for rows.Next() {
		var s storeResult
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&s.ID, &s.Name, &s.Chain, &s.Address, &s.City, &s.State, &s.Zip, &s.PlaceID, &lat, &lng); err != nil {
			continue
		}
		if lat.Valid {
			s.Lat = &lat.Float64
		}
		if lng.Valid {
			s.Lng = &lng.Float64
		}
		stores = append(stores, s)
	}
	rows.Close()

	if stores == nil {
		stores = []storeResult{}
	}
	writeJSON(w, http.StatusOK, stores)
}

// handleStoreSubroute dispatches /api/stores/{id}/... to the appropriate handler.
func (s *Server) handleStoreSubroute(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/stores/")
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		switch parts[1] {
		case "prices":
			s.handleStorePrices(w, r)
			return
		case "price-averages":
			s.handleStorePriceAverages(w, r)
			return
		}
	}
	writeNotFound(w, "Not found")
}

// handleStorePrices handles GET /api/stores/{id}/prices — returns price history for a store.
func (s *Server) handleStorePrices(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireSession(w, r); !ok {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/stores/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "prices" {
		writeBadRequest(w, "Invalid path")
		return
	}
	storeID, err := strconv.Atoi(parts[0])
	if err != nil {
		writeBadRequest(w, "Invalid store ID")
		return
	}

	rows, err := s.db.Query(
		`SELECT product_name, brand, price_cents, source, observed_at
		 FROM store_prices WHERE store_id = ? ORDER BY observed_at DESC LIMIT 100`,
		storeID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load prices"})
		return
	}

	type priceEntry struct {
		ProductName string `json:"product_name"`
		Brand       string `json:"brand"`
		PriceCents  int    `json:"price_cents"`
		Source      string `json:"source"`
		ObservedAt  string `json:"observed_at"`
	}
	var prices []priceEntry
	for rows.Next() {
		var p priceEntry
		if err := rows.Scan(&p.ProductName, &p.Brand, &p.PriceCents, &p.Source, &p.ObservedAt); err != nil {
			continue
		}
		prices = append(prices, p)
	}
	rows.Close()

	if prices == nil {
		prices = []priceEntry{}
	}
	writeJSON(w, http.StatusOK, prices)
}

// handleAdminStoreStats returns aggregate store statistics.
// GET /api/admin/store-stats — admin only.
func (s *Server) handleAdminStoreStats(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	var totalStores, totalPriceObs, uniqueProducts, chainsCount int
	s.db.QueryRow("SELECT COUNT(*) FROM stores").Scan(&totalStores)
	s.db.QueryRow("SELECT COUNT(*) FROM store_prices").Scan(&totalPriceObs)
	s.db.QueryRow("SELECT COUNT(DISTINCT product_name) FROM store_prices").Scan(&uniqueProducts)
	s.db.QueryRow("SELECT COUNT(DISTINCT chain) FROM stores WHERE chain != ''").Scan(&chainsCount)

	// Top chains by store count
	type chainStat struct {
		Chain  string `json:"chain"`
		Stores int    `json:"stores"`
	}
	rows, err := s.db.Query(
		"SELECT chain, COUNT(*) as cnt FROM stores WHERE chain != '' GROUP BY chain ORDER BY cnt DESC LIMIT 10",
	)
	var topChains []chainStat
	if err == nil {
		for rows.Next() {
			var cs chainStat
			rows.Scan(&cs.Chain, &cs.Stores)
			topChains = append(topChains, cs)
		}
		rows.Close()
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_stores":       totalStores,
		"total_price_observations": totalPriceObs,
		"unique_products":    uniqueProducts,
		"chains_count":       chainsCount,
		"top_chains":         topChains,
	})
	log.Printf("[stores] admin stats: %d stores, %d price observations", totalStores, totalPriceObs)
}

// handleStorePriceAverages returns moving-average prices per product at a store.
// GET /api/stores/{id}/price-averages?product=&brand=
func (s *Server) handleStorePriceAverages(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireSession(w, r); !ok {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/stores/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "price-averages" {
		writeBadRequest(w, "Invalid path")
		return
	}
	storeID, err := strconv.Atoi(parts[0])
	if err != nil {
		writeBadRequest(w, "Invalid store ID")
		return
	}

	product := r.URL.Query().Get("product")
	brand := r.URL.Query().Get("brand")

	type priceAvg struct {
		ProductName string `json:"product_name"`
		Brand       string `json:"brand"`
		AvgCents    int    `json:"avg_price_cents"`
		MinCents    int    `json:"min_price_cents"`
		MaxCents    int    `json:"max_price_cents"`
		Observations int   `json:"observations"`
		LastSeen    string `json:"last_seen"`
	}

	var queryStr string
	var args []interface{}
	baseQuery := `SELECT product_name, brand,
		CAST(AVG(price_cents) AS INTEGER) as avg_cents,
		MIN(price_cents) as min_cents,
		MAX(price_cents) as max_cents,
		COUNT(*) as obs,
		MAX(observed_at) as last_seen
		FROM store_prices WHERE store_id = ?`
	args = append(args, storeID)

	if product != "" {
		baseQuery += " AND LOWER(product_name) LIKE ?"
		args = append(args, "%"+strings.ToLower(product)+"%")
	}
	if brand != "" {
		baseQuery += " AND LOWER(brand) LIKE ?"
		args = append(args, "%"+strings.ToLower(brand)+"%")
	}
	queryStr = baseQuery + " GROUP BY product_name, brand ORDER BY obs DESC LIMIT 100"

	rows, err := s.db.Query(queryStr, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to compute averages"})
		return
	}

	var results []priceAvg
	for rows.Next() {
		var p priceAvg
		if err := rows.Scan(&p.ProductName, &p.Brand, &p.AvgCents, &p.MinCents, &p.MaxCents, &p.Observations, &p.LastSeen); err != nil {
			continue
		}
		results = append(results, p)
	}
	rows.Close()

	if results == nil {
		results = []priceAvg{}
	}
	writeJSON(w, http.StatusOK, results)
}

// handlePriceComparison compares prices for a product across stores the user has visited.
// GET /api/price-compare?product=&brand=
func (s *Server) handlePriceComparison(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	product := r.URL.Query().Get("product")
	if product == "" {
		writeBadRequest(w, "product parameter required")
		return
	}
	brand := r.URL.Query().Get("brand")

	type storePrice struct {
		StoreID   int    `json:"store_id"`
		StoreName string `json:"store_name"`
		Chain     string `json:"chain"`
		Address   string `json:"address"`
		AvgCents  int    `json:"avg_price_cents"`
		LastSeen  string `json:"last_seen"`
		Observations int `json:"observations"`
	}

	// Find stores associated with the user's shopping trips
	queryStr := `SELECT s.id, s.name, s.chain, s.address,
		CAST(AVG(sp.price_cents) AS INTEGER) as avg_cents,
		MAX(sp.observed_at) as last_seen,
		COUNT(sp.id) as obs
		FROM stores s
		JOIN store_prices sp ON sp.store_id = s.id
		WHERE LOWER(sp.product_name) LIKE ?`
	args := []interface{}{"%" + strings.ToLower(product) + "%"}

	if brand != "" {
		queryStr += " AND LOWER(sp.brand) LIKE ?"
		args = append(args, "%"+strings.ToLower(brand)+"%")
	}

	// If user has shopping trips linked to stores, prioritize those
	var userStoreIDs []int
	tripRows, err := s.db.Query(
		"SELECT DISTINCT store_id FROM shopping_trips WHERE user_id = ? AND store_id IS NOT NULL",
		session.UserID,
	)
	if err == nil {
		for tripRows.Next() {
			var sid int
			tripRows.Scan(&sid)
			userStoreIDs = append(userStoreIDs, sid)
		}
		tripRows.Close()
	}

	queryStr += " GROUP BY s.id ORDER BY avg_cents ASC LIMIT 20"
	rows, err := s.db.Query(queryStr, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to compare prices"})
		return
	}

	var results []storePrice
	for rows.Next() {
		var sp storePrice
		if err := rows.Scan(&sp.StoreID, &sp.StoreName, &sp.Chain, &sp.Address, &sp.AvgCents, &sp.LastSeen, &sp.Observations); err != nil {
			continue
		}
		results = append(results, sp)
	}
	rows.Close()

	// Mark user's stores
	type comparisonResult struct {
		Stores     []storePrice `json:"stores"`
		UserStores []int        `json:"user_store_ids"`
		Product    string       `json:"product"`
	}

	if results == nil {
		results = []storePrice{}
	}
	writeJSON(w, http.StatusOK, comparisonResult{
		Stores:     results,
		UserStores: userStoreIDs,
		Product:    product,
	})
}

// handleRegionalSwapPerformance returns swap acceptance rates aggregated by state/region.
// GET /api/admin/regional-swaps
func (s *Server) handleRegionalSwapPerformance(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	type regionPerf struct {
		State         string  `json:"state"`
		TotalOffered  int     `json:"total_offered"`
		TotalAccepted int     `json:"total_accepted"`
		TotalDeclined int     `json:"total_declined"`
		AcceptRate    float64 `json:"accept_rate"`
		UniqueUsers   int     `json:"unique_users"`
	}

	// Join swap_events with users → settings (which might have state)
	// or with shopping_trips → stores for location data
	rows, err := s.db.Query(`
		SELECT
			COALESCE(st.state, 'Unknown') as region,
			COALESCE(SUM(CASE WHEN se.action = 'offered' THEN 1 ELSE 0 END), 0) as offered,
			COALESCE(SUM(CASE WHEN se.action = 'accepted' THEN 1 ELSE 0 END), 0) as accepted,
			COALESCE(SUM(CASE WHEN se.action = 'declined' THEN 1 ELSE 0 END), 0) as declined,
			COUNT(DISTINCT se.user_id) as unique_users
		FROM swap_events se
		LEFT JOIN shopping_trips trip ON trip.user_id = se.user_id
		LEFT JOIN stores st ON st.id = trip.store_id
		WHERE se.action IN ('offered', 'accepted', 'declined')
		GROUP BY region
		ORDER BY offered DESC
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to compute regional stats"})
		return
	}

	var results []regionPerf
	for rows.Next() {
		var rp regionPerf
		if err := rows.Scan(&rp.State, &rp.TotalOffered, &rp.TotalAccepted, &rp.TotalDeclined, &rp.UniqueUsers); err != nil {
			continue
		}
		if rp.TotalOffered > 0 {
			rp.AcceptRate = float64(rp.TotalAccepted) / float64(rp.TotalOffered)
		}
		results = append(results, rp)
	}
	rows.Close()

	if results == nil {
		results = []regionPerf{}
	}
	writeJSON(w, http.StatusOK, results)
}

// getStorePrice looks up the most recent price for a product at a specific store.
// Returns price in cents and whether a price was found. Used for store-aware swap pricing.
func (s *Server) getStorePrice(storeID int, productName, brand string) (int, bool) {
	if storeID == 0 {
		return 0, false
	}
	var priceCents int
	err := s.db.QueryRow(
		`SELECT price_cents FROM store_prices
		 WHERE store_id = ? AND LOWER(product_name) LIKE ? AND LOWER(brand) LIKE ?
		 ORDER BY observed_at DESC LIMIT 1`,
		storeID, "%"+strings.ToLower(productName)+"%", "%"+strings.ToLower(brand)+"%",
	).Scan(&priceCents)
	if err != nil {
		return 0, false
	}
	return priceCents, true
}

// getUserActiveStoreID returns the most recent store associated with a user's shopping trips.
func (s *Server) getUserActiveStoreID(userID int) int {
	var storeID int
	s.db.QueryRow(
		"SELECT store_id FROM shopping_trips WHERE user_id = ? AND store_id IS NOT NULL ORDER BY created_at DESC LIMIT 1",
		userID,
	).Scan(&storeID)
	return storeID
}
