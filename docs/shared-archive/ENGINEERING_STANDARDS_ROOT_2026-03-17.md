# Engineering Standards — Mutation Safety & Idempotency

> **Shared across all projects.** Referenced from `~/CLAUDE.md`. This is a living document — update it when new patterns are discovered.

---

## Idempotency Audit Checklist

**Run this audit on every state-mutating handler before shipping.** A "state-mutating handler" is any endpoint or function that performs INSERT, UPDATE, DELETE, or triggers an external side effect (SMS, email, payment, webhook).

### For Each Handler, Verify:

| # | Check | Pass? | Notes |
|---|-------|-------|-------|
| 1 | **Duplicate call safety** — calling this handler twice with the same input produces the same result, not a duplicate record/action | | |
| 2 | **Transaction boundary** — if the handler does 2+ mutations, they're wrapped in a single transaction (all succeed or all fail) | | |
| 3 | **Race condition safety** — concurrent calls from the same user can't create inconsistent state (use `UPDATE ... WHERE` atomic checks, not SELECT-then-UPDATE) | | |
| 4 | **Rate limiting** — server-side rate limit exists and cannot be bypassed by client-side bugs (useEffect loops, retry storms, rapid polling) | | |
| 5 | **Negative-path test exists** — there is a test that calls the handler twice and asserts no duplicate was created | | |
| 6 | **Error state is clean** — if the handler fails mid-execution, no partial state is left behind (orphan records, half-sent messages) | | |
| 7 | **External side effects are gated** — SMS, email, payment charges, webhook calls happen AFTER the database transaction commits, not inside it | | |

### Common Idempotency Patterns

**Pattern 1: Unique constraint + INSERT OR IGNORE**
```sql
-- Good: duplicate inserts are silently ignored
INSERT OR IGNORE INTO swap_events (user_id, deal_id, item_id, action)
VALUES (?, ?, ?, 'accepted');
```

**Pattern 2: Atomic conditional UPDATE**
```sql
-- Good: budget check and deduction are one atomic operation
UPDATE brand_deals
SET budget_spent_cents = budget_spent_cents + ?
WHERE id = ? AND budget_spent_cents + ? <= daily_budget_cents;
-- Check rows affected — if 0, budget was exhausted
```

**Pattern 3: Idempotency key**
```go
// Good: client sends a unique key, server deduplicates
key := r.Header.Get("Idempotency-Key")
if key != "" {
    existing := db.QueryRow("SELECT result FROM idempotency_cache WHERE key = ?", key)
    if existing != nil {
        return existing // Return cached result, don't re-execute
    }
}
```

**Pattern 4: Time-window dedup**
```sql
-- Good: prevent repeated actions within a time window
SELECT COUNT(*) FROM swap_events
WHERE user_id = ? AND deal_id = ? AND action = 'accepted'
  AND created_at > datetime('now', '-1 hour');
-- If count > 0, reject as duplicate
```

---

## Anti-Patterns (From Real Bugs)

These are patterns that caused real bugs across Andy's projects. Learn from them.

### B11: Budget Race Condition (PennyAI)

**Bug:** Two concurrent swap accepts could both pass the budget check and both deduct, overspending the budget.

**Root cause:** SELECT to check budget, then separate UPDATE to deduct. Between the SELECT and UPDATE, another request could also SELECT and see available budget.

**Fix:** Atomic conditional UPDATE: `UPDATE ... WHERE budget_spent_cents + ? <= daily_budget_cents`. Check `RowsAffected()` — if 0, budget was exhausted.

**Lesson:** Never check-then-act. Always act-and-check-result.

### B14: Rebate Farming (PennyAI)

**Bug:** Same user could accept the same deal multiple times, earning multiple rebates.

**Root cause:** No dedup check on the accept handler.

**Fix:** Time-window dedup query before insert. Return 409 Conflict on duplicates.

**Lesson:** Every INSERT needs a "does this already exist?" check. Preferably via UNIQUE constraint, not application logic.

### B16: NPS Re-Prompting (PennyAI)

**Bug:** NPS survey showed on every page reload because dismissed state was only in React state.

**Root cause:** `useEffect` called the check endpoint on every screen change. Dismissal used `useState` which resets on remount.

**Fix:** `useRef` gate for at-most-once check per mount + `sessionStorage` flag on dismiss/submit.

**Lesson:** Dismissal state for any UI prompt must survive page reload. Never rely solely on React state for "has the user seen this." Use sessionStorage/localStorage or a server-side flag.

### B17: Per-Campaign Duplicate Accepts (PennyAI)

**Bug:** Same user could accept the same deal across different items, earning multiple rebates from one campaign.

**Root cause:** B14's dedup only checked same deal+item within 1 hour. Didn't check per-user-per-campaign lifetime.

**Fix:** Added lifetime per-user-per-campaign check on both offer and accept paths.

**Lesson:** Dedup checks need to match the business rule, not just the technical constraint. Think about what "duplicate" means from the business perspective.

---

## Handler Audit Template

When auditing a codebase for mutation safety, use this template:

```
## [Project Name] Handler Audit — [Date]

### Endpoints That Mutate State

| Endpoint | Method | Mutations | Idempotent? | Transaction? | Rate Limited? | Negative Test? |
|----------|--------|-----------|-------------|-------------|---------------|----------------|
| /api/list | POST | INSERT list_items | ? | ? | ? | ? |
| /api/list/{id}/toggle | PATCH | UPDATE list_items | ? | ? | ? | ? |
| /auth/send-code | POST | INSERT + Twilio SMS | ? | ? | ? | ? |
| ... | ... | ... | ... | ... | ... | ... |

### External Side Effects

| Endpoint | Side Effect | Gated? | Deduped? | Reversible? |
|----------|-------------|--------|----------|-------------|
| /auth/send-code | Twilio SMS | ? | ? | No |
| /auth/email/forgot | SendGrid email | ? | ? | No |
| /api/nps | None | N/A | N/A | N/A |
| ... | ... | ... | ... | ... |

### Gaps Found

1. [Endpoint] — [Description of gap] — [Fix]
2. ...
```

---

## When to Run This Audit

- **Before shipping any new handler that mutates state**
- **After fixing any bug in the "Anti-Patterns" section above** — check if the same pattern exists elsewhere
- **At the start of each new phase/milestone** — quick scan of all mutation handlers added since last audit
- **When adding a new external integration** (payment provider, SMS, email) — audit all handlers that touch it
