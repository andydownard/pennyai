import type React from 'react'

// =============================================================================
// DESIGN SYSTEM — colors, typography, and shared style constants
//
// Theme support: colors are defined as CSS custom properties via injectThemeVars().
// The exported constants here are static fallbacks for light mode. Components
// that need to follow the theme should use the CSS var versions directly.
// =============================================================================

// Light theme values (also used as static fallbacks)
export const BRAND = '#35644F'

// Error / destructive

// Success / positive

// Neutral / skeleton

// =============================================================================
// THEME — CSS custom properties (light only)
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
    --penny-bg: #EFECE5;
    --checkbox-border: #6B7280;
    color-scheme: light;
  }
`

// Groflo is light only. Dark mode was removed: drop any choice saved while it
// existed, so nobody is left holding a stale setting.
export function initTheme() {
  try {
    localStorage.removeItem('groflo_theme')
  } catch {
    // Storage blocked: nothing to forget.
  }
  document.documentElement.removeAttribute('data-theme')
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', '#35644F')
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

// Border radius scale
export const RADIUS_LG = '12px'
const RADIUS_PILL = '999px'

// Shadows
export const SHADOW_SM = '0 1px 3px rgba(0,0,0,0.08)'
export const SHADOW_LG = '0 4px 20px rgba(0,0,0,0.15)'

// Disabled / touch
const TOUCH_MIN = '44px'

// Animation durations

// Z-index scale
export const Z_MODAL = 9500

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

// Item-level emoji mapping — used when brand logo is unavailable or AI emoji is missing
// Keys are lowercase substrings matched against item name (most specific first)
// KEEP IN SYNC with inferItemEmoji() in db.go
const ITEM_EMOJIS: [string, string][] = [
  // Produce — fruits
  ['apple', '🍎'], ['banana', '🍌'], ['orange', '🍊'], ['lemon', '🍋'], ['lime', '🍋'],
  ['grape', '🍇'], ['strawberr', '🍓'], ['blueberr', '🫐'], ['raspberry', '🫐'], ['blackberr', '🫐'],
  ['cranberr', '🫐'], ['cherry', '🍒'], ['peach', '🍑'], ['nectarine', '🍑'], ['apricot', '🍑'],
  ['plum', '🍑'], ['pear', '🍐'], ['watermelon', '🍉'], ['melon', '🍈'],
  ['pineapple', '🍍'], ['mango', '🥭'], ['papaya', '🥭'], ['guava', '🥭'],
  ['kiwi', '🥝'], ['coconut', '🥥'], ['avocado', '🥑'], ['tomato', '🍅'],
  ['grapefruit', '🍊'], ['tangerine', '🍊'], ['clementine', '🍊'], ['pomegranate', '🍎'],
  ['fig', '🍇'], ['date', '🍇'],
  ['dragon fruit', '🐉'], ['passion fruit', '🍑'], ['starfruit', '⭐'], ['persimmon', '🍊'],
  ['lychee', '🍑'], ['jackfruit', '🍈'], ['plantain', '🍌'],
  // Produce — vegetables
  ['broccoli', '🥦'], ['cauliflower', '🥦'], ['carrot', '🥕'], ['corn', '🌽'],
  ['jalapeño', '🌶️'], ['jalapeno', '🌶️'], ['habanero', '🌶️'], ['pepper', '🌶️'],
  ['cucumber', '🥒'], ['zucchini', '🥒'], ['squash', '🥒'],
  ['lettuce', '🥬'], ['romaine', '🥬'], ['arugula', '🥬'], ['cabbage', '🥬'],
  ['spinach', '🥬'], ['kale', '🥬'], ['chard', '🥬'], ['collard', '🥬'],
  ['cilantro', '🌿'], ['parsley', '🌿'], ['basil', '🌿'], ['mint', '🌿'], ['dill', '🌿'],
  ['rosemary', '🌿'], ['thyme', '🌿'], ['oregano', '🌿'], ['sage', '🌿'], ['herb', '🌿'],
  ['salad', '🥗'], ['sweet potato', '🍠'], ['yam', '🍠'], ['potato', '🥔'],
  ['onion', '🧅'], ['shallot', '🧅'], ['scallion', '🧅'], ['leek', '🧅'], ['chive', '🧅'],
  ['garlic', '🧄'], ['ginger', '🫚'], ['mushroom', '🍄'], ['eggplant', '🍆'], ['celery', '🥬'],
  ['asparagus', '🥬'], ['artichoke', '🥬'], ['beet', '🥬'], ['turnip', '🥬'], ['radish', '🥬'],
  ['bok choy', '🥬'], ['napa cabbage', '🥬'], ['daikon', '🥕'],
  ['jicama', '🥔'], ['taro', '🥔'], ['yuca', '🥔'],
  ['green bean', '🟢'], ['snap pea', '🟢'], ['edamame', '🫘'], ['pea', '🟢'],
  // Dairy & Eggs
  ['milky way', '🍫'],
  ['half and half', '🥛'], ['half & half', '🥛'], ['almond milk', '🥛'], ['oat milk', '🥛'],
  ['soy milk', '🥛'], ['coconut milk', '🥥'], ['milk', '🥛'], ['cream', '🥛'],
  ['cream cheese', '🧀'], ['cottage cheese', '🧀'], ['goat cheese', '🧀'],
  ['cheddar', '🧀'], ['mozzarella', '🧀'], ['parmesan', '🧀'], ['gouda', '🧀'],
  ['brie', '🧀'], ['feta', '🧀'], ['gruyere', '🧀'], ['provolone', '🧀'],
  ['colby', '🧀'], ['swiss', '🧀'], ['jack', '🧀'], ['ricotta', '🧀'], ['cheese', '🧀'],
  ['butter', '🧈'], ['margarine', '🧈'], ['egg', '🥚'],
  ['yogurt', '🫙'], ['kefir', '🫙'],
  ['ice cream', '🍦'], ['gelato', '🍦'], ['sorbet', '🍦'], ['popsicle', '🍦'],
  // Meat & Seafood
  ['rotisserie', '🍗'], ['chicken', '🍗'], ['thigh', '🍗'], ['drumstick', '🍗'], ['wing', '🍗'],
  ['ribeye', '🥩'], ['sirloin', '🥩'], ['filet', '🥩'], ['brisket', '🥩'],
  ['beef', '🥩'], ['steak', '🥩'], ['ground', '🥩'], ['roast', '🥩'],
  ['pork', '🥩'], ['chop', '🥩'], ['tenderloin', '🥩'], ['ribs', '🥩'],
  ['bacon', '🥓'], ['ham', '🥓'], ['prosciutto', '🥓'], ['pancetta', '🥓'],
  ['sausage', '🌭'], ['hot dog', '🌭'], ['bratwurst', '🌭'], ['kielbasa', '🌭'],
  ['turkey', '🦃'], ['lamb', '🥩'], ['veal', '🥩'], ['venison', '🥩'], ['bison', '🥩'],
  ['deli meat', '🥩'], ['lunch meat', '🥩'], ['jerky', '🥩'],
  ['fish sauce', '🐟'], ['fish food', '🐠'], ['salmon', '🐟'], ['tuna', '🐟'], ['cod', '🐟'],
  ['tilapia', '🐟'], ['trout', '🐟'], ['halibut', '🐟'], ['mahi', '🐟'],
  ['swordfish', '🐟'], ['catfish', '🐟'], ['anchovy', '🐟'], ['fish', '🐟'],
  ['shrimp', '🦐'], ['prawn', '🦐'], ['crab', '🦀'], ['lobster', '🦞'],
  ['scallop', '🦪'], ['oyster sauce', '🦪'], ['oyster', '🦪'], ['clam', '🦪'], ['mussel', '🦪'],
  // Deli & Prepared
  ['hummus', '🫘'], ['guacamole', '🥑'], ['salsa', '🫙'],
  ['sandwich', '🥪'], ['deli', '🥪'], ['burrito', '🌯'], ['wrap', '🌯'],
  ['spring roll', '🥟'], ['dumpling', '🥟'], ['wonton', '🥟'], ['samosa', '🥟'],
  ['falafel', '🧆'], ['quesadilla', '🌮'], ['enchilada', '🌮'], ['tamale', '🫔'],
  ['hot pocket', '🍕'], ['pizza roll', '🍕'],
  ['microwave meal', '🍱'], ['frozen dinner', '🍱'], ['tv dinner', '🍱'],
  ['protein bar', '💪'], ['energy bar', '💪'], ['granola bar', '🥜'],
  // Bakery
  ['sourdough', '🍞'], ['baguette', '🥖'], ['ciabatta', '🍞'], ['brioche', '🍞'],
  ['bread', '🍞'], ['roll', '🍞'], ['bun', '🍞'], ['loaf', '🍞'], ['rye', '🍞'],
  ['pita', '🫓'], ['naan', '🫓'], ['tortilla', '🫓'], ['flatbread', '🫓'],
  ['bagel', '🥯'], ['croissant', '🥐'], ['pretzel', '🥨'],
  ['english muffin', '🧇'], ['muffin', '🧁'], ['scone', '🧁'], ['danish', '🧁'], ['pastry', '🥐'],
  ['cake mix', '🎂'], ['cake', '🎂'], ['cupcake', '🧁'], ['pie', '🥧'], ['tart', '🥧'],
  ['cookie', '🍪'], ['brownie', '🍫'], ['biscuit', '🍪'],
  ['donut', '🍩'], ['doughnut', '🍩'],
  ['waffle', '🧇'], ['pancake', '🥞'], ['mochi', '🍡'],
  // Baking
  ['vanilla extract', '🧁'], ['baking soda', '🧁'], ['baking powder', '🧁'],
  ['cornstarch', '🧁'], ['powdered sugar', '🍚'], ['brown sugar', '🍬'],
  ['frosting', '🧁'], ['sprinkles', '🎂'], ['food coloring', '🎨'],
  ['cupcake liner', '🧁'], ['pie crust', '🥧'],
  ['bread flour', '🌾'],
  // Pantry & Dry Goods
  ['jasmine', '🍚'], ['basmati', '🍚'], ['quinoa', '🍚'], ['couscous', '🍚'], ['rice', '🍚'],
  ['spaghetti', '🍝'], ['penne', '🍝'], ['rigatoni', '🍝'], ['fusilli', '🍝'],
  ['linguine', '🍝'], ['fettuccine', '🍝'], ['macaroni', '🍝'], ['rotini', '🍝'],
  ['orzo', '🍝'], ['farfalle', '🍝'], ['angel hair', '🍝'], ['lasagna', '🍝'], ['pasta', '🍝'],
  ['ramen', '🍜'], ['udon', '🍜'], ['soba', '🍜'], ['lo mein', '🍜'], ['noodle', '🍜'],
  ['miso', '🍜'],
  ['cereal', '🥣'], ['oat', '🥣'], ['granola', '🥣'], ['muesli', '🥣'],
  ['chickpea', '🫘'], ['black bean', '🫘'], ['bean', '🫘'], ['lentil', '🫘'],
  ['tofu', '🫘'], ['tempeh', '🫘'], ['seitan', '🌾'],
  ['peanut butter', '🥜'], ['almond butter', '🥜'], ['tahini', '🥜'],
  ['peanut', '🥜'], ['almond', '🥜'], ['walnut', '🥜'], ['pecan', '🥜'],
  ['cashew', '🥜'], ['pistachio', '🥜'], ['nut', '🥜'],
  ['sunflower seed', '🌻'], ['pumpkin seed', '🎃'], ['sunflower', '🌻'],
  ['chia', '🌻'], ['flax', '🌻'], ['seed', '🌻'],
  ['honey', '🍯'], ['maple syrup', '🍯'], ['agave', '🍯'], ['syrup', '🧴'],
  ['jam', '🫙'], ['jelly', '🫙'], ['preserves', '🫙'], ['marmalade', '🫙'],
  ['coconut oil', '🥥'], ['olive oil', '🫒'], ['sesame oil', '🫒'], ['oil', '🫒'], ['olive', '🫒'],
  ['balsamic', '🫙'], ['vinegar', '🫙'],
  // Condiments
  ['relish', '🌶️'], ['horseradish', '🌶️'], ['aioli', '🧄'], ['pesto', '🌿'],
  ['chimichurri', '🌿'], ['chutney', '🥭'], ['tzatziki', '🥒'], ['tapenade', '🫒'], ['capers', '🫒'],
  ['worcestershire', '🥫'], ['hoisin', '🥫'],
  ['harissa', '🌶️'], ['gochujang', '🌶️'], ['curry paste', '🍛'],
  ['sriracha', '🌶️'], ['hot sauce', '🌶️'], ['tabasco', '🌶️'], ['buffalo sauce', '🌶️'],
  ['soy sauce', '🥫'], ['teriyaki', '🥫'], ['bbq sauce', '🥫'], ['barbecue', '🥫'],
  ['sauce', '🫙'],
  ['ketchup', '🫙'], ['mustard', '🫙'], ['mayo', '🫙'], ['mayonnaise', '🫙'],
  ['dressing', '🫙'], ['ranch', '🫙'],
  ['cumin', '🧂'], ['paprika', '🧂'], ['cinnamon', '🧂'], ['nutmeg', '🧂'], ['turmeric', '🧂'],
  ['seasoning', '🧂'], ['spice', '🧂'], ['salt', '🧂'],
  ['flour', '🌾'], ['sugar', '🧂'], ['baking', '🧁'], ['yeast', '🍞'],
  ['soup', '🍲'], ['broth', '🍲'], ['stock', '🍲'], ['chili', '🍲'], ['stew', '🍲'],
  ['tomato sauce', '🥫'], ['tomato paste', '🥫'], ['can of', '🥫'], ['canned', '🥫'],
  ['taco', '🌮'], ['tortilla chip', '🌮'],
  // International / Specialty
  ['kimchi', '🥬'], ['sauerkraut', '🥬'], ['wasabi', '🟢'],
  ['nori', '🍣'], ['seaweed', '🍣'],
  // Snacks
  ['chip', '🍟'], ['popcorn', '🍿'], ['cracker', '🍘'],
  ['chocolate', '🍫'], ['candy', '🍬'], ['gum', '🍬'], ['gummy', '🍬'],
  ['trail mix', '🥜'], ['fruit snack', '🍬'], ['dried fruit', '🍇'], ['raisin', '🍇'],
  ['cranberry', '🍒'],
  // Beverages
  ['sparkling water', '💧'], ['seltzer', '💧'], ['tonic', '💧'],
  ['la croix', '💧'], ['lacroix', '💧'], ['water', '💧'],
  ['coconut water', '🥥'],
  ['juice', '🧃'], ['oj', '🍊'], ['lemonade', '🍋'],
  ['coke', '🥤'], ['pepsi', '🥤'], ['sprite', '🥤'], ['dr pepper', '🥤'],
  ['ginger ale', '🥤'], ['cola', '🥤'], ['root beer', '🍺'], ['ginger beer', '🍺'],
  ['soda', '🥤'], ['pop', '🥤'],
  ['espresso', '☕'], ['latte', '☕'], ['coffee', '☕'],
  ['matcha', '🍵'], ['chai', '🍵'], ['tea', '🍵'], ['kombucha', '🍵'],
  ['beer', '🍺'], ['ipa', '🍺'], ['lager', '🍺'], ['ale', '🍺'], ['stout', '🍺'],
  ['champagne', '🥂'], ['prosecco', '🍾'], ['wine', '🍷'], ['cider', '🍎'],
  ['whiskey', '🥃'], ['vodka', '🍸'], ['tequila', '🍸'], ['rum', '🍹'], ['gin', '🍸'],
  ['liquor', '🥃'],
  ['protein shake', '💪'], ['smoothie', '🥤'], ['shake', '🥤'],
  ['sports drink', '🥤'], ['energy drink', '⚡'], ['red bull', '⚡'], ['monster', '⚡'],
  ['protein', '💪'],
  // Frozen
  ['frozen', '❄️'], ['pizza', '🍕'],
  // Household & Cleaning
  ['dish soap', '🧼'], ['hand soap', '🧼'], ['soap', '🧼'],
  ['detergent', '🧴'], ['laundry', '🧴'], ['fabric softener', '🧺'], ['dryer sheet', '🧺'],
  ['stain remover', '🧺'],
  ['sponge', '🧽'], ['scrub', '🧽'],
  ['paper towel', '🧻'], ['toilet paper', '🧻'], ['tissue', '🧻'], ['napkin', '🧻'],
  ['trash bag', '🗑️'], ['garbage', '🗑️'],
  ['aluminum foil', '🫕'], ['foil', '🫕'], ['plastic wrap', '🫕'], ['parchment paper', '📜'], ['wax paper', '📜'],
  ['ziplock', '🤐'], ['sandwich bag', '🤐'], ['ziploc', '🛍️'],
  ['bleach', '🧴'], ['disinfectant', '🧴'], ['cleaner', '🧴'], ['wipes', '🧴'],
  ['rubber glove', '🧤'],
  ['mop', '🧹'], ['broom', '🧹'], ['dustpan', '🧹'], ['vacuum bag', '🧹'], ['lint roller', '🧹'],
  ['air freshener', '🌸'], ['light bulb', '💡'], ['battery', '🔋'], ['candle', '🕯️'],
  ['match', '🔥'], ['charcoal', '🔥'], ['propane', '⛽'], ['extension cord', '🔌'],
  ['duct tape', '🛠️'], ['super glue', '🛠️'], ['command strip', '🛠️'],
  ['storage bin', '📦'], ['hanger', '👔'],
  // Health & Personal Care
  ['vitamin', '💊'], ['supplement', '💊'], ['medicine', '💊'], ['tylenol', '💊'], ['advil', '💊'], ['ibuprofen', '💊'],
  ['toothpaste', '🪥'], ['toothbrush', '🪥'], ['floss', '🪥'], ['mouthwash', '🪥'],
  ['shampoo', '🧴'], ['conditioner', '🧴'], ['body wash', '🧴'],
  ['lotion', '🧴'], ['moisturizer', '🧴'], ['sunscreen', '🧴'], ['deodorant', '🧴'],
  ['bandaid', '🩹'], ['band-aid', '🩹'], ['first aid', '🩹'],
  ['razor', '🪒'], ['shaving', '🪒'],
  // Baby
  ['baby food', '🍼'], ['baby cereal', '🍼'], ['baby wipe', '👶'], ['formula', '🍼'],
  ['pacifier', '👶'], ['diaper', '👶'], ['sippy cup', '🥤'], ['bib', '👶'],
  ['teething', '👶'], ['nursing pad', '🤱'], ['baby', '👶'],
  // Pet
  ['dog food', '🐕'], ['dog treat', '🐕'], ['cat food', '🐈'], ['cat litter', '🐱'],
  ['bird seed', '🐦'], ['fish food', '🐠'], ['hamster food', '🐹'], ['rabbit food', '🐰'],
  ['pet treat', '🦴'], ['leash', '🐕'], ['collar', '🐕'], ['flea', '🐕'],
  ['pet shampoo', '🐕'], ['litter', '🐱'], ['chew toy', '🦴'], ['catnip', '🐱'],
  ['pet', '🐾'],
]

/**
 * Check if `lower` contains `kw` using word-boundary matching.
 * For short keywords (<=4 chars), requires start boundary (start or space)
 * and end boundary (end, space, or trailing "s" for plurals like "eggs" matching "egg").
 * For longer keywords (>4 chars), uses simple substring match.
 */
function containsWord(lower: string, kw: string): boolean {
  if (kw.length <= 4) {
    let idx = 0
    while (idx <= lower.length - kw.length) {
      const pos = lower.indexOf(kw, idx)
      if (pos < 0) return false
      const startOK = pos === 0 || lower[pos - 1] === ' '
      const endPos = pos + kw.length
      const endOK = endPos === lower.length || lower[endPos] === ' ' || lower[endPos] === 's'
      if (startOK && endOK) return true
      idx = pos + 1
    }
    return false
  }
  return lower.includes(kw)
}

/** Return an emoji for a grocery item name, falling back to department emoji, then 🛒 */
export function getItemEmoji(itemName: string, department?: string): string {
  const lower = itemName.toLowerCase()
  for (const [keyword, emoji] of ITEM_EMOJIS) {
    if (containsWord(lower, keyword)) return emoji
  }
  if (department && DEPT_ICONS[department]) return DEPT_ICONS[department]
  return '🛒'
}

export const CONFETTI_COLORS = [
  '#35644F', '#377256', '#FF6B6B', '#FFD93D', '#6BCB77', '#4D96FF', '#FF6F91',
]

// Page-level layout styles shared across screens
// Uses CSS custom properties (var(--x)).
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

