<!-- AGENT INSTRUCTIONS — read and obey before writing any code. -->
# Groflo Marketing Site (marketing/ — from thrift.ly repo)

> **Global engineering contract:** https://github.com/rallytasks/agents/blob/main/AGENTS.md
> Every rule there applies. Parent project: [../AGENTS.md](../AGENTS.md). This file mirrors the standalone `thrift.ly` repo; keep the two in sync.
@../../agents/AGENTS.md   <!-- local-checkout convenience; no-op in a standalone clone -->

Canonical marketing site for **Groflo** (formerly Thriftly), the AI shopping-list app where users text **Penny**. Marketplace in the Rally ecosystem (auth + Stripe via timpager). Also holds the investor deck + timpager slide generator as sibling sub-apps.

## Architecture
- **Marketing site** (`src/`): React 19 + TS + Vite SPA, React Router 7, Stripe Elements, vanilla CSS (no Tailwind). Deploys to **Railway** service `marketing` from `production`; the container runs `server.ts` (Node/Express) serving `dist/` plus two API routes (`/.netlify/functions/campaign-lead`, `/.netlify/functions/feedback` — legacy paths preserved); persistence is Postgres (`Postgres-mpxU`). CTAs read `import.meta.env.VITE_APP_URL` (fallback `https://app.groflo.io`).
- **Deck** (`deck/`) and **Timpager** (`timpager/`): as in the thrift.ly repo.
- **Brand source of truth:** `../pitch deck/penny-brand-bible.json` v3.0; brand color `#35644F`, Inter.

## Project rules & overrides
- **Decision authority:** CEO (Andy) final on copy/positioning; PMM recommends; Brand PM = brand-bible consistency; CMO = alignment. Copy changes need CEO approval (minor punctuation/word swaps may bypass with PMM + Brand PM).
- **Privacy + Terms are legal-canonical (Twilio A2P 10DLC).** `src/pages/Privacy.tsx` (12 sections) + `Terms.tsx` (13 sections); pennyai's `handlers_admin.go` mirrors them — edit here first, then sync. MUST render: "Message frequency varies"; "Message and data rates may apply"; STOP/HELP with reply instructions; "We will not share your mobile information with third parties or affiliates for marketing or promotional purposes" (verbatim); the frequency-control promise. Tests enforce these — don't weaken; numbered h2s asserted by role+name.
- **Language standard:** no jargon/buzzwords; no "AI", "orchestration", "coordinate" in headlines. (`STYLE`.)
- **Hero (locked):** "No more 'Did you get milk?'" — ONE hero → THREE pillars → social proof → referral hook → trust. No feature-list hero.
- **Shipping gates:** 5-second "what does Penny do?"; copy ties to household coordination; referral not below fold / behind sign-up; no vaporware (don't market a feature before it ships in the app).
- **Brand checklist:** "Groflo" in user text (the two "Thriftly" mentions are negative test assertions — leave them); `*@groflo.io`; brand green `#35644F`; Inter; footer "Penny by Groflo"; CTAs read `VITE_APP_URL`.
- **deck/ specifics:** idempotency on feedback/session/audit/slide-view; CORS allowlist `groflo.io` + `thrift.ly` + `localhost` (keep `thrift.ly` until Phase 2 cutover + 301s live 30+ days); prepared statements.

## Pattern detection (project-specific)
Color/font drift vs the brand bible; brand-name drift ("Thriftly"/"thrift.ly" in user text); hardcoded `app.groflo.io`; `*@thrift.ly` emails; CORS allowlist missing a transition host; site copy ahead of shipped app features; missing CTIA language.

## Doc index
| Topic | File |
|---|---|
| Parent project contract | [../AGENTS.md](../AGENTS.md) |
| Standalone twin repo | github.com/andydownard/thrift.ly |
| Work tracking | [INBOX.md](./INBOX.md), [BACKLOG.md](./BACKLOG.md) |
