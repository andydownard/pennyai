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

_Last updated: March 8, 2026_

- S1 (Auth Linking Frontend) **done** — profile shows linked methods, inline email/phone linking flows
- S2 (Screen Transitions) **in progress** — worktree agent building ScreenTransition component
- S3 (HomeScreen Extraction) **planned** — will start after S2 merges
- **All Go tests passing** (`go test -count=1 ./...` → OK)
- **Frontend builds clean** (tsc + vite build)
- **Deployed** to Railway (Sprint 8 changes live)

## Next Sprint Preview (Sprint 10)

- M9 Brand Dashboard v2 (Competitive Intel frontend)
- Optimistic updates (R7 increment)
- Continue R6 codebase refactor (ProfileScreen extraction)
