# Groflo Marketing Site (thrift.ly repo)

This repo (`groflo-site` per `package.json`, retained git URL `andydownard/thrift.ly`) is the canonical Netlify-deployed marketing site for **Groflo** (formerly Thriftly), an AI shopping list app where users text **Penny**, the in-app assistant. Groflo is a marketplace inside the Rally ecosystem (auth and Stripe federate through `timpager` / rallytasks.com).

> **Work tracking:** new items land in [INBOX.md](./INBOX.md). Triage moves them to [BACKLOG.md](./BACKLOG.md) (`Backlog` P0–P3, `Bugs (Open)`, `Shipped`, `Not In Scope`). Pattern mirrors `pennyai/ROADMAP.md`. Read [INBOX.md](./INBOX.md) first when picking up unfinished work.

The repo also contains the investor deck and timpager AI slide generator as sibling sub-apps so they can be deployed independently from the same source tree.

**Rebrand status (as of 2026-04-07):** Phase 0 of the Thriftly→Groflo rebrand is shipped. `groflo.io` DNS is live. Privacy + Terms include full A2P 10DLC compliance language. Phases 1-5 (Twilio re-vetting, DNS cutover details, Rally login federation, Stripe Connect consolidation, internal identifier sweep) are queued — see `/Users/andydownard/projects/pennyai/GROFLO_REBRAND_SPRINT.md` for the full plan.

## Architecture

- **Marketing site** (`src/`): React 19 + TypeScript + Vite SPA, React Router DOM 7, Stripe Elements for campaign signup, vanilla CSS with custom properties (no Tailwind). Deploys to Netlify from `main`. Component CTAs route through `import.meta.env.VITE_APP_URL` (fallback `https://app.groflo.io`).
- **Investor deck** (`deck/`): Node.js server + SQLite (WAL mode), Twilio Verify SMS auth, feedback widget with GitHub issue creation, slide view analytics, deployed on Railway. Internal package names (`@thriftly/*`) and asset filenames retain the legacy "thriftly" — Phase 5 cleanup, no user impact.
- **Timpager** (`timpager/`): Node.js server, AI-driven slide generation/update (`ai-generate.js`, `ai-update.js`), slide type renderer (`slide-types.js`), Claude API integration.
- **Brand source of truth**: `../pitch deck/penny-brand-bible.json` — v3.0 with the Groflo identity block (company: Groflo, Inc., app: Groflo, domain: groflo.io, ecosystem: Rally, footer: "Penny by Groflo"). Canonical copy lives at `/Users/andydownard/projects/pennyai/penny-brand-bible.json`. Brand color `#35644F` and Inter typeface are unchanged across the rebrand.
- **Pages** (marketing site): Home (`/`), For Brands (`/for-brands`), Pricing (`/pricing`), About (`/about`), Campaign Signup (`/campaign/signup`), Privacy (`/privacy`), Terms (`/terms`)

---

## Hard Rules

### Secret Keys

- Do NOT commit, log, or expose API keys, tokens, database credentials, or secrets
- All secrets live in environment variables — TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN, GH_PAT, ADMIN_API_KEY, FEEDBACK_CODE, FEEDBACK_PHONES must never appear in source
- If a secret is accidentally committed, rotate it immediately

### Idempotency & Mutation Safety (deck/ and timpager/)

Every handler that mutates state (INSERT, UPDATE, DELETE) must have:

1. **Idempotency guard** — calling it twice with the same input produces the same result, not a duplicate
2. **Transaction boundary** — multi-step mutations are atomic
3. **Server-side rate limit** — not bypassable by client bugs
4. **Negative-path test** — test that calling it twice does NOT produce duplicates

The deck server has state-mutating endpoints: feedback submission, session creation, audit logging, slide view tracking. All must follow these rules.

### Session Security (deck/)

- Do NOT weaken session security: 256-bit random tokens, phone-verified sessions
- Do NOT remove or bypass Twilio Verify SMS authentication
- Do NOT expose FEEDBACK_PHONES or FEEDBACK_CODE in client-side code
- Session validation must happen on every authenticated request

### Test-Driven Development

- Test-to-code ratio must be >= 1.3x
- Marketing site: production code is `src/**/*.{ts,tsx}`, test code is `__tests__/**`, `*.test.*`, `*.spec.*`
- Deck/timpager: test all HTTP endpoints (happy path + error paths), test rate limiting, test auth flows
- No feature or change is "done" until tests pass with zero failures

### Continuous Work Mode

Never stop working. When you finish a task: check for unblocked items, improve test coverage, refactor per these rules. The default state is working, not waiting. Do NOT start a new sprint without confirming with the user unless the current sprint was started in this same session.

---

## Refactoring Constraints

### What is NOT allowed

- Do NOT remove any marketing site page or route
- Do NOT remove Stripe campaign signup functionality
- Do NOT remove the deck feedback widget, auth gate, or analytics tracking
- Do NOT remove timpager AI generation or update capabilities
- Do NOT drop or recreate any SQLite table in deck/ — use ALTER TABLE ADD COLUMN for migrations
- Do NOT change deck server endpoint paths or response shapes without updating the deck.html and widget.js clients

### Database (deck/)

- `CREATE TABLE IF NOT EXISTS` for all tables (sessions, feedback, audit_log, slide_views)
- `ALTER TABLE ADD COLUMN` for new columns — never drop and recreate
- WAL mode for SQLite
- Prepared statements via `db.prepare()` — do NOT use string concatenation for SQL

### Key Patterns to Preserve (deck/)

- Rate limiting via `Map` with sliding time windows
- CORS allowlist: `groflo.io` domains, `thrift.ly` domains (kept during transition), and `localhost` only — do NOT use `*`. Both apex hosts must remain in the allowlist until Phase 2 DNS cutover completes and all clients have migrated.
- Phone normalization via `normalizePhone()` before any comparison
- GitHub issue creation on feedback submission (when GH_PAT configured)
- Audit logging for all auth and admin actions

---

## Marketing & Brand Governance

### Decision Authority

- **CEO (Andy)** — final call on all website copy, positioning, messaging priority, feature prominence
- **VP Product Marketing** — copy recommendations, audience insights, competitive positioning
- **Brand PM** — consistency with `penny-brand-bible.json` v3.0 (Groflo identity block), design tokens, voice
- **CMO** — brand alignment across all surfaces

Changes to copy require explicit CEO approval. Minor tweaks (punctuation, single-word swaps) may bypass CEO review if PMM + Brand PM agree it strengthens clarity without changing messaging intent.

### Privacy + Terms (legal canonical, Twilio-vetting documents)

`src/pages/Privacy.tsx` (12 numbered sections) and `src/pages/Terms.tsx` (13 numbered sections) are the **canonical** legal pages used for Twilio A2P 10DLC campaign vetting. Pennyai's `handlers_admin.go` HTML mirrors them. **Edit here first**, then sync to pennyai. The following CTIA-required language MUST appear and render cleanly:

- "Message frequency varies"
- "Message and data rates may apply"
- STOP / HELP keywords with explicit reply instructions
- "We will not share your mobile information with third parties or affiliates for marketing or promotional purposes" (verbatim)
- Frequency-control promise ("change how often Penny texts you in Settings")

Tests in `src/__tests__/Privacy.test.tsx` and `Terms.test.tsx` enforce all of these. Don't weaken them. The numbered h2 sections are asserted by `getByRole('heading', { level: 2, name: /.../ })` — exact-string `getByText` will fail because of the "1. Section" / "8. Security" prefix.

### Language Standard

Every piece of customer-facing copy must pass the "would I text this to my friend?" test:

- Do NOT use jargon, buzzwords, or corporate speak
- Do NOT use "AI", "orchestration", or "coordinate" in headlines
- Good: "No more 'Did you get milk?' Claim items as you add them."
- Bad: "Facilitate inter-household item coordination via distributed list synchronization."

### Hero Message (Locked)

Core message: "No more 'Did you get milk?'" — All positioning ladders back to coordination + relief.

Story arc: ONE hero message ("Why Penny?") → THREE pillars (text naturally, Penny organizes, family shops together) → social proof → referral hook → trust foundation (privacy FAQ).

Do NOT use feature-list approach in hero. Do NOT give all features equal weight.

---

## Shipping Gate Constraints

- Do NOT ship website copy where a cold visitor cannot answer "What does Penny do?" within 5 seconds of landing (Gate 1)
- Do NOT ship features/copy that don't connect to household coordination pain — duplicates, unclear ownership, miscommunication (Gate 2)
- Do NOT hide referral messaging below the fold or behind sign-up (Gate 3)
- Do NOT add features to website before they ship in the app (vaporware risk)

---

## Brand Consistency Checklist (Before Ship)

- Brand name: **Groflo** in user-visible text. The two intentional remaining "Thriftly" mentions in `src/` are negative test assertions in `Privacy.test.tsx` and `Terms.test.tsx` ("does not contain stale Thriftly references") — leave these.
- Email convention: `*@groflo.io` (privacy@, legal@, hello@, support@, brands@, press@, careers@, noreply@, penny@list.groflo.io)
- Brand Green (#35644F) on buttons, icons, accents
- Inter font in all text (no Roboto, no serif fallbacks)
- Background #F7F6F3 or white for main surfaces
- Tagline present in footer or hero: "The AI shopping list for families"
- Penny personality reflected (warm, helpful, not corporate)
- Footer attribution: "Penny by Groflo" / "© Groflo, Inc."
- App CTAs read `import.meta.env.VITE_APP_URL` (fallback `https://app.groflo.io`) — do NOT hardcode the URL again
- CEO and PMM sign-off documented before deployment

---

## Pattern Detection

Watch for these across the codebase. When detected, audit for all instances.

1. Color/font drift — hex colors or font-families that don't match `penny-brand-bible.json` v3.0
2. Brand-name drift — "Thriftly" or "thrift.ly" in user-visible text (only the negative test assertions in `Privacy.test.tsx` and `Terms.test.tsx` should reference Thriftly)
3. Hardcoded `https://app.groflo.io` in components — should read `import.meta.env.VITE_APP_URL` with fallback
4. Email-address drift — `*@thrift.ly` addresses anywhere in user-visible text or backend (canonical is `*@groflo.io`)
5. Idempotency gaps — deck/ handlers without dedup guards
6. Missing rate limits — deck/ endpoints without server-side rate limiting
7. CORS drift — deck/ CORS allowlist must include both `groflo.io` and `thrift.ly` domains during the transition; do NOT remove `thrift.ly` until Phase 2 is fully complete and 301s have been live for 30+ days
8. Stale content priorities — website copy referencing features not yet shipped in the app
9. Missing CTIA-required language in Privacy/Terms — every edit to those files must preserve all the items listed in "Privacy + Terms" above
