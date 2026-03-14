package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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
		writeBadRequest(w, "Missing domain or brand parameter")
		return
	}
	// Sanitize: only allow alphanumeric, dots, hyphens
	for _, c := range domain {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-') {
			writeBadRequest(w, "Invalid domain")
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

	// No real logo found — return 404 so frontend falls back to emoji
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusNotFound)
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
