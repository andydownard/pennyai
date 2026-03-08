# Thriftly Product Roadmap

> **GUARDRAIL: Never delete items from this roadmap without explicit approval from the founder.** Items that are completed or deprioritized should be moved to the appropriate section (Completed, Nice-to-Have, or Not In Scope) — never removed. Always prompt before modifying priority order.
>
> **See also:** `TECH_DESIGN_PENNY_SWAPS.md` — detailed technical design document for the Penny Swaps MVP with full database schemas, API contracts, pipeline architecture, fraud prevention checklist, and evidence retention policy. Based on 3 focus group panels (18 total participants).

## Focus Group Findings

> Detailed findings from 3 focus group panels (24 participants total) are in `FOCUS_GROUPS.md`. Full technical design in `TECH_DESIGN_PENNY_SWAPS.md`.

| Panel | N | Key Finding |
|---|---|---|
| Grocery Shoppers | 6 | 3 declines = permanent suppression; store-specific pricing required; swaps in planning mode only |
| Receipt Specialists | 6 | Textract + Claude two-stage pipeline; 7-year retention; pHash dedup as MVP fraud defense |
| CPG Brand PMs | 18+6 | Challenger brands pay $1-2/swap immediately; self-serve for non-enterprise; Competitive Swap Intel is the killer brand acquisition feature |

---

## Swap Types Strategy (Product Team + Focus Group Convene, March 2026)

Three swap types were evaluated across 3 rounds with the full product team and 7-person customer panel.

### Priority Ranking

| Priority | Swap Type | Rationale |
|---|---|---|
| **P0 — Ship First** | **SKU-Level Swap** | Highest user value, lowest complexity, lowest trust risk, cleanest analytics. 7/7 panel acceptance. |
| **P1 — Ship Second** | **Brand-Level Swap** | Natural upsell, higher revenue per campaign. 5/7 accept with per-item control. Requires product taxonomy. |
| **P2 — Rethink** | **Department Placement (CPM)** | Must be redesigned as "Deals" feature, NOT a swap. 4/7 reject, 2/7 deal-breaker. Highest trust risk. |

### Key Findings
- **SKU swaps** feel like a helpful friend's recommendation. Inline card below matched item with price comparison + "Sponsored" label.
- **Brand swaps** triggered when 3+ items from same brand detected. Must have per-item review (not blanket swap). Per-user, not per-household.
- **Department placement** is fundamentally advertising, not a swap. Must never be called a "swap." Redesign as optional "Deals near you" section with strict frequency caps (1/session, 2/week).

### Critical UX Rules (from panel)
1. "Sponsored" label visible on all paid swap cards — non-negotiable
2. Attribute matching required (organic, allergen-free, size) — not just category
3. Global swap opt-out toggle in Settings
4. Declined swaps never reshown in same session
5. Swaps are app-only for v1 (no SMS/email swap flow)
6. Per-user preferences, not per-household

### Red Flags
- Department placement erodes Penny's core value proposition (Marcus: "deal-breaker, I'll delete the app")
- "Equivalent" is subjective — brand-defined equivalence must validate on key attributes
- Household members can be confused by accepted swaps — make swap origin clear

### Backlog (prioritized by convene)
- **SKU swap improvements:** auto-accept threshold setting, decline reason capture, 3-decline permanent suppression
- **Brand swap (P1):** product category taxonomy, brand equivalence mapping, bundle swap UI with "Swap all" + per-item undo
- **Deals feature (P2):** separate from swap flow, department classifier, frequency caps, dietary filtering, permanent dismiss

### Extended Swap Types (Sprint 3, March 2026)

Evaluated 10 swap types across product team + 7-person customer panel. Three new types shipping this sprint:

| Rank | Swap Type | Ship When | Revenue/Swap | Description |
|------|-----------|-----------|-------------|-------------|
| 1 | **SKU-Level** | **SHIPPED** | $1.50 | Replace item with competitor alternative |
| 2 | **Trial Size** | **Sprint 3 (schema shipped)** | $1.00-1.50 | Free sample alongside original item (no swap needed) |
| 3 | **Seasonal** | **Sprint 3 (schema shipped)** | $2.00-3.00 | Time-boxed deal with deadline display |
| 4 | **BOGO** | **Sprint 3 (schema shipped)** | $2.50-3.00 | 2x product for price of 1 competitor |
| 5 | Brand-Level | Next sprint | $2.00-2.50 | Replace all items from a brand (needs product taxonomy) |
| 6 | Loyalty | Phase 3 | $0.50-1.00 | Repeat-purchase reward after initial swap |
| 7 | Ingredient-Upgrade | Phase 3 | $2.00-3.00 | Premium/healthier swap based on dietary profile |
| 8 | Cashback | Phase 3 | $2.50-3.00 | Post-purchase rebate via receipt scan |
| 9 | Bundle | Phase 3+ | $3.00-5.00 | Cross-category discount on 2+ related swaps |
| 10 | Department Placement | Phase 3+ (if ever) | CPM $3-5 | Redesign as "Deals" — highest trust risk |

**Key insight:** Trial Size has highest expected acceptance rate (>20%) because user keeps original item. Zero risk = easiest yes.

### Ongoing: A/B Testing on Swap Design
**Priority:** P1 — Revenue optimization
**Status:** Active, ongoing

Continuous A/B testing on swap card UI, copy, layout, and deal presentation to maximize take rate. Tests include:
- Card layout variations (compact vs expanded, badge placement)
- CTA copy ("Swap" vs "Try it" vs "Save $X")
- Price display format (total price vs unit price emphasis)
- Why panel: auto-expanded vs collapsed
- Badge colors and positioning for BOGO/Trial/Seasonal
- Swap suggestion timing (immediate vs delayed after item add)

Results feed directly into swap acceptance rate metrics. Target: 8%→15% acceptance rate through iterative design optimization.

---

## Phase 1 — Penny Swaps MVP + Sharing Fixes (Weeks 1-2)

> **North star:** Get 5 swaps accepted by real users. Prove the unit economics.

### M1. Penny Swaps v0 — Consumer Experience
**Size:** L (2 weeks)
**Priority:** P0 — Revenue blocker
**Status:** Planned

**Description:**
When a user's grocery list contains an item that matches an active brand deal, Penny suggests a swap conversationally. "Hey, I see you have Heinz ketchup — want to try Sir Kensington's? You'd save $1.50." The swap appears inline in the list as a dismissible card below the original item. User taps "Swap" or "No thanks." Penny remembers preferences — if you decline a brand 3 times, she stops suggesting it.

**CX Insight:** Swaps must feel like a helpful friend's recommendation, not an ad. The moment it feels like sponsored content, trust in Penny collapses. Framing matters: "save money" not "try this brand." Penny's enthusiasm level MUST be identical for sponsored and organic suggestions — shoppers detect promotional voice patterns instantly.

**UX Notes (validated by 6-person shopper focus group March 2026):**
- Swap card appears directly below the matched item in the list. No notifications, no modals, no full-screen takeovers (6/6 consensus on inline placement).
- Shows: brand logo, product name, unit-normalized price comparison (per oz/per unit, NOT just raw savings), "Swap" and "No thanks" buttons, "Why?" button with REAL data (ingredient comparison, ratings — NOT marketing copy), "Brand Partner" disclosure label (always visible on paid swaps).
- The "Why?" button is essential but MUST surface real data. An empty "Why?" backed by marketing copy is worse than no "Why?" button — it confirms "just an ad" and destroys trust (Sarah, Marcus, Priya all flagged this independently).
- Card disappears on dismiss with a gentle animation.
- "Pre-trip swap preview" mode: consolidated "Penny's Picks" review before shopping begins. Shoppers prefer making swap decisions in planning mode, not execution mode (James).
- Optional "decline reason" when tapping "No thanks" — feeds back into Penny's learning ("tried it before", "dietary restriction", "brand loyal"). Users want to feel they're teaching, not just dismissing (Devon).

**Shopper Anti-Patterns (will cause app deletion):**
1. **Bait-and-switch pricing:** Savings figure must be accurate at user's actual store — national average pricing is unacceptable (Linda: deal-breaker)
2. **Allergen-blind suggestions:** Swaps that conflict with recorded dietary restrictions are a SAFETY issue — must be hard-filtered at system level, not soft-warned (Priya: deal-breaker)
3. **Mid-trip interruptions:** No swap suggestions after user starts shopping trip — execution mode is sacred (James, Sarah, Linda)
4. **Missed savings shaming:** NEVER show declined swaps as "missed savings" — deliberate choices are not failures (Priya)

**Architect Notes:**
- New tables: `brand_deals`, `swap_events`, `swap_suppressions` (see TECH_DESIGN_PENNY_SWAPS.md for full schema)
- Matching logic: after Penny processes a list addition, check `brand_deals` for active deals where `competitor_product` fuzzy-matches the added item. Personalize the "why" message based on user's value axis (price for budget shoppers, ingredients for health-conscious).
- Penny's system prompt gets swap awareness: when a match exists, she weaves the suggestion into her response naturally
- Frequency cap: default 2-3 per session (user-configurable), unlimited mode for power users
- Decline tracking: **3 declines = permanent suppression** (6/6 shopper consensus, non-negotiable)
- Allergen hard filter: swap suggestions MUST NOT conflict with user's dietary profile
- Cooldown: min 42 days between repeat swaps for same user/brand (brand PM consensus on cooling period)

**Staff Dev Notes:** Fuzzy matching is the hard part. Start with exact UPC/product name match from a curated list. Don't try to match "ketchup" to every ketchup brand — match "Heinz Ketchup 20oz" to the specific deal. Andy seeds the first 10 deals manually via admin dashboard.

**Acceptance Criteria:**
- Active deal matches trigger a swap suggestion in Penny's response
- Swap card renders below the matched item with brand logo, product, unit price comparison, savings
- "Swap" replaces the item on the list and logs `swap_events` with action=accepted
- "No thanks" dismisses the card, logs action=declined, optionally captures decline reason
- "Why?" button shows real data (ingredients, unit price, ratings) — never marketing copy
- "Brand Partner" disclosure label visible on every paid swap card
- Max 2-3 swap suggestions per session (configurable)
- 3 declines for same deal = permanent suppression
- Allergen/dietary conflicts are hard-filtered (never shown)
- Budget decremented on acceptance; campaign pauses at cap (hard stop, not soft)
- Admin can create/edit/deactivate deals via `/admin`

**v1 Stretch Goals (merge when brand portal ships product descriptions):**
- Extend `brand_deals` with: `product_description`, `key_benefits` (JSON array), `brand_story` (text)
- Penny explains WHY the swap is good conversationally, not just via card
- User can ask follow-up questions about the swapped product
- Track conversational depth per swap (messages before accept/decline)
- Commodity items: lean on savings; premium items: lean on product qualities

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

## Phase 3 — Receipt Insights + Dashboard + Scale (Weeks 5-8)

> **North star:** $500/mo in swap revenue. 10+ paying brands. Receipt-verified purchase attribution.

### M7. Receipt Reading for Purchase Verification
**Size:** L (2-3 weeks)
**Priority:** P1 — Closes the attribution loop for brands
**Status:** Planned

**Description:**
Users photograph their grocery receipt after shopping. Penny reads it via a two-stage pipeline (AWS Textract for OCR + Claude for entity resolution), confirms which swapped items were actually purchased, and awards applicable savings. For brands, this closes the attribution loop: "47 swaps offered, 31 accepted, 22 confirmed purchased via receipt." Brands pay a premium ($2.50 vs $1.50) for receipt-verified purchases.

**CX Insight (validated by 6-person shopper + 6-person specialist focus groups):**
- Receipt scanning adoption is bimodal: ~50% enthusiastic (Linda 8/10, Devon 9/10), ~30% prefer digital alternatives (Marcus 3/10, James 4/10 — want email receipt / loyalty app integration). Build both paths.
- Receipt scanning must be OPTIONAL for savings tracking but REQUIRED for premium verified payouts.
- Blurry/partial receipt handling is a UX quality gate: if the first failure is punishing, the feature is abandoned. Accept partial reads gracefully.
- Privacy is a real concern: grocery receipts reveal health conditions, religious practices, financial stress. Specific privacy policy required (not vague "we take your privacy seriously").

**UX Notes:**
- Primary: in-app guided capture with quality gating (contrast/focus threshold, like bank check deposit). Secondary: camera roll upload as fallback (flagged lower trust for fraud scoring).
- Long receipts (Costco): multi-photo with server-side stitching for MVP; scroll capture Phase 2.
- Digital receipts: email forwarding to penny@thrift.ly → parse HTML/JSON (Instacart, Walmart, Target email receipts are structured data with 100% fidelity — Phase 1 priority).
- Loading state: "Penny is reading your receipt..." with typewriter personality (30-60s acceptable with good feedback).
- Partial reads: "I found 3 of 5 items — want to confirm the rest?" NOT "Failed, retake photo."
- User confirmation flow for low-confidence matches: "Is SKEN KETCH 20OZ the Sir Kensington's Ketchup?" → yes/no → feeds abbreviation database.

**Architect Notes (validated by 6-person specialist focus group):**
- **Two-stage pipeline:** AWS Textract Expense Analysis for structured extraction (store, date, line items, totals, UPCs) → Claude for entity resolution and item matching. Textract purpose-built for receipts; Claude for reasoning about abbreviations.
- Image upload endpoint: `POST /api/receipt` (multipart form data)
- At upload: assign UUID, compute SHA-256 hash, store original unmodified image, perceptual hash for dedup, write to append-only audit log. ALL before processing.
- Extraction fields: store_name, store_address, transaction_date, line_items [{raw_text, price, qty, upc_if_present}], subtotal, tax, total, payment_method_type.
- Math validation: line items sum ≈ subtotal + tax ≈ total (failure → flag for review).
- Item matching: (1) UPC lookup (highest confidence), (2) deterministic abbreviation DB with retailer-specific patterns, (3) Claude LLM for unmatched long-tail. Log resolution_method for auditability.
- User confirmations feed back into `retailer_abbreviations` table — this is the flywheel that makes matching improve over time.
- New tables: `receipts`, `receipt_line_items`, `receipt_hashes`, `swap_claims`, `claim_audit_log`, `retailer_abbreviations` (see TECH_DESIGN_PENNY_SWAPS.md for full schema)
- Evidence retention: 2 years active, 5 years cold storage = 7 years total. SHA-256 chain of custody. Original image required.
- **Thriftly = trusted intermediary:** Brands NEVER see receipt images. Only aggregated verification results. This is a competitive moat AND privacy protection.

**Fraud Prevention (see TECH_DESIGN_PENNY_SWAPS.md for full checklist):**
- Tier 1 (at upload): perceptual hash dedup, rate limiting (3/day, 10/week), date consistency, new account gate
- Tier 2 (async): math validation, Error Level Analysis, price plausibility
- Tier 3 (batch): device fingerprint clustering, cross-user dedup, buy-return defense (30-day payout hold)
- Published fraud rate in brand dashboard — transparency builds trust

**Staff Dev Notes:** Two-stage pipeline (Textract + Claude) is the specialist consensus. Claude Vision alone is excellent at reasoning but not optimized for structured receipt extraction. Textract's expense model outperforms on raw OCR; Claude's strength is "SKEN KETCH 20OZ" → "Sir Kensington's Ketchup" reasoning. Play to each tool's strengths.

**Accuracy Targets:**
- Store/date/total: >97%
- Line items extracted: >90%
- Item-to-product matching (lookup DB): >85%
- Item-to-product matching (LLM fallback): >75%
- End-to-end auto-verify rate: >70% (without user confirmation)
- Fraud false positive rate: <2%

**Acceptance Criteria:**
- User can photograph a receipt from the chat interface (guided capture + camera roll)
- Two-stage OCR pipeline extracts line items with >90% accuracy on major retailer receipts
- Perceptual hash dedup prevents receipt reuse
- Partial reads show what was found and offer user confirmation for low-confidence items
- User confirmations feed back into abbreviation database
- Purchased swap items marked as verified in `swap_events`
- Brand dashboard shows verified purchase count alongside swap acceptance count
- Receipt images stored with SHA-256 hash, 7-year retention policy
- 30-day payout hold on all verified claims (buy-return defense)
- Fraud rate published in brand dashboard

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

## Previous Roadmap Items (Still Valid — Marketplace Takes Priority)

> These items remain important for product quality and retention. They will be worked on as bandwidth allows, interleaved with marketplace phases. Priority numbers are relative within this section.

### R1. Conversational Onboarding via Penny
**Size:** L (2-3 weeks)
**Priority:** P1 — Activation gate for swaps (users need items on list for swaps to fire)
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
**Priority:** P1 — Shipping brand portal + payments without E2E coverage is a reliability risk
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

## Priority Order (Groomed March 2026)

| # | Item | Size | Phase | Priority | Rationale | Status |
|---|------|------|-------|----------|-----------|--------|
| 1 | **M3. Swap Admin Tooling** | S | 1 | P0 | Enables M1 — Andy seeds first 10 deals manually. Ship first so deals exist before swaps code lands. | Planned |
| 2 | **B11 + B14. Swap safety bugs** | S | 1 | P0 | Budget race condition (B11) + rebate farming (B14) = swap accept is exploitable. **Must fix before M1 ships.** | **Done** |
| 3 | **M1. Penny Swaps v0** | L | 1 | P0 | Core revenue mechanism. Nothing else matters without this. Depends on M3 + B11/B14. | Planned |
| 4 | **M2. Fix Sharing & Invite Flow** | M | 1 | P0 | Distribution channel for swaps — broken sharing = no growth. Parallel with M1. | **Done** |
| 5 | **Sentry integration** | XS | — | P0 | 2 hours. Required before Stripe webhooks go live. De-risks everything after. (Buy-vs-Build: BUY NOW) | **Done** |
| 6 | **PostHog integration** | S | — | P0 | Half day. Replaces custom analytics in handlers_admin.go. Needed to measure swap acceptance rates. (Buy-vs-Build: BUY NOW) | **Done** |
| 7 | **R5. E2E Tests** | M | 2 | P1 | Brand portal + payments need E2E coverage. Moved up — ship before M4, not after. | **In progress** (Sprint 3) |
| 8 | **R1. Conversational Onboarding** | L | 2 | P1 | Activation gate — swaps only fire when users have items on list. | **Done** (Sprint 3 — server-side flags, refined prompts) |
| 9 | **M4. Brand Portal v0** | L | 2 | P0 | Self-serve brand sign-up = revenue without Andy on every call. Depends on R5. | **Done** (Sprint 4) |
| 10 | **M5. Stripe Payment** | M | 2 | P0 | Proof of willingness to pay. Depends on Sentry (#5) for webhook reliability. | **Done** (Sprint 4) |
| 11 | ~~**B12. Admin DELETE deals FK failure**~~ | XS | — | P1 | ~~Quick fix. Admin can't delete deals.~~ Soft-delete fix shipped. | **Done** |
| 12 | **B13. BrandDeal struct missing columns** | S | — | P1 | Admin can't view/edit price comparison fields. Blocks swap quality. | **Done** |
| 13 | **B15. daily_budget_cents misnomer** | XS | — | P2 | Rename to total_budget_cents or implement daily reset. Low urgency but confusing. | **Done** |
| 14 | **R3. Done Shopping Flow** | M | 3 | P1 | Retention + natural trigger for receipt scanning (M7). | **Done** (Sprint 3 — summary, save list, confetti) |
| 15 | **M7. Receipt Reading** | L | 3 | P1 | Closes attribution loop. Consider Veryfi for OCR layer (Buy-vs-Build: BUY LATER). | **Backend done** (Sprint 5 — needs frontend UI) |
| 16 | **M9. Brand Dashboard v2** | S→M | 3 | P2 | **Scope reduced** — Metabase handles most reporting. Only Competitive Intel + purchase verification funnel need custom code. Was M, now S-M. | Planned |
| 17 | **R2. Trip Planner Discoverability** | S | — | P3 | Quality of life, quick win. Do when there's a spare afternoon. | **Done** (Sprint 4) |
| — | ~~R4. SSE Real-Time Updates~~ | — | — | — | **Replaced by Pusher/Ably** (Buy-vs-Build: BUY NOW). No longer a build item. | Replaced |
| 18 | **R6. Codebase Refactor** | L | — | P2 | Split monolithic files (App.tsx, handlers), extract middleware, shared styles, API layer. Do incrementally between sprints. | Planned |
| 19 | **R7. UX Overhaul** | L | — | P1 | Design system, dark mode, bottom tabs, screen transitions, optimistic updates, offline support. Dedicated sprint after revenue stabilizes. | Planned |

### What changed (March 2026 groom)
- **M3 moved to #1** — deals must exist before swap code ships. M1 depends on M3, not the other way around.
- **B11 + B14 promoted to P0** — swap accept has a budget race condition and no rebate farming dedup. Exploitable on day 1 if unfixed.
- **Sentry + PostHog inserted at #5-6** — both are half-day or less, both de-risk everything after. Do them in the gap between Phase 1 and Phase 2.
- **R5 (E2E tests) moved up to #7** — before brand portal, not after. Shipping payments without E2E is asking for trouble.
- **Bugs B12, B13, B15 given explicit priority slots** — were buried in Bugs section. Now visible in the backlog.
- **M9 scope reduced** — Metabase handles most reporting. Only custom views (Competitive Intel, purchase funnel) need code.
- **R4 dropped from backlog** — replaced by Pusher/Ably buy recommendation.

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
| **Receipt verification rate** | — | — | >= 20% of swaps | < 5% at wk 8 |

### Revenue Model

```
Revenue per swap = $1.50 (Founding Brand rate: $1.00)
Brand pays on acceptance (deducted from pre-funded balance)
Receipt-verified premium: $2.50/swap (Phase 3)

Break-even math:
  200 swaps/day × $1.50 × 30 days = $9,000/mo gross
  Target: 200 swaps/day by week 8
```

---

## Quick Wins — Overnight / Low-Supervision Tasks

> Small, self-contained tasks that can be completed with minimal/no supervision. Good for overnight runs.

- **Error monitoring (Sentry)** — Sentry integration for Go backend + frontend. Error alerting on production crashes. Required before Stripe webhooks go live.
- **Expand Go test coverage to replace manual UAT** — Add integration-style tests that exercise full request flows (multi-step auth → list CRUD → sharing → swaps → receipts) as Go tests, so UAT runs automatically via `go test` with no manual approval. Target: every endpoint has happy path + 3 edge case tests minimum. Priority areas: email webhook parsing, SMS webhook parsing, receipt photo flow, brand resolve, household cross-user scenarios.

### Completed Quick Wins (March 2026)
- ~~GitHub Actions CI~~ — `.github/workflows/ci.yml` created: Go tests + frontend build on every PR
- ~~Haptic feedback on check/uncheck~~ — `navigator.vibrate(10)` on item toggle
- ~~Pull-to-refresh on list~~ — touch gesture triggers list re-fetch
- ~~"Added just now" timestamp~~ — relative timestamps on recent items, fade after 1 hour
- ~~404 page~~ — unknown routes redirect to home with toast
- ~~Loading skeleton~~ — shimmer animation during list fetch
- ~~Help / Bug Report~~ — "Need help?" link in profile with FAQ + bug report form
- ~~User profile photo~~ — Camera/upload button on profile screen, shows avatar in nav and household lists
- ~~Accessibility audit~~ — `aria-label` on interactive elements, WCAG AA color contrast, VoiceOver/TalkBack tested
- ~~Expand Go test coverage~~ — Partially done (Sprint 5 — brand portal tests added, more integration tests needed)
- ~~Swaps indicator + transaction screen~~ — Header shows "$X.XX swaps" (tappable), opens full Swaps screen with summary cards (Total/Pending/Paid), time filter pills, transaction history with receipt upload per rebate. New `GET /api/earnings`, `POST /api/earnings/{id}/upload` endpoints. 12 Go tests. Files: `handlers_earnings.go`, `EarningsScreen.tsx`, `App.tsx`, `models.go`, `db.go`, `server.go`.
- ~~Profile page duplicate fix~~ — Name and phone appeared twice (static display under avatar + editable fields). Removed static display, kept editable fields only.
- ~~Clear list confirmation UX~~ — Backend no longer auto-deletes checked items on "clear list" command. Frontend presents unambiguous options: "Clear N checked items" / "Clear entire list (N items)" / "Cancel". All buttons state exactly what they do.

---

## Google OAuth + Unified Auth Experience

### Google OAuth Infrastructure (P0) — DONE
- ~~GCP project created: **Thriftly** (ID: `thriftly-489605`) under thrift.ly org~~
- ~~OAuth client created: `116812213585-oehqe9jgb8akl861numamhvehn2t5umh.apps.googleusercontent.com`~~
- ~~Authorized origins: `http://localhost:5173`, `http://localhost:8080`~~
- ~~Authorized redirect URIs: `http://localhost:8080/auth/google/callback`, `http://localhost:5173/auth/google/callback`~~
- ~~OAuth consent screen published to production (any Google user can sign in)~~

### Google OAuth Local Testing (P0)
- Add `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` to local `.env` with the new credentials
- Verify Google OAuth login flow works end-to-end locally (consent screen → callback → session created)
- **Why:** Google OAuth has been coded but never tested with real GCP credentials. Must work before production.

### Google OAuth Production Deploy (P1)
- Add production origins and redirect URIs once Railway deploy domain is confirmed
- Verify end-to-end on production
- **Why:** Local-only OAuth is useless without production URLs.

### Login with Google — Profile Auto-Population (P0)
- On Google sign-in, auto-populate the user's **profile photo**, **display name**, and **email** from Google's OAuth response (picture, name, email claims)
- Profile photo should appear immediately in nav bar, profile page, and household member lists — zero effort from the user
- Email should be stored and linked to the account automatically (no "enter your email" step)
- If user already has a profile photo (e.g. uploaded one), Google photo becomes the default only if they haven't set one
- **Why:** Google gives us rich profile data for free. Using it instantly makes the app feel polished and personal. One tap to sign in and your profile is already complete.

### Unified Auth: Easy & Delightful Multi-Method Linking (P0)
**North star:** Every user ends up with both a phone number AND an email, regardless of how they signed up. Both channels are valuable (SMS for Penny texting, email for receipts/password reset). Make it effortless to add the missing one.

- **Signed up with Google?** → After first login, Penny gently prompts: "Want to text me? Add your phone number and I can help you manage your list over SMS." One-tap opens the phone input. No friction, no mandatory gate.
- **Signed up with phone/SMS?** → Penny prompts: "Add your email so I can send you receipts and you can log in from any device." Or: "Sign in with Google to connect your email instantly."
- **Signed up with email/password?** → Penny prompts: "Add your phone number to text me your grocery list on the go."
- All prompts are **one-time, dismissible, non-blocking**. Penny remembers if you dismissed and doesn't re-ask in the same session. Uses the existing NPS-style session persistence pattern.
- **Why:** The more contact methods we have, the stickier the user is. SMS users who also have email are 3x more valuable (receipts, password reset, re-engagement). Google users who also have phone can use Penny over text.

### Account Merging — Simple & Safe (P1)
**Current state:** `resolveOrCreateUser()` auto-merges accounts by matching email. This handles the common case (sign up with email, later sign in with Google using same email).

**Needs improvement:**
- When a merge happens, clearly tell the user: "Welcome back! I linked your Google account to your existing Thriftly account." Don't silently merge — users should feel informed, not confused.
- **Household dedup on merge:** If both the old and new account are in the same household, automatically drop the duplicate membership. Keep the one with more history. No orphaned household members.
- **Conflict resolution:** If both accounts have list items, merge the lists (union, not replace). If both have different profile photos, keep the most recently set one. If both have different display names, prefer the one the user manually set over the auto-populated one.
- **Merge audit trail:** Log merges in `item_modifications` or a new `account_merges` table so we can debug issues. Include: which accounts merged, which auth methods linked, what data was preserved vs dropped.
- **Why:** Multi-auth merging is where trust breaks down if done wrong. Users who lose data or get confused by duplicate households will churn. Do the hard work to make it invisible.

---

## Bugs

### Open
*(No critical open bugs as of Sprint 5. All high-priority issues have been fixed or moved to roadmap.*

### Completed (March 2026)
- ~~B18. Clear List buttons appear before Penny finishes typing~~ — Fixed: added `!pennyTyping` guard to button visibility, changed `handleClearList()` to `handleClearList(true)` for full clear. Files: `PennyBar.tsx:118`, `App.tsx:1364`.
- ~~B13. `BrandDeal` struct missing 9 price/why columns~~ — Fixed: expanded struct in Sprint 3 to include price comparison and why-bullet fields. Admin API and form fully support viewing/editing. Files: `models.go:152-165`, `handlers_admin.go:379`, `App.tsx:437-443`.
- ~~B15. `daily_budget_cents` never resets~~ — Fixed: added `total_budget_cents` column with data migration in Sprint 3, updated all Go code. JSON tag kept for backward compat. Files: `db.go:614`, `handlers_admin.go`, `handlers_list.go:578`, `main_test.go`.
- ~~B17. No per-campaign swap limit~~ — Fixed: same user could accept the same deal multiple times across different items, earning multiple rebates from one campaign. The B14 dedup only checked same deal+item within 1 hour. Fix: added lifetime per-user-per-campaign check on both the offer side (`checkDealsForItems` skips already-accepted deals) and accept side (hard 409 enforcement). Files: `handlers_list.go`.
- ~~B16. NPS survey shows on every page reload~~ — Fixed: `useEffect` called `/api/nps/check` on every `screen` change to `'home'`, and dismissing the modal had no persistence. After dismiss or submit, navigating away and back re-triggered the survey. Fix: added `useRef` gate so the check runs at most once per mount, plus `sessionStorage` flag on dismiss/submit so it never re-prompts within the same browser session. Files: `frontend/src/App.tsx`.
- ~~B11. Budget TOCTOU race in swap accept~~ — Fixed: budget check moved inside transaction with atomic conditional `UPDATE ... WHERE budget_spent_cents + ? <= daily_budget_cents`. Files: `handlers_list.go`.
- ~~B12. Admin DELETE deals FK failure~~ — Fixed: soft-delete (`status='deleted'`) instead of hard DELETE, preserving FK references from `swap_events`/`swap_rebates`. Files: `handlers_admin.go`.
- ~~B14. No rebate farming dedup on swap accept~~ — Fixed: dedup check prevents repeated accepts for same deal+item within 1 hour. Returns 409 Conflict on duplicates. Files: `handlers_list.go`.
- ~~B8. Non-numeric item ID returns plain text error~~ — Fixed: all ID parsers in `handlers_list.go` and `handlers_saved_lists.go` now use `writeJSON()`. 9 tests added.
- ~~B1. `/api/*` unknown paths return 200 HTML instead of 404 JSON~~ — Fixed in `server.go`
- ~~B2. No Open Graph meta tags on referral/invite links~~ — Fixed: `serveWithOGTags()` in `utils.go`, OG tags in `index.html`
- ~~B3. Silent household switch on join~~ — Fixed: warning returned in `handleJoin`
- ~~B4. DELETE returns 200 on nonexistent/cross-user items~~ — Fixed: `RowsAffected()` check
- ~~B5. `quantity` ignored in direct `POST /api/list`~~ — Fixed: `Quantity` field added
- ~~B6. 405 Method Not Allowed responses are plain text~~ — Fixed: uses `writeJSON()`
- ~~B7. Burning invite codes on no-op joins~~ — Fixed: already-in-household check
- ~~B9. No PWA manifest~~ — Fixed: `manifest.json` created, linked in `index.html`
- ~~B10. Clearing list compacts departments instead of empty state~~ — Fixed: `?all=true` param for full clear

---

## Learning Loops — 5-Level Intelligence System (Product Team + CEO Panel, March 2026)

> Full strategy document: `LEARNING_LOOPS_STRATEGY.md`. Designed by product team convene (7 roles) and validated by 6 unicorn CEO panel. Goal: make Penny smarter every day across 5 nested levels.

### Architecture Overview

| Level | Learning Unit | Core Question | Status |
|-------|--------------|---------------|--------|
| **1. Individual** | Single user | "What does THIS person want?" | **In Progress** — signals, learnings, batch processing shipped |
| **2. Household** | Shared group | "How does this household coordinate?" | **In Progress** — coordination score in processLearnings |
| **3. Brand** | Advertiser | "What makes a swap succeed?" | **In Progress** — brand performance metrics in processLearnings |
| **4. Location** | Store/region | "What's available and at what price WHERE?" | Planned — requires receipt data at scale |
| **5. Business** | Platform | "What makes the marketplace healthy?" | **In Progress** — business_metrics_daily |

### Foundation (Shipped)

- [x] `user_signals` table — captures behavioral events (swap accept/decline, item adds)
- [x] `penny_learnings` table — scope-based knowledge store with confidence scores
- [x] `store_prices` table — crowdsourced price data from receipts
- [x] `recordSignal()` — async signal capture on swap and list events
- [x] `processLearnings()` — 6-hour batch: shopping frequency, price sensitivity, preferred day, household coordination, brand performance, store price levels
- [x] `GET /api/penny/insights` — returns accumulated learnings
- [x] Collect-then-write pattern for SQLite safety

### L1: Individual Learning (Current Sprint)

- [x] Shopping frequency pattern (daily/frequent/weekly/occasional)
- [x] Price sensitivity from swap behavior (high/moderate/low)
- [x] Preferred shopping day (most active day)
- [x] **Dietary profile** — user_dietary_profiles table, GET/PATCH /api/preferences, allergen hard-filter on swaps
- [x] **Penny context enrichment** — inject learned preferences into Penny's Claude system prompt
- [x] **Preference reset** — DELETE /api/preferences/reset (right to forget)
- [x] **Structured decline reasons** — decline_reason column on swap_events, captured on decline
- [x] **Repurchase cycle detection** — "You buy milk every 7 days" from list + modification patterns
- [x] **"Penny Knows" insights card** — shows learned preferences in Penny settings, with close button

### L2: Household Learning

- [x] Household coordination score (high/medium/low)
- [x] Active member tracking
- [x] **Household dietary union** — merged allergens across all members, GET /api/household/dietary
- [x] **Duplicate purchase prevention** — alert when 2 members add same item within 4 hours
- [x] **Repurchase nudge** — GET /api/penny/nudges returns items due for restock based on cycle detection
- [x] **Shopping role detection** — classifies household members as primary_shopper/list_builder/occasional

### L3: Brand Learning

- [x] Brand accept rate tracking
- [x] Brand performance classification (strong/average/weak)
- [x] **Deal performance daily** — daily aggregation of impressions/accepts/declines per deal
- [x] **Segment analysis** — processSegmentAnalysis() + GET /api/admin/segment-analysis
- [x] **Category-level insights** — GET /api/admin/category-insights with top decline reasons per category
- [x] **"Optimize This Deal" recommendations** — GET /brand/optimize/{id} rule-based actionable recommendations
- [x] **Brand portal deep dive UI** — BrandDealDeepDive component with performance chart, decline reasons, optimization recommendations

### L4: Location Learning (Phase 3 — Requires Receipt Scale)

- [x] Store directory from receipt data ✅ `stores` table + `upsertStore()` + `GET /api/stores` with search/chain filter
- [x] Per-store product pricing (moving averages) ✅ `GET /api/stores/{id}/price-averages` with product/brand filters, AVG/MIN/MAX/count
- [x] Store-aware swap card pricing ("At YOUR Kroger: $3.48") ✅ `getStorePrice()` + `getUserActiveStoreID()` enrich swap cards with real store prices and store name
- [x] Regional swap performance aggregation ✅ `GET /api/admin/regional-swaps` — offered/accepted/declined by state, admin-only
- [x] Price comparison across user's stores ✅ `GET /api/price-compare?product=&brand=` — compare prices across all stores, sorted cheapest first

### L5: Business Learning

- [x] **Daily business metrics snapshot** — DAU/WAU/MAU, swap funnel, revenue, brand health
- [x] **Cohort retention analysis** — weekly cohorts, GET /api/admin/cohort-analysis
- [x] **Churn prediction signals** — days_inactive, decline_streak, swap_fatigue, list_abandonment
- [x] **Marketplace health index** — daily supply/demand/balance/concentration scoring, GET /api/admin/marketplace-health
- [x] **Enhanced admin dashboard** — business metrics sparkline, churn risk list, cohort retention table, marketplace health display

### Privacy & Safety Requirements

- "Learn from my choices" toggle gates ALL learning — users with this OFF get zero tracking
- Preference reset hard-deletes all learning data (not soft-delete)
- Allergen filtering is a SAFETY feature — zero tolerance, hard-filter before any swap suggestion
- Brand-level aggregates require min 5 users per cohort (no de-anonymization)
- All learning tables have user_id FK for cascading delete (CCPA/GDPR readiness)

---

## CEO Panel Strategic Recommendations (March 2026)

> 6 unicorn CEO/founders reviewed the learning loop strategy. Full analysis in `LEARNING_LOOPS_STRATEGY.md`.
> Key insight: **Thriftly's long-term value is in the data generated by swap interactions, not the swap fee itself.**

### Tier 1: Quick Wins (immediate revenue impact)

- [x] **Swap interaction enrichment** — `time_to_action_ms`, `why_tapped` on swap_events
- [x] **Category taxonomy** — 92 grocery categories across 14 parent groups, auto-categorization, `GET /api/categories`
- [x] **Category-tiered swap pricing** — $1.00 (economy) / $1.50 (standard) / $2.50 (premium) per parent category, enforced in deal creation with `getCategoryPricingTier()`, `GET /brand/pricing-tiers`
- [x] **Household savings visibility** — `swap_savings_cents` on list_items, green badge in frontend

### Tier 2: Medium-term bets (significant revenue within 90 days)

- [x] **Competitive Swap Intel panel** — `GET /brand/competitive-intel` with per-deal acceptance rate, category rank, competing deals, category summary
- [x] **User preference predictor** — per-user acceptance rate by category+brand, confidence-weighted blending, auto-suppression of low-probability swaps
- [x] **Organic swap propagation tracking** — `swap_propagations` table, daily detection of household brand adoption, admin endpoint
- [x] **Store data schema** — `stores` + `store_prices` tables, search/filter API, admin stats, linked to receipts + trips

### Tier 3: Long-term moats (transformative revenue within 12 months)

- [x] **Category intelligence subscriptions** — `GET /brand/category-report/{category}` with free/premium tier gating, brand_subscriptions table
- [x] **Predictive category alerts** — rising star, declining brand, and category heating detection via `processCategoryAlerts()`, dual admin/brand access
- [x] **Dynamic auction pricing** — brands bid for swap slots when competing in same category ✅ done: bid score = discount × predicted acceptance, highest wins, losers logged as 'outbid'

### Unanimous Panel Recommendations

1. Better targeting first, then more volume — acceptance rate is the north star
2. Competitive Swap Intel is the killer B2B acquisition feature
3. Receipt data unlocks store-level intelligence (more strategic than just attribution)
4. Household is the atomic unit — track MAH (Monthly Active Households), not just MAU
5. Never let brands control Penny's voice — brand copy = guardrails, not scripts

---

## Buy-vs-Build — Scope Reduction (March 2026 VC Panel Review)

> Evaluated by a panel of 6 unicorn VCs. Full analysis in `pitch deck/Buy-vs-Build-Evaluation.docx`.
> Goal: get commodity infrastructure off the founder's plate so all build time goes to swap economics.

### BUY NOW — Replace with off-the-shelf

| Area | Current State | Replace With | Effort | Saves | Cost |
|------|--------------|-------------|--------|-------|------|
| **Error monitoring** | log.Printf() only, no alerting | **Sentry** free tier | 30 min | Hours/incident | $0 |
| **Internal analytics** | ~300 LOC custom SQL in handlers_admin.go | **PostHog** free tier (1M events/mo) | 1 day | Ongoing metric requests | $0 |
| **Brand reporting (M9)** | Not built (1-2 week estimate) | **Metabase OSS** on Railway | Half day | M9 entirely | $5/mo |
| **Real-time sync (R4)** | Polling every few seconds | **Pusher** or **Ably** free tier | 1 day | R4 entirely (2-3 weeks) | $0 |
| **Auth system** | ~500 LOC custom (4 auth methods) | **Clerk** free tier (10K MAU) | 3-5 days | Auth maintenance burden | $0 |

### BUY LATER — When Phase 3 starts

| Area | Current State | Replace With | Effort | Cost |
|------|--------------|-------------|--------|------|
| **Receipt OCR (M7 partial)** | Not built (Textract+Claude planned) | **Veryfi API** for OCR layer | 1 day integration | ~$0.08/receipt |

> Veryfi replaces the Textract OCR layer only. Fraud detection, perceptual hashing, swap verification, and evidence retention are still custom-built.

### KEEP BUILDING — Core IP, never outsource

| Area | Why |
|------|-----|
| **Penny AI brain** (penny.go) | Claude integration for parsing, suggestions, personality — the soul of the product |
| **Swap marketplace** (handlers_list.go) | Deal matching, suppression, budget tracking, rebate ledger — the business model |
| **Brand portal** (M4) | Self-serve brand UX is the product for B2B side (use Metabase for analytics layer underneath) |
| **Competitive Swap Intel** | The FOMO engine — custom view even if rest of brand dashboard is Metabase |

### NOT Databricks

Databricks is enterprise data lakehouse tooling ($10K+/yr, requires a data engineer). PostHog + Metabase covers all analytics and reporting needs through $5M ARR for $0-5/mo.

### Total scope reclaimed: ~5-7 weeks of build time + ongoing maintenance

---

## R6. Codebase Refactor

> **Goal:** Improve maintainability, reduce cognitive load, and prepare the codebase for scale — without changing any user-facing behavior or API contracts.

### Backend Refactor
- **Split monolithic Go files** — `handlers_list.go` (~600 lines), `handlers_auth.go` (~500 lines), and `handlers_brand.go` (~800 lines) are large. Extract into focused files by domain (e.g., `handlers_logo.go`, `handlers_swap.go`, `handlers_receipt.go`).
- **Extract middleware** — Rate limiting, auth gating, and panic recovery are inline. Extract into composable middleware functions.
- **Introduce request/response types** — Many handlers use anonymous `struct{}` for JSON parsing. Define named types in `models.go` for readability and reuse.
- **Database query consolidation** — Several handlers have near-identical SQL queries with slight variations. Extract common queries into typed helper functions.
- **Error handling consistency** — Standardize error responses (some use `writeJSON`, some use `http.Error`, some use raw `w.Write`). Single pattern throughout.

### Frontend Refactor
- **Break up App.tsx** — Currently ~4800 lines with all screens inline. Extract each screen into its own component file (`HomeScreen.tsx`, `ProfileScreen.tsx`, `AdminScreen.tsx`, etc.).
- **Extract inline styles** — All styles are inline objects defined in the component. Move to a shared styles system (CSS modules, styled-components, or at minimum a `styles/` directory).
- **State management** — App.tsx has 50+ useState hooks. Extract related state into custom hooks (`useAuth`, `useList`, `useSwaps`, `useTrips`, etc.).
- **API layer** — Fetch calls are scattered throughout components. Extract into an `api/` module with typed request/response functions.
- **Component library** — Buttons, inputs, cards, and modals are re-implemented with inline styles in multiple places. Extract shared UI primitives.

### Testing Refactor
- **Test helpers** — Reduce boilerplate in `main_test.go` (~10K lines). Extract common setup (create user, create session, add items) into reusable test helpers.
- **Test organization** — Split `main_test.go` into domain-specific test files (`auth_test.go`, `list_test.go`, `swap_test.go`, etc.).

### Priority & Timing
This is a **P2** initiative — do incrementally between feature sprints, not as a dedicated sprint. Each refactor PR should be small, tested, and shippable independently.

---

## R7. UX Overhaul

> **Goal:** Elevate the user experience from functional MVP to polished consumer app. Address rough edges, inconsistencies, and missing polish identified through user testing and founder feedback.

### Visual Design
- **Design system** — Establish a formal design system: color palette, typography scale, spacing scale, shadow/elevation levels, border radii. Currently ad-hoc across components.
- **Dark mode** — User-togglable dark mode. Panel consensus: 5/7 want it, especially for late-night grocery planning.
- **Loading states** — Skeleton loaders exist for list but not for profile, settings, admin, share pages. Add consistent loading states everywhere.
- **Empty states** — Several screens have bare "no items" text. Design proper empty state illustrations with clear CTAs.
- **Error states** — Network errors, API failures, and edge cases show generic messages. Design helpful error states with retry actions.

### Navigation & Information Architecture
- **Bottom tab bar** — Replace hamburger menu with persistent bottom tabs (List, Penny, Share, Profile). Reduce tap-to-reach for core actions.
- **Screen transitions** — Add slide/fade transitions between screens for spatial orientation.
- **Deep linking** — Support direct URLs to specific screens (`/list`, `/penny`, `/profile`, `/share`) for bookmarking and sharing.

### Interaction Polish
- **Pull-to-refresh** — Exists on list; add to all data screens.
- **Swipe gestures** — Swipe-to-delete on list items, swipe-to-dismiss on swap cards.
- **Keyboard shortcuts** — Desktop users: Enter to add item, Escape to close sheets, Tab to navigate.
- **Optimistic updates** — Toggle, delete, and edit operations should update UI instantly, then sync to server. Currently waits for server response.
- **Offline support** — Cache list locally, queue mutations, sync when back online. Service worker already registered via PWA manifest.

### Penny Personality
- **Richer animations** — Penny avatar expressions (happy, thinking, excited) based on context.
- **Contextual suggestions** — Penny proactively suggests based on time of day, season, past purchases.
- **Celebration moments** — Confetti on first swap, first shared list, completing a trip, reaching savings milestones.

### Accessibility
- **Screen reader audit** — Full VoiceOver/TalkBack walkthrough of every screen. Fix any missing labels or confusing navigation order.
- **Reduced motion** — Respect `prefers-reduced-motion` for all animations (typewriter, confetti, transitions).
- **Font scaling** — Ensure all text respects system font size preferences without layout breakage.
- **Color contrast** — WCAG AAA compliance for all text (currently AA).

### Priority & Timing
This is a **P1** initiative — should be planned as a dedicated sprint (or spread across 2 sprints) after core revenue features stabilize. The bottom tab bar and design system are highest-impact items.

---

## Nice-to-Have (Low Priority — May Revisit)

- **M10. Shopper-to-Brand Lead Generation** — Post-swap opt-in "Want a coupon from [Brand]?" prompt. $3-5 per consented lead. Consent-driven, not intrusive. Second revenue stream — revisit after core swap economics are proven.
- **Cloudflare CDN** — Free tier in front of Railway. No code changes needed. Setup: point DNS to Cloudflare, SSL "Full (strict)", cache static assets. Instant perf + security win. ~30 min of config.
- **Savings Share Card** — Shareable image showing "I saved $X with Penny this month." Social proof artifact for organic distribution.
- **Swap Recommendation Engine** — ML-based swap suggestions based on user purchase history. Requires receipt data (M7) at scale. Phase 4+.
- **Multi-level Guild** — Extend Guild earnings beyond direct referrals (2-level, 3-level). Requires careful MLM smell-test UX. Phase 4+.
- **Groflo Coupon Integration** — Pull deals from Groflo's coupon repository as a supply source for Penny Swaps. Currently deals are manually seeded; Groflo API integration automates supply. Phase 4+.
- **Tod/Scout AI Outreach** — Automated LinkedIn outreach to CPG brand managers using wattdata.ai's 580M contact records + Scout's social graph. Not needed while Andy does founder-led sales. Revisit at 50+ brands.
- **PopSavings.com Collaboration** — Tim Connors' parallel project with similar swap mechanics. Potential deal inventory sharing or product merge. Revisit after proving Thriftly's swap model independently.

---

## Future (On Roadmap — Distant)

- **M8. PennyWallet + Guild Payouts** — Wallet tracking swap savings + Guild referral revenue (30% of swap revenue from referral tree). Payouts via Stripe Connect. Requires significant infrastructure (Stripe Connect, ACH, wallet UI, guild_tree). **Revisit at $5K MRR.** For now, track Guild earnings as points/credits redeemable against Thriftly features (avoids money-transmitter complexity).
- **Native iOS app (App Store)** — Founder directive: consumer experience stays in the app. Current PWA works but native unlocks push notifications, camera access for receipts, and App Store distribution. Target: after product-market fit confirmed ($5K MRR).
- **Native Android app (Google Play)** — Same rationale as iOS. Consider React Native or Expo for shared codebase with the web app. Target: after iOS launch.
- **Enterprise Brand Portal** — Multi-user team accounts, SSO, invoicing (not credit card), geographic targeting, demographic targeting, API access. Required for P&G-tier brands. Target: after 50+ paying brands.
- **Apple Sign-In** — Required for iOS App Store submission if app offers other social logins. Add when native iOS app ships.
- **Call Penny (Voice)** — Let users call Penny's phone number and interact via voice. Requires Twilio Voice + speech-to-text + text-to-speech integration. Would enable hands-free list management while driving or cooking.
- **Kroger API Integration** — Connect to Kroger's API for real-time pricing, product availability, and online ordering. Enables price-aware suggestions, store-specific stock checks, and "add to Kroger cart" from the list. Requires Kroger Developer Portal partnership.
- **Walmart API Integration** — Connect to Walmart's Open API for pricing, availability, and cart integration. Similar benefits to Kroger: price comparisons, stock checks, and direct ordering. Would unlock price comparison across retailers when combined with Kroger.

---

## Not In Scope (Explicitly Deferred)

- **Postgres migration** — SQLite is correct until ~200 DAU or funding. See architect review.
- **Horizontal scaling** — Single instance is fine for current scale. Blocked by SQLite anyway.
- **Recipe parsing / meal planning** — Adjacent feature, not core to grocery list MVP.
- **Store price comparison** — **ADR:** For Swap MVP, savings are shown as "estimated savings based on national average price. Your store's price may vary." We accept this risk for v0. Store-specific pricing is Phase 3+ via Kroger/Walmart API integration. Focus group consensus (6/6) was that national average pricing = trust collapse, but launching with a disclosure is acceptable for proving the swap mechanism.
- **Push notifications** — SMS/email channels cover this. Revisit with native apps (Future).
