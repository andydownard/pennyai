package main

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// =============================================================================
// EMAIL INGESTION (SendGrid Inbound Parse)
// =============================================================================

// extractEmail parses an email address from formats like "Name <email>" or bare "email".
func extractEmail(from string) string {
	from = strings.TrimSpace(from)
	if idx := strings.Index(from, "<"); idx >= 0 {
		end := strings.Index(from, ">")
		if end > idx {
			return strings.TrimSpace(from[idx+1 : end])
		}
	}
	return strings.ToLower(from)
}

// handleEmailWebhook handles POST /api/email — SendGrid Inbound Parse webhook.
func (s *Server) handleEmailWebhook(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	// Validate webhook secret (configured in SendGrid Inbound Parse URL as ?secret=...)
	if s.config.WebhookBaseURL != "" {
		secret := r.URL.Query().Get("secret")
		if secret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(s.config.EmailWebhookSecret)) != 1 {
			log.Printf("[email] invalid webhook secret — rejecting request")
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
			return
		}
	}

	// Always return 200 so SendGrid doesn't retry
	defer func() {
		if w.Header().Get("Content-Type") == "" {
			w.WriteHeader(http.StatusOK)
		}
	}()

	// Limit request body to 10 MB
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)

	// Parse multipart form (SendGrid sends multipart)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		// Fall back to regular form parse
		if err := r.ParseForm(); err != nil {
			log.Printf("[email] error parsing form: %v", err)
			writeJSON(w, http.StatusOK, map[string]bool{"success": true})
			return
		}
	}

	from := extractEmail(r.FormValue("from"))
	text := strings.TrimSpace(r.FormValue("text"))

	log.Printf("[email] incoming from %s: %q", from, text)

	if from == "" {
		log.Printf("[email] no from address")
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Look up user by email
	var userID string
	err := s.db.QueryRow(
		"SELECT id FROM users WHERE LOWER(email) = LOWER(?)",
		from,
	).Scan(&userID)
	if err != nil {
		log.Printf("[email] unknown sender %s, ignoring", from)
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Collect all image attachments (max 5 MB each)
	const maxImageSize = maxImageBytes
	var images []string
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		for _, headers := range r.MultipartForm.File {
			for _, h := range headers {
				if h.Size > maxImageSize {
					log.Printf("[email] skipping oversized image %s (%d bytes)", h.Filename, h.Size)
					continue
				}
				f, err := h.Open()
				if err != nil {
					continue
				}
				data, err := io.ReadAll(f)
				f.Close()
				if err != nil || len(data) == 0 {
					continue
				}
				ct := h.Header.Get("Content-Type")
				if strings.HasPrefix(ct, "image/") {
					images = append(images, fmt.Sprintf("data:%s;base64,%s", ct, base64.StdEncoding.EncodeToString(data)))
					if len(images) >= 5 {
						break
					}
				}
			}
		}
	}

	if text == "" && len(images) == 0 {
		log.Printf("[email] empty email from %s, ignoring", from)
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Parse items with Claude — all images in one shot
	parsed, err := s.cachedParseItems(s.config.AnthropicAPIKey, text, images)
	if err != nil {
		log.Printf("[email] error parsing items for %s: %v", from, err)
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Add items to user's list
	added, _ := addParsedItems(s.db, userID, parsed, "email")

	log.Printf("[email] added %d items for user %s from %s", len(added), userID, from)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
