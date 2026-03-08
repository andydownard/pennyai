# PennyAI — Sprint 9 (March 15-22, 2026)

> This file is read by Claude at the start of every session. Keep it updated.

## Sprint Goal

Complete auth linking end-to-end so users can connect all their identity methods, add screen transitions for polished navigation, and start extracting screens from App.tsx.

## Sprint Backlog

| # | Item | Size | Priority | Status | Owner |
|---|------|------|----------|--------|-------|
| S1 | Auth Linking Frontend UI | M | P0 | **Done** | Claude |
| S2 | Screen Transitions | S | P1 | In Progress | Claude |
| S3 | HomeScreen Extraction | M | P2 | Planned | Claude |

## What Was Built

### Sprint 8 (completed)
- **Bottom Tab Bar** (`BottomTabBar.tsx`): persistent 4-tab nav (List, Penny, Share, Profile), replaces hamburger menu, safe-area support, 44px touch targets
- **Auth Linking Backend** (`POST /api/auth/link`): link email or phone to existing account, idempotent, conflict detection (409), SMS verification for phone. `GET /auth/me` returns `auth_methods` array.
- **Google Profile Auto-Pop**: fetches profile photo from Google OAuth, validates JPEG/PNG magic bytes, stores in profile_photos table, never overwrites existing photo
- **Receipt Frontend** (`ReceiptScreen.tsx`): receipt history, detail view, item confirmation, Penny typewriter feedback
- **Admin link on Profile**: dashboard accessible from profile page for admins
- **11 new Go tests**: auth linking (7), Google photo (4), all passing

### Sprint 9 Progress

### S1: Auth Linking Frontend UI
- **Profile screen "Sign-in methods" section**: shows Google, Email, Phone with green check (linked) or "Add" button (unlinked)
- **Inline email linking**: tap Add → enter email → calls `POST /api/auth/link` → instant UI update + toast
- **Inline phone linking**: tap Add → enter phone → send SMS code → verify code → calls `POST /api/auth/link` → UI update + toast
- **Error handling**: conflict (email/phone taken), connection errors, validation
- **`authMethods` state**: parsed from `/auth/me` response, drives UI
- **Idempotent**: re-linking returns success without duplicate

## What's Done (Previous Sprints)

| Item | Status |
|------|--------|
| M1 Penny Swaps v0 | Done |
| M2 Sharing & Invite Flow | Done |
| M3 Swap Admin Tooling | Done |
| M4 Brand Portal v0 | Done |
| M5 Stripe Payment | Done |
| M7 Receipt Backend + Frontend | Done |
| R1 Conversational Onboarding | Done |
| R2 Trip Planner Discoverability | Done |
| R3 Done Shopping Flow | Done |
| R5 E2E Tests (partial) | In Progress |
| R7 Bottom Tab Bar | Done |
| Auth Linking Backend | Done |
| Google OAuth Photo | Done |
| B11-B18 all fixed | Done |
| Sentry + PostHog | Done |
| BlueCart pricing | Done |

## Where I Left Off

> **Claude: update this section at the end of every session.**

_Last updated: March 8, 2026 (Sprint 10 session 1)_

### Sprint 10 Progress

**Workstream A: Brand Dashboard v2**
- A1: Competitive Intel API — already existed (`GET /brand/category-intel/:category`)
- A2: Freemium brand registration — already functional (intel visible to all registered brands)
- A3: Purchase funnel visualization — **done** — Offered→Accepted→Purchased funnel with accept/purchase rates, pending claims count
- A4: Dashboard redesign — **done** — Intel preview card above fold (FOMO hook with click-through), purchase funnel, reorganized metrics grid

**Workstream B: Viral Loops**
- B1: "Thriftly Wrapped" monthly savings card — **done**
  - Backend: `GET /api/wrapped` endpoint returns month label, total savings, swaps count, items added, top swap brand, top category, streak days, share text
  - Frontend: `WrappedCard.tsx` component — full-screen modal with savings hero, stats grid, share button (native share API with clipboard fallback)
  - Accessible from Swaps/Earnings screen via gradient CTA banner
- B2: Post-swap share prompt — **done** — floating card after swap accept with savings amount, brand name, share/dismiss buttons, 8s auto-dismiss, native share API
- B3: Enhanced household invite flow — **done** — Penny nudge updated with savings-focused messaging ("Households save 2x more with swaps"), button label changed to "Invite", tracking added
- B4: Share event tracking (PostHog) — **done** — 7 new tracked events: `post_swap_share_shown`, `post_swap_share_tapped`, `wrapped_viewed`, `wrapped_shared`, `list_shared`, `referral_shared`, `referral_copied`, `invite_shared`, `penny_invite_tapped`

**Workstream C: Infrastructure**
- C1: Litestream continuous backup — **done**
  - `litestream.yml` config with S3/R2 replication (10s sync interval, 72h retention)
  - Dockerfile updated: installs litestream v0.3.13 + `run.sh` conditional launcher
  - Also added brand portal frontend build stage (Stage 1b) + host-based routing for `brands.*` subdomain
  - Env vars: `LITESTREAM_REPLICA_BUCKET`, `LITESTREAM_REPLICA_ENDPOINT`, `LITESTREAM_ACCESS_KEY_ID`, `LITESTREAM_SECRET_ACCESS_KEY`
- C2: VACUUM INTO reporting DB — **done**
  - `POST /api/admin/backup` — admin-only endpoint, WAL checkpoint + VACUUM INTO clone
  - `REPORTING_DB_PATH` env var for clone location, auto-opens on startup if file exists
- C3: Admin queries → reporting DB — **done**
  - `adminDB()` helper returns reporting DB if available, falls back to production
  - Dashboard + analytics handlers switched to use `adminDB()` for all read queries

**Tests**
- 11 new Go tests: 6 for `/api/wrapped` (auth, method, no activity, with rebates, month param, response shape), 5 for `/api/admin/backup` (auth, forbidden, method, admin trigger, file-based DB)
- All Go tests passing
- Both frontends build clean (tsc + vite build)

### Previous Sprint Work (Sprints 8-9)
- Auth linking backend + frontend, screen transitions, 12 screen extractions from App.tsx, 14 new E2E tests
- See git history for full details

## Sprint 10 Plan (March 22-29, 2026)

> Full research document: `RESEARCH_BRAND_DASHBOARD_V2.md`

### Sprint Goal
Ship Brand Dashboard v2 with competitive intel freemium hook, implement first viral loop features, set up database disaster recovery.

### Three Parallel Workstreams

**A: Brand Dashboard v2** (5-7 days)
- A1. Competitive Intel API (`GET /brand/category-intel/:category`) — P0
- A2. Freemium brand registration (view intel without paying) — P0
- A3. Purchase funnel visualization (offered → accepted → purchased) — P1
- A4. Dashboard redesign: intel above fold, campaign below — P1
- A5. Weekly email digest for brands — P2

**B: Viral Loops** (3-5 days)
- B1. "Thriftly Wrapped" monthly savings card (shareable image) — P0
- B2. Post-swap share prompt with pre-designed card — P0
- B3. Enhanced household invite flow (Penny nudge) — P0
- B4. Share event tracking (PostHog) — P0
- B5. Savings milestone celebrations — P1

**C: Infrastructure** (3-4 hours)
- C1. Litestream → Cloudflare R2 continuous backup — P0
- C2. Nightly VACUUM INTO clone + reporting DB — P0
- C3. Admin queries → reporting DB — P1
