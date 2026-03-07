# Focus Group Findings — Penny Swaps

> **Full technical design:** See `TECH_DESIGN_PENNY_SWAPS.md` for database schemas, API contracts, pipeline architecture, and fraud prevention.

## Panel Summary

| Panel | N | Conducted | Key Finding |
|---|---|---|---|
| Grocery Shoppers | 6 | March 2026 | 3 declines = permanent suppression; store-specific pricing required; swaps in planning mode only |
| Receipt Verification Specialists | 6 | March 2026 | Textract + Claude two-stage pipeline; 7-year retention; pHash dedup as MVP fraud defense |
| CPG Brand Product Managers | 18+6 | March 2026 | Challenger brands pay $1-2/swap immediately; self-serve for non-enterprise; Competitive Swap Intel is the killer brand acquisition feature |

---

## Grocery Shoppers (Swap UX)

> Panel of 6 shoppers: Sarah (32, suburban mom, Kroger), Marcus (28, urban professional, Whole Foods), Linda (55, empty-nester, Walmart), James (41, dad, Costco/Target), Priya (36, health-conscious, Trader Joe's/Sprouts), Devon (24, recent grad, Aldi). Full report in TECH_DESIGN_PENNY_SWAPS.md.

### Consensus Points (6/6 agreed)

1. **3 declines = permanent suppression.** Non-negotiable. Repeating a declined suggestion destroys trust.
2. **Price accuracy must be store-specific.** National average pricing = trust collapse.
3. **Unit price normalization required.** Don't compare different-size packages with raw savings.
4. **Swaps belong in planning mode, not execution mode.** No suggestions during active shopping.
5. **The "Why?" button must have real data.** Marketing copy behind "Why?" is worse than no button at all.
6. **Penny's tone must be consistent.** No detectable enthusiasm increase for sponsored items.

### Key Insights

| Insight | Source | Impact |
|---|---|---|
| Personalize "why" by user's value axis (price/ingredients/convenience) | Marcus, Priya | Different message template per user profile |
| "Pre-trip swap preview" — consolidated decision point before shopping | James | New UX mode: Penny's Picks |
| Allergen-blind suggestions = safety incident, not preference mismatch | Priya | Hard filter required at system level |
| "Swap Everything" bulk mode for brand-agnostic shoppers | Devon | Power-user feature, Phase 2 |
| Paid ad-free tier — willing to pay subscription for no sponsored suggestions | Priya | Revenue diversification signal |
| Decline reason feedback ("tried before", "dietary") trains Penny | Devon | User feels they're teaching, not dismissing |
| Receipt scanning adoption is bimodal: 50% enthusiastic, 30% prefer digital | Panel split | Build photo AND email receipt paths |

### Receipt Scanning Likelihood (1-10)

| Sarah | Marcus | Linda | James | Priya | Devon | **Average** |
|---|---|---|---|---|---|---|
| 6 | 3 | 8 | 4 | 5 | 9 | **5.8** |

---

## Receipt Verification Specialists

> Panel of 6 specialists: Elena (shopper/expense tracker), Mike (VP Marketing CPG), Dr. Kim (CV researcher), Raj (fintech compliance), Sophie (QA engineer, tested Fetch/Ibotta), Anthony (fraud analyst). Full report in TECH_DESIGN_PENNY_SWAPS.md.

### Key Technical Decisions

| Decision | Recommendation | Rationale |
|---|---|---|
| OCR approach | Two-stage: Textract for extraction + Claude for entity resolution | Each tool's strength; Textract for structure, Claude for reasoning |
| Receipt capture | In-app guided (primary) + camera roll (secondary) + email receipt (Phase 1) | Quality gating at capture filters 70-80% of bad inputs |
| Item matching | Deterministic lookup DB (primary) + LLM fallback (secondary) | Auditable and fast; LLM for long-tail unmatched items |
| Evidence retention | 7 years (2 active + 5 cold) | Fintech standard; covers audit, regulatory, legal dispute periods |
| Chain of custody | SHA-256 hash at upload, append-only audit log | Proves image wasn't tampered with post-submission |
| Brand image access | NEVER — Thriftly = trusted intermediary | PII protection + competitive moat |
| Payout delay | 30 days | Covers typical return window; simple MVP defense against buy-return fraud |
| Fraud detection MVP | pHash dedup + rate limiting + date check + math validation | Highest ROI controls with minimal engineering complexity |

### Fraud Prevention Priority Stack

1. **Perceptual hash deduplication** — blocks receipt reuse (same or cross-user)
2. **Rate limiting** — 3 receipts/day, 10/week per user; $50/week value cap before manual review
3. **Date consistency** — receipt date within promotion window, EXIF plausible
4. **Math validation** — line items must sum to total
5. **Error Level Analysis** — detect image manipulation (Phase 2)
6. **Device fingerprint clustering** — multi-account detection (Phase 2)
7. **Receipt authenticity classifier** — real thermal vs printed/generated fake (Phase 2)

---

## CPG Brand Product Managers

> Three panels of 6 brand product managers each (18 total). Panels structured as: (A) Challenger/DTC brands (Siete, GoodBowl, Banza, Hu Kitchen, Liquid IV, Magic Spoon), (B) Mid-market CPG (Bob's Red Mill, Annie's/General Mills sub-brand, Chobani, Rxbar, Sir Kensington's, Primal Kitchen), (C) Enterprise CPG (P&G brand lead, Unilever brand lead, Kraft Heinz, Nestlé, Colgate-Palmolive, J&M Smucker).

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

### Additional Findings (March 2026 — 6-person Brand PM Deep Dive)

> Panel: Rachel (Banza), Tom (Heinz), Anika (Chobani), Carlos (Magic Spoon), Megan (Sir Kensington's), Dave (Kroger Private Label). Full report in TECH_DESIGN_PENNY_SWAPS.md.

**Billing consensus (6/6):**
- Per-accepted-swap billing ONLY — per-offer/impression pricing is a dealbreaker across the board
- Hard budget caps (daily + weekly + total) with IMMEDIATE pause and email alert within 5 minutes
- Founding Brand rate ($1.00) must be contractually protected for 18 months in MSA (Rachel: "or early adopters feel burned")
- Receipt-verified purchase premium: brands will pay $2.50 per verified vs $1.50 per accepted (Tom: "two-fifty for a verified purchase is a steal vs my $3-5 co-op CPA")
- Enterprise invoicing (Net-30) required for accounts >$10K/month (Anika: "I cannot process pre-funded deposits through our AP system")

**Reporting consensus:**
- Row-level transaction logs and CSV export are non-negotiable (Carlos: "if I can't export it, I can't use it")
- Published platform fraud rate builds trust; hiding it destroys trust (Rachel: "platforms that hide fraud metrics have fraud metrics worth hiding")
- 30-day repeat/loyalty rate is the "killer metric" no other platform provides — multiple brands said this alone would trigger budget increases

**Key strategic insight:**
Dave (Kroger): "The platform you're describing is essentially a digitally-native trade promotion system. That's a $15 billion industry. If Thriftly can become the first AI-mediated trade promotion platform with closed-loop purchase verification, that's category creation, not just a startup product."

**Evidence/audit requirements:**
- Row-level transaction logs reconcilable to invoices (Carlos: "if there's a discrepancy, I need a dispute resolution process, not 'email support and wait three weeks'")
- Dispute resolution SLA: 24h initial response, 5 business day resolution
- Receipt images: brands NEVER see raw images — only extracted, anonymized verification results (Megan, Dave: PII + competitive intelligence risk)
- Append-only audit logs (Anika: "immutable records or I can't trust the data")
