import type React from 'react'

// =============================================================================
// DESIGN SYSTEM — colors, typography, and shared style constants
//
// Theme support: colors are defined as CSS custom properties via injectThemeVars().
// The exported constants here are static fallbacks for light mode. Components
// that need dark mode reactivity should use the CSS var versions directly.
// =============================================================================

// Light theme values (also used as static fallbacks)
export const BRAND = '#35644F'
export const BRAND_LIGHT = '#35644F15'
export const BG = '#F7F6F3'
export const TEXT = '#2A2A2A'
export const TEXT_SECONDARY = '#6B6B6B'
export const BORDER = '#E0DDD6'
export const WHITE = '#FFFFFF'

// Error / destructive
export const ERROR_RED = '#C53030'
export const ERROR_RED_DARK = '#991B1B'
export const ERROR_RED_BG = '#FEF2F2'
export const ERROR_RED_LIGHT = '#FEE2E2'

// Success / positive
export const SUCCESS_GREEN = '#276749'
export const SUCCESS_GREEN_BG = '#F0FFF4'
export const SUCCESS_GREEN_LIGHT = '#27674940'

// Neutral / skeleton
export const SKELETON = '#E8E6E1'
export const SKELETON_SHIMMER = '#F2F1ED'

// =============================================================================
// DARK MODE — CSS custom properties
// =============================================================================

export const THEME_CSS = `
  :root {
    --brand: #35644F;
    --bg: #F7F6F3;
    --text: #2A2A2A;
    --text-secondary: #6B6B6B;
    --text-tertiary: #999999;
    --border: #E0DDD6;
    --surface: #FFFFFF;
    --error: #C53030;
    --error-bg: #FEF2F2;
    --success: #276749;
    --success-bg: #F0FFF4;
    --warning: #D97706;
    --warning-bg: #FEF3C7;
    --skeleton: #E8E6E1;
    --skeleton-shimmer: #F2F1ED;
    --penny-bg: #F5F3EF;
    color-scheme: light;
  }

  [data-theme="dark"] {
    --brand: #4a9470;
    --bg: #121212;
    --text: #E8E6E1;
    --text-secondary: #9CA3AF;
    --text-tertiary: #666666;
    --border: #2D2D2D;
    --surface: #1E1E1E;
    --error: #F87171;
    --error-bg: #2D1515;
    --success: #4ADE80;
    --success-bg: #14291A;
    --warning: #F59E0B;
    --warning-bg: #331F12;
    --skeleton: #2D2D2D;
    --skeleton-shimmer: #3A3A3A;
    --penny-bg: #1A1A1A;
    color-scheme: dark;
  }
`

export function getTheme(): 'light' | 'dark' {
  if (typeof window === 'undefined') return 'light'
  const stored = localStorage.getItem('thriftly_theme')
  if (stored === 'dark' || stored === 'light') return stored
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function setTheme(theme: 'light' | 'dark') {
  localStorage.setItem('thriftly_theme', theme)
  document.documentElement.setAttribute('data-theme', theme)
  // Update meta theme-color for browser chrome
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', theme === 'dark' ? '#121212' : '#35644F')
}

export function initTheme() {
  const theme = getTheme()
  document.documentElement.setAttribute('data-theme', theme)
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', theme === 'dark' ? '#121212' : '#35644F')
}

// Border radius scale
export const RADIUS_SM = '8px'
export const RADIUS_MD = '10px'
export const RADIUS_LG = '12px'
export const RADIUS_XL = '14px'
export const RADIUS_PILL = '999px'

// Shadows
export const SHADOW_SM = '0 1px 3px rgba(0,0,0,0.08)'
export const SHADOW_MD = '0 2px 8px rgba(0,0,0,0.12)'
export const SHADOW_LG = '0 4px 20px rgba(0,0,0,0.15)'

// Disabled / touch
export const OPACITY_DISABLED = 0.4
export const TOUCH_MIN = '44px'

// Animation durations
export const DURATION_FAST = '150ms'
export const DURATION_NORMAL = '250ms'
export const DURATION_SLOW = '350ms'

// Z-index scale
export const Z_DROPDOWN = 100
export const Z_STICKY = 200
export const Z_OVERLAY = 9000
export const Z_MODAL = 9500
export const Z_TOAST = 9999

// Avenir with system fallbacks for platforms that don't have it.
export const FONT = "'Avenir', 'Avenir Next', -apple-system, 'Segoe UI', sans-serif"

export const DEPARTMENTS = [
  'Produce',
  'Bakery & Deli',
  'Meat & Seafood',
  'Dairy & Eggs',
  'Frozen',
  'Pantry & Dry Goods',
  'Snacks & Candy',
  'Beverages',
  'Household & Cleaning',
  'Health & Personal Care',
  'Other',
]

export const DEPT_ICONS: Record<string, string> = {
  'Produce': '🥬',
  'Bakery & Deli': '🥖',
  'Meat & Seafood': '🥩',
  'Dairy & Eggs': '🥛',
  'Frozen': '❄️',
  'Pantry & Dry Goods': '🫙',
  'Snacks & Candy': '🍿',
  'Beverages': '☕',
  'Household & Cleaning': '🧹',
  'Health & Personal Care': '💊',
  'Other': '🛒',
}

export const CONFETTI_COLORS = [
  '#35644F', '#4a9470', '#FF6B6B', '#FFD93D', '#6BCB77', '#4D96FF', '#FF6F91',
]

// Utility: format phone digits as (555) 123-4567
export function formatPhone(digits: string): string {
  if (digits.length === 0) return ''
  if (digits.length <= 3) return `(${digits}`
  if (digits.length <= 6) return `(${digits.slice(0, 3)}) ${digits.slice(3)}`
  return `(${digits.slice(0, 3)}) ${digits.slice(3, 6)}-${digits.slice(6, 10)}`
}

// Page-level layout styles shared across screens
// Uses CSS custom properties (var(--x)) for dark mode reactivity.
export const pageStyles = {
  page: {
    height: '100dvh',
    display: 'flex',
    flexDirection: 'column',
    backgroundColor: 'var(--bg)',
    fontFamily: FONT,
    overflow: 'hidden',
  } as React.CSSProperties,

  listArea: {
    flex: 1,
    overflowY: 'auto' as const,
    overscrollBehavior: 'none',
    WebkitOverflowScrolling: 'touch',
    position: 'relative' as const,
  } as React.CSSProperties,

  headerWordmark: {
    fontFamily: FONT,
    fontSize: '22px',
    fontWeight: 700,
    color: 'var(--brand)',
    letterSpacing: '-0.5px',
  } as React.CSSProperties,

  floatingInput: {
    width: '100%',
    padding: '12px 16px',
    fontSize: '16px',
    fontFamily: FONT,
    color: 'var(--text)',
    backgroundColor: 'var(--surface)',
    border: '1.5px solid var(--border)',
    borderRadius: '12px',
    outline: 'none',
    boxSizing: 'border-box' as const,
  } as React.CSSProperties,

  divider: {
    height: '1px',
    backgroundColor: 'var(--border)',
  } as React.CSSProperties,

  settingsCard: {
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
    padding: '12px 16px',
    backgroundColor: 'var(--bg)',
    borderRadius: '12px',
    cursor: 'pointer',
  } as React.CSSProperties,
}

// Shared style objects used across multiple screens
export const listStyles = {
  overlay: {
    position: 'fixed' as const,
    inset: 0,
    zIndex: 9000,
    backgroundColor: 'rgba(0,0,0,0.4)',
    display: 'flex',
    alignItems: 'flex-end',
    justifyContent: 'center',
  },
  sheet: {
    width: '100%',
    maxWidth: '480px',
    backgroundColor: 'var(--surface)',
    borderRadius: '20px 20px 0 0',
    padding: '16px 20px 40px',
    maxHeight: '82svh',
    overflowY: 'auto' as const,
    display: 'flex',
    flexDirection: 'column' as const,
  },
  sheetTitle: {
    fontFamily: FONT,
    fontSize: '17px',
    fontWeight: 600,
    color: 'var(--text)',
    marginBottom: '16px',
    textAlign: 'center' as const,
    flexShrink: 0,
  },
  sheetButton: {
    width: '100%',
    padding: '15px',
    fontSize: '16px',
    fontFamily: FONT,
    fontWeight: 600,
    color: '#FFFFFF',
    backgroundColor: 'var(--brand)',
    border: 'none',
    borderRadius: '12px',
    cursor: 'pointer',
  } as React.CSSProperties,
}

// Auth screen shared styles
export const authStyles = {
  error: {
    fontFamily: FONT,
    fontSize: '15px',
    color: 'var(--error)',
    margin: '0 0 12px 0',
    fontWeight: 500,
  } as React.CSSProperties,

  subtleLink: {
    fontFamily: FONT,
    fontSize: '13px',
    color: 'var(--text-secondary)',
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    padding: '4px 0',
    textDecoration: 'none',
  } as React.CSSProperties,

  page: {
    backgroundColor: 'var(--bg)',
    fontFamily: FONT,
    minHeight: '100%',
    display: 'flex',
    flexDirection: 'column' as const,
    alignItems: 'center',
    padding: '32px 20px 40px',
    boxSizing: 'border-box' as const,
  } as React.CSSProperties,

  pageCenter: {
    backgroundColor: 'var(--bg)',
    fontFamily: FONT,
    minHeight: '100%',
    display: 'flex',
    flexDirection: 'column' as const,
    alignItems: 'center',
    justifyContent: 'center',
    padding: '32px 20px 40px',
    boxSizing: 'border-box' as const,
  } as React.CSSProperties,

  inner: {
    width: '100%',
    maxWidth: '400px',
    display: 'flex',
    flexDirection: 'column' as const,
    alignItems: 'center',
  } as React.CSSProperties,

  innerFlex: {
    width: '100%',
    maxWidth: '400px',
    display: 'flex',
    flexDirection: 'column' as const,
    alignItems: 'center',
    flex: 1,
  } as React.CSSProperties,

  primaryButton: {
    width: '100%',
    padding: '17px',
    fontSize: '16px',
    fontFamily: FONT,
    fontWeight: 700,
    color: '#FFFFFF',
    backgroundColor: 'var(--brand)',
    border: 'none',
    borderRadius: '14px',
    marginBottom: '16px',
  } as React.CSSProperties,

  dividerRow: {
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
    width: '100%',
  } as React.CSSProperties,

  dividerLine: {
    flex: 1,
    height: '1px',
    backgroundColor: 'var(--border)',
  } as React.CSSProperties,

  dividerText: {
    fontFamily: FONT,
    fontSize: '13px',
    color: 'var(--text-secondary)',
  } as React.CSSProperties,
}
