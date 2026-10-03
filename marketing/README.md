# Groflo Marketing Site

Marketing site for **Groflo** (formerly Thriftly), a shared shopping list app where households text **Penny**. Groflo is a marketplace inside the Rally ecosystem.

> This directory is `marketing/` inside the pennyai monorepo (package name `groflo-site`) and is the deploy source for `groflo.io`. The standalone `/projects/thrift.ly` repo is an older copy that has drifted and does not ship.

## What's in here

| Path | Purpose | Deploys to |
|---|---|---|
| `src/` | React 19 + TypeScript + Vite SPA — pages, components, hooks, styles | `groflo.io` (Railway, `serve`) |
| `netlify/functions/` | Serverless functions (`feedback.mts`, `campaign-lead.mts`) | reference implementation; no runtime on Railway today (see [DEPLOYMENT.md](./DEPLOYMENT.md) § Open gap) |
| `supabase/migrations/` | SQL migrations for the Supabase tables the functions write to | Supabase project |
| `deck/` | Investor deck server — Node.js + SQLite, Twilio Verify SMS auth | not deployed; `groflo.io/deck` is the static export in `public/deck` |
| `archive/` | Historical panel reviews + marketing decisions | — |

## Working in this repo

| Topic | File |
|---|---|
| **Project rules + guardrails** | [CLAUDE.md](./CLAUDE.md) — read first; idempotency, brand, shipping rules |
| **Today's open work** | [INBOX.md](./INBOX.md) — new/unsorted items, dated |
| **Prioritized work + history** | [BACKLOG.md](./BACKLOG.md) — P0–P3, Bugs, Shipped, Not In Scope |
| **Deploy steps** | [DEPLOYMENT.md](./DEPLOYMENT.md) — branch flow, env vars, Railway |

## Running locally

```bash
npm install
npm run dev      # vite dev server at http://localhost:5173
npm run test     # vitest run — 1013 passing across 20 files as of 2026-10-03; must be green before commit
npm run lint     # eslint — must be clean before commit
npm run build    # tsc -b && vite build → dist/
```

## Architecture boundaries

- **Marketing site (this directory)** captures intent: leads, briefs, waitlists. It does **not** collect payment or run a brand dashboard. The fake Stripe form on `CampaignSignup.tsx` was removed on 2026-04-13 and `@stripe/*` is intentionally not a dependency. See [BACKLOG.md § Not In Scope](./BACKLOG.md#not-in-scope).
- **Consumer Penny app** (`app.groflo.io`) lives in `../frontend/` (same repo). Linked from CTAs via `import.meta.env.VITE_APP_URL`.
- **Brand portal** (`brands.groflo.io`) lives in `../frontend-brand/` (same repo). Linked from confirmation via `import.meta.env.VITE_BRAND_PORTAL_URL`. Owns Stripe Checkout, brand-account auth, campaign deposits, reporting dashboards.

## Brand source of truth

`../../pitch deck/penny-brand-source.json` (Groflo identity block). Brand color `#35644F`, Inter typeface, "Penny by Groflo" footer attribution.
