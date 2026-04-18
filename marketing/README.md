# Groflo Marketing Site

Canonical Railway-deployed marketing site for **Groflo** (formerly Thriftly), an AI shopping list app where users text **Penny**, the in-app assistant. Groflo is a marketplace inside the Rally ecosystem.

> Repo name: `groflo-site` (per `package.json`). Git URL: `andydownard/thrift.ly` (legacy, retained for backup chain integrity).

## What's in here

| Path | Purpose | Deploys to |
|---|---|---|
| `src/` | React 19 + TypeScript + Vite SPA — pages, components, hooks, styles | `groflo.io` (Railway service `marketing`) |
| `server.ts` | Node/Express server — serves static `dist/` + two API routes (`campaign-lead`, `feedback`) | same Railway container |
| `deck/` | Investor deck — Node.js + SQLite, Twilio Verify SMS auth | Railway (separate service) |
| `archive/` | Historical panel reviews + marketing decisions | — |

## Working in this repo

| Topic | File |
|---|---|
| **Project rules + guardrails** | [CLAUDE.md](./CLAUDE.md) — read first; idempotency, brand, shipping rules |
| **Today's open work** | [INBOX.md](./INBOX.md) — new/unsorted items, dated |
| **Prioritized work + history** | [BACKLOG.md](./BACKLOG.md) — P0–P3, Bugs, Shipped, Not In Scope |
| **Deploy steps** | [DEPLOYMENT.md](./DEPLOYMENT.md) — branch flow, env vars, Railway/Netlify |

## Running locally

```bash
npm install
npm run dev      # vite dev server at http://localhost:5173
npm run test     # vitest run — must be 1002 passing before commit
npm run lint     # eslint — must be clean before commit
npm run build    # tsc -b && vite build → dist/
```

## Architecture boundaries

- **Marketing site (this repo)** captures intent: leads, briefs, waitlists. It does **not** collect payment or run a brand dashboard. The fake Stripe form on `CampaignSignup.tsx` was removed on 2026-04-13 and `@stripe/*` is intentionally not a dependency. See [BACKLOG.md § Not In Scope](./BACKLOG.md#not-in-scope).
- **Consumer Penny app** (`app.groflo.io`) lives in `../pennyai/frontend/`. Linked from CTAs via `import.meta.env.VITE_APP_URL`.
- **Brand portal** (`brands.groflo.io`) lives in `../pennyai/static-brand/`. Linked from confirmation via `import.meta.env.VITE_BRAND_PORTAL_URL`. Owns Stripe Checkout, brand-account auth, campaign deposits, reporting dashboards.

## Brand source of truth

`../pitch deck/penny-brand-bible.json` v3.0 (Groflo identity block). Brand color `#35644F`, Inter typeface, "Penny by Groflo" footer attribution.
