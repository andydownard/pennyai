package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	mrand "math/rand/v2"
	"net/http"
	"strings"
	"time"
)

// =============================================================================
// ANTHROPIC CLIENT
//
// Calls Claude to parse free-form grocery text into structured items.
// We make raw HTTP calls (no SDK) to keep dependencies minimal.
// =============================================================================

// claudeHTTPClient is used for all Anthropic API calls — 45s timeout prevents
// hung goroutines from accumulating if the API is slow or unresponsive.
// Image parsing can be slower, so 45s gives headroom over 30s.
var claudeHTTPClient = &http.Client{Timeout: 45 * time.Second}
var twilioHTTPClient = &http.Client{Timeout: 10 * time.Second}

// parseCacheEntry caches LLM parse results for common text-only inputs.
type parseCacheEntry struct {
	items     []ParsedItem
	createdAt time.Time
}

const parseCacheTTL = 24 * time.Hour
const parseCacheMaxSize = 1000

// cachedParseItems wraps parseItems with an in-memory cache for text-only
// requests (no images). Common inputs like "milk" or "eggs" hit the cache
// instead of calling the LLM, saving API costs.
func (s *Server) cachedParseItems(apiKey, text string, images []string) ([]ParsedItem, error) {
	// Only cache text-only, short inputs (likely single/simple items)
	if len(images) > 0 || len(text) > 100 {
		return s.parseItems(apiKey, text, images)
	}

	key := strings.ToLower(strings.TrimSpace(text))
	if key == "" {
		return s.parseItems(apiKey, text, images)
	}

	// Check cache
	if cached, ok := s.parseCache.Load(key); ok {
		entry := cached.(*parseCacheEntry)
		if time.Since(entry.createdAt) < parseCacheTTL {
			// Return a copy so callers can't mutate the cached slice
			result := make([]ParsedItem, len(entry.items))
			copy(result, entry.items)
			return result, nil
		}
		s.parseCache.Delete(key)
	}

	// Cache miss — call LLM
	items, err := s.parseItems(apiKey, text, images)
	if err != nil {
		return nil, err
	}

	// Store in cache
	s.parseCache.Store(key, &parseCacheEntry{items: items, createdAt: time.Now()})

	return items, nil
}

// stripCodeFence removes a leading ```[lang] fence and trailing ``` from text
// that Claude sometimes wraps its JSON output in despite being told not to.
func stripCodeFence(text string) string {
	if !strings.HasPrefix(text, "```") {
		return text
	}
	if i := strings.Index(text, "\n"); i >= 0 {
		text = text[i+1:]
	}
	if i := strings.LastIndex(text, "```"); i >= 0 {
		text = text[:i]
	}
	return strings.TrimSpace(text)
}

// ParsedItem is what Claude returns for each grocery item.
type ParsedItem struct {
	Name              string   `json:"name"`
	NewName           string   `json:"new_name"`
	Brand             string   `json:"brand"`
	BrandAlternatives []string `json:"brand_alternatives,omitempty"`
	Department        string   `json:"department"`
	Quantity          int      `json:"quantity"`
	Size              string   `json:"size"`
	Notes             string   `json:"notes"`
	Emoji             string   `json:"emoji"`
}

// SkippedInput represents an item from the user's input that couldn't be parsed into a grocery item.
type SkippedInput struct {
	Text   string `json:"text"`   // the original phrase that couldn't be parsed
	Reason string `json:"reason"` // why it was skipped (e.g. "ambiguous", "not_a_grocery_item", "needs_clarification")
}

// ParsedCommand is the top-level response from Claude for natural language list operations.
type ParsedCommand struct {
	Action        string         `json:"action"`                   // add, remove, update, check, uncheck, save, clear, load_saved_list, search
	Items         []ParsedItem   `json:"items"`                    // items to act on
	Skipped       []SkippedInput `json:"skipped,omitempty"`        // items that couldn't be parsed
	SaveName      string         `json:"save_name"`                // for "save" action
	SavedListName string         `json:"saved_list_name"`          // for "load_saved_list" action
	StoreName     string         `json:"store_name"`               // store association for save/load (e.g. "Costco", "Whole Foods")
	Message       string         `json:"message"`                  // Penny's response message
}

// callClaude is the shared helper for all Anthropic API calls.
func callClaude(apiKey, system string, content interface{}, maxTokens int) (string, error) {
	reqBody := map[string]interface{}{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": maxTokens,
		"messages": []map[string]interface{}{
			{"role": "user", "content": content},
		},
	}
	if system != "" {
		reqBody["system"] = system
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}
	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := claudeHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Anthropic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic returned %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}
	return strings.TrimSpace(apiResp.Content[0].Text), nil
}

// callClaudeWithSearch is like callClaude but includes the web search tool.
// Claude can search the web to answer questions about recipes, products, nutrition, prices, etc.
func callClaudeWithSearch(apiKey, system string, content interface{}, maxTokens int) (string, error) {
	reqBody := map[string]interface{}{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": maxTokens,
		"messages": []map[string]interface{}{
			{"role": "user", "content": content},
		},
		"tools": []map[string]interface{}{
			{"type": "web_search_20250305", "name": "web_search", "max_uses": 3},
		},
	}
	if system != "" {
		reqBody["system"] = system
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}
	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := claudeHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Anthropic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// If web search fails (unsupported, billing, etc.), fall back to regular call
		log.Printf("[penny] web search call failed (%d), falling back: %s", resp.StatusCode, string(body))
		return callClaude(apiKey, system, content, maxTokens)
	}

	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	// Extract text blocks (skip tool_use and web_search_tool_result blocks)
	var texts []string
	for _, block := range apiResp.Content {
		if block.Type == "text" && block.Text != "" {
			texts = append(texts, block.Text)
		}
	}
	if len(texts) == 0 {
		return "", fmt.Errorf("no text content in search response")
	}
	return strings.TrimSpace(strings.Join(texts, "\n")), nil
}

// parseItemsWithClaude sends the user's raw text (and optional image) to Claude
// and gets back a list of grocery items with departments assigned.
func parseItemsWithClaude(apiKey, userText string, images []string) ([]ParsedItem, error) {
	systemPrompt := `You are a grocery shopping assistant. Parse the user's input into individual grocery items.

For each item return:
- "name": the specific product name, properly capitalized, WITHOUT the brand. PRESERVE flavor, variety, and variant info in the name. Examples: "Tangerine Sparkling Water" not just "Sparkling Water", "Grape Sparkling Water" not "Grapes".
- "brand": brand name if mentioned, otherwise "" (e.g. "Jif", "LaCroix", "Waterloo", "Organic Valley")
- "department": one of: Produce, Bakery & Deli, Meat & Seafood, Dairy & Eggs, Frozen, Pantry & Dry Goods, Snacks & Candy, Beverages, Household & Cleaning, Health & Personal Care, Other
- "quantity": number of units to buy, integer, default 1
- "size": package size/weight if specified, otherwise "" (e.g. "16 oz", "1 lb", "Large", "1 gal")
- "notes": any extra detail the user mentioned that doesn't fit above, otherwise "" (e.g. "the one in the blue box", "boneless skinless", "ripe ones")
- "emoji": a single emoji that best represents this specific item. Pick the most specific match (🍌 for bananas, 🥩 for steak, 🧀 for cheese). If no specific emoji fits, use a category emoji (🥬 for produce, 🥛 for dairy). Last resort: 🛒

CRITICAL RULES:
1. NEVER genericize product names. Keep the specific product the user asked for.
2. Flavor/variety is part of the name, NOT the brand. "LaCroix Tangerine" → name: "Tangerine Sparkling Water", brand: "LaCroix"
3. Do NOT rename products. "Waterloo Grape" → name: "Grape Sparkling Water", brand: "Waterloo", NOT "Grapes"
4. When in doubt, keep the user's exact words in the name rather than guessing a generic version.
5. NEVER change defining characteristics like flavor, type, variety, or style. Only fix obvious misspellings. "chcken thigs" → "Chicken Thighs", but "honey mustard" stays "Honey Mustard" not "Mustard".
6. If the user provides extra qualifiers (e.g. "organic", "gluten-free", "sugar-free"), keep them in the name. Put secondary details like color, preference, or preparation style in "notes".

Separate brand from product name. Examples:
- "jif pb" → {"name":"Peanut Butter","brand":"Jif","department":"Pantry & Dry Goods","quantity":1,"size":"","notes":"","emoji":"🥜"}
- "2 boxes kraft mac and cheese" → {"name":"Macaroni & Cheese","brand":"Kraft","department":"Pantry & Dry Goods","quantity":2,"size":"","notes":"","emoji":"🧀"}
- "large organic eggs" → {"name":"Organic Eggs","brand":"","department":"Dairy & Eggs","quantity":1,"size":"Large","notes":"","emoji":"🥚"}
- "gallon of whole milk" → {"name":"Whole Milk","brand":"","department":"Dairy & Eggs","quantity":1,"size":"1 gal","notes":"","emoji":"🥛"}
- "lacroix tangerine" → {"name":"Tangerine Sparkling Water","brand":"LaCroix","department":"Beverages","quantity":1,"size":"","notes":"","emoji":"🍊"}
- "waterloo grape" → {"name":"Grape Sparkling Water","brand":"Waterloo","department":"Beverages","quantity":1,"size":"","notes":"","emoji":"🍇"}
- "tillamook sharp cheddar block" → {"name":"Sharp Cheddar","brand":"Tillamook","department":"Dairy & Eggs","quantity":1,"size":"Block","notes":"","emoji":"🧀"}
- "diet coke 12 pack" → {"name":"Diet Coke","brand":"Coca-Cola","department":"Beverages","quantity":1,"size":"12 pack","notes":"","emoji":"🥤"}

Image handling — adapt to what you see:
- Grocery items or products: extract each item.
- Handwritten or printed shopping list: transcribe each item.
- Recipe (on paper, screen, or in text): extract the ingredients as grocery items.
- Fridge, pantry, or cupboard photo: use the user's text as context (e.g. "what am I missing", "what's running low") to suggest items that appear to be low or absent. If no text is provided, list items that appear to be running low.

IMPORTANT: Do NOT break down foods into their constituent ingredients unless the user explicitly asks (e.g. "what do I need to make lasagna") or a recipe image is provided. Treat compound foods as single items:
- "lasagna" → [{"name":"Lasagna",...}] NOT individual ingredients
- "chicken stir fry" → [{"name":"Chicken Stir Fry",...}]

BRAND AMBIGUITY:
Some brand names are common English words ("Good", "Best", "Simple", "Smart", "Nice").
When the user's input contains a word that BOTH could be a common adjective AND exactly
matches a known brand for that product category, prefer the brand interpretation.

If genuinely ambiguous, return your best guess in "brand" and alternatives in
"brand_alternatives" (array of strings). Include "" (empty string) as an option if
the word might just be descriptive. Only return brand_alternatives when truly ambiguous.
Do NOT return alternatives for unambiguous brands like "Kraft" or "LaCroix".

Examples:
- "good cottage cheese" → {"name":"Cottage Cheese","brand":"Good Culture","brand_alternatives":["Good & Gather",""],...}
- "kraft mac and cheese" → {"name":"Macaroni & Cheese","brand":"Kraft",...} (no alternatives)

Return ONLY a JSON array, no markdown, no code fences, no explanation.`

	// Build message content — either plain text or multi-block (images + text)
	var messageContent interface{}
	if len(images) > 0 {
		var contentBlocks []map[string]interface{}
		for _, img := range images {
			mediaType := "image/jpeg"
			b64Data := img
			if strings.HasPrefix(img, "data:") {
				if idx := strings.Index(img, ";base64,"); idx > 0 {
					mediaType = img[5:idx]
					b64Data = img[idx+8:]
				}
			}
			contentBlocks = append(contentBlocks, map[string]interface{}{
				"type": "image",
				"source": map[string]string{
					"type":       "base64",
					"media_type": mediaType,
					"data":       b64Data,
				},
			})
		}
		textContent := userText
		if textContent == "" {
			textContent = "Identify all grocery items in these images. If it's a fridge or pantry, list items that appear to be running low or missing."
		}
		contentBlocks = append(contentBlocks, map[string]interface{}{
			"type": "text",
			"text": textContent,
		})
		messageContent = contentBlocks
	} else {
		messageContent = userText
	}

	text, err := callClaude(apiKey, systemPrompt, messageContent, 4096)
	if err != nil {
		return nil, err
	}
	text = stripCodeFence(text)

	var items []ParsedItem
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		// Try to salvage partial JSON (e.g. truncated response)
		if idx := strings.LastIndex(text, "},"); idx > 0 {
			salvaged := text[:idx+1] + "]"
			if err2 := json.Unmarshal([]byte(salvaged), &items); err2 == nil {
				log.Printf("[parse] Salvaged %d items from truncated response", len(items))
				return items, nil
			}
		}
		return nil, fmt.Errorf("parsing Claude JSON: %w", err)
	}

	return items, nil
}

// looksLikeSearchQuery returns true when the input looks like a question, recipe request,
// or nutrition query that would benefit from web search. Used to conditionally enable
// the web search tool on Claude calls (which adds latency even when not used).
func looksLikeSearchQuery(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	searchPrefixes := []string{
		"recipe", "how to make", "how to cook", "how do i",
		"how do you", "what is", "what's", "what are",
		"is it", "should i", "tell me about", "where can i",
		"how much", "how many calories", "nutrition",
		"substitute for", "replacement for", "alternative to",
		"what goes with", "what pairs with",
	}
	for _, prefix := range searchPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	// Also match "recipe" or "calories" anywhere in the string
	searchKeywords := []string{"recipe", "calories", "nutrition facts", "ingredients for"}
	for _, kw := range searchKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// parseCommandWithClaude interprets the user's natural language input as a list command.
// It receives the current list items so it can match references like "remove the milk".
func parseCommandWithClaude(apiKey, userText string, currentItems []ListItem, userName, learningContext string) (*ParsedCommand, error) {
	// Build compact list context — name, brand, and notes for matching and updates
	var listContext strings.Builder
	listContext.WriteString("Current shopping list:\n")
	if len(currentItems) == 0 {
		listContext.WriteString("(empty)\n")
	} else {
		for _, item := range currentItems {
			line := fmt.Sprintf("#%d %s", item.ID, item.Name)
			if item.Brand != "" {
				line += " (" + item.Brand + ")"
			}
			if item.Notes != "" {
				line += " [notes: " + item.Notes + "]"
			}
			listContext.WriteString(line + "\n")
		}
	}
	// Note: saved lists context is injected by the caller (handleParseAndAdd) via a wrapper

	nameInstruction := ""
	if userName != "" {
		nameInstruction = fmt.Sprintf("\nThe user's name is %s. Occasionally (about 2%% of the time) address them by name in your message.", userName)
	}

	learningSection := ""
	if learningContext != "" {
		learningSection = "\n" + learningContext + "\n"
	}

	systemPrompt := fmt.Sprintf(`You are Penny, a friendly AI grocery assistant. Interpret the user's message and act on their shopping list. Be warm, brief, and food-focused.

%s
%s%s
Actions:
- "add": Add items. Include name (without brand, preserve flavor/variety), brand, department (Produce|Bakery & Deli|Meat & Seafood|Dairy & Eggs|Frozen|Pantry & Dry Goods|Snacks & Candy|Beverages|Household & Cleaning|Health & Personal Care|Other), quantity (default 1), size, notes.
- "remove": Remove items by name (fuzzy match OK).
- "update": Update items. "name" = current name for matching, "new_name" = renamed item, plus any changed fields.
- "check"/"uncheck": Mark items bought/not bought.
- "save": Save list. Include "save_name", optional "store_name".
- "clear": Clear checked items.
- "restore": Undo a clear or removal.
- "update_preference": Change default brand. Items: [{"name":"category","brand":"NewBrand"}].
- "share": User wants to invite someone to their household.
- "load_saved_list": Load a saved list. Include "saved_list_name" and/or "store_name" if mentioned.
- "delete_saved_list": Delete a saved list. Include "saved_list_name".
- "done_shopping": User is done/at checkout.
- "search": Questions needing web search (recipes, nutrition, product info). Include answer in "message" and any ingredients as "add" items.

CRITICAL: For remove/update/check/uncheck, use EXACT item names from the current list. Flavor/variety is part of name, NOT brand ("LaCroix Tangerine" → name:"Tangerine Sparkling Water", brand:"LaCroix"). Never genericize names. When ambiguous brand vs descriptor, use "brand_alternatives" array.

For ambiguous input, put clear items in "items" and vague phrases in "skipped" with reason. Never add meal phrases literally — break into ingredients or use "search".

JSON response: {"action":"...","items":[...],"skipped":[{"text":"...","reason":"..."}],"save_name":"...","saved_list_name":"...","store_name":"...","message":"1 sentence"}

Examples:
- "add milk and eggs" → {"action":"add","items":[{"name":"Milk","department":"Dairy & Eggs","quantity":1},{"name":"Eggs","department":"Dairy & Eggs","quantity":1}],"message":"Added milk and eggs!"}
- "remove the bread" → {"action":"remove","items":[{"name":"Sourdough Bread"}],"message":"Took bread off the list!"}
- "change eggs to a dozen" → {"action":"update","items":[{"name":"Eggs","quantity":12}],"message":"Updated eggs to 12!"}
- "we got the milk" → {"action":"check","items":[{"name":"Whole Milk"}],"message":"Nice, milk's in the cart!"}
- "save this as my Costco list" → {"action":"save","items":[],"save_name":"Costco List","store_name":"Costco","message":"Saved!"}
- "load my weekly groceries" → {"action":"load_saved_list","items":[],"saved_list_name":"Weekly Groceries","message":"Loading!"}
- "share with my wife" → {"action":"share","items":[],"message":"Tap below to send her an invite!"}
- "I'm done shopping" → {"action":"done_shopping","items":[],"message":"Nice trip! Snap your receipt and I'll check off what you got."}
- "recipe for chicken tacos" → {"action":"search","items":[{"name":"Chicken Breast","department":"Meat & Seafood","quantity":1}],"message":"Season chicken with cumin..."}

Return ONLY JSON, no markdown.`, listContext.String(), nameInstruction, learningSection)

	// Only enable web search for inputs that look like questions/searches.
	// Web search adds significant latency even when Claude doesn't use it.
	needsSearch := looksLikeSearchQuery(userText)
	maxTok := 1024
	var text string
	var err error
	if needsSearch {
		text, err = callClaudeWithSearch(apiKey, systemPrompt, userText, maxTok)
	} else {
		text, err = callClaude(apiKey, systemPrompt, userText, maxTok)
	}
	if err != nil {
		return nil, err
	}
	text = stripCodeFence(text)

	var cmd ParsedCommand
	if err := json.Unmarshal([]byte(text), &cmd); err != nil {
		// Fallback: if LLM returned an array instead of a command, treat as "add"
		var items []ParsedItem
		if err2 := json.Unmarshal([]byte(text), &items); err2 == nil {
			return &ParsedCommand{Action: "add", Items: items, Message: ""}, nil
		}
		return nil, fmt.Errorf("parsing Claude JSON: %w", err)
	}
	if cmd.Action == "" {
		cmd.Action = "add"
	}
	return &cmd, nil
}

// parseReceiptWithClaude extracts item names from a receipt photo and cross-references
// them against the user's current shopping list. Returns matched item names.
// Also performs AI forgery detection — returns empty results if the image is likely fake.
func parseReceiptWithClaude(apiKey string, images []string, currentItems []ListItem) ([]string, error) {
	// Build list context for cross-referencing
	var listNames []string
	for _, item := range currentItems {
		entry := item.Name
		if item.Brand != "" {
			entry += " (" + item.Brand + ")"
		}
		listNames = append(listNames, entry)
	}

	systemPrompt := fmt.Sprintf(`You are a receipt parser with fraud detection capabilities. You have TWO jobs:

**JOB 1: AUTHENTICITY CHECK**
Before extracting items, assess whether this is a genuine receipt photo or an AI-generated/forged image. Check for:

1. **Text quality**: Real receipt text has OCR-like imperfections — slightly uneven spacing, thermal printer artifacts, faded sections. AI-generated text is suspiciously clean and uniform.
2. **Math verification**: If visible, do line item prices actually add up to the subtotal/total? AI fakes often have math errors.
3. **Store plausibility**: Does the store name, address, phone number, and format match a real retailer? AI fakes often use plausible-sounding but non-existent stores.
4. **Layout consistency**: Real receipts have consistent formatting from one specific POS system. AI fakes mix formatting styles.
5. **Visual artifacts**: Look for AI hallmarks — text that warps unnaturally, repeating patterns, inconsistent shadows, blurred regions that should be sharp, or overly perfect paper texture.
6. **Metadata clues**: If the image appears to be a screenshot of generated text rather than a photo of paper, flag it.
7. **C2PA / AI watermarks**: If you detect any embedded AI provenance markers or watermarks (e.g. from DALL-E, Midjourney, Stable Diffusion, or other generators), flag immediately.

**JOB 2: ITEM EXTRACTION** (only if authentic)
Cross-reference receipt items against this shopping list:

%s

For each receipt item that matches a shopping list item (fuzzy match OK — "ORG WHOLE MLK" matches "Whole Milk"), include it.

**RESPONSE FORMAT** — Return ONLY valid JSON, no markdown, no code fences:
{
  "authentic": true,
  "confidence": 0.95,
  "flags": [],
  "matched_items": ["Whole Milk", "Eggs"]
}

If the receipt appears forged or AI-generated:
{
  "authentic": false,
  "confidence": 0.3,
  "flags": ["suspiciously clean text", "math doesn't add up", "store address not plausible"],
  "matched_items": []
}

- "authentic": boolean — true if you believe this is a real receipt photo
- "confidence": float 0-1 — how confident you are in authenticity (1 = definitely real)
- "flags": array of strings — any red flags found (empty array if none)
- "matched_items": array of matched item names exactly as they appear on the shopping list (empty if not authentic)

Return ONLY the JSON object, no explanation.`, strings.Join(listNames, "\n"))

	var contentBlocks []map[string]interface{}
	for _, img := range images {
		mediaType := "image/jpeg"
		b64Data := img
		if strings.HasPrefix(img, "data:") {
			if idx := strings.Index(img, ";base64,"); idx > 0 {
				mediaType = img[5:idx]
				b64Data = img[idx+8:]
			}
		}
		contentBlocks = append(contentBlocks, map[string]interface{}{
			"type": "image",
			"source": map[string]string{
				"type":       "base64",
				"media_type": mediaType,
				"data":       b64Data,
			},
		})
	}
	contentBlocks = append(contentBlocks, map[string]interface{}{
		"type": "text",
		"text": "Extract all items from this receipt and match them against my shopping list.",
	})

	text, err := callClaude(apiKey, systemPrompt, contentBlocks, 4096)
	if err != nil {
		return nil, err
	}
	text = stripCodeFence(text)

	// Try the new authenticity-aware response format first
	var authResp struct {
		Authentic    bool     `json:"authentic"`
		Confidence   float64  `json:"confidence"`
		Flags        []string `json:"flags"`
		MatchedItems []string `json:"matched_items"`
	}
	if err := json.Unmarshal([]byte(text), &authResp); err == nil && (authResp.MatchedItems != nil || len(authResp.Flags) > 0) {
		// Successfully parsed authenticity response
		if !authResp.Authentic {
			log.Printf("[receipt-fraud] REJECTED: confidence=%.2f flags=%v", authResp.Confidence, authResp.Flags)
			return nil, fmt.Errorf("receipt_forgery_detected")
		}
		if authResp.Confidence < 0.5 {
			log.Printf("[receipt-fraud] LOW CONFIDENCE: %.2f flags=%v — rejecting", authResp.Confidence, authResp.Flags)
			return nil, fmt.Errorf("receipt_forgery_detected")
		}
		if len(authResp.Flags) > 0 {
			log.Printf("[receipt-fraud] PASSED with flags: confidence=%.2f flags=%v", authResp.Confidence, authResp.Flags)
		}
		if authResp.MatchedItems == nil {
			return []string{}, nil
		}
		return authResp.MatchedItems, nil
	}

	// Fallback: try legacy plain array format
	var matched []string
	if err := json.Unmarshal([]byte(text), &matched); err != nil {
		log.Printf("[receipt] failed to parse Claude response as JSON: %v (text: %s)", err, text)
		return []string{}, nil
	}
	return matched, nil
}

// =============================================================================
// PENNY QUOTE GENERATOR
// =============================================================================

// Fallback quotes used when the LLM isn't called.
var pennyFallbackQuotes = map[string][]string{
	"login": {
		"I work hard so that shopping is effortless and affordable for your household.",
		"I coordinate with your whole household so nothing gets forgotten and nothing gets doubled.",
		"I make sure you get the right amount of the right things — no more, no less.",
		"I find the best deals so you can spend less time comparing and more time living.",
		"I keep your list organized by aisle so you can breeze through the store.",
		"Text me anytime — I'll handle it.",
		"You can text or email me — I'll add it to your list!",
	},
	"chime": {
		"Looking good!", "Almost there!", "Nice work!", "On a roll!", "Keep going!",
	},
	"done": {
		"You're all done!", "List complete — nice job!", "Everything's checked off!",
	},
}

// generatePennyQuote calls Claude Haiku to produce a fresh Penny message.
func generatePennyQuote(apiKey, context, userName string) (string, error) {
	nameHint := ""
	if userName != "" {
		nameHint = fmt.Sprintf(" The user's name is %s — about 2%% of the time, address them by name.", userName)
	}
	prompts := map[string]string{
		"login": `You are Penny, a warm and helpful AI grocery shopping assistant. Write ONE short sentence (under 120 characters) greeting the user or sharing what you can help with. Be friendly, knowledgeable, and slightly playful. Focus on food, cooking, or grocery logistics — NOT the app or its features. Vary your style — sometimes be witty, sometimes sincere, sometimes use a food metaphor. Don't use emojis. Don't start with "I'm" every time. Don't mention "Groflo" or "the app" or "navigating". Examples of tone: "Your grocery list called — I answered.", "Meal prep starts here, stress ends here.", "I sort the aisles so you don't have to." Return ONLY the quote, nothing else.`,
		"chime": `You are Penny, a wry AI grocery assistant. Write ONE very short phrase (2-5 words) for someone checking items off their shopping list. Be dry, witty, or use a food pun. Never be flattering or sycophantic. Don't use emojis. Examples: "Cart goals.", "Aisle be darned.", "Smooth shopping.", "Check." Return ONLY the phrase, nothing else.`,
		"done":  `You are Penny, a cheerful AI grocery assistant. Write ONE short congratulations (under 60 characters) for someone who just finished their entire shopping list. Be warm and playful. Don't use emojis. Examples: "That's a wrap — pantry's gonna be stoked!", "Mission accomplished, chef!" Return ONLY the phrase, nothing else.`,
	}
	prompt, ok := prompts[context]
	if !ok {
		prompt = prompts["chime"]
	}
	if nameHint != "" {
		prompt += nameHint
	}

	text, err := callClaude(apiKey, "", prompt, 100)
	if err != nil {
		return "", err
	}
	// Strip any surrounding quotes the LLM might add
	quote := strings.Trim(text, "\"'\u201c\u201d\u2018\u2019")
	return quote, nil
}

// handlePennyQuote handles GET /api/penny/quote?context=login|chime|done
// Returns a Penny quote. ~25% of the time generates a fresh one via LLM;
// otherwise picks from fallback + cached quotes.
func (s *Server) handlePennyQuote(w http.ResponseWriter, r *http.Request) {
	ctx := r.URL.Query().Get("context")
	if ctx == "" {
		ctx = "login"
	}
	userName := r.URL.Query().Get("name")
	// Sanitize: limit length, strip non-alpha to prevent prompt injection
	if len(userName) > 30 {
		userName = userName[:30]
	}
	userName = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == ' ' || r == '-' || r == '\'' {
			return r
		}
		return -1
	}, userName)
	userName = strings.TrimSpace(userName)

	// Login context doesn't require auth (user isn't logged in yet)
	// Other contexts require a valid session to prevent API credit abuse
	if ctx != "login" {
		if _, ok := s.requireSession(w, r); !ok {
			return
		}
	}

	// Build the pool: fallback + any cached LLM quotes
	s.pennyQuoteMu.Lock()
	if s.pennyQuotes == nil {
		s.pennyQuotes = make(map[string][]string)
	}
	cached := s.pennyQuotes[ctx]
	s.pennyQuoteMu.Unlock()

	pool := append([]string{}, pennyFallbackQuotes[ctx]...)
	pool = append(pool, cached...)

	// ~25% chance: generate a fresh quote via LLM, rate-limited to 1 call per 10s
	s.pennyQuoteMu.Lock()
	canCallLLM := time.Since(s.pennyQuoteLast) > pennyQuoteRateLimit
	if canCallLLM && mrand.Float64() < 0.25 && s.config.AnthropicAPIKey != "" {
		s.pennyQuoteLast = time.Now() // claim the slot before unlocking
	} else {
		canCallLLM = false
	}
	s.pennyQuoteMu.Unlock()
	if canCallLLM {
		quote, err := generatePennyQuote(s.config.AnthropicAPIKey, ctx, userName)
		if err != nil {
			log.Printf("[penny-quote] LLM error: %v", err)
		} else if quote != "" {
			// Cache it (cap at 20 per context to avoid unbounded growth)
			s.pennyQuoteMu.Lock()
			if len(s.pennyQuotes[ctx]) < 20 {
				s.pennyQuotes[ctx] = append(s.pennyQuotes[ctx], quote)
			}
			s.pennyQuoteMu.Unlock()
			writeJSON(w, http.StatusOK, map[string]string{"quote": quote, "source": "llm"})
			return
		}
	}

	// Pick random from pool
	if len(pool) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"quote": "Hey there!", "source": "fallback"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"quote": pool[mrand.IntN(len(pool))], "source": "cached"})
}

