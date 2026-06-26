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
  it('sets noindex header for the bare /deck path', () => {
    const t = toml()
    expect(t).toContain('for = "/deck"')
    expect(t).toContain('X-Robots-Tag = "noindex, nofollow"')
    // Verify there are two separate [[headers]] blocks for /deck and /deck/*
    const matches = t.match(/for\s*=\s*"\/deck/g)
    expect(matches).toHaveLength(2)
  })
})

describe('public/_headers — deck noindex', () => {
  it('adds an X-Robots-Tag noindex rule for /deck/*', () => {
    const h = read('public/_headers')
    expect(h).toMatch(/\/deck\/\*/)
    expect(h).toMatch(/X-Robots-Tag:\s*noindex/)
  })
  it('covers the bare /deck path with noindex', () => {
    const h = read('public/_headers')
    expect(h).toMatch(/^\/deck\s*\n\s*X-Robots-Tag:\s*noindex/m)
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
