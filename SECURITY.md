# Groflo — Security

## Secret Management

All secrets in environment variables. No exceptions. No `.env` committed.

| Secret | Env var |
|--------|---------|
| Postgres connection | `DATABASE_URL` |
| Claude API | `ANTHROPIC_API_KEY` |
| Twilio SID + token + verify | `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_VERIFY_SID` |
| Twilio phone number | `TWILIO_PHONE_NUMBER` |
| Twilio messaging service | `TWILIO_MESSAGING_SERVICE_SID` |
| Google OAuth client | `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` |
| Stripe secret key + webhook | `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET` |
| SendGrid API key | `SENDGRID_API_KEY` |
| Litestream credentials | `LITESTREAM_ACCESS_KEY_ID`, `LITESTREAM_SECRET_ACCESS_KEY` |
| Sentry DSN | `SENTRY_DSN` |
| PostHog API key | `POSTHOG_API_KEY` |
| LogoDev credentials | `LOGODEV_CLIENT_ID`, `LOGODEV_CLIENT_SECRET` |
| Google Places API key | `GOOGLE_PLACES_API_KEY` |
| Admin API key | `ADMIN_API_KEY` |

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

## Multi-Auth Complexity

Groflo supports four auth methods: **phone + SMS** (Twilio Verify), **Google OAuth**, **email + password** (bcrypt), **magic link** (single-use, 15-min expiry). No single method can be weakened or removed.

### Phone + SMS (Twilio Verify)
- Rate limit: **5 verify attempts per phone per hour**. Enforced server-side, not client.
- Valid phone numbers only (E.164 format, country code required).
- Retry tokens expire after 24 hours.
- Test with real Twilio account before launch.

### Google OAuth
- Scope: `openid email profile` only. NEVER add `drive`, `calendar`, or `contacts` scopes.
- `google_sub` is the stable user ID — never changes, always used as join key.
- New OAuth users are created in `users` table on first successful callback.
- Workspace domains (company email) are allowed. No disposable email domains (mailinator, etc.).

### Email + Password
- Passwords hashed with bcrypt (min cost 12). Never store plaintext.
- Minimum 8 characters, encouraged to be longer.
- Password reset via magic link (see below).

### Magic Link (Email)
- Single-use, 15-minute expiry.
- Token is a UUID v4, stored as `sha256(token)` in database.
- After use, token is deleted. No replay.
- Rate limit: **3 magic link requests per email per 10 minutes**.

**Session Management:**
- 256-bit random session tokens (`crypto.randomBytes(32).toString('hex')`).
- Stored only as `sha256(token)` in `sessions.token_hash`.
- HttpOnly cookies. SameSite=Lax.
- Sessions expire 30 days after creation, sliding window on `last_seen_at`.
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
| Auth (SMS verify) | 5 per hour | per phone |
| Auth (magic link) | 3 per 10 minutes | per email |
| Auth (OAuth callback) | 3 per minute | per IP |
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
