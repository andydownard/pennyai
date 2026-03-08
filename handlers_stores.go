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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid path"})
		return
	}
	storeID, err := strconv.Atoi(parts[0])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid store ID"})
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
