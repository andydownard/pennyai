import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import path from 'node:path'

const ROOT = path.resolve(__dirname, '..')
const read = (rel: string) => readFileSync(path.join(ROOT, rel), 'utf8')

// The marketing site is served on Railway by `serve` (the `serve` npm package),
// NOT Netlify. `serve --single` rewrites EVERY non-file path to the SPA
// index.html, and a `**` catch-all rewrite re-catches the rewritten /deck. So
// the deck is routed by enumerating the known SPA routes in public/serve.json
// (no catch-all) plus an explicit /deck carve-out, and dropping `-s` from the
// start command. These tests lock that contract in.

type Rewrite = { source: string; destination: string }
type HeaderRule = { source: string; headers: { key: string; value: string }[] }

describe('public/serve.json — deck routing under `serve`', () => {
  const cfg = () => JSON.parse(read('public/serve.json')) as { rewrites: Rewrite[]; headers: HeaderRule[] }

  it('rewrites the clean /deck URL to the deck file', () => {
    const r = cfg().rewrites.find((x) => x.source === '/deck')
    expect(r?.destination).toBe('/deck/index.html')
  })

  it('rewrites /deck/ to the deck file too', () => {
    const r = cfg().rewrites.find((x) => x.source === '/deck/')
    expect(r?.destination).toBe('/deck/index.html')
  })

  it('routes every known SPA path to index.html so client-side routing works', () => {
    const rewrites = cfg().rewrites
    for (const p of ['/for-brands', '/pricing', '/campaign/signup', '/about', '/privacy', '/terms']) {
      expect(rewrites.find((x) => x.source === p)?.destination).toBe('/index.html')
    }
  })

  it('has NO catch-all rewrite (a ** rule would cumulatively re-catch /deck and break it)', () => {
    const sources = cfg().rewrites.map((x) => x.source)
    expect(sources).not.toContain('**')
    expect(sources).not.toContain('/**')
    expect(sources).not.toContain('/*')
  })

  it('sets a noindex X-Robots-Tag header for the deck', () => {
    const hdr = cfg().headers.find((h) => h.source.includes('deck'))
    expect(hdr).toBeTruthy()
    const flat = JSON.stringify(hdr!.headers).toLowerCase()
    expect(flat).toContain('x-robots-tag')
    expect(flat).toContain('noindex')
  })
})

describe('railway.toml — marketing start command', () => {
  it('serves without single-page mode so /deck is not rewritten to the SPA', () => {
    const toml = read('railway.toml')
    expect(toml).toMatch(/startCommand\s*=\s*"npx serve dist -l \$PORT"/)
    expect(toml).not.toMatch(/serve dist -s/)
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
