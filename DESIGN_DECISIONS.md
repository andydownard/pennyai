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
