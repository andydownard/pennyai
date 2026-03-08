// =============================================================================
// DESIGN SYSTEM — colors, typography, and shared style constants
// =============================================================================

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
    backgroundColor: WHITE,
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
    color: TEXT,
    marginBottom: '16px',
    textAlign: 'center' as const,
    flexShrink: 0,
  },
}
