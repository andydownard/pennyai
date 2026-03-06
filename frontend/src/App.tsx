// =============================================================================
// thriftly — App.tsx
//
// The entire frontend in one React component. Three screens:
//   1. Phone input: user types 10 digits → formats as (555) 123-4567
//   2. Code input: 6 individual boxes, auto-advances, auto-submits
//   3. Home: logged in (placeholder — we'll build the list/chat later)
//
// DESIGN DECISIONS:
//   - All styles are inline (CSS-in-JS objects) to keep everything in one file.
//     In a bigger app you'd use CSS modules or Tailwind. For now, this is
//     simpler to read and modify.
//   - inputMode="numeric" tells mobile browsers to show the number pad.
//   - All API calls go to the Go backend (no logic in the frontend).
//   - Phone numbers are stored as 10 raw digits. The backend adds "+1".
// =============================================================================

import { useState, useRef, useEffect, useCallback, useMemo } from 'react'

// =============================================================================
// TYPES
// =============================================================================

// The screens the app can show
type Screen = 'phone' | 'code' | 'home' | 'saved' | 'settings' | 'about' | 'invite' | 'settings-profile' | 'settings-share' | 'admin'

// Declare gtag for TypeScript
declare function gtag(...args: unknown[]): void

// =============================================================================
// URL ↔ SCREEN MAPPING (for browser back/forward navigation)
// =============================================================================

const SCREEN_PATHS: Record<Screen, string> = {
  phone: '/login',
  code: '/verify',
  home: '/',
  about: '/about',
  'settings-share': '/share',
  settings: '/settings',
  'settings-profile': '/settings/profile',
  saved: '/settings/saved',
  admin: '/admin',
  invite: '/share',
}

const PATH_TO_SCREEN: Record<string, Screen> = {
  '/login': 'phone',
  '/verify': 'code',
  '/': 'home',
  '/about': 'about',
  '/share': 'settings-share',
  '/settings': 'settings',
  '/settings/profile': 'settings-profile',
  '/settings/saved': 'saved',
  '/admin': 'admin',
}

type ListItem = {
  id: number
  name: string
  brand: string
  department: string
  quantity: number
  size: string
  notes: string
  source: string
  checked: boolean
  urgent: boolean
  created_at: string
  added_by_name: string
  added_by_phone: string
  claimed_by_id: number | null
  claimed_by_name: string
}

type SavedList = {
  id: number
  name: string
  item_count: number
  created_at: string
}

type ItemModification = {
  action: string
  detail: string
  source: string
  created_at: string
  user_name: string
}

type ShoppingTrip = {
  id: number
  user_id: number
  user_name: string
  trip_date: string
  trip_time: string
  time_label: string
  is_mine: boolean
  created_at: string
}

// =============================================================================
// CONSTANTS
// =============================================================================

const BRAND = '#35644F'
const BG = '#F7F6F3'
const TEXT = '#2A2A2A'
const TEXT_SECONDARY = '#6B6B6B'
const BORDER = '#E0DDD6'
const ERROR_RED = '#C53030'
const WHITE = '#FFFFFF'

const DEPARTMENTS = [
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

// Department icons — emoji for clarity and cross-platform consistency
const DEPT_ICONS: Record<string, string> = {
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
  'Other': '📦',
}

function DeptIcon({ dept }: { dept: string }) {
  const icon = DEPT_ICONS[dept]
  if (!icon) return null
  return <span style={{ fontSize: '29px', lineHeight: 1, flexShrink: 0 }}>{icon}</span>
}

// Avenir with system fallbacks for platforms that don't have it.
// Avenir comes pre-installed on macOS and iOS.
// "Avenir Next" is the updated version on Apple platforms.
// On Windows/Android, it falls back to system sans-serif.
const FONT = "'Avenir', 'Avenir Next', -apple-system, 'Segoe UI', sans-serif"

// =============================================================================
// HELPER: format 10 raw digits as (555) 123-4567
// =============================================================================

function formatPhone(digits: string): string {
  if (digits.length === 0) return ''
  if (digits.length <= 3) return `(${digits}`
  if (digits.length <= 6) return `(${digits.slice(0, 3)}) ${digits.slice(3)}`
  return `(${digits.slice(0, 3)}) ${digits.slice(3, 6)}-${digits.slice(6, 10)}`
}

// =============================================================================
// LOGO COMPONENT
//
// An SVG that conveys "AI building a shopping list":
//   - A clipboard/notepad shape (the list)
//   - Checkmarks on completed items (the list being built)
//   - Sparkle accents (the AI doing the work)
// =============================================================================

function Logo() {
  return (
    <svg
      width="72"
      height="72"
      viewBox="0 0 72 72"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-label="Thriftly logo"
    >
      {/* Clipboard body */}
      <rect x="12" y="10" width="42" height="52" rx="6" stroke={BRAND} strokeWidth="2.5" fill={WHITE} />

      {/* Clip tab at top center */}
      <rect x="25" y="4" width="16" height="10" rx="4" fill={BRAND} />
      <rect x="29" y="7" width="8" height="4" rx="2" fill={WHITE} />

      {/* List item 1: checked */}
      <path d="M20 27 L23 30 L28 24" stroke={BRAND} strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
      <line x1="33" y1="27" x2="46" y2="27" stroke={BRAND} strokeWidth="2" strokeLinecap="round" opacity="0.7" />

      {/* List item 2: checked */}
      <path d="M20 37 L23 40 L28 34" stroke={BRAND} strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
      <line x1="33" y1="37" x2="42" y2="37" stroke={BRAND} strokeWidth="2" strokeLinecap="round" opacity="0.7" />

      {/* List item 3: being written (partial, lighter) */}
      <circle cx="24" cy="47" r="2" fill={BRAND} opacity="0.3" />
      <line x1="33" y1="47" x2="39" y2="47" stroke={BRAND} strokeWidth="2" strokeLinecap="round" opacity="0.3" />

      {/* AI sparkle — large */}
      <path
        d="M54 14 L56 19 L61 21 L56 23 L54 28 L52 23 L47 21 L52 19 Z"
        fill={BRAND}
      >
        <animateTransform
          attributeName="transform"
          type="rotate"
          from="0 54 21"
          to="360 54 21"
          dur="12s"
          repeatCount="indefinite"
        />
      </path>

      {/* AI sparkle — small */}
      <path
        d="M60 6 L61 9 L64 10 L61 11 L60 14 L59 11 L56 10 L59 9 Z"
        fill={BRAND}
        opacity="0.5"
      />
    </svg>
  )
}

// =============================================================================
// PENNY AVATAR
// =============================================================================

function PennyAvatar({ size = 28 }: { size?: number }) {
  return (
    <div style={{
      width: size,
      height: size,
      borderRadius: '50%',
      overflow: 'hidden',
      flexShrink: 0,
      backgroundColor: '#e8e4df',
      border: `1px solid ${BORDER}`,
    }}>
      <img
        src="/penny.png"
        alt="Penny"
        width={size}
        height={size}
        style={{ objectFit: 'cover', objectPosition: 'center 15%', display: 'block' }}
      />
    </div>
  )
}

// Brand logo with consistent styling — renders empty box when no brand
function BrandLogo({ brand, size = 52, opacity = 1 }: { brand?: string; size?: number; opacity?: number }) {
  const radius = size > 60 ? '12px' : size > 48 ? '10px' : '8px'
  const pad = size > 60 ? '4px' : size > 48 ? '3px' : '2px'
  return (
    <div style={{ width: size, height: size, flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      {brand ? (
        <img
          src={`/api/logo?brand=${encodeURIComponent(brand)}&v=4`}
          alt=""
          width={size}
          height={size}
          loading="lazy"
          style={{ borderRadius: radius, objectFit: 'contain' as const, backgroundColor: '#FAFAFA', border: '1px solid rgba(0,0,0,0.06)', padding: pad, opacity, transition: 'opacity 0.15s' }}
          onError={e => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
      ) : (
        <div style={{ width: size, height: size, borderRadius: radius, backgroundColor: '#FAFAFA', border: '1px solid rgba(0,0,0,0.06)', opacity }} />
      )}
    </div>
  )
}

// Toggle switch — used for urgent and claim toggles
function ToggleSwitch({ value, onChange }: { value: boolean; onChange: () => void }) {
  return (
    <button
      onClick={onChange}
      style={{
        width: '44px', height: '24px', borderRadius: '12px', border: 'none', cursor: 'pointer',
        backgroundColor: value ? BRAND : '#CBD5E0', position: 'relative', transition: 'background-color 0.2s',
        padding: 0, flexShrink: 0,
      }}
    >
      <span style={{
        position: 'absolute', top: '2px', left: value ? '22px' : '2px',
        width: '20px', height: '20px', borderRadius: '50%', backgroundColor: WHITE,
        transition: 'left 0.2s', boxShadow: '0 1px 3px rgba(0,0,0,0.2)',
      }} />
    </button>
  )
}

// Avatar circle for household members and friends
const avatarCircleStyle = (size = 32): React.CSSProperties => ({
  width: `${size}px`, height: `${size}px`, borderRadius: '50%', backgroundColor: BRAND, color: WHITE,
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  fontFamily: FONT, fontSize: '15px', fontWeight: 600, flexShrink: 0,
})

// Quantity stepper button style
const qtyBtnStyle: React.CSSProperties = {
  width: '34px', height: '34px', border: 'none', background: 'none',
  fontSize: '17px', color: BRAND, cursor: 'pointer',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
}

// =============================================================================
// PENNY LOADING MESSAGES
// =============================================================================

const PENNY_STEP1 = ["Reading your list...", "Let me see what you wrote...", "Hmm, looking at this..."]
const PENNY_STEP2 = ["Checking it twice!", "Sorting by aisle...", "Organizing...", "I know just where these go..."]
const PENNY_STEP3 = ["Adding to your list...", "Almost done!", "Stocking up!"]
const PENNY_STEPS = [PENNY_STEP1, PENNY_STEP2, PENNY_STEP3]
const PENNY_QUOTES = [
  "I work hard so that shopping is effortless and affordable for you and the people you love.",
  "I coordinate with your whole household so nothing gets forgotten and nothing gets doubled.",
  "I make sure you get the right amount of the right things — no more, no less.",
  "I find the best deals so you can spend less time comparing and more time living.",
  "I keep your list organized by aisle so you can breeze through the store.",
  "I'm always listening — text me anytime and I'll take care of the rest.",
  "You can text or email me — I'll add it to your list!",
]

function pickRandom(arr: string[]) {
  return arr[Math.floor(Math.random() * arr.length)]
}

// =============================================================================
// EXTRACTED COMPONENTS
// =============================================================================

function ActionSheet({ isOpen, onClose, title, children, maxHeight }: {
  isOpen: boolean
  onClose: () => void
  title?: string
  children: React.ReactNode
  maxHeight?: string
}) {
  if (!isOpen) return null
  return (
    <div style={listStyles.overlay} onClick={onClose}>
      <div style={{ ...listStyles.sheet, ...(maxHeight ? { maxHeight } : {}) }} onClick={e => e.stopPropagation()}>
        {title && <p style={listStyles.sheetTitle}>{title}</p>}
        {children}
      </div>
    </div>
  )
}

function SectionHeader({ label }: { label: string }) {
  return (
    <p style={{ fontFamily: FONT, fontSize: '12px', fontWeight: 600, color: TEXT_SECONDARY, textTransform: 'uppercase', letterSpacing: '0.5px', marginBottom: '12px' }}>{label}</p>
  )
}

function FloatingLabelField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div style={{ textAlign: 'left' as const }}>
      <div style={{
        fontSize: '11px', fontWeight: 600, color: TEXT_SECONDARY,
        textTransform: 'uppercase' as const, letterSpacing: '0.5px', fontFamily: FONT,
        marginBottom: '6px',
      }}>{label}</div>
      {children}
    </div>
  )
}

function QuantityControl({ value, onChange }: {
  value: number
  onChange: (newValue: number) => void
}) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', border: `1.5px solid ${BORDER}`, borderRadius: '12px', backgroundColor: WHITE, flexShrink: 0 }}>
      <button
        onClick={() => onChange(Math.max(1, value - 1))}
        style={{ width: '38px', height: '42px', border: 'none', background: 'none', fontSize: '17px', color: BRAND, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
      >−</button>
      <span style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, minWidth: '24px', textAlign: 'center' }}>{value}</span>
      <button
        onClick={() => onChange(value + 1)}
        style={{ width: '38px', height: '42px', border: 'none', background: 'none', fontSize: '17px', color: BRAND, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
      >+</button>
    </div>
  )
}

function ConfirmModal({ open, title, message, confirmLabel, onConfirm, onCancel, destructive }: {
  open: boolean
  title: string
  message?: string
  confirmLabel?: string
  onConfirm: () => void
  onCancel: () => void
  destructive?: boolean
}) {
  if (!open) return null
  return (
    <div onClick={onCancel} style={{
      position: 'fixed', inset: 0, zIndex: 9999,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      backgroundColor: 'rgba(0,0,0,0.4)',
    }}>
      <div onClick={e => e.stopPropagation()} style={{
        backgroundColor: '#fff', borderRadius: '14px', width: '270px',
        overflow: 'hidden', textAlign: 'center', fontFamily: FONT,
      }}>
        <div style={{ padding: '20px 16px 0' }}>
          <div style={{ fontSize: '17px', fontWeight: 600, color: TEXT }}>{title}</div>
          {message && <div style={{ fontSize: '12px', color: TEXT_SECONDARY, marginTop: '4px' }}>{message}</div>}
        </div>
        <div style={{ display: 'flex', borderTop: '1px solid #e5e5e5', marginTop: '16px' }}>
          <button onClick={onCancel} style={{
            flex: 1, padding: '12px', border: 'none', background: 'none',
            fontSize: '17px', fontFamily: FONT, color: '#007AFF', cursor: 'pointer',
            borderRight: '1px solid #e5e5e5',
          }}>Cancel</button>
          <button onClick={onConfirm} style={{
            flex: 1, padding: '12px', border: 'none', background: 'none',
            fontSize: '17px', fontWeight: 600, fontFamily: FONT, cursor: 'pointer',
            color: destructive ? ERROR_RED : '#007AFF',
          }}>{confirmLabel || 'Confirm'}</button>
        </div>
      </div>
    </div>
  )
}

function LegalLinks() {
  return (
    <>
      <a
        href="https://thrift.ly"
        target="_blank"
        rel="noopener noreferrer"
        style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, textDecoration: 'none', marginTop: '48px', opacity: 0.5 }}
      >
        powered by <span style={{ fontWeight: 600 }}>thriftly</span>
      </a>
      <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '12px', opacity: 0.45 }}>
        <a href="/privacy" target="_blank" rel="noopener noreferrer" style={{ color: TEXT_SECONDARY, textDecoration: 'none' }}>Privacy</a>
        {' · '}
        <a href="/terms" target="_blank" rel="noopener noreferrer" style={{ color: TEXT_SECONDARY, textDecoration: 'none' }}>Terms</a>
      </div>
    </>
  )
}

// =============================================================================
// CONFETTI COMPONENT — pure CSS, no canvas, no library
// =============================================================================
const CONFETTI_COLORS = ['#35644F', '#4CAF50', '#FF6B6B', '#FFD93D', '#6BCB77', '#4D96FF', '#FF6F91']

function Confetti({ onComplete }: { onComplete: () => void }) {
  const particles = useMemo(() =>
    Array.from({ length: 120 }, (_, i) => ({
      id: i,
      color: CONFETTI_COLORS[i % CONFETTI_COLORS.length],
      left: Math.random() * 100,
      size: 6 + Math.random() * 4,
      delay: Math.random() * 1.2,
      duration: 2 + Math.random() * 1.5,
    })), [])

  const onCompleteRef = useRef(onComplete)
  onCompleteRef.current = onComplete
  useEffect(() => {
    const timer = setTimeout(() => onCompleteRef.current(), 5000)
    return () => clearTimeout(timer)
  }, [])

  return (
    <div style={{ position: 'fixed', inset: 0, pointerEvents: 'none', zIndex: 9999, overflow: 'hidden' }}>
      {particles.map((p: { id: number; color: string; left: number; size: number; delay: number; duration: number }) => (
        <div key={p.id} style={{
          position: 'absolute',
          top: '-10px',
          left: `${p.left}%`,
          width: `${p.size}px`,
          height: `${p.size}px`,
          backgroundColor: p.color,
          borderRadius: p.id % 3 === 0 ? '50%' : '2px',
          animation: `confetti-fall ${p.duration}s ease-in ${p.delay}s forwards`,
          opacity: 0,
        }} />
      ))}
    </div>
  )
}

// =============================================================================
// MAIN APP COMPONENT
// =============================================================================

export default function App() {
  // --- State ---
  const [screen, setScreenRaw] = useState<Screen>('phone')
  const scrollPositions = useRef<Record<string, number>>({})
  const listAreaRef = useRef<HTMLDivElement>(null)
  const navigatingBack = useRef(false)

  const setScreen = (s: Screen, replace = false) => {
    // Save scroll position before leaving current screen
    if (listAreaRef.current) {
      scrollPositions.current[screen] = listAreaRef.current.scrollTop
    }
    setScreenRaw(s)
    const path = SCREEN_PATHS[s] || '/'
    const method = replace ? 'replaceState' : 'pushState'
    window.history[method]({ screen: s }, '', path)
    if (typeof gtag === 'function') {
      gtag('event', 'page_view', { page_title: s, page_location: path })
    }
  }
  const [phone, setPhone] = useState('')           // raw digits: "5551234567"
  const [code, setCode] = useState(['', '', '', '', '', '']) // 6 digits
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState(true)    // checking existing session on load
  const [pennyMessage, setPennyMessage] = useState('')
  const [pennyTyping, setPennyTyping] = useState(false)
  const pennyQuoteIdx = useRef(0)
  const pennyCancelled = useRef(false)

  // Home page Penny typewriter
  const [homePennyText, setHomePennyText] = useState('')
  const [homePennyTyping, setHomePennyTyping] = useState(false)
  const [homePennyBtns, setHomePennyBtns] = useState<{ label: string; onClick: () => void }[]>([])
  const homePennySource = useRef('')
  const homePennyInterval = useRef<ReturnType<typeof setInterval> | null>(null)
  const idleTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const hasGreetedEmpty = useRef(false)

  // --- List state ---
  const [items, setItems] = useState<ListItem[]>([])
  const [chatText, setChatText] = useState('')
  const [chatLoading, setChatLoading] = useState(false)
  const [expandedDepts, setExpandedDepts] = useState<Set<string>>(new Set())
  const [selectedItem, setSelectedItem] = useState<ListItem | null>(null)
  const [editName, setEditName] = useState('')
  const [editBrand, setEditBrand] = useState('')
  const [displayBrand, setDisplayBrand] = useState('')
  const [editDept, setEditDept] = useState('')
  const [editQty, setEditQty] = useState(1)
  const [editSize, setEditSize] = useState('')
  const [editNotes, setEditNotes] = useState('')
  const [editLoading, setEditLoading] = useState(false)
  const [itemHistory, setItemHistory] = useState<ItemModification[]>([])
  const deptRefs = useRef<Record<string, HTMLDivElement | null>>({})

  // --- Toast state ---
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error'; action?: { label: string; onClick: () => void } } | null>(null)
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // --- Review state (items from last parse) ---
  const [lastAdded, setLastAdded] = useState<ListItem[]>([])
  const [lastSkipped, setLastSkipped] = useState<{ parsed: { name: string; brand: string; quantity: number; size: string; department: string }; existing_id: number }[]>([])
  const [showReviewSheet, setShowReviewSheet] = useState(false)
  const [returnToReview, setReturnToReview] = useState(false)

  // --- Photo state ---
  const [pendingImages, setPendingImages] = useState<string[]>([])
  const fileInputRef = useRef<HTMLInputElement>(null)
  const chatInputRef = useRef<HTMLTextAreaElement>(null)

  // --- Hamburger menu state ---
  const [menuOpen, setMenuOpen] = useState(false)

  // --- Done toast state ---
  const [showDoneToast, setShowDoneToast] = useState(false)
  const [showFireworks, setShowFireworks] = useState(false)
  const [showDoneOverlay, setShowDoneOverlay] = useState(false)

  // --- Floating label animation state ---
  const [floatingLabels, setFloatingLabels] = useState<Array<{ text: string; key: number }>>([])

  // --- Shopping trip planner state ---
  const [trips, setTrips] = useState<ShoppingTrip[]>([])
  const [showTripSheet, setShowTripSheet] = useState(false)
  const [tripDay, setTripDay] = useState('')
  const [tripTimeLabel, setTripTimeLabel] = useState('')
  const [tripExactTime, setTripExactTime] = useState('')

  // --- Saved lists state ---
  const [savedLists, setSavedLists] = useState<SavedList[]>([])
  const [showSaveSheet, setShowSaveSheet] = useState(false)
  const [saveNameInput, setSaveNameInput] = useState('')
  const [selectedSavedList, setSelectedSavedList] = useState<SavedList | null>(null)

  // --- Confirm modal state ---
  const [confirmModal, setConfirmModal] = useState<{
    title: string; message?: string; confirmLabel?: string; destructive?: boolean
  } | null>(null)
  const confirmActionRef = useRef<(() => void) | null>(null)
  const openConfirmModal = (opts: { title: string; message?: string; confirmLabel?: string; destructive?: boolean; onConfirm: () => void }) => {
    confirmActionRef.current = opts.onConfirm
    setConfirmModal({ title: opts.title, message: opts.message, confirmLabel: opts.confirmLabel, destructive: opts.destructive })
  }
  const closeConfirmModal = () => {
    setConfirmModal(null)
    confirmActionRef.current = null
  }
  const runConfirmAction = () => {
    const action = confirmActionRef.current
    closeConfirmModal()
    if (action) action()
  }

  // --- Settings state ---
  const [settingsFirstName, setSettingsFirstName] = useState('')
  const [settingsLastName, setSettingsLastName] = useState('')
  const [settingsEmail, setSettingsEmail] = useState('')
  const [settingsAddress, setSettingsAddress] = useState('')
  const [googlePlacesKey, setGooglePlacesKey] = useState('')
  const [settingsMembers, setSettingsMembers] = useState<{ name: string; phone: string }[]>([])
  const [settingsLoading, setSettingsLoading] = useState(false)
  const [inviteLoading, setInviteLoading] = useState(false)

  // --- Referral state ---
  const [referralCode, setReferralCode] = useState('')
  const [referralFriends, setReferralFriends] = useState<{ name: string; created_at: string }[]>([])

  // --- Admin state ---
  const [isAdmin, setIsAdmin] = useState(false)
  const [currentUserId, setCurrentUserId] = useState<number | null>(null)
  const [adminData, setAdminData] = useState<{
    active_users: number; total_users: number; new_users: number;
    k_factor: number; leaderboard: { phone: string; name: string; points: number; referrals: number }[];
    days: number;
    nps_responses: { score: number; comment: string; user_name: string; user_phone: string; created_at: string }[];
    nps_score: number;
    nps_total: number;
    nps_promoters: number;
    nps_detractors: number;
  } | null>(null)
  const [adminDays, setAdminDays] = useState(30)

  // --- NPS state ---
  const [showNPS, setShowNPS] = useState(false)
  const [npsScore, setNpsScore] = useState<number | null>(null)
  const [npsStep, setNpsStep] = useState<1 | 2>(1)
  const [npsComment, setNpsComment] = useState('')
  const [npsTypedText, setNpsTypedText] = useState('')
  const [npsTypingDone, setNpsTypingDone] = useState(false)

  // --- SMS opt-in state ---
  const [smsOptIn, setSmsOptIn] = useState(true)

  // --- New item highlight state ---
  const [newItemIds, setNewItemIds] = useState<Set<number>>(new Set())
  const newItemTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // --- Penny loading spinner state ---
  const [pennyStatus, setPennyStatus] = useState<string | null>(null)

  // --- Penny chime-in state ---
  const [pennyChimeIn, setPennyChimeIn] = useState<string | null>(null)
  const chimeInTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const prevCheckedRef = useRef(-1)

  const CHIME_IN_MESSAGES = [
    'Looking good!', 'Almost there!', 'Nice work!', 'On a roll!', 'Keep going!',
  ]

  const EMPTY_LIST_MESSAGES = [
    "What are we shopping for today?",
    "Ready when you are — just tell me what you need!",
    "Type your grocery list below and I'll get it organized.",
    "What can I help you find at the store?",
  ]

  const IDLE_MESSAGES = [
    "Still here if you need anything!",
    "I'm not going anywhere — add items whenever you're ready.",
    "Need help planning a meal? Just ask!",
    "Tip: you can paste a recipe and I'll pull out the ingredients.",
    "You can also snap a photo of a recipe or your fridge!",
  ]

  const DEPT_DONE_MESSAGES = [
    'done with {dept}!', '{dept} — checked!', '{dept} complete!', 'Finished {dept}!',
  ]

  const prevDeptDoneRef = useRef<Set<string>>(new Set())

  const getDoneDepts = () => {
    const done = new Set<string>()
    for (const dept of DEPARTMENTS) {
      const di = items.filter(i => i.department === dept)
      if (di.length > 0 && di.every(i => i.checked)) done.add(dept)
    }
    return done
  }

  useEffect(() => {
    const checked = items.filter(i => i.checked).length
    const nowDone = getDoneDepts()
    const allChecked = items.length > 0 && items.every(i => i.checked)
    if (checked > prevCheckedRef.current && prevCheckedRef.current >= 0 && items.length > 0) {
      const newlyDone = [...nowDone].filter(d => !prevDeptDoneRef.current.has(d))
      if (newlyDone.length > 0) {
        setTimeout(() => {
          setExpandedDepts(prev => {
            const next = new Set(prev)
            newlyDone.forEach(d => next.delete(d))
            return next
          })
        }, 600)
        // Skip dept chime-in when entire list is done — the done celebration handles it
        if (!allChecked) showChimeIn(pickRandom(DEPT_DONE_MESSAGES).replace('{dept}', newlyDone[0]), 3750)
      } else if (!allChecked && Math.random() < 0.3) {
        fetch(`/api/penny/quote?context=chime${settingsFirstName ? '&name=' + encodeURIComponent(settingsFirstName) : ''}`)
          .then(r => r.ok ? r.json() : null)
          .then(data => showChimeIn(data?.quote || pickRandom(CHIME_IN_MESSAGES), 5000))
          .catch(() => showChimeIn(pickRandom(CHIME_IN_MESSAGES), 5000))
      }
    }
    prevDeptDoneRef.current = nowDone
    prevCheckedRef.current = checked
  }, [items])

  useEffect(() => {
    if (!chatLoading) {
      setPennyStatus(null)
      return
    }
    let stepIdx = 0
    setPennyStatus(pickRandom(PENNY_STEPS[0]))
    const interval = setInterval(() => {
      stepIdx = (stepIdx + 1) % PENNY_STEPS.length
      setPennyStatus(pickRandom(PENNY_STEPS[stepIdx]))
    }, 2000)
    return () => clearInterval(interval)
  }, [chatLoading])

  // --- Refs ---
  // We need refs to programmatically focus inputs (autofocus, auto-advance)
  const phoneInputRef = useRef<HTMLInputElement>(null)
  const codeInputRefs = useRef<(HTMLInputElement | null)[]>([])

  // =========================================================================
  // CHECK EXISTING SESSION ON LOAD
  //
  // When the page loads, call GET /auth/me to see if the user already has
  // a valid session cookie. If so, skip straight to the home screen.
  // =========================================================================

  // Store join code or referral code from URL if present
  useEffect(() => {
    const path = window.location.pathname
    const joinMatch = path.match(/^\/join\/(.+)$/)
    if (joinMatch) {
      localStorage.setItem('thriftly_join_code', joinMatch[1])
      window.history.replaceState({}, '', '/')
    }
    const referralMatch = path.match(/^\/r\/(.+)$/)
    if (referralMatch) {
      localStorage.setItem('referral_code', referralMatch[1])
      window.history.replaceState({}, '', '/')
    }
  }, [])

  // Browser back/forward navigation
  useEffect(() => {
    const handlePopState = (event: PopStateEvent) => {
      const s = event.state?.screen as Screen | undefined
      if (s) {
        // Save scroll before leaving
        if (listAreaRef.current) {
          scrollPositions.current[screen] = listAreaRef.current.scrollTop
        }
        navigatingBack.current = true
        setScreenRaw(s)
        // Close all modals/sheets when navigating
        setSelectedItem(null)
        setShowReviewSheet(false)
        setShowSaveSheet(false)
        setShowTripSheet(false)
        setSelectedSavedList(null)
        setMenuOpen(false)
      } else {
        // No state — resolve from URL
        const path = window.location.pathname
        const resolved = PATH_TO_SCREEN[path]
        if (resolved) {
          if (listAreaRef.current) {
            scrollPositions.current[screen] = listAreaRef.current.scrollTop
          }
          navigatingBack.current = true
          setScreenRaw(resolved)
          setSelectedItem(null)
          setShowReviewSheet(false)
          setShowSaveSheet(false)
          setShowTripSheet(false)
          setSelectedSavedList(null)
          setMenuOpen(false)
        }
      }
    }
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [screen])

  // Try to use join code after login
  const tryJoinCode = useCallback(async () => {
    const joinCode = localStorage.getItem('thriftly_join_code')
    if (joinCode) {
      try {
        const res = await fetch('/api/join', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ code: joinCode }),
        })
        if (res.ok) {
          localStorage.removeItem('thriftly_join_code')
          showToast('Joined household list!')
        } else {
          localStorage.removeItem('thriftly_join_code')
          const data = await res.json().catch(() => null)
          if (data?.error) showToast(data.error, 'error')
        }
      } catch {
        localStorage.removeItem('thriftly_join_code')
      }
    }
    // Claim referral code if present
    const refCode = localStorage.getItem('referral_code')
    if (refCode) {
      try {
        await fetch('/api/referral/claim', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ code: refCode }),
        })
      } catch { /* best effort */ }
      localStorage.removeItem('referral_code')
    }
  }, [])

  useEffect(() => {
    // Migrate old hash-based URLs to pathname
    const hash = window.location.hash.replace('#', '')
    if (hash) {
      const hashScreen = hash as Screen
      const newPath = SCREEN_PATHS[hashScreen]
      if (newPath) {
        window.history.replaceState({ screen: hashScreen }, '', newPath)
      }
    }

    fetch('/auth/me')
      .then(res => {
        if (res.ok) {
          res.clone().json().then((data: { is_admin?: boolean; user_id?: number }) => {
            if (data.is_admin) setIsAdmin(true)
            if (data.user_id) setCurrentUserId(data.user_id)
          }).catch(() => {})
          tryJoinCode()
          // Resolve screen from pathname (or migrated hash)
          const path = window.location.pathname
          const resolved = PATH_TO_SCREEN[path]
          if (resolved && !['phone', 'code'].includes(resolved)) {
            setScreen(resolved, true)
          } else {
            // Check for stored redirect from pre-login deep link
            const redirect = sessionStorage.getItem('thriftly_redirect')
            if (redirect) {
              sessionStorage.removeItem('thriftly_redirect')
              const target = PATH_TO_SCREEN[redirect] || 'home'
              setScreen(target as Screen, true)
            } else {
              setScreen('home', true)
            }
          }
        } else {
          // Not authenticated — store intended destination for post-login redirect
          const path = window.location.pathname
          const resolved = PATH_TO_SCREEN[path]
          if (resolved && !['phone', 'code'].includes(resolved)) {
            sessionStorage.setItem('thriftly_redirect', path)
          }
          setScreen('phone', true)
        }
      })
      .catch(() => {
        // Not logged in — that's fine, show the phone screen
        setScreen('phone', true)
      })
      .finally(() => setChecking(false))
  }, [])

  // =========================================================================
  // AUTOFOCUS MANAGEMENT
  //
  // When the screen changes, focus the appropriate input:
  //   - Phone screen → focus the phone input
  //   - Code screen → focus the first code box
  // =========================================================================

  useEffect(() => {
    if (screen === 'phone' && !checking) {
      setTimeout(() => phoneInputRef.current?.focus(), 150)
    }
    if (screen === 'code') {
      setTimeout(() => codeInputRefs.current[0]?.focus(), 150)
    }
    // Restore scroll position when navigating back
    if (navigatingBack.current) {
      navigatingBack.current = false
      const saved = scrollPositions.current[screen]
      if (saved != null && listAreaRef.current) {
        requestAnimationFrame(() => {
          if (listAreaRef.current) listAreaRef.current.scrollTop = saved
        })
      }
    }
  }, [screen, checking])

  // Penny cycling typewriter — used on login and about pages
  useEffect(() => {
    if (screen !== 'phone' && screen !== 'code' && screen !== 'about') {
      setPennyMessage('')
      setPennyTyping(false)
      return
    }
    pennyCancelled.current = false
    let activeInterval: ReturnType<typeof setInterval> | null = null
    let activeTimeout: ReturnType<typeof setTimeout> | null = null
    const typeQuote = (quote: string) => {
      setPennyTyping(true)
      let i = 0
      activeInterval = setInterval(() => {
        if (pennyCancelled.current) { clearInterval(activeInterval!); return }
        i++
        setPennyMessage(quote.slice(0, i))
        if (i >= quote.length) {
          clearInterval(activeInterval!)
          activeInterval = null
          setPennyTyping(false)
          activeTimeout = setTimeout(() => {
            if (!pennyCancelled.current) runQuote()
          }, 5000)
        }
      }, 30)
    }
    const runQuote = () => {
      fetch('/api/penny/quote?context=login')
        .then(r => r.ok ? r.json() : null)
        .then(data => {
          if (pennyCancelled.current) return
          const quote = data?.quote || PENNY_QUOTES[pennyQuoteIdx.current % PENNY_QUOTES.length]
          pennyQuoteIdx.current++
          typeQuote(quote)
        })
        .catch(() => {
          if (pennyCancelled.current) return
          const quote = PENNY_QUOTES[pennyQuoteIdx.current % PENNY_QUOTES.length]
          pennyQuoteIdx.current++
          typeQuote(quote)
        })
    }
    activeTimeout = setTimeout(() => { if (!pennyCancelled.current) runQuote() }, 1000)
    return () => {
      pennyCancelled.current = true
      if (activeInterval) clearInterval(activeInterval)
      if (activeTimeout) clearTimeout(activeTimeout)
    }
  }, [screen])

  // =========================================================================
  // LOAD LIST when on home screen
  // =========================================================================

  const sortItems = (arr: ListItem[]) =>
    [...arr].sort((a, b) => a.name.localeCompare(b.name))

  const handleSessionExpired = () => {
    setScreen('phone', true)
    setPhone('')
    setError('')
    showToast('Session expired — please log in again', 'error')
  }

  // Centralized fetch wrapper: handles 401 session expiry and JSON parsing.
  // Returns parsed JSON on success, or null on failure.
  const fetchAPI = async (url: string, options?: RequestInit): Promise<Record<string, unknown> | unknown[] | null> => {
    const res = await fetch(url, options)
    if (res.status === 401) { handleSessionExpired(); return null }
    if (!res.ok) return null
    return res.json()
  }

  // Update a single item in the items list by id
  const updateItem = (id: number, updates: Partial<ListItem>) =>
    setItems(prev => prev.map(item => item.id === id ? { ...item, ...updates } : item))

  // Show a Penny chime-in message with auto-clear
  const showChimeIn = (msg: string, duration = 4000) => {
    if (chimeInTimer.current) clearTimeout(chimeInTimer.current)
    setPennyChimeIn(msg)
    chimeInTimer.current = setTimeout(() => setPennyChimeIn(null), duration)
  }

  const reloadList = async () => {
    const data = await fetchAPI('/api/list')
    if (data) {
      const loaded = sortItems(Array.isArray(data) ? data as ListItem[] : [])
      setItems(loaded)
      // Default all departments with items to expanded
      setExpandedDepts(prev => {
        if (prev.size > 0) return prev // preserve user's expand/collapse choices
        const depts = new Set(loaded.map(i => i.department))
        return depts
      })
    }
  }

  const loadTrips = async () => {
    const data = await fetchAPI('/api/trips')
    if (data) setTrips(Array.isArray(data) ? data as ShoppingTrip[] : [])
  }

  useEffect(() => {
    if (screen !== 'home') return
    reloadList().catch(() => {})
    loadTrips().catch(() => {})
    fetchAPI('/api/nps/check')
      .then(data => { if (data && !Array.isArray(data) && data.show_nps) setShowNPS(true) })
      .catch(() => {})
  }, [screen])

  // --- Trip planner helpers ---
  const formatTripTime = (trip: ShoppingTrip) => {
    const today = new Date()
    const tripDate = new Date(trip.trip_date + 'T12:00:00')
    const diffDays = Math.round((tripDate.getTime() - new Date(today.getFullYear(), today.getMonth(), today.getDate()).getTime()) / 86400000)

    let dayStr = ''
    if (diffDays === 0) dayStr = 'Today'
    else if (diffDays === 1) dayStr = 'Tomorrow'
    else if (diffDays > 1 && diffDays <= 6) dayStr = tripDate.toLocaleDateString('en-US', { weekday: 'long' })
    else dayStr = tripDate.toLocaleDateString('en-US', { weekday: 'long', month: 'short', day: 'numeric' })

    if (!trip.trip_time) return dayStr
    if (trip.time_label) return `${dayStr} ${trip.time_label}`

    // Format exact time
    const [h, m] = trip.trip_time.split(':').map(Number)
    const ampm = h >= 12 ? 'pm' : 'am'
    const h12 = h % 12 || 12
    return `${dayStr} at ${h12}${m ? `:${m.toString().padStart(2, '0')}` : ''}${ampm}`
  }

  const isTripInProgress = (trip: ShoppingTrip) => {
    if (!trip.trip_time) {
      // No time set — "in progress" if date is today
      const today = new Date().toISOString().split('T')[0]
      return trip.trip_date === today
    }
    const now = new Date()
    const tripDT = new Date(`${trip.trip_date}T${trip.trip_time}:00`)
    const diff = now.getTime() - tripDT.getTime()
    return diff > 0 && diff < 2 * 3600 * 1000
  }

  const isTripExpired = (trip: ShoppingTrip) => {
    const now = new Date()
    if (!trip.trip_time) {
      // Expires end of the day
      const endOfDay = new Date(trip.trip_date + 'T23:59:59')
      return now > endOfDay
    }
    const tripDT = new Date(`${trip.trip_date}T${trip.trip_time}:00`)
    return now.getTime() - tripDT.getTime() > 2 * 3600 * 1000
  }

  const activeTrips = trips.filter(t => !isTripExpired(t))

  const handleSetTrip = async (addToCalendar = false) => {
    if (!tripDay) return
    const timeMap: Record<string, string> = { morning: '09:00', afternoon: '14:00', evening: '18:00' }
    const tripTime = tripTimeLabel === 'exact' ? tripExactTime : (timeMap[tripTimeLabel] || '')
    const timeLabel = tripTimeLabel && tripTimeLabel !== 'exact' ? tripTimeLabel : ''
    await fetch('/api/trips', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ trip_date: tripDay, trip_time: tripTime, time_label: timeLabel }),
    })
    if (addToCalendar) {
      const fakeTrip = { trip_date: tripDay, trip_time: tripTime, time_label: timeLabel } as ShoppingTrip
      const link = document.createElement('a')
      link.href = generateICSLink(fakeTrip)
      link.download = 'shopping-trip.ics'
      link.click()
    }
    setShowTripSheet(false)
    setTripDay('')
    setTripTimeLabel('')
    setTripExactTime('')
    loadTrips()
    showToast('Trip planned!')
  }

  const handleCancelTrip = async () => {
    await fetch('/api/trips', { method: 'DELETE' })
    loadTrips()
    showToast('Trip cancelled')
  }

  const generateICSLink = (trip: ShoppingTrip) => {
    const date = trip.trip_date.replace(/-/g, '')
    const timeMap: Record<string, string> = { morning: '090000', afternoon: '140000', evening: '180000' }
    let dtStart = date + 'T'
    let dtEnd = date + 'T'
    if (trip.trip_time) {
      dtStart += trip.trip_time.replace(':', '') + '00'
      const [h, m] = trip.trip_time.split(':').map(Number)
      dtEnd += `${String(h + 1).padStart(2, '0')}${String(m).padStart(2, '0')}00`
    } else if (trip.time_label && timeMap[trip.time_label]) {
      dtStart += timeMap[trip.time_label]
      const h = parseInt(timeMap[trip.time_label].slice(0, 2)) + 1
      dtEnd += `${String(h).padStart(2, '0')}0000`
    } else {
      dtStart += '100000'
      dtEnd += '110000'
    }
    const ics = [
      'BEGIN:VCALENDAR',
      'VERSION:2.0',
      'BEGIN:VEVENT',
      `DTSTART:${dtStart}`,
      `DTEND:${dtEnd}`,
      'SUMMARY:Grocery Shopping',
      'DESCRIPTION:Planned via Thriftly',
      'END:VEVENT',
      'END:VCALENDAR',
    ].join('\r\n')
    return 'data:text/calendar;charset=utf-8,' + encodeURIComponent(ics)
  }

  const getDayOptions = () => {
    const options: { label: string; value: string }[] = []
    const now = new Date()
    for (let i = 2; i <= 7; i++) {
      const d = new Date(now)
      d.setDate(d.getDate() + i)
      options.push({
        label: d.toLocaleDateString('en-US', { weekday: 'long', month: 'short', day: 'numeric' }),
        value: d.toISOString().split('T')[0],
      })
    }
    return options
  }

  const todayStr = new Date().toISOString().split('T')[0]
  const tomorrowDate = new Date()
  tomorrowDate.setDate(tomorrowDate.getDate() + 1)
  const tomorrowStr = tomorrowDate.toISOString().split('T')[0]
  const satDate = new Date()
  satDate.setDate(satDate.getDate() + (6 - satDate.getDay() || 7))
  const satStr = satDate.toISOString().split('T')[0]

  // Show Penny chime-in when a household member has a trip planned
  const tripChimeShown = useRef(false)
  useEffect(() => {
    if (tripChimeShown.current) return
    const othersTrips = activeTrips.filter(t => !t.is_mine)
    if (othersTrips.length > 0) {
      tripChimeShown.current = true
      const t = othersTrips[0]
      const who = t.user_name || 'Someone'
      const when = formatTripTime(t)
      showChimeIn(`${who} is heading to the store ${when.toLowerCase()} — add what you need!`, 6000)
    }
  }, [trips])

  const loadSavedLists = () => {
    fetchAPI('/api/saved-lists')
      .then(data => setSavedLists(Array.isArray(data) ? [...(data as SavedList[])].sort((a, b) => a.name.localeCompare(b.name)) : []))
      .catch(() => {})
  }

  useEffect(() => {
    if (screen !== 'saved') return
    loadSavedLists()
  }, [screen])

  const loadSettings = () => {
    fetchAPI('/api/settings')
      .then(data => {
        if (data && !Array.isArray(data)) {
          const parts = ((data.name as string) || '').split(' ')
          setSettingsFirstName(parts[0] || '')
          setSettingsLastName(parts.slice(1).join(' '))
          setSettingsEmail((data.email as string) || '')
          setSettingsAddress((data.address as string) || '')
          if (data.google_places_key) setGooglePlacesKey(data.google_places_key as string)
          setSettingsMembers((data.household_members as { name: string; phone: string }[]) || [])
        }
      })
      .catch(() => {})
  }

  // Google Places autocomplete for address field
  useEffect(() => {
    if (screen !== 'settings-profile' || !googlePlacesKey) return
    const input = document.getElementById('address-autocomplete') as HTMLInputElement
    if (!input) return

    // Load Google Maps JS if not already loaded
    const scriptId = 'google-places-script'
    const initAutocomplete = () => {
      if (!(window as unknown as Record<string, unknown>).google) return
      const google = (window as unknown as Record<string, { maps: { places: { Autocomplete: new (el: HTMLInputElement, opts: Record<string, unknown>) => { addListener: (event: string, cb: () => void) => void; getPlace: () => { formatted_address?: string } } } } }>).google
      const autocomplete = new google.maps.places.Autocomplete(input, {
        types: ['address'],
        componentRestrictions: { country: 'us' },
      })
      autocomplete.addListener('place_changed', () => {
        const place = autocomplete.getPlace()
        if (place.formatted_address) {
          setSettingsAddress(place.formatted_address)
        }
      })
    }

    if (document.getElementById(scriptId)) {
      initAutocomplete()
      return
    }
    const script = document.createElement('script')
    script.id = scriptId
    script.src = `https://maps.googleapis.com/maps/api/js?key=${googlePlacesKey}&libraries=places`
    script.async = true
    script.onload = initAutocomplete
    document.head.appendChild(script)
  }, [screen, googlePlacesKey])

  useEffect(() => {
    if (!['home', 'settings', 'about', 'invite', 'settings-profile', 'settings-share'].includes(screen)) return
    loadSettings()
  }, [screen])

  const loadReferralData = () => {
    fetchAPI('/api/referral')
      .then(data => {
        if (data && !Array.isArray(data)) {
          setReferralCode((data.code as string) || '')
          setReferralFriends((data.friends as { name: string; created_at: string }[]) || [])
        }
      })
      .catch(() => {})
  }

  useEffect(() => {
    if (screen !== 'settings-share') return
    loadReferralData()
  }, [screen])

  useEffect(() => {
    if (screen !== 'admin') return
    fetchAPI(`/api/admin/dashboard?days=${adminDays}`)
      .then(data => { if (data) setAdminData(data as typeof adminData) })
      .catch(() => {})
  }, [screen, adminDays])

  // Redirect legacy invite screen to settings-family
  useEffect(() => {
    if (screen === 'invite') setScreen('settings-share', true)
  }, [screen])

  const showToast = (message: string, type: 'success' | 'error' = 'success', action?: { label: string; onClick: () => void }) => {
    if (toastTimer.current) clearTimeout(toastTimer.current)
    setToast({ message, type, action })
    toastTimer.current = setTimeout(() => setToast(null), action ? 10000 : 6250)
  }

  // Type out a message in Penny's home bar, letter by letter
  const typePennyHome = (msg: string, btns: { label: string; onClick: () => void }[] = []) => {
    if (homePennyInterval.current) clearInterval(homePennyInterval.current)
    if (msg === homePennySource.current && !btns.length) return // don't re-type same message
    homePennySource.current = msg
    setHomePennyText('')
    setHomePennyBtns(btns)
    setHomePennyTyping(true)
    let i = 0
    homePennyInterval.current = setInterval(() => {
      i++
      setHomePennyText(msg.slice(0, i))
      if (i >= msg.length) {
        if (homePennyInterval.current) clearInterval(homePennyInterval.current)
        homePennyInterval.current = null
        setHomePennyTyping(false)
      }
    }, 25)
  }

  // Watch for penny messages on home page and type them out
  useEffect(() => {
    if (screen !== 'home') return
    const allDone = showDoneToast && items.length > 0 && items.every(i => i.checked)
    if (allDone && !pennyStatus) {
      // Clear any chime-in so the done message takes priority
      if (pennyChimeIn) { setPennyChimeIn(null); if (chimeInTimer.current) { clearTimeout(chimeInTimer.current); chimeInTimer.current = null } }
      const doneBtns = [
        { label: 'Save List', onClick: () => { setShowDoneToast(false); setShowFireworks(false); setSaveNameInput(''); pushModalState(); setShowSaveSheet(true) } },
        { label: 'Clear List', onClick: () => { setShowDoneToast(false); setShowFireworks(false); handleClearList() } },
      ]
      fetch(`/api/penny/quote?context=done${settingsFirstName ? '&name=' + encodeURIComponent(settingsFirstName) : ''}`)
        .then(r => r.ok ? r.json() : null)
        .then(data => typePennyHome(data?.quote || "You're done!", doneBtns))
        .catch(() => typePennyHome("You're done!", doneBtns))
    } else if (pennyStatus) {
      typePennyHome(pennyStatus)
    } else if (toast) {
      const btns: { label: string; onClick: () => void }[] = []
      if (toast.action) btns.push({ label: toast.action.label, onClick: () => { toast.action!.onClick(); setToast(null); if (toastTimer.current) clearTimeout(toastTimer.current) } })
      else if ((lastAdded.length > 0 || lastSkipped.length > 0) && toast.type !== 'error') btns.push({ label: 'Review', onClick: () => { pushModalState(); setShowReviewSheet(true); setToast(null); if (toastTimer.current) clearTimeout(toastTimer.current) } })
      typePennyHome(toast.message, btns)
    } else if (pennyChimeIn) {
      // Don't let chime-ins interrupt an actively typing message
      if (!homePennyInterval.current) typePennyHome(pennyChimeIn)
    } else if (items.length === 0 && !hasGreetedEmpty.current) {
      hasGreetedEmpty.current = true
      typePennyHome(pickRandom(EMPTY_LIST_MESSAGES))
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen, pennyStatus, toast?.message, pennyChimeIn, showDoneToast, items.length])

  // Reset empty greeting when items are cleared so it greets again next time
  useEffect(() => {
    if (items.length > 0) hasGreetedEmpty.current = false
  }, [items.length])

  // Idle nudge — if nothing happens for 2 minutes, Penny says something
  useEffect(() => {
    if (screen !== 'home') return
    const resetIdle = () => {
      if (idleTimer.current) clearTimeout(idleTimer.current)
      idleTimer.current = setTimeout(() => {
        // Only nudge if no other message is actively showing
        if (!pennyStatus && !toast && !pennyChimeIn && !showDoneToast) {
          typePennyHome(pickRandom(IDLE_MESSAGES))
        }
      }, 300000) // 5 minutes
    }
    resetIdle()
    // Reset on any user interaction
    const events = ['touchstart', 'click', 'keydown'] as const
    events.forEach(e => window.addEventListener(e, resetIdle, { passive: true }))
    return () => {
      if (idleTimer.current) clearTimeout(idleTimer.current)
      events.forEach(e => window.removeEventListener(e, resetIdle))
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen, pennyStatus, toast, pennyChimeIn, showDoneToast])

  const resizeAndSetPhoto = (file: Blob) => {
    const img = new Image()
    const objectUrl = URL.createObjectURL(file)
    img.onload = () => {
      const MAX = 1024
      let w = img.width
      let h = img.height
      if (w > MAX || h > MAX) {
        if (w > h) { h = Math.round(h * MAX / w); w = MAX }
        else { w = Math.round(w * MAX / h); h = MAX }
      }
      const canvas = document.createElement('canvas')
      canvas.width = w
      canvas.height = h
      const ctx = canvas.getContext('2d')
      ctx?.drawImage(img, 0, 0, w, h)
      setPendingImages(prev => [...prev, canvas.toDataURL('image/jpeg', 0.8)])
      URL.revokeObjectURL(objectUrl)
    }
    img.onerror = () => URL.revokeObjectURL(objectUrl)
    img.src = objectUrl
  }

  const handlePhotoSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files
    if (!files) return
    for (let i = 0; i < files.length; i++) {
      resizeAndSetPhoto(files[i])
    }
    // Reset file input so re-selecting same file triggers change
    e.target.value = ''
  }

  const handlePaste = (e: React.ClipboardEvent) => {
    const items = e.clipboardData?.items
    if (!items) return
    let foundImage = false
    for (const item of items) {
      if (item.type.startsWith('image/')) {
        if (!foundImage) {
          e.preventDefault()
          foundImage = true
        }
        const file = item.getAsFile()
        if (file) resizeAndSetPhoto(file)
      }
    }
  }

  const checkAllDone = (next: ListItem[]) => {
    if (next.length > 0 && next.every(i => i.checked)) {
      setTimeout(() => { setShowDoneToast(true); setShowFireworks(true); setShowDoneOverlay(true) }, 300)
    }
  }

  const handleToggle = async (id: number) => {
    setItems(prev => {
      const next = prev.map(item => item.id === id ? { ...item, checked: !item.checked } : item)
      checkAllDone(next)
      return next
    })
    try {
      const data = await fetchAPI(`/api/list/${id}/toggle`, { method: 'PATCH' })
      if (data) {
        const updated = data as ListItem
        setItems(prev => { const next = prev.map(item => item.id === updated.id ? updated : item); checkAllDone(next); return next })
      } else {
        updateItem(id, { checked: items.find(i => i.id === id)?.checked })
        showToast('Failed to update item', 'error')
      }
    } catch {
      updateItem(id, { checked: items.find(i => i.id === id)?.checked })
      showToast('Failed to update item', 'error')
    }
  }

  const handleUrgentToggle = async (id: number) => {
    const was = items.find(i => i.id === id)?.urgent
    updateItem(id, { urgent: !was })
    try {
      const data = await fetchAPI(`/api/list/${id}/urgent`, { method: 'PATCH' })
      if (data) {
        const updated = data as ListItem
        updateItem(updated.id, updated)
        if (updated.urgent) showChimeIn(`Marked ${updated.name} as urgent!`, 3000)
      } else {
        updateItem(id, { urgent: was })
        showToast('Failed to update item', 'error')
      }
    } catch {
      updateItem(id, { urgent: was })
      showToast('Failed to update item', 'error')
    }
  }

  const handleClaim = async (id: number) => {
    const oldItem = items.find(i => i.id === id)
    const wasClaimed = oldItem?.claimed_by_id === currentUserId
    updateItem(id, { claimed_by_id: wasClaimed ? null : currentUserId, claimed_by_name: wasClaimed ? '' : 'Me' })
    try {
      const data = await fetchAPI(`/api/list/${id}/claim`, { method: 'PATCH' })
      if (data) {
        const updated = data as ListItem
        updateItem(updated.id, updated)
        showChimeIn(updated.claimed_by_id ? `You claimed ${updated.name}!` : `Unclaimed ${updated.name}`, 3000)
      } else {
        if (oldItem) updateItem(id, oldItem)
        showToast('Failed to claim item', 'error')
      }
    } catch {
      if (oldItem) updateItem(id, oldItem)
      showToast('Failed to claim item', 'error')
    }
  }

  const handleSendChat = async () => {
    if ((!chatText.trim() && pendingImages.length === 0) || chatLoading) return
    setChatLoading(true)
    setLastAdded([])
    setLastSkipped([])
    try {
      const payload = JSON.stringify({ text: chatText.trim(), images: pendingImages })
      console.log('[penny] sending parse request, payload size:', payload.length)
      const res = await fetch('/api/list/parse', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload,
      })
      console.log('[penny] parse response status:', res.status)
      if (res.status === 401) { handleSessionExpired(); return }
      if (res.ok) {
        const data = await res.json()
        const action: string = data.action || 'add'
        const addedItems: ListItem[] = data.items || []
        const skippedItems = data.skipped || []
        const removedItems: ListItem[] = data.removed || []
        const updatedItems: ListItem[] = data.updated || []
        const checkedItems: ListItem[] = data.checked || []
        const pennyMsg: string = data.message || ''

        if (action === 'remove' && removedItems.length > 0) {
          // Remove items from local state
          const removedIds = new Set(removedItems.map(i => i.id))
          setItems(prev => sortItems(prev.filter(i => !removedIds.has(i.id))))
          const now = Date.now()
          setFloatingLabels(removedItems.slice(0, 5).map((item, i) => ({
            text: `−${item.name}`,
            key: now + i,
          })))
        } else if (action === 'check' || action === 'uncheck') {
          // Update checked state in local items
          const checkedIds = new Map(checkedItems.map(i => [i.id, i.checked]))
          setItems(prev => sortItems(prev.map(i => checkedIds.has(i.id) ? { ...i, checked: checkedIds.get(i.id)! } : i)))
        } else if (action === 'update' && updatedItems.length > 0) {
          // Merge updated items into local state
          setItems(prev => {
            const map = new Map(prev.map(i => [i.id, i]))
            for (const item of updatedItems) map.set(item.id, item)
            return sortItems(Array.from(map.values()))
          })
        } else if (action === 'save') {
          // Saved — just show message
        } else if (action === 'clear') {
          // Reload list to reflect cleared items
          reloadList()
        } else {
          // Default "add" flow
          setItems(prev => {
            const map = new Map(prev.map(i => [i.id, i]))
            for (const item of addedItems) map.set(item.id, item)
            return sortItems(Array.from(map.values()))
          })
          setLastAdded(addedItems)
          setLastSkipped(skippedItems)
          if (addedItems.length > 0) {
            const now = Date.now()
            setFloatingLabels(addedItems.slice(0, 5).map((item, i) => ({
              text: `+${item.quantity || 1} ${item.name}`,
              key: now + i,
            })))
          }
          if (addedItems.length > 0) {
            const newDepts = new Set(addedItems.map(i => i.department))
            setExpandedDepts(prev => {
              const next = new Set(prev)
              newDepts.forEach(d => next.add(d))
              return next
            })
            const ids = new Set(addedItems.map(i => i.id))
            setNewItemIds(ids)
            if (newItemTimerRef.current) clearTimeout(newItemTimerRef.current)
            newItemTimerRef.current = setTimeout(() => setNewItemIds(new Set()), 1500)
            const firstDept = DEPARTMENTS.find(d => newDepts.has(d))
            if (firstDept) {
              setTimeout(() => {
                deptRefs.current[firstDept]?.scrollIntoView({ behavior: 'smooth', block: 'start' })
              }, 100)
            }
          }
        }
        setChatText('')
        setPendingImages([])
        if (chatInputRef.current) chatInputRef.current.style.height = 'auto'
        setTimeout(() => chatInputRef.current?.focus(), 150)
        // Show Penny's message or fallback toast
        if (pennyMsg) {
          showToast(pennyMsg)
        } else {
          const parts: string[] = []
          if (data.added > 0) parts.push(`Added ${data.added} item${data.added === 1 ? '' : 's'}!`)
          if (skippedItems.length > 0) parts.push(`${skippedItems.length} already on your list`)
          if (removedItems.length > 0) parts.push(`Removed ${removedItems.length} item${removedItems.length === 1 ? '' : 's'}`)
          if (updatedItems.length > 0) parts.push(`Updated ${updatedItems.length} item${updatedItems.length === 1 ? '' : 's'}`)
          if (checkedItems.length > 0) parts.push(`${action === 'check' ? 'Checked' : 'Unchecked'} ${checkedItems.length} item${checkedItems.length === 1 ? '' : 's'}`)
          if (parts.length > 0) showToast(parts.join(' · '))
          else if (action === 'add' && addedItems.length === 0 && skippedItems.length === 0) showToast('No items found', 'error')
        }
      } else if (res.status === 401) {
        setScreen('phone')
        setPhone('')
        setError('')
        showToast('Session expired — please log in again', 'error')
      } else {
        const errData = await res.json().catch(() => null)
        const errMsg = errData?.error || `Error ${res.status}`
        console.error('[penny] parse failed:', res.status, errMsg)
        showToast(errMsg, 'error')
      }
    } catch (err) {
      console.error('[penny] parse error:', err)
      showToast('Network error', 'error')
    }
    finally { setChatLoading(false) }
  }

  const handleNPSSubmit = async () => {
    if (npsScore === null) return
    try {
      await fetch('/api/nps', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ score: npsScore, comment: npsComment }),
      })
      showToast('Thanks for your feedback!')
    } catch { /* ignore */ }
    setShowNPS(false)
    setNpsScore(null)
    setNpsStep(1)
    setNpsComment('')
  }

  const closeNPS = () => {
    setShowNPS(false)
    setNpsScore(null)
    setNpsStep(1)
    setNpsComment('')
    setNpsTypedText('')
    setNpsTypingDone(false)
  }

  // Typewriter effect for NPS questions
  useEffect(() => {
    if (!showNPS) return
    const question = npsStep === 1
      ? 'How likely are you to recommend Thriftly to a friend?'
      : `You picked ${npsScore}! What's the biggest reason for your score?`
    setNpsTypedText('')
    setNpsTypingDone(false)
    let i = 0
    const timer = setInterval(() => {
      i++
      setNpsTypedText(question.slice(0, i))
      if (i >= question.length) {
        clearInterval(timer)
        setNpsTypingDone(true)
      }
    }, 25)
    return () => clearInterval(timer)
  }, [showNPS, npsStep, npsScore])

  // Push a modal history entry so browser back closes the modal
  const pushModalState = () => {
    const path = SCREEN_PATHS[screen] || '/'
    window.history.pushState({ screen, modal: true }, '', path)
  }

  const openDetail = (item: ListItem) => {
    pushModalState()
    setSelectedItem(item)
    setEditName(item.name)
    setEditBrand(item.brand)
    setDisplayBrand(item.brand)
    setEditDept(item.department)
    setEditQty(item.quantity || 1)
    setEditSize(item.size)
    setEditNotes(item.notes || '')
    setItemHistory([])
    fetchAPI(`/api/list/${item.id}/history`)
      .then(data => setItemHistory(Array.isArray(data) ? data as ItemModification[] : []))
      .catch(() => {})
  }

  const openDetailFromReview = (item: ListItem) => {
    setShowReviewSheet(false)
    setReturnToReview(true)
    openDetail(item)
  }

  const popModalState = () => {
    // Pop the modal history entry (only if we're not already handling popstate)
    if (window.history.state?.modal) {
      window.history.back()
    }
  }

  const closeEditSheet = (deletedId?: number, updatedItem?: ListItem) => {
    popModalState()
    setSelectedItem(null)
    if (returnToReview) {
      setReturnToReview(false)
      if (updatedItem) {
        setLastAdded(prev => prev.map(i => i.id === updatedItem.id ? updatedItem : i))
      }
      if (deletedId) {
        setLastAdded(prev => {
          const remaining = prev.filter(i => i.id !== deletedId)
          if (remaining.length === 0 && lastSkipped.length === 0) return remaining
          return remaining
        })
        setLastSkipped(prev => prev.filter(s => s.existing_id !== deletedId))
      }
      // Reopen review if there are still items to review
      const hasAdded = deletedId ? lastAdded.some(i => i.id !== deletedId) : lastAdded.length > 0
      const hasSkipped = deletedId ? lastSkipped.some(s => s.existing_id !== deletedId) : lastSkipped.length > 0
      if (hasAdded || hasSkipped) {
        setTimeout(() => setShowReviewSheet(true), 100)
      }
    }
  }

  const handleEditSave = async () => {
    if (!selectedItem || !editName.trim() || editLoading) return
    setDisplayBrand(editBrand)
    setEditLoading(true)
    try {
      const data = await fetchAPI(`/api/list/${selectedItem.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: editName.trim(), brand: editBrand.trim(), department: editDept, quantity: editQty, size: editSize.trim(), notes: editNotes.trim() }),
      })
      if (data) {
        const updated = data as ListItem
        setItems(prev => sortItems(prev.map(item => item.id === updated.id ? updated : item)))
        closeEditSheet(undefined, updated)
      } else {
        showToast('Failed to save changes', 'error')
      }
    } catch { showToast('Failed to save changes', 'error') }
    finally { setEditLoading(false) }
  }

  const handleEditDelete = async () => {
    if (!selectedItem || editLoading) return
    const deletingId = selectedItem.id
    setEditLoading(true)
    try {
      const res = await fetch(`/api/list/${deletingId}`, { method: 'DELETE' })
      if (res.status === 401) { handleSessionExpired(); return }
      if (res.ok) {
        setItems(prev => prev.filter(item => item.id !== deletingId))
        closeEditSheet(deletingId)
      } else { showToast('Failed to delete item', 'error') }
    } catch { showToast('Failed to delete item', 'error') }
    finally { setEditLoading(false) }
  }

  const handleClearList = async () => {
    const cached = [...items]
    try {
      const res = await fetch('/api/list/clear', { method: 'POST' })
      if (res.status === 401) { handleSessionExpired(); return }
      if (!res.ok) { showToast('Failed to clear list', 'error'); return }
      setItems([]); setExpandedDepts(new Set()); setShowDoneToast(false); setShowFireworks(false); setPennyChimeIn(null)
      showToast('List cleared', 'success', {
        label: 'Undo',
        onClick: () => {
          fetch('/api/list/restore', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(cached.map(item => ({
              name: item.name, brand: item.brand, department: item.department,
              quantity: item.quantity, size: item.size, checked: item.checked, urgent: item.urgent,
            }))),
          }).then(() => reloadList())
        },
      })
    } catch { showToast('Failed to clear list', 'error') }
  }

  const handleSaveList = async () => {
    if (!saveNameInput.trim()) return
    try {
      const data = await fetchAPI('/api/saved-lists', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: saveNameInput.trim() }),
      })
      if (data) { setShowSaveSheet(false); setSaveNameInput(''); showToast('List saved!') }
      else showToast('Failed to save list', 'error')
    } catch { showToast('Failed to save list', 'error') }
  }

  const handleLoadSavedList = async (id: number, mode: 'merge' | 'overwrite' = 'merge') => {
    const url = mode === 'overwrite' ? `/api/saved-lists/${id}/load?mode=overwrite` : `/api/saved-lists/${id}/load`
    popModalState()
    setSelectedSavedList(null)
    try {
      const res = await fetch(url, { method: 'POST' })
      if (res.status === 401) { handleSessionExpired(); return }
      if (!res.ok) { showToast('Failed to load list', 'error'); return }
      setScreen('home')
      showToast(mode === 'overwrite' ? 'List replaced!' : 'List merged!')
    } catch { showToast('Failed to load list', 'error') }
  }

  const handleDeleteSavedList = async (id: number) => {
    try {
      const res = await fetch(`/api/saved-lists/${id}`, { method: 'DELETE' })
      if (res.status === 401) { handleSessionExpired(); return }
      if (res.ok) { setSelectedSavedList(null); loadSavedLists(); showToast('List deleted') }
    } catch { showToast('Failed to delete list', 'error') }
  }

  // =========================================================================
  // PHONE INPUT HANDLER
  //
  // Strips non-digits, caps at 10 characters. The display value is formatted
  // by formatPhone(), but we store only raw digits.
  // =========================================================================

  const handlePhoneChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    // Strip everything that isn't a digit
    let raw = e.target.value.replace(/\D/g, '')

    // If they typed or pasted a number starting with 1 (like 15551234567),
    // drop the leading 1 — we add +1 on the backend
    if (raw.length > 10 && raw.startsWith('1')) {
      raw = raw.slice(1)
    }

    setPhone(raw.slice(0, 10))
    setError('')
  }

  // =========================================================================
  // SEND CODE
  //
  // Called when the user taps "Send Code" (only enabled at 10 digits).
  // Posts the raw 10-digit number to the backend.
  // =========================================================================

  const handleSendCode = async () => {
    if (phone.length !== 10 || loading) return

    setLoading(true)
    setError('')

    try {
      const res = await fetch('/auth/send-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ phone_number: phone }),
      })

      const data = await res.json()

      if (res.ok) {
        setScreen('code')
        setCode(['', '', '', '', '', '']) // reset code inputs
      } else {
        setError(data.error || 'Failed to send code')
      }
    } catch {
      setError('Network error. Please try again.')
    } finally {
      setLoading(false)
    }
  }

// =========================================================================
  // VERIFY CODE (auto-called when 6 digits are entered)
  //
  // This function is called by handleCodeChange when all 6 boxes are filled.
  // It sends the code to the backend and either logs in or shows an error.
  // =========================================================================

  const verifyingRef = useRef(false)

  const verifyCode = useCallback(async (fullCode: string) => {
    // Prevent double-submission — Twilio codes are single-use
    if (verifyingRef.current) return
    verifyingRef.current = true

    setLoading(true)
    setError('')

    try {
      const res = await fetch('/auth/verify-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ phone_number: phone, code: fullCode }),
      })

      const data = await res.json()

      if (res.ok) {
        if (data.is_admin) setIsAdmin(true)
        if (data.user_id) setCurrentUserId(data.user_id)
        tryJoinCode()
        // Check for post-login redirect (deep link before auth)
        const redirect = sessionStorage.getItem('thriftly_redirect')
        if (redirect) {
          sessionStorage.removeItem('thriftly_redirect')
          const target = PATH_TO_SCREEN[redirect]
          setScreen((target || 'home') as Screen, true)
        } else {
          setScreen('home', true)
        }
      } else {
        setError(data.error || 'Invalid code')
        setCode(['', '', '', '', '', ''])
        setTimeout(() => codeInputRefs.current[0]?.focus(), 50)
      }
    } catch {
      setError('Network error. Please try again.')
      setCode(['', '', '', '', '', ''])
    } finally {
      setLoading(false)
      verifyingRef.current = false
    }
  }, [phone])

  // =========================================================================
  // CODE INPUT HANDLER
  //
  // Each of the 6 code boxes is a separate <input>. When a digit is entered:
  //   1. Store it in the code array at the right index
  //   2. Auto-advance focus to the next box
  //   3. If all 6 are filled, auto-submit immediately
  // =========================================================================

const handleCodeChange = (index: number, value: string) => {
    const digit = value.replace(/\D/g, '').slice(-1)

    const newCode = [...code]
    newCode[index] = digit
    setCode(newCode)
    setError('')

    if (digit && index < 5) {
      codeInputRefs.current[index + 1]?.focus()
    }

    // Auto-submit when all 6 digits are filled (one check, not two)
    if (digit && newCode.join('').length === 6) {
      verifyCode(newCode.join(''))
    }
  }

  // Handle backspace in code inputs: go back to previous box
  const handleCodeKeyDown = (index: number, e: React.KeyboardEvent) => {
    if (e.key === 'Backspace' && !code[index] && index > 0) {
      codeInputRefs.current[index - 1]?.focus()
    }
  }

  // Handle paste in code inputs: fill all boxes from clipboard
  const handleCodePaste = (e: React.ClipboardEvent) => {
    e.preventDefault()
    const pasted = e.clipboardData.getData('text').replace(/\D/g, '').slice(0, 6)
    if (pasted.length === 0) return

    const newCode = [...code]
    for (let i = 0; i < pasted.length && i < 6; i++) {
      newCode[i] = pasted[i]
    }
    setCode(newCode)

    // Focus the next empty box, or the last one
    const nextEmpty = newCode.findIndex(d => !d)
    if (nextEmpty >= 0) {
      codeInputRefs.current[nextEmpty]?.focus()
    } else {
      codeInputRefs.current[5]?.focus()
      // All filled — auto-submit
      verifyCode(newCode.join(''))
    }
  }

  // =========================================================================
  // SETTINGS HANDLERS
  // =========================================================================

  const handleSettingsSave = async () => {
    if (settingsLoading) return
    setSettingsLoading(true)
    try {
      const data = await fetchAPI('/api/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: [settingsFirstName.trim(), settingsLastName.trim()].filter(Boolean).join(' '), email: settingsEmail.trim(), address: settingsAddress.trim() }),
      })
      if (data) showToast('Settings saved!')
      else showToast('Failed to save', 'error')
    } catch { showToast('Network error', 'error') }
    finally { setSettingsLoading(false) }
  }

  const withInviteLink = async (onLink: (link: string) => void) => {
    if (inviteLoading) return
    setInviteLoading(true)
    try {
      const data = await fetchAPI('/api/settings/invite', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ phone: '' }),
      })
      if (data && !Array.isArray(data) && data.link) onLink(data.link as string)
      else showToast('Failed to create invite', 'error')
    } catch { showToast('Network error', 'error') }
    finally { setInviteLoading(false) }
  }

  const handleTextInviteLink = () => withInviteLink(async link => {
    const message = `Join our household shopping list on Thriftly! ${link}`
    // On mobile with share API, use native share; on desktop, copy to clipboard
    if (navigator.share && /Mobi|Android/i.test(navigator.userAgent)) {
      navigator.share({ title: 'Join Thriftly', text: message }).catch(() => {})
    } else {
      try {
        await navigator.clipboard.writeText(link)
        showToast('Invite link copied!')
      } catch {
        // Fallback: open SMS
        window.location.href = `sms:?&body=${encodeURIComponent(message)}`
      }
    }
  })

  // =========================================================================
  // LOGOUT
  // =========================================================================

  const handleLogout = async () => {
    await fetch('/auth/logout', { method: 'POST' })
    setMenuOpen(false)
    setScreen('phone', true)
    setPhone('')
    setCode(['', '', '', '', '', ''])
    setError('')
  }

  // =========================================================================
  // LOADING STATE (checking existing session)
  // =========================================================================

  if (checking) {
    return (
      <div style={styles.container}>
        <div style={styles.card}>
          <Logo />
        </div>
      </div>
    )
  }

  // =========================================================================
  // PHONE SCREEN
  // =========================================================================

  if (screen === 'phone') {
    const isValid = phone.length === 10
    return (
      <div style={styles.container}>
        <div style={styles.card}>
          <Logo />
          <h1 style={styles.wordmark}>Shared shopping list</h1>

          {/* Penny chat area — fixed height so input/button below never shift */}
          <div style={{ width: '100%', marginBottom: '24px', height: '100px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', height: '100%' }}>
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', flexShrink: 0 }}>
                <PennyAvatar size={53} />
                <span style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '4px', fontWeight: 500 }}>Penny</span>
              </div>
              <div style={{ flex: 1, display: 'flex', alignItems: 'center' }}>
                {(pennyMessage || pennyTyping) && (
                  <div style={{
                    backgroundColor: BG, borderRadius: '16px', borderTopLeftRadius: '4px',
                    padding: '12px 16px', textAlign: 'left' as const,
                  }}>
                    <span style={{ fontFamily: FONT, fontSize: '15px', color: TEXT, lineHeight: 1.6 }}>
                      {pennyMessage}
                    </span>
                    {pennyTyping && <span className="cursor-blink" style={{ fontWeight: 300, marginLeft: '1px' }}>|</span>}
                  </div>
                )}
              </div>
            </div>
          </div>

          <div style={{ width: '100%', marginBottom: '16px' }}>
            <FloatingLabelField label="Phone number">
              <input
                ref={phoneInputRef}
                type="tel"
                inputMode="numeric"
                autoComplete="tel-national"
                placeholder="(555) 123-4567"
                value={formatPhone(phone)}
                onChange={handlePhoneChange}
                onKeyDown={e => { if (e.key === 'Enter' && isValid) handleSendCode() }}
                style={{ ...listStyles.floatingInput, fontSize: '20px', fontWeight: 500, letterSpacing: '0.5px' }}
                autoFocus
              />
            </FloatingLabelField>
            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', marginTop: '8px', paddingLeft: '16px', cursor: 'pointer', userSelect: 'none' as const }}>
              <input
                type="checkbox"
                checked={smsOptIn}
                onChange={e => setSmsOptIn(e.target.checked)}
                style={{ accentColor: BORDER, width: '14px', height: '14px', flexShrink: 0, opacity: 0.7 }}
              />
              <span style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, opacity: 0.7 }}>
                I want to text Penny AI
              </span>
            </label>
          </div>

          {error && <p style={styles.error}>{error}</p>}

          <button
            onClick={handleSendCode}
            disabled={!isValid || !smsOptIn || loading}
            style={{
              ...styles.button,
              opacity: isValid && smsOptIn && !loading ? 1 : 0.4,
              cursor: isValid && smsOptIn && !loading ? 'pointer' : 'default',
            }}
          >
            {loading ? 'Sending…' : 'Send Code'}
          </button>

          <LegalLinks />
        </div>
      </div>
    )
  }

  // =========================================================================
  // CODE SCREEN
  // =========================================================================

  if (screen === 'code') {
    return (
      <div style={styles.container}>
        <div style={styles.card}>
          <Logo />

          <h1 style={styles.wordmark}>Shared shopping list</h1>

          {/* Penny chat area — fixed height to match phone screen */}
          <div style={{ width: '100%', marginBottom: '24px', height: '100px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', height: '100%' }}>
              <PennyAvatar size={53} />
              <div style={{ flex: 1, display: 'flex', alignItems: 'center' }}>
                {(pennyMessage || pennyTyping) && (
                  <div style={{
                    backgroundColor: BG, borderRadius: '16px', borderTopLeftRadius: '4px',
                    padding: '12px 16px', textAlign: 'left' as const,
                  }}>
                    <span style={{ fontFamily: FONT, fontSize: '15px', color: TEXT, lineHeight: 1.6 }}>
                      {pennyMessage}
                    </span>
                    {pennyTyping && <span className="cursor-blink" style={{ fontWeight: 300, marginLeft: '1px' }}>|</span>}
                  </div>
                )}
              </div>
            </div>
          </div>

          <p style={styles.codePrompt}>
            Enter the code sent to
          </p>
          <button
            onClick={() => { setScreen('phone'); setError('') }}
            style={styles.phoneDisplay}
          >
            {formatPhone(phone)}
          </button>

          <div style={styles.codeRow}>
            {code.map((digit, i) => (
              <input
                key={i}
                ref={el => { codeInputRefs.current[i] = el }}
                type="text"
                inputMode="numeric"
                pattern="[0-9]*"
                autoComplete={i === 0 ? 'one-time-code' : 'off'}
                maxLength={1}
                value={digit}
                onChange={e => handleCodeChange(i, e.target.value)}
                onKeyDown={e => handleCodeKeyDown(i, e)}
                onPaste={i === 0 ? handleCodePaste : undefined}
                disabled={loading}
                style={{
                  ...styles.codeBox,
                  borderColor: digit ? BRAND : BORDER,
                  ...(loading ? { opacity: 0.5 } : {}),
                }}
              />
            ))}
          </div>

          {loading && (
            <p style={styles.verifying}>Verifying…</p>
          )}

          {error && <p style={styles.error}>{error}</p>}

          <LegalLinks />
        </div>
      </div>
    )
  }

  // =========================================================================
  // SHARED HELPERS — menu and toast (extracted to avoid 5x duplication)
  // =========================================================================

  const renderMenu = (active: Screen) => (
    <div style={{ position: 'relative' }}>
      <button onClick={() => setMenuOpen(!menuOpen)} style={listStyles.hamburgerBtn}>
        <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
          <path d="M3 5h14M3 10h14M3 15h14" stroke={TEXT} strokeWidth="1.8" strokeLinecap="round"/>
        </svg>
      </button>
      {menuOpen && (
        <>
          <div style={listStyles.menuOverlay} onClick={() => setMenuOpen(false)} />
          <div style={listStyles.menuDropdown}>
            <button style={{ ...listStyles.menuItem(active === 'home'), display: 'flex', alignItems: 'center', gap: '10px' }} onClick={() => { setMenuOpen(false); setScreen('home') }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="9" cy="21" r="1"/><circle cx="20" cy="21" r="1"/><path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"/></svg>
              Shopping List
            </button>
            <button style={{ ...listStyles.menuItem(active === 'about'), display: 'flex', alignItems: 'center', gap: '10px' }} onClick={() => { setMenuOpen(false); setScreen('about') }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
              How to text Penny
            </button>
            <button style={{ ...listStyles.menuItem(active === 'settings-share'), display: 'flex', alignItems: 'center', gap: '10px' }} onClick={() => { setMenuOpen(false); setScreen('settings-share') }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 2C6.48 2 2 5.58 2 10c0 2.13 1.06 4.04 2.72 5.41L3 22l4.73-2.37C9.1 19.87 10.52 20 12 20c5.52 0 10-3.58 10-8S17.52 2 12 2z"/><path d="M8 10h.01M12 10h.01M16 10h.01"/></svg>
              Share & Earn
            </button>
            <button style={{ ...listStyles.menuItem(active === 'settings' || active === 'settings-profile' as Screen), display: 'flex', alignItems: 'center', gap: '10px' }} onClick={() => { setMenuOpen(false); setScreen('settings') }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
              Settings
            </button>
            {isAdmin && (
              <button style={{ ...listStyles.menuItem(active === 'admin'), display: 'flex', alignItems: 'center', gap: '10px' }} onClick={() => { setMenuOpen(false); setScreen('admin') }}>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="18" y="3" width="4" height="18"/><rect x="10" y="8" width="4" height="13"/><rect x="2" y="13" width="4" height="8"/></svg>
                Dashboard
              </button>
            )}
          </div>
        </>
      )}
    </div>
  )

  const renderMemberList = () => settingsMembers.map((m, i) => (
    <div key={i} style={{
      display: 'flex',
      alignItems: 'center',
      padding: '10px 0',
      borderBottom: `1px solid ${BORDER}`,
      gap: '10px',
    }}>
      <div style={avatarCircleStyle()}>
        {(m.name || m.phone || '?')[0].toUpperCase()}
      </div>
      <div>
        <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT }}>
          {m.name || formatPhone(m.phone)}
        </div>
        {m.name && <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>{formatPhone(m.phone)}</div>}
      </div>
    </div>
  ))

  const pennyBarStyle = {
    position: 'fixed' as const,
    bottom: 0,
    left: 0,
    right: 0,
    display: 'flex',
    alignItems: 'center',
    gap: '10px',
    padding: '14px 16px',
    backgroundColor: '#F5F3EF',
    borderTop: `1px solid ${BORDER}`,
    fontFamily: FONT,
    fontSize: '15px',
    color: TEXT,
    fontWeight: 500,
    zIndex: 40,
    boxShadow: '0 -2px 12px rgba(0,0,0,0.08)',
  } as React.CSSProperties

  const pennyBtnStyle = { background: BRAND, border: 'none', color: WHITE, fontFamily: FONT, fontSize: '12px', fontWeight: 600, cursor: 'pointer', padding: '5px 12px', borderRadius: '12px', flexShrink: 0 } as React.CSSProperties
  const pennyCloseStyle = { background: 'none', border: 'none', color: TEXT_SECONDARY, fontFamily: FONT, fontSize: '17px', cursor: 'pointer', padding: '0 2px', lineHeight: 1, flexShrink: 0, opacity: 0.6 } as React.CSSProperties

  const renderPennyToast = () => toast ? (
    <div style={{ ...pennyBarStyle, backgroundColor: toast.type === 'error' ? '#FEE2E2' : '#F5F3EF', color: toast.type === 'error' ? '#991B1B' : TEXT }}>
      <PennyAvatar size={53} />
      <span style={{ flex: 1 }}>{toast.message}</span>
      {toast.action && (
        <button
          onClick={() => { toast.action!.onClick(); setToast(null); if (toastTimer.current) clearTimeout(toastTimer.current) }}
          style={pennyBtnStyle}
        >{toast.action.label}</button>
      )}
      <button
        onClick={() => { setToast(null); if (toastTimer.current) clearTimeout(toastTimer.current) }}
        style={pennyCloseStyle}
      >×</button>
    </div>
  ) : null

  const pageIcons: Record<string, React.ReactNode> = {
    'Shopping List': <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="9" cy="21" r="1"/><circle cx="20" cy="21" r="1"/><path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"/></svg>,
    'How to text Penny': <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>,
    'Share & Earn': <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 2C6.48 2 2 5.58 2 10c0 2.13 1.06 4.04 2.72 5.41L3 22l4.73-2.37C9.1 19.87 10.52 20 12 20c5.52 0 10-3.58 10-8S17.52 2 12 2z"/><path d="M8 10h.01M12 10h.01M16 10h.01"/></svg>,
    'Settings': <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>,
    'Dashboard': <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="18" y="3" width="4" height="18"/><rect x="10" y="8" width="4" height="13"/><rect x="2" y="13" width="4" height="8"/></svg>,
    'Saved Lists': <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg>,
    'Profile': <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>,
  }

  const renderPageHeader = (pageName: string, active: Screen) => (
    <div style={{ flexShrink: 0, backgroundColor: WHITE, borderBottom: `1px solid ${BORDER}` }}>
      <div style={{ display: 'flex', alignItems: 'center', padding: '16px 20px', position: 'relative' }}>
        <span
          style={{ ...listStyles.headerWordmark, cursor: 'pointer', fontSize: '17px', fontWeight: 700 }}
          onClick={() => setScreen('home')}
        >thriftly</span>
        <span style={{
          position: 'absolute',
          left: '50%',
          transform: 'translateX(-50%)',
          fontFamily: FONT,
          fontSize: '15px',
          fontWeight: 600,
          color: TEXT,
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
        }}>{pageIcons[pageName]}{pageName}</span>
        <div style={{ marginLeft: 'auto' }}>{renderMenu(active)}</div>
      </div>
    </div>
  )

  // =========================================================================
  // SUBPAGE HEADER (with back arrow)
  // =========================================================================

  const renderSubpageHeader = (title: string, backScreen: Screen) => (
    <div style={{ flexShrink: 0, backgroundColor: WHITE, borderBottom: `1px solid ${BORDER}` }}>
      <div style={{ display: 'flex', alignItems: 'center', padding: '16px 20px' }}>
        <button
          onClick={() => window.history.back()}
          style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '4px 8px 4px 0', color: BRAND, fontSize: '20px', fontWeight: 600, display: 'flex', alignItems: 'center' }}
        >
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
            <path d="M13 4l-6 6 6 6" stroke={BRAND} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
        </button>
        <span style={{ fontFamily: FONT, fontSize: '17px', fontWeight: 600, color: TEXT, display: 'flex', alignItems: 'center', gap: '6px' }}>{pageIcons[title]}{title}</span>
        <div style={{ marginLeft: 'auto' }}>{renderMenu(backScreen)}</div>
      </div>
    </div>
  )

  // =========================================================================
  // SETTINGS HUB
  // =========================================================================

  if (screen === 'settings') {
    const settingsRows: { label: string; target: Screen; icon: React.ReactNode }[] = [
      { label: 'Profile', target: 'settings-profile', icon: pageIcons['Profile'] },
      { label: 'Saved Lists', target: 'saved', icon: pageIcons['Saved Lists'] },
    ]
    const chevronSvg = <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M6 3l5 5-5 5" stroke={TEXT_SECONDARY} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/></svg>
    return (
      <div style={listStyles.page}>
        {renderPageHeader('Settings', 'settings')}
        <div style={listStyles.listArea}>
          {settingsRows.map(row => (
            <button
              key={row.label}
              onClick={() => setScreen(row.target)}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '12px',
                width: '100%',
                padding: '16px 20px',
                background: 'none',
                border: 'none',
                borderBottom: `1px solid ${BORDER}`,
                cursor: 'pointer',
                textAlign: 'left' as const,
                WebkitTapHighlightColor: 'transparent',
              }}
            >
              <span style={{ color: BRAND, display: 'flex', alignItems: 'center' }}>{row.icon}</span>
              <span style={{ flex: 1, fontFamily: FONT, fontSize: '16px', fontWeight: 500, color: TEXT }}>{row.label}</span>
              {chevronSvg}
            </button>
          ))}
          <button
            onClick={handleLogout}
            style={{
              display: 'flex',
              alignItems: 'center',
              width: '100%',
              padding: '16px 20px',
              background: 'none',
              border: 'none',
              borderBottom: `1px solid ${BORDER}`,
              cursor: 'pointer',
              textAlign: 'left' as const,
              WebkitTapHighlightColor: 'transparent',
            }}
          >
            <span style={{ flex: 1, fontFamily: FONT, fontSize: '16px', fontWeight: 500, color: ERROR_RED }}>Sign Out</span>
          </button>
        </div>
        {renderPennyToast()}
      </div>
    )
  }

  // =========================================================================
  // SETTINGS — PROFILE
  // =========================================================================

  if (screen === 'settings-profile') {
    return (
      <div style={listStyles.page}>
        {renderSubpageHeader('Profile', 'settings')}
        <div style={listStyles.listArea}>
          <div style={{ padding: '20px', display: 'flex', flexDirection: 'column', gap: '20px' }}>
            <div style={{ display: 'flex', gap: '12px' }}>
              <div style={{ flex: 1 }}>
                <FloatingLabelField label="First name">
                  <input
                    type="text"
                    value={settingsFirstName}
                    onChange={e => setSettingsFirstName(e.target.value)}
                    placeholder="First"
                    style={listStyles.floatingInput}
                  />
                </FloatingLabelField>
              </div>
              <div style={{ flex: 1 }}>
                <FloatingLabelField label="Last name">
                  <input
                    type="text"
                    value={settingsLastName}
                    onChange={e => setSettingsLastName(e.target.value)}
                    placeholder="Last"
                    style={listStyles.floatingInput}
                  />
                </FloatingLabelField>
              </div>
            </div>
            <div>
              <FloatingLabelField label="Email">
                <input
                  type="email"
                  value={settingsEmail}
                  onChange={e => setSettingsEmail(e.target.value)}
                  placeholder="your@email.com"
                  style={listStyles.floatingInput}
                />
              </FloatingLabelField>
              <p style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '4px', paddingLeft: '4px' }}>
                Set your email to add items by emailing penny@list.thrift.ly
              </p>
            </div>
            <div>
              <FloatingLabelField label="Address">
                <input
                  id="address-autocomplete"
                  type="text"
                  value={settingsAddress}
                  onChange={e => setSettingsAddress(e.target.value)}
                  placeholder="Start typing your address…"
                  style={listStyles.floatingInput}
                />
              </FloatingLabelField>
              <p style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '4px', paddingLeft: '4px' }}>
                Used to find nearby stores and deals
              </p>
            </div>
            <button
              onClick={handleSettingsSave}
              disabled={settingsLoading}
              style={{
                ...listStyles.sheetButton,
                opacity: settingsLoading ? 0.5 : 1,
              }}
            >
              {settingsLoading ? 'Saving...' : 'Save'}
            </button>
          </div>
        </div>
        {renderPennyToast()}
      </div>
    )
  }

  // =========================================================================
  // SETTINGS — SHARE & EARN (combined with Household)
  // =========================================================================

  if (screen === 'settings-share') {
    const handleShare = () => {
      const url = `https://app.thrift.ly/r/${referralCode}`
      const text = `Try Thriftly — Penny AI helps your household organize the grocery list. Just text, email, or snap a photo and she handles the rest! ${url}`
      if (navigator.share) {
        navigator.share({ title: 'Thriftly', text }).catch(() => {})
      } else {
        window.open(`sms:?&body=${encodeURIComponent(text)}`, '_self')
      }
    }

    return (
      <div style={listStyles.page}>
        <ConfirmModal open={!!confirmModal} title={confirmModal?.title || ''} message={confirmModal?.message} confirmLabel={confirmModal?.confirmLabel} destructive={confirmModal?.destructive} onConfirm={runConfirmAction} onCancel={closeConfirmModal} />
        {renderPageHeader('Share & Earn', 'settings-share')}
        <div style={listStyles.listArea}>
          <div style={{ padding: '20px', display: 'flex', flexDirection: 'column', gap: '40px' }}>

            {/* ── Your Household section ── */}
            <div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={TEXT_SECONDARY} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
                <span style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 700, color: TEXT, textTransform: 'uppercase' as const, letterSpacing: '0.5px' }}>Your Household</span>
              </div>
              {settingsMembers.length > 0 && (
                <>
                  {renderMemberList()}
                  <button
                    onClick={() => openConfirmModal({
                      title: 'Leave Household',
                      message: 'You will be removed from this household.',
                      confirmLabel: 'Leave',
                      destructive: true,
                      onConfirm: () => {
                        fetch('/api/settings/leave-household', { method: 'POST' })
                          .then(res => {
                            if (res.status === 401) { handleSessionExpired(); return }
                            if (res.ok) {
                              showToast('Left household')
                              loadSettings()
                            } else {
                              res.json().then(d => showToast(d?.error || 'Failed', 'error')).catch(() => showToast('Failed', 'error'))
                            }
                          })
                          .catch(() => showToast('Network error', 'error'))
                      },
                    })}
                    style={{
                      marginTop: '12px',
                      padding: '10px',
                      fontFamily: FONT,
                      fontSize: '15px',
                      fontWeight: 500,
                      color: ERROR_RED,
                      backgroundColor: 'transparent',
                      border: `1px solid ${ERROR_RED}`,
                      borderRadius: '10px',
                      cursor: 'pointer',
                      width: '100%',
                    }}
                  >Leave Household</button>
                </>
              )}
              <button
                onClick={handleTextInviteLink}
                disabled={inviteLoading}
                style={{
                  ...listStyles.sheetButton,
                  marginTop: '12px',
                  opacity: inviteLoading ? 0.5 : 1,
                }}
              >
                {inviteLoading ? 'Creating...' : 'Invite a household member'}
              </button>
              <p style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, marginTop: '8px', lineHeight: 1.5 }}>
                Household members share the same shopping list
              </p>
            </div>

            {/* ── Friends who live elsewhere section ── */}
            <div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={TEXT_SECONDARY} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 2C6.48 2 2 5.58 2 10c0 2.13 1.06 4.04 2.72 5.41L3 22l4.73-2.37C9.1 19.87 10.52 20 12 20c5.52 0 10-3.58 10-8S17.52 2 12 2z"/><path d="M8 10h.01M12 10h.01M16 10h.01"/></svg>
                <span style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 700, color: TEXT, textTransform: 'uppercase' as const, letterSpacing: '0.5px' }}>Friends who live elsewhere</span>
              </div>

              <p style={{ fontFamily: FONT, fontSize: '15px', color: TEXT, margin: '0 0 16px 0', lineHeight: 1.6 }}>
                Share Thriftly with friends — they'll get their own list, and you'll both benefit as more people join.
              </p>

              <button onClick={handleShare} style={listStyles.sheetButton}>
                Share Thriftly with friends
              </button>

              {referralFriends.length > 0 && (
                <div style={{ backgroundColor: BG, borderRadius: '12px', padding: '12px 16px', textAlign: 'left', marginTop: '16px' }}>
                  <span style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: BRAND }}>{referralFriends.length}</span>
                  <span style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY, marginLeft: '8px' }}>{referralFriends.length === 1 ? 'friend joined' : 'friends joined'}</span>
                </div>
              )}

              {referralFriends.length > 0 && (
                <div style={{ marginTop: '16px' }}>
                  <SectionHeader label={`Friends (${referralFriends.length})`} />
                  {referralFriends.map((f, i) => (
                    <div key={i} style={{
                      display: 'flex',
                      alignItems: 'center',
                      padding: '10px 0',
                      borderBottom: `1px solid ${BORDER}`,
                      gap: '10px',
                    }}>
                      <div style={avatarCircleStyle()}>
                        {(f.name || '?')[0].toUpperCase()}
                      </div>
                      <div style={{ flex: 1 }}>
                        <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT }}>{f.name || 'Friend'}</div>
                        <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>{new Date(f.created_at).toLocaleDateString()}</div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>


          </div>
        </div>
        {renderPennyToast()}
      </div>
    )
  }

  // =========================================================================
  // TEXT PENNY SCREEN
  // =========================================================================

  if (screen === 'about') {
    return (
      <div style={listStyles.page}>
        {renderPageHeader('How to text Penny', 'about')}

        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', justifyContent: 'center', padding: '20px', gap: '28px' }}>

            {/* Penny with chat bubble */}
            <div style={{ display: 'flex', alignItems: 'flex-start', gap: '12px', minHeight: '60px' }}>
              <PennyAvatar size={53} />
              <div style={{ flex: 1, minHeight: '48px', display: 'flex', alignItems: 'center' }}>
                <div style={{
                  backgroundColor: BG, borderRadius: '16px', borderTopLeftRadius: '4px',
                  padding: '12px 16px', textAlign: 'left' as const, minHeight: '24px',
                }}>
                  <span style={{ fontFamily: FONT, fontSize: '15px', color: TEXT, lineHeight: 1.6 }}>
                    {pennyMessage}
                  </span>
                  {pennyTyping && <span className="cursor-blink" style={{ fontWeight: 300, marginLeft: '1px' }}>|</span>}
                </div>
              </div>
            </div>

            {/* How it works */}
            <ol style={{ fontFamily: FONT, fontSize: '15px', color: TEXT, margin: 0, paddingLeft: '24px', lineHeight: '1.8' }}>
              <li style={{ marginBottom: '8px' }}><strong>Add Penny to your contacts</strong> using the button below</li>
              <li style={{ marginBottom: '8px' }}><strong>Text her what you need</strong> — a grocery list, a photo of your fridge, pantry, or a recipe</li>
              <li><strong>She adds it to your list</strong> — works in group chats with your household too</li>
            </ol>
          </div>

          {/* Anchored bottom */}
          <div style={{ flexShrink: 0, padding: '0 20px 20px', display: 'flex', flexDirection: 'column', gap: '12px', paddingBottom: 'max(20px, env(safe-area-inset-bottom))' }}>
            <a
              href="/api/penny.vcf"
              style={{
                display: 'block',
                textAlign: 'center',
                padding: '13px',
                fontFamily: FONT,
                fontSize: '15px',
                fontWeight: 600,
                color: WHITE,
                backgroundColor: BRAND,
                borderRadius: '12px',
                textDecoration: 'none',
                cursor: 'pointer',
                border: 'none',
                boxSizing: 'border-box',
              }}
            >
              Add Penny to Contacts
            </a>

            {!settingsEmail && (
              <p style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, textAlign: 'center', margin: 0 }}>
                Set your email in Settings to email Penny
              </p>
            )}
          </div>
        </div>

        {renderPennyToast()}
      </div>
    )
  }


  // =========================================================================
  // SAVED LISTS SCREEN
  // =========================================================================

  if (screen === 'saved') {
    return (
      <div style={listStyles.page}>
        <ConfirmModal open={!!confirmModal} title={confirmModal?.title || ''} message={confirmModal?.message} confirmLabel={confirmModal?.confirmLabel} destructive={confirmModal?.destructive} onConfirm={runConfirmAction} onCancel={closeConfirmModal} />
        {renderSubpageHeader('Saved Lists', 'settings')}

        {/* Saved lists content */}
        <div style={listStyles.listArea}>
          {savedLists.length === 0 ? (
            <div style={listStyles.emptyState}>
              <p style={listStyles.emptyText}>No saved lists yet.</p>
              <p style={listStyles.emptySubtext}>Save your current list to reuse it later.</p>
            </div>
          ) : (
            savedLists.map(sl => (
              <button
                key={sl.id}
                onClick={() => { pushModalState(); setSelectedSavedList(sl) }}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  width: '100%',
                  padding: '14px 20px',
                  background: 'none',
                  border: 'none',
                  borderBottom: `1px solid ${BORDER}`,
                  cursor: 'pointer',
                  textAlign: 'left' as const,
                  WebkitTapHighlightColor: 'transparent',
                  gap: '12px',
                }}
              >
                <div style={{ flex: 1 }}>
                  <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 500, color: TEXT }}>
                    {sl.name}
                  </div>
                  <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '2px' }}>
                    {sl.item_count} item{sl.item_count !== 1 ? 's' : ''} · {new Date(sl.created_at).toLocaleDateString()}
                  </div>
                </div>
              </button>
            ))
          )}
        </div>

        {renderPennyToast()}

        {/* Saved list action sheet */}
        {selectedSavedList && (
          <ActionSheet isOpen={true} onClose={() => { popModalState(); setSelectedSavedList(null) }} title={selectedSavedList.name}>
              <p style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY, margin: '0 0 8px 0' }}>
                {selectedSavedList.item_count} item{selectedSavedList.item_count !== 1 ? 's' : ''}
              </p>
              <button
                onClick={() => handleLoadSavedList(selectedSavedList.id, 'overwrite')}
                style={listStyles.sheetButton}
              >Replace Current List</button>
              <button
                onClick={() => handleLoadSavedList(selectedSavedList.id, 'merge')}
                style={{
                  ...listStyles.sheetButton,
                  backgroundColor: 'transparent',
                  color: BRAND,
                  border: `1.5px solid ${BRAND}`,
                }}
              >Merge into Current List</button>
              <button
                onClick={() => openConfirmModal({
                  title: 'Delete List',
                  message: `Delete "${selectedSavedList.name}"? This cannot be undone.`,
                  confirmLabel: 'Delete',
                  destructive: true,
                  onConfirm: () => handleDeleteSavedList(selectedSavedList.id),
                })}
                style={listStyles.sheetDeleteBtn}
              >Delete List</button>
          </ActionSheet>
        )}
      </div>
    )
  }

  // =========================================================================
  // ADMIN DASHBOARD SCREEN
  // =========================================================================

  if (screen === 'admin') {
    const dayOptions = [7, 30, 90]
    return (
      <div style={listStyles.page}>
        {renderPageHeader('Dashboard', 'admin')}
        <div style={{ ...listStyles.listArea, padding: '20px' }}>
          {/* Time period selector */}
          <div style={{ display: 'flex', gap: '8px', marginBottom: '20px' }}>
            {dayOptions.map(d => (
              <button key={d} onClick={() => setAdminDays(d)} style={{
                flex: 1, padding: '8px', borderRadius: '8px', border: `1px solid ${BORDER}`,
                backgroundColor: adminDays === d ? BRAND : WHITE, color: adminDays === d ? WHITE : TEXT,
                fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer',
              }}>{d}d</button>
            ))}
          </div>

          {adminData ? (
            <>
              {/* Metric cards */}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '32px' }}>
                {[
                  { label: 'Active Users', value: adminData.active_users },
                  { label: 'Total Users', value: adminData.total_users },
                  { label: 'New Users', value: adminData.new_users },
                  { label: 'K-Factor', value: adminData.k_factor.toFixed(2) },
                ].map(m => (
                  <div key={m.label} style={{
                    backgroundColor: WHITE, borderRadius: '12px', padding: '16px',
                    border: `1px solid ${BORDER}`,
                  }}>
                    <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginBottom: '4px' }}>{m.label}</div>
                    <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: TEXT }}>{m.value}</div>
                  </div>
                ))}
              </div>

              {/* Leaderboard */}
              <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginBottom: '12px' }}>Referral Leaderboard</div>
              {adminData.leaderboard.length === 0 ? (
                <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY }}>No referrals yet.</div>
              ) : (
                adminData.leaderboard.map((entry, i) => (
                  <div key={i} style={{
                    display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                    padding: '12px 0', borderBottom: `1px solid ${BORDER}`,
                  }}>
                    <div>
                      <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT }}>
                        {entry.name || entry.phone}
                      </div>
                      {entry.name && <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>{entry.phone}</div>}
                    </div>
                    <div style={{ textAlign: 'right' }}>
                      <div style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: BRAND }}>{entry.points} pts</div>
                      <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>{entry.referrals} referrals</div>
                    </div>
                  </div>
                ))
              )}

              {/* NPS Responses */}
              <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginTop: '28px', marginBottom: '12px' }}>
                NPS: {adminData.nps_score.toFixed(0)} ({adminData.nps_total} responses, {adminData.nps_promoters} promoters, {adminData.nps_detractors} detractors)
              </div>
              {adminData.nps_responses.length === 0 ? (
                <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY }}>No NPS responses yet.</div>
              ) : (
                adminData.nps_responses.map((resp, i) => (
                  <div key={i} style={{
                    padding: '12px 0', borderBottom: `1px solid ${BORDER}`,
                  }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <div style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: TEXT }}>
                        Score: {resp.score}
                      </div>
                      <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>
                        {new Date(resp.created_at).toLocaleDateString()}
                      </div>
                    </div>
                    {resp.comment && (
                      <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT, marginTop: '4px' }}>
                        {resp.comment}
                      </div>
                    )}
                    <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '2px' }}>
                      {resp.user_name || resp.user_phone}
                    </div>
                  </div>
                ))
              )}
            </>
          ) : (
            <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY, textAlign: 'center', padding: '40px 0' }}>Loading...</div>
          )}
        </div>
        {renderPennyToast()}
      </div>
    )
  }

  // =========================================================================
  // HOME SCREEN — Shopping list
  // =========================================================================

  const handleDeptClick = (dept: string) => {
    setExpandedDepts(prev => {
      const next = new Set(prev)
      if (next.has(dept)) {
        next.delete(dept)
      } else {
        next.add(dept)
        setTimeout(() => {
          deptRefs.current[dept]?.scrollIntoView({ behavior: 'smooth', block: 'start' })
        }, 0)
      }
      return next
    })
  }

  // Departments that have items, in store-aisle order
  const activeDepts = DEPARTMENTS.filter(d => items.some(i => i.department === d))

  return (
    <div style={listStyles.page}>
      <ConfirmModal open={!!confirmModal} title={confirmModal?.title || ''} message={confirmModal?.message} confirmLabel={confirmModal?.confirmLabel} destructive={confirmModal?.destructive} onConfirm={runConfirmAction} onCancel={closeConfirmModal} />
      {/* Fixed header — compact */}
      <div style={{ flexShrink: 0, backgroundColor: WHITE, borderBottom: `1px solid ${BORDER}` }}>
        <div style={{ display: 'flex', alignItems: 'center', padding: '16px 20px', position: 'relative' }}>
          <span
            style={{ ...listStyles.headerWordmark, cursor: 'pointer', fontSize: '17px', fontWeight: 700 }}
            onClick={() => setScreen('home')}
          >thriftly</span>
          <span style={{
            position: 'absolute',
            left: '50%',
            transform: 'translateX(-50%)',
            fontFamily: FONT,
            fontSize: '15px',
            fontWeight: 600,
            color: TEXT,
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
          }}>{pageIcons['Shopping List']} Shopping List</span>
          <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '4px' }}>
            <button
              onClick={() => { pushModalState(); setShowTripSheet(true); if (!activeTrips.some(t => t.is_mine)) { setTripDay(''); setTripTimeLabel(''); setTripExactTime('') } }}
              style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '4px', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
              title="Plan a shopping trip"
            >
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={activeTrips.length > 0 ? BRAND : TEXT_SECONDARY} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
              </svg>
            </button>
            {renderMenu('home')}
          </div>
        </div>
        {/* Progress bar */}
        {items.length > 0 && (() => {
          const totalQty = items.reduce((s, i) => s + (i.quantity || 1), 0)
          const checkedQty = items.filter(i => i.checked).reduce((s, i) => s + (i.quantity || 1), 0)
          const pct = Math.round((checkedQty / totalQty) * 100)
          return (
            <div style={{ height: '3px', backgroundColor: BORDER }}>
              <div style={{
                height: '100%',
                width: `${pct}%`,
                backgroundColor: BRAND,
                borderRadius: '0 2px 2px 0',
                transition: 'width 0.3s ease',
              }} />
            </div>
          )
        })()}
      </div>

      {/* Trip banner */}
      {activeTrips.length > 0 && (
        <div style={{ flexShrink: 0, padding: '8px 16px', backgroundColor: WHITE, borderBottom: `1px solid ${BORDER}` }}>
          {activeTrips.map(trip => (
            <div key={trip.id} style={{
              display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px',
              borderLeft: `3px solid ${trip.is_mine ? BRAND : BORDER}`, borderRadius: '4px',
              backgroundColor: BG, marginBottom: activeTrips.indexOf(trip) < activeTrips.length - 1 ? '4px' : 0,
            }}>
              {isTripInProgress(trip) && <span style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: '#4CAF50', flexShrink: 0 }} />}
              <span style={{ flex: 1, fontFamily: FONT, fontSize: '15px', color: TEXT }}>
                <strong>{trip.is_mine ? 'You' : (trip.user_name || 'Someone')}</strong>
                {' — '}
                {isTripInProgress(trip) ? 'Shopping now!' : formatTripTime(trip)}
              </span>
              {trip.is_mine && (
                <div style={{ display: 'flex', gap: '4px' }}>
                  <button
                    onClick={handleCancelTrip}
                    style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '4px', color: TEXT_SECONDARY, display: 'flex' }}
                  >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><path d="M1 1l12 12M13 1L1 13" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/></svg>
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Scrollable list */}
      <div ref={listAreaRef} style={listStyles.listArea}>
        {chatLoading && (
          <div style={{
            position: 'absolute',
            inset: 0,
            backgroundColor: 'rgba(0,0,0,0.04)',
            zIndex: 10,
            pointerEvents: 'auto',
          }} />
        )}
        {items.length === 0 ? (
          <div style={listStyles.emptyState}>
            <p style={listStyles.emptyText}>Your list is empty.</p>
            <p style={listStyles.emptySubtext}>Tell Penny what you need.</p>
          </div>
        ) : (
          activeDepts.map((dept, deptIdx) => {
            const deptItems = items.filter(i => i.department === dept)
            const deptTotal = deptItems.reduce((s, i) => s + (i.quantity || 1), 0)
            const deptChecked = deptItems.filter(i => i.checked).reduce((s, i) => s + (i.quantity || 1), 0)
            const deptDone = deptChecked === deptTotal
            const isOpen = expandedDepts.has(dept)
            return (
              <div key={dept} ref={el => { deptRefs.current[dept] = el }}>
                <button
                  style={listStyles.deptHeader(isOpen, deptIdx === 0)}
                  onClick={() => handleDeptClick(dept)}
                >
                  <span style={{ width: '52px', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0, paddingLeft: '9px', boxSizing: 'border-box' }}>
                    {deptDone ? <span style={{ color: BRAND, fontSize: '15px' }}>✓</span> : <DeptIcon dept={dept} />}
                  </span>
                  <span style={deptDone ? { textDecoration: 'line-through', opacity: 0.5 } : undefined}>{dept}</span>
                  <span style={listStyles.deptCount}>{deptChecked}/{deptTotal}</span>
                  <span style={listStyles.deptChevron(isOpen)}>›</span>
                </button>
                {isOpen && deptItems.map((item) => {
                  return (
                  <div key={item.id} style={{
                    ...listStyles.itemRow,
                    ...(item.checked ? { backgroundColor: '#F5F4F1', opacity: 0.7 } : {}),
                    ...(newItemIds.has(item.id) ? {
                      backgroundColor: `${BRAND}15`,
                      transition: 'background-color 1.5s ease',
                    } : {}),
                    ...(!item.checked && item.urgent ? { backgroundColor: '#FEF2F2' } : {}),
                  }}>
                    <button onClick={() => handleToggle(item.id)} style={listStyles.itemCheckBtn}>
                      <span style={listStyles.itemCheck(item.checked)}>
                        {item.checked && '✓'}
                      </span>
                    </button>
                    <div style={{ marginRight: '10px' }}>
                      <BrandLogo brand={item.brand} size={72} opacity={item.checked ? 0.4 : 1} />
                    </div>
                    <div style={listStyles.itemCenter} onClick={() => handleToggle(item.id)}>
                      <span style={listStyles.itemName(item.checked)}>
                        {item.brand ? `${item.brand} ` : ''}{item.name}
                      </span>
                      {(item.quantity > 1 || item.size || item.notes) && (
                        <span style={listStyles.itemMeta}>
                          {item.quantity > 1 ? `×${item.quantity}` : ''}{item.quantity > 1 && item.size ? ' · ' : ''}{item.size}{(item.quantity > 1 || item.size) && item.notes ? ' · ' : ''}{item.notes}
                        </span>
                      )}
                    </div>
                    {(item.urgent || item.claimed_by_id) && (
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', alignItems: 'flex-end', flexShrink: 0 }}>
                        {item.urgent && (
                          <span style={{
                            backgroundColor: '#FEF2F2', color: '#E53E3E', border: '1px solid #E53E3E40',
                            fontSize: '12px', fontWeight: 600, borderRadius: '10px', padding: '2px 8px',
                            whiteSpace: 'nowrap',
                          }}>🔥 urgent</span>
                        )}
                        {item.claimed_by_id && (
                          <span style={{
                            fontSize: '12px', fontWeight: 600, borderRadius: '10px', padding: '2px 8px',
                            whiteSpace: 'nowrap',
                            ...(item.claimed_by_id === currentUserId
                              ? { backgroundColor: '#F0FFF4', color: '#276749', border: '1px solid #27674940' }
                              : { backgroundColor: '#F7F6F3', color: TEXT, border: `1px solid ${BORDER}` }),
                          }}>
                            {item.claimed_by_id === currentUserId ? 'Me' : (item.claimed_by_name?.[0] || '?')}
                          </span>
                        )}
                      </div>
                    )}
                    <button onClick={() => openDetail(item)} style={listStyles.itemEditBtn}>
                      <span style={{ width: '32px', height: '32px', borderRadius: '50%', backgroundColor: BG, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                          <path d="M11.5 1.5a1.77 1.77 0 0 1 2.5 0 1.77 1.77 0 0 1 0 2.5L5.5 12.5 1.5 14l1.5-4L11.5 1.5z" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"/>
                          <path d="M10 3.5l2.5 2.5" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"/>
                        </svg>
                      </span>
                    </button>
                  </div>
                  )
                })}
              </div>
            )
          })
        )}
      </div>


      {/* Confetti on list completion */}
      {showFireworks && <Confetti onComplete={() => setShowFireworks(false)} />}

      {/* "You're done!" overlay */}
      {showDoneOverlay && (
        <div
          style={{
            position: 'fixed', inset: 0, zIndex: 9998,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            backgroundColor: 'rgba(0,0,0,0.3)',
            animation: 'done-overlay 3s ease forwards',
            pointerEvents: 'none',
          }}
          onAnimationEnd={() => setShowDoneOverlay(false)}
        >
          <span style={{ fontSize: '36px', fontWeight: 700, color: WHITE, fontFamily: FONT, textShadow: '0 2px 12px rgba(0,0,0,0.3)' }}>
            You're done! 🎉
          </span>
        </div>
      )}

      {/* Floating "+N ItemName" drift animations */}
      {floatingLabels.map((label, i) => (
        <div key={label.key} style={{
          position: 'fixed', bottom: '100px', left: '50%', transform: 'translateX(-50%)',
          animation: 'float-up 3000ms cubic-bezier(0.2, 0.6, 0.3, 1) forwards',
          animationDelay: `${i * 600}ms`,
          fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: BRAND,
          backgroundColor: WHITE, padding: '6px 14px', borderRadius: '20px',
          boxShadow: '0 2px 8px rgba(0,0,0,0.12)', zIndex: 40,
          pointerEvents: 'none', opacity: 0,
        }} onAnimationEnd={() => setFloatingLabels(prev => prev.filter(l => l.key !== label.key))}>
          {label.text}
        </div>
      ))}

      {/* "All done" toast — rendered inline below in the penny status bar area */}


      {/* Hidden file input for camera/photo */}
      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        multiple
        onChange={handlePhotoSelect}
        style={{ display: 'none' }}
      />

      {/* Penny — always visible above chat */}
      <div style={{ flexShrink: 0, display: 'flex', alignItems: 'center', gap: '10px', padding: '10px 16px', backgroundColor: '#F5F3EF', borderTop: `1px solid ${BORDER}` }}>
        <PennyAvatar size={53} />
        <span style={{ flex: 1, fontFamily: FONT, fontSize: '14px', color: toast?.type === 'error' ? '#991B1B' : TEXT, fontWeight: 500, minHeight: '20px' }}>
          {homePennyText}
          {homePennyTyping && <span className="cursor-blink" style={{ fontWeight: 300, marginLeft: '1px' }}>|</span>}
        </span>
        {pennyStatus && <span className="penny-pulse" style={{ width: '6px', height: '6px', borderRadius: '50%', backgroundColor: BRAND, flexShrink: 0 }} />}
        {homePennyBtns.map((btn, i) => (
          <button key={i} onClick={btn.onClick} style={pennyBtnStyle}>{btn.label}</button>
        ))}
      </div>

      {/* Share FAB — only when user has no household members */}
      {settingsMembers.length === 0 && (
        <button
          onClick={() => setScreen('settings-share')}
          style={{
            position: 'absolute',
            bottom: '150px',
            right: '56px',
            width: '48px',
            height: '48px',
            borderRadius: '50%',
            backgroundColor: BRAND,
            border: 'none',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            boxShadow: '0 2px 8px rgba(0,0,0,0.2)',
            zIndex: 20,
          }}
          title="Invite household members"
        >
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
            <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            <circle cx="9" cy="7" r="4" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            <line x1="19" y1="8" x2="19" y2="14" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            <line x1="22" y1="11" x2="16" y2="11" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
        </button>
      )}

      {/* Thumbnail strip above chat bar */}
      {pendingImages.length > 0 && (
        <div style={{ display: 'flex', gap: '8px', padding: '8px 12px', overflowX: 'auto', backgroundColor: WHITE, borderTop: `1px solid ${BORDER}`, flexShrink: 0 }}>
          {pendingImages.map((img, idx) => (
            <div key={idx} style={{ position: 'relative', flexShrink: 0 }}>
              <img src={img} alt={`Preview ${idx + 1}`} style={{
                width: '56px',
                height: '56px',
                borderRadius: '8px',
                objectFit: 'cover',
              }} />
              <button
                onClick={() => setPendingImages(prev => prev.filter((_, i) => i !== idx))}
                style={{
                  position: 'absolute',
                  top: '-6px',
                  right: '-6px',
                  width: '18px',
                  height: '18px',
                  borderRadius: '50%',
                  backgroundColor: TEXT,
                  color: WHITE,
                  border: 'none',
                  fontSize: '12px',
                  lineHeight: '18px',
                  textAlign: 'center',
                  cursor: 'pointer',
                  padding: 0,
                }}
              >×</button>
            </div>
          ))}
        </div>
      )}

      {/* Bottom chat bar — flex item, anchors above keyboard via 100dvh */}
      <div style={listStyles.chatBar}>
        <button
          style={listStyles.chatIconBtn}
          title="Add photo"
          onClick={() => fileInputRef.current?.click()}
        >
          <svg width="22" height="22" viewBox="0 0 22 22" fill="none">
            <rect x="1" y="5" width="20" height="14" rx="3" stroke={TEXT_SECONDARY} strokeWidth="1.5"/>
            <circle cx="11" cy="12" r="3.5" stroke={TEXT_SECONDARY} strokeWidth="1.5"/>
            <path d="M7 5l1.5-2h5L15 5" stroke={TEXT_SECONDARY} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
        </button>
        <textarea
          ref={chatInputRef}
          rows={1}
          placeholder="Tell Penny what you need…"
          value={chatText}
          onChange={e => {
            setChatText(e.target.value)
            e.target.style.height = 'auto'
            e.target.style.height = Math.min(e.target.scrollHeight, 120) + 'px'
          }}
          onKeyDown={e => {
            if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSendChat() }
          }}
          onPaste={handlePaste}
          disabled={chatLoading}
          style={{ ...listStyles.chatInput, resize: 'none' as const, overflowY: 'hidden' as const, maxHeight: '120px' }}
        />
        <button
          onClick={handleSendChat}
          disabled={(!chatText.trim() && pendingImages.length === 0) || chatLoading}
          style={listStyles.chatSendBtn((!!chatText.trim() || pendingImages.length > 0) && !chatLoading)}
        >
          {chatLoading
            ? <span style={{ fontSize: '12px', fontFamily: FONT }}>…</span>
            : <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
                <path d="M2 9h14M10 3l6 6-6 6" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
          }
        </button>
      </div>

      {/* Trip planner bottom sheet */}
      {showTripSheet && (() => {
        const myTrip = activeTrips.find(t => t.is_mine)
        if (myTrip) {
          return (
            <ActionSheet isOpen={true} onClose={() => { popModalState(); setShowTripSheet(false) }} title="Your planned trip" maxHeight="60dvh">
              <div style={{ textAlign: 'center', padding: '8px 0 16px' }}>
                <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke={BRAND} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
                </svg>
                <p style={{ fontFamily: FONT, fontSize: '17px', fontWeight: 600, color: TEXT, margin: '12px 0 4px' }}>
                  {isTripInProgress(myTrip) ? 'Shopping now!' : formatTripTime(myTrip)}
                </p>
                <p style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY, margin: 0 }}>
                  Your household can see this trip.
                </p>
              </div>
              <div style={{ display: 'flex', gap: '8px', marginTop: '8px' }}>
                <button
                  onClick={() => { handleCancelTrip(); setShowTripSheet(false) }}
                  style={{ ...listStyles.sheetButton, flex: 1, backgroundColor: '#FEE2E2', color: '#DC2626', border: 'none' }}
                >Cancel Trip</button>
              </div>
            </ActionSheet>
          )
        }
        return (
          <ActionSheet isOpen={true} onClose={() => { popModalState(); setShowTripSheet(false) }} title="Plan a shopping trip" maxHeight="60dvh">
            <p style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY, margin: '-4px 0 16px 0' }}>
              Let your household know when you're heading to the store.
            </p>

            <p style={{ fontFamily: FONT, fontSize: '12px', fontWeight: 600, color: TEXT, marginBottom: '8px' }}>When?</p>
            <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '16px' }}>
              {[
                { label: 'Today', value: todayStr },
                { label: 'Tomorrow', value: tomorrowStr },
                { label: 'This weekend', value: satStr },
              ].map(opt => (
                <button
                  key={opt.value}
                  onClick={() => setTripDay(opt.value)}
                  style={{
                    padding: '8px 16px', borderRadius: '20px', border: `1.5px solid ${tripDay === opt.value ? BRAND : BORDER}`,
                    backgroundColor: tripDay === opt.value ? BRAND : WHITE, color: tripDay === opt.value ? WHITE : TEXT,
                    fontFamily: FONT, fontSize: '15px', fontWeight: 500, cursor: 'pointer',
                  }}
                >{opt.label}</button>
              ))}
              <select
                value={getDayOptions().some(o => o.value === tripDay) ? tripDay : ''}
                onChange={e => setTripDay(e.target.value)}
                style={{
                  padding: '8px 12px', borderRadius: '20px', border: `1.5px solid ${BORDER}`,
                  backgroundColor: WHITE, fontFamily: FONT, fontSize: '15px', color: TEXT, cursor: 'pointer',
                }}
              >
                <option value="">Pick a day...</option>
                {getDayOptions().map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
              </select>
            </div>

            <p style={{ fontFamily: FONT, fontSize: '12px', fontWeight: 600, color: TEXT, marginBottom: '8px' }}>
              What time? <span style={{ fontWeight: 400, color: TEXT_SECONDARY }}>(optional)</span>
            </p>
            <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '12px' }}>
              {['morning', 'afternoon', 'evening', 'exact'].map(t => (
                <button
                  key={t}
                  onClick={() => setTripTimeLabel(tripTimeLabel === t ? '' : t)}
                  style={{
                    padding: '8px 16px', borderRadius: '20px', border: `1.5px solid ${tripTimeLabel === t ? BRAND : BORDER}`,
                    backgroundColor: tripTimeLabel === t ? BRAND : WHITE, color: tripTimeLabel === t ? WHITE : TEXT,
                    fontFamily: FONT, fontSize: '15px', fontWeight: 500, cursor: 'pointer', textTransform: 'capitalize',
                  }}
                >{t}</button>
              ))}
            </div>
            {tripTimeLabel === 'exact' && (
              <input
                type="time"
                value={tripExactTime}
                onChange={e => setTripExactTime(e.target.value)}
                style={{
                  padding: '10px 14px', borderRadius: '12px', border: `1.5px solid ${BORDER}`,
                  fontFamily: FONT, fontSize: '16px', color: TEXT, width: '100%', boxSizing: 'border-box',
                  marginBottom: '12px',
                }}
              />
            )}

            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', marginTop: '8px' }}>
              <button
                onClick={() => handleSetTrip(false)}
                disabled={!tripDay}
                style={{ ...listStyles.sheetButton, opacity: tripDay ? 1 : 0.4 }}
              >Save</button>
              <button
                onClick={() => setShowTripSheet(false)}
                style={{ ...listStyles.sheetButton, backgroundColor: 'transparent', color: TEXT_SECONDARY, border: 'none' }}
              >Cancel</button>
            </div>
          </ActionSheet>
        )
      })()}

      {/* NPS Survey Modal — Penny asks */}
      {showNPS && (
        <div onClick={closeNPS} style={{
          position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
          backgroundColor: 'rgba(0,0,0,0.5)', zIndex: 1000,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          <div onClick={e => e.stopPropagation()} style={{
            backgroundColor: WHITE, borderRadius: '16px', padding: '24px',
            width: '90%', maxWidth: '360px', position: 'relative',
          }}>
            <button onClick={closeNPS} style={{
              position: 'absolute', top: '12px', right: '12px',
              background: 'none', border: 'none', fontSize: '20px', color: TEXT_SECONDARY, cursor: 'pointer', zIndex: 1,
            }}>×</button>

            {/* Penny speech bubble */}
            <div style={{ display: 'flex', gap: '12px', marginBottom: '20px', paddingRight: '24px' }}>
              <PennyAvatar size={53} />
              <div style={{
                flex: 1, backgroundColor: BG, borderRadius: '14px', padding: '12px 14px',
                fontFamily: FONT, fontSize: '15px', color: TEXT, lineHeight: '1.4', position: 'relative',
              }}>
                {npsTypedText}{!npsTypingDone && <span style={{ opacity: 0.4 }}>|</span>}
              </div>
            </div>

            {/* Controls appear after typing finishes */}
            {npsTypingDone && (
              npsStep === 1 ? (
                <>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(11, 1fr)', gap: '4px', marginBottom: '8px' }}>
                    {Array.from({ length: 11 }, (_, i) => (
                      <button key={i} onClick={() => { setNpsScore(i); setNpsStep(2) }} style={{
                        height: '40px', borderRadius: '10px',
                        border: `1.5px solid ${BRAND}30`, backgroundColor: WHITE,
                        fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: BRAND,
                        cursor: 'pointer', padding: 0,
                      }}>{i}</button>
                    ))}
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>
                    <span>Not likely</span><span>Very likely</span>
                  </div>
                </>
              ) : (
                <>
                  <textarea
                    value={npsComment}
                    onChange={e => setNpsComment(e.target.value)}
                    placeholder={`Tell us why you chose ${npsScore}`}
                    rows={3}
                    autoFocus
                    style={{
                      width: '100%', padding: '10px', borderRadius: '8px',
                      border: `1px solid ${BORDER}`, fontFamily: FONT, fontSize: '15px',
                      resize: 'vertical', marginBottom: '16px', boxSizing: 'border-box',
                    }}
                  />
                  <button onClick={handleNPSSubmit} style={{
                    width: '100%', padding: '12px', borderRadius: '10px',
                    backgroundColor: BRAND, color: WHITE, border: 'none',
                    fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer',
                  }}>Submit</button>
                </>
              )
            )}
          </div>
        </div>
      )}

      {/* Item detail sheet */}
      {selectedItem && (
        <ActionSheet isOpen={true} onClose={() => closeEditSheet()} title="Edit item" maxHeight="75dvh">
            {/* Brand field with logo */}
            <div style={{ display: 'flex', alignItems: 'flex-end', gap: '12px', marginBottom: '0' }}>
              <div style={{ flex: 1 }}>
                <FloatingLabelField label="Brand">
                  <input
                    type="text"
                    value={editBrand}
                    onChange={e => setEditBrand(e.target.value)}
                    onBlur={() => setDisplayBrand(editBrand)}
                    placeholder="Optional"
                    style={listStyles.floatingInput}
                  />
                </FloatingLabelField>
              </div>
              <div style={{ marginBottom: '8px' }}>
                <BrandLogo brand={displayBrand || undefined} size={72} />
              </div>
            </div>

            <FloatingLabelField label="Name">
              <input
                type="text"
                value={editName}
                onChange={e => setEditName(e.target.value)}
                placeholder="Item name"
                style={listStyles.floatingInput}
                autoFocus
              />
            </FloatingLabelField>

            <FloatingLabelField label="Department">
              <select
                value={editDept}
                onChange={e => setEditDept(e.target.value)}
                style={listStyles.floatingSelect}
              >
                {DEPARTMENTS.map(d => <option key={d} value={d}>{d}</option>)}
              </select>
            </FloatingLabelField>

            <div style={listStyles.sheetRow}>
              <div style={{ flex: 1 }}>
                <FloatingLabelField label="Size">
                  <input
                    type="text"
                    list="size-suggestions"
                    value={editSize}
                    onChange={e => setEditSize(e.target.value)}
                    placeholder="e.g. 16 oz, 1 lb"
                    style={listStyles.floatingInput}
                  />
                </FloatingLabelField>
                <datalist id="size-suggestions">
                  {['4 oz','8 oz','12 oz','16 oz','24 oz','32 oz','1 lb','2 lb','5 lb','½ gal','1 gal','Small','Medium','Large','XL'].map(s => (
                    <option key={s} value={s} />
                  ))}
                </datalist>
              </div>
              <div>
                <FloatingLabelField label="Qty">
                  <QuantityControl value={editQty} onChange={setEditQty} />
                </FloatingLabelField>
              </div>
            </div>

            <FloatingLabelField label="Notes">
              <input
                type="text"
                value={editNotes}
                onChange={e => setEditNotes(e.target.value)}
                placeholder="e.g. Tangerine flavor, organic"
                style={listStyles.floatingInput}
              />
            </FloatingLabelField>

            {/* Urgent toggle */}
            {selectedItem && (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 0', borderTop: `1px solid ${BORDER}`, marginTop: '4px' }}>
                <span style={{ fontFamily: FONT, fontSize: '15px', color: TEXT }}>🔥 Urgent</span>
                <ToggleSwitch value={selectedItem.urgent} onChange={() => { handleUrgentToggle(selectedItem.id); setSelectedItem(prev => prev ? { ...prev, urgent: !prev.urgent } : prev) }} />
              </div>
            )}

            {/* Claim UI */}
            {selectedItem && (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 0', borderBottom: `1px solid ${BORDER}` }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={selectedItem.claimed_by_id ? BRAND : TEXT} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <circle cx="9" cy="21" r="1"/><circle cx="20" cy="21" r="1"/>
                    <path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"/>
                  </svg>
                  <span style={{ fontFamily: FONT, fontSize: '15px', color: TEXT }}>
                    {!selectedItem.claimed_by_id ? "I'll get it" : selectedItem.claimed_by_id === currentUserId ? "You're getting this" : `${selectedItem.claimed_by_name} is getting this`}
                  </span>
                </div>
                {(!selectedItem.claimed_by_id || selectedItem.claimed_by_id === currentUserId) && (
                  <ToggleSwitch
                    value={selectedItem.claimed_by_id === currentUserId}
                    onChange={() => {
                      const wasClaimed = selectedItem.claimed_by_id === currentUserId
                      handleClaim(selectedItem.id)
                      setSelectedItem(prev => prev ? { ...prev, claimed_by_id: wasClaimed ? null : currentUserId, claimed_by_name: wasClaimed ? '' : 'Me' } : prev)
                    }}
                  />
                )}
              </div>
            )}

            <div style={{ marginTop: '8px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <button
                onClick={handleEditSave}
                disabled={!editName.trim() || editLoading}
                style={{
                  ...listStyles.sheetButton,
                  opacity: editName.trim() && !editLoading ? 1 : 0.4,
                }}
              >
                {editLoading ? 'Saving…' : 'Save'}
              </button>

              <button onClick={handleEditDelete} disabled={editLoading} style={listStyles.sheetDeleteBtn}>
                Delete item
              </button>
            </div>

            {itemHistory.length > 0 && (
              <div style={{ marginTop: '12px', paddingTop: '12px', borderTop: `1px solid ${BORDER}` }}>
                <SectionHeader label="History" />
                {itemHistory.map((mod, i) => {
                  const name = mod.user_name || 'Someone'
                  const via = mod.source === 'sms' ? ' via text' : mod.source === 'email' ? ' via email' : ''
                  const ago = mod.created_at ? (() => {
                    const diff = Date.now() - new Date(mod.created_at).getTime()
                    const mins = Math.floor(diff / 60000)
                    if (mins < 1) return 'just now'
                    if (mins < 60) return `${mins}m ago`
                    const hrs = Math.floor(mins / 60)
                    if (hrs < 24) return `${hrs}h ago`
                    const days = Math.floor(hrs / 24)
                    if (days < 7) return `${days}d ago`
                    return new Date(mod.created_at).toLocaleDateString()
                  })() : ''
                  let description: string
                  switch (mod.action) {
                    case 'added': description = `${name} added this${via}`; break
                    case 'checked': description = `${name} checked this off${via}`; break
                    case 'unchecked': description = `${name} unchecked this${via}`; break
                    case 'updated': description = mod.detail ? `${name} changed ${mod.detail}${via}` : `${name} updated this${via}`; break
                    case 'claimed': description = `${name} claimed this`; break
                    case 'unclaimed': description = `${name} unclaimed this`; break
                    default: description = `${name} ${mod.action}${mod.detail ? ` ${mod.detail}` : ''}${via}`
                  }
                  return (
                    <div key={i} style={{
                      padding: '6px 0',
                      borderBottom: i < itemHistory.length - 1 ? `1px solid ${BORDER}` : 'none',
                      fontFamily: FONT,
                      fontSize: '12px',
                      color: TEXT_SECONDARY,
                    }}>
                      {description}
                      {ago && <span> · {ago}</span>}
                    </div>
                  )
                })}
              </div>
            )}
        </ActionSheet>
      )}

      {/* Review sheet — shows items from last parse */}
      {showReviewSheet && (lastAdded.length > 0 || lastSkipped.length > 0) && (
        <ActionSheet isOpen={true} onClose={() => { popModalState(); setShowReviewSheet(false); setLastAdded([]); setLastSkipped([]) }} title={lastAdded.length > 0 ? 'Added' : undefined}>

            {/* Added items section */}
            {lastAdded.length > 0 && (
              <>
                {lastAdded.map(item => (
                  <div key={item.id} style={{
                    display: 'flex',
                    alignItems: 'center',
                    padding: '10px 0',
                    borderBottom: `1px solid ${BORDER}`,
                    gap: '10px',
                  }}>
                    <BrandLogo brand={item.brand} size={52} />
                    <div style={{ flex: 1, cursor: 'pointer' }} onClick={() => openDetailFromReview(item)}>
                      <span style={{ fontFamily: FONT, fontSize: '16px', color: TEXT }}>
                        {item.brand ? `${item.brand} ` : ''}{item.name}
                      </span>
                      {item.size && (
                        <span style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginLeft: '6px' }}>
                          {item.size}
                        </span>
                      )}
                      <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '2px' }}>
                        {item.department} · <span style={{ color: BRAND, fontSize: '12px' }}>edit</span>
                      </div>
                    </div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '0', border: `1.5px solid ${BORDER}`, borderRadius: '10px', overflow: 'hidden', backgroundColor: BG, flexShrink: 0 }}>
                      <button
                        onClick={async () => {
                          const data = await fetchAPI(`/api/list/${item.id}`, {
                            method: 'PATCH', headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ quantity: Math.max(1, item.quantity - 1) }),
                          })
                          if (data) {
                            const updated = data as ListItem
                            setLastAdded(prev => prev.map(i => i.id === updated.id ? updated : i))
                            setItems(prev => sortItems(prev.map(i => i.id === updated.id ? updated : i)))
                          }
                        }}
                        style={qtyBtnStyle}
                      >−</button>
                      <span style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: TEXT, minWidth: '24px', textAlign: 'center' }}>{item.quantity}</span>
                      <button
                        onClick={async () => {
                          const data = await fetchAPI(`/api/list/${item.id}`, {
                            method: 'PATCH', headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ quantity: item.quantity + 1 }),
                          })
                          if (data) {
                            const updated = data as ListItem
                            setLastAdded(prev => prev.map(i => i.id === updated.id ? updated : i))
                            setItems(prev => sortItems(prev.map(i => i.id === updated.id ? updated : i)))
                          }
                        }}
                        style={qtyBtnStyle}
                      >+</button>
                    </div>
                    <button
                      onClick={async () => {
                        const delRes = await fetch(`/api/list/${item.id}`, { method: 'DELETE' })
                        if (delRes.status === 401) { handleSessionExpired(); return }
                        setItems(prev => prev.filter(i => i.id !== item.id))
                        const remaining = lastAdded.filter(i => i.id !== item.id)
                        setLastAdded(remaining)
                        if (remaining.length === 0 && lastSkipped.length === 0) setShowReviewSheet(false)
                      }}
                      style={{
                        flexShrink: 0,
                        width: '28px',
                        height: '28px',
                        borderRadius: '50%',
                        border: `1px solid ${BORDER}`,
                        background: 'none',
                        color: TEXT_SECONDARY,
                        fontSize: '15px',
                        cursor: 'pointer',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                      }}
                    >×</button>
                  </div>
                ))}
              </>
            )}

            {/* Skipped items section */}
            {lastSkipped.length > 0 && (
              <>
                <p style={{ ...listStyles.sheetTitle, marginTop: lastAdded.length > 0 ? '16px' : '0' } as React.CSSProperties}>Already on your list</p>
                {lastSkipped.map(skipped => {
                  const existing = items.find(i => i.id === skipped.existing_id)
                  return (
                    <div key={skipped.existing_id} style={{
                      display: 'flex',
                      alignItems: 'center',
                      padding: '10px 0',
                      borderBottom: `1px solid ${BORDER}`,
                      gap: '10px',
                    }}>
                      <BrandLogo brand={skipped.parsed.brand} size={52} />
                      <div style={{ flex: 1, cursor: existing ? 'pointer' : undefined }} onClick={() => existing && openDetailFromReview(existing)}>
                        <span style={{ fontFamily: FONT, fontSize: '16px', color: TEXT }}>
                          {skipped.parsed.brand ? `${skipped.parsed.brand} ` : ''}{skipped.parsed.name}
                        </span>
                        <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '2px' }}>
                          Currently: ×{existing?.quantity ?? 1}{existing && <> · <span style={{ color: BRAND, fontSize: '12px' }}>edit</span></>}
                        </div>
                      </div>
                      <button
                        onClick={async () => {
                          const data = await fetchAPI(`/api/list/${skipped.existing_id}`, {
                            method: 'PATCH', headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ quantity: (existing?.quantity ?? 1) + (skipped.parsed.quantity || 1) }),
                          })
                          if (data) {
                            const updated = data as ListItem
                            setItems(prev => sortItems(prev.map(i => i.id === updated.id ? updated : i)))
                            const remaining = lastSkipped.filter(s => s.existing_id !== skipped.existing_id)
                            setLastSkipped(remaining)
                            if (remaining.length === 0 && lastAdded.length === 0) setShowReviewSheet(false)
                          }
                        }}
                        style={{
                          flexShrink: 0,
                          padding: '6px 14px',
                          borderRadius: '16px',
                          border: `1.5px solid ${BRAND}`,
                          background: 'none',
                          color: BRAND,
                          fontFamily: FONT,
                          fontSize: '15px',
                          fontWeight: 600,
                          cursor: 'pointer',
                        }}
                      >+{skipped.parsed.quantity || 1}</button>
                    </div>
                  )
                })}
              </>
            )}

            {lastAdded.length > 0 && (
              <button
                onClick={async () => {
                  for (const item of lastAdded) {
                    const dr = await fetch(`/api/list/${item.id}`, { method: 'DELETE' })
                    if (dr.status === 401) { handleSessionExpired(); return }
                  }
                  setItems(prev => {
                    const ids = new Set(lastAdded.map(i => i.id))
                    return prev.filter(i => !ids.has(i.id))
                  })
                  setLastAdded([])
                  if (lastSkipped.length === 0) setShowReviewSheet(false)
                }}
                style={listStyles.sheetDeleteBtn}
              >
                Undo all
              </button>
            )}

            <button
              onClick={() => { setShowReviewSheet(false); setLastAdded([]); setLastSkipped([]) }}
              style={listStyles.sheetButton}
            >
              Done
            </button>
        </ActionSheet>
      )}

      {/* Save list sheet */}
      {showSaveSheet && (
        <ActionSheet isOpen={true} onClose={() => { popModalState(); setShowSaveSheet(false) }} title="Save List">
            <input
              type="text"
              value={saveNameInput}
              onChange={e => setSaveNameInput(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter' && saveNameInput.trim()) handleSaveList() }}
              placeholder="List name (e.g. Weekly Groceries)"
              style={listStyles.sheetTextInput}
              autoFocus
            />
            <button
              onClick={handleSaveList}
              disabled={!saveNameInput.trim()}
              style={{
                ...listStyles.sheetButton,
                opacity: saveNameInput.trim() ? 1 : 0.4,
              }}
            >Save</button>
        </ActionSheet>
      )}
    </div>
  )
}

// =============================================================================
// STYLES
//
// All styles as JavaScript objects (React's inline style format).
// React uses camelCase instead of kebab-case: fontSize not font-size.
//
// Mobile-first approach: base styles target small screens.
// We use maxWidth to prevent the card from getting too wide on desktop.
// =============================================================================

const styles: Record<string, React.CSSProperties> = {
  container: {
    backgroundColor: BG,
    fontFamily: FONT,
    display: 'flex',
    flexDirection: 'column' as const,
    alignItems: 'center',
    paddingTop: '100px',
    paddingBottom: '40px',
    paddingLeft: '20px',
    paddingRight: '20px',
    minHeight: '100%',
    boxSizing: 'border-box',
  },

  card: {
    width: '100%',
    maxWidth: '380px',
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    textAlign: 'center',
  },

  wordmark: {
    fontFamily: FONT,
    fontSize: '32px',
    fontWeight: 700,
    color: BRAND,
    margin: '16px 0 24px 0',
    letterSpacing: '-0.5px',
  },

  tagline: {
    fontFamily: FONT,
    fontSize: '15px',
    fontWeight: 400,
    color: TEXT_SECONDARY,
    margin: '8px 0 32px 0',
    lineHeight: '1.5',
    maxWidth: '280px',
  },

  inputGroup: {
    width: '100%',
    marginBottom: '16px',
  },

  label: {
    display: 'block',
    fontFamily: FONT,
    fontSize: '12px',
    fontWeight: 500,
    color: TEXT_SECONDARY,
    marginBottom: '8px',
    textAlign: 'left' as const,
    textTransform: 'uppercase' as const,
    letterSpacing: '0.5px',
    paddingLeft: '16px',
  },

  phoneInput: {
    width: '100%',
    padding: '16px',
    fontSize: '20px',
    fontFamily: FONT,
    fontWeight: 500,
    color: TEXT,
    backgroundColor: WHITE,
    border: `2px solid ${BORDER}`,
    borderRadius: '12px',
    outline: 'none',
    boxSizing: 'border-box' as const,
    textAlign: 'left' as const,
    letterSpacing: '0.5px',
    // Prevent iOS zoom on focus (happens when font-size < 16px)
    WebkitTextSizeAdjust: '100%',
  },

  button: {
    width: '100%',
    padding: '16px',
    fontSize: '16px',
    fontFamily: FONT,
    fontWeight: 600,
    color: WHITE,
    backgroundColor: BRAND,
    border: 'none',
    borderRadius: '12px',
    cursor: 'pointer',
    transition: 'opacity 0.2s, transform 0.1s',
    WebkitTapHighlightColor: 'transparent',
    letterSpacing: '0.3px',
  },

  error: {
    fontFamily: FONT,
    fontSize: '15px',
    color: ERROR_RED,
    margin: '0 0 12px 0',
    fontWeight: 500,
  },

  // --- Code screen styles ---

  codePrompt: {
    fontFamily: FONT,
    fontSize: '15px',
    color: TEXT_SECONDARY,
    margin: '12px 0 4px 0',
  },

  phoneDisplay: {
    fontFamily: FONT,
    fontSize: '17px',
    fontWeight: 600,
    color: BRAND,
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    padding: '4px 8px',
    borderRadius: '6px',
    marginBottom: '28px',
    textDecoration: 'underline',
    textDecorationColor: `${BRAND}40`,
    textUnderlineOffset: '3px',
    transition: 'background-color 0.15s',
  },

  codeRow: {
    display: 'flex',
    gap: '8px',
    marginBottom: '20px',
    width: '100%',
    justifyContent: 'center',
  },

  codeBox: {
    width: '48px',
    height: '56px',
    fontSize: '24px',
    fontFamily: FONT,
    fontWeight: 600,
    color: TEXT,
    textAlign: 'center' as const,
    backgroundColor: WHITE,
    border: `2px solid ${BORDER}`,
    borderRadius: '10px',
    outline: 'none',
    caretColor: BRAND,
    transition: 'border-color 0.15s',
    WebkitTextSizeAdjust: '100%',
  },

  verifying: {
    fontFamily: FONT,
    fontSize: '15px',
    color: BRAND,
    fontWeight: 500,
    margin: '0 0 8px 0',
  },

  // --- Home screen styles ---

  logoutButton: {
    fontFamily: FONT,
    fontSize: '15px',
    fontWeight: 500,
    color: TEXT_SECONDARY,
    background: 'none',
    border: `1px solid ${BORDER}`,
    borderRadius: '8px',
    padding: '10px 24px',
    cursor: 'pointer',
    marginTop: '8px',
    transition: 'border-color 0.15s',
  },
}

// =============================================================================
// LIST SCREEN STYLES
// =============================================================================

const listStyles = {
  page: {
    height: '100dvh',
    display: 'flex',
    flexDirection: 'column',
    backgroundColor: BG,
    fontFamily: FONT,
    overflow: 'hidden',
  } as React.CSSProperties,

  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '16px 20px',
    backgroundColor: WHITE,
  } as React.CSSProperties,

  headerWordmark: {
    fontFamily: FONT,
    fontSize: '22px',
    fontWeight: 700,
    color: BRAND,
    letterSpacing: '-0.5px',
  } as React.CSSProperties,

  hamburgerBtn: {
    width: '40px',
    height: '40px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    borderRadius: '8px',
    WebkitTapHighlightColor: 'transparent',
  } as React.CSSProperties,

  menuOverlay: {
    position: 'fixed' as const,
    inset: 0,
    zIndex: 19,
  } as React.CSSProperties,

  menuDropdown: {
    position: 'absolute' as const,
    top: '44px',
    right: 0,
    backgroundColor: WHITE,
    borderRadius: '12px',
    boxShadow: '0 4px 20px rgba(0,0,0,0.15)',
    padding: '6px 0',
    minWidth: '220px',
    whiteSpace: 'nowrap' as const,
    zIndex: 20,
  } as React.CSSProperties,

  menuItem: (active: boolean): React.CSSProperties => ({
    display: 'block',
    width: '100%',
    padding: '10px 18px',
    fontFamily: FONT,
    fontSize: '15px',
    fontWeight: active ? 600 : 400,
    color: active ? BRAND : TEXT,
    background: 'none',
    border: 'none',
    textAlign: 'left' as const,
    cursor: 'pointer',
    WebkitTapHighlightColor: 'transparent',
  }),

  listArea: {
    flex: 1,
    overflowY: 'auto' as const,
    position: 'relative' as const,
  } as React.CSSProperties,

  emptyState: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    paddingTop: '80px',
    gap: '6px',
  } as React.CSSProperties,

  emptyText: {
    fontFamily: FONT,
    fontSize: '17px',
    fontWeight: 500,
    color: TEXT,
  } as React.CSSProperties,

  emptySubtext: {
    fontFamily: FONT,
    fontSize: '15px',
    color: TEXT_SECONDARY,
  } as React.CSSProperties,

  deptHeader: (open: boolean, isFirst = false): React.CSSProperties => ({
    display: 'flex',
    alignItems: 'center',
    width: '100%',
    padding: '13px 20px 13px 0',
    marginTop: isFirst ? 0 : '8px',
    background: open ? `${BRAND}0D` : 'none',
    border: 'none',
    borderTop: isFirst ? 'none' : `1px solid ${BORDER}`,
    borderBottom: `1px solid ${BORDER}`,
    cursor: 'pointer',
    textAlign: 'left' as const,
    WebkitTapHighlightColor: 'transparent',
    gap: '0px',
    fontFamily: FONT,
    fontSize: '12px',
    fontWeight: 600,
    color: open ? BRAND : TEXT_SECONDARY,
    textTransform: 'uppercase' as const,
    letterSpacing: '0.7px',
  }),

  deptCount: {
    fontSize: '12px',
    fontWeight: 500,
    color: TEXT_SECONDARY,
    marginLeft: 'auto',
  } as React.CSSProperties,

  deptChevron: (open: boolean): React.CSSProperties => ({
    fontSize: '17px',
    color: TEXT_SECONDARY,
    transform: open ? 'rotate(90deg)' : 'none',
    transition: 'transform 0.15s',
    lineHeight: 1,
    marginLeft: '4px',
  }),

  itemRow: {
    display: 'flex',
    alignItems: 'center',
    borderBottom: `1px solid ${BORDER}`,
    backgroundColor: WHITE,
  } as React.CSSProperties,

  itemCheckBtn: {
    padding: '12px 10px 12px 20px',
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    flexShrink: 0,
    WebkitTapHighlightColor: 'transparent',
    display: 'flex',
    alignItems: 'center',
  } as React.CSSProperties,

  itemCheck: (checked: boolean): React.CSSProperties => ({
    width: '22px',
    height: '22px',
    borderRadius: '50%',
    border: `2px solid ${checked ? '#C5C0B8' : BORDER}`,
    backgroundColor: checked ? '#C5C0B8' : 'transparent',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
    fontSize: '12px',
    fontWeight: 700,
    color: WHITE,
    transition: 'all 0.15s',
  }),

  itemCenter: {
    flex: 1,
    padding: '12px 0',
    cursor: 'pointer',
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '2px',
    WebkitTapHighlightColor: 'transparent',
  } as React.CSSProperties,

  itemName: (checked: boolean): React.CSSProperties => ({
    fontFamily: FONT,
    fontSize: '17px',
    color: checked ? TEXT_SECONDARY : TEXT,
    textDecoration: checked ? 'line-through' : 'none',
    transition: 'all 0.15s',
  }),

  itemMeta: {
    fontFamily: FONT,
    fontSize: '12px',
    color: TEXT_SECONDARY,
  } as React.CSSProperties,

  itemEditBtn: {
    padding: '8px 12px 8px 4px',
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    color: TEXT_SECONDARY,
    flexShrink: 0,
    WebkitTapHighlightColor: 'transparent',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: '48px',
    height: '48px',
  } as React.CSSProperties,

  chatBar: {
    flexShrink: 0,
    display: 'flex',
    alignItems: 'flex-end',
    gap: '8px',
    padding: '10px 12px',
    paddingBottom: 'max(10px, env(safe-area-inset-bottom))',
    backgroundColor: WHITE,
    borderTop: `1px solid ${BORDER}`,
  } as React.CSSProperties,

  chatIconBtn: {
    flexShrink: 0,
    width: '40px',
    height: '40px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    borderRadius: '50%',
    WebkitTapHighlightColor: 'transparent',
  } as React.CSSProperties,

  chatInput: {
    flex: 1,
    padding: '10px 14px',
    fontSize: '16px',
    fontFamily: FONT,
    color: TEXT,
    backgroundColor: BG,
    border: `1.5px solid ${BORDER}`,
    borderRadius: '20px',
    outline: 'none',
    minWidth: 0,
  } as React.CSSProperties,

  chatSendBtn: (active: boolean): React.CSSProperties => ({
    flexShrink: 0,
    width: '36px',
    height: '36px',
    borderRadius: '50%',
    backgroundColor: active ? BRAND : BORDER,
    color: active ? WHITE : TEXT_SECONDARY,
    border: 'none',
    cursor: active ? 'pointer' : 'default',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    transition: 'background-color 0.15s',
    WebkitTapHighlightColor: 'transparent',
  }),

  overlay: {
    position: 'fixed' as const,
    inset: 0,
    backgroundColor: 'rgba(0,0,0,0.35)',
    display: 'flex',
    alignItems: 'flex-end',
    zIndex: 30,
  } as React.CSSProperties,

  sheet: {
    width: '100%',
    backgroundColor: WHITE,
    borderRadius: '20px 20px 0 0',
    padding: '24px 20px',
    paddingBottom: 'max(32px, env(safe-area-inset-bottom))',
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '14px',
    maxHeight: '85dvh',
    overflowY: 'auto' as const,
  } as React.CSSProperties,

  sheetTitle: {
    fontFamily: FONT,
    fontSize: '17px',
    fontWeight: 600,
    color: TEXT,
  } as React.CSSProperties,

  fieldLabel: {
    display: 'block',
    fontFamily: FONT,
    fontSize: '12px',
    fontWeight: 600,
    color: TEXT_SECONDARY,
    textTransform: 'uppercase' as const,
    letterSpacing: '0.5px',
    marginBottom: '4px',
  } as React.CSSProperties,

  sheetRow: {
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
  } as React.CSSProperties,

  sheetLabel: {
    fontFamily: FONT,
    fontSize: '15px',
    fontWeight: 500,
    color: TEXT_SECONDARY,
    flexShrink: 0,
    width: '36px',
  } as React.CSSProperties,

  sheetTextInput: {
    width: '100%',
    padding: '13px 16px',
    fontSize: '16px',
    fontFamily: FONT,
    color: TEXT,
    backgroundColor: BG,
    border: `1.5px solid ${BORDER}`,
    borderRadius: '12px',
    outline: 'none',
    boxSizing: 'border-box' as const,
  } as React.CSSProperties,

  sheetSelect: {
    width: '100%',
    padding: '13px 16px',
    fontSize: '16px',
    fontFamily: FONT,
    color: TEXT,
    backgroundColor: BG,
    border: `1.5px solid ${BORDER}`,
    borderRadius: '12px',
    outline: 'none',
    boxSizing: 'border-box' as const,
    appearance: 'none' as const,
    WebkitAppearance: 'none' as const,
    backgroundImage: `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='8' viewBox='0 0 12 8'%3E%3Cpath d='M1 1l5 5 5-5' stroke='%236B6B6B' stroke-width='1.5' fill='none' stroke-linecap='round'/%3E%3C/svg%3E")`,
    backgroundRepeat: 'no-repeat',
    backgroundPosition: 'right 16px center',
    paddingRight: '40px',
  } as React.CSSProperties,

  floatingInput: {
    width: '100%',
    padding: '12px 16px',
    fontSize: '16px',
    fontFamily: FONT,
    color: TEXT,
    backgroundColor: WHITE,
    border: `1.5px solid ${BORDER}`,
    borderRadius: '12px',
    outline: 'none',
    boxSizing: 'border-box' as const,
  } as React.CSSProperties,

  floatingSelect: {
    width: '100%',
    padding: '12px 16px',
    fontSize: '16px',
    fontFamily: FONT,
    color: TEXT,
    backgroundColor: WHITE,
    border: `1.5px solid ${BORDER}`,
    borderRadius: '12px',
    outline: 'none',
    boxSizing: 'border-box' as const,
    appearance: 'none' as const,
    WebkitAppearance: 'none' as const,
    backgroundImage: `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='8' viewBox='0 0 12 8'%3E%3Cpath d='M1 1l5 5 5-5' stroke='%236B6B6B' stroke-width='1.5' fill='none' stroke-linecap='round'/%3E%3C/svg%3E")`,
    backgroundRepeat: 'no-repeat',
    backgroundPosition: 'right 16px center',
    paddingRight: '40px',
  } as React.CSSProperties,

  qtyControl: {
    display: 'flex',
    alignItems: 'center',
    gap: '0',
    border: `1.5px solid ${BORDER}`,
    borderRadius: '12px',
    overflow: 'hidden',
    backgroundColor: BG,
  } as React.CSSProperties,

  qtyBtn: {
    width: '44px',
    height: '44px',
    border: 'none',
    background: 'none',
    fontSize: '20px',
    fontWeight: 400,
    color: BRAND,
    cursor: 'pointer',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    WebkitTapHighlightColor: 'transparent',
  } as React.CSSProperties,

  qtyValue: {
    fontFamily: FONT,
    fontSize: '16px',
    fontWeight: 600,
    color: TEXT,
    minWidth: '32px',
    textAlign: 'center' as const,
  } as React.CSSProperties,

  sheetButton: {
    width: '100%',
    padding: '15px',
    fontSize: '16px',
    fontFamily: FONT,
    fontWeight: 600,
    color: WHITE,
    backgroundColor: BRAND,
    border: 'none',
    borderRadius: '12px',
    cursor: 'pointer',
  } as React.CSSProperties,

  sheetDeleteBtn: {
    width: '100%',
    padding: '13px',
    fontSize: '15px',
    fontFamily: FONT,
    fontWeight: 500,
    color: ERROR_RED,
    backgroundColor: 'transparent',
    border: `1px solid ${ERROR_RED}40`,
    borderRadius: '12px',
    cursor: 'pointer',
  } as React.CSSProperties,
}

// =============================================================================
// GLOBAL STYLES
//
// Inject a tiny bit of global CSS for things inline styles can't do:
//   - Focus ring colors on inputs
//   - Remove default body margin
//   - Smooth font rendering
// =============================================================================

const globalCSS = `
  * { margin: 0; padding: 0; box-sizing: border-box; }

  html, body, #root {
    height: 100%;
  }

  body {
    background-color: ${BG};
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  input:focus {
    border-color: ${BRAND} !important;
    box-shadow: 0 0 0 3px ${BRAND}18;
  }

  button:active {
    transform: scale(0.98);
  }

  /* Hide number input spinners */
  input::-webkit-outer-spin-button,
  input::-webkit-inner-spin-button {
    -webkit-appearance: none;
    margin: 0;
  }

  /* Selection color */
  ::selection {
    background-color: ${BRAND}30;
  }

  /* Penny loading pulse */
  @keyframes cursor-blink {
    0%, 100% { opacity: 1; }
    50% { opacity: 0; }
  }
  .cursor-blink { animation: cursor-blink 0.8s step-end infinite; }

  @keyframes penny-pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.2; }
  }
  .penny-pulse {
    animation: penny-pulse 1.2s ease-in-out infinite;
  }

  @keyframes float-up {
    0% { transform: translateY(0) scale(0.9); opacity: 0; }
    10% { transform: translateY(-10px) scale(1); opacity: 1; }
    70% { opacity: 1; }
    100% { transform: translateY(-340px) scale(1); opacity: 0; }
  }

  @keyframes done-overlay {
    0% { opacity: 0; }
    15% { opacity: 1; }
    70% { opacity: 1; }
    100% { opacity: 0; }
  }

  @keyframes confetti-fall {
    0% { transform: translateY(-10px) translateX(0) rotate(0deg); opacity: 1; }
    25% { transform: translateY(25vh) translateX(15px) rotate(120deg); opacity: 1; }
    50% { transform: translateY(50vh) translateX(-10px) rotate(240deg); opacity: 0.8; }
    75% { transform: translateY(75vh) translateX(12px) rotate(360deg); opacity: 0.4; }
    100% { transform: translateY(100vh) translateX(-5px) rotate(480deg); opacity: 0; }
  }
`

// Inject the global styles into the document head.
// This runs once when the module loads.
const styleEl = document.createElement('style')
styleEl.textContent = globalCSS
document.head.appendChild(styleEl)
