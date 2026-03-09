package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleTrips(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Get household_id for the user
	var householdID sql.NullInt64
	s.db.QueryRow("SELECT household_id FROM users WHERE id = ?", session.UserID).Scan(&householdID)
	if !householdID.Valid {
		// User not in a household — use a synthetic household_id of -userID
		householdID.Int64 = int64(-session.UserID)
		householdID.Valid = true
	}

	switch r.Method {
	case http.MethodGet:
		// Clean up old trips (older than 48 hours)
		s.db.Exec("DELETE FROM shopping_trips WHERE trip_date < date('now', '-2 days')")

		// Get all trips for the household
		var rows *sql.Rows
		var err error
		if householdID.Int64 < 0 {
			// Solo user — just their own trips
			rows, err = s.db.Query(
				`SELECT st.id, st.user_id, COALESCE(u.name, ''), st.trip_date, st.trip_time, st.time_label, st.created_at
				 FROM shopping_trips st JOIN users u ON u.id = st.user_id
				 WHERE st.user_id = ? ORDER BY st.trip_date, st.trip_time`,
				session.UserID,
			)
		} else {
			rows, err = s.db.Query(
				`SELECT st.id, st.user_id, COALESCE(u.name, ''), st.trip_date, st.trip_time, st.time_label, st.created_at
				 FROM shopping_trips st JOIN users u ON u.id = st.user_id
				 WHERE st.household_id = ? ORDER BY st.trip_date, st.trip_time`,
				householdID.Int64,
			)
		}
		if err != nil {
			dbError(w, err, "Failed to load trips")
			return
		}
		defer rows.Close()

		type TripResponse struct {
			ID        int    `json:"id"`
			UserID    int    `json:"user_id"`
			UserName  string `json:"user_name"`
			TripDate  string `json:"trip_date"`
			TripTime  string `json:"trip_time"`
			TimeLabel string `json:"time_label"`
			IsMine    bool   `json:"is_mine"`
			CreatedAt string `json:"created_at"`
		}
		trips := []TripResponse{}
		for rows.Next() {
			var t TripResponse
			var tripTime, timeLabel, createdAt sql.NullString
			if err := rows.Scan(&t.ID, &t.UserID, &t.UserName, &t.TripDate, &tripTime, &timeLabel, &createdAt); err != nil {
				log.Printf("[trips] scan error: %v", err)
				continue
			}
			t.TripTime = tripTime.String
			t.TimeLabel = timeLabel.String
			t.CreatedAt = createdAt.String
			t.IsMine = t.UserID == session.UserID
			trips = append(trips, t)
		}
		writeJSON(w, http.StatusOK, trips)

	case http.MethodPost:
		var req struct {
			TripDate  string `json:"trip_date"`
			TripTime  string `json:"trip_time"`
			TimeLabel string `json:"time_label"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1*1024*1024)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
			return
		}
		if req.TripDate == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "trip_date is required"})
			return
		}
		// Validate date format YYYY-MM-DD
		if _, err := time.Parse("2006-01-02", req.TripDate); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid date format"})
			return
		}

		// Delete any existing future trip for this user (one active trip per user)
		s.db.Exec("DELETE FROM shopping_trips WHERE user_id = ? AND trip_date >= date('now', '-1 day')", session.UserID)

		// Use real household_id or 0 for solo users
		hid := householdID.Int64
		if hid < 0 {
			hid = 0
		}

		var tripTime, timeLabel *string
		if req.TripTime != "" {
			tripTime = &req.TripTime
		}
		if req.TimeLabel != "" {
			timeLabel = &req.TimeLabel
		}

		result, err := s.db.Exec(
			"INSERT INTO shopping_trips (user_id, household_id, trip_date, trip_time, time_label) VALUES (?, ?, ?, ?, ?)",
			session.UserID, hid, req.TripDate, tripTime, timeLabel,
		)
		if err != nil {
			dbError(w, err, "Failed to save trip")
			return
		}
		id, _ := result.LastInsertId()
		writeJSON(w, http.StatusOK, map[string]interface{}{"id": id, "ok": true})

	case http.MethodDelete:
		s.db.Exec("DELETE FROM shopping_trips WHERE user_id = ? AND trip_date >= date('now', '-1 day')", session.UserID)
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// handleBrands returns a JSON map of lowercased brand name → domain.
// No auth required — brand data is public and needed for fast page load.

// =============================================================================
// PLACES PROXY HANDLERS
// =============================================================================

// handlePlacesAutocomplete proxies Google Places autocomplete to keep the API key server-side.
func (s *Server) handlePlacesAutocomplete(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireSession(w, r); !ok {
		return
	}
	input := r.URL.Query().Get("input")
	if len(input) < 3 {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}

	url := fmt.Sprintf("https://places.googleapis.com/v1/places:autocomplete")
	body := fmt.Sprintf(`{"input":%q,"includedPrimaryTypes":["street_address","subpremise","route","locality","postal_code"],"includedRegionCodes":["us"],"languageCode":"en"}`, input)
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", s.config.GooglePlacesAPIKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "places api error"})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Suggestions []struct {
			PlacePrediction struct {
				PlaceID string `json:"placeId"`
				Text    struct {
					Text string `json:"text"`
				} `json:"text"`
			} `json:"placePrediction"`
		} `json:"suggestions"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}

	items := make([]map[string]string, 0, len(result.Suggestions))
	for _, s := range result.Suggestions {
		items = append(items, map[string]string{
			"description": s.PlacePrediction.Text.Text,
			"placeId":     s.PlacePrediction.PlaceID,
		})
	}
	writeJSON(w, http.StatusOK, items)
}

// handlePlacesDetail fetches the formatted address for a place ID.
func (s *Server) handlePlacesDetail(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireSession(w, r); !ok {
		return
	}
	placeID := r.URL.Query().Get("place_id")
	if placeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "place_id required"})
		return
	}

	url := fmt.Sprintf("https://places.googleapis.com/v1/places/%s", placeID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Goog-Api-Key", s.config.GooglePlacesAPIKey)
	req.Header.Set("X-Goog-FieldMask", "formattedAddress")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "places api error"})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		FormattedAddress string `json:"formattedAddress"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil || result.FormattedAddress == "" {
		writeJSON(w, http.StatusOK, map[string]string{"address": ""})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"address": result.FormattedAddress})
}

// =============================================================================
// SETTINGS HANDLERS
// =============================================================================

// handleSettings multiplexes GET/PATCH for /api/settings
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleSettingsGet(w, r)
	case http.MethodPatch:
		s.handleSettingsUpdate(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// handleSettingsGet handles GET /api/settings
func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var name, email, phone, address, pennyBrandMode, onboardingFlags sql.NullString
	var pennySlider, pennyAutotune, pennyDeals, swapCreditsCents int
	err := s.db.QueryRow(
		"SELECT COALESCE(name,''), COALESCE(email,''), phone_number, COALESCE(address,''), COALESCE(penny_brand_mode,'ask'), COALESCE(penny_slider,35), COALESCE(penny_autotune,1), COALESCE(penny_deals,0), COALESCE(swap_credits_cents,0), COALESCE(onboarding_flags,'{}') FROM users WHERE id = ?",
		session.UserID,
	).Scan(&name, &email, &phone, &address, &pennyBrandMode, &pennySlider, &pennyAutotune, &pennyDeals, &swapCreditsCents, &onboardingFlags)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load settings"})
		return
	}

	// Get household members in a single query (avoids N+1)
	type memberInfo struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	var members []memberInfo
	memberRows, err := s.db.Query(
		`SELECT COALESCE(u.name,''), u.phone_number FROM users u
		 JOIN household_members hm ON hm.user_id = u.id
		 WHERE hm.household_id = (SELECT household_id FROM users WHERE id = ?)
		 AND u.id != ?`,
		session.UserID, session.UserID,
	)
	if err == nil {
		defer memberRows.Close()
		for memberRows.Next() {
			var mName, mPhone sql.NullString
			if memberRows.Scan(&mName, &mPhone) == nil {
				members = append(members, memberInfo{Name: mName.String, Phone: mPhone.String})
			}
		}
	}
	if members == nil {
		members = []memberInfo{}
	}

	// M2c: Get household list name
	var listName string
	var userHouseholdID sql.NullInt64
	s.db.QueryRow("SELECT household_id FROM users WHERE id = ?", session.UserID).Scan(&userHouseholdID)
	if userHouseholdID.Valid {
		s.db.QueryRow("SELECT COALESCE(name,'') FROM households WHERE id = ?", userHouseholdID.Int64).Scan(&listName)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":               name.String,
		"email":              email.String,
		"phone":              phone.String,
		"address":            address.String,
		"penny_phone":        s.config.TwilioPhoneNumber,
		"household_members":  members,
		"google_places_ready": s.config.GooglePlacesAPIKey != "",
		"penny_brand_mode":  pennyBrandMode.String,
		"penny_slider":      pennySlider,
		"penny_autotune":    pennyAutotune == 1,
		"penny_deals":        pennyDeals == 1,
		"swap_credits_cents": swapCreditsCents,
		"list_name":          listName,
		"onboarding_flags":   onboardingFlags.String,
	})
}

// addSetClause appends a "column = ?" clause and its value to the SQL builder slices.
func addSetClause(setClauses *[]string, args *[]interface{}, column string, value interface{}) {
	*setClauses = append(*setClauses, column+" = ?")
	*args = append(*args, value)
}

// handleSettingsUpdate handles PATCH /api/settings
func (s *Server) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Name            *string `json:"name"`
		Email           *string `json:"email"`
		Phone           *string `json:"phone"`
		Address         *string `json:"address"`
		PennyBrandMode  *string `json:"penny_brand_mode"`
		PennySlider     *int    `json:"penny_slider"`
		PennyAutotune   *bool   `json:"penny_autotune"`
		PennyDeals      *bool   `json:"penny_deals"`
		ListName        *string `json:"list_name"`
		OnboardingFlags *string `json:"onboarding_flags"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.PennyBrandMode != nil {
		mode := strings.TrimSpace(*req.PennyBrandMode)
		if mode != "auto" && mode != "ask" && mode != "deals" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid penny_brand_mode"})
			return
		}
	}

	// Validate email before making any changes
	var email string
	if req.Email != nil {
		email = strings.TrimSpace(strings.ToLower(*req.Email))
		if email != "" {
			if _, err := mail.ParseAddress(email); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid email format"})
				return
			}
			var existingID int
			err := s.db.QueryRow("SELECT id FROM users WHERE email = ? AND id != ?", email, session.UserID).Scan(&existingID)
			if err == nil {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "Email already in use"})
				return
			}
		}
	}

	// Build a single UPDATE with all provided fields
	setClauses := []string{}
	args := []interface{}{}
	if req.PennyBrandMode != nil {
		addSetClause(&setClauses, &args, "penny_brand_mode", strings.TrimSpace(*req.PennyBrandMode))
	}
	if req.Name != nil {
		addSetClause(&setClauses, &args, "name", strings.TrimSpace(*req.Name))
		addSetClause(&setClauses, &args, "name_source", "manual")
	}
	if req.Email != nil {
		addSetClause(&setClauses, &args, "email", email)
	}
	if req.Phone != nil {
		phone := strings.TrimSpace(*req.Phone)
		if phone != "" {
			// Normalize: ensure +1 prefix, digits only
			phone = strings.ReplaceAll(phone, "-", "")
			phone = strings.ReplaceAll(phone, "(", "")
			phone = strings.ReplaceAll(phone, ")", "")
			phone = strings.ReplaceAll(phone, " ", "")
			if !strings.HasPrefix(phone, "+1") {
				phone = "+1" + phone
			}
			if len(phone) != 12 { // +1XXXXXXXXXX
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid phone number"})
				return
			}
		}
		addSetClause(&setClauses, &args, "phone_number", phone)
	}
	if req.Address != nil {
		addSetClause(&setClauses, &args, "address", strings.TrimSpace(*req.Address))
	}
	if req.PennySlider != nil {
		v := *req.PennySlider
		if v < 0 {
			v = 0
		} else if v > 100 {
			v = 100
		}
		addSetClause(&setClauses, &args, "penny_slider", v)
	}
	if req.PennyAutotune != nil {
		val := 0
		if *req.PennyAutotune {
			val = 1
		}
		addSetClause(&setClauses, &args, "penny_autotune", val)
	}
	if req.PennyDeals != nil {
		val := 0
		if *req.PennyDeals {
			val = 1
		}
		addSetClause(&setClauses, &args, "penny_deals", val)
	}
	if req.OnboardingFlags != nil {
		addSetClause(&setClauses, &args, "onboarding_flags", *req.OnboardingFlags)
	}
	if len(setClauses) > 0 {
		args = append(args, session.UserID)
		query := "UPDATE users SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
		if _, err := s.db.Exec(query, args...); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update settings"})
			return
		}
	}

	// M2c: Update household list name
	if req.ListName != nil {
		var householdID sql.NullInt64
		s.db.QueryRow("SELECT household_id FROM users WHERE id = ?", session.UserID).Scan(&householdID)
		if householdID.Valid {
			if _, err := s.db.Exec("UPDATE households SET name = ? WHERE id = ?", strings.TrimSpace(*req.ListName), householdID.Int64); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update list name"})
				return
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// =============================================================================
// AVATAR HANDLERS
// =============================================================================

const maxAvatarBytes = 2 * 1024 * 1024 // 2MB decoded limit

// handleAvatar multiplexes GET/POST/DELETE for /api/avatar
func (s *Server) handleAvatar(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleAvatarGet(w, r)
	case http.MethodPost:
		s.handleAvatarUpload(w, r)
	case http.MethodDelete:
		s.handleAvatarDelete(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// handleAvatarGet serves GET /api/avatar/{id} — public (no auth), serves stored photo.
func (s *Server) handleAvatarGet(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from path: /api/avatar/123
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "User ID required"})
		return
	}
	idStr := parts[len(parts)-1]
	userID, err := strconv.Atoi(idStr)
	if err != nil || userID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
		return
	}

	var photoData []byte
	var contentType string
	err = s.db.QueryRow("SELECT photo_data, content_type FROM profile_photos WHERE user_id = ?", userID).Scan(&photoData, &contentType)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "No profile photo"})
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Length", strconv.Itoa(len(photoData)))
	w.Write(photoData)
}

// handleAvatarUpload handles POST /api/avatar — upload a base64-encoded photo.
func (s *Server) handleAvatarUpload(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Photo string `json:"photo"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Photo == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Photo is required"})
		return
	}

	// Decode base64
	photoData, err := base64.StdEncoding.DecodeString(req.Photo)
	if err != nil {
		// Try URL-safe base64
		photoData, err = base64.URLEncoding.DecodeString(req.Photo)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid base64 encoding"})
			return
		}
	}

	if len(photoData) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Photo is required"})
		return
	}

	if len(photoData) > maxAvatarBytes {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Photo too large (max 2MB)"})
		return
	}

	// Validate magic bytes: JPEG (FF D8) or PNG (89 50 4E 47)
	var contentType string
	if len(photoData) >= 2 && photoData[0] == 0xFF && photoData[1] == 0xD8 {
		contentType = "image/jpeg"
	} else if len(photoData) >= 4 && photoData[0] == 0x89 && photoData[1] == 0x50 && photoData[2] == 0x4E && photoData[3] == 0x47 {
		contentType = "image/png"
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Only JPEG and PNG images are allowed"})
		return
	}

	// Upsert into profile_photos
	_, err = s.db.Exec(
		`INSERT INTO profile_photos (user_id, photo_data, content_type, updated_at)
		 VALUES (?, ?, ?, datetime('now'))
		 ON CONFLICT(user_id) DO UPDATE SET photo_data = excluded.photo_data, content_type = excluded.content_type, updated_at = datetime('now')`,
		session.UserID, photoData, contentType,
	)
	if err != nil {
		dbError(w, err, "Failed to save profile photo")
		return
	}

	log.Printf("user %d uploaded profile photo (%d bytes)", session.UserID, len(photoData))
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleAvatarDelete handles DELETE /api/avatar — remove profile photo.
func (s *Server) handleAvatarDelete(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	s.db.Exec("DELETE FROM profile_photos WHERE user_id = ?", session.UserID)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// =============================================================================
// REFERRAL SYSTEM
// =============================================================================

// handleReferral handles GET /api/referral — returns referral code, points, and friends list.
func (s *Server) handleReferral(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Get or generate referral code
	var referralCode sql.NullString
	var points int
	err := s.db.QueryRow("SELECT referral_code, COALESCE(referral_points, 0) FROM users WHERE id = ?", session.UserID).Scan(&referralCode, &points)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't load your referral info. Please try again."})
		return
	}

	code := referralCode.String
	if !referralCode.Valid || code == "" {
		// Generate a new referral code
		codeBytes := make([]byte, 16)
		if _, err := rand.Read(codeBytes); err != nil {
			log.Printf("[referral] failed to generate code: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate referral code"})
			return
		}
		code = hex.EncodeToString(codeBytes)
		result, err := s.db.Exec("UPDATE users SET referral_code = ? WHERE id = ? AND (referral_code IS NULL OR referral_code = '')", code, session.UserID)
		if err != nil {
			log.Printf("[referral] failed to save referral code: %v", err)
		} else if n, _ := result.RowsAffected(); n == 0 {
			// Another request already set the code — re-read it
			s.db.QueryRow("SELECT referral_code FROM users WHERE id = ?", session.UserID).Scan(&code)
		}
	}

	// Get referred friends
	type referredFriend struct {
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
	}
	var friends []referredFriend
	rows, err := s.db.Query(
		"SELECT COALESCE(u.name, ''), r.created_at FROM referrals r JOIN users u ON u.id = r.referred_id WHERE r.referrer_id = ? ORDER BY r.created_at DESC",
		session.UserID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var f referredFriend
			if rows.Scan(&f.Name, &f.CreatedAt) == nil {
				friends = append(friends, f)
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("[referral] friends row iteration error: %v", err)
		}
	}
	if friends == nil {
		friends = []referredFriend{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code":           code,
		"points":         points,
		"referral_count": len(friends),
		"friends":        friends,
	})
}

// handleReferralRedeem handles POST /api/referral/redeem — redeem 100 points for $10.
func (s *Server) handleReferralRedeem(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your redemption. Please try again."})
		return
	}
	defer tx.Rollback()

	// Atomic check-and-deduct to prevent TOCTOU race on point balance
	result, err := tx.Exec("UPDATE users SET referral_points = referral_points - ? WHERE id = ? AND COALESCE(referral_points, 0) >= ?", referralRedeemThreshold, session.UserID, referralRedeemThreshold)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your redemption. Please try again."})
		return
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "You need at least 100 points to redeem."})
		return
	}
	if _, err := tx.Exec("INSERT INTO redemptions (user_id, points, amount_cents) VALUES (?, ?, ?)", session.UserID, referralRedeemThreshold, referralRedeemCents); err != nil {
		log.Printf("[referral] failed to insert redemption: %v", err)
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your redemption. Please try again."})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "$10 will be Venmo'd to your number!"})
}

// handleReferralClaim handles POST /api/referral/claim — called on signup with a referral code.
func (s *Server) handleReferralClaim(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Referral code is required."})
		return
	}

	// Look up referrer by code
	var referrerID int
	err := s.db.QueryRow("SELECT id FROM users WHERE referral_code = ?", req.Code).Scan(&referrerID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Invalid referral code."})
		return
	}

	// Don't let users refer themselves
	if referrerID == session.UserID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "You can't use your own referral code."})
		return
	}

	// Idempotent — check if already claimed
	var existingID int
	err = s.db.QueryRow("SELECT id FROM referrals WHERE referrer_id = ? AND referred_id = ?", referrerID, session.UserID).Scan(&existingID)
	if err == nil {
		// Already claimed — return success without double-crediting
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your referral. Please try again."})
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec("INSERT INTO referrals (referrer_id, referred_id) VALUES (?, ?)", referrerID, session.UserID); err != nil {
		log.Printf("[referral] failed to insert referral: %v", err)
	}
	if _, err := tx.Exec("UPDATE users SET referral_points = COALESCE(referral_points, 0) + ? WHERE id = ?", referralPointsPerInvite, referrerID); err != nil {
		log.Printf("[referral] failed to update points: %v", err)
	}
	// M2d: Track referral source
	tx.Exec("UPDATE users SET referral_source = 'referral' WHERE id = ? AND referral_source = ''", session.UserID)

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your referral. Please try again."})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// =============================================================================
// REBATE HISTORY
// =============================================================================

// handleRebates handles GET /api/rebates — returns user's rebate history and summary.
func (s *Server) handleRebates(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	type rebateEntry struct {
		ID           int    `json:"id"`
		OfferBrand   string `json:"offer_brand"`
		OfferProduct string `json:"offer_product"`
		AmountCents  int    `json:"amount_cents"`
		Status       string `json:"status"`
		CreatedAt    string `json:"created_at"`
	}

	rows, err := s.db.Query(`
		SELECT sr.id, bd.offer_brand, bd.offer_product, sr.amount_cents, sr.status, sr.created_at
		FROM swap_rebates sr
		JOIN brand_deals bd ON bd.id = sr.deal_id
		WHERE sr.user_id = ?
		ORDER BY sr.created_at DESC
		LIMIT 100
	`, session.UserID)
	if err != nil {
		log.Printf("[rebates] query error for user %d: %v", session.UserID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load rebates"})
		return
	}
	defer rows.Close()

	rebates := []rebateEntry{}
	for rows.Next() {
		var e rebateEntry
		if err := rows.Scan(&e.ID, &e.OfferBrand, &e.OfferProduct, &e.AmountCents, &e.Status, &e.CreatedAt); err != nil {
			log.Printf("[rebates] scan error: %v", err)
			continue
		}
		rebates = append(rebates, e)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[rebates] row iteration error: %v", err)
	}

	// Summary totals
	var pendingCents, paidCents, totalCents int
	for _, r := range rebates {
		totalCents += r.AmountCents
		switch r.Status {
		case "pending":
			pendingCents += r.AmountCents
		case "issued", "paid":
			paidCents += r.AmountCents
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rebates":       rebates,
		"total_cents":   totalCents,
		"pending_cents": pendingCents,
		"paid_cents":    paidCents,
	})
}

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

	writeJSON(w, http.StatusOK, map[string]bool{"show_nps": recentCount == 0})
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Score must be 0-10"})
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

// =============================================================================
