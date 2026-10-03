# Groflo Rebrand Sprint

**Date queued:** 2026-04-07 · **Owner:** Andy · **Status:** Phase 0 complete (drafts queued for review)

This document is the source of truth for the Thriftly → Groflo rebrand. Phase 0 ships in this commit; Phases 1–4 are queued and gated on Andy approval before any production cutover.

---

## Decisions (from panel deliberation 2026-04-07)

| Decision | Value |
|---|---|
| Company | **Groflo, Inc.** (was Thriftly, Inc.) |
| App | **Groflo** (was Thriftly) |
| Assistant | **Penny** — unchanged. Pattern: Groflo : Penny :: Echo : Alexa. |
| Domain | **groflo.io** (apex marketing site + app subdomain `app.groflo.io` + brand portal `brands.groflo.io` + deck `deck.groflo.io`) |
| Email convention | `*@groflo.io` (privacy@, legal@, hello@, support@, brands@, press@, careers@, noreply@, penny@list.groflo.io) |
| Brand color | `#35644F` (unchanged) |
| Ecosystem | Member of **Rally** (canonical user graph at timpager) |
| Twilio | Both A2P 10DLC conversational + Verify campaigns. Both need re-vetting for the new brand name + new privacy URL. |

---

## Phase 0 — Already shipped in this commit

These edits are live in the working tree. Review them before pushing.

### Legal documents (canonical: `/projects/thrift.ly`)
- `src/pages/Privacy.tsx` — Twilio A2P-compliant Privacy Policy (12 sections, includes Rally ecosystem disclosure, mobile-info verbatim language, STOP/HELP, frequency control)
- `src/pages/Terms.tsx` — Terms of Service (13 sections, SMS messaging program detail, Stripe + Rally disclosures, Delaware governing law)
- `src/__tests__/Privacy.test.tsx` — rewritten to assert the new content
- `src/__tests__/Terms.test.tsx` — rewritten to assert the new content

### Mirrored in-app legal pages
- `pennyai/handlers_admin.go` — `/privacy` and `/terms` HTML rewritten to mirror the canonical content. Twilio campaign vetting can audit either URL.

### Brand bible
- `pennyai/penny-brand-bible.json` v3.0:
  - identity.company: Groflo, Inc.
  - identity.app: Groflo
  - identity.domain: groflo.io
  - identity.ecosystem: Rally
  - identity.footer_attribution: "Penny by Groflo"
  - voice + design + colors unchanged

### Marketing site (`/projects/thrift.ly`)
Every Thriftly → Groflo across:
- `index.html` (title, OG cards, Twitter cards, JSON-LD Organization + SoftwareApplication + FAQPage, noscript fallback)
- `src/components/Header.tsx`, `Footer.tsx`
- `src/pages/Home.tsx` (10+ CTA URLs, hero copy, FAQ, deal examples, Wrapped section, sticky CTA)
- `src/pages/About.tsx` (full rewrite of brand mentions)
- `src/pages/ForBrands.tsx` (section labels, brand portal URL)
- `src/pages/Pricing.tsx` (FAQ + alt text)
- `src/pages/CampaignSignup.tsx` (terms link, localStorage key, alt)
- `src/pages/NotFound.tsx`
- `src/components/ErrorBoundary.tsx`, `EmailCapture.tsx`, `SavingsCalculator.tsx`
- `src/hooks/useDocTitle.ts` (BASE_TITLE + BASE_URL)
- `src/App.tsx` (page loader logo)
- `public/og-image.svg` (footer brand stamp)
- `public/robots.txt`, `public/sitemap.xml` (all URLs → groflo.io)
- `netlify.toml` header comment
- `package.json` name field
- All test files updated (About, ForBrands, Footer, Header, Home, useDocTitle, CampaignSignup)

### pennyai user-visible
- `frontend/index.html` — title, OG, Twitter cards, app URL
- `frontend/public/manifest.json` — name, short_name, id field added (PWA continuity per panel R7)
- `frontend/public/sw.js` — CACHE_NAME bumped to `groflo-v7` (forces invalidation of old cache after rebrand)
- `frontend/package.json` — name field
- `frontend/src/App.tsx` — SCREEN_TITLES, aria-label, doc title, share text, PWA install banner, comment header
- `frontend/src/App.tsx` — **legacy storage migration shim** at module load: copies any `thriftly_*` localStorage/sessionStorage keys to `groflo_*` (panel R5: household continuity for existing users)
- `frontend/src/hooks/useAuthState.ts` — sessionStorage keys (groflo_redirect, groflo_merge_msg)
- `frontend/src/hooks/useSwapState.ts` — first-swap key with legacy fallback
- `frontend/src/hooks/useSettingsState.ts` — invite share text
- `frontend/src/styles.ts` — theme storage key
- `frontend/src/components/PhoneScreen.tsx` — **new SMS opt-in copy**:
  - Existing checkbox label preserved ("I want to SMS with Penny", ≤27 chars per UX U1)
  - Italic helper "Change frequency anytime in Settings."
  - A2P 10DLC required disclosure beneath Continue button: "Reply STOP to opt out, HELP for help. Msg & data rates may apply." + Terms/Privacy links
- `frontend/src/components/PennySettingsScreen.tsx` — Penny mailto address
- `frontend/src/components/NPSSurveyModal.tsx` — recommend question
- `frontend/src/components/SharedListScreen.tsx` — sign-up CTA + footer brand
- `frontend/src/components/NavMenu.tsx` — logo aria-label, brand wordmark, brand portal href
- `frontend/src/components/HelpScreen.tsx` — support email
- `frontend/src/components/EmbedView.tsx` — app URL
- `frontend/src/components/ProfileScreen.tsx` — penny@list email
- `frontend/src/components/ShareScreen.tsx` — referral URL, QR canvas branding (4 references in canvas-rendered share image)
- `frontend/src/components/WrappedCard.tsx` — canvas branding, share filename, share text
- `frontend/src/components/PostSwapShareCard.tsx` — share text
- `frontend/src/components/MilestoneCard.tsx` — branding
- `frontend/src/components/LegalLinks.tsx` — "Penny by Groflo" attribution

### pennyai backend
- `handlers_admin.go` — privacy + terms HTML mirrors canonical
- `handlers_brand_payments.go` — Stripe baseURL + product name
- `handlers_brand_digest.go` — entire weekly recap email (header, body, unsubscribe page)
- `handlers_digest.go` — daily digest SMS link
- `handlers_logo.go` — logo.dev Referer header
- `handlers_social.go` — vCard ORG + email, invite SMS body, join-notification SMS, join link
- `handlers_viral.go` — Wrapped share text, milestone share text, "Get Groflo" CTA
- `handlers_auth.go` — merge messages, transactional email from address, magic-link + reset email bodies
- `penny.go` — Penny system prompt (the "don't mention Thriftly" instruction now says "don't mention Groflo")
- `server.go` — startup log, comment fixes, allowed-origin entries (deck.groflo.io added alongside legacy deck.thrift.ly for transition)
- `utils.go` — OG tag injection for `/r/` and `/join/` link previews (sets dynamic Groflo titles)
- `main.go` — comment header

### Memory updates
Persistent memory entries written so future sessions retain context:
- `memory/groflo_rebrand.md` — rebrand decision record
- `memory/rally_ecosystem.md` — Rally relationship and federation model
- `memory/twilio_scope.md` — A2P 10DLC + Verify campaign scope
- `memory/feedback_panels_and_voice.md` — panel-and-voice expectations
- `memory/marketing_repo.md` — `/thrift.ly` is canonical for legal
- `memory/andy_style.md` — collaboration style

---

## Phase 1 — Twilio campaign re-vetting (gated on Andy approval)

The campaign brand name on file with Twilio and the carriers is "Thriftly". Brand-name changes require re-vetting. Privacy and terms URLs must be live before submission.

### Pre-submission checklist
- [ ] DNS pointed: `groflo.io`, `app.groflo.io`, `brands.groflo.io` resolve and serve TLS
- [ ] Privacy live at `https://groflo.io/privacy` (renders the new content)
- [ ] Terms live at `https://groflo.io/terms`
- [ ] In-app `/privacy` and `/terms` (served by handlers_admin.go) match
- [ ] Verify the CTIA-required language is present and renders cleanly:
  - "Message frequency varies"
  - "Message and data rates may apply"
  - STOP / HELP keywords
  - "We will not share your mobile information with third parties or affiliates for marketing or promotional purposes"
- [ ] Sample messages prepared for vetting submission (5 examples reflecting Penny's voice)

### Twilio Console submission
- [ ] **Brand profile**: update legal entity name to Groflo, Inc.; update website URL to groflo.io
- [ ] **A2P 10DLC campaign** (conversational): submit new use case "Conversational shopping list assistant" with privacy + terms URLs and 5 sample messages. Expected vetting: 7–14 days.
- [ ] **Verify service**: update display name to Groflo (sender brand on the OTP). Twilio Verify SID itself does not change.
- [ ] **Brand vetting score**: monitor; Standard or higher is required for full throughput.

### Post-approval
- [ ] Cutover the consumer phone number's display name to Groflo
- [ ] Update vCard `ORG:Groflo` (already done in `handlers_social.go`)
- [ ] Smoke test: text the Groflo number, verify it identifies as Groflo in the iMessage thread header

### Risk
- **Old Thriftly campaign**: leave it active in parallel during the vetting window. If the new campaign is rejected, no service interruption.
- **Customer SMS history**: existing users' SMS threads with Penny do not break — same Twilio number, same conversation. Only the brand name on the message header changes.

---

## Phase 2 — Domain cutover (gated on Phase 1)

Per panel decision (Devon — R2), iPhone link previews and OG cards must show "Groflo" everywhere from this point forward.

### DNS + TLS
- [ ] groflo.io: A/AAAA → marketing site (Netlify) — same as current thrift.ly
- [ ] app.groflo.io: CNAME → Railway production
- [ ] brands.groflo.io: CNAME → Railway brand portal
- [ ] deck.groflo.io: CNAME → deck server
- [ ] Provision TLS certs (Let's Encrypt via Netlify and Railway native)

### 301 redirects (panel R5: household continuity for ≥12 months)
- [ ] Netlify `_redirects`: `https://thrift.ly/*  https://groflo.io/:splat  301`
- [ ] Railway: `https://app.thrift.ly/*  https://app.groflo.io/:splat  301` (server.go middleware)
- [ ] `https://brand.thrift.ly/*  https://brands.groflo.io/:splat  301`
- [ ] Test: every share link in customer SMS history still resolves
- [ ] Test: deep link `https://app.thrift.ly/r/{code}` redirects to `https://app.groflo.io/r/{code}` and the join flow completes

### Apple Smart App Banner / iOS link previews
- [ ] `apple-app-site-association` updated for groflo.io with Penny avatar
- [ ] OG cards verified in iMessage with the Apple Validator
- [ ] Smart Banner meta tag if applicable

### Android assetlinks
- [ ] `assetlinks.json` updated for groflo.io if any TWA / app links

### Cache invalidation
- [ ] Service worker bumped to `groflo-v7` (already done) — old `thriftly-v6` cache will be evicted on next user visit
- [ ] CDN cache purge (Cloudflare or Netlify CDN) for thrift.ly and groflo.io

### Wallet integrity check (panel R1: Linda)
- [ ] Pre-cutover: dump `SUM(amount_cents) FROM swap_rebates` and `SUM(available_cents) FROM wallet_balances`
- [ ] Post-cutover: same query, must be byte-identical
- [ ] Add a Go test that simulates the cutover and checks wallet totals

### Dietary preference preservation (panel R6: Priya)
- [ ] Add a Playwright E2E test:
  1. Register a user
  2. Set dietary_attribute='gluten_free'
  3. Restart the app (simulates rebrand cutover)
  4. Assert dietary attribute still resolves on next brand suggestion
- [ ] Run before cutover; must pass

### In-app announcement (panel R7: Nadia)
- [ ] One-time banner persisted in localStorage `groflo_rebrand_banner_dismissed`
- [ ] Copy: "Penny is now part of Groflo. Same Penny, same list, same household."
- [ ] Show on next visit only; auto-dismiss on tap; do NOT re-show

---

## Phase 3 — Rally login federation (gated on Phase 2)

### Current state
There is **no inbound auth bridge**. `middleware_timpager.go` once held a
header-forwarding one (read `X-Timpager-Session`, call `GET {TIMPAGER_URL}/api/auth/verify`
with a 60s cache, map the timpager user to a local Penny user by phone, fall
through to cookie auth). It was **deleted** in the top-10 code-review pass,
unused, for three reasons:

- Zero call sites. It was never wired into any handler.
- It mapped any timpager session to a pennyai account **by phone alone**,
  creating one if absent: cross-marketplace account takeover by phone with no
  pennyai verification (CLAUDE.md pattern-detection #9).
- Its create path could not work anyway. The INSERT omitted `users.id` (TEXT
  PRIMARY KEY, no default) and wrote an `auth_method` column that does not exist
  on `users` — providers live in `auth_methods`.

Header forwarding is also the wrong mechanism now: Rally identity routes through
timpager's server-to-server identity resolve API, and SSO routes through Clerk
JWKS verification (see the FINAL Clerk satellite plan). Whatever Phase 3 needs
should be built on one of those, not resurrected from git history.

`middleware_timpager.go` still holds `pushRevenueEvent` and `pushPayoutToTimpager`.

Local cookie auth (phone/SMS, Google, email/password, magic link) continues to work — **never break this** per CLAUDE.md.

Timpager's auth verify endpoint (per `/Users/andydownard/projects/timpager/routes/auth.js:345`):
```js
GET /api/auth/verify
  → { valid: true, user_id, phone, display_name, role }
  401 → { valid: false }
```

### Goal
Existing pennyai users can use their phone with timpager's Verify service to sign in to Groflo, and a single Rally login spans Groflo + other Rally apps. Existing Penny accounts are merged into the timpager user graph by phone.

### Plan

**3a. User backfill (one-shot, run before any UI changes)**
- [ ] Write `scripts/backfill_groflo_to_timpager.go` that, for each pennyai user with a phone number:
  1. POST to a new admin endpoint on timpager: `POST /api/admin/users/import` with `{ phone, email, display_name, source: 'groflo' }`
  2. timpager either creates the user OR returns the existing user_id (phone collision means: same person, link them)
  3. Save the timpager user_id back to pennyai's `users` table in a new column `timpager_user_id` (migration: `ALTER TABLE users ADD COLUMN timpager_user_id TEXT`)
- [ ] Add the new admin endpoint on timpager (does not exist yet — needs to be added). It must require an admin API key, not a session.
- [ ] Idempotent: re-run is safe. Each insert uses an idempotency key.
- [ ] Logged to audit_log on both sides.

**3b. "Sign in with Rally" UI**
- [ ] Add a new auth screen option below "Use email instead": "Sign in with Rally"
- [ ] Tapping it opens a popup window to `https://rallytasks.com/auth/redirect?return_url=https://app.groflo.io/auth/rally/callback`
- [ ] timpager handles the auth (Twilio Verify), then redirects back to groflo.io with a session token
- [ ] groflo.io's `/auth/rally/callback` handler validates the token (via the Clerk JWKS path or timpager's identity resolve API — **not** a revived header-forwarding bridge, see Current state) and creates a local session cookie
- [ ] User is now signed in to BOTH Rally and Groflo

**3c. Dual-write window**
- [ ] During Phase 3, every successful Penny phone-auth ALSO creates/updates the corresponding timpager user (idempotent)
- [ ] Length of dual-write window: 30 days minimum, until D7 retention is unaffected
- [ ] After dual-write proves stable, all auth flows can route through timpager

**3d. Fallback**
- [ ] If timpager is unreachable (5xx or network error), pennyai's local auth must continue to work (existing behavior — already in middleware)
- [ ] Add a metric: `auth_fallback_count` to alert if we lose timpager for >10 minutes

### Risk
- **Phone collision**: two different people with the same phone (number reassigned). Handled by timpager's `account_merges` table. Groflo defers to timpager's merge decision.
- **Existing household memberships**: groflo households (`households` table) are NOT migrated to timpager. Timpager only owns the user identity. Households remain a Groflo concern.
- **Don't disable any existing auth method** per CLAUDE.md hard rules.

---

## Phase 4 — Stripe consolidation onto Rally Connect (gated on Phase 3)

### Current state
- pennyai uses Stripe directly for brand payments (`handlers_brand_payments.go`)
- pennyai issues swap rebates as in-app wallet balances; cash-out is currently via direct Stripe transfer (verify path in pennyai)
- timpager has a full Stripe Connect platform (`/Users/andydownard/projects/timpager/lib/stripe.js`) with:
  - `createConnectAccount(userId, email)` → Express account
  - `createAccountLink(stripeConnectId, returnUrl, refreshUrl)` → onboarding URL
  - `transferToUser(stripeConnectId, amountCents, description, idempotencyKey, metadata)` → transfer with idempotency key, metadata includes `marketplace_id`, `marketplace_slug`, `distribution_id`, `transfer_type`
- timpager's `users.stripe_connect_id` column is the canonical store

### Goal
All Groflo → user payouts (swap rebates, cash-out) flow through timpager's Stripe Connect platform, sharing the platform Stripe account. Brand → platform payments stay on pennyai's Stripe (or also migrate — TBD).

### Plan

**4a. Stripe Connect onboarding migration**
- [ ] Existing pennyai users with `stripe_connect_id` already in pennyai users table get migrated:
  - For each user: call `POST {TIMPAGER_URL}/api/admin/users/{id}/stripe-connect/import` with `{stripe_connect_id, source: 'groflo'}`
  - timpager validates with Stripe API that the account exists, then writes to `users.stripe_connect_id` in timpager DB
  - pennyai keeps its local copy as a cache, refreshing from timpager weekly
- [ ] New users sign up: redirect to timpager's onboarding flow when they want to cash out a rebate

**4b. Payout routing**
- [ ] Replace pennyai's direct `stripe.transfers.create` calls with a new `pushPayoutToTimpager` function:
  ```go
  func (s *Server) pushPayoutToTimpager(grofloUserID int, amountCents int, distributionID string) error {
      // Resolve timpager_user_id from local cache
      // POST to {TIMPAGER_URL}/api/marketplaces/penny/payouts
      //   { user_id, amount_cents, distribution_id, idempotency_key: "groflo_payout_{distribution_id}" }
      // 201 = created, 409 = duplicate (treat as success)
  }
  ```
- [ ] Add the timpager endpoint `POST /api/marketplaces/:slug/payouts` (does not exist yet — needs to be added on timpager side)
- [ ] timpager calls `transferToUser(stripeConnectId, amountCents, ...)` with the metadata
- [ ] pennyai records the payout in `swap_rebates` with `status='issued'` and the timpager distribution_id

**4c. Reconciliation**
- [ ] Daily reconciliation job: pennyai's `swap_rebates` totals must match timpager's `distributions` totals scoped to marketplace_slug='penny' (or 'groflo' if we update the slug)
- [ ] Use the existing pattern in timpager's `/api/admin/wallet-reconcile` route

**4d. Brand → platform payments**
- [ ] Decision pending: do brand campaign deposits (`handlers_brand_payments.go`) stay on pennyai's Stripe, or also move to timpager?
- [ ] Argument for keeping on pennyai: simpler accounting, brands invoice Groflo directly
- [ ] Argument for moving: single Stripe account simplifies tax + 1099 generation
- [ ] **Recommendation**: leave brand deposits on pennyai's Stripe for now. Move only user payouts to timpager. Re-evaluate after 90 days.

### Hard rules from timpager CLAUDE.md
- "Stripe transfers MUST use idempotency key: `dist_${distribution.id}`" — already enforced in lib/stripe.js
- "NEVER distribute more than `revenue_share_pct`%" — Groflo's revenue share goes to community pool via timpager's distribution rules
- "NEVER update wallet_balances outside same transaction as distribution INSERT" — timpager's existing reconciliation handles this; pennyai must NOT touch timpager's wallet tables directly

### Risk
- **Stripe API rate limits**: shared platform account means shared rate limits. Coordinate with timpager's rate limit budget.
- **Webhook signature secrets**: each marketplace gets its own webhook signing secret (existing pattern)
- **Money safety**: this is the highest-risk phase. Add extensive negative-path tests. Run in dry-run mode for 7 days before going live.

---

## Phase 5 — Backend identifier sweep (low priority, no user impact)

Internal Go/JS identifiers, file names, env vars containing "thriftly". Kept for last because they have zero user impact and require coordinated changes.

### Items
- [ ] DB filename: `thriftly.db` → leave as-is per panel A1 (Litestream chain). Document the historical name in server.go (already done).
- [ ] Env vars: `GROFLO_*` aliases for any `THRIFTLY_*` env vars (none exist today; nothing to do)
- [ ] Internal Go identifiers (`thriftly` in package main symbols): none exist; package is `package main`
- [ ] Internal CSS class names (`.thriftly-*`): grep finds none beyond brand color hex
- [ ] Test fixtures using `thriftly` strings in expected values: update after Phase 1 ships
- [ ] Node port (`pennyai/node/`): NOT in production; defer or delete entirely
- [ ] frontend-brand/* (brand portal): separate sweep, lower priority. Brand portal users are not consumers.
- [ ] static-brand/* compiled output: regenerated by build, no edit needed
- [ ] pennyai/www/*: copy of /thrift.ly. Decide whether to delete or keep in sync. Recommendation: delete pennyai/www and let /projects/thrift.ly be the only marketing source.
- [ ] Documentation refresh: CLAUDE.md, ROADMAP.md, DATA_FLYWHEEL.md, 11_STAR_NARRATIVE.md, etc. — refresh in a single doc-only PR after Phase 2 cutover

---

## Critical compliance gap to plug BEFORE Phase 1

**Gap**: `handleSendCode` in `handlers_auth.go:653` does not capture or persist SMS opt-in consent. The frontend has the checkbox but the backend never receives or stores the consent. Twilio A2P vetting requires demonstrable consent capture with timestamp.

**Fix** (queued, not yet shipped):
1. Add column: `ALTER TABLE users ADD COLUMN sms_consent_at DATETIME` (migration in `db.go`)
2. Update `PhoneRequest` struct: add `SmsOptIn bool \`json:"sms_opt_in"\``
3. Frontend `useAuthState.handleSendCode`: include `sms_opt_in: smsOptIn` in the POST body
4. Backend `handleSendCode`: write `sms_consent_at = CURRENT_TIMESTAMP` to the user row when `SmsOptIn` is true
5. Reject the request with 400 if `SmsOptIn` is false (matches existing client-side gate, but enforces server-side)
6. Add a negative-path test: re-sending the code does NOT duplicate consent rows
7. Add an audit log entry: `sms_consent_captured` with user_id, IP, user_agent, timestamp

**Why critical**: Twilio campaign vetting will ask "show us evidence of consent capture for each user". Right now we cannot prove it.

---

## Open questions for Andy

1. **Slug**: timpager registers Groflo as marketplace slug `penny`. Do we rename to `groflo` (requires data migration on timpager side) or keep `penny` for backwards compat? **Recommendation: keep `penny` for now**, rename in a coordinated timpager migration later.
2. **pennyai/www**: delete the duplicate marketing site copy or keep in sync with /thrift.ly? **Recommendation: delete `pennyai/www/`.** Verified 2026-04-07: every file in `pennyai/www/src/` differs from the corresponding file in `/projects/thrift.ly/src/` (the Groflo edits are in /thrift.ly only). pennyai/www is now a stale snapshot of the pre-rebrand Thriftly site. Run `rm -rf pennyai/www` after confirming Netlify deploys from /thrift.ly. Andy: tell me which it points at and I'll delete.
3. **Twilio number**: cutover the existing Penny SMS number's display name to Groflo, or provision a new number? **Recommendation: cutover the existing number** — preserves customer SMS history and avoids disruption.
4. **groflo.io domain**: confirm Andy owns it (or has the registration in flight). If not, all Phase 2 work is blocked.

---

## Wave 2 ship list (Phase 0 continuation, 2026-04-07)

After the initial Phase 0 commit, the following landed in the same working tree:

### SMS consent capture (closes the Phase-1-blocking compliance gap)
- `models.go`: `PhoneRequest.SMSOptIn bool` field added
- `schema.go`: migration adds `users.sms_consent_at`, `sms_consent_ip`, `sms_consent_user_agent`, and `timpager_user_id` columns. Repeated in the post-recreation migration block so they survive the legacy users-table recreation.
- `handlers_auth.go`: `handleSendCode` now rejects requests with `sms_opt_in != true` (HTTP 400) and persists consent (timestamp + IP + UA) via `INSERT ... ON CONFLICT(phone_number) DO UPDATE`. Truncates UA to 500 chars.
- `frontend/src/hooks/useAuthState.ts`: `handleSendCode` sends `sms_opt_in: true` and refuses to fire if the checkbox is unchecked.
- `auth_test.go`: existing tests updated; new `TestSendCode_NoConsentRejected` covers both `sms_opt_in: false` and the field being absent. Existing `TestSendCode_Success` now also asserts the consent record persisted with timestamp + IP.
- `coverage_boost_test.go`: 7 existing send-code calls updated to pass `sms_opt_in: true`.

### 301 redirect middleware
- `server.go`: new redirect block at the top of the request handler. Detects `Host` header `thrift.ly`, `www.thrift.ly`, `app.thrift.ly`, `brand.thrift.ly`, `brands.thrift.ly`, `deck.thrift.ly` and 301s to the corresponding `groflo.io` host. Strips port. Logs the redirect. Safe to ship before DNS — takes effect when DNS flips.

### Rebrand banner (panel R7 — Nadia)
- `frontend/src/App.tsx`: new `showRebrandBanner` state initialised from `localStorage.groflo_rebrand_banner_dismissed`. JSX block above the install banner: "**Penny is now part of Groflo.** Same Penny, same list, same household." Dismiss-once persisted; never re-shown. Tracks `rebrand_banner_dismissed` analytics event.

### Rally federation prep (Phase 3 scaffolding)
- `schema.go`: `users.timpager_user_id TEXT` column added (nullable until backfill runs).

### Stripe payout stub (Phase 4 scaffolding)
- `middleware_timpager.go`: new `pushPayoutToTimpager(grofloUserID, amountCents, distributionID)` helper. Currently in `stubMode = true` — logs the intended payout and returns. When the timpager `POST /api/marketplaces/penny/payouts` endpoint ships, flip stubMode to false and the helper begins POSTing with idempotency key `groflo_payout_{distributionID}`. Includes 3-attempt exponential-backoff retry, 30s timeout, and the `409 = success, NEVER retry` rule from timpager's CLAUDE.md.

### Frontend-brand portal (brands.groflo.io)
- `frontend-brand/index.html`: title, OG/Twitter cards, site_name, url all updated
- `frontend-brand/package.json`: name field
- `frontend-brand/src/App.tsx`: nav header (2x), hero copy, footer all reference Groflo

### Config files
- `Dockerfile`: binary name `thriftly` → `groflo` (build artifact + COPY)
- `run.sh`: launches `./groflo` instead of `./thriftly`; comment refers to Groflo server
- `.env.example`: comment explaining why DB_PATH retains the historical name; new `TIMPAGER_URL` and `TIMPAGER_API_KEY` placeholders
- `litestream.yml`: header comment explaining the DB filename retention

### Test fixtures
- `settings_test.go`: vCard assertion updated for `ORG:Groflo` + `penny@list.groflo.io`
- `digest_test.go`: link assertion updated for `https://app.groflo.io`

### Internal docs
- `CLAUDE.md`: title + intro + Rally ecosystem disclosure + brand history paragraph (preserves "formerly Thriftly" context for posterity and migrations)
- `11_STAR_NARRATIVE.md`: product line updated
- `DATA_FLYWHEEL.md`: product line updated

### Test results
- `go test -count=1 ./...`: only one failure, and it's a **pre-existing flake** unrelated to the rebrand:
  - `TestMarketplaceHealth_WithData (admin_test.go:2519): expected 2 health records, got 1`
  - Confirmed pre-existing by stashing the rebrand changes and running the test against pristine main — it fails identically there.
  - Tracked as a separate bug; not blocking the rebrand sprint.
- All SMS consent capture tests pass (success, opt-in absent, opt-in false, invalid phone, twilio fails, full login flow).

---

## Done definition

Phase 0 is "done" when:
- [x] Customer + UX + Architect + PM + BI panels convened, decisions captured
- [x] Privacy + Terms drafted in /thrift.ly canonical location, in-app mirrors updated
- [x] Brand bible identity block updated
- [x] All user-visible PWA + share + opt-in copy updated
- [x] Legacy storage migration shim in place
- [x] SMS consent capture shipped end-to-end (schema + handler + frontend + tests)
- [x] 301 redirect middleware shipped (gated on DNS, ready to take effect)
- [x] Rebrand banner shipped (panel R7)
- [x] Rally federation column added
- [x] Stripe payout stub helper added
- [x] Internal docs (CLAUDE.md, narratives) refreshed
- [x] `go build` clean, `go test ./...` clean except pre-existing TestMarketplaceHealth_WithData flake
- [x] This sprint document queued for Andy review
- [ ] Andy reviews + approves before any merge to main
