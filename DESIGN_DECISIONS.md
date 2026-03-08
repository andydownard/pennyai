# Design Decisions

Documented product decisions that inform implementation. These override default assumptions.

## SMS Opt-In Checkbox (Phone Login Screen)

**Decision:** The phone login screen includes a checkbox labeled "I want to SMS with Penny" that must be checked before the user can submit their phone number. If unchecked, the Continue button is disabled (opacity 0.35, not clickable).

**Rationale:** Users must explicitly consent to SMS communication before we send any texts. This is both a legal requirement (TCPA compliance) and a trust signal — Penny never texts without permission.

**Implementation:**
- `PhoneScreen.tsx`: Checkbox controls `smsOptIn` state
- Button disabled when `!isValid || !smsOptIn || loading`
- Auto-send on 10 digits only fires when `smsOptIn` is true
- Default: `smsOptIn = true` (pre-checked for conversion, user can uncheck)
- The Penny Settings screen has a separate "I want to SMS with Penny" toggle for post-login SMS preferences

## Penny Over Toasts — Communication Philosophy

**Decision:** Wherever possible, Penny communicates feedback directly through her text area in PennyBar — not through toast notifications. Toasts are a last resort, not the default.

**Rationale:** Penny is the personality of the app. When the user does something (adds items, clears the list, accepts a swap), Penny should acknowledge it conversationally in her text area. Generic toasts feel impersonal, disrupt layout, and compete with Penny's voice. The app should feel like a conversation with Penny, not a dashboard firing notifications.

**Rules:**
- **Prefer Penny text:** Use `typePennyHome()` or `homePennyText` to have Penny speak feedback ("Got it — added 3 items", "List cleared. Want to start fresh?")
- **Avoid toasts for success states:** Penny should celebrate or confirm, not a floating card
- **Toasts only for:** errors that Penny can't contextually address (network failures, auth expiry), or very brief confirmations on non-home screens where PennyBar isn't visible
- **Never show both:** If Penny is already speaking about an action, do not also fire a toast for the same event
- **Toast styling:** When toasts are unavoidable, use the lightweight top card (not a bottom bar that overlaps Penny)
