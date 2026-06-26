<!-- AGENT INSTRUCTIONS — read and obey before writing any code. -->
# Groflo Brand Portal (frontend-brand)

> **Global engineering contract:** https://github.com/rallytasks/agents/blob/main/AGENTS.md
> Every rule there applies to this repo. Parent project: [../AGENTS.md](../AGENTS.md). This file adds only what is specific to the brand portal.
@../../agents/AGENTS.md   <!-- local-checkout convenience; no-op in a standalone clone -->

The B2B dashboard at `brands.groflo.io` where CPG brand managers create deals, view competitive intel, and fund accounts.

## Architecture
- **Frontend:** React/TypeScript SPA (`App.tsx` + components), Vite build → `../static-brand/` (served by Go at the `brands.groflo.io` subdomain). Dev `http://localhost:5174`, proxies `/brand/*` → Go `:8080`.
- **Backend:** same Go server as the consumer app, routes prefixed `/brand/`.
- **Auth:** email/password with `brand_users` table (separate from consumer `users`).

## Consumer app vs brand portal (two separate apps)
| | Consumer (`frontend/`) | Brand portal (`frontend-brand/`) |
|---|---|---|
| User | Shoppers | CPG brand managers |
| Design | Mobile-first, warm | Desktop-first, professional (think Stripe/Linear) |
| Auth | Phone/SMS, Google, email | Email/password only |
| Tables | `users`, `sessions` | `brand_users`, `brand_sessions` |
| Build dir | `dist/` | `static-brand/` |

**Shared:** Go backend/binary/DB, design principles, TS conventions (inline styles, no CSS framework), API patterns (`credentials:'include'`, JSON, error shapes). **NOT shared:** React components, design tokens, auth system, build pipeline.

## Design tokens
`BRAND='#35644F'` · `BRAND_LIGHT='#EEF7F2'` · `TEXT='#1A1A1A'` · `TEXT_SEC='#666'` · `BORDER='#E0E0E0'` · `BG='#FAFAFA'` · `ERROR='#D32F2F'` · `FONT="'Inter', -apple-system, …"`. Information-dense, desktop-primary.

## Screens & API
Screens: landing, login, register, dashboard, deals, create-deal, intel, transactions, deposit. Routes (all `/brand/`): `register`, `login`, `logout`, `me`, `dashboard`, `deals` (GET/POST), `deals/:id` (PUT/DELETE), `category-intel/:category`, `transactions`, `deposit`.

## Project rules & overrides
- Inline styles (no CSS-in-JS); functional components + hooks; `api<T>()` typed fetch; `UNAUTHORIZED` → redirect to login. `formatCents()` for all money display.
- Keep `App.tsx` as router/layout shell; extract sub-components >~100 lines; split files when `App.tsx` exceeds ~1000 lines.
- **Do NOT:** import from `../frontend/src/`; use the consumer app's design tokens; add consumer features here; share React components; bolt brand screens onto the consumer `App.tsx`.

## Doc index
| Topic | File |
|---|---|
| Parent project contract | [../AGENTS.md](../AGENTS.md) |
| Architecture / DB | [../ARCHITECTURE.md](../ARCHITECTURE.md) |
