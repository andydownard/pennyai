package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// =============================================================================
// BLUECART API CLIENT
//
// BlueCart API provides real-time Walmart product data including prices.
// Docs: https://docs.trajectdata.com/bluecartapi
// Base URL: https://api.bluecartapi.com/request
// Auth: api_key query parameter
// Rate limit: 10,000 product searches/day
// =============================================================================

const (
	bluecartBaseURL   = "https://api.bluecartapi.com/request"
	priceCacheTTL     = 6 * time.Hour
	bluecartTimeout   = 10 * time.Second
)

// BlueCartPrice holds cached pricing data for a brand+category at a zip code.
type BlueCartPrice struct {
	Brand       string
	Category    string
	Zip         string
	PriceCents  int
	UnitPrice   string // e.g. "$0.12/oz"
	UnitLabel   string // e.g. "oz"
	Size        string // e.g. "20 oz"
	ProductName string
	StoreName   string
	FetchedAt   time.Time
}

// bluecartSearchResponse represents the API response from BlueCart search.
type bluecartSearchResponse struct {
	RequestInfo struct {
		Success bool `json:"success"`
	} `json:"request_info"`
	SearchResults []struct {
		Product struct {
			Title  string `json:"title"`
			ItemID string `json:"item_id"`
		} `json:"product"`
		Offers struct {
			Primary struct {
				Price float64 `json:"price"`
			} `json:"primary"`
		} `json:"offers"`
		Inventory struct {
			InStock bool `json:"in_stock"`
		} `json:"inventory"`
	} `json:"search_results"`
}

// bluecartProductResponse represents the product detail response.
type bluecartProductResponse struct {
	RequestInfo struct {
		Success bool `json:"success"`
	} `json:"request_info"`
	Product struct {
		Title    string `json:"title"`
		BuyboxWinner struct {
			Price         float64 `json:"price"`
			WasPrice      float64 `json:"was_price"`
			UnitPrice     float64 `json:"unit_price"`
			UnitPriceText string  `json:"unit_price_text"`
		} `json:"buybox_winner"`
		Categories []struct {
			Name string `json:"name"`
		} `json:"categories"`
	} `json:"product"`
}

// fetchBrandPrice queries BlueCart for the best-priced product from a given brand
// in a category, localized to the user's zip code.
func (s *Server) fetchBrandPrice(apiKey, brand, category, zip string) (*BlueCartPrice, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("BLUECART_API_KEY not set")
	}

	// Check cache first
	cached, err := s.getCachedPrice(brand, category, zip)
	if err == nil && cached != nil {
		return cached, nil
	}

	// Search for brand + category at this zip
	searchTerm := brand + " " + category
	params := url.Values{
		"api_key":          {apiKey},
		"type":             {"search"},
		"search_term":      {searchTerm},
		"customer_zipcode": {zip},
		"sort_by":          {"best_seller"},
	}

	client := &http.Client{Timeout: bluecartTimeout}
	resp, err := client.Get(bluecartBaseURL + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("bluecart API error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading bluecart response: %w", err)
	}

	var result bluecartSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing bluecart response: %w", err)
	}

	if !result.RequestInfo.Success || len(result.SearchResults) == 0 {
		return nil, fmt.Errorf("no results for %q at zip %s", searchTerm, zip)
	}

	// Take the first in-stock result
	for _, sr := range result.SearchResults {
		if sr.Offers.Primary.Price <= 0 {
			continue
		}

		priceCents := int(sr.Offers.Primary.Price * 100)
		price := &BlueCartPrice{
			Brand:       brand,
			Category:    category,
			Zip:         zip,
			PriceCents:  priceCents,
			ProductName: sr.Product.Title,
			StoreName:   "Walmart",
			FetchedAt:   time.Now(),
		}

		// Try to get unit price from product detail
		if sr.Product.ItemID != "" {
			s.enrichWithProductDetail(apiKey, sr.Product.ItemID, zip, price)
		}

		// Cache the result
		s.cachePrice(price)

		return price, nil
	}

	return nil, fmt.Errorf("no priced results for %q at zip %s", searchTerm, zip)
}

// enrichWithProductDetail fetches product detail for unit price info.
func (s *Server) enrichWithProductDetail(apiKey, itemID, zip string, price *BlueCartPrice) {
	params := url.Values{
		"api_key":          {apiKey},
		"type":             {"product"},
		"item_id":          {itemID},
		"customer_zipcode": {zip},
	}

	client := &http.Client{Timeout: bluecartTimeout}
	resp, err := client.Get(bluecartBaseURL + "?" + params.Encode())
	if err != nil {
		log.Printf("[bluecart] product detail error for %s: %v", itemID, err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result bluecartProductResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return
	}

	if result.Product.BuyboxWinner.UnitPrice > 0 {
		price.UnitPrice = result.Product.BuyboxWinner.UnitPriceText
		// Extract unit label from unit_price_text like "(9.7 cents/oz)"
		if text := result.Product.BuyboxWinner.UnitPriceText; text != "" {
			if idx := strings.LastIndex(text, "/"); idx >= 0 {
				label := strings.TrimRight(text[idx+1:], ")")
				price.UnitLabel = strings.TrimSpace(label)
			}
		}
	}
}

// getCachedPrice checks the price_cache table for a recent price.
func (s *Server) getCachedPrice(brand, category, zip string) (*BlueCartPrice, error) {
	var p BlueCartPrice
	var fetchedAt string
	err := s.db.QueryRow(`
		SELECT brand, category, zip, price_cents, unit_price, unit_label, size,
		       product_name, store_name, fetched_at
		FROM price_cache
		WHERE LOWER(brand) = LOWER(?) AND LOWER(category) = LOWER(?) AND zip = ?
		  AND fetched_at > datetime('now', '-6 hours')
		ORDER BY fetched_at DESC LIMIT 1
	`, brand, category, zip).Scan(
		&p.Brand, &p.Category, &p.Zip, &p.PriceCents,
		&p.UnitPrice, &p.UnitLabel, &p.Size,
		&p.ProductName, &p.StoreName, &fetchedAt,
	)
	if err != nil {
		return nil, err
	}
	p.FetchedAt, _ = time.Parse("2006-01-02 15:04:05", fetchedAt)
	return &p, nil
}

// cachePrice stores a price in the price_cache table.
func (s *Server) cachePrice(p *BlueCartPrice) {
	s.db.Exec(`
		INSERT OR REPLACE INTO price_cache (brand, category, zip, price_cents, unit_price,
		                                     unit_label, size, product_name, store_name, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
	`, p.Brand, p.Category, p.Zip, p.PriceCents, p.UnitPrice,
		p.UnitLabel, p.Size, p.ProductName, p.StoreName)
}

// getUserZip returns the user's zip code from their address, or empty string.
func (s *Server) getUserZip(userID int) string {
	var address sql.NullString
	s.db.QueryRow("SELECT COALESCE(address,'') FROM users WHERE id = ?", userID).Scan(&address)
	if !address.Valid || address.String == "" {
		return ""
	}
	// Extract zip from address — look for 5-digit number
	parts := strings.Fields(address.String)
	for _, p := range parts {
		p = strings.TrimRight(p, ",.")
		if len(p) == 5 {
			allDigit := true
			for _, c := range p {
				if c < '0' || c > '9' {
					allDigit = false
					break
				}
			}
			if allDigit {
				return p
			}
		}
	}
	return ""
}
