package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"strings"
)

// handleSwapAction processes swap accept/decline requests.
// Returns true if handled (caller should return), false if not a swap request.
func (s *Server) handleSwapAction(w http.ResponseWriter, req ParseAndAddRequest, session Session) {
	hhIDs, _ := getHouseholdUserIDs(s.db, session.UserID)
	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID, hhIDs)
	resp := ParseResponse{Items: []ListItem{}, Skipped: []SkippedItem{}}

	if req.Text == "accept_swap" {
		// 1-swap-per-user-per-campaign: prevent accepting the same deal more than once (B14)
		var priorAccepts int
		s.db.QueryRow(
			"SELECT COUNT(*) FROM swap_events WHERE user_id = ? AND deal_id = ? AND action = 'accepted'",
			session.UserID, req.SwapDealID,
		).Scan(&priorAccepts)
		if priorAccepts > 0 {
			writeConflict(w, "Swap already accepted")
			return
		}

		// Wrap entire accept in a transaction — budget check + mutations are atomic (B11 fix)
		tx, err := s.db.Begin()
		if err != nil {
			log.Printf("[swaps] failed to begin tx: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process swap"})
			return
		}
		defer tx.Rollback()

		// Verify deal is active and read deal data — inside tx for atomicity
		var offerBrand, offerProduct, status, dealType, trialProductName, trialSize string
		var discountCents, budgetTotal, budgetSpent, quantityOffered int
		err = tx.QueryRow(
			`SELECT offer_brand, offer_product, discount_cents, total_budget_cents, budget_spent_cents, status,
			        deal_type, quantity_offered, trial_product_name, trial_size
			 FROM brand_deals WHERE id = ?`,
			req.SwapDealID,
		).Scan(&offerBrand, &offerProduct, &discountCents, &budgetTotal, &budgetSpent, &status,
			&dealType, &quantityOffered, &trialProductName, &trialSize)
		if err != nil || status != "active" {
			writeBadRequest(w, "Deal no longer available")
			return
		}
		if budgetTotal > 0 && budgetSpent >= budgetTotal {
			writeBadRequest(w, "Deal budget exhausted")
			return
		}

		// Atomic budget increment — conditional UPDATE prevents overspend under concurrency
		if budgetTotal > 0 {
			result, err := tx.Exec(
				"UPDATE brand_deals SET budget_spent_cents = budget_spent_cents + ? WHERE id = ? AND (total_budget_cents = 0 OR budget_spent_cents + ? <= total_budget_cents)",
				discountCents, req.SwapDealID, discountCents,
			)
			if err != nil {
				log.Printf("[swaps] failed to update deal %d budget: %v", req.SwapDealID, err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process swap"})
				return
			}
			rows, _ := result.RowsAffected()
			if rows == 0 {
				writeBadRequest(w, "Deal budget exhausted")
				return
			}
		} else {
			if _, err := tx.Exec("UPDATE brand_deals SET budget_spent_cents = budget_spent_cents + ? WHERE id = ?",
				discountCents, req.SwapDealID); err != nil {
				log.Printf("[swaps] failed to update deal %d budget: %v", req.SwapDealID, err)
			}
		}

		// Apply deal to list — behavior varies by deal_type
		if req.SwapItemID > 0 {
			switch dealType {
			case "bogo":
				qty := quantityOffered
				if qty < 2 {
					qty = 2
				}
				updateArgs := append([]interface{}{offerProduct, offerBrand, qty, req.SwapItemID}, whereArgs...)
				_, err = tx.Exec(
					"UPDATE list_items SET name = ?, brand = ?, quantity = ? WHERE id = ? AND "+whereClause,
					updateArgs...,
				)
				if err != nil {
					log.Printf("[swaps] bogo accept update error: %v", err)
				}
			case "trial":
				trialResult, trialErr := tx.Exec(
					"INSERT INTO list_items (user_id, name, brand, size, quantity, department, source, swap_savings_cents) VALUES (?, ?, ?, ?, 1, 'Other', 'swap', ?)",
					session.UserID, trialProductName, offerBrand, trialSize, discountCents,
				)
				if trialErr != nil {
					log.Printf("[swaps] trial accept insert error: %v", trialErr)
				}
				_ = trialResult
			default:
				updateArgs := append([]interface{}{offerProduct, offerBrand, req.SwapItemID}, whereArgs...)
				_, err = tx.Exec(
					"UPDATE list_items SET name = ?, brand = ? WHERE id = ? AND "+whereClause,
					updateArgs...,
				)
				if err != nil {
					log.Printf("[swaps] accept update error: %v", err)
				}
			}

			// Record savings on the swapped list item
			if dealType != "trial" {
				savingsArgs := append([]interface{}{discountCents, req.SwapItemID}, whereArgs...)
				if _, err := tx.Exec(
					"UPDATE list_items SET swap_savings_cents = ? WHERE id = ? AND "+whereClause,
					savingsArgs...,
				); err != nil {
					log.Printf("[swaps] failed to set swap_savings_cents on item %d: %v", req.SwapItemID, err)
				}
			}
		}

		// Log accepted event
		whyTappedInt := 0
		if req.WhyTapped {
			whyTappedInt = 1
		}
		result, err := tx.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action, time_to_action_ms, why_tapped) VALUES (?, ?, ?, 'accepted', ?, ?)",
			session.UserID, req.SwapDealID, req.SwapItemID, req.TimeToActionMs, whyTappedInt)
		if err != nil {
			log.Printf("[swaps] failed to insert swap event: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process swap"})
			return
		}
		swapEventID, _ := result.LastInsertId()

		// Insert rebate ledger entry
		if _, err := tx.Exec(
			"INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES (?, ?, ?, ?, 'pending')",
			session.UserID, swapEventID, req.SwapDealID, discountCents,
		); err != nil {
			log.Printf("[swaps] failed to insert rebate: %v", err)
		}

		// Update user counters and rebate balance
		if _, err := tx.Exec("UPDATE users SET deal_accept_count = deal_accept_count + 1, swap_credits_cents = swap_credits_cents + ? WHERE id = ?",
			discountCents, session.UserID); err != nil {
			log.Printf("[swaps] failed to update user credits for user %d: %v", session.UserID, err)
		}

		// Auto-pause deal if budget now exhausted
		var newBudgetSpent int
		tx.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", req.SwapDealID).Scan(&newBudgetSpent)
		if budgetTotal > 0 && newBudgetSpent >= budgetTotal {
			if _, err := tx.Exec("UPDATE brand_deals SET status = 'paused' WHERE id = ?", req.SwapDealID); err != nil {
				log.Printf("[swaps] failed to auto-pause deal %d: %v", req.SwapDealID, err)
			}
			log.Printf("[swaps] deal %d auto-paused: budget exhausted", req.SwapDealID)
		}

		if err := tx.Commit(); err != nil {
			log.Printf("[swaps] failed to commit accept tx: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process swap"})
			return
		}

		// Read updated balance for cumulative total in response
		var newBalance int
		s.db.QueryRow("SELECT swap_credits_cents FROM users WHERE id = ?", session.UserID).Scan(&newBalance)

		// Record learning signal
		go s.recordSignal(session.UserID, "swap_accepted", offerBrand, fmt.Sprintf("%d", discountCents), map[string]interface{}{
			"deal_id": req.SwapDealID, "deal_type": dealType, "discount_cents": discountCents,
		})

		resp.Action = "swap_accepted"
		resp.SwapCreditsCents = newBalance
		switch dealType {
		case "bogo":
			resp.Message = fmt.Sprintf("BOGO! Get %d for the price of 1. You save $%.2f. ($%.2f total saved!)", quantityOffered, float64(discountCents)/100, float64(newBalance)/100)
		case "trial":
			resp.Message = fmt.Sprintf("Free trial added! %s (%s). You save $%.2f. ($%.2f total saved!)", trialProductName, trialSize, float64(discountCents)/100, float64(newBalance)/100)
		default:
			resp.Message = fmt.Sprintf("Swapped! You save $%.2f. ($%.2f total saved!)", float64(discountCents)/100, float64(newBalance)/100)
		}
		if req.SwapItemID > 0 {
			if item, err := getItemByID(s.db, req.SwapItemID, session.UserID); err == nil {
				resp.Updated = []ListItem{item}
			}
		}
	} else {
		// decline_swap
		declineWhyTapped := 0
		if req.WhyTapped {
			declineWhyTapped = 1
		}
		s.db.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action, decline_reason, time_to_action_ms, why_tapped) VALUES (?, ?, ?, 'declined', ?, ?, ?)",
			session.UserID, req.SwapDealID, req.SwapItemID, req.DeclineReason, req.TimeToActionMs, declineWhyTapped)
		s.db.Exec(`INSERT INTO swap_suppressions (user_id, deal_id, decline_count) VALUES (?, ?, 1)
			ON CONFLICT(user_id, deal_id) DO UPDATE SET decline_count = decline_count + 1`,
			session.UserID, req.SwapDealID)

		go s.recordSignal(session.UserID, "swap_declined", "", req.DeclineReason, map[string]interface{}{
			"deal_id": req.SwapDealID, "reason": req.DeclineReason,
		})

		resp.Action = "swap_declined"
		resp.Message = "No problem, keeping your original choice."
	}

	writeJSON(w, http.StatusCreated, resp)
	go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// handleReceiptScan processes receipt photo scanning and cross-references against the list.
func (s *Server) handleReceiptScan(w http.ResponseWriter, session Session, images []string) {
	if s.config.AnthropicAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI parsing not configured"})
		return
	}

	// Rate limit: 3/day, 10/week
	if checkReceiptRateLimit(session.UserID) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "You've reached the receipt scanning limit. Try again tomorrow."})
		return
	}

	// SHA-256 dedup check
	hasher := sha256.New()
	for _, img := range images {
		hasher.Write([]byte(img))
	}
	imageHash := hex.EncodeToString(hasher.Sum(nil))

	var existingReceiptID int
	if err := s.db.QueryRow("SELECT id FROM receipts WHERE user_id = ? AND image_hash = ?",
		session.UserID, imageHash).Scan(&existingReceiptID); err == nil {
		writeJSON(w, http.StatusOK, ParseResponse{
			Items:   []ListItem{},
			Skipped: []SkippedItem{},
			Action:  "receipt_duplicate",
			Message: "Looks like I've already processed this receipt!",
		})
		return
	}

	// Pre-check: image dimension heuristics for AI-generated images
	for _, img := range images {
		if flags := checkImageDimensions(img); len(flags) > 0 {
			log.Printf("[receipt-fraud] Image dimension flags for user %d: %v", session.UserID, flags)
			// Don't reject outright — just log. Claude's visual analysis is more reliable.
			// But if the image is a perfect square (strong AI signal), add extra scrutiny.
		}
	}

	// Load current unchecked list items
	hhIDs, _ := getHouseholdUserIDs(s.db, session.UserID)
	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID, hhIDs)
	rows, err := s.db.Query(
		"SELECT "+listItemColumns+" FROM "+listItemJoins+" WHERE li."+whereClause+" AND li.checked = 0 ORDER BY li.name",
		whereArgs...,
	)
	var currentItems []ListItem
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			item, err := scanListItem(rows)
			if err == nil {
				currentItems = append(currentItems, item)
			}
		}
	}

	// Parse receipt
	receiptParser := s.parseReceipt
	if receiptParser == nil {
		receiptParser = parseReceiptWithClaude
	}
	matchedNames, err := receiptParser(s.config.AnthropicAPIKey, images, currentItems)
	if err != nil {
		if err.Error() == "receipt_forgery_detected" {
			log.Printf("[receipt-fraud] BLOCKED receipt from user %d — AI forgery detected", session.UserID)
			writeJSON(w, http.StatusOK, ParseResponse{
				Items:   []ListItem{},
				Skipped: []SkippedItem{},
				Action:  "receipt_rejected",
				Message: "Hmm, I couldn't verify this receipt. Please try scanning a clear photo of a physical receipt.",
			})
			return
		}
		log.Printf("[receipt] parse error for user %d: %v", session.UserID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Couldn't read the receipt. Try again?"})
		return
	}

	// Check off matched items
	var checkedItems []ListItem
	for _, matchName := range matchedNames {
		for _, item := range currentItems {
			if strings.EqualFold(item.Name, matchName) && !item.Checked {
				s.db.Exec("UPDATE list_items SET checked = 1 WHERE id = ?", item.ID)
				recordModification(s.db, item.ID, session.UserID, "checked", "receipt scan", "app")
				item.Checked = true
				checkedItems = append(checkedItems, item)
				break
			}
		}
	}

	// Insert receipt record
	result, insertErr := s.db.Exec("INSERT INTO receipts (user_id, image_hash, items_found, items_matched) VALUES (?, ?, ?, ?)",
		session.UserID, imageHash, len(matchedNames), len(checkedItems))

	// Enhanced receipt processing (structured extraction + swap verification) — async
	if insertErr == nil {
		receiptID64, _ := result.LastInsertId()
		receiptID := int(receiptID64)
		go s.processReceiptEnhanced(session.UserID, receiptID, matchedNames, images)
	}

	resp := ParseResponse{
		Items:   []ListItem{},
		Skipped: []SkippedItem{},
		Checked: checkedItems,
		Action:  "receipt_processed",
	}
	if len(checkedItems) > 0 {
		resp.Message = fmt.Sprintf("Found %d items on your receipt — checked them off!", len(checkedItems))
	} else {
		resp.Message = "I couldn't match any receipt items to your list. You might have already checked them off!"
	}
	writeJSON(w, http.StatusCreated, resp)
	go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// handleTextCommand processes natural language text commands via Claude parsing.
func (s *Server) handleTextCommand(w http.ResponseWriter, session Session, text string, hhIDs []int) {
	// Get current list items so the LLM can reference them
	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID, hhIDs)
	rows, err := s.db.Query(
		"SELECT "+listItemColumns+" FROM "+listItemJoins+" WHERE li."+whereClause+" ORDER BY li.name",
		whereArgs...,
	)
	var currentItems []ListItem
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			item, err := scanListItem(rows)
			if err == nil {
				currentItems = append(currentItems, item)
			}
		}
	}

	// Get user name for personalized messages
	var userName string
	s.db.QueryRow("SELECT COALESCE(name,'') FROM users WHERE id = ?", session.UserID).Scan(&userName)
	firstName := strings.Split(strings.TrimSpace(userName), " ")[0]

	// Build learning context for Penny's system prompt
	learningCtx := s.buildLearningContext(session.UserID)

	needsEnrichment := false
	cmd, err := s.parseCommand(s.config.AnthropicAPIKey, text, currentItems, firstName, learningCtx)
	if err != nil {
		log.Printf("[parse] command error for user %d: %v — falling back to raw add", session.UserID, err)
		// Fallback: add the raw text as an item instead of showing an error
		cmd = &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: text, Department: "Other", Quantity: 1}},
			Message: fmt.Sprintf("Added %s to your list!", text),
		}
		needsEnrichment = true
	}

	resp := ParseResponse{
		Action:  cmd.Action,
		Message: cmd.Message,
		Items:   []ListItem{},
		Skipped: []SkippedItem{},
	}

	switch cmd.Action {
	case "remove":
		for _, p := range cmd.Items {
			if p.Name == "" {
				continue
			}
			for _, existing := range currentItems {
				if strings.EqualFold(existing.Name, p.Name) || strings.EqualFold(existing.Brand+" "+existing.Name, p.Name) {
					delArgs := append([]interface{}{existing.ID}, whereArgs...)
					if _, err := s.db.Exec("DELETE FROM list_items WHERE id = ? AND "+whereClause, delArgs...); err != nil {
						log.Printf("[parse] remove item error: %v", err)
					}
					resp.Removed = append(resp.Removed, existing)
					break
				}
			}
		}
	case "check":
		for _, p := range cmd.Items {
			if p.Name == "" {
				continue
			}
			for _, existing := range currentItems {
				if strings.EqualFold(existing.Name, p.Name) && !existing.Checked {
					if _, err := s.db.Exec("UPDATE list_items SET checked = 1 WHERE id = ?", existing.ID); err != nil {
						log.Printf("[parse] check item error: %v", err)
					}
					existing.Checked = true
					resp.Checked = append(resp.Checked, existing)
					break
				}
			}
		}
	case "uncheck":
		for _, p := range cmd.Items {
			if p.Name == "" {
				continue
			}
			for _, existing := range currentItems {
				if strings.EqualFold(existing.Name, p.Name) && existing.Checked {
					if _, err := s.db.Exec("UPDATE list_items SET checked = 0 WHERE id = ?", existing.ID); err != nil {
						log.Printf("[parse] uncheck item error: %v", err)
					}
					existing.Checked = false
					resp.Checked = append(resp.Checked, existing)
					break
				}
			}
		}
	case "update":
		for _, p := range cmd.Items {
			if p.Name == "" {
				continue
			}
			for _, existing := range currentItems {
				if strings.EqualFold(existing.Name, p.Name) {
					var setClauses []string
					var setArgs []interface{}
					if p.NewName != "" && !strings.EqualFold(p.NewName, existing.Name) {
						setClauses = append(setClauses, "name = ?")
						setArgs = append(setArgs, p.NewName)
					}
					if p.Quantity > 0 {
						setClauses = append(setClauses, "quantity = ?")
						setArgs = append(setArgs, p.Quantity)
					}
					if p.Size != "" {
						setClauses = append(setClauses, "size = ?")
						setArgs = append(setArgs, p.Size)
					}
					if p.Brand != "" {
						setClauses = append(setClauses, "brand = ?")
						setArgs = append(setArgs, p.Brand)
					}
					if p.Department != "" {
						setClauses = append(setClauses, "department = ?")
						setArgs = append(setArgs, p.Department)
					}
					if p.Notes != "" {
						setClauses = append(setClauses, "notes = ?")
						setArgs = append(setArgs, p.Notes)
					}
					if len(setClauses) > 0 {
						query := "UPDATE list_items SET " + strings.Join(setClauses, ", ") + " WHERE id = ? AND " + whereClause
						setArgs = append(setArgs, existing.ID)
						setArgs = append(setArgs, whereArgs...)
						if _, err := s.db.Exec(query, setArgs...); err != nil {
							log.Printf("[parse] update item error: %v", err)
						}
						updated, err := getItemByID(s.db, existing.ID, session.UserID)
						if err == nil {
							resp.Updated = append(resp.Updated, updated)
						}
					}
					break
				}
			}
		}
	case "save":
		name := cmd.SaveName
		if name == "" {
			name = "Saved List"
		}
		storeName := strings.TrimSpace(cmd.StoreName)
		itemsToSave, err := collectListItems(s.db, session.UserID)
		if err == nil && len(itemsToSave) > 0 {
			tx, err := s.db.Begin()
			if err == nil {
				result, err := tx.Exec("INSERT INTO saved_lists (user_id, name, store_name) VALUES (?, ?, ?)", session.UserID, name, storeName)
				if err == nil {
					savedListID, _ := result.LastInsertId()
					insertSavedListItems(tx, savedListID, itemsToSave)
					tx.Commit()
				} else {
					tx.Rollback()
				}
			}
		}
	case "clear":
		resp.Action = "clear"
	case "restore":
		var stashJSON string
		if err := s.db.QueryRow("SELECT COALESCE(last_cleared_items,'[]') FROM users WHERE id = ?", session.UserID).Scan(&stashJSON); err == nil && stashJSON != "[]" {
			var stashed []ParsedItem
			if json.Unmarshal([]byte(stashJSON), &stashed) == nil && len(stashed) > 0 {
				added, _ := addParsedItems(s.db, session.UserID, stashed, "app")
				resp.Items = added
				resp.Added = len(added)
				s.db.Exec("UPDATE users SET last_cleared_items = '[]' WHERE id = ?", session.UserID)
				resp.Message = cmd.Message
			} else {
				resp.Message = "I don't have anything to restore right now."
			}
		} else {
			resp.Message = "I don't have anything to restore right now."
		}
	case "share":
		resp.Action = "share"
		resp.Message = cmd.Message
	case "load_saved_list":
		s.executeLoadSavedList(cmd, session, hhIDs, &resp)
	case "delete_saved_list":
		s.executeDeleteSavedList(cmd, session, hhIDs, &resp)
	case "done_shopping":
		resp.Action = "done_shopping"
		resp.Message = cmd.Message
		if resp.Message == "" {
			resp.Message = "Nice trip! Want to snap a photo of your receipt? I'll check off what you got."
		}
	case "search":
		resp.Action = "search"
		resp.Message = cmd.Message
		if len(cmd.Items) > 0 {
			added, skippedItems := addParsedItems(s.db, session.UserID, cmd.Items, "app")
			resp.Items = added
			resp.Skipped = skippedItems
			resp.Added = len(added)
		}
	case "update_preference":
		for _, item := range cmd.Items {
			if item.Brand != "" && item.Name != "" {
				category := strings.ToLower(strings.TrimSpace(item.Name))
				recordBrandChoice(s.db, session.UserID, category, item.Brand, "command")
				s.db.Exec("UPDATE brand_preferences SET streak = 3 WHERE user_id = ? AND product_category = ?",
					session.UserID, category)
				updateArgs := append(append([]interface{}{item.Brand}, whereArgs...), "%"+category+"%")
				s.db.Exec("UPDATE list_items SET brand = ? WHERE "+whereClause+" AND checked = 0 AND LOWER(name) LIKE ?",
					updateArgs...)
			}
		}
		resp.Message = cmd.Message
	default:
		// Default "add" action
		added, skippedItems := addParsedItems(s.db, session.UserID, cmd.Items, "app")
		resp.Items = added
		resp.Skipped = skippedItems
		resp.Added = len(added)
		s.enrichAddedItems(session.UserID, added, cmd.Items, &resp)
		// Flag fallback items for async enrichment retry
		if needsEnrichment {
			for _, item := range added {
				s.db.Exec("UPDATE list_items SET needs_enrichment = 1 WHERE id = ?", item.ID)
			}
			go s.retryEnrichItems(session.UserID)
		}
	}

	if resp.Items == nil {
		resp.Items = []ListItem{}
	}
	if resp.Skipped == nil {
		resp.Skipped = []SkippedItem{}
	}
	writeJSON(w, http.StatusCreated, resp)
	go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// handleImageParse processes image-based item parsing requests.
func (s *Server) handleImageParse(w http.ResponseWriter, session Session, text string, images []string) {
	parsed, err := s.cachedParseItems(s.config.AnthropicAPIKey, text, images)
	if err != nil {
		log.Printf("[parse] error for user %d: %v", session.UserID, err)
		dbError(w, err, "We couldn't understand that right now. Please try again.")
		return
	}

	added, skippedItems := addParsedItems(s.db, session.UserID, parsed, "app")

	resp := ParseResponse{
		Items:   added,
		Skipped: skippedItems,
		Added:   len(added),
	}
	s.enrichAddedItems(session.UserID, added, parsed, &resp)

	if resp.Items == nil {
		resp.Items = []ListItem{}
	}
	if resp.Skipped == nil {
		resp.Skipped = []SkippedItem{}
	}

	// Record learning signals for added items (async)
	if len(added) > 0 {
		go func(uid int, items []ListItem) {
			for _, item := range items {
				s.recordSignal(uid, "item_added", item.Department, item.Brand, map[string]interface{}{
					"name": item.Name, "source": item.Source,
				})
			}
		}(session.UserID, added)
	}

	writeJSON(w, http.StatusCreated, resp)
	go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// enrichAddedItems resolves brand alternatives, checks swap deals, and checks household duplicates
// for newly added items. Shared by text-add and image-parse paths.
func (s *Server) enrichAddedItems(userID int, added []ListItem, parsedItems []ParsedItem, resp *ParseResponse) {
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
func (s *Server) executeLoadSavedList(cmd *ParsedCommand, session Session, hhIDs []int, resp *ParseResponse) {
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
func (s *Server) executeDeleteSavedList(cmd *ParsedCommand, session Session, hhIDs []int, resp *ParseResponse) {
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
func (s *Server) loadSavedListIntoActive(savedListID int, userID int) int {
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
func (s *Server) retryEnrichItems(userID int) {
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
