# Data Flywheel: 12-Month Narrative

**Prepared for:** CEO Review
**Date:** March 2026
**Product:** Thriftly / Penny AI

---

## Thesis

Every interaction with Penny generates data that makes her smarter. Smarter Penny means less friction for the user, which means more interactions, which means more data. This document charts the compounding story across four stages and defines the retention targets that prove it is working.

---

## Stage 1: Basic History (Month 0-3)

**What Penny collects:**
- Item add/remove/toggle events per user
- Item frequency (how often each item appears on the list)
- Brand selections and brand corrections (1-tap overrides)
- Time-of-week patterns (when users add items, when they shop)

**How it makes Penny smarter:**
- Penny learns which items each household buys regularly and can pre-populate suggestions
- Brand corrections teach Penny which defaults were wrong, improving first-guess accuracy over time
- Frequency data enables the restock nudge engine ("It's been 10 days since you bought eggs")

**User-visible improvement:**
- By week 4, Penny's brand suggestions match user preference 70%+ of the time (up from ~50% at signup)
- Restock nudges begin appearing for items with 3+ purchase cycles
- "Add your usual?" prompt on Sunday evening if user typically shops Monday

**KPIs to track:**
| Metric | Month 1 Target | Month 3 Target |
|--------|---------------|----------------|
| Brand suggestion acceptance rate | 50% | 70% |
| Restock nudge tap-through rate | -- | 25% |
| Avg items added per session | 3 | 5 |
| Data points per user per week | 10 | 25 |

---

## Stage 2: Brand Memory (Month 3-6)

**What Penny collects:**
- Cumulative brand correction history per user and per household
- Dietary attribute signals (organic, nut-free, dairy-free, vegan) inferred from brand choices
- Price sensitivity signals (frequency of choosing store-brand vs. name-brand)
- Brand loyalty patterns (always Organic Valley milk vs. rotates brands)

**How it makes Penny smarter:**
- Penny builds a per-household "brand profile" -- a probabilistic model of which brand each household prefers for each category
- Dietary attributes are inferred passively (user picks organic 85% of the time -> Penny defaults to organic)
- Price sensitivity tunes whether Penny suggests premium or value brands by default
- Penny stops suggesting brands the household has rejected 3+ times

**User-visible improvement:**
- Brand suggestions feel personalized ("Penny just knows us")
- New items default to dietary-compatible brands without user specifying (e.g., new cereal suggestion is gluten-free because household is 80% gluten-free)
- Penny says "I picked [brand] because your family usually goes organic" -- the reasoning is visible

**KPIs to track:**
| Metric | Month 3 Target | Month 6 Target |
|--------|---------------|----------------|
| Brand suggestion acceptance rate | 70% | 85% |
| Brand corrections per user per week | 3 | 1 |
| Dietary profile accuracy (user-confirmed) | -- | 80% |
| Users with 50+ brand data points | 20% | 60% |

---

## Stage 3: Household Patterns (Month 6-9)

**What Penny collects:**
- Shopping role patterns (who shops on which days, who claims which categories)
- Meal-correlated item clusters (cilantro + lime + tortillas = taco night)
- Repurchase cycles per item per household (milk every 5 days, paper towels every 3 weeks)
- Household coordination signals (claim patterns, list completion rates, multi-member activity)

**How it makes Penny smarter:**
- Penny detects "taco night" patterns and suggests the full ingredient set when one trigger item is added
- Repurchase cycle predictions become household-specific (not just population averages)
- Penny learns which household member typically handles which categories and can route nudges to the right person
- Coordination intelligence: Penny knows that when Partner A adds items on Wednesday, Partner B shops on Thursday

**User-visible improvement:**
- "Looks like taco night -- want me to add cilantro, lime, and sour cream?" (meal pattern detection)
- Restock nudges are precisely timed to household rhythm, not generic intervals
- Penny nudges the right person: "James usually grabs the heavy stuff -- want me to flag this for him?"
- Weekly list suggestions pre-populated with 60%+ of what the household will actually buy

**KPIs to track:**
| Metric | Month 6 Target | Month 9 Target |
|--------|---------------|----------------|
| Meal pattern detection accuracy | -- | 70% |
| Repurchase prediction accuracy (within 2 days) | 40% | 65% |
| Pre-populated list acceptance rate | -- | 50% |
| Multi-member households with role patterns | 20% | 45% |

---

## Stage 4: Cross-Household Trends (Month 9-12)

**What Penny collects:**
- Anonymized, aggregate category intelligence across all households (privacy-respecting, no PII)
- Seasonal patterns (turkey + cranberries spike in November, sunscreen in June)
- Regional brand popularity (Publix-brand dominance in Southeast, H-E-B in Texas)
- New product adoption curves (which households try new brands first, which follow)

**How it makes Penny smarter:**
- Penny leverages cross-household data to improve suggestions for new users (cold-start problem solved)
- Seasonal intelligence means Penny proactively suggests holiday/seasonal items before users think of them
- Regional defaults mean a new user in Austin gets H-E-B brand suggestions, not Kroger
- Trend detection: "A lot of families in your area are trying [new brand] -- want to swap?"

**User-visible improvement:**
- New users get 75%+ accurate brand defaults on day 1 (up from 50% at launch) thanks to population-level learning
- Penny suggests seasonal items 1-2 weeks before the user would have thought of them
- Regional intelligence feels local ("Penny knows my grocery store")
- Penny becomes a discovery engine: surfacing products that similar households love

**KPIs to track:**
| Metric | Month 9 Target | Month 12 Target |
|--------|---------------|-----------------|
| New user brand acceptance rate (day 1) | 65% | 75% |
| Seasonal suggestion relevance (user-rated) | -- | 70% |
| Cross-household data points (aggregate) | 500K | 2M |
| Predictive list completeness (% of final list pre-suggested) | 40% | 60% |

---

## Retention & Activation Targets

These are the north star metrics for the product. Every feature decision, sprint priority, and Gate 3 evaluation ties back to moving these numbers.

| Metric | Definition | Target | Why It Matters |
|--------|-----------|--------|----------------|
| **Day 1 activation** | % of new users who add 3+ items on first day | **60%** | Gate 1 proof point. If a user doesn't add 3 items, they never experience the household invite prompt or Penny's intelligence. 3 items is the minimum for Penny to start learning. |
| **Day 7 retention** | % of activated users who return within 7 days | **50%** | Proves the list is sticky enough to replace the notepad/text-chain. If users don't come back within a week, Penny never accumulates enough data for the flywheel to spin. |
| **Day 30 retention** | % of activated users still active at 30 days | **35%** | The flywheel checkpoint. By day 30, Penny should have enough brand and frequency data that the user feels "Penny knows me." If retention drops below 35%, the flywheel is not spinning. |
| **Household conversion** | % of activated users who invite someone within 14 days | **25%** | The 11-Star Moment requires 2+ people. Household conversion is the single most important leading indicator of whether users will experience "you never ask 'did you get milk?' again." |

### Retention Targets as Decision Framework

When evaluating any feature or sprint priority:

1. **Does it improve Day 1 activation?** If yes, it likely passes Gate 1.
2. **Does it improve Day 7 or Day 30 retention?** If yes, it likely passes Gate 3 (the flywheel is retaining users long enough to compound data).
3. **Does it improve Household conversion?** If yes, it likely passes Gate 2 (the coordination narrative is compelling enough to share).

A feature that moves none of these four numbers should be deprioritized regardless of how polished or requested it is.

---

## The Compounding Story (Executive Summary)

```
Month 0-3:  Penny remembers what you buy.
Month 3-6:  Penny knows your family's brands and dietary needs.
Month 6-9:  Penny anticipates your meals and shopping rhythm.
Month 9-12: Penny knows your neighborhood and discovers new products for you.
```

Each stage builds on the data from the previous stage. A user who has been on the platform for 9 months gets a qualitatively different experience than a new user -- and that gap widens every month. This is the moat.

---

*This document is the canonical reference for Gate 3 decisions and retention target evaluation. See also: `CLAUDE.md` (Product Shipping Gates), `11_STAR_NARRATIVE.md` (Gate 2 companion), and `../pitch deck/penny-brand-source.json` (brand messaging source of truth).*
