package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Retired IDs 404 and every image upload failed. Pin current ones.
func TestAnthropicModels_AreCurrent(t *testing.T) {
	want := map[Complexity]string{Simple: "claude-haiku-4-5", Medium: "claude-sonnet-5-5", Hard: "claude-opus-5-5"}
	for tier, id := range want {
		if anthropicModels[tier] != id {
			t.Errorf("tier %d = %q, want %q", tier, anthropicModels[tier], id)
		}
	}
}

// Current models put a thinking block first; the answer is the text block.
func TestAnthropicText_SkipsThinkingBlocks(t *testing.T) {
	var r anthropicResponse
	json.Unmarshal([]byte(`{"stop_reason":"end_turn","content":[{"type":"thinking","thinking":""},{"type":"text","text":" [\"milk\"] "}]}`), &r)
	got, err := anthropicText("m", r)
	if err != nil || got != `["milk"]` {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestAnthropicText_RefusalAndEmptyAreErrors(t *testing.T) {
	for _, raw := range []string{
		`{"stop_reason":"refusal","content":[]}`,
		`{"stop_reason":"max_tokens","content":[{"type":"thinking","thinking":""}]}`,
	} {
		var r anthropicResponse
		json.Unmarshal([]byte(raw), &r)
		if _, err := anthropicText("m", r); err == nil {
			t.Errorf("expected an error for %s", raw)
		}
	}
}

func TestAnthropicRequestBody_ThinkingModelsGetRoomAndLowEffort(t *testing.T) {
	b := anthropicRequestBody("claude-opus-5-5", "sys", "hi", 4096, nil)
	if b["max_tokens"] != anthropicMinTokensThinking {
		t.Errorf("max_tokens = %v", b["max_tokens"])
	}
	if oc, _ := b["output_config"].(map[string]interface{}); oc["effort"] != "low" {
		t.Errorf("output_config = %v", b["output_config"])
	}
	for _, banned := range []string{"temperature", "thinking", "top_p"} {
		if _, ok := b[banned]; ok {
			t.Errorf("%s must not be sent to current models", banned)
		}
	}
	h := anthropicRequestBody("claude-haiku-4-5", "", "hi", 100, nil)
	if h["max_tokens"] != 100 || h["output_config"] != nil || h["fallbacks"] != nil {
		t.Errorf("haiku body changed: %v", h)
	}
}

func TestPostAnthropic_SendsBetaHeaderAndReadsText(t *testing.T) {
	var beta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		beta = r.Header.Get("anthropic-beta")
		w.Write([]byte(`{"stop_reason":"end_turn","content":[{"type":"thinking","thinking":""},{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()
	old := anthropicBaseURL
	anthropicBaseURL = srv.URL
	defer func() { anthropicBaseURL = old }()

	got, err := callAnthropic("k", "claude-opus-5-5", "", "hi", 10)
	if err != nil || got != "ok" {
		t.Fatalf("got %q, %v", got, err)
	}
	if !strings.Contains(beta, "server-side-fallback") {
		t.Errorf("anthropic-beta = %q", beta)
	}
}
