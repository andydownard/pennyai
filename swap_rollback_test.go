package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A statement that fails partway through accepting a swap must undo the whole
// accept: no budget debit, no rebate, no credit, no accepted event, and the
// user can retry once the fault clears. On PostgreSQL a failed statement aborts
// the transaction, so carrying on and committing would lose or half-apply the
// writes; the handler must return an error and roll back instead.
func TestSwaps_AcceptSwap_MidTransactionFailureRollsBackEverything(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Peanut Butter', 'Skippy', 'Pantry & Dry Goods')")
	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'skippy'").Scan(&dealID)

	// The credit update is the last write before COMMIT; make it fail.
	install := []string{
		`CREATE FUNCTION test_block_credit() RETURNS trigger AS $body$ BEGIN RAISE EXCEPTION 'injected failure'; END $body$ LANGUAGE plpgsql`,
		`CREATE TRIGGER test_block_credit BEFORE UPDATE OF swap_credits_cents ON users FOR EACH ROW EXECUTE FUNCTION test_block_credit()`,
	}
	remove := []string{`DROP TRIGGER test_block_credit ON users`}
	for _, q := range install {
		if _, err := server.db.DB.Exec(q); err != nil {
			t.Fatalf("installing fault: %v", err)
		}
	}

	accept := func() *httptest.ResponseRecorder {
		return doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
			map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)
	}
	rr := accept()
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when a statement fails mid-transaction, got %d: %s", rr.Code, rr.Body.String())
	}

	var accepted, rebates, spent, credits int
	var brand string
	server.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE user_id = 'user-1' AND action = 'accepted'").Scan(&accepted)
	server.db.QueryRow("SELECT COUNT(*) FROM swap_rebates WHERE user_id = 'user-1'").Scan(&rebates)
	server.db.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", dealID).Scan(&spent)
	server.db.QueryRow("SELECT swap_credits_cents FROM users WHERE id = 'user-1'").Scan(&credits)
	server.db.QueryRow("SELECT brand FROM list_items WHERE id = 1").Scan(&brand)
	if accepted != 0 || rebates != 0 || spent != 0 || credits != 0 || brand != "Skippy" {
		t.Errorf("partial write survived: accepted=%d rebates=%d budget_spent=%d credits=%d brand=%q",
			accepted, rebates, spent, credits, brand)
	}

	// Fault cleared: the user can accept (the failed attempt left no claim behind).
	for _, q := range remove {
		if _, err := server.db.DB.Exec(q); err != nil {
			t.Fatalf("removing fault: %v", err)
		}
	}
	rr = accept()
	if rr.Code != http.StatusCreated {
		t.Fatalf("retry after fault cleared: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	server.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE user_id = 'user-1' AND action = 'accepted'").Scan(&accepted)
	server.db.QueryRow("SELECT COUNT(*) FROM swap_rebates WHERE user_id = 'user-1'").Scan(&rebates)
	if accepted != 1 || rebates != 1 {
		t.Errorf("retry should produce exactly one accept and one rebate, got accepted=%d rebates=%d", accepted, rebates)
	}
}
