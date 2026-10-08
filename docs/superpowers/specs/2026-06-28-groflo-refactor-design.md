# Groflo / Penny — Massive Refactor Design Spec

**Date:** 2026-06-28
**Author:** Penny engineering (Claude, ultracode) for Andy's review
**Status:** FINAL (decisions locked 2026-06-28) — awaiting Andy's read-through before generating the Phase 0 implementation plan
**Grounded in:** a 10-agent discovery audit (6 code dimensions + Google/Apple/Shopify UX panel), 48 findings, 22 UX suggestions, all tied to real `file:line` references.

---

## 1. Goals (your six asks → what we will do)

| Your ask | What the audit found | What this plan does |
|---|---|---|
| Make the code simpler | Two monolith `App.tsx` files (4,079 + 1,622 lines); a 1,300-line `initDB`; oversized handler files | Decompose into router + screens; split `initDB` and big handlers; one AI choke point |
| Eliminate cruft | SQLite-era debt (committed WAL/SHM, dead `litestream.yml`/`run.sh`, two migration scripts, stale `archive/`, unfinished module rename) | Remove it all (PG cutover is confirmed complete) |
| Increase testing | Good volume (~1.6×) but weak fidelity: whole Go suite runs on in-memory SQLite; `middleware_timpager.go` has **zero** tests; brand portal has no test infra | Stand up a Postgres integration tier; fill the disaster-rule gaps first |
| Reduce duplicative components to ONE | Feedback widget exists **four ways**; brand color defined three ways; 9 modals re-roll their own sheet | Collapse to one hosted FAB + one shared UI library + one `BottomSheet` |
| DRY everywhere | Two AI orchestrators (one dead); 78 inline 500s; 4 scattered SMS sends; 3 idempotency styles | One AI path, shared helpers, one `sendSMS`, one idempotency primitive |
| Secrets never on frontend; backend hosts all logic | **Already PASS** (verified) | Confirm and keep it that way; one money-path nit to close |
| Make the app faster | Per-character typewriter re-renders the 4k-line tree; missing index; sequential waterfall; analytics block first paint | Memoize, add indexes, parallelize, defer analytics |
| Google / Apple / Shopify UX panel | 22 grounded suggestions delivered | Delivered as a **proposal doc** for your review (see companion file) |

---

## 2. Scope decisions (your answers, 2026-06-28)

1. **Aggressiveness:** Full program, sequenced **tests-first**.
2. **Postgres cutover:** Complete — safe to clean (we still add a one-time verification gate before destructive deletes, per the `DESTRUCTIVE` rule).
3. **Shared code:** **Full shared UI library** — including shared React components (`@groflo/ui`). This requires an `Override` of the brand-portal contract and a React-version decision (see §4).
4. **UX changes:** Proposal only. No UX implementation in this program; panels convened later before any user-facing build.
5. **React strategy:** Option A — align all three apps to React 19 (see §4).
6. **Execution model:** Stacked PRs per phase to `main`, merge-only behind green CI, each healthcheck reporting the new build SHA.
7. **Postgres test tier:** testcontainers (hermetic), kept alongside the fast SQLite unit tier (see §8).
8. **Settled:** record the `Override` of the brand-portal "no shared React components/tokens" rule; start the work fresh off `main` (not `feat-clerk-phase2b`).

So the **implementation program is Phases 0–4** (code, structure, perf, tests, one-component, rename). UX ships as a reviewed document, not code.

---

## 3. Verified facts that shape the plan

- **SECRETS — PASS.** No Anthropic/Twilio/Stripe/Places/LogoDev/timpager secret in any frontend source, `index.html`, vite config, or built `dist`/static HTML. Only public-by-design values are embedded (Sentry DSN send-only, GA IDs, PostHog project key, Google OAuth `client_id` served from `/api/config`).
- **SERVER — PASS.** Client is UX-only. `isAdmin` only gates nav; `requireAdmin` re-checks session phone server-side. Money is display-only `*_cents`; deposit minimums and Stripe amounts are server-set. SMS opt-in is re-gated server-side. All frontends call same-origin only.
- **One money nit (not a SERVER fault):** the no-Stripe-key "test mode" credits `brand_balance` before payment (`handlers_brand_payments.go:219`). Must boot-refuse on a missing Stripe key in prod.
- **React skew is real:** consumer + brand portal are **React 18 / Vite 5**; marketing is **React 19 / Vite 7 / react-router 7**.
- **Brand-portal contract forbids sharing:** `frontend-brand/AGENTS.md:24,35` — "NOT shared: React components, design tokens" and "Do NOT ... share React components." Your "Full shared UI library" choice overrides this; we record the `Override` explicitly.

---

## 4. Key design decision — the React strategy (LOCKED: A)

A shared `@groflo/ui` React component library cannot straddle React 18 and 19 cleanly. Three options:

- **A. Align all three to React 19 (recommended).** Marketing is already there. Bump consumer + brand to 19, then one `@groflo/ui` works everywhere. Cost: a React 18→19 migration on the 4,079-line consumer app (mostly mechanical; some `react-dom` API and types churn), pinned by tests + screenshots. Cleanest long-term; removes the skew permanently.
- **B. Scope `@groflo/ui` to consumer + brand (both React 18); exclude marketing.** Marketing keeps React 19 and consumes only `@groflo/tokens` + `@groflo/utils`. Lower risk, but the shared component lib does not reach the marketing site, so it is not the *full* library you asked for.
- **C. Build `@groflo/ui` with a permissive `react` peerDependency and test against both 18 and 19.** Avoids the migration but is fragile (hooks/runtime differences, double test matrix) and the riskiest to maintain.

**DECISION: A (locked 2026-06-28).** It is the only option that delivers a genuinely shared UI across all three apps, which is what "full shared UI library" means. The React 18→19 bump of the consumer + brand apps becomes a gated step at the start of Phase 4, pinned by render tests + desktop/mobile screenshots before the shared library lands.

---

## 5. Target architecture

```
pennyai/                      # root: add pnpm-workspace.yaml
├─ packages/
│  ├─ tokens/                 # @groflo/tokens — framework-agnostic: tokens.css + tokens.ts (one brand green #35644F, scales)
│  ├─ utils/                  # @groflo/utils — pure TS (formatCents/formatPhone/normalizePhone) + ErrorBoundary (react peerDep)
│  └─ ui/                     # @groflo/ui — shared React components (BottomSheet, Button, Card, inputs, ConfirmModal...)
├─ frontend/                  # consumer app — App.tsx → shell + ScreenRouter + components/screens/*
├─ frontend-brand/            # brand portal — App.tsx → shell + screens/* (records Override to consume @groflo/*)
├─ marketing/                 # Groflo site
└─ *.go                       # backend: module renamed thriftly → groflo; initDB split; metrics_jobs.go etc.
```

Feedback widget is **not** a package — it becomes one SRI-pinned `<script>` tag from the timpager-hosted FAB (`rallytasks.com/shared/feedback-fab@1.js`); AGENTS.md says copy, do not fork.

---

## 6. Phases

Each phase lists the RED tests that gate it, the disaster-rule guards, and acceptance criteria. Phases are merge-only PRs to `main` behind green CI (`GIT`).

### Phase 0 — Pin & clean (no behavior change)

**Tests first (these gate every risky phase that follows):**
- `middleware_timpager_test.go` (new): golden idempotency-key format (`pennyai_*` / `groflo_payout_*`), Authorization header on every push, 409-as-success-no-retry, backoff cap, non-positive/empty-distribution refusal. *(MONEY)* — the `verifyTimpagerSession` cases are gone along with the dormant header-forwarding SSO bridge, deleted unused in the top-10 code-review pass (see GROFLO_REBRAND_SPRINT.md Phase 3 "Current state").
- **Postgres integration test tier** (testcontainers recommended, or a CI Postgres service): run the disaster-rule mutation paths against real Postgres with concurrent connections — `handleSwapAction` atomic UPDATE + its TOCTOU guard cannot exhibit their race on today's single-connection SQLite. *(MUTATION)*
- SMS double-send negative test on each of the 4 send sites — `handlers_digest.go:120`, `handlers_social.go:123`, `handlers_webhook.go:412`, `session.go:217`. *(OUTBOUND, damage risk #1)*
- `handlers_brand_payments.go` TOCTOU: a failing test reproducing the double-credit before any fix. *(MUTATION/MONEY)*
- `handlers_watt.go`: `requireAdmin` auth-negative test (these return person PII). 
- Repair the test net: fix `frontend/src/navigation.test.ts` to import the real constants (it redeclares them today); fix `playwright.config.ts` to point at a test Postgres (`DATABASE_URL`, not the stale `DB_PATH`).

**Cleanup (authorized — cutover complete; one verification gate first):**
- **Verify gate:** confirm Railway start command is `./groflo` (not `run.sh`) and prod is on pennyai Postgres, before any delete. *(DESTRUCTIVE)*
- `git rm --cached thriftly.db-wal thriftly.db-shm` (tracked despite `.gitignore`).
- Delete `litestream.yml` + `run.sh`; fix the doc drift they leave in `CURRENT_SPRINT.md`, `DEFINITION_OF_DONE.md`, `DEPLOYMENT.md`.
- Remove the stale `scripts/migrate_sqlite_to_postgres.sh`; keep the idempotent `.py`.
- Move `archive/*-2026-03-09.md` out of the working tree (git history preserves them).

**Acceptance:** full Go suite + new Postgres tier green; every disaster-rule surface above pinned by failing-then-passing tests; working tree clean; docs match reality.

### Phase 1 — Additive performance wins (all safe)

- `CREATE INDEX IF NOT EXISTS idx_swap_events_deal_id ON swap_events(deal_id)` (or `(deal_id, action)`) — `schema.go:580`. Kills seq-scans across brand intel, metrics, digests, budget checks. *(DB, additive)*
- `idx_item_modifications_user_id` (or `(user_id, action)`) — `schema.go:572`.
- `reloadList` → `Promise.all([list, swaps])` — `useListState.ts:112`. Roughly halves list-load latency.
- Defer Sentry + posthog-js to `requestIdleCallback` / dynamic import after first paint — `main.tsx`.
- Wire the `penny_context_cache` read path (fall back to rebuild on miss); collapse 3 `users` SELECTs into 1 — `handlers_learning_context.go`.

**Acceptance:** `EXPLAIN` shows the new indexes used; list-load round-trips halved; TTI improved (measured before/after); chat-path query count down. Pin with tests where behavior is observable.

### Phase 2 — Low-risk DRY (no disaster surface)

- Delete the dead `ai_providers.go` orchestrator; make `penny.go`'s `callClaude*` the single fallback choke point; source all keys from `s.config` only; add the `AI`-rule obligations both paths skip today (promptHash log, PII redaction, token budget). *(AI)*
- Adopt `dbError` / `writeBadRequest` / `decodeJSON` in the files you touch (78 inline 500s, 14 inline 400s, 10 raw decodes) — route the 10 raw decodes through `decodeJSON` for the 1MB cap (`PATTERNS`, plus a small hardening win).
- Memoize `ItemRow` (`React.memo`) + `useCallback` its handlers + `useMemo` `itemsByDept` + move the typewriter into a leaf `<PennyBar>` so a Penny message re-renders one node, not the 4k-line tree — `useChatState.ts:128`, `ItemRow.tsx:103`.
- Small getters (`getUserCreatedAt`, `getUserPhone`); `countSwapEvents` helper; `doAnthropic` / `postJSON` transport helpers.

**Acceptance:** `ai_providers.go` orchestrator gone (leaf clients kept); suite green; render trace shows no full-tree re-render on typewriter; helpers adopted in touched files.

### Phase 3 — Disaster-rule consolidations (behind Phase 0 tests)

- One `sendSMS(to, body, idempotencyKey)` choke point; route all four sites through it; preserve each path's existing behavior (digest per-cycle claim, webhook STOP/HELP) and its tests. *(OUTBOUND)*
- One idempotency primitive: atomic conditional INSERT/UPDATE of a dedup row committed in the **same transaction** as the effect; fix the `handlers_brand_payments.go` TOCTOU and wrap the balance UPDATE with the ledger INSERT. *(MUTATION/MONEY)*
- Shared session token + cookie builder (`newSessionToken`, `setSessionCookieNamed`); keep the two stores (user vs brand) separate by design. *(SESSION)*
- Confirm the brand deposit "test mode" path boot-refuses without a Stripe key in prod. *(MONEY)*
- Then wire `middleware_timpager.go` federation live (only after its Phase 0 tests are green).

**Acceptance:** each path pinned by positive/negative/idempotency/auth on the Postgres tier; no double-send, no double-credit; federation live behind tests.

### Phase 4 — "One component" + structure + rename

- Collapse the **four** FeedbackWidgets into one SRI-pinned hosted FAB `<script>` in each `index.html`/`Layout`; delete the React copies + dead `withFeedbackFAB` wrappers (~1,300 lines). *(OUTBOUND — it posts feedback; verify the endpoint after)*
- Stand up the pnpm workspace; create `@groflo/tokens`, `@groflo/utils`, `@groflo/ui`.
- Apply the chosen React strategy (recommended: align all three to React 19). Record the `Override` of `frontend-brand/AGENTS.md` "do NOT share React components / tokens."
- Make `BottomSheet` the single sheet primitive (9 modals re-roll it); add a brand-local `ConfirmModal` replacing native `confirm()` at `frontend-brand/App.tsx:716,910`. *(DESTRUCTIVE — real confirm dialog)*
- Decompose consumer `App.tsx` (4,079) into a `ScreenRouter` + `components/screens/*` using the existing `SCREEN_PATHS`/`PATH_TO_SCREEN` tables; decompose brand `App.tsx` (1,622) into shell + `screens/*`. Pin **each** moved screen with a render test + desktop/mobile screenshot. *(no JSON or Penny-behavior change)*
- Migrate the manual URL sub-routing to Go 1.25 method+wildcard patterns, **one handler at a time** behind backward-compat route tests. *(API fence — never change method/JSON shape)*
- Split `schema.go`'s `initDB` into domain helpers + `schema_seed.go` (preserve exact statement order for FK deps); extract `metrics_jobs.go` from `handlers_metrics.go`; same split for other oversized handler files. *(DB/OUTBOUND/API — mechanical, additive)*
- Land the `thriftly` → `groflo` module rename (`go.mod`, `thriftly.db` name, dev binary) after Litestream removal.

**Acceptance:** one feedback component; workspace builds all three apps; both `App.tsx` files decomposed (shell under ~1,000 lines); zero endpoint/JSON/Penny-behavior change (backward-compat tests + screenshots prove it); rename builds and deploys with a new build SHA.

---

## 7. The "one component" consolidation (explicit)

| Today (duplicated) | Becomes | Where |
|---|---|---|
| 4× FeedbackWidget (consumer 544, marketing 641, brand inline ~130, + dupes) | 1 hosted SRI-pinned `<script>` | each `index.html` / `Layout` |
| Brand green `#35644F` defined 3 ways | `@groflo/tokens` (CSS + TS) | `styles.ts:12`, `frontend-brand/App.tsx:7`, `marketing/index.css:7` |
| `formatCents`/`formatPhone`/`normalizePhone`/`ErrorBoundary` scattered | `@groflo/utils` | multiple |
| Shared buttons/cards/inputs/sheets | `@groflo/ui` (React strategy A) | all three apps |
| 9 modals re-rolling their own overlay | one `BottomSheet` primitive | `components/BottomSheet.tsx` |
| 2 AI orchestrators (one dead) | one `callClaude*` choke point | `penny.go` |
| 4 scattered SMS sends | one `sendSMS()` | backend |
| 3 idempotency styles | one idempotency primitive | backend |

---

## 8. Test strategy

- New **Postgres integration tier** is the keystone — it exercises Postgres semantics (`ON CONFLICT`, `RETURNING`, `ILIKE`, integer division, isolation) and real concurrency that today's single-connection SQLite cannot. **Decision: testcontainers** (hermetic, no shared CI DB; needs Docker locally and in CI). Disaster-rule mutation paths run here; the fast SQLite tier stays for unit-level logic.
- Keep the existing SQLite tier for speed; add the Postgres tier for the disaster paths. (We do **not** rip out `pgcompat.go` — that native-Postgres rewrite is contract-protected long-term debt, out of scope here.)
- Four-tests-per-mutation (positive/negative/idempotency/auth) backfilled where missing: `handlePreferenceReset`, `getABVariant`, Watt endpoints, the SMS sites, brand payments.
- Frontend: add vitest to `frontend-brand` (untested 1,622-line money/auth app); cover deposit-resume + auth-failure before decomposing it. Fix the consumer's one self-referential test; repair Playwright so the e2e net actually runs.

---

## 9. Disaster-rule protections (the fences this plan respects)

- **OUTBOUND:** SMS consolidation is behind double-send tests; no refactor may trigger a send.
- **MUTATION/MONEY:** idempotency consolidation keeps the same-transaction commit; TOCTOU fixed with a failing test first; no balance update outside its movement transaction; revenue split unchanged.
- **DB/DESTRUCTIVE:** additive migrations only; cleanup gated on a verification step; preserve `initDB` statement order.
- **API:** no endpoint method or JSON shape changes without a backward-compat test; route migration is one-at-a-time.
- **AI:** the one AI choke point adds promptHash logging, PII redaction, token budget; Penny behaviors and prompt secrecy preserved.
- **TENANT:** household scoping preserved; the global unchecked-items subquery (`handlers_list.go:198`) rewritten as a scoped join, pinned by a household-isolation test.

---

## 10. Out of scope / deferred (recorded, not dropped)

- `pgcompat.go` native-Postgres rewrite (XL, contract-protected) — known long-term debt.
- `BrandLogo` merge — fenced 3-layer caching; noted, not merged.
- The 22 UX changes — proposal only; convened panels decide what to build later.

---

## 11. Decisions (locked 2026-06-28)

1. **React strategy:** A — align all three apps to React 19.
2. **Execution model:** stacked PRs per phase to `main`, merge-only behind green CI.
3. **`Override` recorded:** the brand-portal "no shared React components/tokens" rule is overridden; to be written into `frontend-brand/AGENTS.md` during Phase 4.
4. **Postgres test tier:** testcontainers.
5. **Branch base:** start fresh off `main` (not `feat-clerk-phase2b`).

---

## 12. Acceptance criteria (done when)

- All six asks met: code simpler (both `App.tsx` decomposed, big files split), cruft gone, testing increased (Postgres tier + filled gaps + brand-portal tests), duplicative components collapsed to one (FAB + `@groflo/ui` + `BottomSheet`), DRY (one AI path, shared helpers, one `sendSMS`, one idempotency primitive), secrets/server confirmed clean and kept that way, app faster (indexes, memoization, parallel load, deferred analytics — measured).
- Full Go suite + Postgres tier + frontend (vitest) + Playwright green; **0 failures, 0 new skips**.
- No endpoint method/JSON shape changed without a backward-compat test; all Penny behaviors preserved; every disaster rule intact.
- Each phase shipped as a merge-only PR with green CI and a healthcheck reporting the new build SHA.
