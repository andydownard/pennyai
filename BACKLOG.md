# Groflo Backlog

**Last updated:** 2026-04-16

---

## P0 — Ship Now

| # | What | Why | Size |
|---|------|-----|------|
| _none_ | _G2 PostgreSQL test coverage in progress — see CURRENT_SPRINT_ | — | — |

## P1 — Ship This Sprint

| # | What | Why | Size |
|---|------|-----|------|
| G4 | Brand portal campaign lead hand-off | marketing/campaign-lead.mts redirects to brands.groflo.io but portal doesn't read params. | S |
| G5 | Data migration: SQLite → Postgres | Script ready (scripts/migrate_sqlite_to_postgres.sh). Need to execute against production. | M |
| G6 | Deploy pennyai to timpager Railway canvas | Service created, vars set. Need source repo connected + first deploy. | S |
| G7 | Email campaign CRM integration | 13 templates ready in marketing/emails/. Need Klaviyo/SendGrid wiring. | M |
| G22 | Marketing Home.tsx body copy + alt text sweep | Hero h2 fixed; body copy at line ~724 still reads as B2B SaaS. PMM/Brand PM pass. | S |
| G23 | Twilio webhook redelivery dedup | `sendSMSReply` in handlers_webhook.go:371,377,408,470,476,494 fires without message-ID dedup. Twilio retries on timeout → duplicate SMS. Store `twilio_message_id` with UNIQUE before reply. | M |

## P2 — Next Sprint

| # | What | Why | Size |
|---|------|-----|------|
| G8 | Saved List Discovery Nudges | Post-trip save prompt, repeat-item detection, new member onboarding. Roadmap item 10. | M |
| G9 | Auto-Generated "Your Regulars" List | System-maintained saved list from purchase frequency. Roadmap item 11. | M |
| G10 | AI cost tracking per model/provider | Multi-provider system needs cost visibility. Log tokens, model, provider per call. | M |
| G11 | R2 receipt image storage | Store receipt images in Cloudflare R2 instead of BLOBs. Keys configured. | M |
| G12 | Brand portal fake waitlist fix | ForBrands.tsx form POSTs nothing. Wire to campaign-lead.mts. | S |
| G13 | Department classification improvements | Too many items going to "Other" category. User feedback. | M |
| G14 | Quantity on list view | User feedback: show quantity inline on list items. | S |
| G15 | Group shopping chat notifications | Send list link to household group chat on updates. | L |
| G16 | Go module rename thriftly → groflo | Internal identifier sweep. Phase 5. | S |
| G24 | pgcompat.go unit test coverage | Subtle cases (placeholder counting in string literals, negative datetime offsets, multi-column ON CONFLICT). Round-trip SQLite→PG conversions. | S |
| G25 | Visual regression suite | Playwright `toHaveScreenshot()` baselines for 3 marketing pages + 5 consumer screens. Catches silent layout regressions like the whitespace bug. | M |
| G26 | Brand consistency lint | `pre-commit` hook or `npm run lint:vocab` that fails on user-facing strings containing forbidden vocabulary (`AI assistant`, `chatbot`, `bot`, `Thriftly`, `thrift.ly`) outside test files. | S |
| G27 | Comprehensive idempotency audit | Top-5-Damage-Risks #1. Every state-mutating handler needs dedup guard verified with negative-path test. PG-migration audit only covered the regression surface. | L |

## P3 — Future

| # | What | Why | Size |
|---|------|-----|------|
| G17 | Brand testimonial compliance | ForBrands.tsx testimonials may be composites. Get consent or add disclosure. | S |
| G18 | Supabase migration for marketing site | Run 002_campaign_leads.sql before deploy. | S |
| G19 | Circuit breaker for external services | Claude/Twilio/SendGrid down → fail fast. Timpager has B116 for same. | M |
| G20 | Kroger MCP integration | Product images, correct logos, SKU-level data. | L |
| G21 | Spanish language MVP | Multi-lingual Penny. CST solidarity principle. | L |

## Bugs (Open)

_None currently open._

## Shipped

| # | What | When |
|---|------|------|
| — | PostgreSQL migration (SQLite → Postgres, shared timpager users table) | 2026-04-13 |
| — | Multi-provider AI (Anthropic → Gemini → OpenAI fallback, model tiering) | 2026-04-13 |
| — | Monorepo consolidation (thrift.ly → marketing/) | 2026-04-13 |
| — | penny.rallytasks.com URL support | 2026-04-13 |
| — | main/staging/production branch strategy | 2026-04-13 |
| — | Repo transfer to rallytasks org | 2026-04-13 |
| — | AI-powered receipt fraud protection (11-point check) | 2026-04-13 |
| — | Brand fraud protection marketing copy | 2026-04-13 |
| — | R2/Cloudflare keys configured | 2026-04-13 |
| — | Email campaign templates extracted from thrift.ly | 2026-04-13 |
| G1 | Prompt injection sanitization + output filtering (sanitizeAIInput, sanitizeAIOutput, boundary markers, HTML strip, field validation) | 2026-04-13 |
| G3 | Saved List Sharing via Link (fork/copy, brand fidelity, merge dedup, 12 tests) | 2026-04-13 |
| — | Phone column rename (`phone_number` → `phone`) across all handlers + tests — schema owned by timpager | 2026-04-16 |
| — | PG idempotency fixes: `INSERT OR IGNORE` → explicit `ON CONFLICT (...) DO NOTHING` on auth_methods, savings_milestones, swap_propagations | 2026-04-16 |
| — | PG case-sensitive search: 9 sites migrated `LIKE` → `ILIKE` with `sqliteCompat()` rewriter for SQLite test path | 2026-04-16 |
| — | PG `Atoi` on UUID path removed from `handleAvatarGet` | 2026-04-16 |
| — | Marketing whitespace bug (useOnScreen default `visible=true` for above-fold) | 2026-04-16 |
| — | FeedbackWidget duplicate-render guard | 2026-04-16 |
| — | Email template rebrand-direction fix (`groflo-transition.html`) + CAN-SPAM physical address | 2026-04-16 |
| — | Brand vocabulary drift sweep (useAuthState, PennyBar, PennySettingsScreen, ShareScreen, marketing Home + ForBrands) | 2026-04-16 |
| — | `INSERT OR REPLACE` panic guard in `pgcompat.go` (silent strip previously could turn upserts into duplicate-key crashes) | 2026-04-16 |
| — | Regression test file `regression_pg_migration_test.go` (5 tests: idempotency x3, ILIKE, avatar UUID) | 2026-04-16 |

## Not In Scope

| # | What | Why |
|---|------|-----|
| — | Trip Planner | Convenience-driven, not coordination-driven. Doesn't pass Gate 2. |
| — | Department Placement (CPM ads) | Highest trust risk. Erodes Penny's value proposition. |
| — | Dark mode | Doesn't pass Gates 2/3. |
