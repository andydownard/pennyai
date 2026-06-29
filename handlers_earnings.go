package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// handleEarnings routes GET /api/earnings and POST /api/earnings/{id}/upload.
func (s *Server) handleEarnings(w http.ResponseWriter, r *http.Request) {
	// Route: /api/earnings/{id}/upload
	if strings.HasPrefix(r.URL.Path, "/api/earnings/") && strings.HasSuffix(r.URL.Path, "/upload") {
		s.handleEarningsUpload(w, r)
		return
	}

	// Route: GET /api/earnings
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Optional ?days=7|30|90 filter
	var daysFilter int
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && (parsed == 7 || parsed == 30 || parsed == 90) {
			daysFilter = parsed
		}
	}

	query := `
		SELECT sr.id, bd.offer_brand, bd.offer_product, bd.competitor_product,
		       COALESCE(li.name, ''), sr.amount_cents, sr.status, sr.created_at,
		       sr.attachment_path, sr.verified_amount_cents, COALESCE(sr.verified_at, ''),
		       bd.deal_type
		FROM swap_rebates sr
		JOIN brand_deals bd ON bd.id = sr.deal_id
		LEFT JOIN swap_events se ON se.id = sr.swap_event_id
		LEFT JOIN list_items li ON li.id = se.item_id
		WHERE sr.user_id = ?
	`
	args := []interface{}{session.UserID}

	if daysFilter > 0 {
		query += " AND sr.created_at >= datetime('now', ?)"
		args = append(args, fmt.Sprintf("-%d days", daysFilter))
	}

	query += " ORDER BY sr.created_at DESC LIMIT 200"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		dbError(w, err, "Failed to load earnings")
		return
	}
	defer rows.Close()

	transactions := []EarningTransaction{}
	var totalCents, pendingCents, paidCents int

	for rows.Next() {
		var t EarningTransaction
		if err := rows.Scan(&t.ID, &t.OfferBrand, &t.OfferProduct, &t.CompetitorProduct,
			&t.OriginalItemName, &t.AmountCents, &t.Status, &t.CreatedAt, &t.AttachmentPath,
			&t.VerifiedAmountCents, &t.VerifiedAt, &t.DealType); err != nil {
			log.Printf("[earnings] scan error: %v", err)
			continue
		}
		transactions = append(transactions, t)
		totalCents += t.AmountCents
		switch t.Status {
		case "pending", "pending_purchase":
			pendingCents += t.AmountCents
		case "verified":
			// Verified but in 30-day hold — count toward pending
			pendingCents += t.AmountCents
		case "issued", "paid":
			paidCents += t.AmountCents
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("[earnings] row iteration error: %v", err)
	}

	writeJSON(w, http.StatusOK, EarningsResponse{
		Transactions:  transactions,
		TotalCents:    totalCents,
		PendingCents:  pendingCents,
		PaidCents:     paidCents,
		DisplayAmount: fmt.Sprintf("$%d.%02d", totalCents/100, totalCents%100),
	})
}

// handleEarningsUpload handles POST /api/earnings/{id}/upload — multipart file upload.
func (s *Server) handleEarningsUpload(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Rate limit: 5 uploads per hour per user
	val, _ := s.earningsUploadLimiter.LoadOrStore(session.UserID, &rateBucket{})
	rb := val.(*rateBucket)
	rb.mu.Lock()
	now := time.Now()
	cutoff := now.Add(-time.Hour)
	recent := []time.Time{}
	for _, t := range rb.times {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 5 {
		rb.mu.Unlock()
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Upload rate limit exceeded"})
		return
	}
	rb.times = append(recent, now)
	rb.mu.Unlock()

	// Parse rebate ID from path: /api/earnings/{id}/upload
	path := strings.TrimPrefix(r.URL.Path, "/api/earnings/")
	idStr := strings.TrimSuffix(path, "/upload")
	rebateID, err := strconv.Atoi(idStr)
	if err != nil || rebateID <= 0 {
		writeBadRequest(w, "Invalid rebate ID")
		return
	}

	// Verify ownership
	var ownerID string
	err = s.db.QueryRow("SELECT user_id FROM swap_rebates WHERE id = ?", rebateID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		writeNotFound(w, "Rebate not found")
		return
	}
	if err != nil {
		dbError(w, err, "Failed to verify rebate")
		return
	}
	if ownerID != session.UserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Not your rebate"})
		return
	}

	// Limit request body to 5MB
	r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024)

	// Parse multipart file
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		writeBadRequest(w, "File too large or invalid form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeBadRequest(w, "No file provided")
		return
	}
	defer file.Close()

	// Validate content type
	ct := header.Header.Get("Content-Type")
	if ct == "" {
		// Sniff from extension
		ext := strings.ToLower(filepath.Ext(header.Filename))
		switch ext {
		case ".jpg", ".jpeg":
			ct = "image/jpeg"
		case ".png":
			ct = "image/png"
		case ".pdf":
			ct = "application/pdf"
		case ".webp":
			ct = "image/webp"
		case ".heic":
			ct = "image/heic"
		}
	}
	allowedTypes := map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"image/webp":      true,
		"image/heic":      true,
		"application/pdf": true,
	}
	if !allowedTypes[ct] {
		writeBadRequest(w, "Invalid file type. Must be image or PDF.")
		return
	}

	// Determine upload directory
	uploadDir := filepath.Join("data", "earnings-attachments")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		dbError(w, err, "Upload failed")
		return
	}

	// Save file with unique name
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		switch ct {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "application/pdf":
			ext = ".pdf"
		case "image/webp":
			ext = ".webp"
		case "image/heic":
			ext = ".heic"
		}
	}
	filename := fmt.Sprintf("rebate_%d_%d%s", rebateID, time.Now().UnixMilli(), ext)
	savePath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		dbError(w, err, "Upload failed")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		dbError(w, err, "Upload failed")
		return
	}

	// Update attachment_path in database
	if _, err := s.db.Exec("UPDATE swap_rebates SET attachment_path = ? WHERE id = ?", savePath, rebateID); err != nil {
		dbError(w, err, "Upload saved but DB update failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"attachment_path": savePath,
	})
}
