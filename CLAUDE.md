# Groflo (PennyAI, formerly Thriftly)

Groflo is an AI-powered grocery shopping list app with a personality-driven assistant named **Penny**, household sharing, and multi-channel input (web, SMS, email). Groflo is a member of the **Rally ecosystem** — auth and Stripe payouts federate through `timpager` (rallytasks.com).

**Brand history:** This product was originally "Thriftly" and rebranded to "Groflo" on 2026-04-07 (sprint plan: `GROFLO_REBRAND_SPRINT.md`). The DB filename `thriftly.db`, Litestream replication path, and historical migration symbols retain the legacy name to preserve backup chain integrity. Internal Go identifiers may still reference `thriftly` — those are non-user-visible and scheduled for the Phase 5 internal sweep.

## Architecture

- **Backend**: Go, 14+ files (`server.go`, `db.go`, `models.go`, `penny.go`, `handlers_*.go`, `session.go`, `utils.go`), SQLite (WAL mode), deployed on Railway
- **Frontend**: React/TypeScript, `App.tsx` + extracted components, Vite build
- **Auth**: Multi-auth (phone/SMS via Twilio, Google OAuth, email/password with bcrypt, magic link)
- **Database**: SQLite with inline migrations in `initDB()`
- **Brand source of truth**: `../pitch deck/penny-brand-source.json` — all brand elements, colors (#35644F), fonts (Inter web / Avenir mobile), tagline, and value props derive from this file. Edit it first, sync to all surfaces.

---

## Hard Rules

### Notification & Communication Safety

Duplicate notifications and inadvertent outbound messages must NEVER happen. This is a trust-destroying, potentially app-deleting event.

- Do NOT send SMS, email, or push without a dedup mechanism (idempotency key or "already sent" flag)
- Do NOT trigger outbound messages as a side effect of refactors, migrations, or data backfills
- Do NOT rely on React state alone for dismissal of prompts (NPS, modals, announcements) — persist in sessionStorage/localStorage or server-side
- Do NOT bypass server-side rate limits from client — protect against useEffect loops, retry storms, rapid polling
- ALWAYS test the negative path: "after dismiss, does NOT show again", "after submit, does NOT send again", "rapid calls do NOT produce duplicates"

Before shipping any change to SMS, email, NPS, or notification code: trace every code path that could trigger an outbound message. If you cannot prove it will not duplicate or fire inadvertently, add a guard.

### Secret Keys

- Do NOT commit, log, or expose API keys, tokens, database credentials, or secrets
- All secrets live in environment variables (`.env`), listed in `.gitignore`
- `.env.example` must exist with placeholder values for every required secret
- If a secret is accidentally committed, rotate it immediately

### Idempotency & Mutation Safety

Every handler that mutates state (INSERT, UPDATE, DELETE, or external side effect) must have:

1. **Idempotency guard** — calling it twice with the same input produces the same result, not a duplicate
2. **Transaction boundary** — multi-step mutations are atomic (all succeed or all fail)
3. **Server-side rate limit** — not bypassable by client bugs
4. **Negative-path test** — test that calling it twice does NOT produce duplicates

Use atomic conditional UPDATE (`UPDATE ... WHERE budget_spent_cents + ? <= daily_budget_cents`), not check-then-act. See bugs B11 (budget race), B14 (rebate farming), B16 (NPS re-prompt), B17 (per-campaign limit) in `docs/shared-archive/ENGINEERING_STANDARDS_ROOT_2026-03-17.md` for canonical anti-patterns.

PennyAI has the highest density of state-mutating handlers of any project. Run the idempotency audit on every new endpoint that touches `swap_events`, `brand_deals`, `swap_rebates`, or any Twilio/SendGrid call path.

### Session Security

- Do NOT weaken session security: HttpOnly cookies, 256-bit random tokens, 30-day expiry
- Do NOT remove or disable any auth method (phone/SMS, Google OAuth, email/password, magic link)

### Test-Driven Development

- Test-to-code ratio must be >= 1.3x (130 lines of test per 100 lines of production code)
- Write the failing test first. If you write code before the test, delete it and start over.
- All Go tests must pass: `go test -count=1 ./...`
- All Playwright E2E tests must pass: `cd frontend && npx playwright test`
- No feature or change is "done" until both pass with zero failures

### Continuous Work Mode

Never stop working. When you finish a task: check ROADMAP.md for unblocked items, improve test coverage, refactor per these rules. The default state is working, not waiting. Do NOT start a new sprint without confirming with the user unless the current sprint was started in this same session.

---

## Refactoring Constraints

### What IS allowed

- Splitting Go files into multiple files/packages
- Splitting React components into multiple files
- Extracting shared styles, renaming internal functions, improving error handling
- Adding types/interfaces, reducing duplication, improving test coverage
- Reorganizing file/folder structure

### What is NOT allowed

- Do NOT remove, rename, or change the HTTP method of any existing endpoint
- Do NOT change the JSON request/response shape of any endpoint without a backward-compatibility test
- Do NOT drop or recreate any database table — use `ALTER TABLE ADD COLUMN` for migrations
- Do NOT remove any frontend screen or its functionality
- Do NOT remove any of Penny's personality behaviors (typewriter effect, chime-ins, idle reminders, quotes)
- Do NOT simplify away the 3-layer brand logo caching system (`brand_logos`, `brands`, `brand_owners`)
- Do NOT remove household sharing features (invite, join, leave, view members)
- Do NOT remove email or SMS inbound webhook processing
- Do NOT remove the admin dashboard, analytics, NPS, referral system, saved lists, shopping trips, or address autocomplete
- Do NOT delete items from ROADMAP.md — move deprioritized items to "Nice-to-Have" or "Not In Scope"

### Database Migration Pattern

- `CREATE TABLE IF NOT EXISTS` for new tables
- `ALTER TABLE ADD COLUMN` for new columns — never drop and recreate
- `sql.NullString` for nullable `phone_number` fields
- `COALESCE(u.phone_number,'')` in all SQL queries reading phone_number
- WAL mode for SQLite
- Schema migrations must remain backward-compatible

### Key Patterns to Preserve

- Rate limiting via `sync.Map` with sliding time windows
- `resolveOrCreateUser()` auto-merge logic for multi-auth
- Environment variable graceful degradation (e.g., Google OAuth disabled if `GOOGLE_CLIENT_ID` empty)

---

## Shipping Gate Constraints

- Do NOT add features that require clarifying questions during new-user onboarding (Gate 1: First-5-Minutes Magic)
- Do NOT add features that don't serve household coordination — the "Did you get milk?" narrative (Gate 2: 11-Star Moment) — unless explicitly approved by Andy
- Do NOT add features that generate no compounding data signal (Gate 3: Data Flywheel) unless explicitly approved by Andy
- Trip planner, dark mode, and one-time settings do NOT pass Gates 2/3 on their own — deprioritize unless directed otherwise
- See `11_STAR_NARRATIVE.md` for the full narrative brief; see `DATA_FLYWHEEL.md` for the 12-month flywheel strategy and retention targets (D1: 60%, D7: 50%, D30: 35%, Household conversion: 25%)

---

## Onboarding Rules (Gate 1 Detail)

New user must add first item within 90 seconds with zero clarifying questions.

- **Assume reasonable defaults** — Penny infers quantity (milk → 1 gallon) and selects a popular brand. Do NOT ask clarifying questions.
- **Brand defaults from global popularity** — Use aggregate app-wide preferences (not personal/household tracking) for new users. Label clearly: "I picked [brand] based on popular choices."
- **Brand defaults respect dietary attributes** — If user has ≥3 items of same dietary attribute (organic, nut-free, vegan), prioritize matching attributes over popularity.
- **Brand correction is 1 tap** — ActionSheet with top 5 alternatives, each showing logo + name + dietary attributes. On selection: close sheet, update item, show confirmation toast ("✓ Changed to Organic Valley"), auto-dismiss 3s. No secondary confirmation modal.
- **Household invite after 3rd item** — Soft language: "Want to invite your family to this list?" Dismissal persists in localStorage — do NOT re-trigger in same or future sessions.
- **Popular brand fallback** — If Claude cannot infer brand, use global popularity or generic option ("Best Value", "Kroger brand"). No personal tracking of similar households.

---

## UI/UX Rules

### Penny Settings Page Layout (top to bottom)

1. Contact section — Penny avatar + name, icon-only action buttons (text, email, add to contacts)
2. Divider (1px separator)
3. "I want to SMS with Penny" toggle (only if user has phone)
4. "Learn from my choices" toggle
5. Convenience/savings slider (only when autotune OFF, at bottom)

Autotune ON → slider hidden. Autotune OFF → slider appears with CSS transition (max-height 0.3s + opacity 0.25s). Do NOT use display:none toggling.

### Auth Screens

- All auth screens: PennyAvatar + cycling typewriter value props + form fields + submit + alt auth links
- Password reveal icon: `bottom` positioning, not `top: 50%`

### Touch Targets & Contact Actions

- Minimum 44x44px touch targets, 16px gap minimum between icons
- Use `sms:` link for Penny's phone (not `tel:`)
- Icon-only on mobile for text/email/add-to-contacts — no displayed numbers or addresses

### Navigation Menu Order

1. Shopping List → 2. My Lists → 3. Share & Earn → 4. Penny → ── divider ── → 5. Profile → 6. Dashboard (admin only, always last)

---

## Testing Contract

After any change:

- `go test -count=1 ./...` passes with zero failures
- `cd frontend && npx playwright test` passes with zero failures
- All HTTP endpoints respond with same status codes and JSON shapes
- All frontend screens render and are navigable
- Database migrations run cleanly on existing databases

UAT runs automatically via `go test ./...` covering: auth flows, list CRUD, edge cases (empty bodies, SQL injection, XSS, cross-user access), sharing/invites, swaps, settings, saved lists, trips.

Add new bugs to ROADMAP.md with B-number format. Bugs in the Bugs section are safe to fix without supervision.

---

## Customer Panel

For any user-facing feature, simulate this panel BEFORE implementation. See Panel Trigger Rules below.

| Panelist | Archetype | Decision Framework | Dealbreaker |
|----------|-----------|-------------------|-------------|
| **Linda** (58) | Budget optimizer, coupon clipper | "Will this actually save me money I can verify?" | Hidden costs, unclear savings, unproven claims |
| **Devon** (34) | Tech-savvy early adopter, shares finds | "Would I screenshot this and send to my group chat?" | Cluttered UI, no data transparency, nothing to brag about |
| **Marcus** (41) | Skeptical about app tracking | "What data are you collecting and why?" | Dark patterns, tracking without consent, vague privacy language |
| **Tamika** (29) | Single mom, two kids, juggles work + school | "Can I do this one-handed while holding a toddler?" | Extra steps, clarifying questions, anything requiring focus |
| **James** (45) | Household coordinator, shares list with wife | "Will my wife and I stop buying duplicate milk?" | Confusing coordination, unclear who claimed what |
| **Priya** (31) | Health-conscious, reads every label | "Does Penny respect my dietary needs without me repeating them?" | Allergen-blind suggestions, no ingredient info |
| **Nadia** (62) | Retired teacher, not tech-savvy, lives alone | "Can I figure this out without asking my grandson?" | Small text, jargon, anything requiring instructions |

### Panel Protocol (4 Rounds)

1. **First Reactions** — Gut reaction per panelist (30 seconds each)
2. **Deep Dive** — 2-3 specific questions per panelist from their framework
3. **Dealbreakers** — Each panelist states #1 condition for using / not using
4. **Synthesis** — PM summarizes consensus, flags disagreements, produces requirements with panelist attribution

### Panel Trigger Rules — MANDATORY

Convene Customer Panel BEFORE implementation when ANY of these are true:

- Adding or changing text the user reads (error messages, empty states, Penny dialogue)
- Adding or removing a tap/click from any user flow
- Changing what information is shown or hidden
- Adding a new screen, modal, or bottom sheet

Convene CEO Panel BEFORE implementation when ANY of these are true:

- Feature takes >3 days to build
- Feature affects revenue path (swaps, brands, payments)
- Feature could be cut without user complaint (test for necessity)
- Scope is growing beyond original sprint spec

Do NOT convene panels for: refactors with no user-facing change, test improvements, dependency updates, bug fixes where correct behavior is already specified.

---

## Product Team (7 Roles)

| Role | Focus | Convene when |
|------|-------|-------------|
| **PM** | User value, prioritization, scope | Every feature |
| **Architect** | System design, data flow, scalability | New tables, services, integrations |
| **UX Designer** | Edge cases, touch targets, animation, accessibility | Any user-facing change |
| **QA** | Test coverage, edge cases, failure modes | Every feature |
| **DevOps** | Deploy safety, monitoring, rollback | New infra, env vars, external services |
| **BI Analyst** | Metrics, tracking, dashboards | Features with success metrics |
| **Growth** | Virality, retention, activation | User-facing features, onboarding, sharing |

Always convene before: changes to revenue paths, auth flows, Penny's personality, database schema, external API integrations. Always have UX Designer review BEFORE implementation begins for any user-facing change. PennyAI-specific: always convene before changes to brand system (logo caching, brand preferences) or household sharing flows.

---

## Pattern Detection

Watch for these across the codebase. When detected, do not just fix the instance — audit for all instances of the same pattern.

1. Idempotency gaps — state-mutating handlers without dedup guards
2. Notification duplication — any code path that could send SMS/email/push more than once
3. Missing negative-path tests — tests that verify something does NOT happen
4. TOCTOU races — check-then-act without transaction or atomic operation
5. Stale UI state — React state that should be in sessionStorage/localStorage/server
6. Missing rate limits — external API calls or user-facing mutations without rate limiting
7. Cross-user access — queries that don't scope to current user's data
8. Color/font drift — hex colors or font-families that don't match `penny-brand-source.json`
