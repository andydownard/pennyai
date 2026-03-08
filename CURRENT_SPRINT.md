# PennyAI — Current Sprint (March 7-14, 2026)

> This file is read by Claude at the start of every session. Keep it updated.

## Sprint Goal

Ship real-priced brand swaps with Walmart/BlueCart pricing, add Sentry + PostHog observability, begin conversational onboarding.

## Product Team Convene Summary

- **PM:** Real pricing is the #1 gap. Without accurate store prices, savings claims are meaningless. BlueCart API (Walmart data) gives us real shelf prices by zip code.
- **Engineer:** BlueCart is simple GET + api_key. Cache prices 6h. Use user's zip from address setting.
- **UX Designer:** Graceful degradation critical — show discount-only when price unavailable. "Prices from Walmart" attribution. Unit price must be prominent.
- **QA:** Mock BlueCart in tests. Existing swap tests must still pass.
- **Security:** API key server-side only. Rate limit our calls to stay under 10K/day.

## Focus Group Consensus

- Real pricing from a named source (Walmart) is a trust builder (Linda, Marcus)
- Unit price ($/oz) must be shown (Priya, Devon)
- "Prices at Walmart near [zip]" attribution required (Marcus)
- Brand-level swaps > item-level swaps (Devon)
- Graceful degradation when price unavailable — discount only (Tamika)
- Keep savings amount prominent, price comparison is secondary (Nadia)

## Sprint Backlog

| # | Item | Size | Priority | Status | Owner |
|---|------|------|----------|--------|-------|
| S1 | BlueCart API + Real-Priced Brand Swaps + Seed $20 | M | P0 | Planned | Claude |
| S2 | Sentry Integration | XS | P0 | Planned | Claude |
| S3 | PostHog Integration | S | P0 | Planned | Claude |
| S4 | R1 Conversational Onboarding (start) | S | P1 | Planned | Claude |

## What's Done (Previous Sprint)

| Item | Notes |
|------|-------|
| M1 Penny Swaps v0 | Consumer swap cards, matching, suppression |
| M2 Sharing & Invite Flow | SMS notify inviter, Penny welcome, editable list name, referral source |
| M3 Swap Admin Tooling | CRUD for brand_deals, admin dashboard |
| B11-B18 all fixed | Budget race, rebate farming, FK delete, unit_label, budget rename, PennyBar timing |
| Swap persistence on reload | GET /api/list/swaps endpoint |
| ItemEditSheet scroll fix | min-height: 0 + overscroll-behavior |
| Header cleanup | Share button removed from home header |
| UX Designer role | Added to product team in CLAUDE.md |

## Where I Left Off

> **Claude: update this section at the end of every session.**

_Last updated: March 7, 2026_

- Sprint spec complete, ready to execute
- Next step: S2 (Sentry) → S3 (PostHog) → S1 (BlueCart + seed deals)
- Blockers: Need `BLUECART_API_KEY` env var (user must sign up at app.bluecartapi.com/signup)

## Key Metrics to Watch

| Metric | Current | Target | How to Check |
|--------|---------|--------|-------------|
| Swap acceptance rate | N/A | >= 8% at week 4 | PostHog funnel |
| Swaps offered/day | ~5 (test) | 50 at week 4 | PostHog event count |
| Real pricing coverage | 0% | >= 80% of deals | `SELECT COUNT(*) FROM price_cache WHERE fetched_at > datetime('now', '-6 hours')` |
| Error rate | Unknown | < 1% | Sentry dashboard |

## Next Sprint Preview

- M4 Brand Portal v0 (brands.thrift.ly)
- M5 Stripe Payment
- R5 E2E Tests
- R1 Onboarding (continued)
