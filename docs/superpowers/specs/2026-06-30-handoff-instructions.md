# Handoff — pennyai/Groflo (2026-06-30)

Self-contained context for a fresh session. Repo: `/Users/andydownard/projects/pennyai` (rallytasks/pennyai). Go backend at root, `frontend/` (consumer React 18/Vite 5), `frontend-brand/` (React 18/Vite 5), `marketing/` (React 19/Vite 7). Read `CLAUDE.md` first.

## Deploy model (important — learned this session)
- Flow: `main → staging → production` (git branches). **The consumer app's Railway service `groflo` (rally project, id `fd4fefc7-bc32-4d6b-ad9d-89f6a80a5f4d`) deploys the `production` branch**, not `main`. Builder = RAILPACK. Auto-deploy + "Wait for CI" are on. Serves `app.groflo.io`. pennyai's DB = `Postgres-mpxU` in the rally project.
- **Merging to `main` does NOT deploy.** To ship: `git push origin origin/main:staging` then `git push origin origin/main:production` (fast-forward). There is no separate groflo *staging* Railway deploy, so "verify on staging" is effectively CI-green on the commit + a post-deploy production healthcheck.
- `/health` returns a STATIC `version` string, not a build SHA. Verify a deploy via Railway deployment status (`mcp__railway__list_deployments`, env `production`) reaching SUCCESS for the commit, not via `/health`.
- The legacy **`penny` Railway project** (id `1e994e8a-...`) is a dead orphan that used to spam build-failure emails; it was disconnected this session (watch-pattern set to never-match on both its envs). Don't reconnect it.

## What shipped this session (all on `main`, promoted to `production` 2026-06-30)
- **UX polish** (#115): 30% community-share copy, optimistic multi-item add, dropped blocking onboarding modal, "snap your receipt" nudge, savings/earnings money-hero, swipe + auto-add haptics, "Start from your usual", actionable trip banner, plus perf (deferred analytics, parallel list load, memoized `ItemRow`) and a shared `haptics.ts`. Frontend only.
- **Backend refactor, safe batch** (#116): added the missing `middleware_timpager` (MONEY/auth) + Watt tests, two DB indexes (`idx_swap_events_deal_id`, `idx_item_modifications_user_id`), `penny_context_cache` read path, split `handlers_metrics.go` → `metrics_jobs.go`, adopted `dbError`/`decodeJSON` in social/referral/earnings, removed the dead AI orchestrator in `ai_providers.go`, cleared SQLite-era cruft. Full `go test ./...` green.

## Open / not yet merged
- **PR #117** — Uptime Monitor fix (repoint dead `app.thrift.ly` → `app.groflo.io`, add `issues: write`). Just needs review + merge. The redeploy step still uses `secrets.RAILWAY_SERVICE_ID/ENV_ID` — confirm those target the live `groflo` service, not the disconnected `penny` one.
- The discarded clerk WIP is recoverable in `git stash@{0}` ("overnight-safety-2026-06-28"). Do not resurrect it (it failed closed → lockout risk).

---

## NEXT JOB 1 — Implement Clerk (satellite daughter). Infra is DONE (Andy registered the satellite, DNS/CNAME, paid plan, origins).

Plan: **`docs/superpowers/specs/2026-06-30-clerk-satellite-plan.md`** (read it fully). Architecture = ratified rally-wide Clerk authority (timpager memory `project_clerk_auth_authority_architecture`).

**The shape:** pennyai is a cross-apex Clerk **satellite**. Frontend mounts a satellite `ClerkProvider`; sign-in happens on the primary (`clerk.rallytasks.com`); backend **verifies the Clerk session JWT networklessly via JWKS (NO Clerk secret)**, calls timpager `POST /api/v1/identity/resolve` to get the canonical `rally_user_id`, upserts the local `users` row, and **mints pennyai's own session** so all 4 legacy auth methods + every handler stay unchanged. Ships dark behind `CLERK_ENABLED` (backend) + `VITE_CLERK_PUBLISHABLE_KEY` (frontend). Never lock out.

**Reference implementations (study these — don't reinvent):**
- `/Users/andydownard/projects/projections` — a real cross-apex Clerk satellite (Python/React, no secret). Frontend: `web/src/auth/ClerkAuthProvider.tsx` (satellite props), `ClerkTokenBridge.tsx`/`clerkToken.ts`/`api/authHeader.ts` (getToken→Bearer), `OwnerAuthControls.tsx` (`buildSignInUrl`). Backend verify: `app/clerk/verify.py` (RS256-only, mandatory iss, azp allowlist, fail-closed; boot-refuses Clerk secret).
- `/Users/andydownard/projects/timpager` — the Clerk authority. Reusable daughter verifier `lib/clerk-jwt-verify.js`; resolve API `routes/identity-v1.js` (`POST /api/v1/identity/resolve`, `GET /api/v1/identity/by-clerk`). **Confirm the exact request/response shape of resolve against this code before coding pennyai's call.**
- Existing `clerk.go` on pennyai `main` — `VerifyClerkRequest` already returns pennyai's `Session` and fails open to legacy. Extend this; don't replace it. There's a `LocalUserResolver` seam not yet wired.
- Clerk skills installed at `.agents/skills/clerk-*` (use `clerk-react-patterns`, `clerk-backend-api`, `clerk-setup`). Clerk CLI not installed (`npx` it if needed). No Clerk MCP exists.

**Implementation steps (TDD, gated dark, 4-tests-per-mutation on the exchange path):**
1. DB: add nullable `users.clerk_user_id TEXT UNIQUE` + index (additive; no-op while dark).
2. Backend (Go): networkless JWKS verify via `clerk-sdk-go/v2` jwt.Verify (already an indirect dep in `go.mod` — promote it) OR port timpager's verifier. RS256-only; `iss=https://clerk.rallytasks.com` mandatory; require `exp`+`sub`; `azp` allowlist (`CLERK_AUTHORIZED_PARTIES`); cache JWKS by `kid`, fail-closed. **Boot-refuse if `CLERK_SECRET_KEY` or `CLERK_WEBHOOK_SIGNING_SECRET` is set.** Cookie-blind (read only `Authorization: Bearer`, never `__session`).
3. Bridge: verify → timpager resolve → upsert `users(id=rally_user_id, clerk_user_id)` (idempotent, `IS NULL`-guarded against takeover) → mint pennyai session. **Existing-user mapping (phone/google_sub match → same rally_user_id) is the tenant-critical step; pin a negative cross-tenant "no-merge" test.**
4. Frontend: lazy, kill-switched satellite `ClerkProvider` (`isSatellite`, `domain`, `signInUrl`, `allowedRedirectOrigins`); only `SignedIn`/`SignedOut`/`UserButton`; `clerk.buildSignInUrl()` (never hardcode); bridge `getToken()`→`Authorization: Bearer` into the existing `fetchAPI`. No embedded sign-in form. Provider code-split out when `VITE_CLERK_PUBLISHABLE_KEY` unset.
5. Env (names): backend `CLERK_ENABLED`, `CLERK_JWT_ISSUER`, `CLERK_AUTHORIZED_PARTIES`, `CLERK_PUBLISHABLE_KEY` (forbid the two secrets). Frontend `VITE_CLERK_PUBLISHABLE_KEY`, `VITE_CLERK_DOMAIN`, `VITE_CLERK_SIGN_IN_URL`, `VITE_ALLOWED_REDIRECT_ORIGINS`.

**Gotchas:** issuer host (`clerk.rallytasks.com`) ≠ browser FAPI host (`clerk.groflo.io`); publishable key is baked at build time (enabling needs a rebuild, disabling is just `CLERK_ENABLED=0`); a valid Clerk JWT ≠ authorized here, always resolve to a local row (the vibe101 "__session cookie bleeds across .rallytasks.com" trap); `azp` allowlist mandatory; no passkeys cross-apex. Webhooks/provisioning live in timpager, NOT pennyai.

**Do NOT do without Andy:** flip `CLERK_ENABLED=1` in prod (his call; reversible to 0); remove/weaken any of the 4 legacy methods.

---

## NEXT JOB 2 — Continue the backend refactor (disaster-surface phase). Supervised.

Plan: **`docs/superpowers/specs/2026-06-28-groflo-refactor-design.md`** (Phases 3–4 + the Phase 0 Postgres tier remain). The safe batch (this session) was Phases 0–2 partial.

Order (tests-first; these are money/SMS/schema disaster surfaces — do carefully, not blind-parallel):
1. **Postgres integration test tier** (testcontainers) — the keystone that gates the consolidations. Today the whole Go suite runs on single-connection in-memory SQLite via `pgcompat`, so TOCTOU/concurrency is never exercised.
2. **One `sendSMS(to, body, idempotencyKey)` choke point** — route the 4 scattered Twilio sends through it; pin each with a double-send negative test FIRST (OUTBOUND, damage risk #1).
3. **One idempotency primitive** + fix the `handlers_brand_payments.go` TOCTOU (reproduce the double-credit with a failing test first) (MUTATION/MONEY).
4. **Phase 4 structure:** split `schema.go`'s 1,300-line `initDB` into domain helpers + `schema_seed.go` (preserve statement order); collapse the 4 FeedbackWidgets to the hosted FAB; the `@groflo/ui` shared lib + React-19 alignment (Andy chose "full shared UI library" — overrides the brand-portal contract; record the `Override`); decompose both `App.tsx` files; land the `thriftly → groflo` module rename.

Decisions Andy already locked (see the design doc §11): React 19 across all three apps; stacked PRs per phase; testcontainers; start off `main`.

---

## Verification commands
- Backend: `go test -count=1 ./...` (note: `TestBuildIntegrity_IndexReferencesExist` fails locally if `static/` is polluted by a stray `vite build` — rebuild `frontend` to make `static/` consistent, or trust CI).
- Frontend: `cd frontend && npx tsc --noEmit && npx vitest run && npx vite build`.
- Deploy: `mcp__railway__list_deployments` (rally project, groflo service, env `production`) → wait for SUCCESS on the commit.
