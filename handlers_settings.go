package main

import (
	"database/sql"
	"encoding/base64"
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
	hid := getUserHouseholdID(s.db, session.UserID)

	switch r.Method {
	case http.MethodGet:
		// Clean up old trips (older than 48 hours)
		s.db.Exec("DELETE FROM shopping_trips WHERE trip_date < date('now', '-2 days')")

		// Get all trips for the household
		var rows *sql.Rows
		var err error
		if hid == 0 {
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
				hid,
			)
		}
		if err != nil {
			dbError(w, err, "Failed to load trips")
			return
		}
		defer rows.Close()

		type TripResponse struct {
			ID        int    `json:"id"`
			UserID    string `json:"user_id"`
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
			writeBadRequest(w, "Invalid request")
			return
		}
		if req.TripDate == "" {
			writeBadRequest(w, "trip_date is required")
			return
		}
		// Validate date format YYYY-MM-DD
		if _, err := time.Parse("2006-01-02", req.TripDate); err != nil {
			writeBadRequest(w, "Invalid date format")
			return
		}

		// Delete any existing future trip for this user (one active trip per user)
		s.db.Exec("DELETE FROM shopping_trips WHERE user_id = ? AND trip_date >= date('now', '-1 day')", session.UserID)

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
		writeBadRequest(w, "place_id required")
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
	var pennySlider, pennyAutotune, pennyDeals, swapCreditsCents, storeOrderEnabled, dailyDigestEnabled int
	err := s.db.QueryRow(
		"SELECT COALESCE(name,''), COALESCE(email,''), phone, COALESCE(address,''), COALESCE(penny_brand_mode,'ask'), COALESCE(penny_slider,35), COALESCE(penny_autotune,1), COALESCE(penny_deals,0), COALESCE(swap_credits_cents,0), COALESCE(onboarding_flags,'{}'), COALESCE(store_order_enabled,1), COALESCE(daily_digest_enabled,0) FROM users WHERE id = ?",
		session.UserID,
	).Scan(&name, &email, &phone, &address, &pennyBrandMode, &pennySlider, &pennyAutotune, &pennyDeals, &swapCreditsCents, &onboardingFlags, &storeOrderEnabled, &dailyDigestEnabled)
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
		`SELECT COALESCE(u.name,''), u.phone FROM users u
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
	uhid := getUserHouseholdID(s.db, session.UserID)
	if uhid != 0 {
		s.db.QueryRow("SELECT COALESCE(name,'') FROM households WHERE id = ?", uhid).Scan(&listName)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":                 name.String,
		"email":                email.String,
		"phone":                phone.String,
		"address":              address.String,
		"penny_phone":          s.config.TwilioPhoneNumber,
		"household_members":    members,
		"google_places_ready":  s.config.GooglePlacesAPIKey != "",
		"penny_brand_mode":    pennyBrandMode.String,
		"penny_slider":        pennySlider,
		"penny_autotune":      pennyAutotune == 1,
		"penny_deals":          pennyDeals == 1,
		"swap_credits_cents":   swapCreditsCents,
		"list_name":            listName,
		"onboarding_flags":     onboardingFlags.String,
		"store_order_enabled":  storeOrderEnabled == 1,
		"daily_digest_enabled": dailyDigestEnabled == 1,
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
		Name               *string `json:"name"`
		Email              *string `json:"email"`
		Address            *string `json:"address"`
		PennyBrandMode     *string `json:"penny_brand_mode"`
		PennySlider        *int    `json:"penny_slider"`
		PennyAutotune      *bool   `json:"penny_autotune"`
		PennyDeals         *bool   `json:"penny_deals"`
		ListName           *string `json:"list_name"`
		OnboardingFlags    *string `json:"onboarding_flags"`
		StoreOrderEnabled  *bool   `json:"store_order_enabled"`
		DailyDigestEnabled *bool   `json:"daily_digest_enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.PennyBrandMode != nil {
		mode := strings.TrimSpace(*req.PennyBrandMode)
		if mode != "auto" && mode != "ask" && mode != "deals" {
			writeBadRequest(w, "Invalid penny_brand_mode")
			return
		}
	}

	// Validate email before making any changes
	var email string
	if req.Email != nil {
		email = strings.TrimSpace(strings.ToLower(*req.Email))
		if email != "" {
			if _, err := mail.ParseAddress(email); err != nil {
				writeBadRequest(w, "Invalid email format")
				return
			}
			var existingID string
			err := s.db.QueryRow("SELECT id FROM users WHERE email = ? AND id != ?", email, session.UserID).Scan(&existingID)
			if err == nil {
				writeConflict(w, "Email already in use")
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
	// phone is deliberately not settable here: it is trusted only from a
	// verified Clerk claim (it routes inbound texts and gates admin), so a
	// "phone" field in the request body is ignored.
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
	if req.StoreOrderEnabled != nil {
		val := 0
		if *req.StoreOrderEnabled {
			val = 1
		}
		addSetClause(&setClauses, &args, "store_order_enabled", val)
	}
	if req.DailyDigestEnabled != nil {
		val := 0
		if *req.DailyDigestEnabled {
			val = 1
			// Set opted_in_at only on first enable (if not already set)
			s.db.Exec("UPDATE users SET daily_digest_opted_in_at = datetime('now') WHERE id = ? AND daily_digest_opted_in_at IS NULL", session.UserID)
		}
		addSetClause(&setClauses, &args, "daily_digest_enabled", val)
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
		lnHID := getUserHouseholdID(s.db, session.UserID)
		if lnHID != 0 {
			if _, err := s.db.Exec("UPDATE households SET name = ? WHERE id = ?", strings.TrimSpace(*req.ListName), lnHID); err != nil {
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
// User IDs are TEXT UUIDs (post-PG-migration), not integers.
func (s *Server) handleAvatarGet(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		writeBadRequest(w, "User ID required")
		return
	}
	userID := parts[len(parts)-1]
	if userID == "" {
		writeBadRequest(w, "Invalid user ID")
		return
	}

	var photoData []byte
	var contentType string
	err := s.db.QueryRow("SELECT photo_data, content_type FROM profile_photos WHERE user_id = ?", userID).Scan(&photoData, &contentType)
	if err != nil {
		writeNotFound(w, "No profile photo")
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
		writeBadRequest(w, "Photo is required")
		return
	}

	// Decode base64
	photoData, err := base64.StdEncoding.DecodeString(req.Photo)
	if err != nil {
		// Try URL-safe base64
		photoData, err = base64.URLEncoding.DecodeString(req.Photo)
		if err != nil {
			writeBadRequest(w, "Invalid base64 encoding")
			return
		}
	}

	if len(photoData) == 0 {
		writeBadRequest(w, "Photo is required")
		return
	}

	if len(photoData) > maxAvatarBytes {
		writeBadRequest(w, "Photo too large (max 2MB)")
		return
	}

	// Validate magic bytes: JPEG (FF D8) or PNG (89 50 4E 47)
	var contentType string
	if len(photoData) >= 2 && photoData[0] == 0xFF && photoData[1] == 0xD8 {
		contentType = "image/jpeg"
	} else if len(photoData) >= 4 && photoData[0] == 0x89 && photoData[1] == 0x50 && photoData[2] == 0x4E && photoData[3] == 0x47 {
		contentType = "image/png"
	} else {
		writeBadRequest(w, "Only JPEG and PNG images are allowed")
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

	log.Printf("user %s uploaded profile photo (%d bytes)", session.UserID, len(photoData))
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
