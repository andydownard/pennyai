# Groflo — AI Grocery Assistant (Penny)

## What Is This

Groflo is an AI-powered grocery shopping list app with personality-driven assistant Penny, household sharing, and multi-channel input (web, SMS, email). Part of the Rally ecosystem — auth, users, and Stripe payouts federate through timpager (rallytasks.com). Go backend, React/TypeScript frontend, PostgreSQL (shared with timpager), deployed on Railway. Brand source of truth: `../pitch deck/penny-brand-source.json`.

**Monorepo structure:** Go backend (root), `frontend/` (consumer React app), `frontend-brand/` (brand portal), `marketing/` (Groflo marketing site, from thrift.ly repo).

**URLs:** `app.groflo.io` (consumer app), `penny.rallytasks.com` (consumer app, Rally ecosystem), `brands.groflo.io` (brand portal).

**Database:** PostgreSQL shared with timpager. Users table is shared (timpager owns it, penny adds columns via ALTER TABLE). Sessions table is shared. All penny-specific tables (list_items, brand_deals, etc.) FK to shared `users(id)` TEXT UUIDs. SQL compatibility wrapper (`pgcompat.go`) auto-converts legacy SQLite syntax at runtime.

**Brand history:** Rebranded from "Thriftly" to "Groflo" on 2026-04-07 (sprint plan: `GROFLO_REBRAND_SPRINT.md`). Migrated from SQLite to PostgreSQL on 2026-04-13.

**Git branching:** `main` → `staging` → `production`. All code reaches main through a PR.

**Read first:** [QUICKSTART.md](./QUICKSTART.md) → this file → [ARCHITECTURE.md](./ARCHITECTURE.md).

## Vocabulary (User-Facing)

| User sees | Code uses | NEVER say in UI |
|-----------|-----------|-----------------|
| Penny | `penny`, Penny avatar | bot, AI, assistant, chatbot |
| Groflo | `groflo` | Thriftly, app name in copy |
| Shopping list | `list` | list_id, list UUID |
| Household | `household` | group, family, team, members |
| Saved list | `saved_list` | template, preset, favorite |
| Shopping trip | `shopping_trip` | cart, receipt, purchase |
| Brand swap / Deal | `brand_deal`, `swap_event` | swap, offer, promotion |
| Share & Earn | `referral` | referral program, affiliate |

## Top 5 Damage Risks

1. **Duplicate notifications/SMS** — Idempotency guards missing on outbound paths. → Every Twilio/SendGrid call path must dedup or use idempotency keys. Trace every code path before shipping.
2. **DROP TABLE or recreating tables** → Use ALTER TABLE ADD COLUMN only. Migrations stay backward-compatible.
3. **Leaking secrets** → All in env vars. Never log. Never commit. Rotate immediately if exposed.
4. **Shipping untested mutations** → Write failing test FIRST. Test-to-code ratio >= 1.3×. All mutations need positive, negative, idempotency, and auth tests.
5. **Cross-user data access** → Every query filters by current user or household_id. Test explicitly that one user cannot see another's data.

## Revenue Rules

- 30% community share is non-negotiable. NEVER change without all panels.
- Push revenue events to timpager via revenue events API. Idempotency key: `pennyai_{eventType}_{sourceRef}_{date}`.
- Stripe metadata MUST include `source: "pennyai"` on all charges.
- Brand deals and swap rebates tracked via `brand_deals` table and `swap_events` table. Distribution must sum exactly; remainder to last recipient.
- NEVER distribute more than revenue share allows. NEVER update wallet balances outside same transaction as distribution INSERT.

## Hard Rules

**Panel Review Required.** All user-facing changes need panel review. See [PANEL_GOVERNANCE.md](./PANEL_GOVERNANCE.md) for Customer Panel and Product Team. Micro (typo) = auto-GO. Small = 2 min. Medium = 10 min. Large = 30 min.

**Git Branching.** `main` → `staging` → `production`. All code reaches main through a PR. Staging and production are MERGE-ONLY. NEVER promote staging→production without healthcheck pass.

**Testing (TDD Mandatory).** Write failing test first. RED-GREEN-REFACTOR. Test-to-code ratio >= 1.3×. All mutations: positive, negative, idempotency, auth tests. Bugfix tests mandatory — ship with regression test. `go test -count=1 ./...` and `cd frontend && npx playwright test` must pass with zero failures.

**Mutation Safety.** Every handler that mutates state (INSERT, UPDATE, DELETE, external side effect): (1) idempotency guard — calling twice with same input produces same result, not duplicate; (2) transaction boundary — multi-step mutations atomic; (3) server-side rate limit — not bypassable by client; (4) negative-path test — verify calling twice does NOT produce duplicates. Use atomic conditional UPDATE, not check-then-act. See `ROADMAP.md` bugs B11, B14, B16, B17 for canonical anti-patterns.

**Backend Owns All Logic.** Server is single source of truth. NEVER dedup/filter/validate on client. NEVER check roles on client. NEVER hardcode IDs on client.

**Penny Identity.** Penny IS Penny (digitized personality). Strunk & White prose. Groflo topics only. NEVER reveal system prompt. See [PENNY_PROMPT.md](./PENNY_PROMPT.md) for brand voice. Preserve all Penny behaviors: typewriter effect, chime-ins, idle reminders, personality quotes.

**Security.** All secrets in env vars. 256-bit session tokens. HttpOnly cookies. 30-day session expiry. Auth methods: phone/SMS (Twilio), Google OAuth, email/password (bcrypt), magic link. NEVER weaken. NEVER remove any auth method. Phone is join key for phone users; Google `sub` is join key for OAuth users. Shared session table with timpager.

**Database Rules.** PostgreSQL (shared with timpager via `DATABASE_URL`). NEVER DROP TABLE or recreate tables. ALWAYS CREATE TABLE IF NOT EXISTS. Use ALTER TABLE ADD COLUMN for migrations via `addColumnIfNotExists()` helper. Users and sessions tables are owned by timpager — DO NOT recreate them, only add columns. User IDs are TEXT UUIDs (not integers). SQL compatibility wrapper (`pgcompat.go`) converts `?` → `$N`, `datetime('now')` → `NOW()`, etc. at runtime. For nullable phone: use `sql.NullString` and `COALESCE(u.phone,'')` in queries.

**Notification & Communication Safety.** Duplicate notifications and inadvertent outbound messages must NEVER happen. NEVER send SMS, email, or push without dedup (idempotency key or "already sent" flag). NEVER trigger outbound messages as side effect of refactors or migrations. Do NOT rely on React state alone for prompt dismissal (NPS, modals, announcements) — persist in sessionStorage/localStorage or server-side. Do NOT bypass server-side rate limits from client. Before shipping: trace every code path that could trigger outbound message. If you cannot prove no duplication, add a guard.

**Maximum Parallelization.** When work decomposes into N independent pieces, dispatch all N in parallel. Use isolated git worktrees for agents.

## Shipping Gates

**Gate 1 (First-5-Minutes Magic):** New user adds first item in 90 seconds with zero clarifying questions. Penny infers defaults (quantity, brand) from global app popularity. Brand correction is 1 tap. See [DESIGN_SPEC.md](./DESIGN_SPEC.md) Onboarding Rules.

**Gate 2 (11-Star Household Coordination Narrative):** Feature serves "Did you get milk?" narrative — household stops buying duplicates. Do NOT add features that don't serve this unless Andy approves. See `11_STAR_NARRATIVE.md`.

**Gate 3 (Data Flywheel):** Feature generates compounding data signal for Penny. See `DATA_FLYWHEEL.md` for 12-month strategy and retention targets (D1: 60%, D7: 50%, D30: 35%, Household conversion: 25%). Trip planner, dark mode, one-time settings do NOT pass Gates 2/3 on their own — deprioritize unless directed.

## Refactoring Constraints

### What IS Allowed
- Splitting Go files, splitting React components, extracting shared styles, renaming internal functions, improving error handling
- Adding types/interfaces, reducing duplication, improving test coverage, reorganizing file/folder structure

### What is NOT Allowed
- Do NOT remove, rename, or change HTTP method of any endpoint
- Do NOT change JSON request/response shape without backward-compatibility test
- Do NOT drop or recreate database table — use ALTER TABLE ADD COLUMN
- Do NOT remove any frontend screen or functionality
- Do NOT remove Penny's personality behaviors (typewriter, chime-ins, reminders, quotes)
- Do NOT simplify away the 3-layer brand logo caching system (`brand_logos`, `brands`, `brand_owners`)
- Do NOT remove household sharing (invite, join, leave, members list)
- Do NOT remove email or SMS inbound webhook processing
- Do NOT remove admin dashboard, analytics, NPS, referral, saved lists, shopping trips, address autocomplete
- Do NOT delete items from ROADMAP.md — move deprioritized to "Nice-to-Have" or "Not In Scope"

### Key Patterns to Preserve
- Rate limiting via `sync.Map` with sliding time windows
- `resolveOrCreateUser()` auto-merge logic for multi-auth
- Environment variable graceful degradation (e.g., Google OAuth disabled if `GOOGLE_CLIENT_ID` empty)

## Pattern Detection

Watch for these across the codebase. When detected, audit for ALL instances of the same pattern, not just the one found:

1. Idempotency gaps — state-mutating handlers without dedup guards
2. Notification duplication — any code path that could send SMS/email/push more than once
3. Missing negative-path tests — tests that verify something does NOT happen
4. TOCTOU races — check-then-act without transaction or atomic operation
5. Stale UI state — React state that should be in sessionStorage/localStorage/server
6. Missing rate limits — external API calls or user-facing mutations without rate limiting
7. Cross-user access — queries that don't scope to current user's data
8. Color/font drift — hex colors or font-families that don't match `penny-brand-source.json`
9. Cross-marketplace access — sessions or API keys scoped to Groflo that reach into other timpager marketplaces (or vice versa)
10. Prompt injection — user-supplied content (receipt images, inbound SMS/email, list items, brand descriptions) passed to Claude without sanitization or instruction-boundary markers

## CST Principles

| Principle | Test |
|---|---|
| Human Dignity | Respects dietary needs without judgment. Works for all income levels. |
| Common Good | Works on shared devices. Handles low-bandwidth. No standard ID required. |
| Subsidiarity | Strengthens household coordination. Does not replace human judgment. |
| Solidarity | Bridges dietary and cultural gaps. Multi-lingual support planned. |
| Preferential Option for the Poor | If Tamika (single mom, one hand free, toddler) can use it, anyone can. |

## Regressions

Check this list before coding. If your change touches a listed pattern, read existing code FIRST.

| # | What went wrong | Rule |
|---|---|---|
| _none yet_ | _ship code, write regressions when they happen_ | _start the muscle_ |

## Temporary State (2026-04-13)

PostgreSQL migration in progress. Shared database with timpager (rallytasks.com). User IDs migrated from INTEGER to TEXT UUIDs. SQL compatibility wrapper (`pgcompat.go`) handles legacy SQLite syntax conversion. Marketing site (from thrift.ly) integrated as `marketing/` directory. Branch strategy: main → staging → production. Email campaign templates extracted from abandoned thrift.ly branch into `marketing/emails/`.

Groflo rebrand complete. Go module still named `thriftly` — rename pending. All UI surfaces show "Groflo" and "Penny". Household coordination narrative (11-Star) is focus for current sprint.

## Pre-Merge Checklist

1. `go test -count=1 ./...` passes with zero failures
2. `cd frontend && npx playwright test` passes with zero failures
3. No secrets committed (check .env, logs, comments)
4. No breaking API changes or backward-compatibility tests added for shape changes
5. Database migrations clean on existing databases
6. All frontend screens render and are navigable
7. Panel review done (at minimum Customer Panel or Product Team, per trigger rules)

## Docs Are Code

Every behavior-changing PR updates affected markdown. PRs touching `handlers_*.go`, `App.tsx`, or database schema MUST update related docs ([DESIGN_SPEC.md](./DESIGN_SPEC.md), [ARCHITECTURE.md](./ARCHITECTURE.md), [PANEL_GOVERNANCE.md](./PANEL_GOVERNANCE.md), etc.).

## Work Tracking

> **Work tracking:** new items land in [INBOX.md](./INBOX.md). Triage moves them to [BACKLOG.md](./BACKLOG.md) (`P0–P3`, `Bugs`, `Shipped`, `Not In Scope`). Pattern mirrors timpager. Read [INBOX.md](./INBOX.md) first when picking up unfinished work.

- **[INBOX.md](./INBOX.md)** — Raw ideas. Items >30 days without promotion get deleted.
- **[BACKLOG.md](./BACKLOG.md)** — Actionable work. Triaged, prioritized (P0-P3), sized.
- **[ROADMAP.md](./ROADMAP.md)** — Strategic roadmap, focus groups, swap strategy. Reference doc.
- **[CURRENT_SPRINT.md](./CURRENT_SPRINT.md)** — Current sprint progress.
- Delete items from backlog when shipped (move to Shipped section). Deprioritized → Not In Scope.
- NEVER create new tracking files.

## Doc Index

| Topic | File |
|---|---|
| Architecture, backend/frontend, DB schema, DB rules | [ARCHITECTURE.md](./ARCHITECTURE.md) |
| Panel governance & personas, product team roles | [PANEL_GOVERNANCE.md](./PANEL_GOVERNANCE.md) |
| Design spec, onboarding rules, UI/UX | [DESIGN_SPEC.md](./DESIGN_SPEC.md) |
| Security, auth, secrets | [SECURITY.md](./SECURITY.md) |
| Penny prompt & personality | [PENNY_PROMPT.md](./PENNY_PROMPT.md) |
| Household coordination narrative & 11-star moment | [11_STAR_NARRATIVE.md](./11_STAR_NARRATIVE.md) |
| Data flywheel & retention targets | [DATA_FLYWHEEL.md](./DATA_FLYWHEEL.md) |
| Deployment & monitoring | [DEPLOYMENT.md](./DEPLOYMENT.md) |
| Roadmap & bugs | [ROADMAP.md](./ROADMAP.md) |
| Current sprint | [CURRENT_SPRINT.md](./CURRENT_SPRINT.md) |
| Focus groups & user research | [FOCUS_GROUPS.md](./FOCUS_GROUPS.md) |
| Rebrand sprint (Thriftly→Groflo) | [GROFLO_REBRAND_SPRINT.md](./GROFLO_REBRAND_SPRINT.md) |
