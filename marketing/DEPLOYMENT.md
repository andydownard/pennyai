# Groflo Marketing Site — Deployment Guide

Step-by-step deployment flow for the Groflo marketing site and investor deck. Follow this for every feature, bug fix, or infrastructure change.

**Related:** The PennyAI app is a separate repo at `../pennyai/` with its own `DEPLOYMENT.md`.

---

## Branch Strategy

| Branch | Purpose | Deploys To |
|--------|---------|------------|
| `feature/*` | New features or bug fixes | — (local only) |
| `dev` | Integration branch, all features merge here first | Staging (if configured) |
| `main` | Production-ready code | Production (Railway auto-deploy) |

---

## Deployment Flow

### 1. Create a Feature Branch

```bash
git checkout dev
git pull origin dev
git checkout -b feature/your-feature-name
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
git push origin feature/your-feature-name
```

### 4. Merge into `dev`

```bash
git checkout dev
git pull origin dev
git merge feature/your-feature-name
git push origin dev
```

Or create a PR: `feature/your-feature-name` → `dev` and merge via GitHub.

### 5. Verify on `dev`

- Confirm build passes
- Preview locally with `npm run preview`

### 6. Create PR: `dev` → `main`

```bash
gh pr create --base main --head dev --title "Short description" --body "## Summary
- Change 1
- Change 2

## Test plan
- [ ] Verification step 1
- [ ] Verification step 2"
```

### 7. Merge and Deploy

```bash
gh pr merge <PR_NUMBER> --merge
```

Railway auto-deploys from `main`.

### 8. Verify Production

```bash
curl -sI https://groflo.io | head -5
# Expected: HTTP/2 200
```

---

## Architecture

- **Marketing SPA:** React 19 + TypeScript 5.9 + Vite 7.3
- **Investor Deck:** Node.js + SQLite, Twilio SMS auth (`deck/`)
- **Platform:** Railway (SPA) / Netlify (alternative)
- **Production URL:** `https://groflo.io`
- **App CTAs point to:** `https://app.groflo.io` (PennyAI)

---

## Build

```bash
npm run build    # tsc -b && vite build → outputs to dist/
```

Railway serves with: `npx serve dist -s -l $PORT`

---

## Railway Config

```toml
[build]
watchPatterns = ["src/**", "public/**", "index.html", "package.json", "vite.config.ts"]

[deploy]
startCommand = "npx serve dist -s -l $PORT"
healthcheckPath = "/"
restartPolicyType = "ON_FAILURE"
```

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

`VITE_APP_URL` controls where "Try Free" CTAs link. `VITE_BRAND_PORTAL_URL` controls where the campaign-signup confirmation hands off. Both are baked in at build time by Vite (defaults: `app.groflo.io` / `brands.groflo.io`). `BRAND_PORTAL_URL` (no `VITE_` prefix) is read at runtime by the `campaign-lead` Netlify function for the redirect URL it returns. `IP_HASH_SALT` is used by `campaign-lead` to hash client IPs for rate-limit bookkeeping; rotating it resets per-IP buckets.

Stripe is **not** used on the marketing site. Brand campaign signups submit a brief to Supabase via the `campaign-lead` Netlify function and then hand off to the brand portal (`brands.groflo.io`), which handles account creation, deposits, and Stripe payments. The `@stripe/*` npm packages are intentionally not in this repo's `package.json`.

---

## Investor Deck Server (`deck/`)

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
- [ ] PR reviewed and approved
