package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// brandDigestData holds the metrics for a brand's weekly email digest.
type brandDigestData struct {
	CompanyName       string
	WeekLabel         string
	TotalOffered      int
	TotalAccepted     int
	TotalDeclined     int
	AcceptRate        int
	SpentCentsThisWeek int
	VerifiedPurchases int
	PendingClaims     int
	BalanceCents      int
	ActiveDeals       int
	DashboardURL      string
}

// runBrandDigestLoop starts a background goroutine that checks hourly
// whether any brand users are due for a weekly email digest.
// Sends at most one digest per brand per 7 days. Idempotent and safe.
func (s *Server) runBrandDigestLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		s.sendPendingBrandDigests()
	}
}

// sendPendingBrandDigests finds all brand users who haven't received
// a digest in 7+ days and sends them one.
func (s *Server) sendPendingBrandDigests() {
	if s.config.SendGridAPIKey == "" {
		return // no email capability
	}

	cutoff := time.Now().Add(-7 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	rows, err := s.db.Query(`
		SELECT id, email, company_name FROM brand_users
		WHERE digest_opt_out = 0
		AND (last_digest_at IS NULL OR last_digest_at < ?)
	`, cutoff)
	if err != nil {
		log.Printf("[brand-digest] query error: %v", err)
		return
	}
	defer rows.Close()

	type brandUser struct {
		id          int
		email       string
		companyName string
	}
	var users []brandUser
	for rows.Next() {
		var u brandUser
		rows.Scan(&u.id, &u.email, &u.companyName)
		users = append(users, u)
	}

	for _, u := range users {
		if err := s.sendBrandDigest(u.id, u.email, u.companyName); err != nil {
			log.Printf("[brand-digest] failed for brand_user %d (%s): %v", u.id, u.email, err)
		} else {
			log.Printf("[brand-digest] sent to %s (%s)", u.email, u.companyName)
		}
	}
}

// sendBrandDigest composes and sends a weekly email digest for one brand user.
func (s *Server) sendBrandDigest(brandUserID int, email, companyName string) error {
	now := time.Now()
	weekStart := now.Add(-7 * 24 * time.Hour).Format("2006-01-02")
	weekEnd := now.Format("2006-01-02")
	weekLabel := fmt.Sprintf("%s – %s",
		now.Add(-7*24*time.Hour).Format("Jan 2"),
		now.Format("Jan 2, 2006"))

	// Gather metrics for this brand's deals
	var totalOffered, totalAccepted, totalDeclined int
	s.db.QueryRow(`
		SELECT COUNT(*) FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE bd.brand_user_id = ? AND se.action = 'offered' AND se.created_at >= ? AND se.created_at < ?
	`, brandUserID, weekStart, weekEnd).Scan(&totalOffered)

	s.db.QueryRow(`
		SELECT COUNT(*) FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE bd.brand_user_id = ? AND se.action = 'accepted' AND se.created_at >= ? AND se.created_at < ?
	`, brandUserID, weekStart, weekEnd).Scan(&totalAccepted)

	s.db.QueryRow(`
		SELECT COUNT(*) FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE bd.brand_user_id = ? AND se.action = 'declined' AND se.created_at >= ? AND se.created_at < ?
	`, brandUserID, weekStart, weekEnd).Scan(&totalDeclined)

	acceptRate := 0
	if totalOffered > 0 {
		acceptRate = totalAccepted * 100 / totalOffered
	}

	var spentCents int
	s.db.QueryRow(`
		SELECT COALESCE(SUM(bd.discount_cents), 0) FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE bd.brand_user_id = ? AND se.action = 'accepted' AND se.created_at >= ? AND se.created_at < ?
	`, brandUserID, weekStart, weekEnd).Scan(&spentCents)

	var verifiedPurchases, pendingClaims int
	s.db.QueryRow(`
		SELECT COUNT(*) FROM swap_claims sc
		JOIN brand_deals bd ON bd.id = sc.deal_id
		WHERE bd.brand_user_id = ? AND sc.status = 'verified' AND sc.created_at >= ? AND sc.created_at < ?
	`, brandUserID, weekStart, weekEnd).Scan(&verifiedPurchases)

	s.db.QueryRow(`
		SELECT COUNT(*) FROM swap_claims sc
		JOIN brand_deals bd ON bd.id = sc.deal_id
		WHERE bd.brand_user_id = ? AND sc.status = 'pending'
	`, brandUserID).Scan(&pendingClaims)

	var balanceCents int
	s.db.QueryRow(`SELECT COALESCE(balance_cents, 0) FROM brand_balance WHERE brand_user_id = ?`,
		brandUserID).Scan(&balanceCents)

	var activeDeals int
	s.db.QueryRow(`SELECT COUNT(*) FROM brand_deals WHERE brand_user_id = ? AND status = 'active'`,
		brandUserID).Scan(&activeDeals)

	dashURL := s.config.WebhookBaseURL
	if dashURL == "" {
		dashURL = "http://localhost:" + s.config.Port
	}

	data := brandDigestData{
		CompanyName:        companyName,
		WeekLabel:          weekLabel,
		TotalOffered:       totalOffered,
		TotalAccepted:      totalAccepted,
		TotalDeclined:      totalDeclined,
		AcceptRate:         acceptRate,
		SpentCentsThisWeek: spentCents,
		VerifiedPurchases:  verifiedPurchases,
		PendingClaims:      pendingClaims,
		BalanceCents:       balanceCents,
		ActiveDeals:        activeDeals,
		DashboardURL:       dashURL,
	}

	html := buildBrandDigestHTML(data)
	subject := fmt.Sprintf("Your Thriftly Weekly Recap — %s", data.WeekLabel)

	if err := s.sendTransactionalEmail(email, subject, html); err != nil {
		return err
	}

	// Mark digest as sent (idempotency: update timestamp)
	s.db.Exec(`UPDATE brand_users SET last_digest_at = datetime('now') WHERE id = ?`, brandUserID)
	return nil
}

// buildBrandDigestHTML generates the HTML email body for the weekly brand digest.
func buildBrandDigestHTML(d brandDigestData) string {
	spent := fmt.Sprintf("$%d.%02d", d.SpentCentsThisWeek/100, d.SpentCentsThisWeek%100)
	balance := fmt.Sprintf("$%d.%02d", d.BalanceCents/100, d.BalanceCents%100)

	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;padding:0;background:#f5f5f5;font-family:'Avenir','Avenir Next',-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
<table width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;background:#fff;border-radius:12px;overflow:hidden;margin-top:24px;margin-bottom:24px;">
<tr><td style="background:#35644F;padding:24px 28px;text-align:center;">
  <div style="color:#fff;font-size:24px;font-weight:700;">thriftly</div>
  <div style="color:rgba(255,255,255,0.85);font-size:13px;margin-top:4px;">Weekly Brand Recap</div>
</td></tr>
<tr><td style="padding:28px;">
  <div style="font-size:16px;color:#333;margin-bottom:16px;">Hi %s,</div>
  <div style="font-size:14px;color:#666;margin-bottom:20px;">Here's how your campaigns performed this week (%s):</div>

  <!-- Metrics grid -->
  <table width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:20px;">
  <tr>
    <td width="50%%" style="padding:12px;background:#f9f9f9;border-radius:8px;">
      <div style="font-size:28px;font-weight:700;color:#35644F;">%d</div>
      <div style="font-size:12px;color:#888;">Swaps Offered</div>
    </td>
    <td width="8"></td>
    <td width="50%%" style="padding:12px;background:#f9f9f9;border-radius:8px;">
      <div style="font-size:28px;font-weight:700;color:#35644F;">%d</div>
      <div style="font-size:12px;color:#888;">Accepted</div>
    </td>
  </tr>
  <tr><td colspan="3" height="8"></td></tr>
  <tr>
    <td width="50%%" style="padding:12px;background:#f9f9f9;border-radius:8px;">
      <div style="font-size:28px;font-weight:700;color:#35644F;">%d%%</div>
      <div style="font-size:12px;color:#888;">Accept Rate</div>
    </td>
    <td width="8"></td>
    <td width="50%%" style="padding:12px;background:#f9f9f9;border-radius:8px;">
      <div style="font-size:28px;font-weight:700;color:#35644F;">%d</div>
      <div style="font-size:12px;color:#888;">Verified Purchases</div>
    </td>
  </tr>
  </table>

  <!-- Summary -->
  <table width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:24px;border:1px solid #eee;border-radius:8px;">
  <tr>
    <td style="padding:12px 16px;border-bottom:1px solid #eee;">
      <span style="font-size:13px;color:#888;">Spent this week</span>
      <span style="float:right;font-size:14px;font-weight:600;color:#333;">%s</span>
    </td>
  </tr>
  <tr>
    <td style="padding:12px 16px;border-bottom:1px solid #eee;">
      <span style="font-size:13px;color:#888;">Account balance</span>
      <span style="float:right;font-size:14px;font-weight:600;color:#333;">%s</span>
    </td>
  </tr>
  <tr>
    <td style="padding:12px 16px;border-bottom:1px solid #eee;">
      <span style="font-size:13px;color:#888;">Active deals</span>
      <span style="float:right;font-size:14px;font-weight:600;color:#333;">%d</span>
    </td>
  </tr>
  <tr>
    <td style="padding:12px 16px;">
      <span style="font-size:13px;color:#888;">Pending claims</span>
      <span style="float:right;font-size:14px;font-weight:600;color:#333;">%d</span>
    </td>
  </tr>
  </table>

  <!-- CTA -->
  <div style="text-align:center;margin-bottom:24px;">
    <a href="%s" style="display:inline-block;background:#35644F;color:#fff;text-decoration:none;padding:14px 32px;border-radius:8px;font-size:15px;font-weight:600;">View Dashboard</a>
  </div>

  <div style="font-size:12px;color:#bbb;text-align:center;line-height:1.5;">
    You're receiving this because you have an active brand account on Thriftly.<br>
    <a href="%s/brand/unsubscribe" style="color:#888;">Unsubscribe</a> from weekly recaps.
  </div>
</td></tr>
</table>
</body></html>`,
		escapeHTML(d.CompanyName), d.WeekLabel,
		d.TotalOffered, d.TotalAccepted,
		d.AcceptRate, d.VerifiedPurchases,
		spent, balance,
		d.ActiveDeals, d.PendingClaims,
		d.DashboardURL, d.DashboardURL,
	)
}

// escapeHTML escapes a string for safe inclusion in HTML.
func escapeHTML(s string) string {
	replacer := map[rune]string{
		'<': "&lt;", '>': "&gt;", '&': "&amp;", '"': "&quot;", '\'': "&#39;",
	}
	result := []byte{}
	for _, c := range s {
		if r, ok := replacer[c]; ok {
			result = append(result, []byte(r)...)
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}

// handleBrandUnsubscribe handles GET/POST /brand/unsubscribe — opt-out of weekly digests.
func (s *Server) handleBrandUnsubscribe(w http.ResponseWriter, r *http.Request) {
	// Check brand session without automatic 401 JSON response
	var brandUserID int
	if cookie, err := r.Cookie("brand_session"); err == nil {
		if bs, ok := s.getBrandSession(cookie.Value); ok {
			brandUserID = bs.BrandUserID
		}
	}

	if brandUserID == 0 {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body style="font-family:sans-serif;text-align:center;padding:60px 20px;">
		<h2>Unsubscribe from Thriftly Weekly Recaps</h2>
		<p>Please <a href="/brand/login">log in</a> to manage your email preferences.</p>
		</body></html>`))
		return
	}

	if r.Method == http.MethodPost {
		s.db.Exec("UPDATE brand_users SET digest_opt_out = 1 WHERE id = ?", brandUserID)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body style="font-family:sans-serif;text-align:center;padding:60px 20px;">
		<h2 style="color:#35644F;">Unsubscribed</h2>
		<p>You won't receive weekly recap emails anymore.</p>
		<p><a href="/brand/dashboard">Back to Dashboard</a></p>
		</body></html>`))
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<html><body style="font-family:sans-serif;text-align:center;padding:60px 20px;">
	<h2>Unsubscribe from Weekly Recaps?</h2>
	<p>You'll stop receiving weekly performance summaries.</p>
	<form method="POST"><button type="submit" style="background:#35644F;color:#fff;border:none;padding:12px 32px;border-radius:8px;font-size:15px;cursor:pointer;">Unsubscribe</button></form>
	</body></html>`))
}
