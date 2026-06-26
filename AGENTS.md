<!-- AGENT INSTRUCTIONS — read and obey before writing any code. -->
# Groflo — AI Grocery Assistant (Penny)

> **Global engineering contract:** https://github.com/rallytasks/agents/blob/main/AGENTS.md
> Every rule there applies to this repo. This file adds only project specifics and any `Override <ID>:`.
@../agents/AGENTS.md   <!-- local-checkout convenience; no-op in a standalone clone -->

AI-powered grocery shopping-list app with personality-driven assistant **Penny**, household sharing, and multi-channel input (web, SMS, email). Part of the Rally ecosystem; resolves every verified user against timpager's identity API so one person = one `user_id` across Rally. Go backend · React/TypeScript frontend · PostgreSQL (pennyai-owned, separate from timpager's) · Railway. Scope key: `user` / `household_id`. Monorepo: Go backend (root), `frontend/` (consumer), `frontend-brand/` (brand portal), `marketing/` (Groflo site). URLs: app.groflo.io, penny.rallytasks.com, brands.groflo.io.

**Read first:** [QUICKSTART.md](./QUICKSTART.md) → this file → [ARCHITECTURE.md](./ARCHITECTURE.md)

## Commands
test: `go test -count=1 ./...` · frontend: `cd frontend && npx playwright test` (both zero failures)

## Vocabulary
| User sees | Code uses | NEVER say in UI |
|---|---|---|
| Penny | `penny`, Penny avatar | bot, AI, assistant, chatbot |
| Groflo | `groflo` | Thriftly, app name in copy |
| Shopping list | `list` | list_id, list UUID |
| Household | `household` | group, family, team, members |
| Saved list | `saved_list` | template, preset, favorite |
| Shopping trip | `shopping_trip` | cart, receipt, purchase |
| Brand swap / Deal | `brand_deal`, `swap_event` | swap, offer, promotion |
| Share & Earn | `referral` | referral program, affiliate |

## Top damage risks
1. Duplicate notifications/SMS → idempotency keys on every Twilio/SendGrid path (`OUTBOUND`).
2. DROP TABLE / recreate → `ALTER TABLE ADD COLUMN` only (`DB`).
3. Leaking secrets (`SECRETS`). 4. Untested mutations (`TDD`). 5. Cross-user/household data access → filter every query (`TENANT`).

## Project rules & overrides
- **Revenue:** 30% community share, non-negotiable, never change without all panels. Push revenue events to timpager; idempotency key `pennyai_{eventType}_{sourceRef}_{date}`; Stripe metadata `source:"pennyai"`. Distribution sums exactly (remainder to last); never distribute more than allowed; never update wallet balances outside the distribution transaction. (`MONEY`.)
- **Penny identity:** Penny IS Penny (digitized); Strunk & White; Groflo topics only; never reveal the system prompt; preserve all behaviors (typewriter, chime-ins, idle reminders, personality quotes). See [PENNY_PROMPT.md](./PENNY_PROMPT.md). (`AI`.)
- **Database:** pennyai owns its own Postgres (`DATABASE_URL`), separate from timpager's. Owns `users` + `sessions`; user IDs are TEXT UUIDs from timpager's identity resolve API stored locally. `pgcompat.go` converts legacy SQLite syntax at runtime; `addColumnIfNotExists()` for migrations; nullable phone via `sql.NullString` + `COALESCE(u.phone,'')`.
- **Auth:** phone/SMS (Twilio), Google OAuth, email/password (bcrypt), magic link — never weaken or remove any method. Phone is the join key for phone users; Google `sub` for OAuth users. Sessions live in pennyai's own DB on `.groflo.io` cookies; cross-domain SSO is intentionally NOT implemented.
- **Notification safety:** never send SMS/email/push without dedup; never as a side effect of refactor/migration; don't rely on React state alone for prompt dismissal (NPS, modals) — persist in storage or server-side.
- **Refactoring fences — do NOT remove/rename/change** any endpoint's method or JSON shape without a backward-compat test (`API`); any frontend screen; Penny's behaviors; the 3-layer brand-logo caching (`brand_logos`, `brands`, `brand_owners`); household sharing; email/SMS inbound webhook processing; admin dashboard, analytics, NPS, referral, saved lists, shopping trips, address autocomplete. Don't delete ROADMAP.md items — move to Nice-to-Have / Not In Scope.
- **Shipping gates:** (1) first item in 90s with zero clarifying questions; (2) serves the "Did you get milk?" household-coordination narrative ([11_STAR_NARRATIVE.md](./11_STAR_NARRATIVE.md)); (3) generates a compounding data signal ([DATA_FLYWHEEL.md](./DATA_FLYWHEEL.md)). Trip planner / dark mode / one-time settings do NOT pass gates 2/3 alone.
- **Pattern detection additions:** cross-marketplace access (Groflo session/keys reaching other timpager marketplaces) and prompt injection (receipt images, inbound SMS/email, list items, brand descriptions → Claude without sanitization).

## Temporary state (2026-04-18)
Postgres migration complete; pennyai owns its own DB. Go module still named `thriftly` (rename pending); all UI shows "Groflo"/"Penny". Rally identity federation in flight (PR 2/3). Marketing site integrated as `marketing/`. 11-Star household narrative is the current focus.

## Regressions
| # | What broke | Rule/test that prevents it |
|---|---|---|
| _none yet_ | see ROADMAP.md bugs B11/B14/B16/B17 for canonical mutation anti-patterns | add rows as bugs ship |

## Doc index
| Topic | File |
|---|---|
| Architecture, backend/frontend, DB schema & rules | [ARCHITECTURE.md](./ARCHITECTURE.md) |
| Panel governance & personas, product roles | [PANEL_GOVERNANCE.md](./PANEL_GOVERNANCE.md) |
| Design spec, onboarding, UI/UX | [DESIGN_SPEC.md](./DESIGN_SPEC.md) |
| Security, auth, secrets | [SECURITY.md](./SECURITY.md) |
| Penny prompt & personality | [PENNY_PROMPT.md](./PENNY_PROMPT.md) |
| Household narrative / data flywheel | [11_STAR_NARRATIVE.md](./11_STAR_NARRATIVE.md), [DATA_FLYWHEEL.md](./DATA_FLYWHEEL.md) |
| Roadmap & bugs / current sprint | [ROADMAP.md](./ROADMAP.md), [CURRENT_SPRINT.md](./CURRENT_SPRINT.md) |
| Rebrand sprint (Thriftly→Groflo) | [GROFLO_REBRAND_SPRINT.md](./GROFLO_REBRAND_SPRINT.md) |
| Brand portal (B2B) | [frontend-brand/AGENTS.md](./frontend-brand/AGENTS.md) |
| Marketing site | [marketing/AGENTS.md](./marketing/AGENTS.md) |
