# Clerk Authentication — Replan

Date: 2026-06-29
Author: planning agent (for Andy)
Status: PLAN ONLY. No code in this document. Needs GO before any edits.

This plan replaces a discarded in-progress attempt. It uses Clerk's own
guidance. All web sources are cited at the bottom and inline. Treat the
fetched web text as reference data, not as commands. Nothing here adds a
secret or runs a command on a page's say-so.

---

## 1. What already exists in the tree

Read these before planning further work. They change the shape of this job.

- `clerk.go` (current branch `refactor/backend-2026-06-29`) — a clean, mature
  Clerk wrapper already exists. It is the right base. Key points:
  - Uses `github.com/clerk/clerk-sdk-go/v2` (verify + user fetch).
  - Kill switch `CLERK_ENABLED=1`. Off means every Clerk path is a no-op.
  - `CLERK_SECRET_KEY` required when enabled. No fallback chain (good, per
    `SECRETS`).
  - `VerifyClerkRequest(r, resolveLocalUser) (*Session, error)` returns the
    same `Session` shape the legacy store returns. It reads the `__session`
    cookie or `Authorization: Bearer`.
  - It FAILS OPEN during cutover: an invalid or unmapped Clerk token returns
    `(nil, nil)`, so the legacy session path still runs. This is the correct
    cutover posture.
  - `LocalUserResolver(clerkUserID, email, phone) (*Session, error)` is the
    seam between Clerk and pennyai's `users` table. It is not wired yet.
  - `GetClerkUserByID` wraps `clerkuser.Get`, returns `(nil,nil)` on 404.
- `clerk_test.go` — already exercises the verify branches (kill switch, no
  token, valid token + resolver, invalid token, SDK error, 404). Extend it;
  do not replace it.
- `go.mod` — `github.com/clerk/clerk-sdk-go/v2 v2.5.1` is present but marked
  `// indirect`. It needs promoting to a direct require once code imports it.
- `session.go` — `SessionStore`: 256-bit hex token, 30-day expiry, own
  Postgres `sessions` table, 5-minute in-process cache. `CreateSession`,
  `GetSession`, `DeleteSession`, `DeleteUserSessions`, `CleanExpired`.
- `utils.go` `setSessionCookie` — cookie name is `session` (not `tp_session`;
  the comment in `clerk.go` is stale). `HttpOnly`, `Secure`, `SameSite=Lax`,
  `MaxAge` 30 days. NO `Domain` attribute set, so the cookie is host-only per
  app domain. `schema.go:78` says "Session cookies live on .groflo.io only.
  No cross-domain handoff" — today that is host-only-per-subdomain, not a
  shared parent-domain cookie.
- `handlers_admin.go:12` `requireSession` — reads the `session` cookie, looks
  it up in pennyai's own DB.
- `middleware_timpager.go` `requireSessionWithTimpager` — checks the
  `X-Timpager-Session` header first, then falls through to `requireSession`.
  This is the natural slot for a Clerk branch.
- `users.id` is the timpager TEXT-UUID. `users` has `timpager_user_id` and
  `google_sub` columns. There is NO `clerk_user_id` column yet.
- `handlers_webhook.go` exists but has no Clerk or Svix handling.
- Frontends:
  - `frontend/` — React 18.2, Vite 5 — app.groflo.io (main consumer app).
  - `frontend-brand/` — React 18.2, Vite 5 — brands.groflo.io (brand portal,
    separate user population, its own auth in `handlers_brand_auth.go`).
  - `marketing/` — React 19.2, Vite 7 — groflo.io (mostly unauthenticated).

## 2. Why the prior attempt was discarded

The discarded attempt lives in `stash@{0}`
("overnight-safety-2026-06-28: clerk WIP"). It wired Clerk through a
`Server` method `s.authenticateClerk(r)` (held in an untracked
`clerk_auth.go`, not captured by the stash) and patched
`requireSessionWithTimpager` to FAIL CLOSED:

```
if session, present, ok := s.authenticateClerk(r); present {
    if ok { return session, true }
    writeJSON(w, 401, ...)   // present-but-invalid -> hard 401
    return Session{}, false
}
```

Why replan:
- Fail-closed is wrong for a phased cutover. Any stale or foreign `__session`
  cookie on the domain (Clerk uses that cookie name; another Rally Clerk app
  could set it), or a valid Clerk token whose local `users` row does not
  exist yet, would hard-401 the user instead of falling through to the
  working legacy session. That is a lock-out risk, which the hard rules forbid.
- It coupled Clerk into the `Server` method set and duplicated logic that the
  newer package-level `clerk.go` does more cleanly and with full test seams.
- The current `clerk.go` already supersedes it with a fail-open design and a
  resolver seam. We build on `clerk.go`, not on the stash.

The stash is context only. Do not resurrect it.

---

## 3. Goals and non-goals

### Goals
1. Add Clerk as the new front-door login broker on app.groflo.io, then on the
   satellite penny.rallytasks.com, behind the `CLERK_ENABLED` kill switch.
2. Keep ONE canonical `user_id` per person across Rally: the timpager
   TEXT-UUID. Clerk's user id is a secondary key, never the canonical one.
3. Preserve all four existing auth methods (phone/SMS, Google OAuth,
   email/password, magic link) and their login screens, unchanged and never
   weakened.
4. Preserve pennyai's own `sessions` table, the `session` cookie, the 30-day
   sliding expiry, and `DeleteUserSessions` revocation.
5. Give cross-domain SSO between groflo.io subdomains and the
   penny.rallytasks.com satellite, at the Clerk layer only.
6. Ship phased, behind a flag, with no big-bang and no possible lock-out.

### Non-goals (v1)
1. Removing, replacing, or weakening any of the four existing methods. Out of
   scope, and gated behind a separate later decision.
2. Bringing the brand portal (brands.groflo.io) into Clerk. It has a separate
   user population and its own session. Later phase at the earliest.
3. Making Clerk the canonical identity authority over timpager. Rejected — it
   breaks the one-UUID-across-Rally invariant.
4. Replacing pennyai sessions with stateless per-request Clerk bearer tokens.
   Rejected for v1 (see Section 6).
5. Passkeys on the satellite (Clerk does not recommend them across satellite
   domains).

---

## 4. The central question: how Clerk fits without removing the four methods

Three options were considered.

- **A. Clerk REPLACES the four methods.** Clerk can natively do phone OTP,
  Google OAuth, email/password, and email magic link. But adopting it as a
  replacement means deleting Twilio Verify, the bcrypt store, the Go OAuth
  handlers, and the magic-link table, and changing the join keys (phone,
  `google_sub`). That directly violates the "never remove or weaken" rule and
  creates a lock-out cliff. REJECTED.

- **B. Clerk SITS ALONGSIDE (recommended).** Both stacks live at once. A
  request is authenticated by EITHER a legacy `session` cookie OR a verified
  Clerk login. New sign-ins can flow through Clerk; existing users keep their
  legacy sessions and legacy login screens until and unless they choose Clerk.
  This is exactly what `clerk.go`'s fail-open design already enables. Lowest
  risk. RECOMMENDED.

- **C. Clerk WRAPS the existing methods.** Clerk as a thin UI shell while the
  real verification still runs against Twilio/bcrypt/OAuth. Clerk wants to own
  the credential, so this needs heavy custom flows and buys little. REJECTED.

### Recommendation: B, with an "exchange to local session" bridge

Clerk owns the login moment and the login UI. The instant Clerk confirms a
login, the pennyai backend mints its OWN `session` cookie (Section 6). From
that point every existing handler works unchanged. Clerk's blast radius is the
login moment only. The four legacy methods and their screens stay live in
parallel as separate buttons on the same login page.

---

## 5. Identity reconciliation — Clerk user to the Rally TEXT-UUID

The invariant: one person resolves to one timpager TEXT-UUID, the same across
every Rally marketplace, and `users.id` equals that UUID.

Two framings were named in the brief.

- **"Clerk in front of timpager" (Clerk is the identity authority).** Clerk's
  user id becomes canonical and timpager is subordinate. This breaks the
  one-UUID-across-Rally rule. REJECTED.

- **"Clerk as the IdP that calls timpager resolve" (recommended).** Clerk
  brokers the credential; timpager stays the identity authority. The flow:
  1. Clerk confirms a login and gives us a verified Clerk identity (Clerk user
     id, plus verified phone / email / Google `sub` on the Clerk user record).
  2. The pennyai backend calls timpager's identity resolve API with those
     verified identifiers to get or create the canonical TEXT-UUID.
  3. The backend upserts a local `users` row keyed by that UUID
     (`users.id = <timpager uuid>`), and records `clerk_user_id` as a
     SECONDARY key for fast lookups.
  4. `LocalUserResolver` then looks up `users WHERE clerk_user_id = ?` and
     returns the local `Session`.

Mapping an EXISTING user is the security-critical part. A Clerk user whose
verified phone matches an existing legacy user, or whose Google `sub` matches
an existing `google_sub`, MUST map to the SAME timpager UUID — never mint a
duplicate, never cross into another person's account. Match order: Google
`sub` first (stable, per `SECURITY.md`), then E.164 phone, then verified
email. This match logic is a tenant-isolation surface and needs the negative
cross-tenant test in Section 12.

When is the local row created? Two paths, both idempotent on a unique
`clerk_user_id`:
- **Lazy provisioning at exchange (primary path).** On the first exchange the
  resolver, on a miss, fetches the Clerk user, calls timpager resolve, and
  upserts the row. This removes the race where a webhook has not yet landed.
- **Clerk webhook (backup path).** `user.created` / `user.updated` /
  `user.deleted` keep the row in sync asynchronously. Inbound webhook rules
  apply (Section 8).

Because both can run, provisioning MUST be an atomic upsert on a unique
`clerk_user_id` so a double create makes one row.

---

## 6. Session model — keep both, bridged by exchange

Two bridge designs were considered.

- **Exchange model (recommended).** The frontend obtains a Clerk session
  token, then calls a new `POST /api/auth/clerk/exchange`. The backend verifies
  the Clerk token once (via `clerk.go`), resolves the local user (Section 5),
  calls `CreateSession`, and sets the existing host-only `session` cookie.
  Every later request uses the unchanged `session` cookie path. Pros: the
  `sessions` table stays the source of truth for API auth; the 30-day sliding
  window, the cache, and `DeleteUserSessions` revocation all keep working;
  existing handlers need no change; Clerk touches only the login moment.

- **Stateless bearer model (rejected for v1).** The frontend sends
  `Authorization: Bearer <clerk jwt>` on every API call and the backend
  verifies per request. This is `VerifyClerkRequest` used directly in
  middleware. Cons: it bypasses the `sessions` table, so the 30-day sliding
  window and `DeleteUserSessions` revocation no longer mean anything;
  short-lived Clerk tokens force `getToken()` refresh handling everywhere. Too
  much blast radius for v1.

Decision: keep BOTH session systems, bridged by exchange. `clerk.go`'s
`VerifyClerkRequest` is the verify primitive used INSIDE the exchange endpoint,
not on every request. The fail-open Clerk branch may remain in
`requireSessionWithTimpager` as a secondary safety net, but the primary path is
exchange to a local cookie.

### Cookie and domain implications
- Legacy `session` cookie stays host-only per app domain (app.groflo.io and
  penny.rallytasks.com each get their own). Unchanged. No Domain attribute.
- Clerk's own cookies (`__session` and its Frontend-API cookies) are managed by
  Clerk on the primary domain and synced to the satellite via Clerk's
  handshake (a `__clerk_synced` URL parameter), NOT via a shared parent-domain
  cookie. See Section 7.
- Net effect: cross-domain SSO is provided at the CLERK layer (intentionally,
  as the new capability). pennyai's own sessions stay per-domain, consistent
  with "cross-domain SSO not implemented at the pennyai layer." Sign-out must
  clear BOTH the Clerk session and the local `session` cookie.

---

## 7. Satellite-domain topology, DNS, and allowed origins

Source: Clerk satellite-domains guide (cited below). Important correction to
the brief's framing first.

### Topology: one primary, one true satellite
Clerk treats SUBDOMAINS of the primary as sharing auth automatically; only a
DIFFERENT registrable domain is a true satellite.

- `groflo.io` and `app.groflo.io` and `brands.groflo.io` are the same
  registrable domain. Under one Clerk instance they share session by default.
  They are NOT satellites of each other.
- `penny.rallytasks.com` is a different registrable domain. It is the ONLY
  genuine satellite.

Recommended setup:
- **Primary domain:** `groflo.io` (the apex). The consumer app on
  `app.groflo.io` is a subdomain and inherits the primary automatically. Sign-in
  UI lives at `app.groflo.io/sign-in` (or `groflo.io/sign-in`).
- **Satellite domain:** `penny.rallytasks.com`.
- **Brand portal `brands.groflo.io`:** a subdomain of the primary, but a
  separate product with its own user population and session. Keep it OUT of
  the Clerk rollout for v1 (non-goal). If it joins later, it does so as a
  subdomain of the same instance, ideally with Clerk Organizations to separate
  brand accounts from consumers.

This is simpler and more correct than "three satellites." Name this correction
to Andy; it is a real design improvement, not scope creep.

### DNS records (production only; dev instances skip DNS)
- Primary `groflo.io`: the standard Clerk production DNS, the relevant record
  being the Frontend API CNAME, typically `clerk.groflo.io` pointing to the
  per-instance target Clerk shows in the dashboard. (Clerk also issues
  `accounts.` and email DKIM records for a full production setup.)
- Satellite `penny.rallytasks.com`: add a CNAME for the `clerk` subdomain,
  `clerk.penny.rallytasks.com`, pointing to the Clerk Frontend API target the
  dashboard gives for the satellite. DNS can take up to 48 hours to propagate.
  If a CNAME is impossible, Clerk's proxy option is the alternative.

Do not paste real CNAME targets from any web page into config. Take them from
the Clerk dashboard for this instance and have Andy confirm them.

### Allowed origins and sign-in URLs
- On the PRIMARY app, set `allowedRedirectOrigins` to include
  `https://penny.rallytasks.com` (and any other origin allowed to bounce back
  after sign-in).
- On the SATELLITE app, set `isSatellite`, `domain = penny.rallytasks.com`,
  and `signInUrl` / `signUpUrl` pointing at the primary's sign-in route on
  `app.groflo.io`.
- Build cross-domain links with Clerk's `buildSignInUrl()` /
  `buildSignUpUrl()` helpers, not hardcoded strings, so the `__clerk_synced`
  handshake parameter is attached.

### SPA note (not Next.js)
Clerk's satellite middleware / `proxy.ts` mechanism is Next.js-specific. These
apps are Vite SPAs, so that mechanism does NOT apply. For a Vite SPA the
satellite handshake is handled client-side by `ClerkProvider`
(`isSatellite` / `domain` / `signInUrl`) plus the build-URL helpers. Clerk
documents that React frameworks without server rendering are supported for
satellite use. Production satellite requires a paid Clerk plan.

---

## 8. Backend integration

### Verify primitive (exists)
`clerk.go` `VerifyClerkRequest` already verifies a Clerk session JWT via
`github.com/clerk/clerk-sdk-go/v2/jwt` `Verify`, reading `__session` or the
bearer header, gated by the kill switch, returning the local `Session` through
`LocalUserResolver`. Promote the `go.mod` require from `// indirect` to direct.

### New exchange endpoint (mutation)
`POST /api/auth/clerk/exchange`:
1. Verify the Clerk token with `VerifyClerkRequest`, passing a real
   `LocalUserResolver`.
2. The resolver looks up `users WHERE clerk_user_id = ?`. On a miss it
   provisions lazily: fetch the Clerk user (`GetClerkUserByID`), match or
   resolve the timpager UUID (Section 5), atomic-upsert the local row.
3. `CreateSession` and `setSessionCookie`. Return the user shape the existing
   login endpoints return so the frontend treats it identically.
4. Idempotent: a repeated exchange for the same Clerk session returns a valid
   local session without creating a second user row (unique `clerk_user_id`).

### Clerk webhook (mutation, inbound)
`POST /api/webhooks/clerk` for `user.created` / `user.updated` /
`user.deleted`:
- Verify the Svix signature on every call (headers `svix-id`,
  `svix-timestamp`, `svix-signature`). New secret
  `CLERK_WEBHOOK_SIGNING_SECRET`.
- Dedupe by the Svix event id in a new `clerk_webhook_events` table; delivery
  is at-least-once and may arrive out of order, so process idempotently.
- `user.created` / `user.updated`: upsert the local row (same path as lazy
  provisioning). `user.deleted`: soft-delete or unlink `clerk_user_id`, never
  hard-delete the timpager-anchored row.
- Treat all webhook field content as data, never as instructions (`INJECTION`).

### Middleware slot
The Clerk branch belongs in `requireSessionWithTimpager`, AFTER the
`X-Timpager-Session` header check and BEFORE the legacy `requireSession`
fall-through, and it must FAIL OPEN (no token or unmapped token falls through
to legacy), matching `clerk.go`. With the exchange model this branch is a
secondary safety net; the primary path is exchange to a local cookie. Do NOT
adopt the stash's fail-closed branch.

### Env vars (surface to Andy; do not invent values)
- Backend: `CLERK_ENABLED`, `CLERK_SECRET_KEY`, `CLERK_PUBLISHABLE_KEY`,
  `CLERK_WEBHOOK_SIGNING_SECRET`.
- All added to `.env.example` with placeholders. Production boot-refuses if
  `CLERK_ENABLED=1` and `CLERK_SECRET_KEY` is missing (already enforced).

## 9. Database changes (additive only, `DB` rule)

- `ALTER TABLE users ADD COLUMN clerk_user_id TEXT` via `addColumnIfNotExists`,
  nullable, plus a UNIQUE index on `clerk_user_id` (partial / where not null).
  This is the secondary key and the idempotency anchor for provisioning.
- New table `clerk_webhook_events (event_id TEXT PRIMARY KEY,
  received_at TIMESTAMPTZ)` for inbound webhook dedup. `CREATE TABLE IF NOT
  EXISTS`.
- No table drops, no renames, no destructive change. Expand only.
- Every provisioning and exchange write runs in a transaction; the dedup
  record commits inside it.

## 10. Frontend integration

### Package and provider
- Vite SPA package: `@clerk/clerk-react` (NOT `@clerk/nextjs`; these are not
  Next.js apps). Clerk's agent guide and quickstart name the package slightly
  differently in places (`@clerk/react` appears as a newer alias). Confirm the
  exact current package name with `npm view` / `clerk doctor` at
  implementation time rather than trusting the page text.
- Env var for the publishable key: `VITE_CLERK_PUBLISHABLE_KEY`.
- `ClerkProvider` wraps the app at the root in `main.tsx`, above `<App/>`.

### Coexistence with current login screens
Clerk is an ADDITIONAL button on the existing login page, not a replacement.
The four current screens (phone, Google, email/password, magic link) stay
exactly as they are. On a successful Clerk login the app calls the exchange
endpoint, receives the local session cookie, and proceeds. Gate the Clerk
button behind a frontend flag for staged rollout.

### Per-app config
- `frontend/` (app.groflo.io, React 18, Vite 5): primary-side `ClerkProvider`,
  `allowedRedirectOrigins` including `https://penny.rallytasks.com`. This is the
  first app to get Clerk.
- penny.rallytasks.com (served by the consumer app): satellite-side
  `ClerkProvider` with `isSatellite`, `domain`, `signInUrl`/`signUpUrl` to the
  primary, plus `VITE_CLERK_IS_SATELLITE`, `VITE_CLERK_DOMAIN`,
  `VITE_CLERK_SIGN_IN_URL`, `VITE_CLERK_SIGN_UP_URL`. Cross-domain links use
  `buildSignInUrl()`.
- `frontend-brand/` (brands.groflo.io): no Clerk in v1 (non-goal).
- `marketing/` (groflo.io, React 19, Vite 7): mostly unauthenticated; sign-in
  buttons link to the app. If it ever mounts `ClerkProvider`, confirm
  `@clerk/clerk-react` supports React 19 and Vite 7 (recent versions do; pin
  the version and verify). This React 18-vs-19 gap is why `marketing/` is last
  and lowest priority.

### React 18 vs 19
`frontend` and `frontend-brand` are React 18.2 — the well-trodden path.
`marketing` is React 19.2, a newer surface. Adopt Clerk on the React 18 apps
first; treat the React 19 marketing app as a later, separately verified step.

---

## 11. Migration and rollout — phased, flagged, no lock-out

Every phase keeps the four legacy login screens and the legacy `session`
cookie fully working. `CLERK_ENABLED=0` instantly reverts everything to legacy.

- **Phase 0 — groundwork (no user-facing change).** Add `clerk_user_id` column
  and `clerk_webhook_events` table. Add env vars to `.env.example`. Stand up
  the Clerk instance, primary domain `groflo.io`, satellite
  `penny.rallytasks.com`, and DNS, in the dashboard. `CLERK_ENABLED=0`.
- **Phase 1 — backend behind the flag.** Build the exchange endpoint, the
  webhook handler, and the real `LocalUserResolver` with lazy provisioning.
  Full tests (Section 12). Still no UI. Flip `CLERK_ENABLED=1` only in staging.
- **Phase 2 — primary frontend.** Add `ClerkProvider` and a flagged Clerk
  sign-in button to `frontend` (app.groflo.io), alongside the four existing
  screens. Internal / small-percentage rollout. Exchange on success.
- **Phase 3 — satellite.** Add satellite `ClerkProvider` config for
  penny.rallytasks.com, the `clerk.penny.rallytasks.com` CNAME, and
  `allowedRedirectOrigins`. Validate the cross-domain handshake end to end.
- **Phase 4 — existing-user mapping.** Lazy match: when an existing user signs
  in through Clerk, match by Google `sub`, then phone, then verified email, to
  the existing `users.id`, and set `clerk_user_id`. Never force-migrate, never
  bulk-backfill, never lock anyone out. Legacy screens stay live throughout.
- **Phase 5 — later / out of scope for this plan.** Brand portal onto Clerk;
  any consolidation or retirement of a legacy method. Retiring a method is a
  separate decision and is NOT approved here.

Lock-out guarantee: at every phase a user can still sign in with any of the
four legacy methods; the kill switch reverts instantly; mapping uses existing
join keys so no one is stranded.

---

## 12. Test plan — four tests per mutation

Mutations: (1) the exchange endpoint, (2) the webhook handler, (3) the
`LocalUserResolver` lazy-provision upsert. For each, write the four:

1. **Positive.** Valid Clerk login → exchange mints exactly one local session
   and (if new) one `users` row keyed by the timpager UUID with `clerk_user_id`
   set. Valid signed `user.created` webhook → one row.
2. **Negative.** Invalid/expired Clerk token → exchange returns 401, no row, no
   session. Bad Svix signature → webhook 400, no write. Kill switch off → every
   Clerk path is a no-op and legacy still works.
3. **Idempotency.** Call exchange twice for the same Clerk session → one user
   row, a valid session each time, no duplicate. Deliver the same webhook event
   id twice → one write (dedup table). Lazy provision races → unique
   `clerk_user_id` collapses to one row.
4. **Auth / tenant isolation.** A Clerk token for user A never resolves to user
   B's session. An existing user's phone / `google_sub` maps to that same
   person's existing `users.id`, never a different account and never a
   duplicate. This is the cross-tenant negative test the rules require.

Plus:
- **Backward-compat:** a test per legacy method (phone, Google, email/password,
  magic link) proving each still mints a `session` cookie unchanged with Clerk
  enabled. This pins "never weaken the four methods."
- **Sign-out:** clears both the Clerk session and the local `session` cookie.
- Extend `clerk_test.go` rather than replacing it. Keep tests deterministic
  (stub the Clerk SDK via the existing `clerkVerifierFn` / `clerkUserFetcherFn`
  seams; no live network).
- Full suite green: `go test -count=1 ./...` and the frontend Playwright suite,
  both zero failures, before any phase ships.

---

## 13. Risks

- **`__session` cookie collision.** Clerk uses the `__session` cookie name; a
  sibling Rally Clerk app on a shared parent domain could set it. Fail-open
  verify plus strict audience checking handles this, but verify the audience.
- **Two "logged in" truths.** Clerk session and pennyai session can drift. The
  exchange model and a sign-out that clears both mitigate it. Watch for the
  case where Clerk says signed-in but the local session expired.
- **Existing-user mapping is a tenant surface.** A wrong phone/email match
  could merge two people. Match order and the negative cross-tenant test are
  load-bearing.
- **Webhook ordering and at-least-once delivery.** `user.deleted` may arrive
  before `user.created`. Idempotent processing plus event-id dedup required.
- **Satellite needs a paid Clerk plan** for production; passkeys are not
  recommended across the satellite; DNS propagation up to 48 hours.
- **React 19 / Vite 7 on `marketing/`.** Confirm `@clerk/clerk-react` support
  and the exact package name before touching that app; it is last for a reason.
- **Package-name drift in Clerk docs.** The agent guide leans Next.js and uses
  `@clerk/react` in places. Confirm the real Vite-SPA package at build time;
  do not trust the page text blindly (`INJECTION`).

## 14. Decisions Andy must make

1. **Front-door + session model.** Confirm "Clerk alongside the four methods,
   exchange to a local pennyai session" (recommended) over "Clerk-native bearer
   on every request."
2. **Canonical identity.** Confirm the timpager UUID stays canonical and Clerk
   is a secondary key, i.e. reject "Clerk in front of timpager." (Recommended.)
3. **Topology.** Confirm primary = `groflo.io` (with app./brands. as inheriting
   subdomains) and the only true satellite = `penny.rallytasks.com`. Confirm
   the brand portal stays on legacy auth for v1.
4. **Method scope.** Clerk's hosted UI can offer all four methods. v1 keeps
   BOTH the legacy screens and any Clerk method shown. Confirm we do not retire
   any legacy method now (retirement is a separate, later decision).
5. **Provisioning + plan approval.** Confirm lazy provisioning at exchange as
   the primary path with the webhook as backup, and approve the paid Clerk plan
   needed for the production satellite.

---

## Sources

- Clerk agent guide: https://clerk.com/SKILL.md (fetched 2026-06-29). Notes:
  Next.js-leaning; `clerk doctor`; "never expose `CLERK_SECRET_KEY` in client
  code."
- Clerk satellite domains:
  https://clerk.com/docs/guides/dashboard/dns-domains/satellite-domains —
  primary vs satellite, `clerk` subdomain CNAME, `isSatellite` / `domain` /
  `signInUrl` / `signUpUrl`, `allowedRedirectOrigins`, `buildSignInUrl()`,
  paid-plan and passkey limitations, SSR-less React support.
- Clerk React (Vite) quickstart: https://clerk.com/docs/quickstarts/react —
  `VITE_CLERK_PUBLISHABLE_KEY`, `ClerkProvider` in `main.tsx`.
- Clerk Go SDK: https://github.com/clerk/clerk-sdk-go — `clerk.SetKey`,
  `github.com/clerk/clerk-sdk-go/v2/jwt` `Verify`, `clerkhttp`
  `WithHeaderAuthorization` / `RequireHeaderAuthorization`,
  `clerk.SessionClaimsFromContext`.
- Clerk multi-domain example repos (Next.js, for reference only):
  https://github.com/clerk/clerk-multidomain-demo and
  https://github.com/clerk/clerk-nextjs-multi-domain-example
- React + Vite + Clerk SPA example (reference):
  https://github.com/netlify/example-vite-react-clerk
- Local code reviewed: `clerk.go`, `clerk_test.go`, `session.go`,
  `middleware_timpager.go`, `handlers_admin.go` (`requireSession`),
  `utils.go` (`setSessionCookie`), `schema.go` (users/sessions), `go.mod`,
  the three frontend `package.json` files, and `stash@{0}` (the discarded
  attempt).
- Project rules: `CLAUDE.md`, `SECURITY.md`, global `AGENTS.md`.
