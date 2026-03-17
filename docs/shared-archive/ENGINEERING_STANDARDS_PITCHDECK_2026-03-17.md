# Engineering Standards

> **Canonical location:** `~/pitch deck/ENGINEERING_STANDARDS.md`
> **Referenced by:** pennyai/CLAUDE.md, thrift.ly/CLAUDE.md, vibeflourishing/CLAUDE.md
> **Last updated:** 2026-03-09

---

## Idempotency & Mutation Safety

### Audit Checklist

Before shipping any handler that mutates state:

- [ ] **Idempotency key:** Does the handler use a dedup mechanism?
- [ ] **Rate limiting:** Is there server-side rate limiting?
- [ ] **Retry safety:** What happens if the client retries the same request?
- [ ] **Partial failure:** If step 2 of 3 fails, is step 1 rolled back?
- [ ] **Concurrent access:** What if two users hit this endpoint simultaneously?
- [ ] **Side effects:** Does this trigger SMS, email, or external API calls? Are they gated?

### Anti-Patterns (Learned from Production Bugs)

**B11 — Double SMS Send:** Never trigger outbound messages as a side effect of a refactor or migration. Gate behind explicit user action.

**B14 — NPS Re-prompt:** Any dismissible UI (NPS, modals, announcements) must persist dismissed state in localStorage AND server-side. Never rely solely on React state.

**B16 — Rate Limit Bypass:** Client-side bugs (useEffect loops, rapid polling) can bypass server-side guards. Always implement rate limits on the server, never trust the client.

**B17 — Stale Session:** Session tokens must be validated on every request, not cached. Expired sessions must return 401 immediately.

### Handler Audit Template

For every new endpoint that touches state:

```
Endpoint: [METHOD] [PATH]
Mutates: [table(s) affected]
Side effects: [SMS/email/webhook/external API]
Idempotency: [key/flag/none]
Rate limit: [limit per window]
Failure mode: [what happens on error]
Test coverage: [test file + test names]
```

---

## Database Standards

- **SQLite WAL mode** for all products
- **`CREATE TABLE IF NOT EXISTS`** for all tables
- **`ALTER TABLE ADD COLUMN`** for migrations — never drop and recreate tables
- **Backward-compatible migrations only** — old code must still work after migration runs
- **`sql.NullString`** for nullable fields (Go) or proper null handling (Node.js)
- **`COALESCE()`** in SQL queries for nullable string fields

---

## Security Standards

- **Session tokens:** 256-bit random, HttpOnly cookies, 30-day expiry
- **Rate limiting:** sync.Map (Go) or in-memory Map (Node.js) with sliding time windows
- **Input validation:** All user input validated before database or HTML rendering
- **Parameterized queries only** — never string concatenation for SQL
- **HSTS, X-Content-Type-Options, X-Frame-Options** on all responses
- **CORS:** Explicit origin allowlists, never `*` in production

---

## Testing Standards

### Ratios
- **Target:** 1.3x test-to-code ratio
- **Minimum:** 1.0x (PR blocked below this)
- **Aspirational:** 2.0x for critical paths (auth, payments, notifications)

### What Must Be Tested
- All HTTP endpoints (happy path + error paths)
- All database migrations (forward compatibility)
- All auth flows (each auth method independently)
- All notification paths (SMS, email, push) — including dedup
- All edge cases (empty input, long strings, special characters, SQL injection, XSS)
- Cross-user access (user A cannot see user B's data)

### Test Naming Convention
```
Test[Feature]_[Scenario]_[Expected]
e.g., TestListAdd_EmptyBody_Returns400
e.g., TestSMSLogin_RateLimit_Returns429After5Attempts
```

---

## Notification Safety (PennyAI-Specific)

This section is critical for PennyAI. See `pennyai/CLAUDE.md` "Notification & Communication Safety" for the full Hard Rule.

Summary:
1. No duplicate sends (idempotency key on every outbound message)
2. No inadvertent sends (gate behind explicit user action)
3. No re-prompting dismissed UI (persist in localStorage + server)
4. Rate limit all outbound channels (server-side)
5. Test the negative path (after dismiss → does NOT show again)
