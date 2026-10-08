# PennyAI — Deployment Guide

Step-by-step deployment flow for the PennyAI app. Follow this for every feature, bug fix, or infrastructure change.

**Related:** The Groflo marketing site is a separate repo at `../marketing-site/` with its own `DEPLOYMENT.md`.

---

## Branch Strategy

Railway project `rally`, service `pennyai`, deploys the **`main`** branch. All code reaches `main` through a PR.

**Open decision (Andy):** the earlier model was `main` → `staging` → `production`, merge-only, with a green staging healthcheck before any production merge. Railway no longer follows it. Whether to restore that model or keep deploying `main` has not been decided; do not invent a policy. See [BACKLOG.md](./BACKLOG.md).

---

## Deployment Flow

### 1. Create a Feature Branch from `main`

```bash
git checkout main
git pull origin main
git checkout -b feature/your-feature-name
```

### 2. Develop and Test Locally

```bash
# Backend tests, SQLite tier (required — must pass with zero failures)
go test -count=1 ./...

# Backend tests, PostgreSQL tier (required; needs a local Postgres)
scripts/test-postgres.sh
# or: TEST_DATABASE_URL=postgres://localhost:5432/postgres?sslmode=disable go test .

# Frontend type check + build
cd frontend && npm run build

# E2E tests (creates a throwaway local Postgres database, runs with TEST_MODE=true)
cd frontend && npx playwright test
```

### 3. Commit, Push, and Open PR → `main`

```bash
git add <files>
git commit -m "feat: description of change"
git push -u origin feature/your-feature-name
gh pr create --base main --head feature/your-feature-name --title "Short description"
```

### 4. Merge into `main`

Review, then squash-merge via GitHub. CI (GitHub Actions) runs: Go tests on SQLite, Go tests on PostgreSQL (`test-postgres`), test-to-code ratio, `go vet`, frontend typecheck and tests. A failing Go test now fails the job.

### 5. Railway Deploys `main`

Railway builds the Dockerfile on every push to `main`. Then:

```bash
curl -s https://groflo.rallytasks.com/health
# Expected: {"status":"ok","frontend_ok":true,"db_ok":true,"version":"..."}
```

---

## Architecture

- **Backend:** Go 1.25, PostgreSQL (Railway-managed), single binary `thriftly`
- **Frontend:** React 18 + Vite, built into `static/`
- **Brand Portal:** Separate React app, built into `static-brand/`
- **Container:** Multi-stage Docker (Node → Go → Alpine)
- **Platform:** Railway, project `rally`, service `pennyai` (auto-deploy from `main`; `railway.toml` selects the Dockerfile builder)
- **URL:** `https://groflo.rallytasks.com`
- **Not attached to any service:** `app.groflo.io`, `brands.groflo.io`, `penny.rallytasks.com`, `groflo.io`. The brand portal and the marketing site therefore have no host.

---

## Build Pipeline (Docker)

```
Stage 1:  Node 20 → npm ci → npm run build → /app/static (consumer frontend; `VITE_*` values arrive as Docker build args)
Stage 1b: Node 20 → npm ci → npm run build → /app/static-brand (brand portal)
Stage 2:  Go 1.25 → go build -o thriftly
Stage 3:  Alpine 3.20 → runs the server binary directly
```

The build validates frontend output — checks for `index.html`, JS bundles, and all asset references.

---

## CI Checks (GitHub Actions)

Runs on push to `main` and PRs to `main`:

| Check | Command | Requirement |
|-------|---------|-------------|
| Go tests (SQLite) | `go test -count=1 -race -timeout 20m ./...` | Zero failures |
| Go tests (PostgreSQL) | `scripts/test-postgres.sh` (job `test-postgres`) | Zero failures |
| Test-to-code ratio | `bash scripts/check-test-ratio.sh --ci` | >= 1.3x |
| Go vet | `go vet ./...` | Clean |
| Frontend type check | `tsc --noEmit` | Clean |
| Frontend build | `vite build` | Succeeds |

---

## Database — PostgreSQL (Railway)

pennyai owns its own Railway-managed PostgreSQL instance (`DATABASE_URL`). The database is fresh as of 2026-10-03; the old `groflo` service and its database no longer exist. Backups, durability, and point-in-time recovery are handled by Railway; no application-level backup wrapper is required. The container starts the server binary directly.

---

## Required Environment Variables

Set these in Railway service settings:

```
# Core
PORT=8080
DATABASE_URL=
APP_BASE_URL=https://groflo.rallytasks.com
WEBHOOK_BASE_URL=https://groflo.rallytasks.com

# Rally sign-in (Clerk). Public values only: never set CLERK_SECRET_KEY or
# CLERK_WEBHOOK_SIGNING_SECRET (the server refuses to boot).
CLERK_ENABLED=1
CLERK_JWT_ISSUER=
CLERK_AUTHORIZED_PARTIES=
CLERK_FRONTEND_API_ORIGIN=

# Admin (either grants admin)
ADMIN_PHONE=
ADMIN_CLERK_USER_IDS=

# Twilio (SMS only; optional, SMS is off when missing)
TWILIO_ACCOUNT_SID=
TWILIO_AUTH_TOKEN=
TWILIO_PHONE_NUMBER=

# AI
ANTHROPIC_API_KEY=

# Optional
LOGODEV_TOKEN=
LOGODEV_SECRET=
GOOGLE_PLACES_API_KEY=
SENDGRID_API_KEY=
NPS_INTERVAL_DAYS=
SENTRY_DSN=
POSTHOG_API_KEY=
```

Frontend build values (`VITE_CLERK_PUBLISHABLE_KEY`, `VITE_CLERK_SIGN_IN_URL`, `VITE_CLERK_DOMAIN`, `VITE_ALLOWED_REDIRECT_ORIGINS`, `VITE_APP_URL`, `VITE_BRAND_PORTAL_URL`, `VITE_POSTHOG_KEY`) are Docker build args: Railway passes a service variable to the build only when the Dockerfile declares it as an `ARG`. `VITE_CLERK_DOMAIN` switches the frontend to satellite mode; leave it unset for first-party Clerk.

`VITE_CLERK_SIGN_IN_URL` must be Clerk's Account Portal for the Rally instance, `https://accounts.rallytasks.com/sign-in`. Do not point it at `www.rallytasks.com/sign-in`: that was the retired Timpager app and returns Railway's "Not Found". Changing any `VITE_*` value requires a rebuild, which Railway does when the variable is set.

`TEST_MODE=true` must never be set on Railway; the server refuses to boot with it there.

---

## Health Check & Monitoring

- **Endpoint:** `GET /health` → returns `{"status":"ok","frontend_ok":true,"db_ok":true}`
- **Railway:** healthcheck timeout 120s, restart on failure (max 5 retries)
- **Uptime monitor:** `.github/workflows/uptime.yml` checks every 5 minutes. It still points at the retired `app.thrift.ly` host and needs repointing (see [BACKLOG.md](./BACKLOG.md)).

---

## Setting Up a New Railway Service

1. **Create service** in Railway, connect to GitHub repo
2. **Set branch** to `main`
3. **Add all environment variables** (see `.env.example`), including the `VITE_*` build values
4. **Deploy** — Railway builds the Dockerfile and starts the service
5. **Add custom domain** in Railway Settings > Networking and point DNS at Railway's target
6. **Verify:** `curl https://your-domain/health`

---

## Checklist Before Merging to `main`

- [ ] All Go tests pass (`go test -count=1 ./...` and `scripts/test-postgres.sh`)
- [ ] Test-to-code ratio >= 1.3x
- [ ] Frontend builds without errors
- [ ] E2E tests pass
- [ ] No secrets in committed code
- [ ] No breaking changes to API endpoints or DB schema
- [ ] PR reviewed and approved
