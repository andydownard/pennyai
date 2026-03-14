package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// handleNPSCheck handles GET /api/nps/check — returns whether the NPS survey should show.

func (s *Server) handleNPSCheck(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var loginCount int
	if err := s.db.QueryRow("SELECT COALESCE(login_count, 0) FROM users WHERE id = ?", session.UserID).Scan(&loginCount); err != nil {
		log.Printf("[nps] error checking login count for user %d: %v", session.UserID, err)
	}
	if loginCount < npsMinLogins {
		writeJSON(w, http.StatusOK, map[string]bool{"show_nps": false})
		return
	}

	// Check if user has a recent NPS response
	intervalDays := fmt.Sprintf("-%d days", s.config.NPSIntervalDays)
	var recentCount int
	if err := s.db.QueryRow(
		"SELECT COUNT(*) FROM nps_responses WHERE user_id = ? AND created_at >= datetime('now', ?)",
		session.UserID, intervalDays,
	).Scan(&recentCount); err != nil {
		log.Printf("[nps] error checking recent responses for user %d: %v", session.UserID, err)
	}
	if recentCount > 0 {
		writeJSON(w, http.StatusOK, map[string]bool{"show_nps": false})
		return
	}

	// Check if user dismissed NPS recently (server-side guard survives localStorage clears)
	var dismissedAt string
	if err := s.db.QueryRow(
		"SELECT COALESCE(nps_dismissed_at, '') FROM users WHERE id = ?",
		session.UserID,
	).Scan(&dismissedAt); err != nil {
		log.Printf("[nps] error checking dismiss timestamp for user %d: %v", session.UserID, err)
	}
	if dismissedAt != "" {
		dismissTime, err := time.Parse("2006-01-02 15:04:05", dismissedAt)
		if err == nil && time.Since(dismissTime).Hours() < float64(s.config.NPSIntervalDays*24) {
			writeJSON(w, http.StatusOK, map[string]bool{"show_nps": false})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]bool{"show_nps": true})
}

// handleNPS handles POST /api/nps — submits an NPS response.
func (s *Server) handleNPS(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Score   int    `json:"score"`
		Comment string `json:"comment"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Score < 0 || req.Score > 10 {
		writeBadRequest(w, "Score must be 0-10")
		return
	}
	if len(req.Comment) > 500 {
		req.Comment = req.Comment[:500]
	}
	req.Comment = strings.TrimSpace(req.Comment)

	_, err := s.db.Exec(
		"INSERT INTO nps_responses (user_id, score, comment) VALUES (?, ?, ?)",
		session.UserID, req.Score, req.Comment,
	)
	if err != nil {
		log.Printf("[nps] error inserting response: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save response"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleNPSDismiss handles POST /api/nps/dismiss — records NPS dismissal server-side.
// This ensures the dismiss persists even if the client clears localStorage.
func (s *Server) handleNPSDismiss(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	_, err := s.db.Exec(
		"UPDATE users SET nps_dismissed_at = datetime('now') WHERE id = ?",
		session.UserID,
	)
	if err != nil {
		log.Printf("[nps] error recording dismiss for user %d: %v", session.UserID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to record dismiss"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
