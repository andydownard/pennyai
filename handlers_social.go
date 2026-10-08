package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"
)

// =============================================================================
// FAMILY INVITE FLOW
// =============================================================================

// handleInvite handles POST /api/settings/invite. It creates an invite code and
// returns the join link for the caller to share. It sends nothing; any body
// fields are ignored.
func (s *Server) handleInvite(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Get user's household
	invHID := getUserHouseholdID(s.db, session.UserID)

	// Auto-create household if user doesn't have one (legacy users before ALTER TABLE)
	if invHID == 0 {
		hID, err := createSoloHousehold(s.db, session.UserID)
		if err != nil {
			dbError(w, err, "Failed to create household")
			return
		}
		invHID = int64(hID)
	}

	// One user may mint a limited number of invite links per day.
	if checkRateLimit(&s.inviteLimiter, "inviter:"+session.UserID, inviteLinkWindow, inviteLinksPerInviterPerDay) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{
			"error": "You've made a lot of invites today. Try again tomorrow.",
		})
		return
	}

	// Generate random invite code
	codeBytes := make([]byte, 16)
	if _, err := rand.Read(codeBytes); err != nil {
		dbError(w, err, "Failed to generate code")
		return
	}
	code := hex.EncodeToString(codeBytes)

	// Store invite (7-day expiry)
	expiresAt := time.Now().Add(inviteExpiry)
	_, err := s.db.Exec(
		"INSERT INTO invites (household_id, code, created_by, expires_at) VALUES (?, ?, ?, ?)",
		invHID, code, session.UserID, expiresAt,
	)
	if err != nil {
		dbError(w, err, "Failed to create invite")
		return
	}

	link := fmt.Sprintf("%s/join/%s", s.config.AppBaseURL, code)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"code":    code,
		"link":    link,
	})
}

// handleLeaveHousehold handles POST /api/settings/leave-household
func (s *Server) handleLeaveHousehold(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Remove from current household and create a new solo one
	tx, err := s.db.Begin()
	if err != nil {
		dbError(w, err, "Failed to start transaction")
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM household_members WHERE user_id = ?", session.UserID); err != nil {
		dbError(w, err, "Failed to leave household")
		return
	}
	if _, err := tx.Exec("UPDATE users SET household_id = NULL WHERE id = ?", session.UserID); err != nil {
		dbError(w, err, "Failed to leave household")
		return
	}

	// Create new solo household atomically within the same transaction
	if _, err := createSoloHouseholdTx(tx, session.UserID); err != nil {
		dbError(w, err, "Failed to create household")
		return
	}

	if err := tx.Commit(); err != nil {
		dbError(w, err, "Failed to leave household")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// handleJoin handles POST /api/join
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Code == "" {
		writeBadRequest(w, "Code is required")
		return
	}

	// Look up invite
	var inviteID int
	var householdID int
	var createdBy string
	var expiresAt time.Time
	var usedBy sql.NullString
	err := s.db.QueryRow(
		"SELECT id, household_id, created_by, expires_at, used_by FROM invites WHERE code = ?",
		req.Code,
	).Scan(&inviteID, &householdID, &createdBy, &expiresAt, &usedBy)
	if err != nil {
		writeNotFound(w, "That invite code doesn't look right. Please check and try again.")
		return
	}

	if usedBy.Valid {
		writeJSON(w, http.StatusGone, map[string]string{"error": "This invite has already been used."})
		return
	}

	if time.Now().After(expiresAt) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "This invite has expired. Ask for a new one."})
		return
	}

	if createdBy == session.UserID {
		writeBadRequest(w, "You can't join your own household.")
		return
	}

	// B7: Check if user is already in the target household (don't burn the invite)
	curHID := getUserHouseholdID(s.db, session.UserID)
	if curHID != 0 && int(curHID) == householdID {
		writeJSON(w, http.StatusOK, map[string]string{"message": "You're already in this household."})
		return
	}

	// B3: Warn if user is in a different household (they'll be moved)
	// The frontend can show this warning; here we include it in response
	// but still proceed since the user explicitly chose to join
	leavingHousehold := curHID != 0 && int(curHID) != householdID

	// Move user to the inviter's household — all in one transaction
	tx, err := s.db.Begin()
	if err != nil {
		dbError(w, err, "We couldn't join you to this household. Please try again.")
		return
	}
	defer tx.Rollback()

	// Atomically claim the invite — prevents TOCTOU race with concurrent requests
	claimResult, err := tx.Exec(
		"UPDATE invites SET used_by = ?, used_at = CURRENT_TIMESTAMP WHERE id = ? AND used_by IS NULL AND expires_at > ?",
		session.UserID, inviteID, time.Now(),
	)
	if err != nil {
		dbError(w, err, "We couldn't join you to this household. Please try again.")
		return
	}
	if rows, _ := claimResult.RowsAffected(); rows == 0 {
		writeJSON(w, http.StatusGone, map[string]string{"error": "This invite has already been used or expired."})
		return
	}

	_, err = tx.Exec("UPDATE users SET household_id = ? WHERE id = ?", householdID, session.UserID)
	if err != nil {
		dbError(w, err, "We couldn't join you to this household. Please try again.")
		return
	}

	if _, err := tx.Exec("DELETE FROM household_members WHERE user_id = ?", session.UserID); err != nil {
		log.Printf("[join] failed to delete old household member: %v", err)
	}
	_, err = tx.Exec(
		"INSERT INTO household_members (household_id, user_id, role, invited_by) VALUES (?, ?, 'member', ?)",
		householdID, session.UserID, createdBy,
	)
	if err != nil {
		dbError(w, err, "We couldn't join you to this household. Please try again.")
		return
	}

	if err := tx.Commit(); err != nil {
		dbError(w, err, "We couldn't join you to this household. Please try again.")
		return
	}

	// Award points to inviter for household invite
	if createdBy != "" {
		s.db.Exec("UPDATE users SET referral_points = COALESCE(referral_points, 0) + ? WHERE id = ?", referralPointsPerInvite, createdBy)
		log.Printf("[invite] awarded %d points to user %s for household invite", referralPointsPerInvite, createdBy)
	}
	// M2d: Track referral source
	s.db.Exec("UPDATE users SET referral_source = 'invite' WHERE id = ? AND referral_source = ''", session.UserID)

	// M2b: Penny acknowledges new household member
	var joinerName string
	s.db.QueryRow("SELECT COALESCE(name,'') FROM users WHERE id = ?", session.UserID).Scan(&joinerName)
	pennyMsg := "Welcome to the family! You're all sharing one list now."
	if joinerName != "" {
		pennyMsg = fmt.Sprintf("Welcome aboard, %s! You're all sharing one list now.", joinerName)
	}

	resp := map[string]interface{}{"success": true, "penny_message": pennyMsg}
	if leavingHousehold {
		resp["warning"] = "You left your previous household to join this one."
	}
	writeJSON(w, http.StatusOK, resp)
}

// =============================================================================
