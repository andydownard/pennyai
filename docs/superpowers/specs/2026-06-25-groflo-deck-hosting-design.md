# Host the deck at `groflo.io/deck` — Design

**Date:** 2026-06-25
**Status:** Approved (design), pending spec review
**Scope:** `marketing/` (Groflo marketing site — `groflo-site`, Railway service `marketing`)

## Goal

Serve a Groflo-branded copy of the existing web deck at `https://groflo.io/deck` as an
**unlisted, ungated** page: reachable by direct link, excluded from search indexing, not
linked from any nav, and not behind the SMS auth gate.

## Context / Findings

- `groflo.io` is served by the marketing site (`marketing/`): an Express server (`server.ts`)
  that serves the built Vite SPA from `dist/` with a catch-all (`app.get(/.*/)`, server.ts:635)
  falling back to `index.html`. There is no `/deck` route today.
- A standalone investor-deck app already exists at `marketing/deck/` (Node + SQLite + Twilio
  Verify auth, feedback widget, slide-view analytics, its own Dockerfile + railway.toml). It is
  **not** wired into the marketing server and is deployed (or deployable) independently.
- `marketing/deck/deck.html` is the source web deck. It currently:
  - Has a **built-in SMS auth gate** coupled to `/api/auth/*`, `/api/view`, `/api/feedback`
    (backed by the standalone deck service), and stores a session token in `localStorage`.
  - References assets by absolute root path: `/penny-avatar.png`, `/handwritten-list.png`,
    `/thriftly-logo.svg`.
  - Carries `Thriftly` references (the visible one: `powered by <img src="/thriftly-logo.svg"
    alt="thriftly">` at line 552). Title is already correct: "Penny — Your Family's Smart
    Shopping List".

## Decision

**Approach B — serve a static, gate-free, Groflo-branded copy under `/deck` from the marketing
server.** Single service, no proxy, no new infra. The original gated `deck/` app is left
untouched (so the marketing rule "do not remove the deck auth gate / feedback / analytics" is
not violated — we add a separate unlisted copy rather than editing that app).

Rejected:
- **A (reverse-proxy to the deck service):** keeps it gated unless an ungated-mode flag is added;
  two services + proxy to maintain.
- **C (proxy + ungated-mode flag):** preserves feedback/analytics but most moving parts. User
  confirmed feedback/analytics are not needed on the unlisted copy.

## Design

### 1. Assets → `marketing/public/deck/`

Everything is namespaced under `/deck/` so it is self-contained and adds nothing at the site root.

- Copy into `public/deck/`:
  - `deck.html` (modified, see below)
  - `penny-avatar.png` (≈788 KB)
  - `handwritten-list.png` (≈589 KB)
- Rewrite absolute asset paths inside the copied `deck.html`:
  - `/penny-avatar.png` → `/deck/penny-avatar.png`
  - `/handwritten-list.png` → `/deck/handwritten-list.png`
- Rebrand:
  - `powered by <img src="/thriftly-logo.svg" alt="thriftly">` → Groflo `/logo.svg`,
    `alt="Groflo"` (the marketing site already ships `public/logo.svg`).
  - Sweep remaining `Thriftly` / `thrift.ly` strings (comments, meta) → Groflo so the brand
    negative-test passes.
- Strip the auth gate: remove the gate overlay and the `/api/auth/*`, `/api/view`,
  `/api/feedback` client JS so the deck renders immediately with no backend calls.
- Add `<meta name="robots" content="noindex, nofollow">` to `<head>`.

### 2. Server wiring (`marketing/server.ts`)

Register **before** the SPA catch-all (server.ts:635), reading from the committed `public/deck/`
source (so it works in dev, prod, and tests without a Vite build — consistent with the existing
`fs.existsSync(DIST_DIR)` guard pattern):

```ts
app.use(
  '/deck',
  (_req, res, next) => { res.setHeader('X-Robots-Tag', 'noindex, nofollow'); next() },
  express.static(path.join(__dirname, 'public', 'deck'), { index: 'deck.html' })
)
```

- `groflo.io/deck` → serves the deck (static `index: 'deck.html'`).
- `groflo.io/deck/penny-avatar.png` etc. → assets, all carrying `X-Robots-Tag: noindex`.
- Path resolution must match how `server.ts` already derives directories (e.g. `__dirname` via
  `fileURLToPath(import.meta.url)` for ESM) — confirm and reuse the existing helper at
  implementation time.

### 3. Unlisted, not gated

- `public/robots.txt`: add `Disallow: /deck`.
- `public/sitemap.xml`: unchanged (never lists `/deck`).
- No nav link anywhere. No auth.

### 4. Tests (`marketing/__tests__/`, vitest + supertest)

Maintain the ≥1.3× test-to-code ratio. Drive the Express app via supertest:

- `GET /deck/` → 200, `Content-Type: text/html`, body contains a known deck marker.
- `GET /deck/` response carries `X-Robots-Tag: noindex`.
- `GET /deck/penny-avatar.png` → 200 (asset served).
- Negative: deck HTML contains no `sendCode` / auth-gate markers (gate removed).
- Negative: deck HTML contains no `thriftly` / `thrift.ly` (brand).
- Negative: `sitemap.xml` does not contain `/deck`; `robots.txt` contains `Disallow: /deck`.

## Out of Scope

- Feedback widget, slide-view analytics, SMS auth (remain only on the original gated `deck/` app).
- Any change to marketing SPA pages, nav, or the standalone `deck/` service.
- A password / gate on the `/deck` copy (explicitly "unlisted, not secured").

## Branching / Deploy Notes

- Implement on a dedicated branch off `main` (not the in-flight `feat-clerk-phase2b` working
  tree). `main` → `staging` → `production`; verify healthcheck on staging before production.
- Panel: marketing is user-facing — Brand PM should confirm the Groflo logo swap reads cleanly
  (Small change, ~2 min per PANEL_GOVERNANCE).
