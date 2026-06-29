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

- Worked on branch `overnight/ux-2026-06-28`; your clerk WIP left uncommitted + untouched; full safety snapshot in `stash@{0}`.
- 30% wording: used your exact line, fixed sentence case ("In addition…"), applied to both EarningsScreen spots.
- Savings badge was duplicated byte-for-byte in two header variants; updated both via `replace_all` (noted as a DRY candidate for the refactor).
- Deferred analytics fully (not just init): made Sentry/posthog dynamic in analytics.ts + ErrorBoundary too, and dropped the `observability` manualChunks entry so they become async chunks. Verified built index.html no longer preloads them.
- Fixed vitest config (`include: src/**`) — it was crashing on Playwright e2e specs. Pre-existing bug, safe fix.

## Progress (completed + verified)

| Item | Status | Verification |
|---|---|---|
| 1 — 30% share wording | done | typecheck clean; pending visual QA |
| 5 — savings figure hero | done | typecheck clean; pending visual QA |
| 6 — swipe commit haptic | done | typecheck clean |
| 7 — auto-add ring haptic | done | typecheck clean |
| 8 — earnings Total hero | done | typecheck clean; pending visual QA |
| perf — parallel list load | done | typecheck clean |
| perf — defer analytics | done | build: critical JS ~1.04MB→~393KB, observability not preloaded |
| perf — memoize ItemRow | done | typecheck + build clean |
| DRY — shared haptics.ts | done | typecheck clean (3 dupes removed) |
| test-net — vitest scope fix | done | `vitest run` → 34 passed, e2e excluded |

Commits on `overnight/ux-2026-06-28`: docs, 30% copy, haptics, money-hero, perf+testnet.

## Verification results

- `npx tsc --noEmit` → clean (run after each batch)
- `npx vite build` → success; eager preloads reduced to index + vendor only
- `npx vitest run` → 34 passed / 1 file
- Visual QA (screenshots) pending — see "Still to do".

## Feature items — all done

| Item | Status | Verification |
|---|---|---|
| 2 — optimistic item arriving (multi-item) | done | tokenizer 9 unit tests; typecheck+build; SERVER-safe design (separate state, cleared on completion) |
| 3 — let people type first (drop onboarding modal) | done | **live DOM**: home shows empty state, no blocking modal, input usable |
| 4 — earn-loop "snap your receipt" nudge | done | typecheck+build; live: nudge correctly absent with no pending data |
| 9 — "Start from your usual" chip | done | typecheck+build; chip gated on existing master list |
| 10 — actionable trip banner (frontend) | done (frontend) | typecheck+build; ping-shopper deferred (needs backend send path — OUTBOUND) |

## Final verification (all green)

- `npx tsc --noEmit` → CLEAN
- `npx vitest run` → **43 passed** (34 navigation + 9 parse-utils)
- `npx vite build` → success; critical preload = index + vendor only (analytics deferred)
- **Live app** (backend on :8080 in TEST_MODE against local pennyai DB, serving the production build): logged in, verified via DOM:
  - Item 1: earnings copy reads "Brands pay me to reach you. In addition to a rebate, 30% of what they pay me comes back to you — real cash."
  - Item 3: home renders the empty state with NO blocking onboarding modal; Penny input immediately usable.
  - Item 8: earnings shows "$0.00 / Total earned" as the hero with Pending + Paid as the secondary row.
- Test backend stopped afterward; created one local test user (5551234567) in the local DB only.

### What I could NOT screenshot
The Chrome screenshot tool in this environment has a bug (`clip.scale` deserialization) — pixel capture failed every attempt, and the window reported a 0×0 viewport so coordinate clicks were unreliable. I verified content via the accessibility/DOM tree instead. **Please eyeball these at mobile (375×667) + desktop when you're back:** savings-figure hero (home), earnings Total hero, optimistic shimmer rows on a multi-item add, the receipt nudge (needs pending > 0), the "Start from your usual" chip (needs a saved "My Groceries"), and the trip banner "+ Add for them" (needs another member shopping).

## Commits (branch `overnight/ux-2026-06-28`, off feat-clerk-phase2b)

```
483e202 docs: refactor design spec, UX panel proposal, overnight work log
5ed37bf feat(ux): name the 30% community share on the earnings screen
d7cb6e3 feat(ux): tactile haptics on swipe + auto-add ring; dedupe haptic helper
6829b62 feat(ux): make the money the hero on home + earnings
2223d12 perf: defer analytics off critical path, parallelize list load, memoize rows
003d879 feat(ux): let users type first; add 'Start from your usual' shortcut
6fc693d feat(ux): standing 'snap your receipt' nudge for pending earnings
a4c601d feat(ux): make another member's in-progress trip actionable
f5f9dd5 feat(ux): optimistic "item arriving" rows instead of freezing the list
```

Nothing pushed. Your clerk Phase 2b WIP is still uncommitted and untouched on the working tree; full safety snapshot in `stash@{0}`.

## How to review / run
- See the changes: `git switch overnight/ux-2026-06-28` then `git log --oneline main..HEAD`.
- Run locally: `cd frontend && npm run dev` (or the build is already in `static/`, served by the Go backend).
- If you want to discard everything I did: `git switch feat-clerk-phase2b` (your WIP is right where you left it). The overnight branch can be deleted.

## Refactor program status
Not started tonight — it's backend-heavy and collides with your uncommitted clerk WIP (`middleware_timpager.go`, `schema.go`, `session.go`, `pgcompat.go`), and several phases touch money/SMS/schema disaster surfaces I won't rewrite unsupervised. The plan is ready in `2026-06-28-groflo-refactor-design.md`. To start it we first need to settle the branch base (main is 6 commits behind this branch and missing #59 which the refactor depends on) and what to do with the clerk WIP.
