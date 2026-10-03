# Groflo Marketing Site — Deployment Guide

Step-by-step deployment flow for the Groflo marketing site and investor deck. Follow this for every feature, bug fix, or infrastructure change.

**Related:** This directory is `marketing/` inside the pennyai monorepo. The Go app (`app.groflo.io`), `frontend/` and `frontend-brand/` share this repo; the top-level `DEPLOYMENT.md` covers them.

---

## Branch Strategy

| Branch | Purpose | Deploys To |
|--------|---------|------------|
| `feat/*` | New features or bug fixes | — (local only) |
| `main` | Everything lands here through a PR; CI must be green | — (`main` itself does not deploy) |
| `staging` | Fast-forward from `main` | Marketing staging Railway environment (`marketing-staging-66a2.up.railway.app`) |
| `production` | Fast-forward from `staging` after staging is verified | Production (`groflo.io`) |

**NEVER promote staging→production without groflo.io green on staging.** Verify healthcheck passes on staging before any production merge.

There is no `dev` branch.

---

## Deployment Flow

### 1. Create a Feature Branch

```bash
git checkout main
git pull origin main
git checkout -b feat/your-feature-name
```

### 2. Develop and Test Locally

```bash
# Type check + build
npm run build

# Unit tests
npm run test
```

### 3. Commit and Push

```bash
git add <files>
git commit -m "feat: description of change"
git push origin feat/your-feature-name
```

### 4. Open a PR into `main`

```bash
gh pr create --base main --head feat/your-feature-name --title "Short description" --body "## Summary
- Change 1
- Change 2

## Test plan
- [ ] Verification step 1
- [ ] Verification step 2"
```

CI must be green before merge. Merging to `main` deploys nothing.

### 5. Merge the PR

```bash
gh pr merge <PR_NUMBER> --merge
```

### 6. Fast-Forward `main` → `staging`

```bash
git checkout staging
git pull origin staging
git merge --ff-only main
git push origin staging
```

This deploys the marketing staging Railway environment.

### 7. Verify Staging, Then Fast-Forward `staging` → `production`

```bash
curl -sI https://marketing-staging-66a2.up.railway.app | head -5
# Expected: HTTP/2 200
```

Only once staging is green:

```bash
git checkout production
git pull origin production
git merge --ff-only staging
git push origin production
```

Railway deploys the `production` branch; pushing `main` deploys nothing.

### 8. Verify Production

```bash
curl -sI https://groflo.io | head -5
# Expected: HTTP/2 200
```

Expect `x-railway-*` headers and HTTP 200. An HTTP 404 with `x-railway-fallback: true` means no Railway service is bound to the domain — see **Blockers** below.

---

## Architecture

- **Marketing SPA:** React 19 + TypeScript 5.9 + Vite 7.3
- **Investor Deck:** static export at `groflo.io/deck` from `public/deck`. The Node.js + SQLite server in `deck/` is not deployed.
- **Platform:** Railway, `serve` static package. Netlify is not used; `netlify.toml` and `public/_headers` / `_redirects` are inert.
- **Production URL:** `https://groflo.io`
- **App CTAs point to:** `https://app.groflo.io` (PennyAI)

---

## Build

```bash
npm run build    # tsc -b && vite build → outputs to dist/
```

Railway serves with: `npx serve dist -l $PORT`

---

## Railway Config

Current `railway.toml`:

```toml
[build]
watchPatterns = ["src/**", "public/**", "index.html", "package.json", "vite.config.ts", "tsconfig*.json"]

[deploy]
startCommand = "npx serve dist -l $PORT"
healthcheckPath = "/"
restartPolicyType = "ON_FAILURE"
```

**Routing:** without `-s`, `serve` does not fall back to `index.html`. `public/serve.json` enumerates every SPA route (`/for-brands`, `/pricing`, `/campaign/signup`, `/about`, `/privacy`, `/terms`) plus `/deck` → `/deck/index.html` and the `noindex` header for `deck/**`. A new page requires a new `serve.json` rewrite or it will 404 in production. Known trade-off: unknown URLs get `serve`'s plain 404 instead of the branded React 404 page.

`package.json`'s `start` script is `serve dist -l $PORT`, matching `railway.toml` so local `npm start` reproduces production routing. `railway.toml`'s `startCommand` wins on Railway regardless; keep the two in step.

---

## Required Environment Variables

```
VITE_APP_URL=https://app.groflo.io
VITE_BRAND_PORTAL_URL=https://brands.groflo.io
BRAND_PORTAL_URL=https://brands.groflo.io
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_SERVICE_ROLE_KEY=...
IP_HASH_SALT=<random secret>
```

`VITE_APP_URL` controls where "Try Free" CTAs link. `VITE_BRAND_PORTAL_URL` controls where the campaign-signup confirmation hands off. Both are baked in at build time by Vite (defaults: `app.groflo.io` / `brands.groflo.io`).

**Open gap:** `CampaignSignup.tsx` (`LEAD_ENDPOINT`) and `FeedbackWidget.tsx` (`API_BASE`) POST to `/.netlify/functions/campaign-lead` and `/.netlify/functions/feedback`. `serve` is a static file server; those paths have no runtime on Railway and return 404, so the brand campaign form and the feedback widget do not work in production until the functions get a host. `netlify/functions/*.mts` and their 77 tests remain the reference implementation. `BRAND_PORTAL_URL`, `IP_HASH_SALT`, `SUPABASE_URL` and `SUPABASE_SERVICE_ROLE_KEY` are only needed wherever those functions eventually run.

**Decision needed (Andy):** host them as Go endpoints on the app, as Supabase Edge Functions, or via a small Node server (an unmerged Express port exists on branch `feat/marketing-railway-native` / `feat-clerk-phase2b` but is superseded and not a merge candidate).

Stripe is **not** used on the marketing site. Brand campaign signups submit a brief to Supabase via the `campaign-lead` Netlify function and then hand off to the brand portal (`brands.groflo.io`), which handles account creation, deposits, and Stripe payments. The `@stripe/*` npm packages are intentionally not in this repo's `package.json`.

---

## Investor Deck Server (`deck/`)

**Not currently deployed.** The live `groflo.io/deck` is the static export in `public/deck` (PRs #113, #114). The config below is kept as reference in case the Node + SQLite server is revived.

Separate Railway service with its own Dockerfile:

```
Node 20-alpine → npm install --production → node server.js
```

- **Port:** 3000
- **Health:** `GET /`
- **Auth:** Twilio Verify SMS
- **Required env vars:** `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `GH_PAT`, `ADMIN_API_KEY`, `FEEDBACK_CODE`, `FEEDBACK_PHONES`

### Deck Railway Config

```toml
[build]
builder = "DOCKERFILE"
dockerfilePath = "Dockerfile"
watchPatterns = ["deck/**"]

[deploy]
healthcheckPath = "/"
restartPolicyType = "ON_FAILURE"
```

---

## Checklist Before Merging to `main`

- [ ] `npm run build` succeeds
- [ ] `npm run test` passes
- [ ] No secrets in committed code
- [ ] CTAs point to correct app URL
- [ ] Brand consistency: colors (#35644F), fonts (Inter), logos match Groflo branding
- [ ] `public/serve.json` updated if a route was added
- [ ] Staging URL verified before the production promote
- [ ] PR reviewed and approved

---

## Blockers

BLOCKER: `groflo.rallytasks.com` is not wired up, and cannot be until Andy confirms where the Groflo services live.

Checked 2026-10-03, read-only. The Railway account connected to this toolchain (andy@thriftly.com) can see two projects: `rally` (`320a5e7d-3087-4a2d-b6d9-e48d0e1349db`) and `thriftly.com` (`2829bed0-bd12-4424-9e16-60cc8b597bc4`). Neither contains a `marketing`, `groflo`, `timpager` or `Postgres-mpxU` service. `rally`, which earlier notes record as the home of the groflo app service (`fd4fefc7-...`) and the marketing service, today holds only `mydad`, `vibe101`, `support-thriftly-com`, `day81` and three Postgres instances.

At the same time `groflo.io`, `www.groflo.io`, `app.groflo.io/health`, `brands.groflo.io`, `penny.rallytasks.com/health` and `marketing-staging-66a2.up.railway.app` all return HTTP 404 with body `{"status":"error","code":404,"message":"Application not found"}` and header `x-railway-fallback: true`. That is Railway's answer when a hostname reaches its edge but no service claims it. So either the services were moved to a Railway team this account is not a member of and the custom domains were not re-attached there, or the services were deleted. The first needs a team invite or a new token before any Railway work; the second is a production outage and outranks everything in this document.

Adding `groflo.rallytasks.com` also needs: a confirmed, live marketing service to attach it to; a CNAME in the `rallytasks.com` DNS zone (owned by timpager); and a decision on whether the host 301-redirects to `groflo.io` or serves the site as a second canonical host (recommend 301; two live hosts split SEO and confuse the Privacy/Terms URLs quoted in Twilio vetting).

Owner: Andy. Until this is resolved, no one should run Railway domain, deploy or variable mutations for this site, and the service and project ids in this file and in project memory are unverified.
