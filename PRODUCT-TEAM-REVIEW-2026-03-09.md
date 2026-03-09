# PennyAI Product Team Review — CEO Panel Resolutions
**Date:** 2026-03-09
**Reviewers:** Head of Product, Lead Engineer, UX Designer, QA Lead

---

## EXECUTIVE SUMMARY

Andy's 7 resolutions are **implementable and address all customer panel concerns.** Sequencing is correct (P0 items in weeks 1-2, P0.5 in week 3, P1 deferred). No conflicts with existing CLAUDE.md rules. All panel members' concerns are either fully or partially addressed with clear remediation paths.

**Recommendation:** ✅ **APPROVE all 7 resolutions.** Proceed with Week 1-4 implementation sequence.

---

## RESOLUTION-BY-RESOLUTION REVIEW

### Resolution #1: Brand Defaults with Explicit Signal
**"I picked [brand] — tap to change"**

#### Head of Product Assessment
- **Feasibility:** ✅ High. Extends existing brand resolution logic in `/api/list/brand-resolve`.
- **Effort:** M (Medium) — 3-4 days
- **Dependencies:** Existing brand popularity data in `brands` table. No new DB schema needed.
- **Conflicts with CLAUDE.md:** ❌ None. Aligns with "Assume reasonable defaults" rule (line 78).
- **Risk:** Low. UI-only for MVP; backend already selects brands.

#### Lead Engineer Assessment
- **Backend Changes:** Minimal. `POST /api/list` handler already infers brand via Claude. No code change needed; just wire brand + confidence score to UI.
- **Frontend Changes:** Brand card below item with copy "I picked [brand] — tap to change". Add `onBrandTapChange` handler.
- **DB Schema:** No change. Leverage existing `brand_choices` and `brand_preferences` tables.
- **Testing:**
  - Unit test: Brand card renders with correct brand name + tap handler
  - E2E: New user adds item → brand card appears → tap changes brand → confirm selection
  - Edge case: No brand inference (generic brand fallback) → card shows "I picked Organic Valley (generic) — tap to change"
- **Safety:** No mutation risk. Tap-to-change is read+render only until user confirms selection.

#### UX Designer Assessment
- **Interaction Pattern:** Clear and non-threatening. "I picked" removes ambiguity about app autonomy (addresses Nadia's "without permission" concern). "Tap to change" signals quick override.
- **Mobile UX:** Card fits within list item space. 44x44px tap target minimum for brand change button. No horizontal scrolling needed.
- **Visual Design:** Match existing item card styling. Brand logo (if available) + brand name + chevron to indicate tappable.
- **Accessibility:** ARIA label: "Suggested brand for [item name], tap to change". Screen reader announces brand name + change option.
- **Edge Cases:**
  - ❓ No logo available for brand: Fall back to text + generic icon
  - ❓ Very long brand name: Truncate with ellipsis, full name in tooltip
  - ❓ User adds item while offline: Card still renders, brand tap queues request (handles gracefully on reconnect)

#### QA Lead Assessment
- **Test Plan:**
  - New user adds "milk" → brand card shows "I picked Organic Valley — tap to change"
  - Tap card → brand selector appears with top 5 alternatives (see Resolution #3)
  - After brand selection → card updates to new brand immediately
  - User navigates away and back → card reflects selected brand (persistence check)
  - API returns null brand → card shows generic fallback ("I picked Best Value — tap to change")
  - Rapid taps on card → debounce, no duplicate requests
- **Edge Cases:**
  - Empty brand list (API error) → card hidden, item shows without brand
  - User has never corrected brand before → default card shows on every new item (expected behavior)
- **Automation:** Playwright E2E test covers happy path + generic fallback.

#### Panel Concern Addressed
✅ **Nadia's concern fully addressed.** Assumption is now visible + easy to override. Copy "I picked" signals app agency; "tap to change" signals user control.

---

### Resolution #2: Global Popularity + Privacy-Respecting Sourcing
**"I picked [brand] based on popular choices in our app"**

#### Head of Product Assessment
- **Feasibility:** ✅ High. Already inferred from existing `brands` and `brand_owners` tables (aggregate popularity).
- **Effort:** S (Small) — 1-2 days
- **Implementation:** Clarify labeling in brand card (add subtitle "based on popular choices in our app") + update Claude prompt to source from global popularity, not household tracking.
- **Conflicts with CLAUDE.md:** ❌ None. "Popular brand fallback" rule (line 81) doesn't specify source; this clarifies.
- **Privacy Implication:** ✅ Safe. Use COUNT aggregates on `brand_choices`, not personal household data.

#### Lead Engineer Assessment
- **Backend:** Claude prompt must explicitly exclude personal household tracking. Update `penny.go` to:
  ```
  "Use global popularity from app-wide brand choices to suggest a brand."
  "Do NOT use this user's household history to infer brand."
  "Return: [brand_name] with confidence score 0.0-1.0"
  ```
- **Data Source:** Query `SELECT brand_id, COUNT(*) as popularity FROM brand_choices GROUP BY brand_id ORDER BY popularity DESC LIMIT 5`. Cache in Redis for 1 day (no real-time tracking needed).
- **Privacy Audit:** Verify no household_id or user_id is passed to Claude in brand inference prompt.
- **Testing:**
  - Mock Claude to return "Organic Valley milk" (high confidence 0.9)
  - Verify no personal user data in prompt
  - Verify card copy reflects global source ("based on popular choices in our app")

#### UX Designer Assessment
- **Transparency:** Copy "based on popular choices in our app" is clear. Non-technical, warm tone.
- **Trust Signal:** Helps Marcus (privacy skeptic) understand data source upfront. "Our app" = aggregate, not individual tracking.
- **Alternative Wording:**
  - "Popular in our community" (softer, broader)
  - "Top choice among Penny users" (more explicit, slightly colder)
  - Current is best balance.

#### QA Lead Assessment
- **Test Plan:**
  - New user adds "milk" → Claude returns brand + confidence
  - Verify Claude prompt does NOT include household_id, user_id, or past choices
  - Brand card copy shows "based on popular choices in our app"
  - Two independent users both see same brand for "milk" (consistency check)
  - Cache hit on Redis: second request returns instantly
- **Edge Case:** Cache miss → fresh Claude call (takes 1-2 sec, still fast for user)

#### Panel Concern Addressed
✅ **Marcus's concern fully addressed.** Privacy is explicit in copy. Global sourcing removes tracking fear.

---

### Resolution #3: 1-Tap Brand Correction + Top 5 Alternatives + Confirmation Toast
**"When user taps to change, show quick brand selector (top 5 alternatives) + confirmation toast"**

#### Head of Product Assessment
- **Feasibility:** ✅ High. Requires new modal/sheet for brand selection + existing toast system.
- **Effort:** M (Medium) — 3-4 days
- **Dependencies:** Existing brand logo system (3-layer caching). Leverage for brand card icons.
- **Conflicts with CLAUDE.md:** ❌ None. "Brand correction buttons" rule (line 79) supports this exactly.
- **Risk:** Moderate. Toast timing must be tuned; confirm doesn't feel too fast or slow.

#### Lead Engineer Assessment
- **Backend:**
  - Extend `POST /api/list/brand-resolve` to return top 5 alternative brands (ordered by relevance + popularity).
  - Add endpoint `PATCH /api/list/{id}/brand` to update brand choice (idempotent: same item + same brand = 200 OK, no-op).
- **Frontend:**
  - ActionSheet component (existing pattern from PennyAI codebase) with brand selector.
  - Each brand shows: logo + name + price delta if available (deferred to P1).
  - On brand selection → close sheet → update item brand → show toast "✓ Changed to Organic Valley" → auto-dismiss after 3 sec.
- **Idempotency:** `PATCH /api/list/{id}/brand` must check `RowsAffected()` to detect if brand already set (no duplicate tracking).
- **Testing:**
  - Unit: Brand selector renders top 5 brands
  - E2E: Tap brand change → sheet opens → select brand → toast confirms → item updates
  - Negative: Tap brand change → cancel sheet → item unchanged
  - Edge: No top 5 brands available → show generic + current brand + close button

#### UX Designer Assessment
- **Interaction:** 1-tap is excellent UX. ActionSheet is familiar pattern (used throughout app). Top 5 prevents choice paralysis.
- **Toast Timing:** 3 sec is standard. Copy "✓ Changed to Organic Valley" is clear. Dismiss auto or manual (user tap anywhere dismisses).
- **Mobile Layout:** ActionSheet fits within safe area. Brand items stack vertically. 44x44px tap targets (logo + text).
- **Accessibility:** ARIA live region for toast announcement. Focus trap in sheet (not essential for MVP, but recommended).
- **Confirmation:** Toast is sufficient confirmation (not a second modal). Visual feedback + haptic (optional) = good UX.
- **Edge Case: Price Delta**
  - Resolution #6 defers price signal to P1 (post-activation)
  - For MVP (Week 1-3): Show brand selector with logo + name only
  - No price comparison yet (avoids friction in 5-min window)

#### QA Lead Assessment
- **Test Plan:**
  - New user adds "milk" → sees brand card "I picked Organic Valley — tap to change"
  - Tap card → ActionSheet opens with top 5 milk brands (Organic Valley, Kroger, Horizon, Whole Foods, Darigold)
  - Select "Horizon" → sheet closes → item updates to Horizon → toast "✓ Changed to Horizon" appears → disappears after 3 sec
  - Tap brand again → ActionSheet remembers last view position (nice-to-have)
  - Cancel/swipe-down on sheet → no change, item remains Organic Valley
  - Backend: Two rapid taps → only first PATCH succeeds, second is idempotent (same brand, no-op)
- **Edge Cases:**
  - Network error during brand change → toast shows error ("Unable to change brand") → actionable retry
  - Very long brand name → truncate, don't wrap (keep sheet compact)
  - No alternatives (only 1 brand match) → show current + generic option + close
- **Automation:** Playwright E2E test covers happy path + cancel path + error path

#### Panel Concern Addressed
✅ **Tamika's concern fully addressed.** 1-tap + toast visual confirmation satisfies "what if wrong brand for allergies?" + speed requirement.

---

### Resolution #4: Household Invite After 3rd Item, Soft Language
**"Want to invite your family to this list?" instead of "Add household members now"**

#### Head of Product Assessment
- **Feasibility:** ✅ High. Timing logic already exists (track item count). Just change copy + soften tone.
- **Effort:** S (Small) — 1 day
- **Dependencies:** None. Existing household invite flow (modal/actionsheet).
- **Conflicts with CLAUDE.md:** ✅ **Slightly clarifies existing rule.** Line 80 says "After user adds their 3rd item, Penny prompts household invite" — this resolution specifies exact copy + tone.
- **Risk:** Low. Copy-only change; no behavior change.

#### Lead Engineer Assessment
- **Backend:** No change. Item count trigger remains at count === 3.
- **Frontend:**
  - Update household invite prompt/modal copy: "Want to invite your family to this list?" (softer, question form)
  - Remove CTA language like "Add now" or "Invite immediately" — replace with "Invite" button (optional, can dismiss)
  - Add secondary action: "Maybe later" or just dismiss (X button)
  - Persist dismissal state in localStorage so prompt doesn't re-trigger in same session (Notification Safety Hard Rule)
- **Testing:**
  - User adds item #1, #2 → no prompt
  - User adds item #3 → household invite prompt appears with soft copy
  - User dismisses → prompt never re-triggers (same session + future sessions until uninstall/reset)
  - User invites → prompt dismissed, household join flow triggered
- **Safety:** Dismissal persistence REQUIRED. Test: dismiss → reload → no reprompt (localStorage check).

#### UX Designer Assessment
- **Tone:** "Want to invite your family to this list?" is perfect. Conversational, not pushy. Removes obligation feel.
- **Timing:** After 3rd item feels earned (user has demonstrated the app works). Not too early, not too late.
- **Affordances:**
  - Primary: "Invite" button (invokes household invite modal)
  - Secondary: Dismiss/X (non-threatening)
  - No "Not now" button (implies later; just dismiss instead)
- **Dismissal UX:** Prompt fades/slides away. No "remind me later" (keeps simple).
- **Edge Case:** User adds item #4, #5 without dismissing prompt → prompt stays visible (non-intrusive, floats above) OR auto-dismiss after 5 sec (test both).

#### QA Lead Assessment
- **Test Plan:**
  - New user adds 3 items → after item #3, household invite prompt appears
  - Copy reads: "Want to invite your family to this list?"
  - Tap "Invite" → household invite modal opens (existing flow, no change)
  - Tap dismiss (X) → prompt disappears
  - Reload page → prompt does NOT reappear (localStorage persistence check)
  - User navigates away and back → prompt still dismissed (session persistence)
  - Add item #4 → prompt still does NOT reappear (idempotency)
- **Cross-Browser:** Test localStorage persistence on iOS Safari (known quirks)
- **Automation:** Playwright E2E test covers dismiss + reload + no re-trigger path

#### Panel Concern Addressed
✅ **James's concern fully addressed.** Soft language + dismissal option removes "aggressive" feel. Timing (after 3rd item) is accepted as reasonable.

---

### Resolution #5: Brand Defaults Respect Dietary Attributes
**"If user has previously selected organic/allergen-free items, Penny's brand defaults prioritize matching those attributes"**

#### Head of Product Assessment
- **Feasibility:** ⚠️ Moderate. Requires attribute tracking + brand attribute matching. Complexity higher than other resolutions.
- **Effort:** L (Large) — 5-6 days
- **Implementation:**
  1. Add user-level attribute preferences table: `user_dietary_attributes` (user_id, attribute, confidence_score)
  2. Update brand schema to track attributes: `brand_attributes` (brand_id, attribute, source)
  3. Update Claude prompt: "If user has organic preference (70%+ items), suggest organic brands first"
- **Conflicts with CLAUDE.md:** ❌ None. Complements "Brand correction buttons" and "Popular brand fallback" rules.
- **Dependencies:** Brand attribute data must be seeded (manual or via Claude). Small data prep effort.
- **Risk:** Moderate. Attribute matching logic adds complexity; test edge cases carefully.

#### Lead Engineer Assessment
- **DB Schema:**
  - `CREATE TABLE user_dietary_attributes (id INTEGER PRIMARY KEY, user_id TEXT, attribute TEXT, count INTEGER, last_updated TIMESTAMP)`
  - `CREATE TABLE brand_attributes (id INTEGER PRIMARY KEY, brand_id TEXT, attribute TEXT, source TEXT)`
  - Indexes: (user_id, attribute) on first table, (brand_id, attribute) on second
- **Backend:**
  - New function `getUserDietaryPreferences(userID)` → returns map of attributes with confidence (e.g., {organic: 0.72, nut_free: 0.85})
  - Update Claude prompt in `penny.go`:
    ```
    "User preferences: 72% organic items, 85% nut-free items"
    "Suggest brands that match these attributes when available"
    "Fallback to popular brands if no attribute match"
    ```
  - Track attributes on item add: If item has attribute tag (parsed from description), increment user_dietary_attributes
- **Frontend:**
  - No change. Attributes tracked server-side.
- **Data Seeding:**
  - Brand attributes sourced from: (a) manual brand research, (b) user feedback ("this brand is not vegan"), (c) Claude batch job to label popular brands
  - Start with 20-30 top brands + common attributes (organic, nut-free, vegan, dairy-free)
  - Expand iteratively as data grows
- **Testing:**
  - User has history: 10 items all organic → adds milk → Claude returns "Organic Valley" (organic brand)
  - User has history: 5 nut-free items, 2 organic items → adds bread → Claude returns nut-free bread (priority to nut-free over organic)
  - User has no history → falls back to global popularity (existing behavior)
  - Edge: Attribute match exists but low popularity → compare popularity + attribute match (weighted score)

#### UX Designer Assessment
- **Transparency:** How does Penny signal attribute awareness to user?
  - Option A: In brand card: "I picked Organic Valley (organic) — tap to change" — explicit signal
  - Option B: In brand card: "I picked Organic Valley — tap to change" — silent (no attribute mention)
  - **Recommendation:** Option A. Transparent about *why* the brand was chosen. Builds trust (addresses Priya's "dietary safety" concern).
- **Visual Design:** Add attribute badge (small icon or label) next to brand name. "Organic" badge, "Nut-free" badge, etc.
- **Mobile Layout:** Attribute icon/label fits in card (16x16px icon).
- **Copy:** "I picked Organic Valley (organic) — tap to change" is clear. Attribute in parentheses keeps copy concise.

#### QA Lead Assessment
- **Test Plan:**
  - User with 8+ organic items → adds new item → brand card shows organic brand with attribute label
  - User with no history → adds item → brand card shows popular brand (no attribute label)
  - User with mixed attributes (50% organic, 30% nut-free) → adds milk → prioritize majority attribute (organic)
  - User overrides suggestion (taps to change brand) → selection stored, user attribute preference still respected for future items (not overridden by one choice)
  - Rapid attribute inference edge case: User adds item #1 labeled "dairy-free" → should NOT yet influence brand defaults until pattern established (n >= 3 items of same attribute)
- **Negative Cases:**
  - No matching brand exists for attribute → fall back to popularity (no error)
  - Attribute data missing for brand → assume brand has no attributes (conservative)
  - User has conflicting attributes (50% organic, 50% conventional) → show generic popular brand (no attribute in card)
- **Automation:** Playwright E2E test covers happy path (established preference) + new user (no preference yet)

#### Panel Concern Addressed
✅ **Priya's concern fully addressed.** Dietary attributes now drive brand selection. Label shows Penny "respects" health preferences, not just popularity.

---

### Resolution #6: Price Signal Added to Brand Suggestions (Future, P1)
**"First 5 minutes focuses on speed (no delays). After onboarding, Penny shows price impact on brand switches."**

#### Head of Product Assessment
- **Feasibility:** ✅ High. Requires price data integration + UI for price deltas. Deferred to P1 (post-activation validation).
- **Effort:** M (Medium) — 3-4 days (after MVP Phase 1)
- **Timeline:** Week 5+ (after core flywheel ships and activation metrics validate)
- **Conflicts with CLAUDE.md:** ❌ None. New feature, orthogonal to existing rules.
- **Rationale:** Linda (budget optimizer) wants price visibility. But showing price during first 5 min adds friction (user sees "$0.50 more" and second-guesses). Post-onboarding, user is confident → price becomes an asset, not a barrier.
- **Dependencies:** Pricing data source (Instacart API? Manual brand price mapping?). Requires research Week 1.

#### Lead Engineer Assessment
- **Backend:**
  - New table: `brand_prices` (brand_id, store_id, category, price_usd, last_updated)
  - Seed via: (a) Instacart API (if available), (b) manual research (top 20 brands), (c) user submission (future)
  - Endpoint: `POST /api/list/price-compare?item_id={id}&brand_id={brand_id}` → returns {current_price, alt_brand_price, delta_usd}
- **Frontend:**
  - Brand selector (Resolution #3) updated to show price delta:
    - "Organic Valley: $3.99 (+$0.50)"
    - "Kroger: $3.49 (save $0.15)"
  - Only visible *after* first 5 min (post-onboarding flag on user)
  - A/B test: With price vs. without price → measure conversion, avg order value
- **Activation Gate:** `user.onboarding_complete = true` (set after 3 items added + household prompt dismissed) → unlock price visibility
- **Testing:**
  - New user (onboarding_complete = false) → brand selector shows NO price
  - User after 3 items (onboarding_complete = true) → brand selector shows price delta
  - Edge: Price data missing for brand → "Price not available" (fallback)

#### UX Designer Assessment
- **Placement:** In brand selector (Resolution #3 ActionSheet), right-aligned price delta.
- **Copy:** "Organic Valley: $3.99 (+$0.50)" or "Organic Valley: +$0.50" if price is reference point.
- **Color:** Savings in green (#35644F), premium in neutral gray (not red, to avoid negative framing).
- **Accessibility:** Screen reader announces: "Organic Valley, three dollars ninety-nine cents, plus fifty cents"
- **Post-Onboarding Logic:** Smooth reveal. No jarring price appearance. Maybe accompanied by gentle prompt: "Here's the price impact — choose what works for your budget."

#### QA Lead Assessment
- **Test Plan (deferred to Week 5):**
  - New user with onboarding_complete = false → no price shown in brand selector
  - User completes onboarding (3 items) → onboarding_complete flipped to true
  - Tap brand change → price delta appears
  - Missing price data → fallback text ("price not available")
- **Automation:** A/B test framework to measure conversion impact (price visibility vs. no visibility)

#### Panel Concern Addressed
⚠️ **Linda's concern partially addressed in MVP, fully by Week 5.** Price is acknowledged as important but deferred to avoid first-5-min friction. Clear roadmap (P1) + timeline communicated to Linda.

---

### Resolution #7: Data Transparency for Power Users (P2)
**"Add debug panel in Profile: 'Penny knows: You prefer organic [72% of items]...'"**

#### Head of Product Assessment
- **Feasibility:** ✅ High. UI-only feature, leveraging existing attribute + preference data.
- **Effort:** S (Small) — 1-2 days
- **Timeline:** P2 (Week 6+, after core flywheel + price signals ship)
- **Conflicts with CLAUDE.md:** ❌ None. New user-facing feature that increases transparency (aligns with "make Penny smarter over time" narrative).
- **Rationale:** Devon (power user) wants visibility into "what Penny learned." This builds trust + shows data compounding. Optional feature for engaged users.
- **Risk:** Low. Read-only UI. No mutations.

#### Lead Engineer Assessment
- **Backend:**
  - Endpoint: `GET /api/profile/penny-insights` → returns:
    ```json
    {
      "dietary_attributes": [
        { "attribute": "organic", "confidence": 0.72 },
        { "attribute": "nut_free", "confidence": 0.85 }
      ],
      "patterns": [
        { "pattern": "taco_night_fridays", "confidence": 0.60 }
      ],
      "household_size": 3,
      "favorite_brands": [
        { "brand": "Organic Valley", "selections": 12 }
      ]
    }
    ```
  - Compute on-demand (not cached, real-time data)
  - Only visible if user has >= 5 items (enough data to be meaningful)
- **Frontend:**
  - New section in Profile page: "Penny's Insights" (collapsible, off by default)
  - Show attribute confidences as percentages or bars
  - Show detected patterns (e.g., "Taco night on Fridays" with confidence)
  - Show household size (derived from member count)
  - Show top brands user has chosen
- **Testing:**
  - User with <5 items → insights section hidden
  - User with 10+ items → insights show with accurate data
  - Refresh insights → data updates in real-time

#### UX Designer Assessment
- **Design:** Insights as a collapsible card in Profile. Icon (lightbulb?) signals "here's what I learned."
- **Copy:** "Penny's Learning" or "What Penny Knows About You" — warm, not creepy.
- **Accessibility:** Screen reader announces each insight clearly. Data presented as readable text (not just charts).
- **Trust Signal:** Transparency builds confidence. Users see: "Penny is smart because she knows my preferences." Perfect Gate 3 (data flywheel) visual.
- **Optional Opt-Out:** User can toggle off insights collection (Settings → "Help Penny Learn" toggle). But defaults to ON (transparent learning).

#### QA Lead Assessment
- **Test Plan (deferred to Week 6):**
  - User with <5 items → insights section NOT shown
  - User with 5+ items → insights section shown with calculated data
  - Tap to expand → shows breakdown (organic 72%, nut-free 85%, etc.)
  - Refresh profile → insights update with latest data (not cached)
  - User with no patterns detected → insights still show attributes (patterns section empty or hidden)
- **Automation:** Playwright test covers visibility gate + data accuracy

#### Panel Concern Addressed
✅ **Devon's concern fully addressed (P2).** Data transparency is promised + roadmapped. Builds trust in data flywheel.

---

## PANEL MEMBER CONCERN MAPPING

| Panel Member | Concern | Resolution # | Addressed? | Evidence |
|---|---|---|---|---|
| **Nadia** | Assumptions feel like no permission | #1 | ✅ Fully | "I picked [X] — tap to change" makes assumption visible + overridable. Builds user control. |
| **Marcus** | Privacy tracking concern ("similar households") | #2 | ✅ Fully | Global popularity source + explicit label ("popular choices in our app") removes personal tracking fear. |
| **Tamika** | Wrong brands for allergies; need visual confirmation | #3, #5 | ✅ Fully | 1-tap correction + confirmation toast (#3). Dietary attributes prioritized in defaults (#5). Visual attribute badge in card. |
| **James** | Household invite too aggressive | #4 | ✅ Fully | Soft language ("Want to invite your family?") + dismissal option removes pushiness. Timing (after 3rd item) accepted. |
| **Linda** | Where's the price? | #6 | ⚠️ Partially | Price deferred to P1 (after activation validates). Roadmap clear, timeline communicated. MVP focuses on speed. |
| **Priya** | Brand defaults ignore dietary needs | #5 | ✅ Fully | Dietary attributes now drive brand selection. Attribute label visible in card. |
| **Devon** | Data transparency for power users | #7 | ✅ Fully | Debug panel (P2) shows learned preferences, patterns, household size. Optional visibility toggle. |

---

## TECHNICAL ASSESSMENT BY COMPONENT

### Backend (Go)
- **Complexity:** Moderate. Resolution #5 (dietary attributes) is most complex. Others are straightforward extensions.
- **DB Schema:** 3 new tables needed: `user_dietary_attributes`, `brand_attributes`, `brand_prices` (latter in P1).
- **API Endpoints:**
  - No new endpoint paths required for P0. Extend existing `/api/list/brand-resolve` to return attribute metadata.
  - New P1 endpoints: `GET /api/profile/penny-insights`, `POST /api/list/price-compare` (optional, can be computed client-side with cached data).
- **Rate Limiting:** No new rate limit exposure. All changes are read or tied to existing item add flow.
- **Notification Safety:** Resolution #4 (household invite) must persist dismissal state (localStorage). No Twilio/SendGrid changes.

### Frontend (React)
- **Complexity:** Low-to-moderate. ResUI additions (ActionSheet, toast, dismissal logic).
- **Component Changes:**
  - `ListItem.tsx` → Add brand card with "I picked [X] — tap to change"
  - `BrandSelector.tsx` (new) → ActionSheet with top 5 brands + icons + attributes + price (P1)
  - `HouseholdInviteModal.tsx` → Update copy, add dismissal logic + localStorage persistence
  - `Profile.tsx` → Add collapsible insights section (P2)
- **State Management:** Dismissal state needs to persist across sessions. Use localStorage + sessionStorage.
- **Testing:** Playwright E2E coverage required for all UI flows (brand change, household invite dismissal, insights reveal).

### Data Layer (SQLite)
- **New Tables:**
  - `user_dietary_attributes` (user_id, attribute, count, last_updated) — tracks user preferences
  - `brand_attributes` (brand_id, attribute, source) — brand metadata (organic, nut-free, etc.)
  - `brand_prices` (brand_id, store_id, category, price_usd, last_updated) — P1, deferred
- **Migrations:** Use `ALTER TABLE ADD COLUMN` for any new columns on existing tables. No destructive changes.
- **Data Seeding:** Manual brand attribute research needed (20-30 brands initially). Crowd-source from user feedback later.

### AI / Claude Integration
- **Prompt Changes:**
  - Remove household-tracking language; emphasize global popularity
  - Add dietary attributes to context: "User preferences: 72% organic, 85% nut-free"
  - Still prioritize speed (no new API calls, use cached data)
- **Cost:** Minimal. Same Claude call for brand resolution; just include preference context.

---

## IMPLEMENTATION SEQUENCE (WEEKS 1-4)

### Week 1: Resolutions #1, #2 (Brand Defaults + Privacy Label)
- **Backend:** Update Claude prompt for global popularity. Return brand + confidence to frontend.
- **Frontend:** Brand card UI ("I picked [X] — tap to change"). Test localStorage persistence (prep for #4).
- **Testing:** Unit tests + E2E (new user adds item → brand card appears → correct brand name).
- **QA:** Brand accuracy on iOS + Android. Fallback when no brand inferred.
- **Risk:** Low.

### Week 2: Resolution #3 (1-Tap Brand Correction + Toast)
- **Backend:** Extend `/api/list/brand-resolve` to return top 5 alternatives. Add `PATCH /api/list/{id}/brand` endpoint (idempotent).
- **Frontend:** Brand selector ActionSheet. Toast confirmation. Debounce rapid taps.
- **Testing:** Happy path + cancel path + error path + rapid-tap idempotency.
- **QA:** Mobile ActionSheet layout. Toast timing (3 sec). Swipe-to-dismiss.
- **Risk:** Moderate (ActionSheet interaction complexity, but pattern exists in codebase).

### Week 3: Resolutions #4, #5 (Household Invite Soft Language + Dietary Attributes)
- **Week 3A (Days 1-2): Resolution #4 (Soft Language)**
  - Frontend: Update household invite copy. Add dismissal logic + localStorage persistence.
  - Testing: Dismiss → reload → no re-trigger.
  - Risk: Low. Copy + localStorage only.

- **Week 3B (Days 3-5): Resolution #5 (Dietary Attributes)**
  - Backend: Create `user_dietary_attributes` table. Add attribute tracking to item add flow. Update Claude prompt.
  - Frontend: Parse attributes from item descriptions. Attribute label in brand card ("(organic)").
  - Data: Manually seed `brand_attributes` table (20-30 brands). Assign attributes based on research.
  - Testing: User with history → brand defaults respect attributes. No history → falls back to popularity.
  - Risk: Moderate. Attribute matching logic + brand data seeding.

### Week 4+: Resolutions #6, #7 (Deferred to P1+)
- **P1 (Week 5+):** Price signals in brand selector. A/B test with/without price visibility.
- **P2 (Week 6+):** Insights panel in Profile. Visibility gate (≥5 items) + refresh on demand.

---

## RISK ASSESSMENT & MITIGATION

| Risk | Severity | Mitigation |
|---|---|---|
| **Dismissal state lost (localStorage)** | High | Test localStorage persistence across page reloads + browser restarts (iOS Safari quirks). Fallback: server-side dismissal flag if localStorage fails. |
| **Attribute matching reduces brand diversity** | Medium | Monitor: % of brand suggestions that match attributes vs. global popularity. If too narrow, widen matching logic (e.g., "organic OR conventional" if confidence < 60%). |
| **Price data outdated or missing** | Medium (P1) | Start with manual brand prices (20 brands). Use Instacart API or similar for continuous updates. Graceful fallback: "price unavailable" if no data. |
| **Claude inference latency affects first-5-min** | Medium | Cache Claude brand responses in Redis (1 hour TTL). Measure: p50 + p95 latency for brand resolution. Target <500ms. |
| **Rapid taps on brand change cause duplicate updates** | Medium | Idempotent `PATCH /api/list/{id}/brand` with RowsAffected() check. Test: concurrent requests, same brand → only first succeeds. |
| **Attribute data sparse (new users)** | Low | Fallback to global popularity. As users add more items, attribute confidence increases naturally (compounding). |

---

## DEPENDENCIES & BLOCKERS

- **Data Seeding (Attributes):** Manual research needed for top brands. ~3 hours work. Can start in parallel with development (assign to product/ops).
- **Price Data Source (P1):** Need to decide on Instacart API vs. manual mapping. Blocking decision for Week 5. **Action item:** Andy to approve price source by end of Week 1.
- **Claude Prompt Testing:** New prompt for dietary attributes needs validation. **Action item:** QA to test 10+ items with different attribute distributions (organic/nut-free mix).

---

## RECOMMENDATION

✅ **APPROVE ALL 7 RESOLUTIONS.** Proceed with implementation:
1. **Weeks 1-3:** P0 implementations (resolutions #1-5). Ship activation optimizations.
2. **Week 5+:** P1 (price signals). Measure impact on conversion + AOV.
3. **Week 6+:** P2 (insights). Reinforce data flywheel narrative.

**All panel concerns are addressed.** No conflicts with CLAUDE.md. Sequencing is sound. Risk is manageable with proposed mitigations.

---

## SIGN-OFF

- **Head of Product:** ✅ Implementable, no conflicts, sequencing correct.
- **Lead Engineer:** ✅ Backend + frontend changes are feasible. DB schema migration path clear. Test plan complete.
- **UX Designer:** ✅ Interactions are clear, accessible, mobile-friendly. Copy is warm + transparent.
- **QA Lead:** ✅ Test coverage planned for all resolutions. Edge cases identified. E2E automation roadmap established.

**Date:** 2026-03-09
**Status:** Ready for handoff to implementation team.
