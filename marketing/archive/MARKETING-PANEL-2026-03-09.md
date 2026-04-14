# Product Marketing Panel — 2026-03-09

## Panel Composition
- **VP Product Marketing** — Feature prioritization, messaging, website placement
- **CMO** — Brand consistency, tone, design tokens
- **Head of Product** — Technical readiness, stability, support load
- **CEO (Andy Downard)** — Final decision on website promotions, copy, strategic fit

---

## Briefing: Recent Activity

### App (PennyAI) — Last 7 Days
- **Dark mode** shipped: Full CSS custom properties, system preference detection, toggle in profile
- **Product feedback widget** launched: GitHub integration for triage, admin dashboard tab for feedback analysis
- **Trip planner** enhancements: Duration tracking added, contextual "Plan a trip" nudge in list view
- **Brand system** maturing: Logo caching (3-layer), brand choices per household

### Website (Thrift.ly) — Last 7 Days
- Converted to multi-page SPA (Home, For Brands, Pricing, Campaign Signup, About)
- Stripe Elements integrated for self-serve campaign signup
- GA4 tracking live — beginning to see funnel data

### Pitch Deck & Brand
- 5 warnings in sync audit (color drift, font drift, TAM inconsistency)
- 6 info-level gaps (saved lists, multi-auth, referral system not in marketing)

---

## GAPS EVALUATION

### 1. REFERRAL SYSTEM

**What exists:** App has full referral tracking, rewards, and invite links. Not visible on website.

**VP Product Marketing Reaction:**
> "This is a growth lever. Referral k-factor > 1 by design. We're sitting on a viral mechanic and not talking about it. Word-of-mouth is our strongest CAC=0 channel. I'd put this above dark mode."

**Value Prop:** "Friends who text Penny together stay organized together. Earn [reward] when friends join."
**Placement:** New "Share & Earn" section after "How It Works". CTA: "Generate invite link" (same as app).
**Priority:** **HIGH** — This is organic growth infrastructure.

---

### 2. DARK MODE

**What exists:** Complete dark mode in app with toggle. Shipped 3 days ago.

**VP Product Marketing Reaction:**
> "Late-night shopping is real. Parents checking lists after kids' bedtime. But this is a feature detail, not a headline value prop. It's table stakes now — every app has dark mode."

**Value Prop:** "Use Penny anytime — day or night. Easy on your eyes."
**Placement:** Small badge in Features section. Not a hero section feature.
**Priority:** **MEDIUM** — Nice-to-have, not differentiator.

---

### 3. TRIP PLANNER

**What exists:** Store-aware trip planning with duration estimates and contextual suggestions.

**VP Product Marketing Reaction:**
> "This is underrated. 'Spend 15 min at Whole Foods instead of 30' is concrete value. Parents with tight schedules will love this. But we need to frame it around time/efficiency, not store planning."

**Value Prop:** "Get in, get out. Penny tells you the best order to hit each aisle. Save 15 minutes per trip."
**Placement:** Under Features, group with "Smart sorting". Could have a mini demo (animated aisle walk).
**Priority:** **MEDIUM** — Good detail, not hero.

---

### 4. HOUSEHOLD SHARING

**What exists:** Full household coordination (invite, leave, view members, shared list, claim items).
**Current Marketing:** "Share your list with anyone — roommates, family, a partner. Everyone sees live updates live."

**VP Product Marketing Reaction:**
> "This is vague. We're not showing the coordination value. 'Did you get milk?' 'I claimed it' — that's the magic. And we don't talk about the conflict reduction: one shared list = no duplicate buys. Let's make this more concrete."

**Value Prop:** "Stop asking 'Did you get X?' Claim items so there's no duplication. Everyone knows what's covered."
**Placement:** Keep in Features, but add example conversation in a case study or FAQ.
**Priority:** **MEDIUM** — Already present, just needs better copy.

---

## CMO BRAND CHECK

**Against penny-brand-source.json:**

1. **Tagline:** "The AI shopping list for families" ✓ Present
2. **One-liner:** "Text Penny. She handles the rest." ✓ Present
3. **Mission:** "Helping families eat better, waste less" ✓ Present
4. **Tone:** "Warm, helpful, family-oriented" — Website achieves this. Good.
5. **Design Tokens:**
   - Brand green (#35644F): ✓ Website correct, **app has #2E7D32 drift**
   - Background (#F7F6F3): ✓ Correct
   - Font (Inter on web, Avenir on app): ✓ Correct
   - **Warning:** App has Roboto font found in audit. Should be removed.

**CMO Verdict:** "Website is on brand. App needs minor fixes (green + Roboto). Pitch deck could be warmer — it reads investor-y, which is fine, but make sure marketing copy stays warm."

---

## HEAD OF PRODUCT ASSESSMENT

| Feature | Stable? | Known Issues? | Support Burden? | Ready to Market? |
|---------|---------|--------------|-----------------|------------------|
| **Dark mode** | Yes | None known | Low (user toggle) | **YES** |
| **Trip planner** | Yes | Occasional location service timeout | Low | **YES** |
| **Referral system** | Yes | Rate limit tested | Very low (automated) | **YES** |
| **Household sharing** | Yes | Edge case: leaving household mid-shop | Medium (support tickets) | **YES** — but set expectations |
| **Feedback widget** | Beta | Early triage, GitHub sync flaky 1x | Medium (need ops) | **CAUTION** — hold for 1 week |

**Head of Product:** "Everything is ready except the feedback widget. That needs one more week of ops polish. Dark mode and trip planner can ship this week. Referral system is solid — it's built on the same foundation as the household invite system, which is battle-tested."

---

## CEO DECISION: WHAT TO PROMOTE THIS WEEK

### Tier 1: Ship This Week ✓

**1. Referral System**
- **Copy:** "Friends who shop with Penny stay organized together. [Generate invite link] — earn rewards when friends join."
- **Placement:** New "Share & Earn" page (or section in For Brands page as teaser)
- **Implementation:** Add invite link generator to website, track referral codes in DB
- **Design:** Match app invite flow (simple modal, copy link, share button)

**2. Dark Mode**
- **Copy:** "Use Penny anytime — day or night. Dark mode available in app."
- **Placement:** Features section, small badge under main features grid
- **Implementation:** One-line note, no heavy lifting

### Tier 2: Ship Next 2 Weeks (pending product confirmation)

**3. Trip Planner**
- **Copy:** "Penny shows you the best aisle order. Save 15 minutes per trip."
- **Placement:** Features section, group with "Smart sorting"
- **Implementation:** Add mini gif (fridge → list → store path)

### Tier 3: Hold — Pending Product

**4. Feedback Widget**
- **Status:** Beta in app, ops needs 1 more week
- **Hold:** Don't market until feedback widget is production-ready

### Tier 4: Polish Copy (No Implementation)

**5. Household Sharing**
- **Current Copy:** "Share your list with anyone — roommates, family, a partner. Everyone sees live updates."
- **Improved Copy:** "Stop asking 'Did you get X?' Claim items so there's no duplication. Everyone knows what's covered."
- **Implementation:** Just update website copy (5 min), no shipping needed

---

## Copy Drafts (Plain English, Family Tone)

### Referral System Section
**Headline:** "Friends who shop together, stay organized together."
**Body:** "Text Penny to a roommate, a partner, or a family member. You all see the same list, everyone adds items, and Penny keeps you in sync. Get rewarded when friends join."
**CTA:** "Create invite link"

### Dark Mode Badge
**Copy:** "Dark mode available. Use Penny late at night without straining your eyes."

### Trip Planner Section
**Headline:** "Get in, get out. Fast."
**Body:** "Penny knows the store layout. She shows you the best order to hit each aisle — dairy, then produce, then frozen. Instead of zigzagging for 30 minutes, you're done in 15."

### Household Sharing (Updated)
**Headline:** "No more 'Did you get X?'"
**Body:** "Claim items as you add them. Your roommate knows milk is covered. Your partner doesn't buy duplicates. One shared list, clear ownership, less chaos."

---

## Strategic Questions for CEO

1. **Referral positioning:** Is this the main organic growth story for Tier 1 (after MVP)? Should it be more prominent than we currently show?

2. **Dark mode narrative:** Is this meaningful to our target (busy families with kids), or is it table stakes? Should we de-emphasize it on the marketing site?

3. **Trip planner competitive moat:** Store-aware planning is a data advantage. Should this be part of the pitch deck as an AI moat story?

4. **Feedback loop narrative:** Once ops is solid, how do we talk about learning from customers? "Help shape Penny" — is that part of the brand story?

---

## CEO (Andy) Final Call

*[Awaiting Andy's input on prioritization and copy sign-off. This panel recommends:]*

1. **Ship Tier 1 this week:** Referral system + dark mode badge + improved household sharing copy.
2. **Hold Tier 2:** Trip planner (need design asset), Feedback widget (ops maturity).
3. **Update pitch deck:** Add saved lists + multi-auth to deck's "Why Penny" section if positioning multi-auth as strength.
4. **Fix design drifts ASAP:** Green color (#35644F) and remove Roboto from app.

