import { describe, it, expect, beforeEach } from 'vitest'
import { initTheme } from './styles'

describe('initTheme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
  })

  it('forgets a dark choice saved before dark mode was removed', () => {
    localStorage.setItem('groflo_theme', 'dark')
    document.documentElement.setAttribute('data-theme', 'dark')
    initTheme()
    expect(localStorage.getItem('groflo_theme')).toBeNull()
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false)
  })

  it('leaves other saved choices alone', () => {
    localStorage.setItem('groflo_install_dismissed', '1')
    initTheme()
    expect(localStorage.getItem('groflo_install_dismissed')).toBe('1')
  })
})
