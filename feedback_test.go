package main

import (
	"encoding/json"
	"net/http"
	"testing"
)


// =============================================================================
// handleFeedbackList TESTS (72.2% → target ~95%)
// =============================================================================

func TestFeedbackList_WrongMethod(t *testing.T) {
	server := newTestServer(t)

	// handleFeedbackRoutes dispatches; POST goes to handleFeedback, DELETE should be 405
	rr := doRequest(server.handleFeedbackRoutes, "DELETE", "/api/feedback", nil, nil)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for DELETE, got %d", rr.Code)
	}
}

func TestFeedbackList_NonFeedbackUser(t *testing.T) {
	server := newTestServer(t)

	// A signed-in user who is not an admin
	cookie, _ := loginUser(t, server, "5559999999")

	rr := doRequest(server.handleFeedbackRoutes, "GET", "/api/feedback", nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-feedback user, got %d", rr.Code)
	}
}

func TestFeedbackList_EmptyList(t *testing.T) {
	server := newTestServer(t)

	// Admins can submit and read feedback
	cookie, _ := loginUser(t, server, testAdminPhone)

	rr := doRequest(server.handleFeedbackRoutes, "GET", "/api/feedback", nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty feedback list, got %d: %s", rr.Code, rr.Body.String())
	}

	// Should return an empty JSON array, not null
	var items []interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to parse response as JSON array: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty array, got %d items", len(items))
	}
}
