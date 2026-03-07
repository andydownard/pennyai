package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// =============================================================================
// VCARD ENDPOINT
// =============================================================================

// handleVCard handles GET /api/penny.vcf
func (s *Server) handleVCard(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	phoneNumber := s.config.TwilioPhoneNumber
	if phoneNumber == "" {
		phoneNumber = "+10000000000"
	}

	vcard := fmt.Sprintf(`BEGIN:VCARD
VERSION:3.0
N:AI;Penny;;;
FN:Penny AI
ORG:Thriftly
TITLE:Shopping Assistant
TEL;TYPE=CELL:%s
EMAIL:penny@list.thrift.ly
END:VCARD`, phoneNumber)

	w.Header().Set("Content-Type", "text/vcard")
	w.Header().Set("Content-Disposition", "attachment; filename=\"penny.vcf\"")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, vcard)
}

// =============================================================================
// FAMILY INVITE FLOW
// =============================================================================

// handleInvite handles POST /api/settings/invite
func (s *Server) handleInvite(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Phone string `json:"phone"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	// Phone is optional — if provided, validate it
	var e164 string
	hasPhone := strings.TrimSpace(req.Phone) != ""
	if hasPhone {
		var phoneOK bool
		e164, phoneOK = normalizePhone(req.Phone)
		if !phoneOK {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid phone number"})
			return
		}
	}

	// Get user's household
	var householdID sql.NullInt64
	err := s.db.QueryRow("SELECT household_id FROM users WHERE id = ?", session.UserID).Scan(&householdID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "No household found"})
		return
	}

	// Auto-create household if user doesn't have one (legacy users before ALTER TABLE)
	if !householdID.Valid {
		hID, err := createSoloHousehold(s.db, session.UserID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create household"})
			return
		}
		householdID = sql.NullInt64{Int64: int64(hID), Valid: true}
	}

	// Generate random invite code
	codeBytes := make([]byte, 16)
	if _, err := rand.Read(codeBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate code"})
		return
	}
	code := hex.EncodeToString(codeBytes)

	// Store invite (7-day expiry)
	expiresAt := time.Now().Add(inviteExpiry)
	_, err = s.db.Exec(
		"INSERT INTO invites (household_id, code, created_by, expires_at) VALUES (?, ?, ?, ?)",
		householdID.Int64, code, session.UserID, expiresAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create invite"})
		return
	}

	// Send SMS invite only if phone was provided
	if hasPhone {
		smsBody := fmt.Sprintf("Join my Thriftly list! https://app.thrift.ly/join/%s", code)
		if err := s.twilio.SendSMS(s.config.TwilioPhoneNumber, e164, smsBody); err != nil {
			log.Printf("[invite] error sending SMS to %s: %v", e164, err)
			// Still return success — invite is created, SMS delivery is best-effort
		}
	}

	link := fmt.Sprintf("https://app.thrift.ly/join/%s", code)
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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM household_members WHERE user_id = ?", session.UserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to leave household"})
		return
	}
	if _, err := tx.Exec("UPDATE users SET household_id = NULL WHERE id = ?", session.UserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to leave household"})
		return
	}

	// Create new solo household atomically within the same transaction
	if _, err := createSoloHouseholdTx(tx, session.UserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create household"})
		return
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to leave household"})
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Code is required"})
		return
	}

	// Look up invite
	var inviteID int
	var householdID int
	var createdBy int
	var expiresAt time.Time
	var usedBy sql.NullInt64
	err := s.db.QueryRow(
		"SELECT id, household_id, created_by, expires_at, used_by FROM invites WHERE code = ?",
		req.Code,
	).Scan(&inviteID, &householdID, &createdBy, &expiresAt, &usedBy)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "That invite code doesn't look right. Please check and try again."})
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "You can't join your own household."})
		return
	}

	// B7: Check if user is already in the target household (don't burn the invite)
	var currentHouseholdID sql.NullInt64
	s.db.QueryRow("SELECT household_id FROM users WHERE id = ?", session.UserID).Scan(&currentHouseholdID)
	if currentHouseholdID.Valid && int(currentHouseholdID.Int64) == householdID {
		writeJSON(w, http.StatusOK, map[string]string{"message": "You're already in this household."})
		return
	}

	// B3: Warn if user is in a different household (they'll be moved)
	// The frontend can show this warning; here we include it in response
	// but still proceed since the user explicitly chose to join
	leavingHousehold := currentHouseholdID.Valid && int(currentHouseholdID.Int64) != householdID

	// Move user to the inviter's household — all in one transaction
	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't join you to this household. Please try again."})
		return
	}
	defer tx.Rollback()

	// Atomically claim the invite — prevents TOCTOU race with concurrent requests
	claimResult, err := tx.Exec(
		"UPDATE invites SET used_by = ?, used_at = CURRENT_TIMESTAMP WHERE id = ? AND used_by IS NULL AND expires_at > ?",
		session.UserID, inviteID, time.Now(),
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't join you to this household. Please try again."})
		return
	}
	if rows, _ := claimResult.RowsAffected(); rows == 0 {
		writeJSON(w, http.StatusGone, map[string]string{"error": "This invite has already been used or expired."})
		return
	}

	_, err = tx.Exec("UPDATE users SET household_id = ? WHERE id = ?", householdID, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't join you to this household. Please try again."})
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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't join you to this household. Please try again."})
		return
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't join you to this household. Please try again."})
		return
	}

	// Award points to inviter for household invite
	if createdBy > 0 {
		s.db.Exec("UPDATE users SET referral_points = COALESCE(referral_points, 0) + ? WHERE id = ?", referralPointsPerInvite, createdBy)
		log.Printf("[invite] awarded %d points to user %d for household invite", referralPointsPerInvite, createdBy)
	}

	resp := map[string]interface{}{"success": true}
	if leavingHousehold {
		resp["warning"] = "You left your previous household to join this one."
	}
	writeJSON(w, http.StatusOK, resp)
}

// =============================================================================
