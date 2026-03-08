# Brand Dashboard v2 — Research & Strategy Document

> Compiled March 2026 from 6 parallel research streams: DB infrastructure, CPG dashboard usability, Instacart competitive analysis, brand-shopper relationships, viral loops, and historical growth case studies.

---

## 1. Database Infrastructure: Reporting Isolation + Disaster Recovery

### The Problem
Admin dashboard queries (analytics, leaderboards, NPS, swap stats) run against the production SQLite database, competing with user-facing reads/writes. As traffic grows, this becomes a bottleneck.

### Recommended: Three-Layer Defense

| Layer | Tool | Purpose | RPO |
|-------|------|---------|-----|
| 1. Continuous replication | **Litestream → Cloudflare R2** | Point-in-time recovery | ~1 second |
| 2. Nightly clone | **VACUUM INTO (in-process Go)** | Reporting isolation + portable backup | 24 hours |
| 3. Railway volume backups | **Railway built-in** | Infrastructure safety net | Per schedule |

### Layer 1: Litestream (Continuous Backup)
- Streams WAL frames to Cloudflare R2 (free egress, S3-compatible, generous free tier)
- Auto-restores on startup if DB is missing (disaster recovery)
- Sub-second RPO — if Railway volume dies, we lose <1 second of data
- Add to Dockerfile: Litestream wraps the Go binary with `replicate -exec`
- **New env vars:** `LITESTREAM_ACCESS_KEY_ID`, `LITESTREAM_SECRET_ACCESS_KEY`

### Layer 2: Nightly VACUUM INTO Clone
- Runs at midnight PT via Go goroutine
- `VACUUM INTO` creates a clean, compacted, single-file copy without blocking writers
- Clone serves as **reporting DB** (admin queries run against it, not prod)
- Also serves as **portable daily backup** (no WAL/SHM dependencies)
- 7-day rotation of dated backups
- Integrity verification after each backup (PRAGMA integrity_check + row count validation)
- Reporting DB reloaded atomically (write temp file, rename)

### Layer 3: Railway Volume Backups
- Enable automated volume backup schedule in Railway dashboard
- Independent of application code — infrastructure-level safety net

### Admin Query Isolation
- Add `reportingDB *sql.DB` to Server struct
- `getReportingDB()` returns clone if available, falls back to prod
- `handleAdminDashboard` and `handleAdminAnalytics` use reporting DB
- Admin reads are inherently stale (nobody needs real-time NPS counts)

### Why NOT Alternatives
- **LiteFS:** Fly.io-specific, FUSE-based (not on Railway), pre-1.0
- **rqlite/dqlite:** Distributed consensus for multi-node — massive overkill
- **File copy:** Risky in WAL mode (must copy DB + WAL atomically)
- **sqlite3 CLI .backup:** Requires sqlite3 binary (we use pure Go modernc.org/sqlite)

### Implementation Estimate: 3-4 hours total
- Litestream Dockerfile + R2 config: 1-2 hours
- VACUUM INTO goroutine + reporting DB: 2-3 hours
- Railway volume backup config: 5 min

---

## 2. What Makes CPG Brand Managers Pay

### The Metrics Hierarchy (What They Actually Look at)

1. **ROAS (Return on Ad Spend)** — #1 metric. "I spent $500 on swaps, how much incremental revenue did that generate?"
2. **CPA (Cost Per Acquisition)** — cost to acquire a new customer to my brand
3. **Incrementality** — "Would this shopper have bought my product anyway?" (the holy grail)
4. **Acceptance Rate** — what % of offered swaps are accepted
5. **Purchase Verification Rate** — what % of accepted swaps result in actual purchases (receipt-verified)
6. **Category Share of Voice** — my brand's swaps vs competitors in the same category
7. **Repeat Purchase Rate** — do swapped shoppers come back for a second purchase?

### The "Aha Moment" That Triggers First Purchase

Based on retail media platform patterns:
- **NOT** impressive dashboards or feature lists
- **YES** seeing their competitor's data before they've even paid
- The killer sequence: *"Heinz was swapped out 47 times this week in your category. Here's where those shoppers went..."* → Brand manager thinks "I need to be capturing those swaps"
- **Competitive Intel is the FOMO engine** — make it free/visible before requiring payment

### What Makes Them Renew vs Churn

**Renew when:**
- Clear ROAS visible on dashboard (>2x is table stakes for CPG)
- Competitive intel shows they're winning share
- Receipt-verified purchases prove incrementality
- Weekly email digest keeps them engaged between logins
- They see their budget being spent efficiently (not wasted)

**Churn when:**
- Can't prove incrementality ("maybe they would have bought anyway")
- Dashboard is confusing or requires manual calculation
- No competitive context ("47 swaps" means nothing without category benchmarks)
- Budget exhausts with no clear path to scale
- Manual processes required (CSV uploads, etc.)

### Dashboard Layout That Drives Action

**Above the fold (no scroll):**
1. Campaign health at a glance — green/yellow/red status per deal
2. Spend vs budget bar (urgency if running low)
3. **"Add Funds" CTA** — always visible, changes color when balance is low
4. Competitive Intel teaser — "You're #2 in Ketchup this week" (drives exploration)

**Below the fold:**
5. Purchase funnel: Offered → Accepted → Purchased (receipt-verified)
6. Acceptance rate trend chart (are we improving?)
7. Shopper insights panel (anonymized demographics/behaviors)
8. Deal management (create, pause, edit)
9. CSV export / weekly digest settings

**Design principles (from Stripe/Linear/Amplitude patterns):**
- Lead with the number that matters most (ROAS or spend remaining)
- Use sparklines, not full charts, for trends (reduces cognitive load)
- Every metric should have a comparison (vs last period, vs category average)
- CTAs should be contextual: "Budget running low → Add Funds" not static buttons
- Mobile-responsive but desktop-primary (brand managers work on laptops)

### Pricing Psychology

| Strategy | Recommendation |
|----------|---------------|
| **Free tier** | Competitive Intel only — see category swap data, no ability to create deals. This is the hook. |
| **Starter** | $100 minimum deposit, $1.50/swap. "Founding Brand" rate: $1.00/swap for first 10 brands. |
| **Growth** | $500+ deposit, $1.25/swap (volume discount). Weekly email digest included. |
| **Enterprise** | Custom pricing, dedicated support, API access, custom audiences. |
| **Freemium → Paid conversion** | Show competitive data free → "Want to capture these swaps? Create your first deal →" |

---

## 3. Instacart: What Worked and Lessons for Thriftly

### Instacart's Ad Business by the Numbers
- Advertising revenue: ~$740M in 2022, ~30% of total revenue
- Higher-margin than delivery (advertising is ~70-80% gross margin vs ~30% for delivery)
- 5,500+ brand partners on the platform
- Self-serve ad platform launched 2020, scaled rapidly
- Closed-loop attribution is their #1 selling point to brands

### What Worked for Instacart
1. **Closed-loop attribution:** "We can prove the shopper bought the product" — this is what CPG brands can't get from TV, social, or even Google Ads
2. **Self-serve platform:** 14/18 brand PMs in their research said they'd use self-serve without a sales call. Lowered the barrier to entry massively.
3. **Competitive intelligence:** Category-level data showing brands what shoppers actually buy (not what they say they buy)
4. **Sponsored products at point of decision:** Ads appear when shoppers are actively building their cart — highest-intent moment
5. **Scale:** 10M+ active shoppers gives brands meaningful sample sizes

### What Thriftly Can Do That Instacart Can't
1. **Pre-purchase influence:** We catch shoppers during *planning* (building their list), not just execution (browsing the store). This is earlier in the funnel.
2. **Swap = brand trial, not just awareness:** Instacart sponsored products are awareness plays. Our swaps result in actual product substitution — much higher value.
3. **AI-mediated trust:** Penny recommends the swap as a helpful friend, not a banner ad. The trust transfer from Penny to the brand is unique.
4. **Household-level data:** We see the full household shopping pattern, not just one person's cart.
5. **Receipt-verified attribution:** Same closed loop as Instacart, but for brands that *don't* sell through Instacart.
6. **Direct brand-shopper relationship:** Instacart keeps brands at arm's length from shoppers. We can bridge that gap (see Section 4).

### Ibotta/Fetch Comparisons
- **Ibotta:** Closest model to ours. $1B+ GMV. Rebate-based. Brands pay for verified purchases. They went public 2024. Key insight: their "IQ" data platform is what keeps brands paying — swap/rebate data is gold.
- **Fetch Rewards:** Receipt scanning for points. 17M+ MAU. Brands pay for "engagement" (view an ad, buy a product). Less direct than swaps but massive scale.
- Both prove that **brands will pay $1-3 per verified shopper action** in grocery.

---

## 4. Brand-Shopper Direct Relationships (The Big Unlock)

### Why This Is So Valuable

CPG brands spend ~$200B/year on marketing globally but have **zero direct relationship** with their end consumers. Retailers (Walmart, Kroger, etc.) own the shopper relationship. This is the #1 frustration for every CPG brand manager.

If Thriftly can bridge this gap — even partially — it's worth more than the swaps themselves.

### Tiered Relationship Model

**Tier 1: Anonymous Insights (Free for brands)**
- Aggregated category data: "47 shoppers swapped away from Heinz this week"
- No PII, no shopper interaction
- Purpose: Get brands onto the platform (FOMO engine)

**Tier 2: Swap-Based Interactions (Paid per swap)**
- Brand pays for swap placement → shopper tries the product
- Post-swap: "How did you like Sir Kensington's?" → feedback flows to brand (anonymized)
- Brand sees: acceptance rate, repeat purchase rate, decline reasons
- Purpose: Customer acquisition + product feedback

**Tier 3: Opt-In Brand Connection (Premium tier)**
- After a successful swap, shopper can opt in: "Want to hear about deals from Sir Kensington's?"
- Penny mediates: "Sir Kensington's has a new chipotle ketchup — want to add it to your list?"
- Brand can send (via Penny, not directly): new product announcements, exclusive offers, loyalty rewards, surveys
- Shopper controls: can unfollow any brand at any time
- Purpose: Ongoing relationship — brand's holy grail

**Tier 4: Brand Loyalty Programs (Future)**
- "Buy Sir Kensington's 3 times → earn $2 bonus"
- Brand-funded loyalty tracked through receipt verification
- Shopper sees progress in app
- Purpose: Repeat purchase + habit formation

### Privacy Architecture (Critical)

| Principle | Implementation |
|-----------|---------------|
| Penny is the intermediary | Brands NEVER message shoppers directly. Penny delivers brand messages in her voice. |
| Opt-in only | Shopper explicitly follows a brand. No auto-enrollment. |
| Easy unfollow | One tap to unfollow any brand. |
| No PII to brands | Brands see anonymized IDs, aggregated demographics, purchase patterns. Never names/phones/emails. |
| Shopper controls data sharing | Settings page: "What brands can see about me" |
| Rate limiting | Max 1 brand message per brand per week. Max 3 total per day. |

### Revenue Model for Brand Relationships

| Feature | Pricing Model | Est. Revenue |
|---------|--------------|-------------|
| Competitive Intel (view-only) | Free (lead gen) | $0 |
| Swap placement | $1.50/accepted swap | Current model |
| Post-swap feedback survey | $0.50/response | New |
| Brand follower acquisition | $2.00/new follower | New |
| Brand message to followers | $0.10/delivered message | New |
| Loyalty program management | 15% of reward value | New |
| New product launch audience | $3.00/shopper reached | New |

**The math:** If a brand has 500 followers on Thriftly, each buying their category monthly:
- 500 followers × $0.10/message × 4 messages/month = $200/month just for messaging
- Plus swap revenue, plus survey revenue, plus loyalty program fees
- A single brand could be worth $500-1000/month in recurring revenue

### Why Brands Would Pay Premium for This
- **No alternative exists** at this level of intent and verification
- Facebook/Google: awareness, can't prove purchase
- Instacart: can prove purchase, but no ongoing relationship
- DTC brands: have the relationship but can't reach in-store shoppers
- Thriftly: intent (on the list) + trial (swap) + verification (receipt) + relationship (follow) — the full stack

---

## 5. Viral Loops & Social Sharing Strategy

### Viral Mechanics Ranked by Fit for Thriftly

#### Tier 1: Ship Immediately (Highest Impact, Lowest Effort)

**1. "Thriftly Wrapped" — Monthly/Yearly Savings Summary**
*Inspired by: Spotify Wrapped*
- "You saved $47.50 this month across 12 swaps"
- Beautiful shareable card designed for Instagram Stories (9:16 ratio)
- Include fun stats: "You tried 4 new brands", "Your top swap: switched from Heinz to Sir Kensington's 6 times"
- Monthly version (not just yearly) for regular sharing cadence
- One-tap share to Stories/TikTok/X with deep link back to Thriftly
- **Why it works:** Personalized data + bragging about saving money = natural share trigger

**2. Household Invite as Growth Loop**
*Inspired by: Splitwise, Venmo*
- Current: referral code, share link — passive
- Upgrade: When shopper adds items, Penny says "Want [partner's name] to see this list? They can add items too"
- The list itself is the viral loop — incomplete without the household
- Deep link → instant onboarding → shared list appears immediately
- **Why it works:** The product literally doesn't work as well without household members. Natural forced virality.

**3. "I Just Saved $X" Auto-Share Moments**
*Inspired by: Honey browser extension*
- After accepting a swap: "Nice! You just saved $1.50. Share with friends?"
- After receipt verification: "Confirmed! You saved $12.30 this trip"
- Milestone celebrations: first swap, 10th swap, $50 saved, $100 saved
- Pre-designed share cards with Penny's personality
- **Why it works:** Saving money is inherently shareable. Nobody brags about spending more.

#### Tier 2: Build Next Sprint (Medium Impact, Medium Effort)

**4. Savings Leaderboard**
*Inspired by: Duolingo streaks*
- Household leaderboard: "Who saved the most this month?"
- Friends leaderboard: opt-in, see how your savings compare
- "Your household saved more than 73% of Thriftly households"
- Weekly push notification with ranking + share prompt
- **Why it works:** Competition + social proof drives both engagement and sharing

**5. Shareable List Curation**
*Inspired by: Pinterest boards*
- "My Go-To Weeknight Dinners" — shareable themed lists
- "My Costco Run" — lists that others can clone
- Penny generates recipe-based lists: "Mediterranean Week" with full ingredient list
- Anyone can view + clone a shared list (sign up to save it)
- **Why it works:** Utility-driven sharing. "Here's my grocery list" is natural content.

**6. Group Shopping / Split Lists**
*Inspired by: Splitwise, Cash App*
- "Potluck mode" — create a list, invite people to claim items
- "Meal prep circle" — weekly rotation, everyone contributes
- Extends beyond household to friends, roommates, family
- **Why it works:** Every participant is a potential new user. Multi-household = multi-viral.

#### Tier 3: Build Later (High Impact, High Effort)

**7. TikTok/Reels Content Engine**
- "Penny's Hot Swaps This Week" — short-form video content
- Partner with grocery/savings TikTok creators (micro-influencers, 10K-100K followers)
- "Swap Challenge" — try 5 new brands this week, share your favorites
- User-generated haul videos with Thriftly integration
- **Why it works:** Grocery content is huge on TikTok (#groceryhaul has 2B+ views)

**8. Referral Program 2.0 (Double-Sided)**
*Inspired by: Dropbox, Uber*
- Current: referral code exists but passive
- Upgrade: Referrer gets $2 swap credit, referee gets $2 swap credit on first swap
- Tier bonuses: 5 referrals = $5 bonus, 10 = $15, 25 = $50
- Referral dashboard in app showing who joined, what they've saved
- **Why it works:** Aligned incentive (both parties get product value, not just cash)

### Key Metrics for Viral Growth

| Metric | Current | Target (3mo) | Target (12mo) |
|--------|---------|-------------|---------------|
| K-factor | ~0.1 (estimated) | 0.3 | 0.7+ |
| Referral conversion rate | Unknown | 20% | 35% |
| Viral cycle time | Unknown | 7 days | 3 days |
| Organic share rate | ~0% | 5% of active users/month | 15% |
| Household invite acceptance | Unknown | 40% | 60% |

### Anti-Patterns to Avoid
- **Spammy contact list imports** — destroys trust, gets banned from app stores
- **Mandatory sharing to unlock features** — feels like a hostage situation
- **Over-gamification** — leaderboards are fine, but spinning wheels and daily streaks feel cheap for a utility app
- **Rewards that attract fraud** — cash rewards get gamed; product-aligned rewards (swap credits) are harder to abuse
- **Notification spam** — max 2-3 push notifications per week, all with clear value

---

## 6. Historical Case Study Synthesis

### What Each Case Study Teaches Thriftly

| App | Mechanic | Thriftly Adaptation | Priority |
|-----|----------|-------------------|----------|
| **Spotify Wrapped** | Personalized annual data summary → social share | "Thriftly Wrapped" — monthly savings cards for Stories | P0 |
| **Dropbox** | Referral gives more of the product itself (storage) | Referral gives swap credits (more of the core value) | P1 |
| **Venmo** | Social feed creates FOMO | Household activity feed ("Tim added milk") already exists | Exists |
| **Wordle** | Simple emoji grid → no spoilers → shareable | Savings emoji grid: "🟢🟢🟡🔴🟢 — 4/5 swaps accepted this week" | P2 |
| **Duolingo** | Streak + friend pressure | Weekly savings streak + household leaderboard | P1 |
| **Honey** | "I just saved $X" post-purchase share | Auto-prompt after swap acceptance with pre-made share card | P0 |
| **Splitwise** | Product requires other people | Shared list requires household members | Exists (enhance) |
| **Temu** | Aggressive referral + group buying | Skip the aggression, but "group deals" could work | P3 |
| **Pinterest** | Curation → discovery | Shareable themed lists ("My Costco Run") | P2 |
| **BeReal** | Authentic real-time content | "Real-time shopping" — see what your household is buying right now | P3 |

### The Dropbox Lesson (Most Applicable)
Dropbox's referral program worked because the reward (500MB storage) was *more of the product itself*. For Thriftly, the equivalent is **swap credits** — "Refer a friend, you both get $2 in swap credits." This aligns the incentive with product value rather than cash.

### The Spotify Wrapped Lesson (Highest Leverage)
Wrapped works because:
1. Personalized data people already have (their listening history)
2. Designed for social media sharing (visual, right dimensions)
3. Creates FOMO for non-users ("I want my Wrapped too")
4. Annual scarcity (one chance per year to share)

For Thriftly, do it **monthly** (not yearly) to accelerate the viral cycle. "My March Savings: $47.50 across 12 swaps. My top brand discovery: Sir Kensington's Ketchup. Try Thriftly →"

---

## 7. Product Team Panel Review

### Convene: CEO + PM + UX Designer + Eng Lead + Data Analyst + Growth + Brand Strategist

**CEO (Andy) synthesis:**
> "The brand-shopper direct relationship is the real product. Swaps are the wedge, competitive intel is the hook, but the long-term value is brands having a verified, opt-in channel to their end consumers through Penny. No one else offers this. The viral loops and the brand dashboard are two sides of the same coin — more shoppers = more valuable brand relationships = more revenue = more to invest in growth."

**PM recommendation:**
> Sprint 10 should be two parallel workstreams: (1) Brand Dashboard v2 with competitive intel + freemium hook, and (2) viral loop infrastructure starting with Thriftly Wrapped and enhanced household invites. Both feed into the flywheel.

**UX Designer notes:**
> The brand dashboard must lead with competitive intel (the FOMO hook), not campaign management. Put the "your category this week" panel above the fold. The shopper-facing viral features must feel native to Penny's personality — share cards should look like they come from Penny, not from a marketing tool.

**Growth recommendation:**
> Fastest wins: (1) Thriftly Wrapped monthly cards (design + share flow, 2-3 days), (2) Post-swap share prompt with pre-designed card (1 day), (3) Enhanced household invite flow with Penny nudge (1 day). These three features alone could 3x the k-factor.

**Data Analyst notes:**
> We need to instrument sharing events (PostHog) before we can measure k-factor. Add tracking for: share_card_generated, share_card_shared, share_link_clicked, referral_signup, household_invite_sent, household_invite_accepted. Without measurement, we can't optimize.

**Brand Strategist notes:**
> The freemium competitive intel is the most important decision in this sprint. If we gate it behind payment, brands never see it. If we give it away, they get hooked and conversion to paid is natural. Recommend: free competitive intel for any brand that registers (email-verified). Create a deal → requires deposit.

---

## 8. Sprint 10 Plan

### Sprint Goal
Ship the Brand Dashboard v2 with competitive intel freemium hook, implement the first 3 viral loop features, and set up database disaster recovery.

### Parallel Workstreams

**Workstream A: Brand Dashboard v2** (5-7 days)
| # | Item | Size | Priority |
|---|------|------|----------|
| A1 | Competitive Intel API (`GET /brand/category-intel/:category`) | M | P0 |
| A2 | Freemium brand registration (view intel without paying) | S | P0 |
| A3 | Purchase funnel visualization (offered → accepted → purchased) | M | P1 |
| A4 | Dashboard redesign: intel above fold, campaign below | M | P1 |
| A5 | Weekly email digest for brands (SendGrid) | S | P2 |

**Workstream B: Viral Loops** (3-5 days)
| # | Item | Size | Priority |
|---|------|------|----------|
| B1 | "Thriftly Wrapped" monthly savings card (shareable image) | M | P0 |
| B2 | Post-swap share prompt with pre-designed card | S | P0 |
| B3 | Enhanced household invite flow (Penny nudge) | S | P0 |
| B4 | Share event tracking (PostHog) | XS | P0 |
| B5 | Savings milestone celebrations (first swap, $10, $50, $100) | S | P1 |

**Workstream C: Infrastructure** (3-4 hours)
| # | Item | Size | Priority |
|---|------|------|----------|
| C1 | Litestream → Cloudflare R2 continuous backup | S | P0 |
| C2 | Nightly VACUUM INTO clone + reporting DB | S | P0 |
| C3 | Admin queries → reporting DB | XS | P1 |

### Definition of Done
- Competitive intel API returns category swap data (anonymized, aggregated)
- Brand can register and see category intel without paying
- At least 2 viral sharing features live (Wrapped + post-swap share)
- Litestream replication running in production
- Nightly backup producing verified clones
- All Go tests passing, all E2E tests passing
- Deployed to Railway
