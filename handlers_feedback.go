package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Valid feedback types, urgency levels, and statuses.
var (
	validFeedbackTypes    = map[string]bool{"bug": true, "feature": true, "other": true}
	validFeedbackUrgency  = map[string]bool{"low": true, "normal": true, "high": true, "critical": true}
	validFeedbackStatuses = map[string]bool{"new": true, "reviewed": true, "roadmapped": true, "done": true}
)

// githubHTTPClient has a 10-second timeout to avoid goroutine leaks if GitHub hangs.
var githubHTTPClient = &http.Client{Timeout: 10 * time.Second}

// isFeedbackUser returns true for all authenticated users.
func (s *Server) isFeedbackUser(session Session) bool {
	return session.UserID > 0
}

// requireFeedbackUser checks for a valid session and feedback privileges.
func (s *Server) requireFeedbackUser(w http.ResponseWriter, r *http.Request) (Session, bool) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return Session{}, false
	}
	if !s.isFeedbackUser(session) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
		return Session{}, false
	}
	return session, true
}

// handleFeedbackRoutes dispatches GET /api/feedback and POST /api/feedback.
func (s *Server) handleFeedbackRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleFeedbackList(w, r)
	case http.MethodPost:
		s.handleFeedback(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// handleFeedback handles POST /api/feedback — submit a feedback request.
// Rate limited to 10 submissions per user per hour.
func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireFeedbackUser(w, r)
	if !ok {
		return
	}

	// Rate limit: max 10 submissions per user per hour
	var recentCount int
	s.db.QueryRow(
		"SELECT COUNT(*) FROM feedback_requests WHERE user_id = ? AND created_at > datetime('now', '-1 hour')",
		session.UserID,
	).Scan(&recentCount)
	if recentCount >= 10 {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Easy there — max 10 submissions per hour. Take a breather."})
		return
	}

	var req struct {
		Body    string `json:"body"`
		Type    string `json:"type"`
		Urgency string `json:"urgency"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Tell us what's on your mind — the body can't be empty."})
		return
	}
	if len(req.Body) > 10000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Please keep feedback under 10,000 characters."})
		return
	}

	// Default type and urgency
	if req.Type == "" || !validFeedbackTypes[req.Type] {
		req.Type = "feature"
	}
	if req.Urgency == "" || !validFeedbackUrgency[req.Urgency] {
		req.Urgency = "normal"
	}

	result, err := s.db.Exec(
		"INSERT INTO feedback_requests (user_id, body, type, urgency) VALUES (?, ?, ?, ?)",
		session.UserID, req.Body, req.Type, req.Urgency,
	)
	if err != nil {
		dbError(w, err, "Failed to save feedback")
		return
	}

	id, _ := result.LastInsertId()
	log.Printf("[feedback] user %d submitted %s (%s): %.60s...", session.UserID, req.Type, req.Urgency, req.Body)

	// Create GitHub issue asynchronously
	go s.createGitHubIssue(id, req.Type, req.Urgency, req.Body)

	// Write to repo markdown files so Claude Code sees it on next session
	if req.Urgency == "critical" || req.Urgency == "high" {
		go s.appendToRepoFile("MUST_HAVES.md", req.Body, req.Urgency, "Must-Have Requirements",
			"Items below were submitted via \"Instructions for Product Team\" with high urgency.\nClaude Code MUST implement these. If they conflict with existing code or CLAUDE.md, these override.")
	} else {
		go s.appendToRepoFile("CONSIDER_FOR_ROADMAP.md", req.Body, req.Urgency, "Consider for Roadmap",
			"Items below were submitted via \"Instructions for Product Team\" for consideration.\nClaude Code should evaluate these when planning future work.")
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      id,
		"success": true,
		"message": fmt.Sprintf("Feedback #%d submitted. Urgency and context will inform priority.", id),
	})
}

// truncateRunes safely truncates a string to maxRunes runes, appending "..." if truncated.
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// createGitHubIssue creates a GitHub issue from feedback.
func (s *Server) createGitHubIssue(feedbackID int64, feedbackType, urgency, body string) {
	if s.config.GitHubPAT == "" || s.config.GitHubRepo == "" {
		return
	}

	title := fmt.Sprintf("[%s] %s", feedbackType, truncateRunes(body, 80))

	labels := []string{"feedback", feedbackType}
	if urgency == "critical" || urgency == "high" {
		labels = append(labels, "priority:"+urgency)
	}

	issueBody := fmt.Sprintf("**Type:** %s\n**Urgency:** %s\n**Feedback ID:** %d\n\n---\n\n%s",
		feedbackType, urgency, feedbackID, body)

	ghResult, ok := s.postGitHubIssue(title, issueBody, labels)
	if !ok {
		log.Printf("[feedback] retrying issue creation without labels")
		ghResult, ok = s.postGitHubIssue(title, issueBody, nil)
	}
	if !ok {
		return
	}

	log.Printf("[feedback] created GitHub issue #%d: %s", ghResult.Number, ghResult.HTMLURL)
	if ghResult.HTMLURL != "" {
		s.db.Exec("UPDATE feedback_requests SET github_issue_url = ? WHERE id = ?", ghResult.HTMLURL, feedbackID)
	}
}

type ghIssueResult struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

func (s *Server) postGitHubIssue(title, body string, labels []string) (ghIssueResult, bool) {
	issuePayload := map[string]interface{}{
		"title": title,
		"body":  body,
	}
	if len(labels) > 0 {
		issuePayload["labels"] = labels
	}

	payload, _ := json.Marshal(issuePayload)

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/issues", s.config.GitHubRepo)
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(payload))
	if err != nil {
		log.Printf("[feedback] failed to create GitHub issue request: %v", err)
		return ghIssueResult{}, false
	}
	req.Header.Set("Authorization", "Bearer "+s.config.GitHubPAT)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		log.Printf("[feedback] failed to create GitHub issue: %v", err)
		return ghIssueResult{}, false
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		log.Printf("[feedback] GitHub API returned %d: %s", resp.StatusCode, string(respBody))
		return ghIssueResult{}, false
	}

	var result ghIssueResult
	json.Unmarshal(respBody, &result)
	return result, true
}

// appendToRepoFile writes feedback to a markdown file in the repo via GitHub API.
func (s *Server) appendToRepoFile(filePath, body, urgency, title, description string) {
	if s.config.GitHubPAT == "" || s.config.GitHubRepo == "" {
		log.Printf("[feedback] skipping %s update: no GitHub config", filePath)
		return
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", s.config.GitHubRepo, filePath)

	getReq, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Printf("[feedback] failed to create GET request for %s: %v", filePath, err)
		return
	}
	getReq.Header.Set("Authorization", "Bearer "+s.config.GitHubPAT)
	getReq.Header.Set("Accept", "application/vnd.github+json")

	resp, err := githubHTTPClient.Do(getReq)
	if err != nil {
		log.Printf("[feedback] failed to fetch %s: %v", filePath, err)
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var existingContent string
	var sha string

	if resp.StatusCode == 200 {
		var fileResp struct {
			Content string `json:"content"`
			SHA     string `json:"sha"`
		}
		if err := json.Unmarshal(respBody, &fileResp); err == nil {
			sha = fileResp.SHA
			s2 := strings.ReplaceAll(fileResp.Content, "\n", "")
			decoded, err := base64.StdEncoding.DecodeString(s2)
			if err == nil {
				existingContent = string(decoded)
			}
		}
	}

	timestamp := time.Now().UTC().Format("2006-01-02 15:04 UTC")
	priorityLabel := strings.ToUpper(urgency)
	newEntry := fmt.Sprintf("\n## [%s] %s\n%s\n_Submitted: %s_\n", priorityLabel, timestamp, body, timestamp)

	var newContent string
	if existingContent == "" {
		newContent = fmt.Sprintf("# %s\n\n%s\n%s", title, description, newEntry)
	} else {
		newContent = existingContent + newEntry
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(newContent))
	commitMsg := fmt.Sprintf("feat: add %s item (%s)", filePath, urgency)

	updatePayload := map[string]interface{}{
		"message": commitMsg,
		"content": encoded,
	}
	if sha != "" {
		updatePayload["sha"] = sha
	}

	payloadBytes, _ := json.Marshal(updatePayload)
	putReq, err := http.NewRequest("PUT", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		log.Printf("[feedback] failed to create PUT request for %s: %v", filePath, err)
		return
	}
	putReq.Header.Set("Authorization", "Bearer "+s.config.GitHubPAT)
	putReq.Header.Set("Accept", "application/vnd.github+json")
	putReq.Header.Set("Content-Type", "application/json")

	putResp, err := githubHTTPClient.Do(putReq)
	if err != nil {
		log.Printf("[feedback] failed to update %s: %v", filePath, err)
		return
	}
	defer putResp.Body.Close()
	putBody, _ := io.ReadAll(putResp.Body)

	if putResp.StatusCode >= 300 {
		log.Printf("[feedback] GitHub API returned %d updating %s: %s", putResp.StatusCode, filePath, string(putBody))
		return
	}

	log.Printf("[feedback] updated %s with %s requirement", filePath, urgency)
}

// handleFeedbackList handles GET /api/feedback — list my own feedback submissions.
func (s *Server) handleFeedbackList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	session, ok := s.requireFeedbackUser(w, r)
	if !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT id, user_id, body, type, urgency, status, admin_notes, github_issue_url, created_at, COALESCE(updated_at, '')
		FROM feedback_requests
		WHERE user_id = ?
		ORDER BY id DESC
	`, session.UserID)
	if err != nil {
		dbError(w, err, "Failed to fetch feedback")
		return
	}
	defer rows.Close()

	items := make([]FeedbackRequest, 0)
	for rows.Next() {
		var f FeedbackRequest
		if err := rows.Scan(&f.ID, &f.UserID, &f.Body, &f.Type, &f.Urgency, &f.Status, &f.AdminNotes, &f.GitHubIssueURL, &f.CreatedAt, &f.UpdatedAt); err != nil {
			log.Printf("[warn] scanning feedback: %v", err)
			continue
		}
		items = append(items, f)
	}

	writeJSON(w, http.StatusOK, items)
}

// handleAdminFeedback handles GET /api/admin/feedback — list ALL feedback (admin view).
func (s *Server) handleAdminFeedback(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	query := `
		SELECT f.id, f.user_id, f.body, f.type, f.urgency, f.status, f.admin_notes, f.github_issue_url, f.created_at, COALESCE(f.updated_at, ''),
			COALESCE(u.name, ''), COALESCE(u.phone_number, '')
		FROM feedback_requests f
		LEFT JOIN users u ON u.id = f.user_id
		WHERE 1=1
	`
	args := make([]interface{}, 0)

	if status := r.URL.Query().Get("status"); status != "" && validFeedbackStatuses[status] {
		query += " AND f.status = ?"
		args = append(args, status)
	}
	if typ := r.URL.Query().Get("type"); typ != "" && validFeedbackTypes[typ] {
		query += " AND f.type = ?"
		args = append(args, typ)
	}
	if urg := r.URL.Query().Get("urgency"); urg != "" && validFeedbackUrgency[urg] {
		query += " AND f.urgency = ?"
		args = append(args, urg)
	}

	query += " ORDER BY f.id DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		dbError(w, err, "Failed to fetch feedback")
		return
	}
	defer rows.Close()

	items := make([]FeedbackRequest, 0)
	for rows.Next() {
		var f FeedbackRequest
		var userName, userPhone sql.NullString
		if err := rows.Scan(&f.ID, &f.UserID, &f.Body, &f.Type, &f.Urgency, &f.Status, &f.AdminNotes, &f.GitHubIssueURL, &f.CreatedAt, &f.UpdatedAt,
			&userName, &userPhone); err != nil {
			log.Printf("[warn] scanning admin feedback: %v", err)
			continue
		}
		f.UserName = userName.String
		f.UserPhone = userPhone.String
		items = append(items, f)
	}

	writeJSON(w, http.StatusOK, items)
}

// handleAdminFeedbackByID handles PUT /api/admin/feedback/:id — triage a feedback item.
func (s *Server) handleAdminFeedbackByID(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPut) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/feedback/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid feedback ID"})
		return
	}

	var req struct {
		Status     *string `json:"status"`
		AdminNotes *string `json:"admin_notes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	var exists int
	s.db.QueryRow("SELECT COUNT(*) FROM feedback_requests WHERE id = ?", id).Scan(&exists)
	if exists == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Feedback not found"})
		return
	}

	sets := make([]string, 0)
	args := make([]interface{}, 0)
	if req.Status != nil {
		if !validFeedbackStatuses[*req.Status] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Status must be new, reviewed, roadmapped, or done"})
			return
		}
		sets = append(sets, "status = ?")
		args = append(args, *req.Status)
	}
	if req.AdminNotes != nil {
		sets = append(sets, "admin_notes = ?")
		args = append(args, strings.TrimSpace(*req.AdminNotes))
	}

	if len(sets) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No fields to update"})
		return
	}

	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, id)
	_, err = s.db.Exec("UPDATE feedback_requests SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
	if err != nil {
		dbError(w, err, "Failed to update feedback")
		return
	}

	log.Printf("[feedback] admin triaged feedback %d", id)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
