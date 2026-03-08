package main

import (
	"encoding/json"
	"net/http"
)

// handleDietaryPreferences handles GET and PATCH for /api/preferences.
// GET: returns user's dietary profile (allergens, diets, avoid_brands).
// PATCH: updates dietary profile fields (partial update supported).
func (s *Server) handleDietaryPreferences(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		var allergens, diets, avoidBrands string
		err := s.db.QueryRow(`SELECT allergens, diets, avoid_brands
			FROM user_dietary_profiles WHERE user_id = ?`, session.UserID).
			Scan(&allergens, &diets, &avoidBrands)
		if err != nil {
			// No profile yet — return empty arrays
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"allergens":    []string{},
				"diets":        []string{},
				"avoid_brands": []string{},
			})
			return
		}

		var a, d, ab []string
		json.Unmarshal([]byte(allergens), &a)
		json.Unmarshal([]byte(diets), &d)
		json.Unmarshal([]byte(avoidBrands), &ab)
		if a == nil {
			a = []string{}
		}
		if d == nil {
			d = []string{}
		}
		if ab == nil {
			ab = []string{}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"allergens":    a,
			"diets":        d,
			"avoid_brands": ab,
		})

	case http.MethodPatch:
		var body struct {
			Allergens   *[]string `json:"allergens"`
			Diets       *[]string `json:"diets"`
			AvoidBrands *[]string `json:"avoid_brands"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}

		// Read existing profile (if any) for partial update
		var existAllergens, existDiets, existAvoidBrands string
		err := s.db.QueryRow(`SELECT allergens, diets, avoid_brands
			FROM user_dietary_profiles WHERE user_id = ?`, session.UserID).
			Scan(&existAllergens, &existDiets, &existAvoidBrands)
		if err != nil {
			existAllergens = "[]"
			existDiets = "[]"
			existAvoidBrands = "[]"
		}

		// Merge: only overwrite fields that are present in the request
		if body.Allergens != nil {
			b, _ := json.Marshal(body.Allergens)
			existAllergens = string(b)
		}
		if body.Diets != nil {
			b, _ := json.Marshal(body.Diets)
			existDiets = string(b)
		}
		if body.AvoidBrands != nil {
			b, _ := json.Marshal(body.AvoidBrands)
			existAvoidBrands = string(b)
		}

		_, err = s.db.Exec(`INSERT INTO user_dietary_profiles (user_id, allergens, diets, avoid_brands, updated_at)
			VALUES (?, ?, ?, ?, datetime('now'))
			ON CONFLICT(user_id) DO UPDATE SET
				allergens = excluded.allergens,
				diets = excluded.diets,
				avoid_brands = excluded.avoid_brands,
				updated_at = datetime('now')`,
			session.UserID, existAllergens, existDiets, existAvoidBrands)
		if err != nil {
			dbError(w, err, "Failed to update dietary profile")
			return
		}

		s.recordSignal(session.UserID, "dietary_profile_updated", "preferences", "", map[string]interface{}{
			"allergens":    existAllergens,
			"diets":        existDiets,
			"avoid_brands": existAvoidBrands,
		})

		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// handlePreferenceReset handles DELETE /api/preferences/reset.
// Hard-deletes ALL learned data for the user: dietary profile, learnings,
// signals, and context cache.
func (s *Server) handlePreferenceReset(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodDelete) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Delete from all learning-related tables
	s.db.Exec("DELETE FROM user_dietary_profiles WHERE user_id = ?", session.UserID)
	s.db.Exec("DELETE FROM penny_learnings WHERE scope = 'user' AND scope_id = ?", session.UserID)
	s.db.Exec("DELETE FROM user_signals WHERE user_id = ?", session.UserID)
	s.db.Exec("DELETE FROM penny_context_cache WHERE user_id = ?", session.UserID)

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
