# PennyAI — Deployment Guide

Step-by-step deployment flow for the PennyAI app. Follow this for every feature, bug fix, or infrastructure change.

**Related:** The Groflo marketing site is a separate repo at `../marketing-site/` with its own `DEPLOYMENT.md`.

---

## Branch Strategy

| Branch | Purpose | Deploys To |
|--------|---------|------------|
| `feature/*` | New features or bug fixes | — (local only) |
| `main` | Integration — all PRs land here first | — (local / preview only) |
| `staging` | Merge-only from `main` after CI green | Staging environment |
| `production` | Merge-only from `staging` after staging healthcheck green | Production (app.groflo.io) |

**Hard rule:** `staging` and `production` are MERGE-ONLY. No direct commits, no force-push. Never promote `staging` → `production` without `groflo.io` green on staging.

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
# Backend tests (required — must pass with zero failures)
go test -count=1 ./...

# Frontend type check + build
cd frontend && npm run build

# E2E tests
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

Review, then squash-merge via GitHub. CI (GitHub Actions) runs: Go tests, test-to-code ratio, `go vet`, frontend typecheck + build.

### 5. Promote `main` → `staging`

```bash
git checkout staging
git pull origin staging
git merge --ff-only origin/main  # or non-ff merge commit if branches have diverged
git push origin staging
```

Wait for Railway to finish deploying staging. Then:

```bash
curl -s https://staging.groflo.io/health
# Expected: {"status":"ok","frontend_ok":true,"db_ok":true}
```

### 6. Promote `staging` → `production`

**Only after staging healthcheck is green.** This gate is non-negotiable.

```bash
git checkout production
git pull origin production
git merge --ff-only origin/staging
git push origin production
```

### 7. Verify Production

```bash
curl -s https://app.groflo.io/health
# Expected: {"status":"ok","frontend_ok":true,"db_ok":true,"version":"..."}
```

---

## Architecture

- **Backend:** Go 1.24, SQLite (WAL mode), single binary `thriftly`
- **Frontend:** React 18 + Vite, built into `static/`
- **Brand Portal:** Separate React app, built into `static-brand/`
- **Container:** Multi-stage Docker (Node → Go → Alpine + Litestream)
- **Platform:** Railway (auto-deploy from `staging` and `production` branches)
- **Production URL:** `https://app.groflo.io`
- **DNS:** CNAME in AWS Route 53 (groflo.io) → Railway custom domain

---

## Build Pipeline (Docker)

```
Stage 1:  Node 20 → npm ci → npm run build → /app/static (consumer frontend)
Stage 1b: Node 20 → npm ci → npm run build → /app/static-brand (brand portal)
Stage 2:  Go 1.24 → go build -o thriftly
Stage 3:  Alpine 3.20 + Litestream v0.3.13 → run.sh
```

The build validates frontend output — checks for `index.html`, JS bundles, and all asset references.

---

## CI Checks (GitHub Actions)

Runs on push to `main`, `staging`, `production`, and PRs to any of them:

| Check | Command | Requirement |
|-------|---------|-------------|
| Go tests | `go test -count=1 -race -timeout 20m -v ./...` | Zero failures |
| Test-to-code ratio | `bash scripts/check-test-ratio.sh --ci` | >= 1.3x |
| Go vet | `go vet ./...` | Clean |
| Frontend type check | `tsc --noEmit` | Clean |
| Frontend build | `vite build` | Succeeds |

---

## Database — SQLite + Litestream

**Runtime behavior (`run.sh`):**

1. On startup, checks if `LITESTREAM_REPLICA_BUCKET` is set
2. If set and no local DB exists → restores from S3/R2 replica
3. Starts server wrapped in Litestream for continuous replication (10s sync, 72h retention)
4. If `LITESTREAM_REPLICA_BUCKET` not set → starts server directly (no backup)

**Manual restore (emergency):**
```bash
litestream restore -config /etc/litestream.yml thriftly.db
```

---

## Required Environment Variables

Set these in Railway service settings:

```
# Core
PORT=8080
DB_PATH=thriftly.db

# Twilio (SMS auth + messaging)
TWILIO_ACCOUNT_SID=
TWILIO_AUTH_TOKEN=
TWILIO_VERIFY_SERVICE_SID=
TWILIO_PHONE_NUMBER=

# AI
ANTHROPIC_API_KEY=

# Database backup (Litestream → S3/R2)
LITESTREAM_REPLICA_BUCKET=
LITESTREAM_REPLICA_ENDPOINT=
LITESTREAM_ACCESS_KEY_ID=
LITESTREAM_SECRET_ACCESS_KEY=

# Optional
ADMIN_PHONE=
LOGODEV_TOKEN=
LOGODEV_SECRET=
GOOGLE_PLACES_API_KEY=
NPS_INTERVAL_DAYS=
GOOGLE_CLIENT_ID=
SENTRY_DSN=
POSTHOG_API_KEY=
```

---

## Health Check & Monitoring

- **Endpoint:** `GET /health` → returns `{"status":"ok","frontend_ok":true,"db_ok":true}`
- **Railway:** healthcheck timeout 30s, restart on failure (max 5 retries)
- **Uptime monitor:** GitHub Actions checks every 5 minutes, auto-redeploys via Railway API if down

---

## Setting Up a New Railway Service

1. **Create service** in Railway, connect to GitHub repo
2. **Set branch** — `staging` for the staging service, `production` for the production service
3. **Add all environment variables** (copy from existing service or `.env.example`)
4. **Deploy** — Litestream auto-restores DB from R2 on first boot
5. **Add custom domain** in Railway Settings > Networking
6. **Update DNS** — create CNAME record in AWS Route 53 pointing to Railway's target
7. **Verify:** `curl https://your-domain/health`

---

## Checklist Before Merging to `main`

- [ ] All Go tests pass (`go test -count=1 ./...`)
- [ ] Test-to-code ratio >= 1.3x
- [ ] Frontend builds without errors
- [ ] E2E tests pass
- [ ] No secrets in committed code
- [ ] No breaking changes to API endpoints or DB schema
- [ ] PR reviewed and approved

## Checklist Before Promoting `staging` → `production`

- [ ] Staging healthcheck green (`curl https://staging.groflo.io/health` returns `ok`)
- [ ] Staging service log is clean (no panics / errors since last deploy)
- [ ] Manual smoke test of core flows passed on staging
- [ ] All CI checks green on the merge commit that landed on `staging`
