package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"strings"
)

// enrichAddedItems resolves brand alternatives, checks swap deals, and checks household duplicates
// for newly added items. Shared by text-add and image-parse paths.
func (s *Server) enrichAddedItems(userID string, added []ListItem, parsedItems []ParsedItem, resp *ParseResponse) {
	var pennyBrandMode string
	s.db.QueryRow("SELECT COALESCE(penny_brand_mode,'ask') FROM users WHERE id = ?", userID).Scan(&pennyBrandMode)
	abVariant, _ := getVariant(s.db, "onboarding_v2", userID)
	useDefaults := abVariant == "brand_defaults"
	disambiguations := resolveBrandAlternatives(s.db, userID, pennyBrandMode, added, parsedItems, useDefaults)
	if len(disambiguations) > 0 {
		resp.BrandDisambiguations = disambiguations
	}

	if swap := s.checkDealsForItems(userID, added); swap != nil {
		resp.PendingSwap = swap
	}

	if dupes := s.checkHouseholdDuplicates(userID, added); len(dupes) > 0 {
		resp.DuplicateWarnings = dupes
	}
}

// executeLoadSavedList handles the "load_saved_list" command action.
func (s *Server) executeLoadSavedList(cmd *ParsedCommand, session Session, hhIDs []string, resp *ParseResponse) {
	resp.Action = "load_saved_list"
	slWhereClause, slWhereArgs := householdItemsWhereClause(s.db, session.UserID, hhIDs)
	slQuery := "SELECT sl.id, sl.name, COUNT(sli.id), COALESCE(sl.store_name,''), COALESCE(sl.is_master,0) FROM saved_lists sl LEFT JOIN saved_list_items sli ON sli.saved_list_id = sl.id WHERE sl." + slWhereClause
	cmdStoreName := strings.TrimSpace(cmd.StoreName)
	if cmdStoreName != "" {
		slQuery += " AND LOWER(sl.store_name) = LOWER(?)"
		slWhereArgs = append(slWhereArgs, cmdStoreName)
	}
	slQuery += " GROUP BY sl.id ORDER BY sl.name"
	slRows, slErr := s.db.Query(slQuery, slWhereArgs...)
	if slErr == nil {
		defer slRows.Close()
		for slRows.Next() {
			var sl SavedListSummary
			var isMaster int
			if err := slRows.Scan(&sl.ID, &sl.Name, &sl.ItemCount, &sl.StoreName, &isMaster); err == nil {
				sl.IsMaster = isMaster == 1
				resp.SavedLists = append(resp.SavedLists, sl)
			}
		}
	}
	if len(resp.SavedLists) == 0 {
		if cmdStoreName != "" {
			resp.Message = fmt.Sprintf("You don't have any saved lists for %s yet. Try saving your current list with a store name!", cmdStoreName)
		} else {
			resp.Message = "You don't have any saved lists yet. Tell me to save your current list and give it a name!"
		}
	} else if cmd.SavedListName != "" {
		targetName := strings.ToLower(strings.TrimSpace(cmd.SavedListName))
		var matchedList *SavedListSummary
		for i := range resp.SavedLists {
			if strings.ToLower(resp.SavedLists[i].Name) == targetName {
				matchedList = &resp.SavedLists[i]
				break
			}
		}
		if matchedList == nil {
			for i := range resp.SavedLists {
				if strings.Contains(strings.ToLower(resp.SavedLists[i].Name), targetName) {
					matchedList = &resp.SavedLists[i]
					break
				}
			}
		}
		if matchedList == nil && cmdStoreName != "" {
			for i := range resp.SavedLists {
				if resp.SavedLists[i].IsMaster {
					matchedList = &resp.SavedLists[i]
					break
				}
			}
		}
		if matchedList != nil {
			added := s.loadSavedListIntoActive(matchedList.ID, session.UserID)
			resp.Added = added
			resp.Message = cmd.Message
		} else {
			resp.Message = fmt.Sprintf("I couldn't find a list called %q. Here are your saved lists — pick one!", cmd.SavedListName)
		}
	} else if cmdStoreName != "" && len(resp.SavedLists) == 1 {
		added := s.loadSavedListIntoActive(resp.SavedLists[0].ID, session.UserID)
		resp.Added = added
		resp.Message = cmd.Message
	} else {
		resp.Message = cmd.Message
	}
}

// executeDeleteSavedList handles the "delete_saved_list" command action.
func (s *Server) executeDeleteSavedList(cmd *ParsedCommand, session Session, hhIDs []string, resp *ParseResponse) {
	resp.Action = "delete_saved_list"
	if cmd.SavedListName == "" {
		resp.Message = "Which saved list would you like me to delete? Just tell me the name."
		return
	}
	targetName := strings.ToLower(strings.TrimSpace(cmd.SavedListName))
	slWhereClause, slWhereArgs := householdItemsWhereClause(s.db, session.UserID, hhIDs)
	var matchedID int
	var matchedName string
	rows, err := s.db.Query(
		"SELECT id, name FROM saved_lists WHERE "+slWhereClause,
		slWhereArgs...,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int
			var name string
			if err := rows.Scan(&id, &name); err == nil {
				if strings.ToLower(name) == targetName || strings.Contains(strings.ToLower(name), targetName) {
					matchedID = id
					matchedName = name
					break
				}
			}
		}
	}
	if matchedID > 0 {
		s.db.Exec("DELETE FROM saved_lists WHERE id = ?", matchedID)
		resp.Message = fmt.Sprintf("Deleted your %q list!", matchedName)
	} else {
		resp.Message = fmt.Sprintf("I couldn't find a saved list called %q.", cmd.SavedListName)
	}
}

// loadSavedListIntoActive loads items from a saved list into the active list.
// Returns the number of items added.
func (s *Server) loadSavedListIntoActive(savedListID int, userID string) int {
	loadRows, loadErr := s.db.Query(
		"SELECT name, brand, department, quantity, size, notes FROM saved_list_items WHERE saved_list_id = ?",
		savedListID,
	)
	if loadErr != nil {
		return 0
	}
	var savedItems []itemData
	for loadRows.Next() {
		var si itemData
		if err := loadRows.Scan(&si.Name, &si.Brand, &si.Department, &si.Quantity, &si.Size, &si.Notes); err == nil {
			savedItems = append(savedItems, si)
		}
	}
	loadRows.Close()
	added := 0
	for _, si := range savedItems {
		siEmoji := inferItemEmoji(si.Name, si.Department)
		_, _, err := upsertListItem(s.db, userID, si.Name, si.Brand, si.Department, si.Size, si.Quantity, "app", "", siEmoji)
		if err == nil {
			added++
		}
	}
	return added
}

// retryEnrichItems re-attempts AI parsing for items flagged needs_enrichment.
// Runs async (in a goroutine). Updates department, emoji, brand on success.
func (s *Server) retryEnrichItems(userID string) {
	// Small delay to avoid back-to-back API calls
	rows, err := s.db.Query(
		"SELECT id, name FROM list_items WHERE user_id = ? AND needs_enrichment = 1 LIMIT 10",
		userID,
	)
	if err != nil {
		log.Printf("[enrich-retry] query error: %v", err)
		return
	}
	defer rows.Close()

	type pending struct {
		id   int
		name string
	}
	var items []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.name); err != nil {
			continue
		}
		items = append(items, p)
	}
	if len(items) == 0 {
		return
	}

	for _, item := range items {
		// Re-parse each item individually via Claude
		cmd, err := parseCommandWithClaude(s.config.AnthropicAPIKey, item.name, nil, "", "")
		if err != nil {
			log.Printf("[enrich-retry] failed for item %d (%s): %v", item.id, item.name, err)
			// Mark as no longer needing enrichment after 1 failed retry
			s.db.Exec("UPDATE list_items SET needs_enrichment = 0 WHERE id = ?", item.id)
			continue
		}
		if len(cmd.Items) > 0 {
			p := cmd.Items[0]
			emoji := inferItemEmoji(p.Name, p.Department)
			name := p.Name
			if name == "" {
				name = item.name
			}
			dept := p.Department
			if dept == "" {
				dept = "Other"
			}
			_, err := s.db.Exec(
				"UPDATE list_items SET name = ?, department = ?, brand = ?, emoji = ?, needs_enrichment = 0 WHERE id = ?",
				name, dept, p.Brand, emoji, item.id,
			)
			if err != nil {
				log.Printf("[enrich-retry] update error for item %d: %v", item.id, err)
			} else {
				log.Printf("[enrich-retry] enriched item %d: %s → %s (%s)", item.id, item.name, name, dept)
			}
		} else {
			s.db.Exec("UPDATE list_items SET needs_enrichment = 0 WHERE id = ?", item.id)
		}
	}
}

// checkImageDimensions analyzes base64 image data for AI-generation signals.
// Returns a list of suspicious flags (empty = no issues detected).
func checkImageDimensions(imgData string) []string {
	// Strip data URI prefix if present
	b64Data := imgData
	if strings.HasPrefix(imgData, "data:") {
		if idx := strings.Index(imgData, ";base64,"); idx > 0 {
			b64Data = imgData[idx+8:]
		}
	}

	raw, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		// Try RawStdEncoding (no padding)
		raw, err = base64.RawStdEncoding.DecodeString(b64Data)
		if err != nil {
			return nil // Can't decode — skip heuristics
		}
	}

	var flags []string

	// Decode image config (dimensions only — no full decode needed)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil
	}

	w, h := cfg.Width, cfg.Height

	// AI generators commonly output perfect squares (512, 768, 1024, 2048)
	aiSquares := map[int]bool{512: true, 768: true, 1024: true, 2048: true}
	if w == h && aiSquares[w] {
		flags = append(flags, fmt.Sprintf("perfect square %dx%d (common AI output size)", w, h))
	}

	// Real phone receipt photos are typically tall (portrait) with high resolution
	// AI images rarely exceed 2048 in any dimension
	// A real phone photo is typically 3000-4032px on the long side
	if w <= 1024 && h <= 1024 {
		flags = append(flags, fmt.Sprintf("small dimensions %dx%d (real phone photos are typically 3000+ px)", w, h))
	}

	// Suspiciously exact power-of-2 or common AI dimensions
	aiDims := map[[2]int]bool{
		{512, 512}:   true,
		{512, 768}:   true,
		{768, 512}:   true,
		{768, 1024}:  true,
		{1024, 768}:  true,
		{1024, 1024}: true,
		{1024, 1536}: true,
		{1536, 1024}: true,
		{2048, 2048}: true,
	}
	if aiDims[[2]int{w, h}] {
		flags = append(flags, fmt.Sprintf("exact AI-typical dimensions %dx%d", w, h))
	}

	// File size check: AI-generated PNGs tend to be very large per pixel,
	// while JPEG receipts from phones are compressed
	bytesPerPixel := float64(len(raw)) / float64(w*h)
	if bytesPerPixel > 3.0 && w <= 1024 {
		flags = append(flags, fmt.Sprintf("high bytes-per-pixel ratio %.2f (possible uncompressed AI output)", bytesPerPixel))
	}

	return flags
}

// isMealPhrase returns true if the text looks like a meal/recipe concept rather than
// a specific grocery item. Used to prevent adding phrases like "dinner for 4" as
// literal list items.
func isMealPhrase(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	mealWords := []string{"dinner", "lunch", "breakfast", "meal", "recipe", "cook", "make", "what should", "supper", "brunch"}
	for _, word := range mealWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}
