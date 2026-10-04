# Groflo — Security

## Secret Management

All secrets in environment variables. No exceptions. No `.env` committed.

| Secret | Env var |
|--------|---------|
| Postgres connection | `DATABASE_URL` |
| Claude API | `ANTHROPIC_API_KEY` |
| Twilio SID + token (optional; SMS is off without them) | `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN` |
| Twilio phone number | `TWILIO_PHONE_NUMBER` |
| Twilio messaging service | `TWILIO_MESSAGING_SERVICE_SID` |
| Stripe secret key + webhook | `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET` |
| SendGrid API key | `SENDGRID_API_KEY` |
| Litestream credentials | `LITESTREAM_ACCESS_KEY_ID`, `LITESTREAM_SECRET_ACCESS_KEY` |
| Sentry DSN | `SENTRY_DSN` |
| PostHog API key | `POSTHOG_API_KEY` |
| LogoDev credentials | `LOGODEV_CLIENT_ID`, `LOGODEV_CLIENT_SECRET` |
| Google Places API key | `GOOGLE_PLACES_API_KEY` |
| Admin API key | `ADMIN_API_KEY` |

pennyai must NEVER hold `CLERK_SECRET_KEY` or `CLERK_WEBHOOK_SIGNING_SECRET`; it refuses to boot if either is set. The Clerk settings it does read (`CLERK_ENABLED`, `CLERK_JWT_ISSUER`, `CLERK_AUTHORIZED_PARTIES`) are public configuration, not secrets.

NEVER commit, log, or expose secrets. Generic error messages to clients. If a secret is committed accidentally, **rotate immediately** — deleting the commit is not enough. The `pre-commit` hook runs checks and rejects commits with API key patterns.

---

## Cross-User & Cross-Household Isolation (CRITICAL)

A violation is a security incident. One household must never see another household's shopping lists, members, or brand deals. One user must never see another user's data.

**Enforcement pattern:**
```go
// CORRECT: household_id from auth context
householdID := session.HouseholdID // derived from user → household membership
lists := db.QueryContext(ctx,
  "SELECT * FROM lists WHERE user_id = ? AND household_id = ?", 
  userID, householdID)

// WRONG: household_id from request parameter (could be hijacked)
lists := db.QueryContext(ctx,
  "SELECT * FROM lists WHERE user_id = ? AND household_id = ?", 
  userID, req.HouseholdID)
```

**Membership verification:** Every query scopes to the current user's households. Query the `household_members` table to verify membership server-side, NEVER trust the client.

**Testing:** `test/cross-household.test.js` simulates two households, attempts data access across boundaries, and asserts failure on every cross-household access attempt.

---

## Authentication

Sign-in is **Rally sign-in through Clerk** ("Continue with Rally") and nothing else. Owner decision 2026-10-03: phone code (Twilio Verify), Google OAuth, email + password, magic link, password reset and the auth-link endpoint were removed. Their ten endpoints (`retiredAuthPaths` in `server.go`) answer **410**. Do not reintroduce a legacy sign-in method, or any Timpager integration, without Andy. The brand portal's own email + password login (`handlers_brand_auth.go`, `brand_users` and `brand_sessions`) is separate and unchanged.

### Rally Sign-in (Clerk)

Clerk sign-in is on only when `CLERK_ENABLED=1`; otherwise `POST /auth/clerk/exchange` answers 404. The app runs at `https://groflo.rallytasks.com`, the same apex as Rally's Clerk, so Clerk runs first-party; the frontend falls back to satellite mode only if `VITE_CLERK_DOMAIN` is set. pennyai only verifies; Clerk owns the credential UI.

- **No Clerk secret, ever.** Verification uses the issuer's **public** JWKS (`CLERK_JWT_ISSUER` + `/.well-known/jwks.json`), fetched with plain HTTP and cached per issuer. Clerk's Backend API is never called. The server **refuses to start** if `CLERK_SECRET_KEY` or `CLERK_WEBHOOK_SIGNING_SECRET` is set, independent of `CLERK_ENABLED`, because holding the secret is itself the misconfiguration.
- **Bearer only; cookie-blind.** Only `Authorization: Bearer <jwt>` counts. Clerk's `__session` cookie is **never read**, so a cookie minted for another app on the shared apex cannot authenticate anyone here.
- **Checks beyond the SDK.** `clerk-sdk-go`'s `jwt.Verify` checks the signature, `exp`/`nbf` (10s leeway) and `azp`, but its issuer check is only a *pattern* and it does not require `exp` or `sub` to be present. pennyai adds: an RS256-and-`kid` header pre-check **before any key lookup** (defeats `alg: none` and HS256 confusion), JWK pinning (`kty` RSA, `use` sig, `alg` RS256), **exact** issuer equality, and `exp`/`sub` presence.
- **azp allowlist is mandatory.** `CLERK_AUTHORIZED_PARTIES` must be non-empty or verification is refused outright; a token with no `azp` is always rejected.
- **Identity is local.** The verified token is the whole identity: `users.clerk_user_id` is the join key and pennyai mints its own `users.id`. No other service is called.
- **No account takeover.** A `users` row already bound to one `clerk_user_id` is never rebound to another: the bind is an atomic conditional `UPDATE … WHERE clerk_user_id IS NULL`, and losing that race returns **409**, mints no session, and writes nothing. A partial unique index (`idx_users_clerk_user_id`) is the database-level backstop.
- **Mapping order** (`upsertClerkUser`): `clerk_user_id` match (repeat sign-in); else the token's **verified** phone on a row with no Clerk identity yet (a household member who first reached Penny by text); else a new row. **Email is never a matching signal.**
- **Phone numbers.** A phone is stored only when the token carries `phone` AND `phone_verified` is true; `users.phone` routes inbound texts to a household's list, so an unverified number must never land there. This needs the Clerk instance to emit the custom session-token claims `email`, `phone`, `phone_verified` and `name`. `PATCH /api/settings` does not accept `phone`.
- **Admin.** A session is admin when its phone equals `ADMIN_PHONE`, or its user's Clerk id is in `ADMIN_CLERK_USER_IDS`. Decided server-side (`isAdmin`).
- **Session** is the ordinary pennyai session: 256-bit token, HttpOnly cookie, SameSite=Lax, 30-day expiry. Nothing about session handling is special-cased for Clerk.
- **Test login.** `TEST_MODE=true` enables `POST /auth/test-login` (mints a session without credentials) and swaps in an SMS client that only logs. The server refuses to boot with `TEST_MODE=true` on Railway.
- **Logging.** Never a token, an email, or a phone.

**Session Management:**
- 256-bit random session tokens (32 bytes from `crypto/rand`, hex-encoded).
- Stored in the `sessions` table (`token` column, with `user_id`, `phone`, `created_at`, `last_used`, `expires_at`). The token is not hashed at rest.
- HttpOnly cookies. SameSite=Lax. Secure on Railway.
- Sessions expire 30 days after creation; `last_used` is updated on use.
- Session is per app, per device — user must sign into each separately.

---

## Notification Deduplication

Duplicate SMS or email is a trust-destroying event. Every outbound path MUST be idempotent.

**SMS/Email Sending Rules:**
1. Every Twilio SMS call MUST include an idempotency key: `sent_sms_key = hash(user_id + event_type + date)`. Store the hash in `sent_notifications` table. Check before sending.
2. Every SendGrid email call MUST include `X-SMTPAPI` header with Message-ID: `MessageID: "email_{user_id}_{event_type}_{timestamp}"`.
3. Test the negative path: after sending, calling the same handler again MUST not trigger another send.
4. Rate limits on outbound: max 5 SMS per user per hour, max 10 emails per user per day, max 1 notification per type per user per hour.

**Notification Types:**
- Household invite accepted → SMS to all household members (dedup: one per invite per day)
- Brand deal available → Email to all household members (dedup: one per deal per household per day)
- Item marked bought → SMS to list creator (if they have SMS enabled)
- Nudge: "did you get milk?" → SMS to household (dedup: once per list per day)

**Adversarial test required:** `test/adversarial/no-duplicate-sms.test.js` and `test/adversarial/no-duplicate-email.test.js` call a handler twice with identical input and assert only one SMS/email sent.

---

## Household Invite & Join Safety

Household invites can be abused to enumerate user IDs or send spam invitations.

**Rules:**
- Invite token is UUID v4, single-use, 7-day expiry.
- Inviter must be an existing household member, verified server-side.
- Rate limit: **10 household invites per user per day**. Enforced server-side.
- Accept/reject flow: `POST /api/household/invite/{token}/accept` and `POST /api/household/invite/{token}/reject`.
- Household membership is not exposed in any API response — members see only their own household.
- Household info (name, member count) is private to members only.

---

## Brand Deal & Financial Data Safety

Brand deals create revenue flows. Every deal transaction must be idempotent and audited.

**Rules:**
- Every brand deal completion includes a `swap_event` row with `idempotency_key = "swap_{user_id}_{deal_id}_{timestamp}"`.
- Check idempotency key before crediting tokens. If key exists, treat as duplicate — return success without sending duplicate credit.
- All financial mutations (token grant, household credit) happen in a single database transaction.
- Every financial mutation is logged to `audit_log` with user_id, amount, event_type, timestamp.

---

## Prompt Sanitization

User input reaches Claude in shopping list items, household notes, and feedback. Use `sanitizeForPrompt()` on every AI API call.

**Rules:**
- Strip injection attempts: "ignore previous instructions", "system:", XML/HTML tags.
- Truncate: list item 200 tokens max, household note 500 tokens max.
- Log sanitization events (user_id, timestamp, pattern — NOT full input).

---

## Rate Limiting

| Endpoint | Limit | Scope |
|----------|-------|-------|
| Rally sign-in exchange | 20 per 10 minutes | per IP |
| Household invite | 10 per day | per user |
| Notification (SMS) | 5 per hour | per user |
| Notification (email) | 10 per day | per user |
| API requests (authenticated) | 600 per minute | per session |

Enforce server-side only. Rate limit violations return 429 with `Retry-After` header.

---

## Incident Response

### P0: Cross-Household Data Leak
Any evidence that User A accessed User B's household data.

**Response:**
1. Page Andy immediately. Do not send bulk notification yet.
2. Identify scope: which households, which tables, which date range.
3. Revoke all session tokens for affected users.
4. Audit `audit_log` and Postgres query logs for anomalous access.
5. Notify affected users within 4 hours with specific details.

### P0: Duplicate SMS Blast
An outbound SMS handler fired multiple times, sending duplicate messages to many users.

**Response:**
1. Disable the handler immediately.
2. Audit the code path: where is the idempotency guard missing?
3. Count affected users and send a one-time apology SMS.
4. Fix the root cause: add or restore idempotency guard.
5. Test the negative path before re-enabling.

### P0: Secret Exposure (API Keys, DB Passwords)
Any API key or database credential committed, logged, or exposed in error.

**Response:**
1. Rotate the exposed secret immediately.
2. Audit access logs for the compromised credential.
3. Investigate what was accessed during the exposure window.
4. If user data was accessed, notify affected users.

### P1: High Email Bounce Rate
SendGrid domain reputation degraded (bounce > 5%, complaint > 0.1%).

**Response:**
1. Pause all outbound email.
2. Audit recent sends: inactive addresses, list corruption?
3. Investigate bounce patterns in SendGrid dashboard.
4. Fix, test with small batch, resume.

### P1: Phone Auth Flood
Attacker sending verify codes to a legitimate user's phone to discover if they have an account.

**Response:**
1. User reports receiving unexpected SMS codes.
2. Audit recent verify attempts: one attacker IP?
3. If one IP: block at rate limiter and notify user.
4. Advise user to use strong password on account.

---

## Audit Log

Every security-relevant write and admin action emits an `audit_log` row:
- session create / destroy
- household member add / remove / leave
- auth method add / remove
- household invite sent / accepted / rejected
- brand deal claimed
- financial transaction (token grant, distribution)
- account deletion
- admin action (any write by role=founder)

Audit logs are append-only. No UPDATE or DELETE on `audit_log`. Enforced via database constraints.
