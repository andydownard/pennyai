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
export const TEXT_SECONDARY = '#525252'
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
    --text-secondary: #525252;
    --text-tertiary: #707070;
    --border: #E0DDD6;
    --surface: #FFFFFF;
    --error: #C53030;
    --error-bg: #FEF2F2;
    --success: #276749;
    --success-bg: #F0FFF4;
    --warning: #92400E;
    --warning-bg: #FEF3C7;
    --info: #1E40AF;
    --info-bg: #DBEAFE;
    --neutral-bg: #E5E7EB;
    --skeleton: #E8E6E1;
    --skeleton-shimmer: #F2F1ED;
    --penny-bg: #F5F3EF;
    color-scheme: light;
  }

  [data-theme="dark"] {
    --brand: #377256;
    --bg: #121212;
    --text: #E8E6E1;
    --text-secondary: #9CA3AF;
    --text-tertiary: #8A8A8A;
    --border: #2D2D2D;
    --surface: #1E1E1E;
    --error: #F87171;
    --error-bg: #2D1515;
    --success: #4ADE80;
    --success-bg: #14291A;
    --warning: #F59E0B;
    --warning-bg: #331F12;
    --info: #93C5FD;
    --info-bg: #1E3A5F;
    --neutral-bg: #374151;
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

// Font size scale (rem-based for system font size scaling)
// Base: 1rem = 16px at default browser settings
export const FONT_2XS = '0.75rem'    // 12px
export const FONT_XS = '0.8125rem'   // 13px
export const FONT_SM = '0.875rem'    // 14px
export const FONT_BASE = '0.9375rem' // 15px
export const FONT_MD = '1rem'        // 16px
export const FONT_LG = '1.0625rem'   // 17px
export const FONT_XL = '1.375rem'    // 22px
export const FONT_2XL = '1.5rem'     // 24px
export const FONT_3XL = '2rem'       // 32px

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
  '#35644F', '#377256', '#FF6B6B', '#FFD93D', '#6BCB77', '#4D96FF', '#FF6F91',
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
    fontSize: FONT_XL,
    fontWeight: 700,
    color: 'var(--brand)',
    letterSpacing: '-0.5px',
  } as React.CSSProperties,

  floatingInput: {
    width: '100%',
    padding: '12px 16px',
    fontSize: FONT_MD,
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
    fontSize: FONT_LG,
    fontWeight: 600,
    color: 'var(--text)',
    marginBottom: '16px',
    textAlign: 'center' as const,
    flexShrink: 0,
  },
  sheetButton: {
    width: '100%',
    padding: '15px',
    fontSize: FONT_MD,
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
    fontSize: FONT_BASE,
    color: 'var(--error)',
    margin: '0 0 12px 0',
    fontWeight: 500,
  } as React.CSSProperties,

  subtleLink: {
    fontFamily: FONT,
    fontSize: FONT_XS,
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
    fontSize: FONT_MD,
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
    fontSize: FONT_XS,
    color: 'var(--text-secondary)',
  } as React.CSSProperties,
}

// =============================================================================
// LAYOUT UTILITIES — reusable flex and positioning patterns
// =============================================================================

export const layout = {
  /** display: flex, alignItems: center */
  row: {
    display: 'flex',
    alignItems: 'center',
  } as React.CSSProperties,

  /** display: flex, alignItems: center, gap: 8px */
  rowGap: {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
  } as React.CSSProperties,

  /** display: flex, alignItems: center, justifyContent: space-between */
  rowBetween: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
  } as React.CSSProperties,

  /** display: flex, alignItems: center, justifyContent: center */
  rowCenter: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  } as React.CSSProperties,

  /** display: flex, flexDirection: column */
  column: {
    display: 'flex',
    flexDirection: 'column',
  } as React.CSSProperties,

  /** display: flex, flexDirection: column, gap: 8px */
  columnGap: {
    display: 'flex',
    flexDirection: 'column',
    gap: '8px',
  } as React.CSSProperties,

  /** display: flex, flexDirection: column, alignItems: center */
  columnCenter: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
  } as React.CSSProperties,
}

// =============================================================================
// BUTTON UTILITIES — reusable button reset and common button patterns
// =============================================================================

export const buttons = {
  /** Reset: background none, border none, cursor pointer, padding 0 */
  reset: {
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    padding: 0,
  } as React.CSSProperties,

  /** Icon button: reset + flex center + 44x44 touch target */
  icon: {
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    padding: 0,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    minWidth: TOUCH_MIN,
    minHeight: TOUCH_MIN,
  } as React.CSSProperties,

  /** Primary CTA: brand bg, white text, full width */
  primary: {
    width: '100%',
    padding: '15px',
    fontSize: FONT_MD,
    fontFamily: FONT,
    fontWeight: 600,
    color: '#FFFFFF',
    backgroundColor: 'var(--brand)',
    border: 'none',
    borderRadius: RADIUS_LG,
    cursor: 'pointer',
  } as React.CSSProperties,

  /** Destructive: outlined error color */
  destructive: {
    width: '100%',
    padding: '13px',
    fontSize: FONT_BASE,
    fontFamily: FONT,
    fontWeight: 500,
    color: 'var(--error)',
    backgroundColor: 'transparent',
    border: '1px solid color-mix(in srgb, var(--error) 25%, transparent)',
    borderRadius: RADIUS_LG,
    cursor: 'pointer',
  } as React.CSSProperties,

  /** Pill / tag: small rounded button */
  pill: {
    padding: '6px 14px',
    fontSize: FONT_SM,
    fontFamily: FONT,
    fontWeight: 500,
    borderRadius: RADIUS_PILL,
    border: 'none',
    cursor: 'pointer',
  } as React.CSSProperties,
}

// =============================================================================
// TEXT UTILITIES — common typography patterns
// =============================================================================

export const text = {
  /** Section heading: semibold, normal size */
  heading: {
    fontFamily: FONT,
    fontSize: FONT_LG,
    fontWeight: 600,
    color: 'var(--text)',
  } as React.CSSProperties,

  /** Body text: normal weight, base size */
  body: {
    fontFamily: FONT,
    fontSize: FONT_BASE,
    color: 'var(--text)',
  } as React.CSSProperties,

  /** Secondary/caption text: smaller, muted */
  secondary: {
    fontFamily: FONT,
    fontSize: FONT_SM,
    color: 'var(--text-secondary)',
  } as React.CSSProperties,

  /** Small caption: extra small, muted */
  caption: {
    fontFamily: FONT,
    fontSize: FONT_2XS,
    color: 'var(--text-secondary)',
  } as React.CSSProperties,

  /** Label: small, semibold */
  label: {
    fontFamily: FONT,
    fontSize: FONT_SM,
    fontWeight: 600,
    color: 'var(--text)',
  } as React.CSSProperties,

  /** Error message */
  error: {
    fontFamily: FONT,
    fontSize: FONT_BASE,
    color: 'var(--error)',
    fontWeight: 500,
  } as React.CSSProperties,
}

// =============================================================================
// CARD/CONTAINER UTILITIES
// =============================================================================

export const containers = {
  /** Standard card: surface bg, rounded, padded */
  card: {
    backgroundColor: 'var(--surface)',
    borderRadius: RADIUS_LG,
    padding: '16px',
  } as React.CSSProperties,

  /** Card with border */
  cardBordered: {
    backgroundColor: 'var(--surface)',
    borderRadius: RADIUS_LG,
    padding: '16px',
    border: '1px solid var(--border)',
  } as React.CSSProperties,

  /** Overlay backdrop */
  overlay: {
    position: 'fixed' as const,
    inset: 0,
    backgroundColor: 'rgba(0,0,0,0.4)',
    display: 'flex',
    alignItems: 'flex-end',
    justifyContent: 'center',
    zIndex: Z_OVERLAY,
  } as React.CSSProperties,

  /** Bottom sheet */
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
  } as React.CSSProperties,

  /** Input field: standard text input */
  input: {
    width: '100%',
    padding: '12px 16px',
    fontSize: FONT_MD,
    fontFamily: FONT,
    color: 'var(--text)',
    backgroundColor: 'var(--surface)',
    border: '1.5px solid var(--border)',
    borderRadius: RADIUS_LG,
    outline: 'none',
    boxSizing: 'border-box' as const,
  } as React.CSSProperties,
}
