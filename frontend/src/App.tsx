// =============================================================================
// thriftly — App.tsx
//
// Screens: phone → code → home, penny settings, profile, share, admin
// All styles inline. All API calls go to the Go backend.
// =============================================================================

import { useState, useRef, useEffect, useCallback } from 'react'

import { PennyAvatar } from './components/PennyAvatar'
import { BrandLogo } from './components/BrandLogo'
import { ActionSheet, SectionHeader } from './components/ActionSheet'
import { ConfirmModal } from './components/ConfirmModal'
import { Confetti } from './components/Confetti'
import { FloatingLabelField } from './components/FloatingLabelField'
import { ErrorBoundary } from './components/ErrorBoundary'
import {
  BRAND, BG, TEXT, TEXT_SECONDARY, BORDER, ERROR_RED, WHITE, FONT,
  DEPARTMENTS, DEPT_ICONS,
} from './styles'

// =============================================================================
// TYPES
// =============================================================================

// The screens the app can show
type Screen = 'phone' | 'code' | 'home' | 'penny' | 'profile' | 'settings-share' | 'admin' | 'email-login' | 'email-register' | 'forgot-password' | 'reset-password'

// Declare gtag for TypeScript
declare function gtag(...args: unknown[]): void

// =============================================================================
// URL ↔ SCREEN MAPPING (for browser back/forward navigation)
// =============================================================================

const SCREEN_PATHS: Record<Screen, string> = {
  phone: '/login',
  code: '/verify',
  home: '/',
  penny: '/penny',
  profile: '/profile',
  'settings-share': '/share',
  admin: '/admin',
  'email-login': '/login/email',
  'email-register': '/login/email/register',
  'forgot-password': '/login/forgot',
  'reset-password': '/reset-password',
}

const PATH_TO_SCREEN: Record<string, Screen> = {
  '/login': 'phone',
  '/verify': 'code',
  '/': 'home',
  '/penny': 'penny',
  '/profile': 'profile',
  '/share': 'settings-share',
  '/settings': 'penny',
  '/settings/profile': 'profile',
  '/settings/saved': 'profile',
  '/admin': 'admin',
  '/login/email': 'email-login',
  '/login/email/register': 'email-register',
  '/login/forgot': 'forgot-password',
  '/reset-password': 'reset-password',
}

const AUTH_SCREENS: Screen[] = ['phone', 'code', 'email-login', 'email-register', 'forgot-password', 'reset-password']

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


function DeptIcon({ dept }: { dept: string }) {
  const icon = DEPT_ICONS[dept]
  if (!icon) return null
  return <span style={{ fontSize: '22px', lineHeight: 1, flexShrink: 0 }}>{icon}</span>
}


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


// Toggle switch — used for urgent and claim toggles
function ToggleSwitch({ value, onChange, label }: { value: boolean; onChange: () => void; label?: string }) {
  return (
    <button
      role="switch"
      aria-checked={value}
      aria-label={label}
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

// =============================================================================
// PENNY LOADING MESSAGES
// =============================================================================

const PENNY_STEP1 = ["Reading your list...", "Let me see what you wrote...", "Hmm, looking at this..."]
const PENNY_STEP2 = ["Checking it twice!", "Sorting by aisle...", "Organizing...", "I know just where these go..."]
const PENNY_STEP3 = ["Adding to your list...", "Almost done!", "Stocking up!"]
const PENNY_STEPS = [PENNY_STEP1, PENNY_STEP2, PENNY_STEP3]
function pickRandom(arr: string[]) {
  return arr[Math.floor(Math.random() * arr.length)]
}

function QuantityControl({ value, onChange }: {
  value: number
  onChange: (newValue: number) => void
}) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', border: `1.5px solid ${BORDER}`, borderRadius: '12px', backgroundColor: WHITE, flexShrink: 0 }}>
      <button
        onClick={() => onChange(Math.max(1, value - 1))}
        aria-label={`Decrease quantity, currently ${value}`}
        style={{ width: '38px', height: '42px', border: 'none', background: 'none', fontSize: '17px', color: BRAND, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
      >−</button>
      <span aria-live="polite" aria-atomic="true" style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, minWidth: '24px', textAlign: 'center' }}>{value}</span>
      <button
        onClick={() => onChange(value + 1)}
        aria-label={`Increase quantity, currently ${value}`}
        style={{ width: '38px', height: '42px', border: 'none', background: 'none', fontSize: '17px', color: BRAND, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
      >+</button>
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
        style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, textDecoration: 'none', marginTop: '48px', opacity: 0.7 }}
      >
        powered by <span style={{ fontWeight: 600 }}>thriftly</span>
      </a>
      <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '12px', opacity: 0.6 }}>
        <a href="/privacy" target="_blank" rel="noopener noreferrer" style={{ color: TEXT_SECONDARY, textDecoration: 'none' }}>Privacy</a>
        {' · '}
        <a href="/terms" target="_blank" rel="noopener noreferrer" style={{ color: TEXT_SECONDARY, textDecoration: 'none' }}>Terms</a>
      </div>
    </>
  )
}

// =============================================================================
// MAIN APP COMPONENT
// =============================================================================

function AppInner() {
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


  // Login page Penny typewriter
  const [loginPennyText, setLoginPennyText] = useState('')
  const loginTypingRef = useRef<ReturnType<typeof setTimeout> | null>(null)

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
  const [editForm, setEditForm] = useState({ name: '', brand: '', dept: '', qty: 1, size: '', notes: '' })
  const [displayBrand, setDisplayBrand] = useState('')
  const [editLoading, setEditLoading] = useState(false)
  const [itemHistory, setItemHistory] = useState<ItemModification[]>([])
  const deptRefs = useRef<Record<string, HTMLDivElement | null>>({})

  // --- Toast state ---
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error'; action?: { label: string; onClick: () => void } } | null>(null)
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const settingsSaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const sliderSaveTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)

  // --- Review state (items from last parse) ---
  const [lastAdded, setLastAdded] = useState<ListItem[]>([])
  const [lastSkipped, setLastSkipped] = useState<{ parsed: { name: string; brand: string; quantity: number; size: string; department: string }; existing_id: number }[]>([])
  const [showReviewSheet, setShowReviewSheet] = useState(false)
  const [returnToReview, setReturnToReview] = useState(false)
  const hasReviewedRef = useRef(false)

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
  const [showSaveSheet, setShowSaveSheet] = useState(false)
  const [saveNameInput, setSaveNameInput] = useState('')

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
  const [placesReady, setPlacesReady] = useState(false)
  const [addressSuggestions, setAddressSuggestions] = useState<{ description: string; placeId: string }[]>([])
  const [showAddressSuggestions, setShowAddressSuggestions] = useState(false)
  const addressSuggestTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [settingsMembers, setSettingsMembers] = useState<{ name: string; phone: string }[]>([])
  const [settingsPennyPhone, setSettingsPennyPhone] = useState('')
  const [settingsUserPhone, setSettingsUserPhone] = useState('')
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
  const [analyticsData, setAnalyticsData] = useState<{
    days: number; items_created: number;
    items_by_source: Record<string, number>;
    items_by_department: Record<string, number>;
    events_by_action: Record<string, number>;
    events_by_day: { date: string; count: number }[];
  } | null>(null)

  // --- NPS state ---
  const [showNPS, setShowNPS] = useState(false)
  const [npsScore, setNpsScore] = useState<number | null>(null)
  const [npsStep, setNpsStep] = useState<1 | 2>(1)
  const [npsComment, setNpsComment] = useState('')
  const [npsTypedText, setNpsTypedText] = useState('')
  const [npsTypingDone, setNpsTypingDone] = useState(false)

  // --- SMS opt-in state ---
  const [smsOptIn, setSmsOptIn] = useState(true)

  // --- Email/Google auth state ---
  const [emailInput, setEmailInput] = useState('')
  const [passwordInput, setPasswordInput] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [nameInput, setNameInput] = useState('')
  const [lastNameInput, setLastNameInput] = useState('')
  const [googleClientId, setGoogleClientId] = useState('')
  const [resetToken, setResetToken] = useState('')
  const [resetSent, setResetSent] = useState(false)
  const [magicLinkSent, setMagicLinkSent] = useState(false)

  // --- New item highlight state ---
  const [newItemIds, setNewItemIds] = useState<Set<number>>(new Set())
  const newItemTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // --- Penny loading spinner state ---
  const [pennyStatus, setPennyStatus] = useState<string | null>(null)

  // --- Brand disambiguation state ---
  const [pennyDisambig, setPennyDisambig] = useState<{
    item_id: number; product_category: string; current_brand: string; alternatives: string[];
  }[]>([])
  const [pennyDisambigIndex, setPennyDisambigIndex] = useState(0)
  const [pennySlider, setPennySlider] = useState(35)
  const [pennyAutotune, setPennyAutotune] = useState(true)
  const [pennyDeals, setPennyDeals] = useState(false)
  const [keyboardOpen, setKeyboardOpen] = useState(false)

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
    // Reset done state if items are no longer all checked (e.g., new items added)
    if (showDoneToast && !(items.length > 0 && items.every(i => i.checked))) {
      setShowDoneToast(false)
      setShowFireworks(false)
      setShowDoneOverlay(false)
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

  // Detect keyboard open/close via visualViewport
  useEffect(() => {
    if (!window.visualViewport) return
    const handler = () => {
      const shrunk = window.innerHeight - window.visualViewport!.height
      setKeyboardOpen(shrunk > 100)
    }
    window.visualViewport.addEventListener('resize', handler)
    return () => window.visualViewport!.removeEventListener('resize', handler)
  }, [])

  // Clean up all timers on unmount
  useEffect(() => {
    return () => {
      if (homePennyInterval.current) clearInterval(homePennyInterval.current)
      if (chimeInTimer.current) clearTimeout(chimeInTimer.current)
      if (idleTimer.current) clearTimeout(idleTimer.current)
      if (toastTimer.current) clearTimeout(toastTimer.current)
      if (newItemTimerRef.current) clearTimeout(newItemTimerRef.current)
    }
  }, [])

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
          showToast('You joined the shared list!')
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
          loadSettings() // Load settings once at auth
          // Resolve screen from pathname (or migrated hash)
          const path = window.location.pathname
          const resolved = PATH_TO_SCREEN[path]
          if (resolved && !AUTH_SCREENS.includes(resolved)) {
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
          if (resolved && !AUTH_SCREENS.includes(resolved)) {
            sessionStorage.setItem('thriftly_redirect', path)
          }
          // Check if on a specific auth screen (email login, reset password, etc.)
          if (resolved && AUTH_SCREENS.includes(resolved)) {
            setScreen(resolved, true)
          } else {
            setScreen('phone', true)
          }
        }
      })
      .catch(() => {
        // Not logged in — that's fine, show the phone screen
        setScreen('phone', true)
      })
      .finally(() => setChecking(false))
  }, [])

  // Fetch Google Client ID and extract reset token from URL
  useEffect(() => {
    fetch('/api/config').then(r => r.json()).then((data: { google_client_id?: string }) => {
      if (data.google_client_id) setGoogleClientId(data.google_client_id)
    }).catch(() => {})

    // Extract reset token from URL if present
    const params = new URLSearchParams(window.location.search)
    const token = params.get('token')
    if (token && window.location.pathname === '/reset-password') {
      setResetToken(token)
    }
  }, [])

  // Initialize Google Sign-In when on login screens
  useEffect(() => {
    if (!googleClientId) return
    if (!['phone', 'email-login'].includes(screen)) return

    const g = (window as unknown as { google?: { accounts: { id: { initialize: (opts: unknown) => void; renderButton: (el: HTMLElement, opts: unknown) => void } } } }).google
    if (!g) return

    g.accounts.id.initialize({
      client_id: googleClientId,
      callback: async (response: { credential: string }) => {
        setLoading(true)
        setError('')
        try {
          const res = await fetch('/auth/google', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ credential: response.credential }),
          })
          const data = await res.json()
          if (res.ok && data.success) {
            setScreen('home', true)
          } else {
            setError(data.error || 'Google sign-in failed')
          }
        } catch {
          setError('Google sign-in failed')
        } finally {
          setLoading(false)
        }
      },
    })

    // Render button
    setTimeout(() => {
      const el = document.getElementById('google-signin-btn')
      if (el && g) {
        el.innerHTML = ''
        g.accounts.id.renderButton(el, {
          type: 'standard',
          shape: 'rectangular',
          theme: 'outline',
          text: 'continue_with',
          size: 'large',
          width: Math.min(el.parentElement?.offsetWidth || 400, 400),
        })
      }
    }, 100)
  }, [googleClientId, screen])

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

  // Login page typewriter — cycles Penny quotes with value props
  const LOGIN_MESSAGES = [
    "Hi, I'm Penny — your household's AI shopping assistant.",
    "Tell me what you need and I'll organize your list by aisle.",
    "I learn your brand preferences over time.",
    "Share your list with your whole household.",
    "Text or email me items anytime — I'll add them for you.",
    "Snap a photo of your fridge and I'll spot what's missing.",
  ]

  useEffect(() => {
    if (screen !== 'phone' && screen !== 'email-login' && screen !== 'email-register') {
      if (loginTypingRef.current) clearTimeout(loginTypingRef.current)
      return
    }
    let cancelled = false
    let msgIndex = 0
    let charIndex = 0

    const typeMessage = () => {
      if (cancelled) return
      const msg = LOGIN_MESSAGES[msgIndex]
      charIndex++
      setLoginPennyText(msg.slice(0, charIndex))
      if (charIndex < msg.length) {
        loginTypingRef.current = setTimeout(typeMessage, 35)
      } else {
        // Pause, then move to next message
        loginTypingRef.current = setTimeout(() => {
          if (cancelled) return
          msgIndex = (msgIndex + 1) % LOGIN_MESSAGES.length
          charIndex = 0
          setLoginPennyText('')
          loginTypingRef.current = setTimeout(typeMessage, 400)
        }, 4000)
      }
    }
    setLoginPennyText('')
    loginTypingRef.current = setTimeout(typeMessage, 300)
    return () => {
      cancelled = true
      if (loginTypingRef.current) clearTimeout(loginTypingRef.current)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen])


  // =========================================================================
  // LOAD LIST when on home screen
  // =========================================================================

  const sortItems = (arr: ListItem[]) =>
    [...arr].sort((a, b) => a.name.localeCompare(b.name))

  const handleSessionExpired = () => {
    setChatLoading(false)
    setScreen('phone', true)
    setPhone('')
    setError('')
    showToast('Session expired — please log in again', 'error')
  }

  // Centralized fetch wrapper: handles 401 session expiry and JSON parsing.
  // Returns parsed JSON on success, or null on failure.
  const fetchAPI = async (url: string, options?: RequestInit): Promise<Record<string, unknown> | unknown[] | null> => {
    let res: Response
    try {
      res = await fetch(url, options)
    } catch {
      return null // network error
    }
    if (res.status === 401) { handleSessionExpired(); return null }
    if (!res.ok) return null
    try {
      return await res.json()
    } catch {
      return null // invalid JSON
    }
  }

  // Fetch wrapper for actions that don't need a JSON body back (delete, clear, etc.)
  // Returns { ok, status, error? } — always handles 401.
  const fetchAction = async (url: string, options?: RequestInit): Promise<{ ok: boolean; status: number; error?: string }> => {
    let res: Response
    try {
      res = await fetch(url, options)
    } catch {
      return { ok: false, status: 0, error: 'Network error' }
    }
    if (res.status === 401) { handleSessionExpired(); return { ok: false, status: 401 } }
    if (!res.ok) {
      const errData = await res.json().catch(() => null)
      return { ok: false, status: res.status, error: errData?.error || `Error ${res.status}` }
    }
    return { ok: true, status: res.status }
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
    reloadList().catch(e => console.warn('[load] list failed:', e))
    loadTrips().catch(e => console.warn('[load] trips failed:', e))
    fetchAPI('/api/nps/check')
      .then(data => { if (data && !Array.isArray(data) && data.show_nps) setShowNPS(true) })
      .catch(e => console.warn('[load] nps check failed:', e))
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
      const today = fmtDate(new Date())
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
    const data = await fetchAPI('/api/trips', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ trip_date: tripDay, trip_time: tripTime, time_label: timeLabel }),
    })
    if (!data) { showToast('Couldn\'t save your trip — please try again', 'error'); return }
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
    showToast('Trip added! Everyone will see it.')
  }

  const handleCancelTrip = async () => {
    const data = await fetchAPI('/api/trips', { method: 'DELETE' })
    if (!data) { showToast('Couldn\'t cancel your trip — please try again', 'error'); return }
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
      const endH = (h + 1) % 24
      dtEnd += `${String(endH).padStart(2, '0')}${String(m).padStart(2, '0')}00`
    } else if (trip.time_label && timeMap[trip.time_label]) {
      dtStart += timeMap[trip.time_label]
      const h = parseInt(timeMap[trip.time_label].slice(0, 2)) + 1
      dtEnd += `${String(h).padStart(2, '0')}0000`
    } else {
      dtStart += '100000'
      dtEnd += '110000'
    }
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || 'America/New_York'
    const ics = [
      'BEGIN:VCALENDAR',
      'VERSION:2.0',
      'BEGIN:VEVENT',
      `DTSTART;TZID=${tz}:${dtStart}`,
      `DTEND;TZID=${tz}:${dtEnd}`,
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
        value: fmtDate(d),
      })
    }
    return options
  }

  const fmtDate = (d: Date) => d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0')
  const todayStr = fmtDate(new Date())
  const tomorrowDate = new Date()
  tomorrowDate.setDate(tomorrowDate.getDate() + 1)
  const tomorrowStr = fmtDate(tomorrowDate)
  const satDate = new Date()
  satDate.setDate(satDate.getDate() + (6 - satDate.getDay() || 7))
  const satStr = fmtDate(satDate)

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


  const loadSettings = () => {
    fetchAPI('/api/settings')
      .then(data => {
        if (data && !Array.isArray(data)) {
          const parts = ((data.name as string) || '').split(' ')
          setSettingsFirstName(parts[0] || '')
          setSettingsLastName(parts.slice(1).join(' '))
          setSettingsEmail((data.email as string) || '')
          setSettingsAddress((data.address as string) || '')
          if (data.google_places_ready) setPlacesReady(true)
          if (data.phone) setSettingsUserPhone(data.phone as string)
          if (data.penny_phone) setSettingsPennyPhone(data.penny_phone as string)
          setSettingsMembers((data.household_members as { name: string; phone: string }[]) || [])
          if (data.penny_slider != null) setPennySlider(data.penny_slider as number)
          if (data.penny_autotune != null) setPennyAutotune(data.penny_autotune as boolean)
          if (data.penny_deals != null) setPennyDeals(data.penny_deals as boolean)
        }
      })
      .catch(e => console.warn('[load] settings failed:', e))
  }

  const fetchAddressSuggestions = async (input: string) => {
    if (!placesReady || input.length < 3) {
      setAddressSuggestions([])
      setShowAddressSuggestions(false)
      return
    }
    try {
      const data = await fetchAPI(`/api/places/autocomplete?input=${encodeURIComponent(input)}`)
      const items = (data as { description: string; placeId: string }[]) || []
      setAddressSuggestions(items)
      setShowAddressSuggestions(items.length > 0)
    } catch {
      setAddressSuggestions([])
      setShowAddressSuggestions(false)
    }
  }

  const selectAddressSuggestion = async (placeId: string) => {
    setShowAddressSuggestions(false)
    try {
      const data = await fetchAPI(`/api/places/detail?place_id=${encodeURIComponent(placeId)}`)
      const address = (data as { address: string })?.address
      if (address) {
        setSettingsAddress(address)
        // Auto-save after selecting address
        if (settingsSaveTimer.current) clearTimeout(settingsSaveTimer.current)
        settingsSaveTimer.current = setTimeout(async () => {
          try {
            await fetchAPI('/api/settings', {
              method: 'PATCH',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ name: [settingsFirstName.trim(), settingsLastName.trim()].filter(Boolean).join(' '), email: settingsEmail.trim(), address: address.trim() }),
            })
          } catch { /* silent */ }
        }, 300)
      }
    } catch { /* silent */ }
  }

  useEffect(() => {
    // Only re-fetch settings when entering profile (where edits happen) or share (household changes)
    if (screen !== 'penny' && screen !== 'profile' && screen !== 'settings-share') return
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
      .catch(e => console.warn('[load] referral failed:', e))
  }

  useEffect(() => {
    if (screen !== 'settings-share') return
    loadReferralData()
  }, [screen])

  useEffect(() => {
    if (screen !== 'admin') return
    fetchAPI(`/api/admin/dashboard?days=${adminDays}`)
      .then(data => { if (data) setAdminData(data as typeof adminData) })
      .catch(e => console.warn('[load] admin dashboard failed:', e))
    fetchAPI(`/api/admin/analytics?days=${adminDays}`)
      .then(data => { if (data) setAnalyticsData(data as typeof analyticsData) })
      .catch(e => console.warn('[load] admin analytics failed:', e))
  }, [screen, adminDays])

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
      else if ((lastAdded.length > 0 || lastSkipped.length > 0) && toast.type !== 'error' && !hasReviewedRef.current) btns.push({ label: 'Review', onClick: () => { hasReviewedRef.current = true; pushModalState(); setShowReviewSheet(true); setToast(null); if (toastTimer.current) clearTimeout(toastTimer.current) } })
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

  // Track sessions for invite prompt timing
  useEffect(() => {
    if (screen !== 'home') return
    const count = parseInt(localStorage.getItem('penny_session_count') || '0')
    // Only increment once per mount
    if (!(window as any).__pennySessionCounted) {
      (window as any).__pennySessionCounted = true
      localStorage.setItem('penny_session_count', String(count + 1))
    }
  }, [screen])

  // Penny invite prompt — session 3+, every 14 days, max 4 prompts, stops when household exists
  useEffect(() => {
    if (screen !== 'home' || settingsMembers.length > 0) return
    const promptCount = parseInt(localStorage.getItem('penny_invite_prompt_count') || '0')
    if (promptCount >= 4) return
    const sessionCount = parseInt(localStorage.getItem('penny_session_count') || '0')
    if (sessionCount < 3) return
    const INVITE_INTERVAL = 14 * 24 * 60 * 60 * 1000
    const lastPrompt = localStorage.getItem('penny_invite_prompt')
    if (lastPrompt && Date.now() - Number(lastPrompt) < INVITE_INTERVAL) return

    const timer = setTimeout(() => {
      if (pennyStatus || toast || pennyChimeIn || showDoneToast) return
      localStorage.setItem('penny_invite_prompt', String(Date.now()))
      localStorage.setItem('penny_invite_prompt_count', String(promptCount + 1))
      const messages = [
        "Shopping's better together — invite someone to share this list!",
        "Still flying solo? Invite someone to share this list in real time.",
        "One more nudge — sharing your list takes about 10 seconds.",
        "Last time I'll ask — want to share your list with someone?",
      ]
      typePennyHome(messages[Math.min(promptCount, messages.length - 1)], [{
        label: 'Share',
        onClick: () => setScreen('settings-share'),
      }])
    }, 12000)

    return () => clearTimeout(timer)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen, settingsMembers.length])

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
      if (!ctx) { URL.revokeObjectURL(objectUrl); return }
      ctx.drawImage(img, 0, 0, w, h)
      setPendingImages(prev => [...prev, canvas.toDataURL('image/jpeg', 0.8)])
      URL.revokeObjectURL(objectUrl)
    }
    img.onerror = () => URL.revokeObjectURL(objectUrl)
    img.src = objectUrl
  }

  const handlePhotoSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files
    if (!files) return
    const remaining = 5 - pendingImages.length
    if (remaining <= 0) { showToast('Max 5 images at a time', 'error'); e.target.value = ''; return }
    const count = Math.min(files.length, remaining)
    for (let i = 0; i < count; i++) {
      resizeAndSetPhoto(files[i])
    }
    if (files.length > count) showToast(`Only added ${count} of ${files.length} images (max 5)`, 'error')
    e.target.value = ''
  }

  const handlePaste = (e: React.ClipboardEvent) => {
    const items = e.clipboardData?.items
    if (!items) return
    if (pendingImages.length >= 5) return
    let foundImage = false
    for (const item of items) {
      if (item.type.startsWith('image/')) {
        if (!foundImage) {
          e.preventDefault()
          foundImage = true
        }
        if (pendingImages.length < 5) {
          const file = item.getAsFile()
          if (file) resizeAndSetPhoto(file)
        }
      }
    }
  }

  const checkAllDone = (next: ListItem[]) => {
    if (next.length > 0 && next.every(i => i.checked)) {
      setTimeout(() => { setShowDoneToast(true); setShowFireworks(true); setShowDoneOverlay(true) }, 300)
    }
  }

  const handleToggle = async (id: number) => {
    // Capture current checked state before optimistic update
    const oldChecked = items.find(i => i.id === id)?.checked
    setItems(prev => prev.map(item => item.id === id ? { ...item, checked: !item.checked } : item))
    try {
      const data = await fetchAPI(`/api/list/${id}/toggle`, { method: 'PATCH' })
      if (data) {
        const updated = data as ListItem
        setItems(prev => { const next = prev.map(item => item.id === updated.id ? updated : item); checkAllDone(next); return next })
      } else {
        updateItem(id, { checked: oldChecked })
        showToast('Couldn\'t update that item — please try again', 'error')
      }
    } catch {
      updateItem(id, { checked: oldChecked })
      showToast("Hmm, that change didn't stick. Try again?", 'error')
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
        showToast('Couldn\'t update that item — please try again', 'error')
      }
    } catch {
      updateItem(id, { urgent: was })
      showToast("Hmm, that change didn't stick. Try again?", 'error')
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
        showToast('Couldn\'t claim that item — please try again', 'error')
      }
    } catch {
      if (oldItem) updateItem(id, oldItem)
      showToast("Couldn't claim that item. Try again?", 'error')
    }
  }

  const advanceDisambig = () => {
    const next = pennyDisambigIndex + 1
    if (next >= pennyDisambig.length) {
      setPennyDisambig([])
      setPennyDisambigIndex(0)
      typePennyHome("All set! I'll remember those for next time.")
    } else {
      setPennyDisambigIndex(next)
    }
  }

  const handleBrandPick = async (brand: string) => {
    const d = pennyDisambig[pennyDisambigIndex]
    if (!d) return
    const result = await fetchAction('/api/list/brand-resolve', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ item_id: d.item_id, brand, product_category: d.product_category }),
    })
    if (result.ok) {
      setItems(prev => prev.map(i => i.id === d.item_id ? { ...i, brand } : i))
      setLastAdded(prev => prev.map(i => i.id === d.item_id ? { ...i, brand } : i))
      advanceDisambig()
    } else if (result.status !== 401) {
      showToast('Couldn\'t save brand preference — try again', 'error')
    }
  }

  const handleSendChat = async () => {
    if ((!chatText.trim() && pendingImages.length === 0) || chatLoading) return

    // Disambiguation-aware routing: handle brand picks via chat input
    if (pennyDisambig.length > 0 && chatText.trim() && pendingImages.length === 0) {
      const text = chatText.trim().toLowerCase()
      // Simple phrases to skip/dismiss
      if (['skip', 'next'].includes(text)) {
        setChatText('')
        advanceDisambig()
        return
      }
      if (['skip all', 'just pick for me', 'just pick', 'you decide'].includes(text)) {
        setChatText('')
        setPennyDisambig([])
        setPennyDisambigIndex(0)
        typePennyHome("All set! I'll remember those for next time.")
        return
      }
      // Check if text matches an alternative for the current item
      const d = pennyDisambig[pennyDisambigIndex]
      if (d) {
        const matchedAlt = d.alternatives.find(a => a.toLowerCase() === text)
        if (matchedAlt !== undefined) {
          setChatText('')
          handleBrandPick(matchedAlt || d.current_brand)
          return
        }
        // Treat as a custom brand name for the current item
        if (text.length < 40 && !text.includes(' and ') && !text.includes(',')) {
          setChatText('')
          handleBrandPick(chatText.trim())
          return
        }
      }
      // Otherwise fall through to normal parsing (e.g. "also get bread")
    }

    const chatPayloadText = chatText.trim()
    const chatPayloadImages = [...pendingImages]
    setChatText('')
    setPendingImages([])
    if (chatInputRef.current) chatInputRef.current.style.height = 'auto'
    setChatLoading(true)
    setLastAdded([])
    setLastSkipped([])
    hasReviewedRef.current = false
    setPennyDisambig([])
    setPennyDisambigIndex(0)
    try {
      const payload = JSON.stringify({ text: chatPayloadText, images: chatPayloadImages })

      const res = await fetch('/api/list/parse', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload,
      })

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
        } else if (action === 'share') {
          // Show Penny's message with inline Invite button
          if (pennyMsg) {
            typePennyHome(pennyMsg, [{
              label: 'Invite',
              onClick: () => handleTextInviteLink(),
            }])
          }
        } else if (action === 'save') {
          // Saved — just show message
        } else if (action === 'load_saved_list') {
          const savedListsData: { id: number; name: string; item_count: number }[] = data.saved_lists || []
          if (savedListsData.length === 0) {
            // No saved lists — Penny's message already says so
          } else if (data.added > 0) {
            // A specific list was auto-loaded — reload list
            reloadList()
          } else {
            // Show saved list options as buttons
            const btns = savedListsData.map(sl => ({
              label: `${sl.name} (${sl.item_count})`,
              onClick: () => {
                handleLoadSavedList(sl.id, 'merge')
              },
            }))
            typePennyHome(pennyMsg, btns)
            return // typePennyHome already handles the message
          }
        } else if (action === 'delete_saved_list') {
          // Deleted — just show Penny's message
        } else if (action === 'clear') {
          // Ask for confirmation before clearing ALL items
          typePennyHome(pennyMsg || 'Clear your entire list?', [
            { label: 'Yes, clear it', onClick: () => handleClearList(true) },
            { label: 'Cancel', onClick: () => typePennyHome('No problem, your list is safe!') },
          ])
          return // typePennyHome already handles the message
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
        setTimeout(() => chatInputRef.current?.focus(), 150)

        // Handle brand disambiguations
        const disambiguations = (data.brand_disambiguations || []) as { item_id: number; product_category: string; current_brand: string; alternatives: string[] }[]
        if (disambiguations.length > 0) {
          setPennyDisambig(disambiguations)
          setPennyDisambigIndex(0)
        }

        // Show Penny's message or fallback toast
        if (action === 'share') {
          // Already handled via typePennyHome above
        } else if (pennyMsg) {
          showToast(pennyMsg)
        } else {
          const parts: string[] = []
          if (data.added > 0) parts.push(`Added ${data.added} item${data.added === 1 ? '' : 's'}!`)
          if (skippedItems.length > 0) parts.push(`${skippedItems.length} already on your list`)
          if (removedItems.length > 0) parts.push(`Removed ${removedItems.length} item${removedItems.length === 1 ? '' : 's'}`)
          if (updatedItems.length > 0) parts.push(`Updated ${updatedItems.length} item${updatedItems.length === 1 ? '' : 's'}`)
          if (checkedItems.length > 0) parts.push(`${action === 'check' ? 'Checked' : 'Unchecked'} ${checkedItems.length} item${checkedItems.length === 1 ? '' : 's'}`)
          if (parts.length > 0) showToast(parts.join(' · '))
          else if (action === 'add' && addedItems.length === 0 && skippedItems.length === 0) showToast("I couldn't find any items in that — try rewording it?", 'error')
        }
      } else if (res.status === 429) {
        showToast('Hold on — give it a minute and try again', 'error')
      } else {
        const errData = await res.json().catch(() => null)
        const errMsg = errData?.error || `Error ${res.status}`
        console.error('[penny] parse failed:', res.status, errMsg)
        showToast(errMsg, 'error')
      }
    } catch (err) {
      console.error('[penny] parse error:', err)
      showToast('Couldn\'t connect — check your connection and try again', 'error')
    }
    finally { setChatLoading(false) }
  }

  const handleNPSSubmit = async () => {
    if (npsScore === null) return
    const result = await fetchAction('/api/nps', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ score: npsScore, comment: npsComment }),
    })
    if (result.ok) {
      showToast('Thanks for your feedback!')
      setShowNPS(false)
      setNpsScore(null)
      setNpsStep(1)
      setNpsComment('')
    } else if (result.status !== 401) {
      showToast('Couldn\'t save feedback — please try again', 'error')
    }
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
    setEditForm({ name: item.name, brand: item.brand, dept: item.department, qty: item.quantity || 1, size: item.size, notes: item.notes || '' })
    setDisplayBrand(item.brand)
    setItemHistory([])
    fetchAPI(`/api/list/${item.id}/history`)
      .then(data => setItemHistory(Array.isArray(data) ? data as ItemModification[] : []))
      .catch(e => console.warn('[load] item history failed:', e))
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
    if (!selectedItem || !editForm.name.trim() || editLoading) return
    setDisplayBrand(editForm.brand)
    setEditLoading(true)
    try {
      const data = await fetchAPI(`/api/list/${selectedItem.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: editForm.name.trim(), brand: editForm.brand.trim(), department: editForm.dept, quantity: editForm.qty, size: editForm.size.trim(), notes: editForm.notes.trim() }),
      })
      if (data) {
        const updated = data as ListItem
        setItems(prev => sortItems(prev.map(item => item.id === updated.id ? updated : item)))
        closeEditSheet(undefined, updated)
      } else {
        showToast('Couldn\'t save changes — please try again', 'error')
      }
    } catch { showToast('Couldn\'t save changes — please try again', 'error') }
    finally { setEditLoading(false) }
  }

  const handleEditDelete = async () => {
    if (!selectedItem || editLoading) return
    const deletingId = selectedItem.id
    setEditLoading(true)
    try {
      const result = await fetchAction(`/api/list/${deletingId}`, { method: 'DELETE' })
      if (result.ok) {
        setItems(prev => prev.filter(item => item.id !== deletingId))
        closeEditSheet(deletingId)
      } else if (result.status !== 401) { showToast('Couldn\'t delete that item — please try again', 'error') }
    } finally { setEditLoading(false) }
  }

  const handleClearList = async (all?: boolean) => {
    const url = all ? '/api/list/clear?all=true' : '/api/list/clear'
    const result = await fetchAction(url, { method: 'POST' })
    if (result.ok) {
      if (all) {
        setItems([]); setExpandedDepts(new Set()); setShowDoneToast(false); setShowFireworks(false); setPennyChimeIn(null)
      } else {
        setItems(prev => prev.filter(i => !i.checked)); setExpandedDepts(new Set()); setShowDoneToast(false); setShowFireworks(false); setPennyChimeIn(null)
      }
      showToast('List cleared', 'success')
    } else if (result.status !== 401) { showToast('Couldn\'t clear your list — please try again', 'error') }
  }

  const handleSaveList = async () => {
    if (!saveNameInput.trim()) return
    try {
      const data = await fetchAPI('/api/saved-lists', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: saveNameInput.trim() }),
      })
      if (data) { setShowSaveSheet(false); setSaveNameInput(''); handleClearList(); showToast('List saved and cleared!') }
      else showToast('Couldn\'t save your list — please try again', 'error')
    } catch { showToast('Couldn\'t save your list — please try again', 'error') }
  }

  const handleLoadSavedList = async (id: number, mode: 'merge' | 'overwrite' = 'merge') => {
    const url = mode === 'overwrite' ? `/api/saved-lists/${id}/load?mode=overwrite` : `/api/saved-lists/${id}/load`
    const result = await fetchAction(url, { method: 'POST' })
    if (result.ok) {
      setScreen('home')
      reloadList()
      showToast('List loaded!')
    } else if (result.status !== 401) { showToast('Couldn\'t load that list — please try again', 'error') }
  }

  // =========================================================================
  // EMAIL AUTH HANDLERS
  // =========================================================================

  const handleEmailRegister = async () => {
    setError('')
    if (!emailInput.trim()) { setError('Please enter your email'); return }
    if (passwordInput.length < 8) { setError('Password must be at least 8 characters'); return }
    if (passwordInput !== confirmPassword) { setError('Passwords do not match'); return }
    setLoading(true)
    try {
      const res = await fetch('/auth/email/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: emailInput.trim(), password: passwordInput, name: `${nameInput.trim()} ${lastNameInput.trim()}`.trim() }),
      })
      const data = await res.json()
      if (res.ok && data.success) {
        setPasswordInput(''); setConfirmPassword('')
        setScreen('home', true)
      } else {
        setError(data.error || 'Registration failed')
      }
    } catch { setError('Registration failed') }
    finally { setLoading(false) }
  }

  const handleEmailLogin = async () => {
    setError('')
    if (!emailInput.trim() || !passwordInput) { setError('Please enter email and password'); return }
    setLoading(true)
    try {
      const res = await fetch('/auth/email/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: emailInput.trim(), password: passwordInput }),
      })
      const data = await res.json()
      if (res.ok && data.success) {
        setPasswordInput(''); setConfirmPassword('')
        setScreen('home', true)
      } else {
        setError(data.error || 'Invalid email or password')
      }
    } catch { setError('Sign in failed') }
    finally { setLoading(false) }
  }

  const handleForgotPassword = async () => {
    setError('')
    if (!emailInput.trim()) { setError('Please enter your email'); return }
    setLoading(true)
    try {
      await fetch('/auth/email/forgot', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: emailInput.trim() }),
      })
      setResetSent(true)
    } catch { setError('Failed to send reset link') }
    finally { setLoading(false) }
  }

  const handleMagicLink = async () => {
    setError('')
    if (!emailInput.trim()) { setError('Please enter your email'); return }
    setLoading(true)
    try {
      await fetch('/auth/email/magic-link', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: emailInput.trim() }),
      })
      setMagicLinkSent(true)
    } catch { setError('Couldn\'t send the link — check your connection and try again') }
    finally { setLoading(false) }
  }

  const handleResetPassword = async () => {
    setError('')
    if (passwordInput.length < 8) { setError('Password must be at least 8 characters'); return }
    if (passwordInput !== confirmPassword) { setError('Passwords do not match'); return }
    setLoading(true)
    try {
      const res = await fetch('/auth/email/reset', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token: resetToken, password: passwordInput }),
      })
      const data = await res.json()
      if (res.ok && data.success) {
        showToast('Password reset! Please sign in.')
        setPasswordInput('')
        setConfirmPassword('')
        setScreen('email-login')
      } else {
        setError(data.error || 'Failed to reset password')
      }
    } catch { setError('Failed to reset password') }
    finally { setLoading(false) }
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

    const digits = raw.slice(0, 10)
    setPhone(digits)
    setError('')

    // Auto-advance to code screen when 10 digits entered and SMS opt-in checked
    if (digits.length === 10 && smsOptIn) {
      setTimeout(() => handleSendCode(digits), 300)
    }
  }

  // =========================================================================
  // SEND CODE
  //
  // Called when the user taps "Send Code" (only enabled at 10 digits).
  // Posts the raw 10-digit number to the backend.
  // =========================================================================

  const handleSendCode = async (phoneOverride?: string) => {
    const p = phoneOverride || phone
    if (p.length !== 10 || loading) return

    setLoading(true)
    setError('')

    try {
      const res = await fetch('/auth/send-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ phone_number: p }),
      })

      const data = await res.json().catch(() => ({} as Record<string, string>))

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

      const data = await res.json().catch(() => ({} as Record<string, unknown>))

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

  const settingsFirstNameRef = useRef(settingsFirstName)
  const settingsLastNameRef = useRef(settingsLastName)
  const settingsEmailRef = useRef(settingsEmail)
  const settingsAddressRef = useRef(settingsAddress)
  settingsFirstNameRef.current = settingsFirstName
  settingsLastNameRef.current = settingsLastName
  settingsEmailRef.current = settingsEmail
  settingsAddressRef.current = settingsAddress
  const settingsSaving = useRef(false)
  const settingsQueued = useRef(false)
  const doSettingsSave = async () => {
    if (settingsSaving.current) { settingsQueued.current = true; return }
    settingsSaving.current = true
    try {
      await fetchAPI('/api/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: [settingsFirstNameRef.current.trim(), settingsLastNameRef.current.trim()].filter(Boolean).join(' '), email: settingsEmailRef.current.trim(), address: settingsAddressRef.current.trim() }),
      })
    } catch { /* silent */ }
    settingsSaving.current = false
    if (settingsQueued.current) { settingsQueued.current = false; doSettingsSave() }
  }
  const handleSettingsAutoSave = () => {
    if (settingsSaveTimer.current) clearTimeout(settingsSaveTimer.current)
    settingsSaveTimer.current = setTimeout(doSettingsSave, 500)
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
      else showToast('Couldn\'t create invite link — please try again', 'error')
    } catch { showToast('Couldn\'t connect — check your connection and try again', 'error') }
    finally { setInviteLoading(false) }
  }

  const handleTextInviteLink = () => withInviteLink(async link => {
    const message = `Join our household shopping list on Thriftly! ${link}`
    if (/Mobi|Android/i.test(navigator.userAgent)) {
      // Mobile: open Messages app with pre-filled invite text
      window.location.href = `sms:?&body=${encodeURIComponent(message)}`
    } else {
      // Desktop: copy invite link to clipboard
      try {
        await navigator.clipboard.writeText(link)
        showToast('Invite link copied!')
      } catch {
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
        <div style={{ ...styles.card, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '16px' }}>
          <Logo />
          <p style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY, margin: 0 }}>Loading…</p>
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
      <div style={{
        backgroundColor: BG,
        fontFamily: FONT,
        minHeight: '100%',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        padding: '32px 20px 40px',
        boxSizing: 'border-box',
      }}>
        <div style={{
          width: '100%',
          maxWidth: '400px',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          flex: 1,
        }}>

          {/* Penny hero */}
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', marginBottom: '28px', width: '100%' }}>
            <PennyAvatar size={130} />
            <div style={{
              fontFamily: FONT, fontSize: '18px', fontWeight: 500,
              color: TEXT, lineHeight: 1.5, fontStyle: 'italic',
              textAlign: 'left', marginTop: '16px',
              minHeight: '54px', maxWidth: '320px', width: '100%',
            }}>{loginPennyText}<span style={{ opacity: 0.4 }} className="cursor-blink">|</span></div>
          </div>

          {/* ── Phone input ── */}
          <div style={{ width: '100%', marginBottom: '12px' }}>
            <FloatingLabelField label="Your mobile number">
              <input
                ref={phoneInputRef}
                type="tel"
                inputMode="numeric"
                autoComplete="tel-national"
                placeholder="(555) 123-4567"
                value={formatPhone(phone)}
                onChange={handlePhoneChange}
                onKeyDown={e => { if (e.key === 'Enter' && isValid && smsOptIn) handleSendCode() }}
                style={{ ...listStyles.floatingInput, fontSize: '20px', fontWeight: 500, letterSpacing: '0.5px' }}
                autoFocus
              />
            </FloatingLabelField>
          </div>

          {/* SMS opt-in */}
          <label style={{
            display: 'flex', alignItems: 'flex-start', gap: '8px',
            width: '100%', marginBottom: '20px',
            cursor: 'pointer', userSelect: 'none' as const,
          }}>
            <input
              type="checkbox"
              checked={smsOptIn}
              onChange={e => setSmsOptIn(e.target.checked)}
              style={{ accentColor: BRAND, width: '16px', height: '16px', flexShrink: 0, marginTop: '1px' }}
            />
            <span style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, lineHeight: 1.5, whiteSpace: 'nowrap' }}>
              I agree to receive a one-time login code via text
            </span>
          </label>

          {error && <p style={{ ...styles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}

          <button
            onClick={() => handleSendCode()}
            disabled={!isValid || !smsOptIn || loading}
            style={{
              width: '100%', padding: '17px', fontSize: '16px',
              fontFamily: FONT, fontWeight: 700, color: WHITE,
              backgroundColor: BRAND, border: 'none', borderRadius: '14px',
              cursor: isValid && smsOptIn && !loading ? 'pointer' : 'default',
              transition: 'opacity 0.2s, transform 0.1s',
              WebkitTapHighlightColor: 'transparent',
              letterSpacing: '0.2px',
              opacity: isValid && smsOptIn && !loading ? 1 : 0.35,
              marginBottom: '4px',
            }}
          >
            {loading ? 'Sending…' : 'Continue →'}
          </button>

          {/* Divider + alternative auth */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px', width: '100%', margin: '20px 0 16px' }}>
            <div style={{ flex: 1, height: '1px', backgroundColor: BORDER }} />
            <span style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY }}>or</span>
            <div style={{ flex: 1, height: '1px', backgroundColor: BORDER }} />
          </div>

          {googleClientId && (
            <div id="google-signin-btn" style={{ width: '100%', display: 'flex', justifyContent: 'center', marginBottom: '12px' }} />
          )}

          <button
            onClick={() => { setError(''); setScreen('email-login') }}
            style={listStyles.subtleLink}
          >Use email instead</button>

          <div style={{ marginTop: 'auto', paddingTop: '16px', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
            <LegalLinks />
          </div>
        </div>
      </div>
    )
  }

  // =========================================================================
  // EMAIL LOGIN SCREEN
  // =========================================================================

  if (screen === 'email-login') {
    return (
      <div style={{
        backgroundColor: BG, fontFamily: FONT, minHeight: '100%',
        display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
        padding: '32px 20px 40px', boxSizing: 'border-box',
      }}>
        <div style={{ width: '100%', maxWidth: '400px', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
          {/* Penny hero */}
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', marginBottom: '24px', width: '100%' }}>
            <PennyAvatar size={80} />
            <div style={{
              fontFamily: FONT, fontSize: '15px', fontWeight: 500,
              color: TEXT, lineHeight: 1.5, fontStyle: 'italic',
              textAlign: 'left', marginTop: '12px',
              minHeight: '44px', maxWidth: '320px', width: '100%',
            }}>{loginPennyText}<span style={{ opacity: 0.4 }} className="cursor-blink">|</span></div>
          </div>

          <div style={{ width: '100%', marginBottom: '12px' }}>
            <FloatingLabelField label="Email">
              <input
                type="email"
                autoComplete="email"
                value={emailInput}
                onChange={e => setEmailInput(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter') handleEmailLogin() }}
                style={listStyles.floatingInput}
                autoFocus
              />
            </FloatingLabelField>
          </div>

          <div style={{ width: '100%', marginBottom: '12px', position: 'relative' }}>
            <FloatingLabelField label="Password">
              <input
                type={showPassword ? 'text' : 'password'}
                autoComplete="current-password"
                value={passwordInput}
                onChange={e => setPasswordInput(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter') handleEmailLogin() }}
                style={{ ...listStyles.floatingInput, paddingRight: '44px' }}
              />
            </FloatingLabelField>
            <button type="button" onClick={() => setShowPassword(p => !p)} aria-label={showPassword ? 'Hide password' : 'Show password'} style={{ position: 'absolute', right: '12px', bottom: '12px', background: 'none', border: 'none', cursor: 'pointer', padding: '4px', color: TEXT_SECONDARY }}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">{showPassword ? <><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></> : <><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></>}</svg>
            </button>
          </div>

          {error && <p style={{ ...styles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}
          {magicLinkSent && <p style={{ fontFamily: FONT, fontSize: '14px', color: BRAND, fontWeight: 500, width: '100%', textAlign: 'left' as const, marginBottom: '12px' }}>We sent a sign-in link to your email — check your inbox!</p>}

          <button
            onClick={handleEmailLogin}
            disabled={loading}
            style={{
              width: '100%', padding: '17px', fontSize: '16px',
              fontFamily: FONT, fontWeight: 700, color: WHITE,
              backgroundColor: BRAND, border: 'none', borderRadius: '14px',
              cursor: loading ? 'default' : 'pointer',
              opacity: loading ? 0.5 : 1, marginBottom: '16px',
            }}
          >
            {loading ? 'Signing in…' : 'Sign in'}
          </button>

          <div style={{ display: 'flex', gap: '16px', marginBottom: '16px' }}>
            <button onClick={() => { setError(''); setMagicLinkSent(false); setScreen('forgot-password') }} style={listStyles.subtleLink}>Forgot password?</button>
            <button onClick={handleMagicLink} disabled={loading} style={{ ...listStyles.subtleLink, opacity: loading ? 0.5 : 1 }}>Email me a sign-in link</button>
            <button onClick={() => { setError(''); setMagicLinkSent(false); setScreen('email-register') }} style={listStyles.subtleLink}>Create an account</button>
          </div>

          {/* Divider + alternative auth */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px', width: '100%', margin: '8px 0 16px' }}>
            <div style={{ flex: 1, height: '1px', backgroundColor: BORDER }} />
            <span style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY }}>or</span>
            <div style={{ flex: 1, height: '1px', backgroundColor: BORDER }} />
          </div>

          {googleClientId && (
            <div id="google-signin-btn" style={{ width: '100%', display: 'flex', justifyContent: 'center', marginBottom: '12px' }} />
          )}

          <button onClick={() => { setError(''); setScreen('phone') }} style={listStyles.subtleLink}>Use phone number instead</button>

          <div style={{ marginTop: '12px' }}><LegalLinks /></div>
        </div>
      </div>
    )
  }

  // =========================================================================
  // EMAIL REGISTER SCREEN
  // =========================================================================

  if (screen === 'email-register') {
    return (
      <div style={{
        backgroundColor: BG, fontFamily: FONT, minHeight: '100%',
        display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
        padding: '32px 20px 40px', boxSizing: 'border-box',
      }}>
        <div style={{ width: '100%', maxWidth: '400px', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
          {/* Penny hero */}
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', marginBottom: '24px', width: '100%' }}>
            <PennyAvatar size={80} />
            <div style={{
              fontFamily: FONT, fontSize: '15px', fontWeight: 500,
              color: TEXT, lineHeight: 1.5, fontStyle: 'italic',
              textAlign: 'left', marginTop: '12px',
              minHeight: '44px', maxWidth: '320px', width: '100%',
            }}>{loginPennyText}<span style={{ opacity: 0.4 }} className="cursor-blink">|</span></div>
          </div>

          <div style={{ display: 'flex', gap: '12px', width: '100%', marginBottom: '12px' }}>
            <div style={{ flex: 1 }}>
              <FloatingLabelField label="First name">
                <input type="text" autoComplete="given-name" value={nameInput} onChange={e => setNameInput(e.target.value)} style={listStyles.floatingInput} autoFocus />
              </FloatingLabelField>
            </div>
            <div style={{ flex: 1 }}>
              <FloatingLabelField label="Last name">
                <input type="text" autoComplete="family-name" value={lastNameInput} onChange={e => setLastNameInput(e.target.value)} style={listStyles.floatingInput} />
              </FloatingLabelField>
            </div>
          </div>

          <div style={{ width: '100%', marginBottom: '12px' }}>
            <FloatingLabelField label="Email">
              <input type="email" autoComplete="email" value={emailInput} onChange={e => setEmailInput(e.target.value)} style={listStyles.floatingInput} />
            </FloatingLabelField>
          </div>

          <div style={{ width: '100%', marginBottom: '4px', position: 'relative' }}>
            <FloatingLabelField label="Password">
              <input type={showPassword ? 'text' : 'password'} autoComplete="new-password" value={passwordInput} onChange={e => setPasswordInput(e.target.value)} style={{ ...listStyles.floatingInput, paddingRight: '44px' }} />
            </FloatingLabelField>
            <button type="button" onClick={() => setShowPassword(p => !p)} aria-label={showPassword ? 'Hide password' : 'Show password'} style={{ position: 'absolute', right: '12px', bottom: '12px', background: 'none', border: 'none', cursor: 'pointer', padding: '4px', color: TEXT_SECONDARY }}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">{showPassword ? <><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></> : <><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></>}</svg>
            </button>
          </div>
          <p style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginBottom: '12px', paddingLeft: '4px' }}>At least 8 characters</p>

          <div style={{ width: '100%', marginBottom: '12px', position: 'relative' }}>
            <FloatingLabelField label="Confirm password">
              <input
                type={showPassword ? 'text' : 'password'}
                autoComplete="new-password"
                value={confirmPassword}
                onChange={e => setConfirmPassword(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter') handleEmailRegister() }}
                style={{ ...listStyles.floatingInput, paddingRight: '44px' }}
              />
            </FloatingLabelField>
          </div>

          {error && <p style={{ ...styles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}

          <button
            onClick={handleEmailRegister}
            disabled={loading}
            style={{
              width: '100%', padding: '17px', fontSize: '16px',
              fontFamily: FONT, fontWeight: 700, color: WHITE,
              backgroundColor: BRAND, border: 'none', borderRadius: '14px',
              cursor: loading ? 'default' : 'pointer',
              opacity: loading ? 0.5 : 1, marginBottom: '16px',
            }}
          >
            {loading ? 'Creating account…' : 'Create account'}
          </button>

          <button onClick={() => { setError(''); setScreen('email-login') }} style={listStyles.subtleLink}>Already have an account? Sign in</button>

          <div style={{ marginTop: '12px' }}><LegalLinks /></div>
        </div>
      </div>
    )
  }

  // =========================================================================
  // FORGOT PASSWORD SCREEN
  // =========================================================================

  if (screen === 'forgot-password') {
    return (
      <div style={{
        backgroundColor: BG, fontFamily: FONT, minHeight: '100%',
        display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
        padding: '32px 20px 40px', boxSizing: 'border-box',
      }}>
        <div style={{ width: '100%', maxWidth: '400px', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
          <div style={{ width: '100%', marginBottom: '24px', textAlign: 'center' }}>
            <div style={{ fontFamily: FONT, fontSize: '22px', fontWeight: 700, color: TEXT, letterSpacing: '-0.5px', marginBottom: '6px' }}>
              Reset your password
            </div>
            <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY }}>
              {resetSent ? "If an account exists with that email, we've sent a reset link." : "Enter your email and we'll send you a reset link."}
            </div>
          </div>

          {!resetSent && (
            <>
              <div style={{ width: '100%', marginBottom: '12px' }}>
                <FloatingLabelField label="Email">
                  <input
                    type="email"
                    autoComplete="email"
                    value={emailInput}
                    onChange={e => setEmailInput(e.target.value)}
                    onKeyDown={e => { if (e.key === 'Enter') handleForgotPassword() }}
                    style={listStyles.floatingInput}
                    autoFocus
                  />
                </FloatingLabelField>
              </div>

              {error && <p style={{ ...styles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}

              <button
                onClick={handleForgotPassword}
                disabled={loading}
                style={{
                  width: '100%', padding: '17px', fontSize: '16px',
                  fontFamily: FONT, fontWeight: 700, color: WHITE,
                  backgroundColor: BRAND, border: 'none', borderRadius: '14px',
                  cursor: loading ? 'default' : 'pointer',
                  opacity: loading ? 0.5 : 1, marginBottom: '16px',
                }}
              >
                {loading ? 'Sending…' : 'Send reset link'}
              </button>
            </>
          )}

          <button onClick={() => { setError(''); setResetSent(false); setScreen('email-login') }} style={listStyles.subtleLink}>Back to sign in</button>
        </div>
      </div>
    )
  }

  // =========================================================================
  // RESET PASSWORD SCREEN
  // =========================================================================

  if (screen === 'reset-password') {
    return (
      <div style={{
        backgroundColor: BG, fontFamily: FONT, minHeight: '100%',
        display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
        padding: '32px 20px 40px', boxSizing: 'border-box',
      }}>
        <div style={{ width: '100%', maxWidth: '400px', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
          <div style={{ width: '100%', marginBottom: '24px', textAlign: 'center' }}>
            <div style={{ fontFamily: FONT, fontSize: '22px', fontWeight: 700, color: TEXT, letterSpacing: '-0.5px', marginBottom: '6px' }}>
              Set a new password
            </div>
          </div>

          <div style={{ width: '100%', marginBottom: '12px', position: 'relative' }}>
            <FloatingLabelField label="New password">
              <input type={showPassword ? 'text' : 'password'} autoComplete="new-password" value={passwordInput} onChange={e => setPasswordInput(e.target.value)} style={{ ...listStyles.floatingInput, paddingRight: '44px' }} autoFocus />
            </FloatingLabelField>
            <button type="button" onClick={() => setShowPassword(p => !p)} aria-label={showPassword ? 'Hide password' : 'Show password'} style={{ position: 'absolute', right: '12px', bottom: '12px', background: 'none', border: 'none', cursor: 'pointer', padding: '4px', color: TEXT_SECONDARY }}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">{showPassword ? <><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></> : <><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></>}</svg>
            </button>
          </div>

          <div style={{ width: '100%', marginBottom: '12px' }}>
            <FloatingLabelField label="Confirm password">
              <input
                type={showPassword ? 'text' : 'password'}
                autoComplete="new-password"
                value={confirmPassword}
                onChange={e => setConfirmPassword(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter') handleResetPassword() }}
                style={listStyles.floatingInput}
              />
            </FloatingLabelField>
          </div>

          {error && <p style={{ ...styles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}

          <button
            onClick={handleResetPassword}
            disabled={loading}
            style={{
              width: '100%', padding: '17px', fontSize: '16px',
              fontFamily: FONT, fontWeight: 700, color: WHITE,
              backgroundColor: BRAND, border: 'none', borderRadius: '14px',
              cursor: loading ? 'default' : 'pointer',
              opacity: loading ? 0.5 : 1, marginBottom: '16px',
            }}
          >
            {loading ? 'Resetting…' : 'Reset password'}
          </button>

          <button onClick={() => { setError(''); setScreen('email-login') }} style={listStyles.subtleLink}>Back to sign in</button>
        </div>
      </div>
    )
  }

  // =========================================================================
  // CODE SCREEN
  // =========================================================================

  if (screen === 'code') {
    return (
      <div style={{
        backgroundColor: BG,
        fontFamily: FONT,
        minHeight: '100%',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        padding: '32px 20px 40px',
        boxSizing: 'border-box',
      }}>
        <div style={{
          width: '100%',
          maxWidth: '400px',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          flex: 1,
        }}>

          {/* Headline */}
          <div style={{ width: '100%', marginBottom: '24px', textAlign: 'center' }}>
            <div style={{
              fontFamily: FONT, fontSize: '22px', fontWeight: 700,
              color: TEXT, letterSpacing: '-0.5px', lineHeight: 1.2,
              marginBottom: '6px',
            }}>Check your texts</div>
            <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY, lineHeight: 1.5 }}>
              Enter the code sent to {formatPhone(phone)}
            </div>
          </div>

          {/* ── Code boxes ── */}
          <div style={{ display: 'flex', gap: '8px', marginBottom: '16px', width: '100%', justifyContent: 'center' }}>
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
                onPaste={handleCodePaste}
                disabled={loading}
                style={{
                  width: '48px', height: '60px',
                  fontSize: '26px', fontFamily: FONT, fontWeight: 700,
                  color: TEXT, textAlign: 'center' as const,
                  backgroundColor: WHITE,
                  border: `2px solid ${digit ? BRAND : BORDER}`,
                  borderRadius: '12px', outline: 'none', caretColor: BRAND,
                  transition: 'border-color 0.15s, box-shadow 0.15s',
                  boxShadow: digit ? `0 0 0 3px ${BRAND}20` : 'none',
                  WebkitTextSizeAdjust: '100%',
                  opacity: loading ? 0.5 : 1,
                  boxSizing: 'border-box',
                }}
              />
            ))}
          </div>

          {loading && (
            <p style={{ fontFamily: FONT, fontSize: '15px', color: BRAND, fontWeight: 500, margin: '0 0 12px 0' }}>
              Signing you in…
            </p>
          )}

          {error && <p style={{ ...styles.error, width: '100%', textAlign: 'center' as const }}>{error}</p>}

          {/* Subtle helper links */}
          <div style={{ display: 'flex', gap: '16px', marginTop: '8px' }}>
            <button
              onClick={() => { handleSendCode(phone) }}
              disabled={loading}
              style={{ ...listStyles.subtleLink, opacity: loading ? 0.5 : 1 }}
            >Resend code</button>
            <button
              onClick={() => { setScreen('phone'); setError('') }}
              style={listStyles.subtleLink}
            >Use a different number</button>
          </div>

          <div style={{ marginTop: 'auto', paddingTop: '16px', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
            <LegalLinks />
          </div>
        </div>
      </div>
    )
  }

  // =========================================================================
  // SHARED HELPERS — menu and toast (extracted to avoid 5x duplication)
  // =========================================================================

  const renderMenu = (active: Screen) => (
    <div style={{ position: 'relative' }}>
      <button onClick={() => setMenuOpen(!menuOpen)} style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0, WebkitTapHighlightColor: 'transparent', display: 'flex', alignItems: 'center' }} aria-label="Navigation menu" aria-expanded={menuOpen} aria-haspopup="true">
        <div style={{ ...avatarCircleStyle(30), fontSize: '12px', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          {[settingsFirstName, settingsLastName].map(n => (n || '')[0]?.toUpperCase() || '').filter(Boolean).join('') || <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>}
        </div>
      </button>
      {menuOpen && (
        <>
          <div style={listStyles.menuOverlay} onClick={() => setMenuOpen(false)} />
          <div style={listStyles.menuDropdown}>
            <button style={{ ...listStyles.menuItem(active === 'home'), display: 'flex', alignItems: 'center', gap: '10px' }} onClick={() => { setMenuOpen(false); setScreen('home') }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="9" cy="21" r="1"/><circle cx="20" cy="21" r="1"/><path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"/></svg>
              Shopping List
            </button>
            <button style={{ ...listStyles.menuItem(active === 'settings-share'), display: 'flex', alignItems: 'center', gap: '10px' }} onClick={() => { setMenuOpen(false); setScreen('settings-share') }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/></svg>
              Share & Earn
            </button>
            <button style={{ ...listStyles.menuItem(active === 'penny'), display: 'flex', alignItems: 'center', gap: '10px' }} onClick={() => { setMenuOpen(false); setScreen('penny') }}>
              <PennyAvatar size={18} />
              Penny
            </button>
            <div style={{ height: '1px', backgroundColor: BORDER, margin: '4px 0' }} />
            <button style={{ ...listStyles.menuItem(active === 'profile'), display: 'flex', alignItems: 'center', gap: '10px' }} onClick={() => { setMenuOpen(false); setScreen('profile') }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
              Profile
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
  const pennyCloseStyle = { background: 'none', border: 'none', color: TEXT_SECONDARY, fontFamily: FONT, fontSize: '17px', cursor: 'pointer', padding: '8px 12px', lineHeight: 1, flexShrink: 0, opacity: 0.6, minWidth: '44px', minHeight: '44px', display: 'flex', alignItems: 'center', justifyContent: 'center' } as React.CSSProperties

  const renderPennyToast = () => toast ? (
    <div style={{ ...pennyBarStyle, backgroundColor: toast.type === 'error' ? '#FEE2E2' : '#F5F3EF', color: toast.type === 'error' ? '#991B1B' : TEXT }}>
      <PennyAvatar />
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
    'Penny': <PennyAvatar size={16} />,
    'Share & Earn': <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/></svg>,
    'Settings': <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 2l1.09 3.26L16.36 6l-2.6 2.14L14.53 12 12 9.77 9.47 12l.77-3.86L7.64 6l3.27-.74z"/><path d="M5 18l.7 2.1L8 20.8l-1.6 1.32.47 2.38L5 23.27 3.13 24.5l.47-2.38L2 20.8l2.3-.7z"/><path d="M19 15l.53 1.58 1.72.53-1.23.99.35 1.79L19 19.06l-1.37.83.35-1.79-1.23-.99 1.72-.53z"/></svg>,
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
  // PENNY SETTINGS (merged: Penny + Profile + Saved Lists + Sign Out)
  // =========================================================================

  if (screen === 'penny') {
    const saveSlider = (val: number) => {
      setPennySlider(val)
      if (sliderSaveTimeout.current) clearTimeout(sliderSaveTimeout.current)
      sliderSaveTimeout.current = setTimeout(() => {
        fetchAPI('/api/settings', {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ penny_slider: val }),
        })
      }, 400)
    }
    const toggleAutotune = () => {
      const next = !pennyAutotune
      setPennyAutotune(next)
      fetchAPI('/api/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ penny_autotune: next }),
      })
    }
    const toggleDeals = () => {
      const next = !pennyDeals
      setPennyDeals(next)
      fetchAPI('/api/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ penny_deals: next }),
      })
    }
    const pennyPhoneFormatted = settingsPennyPhone ? formatPhone(settingsPennyPhone.replace('+1', '')) : ''
    return (
      <div style={listStyles.page}>
        <ConfirmModal open={!!confirmModal} title={confirmModal?.title || ''} message={confirmModal?.message} confirmLabel={confirmModal?.confirmLabel} destructive={confirmModal?.destructive} onConfirm={runConfirmAction} onCancel={closeConfirmModal} />
        {renderPageHeader('Penny', 'penny')}
        <div style={listStyles.listArea}>
          <div style={{ padding: '24px 20px', display: 'flex', flexDirection: 'column', gap: '20px' }}>

            {/* ── How Penny works ── */}

            {/* Learn from my choices toggle */}
            <div onClick={toggleAutotune} style={listStyles.settingsCard}>
              {/* Brain/learning icon */}
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={TEXT} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M12 2a7 7 0 0 1 7 7c0 2.38-1.19 4.47-3 5.74V17a1 1 0 0 1-1 1h-6a1 1 0 0 1-1-1v-2.26C6.19 13.47 5 11.38 5 9a7 7 0 0 1 7-7z"/><path d="M9 21h6"/><path d="M10 17v-2.5"/><path d="M14 17v-2.5"/>
              </svg>
              <div style={{ flex: 1 }}>
                <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT }}>Learn from my choices</div>
                <div style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY, marginTop: '1px' }}>Penny remembers your brand preferences over time</div>
              </div>
              <ToggleSwitch value={pennyAutotune} onChange={toggleAutotune} />
            </div>

            {/* Slider — only visible when autotune is OFF */}
            <div style={{
              maxHeight: pennyAutotune ? 0 : '200px',
              opacity: pennyAutotune ? 0 : 1,
              overflow: pennyAutotune ? 'hidden' : 'visible',
              transition: 'max-height 0.3s ease, opacity 0.25s ease',
              pointerEvents: pennyAutotune ? 'none' : 'auto',
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '12px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  {/* Repeat/loop arrow — convenience */}
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={TEXT} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/>
                  </svg>
                  <div>
                    <div style={{ fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: TEXT }}>Convenience</div>
                    <div style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY }}>Just grab my usual</div>
                  </div>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <div style={{ textAlign: 'right' }}>
                    <div style={{ fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: TEXT }}>Savings</div>
                    <div style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY }}>Help me save</div>
                  </div>
                  {/* Price tag with % — savings */}
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={TEXT} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/><text x="13" y="11" fontSize="7" fontWeight="bold" fill={TEXT} stroke="none" fontFamily="sans-serif">%</text>
                  </svg>
                </div>
              </div>
              <input
                type="range"
                min={0}
                max={100}
                value={pennySlider}
                onChange={e => saveSlider(Number(e.target.value))}
                style={{
                  width: '100%', height: '6px', appearance: 'none', WebkitAppearance: 'none',
                  background: `linear-gradient(to right, ${BRAND} 0%, ${BRAND} ${pennySlider}%, ${BORDER} ${pennySlider}%, ${BORDER} 100%)`,
                  borderRadius: '3px', outline: 'none', cursor: 'pointer',
                  padding: '10px 0', margin: 0,
                }}
              />
              <p style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '8px', textAlign: 'center' as const }}>
                {pennySlider <= 30
                  ? 'Penny sticks to your usual picks'
                  : pennySlider <= 60
                    ? 'Penny suggests a mix of favorites and new finds'
                    : 'Penny loves trying new brands and products'}
              </p>
            </div>

            {/* Deals toggle — only show if user has messaging (phone) */}
            {settingsPennyPhone && (
              <div onClick={toggleDeals} style={listStyles.settingsCard}>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={TEXT} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/>
                </svg>
                <div style={{ flex: 1 }}>
                  <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT }}>I want to text with Penny</div>
                  <div style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY, marginTop: '1px' }}>Penny texts you about deals and savings she finds</div>
                </div>
                <ToggleSwitch value={pennyDeals} onChange={toggleDeals} />
              </div>
            )}

            {/* ── Divider ── */}
            <div style={{ height: '1px', backgroundColor: BORDER, margin: '4px 0' }} />

            {/* ── Contact Penny ── */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '14px' }}>
              <PennyAvatar size={52} />
              <div>
                <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT }}>Penny</div>
                <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, marginTop: '2px' }}>Your household's AI shopping assistant</div>
              </div>
              <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '16px' }}>
                {pennyPhoneFormatted && (
                  <a href={`sms:${settingsPennyPhone}`} style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: '44px', height: '44px', borderRadius: '50%', backgroundColor: BG, textDecoration: 'none', WebkitTapHighlightColor: 'transparent' }} aria-label="Text Penny">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={BRAND} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
                  </a>
                )}
                <a href="mailto:penny@list.thrift.ly" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: '44px', height: '44px', borderRadius: '50%', backgroundColor: BG, textDecoration: 'none', WebkitTapHighlightColor: 'transparent' }} aria-label="Email Penny">
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={BRAND} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/><polyline points="22,6 12,13 2,6"/></svg>
                </a>
                <a href="/api/penny.vcf" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: '44px', height: '44px', borderRadius: '50%', backgroundColor: BG, textDecoration: 'none', WebkitTapHighlightColor: 'transparent' }} aria-label="Add Penny to contacts">
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={BRAND} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/></svg>
                </a>
              </div>
            </div>

          </div>
        </div>
        {renderPennyToast()}
      </div>
    )
  }

  // =========================================================================
  // PROFILE PAGE
  // =========================================================================

  if (screen === 'profile') {
    const initials = [settingsFirstName, settingsLastName].map(n => (n || '')[0]?.toUpperCase() || '').filter(Boolean).join('')
    const displayName = [settingsFirstName, settingsLastName].filter(Boolean).join(' ')
    const userPhoneFormatted = settingsUserPhone ? formatPhone(settingsUserPhone.replace('+1', '')) : ''
    return (
      <div style={listStyles.page}>
        <ConfirmModal open={!!confirmModal} title={confirmModal?.title || ''} message={confirmModal?.message} confirmLabel={confirmModal?.confirmLabel} destructive={confirmModal?.destructive} onConfirm={runConfirmAction} onCancel={closeConfirmModal} />
        {renderPageHeader('Profile', 'profile')}
        <div style={listStyles.listArea}>
          <div style={{ padding: '24px 20px', display: 'flex', flexDirection: 'column', gap: '24px' }}>

            {/* Avatar + display info */}
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '8px' }}>
              <div style={{
                width: 80, height: 80, borderRadius: '50%',
                backgroundColor: BRAND, display: 'flex', alignItems: 'center', justifyContent: 'center',
                fontFamily: FONT, fontSize: '28px', fontWeight: 700, color: WHITE,
              }}>
                {initials || '?'}
              </div>
              {displayName && <div style={{ fontFamily: FONT, fontSize: '18px', fontWeight: 600, color: TEXT }}>{displayName}</div>}
              {userPhoneFormatted && <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY }}>{userPhoneFormatted}</div>}
            </div>

            {/* ── Divider ── */}
            <div style={listStyles.divider} />

            {/* ── Profile fields ── */}
            <div style={{ display: 'flex', gap: '12px' }}>
              <div style={{ flex: 1 }}>
                <FloatingLabelField label="First name">
                  <input type="text" value={settingsFirstName} onChange={e => setSettingsFirstName(e.target.value)} onBlur={handleSettingsAutoSave} placeholder="First" style={listStyles.floatingInput} />
                </FloatingLabelField>
              </div>
              <div style={{ flex: 1 }}>
                <FloatingLabelField label="Last name">
                  <input type="text" value={settingsLastName} onChange={e => setSettingsLastName(e.target.value)} onBlur={handleSettingsAutoSave} placeholder="Last" style={listStyles.floatingInput} />
                </FloatingLabelField>
              </div>
            </div>
            <div>
              <FloatingLabelField label="Email">
                <input type="email" value={settingsEmail} onChange={e => setSettingsEmail(e.target.value)} onBlur={handleSettingsAutoSave} placeholder="your@email.com" style={listStyles.floatingInput} />
              </FloatingLabelField>
              <p style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '4px', paddingLeft: '4px' }}>
                Once set, forward any recipe or list to penny@list.thrift.ly and Penny will add it for you.
              </p>
            </div>
            <div style={{ position: 'relative' }}>
              <FloatingLabelField label="Address">
                <input
                  type="text" value={settingsAddress}
                  onChange={e => {
                    setSettingsAddress(e.target.value)
                    if (addressSuggestTimer.current) clearTimeout(addressSuggestTimer.current)
                    addressSuggestTimer.current = setTimeout(() => fetchAddressSuggestions(e.target.value), 300)
                  }}
                  onBlur={() => { setTimeout(() => setShowAddressSuggestions(false), 200); handleSettingsAutoSave() }}
                  onFocus={() => { if (addressSuggestions.length > 0) setShowAddressSuggestions(true) }}
                  placeholder="Start typing your address…" style={listStyles.floatingInput} autoComplete="off"
                />
              </FloatingLabelField>
              {showAddressSuggestions && addressSuggestions.length > 0 && (
                <div style={{ position: 'absolute', top: '100%', left: 0, right: 0, zIndex: 1000, backgroundColor: '#fff', borderRadius: '10px', boxShadow: '0 4px 20px rgba(0,0,0,0.15)', border: `1px solid ${BORDER}`, marginTop: '4px', overflow: 'hidden' }}>
                  {addressSuggestions.map((s, i) => (
                    <button key={s.placeId} onMouseDown={e => { e.preventDefault(); selectAddressSuggestion(s.placeId) }}
                      style={{ display: 'block', width: '100%', padding: '12px 16px', fontFamily: FONT, fontSize: '14px', color: TEXT, backgroundColor: 'transparent', border: 'none', borderTop: i > 0 ? `1px solid ${BORDER}` : 'none', textAlign: 'left', cursor: 'pointer', lineHeight: 1.4 }}
                    >{s.description}</button>
                  ))}
                </div>
              )}
              <p style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '4px', paddingLeft: '4px' }}>
                Used to find nearby stores
              </p>
            </div>

            {/* ── Divider ── */}
            <div style={listStyles.divider} />

            {/* Sign out */}
            <button onClick={handleLogout} style={{
              background: 'none', border: 'none', cursor: 'pointer', padding: 0,
              fontFamily: FONT, fontSize: '15px', fontWeight: 500, color: ERROR_RED,
              textAlign: 'left' as const, WebkitTapHighlightColor: 'transparent',
            }}>Sign Out</button>

          </div>
        </div>

      </div>
    )
  }

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
                <span style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 700, color: TEXT, textTransform: 'uppercase' as const, letterSpacing: '0.5px' }}>Shared List</span>
              </div>
              {settingsMembers.length > 0 && renderMemberList()}
              <button
                onClick={handleTextInviteLink}
                disabled={inviteLoading}
                style={{
                  ...listStyles.sheetButton,
                  marginTop: '12px',
                  opacity: inviteLoading ? 0.5 : 1,
                }}
              >
                {inviteLoading ? 'Creating link…' : 'Invite someone to share'}
              </button>
              <p style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, marginTop: '8px', lineHeight: 1.5 }}>
                Everyone on your list shops together in real time
              </p>
              {settingsMembers.length > 0 && (<>
                <button
                  onClick={() => openConfirmModal({
                    title: 'Leave shared list?',
                    message: "You'll keep your own list but will no longer share with this group.",
                    confirmLabel: 'Leave',
                    destructive: true,
                    onConfirm: () => {
                      fetch('/api/settings/leave-household', { method: 'POST' })
                        .then(res => {
                          if (res.status === 401) { handleSessionExpired(); return }
                          if (res.ok) {
                            showToast('You left the shared list')
                            loadSettings()
                          } else {
                            res.json().then(d => showToast(d?.error || 'Something went wrong — please try again', 'error')).catch(() => showToast('Something went wrong — please try again', 'error'))
                          }
                        })
                        .catch(() => showToast('Couldn\'t connect — check your connection and try again', 'error'))
                    },
                  })}
                  style={{
                    marginTop: '8px',
                    padding: 0,
                    fontFamily: FONT,
                    fontSize: '13px',
                    fontWeight: 500,
                    color: ERROR_RED,
                    backgroundColor: 'transparent',
                    border: 'none',
                    cursor: 'pointer',
                    textAlign: 'left' as const,
                  }}
                >Leave shared list</button>
                <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '4px' }}>You'll still have your own list — you'll just stop sharing with this group.</div>
              </>)}
            </div>

            {/* ── Friends who live elsewhere section ── */}
            <div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={TEXT_SECONDARY} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/></svg>
                <span style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 700, color: TEXT, textTransform: 'uppercase' as const, letterSpacing: '0.5px' }}>Refer a Friend</span>
              </div>

              <p style={{ fontFamily: FONT, fontSize: '15px', color: TEXT, margin: '0 0 16px 0', lineHeight: 1.6 }}>
                Introduce your friends to Penny — they'll get their own list, and you'll both benefit as more people join.
              </p>

              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '10px 14px', backgroundColor: BG, borderRadius: '10px', border: `1px solid ${BORDER}`, marginBottom: '12px' }}>
                <span style={{ flex: 1, fontFamily: FONT, fontSize: '13px', color: TEXT, wordBreak: 'break-all' as const }}>{`https://app.thrift.ly/r/${referralCode}`}</span>
                <button
                  onClick={() => {
                    navigator.clipboard.writeText(`https://app.thrift.ly/r/${referralCode}`).then(() => showToast('Link copied!')).catch(() => showToast('Could not copy', 'error'))
                  }}
                  style={{ padding: '6px 12px', fontFamily: FONT, fontSize: '12px', fontWeight: 600, color: BRAND, backgroundColor: 'transparent', border: `1px solid ${BRAND}`, borderRadius: '8px', cursor: 'pointer', whiteSpace: 'nowrap' as const }}
                >Copy</button>
              </div>

              <button onClick={handleShare} style={listStyles.sheetButton}>
                Introduce your friends to Penny
              </button>

              {referralFriends.length > 0 && (
                <p style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, marginTop: '8px' }}>
                  {referralFriends.length} {referralFriends.length === 1 ? 'friend' : 'friends'} joined
                </p>
              )}
            </div>


          </div>
        </div>
        {renderPennyToast()}
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

              {/* Item Analytics */}
              {analyticsData && (
                <>
                  <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginTop: '28px', marginBottom: '12px' }}>
                    Item Analytics
                  </div>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '20px' }}>
                    <div style={{ backgroundColor: WHITE, borderRadius: '12px', padding: '16px', border: `1px solid ${BORDER}` }}>
                      <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginBottom: '4px' }}>Items Created</div>
                      <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: TEXT }}>{analyticsData.items_created}</div>
                    </div>
                    <div style={{ backgroundColor: WHITE, borderRadius: '12px', padding: '16px', border: `1px solid ${BORDER}` }}>
                      <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginBottom: '4px' }}>Total Events</div>
                      <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: TEXT }}>{Object.values(analyticsData.events_by_action).reduce((a, b) => a + b, 0)}</div>
                    </div>
                  </div>

                  {/* By Source */}
                  {Object.keys(analyticsData.items_by_source).length > 0 && (
                    <>
                      <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT, marginBottom: '8px' }}>By Source</div>
                      {Object.entries(analyticsData.items_by_source).sort((a, b) => b[1] - a[1]).map(([src, cnt]) => (
                        <div key={src} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: `1px solid ${BORDER}` }}>
                          <span style={{ fontFamily: FONT, fontSize: '14px', color: TEXT }}>{src}</span>
                          <span style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT }}>{cnt}</span>
                        </div>
                      ))}
                    </>
                  )}

                  {/* By Department */}
                  {Object.keys(analyticsData.items_by_department).length > 0 && (
                    <>
                      <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT, marginTop: '16px', marginBottom: '8px' }}>Top Departments</div>
                      {Object.entries(analyticsData.items_by_department).sort((a, b) => b[1] - a[1]).slice(0, 10).map(([dept, cnt]) => (
                        <div key={dept} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: `1px solid ${BORDER}` }}>
                          <span style={{ fontFamily: FONT, fontSize: '14px', color: TEXT }}>{dept}</span>
                          <span style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT }}>{cnt}</span>
                        </div>
                      ))}
                    </>
                  )}

                  {/* Events by Action */}
                  {Object.keys(analyticsData.events_by_action).length > 0 && (
                    <>
                      <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT, marginTop: '16px', marginBottom: '8px' }}>Events by Action</div>
                      {Object.entries(analyticsData.events_by_action).sort((a, b) => b[1] - a[1]).map(([action, cnt]) => (
                        <div key={action} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: `1px solid ${BORDER}` }}>
                          <span style={{ fontFamily: FONT, fontSize: '14px', color: TEXT }}>{action}</span>
                          <span style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT }}>{cnt}</span>
                        </div>
                      ))}
                    </>
                  )}

                  {/* Daily Activity */}
                  {analyticsData.events_by_day.length > 0 && (
                    <>
                      <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT, marginTop: '16px', marginBottom: '8px' }}>Daily Activity</div>
                      {analyticsData.events_by_day.slice(-14).map(d => (
                        <div key={d.date} style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0' }}>
                          <span style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY }}>{d.date}</span>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                            <div style={{ width: `${Math.min(120, Math.max(4, d.count * 3))}px`, height: '8px', backgroundColor: BRAND, borderRadius: '4px' }} />
                            <span style={{ fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: TEXT, minWidth: '24px', textAlign: 'right' }}>{d.count}</span>
                          </div>
                        </div>
                      ))}
                    </>
                  )}
                </>
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

  // Departments that have items, in store-aisle order (+ any unknown depts appended)
  const activeDepts = DEPARTMENTS.filter(d => items.some(i => i.department === d))
  const extraDepts = [...new Set(items.map(i => i.department))].filter(d => !DEPARTMENTS.includes(d))
  if (extraDepts.length > 0) activeDepts.push(...extraDepts)

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
          <div style={{ marginLeft: 'auto' }}>
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
                <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                  {isTripInProgress(trip) && (
                    <button
                      onClick={() => {
                        handleCancelTrip()
                        if (items.length > 0) {
                          const allChecked = items.every(i => i.checked)
                          if (allChecked) {
                            setShowDoneToast(true); setShowFireworks(true); setShowDoneOverlay(true)
                          } else {
                            showToast('Trip complete!')
                          }
                        }
                      }}
                      style={{
                        fontFamily: FONT, fontSize: '12px', fontWeight: 600, color: WHITE,
                        backgroundColor: BRAND, border: 'none', borderRadius: '12px',
                        padding: '4px 12px', cursor: 'pointer', whiteSpace: 'nowrap' as const,
                      }}
                    >Done</button>
                  )}
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

      {/* Scrollable list — tap to dismiss keyboard */}
      <div ref={listAreaRef} style={listStyles.listArea} onClick={e => {
        if (e.target === e.currentTarget || !(e.target as HTMLElement).closest('button, a, input, textarea')) {
          chatInputRef.current?.blur()
        }
      }}>
        {chatLoading && (
          <div style={{
            position: 'absolute',
            inset: 0,
            backgroundColor: 'rgba(0,0,0,0.04)',
            zIndex: 10,
            pointerEvents: 'auto',
          }} />
        )}
        {items.length === 0 && !chatLoading ? (
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
                  aria-expanded={isOpen}
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
                      <BrandLogo brand={item.brand} size={52} opacity={item.checked ? 0.4 : 1} />
                    </div>
                    <div style={listStyles.itemCenter} onClick={() => openDetail(item)}>
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


      {/* Hidden file input for camera/photo */}
      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        multiple
        onChange={handlePhotoSelect}
        style={{ display: 'none' }}
        aria-label="Upload photos"
      />

      {/* Penny — always visible above chat, keyboard-aware */}
      <div style={{ flexShrink: 0, backgroundColor: toast?.type === 'error' ? '#FEE2E2' : '#F5F3EF', borderTop: `1px solid ${BORDER}`, overflow: 'hidden' }}>
        <div style={{ padding: keyboardOpen ? '6px 12px' : '12px 16px', transition: 'padding 200ms ease' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: keyboardOpen ? '8px' : '10px' }}>
            <PennyAvatar size={keyboardOpen ? 28 : 36} />
            <span style={{ flex: 1, fontFamily: FONT, fontSize: keyboardOpen ? '13px' : '14px', color: toast?.type === 'error' ? '#991B1B' : TEXT, fontWeight: 500, display: '-webkit-box' as const, WebkitLineClamp: 3, WebkitBoxOrient: 'vertical' as const, overflow: 'hidden', minHeight: keyboardOpen ? undefined : '20px', transition: 'font-size 200ms ease' }}>
              {pennyDisambig.length > 0 && pennyDisambigIndex < pennyDisambig.length
                ? (() => { const d = pennyDisambig[pennyDisambigIndex]; return d.current_brand ? `${d.product_category} (${d.current_brand}) — which brand?` : `${d.product_category} — which brand?` })()
                : homePennyText}
              {pennyDisambig.length === 0 && homePennyTyping && <span className="cursor-blink" style={{ fontWeight: 300, marginLeft: '1px' }}>|</span>}
            </span>
            {pennyDisambig.length > 1 && pennyDisambigIndex < pennyDisambig.length && keyboardOpen && (
              <span style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY, flexShrink: 0 }}>{pennyDisambigIndex + 1}/{pennyDisambig.length}</span>
            )}
            {pennyDisambig.length === 0 && pennyStatus && <span className="penny-pulse" style={{ width: '6px', height: '6px', borderRadius: '50%', backgroundColor: BRAND, flexShrink: 0 }} />}
          </div>
          {pennyDisambig.length === 0 && !keyboardOpen && homePennyBtns.length > 0 && (
            <div style={{ display: 'flex', gap: '8px', marginTop: '8px', paddingLeft: keyboardOpen ? '36px' : '46px' }}>
              {homePennyBtns.map((btn, i) => (
                <button key={i} onClick={btn.onClick} style={pennyBtnStyle}>{btn.label}</button>
              ))}
            </div>
          )}
        </div>
        {pennyDisambig.length > 0 && pennyDisambigIndex < pennyDisambig.length && (() => {
          const d = pennyDisambig[pennyDisambigIndex]
          return (
            <div style={{ position: 'relative' }}>
              <div style={{ display: 'flex', flexWrap: 'nowrap', overflowX: 'auto', gap: '6px', padding: keyboardOpen ? '4px 12px 6px 42px' : '4px 16px 10px 56px', WebkitOverflowScrolling: 'touch' as any, scrollbarWidth: 'none' as any, msOverflowStyle: 'none' as any }}>
                {d.current_brand && !d.alternatives.includes(d.current_brand) && (
                  <button onClick={() => handleBrandPick(d.current_brand)} style={{ ...pennyBtnStyle, padding: keyboardOpen ? '4px 10px' : '5px 12px', flexShrink: 0, boxShadow: `0 0 0 2px ${BRAND}` }}>{d.current_brand}</button>
                )}
                {d.alternatives.filter(a => a !== '').map((alt, i) => (
                  <button key={i} onClick={() => handleBrandPick(alt)} style={{
                    ...pennyBtnStyle,
                    padding: keyboardOpen ? '4px 10px' : '5px 12px',
                    flexShrink: 0,
                    ...(alt === d.current_brand ? { boxShadow: `0 0 0 2px ${BRAND}` } : { background: 'transparent', color: TEXT, border: `1px solid ${BORDER}` }),
                  }}>{alt}</button>
                ))}
                <button onClick={() => advanceDisambig()} style={{ ...pennyBtnStyle, background: 'transparent', color: TEXT_SECONDARY, border: `1px solid ${BORDER}`, padding: keyboardOpen ? '4px 10px' : '5px 12px', flexShrink: 0 }}>Skip</button>
                {pennyDisambig.length > 1 && !keyboardOpen && (
                  <span style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY, alignSelf: 'center', paddingLeft: '4px', flexShrink: 0 }}>{pennyDisambigIndex + 1}/{pennyDisambig.length}</span>
                )}
              </div>
              <div style={{ position: 'absolute', right: 0, top: 0, bottom: 0, width: '28px', background: `linear-gradient(to right, transparent, ${toast?.type === 'error' ? '#FEE2E2' : '#F5F3EF'})`, pointerEvents: 'none' }} />
            </div>
          )
        })()}
      </div>


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
                aria-label={`Remove image ${idx + 1}`}
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

      {/* Bottom chat bar — pill with + icon outside (iOS style) */}
      <div style={listStyles.chatBar}>
        <div style={{ display: 'flex', alignItems: 'flex-end', gap: '4px' }}>
          <button
            title="Add photo"
            onMouseDown={e => { e.preventDefault(); fileInputRef.current?.click() }}
            onTouchEnd={e => { e.preventDefault(); fileInputRef.current?.click() }}
            style={{ flexShrink: 0, width: '34px', height: '34px', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'none', border: 'none', cursor: 'pointer', WebkitTapHighlightColor: 'transparent', padding: 0, marginBottom: '3px' }}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke={BRAND} strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          </button>
        <div style={listStyles.chatPill}>
          <textarea
            ref={chatInputRef}
            rows={1}
            enterKeyHint="send"
            placeholder={pennyDisambig.length > 0 && pennyDisambigIndex < pennyDisambig.length ? "Or type a brand name\u2026" : "Tell Penny what you need\u2026"}
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
            style={{ ...listStyles.chatPillInput, resize: 'none' as const, overflowY: 'hidden' as const, maxHeight: '120px' }}
          />
          <button
            onClick={handleSendChat}
            disabled={(!chatText.trim() && pendingImages.length === 0) || chatLoading}
            style={listStyles.chatPillSend((!!chatText.trim() || pendingImages.length > 0) && !chatLoading)}
          >
            {chatLoading
              ? <span style={{ fontSize: '12px', fontFamily: FONT }}>…</span>
              : <svg width="18" height="18" viewBox="0 0 20 20" fill="none">
                  <path d="M18 2L9 11M18 2l-6 16-3-7-7-3 16-6z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
            }
          </button>
        </div>
        </div>
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
                  Everyone on your list can see this.
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
            <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
              <p style={{ fontFamily: FONT, fontSize: '12px', fontWeight: 600, color: TEXT, margin: 0 }}>When?</p>
              <div style={{ display: 'flex', gap: '6px', flexWrap: 'nowrap', alignItems: 'center' }}>
                {[
                  { label: 'Today', value: todayStr },
                  { label: 'Tomorrow', value: tomorrowStr },
                  { label: 'Weekend', value: satStr },
                ].map(opt => (
                  <button
                    key={opt.value}
                    onClick={() => setTripDay(opt.value)}
                    style={{
                      padding: '6px 12px', borderRadius: '20px', border: `1.5px solid ${tripDay === opt.value ? BRAND : BORDER}`,
                      backgroundColor: tripDay === opt.value ? BRAND : WHITE, color: tripDay === opt.value ? WHITE : TEXT,
                      fontFamily: FONT, fontSize: '14px', fontWeight: 500, cursor: 'pointer', whiteSpace: 'nowrap', flexShrink: 0,
                    }}
                  >{opt.label}</button>
                ))}
                <select
                  value={getDayOptions().some(o => o.value === tripDay) ? tripDay : ''}
                  onChange={e => setTripDay(e.target.value)}
                  style={{
                    padding: '6px 10px', borderRadius: '20px',
                    border: `1.5px solid ${getDayOptions().some(o => o.value === tripDay) ? BRAND : BORDER}`,
                    backgroundColor: getDayOptions().some(o => o.value === tripDay) ? BRAND : WHITE,
                    color: getDayOptions().some(o => o.value === tripDay) ? WHITE : TEXT_SECONDARY,
                    fontFamily: FONT, fontSize: '14px', cursor: 'pointer', flexShrink: 0, minWidth: 0,
                  }}
                >
                  <option value="">More...</option>
                  {getDayOptions().map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                </select>
              </div>

              <p style={{ fontFamily: FONT, fontSize: '12px', fontWeight: 600, color: TEXT, margin: 0 }}>
                What time? <span style={{ fontWeight: 400, color: TEXT_SECONDARY }}>(optional)</span>
              </p>
              {tripTimeLabel === 'exact' ? (
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <input
                    type="time"
                    value={tripExactTime}
                    onChange={e => setTripExactTime(e.target.value)}
                    autoFocus
                    style={{
                      flex: 1, padding: '6px 12px', borderRadius: '20px',
                      border: `1.5px solid ${BRAND}`,
                      fontFamily: FONT, fontSize: '14px', color: TEXT,
                      height: '36px', boxSizing: 'border-box' as const,
                    }}
                  />
                  <button
                    onClick={() => setTripTimeLabel('')}
                    style={{
                      padding: '6px 12px', borderRadius: '20px', border: `1.5px solid ${BORDER}`,
                      backgroundColor: WHITE, color: TEXT_SECONDARY,
                      fontFamily: FONT, fontSize: '14px', cursor: 'pointer', whiteSpace: 'nowrap', flexShrink: 0,
                    }}
                  >Clear</button>
                </div>
              ) : (
                <div style={{ display: 'flex', gap: '6px', flexWrap: 'nowrap' }}>
                  {['Morning', 'Afternoon', 'Evening', 'Exact'].map(t => (
                    <button
                      key={t}
                      onClick={() => setTripTimeLabel(tripTimeLabel === t.toLowerCase() ? '' : t.toLowerCase())}
                      style={{
                        flex: 1, padding: '6px 4px', borderRadius: '20px',
                        border: `1.5px solid ${tripTimeLabel === t.toLowerCase() ? BRAND : BORDER}`,
                        backgroundColor: tripTimeLabel === t.toLowerCase() ? BRAND : WHITE,
                        color: tripTimeLabel === t.toLowerCase() ? WHITE : TEXT,
                        fontFamily: FONT, fontSize: '13px', fontWeight: 500, cursor: 'pointer',
                      }}
                    >{t}</button>
                  ))}
                </div>
              )}

              <button
                onClick={() => handleSetTrip(false)}
                disabled={!tripDay}
                style={{ ...listStyles.sheetButton, padding: '12px', marginTop: '2px', opacity: tripDay ? 1 : 0.4 }}
              >Save</button>
              <button
                onClick={() => setShowTripSheet(false)}
                style={{
                  background: 'none', border: 'none', cursor: 'pointer',
                  fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY,
                  padding: '4px 0', textAlign: 'center' as const, width: '100%',
                }}
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
              <PennyAvatar />
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
                    placeholder={`What's behind your score of ${npsScore}? (optional)`}
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
        <div style={listStyles.overlay} onClick={() => closeEditSheet()} role="presentation">
          <div role="dialog" aria-modal="true" aria-label="Edit item" style={{
            width: '100%', height: '100dvh', backgroundColor: WHITE,
            display: 'flex', flexDirection: 'column' as const,
          }} onClick={e => e.stopPropagation()}>
            {/* Header */}
            <div style={{ display: 'flex', alignItems: 'center', padding: '16px 20px', borderBottom: `1px solid ${BORDER}`, flexShrink: 0 }}>
              <button onClick={() => closeEditSheet()} style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '4px', display: 'flex', color: TEXT }}>
                <svg width="20" height="20" viewBox="0 0 20 20" fill="none"><path d="M15 5L5 15M5 5l10 10" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round"/></svg>
              </button>
              <span style={{ flex: 1, textAlign: 'center', fontFamily: FONT, fontSize: '17px', fontWeight: 600, color: TEXT }}>Edit item</span>
              <div style={{ width: '28px' }} />
            </div>
            {/* Scrollable content */}
            <div style={{ flex: 1, overflowY: 'auto' as const, padding: '20px', display: 'flex', flexDirection: 'column' as const, gap: '14px' }}>
            {/* Brand field with logo */}
            <div style={{ display: 'flex', alignItems: 'flex-end', gap: '12px' }}>
              <div style={{ flex: 1 }}>
                <FloatingLabelField label="Brand">
                  <input
                    type="text"
                    value={editForm.brand}
                    onChange={e => setEditForm(f => ({ ...f, brand: e.target.value }))}
                    onBlur={() => setDisplayBrand(editForm.brand)}
                    placeholder="Optional"
                    style={listStyles.floatingInput}
                  />
                </FloatingLabelField>
              </div>
              <BrandLogo brand={displayBrand || undefined} size={44} />
            </div>

            <FloatingLabelField label="Name">
              <input
                type="text"
                value={editForm.name}
                onChange={e => setEditForm(f => ({ ...f, name: e.target.value }))}
                placeholder="Item name"
                style={listStyles.floatingInput}
                autoFocus
              />
            </FloatingLabelField>

            <FloatingLabelField label="Department">
              <select
                value={editForm.dept}
                onChange={e => setEditForm(f => ({ ...f, dept: e.target.value }))}
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
                    value={editForm.size}
                    onChange={e => setEditForm(f => ({ ...f, size: e.target.value }))}
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
                  <QuantityControl value={editForm.qty} onChange={(v: number) => setEditForm(f => ({ ...f, qty: v }))} />
                </FloatingLabelField>
              </div>
            </div>

            <FloatingLabelField label="Notes">
              <input
                type="text"
                value={editForm.notes}
                onChange={e => setEditForm(f => ({ ...f, notes: e.target.value }))}
                placeholder="e.g. Tangerine flavor, organic"
                style={listStyles.floatingInput}
              />
            </FloatingLabelField>

            {/* Urgent toggle */}
            {selectedItem && (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 0', borderTop: `1px solid ${BORDER}`, marginTop: '4px' }}>
                <span style={{ fontFamily: FONT, fontSize: '15px', color: TEXT }}>🔥 Urgent</span>
                <ToggleSwitch label="Urgent" value={selectedItem.urgent} onChange={() => { handleUrgentToggle(selectedItem.id); setSelectedItem(prev => prev ? { ...prev, urgent: !prev.urgent } : prev) }} />
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
            </div>
            {/* Anchored bottom buttons */}
            <div style={{ flexShrink: 0, padding: '16px 20px', paddingBottom: 'max(16px, env(safe-area-inset-bottom))', borderTop: `1px solid ${BORDER}`, display: 'flex', flexDirection: 'column' as const, gap: '8px' }}>
              <button
                onClick={handleEditSave}
                disabled={!editForm.name.trim() || editLoading}
                style={{
                  ...listStyles.sheetButton,
                  opacity: editForm.name.trim() && !editLoading ? 1 : 0.4,
                }}
              >
                {editLoading ? 'Saving…' : 'Save'}
              </button>
              <button onClick={() => openConfirmModal({ title: 'Delete Item', message: 'Remove this item from your list?', confirmLabel: 'Delete', destructive: true, onConfirm: () => handleEditDelete() })} disabled={editLoading} style={listStyles.sheetDeleteBtn}>
                Delete item
              </button>
            </div>
          </div>
        </div>
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
                        style={{ width: '34px', height: '34px', border: 'none', background: 'none', fontSize: '17px', color: BRAND, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
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
                        style={{ width: '34px', height: '34px', border: 'none', background: 'none', fontSize: '17px', color: BRAND, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
                      >+</button>
                    </div>
                    <button
                      onClick={async () => {
                        const delRes = await fetchAction(`/api/list/${item.id}`, { method: 'DELETE' })
                        if (!delRes.ok) return
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
                          On list: {existing?.quantity ?? 1}{existing && <> · <span style={{ color: BRAND, fontSize: '12px' }}>edit</span></>}
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
                    const dr = await fetchAction(`/api/list/${item.id}`, { method: 'DELETE' })
                    if (!dr.ok) return
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

export default function App() {
  return <ErrorBoundary><AppInner /></ErrorBoundary>
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

  error: {
    fontFamily: FONT,
    fontSize: '15px',
    color: ERROR_RED,
    margin: '0 0 12px 0',
    fontWeight: 500,
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
    padding: '10px 20px 10px 0',
    marginTop: isFirst ? 0 : '8px',
    background: open ? `${BRAND}18` : '#F4F2EE',
    border: 'none',
    borderTop: isFirst ? 'none' : `1px solid ${BORDER}`,
    borderBottom: `1px solid ${BORDER}`,
    borderLeft: open ? `3px solid ${BRAND}` : '3px solid transparent',
    cursor: 'pointer',
    textAlign: 'left' as const,
    WebkitTapHighlightColor: 'transparent',
    gap: '0px',
    fontFamily: FONT,
    fontSize: '13px',
    fontWeight: 700,
    color: open ? BRAND : '#4A4A4A',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.8px',
  }),

  deptCount: {
    fontSize: '12px',
    fontWeight: 600,
    color: BRAND,
    marginLeft: 'auto',
    backgroundColor: `${BRAND}14`,
    borderRadius: '10px',
    padding: '1px 7px',
  } as React.CSSProperties,

  deptChevron: (open: boolean): React.CSSProperties => ({
    fontSize: '17px',
    color: open ? BRAND : TEXT_SECONDARY,
    transform: open ? 'rotate(90deg)' : 'none',
    transition: 'transform 0.15s, color 0.15s',
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
    padding: '10px 10px 10px 20px',
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
    padding: '10px 0',
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


  chatBar: {
    flexShrink: 0,
    padding: '8px 8px 8px 4px',
    paddingBottom: 'max(8px, env(safe-area-inset-bottom))',
    backgroundColor: WHITE,
    borderTop: `1.5px solid #D4CFC7`,
    boxShadow: '0 -2px 12px rgba(0,0,0,0.06)',
  } as React.CSSProperties,

  chatPill: {
    flex: 1,
    display: 'flex',
    alignItems: 'flex-end',
    backgroundColor: BG,
    border: `1.5px solid #C5C0B8`,
    borderRadius: '24px',
    overflow: 'hidden',
    minWidth: 0,
  } as React.CSSProperties,

  chatPillBtn: {
    flexShrink: 0,
    width: '40px',
    height: '40px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    WebkitTapHighlightColor: 'transparent',
  } as React.CSSProperties,

  chatPillInput: {
    flex: 1,
    padding: '10px 14px',
    fontSize: '16px',
    fontFamily: FONT,
    color: TEXT,
    backgroundColor: 'transparent',
    border: 'none',
    outline: 'none',
    minWidth: 0,
  } as React.CSSProperties,

  chatPillSend: (active: boolean): React.CSSProperties => ({
    flexShrink: 0,
    width: '40px',
    height: '40px',
    borderRadius: '50%',
    backgroundColor: 'transparent',
    color: active ? BRAND : TEXT_SECONDARY,
    border: 'none',
    cursor: active ? 'pointer' : 'default',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    transition: 'color 0.15s',
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

  sheetRow: {
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
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

  // Shared card container (used for toggle rows, settings cards)
  settingsCard: {
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
    padding: '12px 16px',
    backgroundColor: BG,
    borderRadius: '12px',
    cursor: 'pointer',
    WebkitTapHighlightColor: 'transparent',
  } as React.CSSProperties,

  // Subtle text link (used for "wrong number?", "resend code", etc.)
  subtleLink: {
    fontFamily: FONT,
    fontSize: '13px',
    color: TEXT_SECONDARY,
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    padding: 0,
    textDecoration: 'underline',
    textDecorationColor: `${TEXT_SECONDARY}40`,
    textUnderlineOffset: '2px',
  } as React.CSSProperties,

  // Divider line
  divider: {
    height: '1px',
    backgroundColor: BORDER,
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

  /* Slider thumb styling */
  input[type="range"]::-webkit-slider-thumb {
    -webkit-appearance: none;
    width: 22px;
    height: 22px;
    border-radius: 50%;
    background: #2D6A4F;
    border: 2px solid white;
    box-shadow: 0 1px 4px rgba(0,0,0,0.2);
    cursor: pointer;
  }
  input[type="range"]::-moz-range-thumb {
    width: 22px;
    height: 22px;
    border-radius: 50%;
    background: #2D6A4F;
    border: 2px solid white;
    box-shadow: 0 1px 4px rgba(0,0,0,0.2);
    cursor: pointer;
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
