# Thrift.ly Marketing Proposals — 2026-03-09

**Prepared by:** Product Marketing Panel (VP PMM, CMO, Head of Product, CEO Andy)
**Date:** 2026-03-09
**Approval Status:** Pending CEO sign-off

---

## EXECUTIVE SUMMARY

### Brand Sync Audit Results
- **Issues:** 11 total (0 critical, 5 warnings, 6 info)
- **Key Gaps:** Referral system not marketed, dark mode not mentioned, trip planner not prominent
- **Brand Alignment:** 85% strong. Design token drifts in app (green, Roboto) should be fixed.

### Marketing Panel Verdict
**Tier 1 (Ship This Week):**
1. Referral system integration + website section
2. Dark mode badge in features
3. Improved household sharing copy (better conflict resolution messaging)

**Tier 2 (Ship Next 2 Weeks):**
4. Trip planner section (pending design asset)

**Hold:**
- Feedback widget (ops needs 1 more week of maturity)

---

## WEBSITE UPDATES (Tier 1 — Ship This Week)

### 1. ADD: "Share & Earn" Referral Section

**Location:** New page or section after "How It Works"
**Copy Focus:** Friends + Shared Learning

#### Headline
"Friends who shop together, stay organized together."

#### Body Copy
"Text Penny to a roommate, a partner, or a family member. You all see the same list, everyone adds items, and Penny keeps you in sync. Get rewarded when friends join."

#### CTA Buttons
- "Create invite link" (primary — opens modal)
- "See referral rewards" (secondary — links to FAQ)

#### Modal: Generate Invite
```
Your Referral Link:
[Link copied to clipboard]

Share it:
- Copy link
- Text to friend
- Email to family
- Share on social

Your rewards:
- For every friend who joins: [reward amount/description]
```

#### Design Notes
- Match app invite flow (same modal, same colors)
- Use brand green (#35644F)
- Keep copy warm and family-oriented
- On mobile: Stack buttons vertically

#### Implementation
- [ ] Add `/share` or `/referral` page to React Router
- [ ] Create ReferralModal component
- [ ] Add "Share & Earn" to main nav (between Pricing and For Brands)
- [ ] Create database migration for referral link generation (if not exists)

**Timeline:** 3 days (design + implementation)
**Owner:** Frontend + Marketing

---

### 2. UPDATE: Dark Mode Feature Badge

**Location:** Features section (Home page)
**Current Feature Card:** (Add dark mode)

#### Card Design
```
Icon: Moon icon (#35644F)
Headline: "Dark mode included"
Copy: "Shop late at night without straining your eyes. Available in the app."
```

#### Technical Note
- Icon: `<Moon />` (from icon library)
- Positioning: Last in features grid (after "Saved lists")
- Mobile: Full width on small screens

#### Implementation
- [ ] Add dark mode card to `src/pages/Home.tsx` features section
- [ ] Add moon icon to assets
- [ ] No backend changes

**Timeline:** 1 day
**Owner:** Frontend

---

### 3. UPDATE: Household Sharing Copy (Reframe)

**Location:** Features section (current "Shared lists" card)
**Change:** More concrete, less generic

#### Current Copy
"Share a list with anyone — roommates, family, a partner. Everyone adds items, everyone sees updates live."

#### New Copy
"No more 'Did you get X?' Claim items as you add them. Your roommate knows milk is covered. Your partner doesn't buy duplicates. One shared list, clear ownership, less chaos."

#### Why This Works
- Addresses real pain: duplicate buys, unclear ownership
- Shows conflict resolution
- Family-oriented tone
- Less about sharing, more about coordination

#### Implementation
- [ ] Update Home.tsx feature card copy
- [ ] No design changes (same card structure)
- [ ] Maybe add subtext example: "Sarah claimed milk. James claimed bread."

**Timeline:** 1 day (copy review + update)
**Owner:** Product Marketing

---

## WEBSITE UPDATES (Tier 2 — Ship Next 2 Weeks)

### 4. ADD: Trip Planner Section (Pending Design)

**Location:** New section after Features (or group with smart sorting)
**Copy Focus:** Time savings + Store efficiency

#### Headline
"Get in, get out. Fast."

#### Body Copy
"Penny knows the store layout. She shows you the best order to hit each aisle — dairy, then produce, then frozen. Instead of zigzagging for 30 minutes, you're done in 15."

#### Visual
- Animated GIF: Phone shows list → store map → optimized aisle walk
- Or: Static screenshot of trip planner UI from app

#### CTA
"See it in action" (links to app)

#### Design Needs
- [ ] GIF or screenshot of trip planner in app
- [ ] (Requested from Product team)

#### Implementation (Waiting on Design)
- [ ] Create TripPlanner section component
- [ ] Add to Home.tsx (after Features or integrated into Features grid)
- [ ] Link CTA to app download

**Timeline:** 5 days (pending design asset)
**Owner:** Frontend + Product (for asset)

---

## SUPPORTING COPY: FAQ & Legal

### FAQ Section: Referral Rewards

**Q: How much do I earn per referral?**
A: [Amount TBD by Andy] — We'll add this to the website once decided.

**Q: How do I claim my reward?**
A: Your reward automatically appears in your account. No signup needed.

**Q: Can I share the link with anyone?**
A: Yes! Penny works for families, roommates, partners — anyone who shops together.

---

## PITCH DECK UPDATES (Cross-Check with Brand Source)

### Missing Elements in Deck
1. **Saved Lists** — App feature not in deck. Suggest adding to "Product Capabilities" slide.
2. **Multi-Auth** — "Works on any phone, any OS, any carrier" — not emphasized in deck.
3. **Referral System** — "Viral growth through word-of-mouth" — not mentioned in deck.

### Recommended Deck Changes
- [ ] Add Saved Lists to "Why Penny" capabilities
- [ ] Add multi-auth to "Why Now" (SMS = zero friction)
- [ ] Add referral k-factor to "Growth Story" slide

**Timeline:** 1 week (post-CEO approval)
**Owner:** Andy (final review), Product Marketing

---

## BRAND SYNC FIXES (Required)

### For App Team
1. **Green Color (#35644F):** Replace all #2E7D32 occurrences
2. **Roboto Font:** Remove from app; confirm only Avenir/Avenir Next

### For Website
- All colors correct ✓
- All fonts correct (Inter) ✓
- Brand copy correct ✓

### For Pitch Deck
- Verify TAM ($12B) appears in all surfaces
- Verify CPM ($0.50) in winning metrics

**Timeline:** 1 week
**Owner:** Engineering (app), Product (deck)

---

## CONTENT UPDATES: What to REMOVE

### Not Ready Yet
- Feedback widget (ops unstable) — don't mention
- Confetti celebration (novelty feature) — don't mention
- Learning loops (internal mechanics) — don't mention

### Keep Stable
- SMS/Email input
- Household sharing
- Saved lists
- Brand memory
- Smart sorting

---

## MEASUREMENT & TRACKING

### Metrics to Watch (After Launch)

**For Referral Section:**
- Click-through to "Create invite link"
- Referral code redemptions
- CAC improvement (organic vs. paid)

**For Website Traffic:**
- Bounce rate before/after updates
- Time on features section
- Clicks to app download

**For Dark Mode Mention:**
- Does it drive app reviews mentioning dark mode?

---

## TIMELINE

| Task | Owner | Timeline | Dependencies |
|------|-------|----------|--------------|
| Referral section (design + code) | Frontend | 3 days | Copy review ✓ |
| Dark mode badge | Frontend | 1 day | Icon assets |
| Household sharing copy update | PMM | 1 day | Copy review (CEO) |
| Trip planner section | Frontend | 5 days | Design asset |
| App color/font fixes | Engineering | 5 days | Audit |
| Pitch deck updates | PMM + Andy | 1 week | CEO approval |

---

## APPROVAL CHECKLIST

- [ ] **CEO (Andy)** approves referral messaging and rewards amount
- [ ] **CEO (Andy)** approves household sharing copy reframe
- [ ] **CEO (Andy)** approves dark mode positioning (feature vs. hero)
- [ ] **CMO** confirms brand alignment (colors, fonts, tone)
- [ ] **Head of Product** confirms feature stability (referral system ready)
- [ ] **VP PMM** confirms copy clarity and compelling messaging

---

## NEXT PHASE: Execution

Once approved, assignments:
1. **Frontend:** Referral section + dark mode badge + trip planner (pending asset)
2. **Engineering:** Color/font fixes in app
3. **Marketing:** Copy review + FAQ + pitch deck updates
4. **Product:** Deliver trip planner design asset

---

## APPENDIX: Detailed Copy Drafts

### Referral Section (Full)

**Hero:**
"Friends who shop together, stay organized together."

**Subheader:**
"Extend Penny to your household. Everyone can add items, everyone sees updates in real time. And you'll both earn rewards along the way."

**Three-Step Process:**
1. "Generate your invite link" — One tap, one link.
2. "Share with friends or family" — Text, email, social.
3. "Both of you get rewarded" — Rewards appear automatically when they join.

**CTA Buttons:**
- Primary: "Get my invite link"
- Secondary: "Learn about rewards"

**Fine Print:**
"No purchase required. Free for all Thriftly users."

---

### Dark Mode Note (Short)

**Copy:** "Dark mode available in the app. Perfect for late-night planning or early morning walks."

**Icon:** Moon (SVG)

**Placement:** Features grid, last card

---

### Household Sharing (Updated)

**Headline:** "No more 'Did you get X?'"

**Copy:** "Claim items as you add them. Your roommate knows milk is covered. Your partner doesn't buy duplicates. One shared list, everyone knows what's done."

**Example Conversation (Optional):**
> **Sarah:** "Need milk?"
> **Jamie:** "Already claimed it."
> **Result:** No duplicate buy. Coordination. Harmony. ✓

---

## SIGN-OFF

**Prepared by:** Product Marketing Panel
**Date:** 2026-03-09
**Status:** Pending CEO & CMO approval

**Awaiting:**
- Andy's approval on referral rewards amount
- Andy's approval on messaging priority (Tier 1 vs. Tier 2)
- CMO final brand check

