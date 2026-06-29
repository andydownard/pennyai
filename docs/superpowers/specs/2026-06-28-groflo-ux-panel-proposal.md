# Groflo / Penny — UX Panel Proposal

**Date:** 2026-06-28
**Panel:** Google (clarity / speed / a11y), Apple (craft / delight / restraint), Shopify (conversion / trust / revenue)
**Status:** PROPOSAL for Andy's review. No implementation in the refactor program. Per your governance, convene the money and design panels before building any user-facing change.

Every suggestion is grounded in the real frontend and tagged `[impact / effort]`. Each respects the protected surfaces: Penny's behaviors (typewriter, chime-ins, idle reminders), the vocabulary table, the auth methods, and the shipping gates (90s-to-first-item, the "Did you get milk?" household narrative, the data flywheel). None weakens a disaster rule.

---

## Top picks (highest impact, lowest risk)

1. **Make the savings figure the hero.** It renders at 15px and dims to opacity 0.6 once "saved" — you are dimming your proudest number. Promote it to a display weight; quiet the caption beneath. Same data, same tap-to-earnings. `App.tsx:2787-2824`. *(Apple, high / S)*
2. **Name the 30% share.** Replace "I share the savings" with "Brands pay to reach you. 30% of what they pay comes back to you — real cash." Closes a `MONEY`-rule transparency gap and is Penny's most credible trust signal. Copy-only; confirm exact wording with the money panel. `EarningsScreen.tsx:186`, `SwapCard.tsx`. *(Shopify, high / S)*
3. **Show the item arriving.** Today sending "milk" greys the whole list with a blocking overlay until Claude returns — reads as frozen. Insert a pending shimmer row in the right aisle, keep the list scrollable, reconcile on the server result. Render-only; server stays the source of truth. `App.tsx:3074`, `useChatState.ts:260`. *(Google, high / M)*
4. **Collapse the banner stack.** On iPhone SE, offline + rebrand + install + restock + trip banners can bury the list before one item shows. Show one ambient prompt by priority; keep every prompt's dismissal persistence. `App.tsx:2890-3050`. *(Apple, high / M)*
5. **Let people type first.** Drop the blocking 3-step onboarding modal (the empty state already teaches the same thing). Move the household-story step into a Penny chime-in after the 3rd item. Preserve `onboarding_complete` persistence; don't double-prompt the invite. `OnboardingOverlay.tsx`, `App.tsx:3554`. *(Apple, high / M)*
6. **Close the earn loop.** Accepting a swap bumps the counter, but money is real only after the receipt step. Add a standing "You have $X pending — snap your receipt" nudge on surfaces users actually see. In-app only; persist dismissal server-side, not in React. `useSwapState.ts:129`, `EarningsScreen.tsx`. *(Shopify, high / M)*

---

## Google — clarity, speed, accessibility

- **Show the item arriving optimistically; stop dimming the list during parse.** `[high / M]` — `App.tsx:3074`, `useChatState.ts:260`. *(SERVER-sensitive: render-only, never trusted client-side.)*
- **Default departments to expanded on cold load.** Empty `expandedDepts` Set means a returning shopper sees only collapsed aisles. Expand when the list is modest (<~25 items); add "Expand all / Collapse all" for long lists. `[high / S]` — `useListState.ts:77`, `App.tsx:3142`.
- **Raise the department header to a 44px tap target.** One of the most-tapped controls renders ~24px; fails WCAG 2.5.5 and your own design spec. Apply the same floor to banner dismiss "×" buttons when touched. `[medium / S]` — `App.tsx:3699`.
- **Add progressive search/filter to the list.** No way to find an item except expanding accordions. Also serves the household story ("is milk already on here?"). Client-side filter; must not displace the Penny input or shrink the savings badge. `[medium / M]` — `App.tsx:2744`.
- **Bump 10px microcopy to 12px.** Item timestamps and per-aisle counts render at 0.625rem — below a comfortable mobile floor. Promote to `FONT_2XS`. `[low / S]` — `ItemRow.tsx:239`, `App.tsx:3722`.
- **Surface who added each item; make the claim badge legible.** The "Did you get milk?" story is carried only by Penny's text; the row never shows `added_by_name` and shows a claim as a single initial. Add a quiet "added by [name]" for household lists and an avatar + accessible name (data already on the item). Hide for solo lists. `[medium / M]` — `ItemRow.tsx:244`.

---

## Apple — craft, delight, restraint

- **Promote the savings figure to a confident key number; stop dimming "saved."** `[high / S]` — `App.tsx:2787`.
- **Collapse the above-the-fold banner stack to one prioritized slot.** Priority: offline > trip > restock > rebrand > install. Must not drop any prompt or weaken dismissal persistence; SMS consent stays a modal. `[high / M]` — `App.tsx:2890`.
- **Let users type immediately; turn onboarding into non-blocking Penny coachmarks.** Keep the household-narrative step as a chime-in after item 3. `[high / M]` — `OnboardingOverlay.tsx`, `App.tsx:3554`.
- **Wire Penny's existing happy/excited moods to the real delight beats.** `PennyAvatar` already ships reduced-motion-aware moods, but the flow only ever shows "thinking." Pass `mood='happy'` on add, `mood='excited'` on list-done/swap-accept. Additive; preserves all behaviors. `[medium / M]` — `PennyAvatar.tsx`, `PennyBar.tsx:207`.
- **Add a commit-threshold haptic to swipe-to-check / swipe-to-delete.** The gesture crosses its commit point with no tactile feedback. `haptic.tap/medium/heavy` already exist — wiring, not invention. `[medium / S]` — `ItemRow.tsx:121`.
- **Confirm the auto-add ring with a gentle haptic on completion.** The 1.5s auto-submit ring fires `onSend()` silently — the most magical moment has no sensory beat. `[medium / S]` — `PennyBar.tsx:169`.
- **Make the earnings Total the hero; demote Pending/Paid.** Three equal cards lead the eye nowhere; use one dominant figure with quiet context (Fitness/Wallet pattern). Layout/typography only. `[medium / S]` — `EarningsScreen.tsx:140`.
- **Turn the first-run example chips into inviting, 44px tap targets.** "milk eggs bread" chips read as disabled (`var(--bg)`, tertiary text, ~28px). Brand-tint them, 44px, Penny-voiced ("milk, eggs, bread"). Keep them honest seed text. `[medium / S]` — `App.tsx:3118`.

---

## Shopify — conversion, trust, revenue

- **Name the 30% community revenue share on Earnings and swap surfaces.** `[high / S]` — `EarningsScreen.tsx:186`, `SwapCard.tsx`. *(MONEY: state it accurately, never imply it can change.)*
- **Close the earn loop with a persistent "$X pending — snap your receipt" nudge.** `[high / M]` — `useSwapState.ts:129`, `EarningsScreen.tsx`, `DoneSummaryModal.tsx`. *(In-app only; server-side dismissal.)*
- **Make "Share & Earn" pay a concrete, real reward instead of vague "points."** ShareScreen promises points with no balance, value, or redemption, disconnected from the real-money Earnings screen. Tie the reward to a stated value that lands in the same balance; show progress. Only show a value the backend actually grants. Keep "referral" out of UI copy. `[high / M]` — `ShareScreen.tsx:103`, `EarningsScreen.tsx`.
- **Separate "cheaper by $X" from "earn $Y back" on the SwapCard CTA.** "Try it — save $X" is shelf-price delta; the cash is the later rebate. Split them to set honest expectations. Needs the real rebate on the payload (additive API field); if absent, show only the save line. `[high / M]` — `SwapCard.tsx:329`.
- **Offer "Start from your usual" on the empty list for returning users.** `DoneSummaryModal` silently builds a "My Groceries" master list, but the empty list never offers it. One-tap reorder cuts time-to-first-item and feeds the flywheel. Reuse the saved-list load; confirm before overwriting; never auto-load. Keep "saved list" wording. `[high / M]` — `App.tsx:1609`, `SavedListsScreen.tsx`, `DoneSummaryModal.tsx:94`.
- **Make the household "shopping now" trip banner actionable.** Today it is passive ("Someone — Shopping now!"). Let members tap to add/claim or ping the shopper — the canonical "Did you get milk?" retention hook. Build additively; preserve TENANT scoping; pin with a household-isolation test. `[medium / L]` — `App.tsx:3001`.
- **Keep "savings" and "cash earned" consistent across milestone and earnings copy.** `MilestoneCard` blends shelf savings with cash rebates ("Brands are literally paying you to shop"); Earnings pays real Venmo cash. Use one clear frame. Copy only. `[medium / S]` — `MilestoneCard.tsx:75`.
- **Add honest, data-gated trust signals to the SwapCard.** A real "receipt-verified" badge or honest acceptance count lifts accept rate. Show only when the backend supplies real numbers; never fabricate; don't crowd the Brand Partner disclosure. `[medium / S]` — `SwapCard.tsx:142`.

---

## Cross-cutting themes the panel agreed on

- **The money is under-told.** Savings, the 30% share, and earnings — the app's whole differentiator — are visually quiet on the exact screens that drive conversion and retention.
- **The 90-second runway is cluttered.** A blocking onboarding modal plus a banner stack fight the first item on a 375px screen.
- **The earn loop leaks.** Accepts feel like money but aren't until the receipt step, which is easy to miss.
- **Penny is the product but her face is flat** at the moments that should feel alive.
- **Strong a11y baseline already exists** (skip link, aria-live, progressbar) — the fixes are targeted, not a rebuild.
