// =============================================================================
// DESIGN SYSTEM — colors, typography, and shared style constants
// =============================================================================

export const BRAND = '#35644F'
export const BG = '#F7F6F3'
export const TEXT = '#2A2A2A'
export const TEXT_SECONDARY = '#6B6B6B'
export const BORDER = '#E0DDD6'
export const ERROR_RED = '#C53030'
export const WHITE = '#FFFFFF'

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
  '#35644F', '#4CAF50', '#FF6B6B', '#FFD93D', '#6BCB77', '#4D96FF', '#FF6F91',
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
