package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// dbExecer abstracts *sql.DB and *sql.Tx so functions can run inside
// or outside a transaction.
type dbExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

// getUserHouseholdID returns the household_id for a given user. Returns 0 if
// the user has no household or doesn't exist. Works with both *sql.DB and *sql.Tx.
func getUserHouseholdID(db dbExecer, userID int) int64 {
	var hid int64
	db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&hid)
	return hid
}


func recordModification(ex dbExecer, itemID, userID int, action, detail, source string) {
	_, err := ex.Exec(
		"INSERT INTO item_modifications (item_id, user_id, action, detail, source) VALUES (?, ?, ?, ?, ?)",
		itemID, userID, action, detail, source,
	)
	if err != nil {
		log.Printf("[mod] error recording modification for item %d: %v", itemID, err)
	}
}

// getGlobalBrandDefault returns the most-chosen brand for a product category
// across all users. Used as a smart default when a new user has no preference.
// Returns ("", false) if no data or no clear winner (top brand needs ≥2 picks
// and ≥40% of total to be a meaningful default).
// If userDiets are provided, tries dietary-matched users first before falling
// back to general popularity.
func getGlobalBrandDefault(db *sql.DB, category string, userDiets ...[]string) (string, bool) {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		return "", false
	}

	// Try dietary-matched defaults first if user has dietary preferences
	if len(userDiets) > 0 && len(userDiets[0]) > 0 {
		if brand, ok := getDietaryBrandDefault(db, category, userDiets[0]); ok {
			return brand, true
		}
		// Fall through to general popularity
	}

	rows, err := db.Query(`
		SELECT brand, COUNT(*) as cnt
		FROM brand_choices
		WHERE product_category = ? AND brand != ''
		GROUP BY brand
		ORDER BY cnt DESC
		LIMIT 2`, category)
	if err != nil {
		return "", false
	}
	defer rows.Close()

	var topBrand string
	var topCount, secondCount int
	i := 0
	for rows.Next() {
		var brand string
		var cnt int
		if err := rows.Scan(&brand, &cnt); err != nil {
			continue
		}
		if i == 0 {
			topBrand = brand
			topCount = cnt
		} else {
			secondCount = cnt
		}
		i++
	}
	if topBrand == "" || topCount < 2 {
		return "", false
	}
	// Require ≥40% share (top must be meaningfully dominant)
	total := topCount + secondCount
	if total > 0 && float64(topCount)/float64(total) < 0.4 {
		return "", false
	}
	return topBrand, true
}

// getUserDiets returns the user's dietary preferences from user_dietary_profiles.
// Returns nil if no profile exists.
func getUserDiets(db *sql.DB, userID int) []string {
	var dietsJSON string
	err := db.QueryRow("SELECT COALESCE(diets, '[]') FROM user_dietary_profiles WHERE user_id = ?", userID).Scan(&dietsJSON)
	if err != nil {
		return nil
	}
	var diets []string
	if err := json.Unmarshal([]byte(dietsJSON), &diets); err != nil || len(diets) == 0 {
		return nil
	}
	return diets
}

// getDietaryBrandDefault returns the most popular brand for a category
// among users who share at least one dietary preference with the requesting user.
// Same thresholds as getGlobalBrandDefault (≥2 picks, ≥40% share).
func getDietaryBrandDefault(db *sql.DB, category string, diets []string) (string, bool) {
	if len(diets) == 0 {
		return "", false
	}
	// Build LIKE conditions for each diet in the user's profile
	// This matches users whose diets JSON array contains any of the same values
	args := make([]interface{}, 0, len(diets)+1)
	args = append(args, category)
	likes := make([]string, len(diets))
	for i, diet := range diets {
		likes[i] = "dp.diets LIKE ?"
		args = append(args, "%"+strings.ToLower(diet)+"%")
	}

	query := fmt.Sprintf(`
		SELECT bc.brand, COUNT(*) as cnt
		FROM brand_choices bc
		JOIN user_dietary_profiles dp ON dp.user_id = bc.user_id
		WHERE bc.product_category = ? AND bc.brand != ''
		AND (%s)
		GROUP BY bc.brand
		ORDER BY cnt DESC
		LIMIT 2`, strings.Join(likes, " OR "))

	rows, err := db.Query(query, args...)
	if err != nil {
		return "", false
	}
	defer rows.Close()

	var topBrand string
	var topCount, secondCount int
	i := 0
	for rows.Next() {
		var brand string
		var cnt int
		if err := rows.Scan(&brand, &cnt); err != nil {
			continue
		}
		if i == 0 {
			topBrand = brand
			topCount = cnt
		} else {
			secondCount = cnt
		}
		i++
	}
	if topBrand == "" || topCount < 2 {
		return "", false
	}
	total := topCount + secondCount
	if total > 0 && float64(topCount)/float64(total) < 0.4 {
		return "", false
	}
	return topBrand, true
}

// recordBrandChoice logs a brand choice and updates the derived preference.
func recordBrandChoice(db *sql.DB, userID int, category, brand, source string) {
	category = strings.ToLower(strings.TrimSpace(category))
	brand = strings.TrimSpace(brand)
	if category == "" || brand == "" {
		return
	}

	// Append-only log
	if _, err := db.Exec("INSERT INTO brand_choices (user_id, product_category, brand, source) VALUES (?, ?, ?, ?)",
		userID, category, brand, source); err != nil {
		log.Printf("[warn] brand choice log insert failed: %v", err)
	}

	// Check existing preference
	var existingBrand string
	var streak int
	err := db.QueryRow("SELECT brand, streak FROM brand_preferences WHERE user_id = ? AND product_category = ?",
		userID, category).Scan(&existingBrand, &streak)

	if err != nil {
		// No existing preference — insert
		if _, err := db.Exec(`INSERT INTO brand_preferences (user_id, product_category, brand, streak, pick_count, last_picked)
			VALUES (?, ?, ?, 1, 1, CURRENT_TIMESTAMP)`, userID, category, brand); err != nil {
			log.Printf("[warn] brand preference insert failed: %v", err)
		}
		return
	}

	if strings.EqualFold(existingBrand, brand) {
		// Same brand — increment streak
		if _, err := db.Exec(`UPDATE brand_preferences SET streak = streak + 1, pick_count = pick_count + 1,
			last_picked = CURRENT_TIMESTAMP WHERE user_id = ? AND product_category = ?`,
			userID, category); err != nil {
			log.Printf("[warn] brand preference update failed: %v", err)
		}
	} else {
		// Different brand — reset streak
		if _, err := db.Exec(`UPDATE brand_preferences SET brand = ?, streak = 1, pick_count = pick_count + 1,
			last_picked = CURRENT_TIMESTAMP WHERE user_id = ? AND product_category = ?`,
			brand, userID, category); err != nil {
			log.Printf("[warn] brand preference update failed: %v", err)
		}
	}
}

// resolveBrandAlternatives applies brand preferences to parsed items with alternatives.
// Returns disambiguations the user still needs to resolve.
// When useGlobalDefaults is true, new users with no preference get the most popular brand auto-assigned.
func resolveBrandAlternatives(db *sql.DB, userID int, pennyBrandMode string, items []ListItem, parsedItems []ParsedItem, useGlobalDefaults ...bool) []BrandDisambiguation {
	globalDefaults := len(useGlobalDefaults) > 0 && useGlobalDefaults[0]
	var disambiguations []BrandDisambiguation

	// Build a lookup from parsed item name to added item (handles skipped items)
	itemByName := make(map[string]ListItem)
	for _, item := range items {
		itemByName[strings.ToLower(item.Name)] = item
	}

	for _, p := range parsedItems {
		if len(p.BrandAlternatives) == 0 {
			continue
		}

		item, ok := itemByName[strings.ToLower(p.Name)]
		if !ok {
			continue
		}
		// Use product name as category key (more specific than department)
		category := strings.ToLower(strings.TrimSpace(p.Name))
		if category == "" {
			category = strings.ToLower(strings.TrimSpace(p.Department))
		}

		// Query preference
		var prefBrand string
		var streak int
		var lastPicked string
		err := db.QueryRow(
			"SELECT brand, streak, last_picked FROM brand_preferences WHERE user_id = ? AND product_category = ?",
			userID, category,
		).Scan(&prefBrand, &streak, &lastPicked)

		hasPref := err == nil
		isStale := false
		if hasPref {
			if lp, err := time.Parse("2006-01-02 15:04:05", lastPicked); err == nil {
				isStale = time.Since(lp) > 90*24*time.Hour && streak < 5
			}
		}
		if isStale {
			hasPref = false
		}

		// Strong preference (streak >= 3): auto-apply in all modes
		if hasPref && streak >= 3 {
			db.Exec("UPDATE list_items SET brand = ? WHERE id = ?", prefBrand, item.ID)
			continue
		}

		// Preference exists + auto mode: use last pick
		if hasPref && pennyBrandMode == "auto" {
			db.Exec("UPDATE list_items SET brand = ? WHERE id = ?", prefBrand, item.ID)
			continue
		}

		// Check global brand popularity as fallback for new users (A/B gated)
		if !hasPref && globalDefaults {
			userDiets := getUserDiets(db, userID)
			if globalBrand, ok := getGlobalBrandDefault(db, category, userDiets); ok {
				// Auto-assign the popular brand and send as auto_assigned so
				// the frontend can show 1-tap correction instead of full prompt
				db.Exec("UPDATE list_items SET brand = ? WHERE id = ?", globalBrand, item.ID)
				alts := make([]string, len(p.BrandAlternatives))
				copy(alts, p.BrandAlternatives)
				disambiguations = append(disambiguations, BrandDisambiguation{
					ItemID:          item.ID,
					ProductCategory: category,
					CurrentBrand:    globalBrand,
					Alternatives:    alts,
					AutoAssigned:    true,
				})
				continue
			}
		}

		// Build disambiguation — reorder with preferred brand first if exists
		alts := make([]string, len(p.BrandAlternatives))
		copy(alts, p.BrandAlternatives)
		if hasPref {
			// Move preferred brand to front
			for j, a := range alts {
				if strings.EqualFold(a, prefBrand) {
					alts[0], alts[j] = alts[j], alts[0]
					break
				}
			}
		}

		disambiguations = append(disambiguations, BrandDisambiguation{
			ItemID:          item.ID,
			ProductCategory: category,
			CurrentBrand:    p.Brand,
			Alternatives:    alts,
		})
	}

	return disambiguations
}

func upsertUser(db *sql.DB, phone string) (User, error) {
	tx, err := db.Begin()
	if err != nil {
		return User{}, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"INSERT OR IGNORE INTO users (phone_number) VALUES (?)",
		phone,
	)
	if err != nil {
		return User{}, fmt.Errorf("upserting user: %w", err)
	}

	var user User
	var householdID sql.NullInt64
	var name, email sql.NullString
	err = tx.QueryRow(
		"SELECT id, phone_number, COALESCE(name,''), COALESCE(email,''), household_id, created_at FROM users WHERE phone_number = ?",
		phone,
	).Scan(&user.ID, &user.PhoneNumber, &name, &email, &householdID, &user.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("fetching user: %w", err)
	}
	user.Name = name.String
	user.Email = email.String
	if householdID.Valid {
		user.HouseholdID = int(householdID.Int64)
	}

	// Track login activity and increment login count
	if _, err := tx.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP, login_count = login_count + 1 WHERE id = ?", user.ID); err != nil {
		log.Printf("[warn] failed to update login count for user %d: %v", user.ID, err)
	}

	// Auto-create household if user doesn't have one
	if !householdID.Valid {
		hID, err := createSoloHouseholdTx(tx, user.ID)
		if err != nil {
			return User{}, err
		}
		user.HouseholdID = hID
	}

	// Ensure auth_methods row exists for phone login
	tx.Exec("INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id) VALUES (?, 'phone', ?)", user.ID, phone)

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("committing upsert: %w", err)
	}

	return user, nil
}

// createSoloHouseholdTx creates a new household with the given user as owner
// using the provided transaction or db handle.
func createSoloHouseholdTx(ex dbExecer, userID int) (int, error) {
	result, err := ex.Exec("INSERT INTO households DEFAULT VALUES")
	if err != nil {
		return 0, fmt.Errorf("creating household: %w", err)
	}
	hID, _ := result.LastInsertId()

	_, err = ex.Exec("UPDATE users SET household_id = ? WHERE id = ?", hID, userID)
	if err != nil {
		return 0, fmt.Errorf("setting household_id: %w", err)
	}

	_, err = ex.Exec(
		"INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'owner')",
		hID, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting household member: %w", err)
	}

	return int(hID), nil
}

// createSoloHousehold wraps createSoloHouseholdTx in a transaction.
func createSoloHousehold(db *sql.DB, userID int) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	hID, err := createSoloHouseholdTx(tx, userID)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing household creation: %w", err)
	}
	return hID, nil
}

// logAccountMerge records an account merge event for audit purposes.
func logAccountMerge(ex dbExecer, survivingUser, mergedUser int, provider, providerID string, actions map[string]string) {
	data, _ := json.Marshal(actions)
	var mergedPtr interface{}
	if mergedUser > 0 {
		mergedPtr = mergedUser
	}
	ex.Exec(
		"INSERT INTO account_merges (surviving_user, merged_user, provider_linked, provider_id, data_actions) VALUES (?, ?, ?, ?, ?)",
		survivingUser, mergedPtr, provider, providerID, string(data),
	)
}

// mergeHouseholds cleans up household membership when two users are merged.
// Returns a description of what was done for the audit log.
func mergeHouseholds(tx dbExecer, survivingID, orphanedID int) string {
	survivingHH := getUserHouseholdID(tx, survivingID)
	orphanedHH := getUserHouseholdID(tx, orphanedID)

	if orphanedHH == 0 {
		return "orphaned user had no household"
	}

	// Remove orphaned user from their household
	tx.Exec("DELETE FROM household_members WHERE user_id = ?", orphanedID)
	tx.Exec("UPDATE users SET household_id = NULL WHERE id = ?", orphanedID)

	if survivingHH != 0 && orphanedHH == survivingHH {
		return fmt.Sprintf("removed duplicate membership in household %d", orphanedHH)
	}
	return fmt.Sprintf("removed orphaned membership in household %d", orphanedHH)
}

// mergeUserData transfers data from an orphaned user to the surviving user.
// Returns a map of action descriptions for the audit log.
func mergeUserData(tx dbExecer, survivingID, orphanedID int) map[string]string {
	actions := map[string]string{}

	// 1. List items: transfer non-duplicate items, merge quantities for duplicates
	rows, err := tx.Query(
		"SELECT id, name, quantity FROM list_items WHERE user_id = ?", orphanedID,
	)
	if err == nil {
		transferred, merged := 0, 0
		type orphanItem struct {
			id       int
			name     string
			quantity int
		}
		var orphanItems []orphanItem
		for rows.Next() {
			var item orphanItem
			if err := rows.Scan(&item.id, &item.name, &item.quantity); err == nil {
				orphanItems = append(orphanItems, item)
			}
		}
		rows.Close()

		for _, item := range orphanItems {
			var existingID, existingQty int
			err := tx.QueryRow(
				"SELECT id, quantity FROM list_items WHERE user_id = ? AND LOWER(name) = LOWER(?)",
				survivingID, item.name,
			).Scan(&existingID, &existingQty)
			if err == nil {
				// Duplicate — add quantities
				tx.Exec("UPDATE list_items SET quantity = ? WHERE id = ?", existingQty+item.quantity, existingID)
				tx.Exec("DELETE FROM list_items WHERE id = ?", item.id)
				merged++
			} else {
				// No duplicate — reassign
				tx.Exec("UPDATE list_items SET user_id = ? WHERE id = ?", survivingID, item.id)
				transferred++
			}
		}
		if transferred+merged > 0 {
			actions["list_items"] = fmt.Sprintf("%d transferred, %d merged quantities", transferred, merged)
		}
	}

	// 2. Transfer item_modifications
	if result, err := tx.Exec("UPDATE item_modifications SET user_id = ? WHERE user_id = ?", survivingID, orphanedID); err == nil {
		if n, _ := result.RowsAffected(); n > 0 {
			actions["item_modifications"] = fmt.Sprintf("%d transferred", n)
		}
	}

	// 3. Profile photo: keep most recent
	var survPhotoTime, orphPhotoTime sql.NullString
	tx.QueryRow("SELECT updated_at FROM profile_photos WHERE user_id = ?", survivingID).Scan(&survPhotoTime)
	tx.QueryRow("SELECT updated_at FROM profile_photos WHERE user_id = ?", orphanedID).Scan(&orphPhotoTime)

	if orphPhotoTime.Valid {
		if !survPhotoTime.Valid {
			// Only orphaned has a photo — move it
			tx.Exec("UPDATE profile_photos SET user_id = ? WHERE user_id = ?", survivingID, orphanedID)
			actions["profile_photo"] = "transferred from merged user (surviving had none)"
		} else if orphPhotoTime.String > survPhotoTime.String {
			// Orphaned's photo is newer — replace
			tx.Exec("DELETE FROM profile_photos WHERE user_id = ?", survivingID)
			tx.Exec("UPDATE profile_photos SET user_id = ? WHERE user_id = ?", survivingID, orphanedID)
			actions["profile_photo"] = "kept merged user's photo (more recent)"
		} else {
			// Surviving's photo is newer or equal — delete orphaned's
			tx.Exec("DELETE FROM profile_photos WHERE user_id = ?", orphanedID)
			actions["profile_photo"] = "kept surviving user's photo (more recent)"
		}
	}

	// 4. Display name: prefer manually set
	var survName, orphName sql.NullString
	var survNameSrc, orphNameSrc string
	tx.QueryRow("SELECT name, COALESCE(name_source,'auto') FROM users WHERE id = ?", survivingID).Scan(&survName, &survNameSrc)
	tx.QueryRow("SELECT name, COALESCE(name_source,'auto') FROM users WHERE id = ?", orphanedID).Scan(&orphName, &orphNameSrc)

	if orphName.Valid && orphName.String != "" && orphNameSrc == "manual" && survNameSrc != "manual" {
		tx.Exec("UPDATE users SET name = ?, name_source = 'manual' WHERE id = ?", orphName.String, survivingID)
		actions["display_name"] = fmt.Sprintf("used merged user's name '%s' (manually set)", orphName.String)
	} else if survName.Valid && survName.String != "" {
		actions["display_name"] = fmt.Sprintf("kept surviving user's name '%s'", survName.String)
	}

	// 5. Saved lists: reassign
	if result, err := tx.Exec("UPDATE saved_lists SET user_id = ? WHERE user_id = ?", survivingID, orphanedID); err == nil {
		if n, _ := result.RowsAffected(); n > 0 {
			actions["saved_lists"] = fmt.Sprintf("%d transferred", n)
		}
	}

	// 6. Brand preferences: transfer non-duplicates
	if result, err := tx.Exec(`
		UPDATE brand_preferences SET user_id = ? WHERE user_id = ? AND product_category NOT IN (
			SELECT product_category FROM brand_preferences WHERE user_id = ?
		)`, survivingID, orphanedID, survivingID); err == nil {
		if n, _ := result.RowsAffected(); n > 0 {
			actions["brand_preferences"] = fmt.Sprintf("%d transferred", n)
		}
	}
	tx.Exec("DELETE FROM brand_preferences WHERE user_id = ?", orphanedID)

	// 7. Swap events/rebates: reassign
	if result, err := tx.Exec("UPDATE swap_events SET user_id = ? WHERE user_id = ?", survivingID, orphanedID); err == nil {
		if n, _ := result.RowsAffected(); n > 0 {
			actions["swap_events"] = fmt.Sprintf("%d transferred", n)
		}
	}
	if result, err := tx.Exec("UPDATE swap_rebates SET user_id = ? WHERE user_id = ?", survivingID, orphanedID); err == nil {
		if n, _ := result.RowsAffected(); n > 0 {
			actions["swap_rebates"] = fmt.Sprintf("%d transferred", n)
		}
	}

	// 8. Auth methods: reassign remaining
	tx.Exec("UPDATE OR IGNORE auth_methods SET user_id = ? WHERE user_id = ?", survivingID, orphanedID)
	tx.Exec("DELETE FROM auth_methods WHERE user_id = ?", orphanedID) // clean up any that couldn't be moved due to unique constraint

	// 9. Sessions: delete orphaned
	tx.Exec("DELETE FROM sessions WHERE user_id = ?", orphanedID)

	// 10. Household cleanup
	hhAction := mergeHouseholds(tx, survivingID, orphanedID)
	actions["household"] = hhAction

	// 11. Soft-delete the orphaned user
	tx.Exec("UPDATE users SET email = '', phone_number = NULL, name = ? WHERE id = ?",
		fmt.Sprintf("[merged into user %d]", survivingID), orphanedID)
	actions["orphaned_user"] = fmt.Sprintf("user %d soft-deleted", orphanedID)

	return actions
}

// resolveOrCreateUser finds or creates a user for the given auth provider.
// 1. Check auth_methods for existing (provider, provider_id) → return that user
// 2. If email provided, check users for matching email → link auth method (auto-merge)
// 3. Otherwise: create new user + auth_methods row + solo household
func resolveOrCreateUser(db *sql.DB, provider, providerID, email, name string) (User, error) {
	tx, err := db.Begin()
	if err != nil {
		return User{}, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Check if this auth method already exists
	var userID int
	err = tx.QueryRow(
		"SELECT user_id FROM auth_methods WHERE provider = ? AND provider_id = ?",
		provider, providerID,
	).Scan(&userID)
	if err == nil {
		// Auth method exists — fetch and return the user
		var user User
		var householdID sql.NullInt64
		var uName, uEmail, uPhone sql.NullString
		err = tx.QueryRow(
			"SELECT id, phone_number, COALESCE(name,''), COALESCE(email,''), household_id, created_at FROM users WHERE id = ?",
			userID,
		).Scan(&user.ID, &uPhone, &uName, &uEmail, &householdID, &user.CreatedAt)
		if err != nil {
			return User{}, fmt.Errorf("fetching user: %w", err)
		}
		user.PhoneNumber = uPhone.String
		user.Name = uName.String
		user.Email = uEmail.String
		if householdID.Valid {
			user.HouseholdID = int(householdID.Int64)
		}
		// Track login
		tx.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP, login_count = login_count + 1 WHERE id = ?", user.ID)
		if err := tx.Commit(); err != nil {
			return User{}, fmt.Errorf("committing: %w", err)
		}
		return user, nil
	}

	// 2. If email provided, try to merge with existing user by email
	if email != "" {
		var existingID int
		err = tx.QueryRow(
			"SELECT id FROM users WHERE LOWER(email) = LOWER(?) AND email != ''",
			email,
		).Scan(&existingID)
		if err == nil {
			// Found existing user with same email — link this auth method
			result, err := tx.Exec(
				"INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id) VALUES (?, ?, ?)",
				existingID, provider, providerID,
			)
			if err != nil {
				return User{}, fmt.Errorf("linking auth method: %w", err)
			}
			// Check if a new auth method was actually linked (not a duplicate)
			rowsInserted, _ := result.RowsAffected()
			merged := rowsInserted > 0

			// Update name if empty
			if name != "" {
				tx.Exec("UPDATE users SET name = ? WHERE id = ? AND (name IS NULL OR name = '')", name, existingID)
			}

			// Check for orphaned users with the same email (true user-level merge)
			fullMerged := false
			orphanRows, _ := tx.Query(
				"SELECT id FROM users WHERE LOWER(email) = LOWER(?) AND id != ? AND email != '' AND name NOT LIKE '[merged into user%'",
				email, existingID,
			)
			if orphanRows != nil {
				var orphanIDs []int
				for orphanRows.Next() {
					var oid int
					if err := orphanRows.Scan(&oid); err == nil {
						orphanIDs = append(orphanIDs, oid)
					}
				}
				orphanRows.Close()
				for _, orphanID := range orphanIDs {
					actions := mergeUserData(tx, existingID, orphanID)
					logAccountMerge(tx, existingID, orphanID, provider, providerID, actions)
					log.Printf("[auth] full user merge: user %d absorbed user %d (%s)", existingID, orphanID, email)
					merged = true
					fullMerged = true
				}
			}

			// Log the auth-method-only merge if no full user merge happened
			if merged && !fullMerged {
				logAccountMerge(tx, existingID, 0, provider, providerID, map[string]string{
					"auth_method": fmt.Sprintf("linked %s:%s", provider, providerID),
				})
			}

			var user User
			var householdID sql.NullInt64
			var uName, uEmail, uPhone sql.NullString
			err = tx.QueryRow(
				"SELECT id, phone_number, COALESCE(name,''), COALESCE(email,''), household_id, created_at FROM users WHERE id = ?",
				existingID,
			).Scan(&user.ID, &uPhone, &uName, &uEmail, &householdID, &user.CreatedAt)
			if err != nil {
				return User{}, fmt.Errorf("fetching merged user: %w", err)
			}
			user.PhoneNumber = uPhone.String
			user.Name = uName.String
			user.Email = uEmail.String
			user.Merged = merged
			if householdID.Valid {
				user.HouseholdID = int(householdID.Int64)
			}
			tx.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP, login_count = login_count + 1 WHERE id = ?", user.ID)
			if err := tx.Commit(); err != nil {
				return User{}, fmt.Errorf("committing: %w", err)
			}
			if merged {
				log.Printf("[auth] account merged: user %d linked %s auth to existing account (%s)", user.ID, provider, email)
			}
			return user, nil
		}
	}

	// 3. Create new user
	result, err := tx.Exec(
		"INSERT INTO users (phone_number, name, email) VALUES (NULL, ?, ?)",
		name, email,
	)
	if err != nil {
		return User{}, fmt.Errorf("creating user: %w", err)
	}
	newID, _ := result.LastInsertId()

	// Create auth method
	_, err = tx.Exec(
		"INSERT INTO auth_methods (user_id, provider, provider_id) VALUES (?, ?, ?)",
		newID, provider, providerID,
	)
	if err != nil {
		return User{}, fmt.Errorf("creating auth method: %w", err)
	}

	// Auto-create household
	hID, err := createSoloHouseholdTx(tx, int(newID))
	if err != nil {
		return User{}, err
	}

	tx.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP, login_count = 1 WHERE id = ?", newID)

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("committing: %w", err)
	}

	return User{
		ID:          int(newID),
		Name:        name,
		Email:       email,
		HouseholdID: hID,
	}, nil
}

func getHouseholdUserIDs(db *sql.DB, userID int) ([]int, error) {
	rows, err := db.Query(
		"SELECT user_id FROM household_members WHERE household_id = (SELECT household_id FROM users WHERE id = ? AND household_id IS NOT NULL)",
		userID,
	)
	if err != nil {
		log.Printf("[warn] household lookup failed for user %d, falling back to single user: %v", userID, err)
		return []int{userID}, nil
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return []int{userID}, nil
	}
	return ids, nil
}

// householdItemsWhereClause returns a SQL fragment "user_id IN (?,?,...)" and
// the corresponding args, for queries that should span the whole household.
// Pass optional pre-computed household IDs to avoid re-querying.
func householdItemsWhereClause(db *sql.DB, userID int, cachedIDs ...[]int) (string, []interface{}) {
	var ids []int
	if len(cachedIDs) > 0 && cachedIDs[0] != nil {
		ids = cachedIDs[0]
	} else {
		ids, _ = getHouseholdUserIDs(db, userID) // error logged inside, falls back to [userID]
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	return "user_id IN (" + strings.Join(placeholders, ",") + ")", args
}

// insertSavedListItems inserts items into saved_list_items within a transaction.
func insertSavedListItems(tx *sql.Tx, savedListID int64, items []itemData) error {
	for _, d := range items {
		if _, err := tx.Exec(
			"INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size, notes) VALUES (?, ?, ?, ?, ?, ?, ?)",
			savedListID, d.Name, d.Brand, d.Department, d.Quantity, d.Size, d.Notes,
		); err != nil {
			return err
		}
	}
	return nil
}
func scanListItem(scanner interface{ Scan(dest ...interface{}) error }) (ListItem, error) {
	var item ListItem
	var checked, urgent int
	var addedByName, addedByPhone sql.NullString
	var claimedByID sql.NullInt64
	var claimedByName sql.NullString
	err := scanner.Scan(&item.ID, &item.Name, &item.Brand, &item.Department, &item.Quantity, &item.Size, &item.Notes, &item.Source, &checked, &urgent, &item.CreatedAt, &addedByName, &addedByPhone, &claimedByID, &claimedByName, &item.SwapSavingsCents, &item.Emoji)
	if err != nil {
		return ListItem{}, err
	}
	item.Checked = checked == 1
	item.Urgent = urgent == 1
	item.AddedByName = addedByName.String
	item.AddedByPhone = addedByPhone.String
	if claimedByID.Valid {
		id := int(claimedByID.Int64)
		item.ClaimedByID = &id
		item.ClaimedByName = claimedByName.String
	}
	return item, nil
}

// maskPhone masks a phone number: "3101234567" → "310***567"
func maskPhone(phone string) string {
	if len(phone) == 10 {
		return phone[:3] + "***" + phone[7:]
	}
	// E.164 format like +15551234567
	if len(phone) >= 12 && phone[0] == '+' {
		return phone[:5] + "***" + phone[8:]
	}
	if len(phone) <= 4 {
		return "***"
	}
	return phone[:2] + "***" + phone[len(phone)-2:]
}

// getItemByID fetches a single list item belonging to a household.
func getItemByID(db *sql.DB, id, userID int) (ListItem, error) {
	whereClause, whereArgs := householdItemsWhereClause(db, userID)
	args := append([]interface{}{id}, whereArgs...)
	row := db.QueryRow(
		"SELECT "+listItemColumns+" FROM "+listItemJoins+" WHERE li.id = ? AND li."+whereClause,
		args...,
	)
	return scanListItem(row)
}

// upsertListItem adds a new item or merges into an existing unchecked item
// with the same name (case-insensitive) across the household. Returns the
// resulting ListItem and whether it was newly created.
// The SELECT+UPDATE/INSERT is wrapped in a transaction to prevent duplicate
// items from concurrent requests.
func upsertListItem(db *sql.DB, userID int, name, brand, department, size string, quantity int, source, notes, emoji string) (ListItem, bool, error) {
	if quantity < 1 {
		quantity = 1
	}
	if department == "" {
		department = "Other"
	}
	if source == "" {
		source = "app"
	}

	// Get household user IDs before the transaction (uses its own query)
	whereClause, whereArgs := householdItemsWhereClause(db, userID)

	tx, err := db.Begin()
	if err != nil {
		return ListItem{}, false, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Look for an existing item with the same name across household (checked or unchecked)
	queryArgs := append(append([]interface{}{}, whereArgs...), name)
	var existingID int
	var existingQty int
	var existingBrand, existingSize, existingNotes string
	var existingChecked int
	err = tx.QueryRow(
		"SELECT id, quantity, brand, size, checked, COALESCE(notes, '') FROM list_items WHERE "+whereClause+" AND LOWER(name) = LOWER(?)",
		queryArgs...,
	).Scan(&existingID, &existingQty, &existingBrand, &existingSize, &existingChecked, &existingNotes)

	var itemID int
	var isNew bool

	if err == nil {
		if existingChecked == 1 {
			// Re-adding a checked item — uncheck it and set new quantity
			newBrand := brand
			if newBrand == "" {
				newBrand = existingBrand
			}
			newSize := size
			if newSize == "" {
				newSize = existingSize
			}
			newEmoji := emoji
			_, err := tx.Exec(
				"UPDATE list_items SET checked = 0, quantity = ?, brand = ?, size = ?, notes = ?, emoji = CASE WHEN ? != '' THEN ? ELSE emoji END WHERE id = ?",
				quantity, newBrand, newSize, notes, newEmoji, newEmoji, existingID,
			)
			if err != nil {
				return ListItem{}, false, fmt.Errorf("updating existing item: %w", err)
			}
			recordModification(tx, existingID, userID, "re-added", "", source)
			itemID = existingID
			isNew = true
		} else {
			// Existing unchecked item — merge quantity
			newQty := existingQty + quantity
			newBrand := existingBrand
			if newBrand == "" && brand != "" {
				newBrand = brand
			}
			newSize := existingSize
			if newSize == "" && size != "" {
				newSize = size
			}
			mergeNotes := notes
			if mergeNotes == "" {
				mergeNotes = existingNotes
			}
			_, err := tx.Exec(
				"UPDATE list_items SET quantity = ?, brand = ?, size = ?, notes = ?, emoji = CASE WHEN ? != '' THEN ? ELSE emoji END WHERE id = ?",
				newQty, newBrand, newSize, mergeNotes, emoji, emoji, existingID,
			)
			if err != nil {
				return ListItem{}, false, fmt.Errorf("updating existing item: %w", err)
			}
			detail := fmt.Sprintf("quantity %d → %d", existingQty, newQty)
			recordModification(tx, existingID, userID, "quantity_changed", detail, source)
			itemID = existingID
			// For SMS/email sources, report as "added" (not skipped) since the user
			// can't interactively dedupe via one-way communication.
			isNew = source == "sms" || source == "email"
		}
	} else {
		// No match — insert new (attributed to the requesting user)
		// Guarantee every new item has an emoji
		if emoji == "" {
			emoji = inferItemEmoji(name, department)
		}
		result, err := tx.Exec(
			"INSERT INTO list_items (user_id, name, brand, department, quantity, size, source, notes, emoji) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			userID, name, brand, department, quantity, size, source, notes, emoji,
		)
		if err != nil {
			return ListItem{}, false, fmt.Errorf("inserting item: %w", err)
		}
		id, _ := result.LastInsertId()
		recordModification(tx, int(id), userID, "added", "", source)
		itemID = int(id)
		isNew = true
	}

	if err := tx.Commit(); err != nil {
		return ListItem{}, false, fmt.Errorf("committing upsert: %w", err)
	}

	item, err := getItemByID(db, itemID, userID)
	return item, isNew, err
}

// stripBrandFromName removes the brand name from the item name if it appears
// as a prefix (case-insensitive). e.g. "Budweiser Beer" with brand "Budweiser" → "Beer".
func stripBrandFromName(name, brand string) string {
	if brand == "" {
		return name
	}
	lower := strings.ToLower(name)
	brandLower := strings.ToLower(brand)
	if strings.HasPrefix(lower, brandLower+" ") {
		stripped := strings.TrimSpace(name[len(brand)+1:])
		if stripped != "" {
			return stripped
		}
	}
	if strings.HasSuffix(lower, " "+brandLower) {
		stripped := strings.TrimSpace(name[:len(name)-len(brand)-1])
		if stripped != "" {
			return stripped
		}
	}
	return name
}

// addParsedItems loops over parsed items, skips empties, and upserts each one.
// Returns the list of added items and skipped items (already on the list).
func addParsedItems(db *sql.DB, userID int, parsed []ParsedItem, source string) (added []ListItem, skipped []SkippedItem) {
	for _, p := range parsed {
		if strings.TrimSpace(p.Name) == "" {
			continue
		}
		p.Name = stripBrandFromName(p.Name, p.Brand)
		// Guarantee emoji even when Claude returns empty
		if p.Emoji == "" {
			p.Emoji = inferItemEmoji(p.Name, p.Department)
		}
		item, isNew, err := upsertListItem(db, userID, p.Name, p.Brand, p.Department, p.Size, p.Quantity, source, p.Notes, p.Emoji)
		if err != nil {
			log.Printf("[%s] error upserting item %q: %v", source, p.Name, err)
			continue
		}
		if isNew {
			added = append(added, item)
		} else {
			skipped = append(skipped, SkippedItem{Parsed: p, ExistingID: item.ID})
		}
	}
	return added, skipped
}
func collectListItems(db *sql.DB, userID int) ([]itemData, error) {
	whereClause, whereArgs := householdItemsWhereClause(db, userID)
	rows, err := db.Query(
		"SELECT name, brand, department, quantity, size, notes FROM list_items WHERE "+whereClause,
		whereArgs...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []itemData
	for rows.Next() {
		var d itemData
		if err := rows.Scan(&d.Name, &d.Brand, &d.Department, &d.Quantity, &d.Size, &d.Notes); err != nil {
			continue
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return items, fmt.Errorf("row iteration: %w", err)
	}
	return items, nil
}

// containsWord checks if `lower` contains `kw` using word-boundary matching.
// For short keywords (len <= 4), requires start boundary (start-of-string or space)
// and end boundary (end-of-string, space, or trailing "s"/"es" for plurals).
// For longer keywords (len > 4), uses simple substring match (they're specific enough).
func containsWord(lower, kw string) bool {
	if len(kw) <= 4 {
		idx := 0
		for {
			pos := strings.Index(lower[idx:], kw)
			if pos < 0 {
				return false
			}
			absPos := idx + pos
			startOK := absPos == 0 || lower[absPos-1] == ' '
			endPos := absPos + len(kw)
			endOK := endPos == len(lower) || lower[endPos] == ' ' || lower[endPos] == 's'
			// Also allow "es" plural (e.g., "tomatoes" for "tomato" — but tomato is >4 chars)
			if startOK && endOK {
				return true
			}
			idx = absPos + 1
			if idx >= len(lower) {
				return false
			}
		}
	}
	return strings.Contains(lower, kw)
}

// inferItemEmoji returns a specific emoji for a grocery item name based on keyword matching.
// Falls back to department emoji, then 🛒.
// Short keywords (<=3 chars) use word-boundary matching to prevent false positives.
func inferItemEmoji(name, department string) string {
	lower := strings.ToLower(name)
	// Item-specific keywords → emoji (most specific first)
	keywords := []struct {
		kw    string
		emoji string
	}{
		// Produce — fruits
		{"apple", "🍎"}, {"banana", "🍌"}, {"orange", "🍊"}, {"lemon", "🍋"}, {"lime", "🍋"},
		{"grape", "🍇"}, {"strawberr", "🍓"}, {"blueberr", "🫐"}, {"raspberry", "🫐"}, {"blackberr", "🫐"},
		{"cherry", "🍒"}, {"peach", "🍑"}, {"pear", "🍐"}, {"watermelon", "🍉"}, {"melon", "🍈"},
		{"pineapple", "🍍"}, {"mango", "🥭"}, {"kiwi", "🥝"}, {"coconut", "🥥"}, {"avocado", "🥑"},
		{"tomato", "🍅"}, {"plum", "🍑"}, {"fig", "🍇"}, {"date", "🍇"}, {"pomegranate", "🍎"},
		{"cranberr", "🫐"}, {"grapefruit", "🍊"}, {"tangerine", "🍊"}, {"clementine", "🍊"},
		{"nectarine", "🍑"}, {"apricot", "🍑"}, {"papaya", "🥭"}, {"guava", "🥭"},
		{"dragon fruit", "🐉"}, {"passion fruit", "🍑"}, {"starfruit", "⭐"}, {"persimmon", "🍊"},
		{"lychee", "🍑"}, {"jackfruit", "🍈"}, {"plantain", "🍌"},
		// Produce — vegetables
		{"broccoli", "🥦"}, {"cauliflower", "🥦"}, {"carrot", "🥕"}, {"corn", "🌽"},
		{"pepper", "🌶️"}, {"jalapeño", "🌶️"}, {"jalapeno", "🌶️"}, {"habanero", "🌶️"},
		{"cucumber", "🥒"}, {"zucchini", "🥒"}, {"squash", "🥒"},
		{"lettuce", "🥬"}, {"spinach", "🥬"}, {"kale", "🥬"}, {"arugula", "🥬"}, {"chard", "🥬"},
		{"cabbage", "🥬"}, {"collard", "🥬"}, {"romaine", "🥬"}, {"cilantro", "🌿"}, {"parsley", "🌿"},
		{"basil", "🌿"}, {"mint", "🌿"}, {"dill", "🌿"}, {"rosemary", "🌿"}, {"thyme", "🌿"},
		{"oregano", "🌿"}, {"sage", "🌿"}, {"herb", "🌿"},
		{"sweet potato", "🍠"}, {"potato", "🥔"}, {"yam", "🍠"},
		{"onion", "🧅"}, {"shallot", "🧅"}, {"scallion", "🧅"}, {"leek", "🧅"}, {"chive", "🧅"},
		{"garlic", "🧄"}, {"ginger", "🫚"},
		{"mushroom", "🍄"}, {"eggplant", "🍆"}, {"celery", "🥬"},
		{"asparagus", "🥬"}, {"artichoke", "🥬"}, {"beet", "🥬"}, {"turnip", "🥬"}, {"radish", "🥬"},
		{"bok choy", "🥬"}, {"napa cabbage", "🥬"}, {"daikon", "🥕"},
		{"jicama", "🥔"}, {"taro", "🥔"}, {"yuca", "🥔"},
		{"snap pea", "🟢"}, {"green bean", "🟢"}, {"edamame", "🫘"}, {"pea", "🟢"},
		{"salad", "🥗"},
		// Dairy & Eggs
		{"milky way", "🍫"},
		{"half and half", "🥛"}, {"half & half", "🥛"}, {"almond milk", "🥛"}, {"oat milk", "🥛"},
		{"soy milk", "🥛"}, {"coconut milk", "🥥"}, {"milk", "🥛"}, {"cream", "🥛"},
		{"cream cheese", "🧀"}, {"cottage cheese", "🧀"}, {"goat cheese", "🧀"},
		{"cheese", "🧀"}, {"cheddar", "🧀"}, {"mozzarella", "🧀"}, {"parmesan", "🧀"},
		{"gouda", "🧀"}, {"brie", "🧀"}, {"feta", "🧀"}, {"gruyere", "🧀"}, {"provolone", "🧀"},
		{"colby", "🧀"}, {"swiss", "🧀"}, {"jack", "🧀"}, {"ricotta", "🧀"},
		{"butter", "🧈"}, {"margarine", "🧈"},
		{"egg", "🥚"},
		{"yogurt", "🫙"}, {"kefir", "🫙"},
		{"ice cream", "🍦"}, {"gelato", "🍦"}, {"sorbet", "🍦"}, {"popsicle", "🍦"},
		// Meat & Seafood
		{"rotisserie", "🍗"}, {"chicken", "🍗"}, {"thigh", "🍗"}, {"drumstick", "🍗"}, {"wing", "🍗"},
		{"beef", "🥩"}, {"steak", "🥩"}, {"ribeye", "🥩"}, {"sirloin", "🥩"}, {"filet", "🥩"},
		{"ground", "🥩"}, {"roast", "🥩"}, {"brisket", "🥩"},
		{"pork", "🥩"}, {"chop", "🥩"}, {"tenderloin", "🥩"}, {"ribs", "🥩"},
		{"bacon", "🥓"}, {"ham", "🥓"}, {"prosciutto", "🥓"}, {"pancetta", "🥓"},
		{"sausage", "🌭"}, {"hot dog", "🌭"}, {"bratwurst", "🌭"}, {"kielbasa", "🌭"},
		{"turkey", "🦃"}, {"lamb", "🥩"}, {"veal", "🥩"}, {"venison", "🥩"}, {"bison", "🥩"},
		{"deli meat", "🥩"}, {"lunch meat", "🥩"}, {"jerky", "🥩"},
		{"fish sauce", "🐟"}, {"fish food", "🐠"}, {"fish", "🐟"}, {"salmon", "🐟"}, {"tuna", "🐟"},
		{"cod", "🐟"}, {"tilapia", "🐟"}, {"trout", "🐟"}, {"halibut", "🐟"}, {"mahi", "🐟"},
		{"swordfish", "🐟"}, {"catfish", "🐟"}, {"anchovy", "🐟"},
		{"shrimp", "🦐"}, {"prawn", "🦐"}, {"crab", "🦀"}, {"lobster", "🦞"},
		{"scallop", "🦪"}, {"oyster sauce", "🦪"}, {"oyster", "🦪"}, {"clam", "🦪"}, {"mussel", "🦪"},
		// Deli & Prepared
		{"deli", "🥪"}, {"sandwich", "🥪"}, {"wrap", "🌯"}, {"burrito", "🌯"},
		{"spring roll", "🥟"}, {"dumpling", "🥟"}, {"wonton", "🥟"}, {"samosa", "🥟"},
		{"falafel", "🧆"}, {"quesadilla", "🌮"}, {"enchilada", "🌮"}, {"tamale", "🫔"},
		{"hot pocket", "🍕"}, {"pizza roll", "🍕"},
		{"microwave meal", "🍱"}, {"frozen dinner", "🍱"}, {"tv dinner", "🍱"},
		{"protein bar", "💪"}, {"energy bar", "💪"}, {"granola bar", "🥜"},
		{"hummus", "🫘"}, {"guacamole", "🥑"}, {"salsa", "🫙"},
		// Bakery
		{"sourdough", "🍞"}, {"baguette", "🥖"}, {"ciabatta", "🍞"}, {"brioche", "🍞"},
		{"bread", "🍞"}, {"roll", "🍞"}, {"bun", "🍞"}, {"loaf", "🍞"}, {"rye", "🍞"}, {"wheat bread", "🍞"},
		{"pita", "🫓"}, {"naan", "🫓"}, {"tortilla", "🫓"}, {"flatbread", "🫓"},
		{"bagel", "🥯"}, {"croissant", "🥐"}, {"pretzel", "🥨"},
		{"english muffin", "🧇"}, {"muffin", "🧁"}, {"scone", "🧁"}, {"danish", "🧁"}, {"pastry", "🥐"},
		{"cake mix", "🎂"}, {"cake", "🎂"}, {"cupcake", "🧁"}, {"pie", "🥧"}, {"tart", "🥧"},
		{"cookie", "🍪"}, {"brownie", "🍫"}, {"biscuit", "🍪"},
		{"donut", "🍩"}, {"doughnut", "🍩"},
		{"waffle", "🧇"}, {"pancake", "🥞"}, {"mochi", "🍡"},
		// Baking
		{"vanilla extract", "🧁"}, {"baking soda", "🧁"}, {"baking powder", "🧁"},
		{"cornstarch", "🧁"}, {"powdered sugar", "🍚"}, {"brown sugar", "🍬"},
		{"frosting", "🧁"}, {"sprinkles", "🎂"}, {"food coloring", "🎨"},
		{"cupcake liner", "🧁"}, {"pie crust", "🥧"},
		{"bread flour", "🌾"},
		// Pantry & Dry Goods
		{"rice", "🍚"}, {"jasmine", "🍚"}, {"basmati", "🍚"}, {"quinoa", "🍚"}, {"couscous", "🍚"},
		{"pasta", "🍝"}, {"spaghetti", "🍝"}, {"penne", "🍝"}, {"rigatoni", "🍝"}, {"fusilli", "🍝"},
		{"linguine", "🍝"}, {"fettuccine", "🍝"}, {"macaroni", "🍝"}, {"rotini", "🍝"},
		{"orzo", "🍝"}, {"farfalle", "🍝"}, {"angel hair", "🍝"}, {"lasagna", "🍝"},
		{"ramen", "🍜"}, {"noodle", "🍜"}, {"udon", "🍜"}, {"soba", "🍜"}, {"lo mein", "🍜"},
		{"miso", "🍜"},
		{"cereal", "🥣"}, {"oat", "🥣"}, {"granola", "🥣"}, {"muesli", "🥣"},
		{"chickpea", "🫘"}, {"black bean", "🫘"}, {"bean", "🫘"}, {"lentil", "🫘"},
		{"tofu", "🫘"}, {"tempeh", "🫘"}, {"seitan", "🌾"},
		{"peanut butter", "🥜"}, {"almond butter", "🥜"}, {"tahini", "🥜"},
		{"peanut", "🥜"}, {"almond", "🥜"}, {"walnut", "🥜"}, {"pecan", "🥜"},
		{"cashew", "🥜"}, {"pistachio", "🥜"}, {"nut", "🥜"},
		{"sunflower seed", "🌻"}, {"pumpkin seed", "🎃"}, {"sunflower", "🌻"},
		{"chia", "🌻"}, {"flax", "🌻"}, {"seed", "🌻"},
		{"honey", "🍯"}, {"maple syrup", "🍯"}, {"agave", "🍯"},
		{"jam", "🫙"}, {"jelly", "🫙"}, {"preserves", "🫙"}, {"marmalade", "🫙"},
		{"coconut oil", "🥥"}, {"olive oil", "🫒"}, {"sesame oil", "🫒"}, {"oil", "🫒"}, {"olive", "🫒"},
		{"vinegar", "🫙"}, {"balsamic", "🫙"},
		// Condiments
		{"relish", "🌶️"}, {"horseradish", "🌶️"}, {"aioli", "🧄"}, {"pesto", "🌿"},
		{"chimichurri", "🌿"}, {"chutney", "🥭"}, {"tzatziki", "🥒"}, {"tapenade", "🫒"}, {"capers", "🫒"},
		{"worcestershire", "🥫"}, {"hoisin", "🥫"},
		{"harissa", "🌶️"}, {"gochujang", "🌶️"}, {"curry paste", "🍛"},
		{"sriracha", "🌶️"}, {"hot sauce", "🌶️"}, {"tabasco", "🌶️"}, {"buffalo sauce", "🌶️"},
		{"soy sauce", "🥫"}, {"teriyaki", "🥫"}, {"bbq sauce", "🥫"}, {"barbecue", "🥫"},
		{"sauce", "🫙"},
		{"ketchup", "🫙"}, {"mustard", "🫙"}, {"mayo", "🫙"}, {"mayonnaise", "🫙"},
		{"dressing", "🫙"}, {"ranch", "🫙"},
		{"cumin", "🧂"}, {"paprika", "🧂"}, {"cinnamon", "🧂"}, {"nutmeg", "🧂"}, {"turmeric", "🧂"},
		{"salt", "🧂"}, {"seasoning", "🧂"}, {"spice", "🧂"},
		{"flour", "🌾"}, {"sugar", "🧂"}, {"baking", "🧁"}, {"yeast", "🍞"},
		{"soup", "🍲"}, {"broth", "🍲"}, {"stock", "🍲"}, {"chili", "🍲"}, {"stew", "🍲"},
		{"tomato sauce", "🥫"}, {"tomato paste", "🥫"}, {"can of", "🥫"}, {"canned", "🥫"},
		{"taco", "🌮"}, {"tortilla chip", "🌮"},
		// International / Specialty
		{"kimchi", "🥬"}, {"sauerkraut", "🥬"}, {"wasabi", "🟢"},
		{"nori", "🍣"}, {"seaweed", "🍣"},
		// Snacks
		{"chip", "🍟"}, {"popcorn", "🍿"}, {"cracker", "🍘"},
		{"chocolate", "🍫"}, {"candy", "🍬"}, {"gum", "🍬"}, {"gummy", "🍬"},
		{"trail mix", "🥜"}, {"fruit snack", "🍬"}, {"dried fruit", "🍇"}, {"raisin", "🍇"},
		{"cranberry", "🍒"},
		// Beverages
		{"sparkling water", "💧"}, {"seltzer", "💧"}, {"tonic", "💧"},
		{"la croix", "💧"}, {"lacroix", "💧"}, {"water", "💧"},
		{"coconut water", "🥥"},
		{"juice", "🧃"}, {"oj", "🍊"}, {"lemonade", "🍋"},
		{"coke", "🥤"}, {"pepsi", "🥤"}, {"sprite", "🥤"}, {"dr pepper", "🥤"},
		{"ginger ale", "🥤"}, {"cola", "🥤"}, {"root beer", "🍺"}, {"ginger beer", "🍺"},
		{"soda", "🥤"}, {"pop", "🥤"},
		{"espresso", "☕"}, {"latte", "☕"}, {"coffee", "☕"},
		{"matcha", "🍵"}, {"chai", "🍵"}, {"tea", "🍵"}, {"kombucha", "🍵"},
		{"beer", "🍺"}, {"ipa", "🍺"}, {"lager", "🍺"}, {"ale", "🍺"}, {"stout", "🍺"},
		{"champagne", "🥂"}, {"prosecco", "🍾"}, {"wine", "🍷"}, {"cider", "🍎"},
		{"whiskey", "🥃"}, {"vodka", "🍸"}, {"tequila", "🍸"}, {"rum", "🍹"}, {"gin", "🍸"},
		{"liquor", "🥃"},
		{"protein shake", "💪"}, {"smoothie", "🥤"}, {"shake", "🥤"},
		{"sports drink", "🥤"}, {"energy drink", "⚡"}, {"red bull", "⚡"}, {"monster", "⚡"},
		{"protein", "💪"},
		// Frozen
		{"frozen", "❄️"}, {"pizza", "🍕"},
		// Household & Cleaning
		{"dish soap", "🧼"}, {"hand soap", "🧼"}, {"soap", "🧼"},
		{"detergent", "🧴"}, {"laundry", "🧴"}, {"fabric softener", "🧺"}, {"dryer sheet", "🧺"},
		{"stain remover", "🧺"},
		{"sponge", "🧽"}, {"scrub", "🧽"}, {"brush", "🧽"},
		{"paper towel", "🧻"}, {"toilet paper", "🧻"}, {"tissue", "🧻"}, {"napkin", "🧻"},
		{"trash bag", "🗑️"}, {"garbage", "🗑️"},
		{"aluminum foil", "🫕"}, {"foil", "🫕"}, {"plastic wrap", "🫕"}, {"parchment paper", "📜"}, {"wax paper", "📜"},
		{"ziplock", "🤐"}, {"sandwich bag", "🤐"}, {"zip lock", "🛍️"}, {"ziploc", "🛍️"},
		{"bleach", "🧴"}, {"disinfectant", "🧴"}, {"cleaner", "🧴"}, {"wipes", "🧴"},
		{"rubber glove", "🧤"},
		{"mop", "🧹"}, {"broom", "🧹"}, {"dustpan", "🧹"}, {"vacuum bag", "🧹"}, {"lint roller", "🧹"},
		{"air freshener", "🌸"}, {"light bulb", "💡"}, {"battery", "🔋"}, {"candle", "🕯️"},
		{"match", "🔥"}, {"charcoal", "🔥"}, {"propane", "⛽"}, {"extension cord", "🔌"},
		{"duct tape", "🛠️"}, {"super glue", "🛠️"}, {"command strip", "🛠️"},
		{"storage bin", "📦"}, {"hanger", "👔"},
		// Health & Personal Care
		{"vitamin", "💊"}, {"supplement", "💊"}, {"medicine", "💊"}, {"tylenol", "💊"}, {"advil", "💊"}, {"ibuprofen", "💊"},
		{"toothpaste", "🪥"}, {"toothbrush", "🪥"}, {"floss", "🪥"}, {"mouthwash", "🪥"},
		{"shampoo", "🧴"}, {"conditioner", "🧴"}, {"body wash", "🧴"},
		{"lotion", "🧴"}, {"moisturizer", "🧴"}, {"sunscreen", "🧴"}, {"deodorant", "🧴"},
		{"bandaid", "🩹"}, {"band-aid", "🩹"}, {"first aid", "🩹"},
		{"razor", "🪒"}, {"shaving", "🪒"},
		// Baby
		{"baby food", "🍼"}, {"baby cereal", "🍼"}, {"baby wipe", "👶"}, {"formula", "🍼"},
		{"pacifier", "👶"}, {"diaper", "👶"}, {"sippy cup", "🥤"}, {"bib", "👶"},
		{"teething", "👶"}, {"nursing pad", "🤱"}, {"baby", "👶"},
		// Pet
		{"dog food", "🐕"}, {"dog treat", "🐕"}, {"cat food", "🐈"}, {"cat litter", "🐱"},
		{"bird seed", "🐦"}, {"fish food", "🐠"}, {"hamster food", "🐹"}, {"rabbit food", "🐰"},
		{"pet treat", "🦴"}, {"leash", "🐕"}, {"collar", "🐕"}, {"flea", "🐕"},
		{"pet shampoo", "🐕"}, {"litter", "🐱"}, {"chew toy", "🦴"}, {"catnip", "🐱"},
		{"pet", "🐾"},
	}

	for _, kw := range keywords {
		if containsWord(lower, kw.kw) {
			return kw.emoji
		}
	}

	// Department fallback
	deptEmojis := map[string]string{
		"Produce":              "🥬",
		"Dairy & Eggs":         "🥛",
		"Meat & Seafood":       "🥩",
		"Deli":                 "🥪",
		"Bakery":               "🍞",
		"Pantry & Dry Goods":   "🥫",
		"Snacks":               "🍿",
		"Snacks & Candy":       "🍿",
		"Beverages":            "🥤",
		"Frozen":               "❄️",
		"Household & Cleaning": "🧹",
		"Health & Personal":    "💊",
		"Health & Personal Care": "💊",
		"Baby & Pet":           "🐾",
	}
	if e, ok := deptEmojis[department]; ok {
		return e
	}
	return "🛒"
}

