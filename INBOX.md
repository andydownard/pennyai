# Inbox

Raw ideas, observations, and notes. Not yet triaged.

---

## 2026-04-18 — QA panel sweep (brand portal + consumer app)

Panels (Customer + Product Team) audited both frontends and backend contracts. Shipped fixes committed; open items triaged below.

**Shipped this pass:**
- Backend seed bug — `ends_at` text→TIMESTAMPTZ cast (`schema.go:1641`). Was silently failing "seeding bogo deal cheerios→Magic Spoon" on every cold start.
- Brand portal rebrand — `frontend-brand/CLAUDE.md` Thriftly→Groflo, `brands.thrift.ly`→`brands.groflo.io`.
- Brand portal tap targets — `smallBtn` padding 4px→8px to clear 44px WCAG target for Nadia/Tamika.
- Brand portal form validation — `min` attrs on discount / budget / price inputs on CreateDealForm.
- Brand portal logo upload error path — `reader.onerror` handler + ref reset so user can retry.
- Brand portal empty state — warmer copy that names Groflo shoppers (was: cold "No deals yet").
- Consumer vocab drift — "group" → "household" in leave-confirm dialog (`App.tsx:2502`, `ShareScreen.tsx:267`).

**New open items (promoted to BACKLOG):**
- **G28** — Swipe-delete needs undo (P1). `ItemRow.tsx` deletes on swipe >80px with no recovery path. Undo toast preferred over confirmation modal (one-handed pattern).
- **G29** — Swap accept button needs in-flight state (P1). `SwapCard.tsx` has no disabled/loading on submit; double-tap risks duplicate swap_events on lag.
- **G30** — Penny allergen capture in onboarding (P1). Priya persona fails: no dietary field in OnboardingOverlay, so Penny has nothing to remember. Panel convene needed.
- **G31** — Household "who claimed what" indicator (P2). James persona: no `claimed_by` badge on ItemRow; duplicate-prevention narrative weakened.
- **G32** — `fully_free` deal concurrent-budget race (P1). `handlers_parse.go:62–84` skips debit for fully_free path; two concurrent accepts can both pass. Needs atomic conditional UPDATE.
- **G33** — Deal detail error state (P2). Partial fetch failures leave "Loading performance data…" spinner forever instead of showing a recoverable error banner.
- **G34** — Deal save success toast (P2). Budget/discount edits silently refresh with no confirmation.

**Observed but no fix needed:**
- Brand green `#35644F` on white measures ~6.2:1 contrast — passes WCAG AA for normal text; audit claim about sub-AA was wrong.
- `TEXT_SECONDARY=#525252` in `styles.ts` is darker than brand-source's `#6B6B6B`. Darker is more readable for Nadia; brand source should update, not the code. Noted for next brand-source sync.
- `SmsConsentModal.tsx:16` "Thriftly" reference is historical context in a code comment explaining the pre-rebrand consent migration. Keep.

---

## 2026-04-16 — Session triage output

The max-effort PG/marketing audit pass from earlier today has been triaged and the resolved items moved to BACKLOG's Shipped section. This inbox is now empty of PG-migration items. Items still open — all already promoted to BACKLOG:

- **G22** — Marketing Home.tsx body copy + alt text sweep (P1)
- **G23** — Twilio webhook redelivery dedup (P1)
- **G24** — pgcompat.go unit test coverage (P2)
- **G25** — Visual regression suite (P2)
- **G26** — Brand consistency lint (P2)
- **G27** — Comprehensive idempotency audit (P2)

Audit findings kept for posterity (LIKE sweep and outbound-dedup audit):

### LIKE pattern audit — clean
7 remaining `LIKE` patterns in production Go are either (1) internal structured-key prefix matches (`LIKE 'segment:%'`), (2) already case-normalized with `LOWER()`, or (3) already migrated to `ILIKE`. No gaps.

### Outbound message dedup — 2 P1 gaps, rest OK
**Category A (guard present, 5 sites):** `handlers_digest.go:134` (daily SMS), `handlers_social.go:307` (invite-accepted notification), `handlers_brand_digest.go:164-170` (brand weekly email), `handlers_auth.go:380,449` (magic link, password reset).

**Category B (consent/rate-limit only, low risk):** `handlers_social.go:123` (SMS invite).

**P1 gap (→ BACKLOG G23):** `handlers_webhook.go:371,377,408,470,476,494` — inbound SMS webhook replies fire without Twilio message-ID dedup. Twilio retries on timeout → duplicate SMS. Store `twilio_message_id` with UNIQUE constraint before reply.

### pgExecWithReturning — safe, no action
All 3 tables flagged in earlier audit (`auth_methods`, `savings_milestones`, `swap_propagations`) have `id SERIAL PRIMARY KEY`, and the fallback-to-Exec branch covers the edge case where `RETURNING id` fails. No active breakage. A stricter implementation that accepts a column name parameter is a low-priority refactor, not a bug.

### Branching gap
CLAUDE.md mandates `main → staging → production` with merges only. Neither `staging` nor `production` branch exists locally or on `origin`. Recommend creating them from current green main and pinning Railway's staging service to `staging`, production to `production`. Out-of-scope for this session — ops/CI work.

---
