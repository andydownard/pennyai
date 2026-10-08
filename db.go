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
func getUserHouseholdID(db dbExecer, userID string) int64 {
	var hid int64
	db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&hid)
	return hid
}


func recordModification(ex dbExecer, itemID int, userID string, action, detail, source string) {
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
func getGlobalBrandDefault(db dbExecer, category string, userDiets ...[]string) (string, bool) {
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
func getUserDiets(db dbExecer, userID string) []string {
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
func getDietaryBrandDefault(db dbExecer, category string, diets []string) (string, bool) {
	if len(diets) == 0 {
		return "", false
	}
	// Build LIKE conditions for each diet in the user's profile
	// This matches users whose diets JSON array contains any of the same values
	args := make([]interface{}, 0, len(diets)+1)
	args = append(args, category)
	likes := make([]string, len(diets))
	for i, diet := range diets {
		likes[i] = "CAST(dp.diets AS TEXT) ILIKE ?" // diets is JSONB on Postgres; ILIKE needs text
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
func recordBrandChoice(db dbExecer, userID string, category, brand, source string) {
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
func resolveBrandAlternatives(db dbExecer, userID string, pennyBrandMode string, items []ListItem, parsedItems []ParsedItem, useGlobalDefaults ...bool) []BrandDisambiguation {
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
			if lp, err := parseDBTime(lastPicked); err == nil {
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

// createSoloHouseholdTx creates a new household with the given user as owner
// using the provided transaction or db handle.
func createSoloHouseholdTx(ex dbExecer, userID string) (int, error) {
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
func createSoloHousehold(db *PGDB, userID string) (int, error) {
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

func getHouseholdUserIDs(db dbExecer, userID string) ([]string, error) {
	rows, err := db.Query(
		"SELECT user_id FROM household_members WHERE household_id = (SELECT household_id FROM users WHERE id = ? AND household_id IS NOT NULL)",
		userID,
	)
	if err != nil {
		log.Printf("[warn] household lookup failed for user %s, falling back to single user: %v", userID, err)
		return []string{userID}, nil
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return []string{userID}, nil
	}
	return ids, nil
}

// householdItemsWhereClause returns a SQL fragment "user_id IN (?,?,...)" and
// the corresponding args, for queries that should span the whole household.
// Pass optional pre-computed household IDs to avoid re-querying.
func householdItemsWhereClause(db dbExecer, userID string, cachedIDs ...[]string) (string, []interface{}) {
	var ids []string
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
func insertSavedListItems(tx dbExecer, savedListID int64, items []itemData) error {
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
	var addedByName sql.NullString
	var claimedByID sql.NullString
	var claimedByName sql.NullString
	err := scanner.Scan(&item.ID, &item.Name, &item.Brand, &item.Department, &item.Quantity, &item.Size, &item.Notes, &item.Source, &checked, &urgent, &item.CreatedAt, &addedByName, &claimedByID, &claimedByName, &item.SwapSavingsCents, &item.Emoji)
	if err != nil {
		return ListItem{}, err
	}
	item.Checked = checked == 1
	item.Urgent = urgent == 1
	item.AddedByName = addedByName.String
	if claimedByID.Valid {
		id := claimedByID.String
		item.ClaimedByID = &id
		item.ClaimedByName = claimedByName.String
	}
	return item, nil
}

// getItemByID fetches a single list item belonging to a household.
func getItemByID(db dbExecer, id int, userID string) (ListItem, error) {
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
func upsertListItem(db *PGDB, userID string, name, brand, department, size string, quantity int, source, notes, emoji string) (ListItem, bool, error) {
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

	// Serialise concurrent upserts for the same list. The look-up below followed
	// by an INSERT is check-then-act: under READ COMMITTED two requests can both
	// find nothing and both insert, leaving duplicate rows (a double-tapped add,
	// or an SMS and the app adding "eggs" together). The list is shared across a
	// household with no unique key to lean on, so take a transaction-scoped
	// advisory lock keyed on the household (or the user, when solo). It releases
	// at COMMIT/ROLLBACK.
	lockKey := "list_items:u:" + userID
	if hid := getUserHouseholdID(db, userID); hid != 0 {
		lockKey = fmt.Sprintf("list_items:h:%d", hid)
	}
	if _, err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", lockKey); err != nil {
		return ListItem{}, false, fmt.Errorf("locking list for upsert: %w", err)
	}

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
			// For email sources, report as "added" (not skipped) since the user
			// can't interactively dedupe via one-way communication.
			isNew = source == "email"
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
func addParsedItems(db *PGDB, userID string, parsed []ParsedItem, source string) (added []ListItem, skipped []SkippedItem) {
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
func collectListItems(db dbExecer, userID string) ([]itemData, error) {
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

