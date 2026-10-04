import { describe, it, expect, afterEach } from 'vitest'
import { signInRedirectTarget } from './signInRedirect'

const original = window.location.href

function setHref(href: string) {
  Object.defineProperty(window, 'location', {
    value: new URL(href),
    writable: true,
    configurable: true,
  })
}

describe('signInRedirectTarget', () => {
  afterEach(() => setHref(original))

  it('returns the current href unchanged when there is no fragment', () => {
    setHref('https://app.groflo.io/join/ABC123')
    expect(signInRedirectTarget()).toBe('https://app.groflo.io/join/ABC123')
  })

  it('preserves the query string — deep links must round-trip', () => {
    setHref('https://app.groflo.io/r/XYZ?src=sms&n=2')
    expect(signInRedirectTarget()).toBe('https://app.groflo.io/r/XYZ?src=sms&n=2')
  })

  it('strips the fragment so nothing after # reaches the primary', () => {
    setHref('https://app.groflo.io/settings#secret-anchor')
    expect(signInRedirectTarget()).toBe('https://app.groflo.io/settings')
  })

  it('strips the fragment but keeps the query', () => {
    setHref('https://app.groflo.io/join/ABC?x=1#frag')
    expect(signInRedirectTarget()).toBe('https://app.groflo.io/join/ABC?x=1')
  })

  it('handles a bare fragment marker', () => {
    setHref('https://app.groflo.io/#')
    expect(signInRedirectTarget()).toBe('https://app.groflo.io/')
  })
})
