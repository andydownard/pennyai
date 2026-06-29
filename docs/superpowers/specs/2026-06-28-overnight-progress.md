# Overnight work log — 2026-06-28 → morning

Andy went to bed and said "keep going overnight." I'm working autonomously. This log is the morning report. Updated as I go.

## Setup decisions (autonomous, conservative)

- **Your clerk Phase 2b WIP is preserved and untouched.** It builds (`go build ./...` OK). It stays uncommitted in the working tree. I am not editing any file it touches: `schema.go`, `middleware_timpager.go`, `session.go`, `pgcompat.go`, `server.go`, `handlers_digest.go`, `handlers_metrics.go`, `models.go`, `go.mod`, `go.sum`, the modified `*_test.go`, `clerk_auth*.go`, `.env.example`, `DEPLOYMENT.md`, `README.md`.
- **Full safety snapshot** captured at `stash@{0}` ("overnight-safety-2026-06-28") — everything recoverable, tree verified unchanged after snapshot.
- **Working on branch `overnight/ux-2026-06-28`** (off `feat-clerk-phase2b` tip, so it has #59 + marketing port). I commit my work here in logical units. I do **not** push or open PRs.
- **Scope tonight:** the UX queue you dictated (all frontend) + the frontend-safe refactor perf/DRY wins. **Deferred** (documented below): anything touching your WIP files, the disaster-surface refactor (SMS/idempotency/money/schema/federation), and the full `App.tsx` decomposition.

## Verification approach

- `npm run build` (tsc + vite build) after each batch — must pass.
- `npx vitest run` for unit logic.
- Screenshots at 375×667 (mobile) + desktop for visual changes, against the running stack (your vite dev server is up on :5173; I'll bring the Go backend up on :8080 against local Postgres).
- I will NOT mark a visual item "done" without a screenshot. Items verified by build/typecheck only are marked "pending visual QA."

## UX queue (your picks)

| # | Item | Files | Size | Status |
|---|---|---|---|---|
| 1 | Name the 30% community share | EarningsScreen, SwapCard | S | done (copy) — pending visual QA |
| 2 | Optimistic "item arriving" (handle multi-item) | App.tsx, useChatState | M | pending |
| 3 | Let people type first / onboarding coachmarks | OnboardingOverlay, App.tsx | M | pending |
| 4 | Close earn loop: "snap your receipt" pending nudge | useSwapState, EarningsScreen, App.tsx | M | pending |
| 5 | Savings figure hero; stop dimming "saved" | App.tsx:2787 | S | pending |
| 6 | Commit-threshold haptic on swipe | ItemRow.tsx | S | pending |
| 7 | Auto-add ring haptic | PennyBar.tsx | S | pending |
| 8 | Earnings Total hero; demote Pending/Paid | EarningsScreen.tsx | S | pending |
| 9 | "Start from your usual" on empty list | App.tsx, SavedListsScreen, DoneSummaryModal | M | pending |
| 10 | Actionable household trip banner | App.tsx:3001 | L | pending (frontend part; backend deferred) |

## Refactor — frontend-safe wins (alongside UX)

| Item | Files | Status |
|---|---|---|
| Extract shared `haptic` helper (dup'd 3×) | new `haptics.ts` | pending |
| Memoize `ItemRow` + `useCallback` handlers | ItemRow, App.tsx | pending |
| Move typewriter to leaf so it doesn't re-render the tree | PennyBar | pending |
| `reloadList` → `Promise.all` | useListState | pending |
| Defer Sentry/posthog past first paint | main.tsx | pending |
| Fix `navigation.test.ts` to import real constants | navigation.test.ts | pending |

## Deferred to supervised time (with reasons)

- **Backend refactor phases 0/1/3 + schema split (Phase 4)** — collide with your uncommitted clerk WIP and/or touch money/SMS/DB disaster surfaces. Not safe to do unsupervised.
- **Full `App.tsx` decomposition (Phase 4)** — XL, would conflict with tonight's UX edits; needs review.
- **Trip-banner backend (item 10)** — new endpoint + TENANT isolation test; I'll do the frontend affordance defensively and leave the wire-up for review.
- **Cruft deletes (archive/, litestream, etc.)** — DESTRUCTIVE; behind your verify gate.

## Decision log

- (will append each non-trivial call I make)

## Verification results

- (will append build/test/screenshot evidence per item)
