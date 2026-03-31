# PennyAI — Deployment Guide

Step-by-step deployment flow for the PennyAI app. Follow this for every feature, bug fix, or infrastructure change.

**Related:** The Groflo marketing site is a separate repo at `../marketing-site/` with its own `DEPLOYMENT.md`.

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
# Backend tests (required — must pass with zero failures)
go test -count=1 ./...

# Frontend type check + build
cd frontend && npm run build

# E2E tests
cd frontend && npx playwright test
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

- Confirm CI passes (GitHub Actions: tests, type check, test-to-code ratio)
- Test on staging if configured

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
curl -s https://app.groflo.io/health
# Expected: {"status":"ok","frontend_ok":true,"db_ok":true,"version":"..."}
```

---

## Architecture

- **Backend:** Go 1.24, SQLite (WAL mode), single binary `thriftly`
- **Frontend:** React 18 + Vite, built into `static/`
- **Brand Portal:** Separate React app, built into `static-brand/`
- **Container:** Multi-stage Docker (Node → Go → Alpine + Litestream)
- **Platform:** Railway (auto-deploy from `main`)
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

Runs on push to `main` and PRs to `main`:

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
2. **Set branch** to `main`
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
