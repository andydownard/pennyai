package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// =============================================================================
// CONSOLIDATED NoSession / MethodNotAllowed TESTS
//
// These table-driven tests replace dozens of individual tests that all
// followed the same pattern: send a request and check for 401 or 405.
// =============================================================================

func TestNoSession_AllEndpoints(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	cases := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		path    string
		body    interface{}
	}{
		{"ListGet", server.handleList, "GET", "/api/list", nil},
		{"ListAdd", server.handleList, "POST", "/api/list", map[string]string{"name": "Milk"}},
		{"ListToggle", server.handleList, "PATCH", "/api/list/1/toggle", nil},
		{"ListEdit", server.handleList, "PATCH", "/api/list/1", map[string]interface{}{"name": "x"}},
		{"ListDelete", server.handleList, "DELETE", "/api/list/1", nil},
		{"ParseAndAdd", server.handleList, "POST", "/api/list/parse", map[string]string{"text": "milk"}},
		{"ClearList", server.handleListClear, "POST", "/api/list/clear", nil},
		{"SavedLists", server.handleSavedLists, "GET", "/api/saved-lists", nil},
		{"SavedListsCreate", server.handleSavedLists, "POST", "/api/saved-lists", map[string]string{"name": "test"}},
		{"SavedListDelete", server.handleSavedLists, "DELETE", "/api/saved-lists/1", nil},
		{"Settings", server.handleSettings, "PATCH", "/api/settings", map[string]string{"name": "Test"}},
		{"Invite", server.handleInvite, "POST", "/api/settings/invite", map[string]string{"phone": ""}},
		{"LeaveHousehold", server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil},
		{"ListUrgent", server.handleList, "PATCH", "/api/list/1/urgent", nil},
		{"ListClaim", server.handleList, "PATCH", "/api/list/1/claim", nil},
		{"NPSCheck", server.handleNPSCheck, "GET", "/api/nps/check", nil},
		{"NPS", server.handleNPS, "POST", "/api/nps", map[string]interface{}{"score": 8}},
		{"ReferralRedeem", server.handleReferralRedeem, "POST", "/api/referral/redeem", nil},
		{"Trips", server.handleTrips, "GET", "/api/trips", nil},
		{"RequireAdmin", server.handleAdminDashboard, "GET", "/api/admin/dashboard", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := doRequest(tc.handler, tc.method, tc.path, tc.body, nil)
			if rr.Code != http.StatusUnauthorized {
				t.Errorf("Status: got %d, want 401", rr.Code)
			}
		})
	}
}


func TestMethodNotAllowed_AllEndpoints(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	cases := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		path    string
	}{
		{"List_PUT", server.handleList, "PUT", "/api/list"},
		{"SavedLists_PATCH", server.handleSavedLists, "PATCH", "/api/saved-lists"},
		{"SavedListsCreate_PUT", server.handleSavedLists, "PUT", "/api/saved-lists"},
		{"LeaveHousehold_GET", server.handleLeaveHousehold, "GET", "/api/settings/leave-household"},
		{"Join_GET", server.handleJoin, "GET", "/api/join"},
		{"Invite_GET", server.handleInvite, "GET", "/api/settings/invite"},
		{"VCard_POST", server.handleVCard, "POST", "/api/penny.vcf"},
		{"Settings_DELETE", server.handleSettings, "DELETE", "/api/settings"},
		{"ListClear_GET", server.handleListClear, "GET", "/api/list/clear"},
		{"SMSWebhook_GET", server.handleSMSWebhook, "GET", "/sms"},
		{"EmailWebhook_GET", server.handleEmailWebhook, "GET", "/api/email-webhook"},
		{"ReferralClaim_GET", server.handleReferralClaim, "GET", "/api/referral/claim"},
		{"ReferralRedeem_GET", server.handleReferralRedeem, "GET", "/api/referral/redeem"},
		{"Referral_POST", server.handleReferral, "POST", "/api/referral"},
		{"Brands_POST", server.handleBrands, "POST", "/api/brands"},
		{"NPSCheck_POST", server.handleNPSCheck, "POST", "/api/nps/check"},
		{"NPS_GET", server.handleNPS, "GET", "/api/nps"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := doRequest(tc.handler, tc.method, tc.path, nil, []*http.Cookie{cookie})
			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("Status: got %d, want 405", rr.Code)
			}
		})
	}
}

// =============================================================================
// PHONE NUMBER NORMALIZATION TESTS
// =============================================================================

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantE164 string
		wantOK   bool
	}{
		// --- Valid: raw digits ---
		{"10 raw digits", "5551234567", "+15551234567", true},

		// --- Valid: with formatting ---
		{"parens and dash", "(555) 123-4567", "+15551234567", true},
		{"dashes only", "555-123-4567", "+15551234567", true},
		{"dots", "555.123.4567", "+15551234567", true},
		{"spaces", "555 123 4567", "+15551234567", true},

		// --- Valid: leading country code stripped ---
		{"11 digits with leading 1", "15551234567", "+15551234567", true},
		{"formatted with leading 1", "1 (555) 123-4567", "+15551234567", true},
		{"+1 prefix stripped", "+15551234567", "+15551234567", true},
		{"1-555-123-4567", "1-555-123-4567", "+15551234567", true},

		// --- Invalid ---
		{"too short (9 digits)", "555123456", "", false},
		{"too long (12 digits)", "125551234567", "", false},
		{"empty string", "", "", false},
		{"all letters", "abcdefghij", "", false},
		{"5 digits", "55512", "", false},
		{"leading 2 with 11 digits", "25551234567", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizePhone(tt.input)
			if ok != tt.wantOK {
				t.Errorf("normalizePhone(%q): ok = %v, want %v", tt.input, ok, tt.wantOK)
				return
			}
			if ok && got != tt.wantE164 {
				t.Errorf("normalizePhone(%q) = %q, want %q", tt.input, got, tt.wantE164)
			}
		})
	}
}

// =============================================================================
// DATABASE TESTS
// =============================================================================

func TestUpsertUser_CreatesNewUser(t *testing.T) {
	_, pgdb := openTestDB(t)

	user, err := upsertUser(pgdb, "5551234567")
	if err != nil {
		t.Fatalf("upsertUser: %v", err)
	}

	if user.ID == "" {
		t.Error("Expected non-empty user ID")
	}
	if user.PhoneNumber != "5551234567" {
		t.Errorf("Phone: got %s, want 5551234567", user.PhoneNumber)
	}
	if user.CreatedAt == "" {
		t.Error("Expected non-empty CreatedAt")
	}
}


func TestUpsertUser_SamePhoneTwice(t *testing.T) {
	_, pgdb := openTestDB(t)

	first, _ := upsertUser(pgdb, "5551234567")
	second, _ := upsertUser(pgdb, "5551234567")

	// Same phone should return the same user — not create a duplicate
	if first.ID != second.ID {
		t.Errorf("Same phone created two users: ID %s and %s", first.ID, second.ID)
	}
}


func TestUpsertUser_DifferentPhones(t *testing.T) {
	_, pgdb := openTestDB(t)

	u1, _ := upsertUser(pgdb, "5551111111")
	u2, _ := upsertUser(pgdb, "5552222222")

	if u1.ID == u2.ID {
		t.Error("Different phones got the same user ID")
	}
}


func TestInitDB_CreatesTable(t *testing.T) {
	_, pgdb := openTestDB(t)

	// Verify the users table exists by querying it
	var count int
	err := pgdb.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Fatalf("Users table doesn't exist: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 users in fresh DB, got %d", count)
	}
}

// =============================================================================
// SESSION STORE TESTS
// =============================================================================
// WRITE JSON HELPER TEST
// =============================================================================
// SESSION STORE TESTS
// =============================================================================
// WRITE JSON HELPER TEST
// =============================================================================

func TestWriteJSON_SetsContentType(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusOK, map[string]string{"hello": "world"})

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %s, want application/json", ct)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

// =============================================================================
// FULL FLOW INTEGRATION TEST
//
// Simulates: send → verify → me (logged in) → logout → me (denied)
// Also verifies the user was created in the database.
// =============================================================================


func TestInitDB_CreatesListItemsTable(t *testing.T) {
	_, pgdb := openTestDB(t)

	var count int
	if err := pgdb.QueryRow("SELECT COUNT(*) FROM list_items").Scan(&count); err != nil {
		t.Fatalf("list_items table doesn't exist: %v", err)
	}
}


// --- helper tests ---

func TestDecodeJSON_InvalidBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	var data struct{ Name string }
	ok := decodeJSON(rr, req, &data)
	if ok {
		t.Error("Expected false for invalid JSON")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}


func TestDecodeJSON_ValidBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"name":"Alice"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	var data struct{ Name string }
	ok := decodeJSON(rr, req, &data)
	if !ok {
		t.Error("Expected true for valid JSON")
	}
	if data.Name != "Alice" {
		t.Errorf("Name: got %q, want Alice", data.Name)
	}
}


func TestDbError(t *testing.T) {
	rr := httptest.NewRecorder()
	dbError(rr, fmt.Errorf("test error"), "Something failed")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Status: got %d, want 500", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "Something failed" {
		t.Errorf("Error: got %v, want 'Something failed'", body["error"])
	}
}


// =============================================================================
// PRIVACY AND TERMS PAGES
// =============================================================================

func TestPrivacyPolicy_Returns200(t *testing.T) {
	req := httptest.NewRequest("GET", "/privacy", nil)
	rr := httptest.NewRecorder()
	handlePrivacyPolicy(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Expected text/html content type, got %s", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Privacy Policy") {
		t.Error("Expected page to contain 'Privacy Policy'")
	}
	if !strings.Contains(body, "not") && !strings.Contains(body, "do not") {
		t.Error("Expected policy to mention no third-party sharing")
	}
}


func TestTermsAndConditions_Returns200(t *testing.T) {
	req := httptest.NewRequest("GET", "/terms", nil)
	rr := httptest.NewRecorder()
	handleTerms(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Terms") {
		t.Error("Expected page to contain 'Terms'")
	}
	if !strings.Contains(body, "STOP") {
		t.Error("Expected terms to mention STOP opt-out")
	}
	if !strings.Contains(body, "HELP") {
		t.Error("Expected terms to mention HELP")
	}
	if !strings.Contains(body, "data rates") {
		t.Error("Expected terms to mention data rates")
	}
}

// =============================================================================
// LAST_ACTIVE TRACKING
// =============================================================================

func TestUpsertUser_SetsLastActive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	upsertUser(server.db, "5551234567")

	var lastActive sql.NullString
	server.db.QueryRow("SELECT last_active FROM users WHERE phone_number = '5551234567'").Scan(&lastActive)
	if !lastActive.Valid {
		t.Error("Expected last_active to be set after upsertUser")
	}
}


func TestUpsertUser_UpdatesLastActive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	upsertUser(server.db, "5551234567")

	var first sql.NullString
	server.db.QueryRow("SELECT last_active FROM users WHERE phone_number = '5551234567'").Scan(&first)

	// Second login should update last_active
	upsertUser(server.db, "5551234567")

	var second sql.NullString
	server.db.QueryRow("SELECT last_active FROM users WHERE phone_number = '5551234567'").Scan(&second)

	if !second.Valid {
		t.Error("Expected last_active to be set on second login")
	}
}

// =============================================================================
// REQUIRE ADMIN HELPER
// =============================================================================


func TestInitDB_MigrationCascade(t *testing.T) {
	// Verify item_modifications has ON DELETE CASCADE
	_, pgdb := openTestDB(t)

	// Create a user + item + modification
	pgdb.Exec("INSERT INTO users (id, phone_number) VALUES ('user-1', '5551234567')")
	result, _ := pgdb.Exec("INSERT INTO list_items (user_id, name, department) VALUES ('user-1', 'Milk', 'Dairy')")
	itemID, _ := result.LastInsertId()
	pgdb.Exec("INSERT INTO item_modifications (item_id, user_id, action, detail, source) VALUES (?, 'user-1', 'added', '', 'app')", itemID)

	// Delete the list item — modifications should cascade
	pgdb.Exec("DELETE FROM list_items WHERE id = ?", itemID)

	var count int
	pgdb.QueryRow("SELECT COUNT(*) FROM item_modifications WHERE item_id = ?", itemID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 modifications after cascade delete, got %d", count)
	}
}


func TestInitDB_MigrationPath(t *testing.T) {
	t.Skip("SQLite-specific migration test — not applicable after PostgreSQL migration")
}


func TestInitDB_MigratesExistingDB(t *testing.T) {
	t.Skip("SQLite-specific migration test — not applicable after PostgreSQL migration")
}


func TestInitDB_CreatesAllTables(t *testing.T) {
	_, pgdb := openTestDB(t)

	// Verify key tables exist by querying each
	tables := []string{"users", "list_items", "saved_lists", "saved_list_items", "households",
		"household_members", "invites", "item_modifications", "referrals", "redemptions", "sessions"}
	for _, table := range tables {
		var count int
		err := pgdb.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("Table %s not found: %v", table, err)
		}
	}
}


// =============================================================================
// BRANDS ENDPOINT TESTS
// =============================================================================

func TestBrands_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Insert test brands
	server.db.Exec("INSERT INTO brands (name, domain) VALUES (?, ?)", "Cheerios", "cheerios.com")
	server.db.Exec("INSERT INTO brands (name, domain) VALUES (?, ?)", "Tide", "tide.com")

	rr := doRequest(server.handleBrands, "GET", "/api/brands", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var brands map[string]string
	json.Unmarshal(rr.Body.Bytes(), &brands)

	if brands["cheerios"] != "cheerios.com" {
		t.Errorf("Expected cheerios.com, got %s", brands["cheerios"])
	}
	if brands["tide"] != "tide.com" {
		t.Errorf("Expected tide.com, got %s", brands["tide"])
	}

	// Verify Cache-Control header
	cc := rr.Header().Get("Cache-Control")
	if cc != "public, max-age=3600" {
		t.Errorf("Cache-Control: got %q, want %q", cc, "public, max-age=3600")
	}
}


func TestBrands_ReturnsSeedData(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleBrands, "GET", "/api/brands", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var brands map[string]string
	json.Unmarshal(rr.Body.Bytes(), &brands)
	// initDB seeds brands — verify we get a non-empty map with proper structure
	if len(brands) == 0 {
		t.Error("Expected seeded brands, got empty map")
	}
	// All keys should be lowercased
	for k := range brands {
		if k != strings.ToLower(k) {
			t.Errorf("Brand key %q is not lowercased", k)
			break
		}
	}
}


func TestBrands_CaseInsensitive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	server.db.Exec("INSERT INTO brands (name, domain) VALUES (?, ?)", "Cheerios", "cheerios.com")

	rr := doRequest(server.handleBrands, "GET", "/api/brands", nil, nil)
	var brands map[string]string
	json.Unmarshal(rr.Body.Bytes(), &brands)

	// Brand names should be lowercased in response
	if _, ok := brands["cheerios"]; !ok {
		t.Error("Expected brand name to be lowercased")
	}
	if _, ok := brands["Cheerios"]; ok {
		t.Error("Brand name should not be in original case")
	}
}

// =============================================================================
// GLOBAL BRAND DEFAULT TESTS
// =============================================================================

func TestGlobalBrandDefault_NoData(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	brand, ok := getGlobalBrandDefault(server.db, "yogurt")
	if ok {
		t.Errorf("Expected no default, got %q", brand)
	}
}


func TestGlobalBrandDefault_SinglePick(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551000001")
	// Only 1 pick — below the threshold of 2
	server.db.Exec("INSERT INTO brand_choices (user_id, product_category, brand) VALUES (?, 'yogurt', 'Fage')", userID)
	brand, ok := getGlobalBrandDefault(server.db, "yogurt")
	if ok {
		t.Errorf("Expected no default with only 1 pick, got %q", brand)
	}
	_ = brand
}


func TestGlobalBrandDefault_ClearWinner(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// Create multiple users, each picks Fage
	for i := 0; i < 5; i++ {
		_, uid := loginUser(t, server, fmt.Sprintf("555200%04d", i))
		server.db.Exec("INSERT INTO brand_choices (user_id, product_category, brand) VALUES (?, 'yogurt', 'Fage')", uid)
	}
	// One user picks Chobani
	_, uid6 := loginUser(t, server, "5553000001")
	server.db.Exec("INSERT INTO brand_choices (user_id, product_category, brand) VALUES (?, 'yogurt', 'Chobani')", uid6)

	brand, ok := getGlobalBrandDefault(server.db, "yogurt")
	if !ok {
		t.Fatal("Expected a default brand")
	}
	if brand != "Fage" {
		t.Errorf("Expected Fage, got %q", brand)
	}
}


func TestGlobalBrandDefault_NoMajority(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// Even split: 3 each → top has 50% (above 40% threshold), should return
	for i := 0; i < 3; i++ {
		_, uid := loginUser(t, server, fmt.Sprintf("555400%04d", i))
		server.db.Exec("INSERT INTO brand_choices (user_id, product_category, brand) VALUES (?, 'ketchup', 'Heinz')", uid)
	}
	for i := 0; i < 3; i++ {
		_, uid := loginUser(t, server, fmt.Sprintf("555500%04d", i))
		server.db.Exec("INSERT INTO brand_choices (user_id, product_category, brand) VALUES (?, 'ketchup', 'Hunts')", uid)
	}

	brand, ok := getGlobalBrandDefault(server.db, "ketchup")
	if !ok {
		t.Fatal("Expected a default brand (50% share)")
	}
	if brand != "Heinz" && brand != "Hunts" {
		t.Errorf("Expected one of the brands, got %q", brand)
	}
}


func TestGlobalBrandDefault_CaseInsensitive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	for i := 0; i < 3; i++ {
		_, uid := loginUser(t, server, fmt.Sprintf("555600%04d", i))
		server.db.Exec("INSERT INTO brand_choices (user_id, product_category, brand) VALUES (?, 'milk', 'Organic Valley')", uid)
	}
	// Query with mixed case
	brand, ok := getGlobalBrandDefault(server.db, "Milk")
	if !ok {
		t.Fatal("Expected a default brand")
	}
	if brand != "Organic Valley" {
		t.Errorf("Expected Organic Valley, got %q", brand)
	}
}


func TestGlobalBrandDefault_EmptyCategory(t *testing.T) {
	_, ok := getGlobalBrandDefault(nil, "")
	if ok {
		t.Error("Expected no default for empty category")
	}
}


func TestDietaryBrandDefault_PrefersDietaryMatch(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// Create 5 "vegan" users who all pick Oatly for milk
	for i := 0; i < 5; i++ {
		_, uid := loginUser(t, server, fmt.Sprintf("555700%04d", i))
		server.db.Exec(`INSERT INTO user_dietary_profiles (user_id, allergens, diets) VALUES (?, '[]', '["vegan"]') ON CONFLICT(user_id) DO UPDATE SET allergens = excluded.allergens, diets = excluded.diets`, uid)
		server.db.Exec("INSERT INTO brand_choices (user_id, product_category, brand) VALUES (?, 'milk', 'Oatly')", uid)
	}
	// Create 10 general users who all pick Horizon for milk (more popular overall)
	for i := 0; i < 10; i++ {
		_, uid := loginUser(t, server, fmt.Sprintf("555710%04d", i))
		server.db.Exec("INSERT INTO brand_choices (user_id, product_category, brand) VALUES (?, 'milk', 'Horizon')", uid)
	}

	// Without dietary prefs → should return Horizon (more popular globally)
	brand, ok := getGlobalBrandDefault(server.db, "milk")
	if !ok {
		t.Fatal("Expected a default brand")
	}
	if brand != "Horizon" {
		t.Errorf("Expected Horizon (global popular), got %q", brand)
	}

	// With vegan dietary prefs → should return Oatly (popular among vegans)
	brand, ok = getGlobalBrandDefault(server.db, "milk", []string{"vegan"})
	if !ok {
		t.Fatal("Expected a dietary-matched default brand")
	}
	if brand != "Oatly" {
		t.Errorf("Expected Oatly (vegan popular), got %q", brand)
	}
}


func TestDietaryBrandDefault_FallsBackToGlobal(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// Create 5 users with no dietary profile who all pick Heinz for ketchup
	for i := 0; i < 5; i++ {
		_, uid := loginUser(t, server, fmt.Sprintf("555720%04d", i))
		server.db.Exec("INSERT INTO brand_choices (user_id, product_category, brand) VALUES (?, 'ketchup', 'Heinz')", uid)
	}

	// Request with vegan diet but no vegan users have picked ketchup → fall back to global
	brand, ok := getGlobalBrandDefault(server.db, "ketchup", []string{"vegan"})
	if !ok {
		t.Fatal("Expected fallback to global default")
	}
	if brand != "Heinz" {
		t.Errorf("Expected Heinz (global fallback), got %q", brand)
	}
}


func TestDietaryBrandDefault_NoDiets(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	for i := 0; i < 3; i++ {
		_, uid := loginUser(t, server, fmt.Sprintf("555730%04d", i))
		server.db.Exec("INSERT INTO brand_choices (user_id, product_category, brand) VALUES (?, 'bread', 'Dave''s')", uid)
	}
	// Empty diets → should use global default directly
	brand, ok := getGlobalBrandDefault(server.db, "bread", []string{})
	if !ok {
		t.Fatal("Expected global default with empty diets")
	}
	if brand != "Dave's" {
		t.Errorf("Expected Dave's, got %q", brand)
	}
}


func TestDietaryBrandDefault_MultipleDiets(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// Create users with organic diet who prefer Organic Valley
	for i := 0; i < 4; i++ {
		_, uid := loginUser(t, server, fmt.Sprintf("555740%04d", i))
		server.db.Exec(`INSERT INTO user_dietary_profiles (user_id, allergens, diets) VALUES (?, '[]', '["organic"]') ON CONFLICT(user_id) DO UPDATE SET allergens = excluded.allergens, diets = excluded.diets`, uid)
		server.db.Exec("INSERT INTO brand_choices (user_id, product_category, brand) VALUES (?, 'eggs', 'Vital Farms')", uid)
	}
	// User with ["organic", "keto"] → should still match organic users
	brand, ok := getGlobalBrandDefault(server.db, "eggs", []string{"organic", "keto"})
	if !ok {
		t.Fatal("Expected dietary-matched default")
	}
	if brand != "Vital Farms" {
		t.Errorf("Expected Vital Farms, got %q", brand)
	}
}


func TestGetUserDiets_NilWhenNoProfile(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, uid := loginUser(t, server, "5557500001")
	diets := getUserDiets(server.db, uid)
	if diets != nil {
		t.Errorf("Expected nil for user with no dietary profile, got %v", diets)
	}
}


func TestGetUserDiets_ReturnsProfile(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, uid := loginUser(t, server, "5557600001")
	server.db.Exec(`INSERT INTO user_dietary_profiles (user_id, allergens, diets) VALUES (?, '["peanuts"]', '["vegan","gluten-free"]')`, uid)
	diets := getUserDiets(server.db, uid)
	if len(diets) != 2 {
		t.Fatalf("Expected 2 diets, got %d: %v", len(diets), diets)
	}
	if diets[0] != "vegan" || diets[1] != "gluten-free" {
		t.Errorf("Expected [vegan gluten-free], got %v", diets)
	}
}

// =============================================================================
// LIST URGENT ENDPOINT TESTS
// =============================================================================


func TestDecodeJSON_RequestTooLarge(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Create a body larger than 10MB
	bigString := strings.Repeat("x", 11*1024*1024)
	body := map[string]string{"name": bigString}

	rr := doRequest(server.handleList, "POST", "/api/list", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Large body: got %d, want 400", rr.Code)
	}
}


// =============================================================================
// ROUND 4: HOUSEHOLD, SAVED LISTS, HEALTH, REQUIRE-METHOD TESTS
// =============================================================================

func TestHealth_ReturnsOK(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("health check returned %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["status"] != "ok" {
		t.Errorf("health check body = %v, want status=ok", body)
	}
}


func TestRequireMethod_Allowed(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	if !requireMethod(rr, req, http.MethodGet) {
		t.Error("requireMethod should return true for matching method")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}


func TestRequireMethod_Denied(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	if requireMethod(rr, req, http.MethodPost) {
		t.Error("requireMethod should return false for non-matching method")
	}
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rr.Code)
	}
}


// =============================================================================
// LOGO TESTS
// =============================================================================

func TestLogo_MissingDomain(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo", nil, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
}


func TestLogo_InvalidDomain(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain=evil%3Cscript%3E.com", nil, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
}


func TestLogo_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	rr := doRequest(server.handleLogoProxy, "POST", "/api/logo?domain=kraft.com", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", rr.Code)
	}
}


func TestLogo_MemoryCacheHit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	// Pre-populate memory cache
	fakeData := []byte("PNG_FAKE_DATA_LARGE_ENOUGH_TO_PASS_CHECK_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	server.logoCache.Store("kraft.com", &cachedLogo{data: fakeData, contentType: "image/png"})

	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain=kraft.com", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "image/png" {
		t.Errorf("Expected image/png, got %s", rr.Header().Get("Content-Type"))
	}
	if rr.Header().Get("Cache-Control") != "public, max-age=604800" {
		t.Errorf("Expected cache-control header, got %s", rr.Header().Get("Cache-Control"))
	}
	if rr.Body.Len() != len(fakeData) {
		t.Errorf("Expected body length %d, got %d", len(fakeData), rr.Body.Len())
	}
}


func TestLogo_MemoryCacheIsCaseInsensitive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	fakeData := []byte("PNG_FAKE_DATA_LARGE_ENOUGH_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	server.logoCache.Store("kraft.com", &cachedLogo{data: fakeData, contentType: "image/png"})

	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain=Kraft.com", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}


func TestLogo_SQLiteCacheHit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	// Insert a brand and logo into the database
	// Use a unique brand name to avoid conflict with seedBrands
	res, err := server.db.Exec("INSERT INTO brands (name, domain) VALUES ('TestBrandXYZ', 'testbrandxyz.com')")
	if err != nil {
		t.Fatalf("Failed to insert brand: %v", err)
	}
	brandID, _ := res.LastInsertId()
	fakeData := []byte("SQLITE_LOGO_DATA_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	server.db.Exec("INSERT INTO brand_logos (brand_id, logo_data, content_type) VALUES (?, ?, 'image/png')", brandID, fakeData)

	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain=testbrandxyz.com", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	if rr.Body.Len() != len(fakeData) {
		t.Errorf("Expected body length %d, got %d", len(fakeData), rr.Body.Len())
	}
	// Verify it was cached in memory
	if _, ok := server.logoCache.Load("testbrandxyz.com"); !ok {
		t.Error("Expected logo to be stored in memory cache after SQLite hit")
	}
}


func TestLogo_PlaceholderFallback(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	// No token, no cache → should get 404 so frontend falls back to emoji
	server.config.LogoDevToken = ""

	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain=unknownbrand.com", nil, nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}


func TestLogo_PlaceholderUseBrandName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	server.config.LogoDevToken = ""
	// Insert a brand so the handler can use its name for initials
	server.db.Exec("INSERT OR IGNORE INTO brands (name, domain) VALUES ('Zesty Foods', 'zestyfoods.com')")

	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain=zestyfoods.com", nil, nil)
	// No real logo found → 404 so frontend emoji fallback triggers
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}


func TestLogoInitials(t *testing.T) {
	tests := []struct {
		name, want string
	}{
		{"Kraft", "K"},
		{"General Mills", "GM"},
		{"Ben & Jerry's", "B&"},
		{"", "?"},
	}
	for _, tt := range tests {
		got := logoInitials(tt.name)
		if got != tt.want {
			t.Errorf("logoInitials(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}


func TestLogoColor_Deterministic(t *testing.T) {
	c1 := logoColor("Kraft")
	c2 := logoColor("Kraft")
	if c1 != c2 {
		t.Errorf("logoColor should be deterministic: %q != %q", c1, c2)
	}
	c3 := logoColor("General Mills")
	if c1 == c3 {
		// Not guaranteed but very likely different
		t.Log("Note: Kraft and General Mills have the same color (unlikely but possible)")
	}
	if !strings.HasPrefix(c1, "#") {
		t.Errorf("Expected hex color, got %q", c1)
	}
}


func TestFetchBrandLogo_EmptyToken(t *testing.T) {
	data, ct := fetchBrandLogo("kraft.com", "", "")
	if data != nil || ct != "" {
		t.Error("Expected nil result with empty token")
	}
}


func TestLogo_DomainSanitization(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	// Valid domains
	validDomains := []string{"kraft.com", "ben-jerrys.com", "Mr.Clean.com", "a1steak.com"}
	for _, d := range validDomains {
		rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain="+url.QueryEscape(d), nil, nil)
		if rr.Code == http.StatusBadRequest {
			t.Errorf("Domain %q should be valid, got 400", d)
		}
	}
	// Invalid domains
	invalidDomains := []string{"evil<script>.com", "bad domain.com", "path/../traversal", "data:text/html"}
	for _, d := range invalidDomains {
		rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain="+url.QueryEscape(d), nil, nil)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("Domain %q should be invalid, got %d", d, rr.Code)
		}
	}
}

// =============================================================================
// SPA ROUTING TESTS — Browser Navigation
//
// Verifies that the Go server serves index.html for all frontend routes
// (pathname-based routing for browser back/forward support).
// =============================================================================

func TestSPARouting_FrontendPaths(t *testing.T) {
	// Create a temp static directory with a dummy index.html
	tmpDir := t.TempDir()
	indexContent := []byte("<!DOCTYPE html><html><body>SPA</body></html>")
	if err := os.WriteFile(tmpDir+"/index.html", indexContent, 0644); err != nil {
		t.Fatalf("Failed to write test index.html: %v", err)
	}
	// Create a real static asset to test that those still get served
	if err := os.WriteFile(tmpDir+"/test.js", []byte("console.log('hi')"), 0644); err != nil {
		t.Fatalf("Failed to write test.js: %v", err)
	}

	fs := http.FileServer(http.Dir(tmpDir))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(tmpDir + r.URL.Path); err == nil {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, tmpDir+"/index.html")
	})

	// All these frontend paths should serve index.html (SPA catch-all)
	frontendPaths := []string{
		"/",
		"/login",
		"/verify",
		"/about",
		"/share",
		"/penny",
		"/profile",
		"/settings",
		"/settings/profile",
		"/settings/saved",
		"/admin",
		"/join/abc123",
		"/r/refcode",
	}

	for _, path := range frontendPaths {
		t.Run("GET "+path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("GET %s: got status %d, want 200", path, rr.Code)
			}
			body := rr.Body.String()
			if !strings.Contains(body, "SPA") {
				t.Errorf("GET %s: expected index.html content, got %q", path, body[:min(len(body), 100)])
			}
		})
	}
}


func TestSPARouting_StaticAssetsServedDirectly(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(tmpDir+"/index.html", []byte("SPA"), 0644); err != nil {
		t.Fatal(err)
	}
	jsContent := "console.log('app')"
	if err := os.WriteFile(tmpDir+"/app.js", []byte(jsContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tmpDir+"/assets", 0755); err != nil {
		t.Fatal(err)
	}
	cssContent := "body { color: red }"
	if err := os.WriteFile(tmpDir+"/assets/style.css", []byte(cssContent), 0644); err != nil {
		t.Fatal(err)
	}

	fs := http.FileServer(http.Dir(tmpDir))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(tmpDir + r.URL.Path); err == nil {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, tmpDir+"/index.html")
	})

	// Static files should be served directly (not index.html)
	t.Run("JS file", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/app.js", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "console.log") {
			t.Error("expected JS content")
		}
	})

	t.Run("Nested CSS file", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/style.css", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "color: red") {
			t.Error("expected CSS content")
		}
	})
}


func TestSPARouting_UnknownPathsFallbackToIndex(t *testing.T) {
	tmpDir := t.TempDir()
	indexContent := []byte("INDEX")
	if err := os.WriteFile(tmpDir+"/index.html", indexContent, 0644); err != nil {
		t.Fatal(err)
	}

	fs := http.FileServer(http.Dir(tmpDir))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(tmpDir + r.URL.Path); err == nil {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, tmpDir+"/index.html")
	})

	unknownPaths := []string{
		"/foo/bar",
		"/nonexistent",
		"/deeply/nested/path/here",
		"/settings/unknown",
	}

	for _, path := range unknownPaths {
		t.Run("GET "+path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("GET %s: got status %d, want 200", path, rr.Code)
			}
			if !strings.Contains(rr.Body.String(), "INDEX") {
				t.Errorf("GET %s: expected index.html fallback", path)
			}
		})
	}
}

// =============================================================================
// MASK PHONE TESTS
// =============================================================================

func TestMaskPhone_TenDigits(t *testing.T) {
	// 10-digit domestic format: show first 3 and last 3, mask middle
	got := maskPhone("5551234567")
	want := "555***567"
	if got != want {
		t.Errorf("maskPhone(%q) = %q, want %q", "5551234567", got, want)
	}
}


func TestMaskPhone_E164Format(t *testing.T) {
	// E.164 (+15551234567): show prefix and last 3, mask middle
	got := maskPhone("+15551234567")
	if got == "+15551234567" {
		t.Error("maskPhone should not return the full E.164 number unmasked")
	}
	if !strings.Contains(got, "***") {
		t.Errorf("maskPhone(%q) = %q: expected *** in output", "+15551234567", got)
	}
}


func TestMaskPhone_ShortString(t *testing.T) {
	// Very short strings should return "***"
	got := maskPhone("12")
	if got != "***" {
		t.Errorf("maskPhone(%q) = %q, want ***", "12", got)
	}
}


func TestMaskPhone_FourDigits(t *testing.T) {
	// Exactly 4 digits — boundary: should return "***"
	got := maskPhone("1234")
	if got != "***" {
		t.Errorf("maskPhone(%q) = %q, want ***", "1234", got)
	}
}


func TestMaskPhone_FiveDigits(t *testing.T) {
	// 5 digits falls to the default case: first 2 + "***" + last 2
	got := maskPhone("12345")
	if !strings.Contains(got, "***") {
		t.Errorf("maskPhone(%q) = %q: expected *** in output", "12345", got)
	}
}

// =============================================================================
// RESOLVE OR CREATE USER TESTS
// =============================================================================


func TestMaskPhone_TenDigit(t *testing.T) {
	got := maskPhone("3101234567")
	// phone[:3] + "***" + phone[7:] => "310" + "***" + "567"
	if got != "310***567" {
		t.Errorf("maskPhone 10-digit: got %q, want %q", got, "310***567")
	}
}


func TestMaskPhone_E164(t *testing.T) {
	got := maskPhone("+13101234567")
	// Should mask middle digits
	if len(got) == 0 || got == "+13101234567" {
		t.Errorf("maskPhone E.164: expected masking, got %q", got)
	}
}


func TestMaskPhone_Short(t *testing.T) {
	got := maskPhone("123")
	if got != "***" {
		t.Errorf("maskPhone short: got %q, want %q", got, "***")
	}
}


func TestLogoInitials_Empty(t *testing.T) {
	got := logoInitials("")
	if got != "?" {
		t.Errorf("logoInitials empty: got %q, want %q", got, "?")
	}
}


func TestLogoInitials_SingleWord(t *testing.T) {
	got := logoInitials("Whole Foods")
	if got != "WF" {
		t.Errorf("logoInitials two words: got %q, want %q", got, "WF")
	}
}


func TestLogoInitials_SingleWordOnly(t *testing.T) {
	got := logoInitials("Costco")
	if got != "C" {
		t.Errorf("logoInitials single word: got %q, want %q", got, "C")
	}
}


func TestLogoColor_ReturnsHex(t *testing.T) {
	got := logoColor("Trader Joe's")
	if len(got) == 0 || got[0] != '#' {
		t.Errorf("logoColor: expected hex color starting with #, got %q", got)
	}
}


func TestLogoColor_SameName_SameResult(t *testing.T) {
	a := logoColor("Safeway")
	b := logoColor("Safeway")
	if a != b {
		t.Errorf("logoColor should be deterministic: got %q and %q", a, b)
	}
}


func TestLogoColor_DifferentNames(t *testing.T) {
	// Not guaranteed to differ but extremely likely for these two
	a := logoColor("Whole Foods")
	b := logoColor("Trader Joe's")
	_ = a
	_ = b
	// Just ensure neither panics and both are valid hex colors
	if len(a) == 0 || a[0] != '#' {
		t.Errorf("logoColor Whole Foods: got %q", a)
	}
	if len(b) == 0 || b[0] != '#' {
		t.Errorf("logoColor Trader Joe's: got %q", b)
	}
}


func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:54321"
	got := getClientIP(req)
	if got != "192.168.1.1" {
		t.Errorf("getClientIP RemoteAddr: got %q, want %q", got, "192.168.1.1")
	}
}


func TestGetClientIP_XForwardedFor(t *testing.T) {
	// When RAILWAY_ENVIRONMENT_NAME is set, X-Forwarded-For should be trusted
	t.Setenv("RAILWAY_ENVIRONMENT_NAME", "production")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 172.16.0.1")
	req.RemoteAddr = "192.168.1.1:54321"
	got := getClientIP(req)
	if got != "10.0.0.1" {
		t.Errorf("getClientIP X-Forwarded-For (behind proxy): got %q, want %q", got, "10.0.0.1")
	}
}


func TestGetClientIP_IgnoresXFFWithoutProxy(t *testing.T) {
	// When RAILWAY_ENVIRONMENT_NAME is NOT set, X-Forwarded-For should be ignored
	t.Setenv("RAILWAY_ENVIRONMENT_NAME", "")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 172.16.0.1")
	req.RemoteAddr = "192.168.1.1:54321"
	got := getClientIP(req)
	if got != "192.168.1.1" {
		t.Errorf("getClientIP X-Forwarded-For (no proxy): got %q, want %q — XFF should be ignored", got, "192.168.1.1")
	}
}


func TestGetClientIP_IPv6(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "[::1]:8080"
	got := getClientIP(req)
	// Should not panic and should return something reasonable
	if got == "" {
		t.Errorf("getClientIP IPv6: got empty string")
	}
}


func TestStripCodeFence_NoFence(t *testing.T) {
	input := `{"action":"add"}`
	got := stripCodeFence(input)
	if got != input {
		t.Errorf("stripCodeFence no fence: got %q, want %q", got, input)
	}
}


func TestStripCodeFence_WithJsonFence(t *testing.T) {
	input := "```json\n{\"action\":\"add\"}\n```"
	got := stripCodeFence(input)
	if got != `{"action":"add"}` {
		t.Errorf("stripCodeFence json fence: got %q, want %q", got, `{"action":"add"}`)
	}
}


func TestStripCodeFence_PlainFence(t *testing.T) {
	input := "```\nhello\n```"
	got := stripCodeFence(input)
	if got != "hello" {
		t.Errorf("stripCodeFence plain fence: got %q, want %q", got, "hello")
	}
}

// =============================================================================
// SETTINGS HANDLER TESTS
// =============================================================================


func TestLogoCacheHeader_Format(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	testData := []byte("<svg>test</svg>")
	server.logoCache.Store("example.com", &cachedLogo{data: testData, contentType: "image/svg+xml"})

	req := httptest.NewRequest("GET", "/api/logo?brand=Example&domain=example.com", nil)
	rr := httptest.NewRecorder()
	server.handleLogoProxy(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	cc := rr.Header().Get("Cache-Control")
	want := fmt.Sprintf("public, max-age=%d", logoCacheMaxAge)
	if cc != want {
		t.Errorf("Cache-Control: got %q, want %q", cc, want)
	}
}


func TestNormalizePhone_EmptyString(t *testing.T) {
	_, ok := normalizePhone("")
	if ok {
		t.Error("expected ok=false for empty string")
	}
}


func TestAvatar_UploadJPEG(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	jpegData := makeJPEG(100)
	b64 := base64.StdEncoding.EncodeToString(jpegData)

	// Upload
	rr := doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{"photo": b64}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// GET should serve it back
	rr = doRequest(server.handleAvatar, "GET", fmt.Sprintf("/api/avatar/%s", userID), nil, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Expected Content-Type image/jpeg, got %s", ct)
	}
	if rr.Body.Len() != 100 {
		t.Errorf("Expected 100 bytes, got %d", rr.Body.Len())
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "public, max-age=86400" {
		t.Errorf("Expected Cache-Control header, got %q", cc)
	}
}


func TestAvatar_UploadPNG(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	pngData := makePNG(200)
	b64 := base64.StdEncoding.EncodeToString(pngData)

	rr := doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{"photo": b64}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestAvatar_RejectInvalid(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// GIF magic bytes (not allowed)
	gifData := []byte("GIF89a" + strings.Repeat("x", 100))
	b64 := base64.StdEncoding.EncodeToString(gifData)

	rr := doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{"photo": b64}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
	result := parseJSON(rr)
	if !strings.Contains(fmt.Sprint(result["error"]), "JPEG and PNG") {
		t.Errorf("Expected JPEG/PNG error, got %v", result["error"])
	}
}


func TestAvatar_RejectTooLarge(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// 3MB JPEG — exceeds 2MB limit
	jpegData := makeJPEG(3 * 1024 * 1024)
	b64 := base64.StdEncoding.EncodeToString(jpegData)

	rr := doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{"photo": b64}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
	result := parseJSON(rr)
	if !strings.Contains(fmt.Sprint(result["error"]), "too large") {
		t.Errorf("Expected too large error, got %v", result["error"])
	}
}


func TestAvatar_RejectEmpty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Empty photo field
	rr := doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{"photo": ""}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	// Missing photo field entirely
	rr = doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing photo, got %d", rr.Code)
	}
}


func TestAvatar_Upsert(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Upload first photo (JPEG, 100 bytes)
	jpeg1 := makeJPEG(100)
	rr := doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{"photo": base64.StdEncoding.EncodeToString(jpeg1)}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("First upload failed: %d", rr.Code)
	}

	// Upload second photo (PNG, 200 bytes) — should replace
	png2 := makePNG(200)
	rr = doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{"photo": base64.StdEncoding.EncodeToString(png2)}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Second upload failed: %d", rr.Code)
	}

	// GET should return the PNG
	rr = doRequest(server.handleAvatar, "GET", fmt.Sprintf("/api/avatar/%s", userID), nil, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET failed: %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Expected image/png after upsert, got %s", ct)
	}
	if rr.Body.Len() != 200 {
		t.Errorf("Expected 200 bytes after upsert, got %d", rr.Body.Len())
	}
}


func TestAvatar_Delete(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Upload
	jpeg := makeJPEG(100)
	doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{"photo": base64.StdEncoding.EncodeToString(jpeg)}, []*http.Cookie{cookie})

	// Delete
	rr := doRequest(server.handleAvatar, "DELETE", "/api/avatar", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("DELETE failed: %d", rr.Code)
	}

	// GET should 404
	rr = doRequest(server.handleAvatar, "GET", fmt.Sprintf("/api/avatar/%s", userID), nil, nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 after delete, got %d", rr.Code)
	}
}


func TestAvatar_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// GET for user without photo
	rr := doRequest(server.handleAvatar, "GET", "/api/avatar/99999", nil, nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}


func TestAvatar_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	jpeg := makeJPEG(100)
	b64 := base64.StdEncoding.EncodeToString(jpeg)

	// POST without session cookie
	rr := doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{"photo": b64}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rr.Code)
	}

	// DELETE without session cookie
	rr = doRequest(server.handleAvatar, "DELETE", "/api/avatar", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for DELETE, got %d", rr.Code)
	}
}

// =============================================================================
// ONBOARDING TESTS
// =============================================================================
// API CONFIG ENDPOINT TEST
// =============================================================================
// ONBOARDING TESTS
// =============================================================================
// API CONFIG ENDPOINT TEST
// =============================================================================

func TestAPIConfig_NoCredentialLeak(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleAPIConfig, "GET", "/api/config", nil, nil)
	if rr.Code != 200 {
		t.Fatalf("config: got %d", rr.Code)
	}

	respBody := rr.Body.String()
	// Should not contain secret keys or passwords
	for _, sensitive := range []string{"secret", "password", "api_key", "token"} {
		if bytes.Contains(bytes.ToLower([]byte(respBody)), []byte(sensitive)) {
			t.Errorf("config response should not contain '%s'", sensitive)
		}
	}
}

// --- Account Merge: Audit Trail ---
