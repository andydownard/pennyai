# Host the Deck at `groflo.io/deck` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Serve a Groflo-branded, gate-free copy of the existing web deck at `https://groflo.io/deck` as an unlisted (noindex), ungated page.

**Architecture:** `main`'s marketing site is a **Netlify-deployed Vite SPA** (`netlify.toml` with a `/* → /index.html` catch-all, `public/_headers`, Netlify Functions — there is no Express `server.ts` on `main`). Add a static, self-contained copy of the deck under `marketing/public/deck/` (Vite copies `public/` into `dist/`, which Netlify serves), and route it with a `netlify.toml` redirect (`/deck → /deck/deck.html`, status 200, placed **before** the SPA catch-all) plus a `/deck/*` `X-Robots-Tag: noindex` header. The standalone gated `marketing/deck/` app is left untouched.

**Tech Stack:** Vite static `public/`, Netlify config (`netlify.toml`, `public/_headers`, `public/robots.txt`), vitest (fs-based config/content tests — no Express/supertest on `main`).

## Global Constraints

- Work in `marketing/` only. Do NOT modify the standalone `marketing/deck/` app.
- Spec: `docs/superpowers/specs/2026-06-25-groflo-deck-hosting-design.md` (spec describes the Express variant; this plan targets the `main`/Netlify architecture per the chosen base branch).
- Brand: user-visible text must say **Groflo** / **Penny**, never **Thriftly** / **thrift.ly**. App links use `https://app.groflo.io`.
- Test-to-code ratio ≥ 1.3×. TDD: write the failing test first, watch it fail, implement, watch it pass, commit.
- The `/deck` copy is **ungated** (no SMS auth) and **noindex** — it must contain no `/api/` calls, no `sendCode`/`verifyCode`, and no `localStorage` access.
- Vitest test files live in `marketing/__tests__/`. Run tests with `npm test` (from `marketing/`).
- Base branch is `feat-deck-hosting`, branched off `main` (Netlify marketing). The orchestrator has created an isolated worktree at `/Users/andydownard/projects/pennyai-worktrees/feat-deck-hosting`; all implementation happens there. Do NOT touch `feat-clerk-phase2b`.
- The clean URL `/deck` resolves via Netlify in production; locally (`npm start` → `serve dist -s`) only `/deck/deck.html` and `/deck/` files resolve directly. Verify via the built `dist/deck/` output, not a clean-URL request to the local static server.

## File Structure

- Create: `marketing/public/deck/deck.html` — stripped/rebranded deck (from `marketing/deck/deck.html`).
- Create: `marketing/public/deck/penny-avatar.png` — copy of `marketing/deck/penny-avatar.png`.
- Create: `marketing/public/deck/handwritten-list.png` — copy of `marketing/deck/handwritten-list.png`.
- Create: `marketing/public/deck/logo.svg` — copy of `marketing/public/logo.svg` (Groflo logo for footer).
- Create: `marketing/__tests__/deck-content.test.ts` — fs-based content invariants for the deck file + assets.
- Create: `marketing/__tests__/deck-config.test.ts` — fs-based assertions on `netlify.toml`, `public/_headers`, `public/robots.txt`, `public/sitemap.xml`.
- Modify: `marketing/netlify.toml` — add `/deck` redirect (before SPA catch-all) + `/deck/*` noindex header.
- Modify: `marketing/public/_headers` — add `/deck/*` noindex header.
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

### Task 2: Route `/deck` via Netlify config and mark it unlisted

**Files:**
- Modify: `marketing/netlify.toml`
- Modify: `marketing/public/_headers`
- Modify: `marketing/public/robots.txt`
- Test: `marketing/__tests__/deck-config.test.ts`

**Interfaces:**
- Consumes: `marketing/public/deck/deck.html` + assets from Task 1.

- [ ] **Step 1: Write the failing config test**

Create `marketing/__tests__/deck-config.test.ts`:
```ts
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import path from 'node:path'

const ROOT = path.resolve(__dirname, '..')
const read = (rel: string) => readFileSync(path.join(ROOT, rel), 'utf8')

describe('netlify.toml — deck routing', () => {
  const toml = () => read('netlify.toml')
  it('redirects the clean /deck URL to the deck file', () => {
    expect(toml()).toMatch(/from\s*=\s*"\/deck"/)
    expect(toml()).toMatch(/to\s*=\s*"\/deck\/deck\.html"/)
  })
  it('puts the /deck redirect before the SPA catch-all', () => {
    const t = toml()
    expect(t.indexOf('"/deck"')).toBeGreaterThan(-1)
    expect(t.indexOf('"/deck"')).toBeLessThan(t.indexOf('"/*"'))
  })
  it('sets a noindex X-Robots-Tag header for /deck/*', () => {
    expect(toml()).toMatch(/for\s*=\s*"\/deck\/\*"/)
    expect(toml()).toMatch(/X-Robots-Tag\s*=\s*"noindex/)
  })
})

describe('public/_headers — deck noindex', () => {
  it('adds an X-Robots-Tag noindex rule for /deck/*', () => {
    const h = read('public/_headers')
    expect(h).toMatch(/\/deck\/\*/)
    expect(h).toMatch(/X-Robots-Tag:\s*noindex/)
  })
})

describe('deck is unlisted', () => {
  it('robots.txt disallows /deck', () => {
    expect(read('public/robots.txt')).toMatch(/Disallow:\s*\/deck/)
  })
  it('sitemap.xml does not list /deck', () => {
    expect(read('public/sitemap.xml')).not.toContain('/deck')
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npm test -- deck-config`
Expected: FAIL — no `/deck` redirect or `/deck/*` header exists yet; `robots.txt` has no `Disallow: /deck`.

- [ ] **Step 3: Add the `/deck` redirect + noindex header to `netlify.toml`**

In `marketing/netlify.toml`, the existing functions redirect block ends at:
```toml
[[redirects]]
  from = "/.netlify/functions/*"
  to = "/.netlify/functions/:splat"
  status = 200
```
Immediately AFTER that block and BEFORE the `# SPA client-side routing:` catch-all (`from = "/*"`), insert:
```toml
# Unlisted deck: clean URL → deck file (must be before the SPA catch-all)
[[redirects]]
  from = "/deck"
  to = "/deck/deck.html"
  status = 200
```
Then, after the existing `[[headers]]` blocks (at the end of the file), append:
```toml
# Keep the unlisted deck out of search indexes
[[headers]]
  for = "/deck/*"
  [headers.values]
    X-Robots-Tag = "noindex, nofollow"
```

- [ ] **Step 4: Add the noindex rule to `public/_headers`**

Append to `marketing/public/_headers`:
```
# Unlisted deck — keep out of search indexes
/deck/*
  X-Robots-Tag: noindex, nofollow
```

- [ ] **Step 5: Add the robots.txt disallow**

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

- [ ] **Step 6: Run the test to verify it passes**

Run: `npm test -- deck-config`
Expected: PASS.

- [ ] **Step 7: Run the full marketing suite**

Run: `npm test`
Expected: PASS with zero failures (existing suites plus the two new deck test files).

- [ ] **Step 8: Commit**

```bash
git add marketing/netlify.toml marketing/public/_headers marketing/public/robots.txt marketing/__tests__/deck-config.test.ts
git commit -m "feat(marketing): route unlisted deck at /deck with noindex"
```

---

### Task 3: Build verification and docs

**Files:**
- Modify: `marketing/BACKLOG.md` (move/mark the deck-hosting item Shipped, if present) — optional, only if an item exists.

- [ ] **Step 1: Verify the production build includes the deck**

Run (from `marketing/`):
```bash
npm run build
ls dist/deck/deck.html dist/deck/penny-avatar.png dist/deck/handwritten-list.png dist/deck/logo.svg
```
Expected: all four files exist (Vite copies `public/` into `dist/`).

- [ ] **Step 2: Verify the built deck is gate-free and noindex**

Run:
```bash
grep -ciE "thriftly|/api/|localStorage|sendCode" dist/deck/deck.html   # expect 0
grep -c "noindex" dist/deck/deck.html                                   # expect >= 1
```
Expected: first command prints `0`, second prints `1` or more.

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
- noindex meta (in HTML) + `X-Robots-Tag` header (Netlify) → Task 1 step 1, Task 2 steps 3–4. ✓
- `/deck` clean-URL redirect before SPA catch-all → Task 2 step 3 (+ ordering test). ✓
- robots.txt Disallow, sitemap unchanged, no nav link → Task 2 step 5 + tests; no nav edit anywhere. ✓
- Tests with ≥1.3× ratio (two fs-based test files, small config/content delta) → Tasks 1–2. ✓
- Original `deck/` app untouched → not modified by any task. ✓

**Placeholder scan:** No TBD/TODO; every code/test block is complete; removal targets are anchored on exact unique strings.

**Type consistency:** Netlify config keys match the existing `netlify.toml` shape (`[[redirects]]` with `from`/`to`/`status`; `[[headers]]` with `for` + `[headers.values]`) and the `public/_headers` two-line path/rule format. Tests are pure fs reads via vitest (present on `main`), needing no Express/supertest harness.
