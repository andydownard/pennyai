# Shared Ground Rules

> **Canonical location:** `~/pitch deck/shared_ground_rules.md`
> **Referenced by:** pennyai/CLAUDE.md, thrift.ly/CLAUDE.md, vibeflourishing/CLAUDE.md
> **Last updated:** 2026-03-09

---

## Products Under This Umbrella

| Product | Repo | Live URL | Stack |
|---------|------|----------|-------|
| **PennyAI** (Thriftly) | ~/pennyai | app.thrift.ly | Go + React/TS + SQLite |
| **Thrift.ly** (Marketing) | ~/thrift.ly | thrift.ly | React/TS + Vite SPA |
| **Vibeflourishing** (Pitch Deck) | ~/vibeflourishing | vibeflourishing.com | React/TS + Node.js + SQLite |
| **Rally** (Coach Tips) | ~/rally | rallytasks.com | TBD |

---

## Secret Keys — Hard Rule

**Never commit secrets to source code.** API keys, database passwords, auth tokens, and encryption keys live in environment variables only. If a secret is accidentally committed, rotate it immediately and force-push to remove from history.

---

## Engineering Preferences

- **SQLite in WAL mode** for all products (simple, fast, no external dependencies)
- **Railway** for all deployments (Docker, auto-deploy from GitHub)
- **Twilio** for SMS, **SendGrid** for email, **Claude API** for AI
- **Go** for backends (PennyAI, Rally), **Node.js** for lighter backends (vibeflourishing)
- **React 19 + TypeScript + Vite** for all frontends
- **Inter** as canonical web font across all surfaces
- **Brand green #35644F** across all surfaces (see `penny-brand-source.json`)

---

## Test-Driven Development — Iron Law

**Test-to-code ratio must be >= 1.3x.** This means for every 100 lines of production code, there must be at least 130 lines of test code. No exceptions. No "we'll add tests later."

- **Production code:** Any `.ts`, `.tsx`, `.go` file that ships to users
- **Test code:** Any file in `__tests__/`, `*.test.*`, `*.spec.*`, `*_test.go`
- **Enforcement:** Check ratio before every PR merge. If ratio drops below 1.3, the PR is blocked until tests are added.

---

## Parallel Development & Branching Strategy

- **Max 3 parallel workstreams** at any time across all products
- **Git worktrees required** for parallel work (not branches switching in same directory)
- **Main branch is always deployable** — never push broken code to main
- **Feature branches** for anything that takes >1 hour

---

## Sprint Planning Format

All sprint plans follow this structure:
1. **1-page plan** with scope, timeline, and owner
2. **User stories with child acceptance criteria** — each story has testable ACs
3. **Atomic task decomposition** — no task takes >4 hours
4. **Epic chunking** — group related stories into epics for tracking

---

## Plan Review Mode (Garry Tan Mega Review)

Three operating modes for any plan review:
- **EXPANSION:** Build the cathedral. Push scope up. Default for greenfield.
- **HOLD SCOPE:** Accept boundaries. Maximum rigor. Default for fixes/refactors.
- **SCOPE REDUCTION:** Surgical minimalism. Default for tight timelines.

All plans reviewed against 10 dimensions: Architecture, Error & Rescue Map, Security & Threat Model, Data Flow & Edge Cases, Testing, Performance, Deployment, Monitoring, Documentation, User Experience.

Full framework details in `VIBECODING-BEST-PRACTICES.md`.

---

## Product Team (7 Agent Roles)

| Role | Responsibility |
|------|---------------|
| **CEO** (Andy) | Final decision authority on all product, messaging, and priority |
| **VP Product Marketing** | Copy recommendations, audience insights, competitive positioning |
| **Brand PM** | Brand consistency (penny-brand-source.json), design tokens, voice |
| **CMO** | Brand alignment across all surfaces |
| **UX Designer** | Edge cases, scroll/layout, touch targets, animation, accessibility |
| **Tech Lead** | Architecture, security, performance, testing |
| **Data Scientist** | Metrics, A/B testing, flywheel measurement |

### Convene Rules
- Convene the full team before shipping any user-facing feature
- UX Designer reviews BEFORE implementation begins
- CEO has final sign-off on messaging and priority

---

## Focus Groups Protocol

For any user-facing feature, simulate a panel of 7 customers before building.

### Rounds
1. **Round 1:** Initial reactions (unfiltered first impressions)
2. **Round 2:** Prioritization (what matters most, what can wait)
3. **Round 3:** Risk assessment (what could go wrong)
4. **Round 4:** Final directives (concrete actions, owners, timelines)

Panel members are product-specific (defined in each product's CLAUDE.md).

---

## Pattern Detection Rules

When working across codebases, watch for:
- **Color drift:** Any hex color that doesn't match `penny-brand-source.json`
- **Font drift:** Any font-family that doesn't match the product's canonical font
- **API contract drift:** Endpoint paths or response shapes that change without updating CLAUDE.md
- **Duplicate logic:** Same business logic implemented differently in two places
- **Stale documentation:** Markdown that references features that no longer exist

---

## Brand Source of Truth

**`~/pitch deck/penny-brand-source.json`** (v1.1.0) is the single source of truth for:
- Company/product names, tagline, one-liner
- Design system (colors, fonts, spacing, effects)
- Value propositions and website story arc
- 11-Star Moment narratives (PennyAI + Rally)
- Shipping gates (First-5-Minutes Magic, 11-Star Moment, Data Flywheel)
- Onboarding defaults and dietary respect rules

All products derive their brand elements from this file. Edit it first, then sync to all surfaces.

---

## Three Shipping Gates

Every feature must pass at least 2 of 3 gates:

### Gate 1: First-5-Minutes Magic (Stewart Butterfield)
Can a new user feel "this just works" in under 5 minutes?

### Gate 2: 11-Star Moment (Brian Chesky)
Does this feature ladder to the core emotional narrative?
- PennyAI: "You never ask 'Did you get milk?' again"
- Rally: "Freedom + Trust"
- Vibeflourishing: "Investor sees thesis proven live"

### Gate 3: Data Flywheel (Katrina Lake)
Does this feature make the product smarter over time?

---

## Daily Sync (9AM Standup)

Automated daily process:
1. Run `sync-audit.py` for brand consistency
2. Scan all mounted codebases for drift
3. Convene simulated CEO panel
4. Execute recommendations
5. Write executive summary + manifest

Output files go to `~/pitch deck/` with date suffixes. Archive files older than 3 days to `~/pitch deck/archive/`.
