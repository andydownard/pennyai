# Host the Deck at `groflo.io/deck` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Serve a Groflo-branded, gate-free copy of the existing web deck at `https://groflo.io/deck` as an unlisted (noindex), ungated page.

**Architecture:** Add a static, self-contained copy of the deck under `marketing/public/deck/` and mount it from the marketing Express server (`server.ts`) at `/deck` with a `noindex` header, before the SPA catch-all. No proxy, no new service. The standalone gated `marketing/deck/` app is left untouched.

**Tech Stack:** Node/Express (`marketing/server.ts`), Vite static `public/`, vitest + supertest tests.

## Global Constraints

- Work in `marketing/` only. Do NOT modify the standalone `marketing/deck/` app.
- Spec: `docs/superpowers/specs/2026-06-25-groflo-deck-hosting-design.md`.
- Brand: user-visible text must say **Groflo** / **Penny**, never **Thriftly** / **thrift.ly**. App links use `https://app.groflo.io`.
- Test-to-code ratio ≥ 1.3×. TDD: write the failing test first, watch it fail, implement, watch it pass, commit.
- The `/deck` copy is **ungated** (no SMS auth) and **noindex** — it must contain no `/api/` calls, no `sendCode`/`verifyCode`, and no `localStorage` access.
- Vitest test files live in `marketing/__tests__/`. Run tests with `npm test` (from `marketing/`).
- Implement on a dedicated branch off `main` (e.g. `feat-deck-hosting`), NOT on `feat-clerk-phase2b`.

## File Structure

- Create: `marketing/public/deck/deck.html` — stripped/rebranded deck (from `marketing/deck/deck.html`).
- Create: `marketing/public/deck/penny-avatar.png` — copy of `marketing/deck/penny-avatar.png`.
- Create: `marketing/public/deck/handwritten-list.png` — copy of `marketing/deck/handwritten-list.png`.
- Create: `marketing/public/deck/logo.svg` — copy of `marketing/public/logo.svg` (Groflo logo for footer).
- Create: `marketing/__tests__/deck-content.test.ts` — fs-based content invariants for the deck file + assets + robots/sitemap.
- Create: `marketing/__tests__/deck-route.test.ts` — supertest route behavior (`/deck`, asset, `X-Robots-Tag`).
- Modify: `marketing/server.ts` — add `/deck` static mount before the SPA catch-all (server.ts:635).
- Modify: `marketing/public/robots.txt` — add `Disallow: /deck`.

---

### Task 1: Stage assets and produce the gate-free, rebranded deck

**Files:**
- Create: `marketing/public/deck/deck.html`, `penny-avatar.png`, `handwritten-list.png`, `logo.svg`
- Test: `marketing/__tests__/deck-content.test.ts`

**Interfaces:**
- Produces: the file `marketing/public/deck/deck.html` and sibling assets, consumed by Task 2's route.

- [ ] **Step 1: Copy assets and the source deck into `public/deck/`**

Run (from `marketing/`):
```bash
mkdir -p public/deck
cp deck/penny-avatar.png public/deck/penny-avatar.png
cp deck/handwritten-list.png public/deck/handwritten-list.png
cp public/logo.svg public/deck/logo.svg
cp deck/deck.html public/deck/deck.html
```

- [ ] **Step 2: Write the failing content test**

Create `marketing/__tests__/deck-content.test.ts`:
```ts
import { describe, it, expect } from 'vitest'
import { readFileSync, existsSync } from 'node:fs'
import path from 'node:path'

const ROOT = path.resolve(__dirname, '..')
const deckPath = path.join(ROOT, 'public', 'deck', 'deck.html')
const html = () => readFileSync(deckPath, 'utf8')

describe('public/deck/deck.html — assets present', () => {
  it('ships the deck and its image assets', () => {
    expect(existsSync(deckPath)).toBe(true)
    expect(existsSync(path.join(ROOT, 'public', 'deck', 'penny-avatar.png'))).toBe(true)
    expect(existsSync(path.join(ROOT, 'public', 'deck', 'handwritten-list.png'))).toBe(true)
    expect(existsSync(path.join(ROOT, 'public', 'deck', 'logo.svg'))).toBe(true)
  })
})

describe('public/deck/deck.html — renders as a deck', () => {
  it('keeps the deck content', () => {
    const h = html()
    expect(h).toMatch(/Saving families \$100\/month on groceries\./)
    expect(h).toContain('id="deck"')
  })
  it('shows the deck without a display:none gate', () => {
    expect(html()).not.toMatch(/id="deck"[^>]*display:none/)
  })
  it('marks the page noindex', () => {
    expect(html()).toMatch(/<meta\s+name="robots"\s+content="noindex/i)
  })
})

describe('public/deck/deck.html — ungated, no backend coupling', () => {
  const h = () => html()
  it('has no auth gate', () => {
    expect(h()).not.toContain('id="authGate"')
    expect(h()).not.toMatch(/sendCode|verifyCode/)
  })
  it('makes no API calls', () => {
    expect(h()).not.toContain('/api/')
  })
  it('does not touch localStorage', () => {
    expect(h()).not.toContain('localStorage')
  })
  it('drops the viewer-analytics slide', () => {
    expect(h()).not.toContain('loadMetrics')
    expect(h()).not.toContain('id="metricsContent"')
    expect(h()).not.toContain('data-slide="10"')
  })
})

describe('public/deck/deck.html — Groflo brand', () => {
  it('contains no Thriftly references', () => {
    expect(html().toLowerCase()).not.toMatch(/thriftly|thrift\.ly/)
  })
  it('namespaces image assets under /deck/', () => {
    const h = html()
    expect(h).toContain('/deck/penny-avatar.png')
    expect(h).toContain('/deck/handwritten-list.png')
    expect(h).not.toMatch(/src="\/penny-avatar\.png"/)
    expect(h).not.toMatch(/src="\/handwritten-list\.png"/)
  })
})
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `npm test -- deck-content`
Expected: FAIL — the freshly-copied `deck.html` still has `id="authGate"`, `/api/`, `localStorage`, `thriftly`, root-level asset paths, no `noindex` meta.

- [ ] **Step 4: Transform `public/deck/deck.html`**

Apply these exact edits to `marketing/public/deck/deck.html` (anchors are unique strings in the file). After editing, the tests in Step 2 define "done" — re-run and iterate until green.

1. **Add noindex** — in `<head>`, immediately after the existing `<title>Penny — Your Family's Smart Shopping List</title>` line, add:
   ```html
   <meta name="robots" content="noindex, nofollow">
   ```

2. **Namespace image assets** (replace ALL occurrences):
   - `/penny-avatar.png` → `/deck/penny-avatar.png`
   - `/handwritten-list.png` → `/deck/handwritten-list.png`

3. **Remove the auth gate** — delete the entire element starting at `<div class="auth-gate" id="authGate">` through its matching closing `</div>` (the block contains `id="authPhoneStep"`, `id="authCodeStep"`, the `authTypewriter` span, and ends just before the `<!-- ═══ DECK ═══ -->` comment).

4. **Show the deck by default** — change:
   ```html
   <div class="deck" id="deck" style="display:none">
   ```
   to:
   ```html
   <div class="deck" id="deck">
   ```

5. **Remove the viewer-analytics slide (slide 11)** — delete from the comment `<!-- ═══ SLIDE 11: Engagement Metrics ═══ -->` through the `</div>` that closes its `<div class="slide-container" data-slide="10">` (the block contains `id="metricsLoading"` and `id="metricsContent"`; it ends right before the `<!-- Nav -->` comment).

6. **Rebrand the footer logo** — replace:
   ```html
   powered by <img src="/thriftly-logo.svg" alt="thriftly"
   ```
   with:
   ```html
   powered by <img src="/deck/logo.svg" alt="Groflo"
   ```

7. **Remove the feedback widget container** — delete the line:
   ```html
   <div id="fbWidget" style="display:none"></div>
   ```

8. **Fix the slide count** — in the `<script>`, change `const T=11;` to `const T=10;` and remove the trailing `,'Engagement'` element from the `const TITLES=[...]` array (last entry).

9. **Remove the analytics trigger** — delete the line `if(c===10)loadMetrics();` inside `render()`.

10. **Remove all auth/analytics/feedback JS.** Delete these definitions entirely:
    - `let authPhone='';` and `function formatPhone(...)`, `function backToPhone(...)`, and the auth-gate typewriter IIFE that references `authTypewriter`.
    - `async function sendCode(){...}` and `async function verifyCode(){...}`.
    - `function revealDeck(){...}` and the session-check IIFE immediately after it (`// Check existing session on load` … the `(function(){ … showGate … })();` block).
    - `function loadMetrics(){...}`.
    - The slide-view tracking block: `let viewStart=...`, `function trackView(){...}`, and `window.addEventListener('beforeunload',trackView);`.
    - The entire `// ─── Feedback Widget ───` IIFE (`(function(){ … })();` that defines `renderFB`, `window.fbOpen`, `fbSubmit`, `fbLoadHist`, `showToast`, and ends with `setTimeout(renderFB,1000);`).

11. **Render the deck on load** — where `revealDeck`/the session IIFE used to be, the deck must initialize itself. Ensure the script, before its final `setTimeout(recalc,100);window.addEventListener('resize',recalc);` line, calls:
    ```js
    render();typewriter();
    ```
    (The hash-bootstrap IIFE that sets `c` from `location.hash` stays; `render`, `typewriter`, `recalc`, `goTo`, `next`, `prev` are all defined earlier and remain.)

12. **Sweep remaining brand refs** — replace any remaining `thrift.ly` / `Thriftly` strings (e.g. the `app.thrift.ly` link and comments) with `groflo.io` / `app.groflo.io` / `Groflo`. Verify with:
    ```bash
    grep -niE "thriftly|thrift\.ly" public/deck/deck.html   # expect no output
    ```

- [ ] **Step 5: Run the test to verify it passes**

Run: `npm test -- deck-content`
Expected: PASS (all assertions). If any negative assertion fails, grep the file for the offending token and remove that block.

- [ ] **Step 6: Commit**

```bash
git add marketing/public/deck marketing/__tests__/deck-content.test.ts
git commit -m "feat(marketing): add gate-free Groflo deck under public/deck"
```

---

### Task 2: Mount `/deck` from the server and mark it unlisted

**Files:**
- Modify: `marketing/server.ts` (add mount before the SPA catch-all at server.ts:635)
- Modify: `marketing/public/robots.txt`
- Test: `marketing/__tests__/deck-route.test.ts`

**Interfaces:**
- Consumes: `marketing/public/deck/deck.html` + assets from Task 1.
- Consumes: `createApp()` exported from `../server` (used by existing `__tests__/server.test.ts`).

- [ ] **Step 1: Write the failing route test**

Create `marketing/__tests__/deck-route.test.ts`:
```ts
import { describe, it, expect } from 'vitest'
import { vi } from 'vitest'
vi.mock('pg', () => import('./_pg-mock'))
process.env.DATABASE_URL ||= 'postgres://test'
import request from 'supertest'
import { readFileSync } from 'node:fs'
import path from 'node:path'
// eslint-disable-next-line import/first
import { createApp } from '../server'

const app = createApp()
const ROOT = path.resolve(__dirname, '..')

describe('GET /deck', () => {
  it('serves the deck HTML', async () => {
    const res = await request(app).get('/deck/')
    expect(res.status).toBe(200)
    expect(res.headers['content-type']).toMatch(/text\/html/)
    expect(res.text).toContain('id="deck"')
  })
  it('marks the deck noindex via header', async () => {
    const res = await request(app).get('/deck/')
    expect(res.headers['x-robots-tag']).toMatch(/noindex/)
  })
  it('serves deck image assets', async () => {
    const res = await request(app).get('/deck/penny-avatar.png')
    expect(res.status).toBe(200)
    expect(res.headers['content-type']).toMatch(/image\/png/)
  })
})

describe('deck is unlisted', () => {
  it('robots.txt disallows /deck', () => {
    const robots = readFileSync(path.join(ROOT, 'public', 'robots.txt'), 'utf8')
    expect(robots).toMatch(/Disallow:\s*\/deck/)
  })
  it('sitemap.xml does not list /deck', () => {
    const sitemap = readFileSync(path.join(ROOT, 'public', 'sitemap.xml'), 'utf8')
    expect(sitemap).not.toContain('/deck')
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npm test -- deck-route`
Expected: FAIL — `GET /deck/` returns 404 (no route yet); `robots.txt` has no `Disallow: /deck`.

- [ ] **Step 3: Add the `/deck` mount in `server.ts`**

In `marketing/server.ts`, locate the static + SPA-fallback block (the `if (fs.existsSync(DIST_DIR)) {` section ending with the catch-all `app.get(/.*/, ...)` near line 635). Immediately BEFORE that `if (fs.existsSync(DIST_DIR))` block, insert:

```ts
  // ── Unlisted deck (groflo.io/deck) ──
  // Served from committed public/deck so it works without a Vite build.
  // noindex header keeps it out of search; no auth (unlisted, not gated).
  app.use(
    '/deck',
    (_req: Request, res: Response, next: NextFunction) => {
      res.setHeader('X-Robots-Tag', 'noindex, nofollow')
      next()
    },
    express.static(path.join(__dirname, 'public', 'deck'), { index: 'deck.html' })
  )
```

(`express`, `path`, `__dirname`, and the `Request`/`Response`/`NextFunction` types are already imported at the top of `server.ts`.)

- [ ] **Step 4: Add the robots.txt disallow**

Edit `marketing/public/robots.txt` from:
```
User-agent: *
Allow: /
Sitemap: https://groflo.io/sitemap.xml
```
to:
```
User-agent: *
Allow: /
Disallow: /deck
Sitemap: https://groflo.io/sitemap.xml
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `npm test -- deck-route`
Expected: PASS.

- [ ] **Step 6: Run the full marketing suite**

Run: `npm test`
Expected: PASS with zero failures (existing `server.test.ts` plus the two new files).

- [ ] **Step 7: Commit**

```bash
git add marketing/server.ts marketing/public/robots.txt marketing/__tests__/deck-route.test.ts
git commit -m "feat(marketing): serve unlisted deck at /deck with noindex"
```

---

### Task 3: Build verification and docs

**Files:**
- Modify: `marketing/BACKLOG.md` (move/mark the deck-hosting item Shipped, if present) — optional, only if an item exists.

- [ ] **Step 1: Verify the production build includes the deck**

Run (from `marketing/`):
```bash
npm run build
ls dist/deck/deck.html dist/deck/penny-avatar.png
```
Expected: both files exist (Vite copies `public/` into `dist/`).

- [ ] **Step 2: Smoke-test the running server**

Run:
```bash
npm start &
sleep 2
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:${PORT:-3000}/deck/
curl -s -D - -o /dev/null http://localhost:${PORT:-3000}/deck/ | grep -i x-robots-tag
kill %1
```
Expected: `200`, and an `X-Robots-Tag: noindex, nofollow` header.

- [ ] **Step 3: Commit any doc updates**

```bash
git add marketing/BACKLOG.md
git commit -m "docs(marketing): mark deck hosting shipped"
```
(Skip if no doc change was needed.)

---

## Self-Review

**Spec coverage:**
- Static gate-free copy under `public/deck/` → Task 1. ✓
- Rebrand Thriftly→Groflo + namespaced asset paths → Task 1 steps 2, 6, 12. ✓
- Strip auth gate / `/api/` / analytics / feedback → Task 1 steps 3, 5, 7, 9, 10. ✓
- noindex meta + `X-Robots-Tag` header → Task 1 step 1, Task 2 step 3. ✓
- `/deck` mount before SPA catch-all → Task 2 step 3. ✓
- robots.txt Disallow, sitemap unchanged, no nav link → Task 2 steps 4, tests; no nav edit anywhere. ✓
- Tests with ≥1.3× ratio (two test files, small code delta) → Tasks 1–2. ✓
- Original `deck/` app untouched → not modified by any task. ✓

**Placeholder scan:** No TBD/TODO; every code/test block is complete; removal targets are anchored on exact unique strings.

**Type consistency:** Route uses `express.static` + `path.join(__dirname, 'public', 'deck')`, matching existing `server.ts` imports (`express`, `path`, `__dirname`, `Request/Response/NextFunction`). Tests use `createApp()` exactly as `server.test.ts` does, with the same `pg` mock + `DATABASE_URL` guard.
