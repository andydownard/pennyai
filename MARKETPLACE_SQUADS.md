# Marketplace MVP — Squad Competition Results

> Conducted March 2026. Three product squads independently designed marketplace MVP roadmaps. CEO synthesized the final recommendation.

---

## Squad Alpha: "Consumer First"

**Team:** PM Mira (consumer growth), UX Darren (mobile-first), Architect Suki (event-driven), Staff Dev Marcus (ship fast)

**Thesis:** Consumer-to-consumer virality is the prerequisite for everything else. Build the sharing flywheel first, prove k > 1, then use that traction to attract brands.

**Sequencing:**
- Week 1-2: Fix sharing (broken invite link, SMS notifications on join, group text on join, editable list names, share sheet, invite tracking, fix wrong logos)
- Week 3-4: Penny Swaps v0 (hardcoded deals from Groflo), savings tracker, savings share card, referral incentive v0
- Week 5-8: Brand self-serve portal, deal targeting by category, brand dashboard, PennyWallet v0, Penny Guild framing, Tod/Scout outreach

**Key insight:** "We don't need all 3 loops. Loop 1 (C2C) at k>=1, then Loop 2 provides revenue. Loop 3 is emergent."

---

## Squad Beta: "Supply-Side Warm Start"

**Team:** PM Jordan (marketplace specialist), UX Ava (enterprise UX), Architect Raj (API-first), Staff Dev Chen (data pipelines)

**Thesis:** The fastest path to k > 1 is not more consumers — it's giving consumers a reason to evangelize. Deals ARE that reason. Get 20-30 brand deals live BEFORE scaling consumer acquisition.

**Sequencing:**
- Week 1-2: Fix sharing bugs AND build brand infrastructure in parallel (tables, brand signup page, deal creation flow, brand dashboard v0, manual deal seeding)
- Week 3-4: Penny Swaps live (swap rendering, conversational swaps, frequency caps), brand recruitment sprint (20-30 challenger brands, "Founding Brand" program)
- Week 5-8: Guild enrollment, PennyWallet, revenue attribution, payouts, guild leaderboard, brand dashboard v2, case study generator

**Key insight:** "You don't need 50K MAU to recruit challenger brands. Siete, GoodBowl will pilot at ANY scale because Meta CPA is $15-25."

---

## Squad Gamma: "The Guild Unlocks Everything"

**Team:** PM Priya (growth hacker), UX Felix (social product), Architect Tomoko (ML/AI), Staff Dev Diego (mobile-first fullstack)

**Thesis:** The product already works. The bottleneck is incentive to share. Guild (earn 30% of revenue) turns every user into a paid distribution channel, collapsing CAC to zero at scale.

**Sequencing:**
- Week 1-2: Fix sharing, add referral attribution tracking, deep linking
- Week 3-4: PennyWallet, Penny Swaps v0 (5-10 pilot brands), revenue attribution, fix brand logos, PWA improvements
- Week 5-8: Brand self-serve v0, share receipt/brag cards, swap recommendation engine v1, payout infrastructure, SMS re-engagement

**Key insight:** "Guild is the user acquisition engine. Swaps are the monetization engine. Build the growth engine first."

---

## Interview Findings

### Brand Manager Panel (6 interviews: P&G, Unilever, Siete/mid-size, DTC startup, regional, private label)

| Finding | Implication |
|---------|-------------|
| Intent targeting ("they have ketchup on their list") is the killer differentiator | Pitch is "target at point of decision," not "another ad channel" |
| Mid-size/DTC brands will pilot at ANY scale; enterprise needs 50K+ MAU | Start with challenger brands, not P&G |
| Current CPA: $2-25. $1-2/swap is very competitive | Room to raise pricing |
| Self-serve essential ("15 minutes or I'm out") | Simple portal, no contracts |
| First-mover FOMO is the strongest brand-to-brand viral trigger | "Your competitor is already on Penny" |
| Attribution data is the #1 ask | Dashboard with real conversion data |

### Shopper Panel (6 interviews: young couple, family of 5, single mom, empty nesters, health millennial, college student)

| Finding | Implication |
|---------|-------------|
| Household sharing is natural first share (utility-driven) | Loop 1 starts with cohabitants |
| High switching for commodity items, low for identity items | Target paper goods, basics, condiments first |
| $1 off minimum for commodity; $0.50 for budget shoppers | Set brand floor at $1/swap |
| Guild earning: $5/mo = nobody cares, $20-30/mo = changes behavior | Payouts must be meaningful |
| "MLM smell test" is real | Frame as "save together" not "recruit" |
| Proof of savings is #1 viral artifact | Build savings share card |
| Real cash, not points | PennyWallet must have real payout path |

---

## CEO Synthesis

**Verdict:** Beta wins the framework, Alpha wins the sequencing, Gamma wins the incentive layer.

**The synthesized thesis:** Fix sharing so it works. Seed deals manually so sharing has a reason. Measure whether people swap. Then build the infrastructure.

The critical insight: you do not need a brand portal to have brand deals. You need Andy on the phone with 10 brand managers and a database seed script.

---

## Key Metrics

| Milestone | Metric | Target | Kill Threshold |
|-----------|--------|--------|----------------|
| Week 2 | List sharing rate | 50% of users shared | < 25% |
| Week 2 | Invite→join conversion | >= 30% | < 15% |
| Week 4 | Swap acceptance rate | >= 8% | < 5% |
| Week 4 | Active brand deals | >= 10 | < 5 |
| Week 8 | k-factor | >= 0.7 | < 0.3 |
| Week 8 | Active brands | >= 30 | < 15 |
| Week 8 | Guild enrollment | >= 15% of users | < 5% |
| Week 8 | Monthly swap revenue | >= $500 | < $100 |
