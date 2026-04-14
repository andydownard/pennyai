# Groflo Marketing Site — Backlog

> **Structure:** [INBOX.md](./INBOX.md) → new/unsorted items. `Backlog` (this file) → prioritized, not started. `Shipped` → compact log of what landed. `Not In Scope` → explicitly deferred.
>
> **Scope:** This file tracks work for the marketing site (`groflo.io`, this repo). App work lives in [`pennyai/ROADMAP.md`](../pennyai/ROADMAP.md). Brand-portal work (`brands.groflo.io`) also lives in pennyai. Cross-repo items reference the boundary explicitly.
>
> **Guardrail:** Never delete items without founder approval. Move to `Shipped`, `Not In Scope`, or back to `INBOX.md` for re-triage — don't remove.

## Backlog

### P0 — Launch Blockers

- **`feedback.mts` idempotency guard.** CLAUDE.md "Idempotency & Mutation Safety" requires it. Documented in `INBOX.md` 2026-04-13 and in test `KNOWN GAP: duplicate POST currently creates two rows`. Three-file change: migration + function + `FeedbackWidget.tsx`. Blocks CLAUDE.md compliance audit.
- **Netlify env vars set in production.** `BRAND_PORTAL_URL`, `IP_HASH_SALT`, `SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY`. Missing any returns 500 from `campaign-lead.mts` to real brand visitors.
- **Supabase migration `002_campaign_leads.sql` applied to prod.** Required before `campaign-lead.mts` deploy. Idempotent SQL.
- **Brand-portal lead pickup.** `brands.groflo.io` does not yet read `?from=campaign_signup&lead=<id>`. Without portal-side wiring, brands land on a blank login screen after submitting their brief. Cross-repo: needs work in `pennyai/handlers_brand_auth.go`.

### P1 — Next Sprint Candidates

- **Reporting copy validation.** Marketing copy promises real-time analytics the brand portal must deliver. Walk each through against current `brands.groflo.io` capability. Either soften the copy or ship the metric. Items: `ForBrands.tsx:77`, `ForBrands.tsx:203-204`, `Pricing.tsx:28`, `Pricing.tsx:40`, `Pricing.tsx:86`. Per CLAUDE.md governance: copy changes require CEO sign-off.
- **Fake waitlist form on `ForBrands.tsx:346-349`.** `onSubmit` clears input + swaps placeholder; POSTs nothing. Wire to `campaign-lead.mts` (`source: 'brands_waitlist'`) or replace with `mailto:` / Calendly link.
- **Brand testimonials integrity (`ForBrands.tsx:361, :370, :379`).** "Sarah K.", "Marcus T.", "Priya M." appear illustrative. Either get real-partner consent or add a "Composite based on early partner conversations" disclosure.
- **Stable test command.** `npm test` runs vitest once; consider adding `npm run test:functions` and `npm run test:ui` so CI can fan out the 1002-test suite. Today's full run takes ~21s — fine for now, but worth splitting before it doubles.

### P2 — Quarter Candidates

- **Reduce `index-*.js` chunk size (310 KB → target <250 KB).** Vite warns at >500 KB but our hero/Home page is heavy. Lazy-load `FeedbackWidget` (admin-only, currently in main bundle). Audit bundle for unused Supabase client in pages that don't need it.
- **Replace IntersectionObserver fade-ins with `content-visibility: auto` + CSS `@starting-style`** where browser support allows. Removes `useOnScreen` hook and 30 call sites; no JS for animation.
- **Migrate `archive/MARKETING-PANEL-*.md` series to a dated `archive/panels/` subdir.** Currently four panel docs sit at the top of `archive/`. Pennyai's archive uses subfolders (`shared-archive`); align.

### P3 — Distant / Future

- **Replace `useDocTitle` with React 19 `<title>` / `<meta>` rendering.** React 19 supports document metadata in components natively; the custom hook predates that.
- **Storybook for marketing components.** Header, Footer, SavingsCalculator, FeedbackWidget all render in isolation; would benefit visual regression coverage.
- **Per-page Open Graph images.** Currently a single OG image; pages that get shared (For Brands, Pricing) should have their own.

### Bugs (Open)

- _(none currently — log new bugs in [INBOX.md](./INBOX.md) first)_

## Shipped

### 2026-04

- **2026-04-13 — Pre-launch money-path refactor (option A).** Marketing-site campaign signup is now lead-capture only; money lives in `brands.groflo.io`.
  - Removed `@stripe/react-stripe-js` and `@stripe/stripe-js` from `package.json` and the fake `PaymentForm` from `CampaignSignup.tsx` (was a `setTimeout` mock that always reported success — would have lied to brands at launch about card authorization).
  - Added `netlify/functions/campaign-lead.mts` with client-supplied `Idempotency-Key`, per-IP rate limit (5/hr), salted SHA-256 IP hashing, CORS allowlist (groflo.io + thrift.ly transition window), origin-reflecting CORS, and Stripe-style replay handling on the unique-key race.
  - Added `supabase/migrations/002_campaign_leads.sql` — idempotency-key unique index, status enum, RLS enabled.
  - Added `netlify/functions/__tests__/` harness with in-memory Supabase mock; **77 new function tests** (40 `campaign-lead`, 37 `feedback`).
  - Refactored `CampaignSignup.tsx`: stable `crypto.randomUUID` idempotency key per signup session, real `fetch` with error/network handling, in-flight disable, server-supplied `redirectUrl` on confirmation, "Brief Received" status (replaces fake "Under Review").
  - Pricing FAQ #2 + #4 rewritten — no longer claims this site authorizes a card.
  - `useOnScreen` switched to `[ref, visible]` tuple return; 30 call sites refactored across 5 files. Resolves 64 `react-hooks/refs` lint errors.
  - `eslint.config.js` adds `no-unused-vars` `_`-prefix exception (standard convention).
  - `.env.example` + `DEPLOYMENT.md` purged of `VITE_STRIPE_PUBLISHABLE_KEY`; added `VITE_BRAND_PORTAL_URL`, `BRAND_PORTAL_URL`, `IP_HASH_SALT`.
  - Final state: lint clean, build clean, **1002/1002 tests pass** across 18 files.
- **2026-04-07 — Phase 0 Groflo rebrand.** Privacy + Terms with full A2P 10DLC compliance, marketing copy refresh, `groflo.io` DNS live. Component CTAs route through `VITE_APP_URL` (commit `4505da4`). Remaining phases (Twilio re-vetting, DNS cutover, Rally login federation, Stripe Connect consolidation, internal identifier sweep) tracked in `pennyai/GROFLO_REBRAND_SPRINT.md`.

### 2026-03

- **Marketing site multi-page SPA.** Home, For Brands, Pricing, Campaign Signup, About, Privacy, Terms, NotFound. React 19 + TypeScript + Vite + React Router 7. Vanilla CSS with custom properties.
- **Privacy + Terms canonical legal pages** with CTIA-required language for Twilio A2P 10DLC vetting (12-section Privacy, 13-section Terms). Tests in `__tests__/Privacy.test.tsx` + `Terms.test.tsx` enforce required language.
- **GA4 tracking** — beginning to see funnel data. (Per `archive/MARKETING-PANEL-2026-03-09.md`.)
- **Investor deck** at `deck/` — Twilio Verify SMS auth, feedback widget with GitHub issue creation, slide view analytics, deployed on Railway.
- **Deck idempotency + auth hardening.** All state-mutating endpoints (feedback, sessions, audit, slide views) follow CLAUDE.md "Idempotency & Mutation Safety" rules.
- **Marketing panel reviews.** See `archive/MARKETING-PANEL-2026-03-09.md`, `MARKETING-PROPOSALS-2026-03-09.md`, `MARKETING-REVIEW-2026-03-09.md`, `PRODUCT-TEAM-REVIEW-2026-03-09.md` for the panel decisions that shaped current copy.

## Not In Scope

- **Stripe / payment flows in this repo.** Option B from the 2026-04-13 architecture review (build a parallel PaymentIntent endpoint as a Netlify function) was rejected. Money lives in `brands.groflo.io` — one Stripe account, one PCI-aware surface, one webhook idempotency story. The marketing site captures intent and links out. See [memory/marketing_money_boundary.md](../../.claude/projects/-Users-andydownard-projects-thrift-ly/memory/marketing_money_boundary.md).
- **Brand-portal UI in this repo.** `brands.groflo.io` is served from `pennyai/static-brand/` (separate Vite build, host-based subdomain routing). Do not duplicate brand-portal screens here.
- **Consumer Penny app screens in this repo.** Lives in `pennyai/frontend/`. The `Try Free` CTA links to `import.meta.env.VITE_APP_URL` (default `https://app.groflo.io`).
- **Removing `thrift.ly` from CORS allowlist.** Per CLAUDE.md, both `groflo.io` and `thrift.ly` domains stay until Phase 2 DNS cutover completes and 301s have been live for 30+ days.
- **`/embed` route from pennyai.** Vibeflourishing's pitch-deck embed flow lives in pennyai. Do not port to this repo.
- **Server-side rendering / Next.js migration.** Vite SPA + Netlify CDN serves the marketing-site shape correctly. SPA caching headers (`netlify.toml`) already in place.
