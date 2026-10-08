package main

import (
	"strings"
	"testing"
)

// TestContextCache_ReadHit verifies buildLearningContext returns the precomputed
// row from penny_context_cache instead of rebuilding when a cache row exists.
func TestContextCache_ReadHit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone, penny_autotune) VALUES ('user-cc1', '+15557770001', 1)")

	sentinel := "What Penny knows about this user: CACHED SENTINEL VALUE"
	if _, err := server.db.Exec("INSERT INTO penny_context_cache (user_id, context_json) VALUES ('user-cc1', ?)", encodeContextCache(sentinel)); err != nil {
		t.Fatalf("seeding cache: %v", err)
	}

	// Add a real learning that WOULD appear in a rebuild, to prove the cache
	// (not the rebuild) was used.
	server.updateLearning("user", "user-cc1", "shopping_frequency", "weekly", 0.9)

	ctx := server.buildLearningContext("user-cc1")
	if ctx != sentinel {
		t.Errorf("Expected cached context %q, got %q", sentinel, ctx)
	}
	if strings.Contains(ctx, "weekly") {
		t.Error("Rebuild ran instead of using cache (found rebuilt 'weekly' content)")
	}
}

// TestContextCache_MissFallsBackToRebuild verifies that with no cache row,
// buildLearningContext rebuilds exactly as before (unchanged behavior).
func TestContextCache_MissFallsBackToRebuild(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone, penny_autotune) VALUES ('user-cc2', '+15557770002', 1)")
	server.updateLearning("user", "user-cc2", "shopping_frequency", "weekly", 0.9)

	ctx := server.buildLearningContext("user-cc2")
	if !strings.Contains(ctx, "weekly") {
		t.Errorf("Expected rebuilt context containing 'weekly' on cache miss, got %q", ctx)
	}
}

// TestContextCache_AutotuneOffIgnoresStaleCache pins the autotune gate ordering:
// a user who turned off "learn from my choices" gets no context even if a stale
// cache row still exists.
func TestContextCache_AutotuneOffIgnoresStaleCache(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone, penny_autotune) VALUES ('user-cc3', '+15557770003', 0)")
	server.db.Exec("INSERT INTO penny_context_cache (user_id, context_json) VALUES ('user-cc3', 'STALE CONTEXT')")

	ctx := server.buildLearningContext("user-cc3")
	if ctx != "" {
		t.Errorf("Expected empty context when autotune is off, got %q", ctx)
	}
}

// The refresh job must recompute, not read back the cache it is refreshing.
func TestContextCache_RefreshReplacesStaleValue(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone, penny_autotune) VALUES ('user-cc9', '+15557770009', 1)")
	if _, err := server.db.Exec("INSERT INTO penny_context_cache (user_id, context_json) VALUES ('user-cc9', ?)", encodeContextCache("STALE")); err != nil {
		t.Fatalf("seeding cache: %v", err)
	}
	server.updateLearning("user", "user-cc9", "shopping_frequency", "weekly", 0.9)

	if got := server.rebuildLearningContext("user-cc9"); !strings.Contains(got, "weekly") || strings.Contains(got, "STALE") {
		t.Fatalf("rebuild returned %q, want fresh content without the stale cache", got)
	}
	if got := server.buildLearningContext("user-cc9"); got != "STALE" {
		t.Fatalf("cached read returned %q, want the cached value", got)
	}
}

func TestContextCache_EncodingRoundTrips(t *testing.T) {
	for _, in := range []string{"plain", "line one\nline \"two\"", ""} {
		if got := decodeContextCache(encodeContextCache(in)); got != in {
			t.Errorf("round trip %q -> %q", in, got)
		}
	}
	if got := decodeContextCache("legacy plain text"); got != "legacy plain text" {
		t.Errorf("legacy plain text decoded to %q", got)
	}
}
