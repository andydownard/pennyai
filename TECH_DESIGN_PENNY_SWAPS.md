# Penny Swaps MVP — Technical Design Document

> **Status:** Draft v1 — March 2026
> **Author:** Product Team (PM, UX, Architect, Staff Dev)
> **Based on:** 3 focus group panels (18 participants total): 6 grocery shoppers, 6 CPG brand PMs, 6 receipt/fraud/compliance specialists

---

## 1. Executive Summary

Penny Swaps is the revenue engine for Thriftly. When a shopper's grocery list contains an item that matches an active brand campaign, Penny suggests an alternative product conversationally. The brand pays per accepted swap. Receipt verification closes the attribution loop.

**Four pillars of the swap lifecycle:**

```
┌─────────────┐    ┌──────────────┐    ┌───────────────┐    ┌──────────────┐
│  1. OFFER   │───▶│  2. VERIFY   │───▶│  3. CHARGE    │───▶│  4. REPORT   │
│  to shopper │    │  with receipt │    │  the brand    │    │  to brand PM │
└─────────────┘    └──────────────┘    └───────────────┘    └──────────────┘
  Inline swap        Photo/email        Per-acceptance       Dashboard +
  suggestion in      receipt scan       billing with         CSV export +
  shopping list      with OCR +         budget caps          competitive
                     item matching      and fraud            intel
                                        protection
```

---

## 2. System Architecture

```
                          PENNY SWAPS SYSTEM ARCHITECTURE
                          ================================

    ┌────────────────────────────────────────────────────────────────┐
    │                        SHOPPER APP                             │
    │  ┌──────────┐  ┌──────────┐  ┌───────────┐  ┌─────────────┐ │
    │  │ Shopping  │  │ Swap     │  │ Receipt   │  │ Savings     │ │
    │  │ List      │  │ Cards    │  │ Scanner   │  │ Dashboard   │ │
    │  └──────────┘  └──────────┘  └───────────┘  └─────────────┘ │
    └──────────────────────────┬─────────────────────────────────────┘
                               │
                    ┌──────────▼──────────┐
                    │   THRIFTLY API      │
                    │   (Go backend)      │
                    ├─────────────────────┤
                    │ /api/list           │ ← existing
                    │ /api/list/parse     │ ← existing (add swap matching)
                    │ /api/swaps/match    │ ← NEW: check for matching deals
                    │ /api/receipt        │ ← NEW: submit receipt image
                    │ /api/receipt/status │ ← NEW: check processing status
                    │ /api/savings        │ ← NEW: user savings summary
                    └──────────┬──────────┘
                               │
              ┌────────────────┼────────────────┐
              │                │                │
    ┌─────────▼────┐  ┌───────▼───────┐  ┌────▼──────────┐
    │   SQLite DB  │  │ Receipt Queue │  │ Image Storage │
    │  (existing)  │  │  (async)      │  │  (S3/Railway  │
    │  + new tables│  │               │  │   volume)     │
    └──────────────┘  └───────┬───────┘  └───────────────┘
                              │
                    ┌─────────▼─────────┐
                    │ PROCESSING PIPELINE│
                    │                    │
                    │ 1. Quality check   │
                    │ 2. Dedup (pHash)   │
                    │ 3. OCR extraction  │
                    │    (Textract)      │
                    │ 4. Entity resolve  │
                    │    (Claude)        │
                    │ 5. Item matching   │
                    │ 6. Fraud scoring   │
                    │ 7. Claim creation  │
                    └────────────────────┘

    ┌────────────────────────────────────────────────────────────────┐
    │                     BRAND PORTAL (Phase 2)                     │
    │  ┌──────────┐  ┌──────────┐  ┌───────────┐  ┌─────────────┐ │
    │  │ Campaign  │  │ Budget   │  │ Dashboard │  │ Competitive │ │
    │  │ Setup     │  │ Manager  │  │ & Reports │  │ Intel       │ │
    │  └──────────┘  └──────────┘  └───────────┘  └─────────────┘ │
    └────────────────────────────────────────────────────────────────┘
```

---

## 3. Pillar 1: Offer to Shopper

### 3.1 How Swap Matching Works

When a user's list is loaded or items are parsed, the backend checks for matching active campaigns:

```
User adds "Heinz Ketchup" to list
        │
        ▼
┌─────────────────────┐
│ Swap Matcher         │
│                      │
│ 1. Normalize item    │──▶ "heinz ketchup" → brand:"Heinz", category:"Ketchup"
│ 2. Find campaigns    │──▶ SELECT from brand_deals WHERE competitor matches
│ 3. Check eligibility │──▶ User not at frequency cap? Budget remaining?
│ 4. Check suppressions│──▶ User hasn't declined this swap 3x?
│ 5. Rank by relevance │──▶ User profile axis: price? ingredients? convenience?
│ 6. Return top match  │──▶ { deal, product, discount, why }
└─────────────────────┘
```

### 3.2 UX Requirements (from Shopper Focus Group)

| Requirement | Detail | Source |
|---|---|---|
| **Inline placement** | Swap card appears below the matched item in the list, not as notification or modal | 6/6 consensus |
| **Personalized "why"** | Lead with user's primary value axis: price for budget shoppers, ingredients for health-conscious | Marcus, Priya, Sarah |
| **Unit price normalization** | Always show price-per-oz or price-per-unit, not just raw savings | Linda, James, Devon |
| **3-strike rule** | After 3 declines of a specific swap, permanently suppress for that user | 6/6 unanimous |
| **Planning mode only** | No swap suggestions after user starts a shopping trip | James, Sarah, Linda |
| **Frequency cap** | Default 2-3 per session, user-configurable, unlimited mode available | Compromise position |
| **Pre-trip preview** | "Penny's Picks" — consolidated swap review before shopping begins | James (strong signal) |
| **Allergen hard filter** | Swap suggestions MUST NOT conflict with user's dietary restrictions | Priya (safety issue) |
| **Sponsored label** | Every paid swap carries a visible "Brand Partner" disclosure | Sarah, Marcus, Priya |
| **Consistent tone** | Penny's enthusiasm level identical for sponsored and organic suggestions | James, Devon |
| **Decline feedback** | User can optionally say WHY they declined (trains Penny) | Devon |

### 3.3 Swap Card Design

```
┌────────────────────────────────────────────────┐
│  [Logo]  Sir Kensington's Classic Ketchup      │
│          20oz · $4.49 ($0.22/oz)               │
│          Save $1.50 vs Heinz ($0.30/oz)        │
│                                    Brand Partner│
│  ┌──────────┐  ┌──────────┐  ┌────┐           │
│  │   Swap   │  │ No thanks│  │Why?│           │
│  └──────────┘  └──────────┘  └────┘           │
└────────────────────────────────────────────────┘
```

- "Swap" = primary action (filled button, brand color)
- "No thanks" = secondary (ghost button)
- "Why?" opens expandable panel with real data (ingredients comparison, ratings, etc.)
- "Brand Partner" = small text label, always visible on paid swaps

### 3.4 Database Tables

```sql
-- Active brand campaigns
CREATE TABLE brand_deals (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    brand_id            INTEGER NOT NULL REFERENCES brands(id),
    campaign_name       TEXT NOT NULL,
    competitor_brand    TEXT,               -- NULL = category-wide targeting
    competitor_product  TEXT,               -- specific product to match against
    offer_product       TEXT NOT NULL,      -- the product being suggested
    offer_product_id    INTEGER REFERENCES products(id),
    discount_cents      INTEGER NOT NULL,   -- savings for the shopper
    charge_cents        INTEGER NOT NULL,   -- what the brand pays per swap
    budget_total_cents  INTEGER NOT NULL,   -- total campaign budget
    budget_remaining    INTEGER NOT NULL,   -- decremented on each accepted swap
    daily_cap_cents     INTEGER,            -- optional daily budget cap
    daily_spent_cents   INTEGER DEFAULT 0,  -- reset at midnight UTC
    max_swaps_per_user  INTEGER DEFAULT 3,  -- per-campaign per-user limit
    cooldown_days       INTEGER DEFAULT 42, -- minimum days between repeat swaps for same user
    status              TEXT NOT NULL DEFAULT 'active',
        -- 'draft', 'active', 'paused', 'exhausted', 'completed', 'cancelled'
    copy_template       TEXT,               -- brand-approved Penny copy (optional)
    starts_at           DATETIME NOT NULL,
    ends_at             DATETIME NOT NULL,
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Every swap interaction
CREATE TABLE swap_events (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id             INTEGER NOT NULL REFERENCES users(id),
    household_id        INTEGER REFERENCES households(id),
    deal_id             INTEGER NOT NULL REFERENCES brand_deals(id),
    list_item_id        INTEGER REFERENCES list_items(id),
    action              TEXT NOT NULL,
        -- 'offered', 'accepted', 'declined', 'dismissed', 'expired'
    decline_reason      TEXT,              -- user-provided: 'tried_before', 'dietary', 'brand_loyal', etc.
    original_item       TEXT NOT NULL,     -- what was on their list
    swap_item           TEXT NOT NULL,     -- what was suggested
    discount_cents      INTEGER NOT NULL,
    charge_cents        INTEGER NOT NULL,  -- what brand was charged (0 if not accepted)
    receipt_verified    INTEGER DEFAULT 0, -- set to 1 when receipt confirms purchase
    receipt_id          TEXT,              -- FK to receipts table
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Track suppressions (3-strike rule)
CREATE TABLE swap_suppressions (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id             INTEGER NOT NULL REFERENCES users(id),
    deal_id             INTEGER NOT NULL REFERENCES brand_deals(id),
    decline_count       INTEGER NOT NULL DEFAULT 1,
    suppressed          INTEGER NOT NULL DEFAULT 0, -- 1 = permanently suppressed
    last_declined_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, deal_id)
);
```

### 3.5 API Endpoints

```
GET  /api/swaps/match
  Query: (uses session user + their current list)
  Response: [{ deal_id, offer_product, discount_cents, why_text, brand_logo_url, unit_price_comparison }]
  Notes: Called when list loads and after items are added. Returns max N matches
         based on user's frequency preference. Excludes suppressed deals.

POST /api/swaps/respond
  Body: { deal_id, action: "accepted"|"declined"|"dismissed", decline_reason? }
  Response: { success: true, savings_cents?, list_item_updated? }
  Notes: On accept: replaces item on list, decrements budget, logs swap_event.
         On decline: increments suppression counter. 3rd decline = permanent suppress.
```

---

## 4. Pillar 2: Verify with Receipt

### 4.1 Receipt Processing Pipeline

```
                    RECEIPT VERIFICATION PIPELINE
                    =============================

CAPTURE                          INGEST                     PROCESS
───────                          ──────                     ───────
[In-app guided   ]              [Upload API ]              [Quality check    ]
[  capture       ]──┐           [           ]              [  (blur/dark)    ]
[  (quality gate)]  │           [1. UUID     ]              │
                    ├──────────▶[2. Timestamp]──────────────▶[Perceptual hash ]
[Camera roll     ]  │           [3. SHA-256  ]              [  (dedup check) ]
[  upload        ]──┘           [4. Store S3 ]              │
                                [5. Audit log]              [Date check      ]
[Email receipt   ]──────────────[6. Return ID]              [  (EXIF vs print)]
[  forward       ]                                          │
                                                            ▼
EXTRACT                          MATCH                      DECIDE
───────                          ─────                      ──────
[AWS Textract    ]              [Lookup DB   ]             [confidence > 0.85]
[  Expense Model ]──────────────▶[  UPC match ]────────────▶[  AUTO VERIFY   ]
[                ]              [  Brand fuzzy]             │
[Returns:        ]              [  Abbrev DB  ]             [0.65 < c < 0.85 ]
[  store, date   ]              │                          [  USER CONFIRM   ]
[  line items    ]              [Claude LLM   ]             │
[  totals        ]              [  (long tail)]             [confidence < 0.65]
[  UPCs          ]              [  Flag as    ]             [  UNMATCHED      ]
                                [  llm_resolved]            │
                                                            ▼
                                                   [Match to campaigns ]
                                                   [Create swap_claim  ]
                                                   [30-day payout hold ]
                                                   [Emit analytics event]
```

### 4.2 Accuracy Targets

| Stage | Target | Minimum | Notes |
|---|---|---|---|
| Store name | 97% | 93% | Critical for store-specific campaigns |
| Transaction date | 99% | 97% | Critical for fraud prevention |
| Total amount | 99% | 97% | Critical for dispute resolution |
| Line items extracted | 90% | 85% | Acceptable to miss some; must not false-positive |
| Item-to-product match (lookup) | 85% | 80% | Deterministic, auditable |
| Item-to-product match (LLM fallback) | 75% | 65% | For unmatched long-tail |
| End-to-end auto-verify rate | 70%+ | 60% | Without user confirmation |
| Fraud false positive rate | <2% | <4% | Legitimate receipts wrongly flagged |

### 4.3 Receipt Database Tables

```sql
CREATE TABLE receipts (
    id                  TEXT PRIMARY KEY,            -- UUID
    user_id             INTEGER NOT NULL REFERENCES users(id),
    household_id        INTEGER REFERENCES households(id),
    submitted_at        DATETIME NOT NULL,           -- server timestamp
    capture_method      TEXT NOT NULL,               -- 'in_app_live', 'camera_roll', 'email_parsed'
    device_type         TEXT,
    image_storage_key   TEXT,                        -- S3/volume path
    image_sha256        TEXT NOT NULL,
    perceptual_hash     TEXT,                        -- for dedup
    raw_ocr_text        TEXT,
    store_name          TEXT,
    store_chain         TEXT,                        -- normalized: "Walmart"
    transaction_date    DATE,
    total_cents         INTEGER,
    math_valid          INTEGER,                     -- 1 = line items sum to total
    fraud_score         REAL,                        -- 0.0-1.0 composite
    processing_status   TEXT NOT NULL DEFAULT 'pending',
        -- 'pending', 'processing', 'complete', 'rejected_fraud', 'rejected_quality'
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE receipt_line_items (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    receipt_id          TEXT NOT NULL REFERENCES receipts(id),
    line_number         INTEGER,
    raw_text            TEXT NOT NULL,
    quantity            REAL,
    unit_price_cents    INTEGER,
    line_total_cents    INTEGER,
    upc_raw             TEXT,
    resolved_brand      TEXT,
    resolved_product    TEXT,
    resolution_method   TEXT,     -- 'upc', 'lookup', 'llm', 'user_confirmed', 'unmatched'
    resolution_confidence REAL,
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Receipt hash registry for dedup
CREATE TABLE receipt_hashes (
    perceptual_hash     TEXT PRIMARY KEY,
    first_receipt_id    TEXT NOT NULL,
    first_user_id       INTEGER NOT NULL,
    submitted_at        DATETIME NOT NULL,
    collision_count     INTEGER DEFAULT 0
);

-- Swap claims tied to receipt verification
CREATE TABLE swap_claims (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    swap_event_id       INTEGER NOT NULL REFERENCES swap_events(id),
    receipt_id          TEXT NOT NULL REFERENCES receipts(id),
    receipt_line_item_id INTEGER REFERENCES receipt_line_items(id),
    status              TEXT NOT NULL DEFAULT 'pending',
        -- 'pending', 'verified', 'rejected', 'paid_out', 'reversed'
    match_confidence    REAL,
    fraud_hold          INTEGER DEFAULT 0,
    verified_at         DATETIME,
    eligible_payout_at  DATETIME,           -- verified_at + 30 days
    paid_out_at         DATETIME,
    reward_cents        INTEGER,
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Immutable audit log for all claim transitions
CREATE TABLE claim_audit_log (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    claim_id            INTEGER REFERENCES swap_claims(id),
    receipt_id          TEXT REFERENCES receipts(id),
    event_type          TEXT NOT NULL,
        -- 'submitted', 'extracted', 'matched', 'verified', 'rejected',
        -- 'fraud_flagged', 'payout_scheduled', 'paid_out', 'reversed', 'disputed'
    actor               TEXT NOT NULL,      -- 'system', 'user', 'admin'
    metadata            TEXT,               -- JSON details
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Retailer abbreviation dictionary (grows from user confirmations)
CREATE TABLE retailer_abbreviations (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    store_chain         TEXT,               -- NULL = cross-retailer
    raw_text            TEXT NOT NULL,       -- "SKEN KETCH 20OZ"
    resolved_brand      TEXT NOT NULL,       -- "Sir Kensington's"
    resolved_product    TEXT,                -- "Classic Ketchup"
    confidence          REAL DEFAULT 1.0,
    source              TEXT,                -- 'curated', 'user_confirmed', 'llm'
    confirmation_count  INTEGER DEFAULT 1,
    UNIQUE(store_chain, raw_text)
);
```

### 4.4 API Endpoints

```
POST /api/receipt
  Body: multipart form data with image file
  Response: { receipt_id, status: "processing" }
  Notes: Stores image, computes hash, checks dedup, queues for processing.
         Returns immediately — processing is async.

GET  /api/receipt/{id}/status
  Response: { status, items_found?, matches?, savings_cents? }
  Notes: Polled by frontend. When complete, shows what was found.

POST /api/receipt/{id}/confirm
  Body: { line_item_id, confirmed_product: "Sir Kensington's Ketchup" }
  Notes: User confirms a low-confidence match. Feeds back into abbreviation DB.
```

### 4.5 Evidence Retention Policy

| Tier | Duration | What | Where |
|---|---|---|---|
| Active | 0-24 months | Original image, OCR text, all audit logs | Primary storage (S3/volume) |
| Cold | 24-84 months | Same, compressed | Archival storage |
| **Total** | **7 years** | Full evidence chain | Per compliance requirements |

- **Right to delete:** Honored except for fraud-flagged accounts
- **Chain of custody:** SHA-256 hash computed at upload, stored in append-only audit log
- **Brand access:** NEVER see receipt images — only aggregated verification results
- **Thriftly = trusted intermediary** between shopper PII and brand reporting

### 4.6 Fraud Prevention (Priority Order)

**Tier 1 — Prevent (at upload, near-zero cost):**
1. Perceptual hash deduplication (block reused receipts)
2. Rate limiting: 3 receipts/day, 10/week per user
3. Date consistency: receipt date within promotion window, EXIF plausible
4. New account gate: accounts <24h old → manual review

**Tier 2 — Detect (async processing):**
5. Math validation: line items must sum to total
6. Error Level Analysis: detect image manipulation
7. Price plausibility: prices in expected range for product

**Tier 3 — Investigate (batch/flagged):**
8. Device fingerprint clustering (multi-account detection)
9. Cross-user receipt dedup (same receipt, different users)
10. Buy-return defense: 30-day payout hold on all claims

---

## 5. Pillar 3: Charge the Brand

### 5.1 Billing Model

| Parameter | Value | Source |
|---|---|---|
| **Billing trigger** | Per accepted swap only (NOT per offer/impression) | 6/6 brand PM consensus |
| **Standard rate** | $1.50 per accepted swap | Roadmap |
| **Founding Brand rate** | $1.00 per accepted swap, **18-month contractual lock** | Rachel's requirement |
| **Verified purchase premium** | +$1.00 per receipt-verified purchase (total $2.50) | Tom willing to pay |
| **Payment: small accounts** | Credit card, pre-funded deposit | Carlos |
| **Payment: enterprise** | Net-30 invoicing for accounts >$10K/month | Anika (non-negotiable) |

### 5.2 Budget Controls

```
Campaign Budget Architecture:

                    ┌──────────────────────┐
                    │   TOTAL CAMPAIGN     │
                    │   BUDGET ($10,000)   │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │   DAILY CAP ($500)   │ ← Hard stop, not soft limit
                    │   Reset: midnight UTC│
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │   WEEKLY CAP ($2000) │ ← Optional
                    │   Reset: Monday UTC  │
                    └──────────────────────┘

On ANY cap hit:
  1. Campaign pauses IMMEDIATELY (mid-request if necessary)
  2. Email alert sent to brand PM within 5 minutes
  3. Campaign resumes ONLY on explicit brand action (no auto-resume)
  4. "Cap hit" event logged in campaign audit trail
```

### 5.3 Billing Edge Cases

| Scenario | Behavior | Rationale |
|---|---|---|
| Swap accepted but receipt shows competitor bought | Brand NOT charged for verified tier | Rachel's requirement — charge only on verified purchase |
| Budget exhausted mid-day | Campaign pauses, no degraded UX (no "offer expired" cards) | Megan's requirement |
| User accepts swap then returns product | 30-day hold covers typical return windows | Anthony's recommendation |
| Fraud detected after billing | Credit issued within one billing cycle | Carlos's requirement |
| Dispute on swap count | Self-service reconciliation tool: brand sees row-level log | Carlos, Tom |

### 5.4 Database Tables

```sql
-- Brand billing accounts
CREATE TABLE brand_accounts (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id             INTEGER NOT NULL REFERENCES users(id), -- brand PM's user account
    company_name        TEXT NOT NULL,
    billing_type        TEXT NOT NULL DEFAULT 'prepaid',  -- 'prepaid', 'invoiced'
    balance_cents       INTEGER DEFAULT 0,               -- prepaid balance
    stripe_customer_id  TEXT,
    founding_brand      INTEGER DEFAULT 0,               -- 1 = $1.00 rate, locked 18 months
    founding_rate_expires DATETIME,
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Transaction ledger (append-only)
CREATE TABLE brand_transactions (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    brand_account_id    INTEGER NOT NULL REFERENCES brand_accounts(id),
    type                TEXT NOT NULL,
        -- 'deposit', 'swap_charge', 'verified_charge', 'refund', 'credit', 'payout'
    amount_cents        INTEGER NOT NULL,  -- positive = credit, negative = charge
    deal_id             INTEGER REFERENCES brand_deals(id),
    swap_event_id       INTEGER REFERENCES swap_events(id),
    description         TEXT,
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## 6. Pillar 4: Report to Brand PM

### 6.1 Dashboard Metrics (Priority Order)

**MVP Dashboard (required at launch):**

| # | Metric | Definition | Refresh |
|---|---|---|---|
| 1 | Total spend | Cumulative charges, current campaign | Real-time |
| 2 | Swaps accepted | Count of accepted swaps | Daily |
| 3 | Acceptance rate | Accepted / offered | Daily |
| 4 | Cost per swap | Total spend / accepted swaps | Daily |
| 5 | Budget remaining | Total + daily caps + burn rate | Real-time |
| 6 | Verified purchases | Receipt-confirmed count (when available) | Daily |
| 7 | Cost per verified purchase | Spend / verified count | Daily |
| 8 | Fraud rate | Platform-published aggregate fraud flag % | Weekly |

**Phase 2 Dashboard (enterprise retention):**

| # | Metric | Definition | Why |
|---|---|---|---|
| 1 | First-time trial rate | % of converters with no prior purchase history | "Biggest budget-increase trigger" — Rachel |
| 2 | 30-day repeat rate | % of converters buying again within 30 days | "If 35% repeat, I triple budget" — Anika |
| 3 | Geographic breakdown | Acceptance rate by state/metro | Required for regional brands |
| 4 | Competitive Swap Intel | Products swapped OUT of brand, anonymized, min threshold | "Killer feature" — Tom; FOMO engine |
| 5 | SKU-level breakdown | All metrics per individual product SKU | Tom multi-SKU requirement |

### 6.2 Export & Integration

| Format | Content | When |
|---|---|---|
| **CSV** | Row-level transaction log (non-negotiable for all brands) | On-demand + daily auto |
| **PDF** | Campaign summary for exec presentations | On-demand |
| **API** | REST endpoint: metrics by campaign_id + date range | Phase 2 (for Tableau/BI) |

### 6.3 Competitive Swap Intel (The FOMO Engine)

> "If I saw Heinz ketchup getting swapped for Sir Kensington's 50 times last week, I'd sign up today." — Kraft Heinz PM

Shows anonymized, aggregated data to ALL brands in a category:
- "In [Ketchup] category, [Brand X] was swapped out N times this week"
- "Top replacements: [Brand A] (23), [Brand B] (14), Other (10)"
- **Minimum threshold:** Don't show data for <10 events (prevents inference attacks)
- Visible even to brands without active campaigns (drives sign-ups)

### 6.4 Dispute Resolution

| Step | SLA | Action |
|---|---|---|
| Brand reports discrepancy | — | Self-service: compare dashboard count to CSV export |
| If discrepancy confirmed | 24h initial response | Thriftly support investigates |
| Resolution | 5 business days | Credit issued if Thriftly error confirmed |
| Fraud credits | Within 1 billing cycle | Retroactive refund for confirmed fraud |

---

## 7. Shopper Savings Experience

### 7.1 Savings Dashboard (PennyWallet)

```
┌────────────────────────────────────────┐
│  🎉  You've saved $14.50 this month!  │
│                                        │
│  This trip ········· $3.20             │
│  This month ········ $14.50            │
│  All time ·········· $47.80            │
│                                        │
│  ┌──────────────────────────────────┐  │
│  │  $5.50 more to reach $20! 🎯    │  │
│  │  ████████████████░░░░░░ 73%     │  │
│  └──────────────────────────────────┘  │
│                                        │
│  Recent savings:                       │
│  ✓ Sir Kensington's Ketchup  -$1.50   │
│  ✓ Banza Rotini             -$1.00    │
│  ○ Magic Spoon Cereal       -$0.70    │
│    ↳ Pending receipt verification      │
└────────────────────────────────────────┘
```

**Key UX Rules (from shopper focus group):**
- Show per-trip, monthly, AND all-time savings (three views)
- Goals motivate: "$5.50 away from $20 saved this month!"
- NEVER show declined swaps as "missed savings" — this is condescending
- Distinguish realized savings from pending (receipt verification)
- Referral earnings shown separately: "Swap savings: $4.50 | Guild earnings: $12.30"
- Savings must be redeemable via a clear, short path (no trapped wallet)

---

## 8. Anti-Patterns (Things We Must NEVER Do)

These emerged across all three focus groups as trust-destroying behaviors:

| # | Anti-Pattern | Why It's Fatal | Mitigation |
|---|---|---|---|
| 1 | **Bait-and-switch pricing** | Savings figure wrong at user's actual store → trust collapse | Store-specific price verification required before surfacing swap |
| 2 | **Hidden sponsorship** | User discovers paid placements weren't disclosed → app deletion | "Brand Partner" label on every paid swap, always |
| 3 | **Relentless re-suggestion** | Same swap after explicit decline → "Penny isn't listening" | 3-strike permanent suppression, unanimous requirement |
| 4 | **Mid-trip interruption** | Swap suggestions during active shopping → friction in execution mode | Planning mode / shopping mode state boundary |
| 5 | **Empty "Why?"** | Marketing copy behind "Why?" button → confirms "just an ad" hypothesis | Real data: ingredient comparison, unit price, ratings |
| 6 | **Allergen-blind suggestions** | Swap conflicts with dietary restriction → safety incident | Hard pre-filter on allergen/dietary data, not soft warning |
| 7 | **Trapped wallet** | Earnings accumulate but can't be redeemed → resentment | Clear redemption path disclosed upfront |
| 8 | **Missed savings shaming** | "You could have saved $14.50" on declined swaps → condescending | Never frame deliberate choices as failures |
| 9 | **Excited Penny for paid products** | Detectable enthusiasm increase for sponsored items → fake feeling | Same tone whether sponsored or organic, enforce in prompt |
| 10 | **Silent swap card failures** | Cards fail to load in dead zones → broken experience | Pre-load swap data before trip or fail gracefully |
| 11 | **Charging per offer** | Per-impression dressed as performance → brands leave immediately | Per-acceptance only, non-negotiable |
| 12 | **Soft budget limits** | Budget "exceeded" → brand trust destroyed | Hard stops with immediate email alert |
| 13 | **Opaque fraud metrics** | Hidden fraud rate → brands assume the worst | Publish fraud rate in dashboard |

---

## 9. MVP Scope vs Phase 2

### MVP (Weeks 1-3)

| Component | Scope |
|---|---|
| **Swap matching** | Check list items against `brand_deals`, return matches, respect suppressions |
| **Swap UX** | Inline card below item, "Swap" / "No thanks", "Why?" with real data |
| **Swap events** | Log all offers, accepts, declines; decrement budget |
| **Admin seeding** | Andy manually creates first 10 deals via admin endpoint |
| **Receipt capture** | In-app guided photo + camera roll upload |
| **Receipt processing** | Textract extraction → Claude entity resolution → item matching |
| **Dedup** | Perceptual hash on upload |
| **Fraud basics** | Rate limiting, date check, math validation, 30-day payout hold |
| **Savings display** | Penny tells user their savings in chat |
| **Brand billing** | Pre-funded deposit, per-accepted-swap charge |
| **Brand reporting** | Simple dashboard: spend, swaps, acceptance rate, budget remaining |
| **CSV export** | Row-level transaction log |
| **Evidence** | Original image stored with SHA-256 hash, 2-year retention |
| **Audit log** | Append-only `claim_audit_log` for all state transitions |

### Phase 2 (Weeks 4-8)

| Component | Scope |
|---|---|
| **Brand Portal** | Self-serve campaign creation at brands.thrift.ly |
| **Stripe integration** | Card-on-file, pre-funded deposits, eventual invoicing |
| **Conversational swaps** | Penny explains WHY, answers follow-up questions about products |
| **Email receipts** | Parse forwarded email receipts from Instacart/Walmart/Target |
| **Advanced fraud** | ELA, font classifier, device fingerprinting, account clustering |
| **Retention metrics** | 30-day repeat rate, first-time trial rate |
| **Competitive Intel** | Category swap-out data with min threshold |
| **Reporting API** | REST endpoints for BI tool integration |
| **Invoicing** | Net-30 for enterprise accounts |
| **Copy approval** | Brand reviews Penny's swap suggestion language before campaign goes live |
| **PennyWallet** | Savings balance with real payout path |
| **Holdout testing** | Incrementality measurement for brands |

---

## 10. Key Decisions Log

| Decision | Choice | Alternatives Considered | Rationale |
|---|---|---|---|
| OCR provider | AWS Textract Expense Model | Google Document AI, Claude Vision only | Textract has purpose-built receipt model; Claude for reasoning layer on top |
| Billing trigger | Per accepted swap | Per offer, per verified purchase | Brand consensus unanimous; verified = premium tier |
| Receipt storage | Original image + SHA-256 | Compressed only, extracted text only | Chain of custody requires unmodified original |
| Matching primary | Deterministic lookup DB | LLM-only, hybrid only | Auditable, fast, consistent; LLM = fallback for long tail |
| Payout hold | 30 days | 7 days, 14 days, 60 days | Covers typical return window without excessive delay |
| Brand receipt access | Never (Thriftly = intermediary) | Aggregated access, on-demand image | PII protection + competitive moat as trusted intermediary |
| Exclusivity | No forced exclusivity | Category lockout option | Structurally favors incumbents, hurts challenger brands |
| Auction mechanism | Documented, transparent rules | Opaque algorithmic, highest bidder | All brand PMs demanded transparency |
| Frequency cap default | 2-3 per session | Unlimited, 1 per session | Compromise: curated default, power-user unlimited mode |
| Suppression rule | 3 declines = permanent | 5 declines, time-based reset | 6/6 shopper consensus on permanent after 3 |

---

## 11. Focus Group Participant Insights Worth Revisiting

### From Shoppers

- **Devon's "Swap Everything" mode:** Bulk-swap entire list to cheapest alternatives, review once. Power-user feature for brand-agnostic price-hunters. Consider for Phase 2.
- **Priya's paid ad-free tier:** Willing to pay subscription for no sponsored suggestions. Revenue diversification signal. Consider for Phase 3.
- **Marcus's two-tier badges:** "Independently Recommended" vs "Sponsored" as distinct trust tiers. Compelling but requires editorial judgment on which non-paid suggestions earn the badge.
- **James's pre-trip swap preview:** Consolidated "Penny's Picks" review before shopping. Aligns decisions with planning psychology. Build in MVP.

### From Brand PMs

- **Dave's category creation insight:** Penny Swaps positioned as digitally-native trade promotion = $15B market, not "shopper marketing app." Frame accordingly in investor materials and brand sales.
- **Anika's incrementality via holdout testing:** Standard in pharma/academic marketing, rare in shopper marketing. Single highest-leverage feature for unlocking enterprise budgets. Phase 2 priority.
- **Megan's copy approval:** Premium brands need creative control. Simple workflow, huge unlock. Phase 2.
- **Tom's verified purchase premium:** Brands will pay $2.50 per verified vs $1.50 per accepted. Two-tier pricing doubles revenue per verified swap.

### From Receipt Specialists

- **Two-stage pipeline consensus:** Dedicated OCR (Textract) + LLM reasoning (Claude) = better than either alone.
- **User confirmation flywheel:** Confirmations feed abbreviation DB → matching improves → fewer confirmations needed → better UX. Build the feedback loop in MVP.
- **30-day payout delay as MVP fraud defense:** Simple, effective, no engineering complexity. Buys time to build more sophisticated fraud detection.
- **Thriftly as trusted intermediary:** The privacy boundary (brands never see receipt images) is a competitive moat, not just compliance. Emphasize in brand sales.
