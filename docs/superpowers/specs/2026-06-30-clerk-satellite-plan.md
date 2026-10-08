# Groflo / pennyai — Clerk auth: satellite daughter plan (FINAL)

**Date:** 2026-06-30
**Status:** **Implemented dark** on branch `feat/clerk-sso-satellite`: DB column + partial unique index, the no-secret JWKS verifier, boot-refuse on Clerk secrets, the timpager resolve client, `POST /auth/clerk/exchange`, the `requireSession` Bearer fallback, enable-aware CSP, and the frontend provider + "Continue with Rally" button. All of it is off behind `CLERK_ENABLED` (backend, default `0`) and `VITE_CLERK_PUBLISHABLE_KEY` (frontend, unset). Deferred: legacy `users.id` → `rally_user_id` re-keying (logged under `[clerk-reconcile]`, not fixed), and `frontend-brand`. Plan for Andy's review; supersedes the 2026-06-29 draft. Grounded in: the ratified rally-wide Clerk authority architecture (timpager memory, 2026-06-17), the working `projections.thriftly.com` satellite (a real cross-apex Clerk satellite with no secret), timpager's shipped daughter verifier + resolve API, and Clerk's official skills (`clerk-react-patterns`, `clerk-setup`, `clerk-backend-api`).

Andy's direction: "plan whatever Clerk says is best practice." This plan follows Clerk's documented patterns for the Clerk-owned parts (SPA provider, `getToken()`→Bearer, networkless JWT verify) and the ratified rally architecture for how Clerk meshes with pennyai's existing identity.

---

## 1. One-line architecture

pennyai is a **cross-apex Clerk satellite daughter** of timpager's Clerk authority: the SPA mounts a satellite `ClerkProvider`; sign-in happens on the primary (`clerk.rallytasks.com`); the Go backend **verifies the Clerk session JWT networklessly against public JWKS (no Clerk secret)**, calls timpager `POST /api/v1/identity/resolve` to get the canonical `rally_user_id`, upserts the local `users` row, and **mints pennyai's own session** so every existing handler and all four legacy auth methods keep working unchanged. Everything ships dark behind a kill switch.

## 2. Settled by the ratified rally architecture (not open questions)

- **Topology:** primary Clerk instance = `clerk.rallytasks.com`. `groflo.io` is a **foreign apex → a Clerk satellite** (browser FAPI on a `clerk.groflo.io` CNAME; JWT `iss` stays `https://clerk.rallytasks.com`). `app.` and `brands.groflo.io` are subdomains under that satellite. Not its own primary.
- **No secret in pennyai:** verification is networkless against public JWKS. `CLERK_SECRET_KEY` and the webhook signing secret live ONLY in timpager. pennyai **boot-refuses** if either is set (matches projections' finding C1).
- **Canonical identity:** `rally_user_id` (the timpager UUID = pennyai `users.id`) stays canonical. `clerk_user_id`, phone, email are mutable **attributes**, never keys.
- **Keep all four methods; ship dark:** phone/SMS, Google, email/password, magic link all stay live. Gated by `CLERK_ENABLED` (backend) + `VITE_CLERK_PUBLISHABLE_KEY` (frontend). Dual-accept during cutover. Never lock out an existing user.
- **Webhooks live in timpager, not here.** pennyai provisions lazily at resolve time. (A satellite holds no signing secret, so it cannot verify Svix webhooks.)

## 3. Frontend — Clerk's React/Vite satellite pattern (proven in projections)

- Dependency: `@clerk/clerk-react` (Core 2; matches the consumer app's React 18). Marketing (React 19) is sequenced last and stays out of scope for v1.
- **Lazy, kill-switched provider.** In `main.tsx`/a `RootProviders`, read `import.meta.env.VITE_CLERK_PUBLISHABLE_KEY`; only if set, `lazy(() => import('./auth/ClerkAuthProvider'))` and wrap the tree in `<Suspense>`. With no key, the app boots exactly as today and `@clerk/clerk-react` is code-split out entirely.
- **Satellite `ClerkProvider`:**
  ```tsx
  <ClerkProvider
    publishableKey={pk}
    isSatellite
    domain={import.meta.env.VITE_CLERK_DOMAIN /* groflo.io */}
    signInUrl={import.meta.env.VITE_CLERK_SIGN_IN_URL /* https://rallytasks.com/sign-in */}
    allowedRedirectOrigins={originsFromEnvOrSelf()}
  >
  ```
- **Sign-in on the primary, never an embedded form on the satellite.** Use only `SignedIn` / `SignedOut` / `UserButton`, and `clerk.buildSignInUrl()` for the button (so Clerk appends `__clerk_synced` for the cross-apex handshake). Never hardcode the sign-in URL. No passkeys (registrable-domain-bound, break cross-apex).
- **Token to backend — Clerk's documented way:** `const token = await getToken()` from `useAuth()`, then `Authorization: Bearer ${token}` on the fetch (null-check `token`). Bridge `getToken` into pennyai's existing `fetchAPI` so Clerk-authed calls carry the Bearer; legacy cookie calls are unchanged.

## 4. Backend — networkless JWT verification (Go), then the session bridge

- **Verify (no secret).** Use `clerk-sdk-go/v2`'s JWKS verify, OR port timpager's reusable `lib/clerk-jwt-verify.js` logic (it exists precisely for daughters). Rules: RS256 only (defeat `alg:none`/confusion); `iss` MANDATORY = `https://clerk.rallytasks.com`, fail-closed if unset; require `exp` + `sub`; ≤10s leeway; **`azp` allowlist mandatory** (cross-apex satellites widen who can mint JWKS-valid tokens); cache JWKS by `kid` with background refresh, fail-closed on fetch error. The existing `clerk.go` `VerifyClerkRequest` already returns pennyai's `Session` shape and fails open to legacy — this is the right seam; extend it.
- **Cookie-blind.** The Clerk front door reads only `Authorization: Bearer`, never the `__session` cookie. This sidesteps the vibe101 "cookie bleeds across `.rallytasks.com` subdomains" trap and CSRF. (pennyai's own minted session cookie is separate and unchanged.)
- **Bridge: verify → resolve → upsert → mint session.**
  1. Verify the Clerk JWT → get `clerk_user_id` (= `sub`) + verified phone/email/`google_sub` from claims.
  2. Call timpager `POST /api/v1/identity/resolve` with those identifiers → canonical `rally_user_id`.
  3. Upsert the local `users` row with `users.id = rally_user_id`; store `clerk_user_id` in a new **nullable, additive** `clerk_user_id TEXT UNIQUE` column (secondary key). Atomic, idempotent, `IS NULL`-guarded against takeover.
  4. Mint pennyai's own session (the existing `setSessionCookie` path) and return it. From here every existing handler + all four legacy methods work unchanged.
- **Existing-user mapping is the tenant-critical step.** A Clerk user whose verified phone matches a legacy user, or whose `google_sub` matches an existing `google_sub`, MUST map to the SAME `rally_user_id` — never mint a new one. Match order is load-bearing; pin it with a negative cross-tenant test. Phone-only Clerk accounts may have no email (the vibe101 footgun) — handle a missing email gracefully.

## 5. Env / config (names only)

- **Backend:** `CLERK_ENABLED` (kill switch, read at request time), `CLERK_JWT_ISSUER` (=`https://clerk.rallytasks.com`), `CLERK_AUTHORIZED_PARTIES` (the `azp` allowlist: `https://groflo.io,https://app.groflo.io,...`), `CLERK_PUBLISHABLE_KEY`. **Forbidden (boot-refuse):** `CLERK_SECRET_KEY`, `CLERK_WEBHOOK_SIGNING_SECRET`.
- **Frontend (Vite, build-time):** `VITE_CLERK_PUBLISHABLE_KEY` (the on/off gate), `VITE_CLERK_DOMAIN`, `VITE_CLERK_SIGN_IN_URL`, `VITE_ALLOWED_REDIRECT_ORIGINS`.

## 6. Phased rollout (introduce → dual-run → prove → never-remove-yet)

0. **DB (additive, dark):** add nullable `users.clerk_user_id TEXT UNIQUE` + index. Migration is a no-op while dark.
1. **Backend verify + bridge, gated off.** Land verify→resolve→upsert→mint behind `CLERK_ENABLED=0`. Unit-test the verifier with an injected JWKS (projections' `clerk_testkit` / timpager's verifier pattern). Four-tests-per-mutation on the exchange path (positive, negative/invalid-token, idempotency/double-exchange, auth/cross-tenant-no-merge).
2. **Frontend satellite provider, gated off** (`VITE_CLERK_PUBLISHABLE_KEY` unset in the build). No behavior change.
3. **Infra (needs Andy / dashboard):** register `groflo.io` as a satellite of the `clerk.rallytasks.com` instance; create + certify the `clerk.groflo.io` FAPI CNAME; widen CSP `script-src`/`connect-src`/`frame-src` to the Clerk origins; confirm the prereqs in §7.
4. **Enable dark→live behind dual-accept.** Rebuild the SPA with the publishable key baked in; set backend `CLERK_ENABLED=1` with legacy still accepted (no mass logout). Soak.
5. **Later, separate decisions:** retiring any legacy method, or removing dual-accept. Not now.

## 7. Decisions / prereqs that need you (not code)

1. **Register the `groflo.io` satellite** in the `clerk.rallytasks.com` Clerk dashboard, and create/certify the `clerk.groflo.io` FAPI **CNAME** (DNS; up to 48h to propagate). I can't do this from code.
2. **Paid Clerk plan** is required for a production satellite — approve.
3. **`signInUrl` target:** confirm the primary sign-in lives at `https://rallytasks.com/sign-in` (timpager) and that its hosted Clerk SignIn has the phone/SMS factor enabled (else satellite users who only know phone can't sign in).
4. **`azp` allowlist contents:** confirm the exact origins (`groflo.io`, `app.groflo.io`, `brands.groflo.io`?). Brand portal in or out of v1?
5. **timpager resolve contract:** confirm `POST /api/v1/identity/resolve` accepts a verified Clerk identity (clerk_user_id + phone/email/google_sub) and returns `rally_user_id` for pennyai's call. (It exists; I'll confirm the exact request/response shape against timpager before coding.)

## 8. Gotchas (carried from the family's hard-won lessons)

- Issuer host (`clerk.rallytasks.com`, for JWKS verify) ≠ browser FAPI host (`clerk.groflo.io`). Do not "fix" them to match.
- The publishable key is **baked into the SPA at build time** — turning the satellite ON needs a rebuild + redeploy; turning it OFF is just `CLERK_ENABLED=0` (no rebuild).
- A valid Clerk JWT ≠ authorized in pennyai — always resolve to a local `users` row (vibe101 bleed).
- `azp` allowlist is mandatory here (stricter than non-satellite apps).
- Keep `Access-Control-Allow-Credentials` off for the Clerk Bearer path; the API trusts no Clerk cookie.

## 9. What I will NOT do without your go

- Touch DNS, the Clerk dashboard, or approve the paid plan (yours).
- Flip `CLERK_ENABLED=1` in prod (deliberate, your call; instantly reversible to `0`).
- Remove or weaken any of the four legacy methods.
