import { describe, it, expect, beforeEach, vi } from 'vitest'
import { getTheme } from './styles'

describe('getTheme', () => {
  beforeEach(() => localStorage.clear())

  it('defaults to light even when the device prefers dark', () => {
    vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('dark'), media: q, addEventListener() {}, removeEventListener() {} }))
    expect(getTheme()).toBe('light')
    vi.unstubAllGlobals()
  })

  it('keeps dark when the user chose it', () => {
    localStorage.setItem('groflo_theme', 'dark')
    expect(getTheme()).toBe('dark')
  })
})
