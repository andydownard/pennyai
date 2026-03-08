# Thriftly (PennyAI)

Thriftly is an AI-powered grocery shopping list app with a personality-driven assistant named Penny, household sharing, and multi-channel input (web, SMS, email).

## Architecture

- **Backend**: Go, split across 14 files (`server.go`, `db.go`, `models.go`, `penny.go`, `handlers_*.go`, `session.go`, `utils.go`), SQLite (WAL mode), deployed on Railway
- **Frontend**: React/TypeScript, `App.tsx` + extracted components (`PennyAvatar`, `ActionSheet`, `ConfirmModal`, `FloatingLabelField`, etc.), Vite build
- **Auth**: Multi-auth (phone/SMS via Twilio, Google OAuth, email/password with bcrypt, magic link)
- **Database**: SQLite with inline migrations in `initDB()`

## Notification & Communication Safety — HARD RULE

**Duplicate notifications and inadvertent outbound messages must NEVER happen.** This is a trust-destroying, potentially app-deleting event. The bar for any code that sends SMS, emails, or triggers user-visible prompts (NPS, modals, push) is significantly higher than normal code.

**Rules:**
1. **No duplicate sends.** Every outbound SMS, email, or notification must have a dedup mechanism (idempotency key, "already sent" flag, or equivalent). Test the dedup path explicitly.
2. **No inadvertent sends.** Code that triggers Twilio SMS, SendGrid email, or any external communication must be gated behind explicit user action or a well-tested automated trigger. Never send as a side effect of a refactor, migration, or data backfill.
3. **No re-prompting dismissed UI.** Any dismissible prompt (NPS survey, onboarding modal, feature announcement) must persist its dismissed state. Dismissal must survive page reload and navigation within the same session at minimum. Never rely solely on React state for "has the user seen this" — use sessionStorage/localStorage or a server-side flag.
4. **Rate limit all outbound channels.** SMS and email sends must have server-side rate limits that cannot be bypassed by client-side bugs (rapid polling, useEffect loops, retry storms).
5. **Test the negative path.** When writing tests for notification/prompt features, always include: "after dismiss, does NOT show again", "after submit, does NOT send again", "rapid calls do NOT produce duplicates."

**Before shipping any change to SMS, email, NPS, or notification code:** manually trace every code path that could trigger an outbound message or user prompt. If you can't prove it won't duplicate or fire inadvertently, add a guard.

## Refactoring Rules

### What IS allowed

- Splitting `main.go` into multiple Go files/packages
- Splitting `App.tsx` into multiple React components/files
- Extracting shared styles into CSS modules or a styles file
- Renaming internal functions (NOT endpoint paths)
- Improving error handling patterns
- Adding types/interfaces
- Reducing code duplication via shared utilities
- Improving test coverage
- Reorganizing file/folder structure

### What is NOT allowed

- **Deleting items from ROADMAP.md** — always ask the founder before removing any roadmap item. Move deprioritized items to "Nice-to-Have" or "Not In Scope" instead.
- Removing any HTTP endpoint or changing its path/method/request/response shape
- Removing any frontend screen or its functionality
- Dropping any database table or column
- Removing any of Penny's personality behaviors (typewriter effect, chime-ins, idle reminders, quotes)
- Weakening session security (HttpOnly cookies, 256-bit tokens, 30-day expiry)
- Simplifying away the 3-layer brand logo caching system
- Removing any auth method (phone/SMS, Google OAuth, email/password)
- Removing household sharing features (invite, join, leave, view members)
- Removing email or SMS inbound webhook processing
- Removing the admin dashboard or analytics
- Removing NPS survey functionality
- Removing referral system
- Removing saved lists functionality
- Removing shopping trips functionality
- Removing address autocomplete (Google Places)

## Must-Have Features

### Authentication
- Phone/SMS login (send code, verify code via Twilio)
- Google OAuth login
- Email/password registration and login
- Password reset via email (SendGrid)
- Session management (HttpOnly, 256-bit tokens, 30-day expiry)
- Auto-merge accounts by email (`resolveOrCreateUser()`)

### Penny AI Assistant
- Typewriter text animation effect
- Chime-in suggestions on the shopping list
- Idle reminders after inactivity
- Penny quotes (`/api/penny/quote`)
- Natural language item parsing (`/api/list/parse`)
- Greeting on empty list
- Nudge after 5min idle

### Shopping List
- Add, edit, delete, toggle (check/uncheck) items
- Mark items as urgent
- Claim items (for household coordination)
- Clear checked items with undo/restore
- Item modification history
- Brand resolution and logos

### Saved Lists
- Create, rename, delete saved lists
- Load saved list items into active list (via upsertListItem for dupe prevention)
- **Saved lists are NOT displayed in the Profile UI** — they are only accessible via Penny chat
- Penny handles: "load my weekly groceries", "what saved lists do I have", etc.
- When user asks for a specific list, Penny auto-loads it; when they ask generally, Penny shows buttons for each list

### Household Sharing
- Create/join households via invite codes
- Leave household
- View household members
- Shared list across household

### Shopping Trips
- Create and delete shopping trips
- Store-based trip planning

### Multi-Channel Input
- SMS inbound webhook (Twilio) — parse and add items via text
- Email inbound webhook (SendGrid) — parse and add items via email
- Penny contact card download (`/api/penny.vcf`)

### Brand System
- Brand logo lookup and caching (3-layer: `brand_logos`, `brands`, `brand_owners`)
- Brand preferences and choices
- Logo proxy endpoint

### Address Autocomplete
- Google Places autocomplete and detail lookup

### Referrals
- Referral code generation and redemption

### NPS Survey
- Check eligibility and submit NPS responses

### Admin
- Admin dashboard with user/usage stats
- Analytics endpoint

### Legal
- Privacy policy page
- Terms of service page

## Frontend Screens (all must remain)

1. `phone` — Phone number entry
2. `code` — SMS verification code
3. `home` — Main shopping list
4. `penny` — Text Penny / AI assistant
5. `profile` — User profile
6. `settings-share` — Household sharing settings
7. `admin` — Admin dashboard
8. `email-login` — Email sign-in
9. `email-register` — Email registration
10. `forgot-password` — Password recovery
11. `reset-password` — Password reset with token
12. `shared-list` — Read-only shared list view (no auth required)

## API Contract (all endpoints must remain with same paths and JSON shapes)

### Health & Config
- `GET /health`
- `GET /api/config`

### Authentication
- `POST /auth/send-code`
- `POST /auth/verify-code`
- `POST /auth/google`
- `POST /auth/email/register`
- `POST /auth/email/login`
- `POST /auth/email/forgot`
- `POST /auth/email/reset`
- `POST /auth/logout`
- `GET /auth/me`
- `POST /api/auth/link`

### List Management
- `GET /api/list`
- `POST /api/list`
- `PATCH /api/list/{id}/toggle`
- `PATCH /api/list/{id}/urgent`
- `PATCH /api/list/{id}/claim`
- `PATCH /api/list/{id}` (edit)
- `DELETE /api/list/{id}`
- `POST /api/list/clear`
- `POST /api/list/restore`
- `POST /api/list/parse`
- `GET /api/list/{id}/history`
- `POST /api/list/brand-resolve`
- `POST /api/list/share`

### Shared List
- `GET /api/shared/{code}`

### Saved Lists
- `GET /api/saved-lists`
- `POST /api/saved-lists`
- `PUT /api/saved-lists/{id}`
- `DELETE /api/saved-lists/{id}`
- `POST /api/saved-lists/{id}/load`

### Places
- `GET /api/places/autocomplete`
- `GET /api/places/detail`

### Settings & Household
- `GET /api/settings`
- `PATCH /api/settings`
- `POST /api/settings/invite`
- `POST /api/settings/leave-household`

### Brands
- `GET /api/brands`
- `POST /api/logo`

### Trips
- `GET /api/trips`
- `POST /api/trips`
- `DELETE /api/trips`

### Sharing & Contact
- `GET /api/penny.vcf`
- `POST /api/join`

### Webhooks
- `POST /api/email` (SendGrid inbound)
- `POST /api/sms` (Twilio inbound)

### Referrals
- `GET /api/referral`
- `POST /api/referral/redeem`
- `POST /api/referral/claim`

### NPS
- `GET /api/nps/check`
- `POST /api/nps`

### Penny
- `GET /api/penny/quote`

### Admin
- `GET /api/admin/dashboard`
- `GET /api/admin/analytics`
- `GET /api/admin/payouts`
- `POST /api/admin/payouts`

### Legal
- `GET /privacy`
- `GET /terms`

## Database Invariants (tables that must NOT be dropped)

1. `users`
2. `list_items`
3. `saved_lists`
4. `saved_list_items`
5. `households`
6. `household_members`
7. `invites`
8. `item_modifications`
9. `referrals`
10. `redemptions`
11. `nps_responses`
12. `sessions`
13. `auth_methods`
14. `password_resets`
15. `brand_owners`
16. `brands`
17. `brand_logos`
18. `shopping_trips`
19. `brand_choices`
20. `brand_preferences`
21. `shared_links`

Schema migrations must remain backward-compatible. Use `ALTER TABLE ADD COLUMN` for new columns — never drop and recreate tables with existing data.

## Testing Contract

After any refactor:
- All existing Go tests must pass (`go test ./...`)
- All Playwright E2E tests must pass (`cd frontend && npx playwright test`)
- All HTTP endpoints must respond with the same status codes and JSON shapes
- All frontend screens must render and be navigable
- Database migrations must run cleanly on existing databases
- No functionality listed in "Must-Have Features" may be missing or broken

### Definition of Done

A feature or change is "done" when ALL of the following are true:
1. Go unit/integration tests written and passing (target 1.3x test:code ratio)
2. Playwright E2E tests written and passing for any new UI flow or screen change
3. `go test ./...` passes with zero failures
4. `cd frontend && npx playwright test` passes with zero failures
5. No regressions in existing functionality

## Automated UAT

UAT runs automatically via `go test ./...` — no live server or manual approval needed. The test suite covers:

1. **Auth flows** — phone/SMS, Google OAuth, email/password, magic link, session management, rate limiting
2. **List CRUD** — add, edit, delete, toggle, urgent, claim, clear, restore, parse, brand resolve
3. **Edge cases** — empty bodies, long strings, special chars, SQL injection payloads, XSS payloads, cross-user access, invalid IDs, boundary values
4. **Sharing & invites** — invite generation, join, household sharing, leave, referral system, admin auth gating, NPS
5. **Swaps** — deal matching, accept/decline, suppression, budget exhaustion, auto-pause, rebate ledger
6. **Settings** — all fields, slider clamping, brand mode validation, autotune/deals toggles
7. **Saved lists** — CRUD, load, empty names, nonexistent lists, special chars
8. **Trips** — CRUD, date validation, time labels, idempotent delete

After any significant work, run `go test -count=1 ./...` to verify. All tests must pass. When gaps are found, add new Go tests rather than relying on manual testing.

Add any new bugs found to the "Bugs" section in ROADMAP.md with the same format (B-number, description, file references, fix guidance). Bugs in the Bugs section are safe to fix without supervision — they are well-scoped and low-risk.

## Key Implementation Details to Preserve

- `sql.NullString` for nullable `phone_number` fields throughout
- `COALESCE(u.phone_number,'')` in all SQL queries reading phone_number
- Rate limiting via `sync.Map` with sliding time windows
- `CREATE TABLE IF NOT EXISTS` + `ALTER TABLE ADD COLUMN` migration pattern
- `resolveOrCreateUser()` auto-merge logic for multi-auth
- WAL mode for SQLite
- Environment variable graceful degradation (e.g., Google OAuth disabled if `GOOGLE_CLIENT_ID` empty)

## UI/UX Rules

### Penny Settings Page Layout (top to bottom)
1. **Contact section** — Penny avatar + name, icon-only action buttons (text, email, add to contacts) on the right
2. **Divider line** — subtle 1px separator between contact and settings
3. **"I want to SMS with Penny" toggle** (only visible if user has phone/SMS)
4. **"Learn from my choices" toggle**
5. **Convenience/savings slider** — at the bottom (only visible when autotune OFF), so expanding it doesn't shift anything above

### Conditional UI Behavior
- When "Learn from my choices" is ON → the convenience/savings slider is hidden (Penny auto-tunes)
- When "Learn from my choices" is OFF → the slider smoothly appears (CSS transition: max-height 0.3s + opacity 0.25s)
- "I want to SMS with Penny" toggle only appears if user has a phone number (opted in to messaging)
- Transitions must be smooth — no janky layout jumps. Use max-height + opacity transitions, never display:none toggling

### Auth Screens (Phone, Email Login, Email Register)
- All auth screens must include PennyAvatar + cycling typewriter value props
- Design must be consistent: Penny avatar → typewriter text → form fields → submit button → alternative auth links
- Password reveal icon must align vertically with the input field (use `bottom` positioning, not `top: 50%`)

### Contact Actions
- On mobile, use icon-only tap targets (44x44px minimum) for text/email/add-to-contacts — no displayed phone numbers or email addresses
- Use `sms:` link (not `tel:`) for Penny's phone — the primary action is texting, not calling
- Icons must have sufficient spacing (16px gap minimum) to prevent fat-fingering

### Navigation Menu Order
1. Shopping List
2. Share & Earn
3. Penny
4. ── divider ──
5. Profile (contains sign out)
6. Dashboard (admin only — only visible to admins, always last)

## Shared Ground Rules, Engineering Standards

> **⚠️ REQUIRED:** Before starting any work, verify that `../shared_ground_rules.md` and `../ENGINEERING_STANDARDS.md` exist. If either file is missing, STOP and alert the user — do not proceed without shared engineering standards.
>
> See `../shared_ground_rules.md` for: Secret Keys — Hard Rule, Engineering Preferences, Test-Driven Development (Iron Law — 100% TDD mandate), Parallel Development & Branching Strategy (max 3 parallel workstreams, git worktrees required), Sprint Planning Format (1-page plans, user stories with child ACs, atomic task decomposition, epic chunking), Plan Review Mode (Garry Tan Mega Review), Product Team (7 agent roles), Focus Groups protocol, Pattern Detection rules.
>
> See `../ENGINEERING_STANDARDS.md` for: Idempotency & Mutation Safety audit checklist, anti-patterns from B11/B14/B16/B17, handler audit template.

### PennyAI-Specific Mutation Safety Notes

PennyAI has the highest density of state-mutating handlers of any project (swap accept, budget deduction, rebate tracking, NPS, notifications). Run the `ENGINEERING_STANDARDS.md` handler audit on every new endpoint that touches `swap_events`, `brand_deals`, `swap_rebates`, or any Twilio/SendGrid call path.

## Product Team (Agent Roles)

> The 7-role product team and convene rules are defined in `../shared_ground_rules.md` and inherited automatically.
> This section documents PennyAI-specific convene guidance only.

### UX Designer / Reviewer

**Role:** Reviews every user-facing feature spec before implementation. Produces detailed UX specifications covering:
- **Edge cases:** empty states, error states, loading states, offline behavior
- **Scroll & layout:** overflow behavior, overscroll, momentum scrolling, safe areas, keyboard interactions
- **Touch targets:** minimum 44x44px, spacing to prevent fat-fingering, hover/active/focus states
- **Animation & timing:** transitions, typewriter timing, skeleton loaders, perceived performance
- **Accessibility:** screen reader flow, contrast ratios, focus management, aria labels
- **Mobile-specific:** bottom sheet heights, swipe gestures, pull-to-refresh, orientation changes

**Workflow:** The UX Designer reviews the feature requirements and produces a detailed UX spec. The PM synthesizes the UX spec with product requirements before implementation begins. No user-facing feature ships without UX review.

**PennyAI-specific:** Always convene before changes to Penny's personality (typewriter effect, chime-ins, quotes, nudges), the brand system (logo caching, brand preferences), or household sharing flows.

## Focus Group (Customer Panel)

For any **user-facing feature** (new UI, new interaction, changed flow), simulate a panel of 7 customers before building. Protocol (Rounds 1–4) is defined in `../shared_ground_rules.md`.

### Panel members

| Name | Persona | Key concern |
|------|---------|-------------|
| **Linda** (58) | Budget optimizer, coupon clipper | Price accuracy, clear savings, no tricks |
| **Devon** (34) | Tech-savvy early adopter, shares finds with friends | Clean UI, data transparency, power-user features, referral motivation |
| **Marcus** (41) | Skeptical about app tracking | Privacy, proof it's real, no dark patterns |
| **Tamika** (29) | Single mom, two kids, juggles work + school | Speed, simplicity, Penny should anticipate needs, zero tolerance for extra steps |
| **James** (45) | Household coordinator, shares list with wife | Household clarity, simple actions, coordination |
| **Priya** (31) | Health-conscious, reads every label | Dietary safety, ingredient info, no allergen-blind suggestions |
| **Nadia** (62) | Retired teacher, not tech-savvy, lives alone | Large text, clear instructions, no jargon, feels cared for |

## Pattern Detection

> Pattern detection rules are defined in `../shared_ground_rules.md` and inherited automatically.
> Idempotency audit checklist and anti-patterns (B11, B14, B16, B17) are in `../ENGINEERING_STANDARDS.md`.
