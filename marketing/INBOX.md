# Groflo Marketing Site — Inbox

> New, unsorted items drop here with a date. Triage moves them to [BACKLOG.md](./BACKLOG.md) (`Backlog` for to-do, `Not In Scope` for explicitly deferred) — items leave the inbox only after they're filed.
>
> **Guardrail:** Never delete inbox items without founder approval. If something doesn't belong, file it — don't drop it.

## 2026-04-13 — Pre-launch money-path audit

Surfaced during the option-A campaign-signup refactor. Each item is a real risk to "we're going live"; none were fixed in that session because they were out of approved scope.

- **`feedback.mts` has no idempotency guard.** CLAUDE.md "Idempotency & Mutation Safety" requires every state-mutating handler to dedup. The current rate limit caps abuse at 10/hr per phone but does not prevent double-clicks from creating duplicate rows. Test `KNOWN GAP: duplicate POST currently creates two rows` in `netlify/functions/__tests__/feedback.test.ts:268` documents the gap and will need to flip when fixed. Fix is a 3-file change: Supabase migration adding `idempotency_key text unique` column to `feedback_requests`, function reading `Idempotency-Key` header (mirroring `campaign-lead.mts:189-205`), `FeedbackWidget.tsx` generating a stable key per draft.
- **Brand-portal hand-off is half-built.** `campaign-lead.mts` returns `redirectUrl: brands.groflo.io?from=campaign_signup&lead=<id>` but `brands.groflo.io` (in pennyai) does not yet read those query params, look up the lead in Supabase, or pre-fill a registration form. Without portal-side wiring, the user lands on a blank login page after submitting their brief. Coordinate with `pennyai/handlers_brand_auth.go:handleBrandRegister` — needs an unauthenticated lookup endpoint or a draft-pickup flow.
- **Reporting copy promises the brand portal must keep.** Marketing site claims real-time analytics that the external dashboard delivers. Validate each against current `brands.groflo.io` capability before launch:
  - `src/pages/ForBrands.tsx:77` — "Track every swap, acceptance rate, and repeat purchase from your dashboard"
  - `src/pages/ForBrands.tsx:203-204` — "Real-time swap analytics … swap acceptance rates, repeat purchases, and cost per acquired customer"
  - `src/pages/Pricing.tsx:28` — "you can pause, resume, or stop your campaign at any time from your dashboard"
  - `src/pages/Pricing.tsx:40` — "Track impressions, clicks, list adds, and budget spend in real time from your campaign dashboard"
  - `src/pages/Pricing.tsx:86` — "Track repeat purchases after the free trial"
- **Fake waitlist form on `ForBrands.tsx:346-349`.** Same lying-form pattern just removed from `CampaignSignup.tsx`. `onSubmit` clears the input, swaps the placeholder to "Thanks!" and POSTs nothing. Either wire it to `campaign-lead.mts` (with `source: 'brands_waitlist'`) or replace with a `mailto:` / Calendly link. Brand integrity test: a waitlist that never reaches a human is worse than a phone number.
- **Brand testimonials on `ForBrands.tsx:361, :370, :379`.** "Sarah K. — VP Marketing, Emerging Snack Brand", "Marcus T. — Brand Manager, Organic Dairy", "Priya M. — Head of Growth, Plant-Based Foods" appear to be illustrative composites. Either obtain real-partner consent + name them, or add a small disclosure ("Composite based on early partner conversations").
- **Netlify env vars must be set before `campaign-lead.mts` deploy.** `BRAND_PORTAL_URL`, `IP_HASH_SALT`, `SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY`. Missing any of them returns 500 to brand visitors. See `.env.example` and `DEPLOYMENT.md` § Required Environment Variables.
- **Supabase migration `002_campaign_leads.sql` must run before deploy.** New table `campaign_leads`, idempotency unique index, RLS enabled. Idempotent SQL (uses `create table if not exists`), safe to re-run.

## 2026-10-03 — Vocabulary conflict in marketing copy

- **"AI" phrasing violates the top-level Vocabulary table.** The footer tagline "The AI shopping list for families", `About.tsx` line 52 ("Penny, our AI assistant"), `Home.tsx` line 459 ("AI that learns your brands") and `Footer.tsx` line 13 conflict with `pennyai/CLAUDE.md`'s Vocabulary table, which forbids "AI", "assistant" and "bot" in user-facing copy. `Privacy.tsx` and `Terms.tsx` also say "AI assistant" but are Twilio-vetted legal text and should not be reworded without re-vetting. Resolving this is a copy change: needs CEO sign-off and a Customer Panel pass.
