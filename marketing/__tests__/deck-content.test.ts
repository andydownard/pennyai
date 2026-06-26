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
  it('has no leftover analytics references', () => {
    expect(h()).not.toContain('trackView')
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
