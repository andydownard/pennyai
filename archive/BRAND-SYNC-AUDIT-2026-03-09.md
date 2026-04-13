# Brand Sync Audit — 2026-03-09

## PHASE 1: Technical Sync Audit Summary

**Audit Date:** 2026-03-08 11:44 AM
**Source Version:** 1.0.0
**Codebases Scanned:** App (114 files), Website (33 files)

### Total Issues: 11
- **Critical:** 0
- **Warnings:** 5
- **Info:** 6

### Critical Issues

None. No blocking issues found.

### Warnings (5)

1. **Market TAM in Winning Metrics** — Missing $12B TAM value in winning-metrics sheet. Suggestion: Add to metrics.
2. **TAM Inconsistency** — Deck says $12B, winning metrics missing. Cross-check and align.
3. **Off-Brand Green in App** — Found #2E7D32 (non-brand green). Suggestion: Replace with brand green #35644F.
4. **Unexpected Font (Roboto)** — App should use Inter, Avenir, or Avenir Next only. Found Roboto. Remove unexpected fonts.
5. **Referral System Not on Website** — App has referral system for growth, not mentioned on thrift.ly. Should add to marketing.

### Info (6)

1. **Monetization Example Missing** — $0.50 CPM missing from winning metrics.
2. **Warm Contacts Missing** — 20K+ warm charity contacts missing from metrics.
3. **AI Organization Value Prop** — "Penny organizes" not clearly highlighted in app copy.
4. **Family Sync Value Prop** — "Family shops together" missing or different from brand source.
5. **Saved Lists Not in Deck** — App has saved lists feature (weekly groceries), not mentioned in pitch deck.
6. **Multi-Auth Not in Deck** — App has SMS, Google OAuth, email/password auth, not in pitch deck.

---

## PHASE 2: Feature Coverage Scan

### Recent Changes (Last 7 Days)

**PennyAI:**
- Dark mode: full theme toggle with CSS custom properties
- Product feedback system: widget + GitHub integration
- Trip planner improvements: duration tracking, contextual discoverability
- First-swap celebration: confetti + special toast
- Learning loop enhancements (L1-L3, L5 complete)

**Rally:**
- Helper signals, portfolio, leaderboards (Sprint 14)
- Coach feedback UI, streaks, admin analytics
- Production hardening (backups, graceful shutdown, integrity checks)

**Thrift.ly Website:**
- New multi-page routing (Home, For Brands, Pricing, Campaign Signup, About)
- Stripe Elements integration for campaign signup
- GA4 tracking added

### Features in App NOT Yet Promoted on Website

| Feature | App Status | Website Status | Value Prop | Priority |
|---------|-----------|----------------|-----------|----------|
| **Dark mode** | Live (CSS custom props) | Not mentioned | "Shop late at night, easy on eyes" | Medium |
| **Trip planner** | Live with duration + discoverability | Not mentioned | "Plan store layout, shop faster" | Medium |
| **Product feedback widget** | Live (GitHub integration) | Not mentioned | "Help shape Penny" | Low |
| **Referral system** | Live, tracked in app | Not mentioned on website | "Grow by word-of-mouth" | High |
| **Saved lists** | Live (weekly staples, templates) | Mentioned briefly in features | "Templates for every occasion" | Low (already there) |
| **SMS/Email inbound** | Live (Twilio, SendGrid) | Mentioned vaguely | "Text Penny. She handles it." | Low (copy exists) |
| **Household sharing** | Live, full coordination | Not emphasized | "No more 'Did you get X?'" | Medium |
| **Brand loyalty learning** | Live per-member | Mentioned as "brand memory" | "Learns your favorites" | Low (already there) |

---

## PHASE 3: Brand Consistency Check

### Against penny-brand-source.json

**Tagline Match:** "The AI shopping list for families" ✓ Correct on website
**One-liner Match:** "Text Penny. She handles the rest." ✓ Correct
**Mission Match:** "Helping families eat better, waste less" ✓ Present

### Design Tokens

| Token | Brand | App | Website | Status |
|-------|-------|-----|---------|--------|
| Brand Green | #35644F | #2E7D32 found | #35644F | ⚠️ App has drift |
| Background | #F7F6F3 | Correct | Correct | ✓ |
| Font Family | Inter | Avenir (correct for mobile) | Inter | ✓ |
| Text Primary | #2A2A2A | Correct | Correct | ✓ |

### Tone & Language

**Website language check:** "Easy on eyes," "no more friction," "smart sorting" — all plain English, family-friendly. ✓ Good
**App language:** Some verbose copy in learning loops, but overall warm. Acceptable.
**Pitch deck:** More formal, investor-focused. Acceptable for audience.

---

## Actionable Recommendations

### For PennyAI App
1. **Fix green color drift** — Replace #2E7D32 with #35644F across all theme files.
2. **Remove Roboto font** — Audit and remove any Roboto references; stick to Avenir/Avenir Next.
3. **Preserve dark mode** — This is a feature, don't regress.

### For Thrift.ly Website
1. **Add referral system section** — "Earn $X when friends join. Your way to grow together."
2. **Highlight trip planner** — Under "Features" or new "For Shoppers" section.
3. **Add dark mode to marketing** — Small "Dark mode available" badge in features.
4. **Expand household sharing section** — Show coordination use cases (parent + kid, roommates, couples).

### For Pitch Deck (Winning Metrics & Deck)
1. **Add saved lists to deck** — "Reusable templates for weekly staples, holiday prep, kids' lunches."
2. **Cross-check TAM** — Ensure $12B appears in all surfaces (deck, metrics, brand source).
3. **Highlight multi-auth** — "Works on any phone, any OS, any carrier. No app download."
4. **Add monetization examples** — $0.50 CPM, warm contacts 20K+.

---

## Summary

**Brand alignment:** 85% strong. One design token drift (green), one font drift.
**Feature coverage:** 6 app features not yet marketed on website. Highest priority: referral system (growth lever).
**Next steps:** Ship marketing updates (Phase 6), fix design drifts (Phase 5), convene for final CEO approval.

