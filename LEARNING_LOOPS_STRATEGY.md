# Thriftly Learning Loops Strategy — Product Team Convene

> **Date:** March 8, 2026
> **Convened by:** PM (Product Manager)
> **Participants:** All 7 roles — PM, UX Designer, Architect, Data Scientist, Growth Lead, QA Lead, Security/Privacy
> **Objective:** Design a 5-level learning system that makes Penny smarter every day — for individual users, households, brands, locations, and the business overall.

---

## Executive Summary

Thriftly already captures rich behavioral signals — swap events, brand choices, brand preferences, receipt line items, shopping trips, household membership, and list activity. But today these signals are siloed: brand preferences inform one-off disambiguation, swap suppressions prevent bad re-offers, and receipt matching improves abbreviation lookups. There is no unified learning layer that synthesizes signals across levels to make Penny progressively smarter.

This strategy defines 5 nested learning loops, each building on the one below:

| Level | Learning Unit | Core Question | Key Flywheel |
|-------|--------------|---------------|--------------|
| 1. Individual | Single user | "What does THIS person want?" | Better suggestions reduce friction, increasing engagement, generating more signal |
| 2. Household | Shared group | "How does this household coordinate?" | Better coordination reduces duplicate trips/purchases, increasing household value |
| 3. Brand | Advertiser | "What makes a swap succeed?" | Better matching increases acceptance rates, increasing brand ROI and spend |
| 4. Location | Store/region | "What's available and at what price WHERE?" | Store-aware suggestions increase trust and accuracy, driving receipt submission |
| 5. Business | Platform | "What makes the marketplace healthy?" | Better unit economics fund better product, attracting more users and brands |

**PM Decision:** Ship Level 1 and Level 3 together in the next sprint (they share the same data pipeline and have the highest revenue impact). Level 2 follows in the sprint after. Levels 4 and 5 are Phase 3 items that require receipt data at scale.

---

## Role-by-Role Commentary

### PM (Product Manager)
The core insight is that learning loops are not a feature — they are infrastructure that makes every feature better. A smarter Penny increases swap acceptance rates (revenue), reduces list friction (retention), and creates competitive moat (defensibility). The prioritization is ruthlessly revenue-first: individual preference learning and brand performance learning ship first because they directly increase swap acceptance rate (our #1 revenue lever). Household and location learning are retention/engagement plays that follow.

### UX Designer
Learning must be invisible and delightful, never creepy. Users should feel "Penny knows me" not "Penny is tracking me." Three design principles:
1. **Explain when it matters.** If Penny suggests something because of a learned preference, she should be able to say why: "I noticed you usually pick organic — here's an organic option." Never use opaque "recommended for you" language.
2. **Let users correct and control.** Every learned preference should be editable. If Penny is wrong about something, the user must be able to tell her, and she must listen immediately (not after 3 more wrong suggestions).
3. **Progressive disclosure.** New users see simple suggestions. Power users see Penny surface deeper insights. Never overwhelm on day 1.

**Edge cases to handle:** Empty learning state (new users), conflicting household preferences, dietary restriction changes, moving to a new city (invalidating location data), seasonal preference shifts.

### Architect
SQLite is the right database for this stage. The learning tables are append-only event streams with materialized preference summaries — a pattern SQLite handles well. Key architectural decisions:
1. **Event sourcing for all learning signals.** Raw events go into append-only tables. Derived preferences are materialized views that can be recomputed.
2. **Batch aggregation, not real-time ML.** At our scale (<200 DAU), a nightly batch job that recomputes preference scores from events is simpler and sufficient. No Spark/Flink needed.
3. **Penny's system prompt enrichment.** Learning data flows into Penny's Claude prompt as structured context: "This user prefers organic, is price-sensitive, has declined Brand X twice, usually shops at Kroger on Sundays."
4. **Keep the reporting DB pattern.** Heavy aggregation runs against the VACUUM-INTO clone, not production.

### Data Scientist
The five levels form a hierarchy of aggregation. Key metrics and models:

| Level | Primary Metric | Model Type | Training Signal |
|-------|---------------|------------|-----------------|
| Individual | Swap acceptance rate per user | Collaborative filtering (user-item) | Swap events + list additions + receipt items |
| Household | List completeness rate | Association rules | Shared list patterns + trip data |
| Brand | Campaign ROI | Logistic regression | Accept/decline events + user segments |
| Location | Price accuracy score | Moving average | Receipt prices + API prices |
| Business | Marketplace health index | Composite score | All of the above |

**Critical insight:** We do NOT need ML for v1. Simple heuristics (frequency counts, recency weighting, co-occurrence) outperform ML models at small data volumes. ML becomes valuable at >1000 MAU. Build the data pipeline now; add ML later.

### Growth Lead
Learning loops are the ultimate retention and viral mechanic:
1. **Personalization = retention.** Users who feel Penny "gets them" churn 60% less (industry benchmark from Stitch Fix, Netflix).
2. **Household learning = network effects.** A household that coordinates well through Penny is locked in — switching costs are enormous.
3. **Brand learning = revenue growth.** Better swap matching means higher acceptance rates, which means brands get better ROI, which means they spend more.
4. **"Penny knows" moments = viral loops.** When Penny predicts something correctly ("I bet you need milk — you buy it every 8 days"), that is a shareable moment. Build share prompts around prediction hits.

### QA Lead
Learning systems introduce a new class of bugs: **stale learning, runaway feedback loops, and preference drift.**

Test strategy must include:
1. **Preference correctness.** After N brand choices, the preference table reflects the right brand. After a user corrects a preference, the correction takes effect immediately.
2. **No feedback loops.** If Penny suggests X because user accepted X before, and user accepts again (because it was suggested), this should not create runaway over-suggestion of X. Diversity must be maintained.
3. **Learning reset.** Users must be able to reset what Penny knows about them. This must actually clear all derived data, not just hide it.
4. **Cross-user isolation.** User A's preferences must never leak to User B (except via household sharing, which is explicitly opted-in).
5. **Backward compatibility.** Existing users with no learning data must not see degraded behavior.

### Security/Privacy
Learning data is the most sensitive data in the system. Grocery purchase patterns reveal:
- Health conditions (diabetes supplies, medications, vitamins)
- Religious practices (halal, kosher, Lent-period purchases)
- Financial stress (coupon usage patterns, store-brand preference shifts)
- Household composition (baby products, pet food, eldercare items)

**Hard requirements:**
1. **Explicit consent.** "Learn from my choices" toggle already exists. It must gate ALL learning, not just autotune. Users with this OFF get zero preference tracking.
2. **Data minimization.** Store aggregated preferences, not raw purchase history (when possible). "Prefers organic" is less sensitive than "bought prenatal vitamins on March 3."
3. **Right to delete.** "Reset what Penny knows about me" must purge ALL learning tables for that user. Not soft-delete — hard delete with confirmation.
4. **No cross-level leakage without consent.** Individual learning stays individual. Household learning requires household membership. Brand-level data is always aggregated and anonymized (min 5 users per cohort).
5. **Receipt data isolation.** Receipt images and line items are the most sensitive data. The existing "Thriftly as trusted intermediary" principle (brands never see receipts) must extend to learning: brand-level learning aggregates must never be de-anonymizable to individual users.
6. **CCPA/GDPR readiness.** Learning data must be included in data export and data deletion requests. Design tables with user_id FK so cascading delete works.

---

## LEVEL 1: INDIVIDUAL LEARNING — Penny Learns About Each User

### What to Track

| Signal Category | Specific Data | Source | Already Exists? |
|----------------|--------------|--------|-----------------|
| Brand preferences | Which brands user picks in disambiguation | `brand_choices` table | YES |
| Brand preference streaks | Consecutive picks of same brand in category | `brand_preferences` table | YES |
| Swap behavior | Accept/decline per deal, decline reasons | `swap_events`, `swap_suppressions` | YES |
| Price sensitivity | Convenience/savings slider position | `users.penny_slider` | YES |
| Autotune signal | Accept/decline ratio drives slider | `users.penny_autotune` | YES |
| Shopping frequency | Time between list creation events | Derivable from `list_items.created_at` | PARTIAL |
| Category affinity | Which departments user shops most | Derivable from `list_items.department` | PARTIAL |
| Dietary profile | Allergens, preferences (organic, vegan, etc.) | **NEW** | NO |
| Time-of-day patterns | When user adds items, when they shop | Derivable from timestamps | PARTIAL |
| Item repurchase cycles | How often user buys milk, bread, etc. | **NEW** (derived from receipts + list) | NO |
| Decline reasons | Why user said no to a swap | `swap_events` (action column) | PARTIAL — no structured reason field |

### New DB Tables and Columns

```
-- User dietary/preference profile (explicit, user-set)
CREATE TABLE IF NOT EXISTS user_dietary_profiles (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL UNIQUE REFERENCES users(id),
    allergens       TEXT NOT NULL DEFAULT '[]',     -- JSON array: ["peanuts","gluten","dairy"]
    diets           TEXT NOT NULL DEFAULT '[]',     -- JSON array: ["organic","vegan","keto"]
    avoid_brands    TEXT NOT NULL DEFAULT '[]',     -- JSON array: ["Nestle","Monsanto"]
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Learned shopping patterns (derived, system-computed)
CREATE TABLE IF NOT EXISTS user_shopping_patterns (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id),
    pattern_type    TEXT NOT NULL,          -- 'repurchase_cycle', 'shopping_day', 'category_affinity', 'price_tier'
    pattern_key     TEXT NOT NULL,          -- e.g., 'milk', 'sunday', 'produce', 'budget'
    pattern_value   TEXT NOT NULL,          -- e.g., '8' (days), '0.72' (affinity score), 'high' (tier)
    confidence      REAL NOT NULL DEFAULT 0.0,  -- 0.0 to 1.0
    sample_count    INTEGER NOT NULL DEFAULT 0,
    last_computed   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(user_id, pattern_type, pattern_key)
);

-- Structured decline reasons on swap events
ALTER TABLE swap_events ADD COLUMN decline_reason TEXT NOT NULL DEFAULT '';
-- Values: '', 'tried_before', 'dietary', 'brand_loyal', 'price', 'other'

-- Penny context cache (pre-computed prompt enrichment per user)
CREATE TABLE IF NOT EXISTS penny_context_cache (
    user_id         INTEGER PRIMARY KEY REFERENCES users(id),
    context_json    TEXT NOT NULL DEFAULT '{}',
    computed_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_user_dietary_user ON user_dietary_profiles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_patterns_user_type ON user_shopping_patterns(user_id, pattern_type);
```

### New API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/preferences` | Returns user's full preference profile (dietary, brand prefs, learned patterns) |
| `PATCH` | `/api/preferences` | Update dietary profile (allergens, diets, avoid-brands) |
| `DELETE` | `/api/preferences/reset` | Hard-delete all learned data for the user (right to forget) |
| `GET` | `/api/preferences/insights` | Returns Penny's insights about the user ("You buy milk every 8 days") |

### New Frontend Features

1. **Dietary Profile Setup** — Accessible from Profile screen. Multi-select chips for allergens (peanuts, tree nuts, dairy, gluten, shellfish, soy, eggs, fish) and diets (organic, vegan, vegetarian, keto, paleo, halal, kosher). Penny can also set these conversationally: "I'm lactose intolerant" -> Penny updates profile.
2. **"Penny Knows" Insights Card** — On the Penny chat screen, a subtle card showing what Penny has learned: "You prefer organic produce, shop mostly on Sundays, and buy milk every 8 days." Tappable to edit/correct any insight.
3. **Structured Decline Reasons** — When declining a swap, optional quick-tap reasons appear: "Tried it", "Dietary", "Brand loyal", "Too expensive", "Not interested". Not mandatory — "No thanks" still works with no reason.
4. **Preference Reset** — In Profile > Privacy section: "Reset what Penny knows about me" button with confirmation modal.

### PostHog Events

| Event | Properties | When |
|-------|-----------|------|
| `dietary_profile_updated` | `allergens_count`, `diets_count`, `source` (manual/conversational) | User changes dietary profile |
| `decline_reason_provided` | `reason`, `deal_id`, `brand` | User provides structured decline reason |
| `decline_reason_skipped` | `deal_id` | User declines without giving reason |
| `preference_insight_viewed` | `insights_count` | User views Penny's learned insights |
| `preference_insight_corrected` | `pattern_type`, `old_value`, `new_value` | User corrects a learned insight |
| `preference_reset` | `patterns_cleared_count`, `dietary_cleared` | User resets all learning data |
| `penny_prediction_hit` | `prediction_type`, `item` | Penny's suggestion matches what user was going to add |
| `penny_prediction_miss` | `prediction_type`, `item` | Penny's suggestion was dismissed/ignored |

### Feedback Loop Mechanism

```
[User adds items to list]
       |
       v
[System records: item, brand, department, time, frequency]
       |
       v
[Nightly batch job computes patterns]
  - Repurchase cycles (item frequency over receipts + list additions)
  - Category affinities (department distribution)
  - Shopping time preferences (day-of-week, time-of-day)
  - Price tier (slider position + accept/decline ratio)
       |
       v
[penny_context_cache updated with structured JSON]
       |
       v
[Next Penny interaction: context injected into Claude system prompt]
  - "User prefers organic (confidence 0.85), buys milk every 8 days (last: 6 days ago),
     shops Sundays, price-sensitive (slider 20/30), has declined Brand X twice"
       |
       v
[Penny makes better suggestions, user responds]
       |
       v  (loop restarts)
```

### Acceptance Criteria

- [ ] User can set dietary allergens and diet preferences via Profile UI
- [ ] Penny conversationally understands "I'm vegan" and updates dietary profile
- [ ] Swap suggestions are hard-filtered against allergen profile (SAFETY — zero tolerance)
- [ ] Decline reason capture works on swap dismissal (optional, not blocking)
- [ ] Nightly batch job computes shopping patterns and updates `user_shopping_patterns`
- [ ] `penny_context_cache` is populated and Penny's prompt includes learned context
- [ ] Penny's suggestions measurably improve (A/B test: users with learning ON vs OFF)
- [ ] "Reset what Penny knows" hard-deletes all learning tables for the user
- [ ] Users with "Learn from my choices" OFF have zero rows in learning tables

### Timeline: M (2-3 sprints)

- Sprint 1: Dietary profile table + UI + API + allergen hard-filter on swaps
- Sprint 2: Pattern computation batch job + context cache + prompt enrichment
- Sprint 3: Insights card + decline reasons + preference correction UI

---

## LEVEL 2: HOUSEHOLD LEARNING — Penny Learns About Household Dynamics

### What to Track

| Signal Category | Specific Data | Source | Already Exists? |
|----------------|--------------|--------|-----------------|
| Who shops when | Which household member completes trips | `shopping_trips` + `household_members` | PARTIAL |
| Item ownership | Who adds which items, who checks them off | `list_items.user_id` + `item_modifications` | PARTIAL |
| Shared preferences | Preferences that overlap across members | Derivable from Level 1 data | NO |
| Dietary conflicts | Allergens across household members | Derivable from `user_dietary_profiles` | NO (needs Level 1 first) |
| Budget patterns | Household-level spending from receipts | `receipts` aggregate | PARTIAL |
| Shopping cadence | How often the household shops as a unit | `shopping_trips` timestamps | PARTIAL |
| Role patterns | Primary shopper, list builder, etc. | Derivable from activity | NO |

### New DB Tables and Columns

```
-- Household-level computed patterns
CREATE TABLE IF NOT EXISTS household_patterns (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    household_id    INTEGER NOT NULL REFERENCES households(id),
    pattern_type    TEXT NOT NULL,          -- 'shopping_cadence', 'primary_shopper', 'budget_weekly', 'dietary_union'
    pattern_key     TEXT NOT NULL,          -- e.g., 'avg_days_between_trips', 'user_id_123', 'total_cents'
    pattern_value   TEXT NOT NULL,
    confidence      REAL NOT NULL DEFAULT 0.0,
    sample_count    INTEGER NOT NULL DEFAULT 0,
    last_computed   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(household_id, pattern_type, pattern_key)
);

-- Household dietary union (merged allergens across all members)
CREATE TABLE IF NOT EXISTS household_dietary_union (
    household_id    INTEGER PRIMARY KEY REFERENCES households(id),
    all_allergens   TEXT NOT NULL DEFAULT '[]',    -- JSON: union of all members' allergens
    all_diets       TEXT NOT NULL DEFAULT '[]',    -- JSON: union of all members' diets
    conflict_notes  TEXT NOT NULL DEFAULT '[]',    -- JSON: ["James is vegan but Linda is not"]
    computed_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_household_patterns_hh ON household_patterns(household_id, pattern_type);
```

### New API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/household/insights` | Returns Penny's household insights (shopping cadence, roles, dietary summary) |
| `GET` | `/api/household/dietary` | Returns merged dietary profile for the household |

### New Frontend Features

1. **Household Insights in Share Screen** — The settings-share screen gains a "Household Insights" section: "Your household shops every 5 days on average. James usually adds items, Linda usually shops. You have 2 shared dietary restrictions."
2. **Dietary Conflict Warning** — When a household member sets a dietary restriction, Penny alerts the household: "Heads up — James just marked a peanut allergy. I'll filter peanut products from swap suggestions for the whole household."
3. **"You might be forgetting..." Nudge** — Based on repurchase cycle analysis across the household, Penny can prompt: "Your household usually buys paper towels every 3 weeks — it's been 4 weeks. Add to list?" (Dismissible, once per item per cycle.)
4. **Duplicate Purchase Prevention** — If two household members both add "milk" within a short window, Penny asks the second one: "Linda already added milk 2 hours ago — still want to add more?"

### PostHog Events

| Event | Properties | When |
|-------|-----------|------|
| `household_insight_viewed` | `household_size`, `insights_count` | User views household insights |
| `household_dietary_conflict` | `allergen`, `members_affected` | Dietary conflict detected |
| `household_repurchase_nudge` | `item`, `days_overdue`, `accepted` | Repurchase reminder shown |
| `household_duplicate_prevented` | `item`, `original_adder` | Duplicate add caught |
| `household_duplicate_added_anyway` | `item` | User overrode duplicate warning |

### Feedback Loop Mechanism

```
[Household members interact with shared list]
       |
       v
[System aggregates: who adds/checks what, trip frequency, receipt totals]
       |
       v
[Nightly batch: compute household_patterns + household_dietary_union]
  - Primary shopper (most trips completed)
  - Shopping cadence (avg days between trips)
  - Budget range (receipt total moving average)
  - Dietary union (allergen merge across members)
  - Repurchase gaps (items overdue based on household cycle)
       |
       v
[Penny enrichment: household context added to prompts for ALL members]
  - "This is a 3-person household. Primary shopper: Linda.
     Shared allergens: none. James is vegan. Avg shopping: every 5 days.
     Overdue items: paper towels (4 weeks, usually 3)."
       |
       v
[Better coordination suggestions, fewer conflicts, smarter nudges]
       |
       v  (loop restarts)
```

### Acceptance Criteria

- [ ] Household dietary union correctly merges all members' allergens
- [ ] Dietary conflict alerts fire when a new restriction is added
- [ ] Repurchase nudge fires for items overdue by >25% of their cycle
- [ ] Nudge is dismissible and does not re-fire for the same item in the same cycle
- [ ] Duplicate add prevention catches same-item adds within 4 hours by different members
- [ ] User can override duplicate warning ("Add anyway")
- [ ] Household insights display correctly for 1-member, 2-member, and 5-member households
- [ ] Leaving a household removes that member's data from household patterns
- [ ] All household learning respects individual "Learn from my choices" toggles

### Timeline: M (2 sprints)

- Sprint 1: Household pattern computation + dietary union + API
- Sprint 2: Frontend insights + nudges + duplicate prevention

---

## LEVEL 3: BRAND LEARNING — Penny Learns What Makes Swaps Work

### What to Track

| Signal Category | Specific Data | Source | Already Exists? |
|----------------|--------------|--------|-----------------|
| Acceptance rate by deal | Per-deal accept/decline/ignore | `swap_events` | YES |
| Acceptance by user segment | Rate by price_tier, dietary_profile, shopping_frequency | Join `swap_events` with Level 1 data | NO |
| Decline reasons by brand | Aggregated decline reasons per deal/brand | `swap_events.decline_reason` (new) | NO (needs Level 1) |
| Price elasticity | At what discount % does acceptance spike | `swap_events` + `brand_deals.discount_cents` | PARTIAL |
| Time-to-decision | How long between swap shown and accept/decline | **NEW** | NO |
| "Why?" engagement | How often users tap "Why?" and whether it correlates with acceptance | **NEW** | NO |
| Category performance | Which product categories have highest swap rates | `brand_deals` + `swap_events` | PARTIAL |
| Seasonal patterns | Acceptance rate variation by time of year | `swap_events.created_at` | PARTIAL |
| Competitive landscape | Which brands are being swapped OUT vs IN most | `swap_events` + `brand_deals` aggregate | PARTIAL (category-intel exists) |
| Receipt verification rate | % of accepted swaps that are purchase-verified | `swap_claims` | YES |

### New DB Tables and Columns

```
-- Brand deal performance metrics (materialized, recomputed daily)
CREATE TABLE IF NOT EXISTS deal_performance_daily (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    deal_id         INTEGER NOT NULL REFERENCES brand_deals(id),
    date            TEXT NOT NULL,           -- YYYY-MM-DD
    impressions     INTEGER NOT NULL DEFAULT 0,
    accepts         INTEGER NOT NULL DEFAULT 0,
    declines        INTEGER NOT NULL DEFAULT 0,
    ignores         INTEGER NOT NULL DEFAULT 0,
    why_taps        INTEGER NOT NULL DEFAULT 0,
    avg_decision_ms INTEGER NOT NULL DEFAULT 0,  -- avg milliseconds from show to action
    revenue_cents   INTEGER NOT NULL DEFAULT 0,
    verified_count  INTEGER NOT NULL DEFAULT 0,
    UNIQUE(deal_id, date)
);

-- Segment-level performance (which user segments accept which deals)
CREATE TABLE IF NOT EXISTS deal_segment_performance (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    deal_id         INTEGER NOT NULL REFERENCES brand_deals(id),
    segment_type    TEXT NOT NULL,           -- 'price_tier', 'diet', 'frequency', 'category_affinity'
    segment_value   TEXT NOT NULL,           -- 'budget', 'organic', 'weekly', 'produce'
    impressions     INTEGER NOT NULL DEFAULT 0,
    accepts         INTEGER NOT NULL DEFAULT 0,
    declines        INTEGER NOT NULL DEFAULT 0,
    acceptance_rate REAL NOT NULL DEFAULT 0.0,
    last_computed   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(deal_id, segment_type, segment_value)
);

-- Swap interaction timing (for time-to-decision analytics)
ALTER TABLE swap_events ADD COLUMN shown_at TEXT NOT NULL DEFAULT '';
ALTER TABLE swap_events ADD COLUMN acted_at TEXT NOT NULL DEFAULT '';
ALTER TABLE swap_events ADD COLUMN why_tapped INTEGER NOT NULL DEFAULT 0;

-- Category-level learning (aggregated across all deals in a category)
CREATE TABLE IF NOT EXISTS category_swap_insights (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    category        TEXT NOT NULL UNIQUE,
    avg_acceptance_rate REAL NOT NULL DEFAULT 0.0,
    avg_discount_pct    REAL NOT NULL DEFAULT 0.0,
    top_decline_reason  TEXT NOT NULL DEFAULT '',
    best_time_of_week   TEXT NOT NULL DEFAULT '',  -- e.g., 'saturday_morning'
    total_swaps         INTEGER NOT NULL DEFAULT 0,
    last_computed       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_deal_perf_daily_deal ON deal_performance_daily(deal_id, date);
CREATE INDEX IF NOT EXISTS idx_deal_segment_deal ON deal_segment_performance(deal_id, segment_type);
```

### New API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/brand/deal-performance/:id` | Detailed performance for a specific deal (daily breakdown, segments) |
| `GET` | `/brand/category-insights/:category` | Category-level learning insights (best discount, best time, decline reasons) |
| `GET` | `/brand/segment-analysis/:id` | Which user segments respond best to this deal |
| `GET` | `/brand/recommendations` | AI-generated recommendations for improving deal performance |

### New Frontend Features (Brand Portal)

1. **Deal Performance Deep Dive** — Click any deal on the brand dashboard to see daily performance chart, segment breakdown, decline reason distribution, and time-to-decision histogram.
2. **"Optimize This Deal" Recommendations** — AI-generated suggestions based on learning data: "Your acceptance rate is 6%. Deals in this category average 11%. Consider: increasing discount by $0.25 (similar deals at $1.75 see 14% acceptance), adding 'No artificial sweeteners' to your why-bullets (top concern for decliners in this category)."
3. **Segment Targeting** — Brand dashboard shows which user segments respond best: "Budget shoppers accept this deal at 18% vs 5% for organic-preferring shoppers. Consider creating a separate deal with organic positioning for that segment."
4. **Competitive Position Chart** — Enhanced category intel showing brand's position: swap-in rate, swap-out rate, net swap momentum over time.

### New Frontend Features (Consumer App)

1. **Swap Timing Tracking** — Invisible to user; records when swap card enters viewport and when user acts. Used for time-to-decision metric.
2. **"Why?" Tap Tracking** — Already exists as a button; add PostHog event tracking on tap.

### PostHog Events

| Event | Properties | When |
|-------|-----------|------|
| `swap_card_shown` | `deal_id`, `brand`, `category`, `timestamp` | Swap card enters viewport |
| `swap_card_acted` | `deal_id`, `action`, `time_to_decision_ms`, `why_tapped` | User accepts/declines |
| `swap_why_tapped` | `deal_id`, `brand` | User taps "Why?" on swap card |
| `brand_perf_viewed` | `deal_id`, `brand_user_id` | Brand views deal performance |
| `brand_recommendation_viewed` | `deal_id`, `recommendation_type` | Brand views optimization suggestion |
| `brand_recommendation_applied` | `deal_id`, `recommendation_type`, `change` | Brand acts on suggestion |

### Feedback Loop Mechanism

```
[User interacts with swap card (accept/decline/ignore/why)]
       |
       v
[swap_events records action + timing + decline reason]
       |
       v
[Nightly batch: aggregate into deal_performance_daily + deal_segment_performance]
  - Per-deal daily metrics
  - Per-segment acceptance rates
  - Category-level patterns
  - Time-to-decision distributions
       |
       v
[Brand dashboard shows performance + AI recommendations]
       |
       v
[Brand adjusts deal (price, why-bullets, targeting)]
       |
       v
[Better deals offered to users → higher acceptance → more signal]
       |
       v  (loop restarts)

ALSO:

[Aggregated brand learning feeds back into Penny's deal matching]
       |
       v
[Penny prioritizes deals with high predicted acceptance for each user's segment]
  - Budget user? Show the high-discount deal first.
  - Organic user? Show the deal with "organic" why-bullets.
  - Sunday shopper? Time the swap suggestion for planning mode, not mid-trip.
```

### Acceptance Criteria

- [ ] `deal_performance_daily` is populated by nightly batch for all active deals
- [ ] Brand dashboard shows daily performance chart with impressions/accepts/declines
- [ ] Decline reason distribution is visible per deal (pie chart or bar)
- [ ] Segment analysis shows acceptance rate by price_tier, diet, and frequency
- [ ] "Optimize This Deal" generates at least 2 actionable recommendations per deal
- [ ] Recommendations reference actual data (not generic advice)
- [ ] Category insights aggregate correctly across all deals in a category
- [ ] All brand-facing metrics use aggregated data with min 5 users per cohort (privacy)
- [ ] Swap matching prioritizes deals with higher predicted acceptance for the user's segment

### Timeline: M (2-3 sprints)

- Sprint 1: Timing columns on swap_events + deal_performance_daily batch job
- Sprint 2: Segment analysis + brand dashboard deep dive UI
- Sprint 3: AI recommendations engine + deal matching improvements

---

## LEVEL 4: LOCATION LEARNING — Penny Learns About Stores and Geography

### What to Track

| Signal Category | Specific Data | Source | Already Exists? |
|----------------|--------------|--------|-----------------|
| Store prices | Actual per-item prices from receipts | `receipt_line_items` + `receipts.store_name` | YES (needs aggregation) |
| Store identification | Which stores users shop at | `receipts.store_name` + `shopping_trips` | PARTIAL |
| Regional brand preferences | Which brands are popular by zip code | `brand_choices` + `users.address` | PARTIAL |
| Store-specific availability | Which products are found at which stores | Receipt data + user-reported | NO |
| Price variance by store | Same product, different prices across stores | `receipt_line_items` cross-store | NO |
| Store-specific swap performance | Acceptance rates vary by store context | `swap_events` + store data | NO |
| Regional deal effectiveness | Deals work differently in different markets | `swap_events` + geography | NO |

### New DB Tables and Columns

```
-- Normalized store directory (populated from receipt data + manual curation)
CREATE TABLE IF NOT EXISTS stores (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,              -- "Kroger"
    chain           TEXT NOT NULL DEFAULT '',   -- "Kroger Co" (parent company)
    address         TEXT NOT NULL DEFAULT '',
    city            TEXT NOT NULL DEFAULT '',
    state           TEXT NOT NULL DEFAULT '',
    zip             TEXT NOT NULL DEFAULT '',
    lat             REAL DEFAULT NULL,
    lng             REAL DEFAULT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(name, address)
);

-- Per-store product pricing (derived from receipt data)
CREATE TABLE IF NOT EXISTS store_prices (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    store_id        INTEGER NOT NULL REFERENCES stores(id),
    product_name    TEXT NOT NULL,              -- normalized product name
    brand           TEXT NOT NULL DEFAULT '',
    category        TEXT NOT NULL DEFAULT '',
    price_cents     INTEGER NOT NULL,
    unit_price_cents INTEGER NOT NULL DEFAULT 0,
    unit_label      TEXT NOT NULL DEFAULT '',
    sample_count    INTEGER NOT NULL DEFAULT 1,
    last_seen       TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(store_id, product_name, brand)
);

-- User-store associations (which stores a user frequents)
CREATE TABLE IF NOT EXISTS user_stores (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id),
    store_id        INTEGER NOT NULL REFERENCES stores(id),
    visit_count     INTEGER NOT NULL DEFAULT 1,
    last_visit      TEXT NOT NULL DEFAULT (datetime('now')),
    is_primary      INTEGER NOT NULL DEFAULT 0,
    UNIQUE(user_id, store_id)
);

-- Regional swap performance
CREATE TABLE IF NOT EXISTS regional_swap_performance (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    deal_id         INTEGER NOT NULL REFERENCES brand_deals(id),
    region          TEXT NOT NULL,              -- state or zip prefix (first 3 digits)
    impressions     INTEGER NOT NULL DEFAULT 0,
    accepts         INTEGER NOT NULL DEFAULT 0,
    acceptance_rate REAL NOT NULL DEFAULT 0.0,
    last_computed   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(deal_id, region)
);

-- Link receipts to stores
ALTER TABLE receipts ADD COLUMN store_id INTEGER DEFAULT NULL;

CREATE INDEX IF NOT EXISTS idx_store_prices_store ON store_prices(store_id, product_name);
CREATE INDEX IF NOT EXISTS idx_store_prices_product ON store_prices(product_name, brand);
CREATE INDEX IF NOT EXISTS idx_user_stores_user ON user_stores(user_id);
CREATE INDEX IF NOT EXISTS idx_regional_swap_deal ON regional_swap_performance(deal_id, region);
```

### New API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/stores/nearby` | Returns stores near user's address (from receipt data, not Google Maps) |
| `GET` | `/api/stores/:id/prices` | Returns known prices at a specific store |
| `GET` | `/api/prices/compare` | Compare prices for an item across user's stores |
| `GET` | `/brand/regional-performance/:id` | Regional swap performance for a deal (brand portal) |

### New Frontend Features

1. **Store-Aware Price Comparison** — Swap cards show "At YOUR Kroger: $3.48 vs $4.73" instead of national average prices. Falls back to national average when store-specific data is unavailable.
2. **"Prices at Your Store" on Item Detail** — Tap any list item to see known prices at the user's frequented stores (populated from receipt data over time).
3. **Store Selector on Trip** — When planning a trip, user selects their store. Penny tailors suggestions to that store's known inventory and prices.
4. **"Not Available" Reporting** — If a user couldn't find a swapped product, they can tap "Not available at my store." This feeds store availability data.

### PostHog Events

| Event | Properties | When |
|-------|-----------|------|
| `store_identified` | `store_name`, `chain`, `source` (receipt/manual) | Store matched from receipt or user-reported |
| `store_price_recorded` | `store_id`, `product`, `price_cents` | Price extracted from receipt |
| `price_comparison_viewed` | `item`, `stores_count` | User views price comparison |
| `store_selected_for_trip` | `store_id`, `store_name` | User selects store for shopping trip |
| `item_unavailable_reported` | `store_id`, `product`, `deal_id` | User reports item not available |
| `store_specific_swap_shown` | `deal_id`, `store_id`, `price_source` | Swap shown with store-specific pricing |

### Feedback Loop Mechanism

```
[User submits receipt after shopping trip]
       |
       v
[Receipt pipeline: extract store name, address, line items with prices]
       |
       v
[Store matching: resolve receipt store to stores table (fuzzy match on name+address)]
       |
       v
[Price recording: each line item → store_prices (upsert, moving average)]
       |
       v
[User-store association: update user_stores.visit_count + last_visit]
       |
       v
[Nightly batch: compute regional_swap_performance from swap_events + geography]
       |
       v
[Price enrichment: next swap card for this user uses store-specific prices]
       |
       v
[More accurate prices → higher trust → more receipt submissions → more data]
       |
       v  (loop restarts — this is the strongest flywheel)
```

### Acceptance Criteria

- [ ] Receipt processing populates `stores` and `store_prices` automatically
- [ ] Store matching handles common abbreviations (KRO → Kroger, WM → Walmart)
- [ ] Store-specific prices appear on swap cards when available, with fallback to `price_cache`
- [ ] Price comparison shows prices across user's top 3 stores
- [ ] "Not available" reports are recorded and suppress that product at that store for future swaps
- [ ] Regional swap performance aggregates correctly with min 10 users per region
- [ ] Brand portal shows regional breakdown for each deal
- [ ] All location data respects "Learn from my choices" toggle

### Timeline: L (3-4 sprints)

- Sprint 1: Stores table + receipt-to-store matching + store_prices population
- Sprint 2: Store-aware swap card pricing + user_stores tracking
- Sprint 3: Price comparison UI + store selector on trips
- Sprint 4: Regional performance aggregation + brand portal regional view

**Dependency:** Requires receipt scanning (M7) to be actively used. Without receipt submissions, there is no location data. This level only becomes valuable when >20% of users submit receipts.

---

## LEVEL 5: BUSINESS LEARNING — The Platform Learns What Makes the Marketplace Work

### What to Track

| Signal Category | Specific Data | Source | Already Exists? |
|----------------|--------------|--------|-----------------|
| Unit economics per swap type | Revenue, cost, margin by deal_type | `brand_deals` + `swap_events` + `brand_transactions` | PARTIAL |
| LTV by acquisition channel | User value by referral_source | `users` + `swap_events` + receipts | PARTIAL |
| Viral coefficient | Invites sent / users acquired | `invites` + `referrals` + `household_members` | PARTIAL |
| Marketplace balance | Supply (active deals) vs demand (active users with matching items) | `brand_deals` + `list_items` | NO |
| Brand retention | Do brands re-fund after first deposit exhausted? | `brand_transactions` + `brand_balance` | PARTIAL |
| User engagement cohorts | Retention by signup week, by feature adoption | `users` + activity data | PARTIAL |
| Swap funnel health | Overall offered → accepted → verified → paid | All swap tables | PARTIAL |
| Revenue concentration risk | % of revenue from top brand / top deal | `brand_transactions` | PARTIAL |
| Churn predictors | Signals that precede user/brand churn | Activity patterns + churn events | NO |
| Pricing optimization | What swap price maximizes brand count * acceptance rate | `brand_deals` + `swap_events` | NO |

### New DB Tables and Columns

```
-- Daily business metrics snapshot (one row per day)
CREATE TABLE IF NOT EXISTS business_metrics_daily (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    date            TEXT NOT NULL UNIQUE,         -- YYYY-MM-DD
    -- User metrics
    dau             INTEGER NOT NULL DEFAULT 0,
    wau             INTEGER NOT NULL DEFAULT 0,
    mau             INTEGER NOT NULL DEFAULT 0,
    new_users       INTEGER NOT NULL DEFAULT 0,
    churned_users   INTEGER NOT NULL DEFAULT 0,   -- no activity in 30 days
    -- Swap metrics
    swaps_offered   INTEGER NOT NULL DEFAULT 0,
    swaps_accepted  INTEGER NOT NULL DEFAULT 0,
    swaps_declined  INTEGER NOT NULL DEFAULT 0,
    swaps_verified  INTEGER NOT NULL DEFAULT 0,
    acceptance_rate REAL NOT NULL DEFAULT 0.0,
    -- Revenue metrics
    gross_revenue_cents INTEGER NOT NULL DEFAULT 0,
    brand_spend_cents   INTEGER NOT NULL DEFAULT 0,
    rebates_owed_cents  INTEGER NOT NULL DEFAULT 0,
    -- Supply/demand
    active_deals    INTEGER NOT NULL DEFAULT 0,
    active_users_with_items INTEGER NOT NULL DEFAULT 0,
    match_rate      REAL NOT NULL DEFAULT 0.0,    -- % of items with a matching deal
    -- Viral
    invites_sent    INTEGER NOT NULL DEFAULT 0,
    invites_joined  INTEGER NOT NULL DEFAULT 0,
    referrals_redeemed INTEGER NOT NULL DEFAULT 0,
    -- Brand health
    active_brands   INTEGER NOT NULL DEFAULT 0,
    new_brands      INTEGER NOT NULL DEFAULT 0,
    brands_depleted INTEGER NOT NULL DEFAULT 0,   -- brands that hit $0 balance
    computed_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Cohort analysis table (weekly cohorts)
CREATE TABLE IF NOT EXISTS user_cohorts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id),
    cohort_week     TEXT NOT NULL,            -- YYYY-WXX
    signup_source   TEXT NOT NULL DEFAULT '',  -- 'organic', 'referral', 'brand_invite'
    first_swap_at   TEXT DEFAULT NULL,
    first_receipt_at TEXT DEFAULT NULL,
    first_share_at  TEXT DEFAULT NULL,
    total_swaps     INTEGER NOT NULL DEFAULT 0,
    total_savings_cents INTEGER NOT NULL DEFAULT 0,
    last_active     TEXT DEFAULT NULL,
    is_churned      INTEGER NOT NULL DEFAULT 0,
    UNIQUE(user_id)
);

-- Churn prediction signals (computed weekly)
CREATE TABLE IF NOT EXISTS churn_signals (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id),
    signal_type     TEXT NOT NULL,            -- 'days_inactive', 'decline_streak', 'swap_fatigue', 'list_abandonment'
    signal_value    REAL NOT NULL,
    risk_score      REAL NOT NULL DEFAULT 0.0, -- 0.0 to 1.0
    computed_at     TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(user_id, signal_type)
);

-- Marketplace health metrics (computed daily)
CREATE TABLE IF NOT EXISTS marketplace_health (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    date            TEXT NOT NULL UNIQUE,
    supply_score    REAL NOT NULL DEFAULT 0.0,  -- 0-100: deal coverage across categories
    demand_score    REAL NOT NULL DEFAULT 0.0,  -- 0-100: user engagement with swaps
    balance_score   REAL NOT NULL DEFAULT 0.0,  -- 0-100: supply/demand equilibrium
    revenue_concentration REAL NOT NULL DEFAULT 0.0, -- HHI: 0=diverse, 1=single brand
    health_index    REAL NOT NULL DEFAULT 0.0,  -- composite 0-100
    computed_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_business_metrics_date ON business_metrics_daily(date);
CREATE INDEX IF NOT EXISTS idx_user_cohorts_week ON user_cohorts(cohort_week);
CREATE INDEX IF NOT EXISTS idx_churn_signals_user ON churn_signals(user_id, signal_type);
```

### New API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/admin/business-metrics` | Daily business metrics with date range filter |
| `GET` | `/api/admin/cohort-analysis` | Cohort retention and engagement data |
| `GET` | `/api/admin/churn-risk` | Users at risk of churning with signal breakdown |
| `GET` | `/api/admin/marketplace-health` | Marketplace health index over time |
| `GET` | `/api/admin/revenue-forecast` | Simple revenue projection based on current trends |

### New Frontend Features (Admin Dashboard)

1. **Business Metrics Dashboard** — Time-series charts for DAU/WAU/MAU, swap funnel (offered -> accepted -> verified), revenue, and brand health. Date range selector. This replaces the basic admin analytics with a proper business intelligence view.
2. **Cohort Retention Matrix** — Classic cohort table: rows are signup weeks, columns are weeks since signup, cells are retention %. Color-coded (green=good, red=bad). Shows which cohorts retain best and why.
3. **Churn Risk List** — Sortable table of users with churn risk score >0.5. Shows risk signals (days inactive, decline streak, etc.). Admin can tap to view user profile and take action (send re-engagement SMS, offer deal).
4. **Marketplace Health Gauge** — Single composite score 0-100 with component breakdown (supply, demand, balance, concentration). Alerts when any component drops below threshold.
5. **Revenue Forecast** — Simple linear projection: "At current rates, you'll hit $X/month by [date]." Shows optimistic/pessimistic bands.

### PostHog Events

| Event | Properties | When |
|-------|-----------|------|
| `admin_metrics_viewed` | `date_range`, `metrics_type` | Admin views business metrics |
| `admin_cohort_viewed` | `cohort_count` | Admin views cohort analysis |
| `admin_churn_action` | `user_id`, `action_type` | Admin takes action on churn-risk user |
| `marketplace_health_alert` | `component`, `score`, `threshold` | Health score drops below threshold |

### Feedback Loop Mechanism

```
[All Level 1-4 data accumulates daily]
       |
       v
[Nightly batch: compute business_metrics_daily from raw tables]
  - Aggregate DAU/WAU/MAU from sessions
  - Aggregate swap funnel from swap_events + swap_claims
  - Aggregate revenue from brand_transactions
  - Compute marketplace balance (active deals vs active users)
       |
       v
[Weekly batch: compute user_cohorts + churn_signals + marketplace_health]
  - Cohort retention curves
  - Churn risk scoring (logistic regression on inactivity signals)
  - Marketplace health composite score
       |
       v
[Admin dashboard surfaces insights]
       |
       v
[Business decisions: pricing changes, feature priorities, brand outreach]
  - "Acceptance rate dropping for organic segment → prioritize dietary matching"
  - "Top brand is 60% of revenue → diversify, do brand outreach"
  - "Cohort 12 has 2x retention → investigate what's different (feature release?)"
  - "5 users at high churn risk → send Penny re-engagement message"
       |
       v
[Decisions improve product → better metrics → better decisions]
       |
       v  (loop restarts — slowest loop, but highest leverage)
```

### Acceptance Criteria

- [ ] `business_metrics_daily` is populated every night by batch job
- [ ] Admin dashboard shows time-series charts for all key metrics
- [ ] Cohort analysis correctly buckets users by signup week
- [ ] Retention matrix renders with correct percentages and color coding
- [ ] Churn risk score correctly identifies users with >7 days inactivity + declining engagement
- [ ] Marketplace health index components are individually visible
- [ ] Revenue forecast uses trailing 30-day trend for projection
- [ ] All admin endpoints require admin auth
- [ ] No PII is exposed in aggregate metrics (min 5 users per cohort cell)
- [ ] Batch jobs run against reporting DB, not production (existing pattern)

### Timeline: L (3-4 sprints)

- Sprint 1: business_metrics_daily batch job + admin API + basic dashboard charts
- Sprint 2: Cohort analysis computation + retention matrix UI
- Sprint 3: Churn signals + risk scoring + admin churn risk list
- Sprint 4: Marketplace health index + revenue forecast + alerting

---

## CROSS-CUTTING CONCERNS

### Batch Job Architecture

All learning computation runs as nightly batch jobs against the reporting DB (the VACUUM INTO clone, per the existing C2 infrastructure from Sprint 10).

```
Batch Job Schedule:
  - 02:00 UTC: VACUUM INTO → create fresh reporting clone
  - 02:15 UTC: Level 1 — user_shopping_patterns + penny_context_cache
  - 02:30 UTC: Level 2 — household_patterns + household_dietary_union
  - 02:45 UTC: Level 3 — deal_performance_daily + deal_segment_performance + category_swap_insights
  - 03:00 UTC: Level 4 — store_prices aggregation + regional_swap_performance
  - 03:15 UTC: Level 5 — business_metrics_daily + user_cohorts + churn_signals + marketplace_health
  - 03:30 UTC: Copy computed tables back to production DB
```

**Architect note:** The batch jobs write to the reporting DB, then a controlled sync copies only the materialized/computed tables back to production. Raw production tables are never written to by batch jobs. This keeps the write path clean and the batch job failure-safe (if a batch fails, production still has yesterday's computed data).

### Privacy Architecture

```
+--------------------+----------------------------+-------------------------+
| Data Type          | Retention                  | Deletion on "Reset"     |
+--------------------+----------------------------+-------------------------+
| Raw swap events    | 7 years (regulatory)       | Anonymized, not deleted |
| User preferences   | While account active       | Hard delete             |
| Shopping patterns  | While account active       | Hard delete             |
| Penny context      | Recomputed daily           | Nullified immediately   |
| Receipt data       | 7 years (evidence)         | Anonymized, not deleted |
| Store prices       | Aggregated (no PII)        | N/A                     |
| Business metrics   | Aggregated (no PII)        | N/A                     |
| Household patterns | While in household         | Recomputed on leave     |
+--------------------+----------------------------+-------------------------+
```

**Hard rule:** When a user toggles "Learn from my choices" OFF:
1. Immediately nullify `penny_context_cache` for that user
2. Stop writing to `user_shopping_patterns` for that user
3. Do NOT delete historical `swap_events` (needed for brand reporting integrity)
4. Do NOT delete `user_dietary_profiles` (these are explicitly user-set, not learned)
5. Do continue swap suppression (3 declines = permanent suppress — this is a safety feature, not a learning feature)

### "Learn from my choices" Toggle — Scope Clarification

| Feature | Toggle ON | Toggle OFF |
|---------|-----------|------------|
| Brand preference learning | Active | Paused (existing data preserved but not used) |
| Shopping pattern detection | Active | Paused |
| Penny context enrichment | Active | Disabled (generic prompts only) |
| Swap suggestions | Personalized | Generic (still shown, just not personalized) |
| Dietary hard-filters | Always active | Always active (safety, not learning) |
| Swap suppression (3 declines) | Always active | Always active (safety, not learning) |
| Household pattern contribution | Active | Excluded from household aggregation |
| Receipt price recording | Active | Active (aggregated, no PII) |

### Notification Safety (per CLAUDE.md Hard Rule)

Learning-driven notifications (repurchase nudges, churn re-engagement, dietary conflict alerts) are subject to the same duplicate prevention and rate limiting as all other notifications:

1. **Repurchase nudges:** Max 1 per item per repurchase cycle. Dedup key: `user_id + item_name + cycle_start_date`. Stored in `user_shopping_patterns` as `last_nudged_at`.
2. **Dietary conflict alerts:** Max 1 per allergen per household per 24 hours. Dedup key: `household_id + allergen + date`.
3. **Churn re-engagement:** Admin-initiated only. No automated sends. Rate limit: 1 per user per 7 days.
4. **"Penny prediction" nudges:** Max 2 per session. Session-scoped with sessionStorage persistence on dismiss.

---

## PRIORITIZED IMPLEMENTATION ROADMAP

### Phase A: Foundation (Sprints 11-12) — Individual + Brand Learning MVP

| # | Item | Level | Size | Priority | Dependencies |
|---|------|-------|------|----------|-------------|
| L1 | Dietary profile table + API + allergen hard-filter | 1 | S | P0 | None |
| L2 | Structured decline reasons on swap cards | 1+3 | S | P0 | None |
| L3 | Swap timing columns (shown_at, acted_at, why_tapped) | 3 | XS | P0 | None |
| L4 | deal_performance_daily batch job | 3 | M | P0 | L3 |
| L5 | Dietary profile UI in Profile screen | 1 | S | P1 | L1 |
| L6 | Brand deal performance deep dive (brand portal) | 3 | M | P1 | L4 |

**Phase A outcome:** Swaps are allergen-safe, decline reasons are captured, brand performance is measurable daily, brands can see what's working.

### Phase B: Personalization (Sprints 13-14) — Individual Learning Full + Household

| # | Item | Level | Size | Priority | Dependencies |
|---|------|-------|------|----------|-------------|
| L7 | user_shopping_patterns batch computation | 1 | M | P0 | Phase A |
| L8 | penny_context_cache + prompt enrichment | 1 | M | P0 | L7 |
| L9 | "Penny Knows" insights card | 1 | S | P1 | L8 |
| L10 | Preference reset ("forget me") | 1 | S | P1 | L7, L8 |
| L11 | Household dietary union computation | 2 | S | P1 | L1 |
| L12 | Duplicate purchase prevention | 2 | S | P2 | None |
| L13 | deal_segment_performance computation | 3 | M | P1 | L7, L4 |
| L14 | "Optimize This Deal" brand recommendations | 3 | M | P2 | L13 |

**Phase B outcome:** Penny's suggestions are personalized per user. Households get dietary safety. Brands get segment analysis.

### Phase C: Location + Business Intelligence (Sprints 15-17) — Full Learning Stack

| # | Item | Level | Size | Priority | Dependencies |
|---|------|-------|------|----------|-------------|
| L15 | stores table + receipt-to-store matching | 4 | M | P1 | M7 (receipt reading) |
| L16 | store_prices population from receipts | 4 | M | P1 | L15 |
| L17 | Store-aware swap card pricing | 4 | M | P0 | L16 |
| L18 | business_metrics_daily batch + admin dashboard | 5 | L | P1 | Phase A, B |
| L19 | Cohort analysis + retention matrix | 5 | M | P2 | L18 |
| L20 | Household repurchase nudges | 2 | S | P2 | L7, L11 |
| L21 | Churn risk scoring + admin list | 5 | M | P2 | L18 |
| L22 | Marketplace health index + alerting | 5 | M | P3 | L18 |
| L23 | Regional swap performance (brand portal) | 4 | S | P2 | L15 |

**Phase C outcome:** Prices are store-specific (highest trust lever), business has full intelligence, churn is predictable.

### Total Estimated Timeline

| Phase | Sprints | Calendar Estimate | Key Outcome |
|-------|---------|------------------|-------------|
| A: Foundation | 2 | 2-3 weeks | Allergen safety + brand analytics |
| B: Personalization | 2 | 2-3 weeks | Penny gets smart + segment targeting |
| C: Full Stack | 3 | 3-4 weeks | Store-aware pricing + business intelligence |
| **Total** | **7** | **7-10 weeks** | **Full 5-level learning system** |

---

## METRICS TO TRACK SUCCESS

### Leading Indicators (measure weekly)

| Metric | Baseline (now) | Phase A Target | Phase C Target |
|--------|---------------|---------------|---------------|
| Swap acceptance rate | ~8% | 12% | 18% |
| Allergen-conflicting swaps shown | Unknown (risk) | 0 | 0 |
| Decline reasons captured (% of declines) | 0% | 40% | 60% |
| Users with dietary profile set | 0% | 15% | 40% |
| Brand portal daily active brands | ~3 | 5 | 15 |
| Store-specific prices on swap cards | 0% | 0% | 50% |
| Penny prediction accuracy (suggestion accepted) | Unknown | Baseline | +30% vs baseline |

### Lagging Indicators (measure monthly)

| Metric | Baseline | 3-Month Target | 6-Month Target |
|--------|----------|---------------|---------------|
| Revenue per user per month | ~$0.50 | $1.20 | $2.50 |
| 30-day retention | ~35% | 45% | 55% |
| Brand deal average ROI | Unknown | 3x | 5x |
| Receipt submission rate | ~5% | 15% | 30% |
| Household coordination score | N/A | Baseline | +25% |

---

## OPEN QUESTIONS FOR FOUNDER

1. **Dietary profile source of truth:** Should Penny's conversational understanding ("I'm vegan") override the manual profile UI, or should it prompt the user to confirm? (UX Designer recommends: conversational sets it, UI confirms with a toast.)

2. **Learning data portability:** If a user requests a data export (CCPA), should we include raw learning data (shopping patterns, preference scores) in the export, or just the source data (list items, swap events)?

3. **Brand access to segment data:** How granular should brand segment analysis be? Currently proposed: price_tier (budget/mid/premium), dietary (organic/conventional), frequency (weekly/biweekly/monthly). Should we add demographics if we collect them?

4. **Churn re-engagement automation:** Currently proposed as admin-initiated only. Should we allow Penny to send a "We miss you!" SMS automatically for users inactive >14 days? (Security/Privacy recommends NO — too high risk of notification fatigue. Growth Lead recommends YES with strict frequency cap.)

5. **Store-specific pricing accuracy bar:** At what confidence level should we show store-specific prices? Proposed: show store price if based on >3 receipt data points within 30 days. Otherwise fall back to national average with "estimated" label. Is this threshold right?

6. **Learning across accounts on merge:** When two accounts merge (resolveOrCreateUser), should learning data be merged too? Proposed: keep the data from the account with more history, discard the other. This is simpler than merge logic.

---

*This document was produced by a full product team convene. All 7 roles contributed. The PM synthesized all inputs into the prioritized roadmap above. Next step: founder review, then implementation begins in Sprint 11.*
