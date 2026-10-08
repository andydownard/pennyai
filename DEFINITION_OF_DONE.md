# Definition of Done — Groflo (Penny)

A feature is DONE when it passes both tiers. Tier 1 (GATE) blocks every commit. Tier 2 (QUALITY) blocks PR merge but individual items can be follow-ups per panel discretion.

## Tier 1: GATE (Blocks Every Commit)

All must pass before code reaches main.

- [ ] **Go tests pass.** `go test -count=1 ./...` exits 0 (SQLite tier), and `scripts/test-postgres.sh` exits 0 (PostgreSQL tier). All new/modified tests pass. No skipped tests.
- [ ] **Frontend tests pass.** `cd frontend && npx playwright test` exits 0. All new/modified tests pass.
- [ ] **No secrets in code.** Grep for API keys, tokens, phone numbers. Check .env, logs, comments, URLs. Secrets only in env vars.
- [ ] **No DROP TABLE or table recreation.** Use ALTER TABLE ADD COLUMN only. All migrations backward-compatible.
- [ ] **No breaking API changes.** Endpoint paths, HTTP methods, JSON request/response shapes unchanged. If shape changes, add backward-compatibility test.
- [ ] **Feature branch only.** Code on feature branch, never pushed to main/staging/production directly.
- [ ] **Idempotency guards in place.** Every mutation handler (INSERT/UPDATE list, record item, process shopping trip) has dedup logic. Calling twice with same input = same result, no duplicates. Check before INSERT: "have we already created this record?"
- [ ] **No duplicate notification paths.** Trace every code path that sends email (SendGrid), or push notification. Verify: dedup guard OR idempotency key OR "already sent" flag. If you cannot prove no duplication, add a guard.
- [ ] **Data isolation verified.** Every query filters by current user or household_id. Query audit: spot-check 5+ queries to confirm no cross-user leaks. Write a specific test: "User A cannot see User B's lists or household."
- [ ] **Rate limits enforced server-side.** Rate limiting via `sync.Map` with sliding time windows. Limits non-bypassable from client. Trace: list creation, item addition, SMS submissions.
- [ ] **Security rules not violated.** Read [SECURITY.md](./SECURITY.md). Confirm: no phone/email exposure across user boundary, no PII in logs (IDs only), Clerk sign-in intact and no legacy sign-in or Timpager call reintroduced.
- [ ] **Penny personality intact.** Typewriter effect, chime-ins, idle reminders, personality quotes still present and functional. No refactoring removed them.

## Tier 2: QUALITY (Blocks PR Merge)

All must pass before merge. Individual items can be follow-ups if panel approves.

- [ ] **Test ratio >= 1.3x.** Lines of test code ≥ 1.3 × lines of new source code. `go test -count=1 ./...`, `scripts/test-postgres.sh` and `cd frontend && npx playwright test` all pass.
- [ ] **Every mutation has 4 test types:** (1) Positive path (happy case succeeds). (2) Negative path (validation fails or no auth). (3) Idempotency (call twice = once, no duplicates). (4) Cross-user prevention (other user cannot access/modify).
- [ ] **No endpoint removed or renamed.** Read [CLAUDE.md](./CLAUDE.md) §Refactoring Constraints. Do NOT remove, rename, or change HTTP method of any endpoint. Do NOT change JSON request/response shapes without backward-compatibility test.
- [ ] **No database table dropped or recreated.** Use ALTER TABLE ADD COLUMN. Migrations stay inline in `initDB()`, numbered sequentially.
- [ ] **No Penny personality behaviors removed.** Typewriter effect, chime-ins, idle reminders, personality quotes must still function. Check [PENNY_PROMPT.md](./PENNY_PROMPT.md) for full list.
- [ ] **Brand logo caching system preserved.** 3-layer system (`brand_logos`, `brands`, `brand_owners`) still intact. Do NOT simplify away.
- [ ] **Household sharing intact.** Invite, join, leave, members list all functional. No household removal, no shortcuts to skip shared context.
- [ ] **Email inbound webhook working.** Email parsing still functional. (There is no SMS; the texting paths answer 410.)
- [ ] **Panel review done.** See [PANEL_GOVERNANCE.md](./PANEL_GOVERNANCE.md). Minimum for user-facing changes: Customer Panel or Product Team sign-off. Micro auto-go. Small = 2 min, Medium = 10 min, Large = 30 min.
- [ ] **CST checklist passed.** Review [CLAUDE.md](./CLAUDE.md) §CST Principles. Human Dignity: respects dietary needs without judgment. Common Good: works on shared devices, low bandwidth. Subsidiarity: strengthens household coordination. Solidarity: bridges dietary/cultural gaps. Preferential Option: if Tamika (single mom, one hand free, toddler) can use it, anyone can.
- [ ] **Shipping Gates verified.** Gate 1 (first-5-minutes magic): new user adds first item in 90 sec, zero questions. Gate 2 (household coordination): feature serves "Did you get milk?" narrative. Gate 3 (data flywheel): feature generates data signal for Penny. If not serving all three, deprioritize unless Andy approves. See [11_STAR_NARRATIVE.md](./11_STAR_NARRATIVE.md) and [DATA_FLYWHEEL.md](./DATA_FLYWHEEL.md).
- [ ] **Regression list checked.** Review [CLAUDE.md](./CLAUDE.md) §Regressions. If your change touches a listed pattern, read existing code first.
- [ ] **Docs updated.** PRs touching `handlers_*.go`, `App.tsx`, or database schema MUST update [DESIGN_SPEC.md](./DESIGN_SPEC.md), [ARCHITECTURE.md](./ARCHITECTURE.md), or [PANEL_GOVERNANCE.md](./PANEL_GOVERNANCE.md). Behavior-changing PRs update affected markdown.
- [ ] **ROADMAP.md updated.** Shipped features deleted from [ROADMAP.md](./ROADMAP.md). Deprioritized items moved to "Nice-to-Have" or "Not In Scope", not left hanging.

## Mutation Safety Checklist

Every handler that mutates state (INSERT/UPDATE list, record item, send SMS, process shopping trip) must have:

1. **Idempotency guard.** Check before INSERT: has this user already created this list with same timestamp? Has this SMS already been sent? Use atomic conditional UPDATE (`UPDATE ... WHERE user_id = ? AND created_at = ? AND status = 'pending'`), not check-then-act. Store dedup key or flag.
2. **Transaction boundary.** Multi-step mutations (create list → add item → notify household) in single transaction. Rollback if any step fails.
3. **Server-side rate limit (non-bypassable).** List creation, item addition, SMS submission all rate-limited. Never rely on client validation alone.
4. **Negative-path test.** Write a test that calls the handler twice with identical input. Verify: (a) first call succeeds, (b) second call returns same result, (c) no duplicate DB rows, (d) no duplicate SMS/email sends.

## Failure Mode Registry

For each item below, write a test or runbook.

| Mode | Symptom | Mitigation | Test |
|------|---------|-----------|------|
| Duplicate SMS sent | User receives "Did you get milk?" three times. Customer support escalation. | Idempotency key on every Twilio call. Check "already sent" flag before send. Retry with exponential backoff. | Mock Twilio 500; verify no duplicate SMS on retry. Call handler twice → only one SMS. |
| Twilio down (network timeout) | SMS queue backs up. Household notifications delayed/lost. | Retry with exponential backoff (3 attempts). Queue to async job. Alert ops. Never lose SMS data. | Mock Twilio timeout; verify retry loop, async queue, no data loss. |
| Claude API down | Penny cannot infer item details (quantity, brand). New-user experience breaks. | Fall back to empty defaults. Trigger re-inference later. Never block list creation. | Mock Claude 500; verify list created, Penny suggests defaults, re-inference queued. |
| Cross-user data leak | User A sees User B's lists, shopping trips, household members. | Every SELECT filters by user_id or household_id. Negative test: "Other user 403 on GET /lists/other_id." Query audit before merge. | Write: `GET /api/lists/{OTHER_USER_LIST_ID}` → 403 Forbidden. |
| Duplicate notification on refactor | Code refactor accidentally triggers SMS twice (e.g., added logging that calls send twice). | Before merge: trace every code path that touches SMS, email, push. If you cannot prove no duplication, add a guard flag. Test: refactor → trigger notification → verify only one sent. | Grep code for all `SendGridManager.Send()` and `Twilio.SMS()` calls. Verify each in separate test: calling twice = once. |
| Postgres data loss or corruption | Database loses data or becomes inaccessible on failure. App goes down. | Railway-managed PostgreSQL with automated backups and point-in-time recovery. Migrations additive only (ALTER TABLE ADD COLUMN). Recovery runbook: restore from Railway backup. Monitoring: `db_ok` on each health check. | Test: verify `/health` reports `db_ok`, verify migrations are additive, verify the restore runbook. |

