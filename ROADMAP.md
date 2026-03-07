# Thriftly Product Roadmap

> **GUARDRAIL: Never delete items from this roadmap without explicit approval from the founder.** Items that are completed or deprioritized should be moved to the appropriate section (Completed, Nice-to-Have, or Not In Scope) — never removed. Always prompt before modifying priority order.

## Product Team

| Role | Responsibility |
|------|----------------|
| **PM** (conducts) | Prioritizes backlog, defines acceptance criteria, runs reviews, owns the roadmap |
| **UX** | User research, interaction design, usability testing, accessibility |
| **Architect** | System design, scaling strategy, tech debt management, ADRs |
| **Staff Dev** | Core implementation, code quality, mentorship, cross-cutting concerns |
| **DevOps** | CI/CD, infrastructure, monitoring, deployments, incident response |
| **BI / Reporting** | Analytics, metrics dashboards, funnel analysis, experiment design |
| **CX (Voice of Customer)** | User feedback synthesis, support patterns, NPS analysis, feature requests |

---

## Focus Group Findings — CPG Brand Product Managers

> Three panels of 6 brand product managers each (18 total) conducted March 2026. Panels structured as: (A) Challenger/DTC brands (Siete, GoodBowl, Banza, Hu Kitchen, Liquid IV, Magic Spoon), (B) Mid-market CPG (Bob's Red Mill, Annie's/General Mills sub-brand, Chobani, Rxbar, Sir Kensington's, Primal Kitchen), (C) Enterprise CPG (P&G brand lead, Unilever brand lead, Kraft Heinz, Nestlé, Colgate-Palmolive, J&M Smucker).

### Q1: Would you pay $1-2 per swap on a grocery list app?

| Panel | Response | Key Quotes |
|-------|----------|------------|
| **A — Challenger** | 6/6 yes, 4/6 said "that's cheaper than anything we run" | "My Meta CPA is $18. A dollar for someone who already has my competitor in their cart? I'd pay $5." — Banza PM |
| **B — Mid-market** | 5/6 yes, 1 wanted volume pricing | "At $2 I need to see it work. At $1 it's an impulse pilot budget." — Chobani PM |
| **C — Enterprise** | 2/6 definite yes, 4/6 said "interesting but need scale proof" | "We can't justify a new vendor for <50K impressions/month. But the intent signal is unlike anything else." — P&G brand lead |

**Takeaway:** Challenger and mid-market brands will pay immediately at $1-2/swap. Enterprise needs scale but is intrigued by intent targeting. **Start with challenger brands. Price at $1.50/swap, offer $1.00 "Founding Brand" rate for first 10 brands.**

### Q2: What reporting would you need to justify the spend?

| Rank | Metric | Mentioned By |
|------|--------|-------------|
| 1 | **Swap acceptance rate** (offered vs accepted) | 16/18 |
| 2 | **Category share shift** (your brand vs competitor in swaps) | 14/18 |
| 3 | **Redemption/purchase confirmation** (receipt proof) | 13/18 |
| 4 | **Cost per acquired customer vs other channels** | 11/18 |
| 5 | **Geographic/demographic breakdown** | 8/18 |
| 6 | **Repeat purchase rate** (did they swap again next trip?) | 7/18 |

**Takeaway:** Ship swap acceptance rate and category share on day 1. Receipt-based purchase confirmation is the "wow" feature that closes enterprise deals later. Demographics can wait.

### Q3: Would you sign up with just a corporate email, or do you need a sales call?

| Panel | Self-serve only | Self-serve + optional call | Sales call required |
|-------|----------------|---------------------------|-------------------|
| **A — Challenger** | 5/6 | 1/6 | 0/6 |
| **B — Mid-market** | 3/6 | 3/6 | 0/6 |
| **C — Enterprise** | 0/6 | 1/6 | 5/6 |

**Takeaway:** Self-serve with corporate email validation is correct for launch. Add a "Request Demo" button for enterprise. No sales team needed yet — Andy does founder-led sales for the first 10.

### Q4: If you could see your competitors' products being swapped out for yours, would that drive urgency?

**Unanimous yes across all 18 participants.** This was the single most visceral reaction in all three panels.

| Key Quotes |
|------------|
| "If I saw Heinz ketchup getting swapped for Sir Kensington's 50 times last week, I'd sign up today." — Kraft Heinz PM |
| "That's the data we literally cannot get anywhere else. Not from Nielsen, not from IRI, not from Instacart." — Unilever PM |
| "Competitor swap data is the hook. The swap itself is just the mechanism." — Magic Spoon PM |

**Takeaway:** The "Competitive Swap Intel" view is the killer feature for the brand dashboard. Show anonymized, aggregated data: "Last 7 days: [Competitor X] was swapped out for [other brands] N times in [category]." This drives FOMO-based sign-ups. **Build this into the Phase 2 dashboard — it's the single best brand acquisition tool.**

### Q5: What's the minimum feature set for you to run a pilot?

Aggregated across all 18, the minimum viable pilot requires:

1. **Swap creation** — I can pick my product, pick the competitor, set the offer ($X off)
2. **Swap delivery** — It shows up when someone has the competitor on their list
3. **Acceptance tracking** — I can see how many people swapped
4. **Spend tracking** — I can see how much I've spent and set a budget cap
5. **Corporate email login** — No procurement, no contracts, credit card billing

Items explicitly called out as NOT required for a pilot: receipt verification, demographic targeting, API access, multi-user team accounts, geographic targeting.

**Takeaway:** The MVP is simpler than we thought. Swap CRUD + basic dashboard + Stripe billing = pilot-ready.

---

## Phase 1 — Penny Swaps MVP + Sharing Fixes (Weeks 1-2)

> **North star:** Get 5 swaps accepted by real users. Prove the unit economics.

### M1. Penny Swaps v0 — Consumer Experience
**Size:** L (2 weeks)
**Priority:** P0 — Revenue blocker
**Status:** Planned

**Description:**
When a user's grocery list contains an item that matches an active brand deal, Penny suggests a swap conversationally. "Hey, I see you have Heinz ketchup — want to try Sir Kensington's? You'd save $1.50." The swap appears inline in the list as a dismissible card below the original item. User taps "Swap" or "No thanks." Penny remembers preferences — if you decline a brand 3 times, she stops suggesting it.

**CX Insight:** Swaps must feel like a helpful friend's recommendation, not an ad. The moment it feels like sponsored content, trust in Penny collapses. Framing matters: "save money" not "try this brand."

**UX Notes:** Swap card appears directly below the matched item in the list. Shows: brand logo, product name, savings amount, "Swap" and "No thanks" buttons. Subtle "Suggested by Penny" label. Card disappears on dismiss with a gentle animation. No interstitials, no modals, no full-screen takeovers.

**Architect Notes:**
- New tables: `brand_deals` (brand_id, competitor_product, offer_product, discount_cents, budget_remaining_cents, status, created_at), `swap_events` (user_id, deal_id, action [offered/accepted/declined], created_at)
- Matching logic in the parse response: after Penny processes a list addition, check `brand_deals` for any active deal where `competitor_product` fuzzy-matches the added item
- Penny's system prompt gets swap awareness: when a match exists, she weaves the suggestion into her response naturally
- Frequency cap: max 2 swap suggestions per session, max 1 per item
- Decline tracking: 3 declines for the same brand = suppress for 30 days

**Staff Dev Notes:** Fuzzy matching is the hard part. Start with exact UPC/product name match from a curated list. Don't try to match "ketchup" to every ketchup brand — match "Heinz Ketchup 20oz" to the specific deal. Andy seeds the first 10 deals manually via admin dashboard.

**Acceptance Criteria:**
- Active deal matches trigger a swap suggestion in Penny's response
- Swap card renders below the matched item with brand logo, product, savings
- "Swap" replaces the item on the list and logs `swap_events` with action=accepted
- "No thanks" dismisses the card and logs action=declined
- Max 2 swap suggestions per session
- 3 declines for same brand suppresses it for 30 days
- Admin can create/edit/deactivate deals via `/admin`

---

### M2. Fix Sharing & Invite Flow
**Size:** M (1 week, parallel with M1)
**Priority:** P0 — Sharing is the distribution channel for swaps
**Status:** Planned

**Description:**
Sharing is the primary distribution mechanism. Fix the known bugs and add attribution tracking so we can measure the viral loop that makes swaps valuable to brands.

**CX Insight:** Every broken invite link is a lost household. Households are the atomic unit of grocery shopping — if sharing doesn't work flawlessly, the app is a single-player notepad competing with Apple Notes.

**UX Notes:** Share button should use the native Web Share API where available, falling back to copy-link. The invite link should show a branded preview card when pasted in iMessage/WhatsApp (Open Graph tags). After joining, both users should see a confirmation in Penny's chat.

**Architect Notes:**
- Fix: invite links that 404 or redirect incorrectly
- Fix: wrong/missing brand logos on shared lists
- Add: `referral_source` tracking on user creation (which invite link, which user)
- Add: Open Graph meta tags on invite URLs for rich previews
- Add: Penny notification to inviter when someone joins their household

**Staff Dev Notes:** Editable list names already exist in saved lists — surface the rename option on the main list view. The SMS notification on join uses existing Twilio infrastructure.

**Acceptance Criteria:**
- Invite links work on all tested browsers (Chrome, Safari, Firefox mobile)
- Joining a household sends SMS/push to the inviter
- Penny acknowledges the new household member in chat
- Invite links show branded preview cards in iMessage/WhatsApp
- Referral attribution tracks which user invited which
- Brand logos render correctly on shared lists
- List name is editable (tap to rename, e.g. "Tim and Wendy's List")

---

### M3. Swap Admin Tooling (Seed Deals)
**Size:** S (3-4 days, parallel with M1)
**Priority:** P0 — Enables M1
**Status:** Planned

**Description:**
Extend the existing admin dashboard so Andy can manually create and manage brand deals. This is the "database seed script" from the CEO synthesis — no brand portal needed yet, just admin CRUD.

**Architect Notes:**
- Add admin routes: `POST /admin/deals`, `PUT /admin/deals/:id`, `DELETE /admin/deals/:id`, `GET /admin/deals`
- Deal fields: brand name, brand logo URL, competitor product (text), offer product (text), discount amount (cents), daily budget (cents), status (active/paused/expired), start/end dates
- Add deal performance view: impressions, acceptances, declines, acceptance rate, spend-to-date

**Acceptance Criteria:**
- Admin can create a deal with all required fields
- Admin can pause/resume/delete deals
- Admin sees deal performance metrics (offered, accepted, declined, acceptance rate)
- Admin can set and see budget remaining
- Deals with $0 budget remaining auto-pause

---

## Phase 2 — Brand Portal + Payment (Weeks 3-4)

> **North star:** First brand pays with a credit card. Revenue = real.

### M4. Brand Portal v0 (brands.thrift.ly)
**Size:** L (2 weeks)
**Priority:** P0 — Revenue infrastructure
**Status:** Planned

**Description:**
A separate web app at `brands.thrift.ly` where brand product managers can sign up, create deals, set budgets, and see performance. This is the self-serve experience that 14/18 focus group participants said they'd use without a sales call.

**UX Notes:**
- Landing page: "Put your product on every grocery list. $1.50 per swap." Clear value prop, no jargon.
- Sign-up: corporate email only (validate domain against a CPG company list + catch-all validation). "Request Demo" link for enterprise.
- Dashboard: single-page view with (1) active deals, (2) create new deal form, (3) performance chart (swaps over time), (4) spend tracker, (5) **Competitive Swap Intel** panel.
- **Competitive Swap Intel** (the killer feature from focus groups): "In the [Ketchup] category, [Heinz] was swapped out 47 times this week. Top replacements: Sir Kensington's (23), Primal Kitchen (14), Other (10)." This is anonymized, aggregated data — no PII. This view is visible to ALL brands in the category, even those without active deals. It's the FOMO engine.
- Design: clean, professional, minimal. Think Linear or Stripe dashboard, not enterprise software.

**Architect Notes:**
- Separate frontend (React app) served from `brands.thrift.ly` subdomain. Same Go backend, new route prefix `/brand/`.
- New tables: `brand_users` (id, email, company_name, verified, created_at), `brand_sessions` (same pattern as user sessions)
- Corporate email validation: check domain against a curated list of known CPG domains + MX record validation. Manual approval queue for unknown domains.
- Auth flow: email + password, email verification link, no phone/SMS needed.
- API routes: `POST /brand/register`, `POST /brand/login`, `GET /brand/dashboard`, `POST /brand/deals`, `GET /brand/deals`, `GET /brand/category-intel/:category`

**Staff Dev Notes:** The brand portal is a second React app — do NOT bolt it onto the consumer App.tsx. Separate Vite config, separate entry point. Shares the Go backend. Deploy as a separate Railway service or as a subdomain route on the same service.

**Acceptance Criteria:**
- Brand manager can register with corporate email and receive verification
- Non-CPG email domains are rejected with "Contact us for access"
- Brand manager can create a deal (pick category, competitor product, their product, discount, budget)
- Dashboard shows: active deals, total swaps, acceptance rate, spend, budget remaining
- Competitive Swap Intel panel shows anonymized category swap data
- "Founding Brand" badge for first 10 brands

---

### M5. Stripe Payment Integration
**Size:** M (1 week)
**Priority:** P0 — Proof of willingness to pay
**Status:** Planned

**Description:**
Brand managers add a credit card and fund their deal budget. Charges are per-swap (post-pay) or pre-funded (deposit). Start with pre-funded deposits for simplicity — brand deposits $100, swaps deduct from balance.

**Architect Notes:**
- Stripe Checkout for initial deposit. Stripe Customer + PaymentMethod stored in `brand_users`.
- New table: `brand_balance` (brand_user_id, balance_cents, last_deposit_at). Deducted on each accepted swap.
- Webhook: `payment_intent.succeeded` credits the brand balance.
- Low-balance email alert at 20% remaining. Auto-pause deals at $0.
- Future: move to per-swap billing (Stripe Usage Records) when volume justifies it.

**Staff Dev Notes:** Use Stripe Checkout Sessions — do NOT build a custom payment form. Redirect to Stripe, handle the webhook. Keeps us out of PCI scope.

**Acceptance Criteria:**
- Brand manager can deposit funds via Stripe Checkout ($50 minimum)
- Balance displays on dashboard, updates in near-real-time after swaps
- Deals auto-pause when balance hits $0
- Low-balance email at 20% remaining
- Stripe webhook processes deposits reliably (idempotent)
- Brand can view transaction history (deposits, swap charges)

---

### M6. Penny Swaps v1 — Conversational Swaps
**Size:** M (1 week, parallel with M4/M5)
**Priority:** P1 — Improves swap acceptance rate
**Status:** Planned

**Description:**
Upgrade Penny's swap suggestions from static cards to conversational engagement. Instead of just showing a card, Penny explains WHY the swap is good: "Sir Kensington's uses cage-free eggs and no high fructose corn syrup — plus you'd save $1.50." If the user asks questions ("is it any good?"), Penny can respond with product details.

**CX Insight:** Focus group data (from MARKETPLACE_SQUADS.md) shows commodity items have high switching willingness. Penny should lean harder on savings for commodities ("save $1.50 on paper towels") and lean on product qualities for premium items ("this ketchup uses real tomatoes, no HFCS").

**Architect Notes:**
- Extend `brand_deals` with: `product_description`, `key_benefits` (JSON array), `brand_story` (text)
- Penny's system prompt includes deal context when suggesting a swap — she can answer questions about the product
- Track: conversational depth per swap (how many messages before accept/decline)

**Acceptance Criteria:**
- Penny explains product benefits when suggesting a swap
- User can ask follow-up questions about the swapped product
- Penny provides accurate product information (from deal metadata, not hallucinated)
- Conversational swaps have higher acceptance rate than card-only swaps (BI tracks this)

---

## Phase 3 — Receipt Insights + Guild Payouts + Scale (Weeks 5-8)

> **North star:** $500/mo in swap revenue. 10+ paying brands. Guild members earning real money.

### M7. Receipt Reading for Purchase Verification
**Size:** L (2-3 weeks)
**Priority:** P1 — Closes the attribution loop for brands
**Status:** Planned

**Description:**
Users photograph their grocery receipt after shopping. Penny reads it (via Claude vision or a receipt OCR API), confirms which swapped items were actually purchased, and awards any applicable savings to the user's PennyWallet. For brands, this closes the attribution loop: "47 swaps offered, 31 accepted, 22 confirmed purchased via receipt."

**CX Insight:** Receipt scanning must be frictionless. "Snap a pic of your receipt and I'll track your savings" — Penny handles it. No manual entry.

**UX Notes:** Camera button in the chat input area. User takes photo, Penny processes it, shows a confirmation: "I found 3 items from your list including the Sir Kensington's swap. You saved $1.50!" Photo upload with loading state.

**Architect Notes:**
- Image upload endpoint: `POST /api/receipt` (multipart form data)
- Receipt processing: send image to Claude with a structured extraction prompt. Extract: store name, date, line items (product name, quantity, price), total.
- Match line items against the user's recent swap acceptances. Confirmed matches update `swap_events` with `verified=true` and `receipt_id`.
- New table: `receipts` (id, user_id, image_url, store_name, purchase_date, total_cents, items_json, processed_at)
- Store receipt images in S3 or Railway volumes. Retain for 90 days, then delete image but keep extracted data.

**Staff Dev Notes:** Claude vision is the simplest path — no third-party OCR vendor. Send the image with a prompt like "Extract all line items with product name and price from this grocery receipt. Return as JSON." Handle common receipt formats (Walmart, Kroger, Target, Costco).

**Acceptance Criteria:**
- User can photograph a receipt from the chat interface
- Penny extracts line items with >90% accuracy on major retailer receipts
- Purchased swap items are marked as verified in `swap_events`
- Brand dashboard shows verified purchase count alongside swap acceptance count
- Receipt data is stored securely, images deleted after 90 days

---

### M8. PennyWallet + Guild Payouts
**Size:** L (2-3 weeks)
**Priority:** P1 — Incentive layer for viral distribution
**Status:** Planned

**Description:**
PennyWallet tracks each user's earnings from (1) swap savings (consumer benefit) and (2) Guild referral revenue (30% of swap revenue from their referral tree). Guild members earn real money when people they invited accept swaps. Payouts via Stripe Connect (ACH) or PayPal when balance exceeds $10.

**CX Insight:** Shopper panel data: "$5/mo = nobody cares, $20-30/mo = changes behavior." Guild payouts must be meaningful. At $1.50/swap with 30% Guild share = $0.45/swap. A Guild member needs ~45 referred swap acceptances/month for $20. That means referring 5-10 active households.

**UX Notes:**
- Wallet tab in the consumer app showing: total earned, pending, available for payout
- Earnings breakdown: "Swap savings: $4.50 | Guild earnings: $12.30"
- Payout button when balance >= $10
- Guild section showing: your referrals, their activity (anonymized), your earning rate
- Frame as "save together" not "recruit" — the shopper panel was explicit about MLM smell test

**Architect Notes:**
- New tables: `wallet` (user_id, balance_cents, lifetime_earned_cents), `wallet_transactions` (id, user_id, type [swap_saving/guild_earning/payout], amount_cents, reference_id, created_at), `guild_tree` (user_id, referred_by_user_id, depth)
- Guild revenue calculation: on each accepted swap, compute 30% of deal price, walk up the `guild_tree` to allocate earnings. Simple 1-level referral for MVP (you earn on people you directly invited). Multi-level is future.
- Payout: Stripe Connect Express accounts for US users. Trigger payout via `POST /api/wallet/payout`. Minimum $10.

**Staff Dev Notes:** Start with 1-level referral only. The guild_tree table supports multi-level but the calculation for MVP is just: "Did this user get referred by someone? If yes, that someone gets 30% of the swap revenue." Keep it simple.

**Acceptance Criteria:**
- Users see their wallet balance in the app
- Swap savings credited immediately on acceptance (pending until receipt verification if available)
- Guild earnings credited when a referred user accepts a swap
- Payout request triggers Stripe Connect transfer (>= $10 minimum)
- Earnings breakdown shows swap savings vs Guild earnings
- No MLM language anywhere in the UI — "earn by sharing" not "build your team"

---

### M9. Brand Dashboard v2 — Full Reporting
**Size:** M (1-2 weeks)
**Priority:** P1 — Justifies continued brand spend
**Status:** Planned

**Description:**
Upgrade the brand dashboard with the reporting metrics that 16/18 focus group participants said they need. This is what turns a pilot into recurring spend.

**UX Notes:** Dashboard sections:
1. **Campaign Overview** — active deals, total spend, swaps, acceptance rate, CPA vs industry benchmarks
2. **Competitive Intel** — category swap trends, competitor swap-out volume, your brand's share of swaps
3. **Purchase Verification** — receipt-confirmed purchases, conversion funnel (offered → accepted → purchased)
4. **Trend Charts** — swaps/day over time, acceptance rate over time, spend over time
5. **Export** — CSV download for all data, email digest (weekly summary)

**Architect Notes:**
- All metrics computed from existing tables: `swap_events`, `brand_deals`, `receipts`
- Add materialized views or cached aggregations for dashboard performance (don't compute on every page load)
- Weekly email digest: cron job sends HTML email with key metrics to brand users (via SendGrid)
- CSV export: server-side generation, signed download URL

**Acceptance Criteria:**
- Dashboard loads in <2s with up to 10K swap events
- Competitive Intel shows category-level swap data (anonymized)
- Purchase verification funnel shows offered → accepted → purchased
- Weekly email digest sent every Monday with prior week's metrics
- CSV export works for all data views
- Brand manager can set date range filters on all charts

---

### M10. Shopper-to-Brand Lead Generation
**Size:** S (3-5 days)
**Priority:** P2 — Additional revenue stream
**Status:** Planned

**Description:**
When a user accepts a swap, offer them the option to "Learn more from [Brand]" — opt-in to receive an email or text from the brand with a coupon, sample offer, or product info. This gives brands a direct lead (with consent) and creates a second revenue stream: lead gen fees on top of swap fees.

**CX Insight:** Andy's direction: "We need some way of getting shoppers to email/text/fill out forms for CPGs to get the brand managers doing things." This is that mechanism — but consent-driven, not intrusive.

**UX Notes:** After a swap is accepted, a subtle "Want a coupon from [Brand]? Share your email." prompt. One tap, pre-filled with their account email. Completely optional.

**Architect Notes:**
- New table: `brand_leads` (id, brand_user_id, consumer_user_id, email, phone, opted_in_at, source [swap/receipt/organic])
- Lead delivered to brand via webhook or email notification
- Price: $3-5 per lead (separate from swap pricing)
- Consent recorded with timestamp, revocable

**Acceptance Criteria:**
- Post-swap opt-in prompt appears (dismissible)
- User email is pre-filled, requires one tap to confirm
- Brand receives lead notification (email to their dashboard inbox)
- Lead appears in brand dashboard with source attribution
- User can revoke consent at any time in settings

---

## Previous Roadmap Items (Still Valid — Marketplace Takes Priority)

> These items remain important for product quality and retention. They will be worked on as bandwidth allows, interleaved with marketplace phases. Priority numbers are relative within this section.

### R1. Conversational Onboarding via Penny
**Size:** L (2-3 weeks)
**Priority:** P2 — Critical for activation & retention
**Status:** Planned

**Description:**
New users currently land on an empty list with no guidance beyond Penny's greeting. Onboarding should happen naturally through conversation with Penny — she introduces herself, asks what the user is shopping for, offers to create their first list, and teaches key features (sharing, SMS, saved lists) organically as the user engages.

**CX Insight:** First-session drop-off is the biggest risk for a new app. Users who don't add 3+ items in their first session are unlikely to return. Penny should make the first interaction feel like chatting with a helpful friend, not figuring out an app.

**UX Notes:** No tutorial screens or modals. The onboarding IS the product — Penny chats, user responds, items appear. Progressive disclosure of features (sharing, trips, SMS) happens over the first few sessions, not all at once. **Updated:** Onboarding should now also introduce swaps naturally — "I can help you find deals too!"

**Architect Notes:** Requires tracking onboarding state per user (e.g. `onboarding_step` column or a flags set). Penny's system prompt needs onboarding-aware logic — detect new/empty-list users and shift personality to welcoming mode.

**Acceptance Criteria:**
- New user sees Penny greeting within 1s of first login
- Penny asks an open-ended question ("What do you need from the store?")
- First items are added through conversation
- Sharing/SMS features introduced in session 2-3, not session 1
- Swaps introduced naturally after first list has 3+ items
- No modals, tooltips, or tutorial overlays

---

### R2. Trip Planner Discoverability
**Size:** S (2-3 days)
**Priority:** P3 — Quality of life
**Status:** Planned

**Description:**
The trip planner is buried in an action sheet that users don't know to open. Penny should proactively suggest planning a trip when the list has 5+ items, or when the user says things like "I need to go shopping" or "heading to the store."

**CX Insight:** Users who use trips are 3x more engaged (they come back for the "done shopping" flow). But most users don't know trips exist.

**UX Notes:** A subtle, dismissible prompt near the bottom of the list — not a modal. Something like "Ready to shop? Plan a trip" that appears contextually. Penny can also mention it conversationally.

**Architect Notes:** Add trip suggestion logic to the parse response — when the list grows past a threshold or user mentions shopping, Penny can include a trip CTA in her response.

**Acceptance Criteria:**
- Penny suggests a trip when list has 5+ unchecked items (at most once per session)
- "Plan a trip" appears as a tappable suggestion in Penny's response
- Dismissing the suggestion remembers the preference for that session

---

### R3. Full "Done Shopping" Flow
**Size:** M (1-2 weeks)
**Priority:** P2 — Retention driver + receipt scanning trigger
**Status:** Planned

**Description:**
Currently the "Done" button just ends the trip. A complete post-shopping experience should include: trip summary (items checked off, time spent), Penny congratulating the user, option to save the list for next time, and clearing checked items. **Updated:** This is also the natural trigger for receipt scanning (M7) — "Nice trip! Snap a photo of your receipt and I'll track your savings."

**CX Insight:** The end-of-trip moment is a natural engagement peak — users feel accomplished. Celebrating that moment drives habit formation. "You got 14 items in 23 minutes!" feels good.

**UX Notes:** Full-screen summary card with Penny's personality. Confetti for first trip. Option to "Save this list" for reuse. Auto-clear checked items after confirmation. Receipt scan prompt after summary.

**Architect Notes:** Requires storing trip start time (already have it), calculating duration, counting items checked during the trip window. The saved-list flow already exists — just needs a trigger point.

**Acceptance Criteria:**
- Tapping "Done" shows a summary screen with item count and trip duration
- Penny says something encouraging and personalized
- "Save this list" option creates a saved list from the trip's items
- Checked items are cleared after user confirms
- First-ever trip completion shows confetti
- "Scan your receipt" prompt appears after summary (when M7 is built)

---

### R4. Server-Sent Events for Real-Time List Updates
**Size:** L (2-3 weeks)
**Priority:** P3 — Scale optimization
**Status:** Planned — not urgent until >100 DAU

**Description:**
Replace the current polling model (frontend fetches `GET /api/list` every few seconds) with SSE. When any household member modifies the list, push the delta to all connected members instantly.

**Architect Notes:** Add `GET /api/list/events` endpoint that holds an SSE connection. On any list mutation (add, remove, toggle, claim), broadcast to all connected household members. This eliminates 80-90% of server requests and gives sub-second sync. Works through Cloudflare and Railway's proxy. No websocket complexity needed.

**DevOps Notes:** SSE connections are long-lived — need to monitor connection counts and set reasonable timeouts (30s heartbeat, 5min max before reconnect). Railway's proxy timeout may need adjustment.

**BI Notes:** Track: polling requests/min before vs SSE connections after. Measure: server CPU reduction, bandwidth reduction, time-to-sync for household changes.

**Acceptance Criteria:**
- List updates appear on other household members' screens within 1s
- No polling requests when SSE is connected
- Graceful fallback to polling if SSE connection fails
- Server handles 100+ concurrent SSE connections without issues

---

### R5. End-to-End Tests (Playwright)
**Size:** M (1-2 weeks)
**Priority:** P2 — Development velocity
**Status:** Config exists, no tests written

**Description:**
Playwright E2E tests covering critical user flows: login, add items, check items, share with household, trip planning. The config and directory structure (`frontend/e2e/`, `playwright.config.ts`) already exist. **Updated:** Add swap acceptance flow and brand portal login to the critical path tests.

**Staff Dev Notes:** Start with 5 critical-path tests, not comprehensive coverage. The 80/20 is: (1) phone login → add item → see it on list, (2) email register → login, (3) household invite → join → shared list, (4) Penny chat → items added, (5) trip plan → done shopping. Add (6) swap offered → accepted and (7) brand portal login → create deal when marketplace ships.

**DevOps Notes:** Run Playwright in CI on every PR. Use `--project=chromium` only (skip webkit/firefox) to keep CI fast. Consider running full browser matrix only on main branch merges.

**BI Notes:** Track: test suite duration, flakiness rate, bugs caught by E2E vs found in production.

**Acceptance Criteria:**
- 5-7 critical-path E2E tests pass in CI
- Tests run in <3 minutes
- Tests use mock Twilio/Claude (no real API calls)
- CI blocks merge on E2E failure
- Flakiness rate <5% over 30 days

---

## Priority Order

| # | Item | Size | Phase | Priority | Rationale |
|---|------|------|-------|----------|-----------|
| 1 | M1. Penny Swaps v0 | L | 1 | P0 | Core revenue mechanism. Nothing else matters without this |
| 2 | M2. Fix Sharing & Invite Flow | M | 1 | P0 | Distribution channel for swaps — broken sharing = no growth |
| 3 | M3. Swap Admin Tooling | S | 1 | P0 | Enables M1 — Andy seeds first 10 deals manually |
| 4 | M4. Brand Portal v0 | L | 2 | P0 | Self-serve brand sign-up = revenue without Andy on every call |
| 5 | M5. Stripe Payment | M | 2 | P0 | Proof of willingness to pay. $ = validation |
| 6 | M6. Penny Swaps v1 | M | 2 | P1 | Improves acceptance rate, makes swaps feel natural |
| 7 | M7. Receipt Reading | L | 3 | P1 | Closes attribution loop, unlocks enterprise brand interest |
| 8 | M8. PennyWallet + Guild Payouts | L | 3 | P1 | Incentive layer — turns users into paid distribution |
| 9 | M9. Brand Dashboard v2 | M | 3 | P1 | Justifies recurring brand spend |
| 10 | M10. Shopper-to-Brand Leads | S | 3 | P2 | Second revenue stream, low effort |
| 11 | R1. Conversational Onboarding | L | — | P2 | Activation/retention, work as bandwidth allows |
| 12 | R3. Done Shopping Flow | M | — | P2 | Retention + receipt scan trigger |
| 13 | R5. E2E Tests | M | — | P2 | Dev velocity, interleave with feature work |
| 14 | R2. Trip Planner Discoverability | S | — | P3 | Quality of life, quick win |
| 15 | R4. SSE Real-Time Updates | L | — | P3 | Scale optimization, not urgent yet |

---

## Key Metrics — Revenue is the North Star

### Weekly Scorecard

| Metric | Week 2 Target | Week 4 Target | Week 8 Target | Kill Threshold |
|--------|---------------|---------------|---------------|----------------|
| **Swap revenue (total $)** | $0 (building) | $50 | $500/mo run rate | < $100/mo at wk 8 |
| **Paying brands** | 0 | 3 | 10 | < 3 at wk 8 |
| **Swap acceptance rate** | — | >= 8% | >= 12% | < 5% at wk 4 |
| **Swaps offered/day** | 5 (test) | 50 | 200 | < 20/day at wk 8 |
| **Brand portal sign-ups** | 0 | 5 | 20 | < 5 at wk 8 |
| **Average deal size (deposit)** | — | $100 | $200 | < $50 |
| **List sharing rate** | 50% | 50% | 60% | < 25% |
| **Invite → join conversion** | >= 30% | >= 35% | >= 40% | < 15% |
| **Guild enrollment** | — | — | >= 15% of users | < 5% at wk 8 |
| **Guild payout requests** | — | — | >= 5 | 0 at wk 8 |

### Revenue Model

```
Revenue per swap = $1.50 (Founding Brand rate: $1.00)
Brand pays on acceptance (deducted from pre-funded balance)
Guild member earns 30% = $0.45/swap from their referrals
Thriftly keeps 70% = $1.05/swap (or $0.70 at Founding rate)

Break-even math:
  200 swaps/day × $1.05 net × 30 days = $6,300/mo gross
  Target: 200 swaps/day by week 8

Lead gen (M10, future):
  $3-5 per consented lead, 100% margin
```

---

## Quick Wins — Overnight / Low-Supervision Tasks

> Small, self-contained tasks that can be completed with minimal/no supervision. Good for overnight runs.

- **User profile photo** — Add camera/upload button on profile screen. Store as base64 or upload to S3/Railway volume. Show avatar in hamburger menu, household member list, and "added by" labels. Falls back to person icon if no photo.
- **Open Graph meta tags on invite links** — When an invite link is pasted in iMessage/WhatsApp, show a branded preview card (Penny logo, "Join [Name]'s grocery list on Thriftly"). Requires `<meta og:*>` tags on the invite URL route.
- **Haptic feedback on check/uncheck** — Add `navigator.vibrate(10)` on item toggle for satisfying tactile feedback on mobile.
- **Pull-to-refresh on list** — Standard mobile pattern. Trigger a list re-fetch on pull-down gesture.
- **"Added just now" timestamp** — Show relative timestamps on recently added items ("just now", "2 min ago") that fade to nothing after 1 hour.
- **Favicon + PWA icons** — Ensure app icon renders correctly on iOS home screen, Android home screen, and browser tabs. Add apple-touch-icon, manifest icons at all required sizes.
- **Accessibility audit** — Add `aria-label` to all interactive elements, ensure color contrast meets WCAG AA, test with VoiceOver/TalkBack.
- **404 page** — Currently unknown routes show a blank page. Add a friendly "Page not found — here's your list" redirect.
- **Loading skeleton** — Replace the blank screen during list fetch with a skeleton/shimmer animation. Feels faster.
- **Penny typing indicator** — Show "Penny is thinking..." with animated dots while waiting for Claude response. Already partially exists; verify it works consistently.

---

## Bugs (Fix When Bandwidth Allows — No Supervision Needed)

> These bugs were found during automated UAT (March 2026). None are blocking. Safe to fix overnight — they are well-scoped, low-risk, and testable in isolation.

- **B1. `/api/*` unknown paths return 200 HTML instead of 404 JSON** — SPA fallback catches `/api/` routes. Add a catch-all handler for `/api/` prefix that returns `{"error":"Not found"}` with 404 before the SPA fallback. Files: `server.go` (route registration).
- **B2. No Open Graph meta tags on referral/invite links** — `/r/:code` and `/join/:code` serve bare `index.html` with no `og:title`, `og:description`, or `og:image`. Shared links in iMessage/WhatsApp show no preview card. Fix: add a server-side handler for these paths that injects OG tags before serving HTML. Files: `server.go`, `frontend/index.html`.
- **B3. Silent household switch on join** — User already in household A can join household B with no warning. They get silently moved. Fix: return `{"warning":"You will leave your current household"}` or require confirmation. Files: `handlers_social.go` (`handleJoin`).
- **B4. DELETE returns 200 on nonexistent/cross-user items** — `DELETE /api/list/:id` returns `{"success":true}` even when 0 rows affected. Fix: check `RowsAffected()` and return 404 if zero. Files: `handlers_list.go` (`handleListDelete`).
- **B5. `quantity` ignored in direct `POST /api/list`** — The `handleListAdd` struct only unmarshals `name` and `department`. Quantity is always 1. Fix: add `Quantity int` to the struct with default-to-1 fallback. Files: `handlers_list.go` (`handleListAdd`).
- **B6. 405 Method Not Allowed responses are plain text** — `requireMethod()` uses `http.Error()` which returns `text/plain`. All other errors return JSON. Fix: use `writeJSON()` instead. Files: `utils.go` (`requireMethod`).
- **B7. Burning invite codes on no-op joins** — User already in a household can use a new invite from the same household creator. The code is consumed but no state changes. Fix: check if user is already in the target household before consuming the code. Files: `handlers_social.go` (`handleJoin`).
- **B8. Non-numeric item ID returns plain text error** — `/api/list/abc` returns `"Invalid ID"` as `text/plain` instead of JSON. Fix: use `writeJSON()` in the ID parser. Files: `handlers_list.go`.
- **B9. No PWA manifest** — No `manifest.json` or `manifest.webmanifest` exists. Needed for Add-to-Home-Screen on mobile. Fix: create manifest with app name, icons, theme color, start URL. Files: `frontend/public/manifest.json`, `frontend/index.html`.

---

## Nice-to-Have (Low Priority — May Revisit)

- **Cloudflare CDN** — Free tier in front of Railway. No code changes needed. Setup: point DNS to Cloudflare, SSL "Full (strict)", cache static assets. Instant perf + security win. ~30 min of config.
- **Savings Share Card** — Shareable image showing "I saved $X with Penny this month." Social proof artifact for organic distribution. Blocked by PennyWallet (M8).
- **Swap Recommendation Engine** — ML-based swap suggestions based on user purchase history. Requires receipt data (M7) at scale. Phase 4+.
- **Multi-level Guild** — Extend Guild earnings beyond direct referrals (2-level, 3-level). Requires careful MLM smell-test UX. Phase 4+.
- **Groflo Coupon Integration** — Pull deals from Groflo's coupon repository as a supply source for Penny Swaps. Currently deals are manually seeded; Groflo API integration automates supply. Phase 4+.
- **Tod/Scout AI Outreach** — Automated LinkedIn outreach to CPG brand managers using wattdata.ai's 580M contact records + Scout's social graph. Not needed while Andy does founder-led sales. Revisit at 50+ brands.
- **PopSavings.com Collaboration** — Tim Connors' parallel project with similar swap mechanics. Potential deal inventory sharing or product merge. Revisit after proving Thriftly's swap model independently.

---

## Future (On Roadmap — Distant)

- **Native iOS app (App Store)** — Founder directive: consumer experience stays in the app. Current PWA works but native unlocks push notifications, camera access for receipts, and App Store distribution. Target: after product-market fit confirmed ($5K MRR).
- **Native Android app (Google Play)** — Same rationale as iOS. Consider React Native or Expo for shared codebase with the web app. Target: after iOS launch.
- **Enterprise Brand Portal** — Multi-user team accounts, SSO, invoicing (not credit card), geographic targeting, demographic targeting, API access. Required for P&G-tier brands. Target: after 50+ paying brands.
- **Apple Sign-In** — Required for iOS App Store submission if app offers other social logins. Add when native iOS app ships.
- **Call Penny (Voice)** — Let users call Penny's phone number and interact via voice. Requires Twilio Voice + speech-to-text + text-to-speech integration. Would enable hands-free list management while driving or cooking.

---

## Not In Scope (Explicitly Deferred)

- **Postgres migration** — SQLite is correct until ~200 DAU or funding. See architect review.
- **Horizontal scaling** — Single instance is fine for current scale. Blocked by SQLite anyway.
- **Recipe parsing / meal planning** — Adjacent feature, not core to grocery list MVP.
- **Store price comparison** — Requires partnerships and data sources not yet available.
- **Push notifications** — SMS/email channels cover this. Revisit with native apps (Future).
