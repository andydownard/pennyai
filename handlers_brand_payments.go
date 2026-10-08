package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// BRAND TRANSACTIONS
// =============================================================================

func (s *Server) handleBrandTransactions(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT id, type, amount_cents, description, COALESCE(stripe_id, ''), created_at
		FROM brand_transactions WHERE brand_user_id = ?
		ORDER BY created_at DESC LIMIT 100`, bs.BrandUserID)
	if err != nil {
		dbError(w, err, "Failed to load transactions")
		return
	}
	defer rows.Close()

	type txn struct {
		ID          int    `json:"id"`
		Type        string `json:"type"`
		AmountCents int    `json:"amount_cents"`
		Description string `json:"description"`
		StripeID    string `json:"stripe_id,omitempty"`
		CreatedAt   string `json:"created_at"`
	}
	txns := []txn{}
	for rows.Next() {
		var t txn
		rows.Scan(&t.ID, &t.Type, &t.AmountCents, &t.Description, &t.StripeID, &t.CreatedAt)
		txns = append(txns, t)
	}

	writeJSON(w, http.StatusOK, txns)
}

// =============================================================================
// CSV EXPORT (M9)
// =============================================================================

func (s *Server) handleBrandExport(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	exportType := r.URL.Query().Get("type")
	if exportType == "" {
		exportType = "swaps"
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s_export_%s.csv", exportType, time.Now().Format("2006-01-02")))

	switch exportType {
	case "swaps":
		w.Write([]byte("date,deal_id,competitor_product,offer_product,action,user_id\n"))
		rows, err := s.db.Query(`
			SELECT COALESCE(se.created_at,''), se.deal_id, COALESCE(bd.competitor_product,''),
				   COALESCE(bd.offer_product,''), se.action, se.user_id
			FROM swap_events se
			JOIN brand_deals bd ON se.deal_id = bd.id
			WHERE bd.brand_user_id = ?
			ORDER BY se.created_at DESC`, bs.BrandUserID)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var date, comp, offer, action string
			var dealID int
			var userID string
			rows.Scan(&date, &dealID, &comp, &offer, &action, &userID)
			fmt.Fprintf(w, "%s,%d,%q,%q,%s,%s\n", date, dealID, comp, offer, action, userID)
		}

	case "transactions":
		w.Write([]byte("date,type,amount_cents,description,stripe_id\n"))
		rows, err := s.db.Query(`
			SELECT COALESCE(created_at,''), type, amount_cents, COALESCE(description,''), COALESCE(stripe_id,'')
			FROM brand_transactions WHERE brand_user_id = ?
			ORDER BY created_at DESC`, bs.BrandUserID)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var date, txType, desc, stripeID string
			var amount int
			rows.Scan(&date, &txType, &amount, &desc, &stripeID)
			fmt.Fprintf(w, "%s,%s,%d,%q,%s\n", date, txType, amount, desc, stripeID)
		}

	case "deals":
		w.Write([]byte("id,competitor_product,offer_brand,offer_product,discount_cents,budget_cents,spent_cents,status,deal_type,created_at\n"))
		rows, err := s.db.Query(`
			SELECT id, COALESCE(competitor_product,''), COALESCE(offer_brand,''), COALESCE(offer_product,''),
				   discount_cents, total_budget_cents, budget_spent_cents, status,
				   COALESCE(deal_type,'sku_swap'), COALESCE(created_at,'')
			FROM brand_deals WHERE brand_user_id = ? AND status != 'deleted'
			ORDER BY created_at DESC`, bs.BrandUserID)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var comp, brand, offer, status, dealType, created string
			var id, discount, budget, spent int
			rows.Scan(&id, &comp, &brand, &offer, &discount, &budget, &spent, &status, &dealType, &created)
			fmt.Fprintf(w, "%d,%q,%q,%q,%d,%d,%d,%s,%s,%s\n", id, comp, brand, offer, discount, budget, spent, status, dealType, created)
		}

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid export type. Use: swaps, transactions, deals"})
	}
}

// =============================================================================
// STRIPE DEPOSIT (M5)
// =============================================================================

func (s *Server) handleBrandDeposit(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	var req struct {
		AmountCents int `json:"amount_cents"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.AmountCents < 5000 { // $50 minimum
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Minimum deposit is $50.00"})
		return
	}

	// If Stripe is configured, create a Checkout Session (redirect flow).
	// Otherwise, fall back to direct credit (dev/test mode).
	if s.config.StripeSecretKey != "" {
		// Stripe must redirect back to the BRAND PORTAL.
		// Hardcode to the known working domain — Origin header is unreliable
		// (may be empty behind proxies, or may contain a non-resolving subdomain).
		baseURL := "https://brands.groflo.io"
		log.Printf("[stripe] deposit: using baseURL=%s (origin was %q)", baseURL, r.Header.Get("Origin"))

		form := buildDepositCheckoutForm(bs.BrandUserID, req.AmountCents, baseURL)

		httpReq, _ := http.NewRequest("POST", "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(form.Encode()))
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		httpReq.SetBasicAuth(s.config.StripeSecretKey, "")

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			log.Printf("[stripe] checkout session request failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Payment service unavailable"})
			return
		}
		defer resp.Body.Close()

		var session struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&session); err != nil || session.URL == "" {
			log.Printf("[stripe] failed to parse checkout session response (status %d): %v", resp.StatusCode, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Payment service error"})
			return
		}

		log.Printf("[brand-portal] created Stripe checkout session %s for brand user %s (%d cents)", session.ID, bs.BrandUserID, req.AmountCents)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"checkout_url": session.URL,
		})
		return
	}

	// Dev/test fallback: credit directly
	_, err := s.db.Exec(`
		UPDATE brand_balance SET balance_cents = balance_cents + ?, last_deposit_at = CURRENT_TIMESTAMP
		WHERE brand_user_id = ?`, req.AmountCents, bs.BrandUserID)
	if err != nil {
		dbError(w, err, "Failed to process deposit")
		return
	}
	s.db.Exec(`INSERT INTO brand_transactions (brand_user_id, type, amount_cents, description, stripe_id)
		VALUES (?, 'deposit', ?, 'Balance deposit (test)', '')`, bs.BrandUserID, req.AmountCents)
	log.Printf("[brand-portal] brand user %s deposited %d cents (test mode)", bs.BrandUserID, req.AmountCents)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "Deposit processed",
		"amount_cents": req.AmountCents,
	})
}

// =============================================================================
// STRIPE WEBHOOK (M5)
// =============================================================================

func (s *Server) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	// Read raw body
	r.Body = http.MaxBytesReader(w, r.Body, 1*1024*1024)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to read body"})
		return
	}

	// Verify Stripe signature if webhook secret is configured
	if s.config.StripeWebhookSecret != "" {
		sig := r.Header.Get("Stripe-Signature")
		if !verifyStripeSignature(body, sig, s.config.StripeWebhookSecret) {
			log.Printf("[stripe] webhook signature verification failed")
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid signature"})
			return
		}
	} else {
		log.Printf("[stripe] WARNING: webhook received without signature verification (STRIPE_WEBHOOK_SECRET not set)")
	}

	// Parse the event
	var event struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid payload"})
		return
	}

	switch event.Type {
	case "payment_intent.succeeded":
		// Parse payment intent and credit brand balance
		var pi struct {
			Object struct {
				ID       string `json:"id"`
				Amount   int    `json:"amount"`
				Metadata struct {
					BrandUserID string `json:"brand_user_id"`
				} `json:"metadata"`
			} `json:"object"`
		}
		if err := json.Unmarshal(event.Data, &pi); err != nil {
			log.Printf("[stripe] failed to parse payment_intent: %v", err)
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid payload"})
			return
		}

		brandUserID := pi.Object.Metadata.BrandUserID
		if brandUserID == "" {
			log.Printf("[stripe] payment_intent missing brand_user_id metadata")
			w.WriteHeader(http.StatusOK) // Acknowledge to prevent retries
			return
		}

		if err := s.creditBrandDeposit(brandUserID, pi.Object.ID, pi.Object.Amount); err != nil {
			// Tell Stripe to retry: nothing was committed.
			log.Printf("[stripe] crediting payment_intent %s failed: %v", pi.Object.ID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not record deposit"})
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// buildDepositCheckoutForm builds the form body for a brand deposit Stripe
// Checkout Session. Extracted so the encoded body — in particular the required
// metadata — is testable without calling Stripe.
//
// CLAUDE.md Revenue Rules: Stripe metadata MUST include source: "pennyai" on all
// charges. Without it, attribution is broken for every deposit.
func buildDepositCheckoutForm(brandUserID string, amountCents int, baseURL string) url.Values {
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("line_items[0][price_data][currency]", "usd")
	form.Set("line_items[0][price_data][product_data][name]", "Groflo Brand Account Deposit")
	form.Set("line_items[0][price_data][unit_amount]", strconv.Itoa(amountCents))
	form.Set("line_items[0][quantity]", "1")
	form.Set("payment_intent_data[metadata][brand_user_id]", brandUserID)
	form.Set("payment_intent_data[metadata][source]", "pennyai")
	form.Set("success_url", baseURL+"?deposit=success")
	form.Set("cancel_url", baseURL+"?deposit=cancel")
	return form
}

// creditBrandDeposit records one Stripe deposit exactly once.
//
// The ledger INSERT is the FIRST mutation and carries ON CONFLICT DO NOTHING
// against the partial unique index on brand_transactions(stripe_id), so the
// storage layer decides the winner. A duplicate delivery affects zero rows and
// returns nil without touching the balance — Stripe gets its 200 and stops
// retrying. The balance UPDATE runs in the same transaction as the ledger
// INSERT, per the Revenue Rule that wallet balances never move outside the
// transaction holding the ledger row.
//
// A nil return means "settled" (credited now, or already credited earlier).
// A non-nil return means nothing was committed and the caller should let
// Stripe retry.
func (s *Server) creditBrandDeposit(brandUserID, stripeID string, amountCents int) error {
	if stripeID == "" {
		return fmt.Errorf("refusing to credit a deposit with an empty stripe id")
	}
	if amountCents <= 0 {
		return fmt.Errorf("refusing to credit a non-positive amount (%d cents)", amountCents)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Claim the payment_intent. Losers of the race affect zero rows.
	res, err := tx.Exec(`INSERT INTO brand_transactions (brand_user_id, type, amount_cents, description, stripe_id)
		VALUES (?, 'deposit', ?, 'Stripe deposit', ?) ON CONFLICT DO NOTHING`,
		brandUserID, amountCents, stripeID)
	if err != nil {
		return fmt.Errorf("recording ledger row: %w", err)
	}
	claimed, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("reading ledger insert result: %w", err)
	}
	if claimed != 1 {
		log.Printf("[stripe] duplicate payment_intent %s, already credited — skipping", stripeID)
		return nil
	}

	// Credit the balance in the same transaction. Upsert, so a brand with no
	// balance row yet never has its deposit silently swallowed.
	res, err = tx.Exec(`INSERT INTO brand_balance (brand_user_id, balance_cents, last_deposit_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (brand_user_id) DO UPDATE SET
			balance_cents = brand_balance.balance_cents + ?,
			last_deposit_at = CURRENT_TIMESTAMP`,
		brandUserID, amountCents, amountCents)
	if err != nil {
		return fmt.Errorf("crediting balance: %w", err)
	}
	credited, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("reading balance update result: %w", err)
	}
	if credited != 1 {
		return fmt.Errorf("balance credit affected %d rows, want 1", credited)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing deposit: %w", err)
	}

	log.Printf("[stripe] credited %d cents to brand user %s (pi: %s)", amountCents, brandUserID, stripeID)
	return nil
}

// verifyStripeSignature verifies a Stripe webhook signature (v1 scheme).
// Format: "t=timestamp,v1=signature[,v1=signature...]"
func verifyStripeSignature(payload []byte, sigHeader, secret string) bool {
	if sigHeader == "" {
		return false
	}

	var timestamp string
	var signatures []string
	for _, part := range strings.Split(sigHeader, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}

	if timestamp == "" || len(signatures) == 0 {
		return false
	}

	// Replay protection: reject if timestamp is older than 5 minutes
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if math.Abs(float64(time.Now().Unix()-ts)) > 300 {
		return false
	}

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expected)) {
			return true
		}
	}
	return false
}
