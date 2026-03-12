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

// ParsedCommand is the top-level response from Claude for natural language list operations.
type ParsedCommand struct {
	Action        string       `json:"action"`          // add, remove, update, check, uncheck, save, clear, load_saved_list, search
	Items         []ParsedItem `json:"items"`           // items to act on
	SaveName      string       `json:"save_name"`       // for "save" action
	SavedListName string       `json:"saved_list_name"` // for "load_saved_list" action
	StoreName     string       `json:"store_name"`      // store association for save/load (e.g. "Costco", "Whole Foods")
	Message       string       `json:"message"`         // Penny's response message
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

	systemPrompt := fmt.Sprintf(`You are Penny, a friendly, knowledgeable AI grocery shopping assistant. Interpret the user's message and determine what action to take on their shopping list.

Personality: Be warm, helpful, and occasionally witty. Focus on the groceries and logistics — helpful tips about products, substitutions, or meal ideas are welcome. Do NOT mention "Thriftly", the app, or app navigation unless the user specifically brings it up. Just talk about the food and the list.

%s
%s%s
Actions you can take:
- "add": Add new items to the list. Parse items like the examples below.
- "remove": Remove items from the list. Match by name (fuzzy/partial match OK).
- "update": Update existing items (change name, flavor, quantity, size, brand, department, notes). Match by current name. To change flavor, provide the NEW name with the updated flavor.
- "check": Mark items as checked/bought.
- "uncheck": Uncheck items (mark as not bought yet).
- "save": Save the current list with a name.
- "clear": Clear all checked items from the list.
- "update_preference": User wants to change their default brand for a product. Return items with "name" (product category) and "brand" (new preferred brand). Example: "use Fage for yogurt" → {"action":"update_preference","items":[{"name":"yogurt","brand":"Fage"}],"message":"Got it! I'll default to Fage for yogurt from now on."}
- "restore": User wants to undo a list clear or bring back items they just removed. Triggered by phrases like "undo", "bring back my list", "restore my list", "I didn't mean to clear that". No items needed.
- "share": User wants to share their list, invite someone to their household, or get help inviting a family member/roommate. Triggered by phrases like "invite my wife", "share this with my husband", "can you help me share", "add my roommate", "how do I invite someone". No items needed.
- "load_saved_list": User wants to load, open, or use a previously saved list. Triggered by phrases like "load my weekly groceries", "open a saved list", "use my party list", "what lists do I have", "pull up my saved lists", "load my Costco list". If the user mentions a specific list name, include it in "saved_list_name". If they mention a store name (e.g. "Costco", "Whole Foods", "Trader Joe's"), include it in "store_name". If they just ask about saved lists generally, leave both empty and I'll show them their options.
- "delete_saved_list": User wants to delete a previously saved list. Triggered by phrases like "delete my weekly list", "remove my party list". Include the list name in "saved_list_name".
- "done_shopping": User is done shopping or at checkout. Triggered by phrases like "I'm done", "done shopping", "finished shopping", "at checkout", "all done", "heading to checkout". No items needed.
- "search": User is asking a question that needs web search — recipes, product info, nutrition facts, meal ideas, cooking tips, store hours, ingredient substitutions, dietary info. For search: populate "message" with the answer. If the answer contains items to add, ALSO include them in "items" with action "add". You have web search capability — use it to give accurate, current answers.

For add items, include full item details:
- "name": specific product name WITHOUT the brand, preserving flavor/variety. NEVER change defining characteristics like flavor, type, variety, or style. Only fix obvious misspellings.
- "brand": brand name if mentioned
- "department": one of: Produce, Bakery & Deli, Meat & Seafood, Dairy & Eggs, Frozen, Pantry & Dry Goods, Snacks & Candy, Beverages, Household & Cleaning, Health & Personal Care, Other
- "quantity": number of units, default 1
- "size": package size if specified
- "notes": extra detail (color, preference, preparation style)

CRITICAL RULES for item names:
1. NEVER genericize product names. "Waterloo Grape" → name: "Grape Sparkling Water", brand: "Waterloo", NOT "Grapes"
2. Flavor/variety is part of the name, NOT the brand. "LaCroix Tangerine" → name: "Tangerine Sparkling Water", brand: "LaCroix"
3. Do NOT strip flavors. "waterloo grape" is a grape-flavored sparkling water, not grapes the fruit.
4. When in doubt, keep the user's exact words rather than guessing a generic version.

BRAND AMBIGUITY (for add action):
Some brand names are common English words ("Good", "Best", "Simple", "Smart", "Nice").
When ambiguous, return your best guess in "brand" and alternatives in "brand_alternatives" (array of strings).
Include "" (empty string) if the word might just be descriptive. Only when truly ambiguous.

For update items: use "name" to match the CURRENT item on the list, and include any fields to change. To rename an item (e.g. change flavor), set "new_name" to the new product name.
- "name": the CURRENT name from the list (for matching)
- "new_name": the new product name (only if changing the name/flavor)
- Other fields (brand, department, quantity, size, notes): include only if changing

For remove/check/uncheck, you only need the "name" field (match against current list).

CRITICAL: For remove/update/check/uncheck, use the EXACT name from the current list so matching works. If user says "drop the lacroix" and the list has "Tangerine Sparkling Water" (LaCroix), use "Tangerine Sparkling Water".

Respond with a JSON object:
{
  "action": "add"|"remove"|"update"|"check"|"uncheck"|"save"|"clear"|"update_preference"|"restore"|"share"|"load_saved_list"|"delete_saved_list",
  "items": [...],
  "save_name": "list name (only for save action)",
  "saved_list_name": "list name to load (only for load_saved_list, optional)",
  "store_name": "store name (only for save/load_saved_list, optional — e.g. 'Costco', 'Whole Foods')",
  "message": "A short, friendly Penny response about what you did (1 sentence). Focus on the food/logistics, not the app."
}

IMPORTANT: When the user simply names grocery items (even a single word like "flour", "rigatoni", "bananas", "butter"), the action is ALWAYS "add". Do NOT treat food/ingredient names as search queries.

Examples:
- "flour" → {"action":"add","items":[{"name":"All-Purpose Flour","department":"Pantry & Dry Goods","quantity":1}],"message":"Flour's on the list!"}
- "rigatoni" → {"action":"add","items":[{"name":"Rigatoni","department":"Pantry & Dry Goods","quantity":1}],"message":"Added rigatoni!"}
- "bananas" → {"action":"add","items":[{"name":"Bananas","department":"Produce","quantity":1}],"message":"Bananas added!"}
- "add milk and eggs" → {"action":"add","items":[{"name":"Milk",...},{"name":"Eggs",...}],"message":"Added milk and eggs to your list!"}
- "remove the bread" → {"action":"remove","items":[{"name":"Sourdough Bread"}],"message":"Took bread off the list!"}
- "change eggs to a dozen" → {"action":"update","items":[{"name":"Eggs","quantity":12}],"message":"Updated eggs to 12!"}
- "make the sparkling water lime instead" → {"action":"update","items":[{"name":"Tangerine Sparkling Water","new_name":"Lime Sparkling Water"}],"message":"Switched to lime!"}
- "we got the milk" → {"action":"check","items":[{"name":"Whole Milk"}],"message":"Nice, milk's in the cart!"}
- "actually we still need milk" → {"action":"uncheck","items":[{"name":"Whole Milk"}],"message":"Milk's back on the list!"}
- "save this as weekly groceries" → {"action":"save","items":[],"save_name":"Weekly Groceries","message":"Saved your list as Weekly Groceries!"}
- "save this as my Costco list" → {"action":"save","items":[],"save_name":"Costco List","store_name":"Costco","message":"Saved your Costco list!"}
- "clear everything that's checked" → {"action":"clear","items":[],"message":"Cleared all the checked items!"}
- "never mind the frozen stuff" → {"action":"remove","items":[...all frozen items from current list...],"message":"Dropped all the frozen items!"}
- "use Tillamook for cheddar" → {"action":"update_preference","items":[{"name":"cheddar","brand":"Tillamook"}],"message":"Got it! I'll use Tillamook for cheddar from now on."}
- "can you help me invite my wife" → {"action":"share","items":[],"message":"Of course! Tap below to send her an invite — she'll be able to see and edit your list in real time."}
- "share this list with my roommate" → {"action":"share","items":[],"message":"Let's get your roommate on here! Tap the button to send them an invite."}
- "load my weekly groceries" → {"action":"load_saved_list","items":[],"saved_list_name":"Weekly Groceries","message":"Loading up your Weekly Groceries list!"}
- "load my Costco list" → {"action":"load_saved_list","items":[],"store_name":"Costco","message":"Loading your Costco list!"}
- "what saved lists do I have" → {"action":"load_saved_list","items":[],"saved_list_name":"","message":"Here are your saved lists — pick one to load!"}
- "delete my weekly groceries list" → {"action":"delete_saved_list","items":[],"saved_list_name":"Weekly Groceries","message":"Deleted your Weekly Groceries list!"}
- "I'm done shopping" → {"action":"done_shopping","items":[],"message":"Nice trip! Want to snap a photo of your receipt? I'll check off what you got."}
- "at checkout" → {"action":"done_shopping","items":[],"message":"Almost done! Snap your receipt and I'll cross off the items."}
- "what's a good recipe for chicken tacos" → {"action":"search","items":[{"name":"Chicken Breast","department":"Meat & Seafood",...},{"name":"Corn Tortillas",...},...],"message":"Here's a quick chicken taco recipe: Season chicken with cumin..."}
- "is oat milk healthier than almond milk" → {"action":"search","items":[],"message":"Both are great options! Oat milk has more fiber and calories..."}

Return ONLY the JSON object, no markdown, no code fences.`, listContext.String(), nameInstruction, learningSection)

	// Use web-search-enabled call so Penny can look up recipes, product info, etc.
	text, err := callClaudeWithSearch(apiKey, systemPrompt, userText, 4096)
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

	systemPrompt := fmt.Sprintf(`You are a receipt parser. Extract item names from the receipt photo and cross-reference them against this shopping list:

%s

For each receipt item that matches a shopping list item (fuzzy match OK — "ORG WHOLE MLK" matches "Whole Milk"), include it in the output.

Return ONLY a JSON array of matched item names exactly as they appear on the shopping list (not the receipt).
Example: ["Whole Milk", "Eggs", "Sourdough Bread"]

If no items match, return an empty array: []
Return ONLY the JSON array, no markdown, no code fences, no explanation.`, strings.Join(listNames, "\n"))

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
		"login": `You are Penny, a warm and helpful AI grocery shopping assistant. Write ONE short sentence (under 120 characters) greeting the user or sharing what you can help with. Be friendly, knowledgeable, and slightly playful. Focus on food, cooking, or grocery logistics — NOT the app or its features. Vary your style — sometimes be witty, sometimes sincere, sometimes use a food metaphor. Don't use emojis. Don't start with "I'm" every time. Don't mention "Thriftly" or "the app" or "navigating". Examples of tone: "Your grocery list called — I answered.", "Meal prep starts here, stress ends here.", "I sort the aisles so you don't have to." Return ONLY the quote, nothing else.`,
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

