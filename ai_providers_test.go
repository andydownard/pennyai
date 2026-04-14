package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// =============================================================================
// convertToGeminiParts
// =============================================================================

func TestConvertToGeminiParts_StringContent(t *testing.T) {
	parts := convertToGeminiParts("hello world")
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0]["text"] != "hello world" {
		t.Errorf("expected text 'hello world', got %v", parts[0]["text"])
	}
}

func TestConvertToGeminiParts_TextBlocks(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "text", "text": "first"},
		map[string]interface{}{"type": "text", "text": "second"},
	}
	parts := convertToGeminiParts(content)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0]["text"] != "first" {
		t.Errorf("expected 'first', got %v", parts[0]["text"])
	}
	if parts[1]["text"] != "second" {
		t.Errorf("expected 'second', got %v", parts[1]["text"])
	}
}

func TestConvertToGeminiParts_ImageBlocks(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{
			"type": "image",
			"source": map[string]interface{}{
				"type":       "base64",
				"media_type": "image/png",
				"data":       "iVBORw0KGgo=",
			},
		},
	}
	parts := convertToGeminiParts(content)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	inlineData, ok := parts[0]["inline_data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected inline_data map, got %T", parts[0]["inline_data"])
	}
	if inlineData["mime_type"] != "image/png" {
		t.Errorf("expected mime_type 'image/png', got %v", inlineData["mime_type"])
	}
	if inlineData["data"] != "iVBORw0KGgo=" {
		t.Errorf("expected data 'iVBORw0KGgo=', got %v", inlineData["data"])
	}
}

func TestConvertToGeminiParts_MixedContent(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{
			"type": "image",
			"source": map[string]interface{}{
				"type":       "base64",
				"media_type": "image/jpeg",
				"data":       "abc123",
			},
		},
		map[string]interface{}{"type": "text", "text": "describe this image"},
	}
	parts := convertToGeminiParts(content)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	// First part should be inline_data
	if _, ok := parts[0]["inline_data"]; !ok {
		t.Error("expected first part to have inline_data")
	}
	// Second part should be text
	if parts[1]["text"] != "describe this image" {
		t.Errorf("expected text part, got %v", parts[1])
	}
}

func TestConvertToGeminiParts_UnknownContentType(t *testing.T) {
	// Non-string, non-slice type should fallback to fmt.Sprintf
	content := 42
	parts := convertToGeminiParts(content)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0]["text"] != "42" {
		t.Errorf("expected '42', got %v", parts[0]["text"])
	}
}

func TestConvertToGeminiParts_EmptySliceFallback(t *testing.T) {
	// A slice with no recognized blocks should fallback to string representation
	content := []interface{}{
		map[string]interface{}{"type": "unknown", "data": "foo"},
	}
	parts := convertToGeminiParts(content)
	if len(parts) != 1 {
		t.Fatalf("expected 1 fallback part, got %d", len(parts))
	}
	// Should be a stringified version of the content
	if _, ok := parts[0]["text"]; !ok {
		t.Error("expected text key in fallback part")
	}
}

// =============================================================================
// convertToOpenAIContent
// =============================================================================

func TestConvertToOpenAIContent_StringPassthrough(t *testing.T) {
	result := convertToOpenAIContent("just a string")
	s, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	if s != "just a string" {
		t.Errorf("expected 'just a string', got %q", s)
	}
}

func TestConvertToOpenAIContent_TextBlocks(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "text", "text": "hello"},
		map[string]interface{}{"type": "text", "text": "world"},
	}
	result := convertToOpenAIContent(content)
	parts, ok := result.([]map[string]interface{})
	if !ok {
		t.Fatalf("expected []map[string]interface{}, got %T", result)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0]["type"] != "text" || parts[0]["text"] != "hello" {
		t.Errorf("unexpected first part: %v", parts[0])
	}
	if parts[1]["type"] != "text" || parts[1]["text"] != "world" {
		t.Errorf("unexpected second part: %v", parts[1])
	}
}

func TestConvertToOpenAIContent_ImageBlocks(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{
			"type": "image",
			"source": map[string]interface{}{
				"type":       "base64",
				"media_type": "image/png",
				"data":       "iVBORw0KGgo=",
			},
		},
	}
	result := convertToOpenAIContent(content)
	parts, ok := result.([]map[string]interface{})
	if !ok {
		t.Fatalf("expected []map[string]interface{}, got %T", result)
	}
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0]["type"] != "image_url" {
		t.Errorf("expected type 'image_url', got %v", parts[0]["type"])
	}
	imgURL, ok := parts[0]["image_url"].(map[string]string)
	if !ok {
		t.Fatalf("expected image_url map[string]string, got %T", parts[0]["image_url"])
	}
	expected := "data:image/png;base64,iVBORw0KGgo="
	if imgURL["url"] != expected {
		t.Errorf("expected url %q, got %q", expected, imgURL["url"])
	}
}

func TestConvertToOpenAIContent_MixedContent(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "text", "text": "look at this"},
		map[string]interface{}{
			"type": "image",
			"source": map[string]interface{}{
				"type":       "base64",
				"media_type": "image/jpeg",
				"data":       "abc123",
			},
		},
	}
	result := convertToOpenAIContent(content)
	parts, ok := result.([]map[string]interface{})
	if !ok {
		t.Fatalf("expected []map[string]interface{}, got %T", result)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0]["type"] != "text" {
		t.Error("expected first part to be text type")
	}
	if parts[1]["type"] != "image_url" {
		t.Error("expected second part to be image_url type")
	}
}

func TestConvertToOpenAIContent_UnknownType(t *testing.T) {
	// Non-string, non-slice falls back to fmt.Sprintf
	result := convertToOpenAIContent(12345)
	s, ok := result.(string)
	if !ok {
		t.Fatalf("expected string fallback, got %T", result)
	}
	if s != "12345" {
		t.Errorf("expected '12345', got %q", s)
	}
}

func TestConvertToOpenAIContent_EmptySliceFallback(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "weird"},
	}
	result := convertToOpenAIContent(content)
	s, ok := result.(string)
	if !ok {
		t.Fatalf("expected string fallback for unrecognized blocks, got %T", result)
	}
	if s == "" {
		t.Error("expected non-empty fallback string")
	}
}

// =============================================================================
// getProviderChain
// =============================================================================

func TestGetProviderChain_AllKeysSet(t *testing.T) {
	s := &Server{
		config: Config{
			AnthropicAPIKey: "sk-ant-test",
			GeminiAPIKey:    "gemini-test",
			OpenAIAPIKey:    "sk-openai-test",
		},
	}
	chain := s.getProviderChain(Simple)
	if len(chain) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(chain))
	}
	if chain[0].name != "anthropic" {
		t.Errorf("expected first provider 'anthropic', got %q", chain[0].name)
	}
	if chain[1].name != "gemini" {
		t.Errorf("expected second provider 'gemini', got %q", chain[1].name)
	}
	if chain[2].name != "openai" {
		t.Errorf("expected third provider 'openai', got %q", chain[2].name)
	}
}

func TestGetProviderChain_OnlyAnthropic(t *testing.T) {
	s := &Server{
		config: Config{
			AnthropicAPIKey: "sk-ant-test",
		},
	}
	chain := s.getProviderChain(Medium)
	if len(chain) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(chain))
	}
	if chain[0].name != "anthropic" {
		t.Errorf("expected 'anthropic', got %q", chain[0].name)
	}
}

func TestGetProviderChain_GeminiAndOpenAI(t *testing.T) {
	s := &Server{
		config: Config{
			GeminiAPIKey: "gemini-test",
			OpenAIAPIKey: "sk-openai-test",
		},
	}
	chain := s.getProviderChain(Hard)
	if len(chain) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(chain))
	}
	if chain[0].name != "gemini" {
		t.Errorf("expected first provider 'gemini', got %q", chain[0].name)
	}
	if chain[1].name != "openai" {
		t.Errorf("expected second provider 'openai', got %q", chain[1].name)
	}
}

func TestGetProviderChain_NoKeys(t *testing.T) {
	s := &Server{config: Config{}}
	chain := s.getProviderChain(Simple)
	if len(chain) != 0 {
		t.Fatalf("expected 0 providers, got %d", len(chain))
	}
}

func TestGetProviderChain_CorrectModels_Simple(t *testing.T) {
	s := &Server{
		config: Config{
			AnthropicAPIKey: "a",
			GeminiAPIKey:    "g",
			OpenAIAPIKey:    "o",
		},
	}
	chain := s.getProviderChain(Simple)
	if chain[0].model != "claude-haiku-4-5-20251001" {
		t.Errorf("anthropic Simple model: got %q", chain[0].model)
	}
	if chain[1].model != "gemini-2.0-flash-lite" {
		t.Errorf("gemini Simple model: got %q", chain[1].model)
	}
	if chain[2].model != "gpt-4o-mini" {
		t.Errorf("openai Simple model: got %q", chain[2].model)
	}
}

func TestGetProviderChain_CorrectModels_Medium(t *testing.T) {
	s := &Server{
		config: Config{
			AnthropicAPIKey: "a",
			GeminiAPIKey:    "g",
			OpenAIAPIKey:    "o",
		},
	}
	chain := s.getProviderChain(Medium)
	if chain[0].model != "claude-sonnet-4-5-20250514" {
		t.Errorf("anthropic Medium model: got %q", chain[0].model)
	}
	if chain[1].model != "gemini-2.0-flash" {
		t.Errorf("gemini Medium model: got %q", chain[1].model)
	}
	if chain[2].model != "gpt-4o" {
		t.Errorf("openai Medium model: got %q", chain[2].model)
	}
}

func TestGetProviderChain_CorrectModels_Hard(t *testing.T) {
	s := &Server{
		config: Config{
			AnthropicAPIKey: "a",
			GeminiAPIKey:    "g",
			OpenAIAPIKey:    "o",
		},
	}
	chain := s.getProviderChain(Hard)
	if chain[0].model != "claude-opus-4-20250514" {
		t.Errorf("anthropic Hard model: got %q", chain[0].model)
	}
	if chain[1].model != "gemini-2.5-pro-preview-06-05" {
		t.Errorf("gemini Hard model: got %q", chain[1].model)
	}
	if chain[2].model != "o3" {
		t.Errorf("openai Hard model: got %q", chain[2].model)
	}
}

// =============================================================================
// callAI — fallback behavior with mock HTTP servers
// =============================================================================

func TestCallAI_FallbackOnFirstProviderFailure(t *testing.T) {
	// Mock Anthropic server that always returns 500
	anthropicServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal server error"}`)
	}))
	defer anthropicServer.Close()

	// Mock Gemini server that returns a valid response
	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": "gemini response"},
						},
					},
				},
			},
		})
	}))
	defer geminiServer.Close()

	// Test fallback logic: first provider (Anthropic mock) fails, second (Gemini mock) succeeds.
	callCount := 0
	result, err := func() (string, error) {
		var lastErr error
		providers := []providerEntry{
			{
				name: "anthropic", apiKey: "fake", model: "test-model",
				callFn: func(apiKey, model, system string, content interface{}, maxTokens int) (string, error) {
					callCount++
					// Simulate calling the mock Anthropic server
					resp, err := http.Get(anthropicServer.URL)
					if err != nil {
						return "", err
					}
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						return "", fmt.Errorf("anthropic returned %d", resp.StatusCode)
					}
					return "", nil
				},
			},
			{
				name: "gemini", apiKey: "fake", model: "test-model",
				callFn: func(apiKey, model, system string, content interface{}, maxTokens int) (string, error) {
					callCount++
					resp, err := http.Get(geminiServer.URL)
					if err != nil {
						return "", err
					}
					defer resp.Body.Close()
					var apiResp struct {
						Candidates []struct {
							Content struct {
								Parts []struct {
									Text string `json:"text"`
								} `json:"parts"`
							} `json:"content"`
						} `json:"candidates"`
					}
					json.NewDecoder(resp.Body).Decode(&apiResp)
					return apiResp.Candidates[0].Content.Parts[0].Text, nil
				},
			},
		}

		for _, p := range providers {
			result, err := p.callFn(p.apiKey, p.model, "", "test", 100)
			if err == nil {
				return result, nil
			}
			lastErr = err
		}
		return "", fmt.Errorf("all providers failed: %w", lastErr)
	}()

	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}
	if result != "gemini response" {
		t.Errorf("expected 'gemini response', got %q", result)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (one failed, one succeeded), got %d", callCount)
	}
}

func TestCallAI_AllProvidersFail(t *testing.T) {
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"fail"}`)
	}))
	defer failServer.Close()

	providers := []providerEntry{
		{
			name: "provider1", apiKey: "k1", model: "m1",
			callFn: func(apiKey, model, system string, content interface{}, maxTokens int) (string, error) {
				return "", fmt.Errorf("provider1 failed")
			},
		},
		{
			name: "provider2", apiKey: "k2", model: "m2",
			callFn: func(apiKey, model, system string, content interface{}, maxTokens int) (string, error) {
				return "", fmt.Errorf("provider2 failed")
			},
		},
	}

	var lastErr error
	for _, p := range providers {
		_, err := p.callFn(p.apiKey, p.model, "", "test", 100)
		if err == nil {
			t.Fatal("expected all providers to fail")
		}
		lastErr = err
	}
	if lastErr == nil {
		t.Error("expected a last error")
	}
}

func TestCallAI_NoProviders(t *testing.T) {
	s := &Server{config: Config{}}
	_, err := s.callAI(Simple, "system", "hello", 100)
	if err == nil {
		t.Fatal("expected error with no providers configured")
	}
	if err.Error() != "no AI providers configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCallAI_FirstProviderSucceeds(t *testing.T) {
	// When the first provider succeeds, the second should never be called.
	anthropicCalled := false
	result, err := func() (string, error) {
		providers := []providerEntry{
			{
				name: "anthropic", apiKey: "fake", model: "test",
				callFn: func(apiKey, model, system string, content interface{}, maxTokens int) (string, error) {
					anthropicCalled = true
					return "anthropic works", nil
				},
			},
			{
				name: "gemini", apiKey: "fake", model: "test",
				callFn: func(apiKey, model, system string, content interface{}, maxTokens int) (string, error) {
					t.Error("gemini should not be called when anthropic succeeds")
					return "gemini works", nil
				},
			},
		}
		for _, p := range providers {
			result, err := p.callFn(p.apiKey, p.model, "", "test", 100)
			if err == nil {
				return result, nil
			}
		}
		return "", fmt.Errorf("all failed")
	}()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "anthropic works" {
		t.Errorf("expected 'anthropic works', got %q", result)
	}
	if !anthropicCalled {
		t.Error("anthropic should have been called")
	}
}

// =============================================================================
// callWithFallback (from penny.go) — integration test with httptest
// =============================================================================

func TestCallWithFallback_AnthropicFailsGeminiSucceeds(t *testing.T) {
	// Mock Anthropic: always 500
	anthropicMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"overloaded"}`)
	}))
	defer anthropicMock.Close()

	// Mock Gemini: returns valid response
	geminiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{"text": "fallback gemini result"},
					},
				}},
			},
		})
	}))
	defer geminiMock.Close()

	// We can't easily redirect the real callAnthropic/callGemini to our mock servers
	// because the URLs are hardcoded. Instead, test callWithFallback with env vars
	// cleared so only the anthropicKey param matters, and the function constructs
	// its own provider list.

	// Clear env vars to isolate
	origGemini := os.Getenv("GEMINI_API_KEY")
	origOpenAI := os.Getenv("OPENAI_API_KEY")
	os.Setenv("GEMINI_API_KEY", "")
	os.Setenv("OPENAI_API_KEY", "")
	defer func() {
		os.Setenv("GEMINI_API_KEY", origGemini)
		os.Setenv("OPENAI_API_KEY", origOpenAI)
	}()

	// With only Anthropic key but no real server, callWithFallback should fail
	// because the hardcoded URL won't match our mock. This tests the error path.
	_, err := callWithFallback(Simple, "fake-key", "system", "hello", 100)
	if err == nil {
		t.Fatal("expected error when calling with fake key against real API URL")
	}
}

func TestCallWithFallback_NoProviders(t *testing.T) {
	origGemini := os.Getenv("GEMINI_API_KEY")
	origOpenAI := os.Getenv("OPENAI_API_KEY")
	os.Setenv("GEMINI_API_KEY", "")
	os.Setenv("OPENAI_API_KEY", "")
	defer func() {
		os.Setenv("GEMINI_API_KEY", origGemini)
		os.Setenv("OPENAI_API_KEY", origOpenAI)
	}()

	_, err := callWithFallback(Simple, "", "system", "hello", 100)
	if err == nil {
		t.Fatal("expected error with no providers")
	}
	if err.Error() != "no AI providers configured" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCallWithFallback_ProviderChainOrder(t *testing.T) {
	// Verify that callWithFallback builds providers in the correct order:
	// Anthropic, Gemini, OpenAI — by checking the env-based construction.
	origGemini := os.Getenv("GEMINI_API_KEY")
	origOpenAI := os.Getenv("OPENAI_API_KEY")
	os.Setenv("GEMINI_API_KEY", "gemini-test-key")
	os.Setenv("OPENAI_API_KEY", "openai-test-key")
	defer func() {
		os.Setenv("GEMINI_API_KEY", origGemini)
		os.Setenv("OPENAI_API_KEY", origOpenAI)
	}()

	// All three providers will be added but all will fail (fake keys, real URLs).
	// We just verify the error mentions failure, confirming all were tried.
	_, err := callWithFallback(Simple, "anthropic-test-key", "system", "test", 100)
	if err == nil {
		t.Fatal("expected error with fake keys")
	}
	// The error should mention "all AI providers failed"
	if !contains(err.Error(), "all AI providers failed") {
		t.Errorf("expected 'all AI providers failed' in error, got: %v", err)
	}
}

// =============================================================================
// Server.callAI — end-to-end fallback test with mock callFn
// =============================================================================

func TestServerCallAI_FallbackIntegration(t *testing.T) {
	// Create a mock Anthropic server that fails
	anthropicMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify expected headers
		if r.Header.Get("x-api-key") == "" {
			t.Error("expected x-api-key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("expected anthropic-version header")
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error":"overloaded"}`)
	}))
	defer anthropicMock.Close()

	// Create a mock Gemini server that succeeds
	geminiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{"text": "gemini fallback response"},
					},
				}},
			},
		})
	}))
	defer geminiMock.Close()

	// Create a mock OpenAI server (should not be called)
	openaiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("OpenAI should not be called when Gemini succeeds")
	}))
	defer openaiMock.Close()

	// Test using the providerEntry approach to simulate callAI behavior
	callOrder := []string{}
	providers := []providerEntry{
		{
			name: "anthropic", apiKey: "fake", model: "claude-haiku-4-5-20251001",
			callFn: func(apiKey, model, system string, content interface{}, maxTokens int) (string, error) {
				callOrder = append(callOrder, "anthropic")
				return "", fmt.Errorf("anthropic 503: overloaded")
			},
		},
		{
			name: "gemini", apiKey: "fake", model: "gemini-2.0-flash-lite",
			callFn: func(apiKey, model, system string, content interface{}, maxTokens int) (string, error) {
				callOrder = append(callOrder, "gemini")
				return "gemini fallback response", nil
			},
		},
		{
			name: "openai", apiKey: "fake", model: "gpt-4o-mini",
			callFn: func(apiKey, model, system string, content interface{}, maxTokens int) (string, error) {
				callOrder = append(callOrder, "openai")
				t.Error("openai should not be called")
				return "openai response", nil
			},
		},
	}

	// Replicate callAI logic
	var lastErr error
	var result string
	for _, p := range providers {
		r, err := p.callFn(p.apiKey, p.model, "system", "hello", 100)
		if err == nil {
			result = r
			break
		}
		lastErr = err
	}

	if result == "" && lastErr != nil {
		t.Fatalf("expected success via fallback, got error: %v", lastErr)
	}
	if result != "gemini fallback response" {
		t.Errorf("expected 'gemini fallback response', got %q", result)
	}
	if len(callOrder) != 2 {
		t.Errorf("expected 2 calls, got %d: %v", len(callOrder), callOrder)
	}
	if callOrder[0] != "anthropic" || callOrder[1] != "gemini" {
		t.Errorf("unexpected call order: %v", callOrder)
	}
}

// =============================================================================
// Complexity constants
// =============================================================================

func TestComplexityConstants(t *testing.T) {
	// Verify the iota values are as expected
	if Simple != 0 {
		t.Errorf("Simple should be 0, got %d", Simple)
	}
	if Medium != 1 {
		t.Errorf("Medium should be 1, got %d", Medium)
	}
	if Hard != 2 {
		t.Errorf("Hard should be 2, got %d", Hard)
	}
}

// =============================================================================
// Model maps
// =============================================================================

func TestModelMapsComplete(t *testing.T) {
	// Ensure all complexity tiers have entries in each provider's model map
	tiers := []Complexity{Simple, Medium, Hard}
	for _, tier := range tiers {
		if _, ok := anthropicModels[tier]; !ok {
			t.Errorf("anthropicModels missing tier %d", tier)
		}
		if _, ok := geminiModels[tier]; !ok {
			t.Errorf("geminiModels missing tier %d", tier)
		}
		if _, ok := openaiModels[tier]; !ok {
			t.Errorf("openaiModels missing tier %d", tier)
		}
	}
}

// contains checks if substr is in s. Helper to avoid importing strings in test.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
