package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// =============================================================================
// MULTI-PROVIDER AI WITH MODEL TIERING AND FALLBACK
//
// Three providers: Anthropic (Claude), Google (Gemini), OpenAI (GPT)
// Three complexity tiers: simple, medium, hard
// Fallback chain: primary provider fails → try next → try last
// =============================================================================

// Complexity determines which model tier to use per provider.
type Complexity int

const (
	// Simple: text parsing, single items, greetings. Cheapest/fastest models.
	Simple Complexity = iota
	// Medium: multi-item commands, list operations, brand disambiguation.
	Medium
	// Hard: image parsing, receipt reading, complex multi-step reasoning.
	Hard
)

// Model mapping per provider and complexity tier.
var anthropicModels = map[Complexity]string{
	Simple: "claude-haiku-4-5-20251001",
	Medium: "claude-sonnet-4-5-20250514",
	Hard:   "claude-opus-4-20250514",
}

var geminiModels = map[Complexity]string{
	Simple: "gemini-2.0-flash-lite",
	Medium: "gemini-2.0-flash",
	Hard:   "gemini-2.5-pro-preview-06-05",
}

var openaiModels = map[Complexity]string{
	Simple: "gpt-4o-mini",
	Medium: "gpt-4o",
	Hard:   "o3",
}

// aiHTTPClient is shared across all AI provider calls.
var aiHTTPClient = &http.Client{Timeout: 60 * time.Second}

// =============================================================================
// ANTHROPIC (Claude)
// =============================================================================

func callAnthropic(apiKey, model, system string, content interface{}, maxTokens int) (string, error) {
	reqBody := map[string]interface{}{
		"model":      model,
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

	resp, err := aiHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Anthropic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic %s returned %d: %s", model, resp.StatusCode, string(body))
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
		return "", fmt.Errorf("empty response from Claude %s", model)
	}
	return strings.TrimSpace(apiResp.Content[0].Text), nil
}

func callAnthropicWithSearch(apiKey, model, system string, content interface{}, maxTokens int) (string, error) {
	reqBody := map[string]interface{}{
		"model":      model,
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

	resp, err := aiHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Anthropic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic search %s returned %d: %s", model, resp.StatusCode, string(body))
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

// =============================================================================
// GOOGLE GEMINI
// =============================================================================

func callGemini(apiKey, model, system string, content interface{}, maxTokens int) (string, error) {
	// Convert content to Gemini format
	parts := convertToGeminiParts(content)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": parts},
		},
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": maxTokens,
		},
	}
	if system != "" {
		reqBody["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]string{{"text": system}},
		}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling Gemini request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)
	req, err := http.NewRequest("POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("creating Gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := aiHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gemini %s returned %d: %s", model, resp.StatusCode, string(body))
	}

	var apiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("decoding Gemini response: %w", err)
	}
	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini %s", model)
	}
	return strings.TrimSpace(apiResp.Candidates[0].Content.Parts[0].Text), nil
}

// convertToGeminiParts converts Anthropic-style content to Gemini parts format.
func convertToGeminiParts(content interface{}) []map[string]interface{} {
	switch c := content.(type) {
	case string:
		return []map[string]interface{}{{"text": c}}
	case []interface{}:
		var parts []map[string]interface{}
		for _, block := range c {
			if m, ok := block.(map[string]interface{}); ok {
				switch m["type"] {
				case "text":
					parts = append(parts, map[string]interface{}{"text": m["text"]})
				case "image":
					if src, ok := m["source"].(map[string]interface{}); ok {
						parts = append(parts, map[string]interface{}{
							"inline_data": map[string]interface{}{
								"mime_type": src["media_type"],
								"data":      src["data"],
							},
						})
					}
				}
			}
		}
		if len(parts) == 0 {
			return []map[string]interface{}{{"text": fmt.Sprintf("%v", content)}}
		}
		return parts
	default:
		return []map[string]interface{}{{"text": fmt.Sprintf("%v", content)}}
	}
}

// =============================================================================
// OPENAI (GPT)
// =============================================================================

func callOpenAI(apiKey, model, system string, content interface{}, maxTokens int) (string, error) {
	messages := []map[string]interface{}{}
	if system != "" {
		messages = append(messages, map[string]interface{}{"role": "system", "content": system})
	}

	// Convert content to OpenAI format
	openaiContent := convertToOpenAIContent(content)
	messages = append(messages, map[string]interface{}{"role": "user", "content": openaiContent})

	reqBody := map[string]interface{}{
		"model":      model,
		"messages":   messages,
		"max_tokens": maxTokens,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling OpenAI request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("creating OpenAI request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := aiHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling OpenAI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai %s returned %d: %s", model, resp.StatusCode, string(body))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("decoding OpenAI response: %w", err)
	}
	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from OpenAI %s", model)
	}
	return strings.TrimSpace(apiResp.Choices[0].Message.Content), nil
}

// convertToOpenAIContent converts Anthropic-style content to OpenAI format.
func convertToOpenAIContent(content interface{}) interface{} {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var parts []map[string]interface{}
		for _, block := range c {
			if m, ok := block.(map[string]interface{}); ok {
				switch m["type"] {
				case "text":
					parts = append(parts, map[string]interface{}{
						"type": "text",
						"text": m["text"],
					})
				case "image":
					if src, ok := m["source"].(map[string]interface{}); ok {
						mediaType, _ := src["media_type"].(string)
						data, _ := src["data"].(string)
						parts = append(parts, map[string]interface{}{
							"type": "image_url",
							"image_url": map[string]string{
								"url": fmt.Sprintf("data:%s;base64,%s", mediaType, data),
							},
						})
					}
				}
			}
		}
		if len(parts) == 0 {
			return fmt.Sprintf("%v", content)
		}
		return parts
	default:
		return fmt.Sprintf("%v", content)
	}
}
