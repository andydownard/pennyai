package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
		var discountCents, budgetTotal, budgetSpent, quantityOffered, maxRebateCents int
		err = tx.QueryRow(
			`SELECT offer_brand, offer_product, discount_cents, total_budget_cents, budget_spent_cents, status,
			        deal_type, quantity_offered, trial_product_name, trial_size, max_rebate_cents
			 FROM brand_deals WHERE id = ?`,
			req.SwapDealID,
		).Scan(&offerBrand, &offerProduct, &discountCents, &budgetTotal, &budgetSpent, &status,
			&dealType, &quantityOffered, &trialProductName, &trialSize, &maxRebateCents)
		if err != nil || status != "active" {
			writeBadRequest(w, "Deal no longer available")
			return
		}
		if budgetTotal > 0 && budgetSpent >= budgetTotal {
			writeBadRequest(w, "Deal budget exhausted")
			return
		}

		// For fully_free deals: NO budget deduction on accept — debit happens on receipt verification
		isFullyFree := dealType == "fully_free"
		if !isFullyFree {
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
			case "fully_free":
				// For fully_free: add the free product to the list (like trial)
				freeResult, freeErr := tx.Exec(
					"INSERT INTO list_items (user_id, name, brand, quantity, department, source, swap_savings_cents) VALUES (?, ?, ?, 1, 'Other', 'swap', ?)",
					session.UserID, offerProduct, offerBrand, maxRebateCents,
				)
				if freeErr != nil {
					log.Printf("[swaps] fully_free accept insert error: %v", freeErr)
				}
				_ = freeResult
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
			if dealType != "trial" && dealType != "fully_free" {
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

		// Insert rebate ledger entry — fully_free uses 'pending_purchase' status (budget debit deferred to verification)
		rebateStatus := "pending"
		rebateAmount := discountCents
		if isFullyFree {
			rebateStatus = "pending_purchase"
			rebateAmount = maxRebateCents // will be adjusted on receipt verification
		}
		if _, err := tx.Exec(
			"INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES (?, ?, ?, ?, ?)",
			session.UserID, swapEventID, req.SwapDealID, rebateAmount, rebateStatus,
		); err != nil {
			log.Printf("[swaps] failed to insert rebate: %v", err)
		}

		// Update user counters — for fully_free, don't credit until verified
		if isFullyFree {
			if _, err := tx.Exec("UPDATE users SET deal_accept_count = deal_accept_count + 1 WHERE id = ?",
				session.UserID); err != nil {
				log.Printf("[swaps] failed to update user accept count for user %s: %v", session.UserID, err)
			}
		} else {
			if _, err := tx.Exec("UPDATE users SET deal_accept_count = deal_accept_count + 1, swap_credits_cents = swap_credits_cents + ? WHERE id = ?",
				discountCents, session.UserID); err != nil {
				log.Printf("[swaps] failed to update user credits for user %s: %v", session.UserID, err)
			}
		}

		// Auto-pause deal if budget now exhausted (not applicable for fully_free since budget debit is deferred)
		if !isFullyFree {
			var newBudgetSpent int
			tx.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", req.SwapDealID).Scan(&newBudgetSpent)
			if budgetTotal > 0 && newBudgetSpent >= budgetTotal {
				if _, err := tx.Exec("UPDATE brand_deals SET status = 'paused' WHERE id = ?", req.SwapDealID); err != nil {
					log.Printf("[swaps] failed to auto-pause deal %d: %v", req.SwapDealID, err)
				}
				log.Printf("[swaps] deal %d auto-paused: budget exhausted", req.SwapDealID)
			}
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
		case "fully_free":
			resp.Message = fmt.Sprintf("It's FREE! Add %s %s to your cart — scan your receipt after purchase to get $%.2f back.", offerBrand, offerProduct, float64(maxRebateCents)/100)
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
			log.Printf("[receipt-fraud] Image dimension flags for user %s: %v", session.UserID, flags)
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
			log.Printf("[receipt-fraud] BLOCKED receipt from user %s — AI forgery detected", session.UserID)
			writeJSON(w, http.StatusOK, ParseResponse{
				Items:   []ListItem{},
				Skipped: []SkippedItem{},
				Action:  "receipt_rejected",
				Message: "Hmm, I couldn't verify this receipt. Please try scanning a clear photo of a physical receipt.",
			})
			return
		}
		log.Printf("[receipt] parse error for user %s: %v", session.UserID, err)
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
