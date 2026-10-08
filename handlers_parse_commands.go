package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// isSimpleAddInput returns true if the text looks like plain grocery items to add
// (not a command like remove/check/save/search/recipe). This enables a fast path
// that skips the full command parser and its web search tool, cutting latency from
// 10-30s to 1-3s for simple adds.
func isSimpleAddInput(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}

	// Command keywords that require the full parser
	commandPrefixes := []string{
		"remove", "delete", "drop", "take off", "take out",
		"check", "uncheck", "mark", "got the", "we got",
		"save", "load", "open", "pull up", "what lists", "what saved",
		"clear", "undo", "restore", "bring back",
		"update", "change", "switch", "use ", "make the",
		"share", "invite",
		"done", "finished", "checkout", "at checkout",
		"recipe", "how to make", "how do i", "how do you",
		"what is", "what's", "is it", "should i", "tell me about",
		"help", "never mind", "cancel",
		"actually", "wait", "no ",
	}
	for _, prefix := range commandPrefixes {
		if strings.HasPrefix(lower, prefix) || strings.Contains(lower, prefix) {
			return false
		}
	}

	// Meal phrases need the full parser for recipe search
	if isMealPhrase(text) {
		return false
	}

	return true
}

// handleTextCommand processes natural language text commands via Claude parsing.
func (s *Server) handleTextCommand(w http.ResponseWriter, r *http.Request, session Session, text string, hhIDs []string) {
	start := time.Now()

	// --- FAST PATH: simple grocery items bypass the full command parser ---
	// This uses cachedParseItems (simpler prompt, no web search, cached results)
	// instead of parseCommandWithClaude (massive prompt, web search enabled).
	if isSimpleAddInput(text) {
		parsed, err := s.cachedParseItems(s.config.AnthropicAPIKey, text, nil)
		if err == nil && len(parsed) > 0 {
			added, skippedItems := addParsedItems(s.db, session.UserID, parsed, "app")
			resp := ParseResponse{
				Action:  "add",
				Items:   added,
				Skipped: skippedItems,
				Added:   len(added),
			}
			// Generate a friendly message
			if len(added) == 1 {
				resp.Message = fmt.Sprintf("Added %s!", added[0].Name)
			} else if len(added) > 1 {
				names := make([]string, len(added))
				for i, a := range added {
					names[i] = a.Name
				}
				if len(names) <= 3 {
					resp.Message = fmt.Sprintf("Added %s!", strings.Join(names, ", "))
				} else {
					resp.Message = fmt.Sprintf("Added %d items!", len(added))
				}
			}

			s.enrichAddedItems(session.UserID, added, parsed, &resp)

			// Record learning signals async
			if len(added) > 0 {
				go func(uid string, items []ListItem) {
					for _, item := range items {
						s.recordSignal(uid, "item_added", item.Department, item.Brand, map[string]interface{}{
							"name": item.Name, "source": item.Source,
						})
					}
				}(session.UserID, added)
			}

			if resp.Items == nil {
				resp.Items = []ListItem{}
			}
			if resp.Skipped == nil {
				resp.Skipped = []SkippedItem{}
			}
			log.Printf("[parse] fast-path add for user %s: %q → %d items in %v", session.UserID, text, len(added), time.Since(start))
			writeJSON(w, http.StatusCreated, resp)
			go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
			return
		}
		// Fast path failed — fall through to full parser
		log.Printf("[parse] fast-path failed for %q, falling back to full parser: %v", text, err)
	}

	// --- FULL PATH: commands, searches, updates, etc. ---
	// If the client requests async processing (has SSE connection), return 202
	// immediately and process in the background. Otherwise process synchronously.
	wantAsync := r.Header.Get("X-Async") == "1"
	if wantAsync {
		writeJSON(w, http.StatusAccepted, ParseResponse{
			Action:  "processing",
			Message: "Working on it...",
			Items:   []ListItem{},
			Skipped: []SkippedItem{},
		})
		go s.executeSlowPathCommand(session, text, hhIDs, start)
		return
	}

	// Synchronous path (tests, email webhook, clients without SSE)
	resp := s.buildSlowPathResponse(session, text, hhIDs, start)
	writeJSON(w, http.StatusCreated, resp)
	go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// executeSlowPathCommand runs the full Claude command parser in the background
// and pushes the result to the client via SSE.
func (s *Server) executeSlowPathCommand(session Session, text string, hhIDs []string, start time.Time) {
	resp := s.buildSlowPathResponse(session, text, hhIDs, start)
	s.broadcastCommandResult(session.UserID, resp)
	s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// buildSlowPathResponse runs the full Claude command parser and returns the response.
// Used by both sync (HTTP 201) and async (SSE push) paths.
func (s *Server) buildSlowPathResponse(session Session, text string, hhIDs []string, start time.Time) ParseResponse {
	// Get current list items so the LLM can reference them
	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID, hhIDs)
	rows, err := s.db.Query(
		"SELECT "+listItemColumns+" FROM "+listItemJoins+" WHERE li."+whereClause+" ORDER BY li.name",
		whereArgs...,
	)
	var currentItems []ListItem
	if err == nil {
		for rows.Next() {
			item, err := scanListItem(rows)
			if err == nil {
				currentItems = append(currentItems, item)
			}
		}
		rows.Close()
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
		log.Printf("[parse] command error for user %s: %v — falling back to raw add", session.UserID, err)
		if isMealPhrase(text) {
			// Don't add meal phrases as literal list items — give a helpful response instead
			cmd = &ParsedCommand{
				Action:  "search",
				Items:   []ParsedItem{},
				Message: "I'd love to help with that! Try telling me the specific ingredients you need — like 'chicken, rice, broccoli, soy sauce'. I'll add them to your list right away!",
			}
		} else {
			// Fallback: add the raw text as an item instead of showing an error
			cmd = &ParsedCommand{
				Action:  "add",
				Items:   []ParsedItem{{Name: text, Department: "Other", Quantity: 1}},
				Message: fmt.Sprintf("Added %s to your list!", text),
			}
			needsEnrichment = true
		}
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
				s.db.Exec("UPDATE list_items SET brand = ? WHERE "+whereClause+" AND checked = 0 AND name ILIKE ?",
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

		// Merge Claude-level skipped (ambiguous/unparseable) items into the response
		if len(cmd.Skipped) > 0 {
			for _, s := range cmd.Skipped {
				resp.Skipped = append(resp.Skipped, SkippedItem{
					Parsed:   ParsedItem{Name: s.Text},
					Reason:   s.Reason,
					RawInput: s.Text,
				})
			}
		}

		// Build partial success message when there are both added items AND skipped items
		if len(added) > 0 && len(cmd.Skipped) > 0 {
			skippedTexts := make([]string, 0, len(cmd.Skipped))
			for _, s := range cmd.Skipped {
				skippedTexts = append(skippedTexts, "'"+s.Text+"'")
			}
			partialMsg := fmt.Sprintf("Added %d item%s", len(added), pluralS(len(added)))
			partialMsg += fmt.Sprintf(". I wasn't sure about: %s — try being more specific!", strings.Join(skippedTexts, ", "))
			// Only override if Claude didn't already provide a message mentioning the skipped items
			if resp.Message == "" || !strings.Contains(strings.ToLower(resp.Message), "sure about") {
				resp.Message = partialMsg
			}
		}

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
	if resp.Items == nil {
		resp.Items = []ListItem{}
	}
	if resp.Skipped == nil {
		resp.Skipped = []SkippedItem{}
	}
	log.Printf("[parse] full-path for user %s: %q → action=%s in %v", session.UserID, text, resp.Action, time.Since(start))
	return resp
}

// handleImageParse processes image-based item parsing requests.
func (s *Server) handleImageParse(w http.ResponseWriter, session Session, text string, images []string) {
	parsed, err := s.cachedParseItems(s.config.AnthropicAPIKey, text, images)
	if err != nil {
		log.Printf("[parse] error for user %s: %v", session.UserID, err)
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
		go func(uid string, items []ListItem) {
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

