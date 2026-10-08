# Groflo — Design Spec

## Onboarding Rules (Gate 1 Detail)

New user must add first item within 90 seconds with zero clarifying questions.

- **Assume reasonable defaults** — Penny infers quantity (milk → 1 gallon) and selects a popular brand. Do NOT ask clarifying questions.
- **Brand defaults from global popularity** — Use aggregate app-wide preferences (not personal/household tracking) for new users. Label clearly: "I picked [brand] based on popular choices."
- **Brand defaults respect dietary attributes** — If user has ≥3 items of same dietary attribute (organic, nut-free, vegan), prioritize matching attributes over popularity.
- **Brand correction is 1 tap** — ActionSheet with top 5 alternatives, each showing logo + name + dietary attributes. On selection: close sheet, update item, show confirmation toast ("✓ Changed to Organic Valley"), auto-dismiss 3s. No secondary confirmation modal.
- **Household invite after 3rd item** — Soft language: "Want to invite your family to this list?" Dismissal persists in localStorage — do NOT re-trigger in same or future sessions.
- **Popular brand fallback** — If Claude cannot infer brand, use global popularity or generic option ("Best Value", "Kroger brand"). No personal tracking of similar households.

## UI/UX Rules

### Penny Settings Page Layout (top to bottom)

1. Contact section — Penny avatar + name, icon-only action buttons (text, email, add to contacts)
2. Divider (1px separator)
3. "I want to SMS with Penny" toggle (only if user has phone)
4. "Learn from my choices" toggle
5. Convenience/savings slider (only when autotune OFF, at bottom)

Autotune ON → slider hidden. Autotune OFF → slider appears with CSS transition (max-height 0.3s + opacity 0.25s). Do NOT use display:none toggling.

### Auth Screens

- There is one sign-in screen (`LoginScreen.tsx`): PennyAvatar, Penny's text, "Continue with Rally", legal links. It collects no credentials; Rally (Clerk) hosts them. Phone-code, Google, email/password and magic-link screens were removed 2026-10-03. SMS consent is not asked on this screen.
- Copy never says SSO, bot, AI, or assistant.

### Touch Targets & Contact Actions

- Minimum 44x44px touch targets, 16px gap minimum between icons
- Use `sms:` link for Penny's phone (not `tel:`)
- Icon-only on mobile for text/email/add-to-contacts — no displayed numbers or addresses

### Navigation Menu Order

1. Shopping List → 2. My Lists → 3. Share & Earn → 4. Penny → ── divider ── → 5. Profile → 6. Dashboard (admin only, always last)
