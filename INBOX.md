# Inbox

Raw ideas, observations, and notes. Not yet triaged.

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
