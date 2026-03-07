# Thriftly (PennyAI)

Thriftly is an AI-powered grocery shopping list app with a personality-driven assistant named Penny, household sharing, and multi-channel input (web, SMS, email).

## Architecture

- **Backend**: Go, split across 14 files (`server.go`, `db.go`, `models.go`, `penny.go`, `handlers_*.go`, `session.go`, `utils.go`), SQLite (WAL mode), deployed on Railway
- **Frontend**: React/TypeScript, `App.tsx` + extracted components (`PennyAvatar`, `ActionSheet`, `ConfirmModal`, `FloatingLabelField`, etc.), Vite build
- **Auth**: Multi-auth (phone/SMS via Twilio, Google OAuth, email/password with bcrypt, magic link)
- **Database**: SQLite with inline migrations in `initDB()`

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

Schema migrations must remain backward-compatible. Use `ALTER TABLE ADD COLUMN` for new columns — never drop and recreate tables with existing data.

## Testing Contract

After any refactor:
- All existing Go tests must pass (`go test ./...`)
- All HTTP endpoints must respond with the same status codes and JSON shapes
- All frontend screens must render and be navigable
- Database migrations must run cleanly on existing databases
- No functionality listed in "Must-Have Features" may be missing or broken

## Automated UAT

When the server is running locally (`http://localhost:8080`), run automated UAT by launching 4 parallel agents:

1. **New user onboarding** — Test all 3 auth flows (phone, Google, email), registration, login, session management, rate limiting
2. **Core list features** — Create a test user, test all list CRUD, parse, saved lists, settings. Include edge cases: empty bodies, long strings, special chars, SQL injection, XSS payloads, cross-user access
3. **Sharing & invite flow** — Create 2 users, test invite generation, join, household sharing, leave, referral system, admin auth gating, NPS
4. **Frontend & assets** — Check HTML structure, JS bundle, favicons, security headers, SPA fallback, PWA manifest

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
1. **Settings section** — "Learn from my choices" toggle, conditional slider (only visible when autotune OFF), "Notify me about deals" toggle (only visible if user has phone/SMS)
2. **Divider line** — subtle 1px separator between settings and contact
3. **Contact section** — Penny avatar + name, icon-only action buttons (text, email, add to contacts) on the right

### Conditional UI Behavior
- When "Learn from my choices" is ON → the convenience/savings slider is hidden (Penny auto-tunes)
- When "Learn from my choices" is OFF → the slider smoothly appears (CSS transition: max-height 0.3s + opacity 0.25s)
- "Notify me about deals" toggle only appears if user has a phone number (opted in to messaging)
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

## Engineering Preferences

* DRY is important — flag repetition aggressively.
* Well-tested code is non-negotiable; test:code ratio target is 1.3x or higher.
* Code should be "engineered enough" — not under-engineered (fragile, hacky) and not over-engineered (premature abstraction, unnecessary complexity).
* Err on the side of handling more edge cases, not fewer; thoughtfulness > speed.
* Bias toward explicit over clever.
* Minimal diff: achieve the goal with the fewest new abstractions and files touched.
* Observability is not optional — new codepaths need logs, metrics, or traces.
* Security is not optional — new codepaths need threat modeling.
* ASCII diagrams in code comments for complex designs.

## Plan Review Mode (Mega Review)

When reviewing implementation plans, use the following framework. Three modes:

- **SCOPE EXPANSION:** Build the cathedral. Push scope UP. Ask "what would make this 10x better for 2x the effort?"
- **HOLD SCOPE:** Rigorous review. The plan's scope is accepted. Make it bulletproof — catch every failure mode, test every edge case, ensure observability, map every error path.
- **SCOPE REDUCTION:** Surgeon mode. Find the minimum viable version. Cut everything else. Be ruthless.

Context-dependent defaults:
- Greenfield feature → default EXPANSION
- Bug fix or hotfix → default HOLD SCOPE
- Refactor → default HOLD SCOPE
- Plan touching >15 files → suggest REDUCTION unless user pushes back

Once a mode is selected, COMMIT to it. Do not silently drift.

### Prime Directives

1. **Zero silent failures.** Every failure mode must be visible — to the system, to the team, to the user.
2. **Every error has a name.** Don't say "handle errors." Name the specific exception, what triggers it, what rescues it, what the user sees, and whether it's tested.
3. **Data flows have shadow paths.** Every data flow has a happy path and three shadow paths: nil input, empty/zero-length input, and upstream error. Trace all four.
4. **Interactions have edge cases.** Every user-visible interaction: double-click, navigate-away-mid-action, slow connection, stale state, back button. Map them.
5. **Observability is scope, not afterthought.** Dashboards, alerts, and runbooks are first-class deliverables.
6. **Diagrams are mandatory.** No non-trivial flow goes undiagrammed. ASCII art for data flows, state machines, pipelines, dependency graphs.
7. **Everything deferred must be written down.** Vague intentions are lies. TODOS.md or it doesn't exist.
8. **Optimize for the 6-month future, not just today.**
9. **You have permission to say "scrap it and do this instead."**

### Review Sections (when doing plan review)

1. **Architecture Review** — system design, data flows (all 4 paths: happy/nil/empty/error), state machines, coupling, scaling, security, rollback posture
2. **Error & Rescue Map** — for every method that can fail: what goes wrong, exception class, rescued?, rescue action, what user sees
3. **Security & Threat Model** — attack surface, input validation, authorization, secrets, injection vectors, audit logging
4. **Data Flow & Interaction Edge Cases** — trace inputs through validation→transform→persist→output with shadow paths; map UI edge cases (double-click, navigate-away, slow connection, stale state)
5. **Code Quality Review** — DRY violations, naming, error handling patterns, missing edge cases, over/under-engineering, cyclomatic complexity
6. **Test Review** — diagram all new UX flows, data flows, codepaths, async work, integrations, error paths. For each: what type of test covers it? Happy path test? Failure path test? Edge case test?
7. **Performance Review** — N+1 queries, memory usage, indexes, caching, background job sizing, slow paths, connection pool pressure
8. **Observability & Debuggability** — logging, metrics, tracing, alerting, dashboards, debuggability, admin tooling, runbooks
9. **Deployment & Rollout** — migration safety, feature flags, rollout order, rollback plan, deploy-time risk, post-deploy verification
10. **Long-Term Trajectory** — tech debt introduced, path dependency, knowledge concentration, reversibility (1-5 scale), ecosystem fit

### Required Review Outputs

- **"NOT in scope" section** — work considered and explicitly deferred
- **"What already exists" section** — existing code that partially solves sub-problems
- **Error & Rescue Registry** — complete table of every method that can fail
- **Failure Modes Registry** — codepath, failure mode, rescued?, tested?, user sees?, logged?
- **TODOS.md updates** — deferred work with what/why/context/effort/priority
- **Diagrams** — system architecture, data flow, state machines, error flow, deployment sequence, rollback flowchart

### For Each Issue Found

- Describe concretely with file and line references
- Present 2-3 options including "do nothing"
- Lead with recommendation as a directive
- For each option: effort, risk, maintenance burden in one line
