package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// wattHTTPClient has a 15-second timeout for Watt Data API calls.
var wattHTTPClient = &http.Client{Timeout: 15 * time.Second}

// --- Watt Data MCP request/response types ---

type wattMCPRequest struct {
	Method string          `json:"method"`
	Params wattToolRequest `json:"params"`
}

type wattToolRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type wattMCPResponse struct {
	Content []wattContent `json:"content"`
	IsError bool          `json:"isError"`
}

type wattContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// --- Admin endpoint: search for CPG brand managers ---

// handleWattBrandSearch handles POST /api/admin/watt/brand-search
// Finds entities at CPG companies for outreach using Watt Data's entity_find + entity_enrich.
func (s *Server) handleWattBrandSearch(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.config.WattAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Watt Data API not configured"})
		return
	}

	var req struct {
		Expression     string   `json:"expression"`      // Boolean trait expression (e.g., "T12345 AND T67890")
		EntityType     string   `json:"entity_type"`      // "person" or "business" (default: "person")
		Domains        []string `json:"domains"`          // Enrichment domains (default: name, email, employment)
		Limit          int      `json:"limit"`            // Max results (default: 100)
		IdentifierType string   `json:"identifier_type"`  // Identifier to return (default: "email")
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Expression == "" {
		writeBadRequest(w, "expression is required — use trait IDs from trait_search")
		return
	}
	if req.EntityType == "" {
		req.EntityType = "person"
	}
	if req.Limit <= 0 || req.Limit > 1000 {
		req.Limit = 100
	}
	if req.IdentifierType == "" {
		req.IdentifierType = "email"
	}
	if len(req.Domains) == 0 {
		req.Domains = []string{"name", "email", "employment"}
	}

	// Step 1: entity_find — get matching entity IDs
	findArgs := map[string]interface{}{
		"entity_type":     req.EntityType,
		"expression":      req.Expression,
		"identifier_type": req.IdentifierType,
		"audience_limit":  req.Limit,
	}

	findResult, err := s.callWattTool("entity_find", findArgs)
	if err != nil {
		log.Printf("[watt] entity_find error: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Watt Data entity_find failed: " + err.Error()})
		return
	}

	// Parse entity IDs from find result
	var findData struct {
		Total         int    `json:"total"`
		ReturnedCount int    `json:"returned_count"`
		HasMore       bool   `json:"has_more"`
		WorkflowID    string `json:"workflow_id"`
		Sample        []struct {
			EntityID string `json:"entity_id"`
			Email    string `json:"email"`
		} `json:"sample"`
	}
	if err := json.Unmarshal([]byte(findResult), &findData); err != nil {
		log.Printf("[watt] error parsing entity_find response: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Failed to parse Watt Data response"})
		return
	}

	if findData.ReturnedCount == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"total":    0,
			"profiles": []interface{}{},
		})
		return
	}

	// Step 2: entity_enrich — get detailed profiles
	entityIDs := make([]string, 0, len(findData.Sample))
	for _, s := range findData.Sample {
		entityIDs = append(entityIDs, s.EntityID)
	}

	enrichArgs := map[string]interface{}{
		"entity_type": req.EntityType,
		"entity_ids":  entityIDs,
		"domains":     req.Domains,
	}

	enrichResult, err := s.callWattTool("entity_enrich", enrichArgs)
	if err != nil {
		log.Printf("[watt] entity_enrich error: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Watt Data entity_enrich failed: " + err.Error()})
		return
	}

	// Parse enriched profiles
	var enrichData struct {
		Profiles []json.RawMessage `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(enrichResult), &enrichData); err != nil {
		log.Printf("[watt] error parsing entity_enrich response: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Failed to parse enrichment response"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":       findData.Total,
		"returned":    findData.ReturnedCount,
		"has_more":    findData.HasMore,
		"workflow_id": findData.WorkflowID,
		"profiles":    enrichData.Profiles,
	})
}

// handleWattTraitSearch handles POST /api/admin/watt/trait-search
// Searches for Watt Data traits by keyword — needed to build expressions for entity_find.
func (s *Server) handleWattTraitSearch(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.config.WattAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Watt Data API not configured"})
		return
	}

	var req struct {
		Query string `json:"query"` // Search keyword (e.g., "brand manager", "CPG", "food")
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		writeBadRequest(w, "query is required")
		return
	}

	result, err := s.callWattTool("trait_search", map[string]interface{}{
		"query": strings.TrimSpace(req.Query),
	})
	if err != nil {
		log.Printf("[watt] trait_search error: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Watt Data trait_search failed: " + err.Error()})
		return
	}

	// Pass through the raw trait results
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, result)
}

// handleWattEntityResolve handles POST /api/admin/watt/resolve
// Resolves known identifiers (email, phone) to Watt entity IDs.
func (s *Server) handleWattEntityResolve(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.config.WattAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Watt Data API not configured"})
		return
	}

	var req struct {
		EntityType  string `json:"entity_type"` // "person" or "business"
		Identifiers []struct {
			IDType   string `json:"id_type"`   // "email", "phone", "address"
			IDValue  string `json:"id_value"`
			HashType string `json:"hash_type"` // "plaintext", "md5", "sha256"
		} `json:"identifiers"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Identifiers) == 0 {
		writeBadRequest(w, "at least one identifier is required")
		return
	}
	if req.EntityType == "" {
		req.EntityType = "person"
	}

	// Build identifiers array for Watt
	identifiers := make([]map[string]string, len(req.Identifiers))
	for i, id := range req.Identifiers {
		ht := id.HashType
		if ht == "" {
			ht = "plaintext"
		}
		identifiers[i] = map[string]string{
			"id_type":   id.IDType,
			"id_value":  id.IDValue,
			"hash_type": ht,
		}
	}

	result, err := s.callWattTool("entity_resolve", map[string]interface{}{
		"entity_type": req.EntityType,
		"identifiers": identifiers,
	})
	if err != nil {
		log.Printf("[watt] entity_resolve error: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Watt Data entity_resolve failed: " + err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, result)
}

// callWattTool calls a Watt Data MCP tool via the HTTP API.
func (s *Server) callWattTool(toolName string, arguments map[string]interface{}) (string, error) {
	argsJSON, err := json.Marshal(arguments)
	if err != nil {
		return "", fmt.Errorf("marshal arguments: %w", err)
	}

	reqBody := wattMCPRequest{
		Method: "tools/call",
		Params: wattToolRequest{
			Name:      toolName,
			Arguments: argsJSON,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.wattdata.ai/v2/mcp", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+s.config.WattAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := wattHTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse MCP response envelope
	var mcpResp wattMCPResponse
	if err := json.Unmarshal(body, &mcpResp); err != nil {
		// If it doesn't parse as MCP response, return raw body
		return string(body), nil
	}

	if mcpResp.IsError {
		if len(mcpResp.Content) > 0 {
			return "", fmt.Errorf("Watt API error: %s", mcpResp.Content[0].Text)
		}
		return "", fmt.Errorf("Watt API returned error with no details")
	}

	// Extract text content
	var texts []string
	for _, c := range mcpResp.Content {
		if c.Type == "text" && c.Text != "" {
			texts = append(texts, c.Text)
		}
	}
	if len(texts) == 0 {
		return string(body), nil
	}

	return strings.Join(texts, "\n"), nil
}
