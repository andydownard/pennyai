// =============================================================================
// thriftly — App.tsx
//
// Screens: phone → code → home, penny settings, profile, share, admin
// All styles inline. All API calls go to the Go backend.
// =============================================================================

import { useState, useRef, useEffect, useCallback } from 'react'

import { PennyAvatar } from './components/PennyAvatar'
import { ActionSheet } from './components/ActionSheet'
import { ConfirmModal } from './components/ConfirmModal'
import { Confetti } from './components/Confetti'
import { FloatingLabelField } from './components/FloatingLabelField'
import { ErrorBoundary } from './components/ErrorBoundary'
import { type SwapOffer } from './components/SwapCard'
import { ItemRow, type ListItem } from './components/ItemRow'
import { PennyBar } from './components/PennyBar'
import { NPSSurveyModal } from './components/NPSSurveyModal'
import { track, identify } from './analytics'
import { TripPlannerSheet, type ShoppingTrip } from './components/TripPlannerSheet'
import { ItemEditSheet } from './components/ItemEditSheet'
import { ReviewSheet } from './components/ReviewSheet'
import { EarningsScreen } from './components/EarningsScreen'
import { ReceiptScreen } from './components/ReceiptScreen'
import { BottomTabBar, type TabScreen } from './components/BottomTabBar'
import { type ItemModification } from './types'
import {
  BRAND, BG, TEXT, TEXT_SECONDARY, BORDER, ERROR_RED, ERROR_RED_BG, ERROR_RED_DARK, ERROR_RED_LIGHT,
  WHITE, FONT, DEPARTMENTS, DEPT_ICONS, SKELETON, SKELETON_SHIMMER,
} from './styles'

// =============================================================================
// TYPES
// =============================================================================

// The screens the app can show
type Screen = 'phone' | 'code' | 'home' | 'penny' | 'profile' | 'settings-share' | 'admin' | 'email-login' | 'email-register' | 'forgot-password' | 'reset-password' | 'shared-list' | 'help' | 'earnings' | 'receipts'

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
  'shared-list': '/shared',
  help: '/help',
  earnings: '/earnings',
  receipts: '/receipts',
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
  '/help': 'help',
  '/earnings': 'earnings',
  '/receipts': 'receipts',
}

const AUTH_SCREENS: Screen[] = ['phone', 'code', 'email-login', 'email-register', 'forgot-password', 'reset-password']

// =============================================================================
// CONSTANTS
// =============================================================================


// Relative timestamp for recently-added items (< 1 hour). Returns null for older items.
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
// HAPTIC FEEDBACK
// =============================================================================

const haptic = {
  light:   () => navigator.vibrate?.([5]),
  tap:     () => navigator.vibrate?.([10]),
  medium:  () => navigator.vibrate?.([15]),
  heavy:   () => navigator.vibrate?.([20]),
  success: () => navigator.vibrate?.([10, 50, 5, 50, 15]),
  error:   () => navigator.vibrate?.([20, 40, 20]),
  double:  () => navigator.vibrate?.([5, 30, 5]),
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

  // Shared list deep link state
  const [sharedCode, setSharedCode] = useState<string | null>(null)
  const [sharedData, setSharedData] = useState<{ items: { name: string; quantity: number; checked: boolean }[]; sharer_name: string; expires_at: string; invite_code?: string } | null>(null)
  const [sharedError, setSharedError] = useState<string | null>(null)
  const [sharedLoading, setSharedLoading] = useState(false)
  // Post-auth join prompt
  const [showJoinPrompt, setShowJoinPrompt] = useState(false)
  const [joinPromptName, setJoinPromptName] = useState('')
  const [joinPromptCode, setJoinPromptCode] = useState('')


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
  const [listLoading, setListLoading] = useState(true)
  const [pullRefreshing, setPullRefreshing] = useState(false)
  const pullStartY = useRef(0)
  const [pullDistance, setPullDistance] = useState(0)
  const isPulling = useRef(false)
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
  const receiptInputRef = useRef<HTMLInputElement>(null)
  const chatInputRef = useRef<HTMLTextAreaElement>(null)

  // Menu state is now internal to NavigationMenu component

  // --- Done toast state ---
  const [showDoneToast, setShowDoneToast] = useState(false)
  const [showFireworks, setShowFireworks] = useState(false)
  const [showDoneOverlay, setShowDoneOverlay] = useState(false)
  const [showDoneSummary, setShowDoneSummary] = useState(false)
  const [doneSummaryData, setDoneSummaryData] = useState<{ checkedCount: number; totalCount: number; tripStartedAt: string } | null>(null)
  const [isFirstTrip, setIsFirstTrip] = useState(false)

  // --- Floating label animation state ---
  const [floatingLabels, setFloatingLabels] = useState<Array<{ text: string; key: number }>>([])

  // --- Shopping trip planner state ---
  const [trips, setTrips] = useState<ShoppingTrip[]>([])
  const [showTripSheet, setShowTripSheet] = useState(false)

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

  // --- Onboarding state (persisted via localStorage + server) ---
  const [loginCount, setLoginCount] = useState(0)
  const onboardingNameAsked = useRef(localStorage.getItem('ob_name') === '1')
  const onboardingShareMentioned = useRef(localStorage.getItem('ob_share') === '1')
  const onboardingTextMentioned = useRef(localStorage.getItem('ob_text') === '1')
  const onboardingSwapsMentioned = useRef(localStorage.getItem('ob_swaps') === '1')
  const nameInputMode = useRef(false)

  const syncOnboardingFlags = () => {
    const flags: Record<string, string> = {}
    if (onboardingNameAsked.current) flags.ob_name = '1'
    if (onboardingShareMentioned.current) flags.ob_share = '1'
    if (onboardingTextMentioned.current) flags.ob_text = '1'
    if (onboardingSwapsMentioned.current) flags.ob_swaps = '1'
    fetchAPI('/api/settings', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ onboarding_flags: JSON.stringify(flags) }),
    }).catch(() => {})
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
  const [listName, setListName] = useState('')
  const [editingListName, setEditingListName] = useState(false)
  const [listNameDraft, setListNameDraft] = useState('')
  const [settingsPennyPhone, setSettingsPennyPhone] = useState('')
  const [settingsUserPhone, setSettingsUserPhone] = useState('')
  const [inviteLoading, setInviteLoading] = useState(false)
  const [avatarVersion, setAvatarVersion] = useState(0)
  const [avatarUploading, setAvatarUploading] = useState(false)
  const [avatarError, setAvatarError] = useState(false)

  // --- Referral state ---
  const [referralCode, setReferralCode] = useState('')
  const [referralFriends, setReferralFriends] = useState<{ name: string; created_at: string }[]>([])

  // --- Swap offer state ---
  const [swapsByItemId, setSwapsByItemId] = useState<Record<number, SwapOffer>>({})
  const [pendingReceiptMode, setPendingReceiptMode] = useState(false)

  // --- Help / FAQ state ---
  const [openFaq, setOpenFaq] = useState<number | null>(null)
  const [bugDescription, setBugDescription] = useState('')
  const [bugSubmitting, setBugSubmitting] = useState(false)
  const [bugSubmitted, setBugSubmitted] = useState(false)

  // --- Admin state ---
  const [isAdmin, setIsAdmin] = useState(false)
  const [currentUserId, setCurrentUserId] = useState<number | null>(null)
  const [adminDeals, setAdminDeals] = useState<{
    id: number; competitor_product: string; offer_brand: string; offer_product: string;
    discount_cents: number; daily_budget_cents: number; budget_spent_cents: number;
    status: string; starts_at: string; ends_at: string; created_at: string;
    offer_price_cents: number; competitor_price_cents: number;
    offer_size: string; competitor_size: string;
    offer_unit_price: string; competitor_unit_price: string;
    why_bullets: string; allergen_notes: string;
    deal_type: string; quantity_offered: number;
    trial_product_name: string; trial_size: string; campaign_tag: string;
  }[]>([])
  const [showDealForm, setShowDealForm] = useState(false)
  const [dealForm, setDealForm] = useState({
    competitor_product: '', offer_brand: '', offer_product: '',
    discount_cents: 0, daily_budget_cents: 0, status: 'active', ends_at: '',
    offer_price_cents: 0, competitor_price_cents: 0,
    offer_size: '', competitor_size: '',
    offer_unit_price: '', competitor_unit_price: '',
    why_bullets: '[]', allergen_notes: '',
    deal_type: 'sku_swap', quantity_offered: 1,
    trial_product_name: '', trial_size: '', campaign_tag: '',
  })
  const [editingDealId, setEditingDealId] = useState<number | null>(null)
  const [adminData, setAdminData] = useState<{
    active_users: number; total_users: number; new_users: number;
    k_factor: number; leaderboard: { phone: string; name: string; points: number; referrals: number }[];
    days: number;
    nps_responses: { score: number; comment: string; user_name: string; user_phone: string; created_at: string }[];
    nps_score: number;
    nps_total: number;
    nps_promoters: number;
    nps_detractors: number;
    swap_offered?: number;
    swap_accepted?: number;
    swap_declined?: number;
    swap_accept_rate?: number;
  } | null>(null)
  const [adminDays, setAdminDays] = useState(30)
  const [analyticsData, setAnalyticsData] = useState<{
    days: number; items_created: number;
    items_by_source: Record<string, number>;
    items_by_department: Record<string, number>;
    events_by_action: Record<string, number>;
    events_by_day: { date: string; count: number }[];
  } | null>(null)
  const [adminPayouts, setAdminPayouts] = useState<{
    user_id: number; name: string; email: string; phone: string;
    total_cents: number; swap_count: number; oldest_swap: string;
  }[]>([])

  // --- NPS state ---
  const [showNPS, setShowNPS] = useState(false)

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

  // --- Savings/rebate state ---
  const [swapCreditsCents, setSwapCreditsCents] = useState(0)
  const [showSavingsSheet, setShowSavingsSheet] = useState(false)
  const [rebateHistory, setRebateHistory] = useState<{
    id: number; offer_brand: string; offer_product: string;
    amount_cents: number; status: string; created_at: string;
  }[]>([])
  const [rebateSummary, setRebateSummary] = useState<{ total_cents: number; pending_cents: number; paid_cents: number }>({ total_cents: 0, pending_cents: 0, paid_cents: 0 })
  const [earningsTotalCents, setEarningsTotalCents] = useState(0)
  const [keyboardOpen, setKeyboardOpen] = useState(false)

  // --- Penny chime-in state ---
  const [pennyChimeIn, setPennyChimeIn] = useState<string | null>(null)
  const chimeInTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const prevCheckedRef = useRef(-1)

  const CHIME_IN_MESSAGES = [
    'Looking good!', 'Almost there!', 'Nice work!', 'On a roll!', 'Keep going!',
  ]

  const FIRST_VISIT_GREETINGS = [
    "Hey! I'm Penny. What do you need from the store?",
    "Hi there! I'm Penny, your grocery sidekick. What are we shopping for?",
    "Hey! I'm Penny. Tell me what you need and I'll get your list started.",
  ]

  const RETURNING_GREETINGS = [
    "What are we shopping for today?",
    "Ready when you are — just tell me what you need!",
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

  // Store join code or referral code from URL if present, and redirect unknown paths
  useEffect(() => {
    const path = window.location.pathname
    const joinMatch = path.match(/^\/join\/(.+)$/)
    if (joinMatch) {
      localStorage.setItem('thriftly_join_code', joinMatch[1])
      window.history.replaceState({}, '', '/')
      return
    }
    const referralMatch = path.match(/^\/r\/(.+)$/)
    if (referralMatch) {
      localStorage.setItem('referral_code', referralMatch[1])
      window.history.replaceState({}, '', '/')
      return
    }
    // Shared list deep link — load read-only view (no auth needed)
    const sharedMatch = path.match(/^\/shared\/(.+)$/)
    if (sharedMatch) {
      setSharedCode(sharedMatch[1])
      return // Don't redirect — we'll show the shared-list screen
    }
    // Unknown route: fix URL and flag for toast after auth resolves
    if (path !== '/' && !PATH_TO_SCREEN[path]) {
      window.history.replaceState({}, '', '/')
      sessionStorage.setItem('thriftly_404', '1')
    }
  }, [])

  // Fetch shared list data when a shared code is detected
  useEffect(() => {
    if (!sharedCode) return
    setSharedLoading(true)
    fetch(`/api/shared/${sharedCode}`)
      .then(res => {
        if (res.status === 410) {
          setSharedError('expired')
          return null
        }
        if (!res.ok) {
          setSharedError('not_found')
          return null
        }
        return res.json()
      })
      .then(data => {
        if (data) {
          setSharedData(data)
          // Stash invite code for post-auth join prompt
          if (data.invite_code) {
            localStorage.setItem('thriftly_share_invite', data.invite_code)
            localStorage.setItem('thriftly_share_name', data.sharer_name || '')
          }
        }
      })
      .catch(() => setSharedError('not_found'))
      .finally(() => setSharedLoading(false))
  }, [sharedCode])

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
        const data = await res.json().catch(() => null)
        if (res.ok) {
          localStorage.removeItem('thriftly_join_code')
          showToast(data?.penny_message || 'You joined the shared list!')
        } else {
          localStorage.removeItem('thriftly_join_code')
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
          res.clone().json().then((data: { is_admin?: boolean; user_id?: number; login_count?: number; name?: string }) => {
            if (data.is_admin) setIsAdmin(true)
            if (data.user_id) { setCurrentUserId(data.user_id); identify(data.user_id) }
            if (data.login_count != null) setLoginCount(data.login_count)
            if (data.name) {
              const parts = data.name.split(' ')
              setSettingsFirstName(parts[0] || '')
              setSettingsLastName(parts.slice(1).join(' '))
            }
          }).catch(() => {})
          tryJoinCode()
          loadSettings() // Load settings once at auth
          // Check for post-auth join prompt from shared list link
          const shareInvite = localStorage.getItem('thriftly_share_invite')
          const shareName = localStorage.getItem('thriftly_share_name')
          if (shareInvite) {
            localStorage.removeItem('thriftly_share_invite')
            localStorage.removeItem('thriftly_share_name')
            setJoinPromptCode(shareInvite)
            setJoinPromptName(shareName || '')
            setShowJoinPrompt(true)
            setScreen('home', true)
          } else {
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
      .finally(() => {
        setChecking(false)
        // Show 404 toast if user landed on an unknown route
        if (sessionStorage.getItem('thriftly_404')) {
          sessionStorage.removeItem('thriftly_404')
          showToast("Page not found — here's your list")
        }
      })
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
    // Restore pending swaps from server
    try {
      const swaps = await fetchAPI('/api/list/swaps')
      if (Array.isArray(swaps) && swaps.length > 0) {
        const byItem: Record<number, SwapOffer> = {}
        for (const s of swaps) byItem[(s as SwapOffer).item_id] = s as SwapOffer
        setSwapsByItemId(prev => ({ ...prev, ...byItem }))
      }
    } catch { /* swaps are optional */ }
    setListLoading(false)
  }

  const loadTrips = async () => {
    const data = await fetchAPI('/api/trips')
    if (data) setTrips(Array.isArray(data) ? data as ShoppingTrip[] : [])
  }

  const npsCheckedRef = useRef(false)
  useEffect(() => {
    if (screen !== 'home') return
    reloadList().catch(e => console.warn('[load] list failed:', e))
    loadTrips().catch(e => console.warn('[load] trips failed:', e))
    // Only check NPS once per session — never re-prompt after dismiss or submit
    if (!npsCheckedRef.current && !sessionStorage.getItem('nps_dismissed')) {
      npsCheckedRef.current = true
      fetchAPI('/api/nps/check')
        .then(data => { if (data && !Array.isArray(data) && data.show_nps) setShowNPS(true) })
        .catch(e => console.warn('[load] nps check failed:', e))
    }
  }, [screen])

  // --- Pull-to-refresh handlers ---
  useEffect(() => {
    if (screen !== 'home') return
    const el = listAreaRef.current
    if (!el) return

    const onTouchStart = (e: TouchEvent) => {
      if (el.scrollTop <= 0 && !pullRefreshing) {
        pullStartY.current = e.touches[0].clientY
        isPulling.current = true
      }
    }
    const onTouchMove = (e: TouchEvent) => {
      if (!isPulling.current) return
      const dy = e.touches[0].clientY - pullStartY.current
      if (dy > 0) {
        // Dampen the pull distance (feels more natural)
        setPullDistance(Math.min(dy * 0.5, 80))
        if (dy > 20) e.preventDefault()
      } else {
        isPulling.current = false
        setPullDistance(0)
      }
    }
    const onTouchEnd = () => {
      if (!isPulling.current) return
      isPulling.current = false
      if (pullDistance >= 50) {
        setPullRefreshing(true)
        setPullDistance(50)
        reloadList()
          .then(() => loadTrips())
          .catch(e => console.warn('[pull-refresh] failed:', e))
          .finally(() => {
            setPullRefreshing(false)
            setPullDistance(0)
          })
      } else {
        setPullDistance(0)
      }
    }

    el.addEventListener('touchstart', onTouchStart, { passive: true })
    el.addEventListener('touchmove', onTouchMove, { passive: false })
    el.addEventListener('touchend', onTouchEnd)
    return () => {
      el.removeEventListener('touchstart', onTouchStart)
      el.removeEventListener('touchmove', onTouchMove)
      el.removeEventListener('touchend', onTouchEnd)
    }
  }, [screen, pullRefreshing, pullDistance])

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

  const handleSetTrip = async (day: string, timeLbl: string, exactTime: string) => {
    if (!day) return
    const timeMap: Record<string, string> = { morning: '09:00', afternoon: '14:00', evening: '18:00' }
    const tripTime = timeLbl === 'exact' ? exactTime : (timeMap[timeLbl] || '')
    const timeLabel = timeLbl && timeLbl !== 'exact' ? timeLbl : ''
    const data = await fetchAPI('/api/trips', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ trip_date: day, trip_time: tripTime, time_label: timeLabel }),
    })
    if (!data) { showToast('Couldn\'t save your trip — please try again', 'error'); return }
    setShowTripSheet(false)
    loadTrips()
    showToast('Trip added! Everyone will see it.')
  }

  const handleCancelTrip = async () => {
    const data = await fetchAPI('/api/trips', { method: 'DELETE' })
    if (!data) { showToast('Couldn\'t cancel your trip — please try again', 'error'); return }
    loadTrips()
    showToast('Trip cancelled')
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
          if (data.swap_credits_cents != null) {
            setSwapCreditsCents(data.swap_credits_cents as number)
            setEarningsTotalCents(data.swap_credits_cents as number)
          }
          if (data.list_name != null) setListName(data.list_name as string)
          // Hydrate onboarding flags from server (server is source of truth)
          if (data.onboarding_flags && typeof data.onboarding_flags === 'string') {
            try {
              const flags = JSON.parse(data.onboarding_flags as string) as Record<string, string>
              if (flags.ob_name) { onboardingNameAsked.current = true; localStorage.setItem('ob_name', '1') }
              if (flags.ob_share) { onboardingShareMentioned.current = true; localStorage.setItem('ob_share', '1') }
              if (flags.ob_text) { onboardingTextMentioned.current = true; localStorage.setItem('ob_text', '1') }
              if (flags.ob_swaps) { onboardingSwapsMentioned.current = true; localStorage.setItem('ob_swaps', '1') }
            } catch { /* ignore parse errors */ }
          }
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
    fetch('/api/admin/deals').then(r => r.ok ? r.json() : []).then(setAdminDeals).catch(() => {})
    fetch('/api/admin/payouts').then(r => r.ok ? r.json() : []).then(setAdminPayouts).catch(() => {})
  }, [screen, adminDays])

  const showToast = (message: string, type: 'success' | 'error' = 'success', action?: { label: string; onClick: () => void }) => {
    if (type === 'error') haptic.error()
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
        { label: `Clear all ${items.length} items`, onClick: () => { setShowDoneToast(false); setShowFireworks(false); handleClearList(true) } },
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
      if (loginCount <= 1 && !settingsFirstName) {
        // First visit, no name yet — introduce Penny
        typePennyHome(pickRandom(FIRST_VISIT_GREETINGS))
      } else if (settingsFirstName) {
        // We know their name — personal greeting
        typePennyHome(`Hey ${settingsFirstName}! What do we need from the store?`)
      } else {
        typePennyHome(pickRandom(RETURNING_GREETINGS))
      }
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

    // Receipt mode: send photo directly as receipt
    if (pendingReceiptMode && files.length > 0) {
      const file = files[0]
      const reader = new FileReader()
      reader.onload = () => {
        const dataUrl = reader.result as string
        handleReceiptSubmit([dataUrl])
      }
      reader.readAsDataURL(file)
      e.target.value = ''
      return
    }

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

  // --- Swap accept/decline ---
  const handleSwapAccept = async (dealId: number, itemId: number) => {
    haptic.success()
    try {
      const res = await fetch('/api/list/parse', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: 'accept_swap', swap_deal_id: dealId, swap_item_id: itemId }),
      })
      if (res.status === 401) { handleSessionExpired(); return }
      if (res.ok) {
        const data = await res.json()
        if (data.updated?.length > 0) {
          setItems(prev => {
            const map = new Map(prev.map(i => [i.id, i]))
            for (const item of data.updated) map.set(item.id, item)
            return sortItems(Array.from(map.values()))
          })
        }
        setSwapsByItemId(prev => { const next = { ...prev }; delete next[itemId]; return next })
        if (data.swap_credits_cents != null) {
          setSwapCreditsCents(data.swap_credits_cents)
          setEarningsTotalCents(data.swap_credits_cents)
        }
        showToast(data.message || 'Swapped!')
      } else {
        const err = await res.json().catch(() => null)
        showToast(err?.error || 'Couldn\'t accept the swap', 'error')
      }
    } catch { showToast('Connection error — try again', 'error') }
  }

  const handleSwapDecline = async (dealId: number, itemId: number) => {
    haptic.double()
    track('swap_declined', { deal_id: dealId })
    setSwapsByItemId(prev => { const next = { ...prev }; delete next[itemId]; return next })
    try {
      await fetch('/api/list/parse', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: 'decline_swap', swap_deal_id: dealId, swap_item_id: itemId }),
      })
    } catch { /* silent — optimistic removal already done */ }
  }

  // --- Receipt submit ---
  const handleReceiptSubmit = async (imageData: string[]) => {
    haptic.tap()
    setChatLoading(true)
    typePennyHome('Reading your receipt...')
    try {
      const res = await fetch('/api/list/parse', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: '', images: imageData, is_receipt: true }),
      })
      if (res.status === 401) { handleSessionExpired(); return }
      if (res.ok) {
        const data = await res.json()
        if (data.checked?.length > 0) {
          const checkedIds = new Set(data.checked.map((i: ListItem) => i.id))
          setItems(prev => sortItems(prev.map(i => checkedIds.has(i.id) ? { ...i, checked: true } : i)))
          typePennyHome(`Found ${data.checked.length} item${data.checked.length !== 1 ? 's' : ''} on your receipt — checked them off!`)
        } else {
          typePennyHome(data.message || 'Receipt processed!')
        }
        showToast(data.message || 'Receipt processed!')
      } else {
        typePennyHome('Hmm, I couldn\'t read that receipt. Try a clearer photo?')
        showToast('Couldn\'t process receipt — try again', 'error')
      }
    } catch {
      typePennyHome('Connection hiccup — try again in a moment.')
      showToast('Connection error — try again', 'error')
    }
    finally {
      setChatLoading(false)
      setPendingReceiptMode(false)
    }
  }

  const handleToggle = async (id: number) => {
    haptic.tap()
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

    // Name input mode: user typed their name in response to "what should I call you?"
    if (nameInputMode.current && chatText.trim() && pendingImages.length === 0) {
      nameInputMode.current = false
      const name = chatText.trim()
      setChatText('')
      // Save name to backend
      const parts = name.split(' ')
      const firstName = parts[0] || ''
      const lastName = parts.slice(1).join(' ')
      setSettingsFirstName(firstName)
      setSettingsLastName(lastName)
      fetch('/api/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name }),
      }).catch(() => {})
      typePennyHome(`Nice to meet you, ${firstName}! Keep adding — I'll sort everything by aisle.`)
      return
    }

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
          // Present clear options based on what's on the list
          const checkedCount = items.filter(i => i.checked).length
          const totalCount = items.length
          if (totalCount === 0) {
            typePennyHome('Your list is already empty!')
          } else if (checkedCount > 0) {
            // Offer both options — make it clear what each does
            const btns: { label: string; onClick: () => void }[] = [
              { label: `Clear ${checkedCount} checked item${checkedCount !== 1 ? 's' : ''}`, onClick: () => { handleClearList(false); typePennyHome(`Cleared ${checkedCount} checked item${checkedCount !== 1 ? 's' : ''}!`) } },
            ]
            if (totalCount > checkedCount) {
              btns.push({ label: `Clear entire list (${totalCount} items)`, onClick: () => { handleClearList(true); typePennyHome('List cleared!') } })
            } else {
              // All items are checked — "clear checked" and "clear all" are the same
              // Just show one option
            }
            btns.push({ label: 'Cancel', onClick: () => typePennyHome('No problem, your list is safe!') })
            typePennyHome(pennyMsg || `You have ${checkedCount} checked and ${totalCount - checkedCount} unchecked item${totalCount - checkedCount !== 1 ? 's' : ''}. What would you like to clear?`, btns)
          } else {
            // No checked items — only option is clear all
            typePennyHome(pennyMsg || `Clear all ${totalCount} item${totalCount !== 1 ? 's' : ''} from your list?`, [
              { label: `Yes, clear all ${totalCount} items`, onClick: () => { handleClearList(true); typePennyHome('List cleared!') } },
              { label: 'Cancel', onClick: () => typePennyHome('No problem, your list is safe!') },
            ])
          }
          return // typePennyHome already handles the message
        } else if (action === 'done_shopping') {
          typePennyHome(pennyMsg || 'Nice trip! Snap your receipt and I\'ll check off what you got.', [
            {
              label: 'Snap receipt',
              onClick: () => {
                setPendingReceiptMode(true)
                receiptInputRef.current?.click()
              },
            },
            { label: 'Skip', onClick: () => typePennyHome('All good! Happy shopping!') },
          ])
          return
        } else if (action === 'receipt_processed') {
          // Receipt was processed — update checked items in local state
          if (checkedItems.length > 0) {
            const checkedIds = new Set(checkedItems.map(i => i.id))
            setItems(prev => sortItems(prev.map(i => checkedIds.has(i.id) ? { ...i, checked: true } : i)))
          }
          setPendingReceiptMode(false)
        } else if (action === 'receipt_duplicate') {
          setPendingReceiptMode(false)
        } else if (action === 'swap_accepted') {
          track('swap_accepted', { items: updatedItems.length })
          // Swap accepted — update the item in local state
          if (updatedItems.length > 0) {
            setItems(prev => {
              const map = new Map(prev.map(i => [i.id, i]))
              for (const item of updatedItems) map.set(item.id, item)
              return sortItems(Array.from(map.values()))
            })
          }
          // Remove swap offer from local state
          setSwapsByItemId(prev => {
            const next = { ...prev }
            for (const item of updatedItems) delete next[item.id]
            return next
          })
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
            track('items_added', { count: addedItems.length })
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

          // Store any pending swap offer
          if (data.pending_swap) {
            const swap = data.pending_swap
            setSwapsByItemId(prev => ({ ...prev, [swap.item_id]: swap }))
            track('swap_offered', { deal_id: swap.deal_id, offer_brand: swap.offer_brand, discount_cents: swap.discount_cents })
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

        // --- Onboarding nudges (spaced, non-intrusive) ---
        if (action === 'add' && addedItems.length > 0) {
          // Session 1: After first add, casually ask for name (once per session)
          if (loginCount <= 1 && !settingsFirstName && !onboardingNameAsked.current) {
            onboardingNameAsked.current = true
            localStorage.setItem('ob_name', '1')
            syncOnboardingFlags()
            setTimeout(() => {
              typePennyHome("What do you need from the store? I'll get your list started.", [
                { label: 'Skip', onClick: () => typePennyHome("No worries! Just type or paste what you need.") },
              ])
              nameInputMode.current = true
            }, 2500)
          }
          // Session 2: Mention sharing (once per session, after 2+ items)
          else if (loginCount === 2 && !onboardingShareMentioned.current && addedItems.length >= 1) {
            onboardingShareMentioned.current = true
            localStorage.setItem('ob_share', '1')
            syncOnboardingFlags()
            setTimeout(() => {
              typePennyHome("Shopping for the family? You can invite them to share this list.", [
                { label: 'Invite someone', onClick: () => handleTextInviteLink() },
                { label: 'Just me', onClick: () => typePennyHome("Got it — solo mission!") },
              ])
            }, 3000)
          }
          // Session 3: Mention texting Penny
          else if (loginCount === 3 && !onboardingTextMentioned.current) {
            onboardingTextMentioned.current = true
            localStorage.setItem('ob_text', '1')
            syncOnboardingFlags()
            setTimeout(() => {
              typePennyHome("Quick tip — you can text me your list anytime. Just save my number!", [
                { label: 'Save contact', onClick: () => { window.open('/api/penny.vcf', '_blank') } },
                { label: 'Maybe later', onClick: () => typePennyHome("I'll be here!") },
              ])
            }, 3000)
          }
          // After 3+ items on list and haven't mentioned swaps yet
          else if (!onboardingSwapsMentioned.current && items.length + addedItems.length >= 3) {
            onboardingSwapsMentioned.current = true
            localStorage.setItem('ob_swaps', '1')
            syncOnboardingFlags()
            setTimeout(() => {
              typePennyHome("By the way — I keep an eye out for deals on your items. If I spot a swap that saves you money, I'll let you know!")
            }, 3500)
          }
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

  const handleNPSSubmit = async (score: number, comment: string) => {
    const result = await fetchAction('/api/nps', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ score, comment }),
    })
    if (result.ok) {
      showToast('Thanks for your feedback!')
      setShowNPS(false)
      sessionStorage.setItem('nps_dismissed', '1')
    } else if (result.status !== 401) {
      showToast('Couldn\'t save feedback — please try again', 'error')
    }
  }

  const closeNPS = () => {
    setShowNPS(false)
    sessionStorage.setItem('nps_dismissed', '1')
  }


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
    haptic.heavy()
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
  const settingsPhoneRef = useRef(settingsUserPhone)
  const settingsAddressRef = useRef(settingsAddress)
  settingsFirstNameRef.current = settingsFirstName
  settingsLastNameRef.current = settingsLastName
  settingsEmailRef.current = settingsEmail
  settingsPhoneRef.current = settingsUserPhone
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
        body: JSON.stringify({ name: [settingsFirstNameRef.current.trim(), settingsLastNameRef.current.trim()].filter(Boolean).join(' '), email: settingsEmailRef.current.trim(), phone: settingsPhoneRef.current.trim() || undefined, address: settingsAddressRef.current.trim() }),
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
    setScreen('phone', true)
    setPhone('')
    setCode(['', '', '', '', '', ''])
    setError('')
  }

  // =========================================================================
  // LOADING STATE (checking existing session)
  // =========================================================================

  // =========================================================================
  // SHARED LIST SCREEN (standalone, no auth required)
  // =========================================================================

  if (sharedCode) {
    const timeLeft = sharedData ? (() => {
      const ms = new Date(sharedData.expires_at).getTime() - Date.now()
      if (ms <= 0) return 'expired'
      const hours = Math.floor(ms / 3600000)
      if (hours > 0) return `${hours} hour${hours !== 1 ? 's' : ''}`
      const mins = Math.floor(ms / 60000)
      return `${mins} minute${mins !== 1 ? 's' : ''}`
    })() : null

    return (
      <div style={{ minHeight: '100vh', backgroundColor: BG, display: 'flex', flexDirection: 'column' }}>
        {/* Header */}
        <div style={{ backgroundColor: WHITE, borderBottom: `1px solid ${BORDER}`, padding: '20px', textAlign: 'center' }}>
          <div style={{ display: 'flex', justifyContent: 'center', marginBottom: '12px' }}>
            <PennyAvatar size={48} />
          </div>
          <div style={{ fontFamily: FONT, fontSize: '20px', fontWeight: 700, color: TEXT }}>
            {sharedData ? `${sharedData.sharer_name || 'Someone'}'s Shopping List` : 'Shared Shopping List'}
          </div>
          <div style={{ fontFamily: FONT, fontSize: '13px', color: BRAND, fontWeight: 600, letterSpacing: '-0.3px', marginTop: '4px' }}>thriftly</div>
        </div>

        {/* Expiry banner */}
        {sharedData && timeLeft && timeLeft !== 'expired' && (
          <div style={{ backgroundColor: '#FEF3C7', padding: '10px 20px', fontFamily: FONT, fontSize: '13px', color: '#92400E', textAlign: 'center' }}>
            This link expires in {timeLeft}
          </div>
        )}

        {/* Content */}
        <div style={{ flex: 1, padding: '16px 20px', overflowY: 'auto' }}>
          {sharedLoading && (
            <div style={{ textAlign: 'center', padding: '40px 0' }}>
              <p style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY }}>Loading list...</p>
            </div>
          )}

          {sharedError === 'expired' && (
            <div style={{ textAlign: 'center', padding: '40px 20px' }}>
              <div style={{ fontSize: '48px', marginBottom: '16px' }}>⏰</div>
              <h2 style={{ fontFamily: FONT, fontSize: '20px', color: TEXT, margin: '0 0 8px' }}>This link has expired</h2>
              <p style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY, margin: '0 0 24px' }}>
                Share links are valid for 48 hours. Ask the sender to share again.
              </p>
              <button
                onClick={() => { setSharedCode(null); setSharedError(null); window.history.replaceState({}, '', '/login/email/register'); setScreen('email-register', true) }}
                style={{ backgroundColor: BRAND, color: WHITE, border: 'none', borderRadius: '10px', padding: '14px 28px', fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer' }}
              >
                Sign up for Thriftly
              </button>
            </div>
          )}

          {sharedError === 'not_found' && (
            <div style={{ textAlign: 'center', padding: '40px 20px' }}>
              <div style={{ fontSize: '48px', marginBottom: '16px' }}>🔗</div>
              <h2 style={{ fontFamily: FONT, fontSize: '20px', color: TEXT, margin: '0 0 8px' }}>Link not found</h2>
              <p style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY, margin: '0 0 24px' }}>
                This share link doesn't exist. Check the URL and try again.
              </p>
              <button
                onClick={() => { setSharedCode(null); setSharedError(null); window.history.replaceState({}, '', '/login/email/register'); setScreen('email-register', true) }}
                style={{ backgroundColor: BRAND, color: WHITE, border: 'none', borderRadius: '10px', padding: '14px 28px', fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer' }}
              >
                Sign up for Thriftly
              </button>
            </div>
          )}

          {sharedData && !sharedError && (
            <>
              {sharedData.items.length === 0 ? (
                <div style={{ textAlign: 'center', padding: '40px 0' }}>
                  <p style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY }}>This list is empty</p>
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                  {sharedData.items.map((item, i) => (
                    <div key={i} style={{
                      display: 'flex', alignItems: 'center', gap: '12px',
                      padding: '14px 16px', backgroundColor: WHITE, borderRadius: '10px',
                      opacity: item.checked ? 0.5 : 1,
                    }}>
                      <div style={{
                        width: '22px', height: '22px', borderRadius: '6px', flexShrink: 0,
                        border: item.checked ? 'none' : `2px solid ${BORDER}`,
                        backgroundColor: item.checked ? BRAND : 'transparent',
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                      }}>
                        {item.checked && <span style={{ color: WHITE, fontSize: '14px', fontWeight: 700 }}>✓</span>}
                      </div>
                      <span style={{
                        fontFamily: FONT, fontSize: '15px', color: TEXT, flex: 1,
                        textDecoration: item.checked ? 'line-through' : 'none',
                      }}>
                        {item.quantity > 1 ? `${item.quantity}x ` : ''}{item.name}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </>
          )}
        </div>

        {/* Sign-up CTA */}
        <div style={{ padding: '20px', backgroundColor: WHITE, borderTop: `1px solid ${BORDER}`, textAlign: 'center' }}>
          <p style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, margin: '0 0 12px' }}>
            Sign up for Thriftly to make your own list and earn rebates
          </p>
          <button
            onClick={() => { setSharedCode(null); window.history.replaceState({}, '', '/login/email/register'); setScreen('email-register', true) }}
            style={{ backgroundColor: BRAND, color: WHITE, border: 'none', borderRadius: '10px', padding: '14px 28px', fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer', width: '100%', maxWidth: '320px' }}
          >
            Get Started
          </button>
        </div>
      </div>
    )
  }

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

          {error && <p role="alert" style={{ ...styles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}

          <button
            onClick={() => handleSendCode()}
            disabled={!isValid || !smsOptIn || loading}
            style={{
              width: '100%', padding: '17px', fontSize: '16px',
              fontFamily: FONT, fontWeight: 700, color: WHITE,
              backgroundColor: BRAND, border: 'none', borderRadius: '14px',
              cursor: isValid && smsOptIn && !loading ? 'pointer' : 'default',
              transition: 'opacity 0.2s, transform 0.1s',
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

          {error && <p role="alert" style={{ ...styles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}
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

          {error && <p role="alert" style={{ ...styles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}

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

              {error && <p role="alert" style={{ ...styles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}

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

          {error && <p role="alert" style={{ ...styles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}

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
                aria-label={`Verification code digit ${i + 1} of 6`}
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

          {error && <p role="alert" style={{ ...styles.error, width: '100%', textAlign: 'center' as const }}>{error}</p>}

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

  const getActiveTab = (s: Screen): TabScreen => {
    switch (s) {
      case 'home': case 'earnings': case 'receipts': return 'home'
      case 'penny': return 'penny'
      case 'settings-share': return 'settings-share'
      default: return 'profile'
    }
  }

  const renderBottomTabs = () => (
    <BottomTabBar
      active={getActiveTab(screen)}
      onNavigate={setScreen}
    />
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
    <div role={toast.type === 'error' ? 'alert' : 'status'} style={{ ...pennyBarStyle, backgroundColor: toast.type === 'error' ? ERROR_RED_LIGHT : '#F5F3EF', color: toast.type === 'error' ? ERROR_RED_DARK : TEXT }}>
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
        aria-label="Dismiss notification"
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
    'Swaps': <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/></svg>,
  }

  const SUB_SCREENS: Screen[] = ['earnings', 'receipts', 'help', 'admin']

  const renderPageHeader = (pageName: string, active: Screen) => {
    const isSubScreen = SUB_SCREENS.includes(active)
    return (
      <div style={{ flexShrink: 0, backgroundColor: WHITE, borderBottom: `1px solid ${BORDER}` }}>
        <div style={{ display: 'flex', alignItems: 'center', padding: '16px 20px', position: 'relative' }}>
          {isSubScreen ? (
            <button
              onClick={() => setScreen(active === 'earnings' || active === 'receipts' ? 'home' : active === 'admin' ? 'profile' : 'profile')}
              aria-label="Back"
              style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '0 4px', display: 'flex', alignItems: 'center', minWidth: '44px', minHeight: '44px' }}
            >
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={TEXT} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="15 18 9 12 15 6"/>
              </svg>
            </button>
          ) : earningsTotalCents > 0 ? (
            <span
              style={{ cursor: 'pointer', display: 'flex', flexDirection: 'column', alignItems: 'flex-start', lineHeight: 1 }}
              onClick={() => setScreen('earnings')}
            >
              <span style={{ fontFamily: FONT, fontSize: '17px', fontWeight: 700, color: BRAND }}>
                ${(earningsTotalCents / 100).toFixed(2)}
              </span>
              <span style={{ fontFamily: FONT, fontSize: '10px', color: TEXT_SECONDARY, marginTop: '1px' }}>swaps</span>
            </span>
          ) : (
            <span
              style={{ ...listStyles.headerWordmark, cursor: 'pointer', fontSize: '17px', fontWeight: 700 }}
              onClick={() => setScreen('home')}
            >thriftly</span>
          )}
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
          {/* Spacer to keep title centered */}
          <div style={{ marginLeft: 'auto', width: '44px' }} />
        </div>
      </div>
    )
  }

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

            {/* ── Contact Penny (top) ── */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '14px' }}>
              <PennyAvatar size={52} />
              <div>
                <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT }}>Penny</div>
                <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, marginTop: '2px' }}>Your household's AI shopping assistant</div>
              </div>
              <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '16px' }}>
                {pennyPhoneFormatted && (
                  <a href={`sms:${settingsPennyPhone}`} style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: '44px', height: '44px', borderRadius: '50%', backgroundColor: BG, textDecoration: 'none' }} aria-label="Text Penny">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={BRAND} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
                  </a>
                )}
                <a href="mailto:penny@list.thrift.ly" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: '44px', height: '44px', borderRadius: '50%', backgroundColor: BG, textDecoration: 'none' }} aria-label="Email Penny">
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={BRAND} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/><polyline points="22,6 12,13 2,6"/></svg>
                </a>
                <a href="/api/penny.vcf" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: '44px', height: '44px', borderRadius: '50%', backgroundColor: BG, textDecoration: 'none' }} aria-label="Add Penny to contacts">
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={BRAND} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/></svg>
                </a>
              </div>
            </div>

            {/* ── Divider ── */}
            <div style={{ height: '1px', backgroundColor: BORDER, margin: '4px 0' }} />

            {/* ── Settings ── */}

            {/* Deals toggle — only show if user has messaging (phone) */}
            {settingsPennyPhone && (
              <div onClick={toggleDeals} style={listStyles.settingsCard}>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={TEXT} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/>
                </svg>
                <div style={{ flex: 1 }}>
                  <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT }}>I want to SMS with Penny</div>
                  <div style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY, marginTop: '1px' }}>Penny texts you about deals and savings she finds</div>
                </div>
                <ToggleSwitch value={pennyDeals} onChange={toggleDeals} />
              </div>
            )}

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

            {/* Slider — only visible when autotune is OFF, at bottom so it doesn't shift content above */}
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
                aria-label="Convenience versus savings preference"
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

          </div>
        </div>
        {renderPennyToast()}
        {renderBottomTabs()}
      </div>
    )
  }

  // =========================================================================
  // PROFILE PAGE
  // =========================================================================

  if (screen === 'profile') {
    const initials = [settingsFirstName, settingsLastName].map(n => (n || '')[0]?.toUpperCase() || '').filter(Boolean).join('')

    const avatarUrl = currentUserId ? `/api/avatar/${currentUserId}?v=${avatarVersion}` : null

    const handleAvatarPick = () => {
      const input = document.createElement('input')
      input.type = 'file'
      input.accept = 'image/*'
      input.onchange = async () => {
        const file = input.files?.[0]
        if (!file) return
        setAvatarUploading(true)
        try {
          // Load image and resize to 256x256 JPEG
          const bitmap = await createImageBitmap(file)
          const canvas = document.createElement('canvas')
          canvas.width = 256
          canvas.height = 256
          const ctx = canvas.getContext('2d')!
          // Scale to cover, center crop
          const scale = Math.max(256 / bitmap.width, 256 / bitmap.height)
          const sw = 256 / scale, sh = 256 / scale
          const sx = (bitmap.width - sw) / 2, sy = (bitmap.height - sh) / 2
          ctx.drawImage(bitmap, sx, sy, sw, sh, 0, 0, 256, 256)
          const blob = await new Promise<Blob>((resolve) => canvas.toBlob(b => resolve(b!), 'image/jpeg', 0.85))
          const buf = await blob.arrayBuffer()
          const base64 = btoa(String.fromCharCode(...new Uint8Array(buf)))
          const res = await fetch('/api/avatar', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ photo: base64 }) })
          if (res.status === 401) { handleSessionExpired(); return }
          if (res.ok) {
            setAvatarVersion(v => v + 1)
            showToast('Photo updated!')
          } else {
            const data = await res.json().catch(() => ({}))
            showToast(data?.error || 'Upload failed', 'error')
          }
        } catch {
          showToast('Could not process image', 'error')
        } finally {
          setAvatarUploading(false)
        }
      }
      input.click()
    }

    return (
      <div style={listStyles.page}>
        <ConfirmModal open={!!confirmModal} title={confirmModal?.title || ''} message={confirmModal?.message} confirmLabel={confirmModal?.confirmLabel} destructive={confirmModal?.destructive} onConfirm={runConfirmAction} onCancel={closeConfirmModal} />
        {renderPageHeader('Profile', 'profile')}
        <div style={listStyles.listArea}>
          <div style={{ padding: '24px 20px', display: 'flex', flexDirection: 'column', gap: '24px' }}>

            {/* Avatar + display info */}
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '8px' }}>
              <div
                onClick={handleAvatarPick}
                role="button"
                tabIndex={0}
                onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleAvatarPick() } }}
                style={{
                  width: 80, height: 80, borderRadius: '50%',
                  backgroundColor: BRAND, display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontFamily: FONT, fontSize: '28px', fontWeight: 700, color: WHITE,
                  cursor: 'pointer', position: 'relative', overflow: 'hidden',
                }}
                aria-label="Change profile photo"
                title="Tap to change photo"
              >
                {avatarUrl && !avatarError ? (
                  <img
                    key={avatarVersion}
                    src={avatarUrl}
                    alt="Profile"
                    onError={() => setAvatarError(true)}
                    style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                  />
                ) : (
                  initials || '?'
                )}
                {avatarUploading && (
                  <div style={{
                    position: 'absolute', inset: 0, backgroundColor: 'rgba(0,0,0,0.4)',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                  }}>
                    <div style={{ width: 24, height: 24, border: '3px solid rgba(255,255,255,0.3)', borderTopColor: WHITE, borderRadius: '50%', animation: 'spin 0.8s linear infinite' }} />
                  </div>
                )}
                {!avatarUploading && (
                  <div style={{
                    position: 'absolute', bottom: 0, left: 0, right: 0,
                    backgroundColor: 'rgba(0,0,0,0.5)', padding: '2px 0',
                    display: 'flex', justifyContent: 'center',
                  }}>
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
                      <circle cx="12" cy="13" r="4"/>
                    </svg>
                  </div>
                )}
              </div>
            </div>

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
            <div>
              <FloatingLabelField label="Cell phone">
                <input
                  type="tel"
                  value={settingsUserPhone ? formatPhone(settingsUserPhone.replace('+1', '')) : ''}
                  onChange={e => {
                    const digits = e.target.value.replace(/\D/g, '').slice(0, 10)
                    setSettingsUserPhone(digits ? `+1${digits}` : '')
                  }}
                  onBlur={handleSettingsAutoSave}
                  placeholder="(555) 123-4567"
                  style={listStyles.floatingInput}
                />
              </FloatingLabelField>
              <p style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '4px', paddingLeft: '4px' }}>
                Text Penny your grocery list anytime.
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

            {/* Need Help? */}
            <button onClick={() => setScreen('help')} style={{
              background: 'none', border: 'none', cursor: 'pointer', padding: 0,
              fontFamily: FONT, fontSize: '15px', fontWeight: 500, color: BRAND,
              textAlign: 'left' as const,
              display: 'flex', alignItems: 'center', gap: '8px',
            }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={BRAND} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="10"/>
                <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/>
                <line x1="12" y1="17" x2="12.01" y2="17"/>
              </svg>
              Need Help?
            </button>

            {/* Admin dashboard — admin only */}
            {isAdmin && (
              <button onClick={() => setScreen('admin')} style={{
                background: 'none', border: 'none', cursor: 'pointer', padding: 0,
                fontFamily: FONT, fontSize: '15px', fontWeight: 500, color: BRAND,
                textAlign: 'left' as const,
                display: 'flex', alignItems: 'center', gap: '8px',
              }}>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={BRAND} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="18" y="3" width="4" height="18"/><rect x="10" y="8" width="4" height="13"/><rect x="2" y="13" width="4" height="8"/>
                </svg>
                Dashboard
              </button>
            )}

            {/* Sign out */}
            <button onClick={handleLogout} style={{
              background: 'none', border: 'none', cursor: 'pointer', padding: 0,
              fontFamily: FONT, fontSize: '15px', fontWeight: 500, color: ERROR_RED,
              textAlign: 'left' as const,
            }}>Sign Out</button>

          </div>
        </div>

        {renderBottomTabs()}
      </div>
    )
  }

  // =========================================================================
  // HELP / FAQ / BUG REPORT PAGE
  // =========================================================================

  if (screen === 'help') {
    const faqs = [
      { q: 'How do I share my list with someone?', a: 'Go to Share & Earn from the menu. Tap "Invite to Household" to generate an invite link. Anyone who opens it can join your shared list.' },
      { q: 'How do Penny\'s swaps work?', a: 'When you add an item, Penny may suggest a swap from a brand partner. If you accept, the brand covers part of the cost. You can always decline — no pressure!' },
      { q: 'How do I add items via text message?', a: 'Text your grocery items to Penny\'s number (you can find it in the Penny settings screen). She\'ll parse your message and add them to your list automatically.' },
      { q: 'How do I save a list for later?', a: 'Just ask Penny! Say something like "save this as my weekly groceries" in the Penny chat. You can load saved lists later by asking Penny "what lists do I have?"' },
      { q: 'How do I delete my account?', a: 'Email us at support@thrift.ly and we\'ll take care of it within 48 hours. We\'ll delete all your data permanently.' },
    ]

    const handleBugSubmit = async () => {
      if (!bugDescription.trim() || bugSubmitting) return
      setBugSubmitting(true)
      try {
        const res = await fetch('/api/bug-report', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ description: bugDescription.trim() }),
        })
        if (res.status === 401) { handleSessionExpired(); return }
        if (res.ok) {
          setBugSubmitted(true)
          setBugDescription('')
          showToast('Bug report submitted — thank you!')
        } else {
          const data = await res.json().catch(() => ({}))
          showToast(data?.error || 'Failed to submit report', 'error')
        }
      } catch {
        showToast('Something went wrong', 'error')
      } finally {
        setBugSubmitting(false)
      }
    }

    return (
      <div style={listStyles.page}>
        {renderPageHeader('Help', 'help')}
        <div style={listStyles.listArea}>
          <div style={{ padding: '24px 20px', display: 'flex', flexDirection: 'column', gap: '24px' }}>

            {/* FAQ Section */}
            <div>
              <h2 style={{ fontFamily: FONT, fontSize: '18px', fontWeight: 700, color: TEXT, marginBottom: '16px' }}>
                Frequently Asked Questions
              </h2>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0px', borderRadius: '12px', overflow: 'hidden', border: `1px solid ${BORDER}` }}>
                {faqs.map((faq, i) => (
                  <div key={i}>
                    {i > 0 && <div style={{ height: '1px', backgroundColor: BORDER }} />}
                    <button
                      onClick={() => setOpenFaq(openFaq === i ? null : i)}
                      style={{
                        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                        width: '100%', padding: '14px 16px',
                        backgroundColor: openFaq === i ? `${BRAND}08` : WHITE,
                        border: 'none', cursor: 'pointer', textAlign: 'left',
                        fontFamily: FONT, fontSize: '15px', fontWeight: 500, color: TEXT,
                        gap: '12px',
                      }}
                    >
                      <span style={{ flex: 1 }}>{faq.q}</span>
                      <span style={{
                        fontSize: '18px', color: TEXT_SECONDARY, flexShrink: 0,
                        transform: openFaq === i ? 'rotate(180deg)' : 'none',
                        transition: 'transform 0.2s ease',
                      }}>
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={TEXT_SECONDARY} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                          <polyline points="6 9 12 15 18 9"/>
                        </svg>
                      </span>
                    </button>
                    <div style={{
                      maxHeight: openFaq === i ? '200px' : '0px',
                      opacity: openFaq === i ? 1 : 0,
                      overflow: 'hidden',
                      transition: 'max-height 0.3s ease, opacity 0.25s ease',
                    }}>
                      <div style={{
                        padding: '0 16px 14px 16px',
                        fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, lineHeight: 1.5,
                        backgroundColor: openFaq === i ? `${BRAND}08` : WHITE,
                      }}>
                        {faq.a}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* Divider */}
            <div style={listStyles.divider} />

            {/* Bug Report Section */}
            <div>
              <h2 style={{ fontFamily: FONT, fontSize: '18px', fontWeight: 700, color: TEXT, marginBottom: '4px' }}>
                Report a Bug
              </h2>
              <p style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, marginBottom: '16px' }}>
                Found something that doesn't look right? Let us know and we'll fix it.
              </p>
              {bugSubmitted ? (
                <div style={{
                  padding: '20px', borderRadius: '12px',
                  backgroundColor: `${BRAND}10`, textAlign: 'center',
                }}>
                  <p style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 500, color: BRAND }}>
                    Thanks for your report! We'll look into it.
                  </p>
                  <button
                    onClick={() => setBugSubmitted(false)}
                    style={{
                      marginTop: '12px', background: 'none', border: 'none', cursor: 'pointer',
                      fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, textDecoration: 'underline',
                    }}
                  >
                    Submit another report
                  </button>
                </div>
              ) : (
                <>
                  <textarea
                    value={bugDescription}
                    onChange={e => setBugDescription(e.target.value)}
                    placeholder="Describe the issue you encountered..."
                    maxLength={2000}
                    rows={5}
                    style={{
                      width: '100%', padding: '13px 16px',
                      fontSize: '16px', fontFamily: FONT, color: TEXT,
                      backgroundColor: WHITE, border: `1.5px solid ${BORDER}`,
                      borderRadius: '12px', outline: 'none',
                      boxSizing: 'border-box', resize: 'vertical',
                      lineHeight: 1.5,
                    }}
                    onFocus={e => { e.target.style.borderColor = BRAND }}
                    onBlur={e => { e.target.style.borderColor = BORDER }}
                  />
                  <button
                    onClick={handleBugSubmit}
                    disabled={!bugDescription.trim() || bugSubmitting}
                    style={{
                      marginTop: '12px', width: '100%', padding: '14px',
                      backgroundColor: (!bugDescription.trim() || bugSubmitting) ? `${BRAND}60` : BRAND,
                      color: WHITE, border: 'none', borderRadius: '12px',
                      fontFamily: FONT, fontSize: '16px', fontWeight: 600,
                      cursor: (!bugDescription.trim() || bugSubmitting) ? 'not-allowed' : 'pointer',
                      transition: 'background-color 0.2s',
                    }}
                  >
                    {bugSubmitting ? 'Submitting...' : 'Submit Report'}
                  </button>
                </>
              )}
            </div>

          </div>
        </div>
        {renderBottomTabs()}
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

            {/* ── Share a List (read-only, 7-day link) ── */}
            <div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={TEXT_SECONDARY} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8"/><polyline points="16 6 12 2 8 6"/><line x1="12" y1="2" x2="12" y2="15"/></svg>
                <span style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 700, color: TEXT, textTransform: 'uppercase' as const, letterSpacing: '0.5px' }}>Share a List</span>
              </div>
              <p style={{ fontFamily: FONT, fontSize: '15px', color: TEXT, margin: '0 0 16px 0', lineHeight: 1.6 }}>
                Send a read-only snapshot of your list to anyone — no account needed. Link expires in 7 days.
              </p>
              <button
                onClick={async () => {
                  try {
                    const res = await fetch('/api/list/share', { method: 'POST' })
                    if (res.status === 401) { handleSessionExpired(); return }
                    if (res.ok) {
                      const data = await res.json()
                      const url = `${window.location.origin}${data.url}`
                      if (navigator.share) {
                        navigator.share({ title: 'My grocery list', url }).catch(() => {})
                      } else {
                        navigator.clipboard.writeText(url).then(() => showToast('Share link copied!')).catch(() => showToast('Could not copy', 'error'))
                      }
                    } else {
                      showToast('Could not create share link', 'error')
                    }
                  } catch { showToast('Connection error', 'error') }
                }}
                style={listStyles.sheetButton}
              >
                Share my list for 7 days
              </button>
            </div>

            {/* ── Refer a Friend ── */}
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
        {renderBottomTabs()}
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

              {/* Swap Stats */}
              {(adminData.swap_offered != null && adminData.swap_offered > 0) && (
                <>
                  <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginTop: '28px', marginBottom: '12px' }}>
                    Swap Deals
                  </div>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '20px' }}>
                    {[
                      { label: 'Offered', value: adminData.swap_offered },
                      { label: 'Accepted', value: adminData.swap_accepted },
                      { label: 'Declined', value: adminData.swap_declined },
                      { label: 'Accept Rate', value: `${(adminData.swap_accept_rate || 0).toFixed(1)}%` },
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
                </>
              )}

              {/* Payout Checklist */}
              {adminPayouts.length > 0 && (
                <>
                  <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginTop: '28px', marginBottom: '12px' }}>
                    Pending Payouts
                  </div>
                  {adminPayouts.map(p => (
                    <div key={p.user_id} style={{
                      display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                      padding: '12px 0', borderBottom: `1px solid ${BORDER}`,
                    }}>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT, fontWeight: 500 }}>
                          {p.name || 'No name'} — ${(p.total_cents / 100).toFixed(2)}
                        </div>
                        <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>
                          {p.phone || p.email || 'No contact'} · {p.swap_count} swap{p.swap_count !== 1 ? 's' : ''} · since {new Date(p.oldest_swap).toLocaleDateString()}
                        </div>
                      </div>
                      <button
                        onClick={async () => {
                          const res = await fetch('/api/admin/payouts', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ user_id: p.user_id }) })
                          if (res.ok) {
                            setAdminPayouts(prev => prev.filter(x => x.user_id !== p.user_id))
                            showToast(`Marked $${(p.total_cents / 100).toFixed(2)} as paid for ${p.name || 'user'}`)
                          } else { showToast('Failed to mark as paid', 'error') }
                        }}
                        style={{ fontSize: '12px', padding: '6px 12px', borderRadius: '6px', border: `1px solid ${BRAND}`, backgroundColor: WHITE, cursor: 'pointer', fontFamily: FONT, color: BRAND, fontWeight: 600, flexShrink: 0 }}
                      >Mark Paid</button>
                    </div>
                  ))}
                </>
              )}

              {/* Deal Management */}
              <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginTop: '28px', marginBottom: '12px' }}>
                Manage Deals
                <button
                  onClick={() => { setShowDealForm(true); setEditingDealId(null); setDealForm({ competitor_product: '', offer_brand: '', offer_product: '', discount_cents: 0, daily_budget_cents: 0, status: 'active', ends_at: '', offer_price_cents: 0, competitor_price_cents: 0, offer_size: '', competitor_size: '', offer_unit_price: '', competitor_unit_price: '', why_bullets: '[]', allergen_notes: '', deal_type: 'sku_swap', quantity_offered: 1, trial_product_name: '', trial_size: '', campaign_tag: '' }) }}
                  style={{ marginLeft: '12px', backgroundColor: BRAND, color: WHITE, border: 'none', borderRadius: '6px', padding: '4px 12px', fontFamily: FONT, fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}
                >+ Add</button>
              </div>

              {showDealForm && (
                <div style={{ backgroundColor: WHITE, borderRadius: '12px', padding: '16px', border: `1px solid ${BORDER}`, marginBottom: '16px' }}>
                  {/* Deal Type selector */}
                  <div style={{ marginBottom: '8px' }}>
                    <label style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, display: 'block', marginBottom: '2px' }}>Deal Type</label>
                    <select
                      value={dealForm.deal_type}
                      onChange={e => setDealForm(prev => ({ ...prev, deal_type: e.target.value, quantity_offered: e.target.value === 'bogo' ? 2 : 1 }))}
                      style={{ width: '100%', padding: '8px', borderRadius: '6px', border: `1px solid ${BORDER}`, fontFamily: FONT, fontSize: '14px' }}
                    >
                      <option value="sku_swap">SKU Swap</option>
                      <option value="bogo">BOGO (Buy One Get One)</option>
                      <option value="trial">Trial (Free Sample)</option>
                      <option value="seasonal">Seasonal / Limited-Time</option>
                    </select>
                  </div>
                  {[
                    { label: 'Competitor Product', key: 'competitor_product' as const, type: 'text' },
                    { label: 'Offer Brand', key: 'offer_brand' as const, type: 'text' },
                    { label: 'Offer Product', key: 'offer_product' as const, type: 'text' },
                    { label: 'Discount (cents)', key: 'discount_cents' as const, type: 'number' },
                    { label: 'Total Budget (cents)', key: 'daily_budget_cents' as const, type: 'number' },
                    { label: 'Offer Price (cents)', key: 'offer_price_cents' as const, type: 'number' },
                    { label: 'Competitor Price (cents)', key: 'competitor_price_cents' as const, type: 'number' },
                    { label: 'Offer Size', key: 'offer_size' as const, type: 'text' },
                    { label: 'Competitor Size', key: 'competitor_size' as const, type: 'text' },
                    { label: 'Offer Unit Price', key: 'offer_unit_price' as const, type: 'text' },
                    { label: 'Competitor Unit Price', key: 'competitor_unit_price' as const, type: 'text' },
                    { label: 'Why Bullets (JSON array)', key: 'why_bullets' as const, type: 'text' },
                    { label: 'Allergen Notes', key: 'allergen_notes' as const, type: 'text' },
                  ].map(f => (
                    <div key={f.key} style={{ marginBottom: '8px' }}>
                      <label style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, display: 'block', marginBottom: '2px' }}>{f.label}</label>
                      <input
                        type={f.type}
                        value={dealForm[f.key]}
                        onChange={e => setDealForm(prev => ({ ...prev, [f.key]: f.type === 'number' ? parseInt(e.target.value) || 0 : e.target.value }))}
                        style={{ width: '100%', padding: '8px', borderRadius: '6px', border: `1px solid ${BORDER}`, fontFamily: FONT, fontSize: '14px' }}
                      />
                    </div>
                  ))}
                  {/* Conditional fields by deal type */}
                  {dealForm.deal_type === 'bogo' && (
                    <div style={{ marginBottom: '8px' }}>
                      <label style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, display: 'block', marginBottom: '2px' }}>Quantity Offered</label>
                      <input type="number" value={dealForm.quantity_offered} onChange={e => setDealForm(prev => ({ ...prev, quantity_offered: parseInt(e.target.value) || 2 }))}
                        style={{ width: '100%', padding: '8px', borderRadius: '6px', border: `1px solid ${BORDER}`, fontFamily: FONT, fontSize: '14px' }} />
                    </div>
                  )}
                  {dealForm.deal_type === 'trial' && (
                    <>
                      <div style={{ marginBottom: '8px' }}>
                        <label style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, display: 'block', marginBottom: '2px' }}>Trial Product Name</label>
                        <input type="text" value={dealForm.trial_product_name} onChange={e => setDealForm(prev => ({ ...prev, trial_product_name: e.target.value }))}
                          style={{ width: '100%', padding: '8px', borderRadius: '6px', border: `1px solid ${BORDER}`, fontFamily: FONT, fontSize: '14px' }} />
                      </div>
                      <div style={{ marginBottom: '8px' }}>
                        <label style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, display: 'block', marginBottom: '2px' }}>Trial Size</label>
                        <input type="text" value={dealForm.trial_size} onChange={e => setDealForm(prev => ({ ...prev, trial_size: e.target.value }))}
                          style={{ width: '100%', padding: '8px', borderRadius: '6px', border: `1px solid ${BORDER}`, fontFamily: FONT, fontSize: '14px' }} />
                      </div>
                    </>
                  )}
                  {dealForm.deal_type === 'seasonal' && (
                    <div style={{ marginBottom: '8px' }}>
                      <label style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, display: 'block', marginBottom: '2px' }}>Campaign Tag</label>
                      <input type="text" value={dealForm.campaign_tag} onChange={e => setDealForm(prev => ({ ...prev, campaign_tag: e.target.value }))}
                        placeholder="e.g. holiday_2026"
                        style={{ width: '100%', padding: '8px', borderRadius: '6px', border: `1px solid ${BORDER}`, fontFamily: FONT, fontSize: '14px' }} />
                    </div>
                  )}
                  <div style={{ display: 'flex', gap: '8px', marginTop: '12px' }}>
                    <button
                      onClick={async () => {
                        const method = editingDealId ? 'PUT' : 'POST'
                        const body = editingDealId ? { ...dealForm, id: editingDealId } : dealForm
                        const res = await fetch('/api/admin/deals', { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
                        if (res.ok) {
                          const dealsRes = await fetch('/api/admin/deals')
                          if (dealsRes.ok) setAdminDeals(await dealsRes.json())
                          setShowDealForm(false)
                          showToast(editingDealId ? 'Deal updated' : 'Deal created')
                        } else { showToast('Failed to save deal', 'error') }
                      }}
                      style={{ flex: 1, backgroundColor: BRAND, color: WHITE, border: 'none', borderRadius: '8px', padding: '10px', fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer' }}
                    >{editingDealId ? 'Update' : 'Create'}</button>
                    <button
                      onClick={() => setShowDealForm(false)}
                      style={{ padding: '10px 16px', borderRadius: '8px', border: `1px solid ${BORDER}`, backgroundColor: WHITE, fontFamily: FONT, fontSize: '15px', cursor: 'pointer', color: TEXT }}
                    >Cancel</button>
                  </div>
                </div>
              )}

              {adminDeals.map(deal => (
                <div key={deal.id} style={{
                  display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                  padding: '12px 0', borderBottom: `1px solid ${BORDER}`,
                }}>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT, fontWeight: 500, display: 'flex', alignItems: 'center', gap: '6px' }}>
                      {deal.competitor_product} → {deal.offer_brand} {deal.offer_product}
                      {deal.deal_type && deal.deal_type !== 'sku_swap' && (
                        <span style={{ fontSize: '10px', fontWeight: 700, color: WHITE, backgroundColor: deal.deal_type === 'bogo' ? '#E67E22' : deal.deal_type === 'trial' ? '#27AE60' : '#E74C3C', padding: '1px 5px', borderRadius: '3px', textTransform: 'uppercase' as const }}>{deal.deal_type}</span>
                      )}
                    </div>
                    <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>
                      ${(deal.discount_cents / 100).toFixed(2)} off · Budget: ${(deal.daily_budget_cents / 100).toFixed(2)} · Spent: ${(deal.budget_spent_cents / 100).toFixed(2)} · {deal.status}
                    </div>
                  </div>
                  <div style={{ display: 'flex', gap: '6px', flexShrink: 0 }}>
                    <button
                      onClick={async () => {
                        const newStatus = deal.status === 'active' ? 'paused' : 'active'
                        await fetch('/api/admin/deals', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...deal, status: newStatus }) })
                        const res = await fetch('/api/admin/deals')
                        if (res.ok) setAdminDeals(await res.json())
                      }}
                      style={{ fontSize: '12px', padding: '4px 8px', borderRadius: '6px', border: `1px solid ${BORDER}`, backgroundColor: WHITE, cursor: 'pointer', fontFamily: FONT, color: TEXT }}
                    >{deal.status === 'active' ? 'Pause' : 'Resume'}</button>
                    <button
                      onClick={() => {
                        setEditingDealId(deal.id)
                        setDealForm({ competitor_product: deal.competitor_product, offer_brand: deal.offer_brand, offer_product: deal.offer_product, discount_cents: deal.discount_cents, daily_budget_cents: deal.daily_budget_cents, status: deal.status, ends_at: deal.ends_at, offer_price_cents: deal.offer_price_cents, competitor_price_cents: deal.competitor_price_cents, offer_size: deal.offer_size, competitor_size: deal.competitor_size, offer_unit_price: deal.offer_unit_price, competitor_unit_price: deal.competitor_unit_price, why_bullets: deal.why_bullets, allergen_notes: deal.allergen_notes, deal_type: deal.deal_type || 'sku_swap', quantity_offered: deal.quantity_offered || 1, trial_product_name: deal.trial_product_name || '', trial_size: deal.trial_size || '', campaign_tag: deal.campaign_tag || '' })
                        setShowDealForm(true)
                      }}
                      style={{ fontSize: '12px', padding: '4px 8px', borderRadius: '6px', border: `1px solid ${BORDER}`, backgroundColor: WHITE, cursor: 'pointer', fontFamily: FONT, color: TEXT }}
                    >Edit</button>
                    <button
                      onClick={async () => {
                        await fetch('/api/admin/deals', { method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id: deal.id }) })
                        const res = await fetch('/api/admin/deals')
                        if (res.ok) setAdminDeals(await res.json())
                      }}
                      style={{ fontSize: '12px', padding: '4px 8px', borderRadius: '6px', border: `1px solid ${ERROR_RED}40`, backgroundColor: ERROR_RED_BG, cursor: 'pointer', fontFamily: FONT, color: ERROR_RED }}
                    >Del</button>
                  </div>
                </div>
              ))}

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
        {renderBottomTabs()}
      </div>
    )
  }

  // =========================================================================
  // EARNINGS SCREEN
  // =========================================================================

  if (screen === 'earnings') {
    return (
      <div style={listStyles.page}>
        {renderPageHeader('Swaps', 'earnings')}
        <div style={{ flex: 1, overflowY: 'auto' }}>
          <EarningsScreen
            onBack={() => setScreen('home')}
            fetchAPI={fetchAPI}
            onViewReceipts={() => setScreen('receipts')}
          />
        </div>
        {renderBottomTabs()}
      </div>
    )
  }

  // =========================================================================
  // RECEIPTS SCREEN
  // =========================================================================

  if (screen === 'receipts') {
    return (
      <div style={listStyles.page}>
        {renderPageHeader('Receipts', 'receipts')}
        <div style={{ flex: 1, overflowY: 'auto' }}>
          <ReceiptScreen
            fetchAPI={fetchAPI}
            onSnapReceipt={() => {
              setPendingReceiptMode(true)
              receiptInputRef.current?.click()
            }}
          />
        </div>
        {renderBottomTabs()}
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
          {earningsTotalCents > 0 ? (
            <span
              style={{ cursor: 'pointer', display: 'flex', flexDirection: 'column', alignItems: 'flex-start', lineHeight: 1 }}
              onClick={() => setScreen('earnings')}
            >
              <span style={{ fontFamily: FONT, fontSize: '17px', fontWeight: 700, color: BRAND }}>
                ${(earningsTotalCents / 100).toFixed(2)}
              </span>
              <span style={{ fontFamily: FONT, fontSize: '10px', color: TEXT_SECONDARY, marginTop: '1px' }}>swaps</span>
            </span>
          ) : (
            <span
              style={{ ...listStyles.headerWordmark, cursor: 'pointer', fontSize: '17px', fontWeight: 700 }}
              onClick={() => setScreen('home')}
            >thriftly</span>
          )}
          {editingListName ? (
            <input
              autoFocus
              aria-label="List name"
              value={listNameDraft}
              onChange={e => setListNameDraft(e.target.value)}
              onBlur={() => {
                const trimmed = listNameDraft.trim()
                setListName(trimmed)
                setEditingListName(false)
                fetchAPI('/api/settings', {
                  method: 'PATCH',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify({ list_name: trimmed }),
                })
              }}
              onKeyDown={e => { if (e.key === 'Enter') (e.target as HTMLInputElement).blur() }}
              maxLength={40}
              placeholder="Shopping List"
              style={{
                position: 'absolute', left: '50%', transform: 'translateX(-50%)',
                fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: TEXT,
                background: 'transparent', border: 'none', borderBottom: `1.5px solid ${BRAND}`,
                outline: 'none', textAlign: 'center', width: '160px', padding: '2px 0',
              }}
            />
          ) : (
            <span
              onClick={() => { setListNameDraft(listName); setEditingListName(true) }}
              style={{
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
                cursor: 'pointer',
              }}
            >{pageIcons['Shopping List']} {listName || 'Shopping List'}</span>
          )}
          {/* Spacer to keep title centered */}
          <div style={{ marginLeft: 'auto', width: '44px' }} />
        </div>
        {/* Progress bar */}
        {items.length > 0 && (() => {
          const totalQty = items.reduce((s, i) => s + (i.quantity || 1), 0)
          const checkedQty = items.filter(i => i.checked).reduce((s, i) => s + (i.quantity || 1), 0)
          const pct = Math.round((checkedQty / totalQty) * 100)
          return (
            <div role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100} aria-label={`Shopping list ${pct}% complete`} style={{ height: '3px', backgroundColor: BORDER }}>
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
              {isTripInProgress(trip) && <span style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: '#4a9470', flexShrink: 0 }} />}
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
                        const checkedCount = items.filter(i => i.checked).length
                        const totalCount = items.length
                        setDoneSummaryData({ checkedCount, totalCount, tripStartedAt: trip.created_at })
                        // Check if this is the user's first completed trip
                        fetchAPI('/api/trips').then(data => {
                          const d = data as { trips?: unknown[] } | null
                          setIsFirstTrip(!d?.trips || d.trips.length <= 1)
                        }).catch(() => setIsFirstTrip(false))
                        setShowDoneSummary(true)
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
                    aria-label="Cancel trip"
                    style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '4px', color: TEXT_SECONDARY, display: 'flex' }}
                  >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true"><path d="M1 1l12 12M13 1L1 13" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/></svg>
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Savings banner — tappable to see rebate history */}
      {swapCreditsCents > 0 && (
        <div
          onClick={() => {
            fetchAPI('/api/rebates').then(data => {
              const d = data as Record<string, unknown> | null
              if (d?.rebates) {
                setRebateHistory(d.rebates as typeof rebateHistory)
                setRebateSummary({ total_cents: (d.total_cents as number) || 0, pending_cents: (d.pending_cents as number) || 0, paid_cents: (d.paid_cents as number) || 0 })
              }
            }).catch(() => {})
            pushModalState()
            setShowSavingsSheet(true)
          }}
          style={{
            flexShrink: 0, padding: '10px 20px', backgroundColor: '#F0F7F4',
            borderBottom: `1px solid ${BORDER}`, cursor: 'pointer',
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          }}
        >
          <span style={{ fontFamily: FONT, fontSize: '14px', color: BRAND, fontWeight: 600 }}>
            ${(swapCreditsCents / 100).toFixed(2)} earned from swaps — brands pay, you benefit
          </span>
          <span style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>View details ›</span>
        </div>
      )}

      {/* Scrollable list — tap to dismiss keyboard */}
      <div ref={listAreaRef} style={{
        ...listStyles.listArea,
        ...(pullDistance > 0 ? { transform: `translateY(${pullDistance}px)`, transition: isPulling.current ? 'none' : 'transform 0.2s ease' } : {}),
      }} onClick={e => {
        if (e.target === e.currentTarget || !(e.target as HTMLElement).closest('button, a, input, textarea')) {
          chatInputRef.current?.blur()
        }
      }}>
        {/* Pull-to-refresh indicator */}
        {(pullDistance > 0 || pullRefreshing) && (
          <div style={{
            position: 'absolute', top: -40, left: 0, right: 0,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            height: '40px', fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY,
          }}>
            {pullRefreshing ? 'Refreshing...' : pullDistance >= 50 ? 'Release to refresh' : 'Pull to refresh'}
          </div>
        )}
        {chatLoading && (
          <div style={{
            position: 'absolute',
            inset: 0,
            backgroundColor: 'rgba(0,0,0,0.04)',
            zIndex: 10,
            pointerEvents: 'auto',
          }} />
        )}
        {listLoading && items.length === 0 ? (
          <div style={{ padding: '8px 0' }}>
            {[1, 2, 3, 4].map(i => (
              <div key={i} style={{
                display: 'flex', alignItems: 'center', padding: '12px 20px',
                borderBottom: `1px solid ${BORDER}`, backgroundColor: WHITE,
              }}>
                <div className="skeleton-shimmer" style={{
                  width: '22px', height: '22px', borderRadius: '50%',
                  backgroundColor: SKELETON, marginRight: '12px', flexShrink: 0,
                }} />
                <div className="skeleton-shimmer" style={{
                  width: '44px', height: '44px', borderRadius: '8px',
                  backgroundColor: SKELETON, marginRight: '10px', flexShrink: 0,
                }} />
                <div style={{ flex: 1, display: 'flex', flexDirection: 'column' as const, gap: '6px' }}>
                  <div className="skeleton-shimmer" style={{
                    height: '14px', borderRadius: '4px', backgroundColor: SKELETON,
                    width: `${50 + i * 10}%`,
                  }} />
                  <div className="skeleton-shimmer" style={{
                    height: '10px', borderRadius: '4px', backgroundColor: SKELETON,
                    width: `${30 + i * 5}%`,
                  }} />
                </div>
              </div>
            ))}
          </div>
        ) : items.length === 0 && !chatLoading ? (
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
                  aria-label={`${dept}, ${deptChecked} of ${deptTotal} items checked`}
                >
                  <span style={{ width: '52px', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0, paddingLeft: '9px', boxSizing: 'border-box' }} aria-hidden="true">
                    {deptDone ? <span style={{ color: BRAND, fontSize: '15px' }}>✓</span> : <DeptIcon dept={dept} />}
                  </span>
                  <span style={deptDone ? { textDecoration: 'line-through', opacity: 0.5 } : undefined}>{dept}</span>
                  <span style={listStyles.deptCount} aria-hidden="true">{deptChecked}/{deptTotal}</span>
                  <span style={listStyles.deptChevron(isOpen)} aria-hidden="true">›</span>
                </button>
                {isOpen && deptItems.map((item) => (
                  <ItemRow
                    key={item.id}
                    item={item}
                    isNew={newItemIds.has(item.id)}
                    currentUserId={currentUserId}
                    swap={swapsByItemId[item.id]}
                    onToggle={handleToggle}
                    onTap={openDetail}
                    onSwapAccept={handleSwapAccept}
                    onSwapDecline={handleSwapDecline}
                  />
                ))}
              </div>
            )
          })
        )}
      </div>


      {/* Trip planner suggestion banner — once per session, 5+ unchecked items */}
      {/* Trip suggestion moved to Penny chime-in — no banner */}

      {/* Confetti on list completion */}
      {showFireworks && <Confetti onComplete={() => setShowFireworks(false)} />}

      {/* Done Shopping Summary */}
      {showDoneSummary && doneSummaryData && (
        <div role="presentation" style={{
          position: 'fixed', inset: 0, zIndex: 9999,
          display: 'flex', flexDirection: 'column' as const, alignItems: 'center', justifyContent: 'center',
          backgroundColor: 'rgba(0,0,0,0.6)',
          padding: '24px',
        }}>
          <div role="dialog" aria-modal="true" aria-label="Trip summary" style={{
            backgroundColor: WHITE, borderRadius: '20px', padding: '32px 24px',
            maxWidth: '360px', width: '100%', textAlign: 'center' as const,
          }}>
            <div style={{ fontSize: '48px', marginBottom: '12px' }}>
              {isFirstTrip ? '🎉' : '✓'}
            </div>
            <div style={{ fontFamily: FONT, fontSize: '22px', fontWeight: 700, color: TEXT, marginBottom: '4px' }}>
              {isFirstTrip ? 'First trip complete!' : 'Nice work!'}
            </div>
            <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY, marginBottom: '20px' }}>
              {doneSummaryData.checkedCount} of {doneSummaryData.totalCount} item{doneSummaryData.totalCount !== 1 ? 's' : ''} checked off
            </div>

            {/* Save as list option */}
            {doneSummaryData.checkedCount > 0 && (
              <button
                onClick={async () => {
                  const listName = `Trip ${new Date().toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}`
                  const res = await fetchAPI('/api/saved-lists', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name: listName, items: items.filter(i => i.checked).map(i => ({ name: i.name, brand: i.brand, department: i.department, quantity: i.quantity, size: i.size, notes: i.notes })) }),
                  })
                  if (res) showToast(`Saved as "${listName}"`)
                  else showToast('Could not save list', 'error')
                }}
                style={{
                  width: '100%', padding: '12px', borderRadius: '10px',
                  border: `1px solid ${BORDER}`, backgroundColor: BG,
                  fontFamily: FONT, fontSize: '14px', color: TEXT, cursor: 'pointer',
                  marginBottom: '8px',
                }}
              >
                Save this list for next time
              </button>
            )}

            {/* Clear checked items */}
            <button
              onClick={async () => {
                await handleCancelTrip()
                if (doneSummaryData.checkedCount > 0) {
                  await handleClearList()
                }
                setShowDoneSummary(false)
                setDoneSummaryData(null)
                if (isFirstTrip) {
                  setShowFireworks(true)
                }
                showToast('Trip complete!')
              }}
              style={{
                width: '100%', padding: '14px', borderRadius: '10px',
                backgroundColor: BRAND, color: WHITE, border: 'none',
                fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer',
                marginBottom: '8px',
              }}
            >
              {doneSummaryData.checkedCount > 0 ? 'Clear checked & finish' : 'Finish trip'}
            </button>

            <button
              onClick={() => { setShowDoneSummary(false); setDoneSummaryData(null) }}
              style={{
                width: '100%', padding: '10px', background: 'none', border: 'none',
                fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, cursor: 'pointer',
              }}
            >
              Keep shopping
            </button>

            {/* Receipt scan button */}
            <button
              onClick={() => {
                setPendingReceiptMode(true)
                receiptInputRef.current?.click()
              }}
              style={{
                width: '100%', marginTop: '16px', padding: '12px', borderRadius: '10px',
                border: `1px solid ${BRAND}`, backgroundColor: `${BRAND}08`,
                fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: BRAND,
                cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px',
              }}
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={BRAND} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
                <circle cx="12" cy="13" r="4"/>
              </svg>
              Snap your receipt
            </button>
          </div>
        </div>
      )}

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
      {/* Separate hidden input for receipt capture — rear camera */}
      <input
        ref={receiptInputRef}
        type="file"
        accept="image/*"
        capture="environment"
        onChange={handlePhotoSelect}
        style={{ display: 'none' }}
        aria-label="Capture receipt"
      />

      {/* Penny text + disambiguation + chat input */}
      <PennyBar
        pennyText={homePennyText}
        pennyTyping={homePennyTyping}
        pennyStatus={!!pennyStatus}
        buttons={homePennyBtns}
        toastError={toast?.type === 'error'}
        keyboardOpen={keyboardOpen}
        disambig={pennyDisambig.map(d => ({ product_category: d.product_category, current_brand: d.current_brand, alternatives: d.alternatives }))}
        disambigIndex={pennyDisambigIndex}
        onBrandPick={handleBrandPick}
        onSkipDisambig={advanceDisambig}
        pendingImages={pendingImages}
        onRemoveImage={(idx) => setPendingImages(prev => prev.filter((_, i) => i !== idx))}
        chatText={chatText}
        onChatTextChange={setChatText}
        chatLoading={chatLoading}
        onSend={handleSendChat}
        onPaste={handlePaste}
        onAddPhoto={() => fileInputRef.current?.click()}
        chatInputRef={chatInputRef}
      />

      {renderBottomTabs()}

      {/* Trip planner bottom sheet */}
      {showTripSheet && (
        <TripPlannerSheet
          activeTrips={activeTrips}
          onClose={() => { popModalState(); setShowTripSheet(false) }}
          onSaveTrip={handleSetTrip}
          onCancelTrip={handleCancelTrip}
          formatTripTime={formatTripTime}
          isTripInProgress={isTripInProgress}
          getDayOptions={getDayOptions}
          todayStr={todayStr}
          tomorrowStr={tomorrowStr}
          satStr={satStr}
          listStyles={listStyles}
        />
      )}

      {/* NPS Survey Modal — Penny asks */}
      {showNPS && (
        <NPSSurveyModal onClose={closeNPS} onSubmit={handleNPSSubmit} />
      )}

      {/* Post-auth join prompt from shared list link */}
      {showJoinPrompt && (
        <div role="presentation" style={{ position: 'fixed', inset: 0, zIndex: 9999, backgroundColor: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '20px' }}>
          <div role="dialog" aria-modal="true" aria-label="Join shared list" style={{ backgroundColor: WHITE, borderRadius: '16px', padding: '32px 24px', maxWidth: '360px', width: '100%', textAlign: 'center' }}>
            <PennyAvatar size={64} />
            <h2 style={{ fontFamily: FONT, fontSize: '20px', fontWeight: 700, color: TEXT, margin: '16px 0 8px' }}>
              {joinPromptName ? `${joinPromptName} wants to shop together!` : 'Shop together!'}
            </h2>
            <p style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, margin: '0 0 24px', lineHeight: 1.5 }}>
              You'll share one shopping list — items either of you add will show up for both of you.
            </p>
            <button
              onClick={async () => {
                try {
                  const res = await fetch('/api/join', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ code: joinPromptCode }),
                  })
                  const data = await res.json().catch(() => null)
                  if (res.ok) {
                    showToast(data?.penny_message || `You joined ${joinPromptName ? joinPromptName + "'s" : 'the shared'} list!`)
                  } else {
                    showToast(data?.error || 'Could not join — the invite may have expired', 'error')
                  }
                } catch {
                  showToast('Could not join right now', 'error')
                }
                setShowJoinPrompt(false)
              }}
              style={{ backgroundColor: BRAND, color: WHITE, border: 'none', borderRadius: '10px', padding: '14px 0', fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer', width: '100%', marginBottom: '12px' }}
            >
              Shop together{joinPromptName ? ` with ${joinPromptName}` : ''}
            </button>
            <button
              onClick={() => setShowJoinPrompt(false)}
              style={{ background: 'none', border: 'none', fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, cursor: 'pointer', padding: '8px' }}
            >
              Start my own list
            </button>
          </div>
        </div>
      )}

      {/* Item detail sheet */}
      {selectedItem && (
        <ItemEditSheet
          item={selectedItem}
          editForm={editForm}
          setEditForm={setEditForm}
          displayBrand={displayBrand}
          setDisplayBrand={setDisplayBrand}
          editLoading={editLoading}
          currentUserId={currentUserId}
          itemHistory={itemHistory}
          onSave={handleEditSave}
          onDelete={handleEditDelete}
          onClose={() => closeEditSheet()}
          onUrgentToggle={handleUrgentToggle}
          onClaim={handleClaim}
          setSelectedItem={setSelectedItem}
          openConfirmModal={openConfirmModal}
          listStyles={listStyles}
          ToggleSwitch={ToggleSwitch}
          QuantityControl={QuantityControl}
        />
      )}

      {/* Review sheet — shows items from last parse */}
      {showReviewSheet && (lastAdded.length > 0 || lastSkipped.length > 0) && (
        <ReviewSheet
          lastAdded={lastAdded}
          lastSkipped={lastSkipped}
          items={items}
          onClose={() => { popModalState(); setShowReviewSheet(false); setLastAdded([]); setLastSkipped([]) }}
          onUpdateQuantity={async (item, delta) => {
            const data = await fetchAPI(`/api/list/${item.id}`, {
              method: 'PATCH', headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ quantity: Math.max(1, item.quantity + delta) }),
            })
            if (data) {
              const updated = data as ListItem
              setLastAdded(prev => prev.map(i => i.id === updated.id ? updated : i))
              setItems(prev => sortItems(prev.map(i => i.id === updated.id ? updated : i)))
            }
          }}
          onDeleteItem={async (item) => {
            const delRes = await fetchAction(`/api/list/${item.id}`, { method: 'DELETE' })
            if (!delRes.ok) return
            setItems(prev => prev.filter(i => i.id !== item.id))
            const remaining = lastAdded.filter(i => i.id !== item.id)
            setLastAdded(remaining)
            if (remaining.length === 0 && lastSkipped.length === 0) setShowReviewSheet(false)
          }}
          onUndoAll={async () => {
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
          onAddSkipped={async (skipped, existing) => {
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
          onOpenDetail={openDetailFromReview}
          listStyles={listStyles}
        />
      )}

      {/* Savings detail sheet */}
      {showSavingsSheet && (
        <ActionSheet isOpen={true} onClose={() => { popModalState(); setShowSavingsSheet(false) }} title="Your Swaps" maxHeight="70dvh">
          <div style={{ padding: '0 4px' }}>
            {/* Summary cards — hide Paid Out when $0 */}
            <div style={{ display: 'flex', gap: '8px', marginBottom: '16px' }}>
              {[
                { label: 'Total Earned', cents: rebateSummary.total_cents, color: BRAND, show: true },
                { label: 'Pending Payout', cents: rebateSummary.pending_cents, color: '#D97706', show: true },
                { label: 'Paid Out', cents: rebateSummary.paid_cents, color: '#059669', show: rebateSummary.paid_cents > 0 },
              ].filter(c => c.show).map(card => (
                <div key={card.label} style={{
                  flex: 1, padding: '12px 8px', borderRadius: '8px', backgroundColor: BG,
                  textAlign: 'center' as const,
                }}>
                  <div style={{ fontFamily: FONT, fontSize: '18px', fontWeight: 700, color: card.color }}>
                    ${(card.cents / 100).toFixed(2)}
                  </div>
                  <div style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY, marginTop: '2px' }}>
                    {card.label}
                  </div>
                </div>
              ))}
            </div>

            {/* Rebate history */}
            {rebateHistory.length === 0 ? (
              <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, textAlign: 'center' as const, padding: '20px 0' }}>
                Accept a swap to start earning!
              </div>
            ) : (
              rebateHistory.map(r => (
                <div key={r.id} style={{
                  display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                  padding: '10px 0', borderBottom: `1px solid ${BORDER}`,
                }}>
                  <div>
                    <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT, fontWeight: 500 }}>
                      {r.offer_brand} {r.offer_product}
                    </div>
                    <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '2px' }}>
                      {new Date(r.created_at + 'Z').toLocaleDateString()}
                    </div>
                  </div>
                  <div style={{ textAlign: 'right' as const }}>
                    <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: BRAND }}>
                      ${(r.amount_cents / 100).toFixed(2)}
                    </div>
                    <div style={{
                      fontFamily: FONT, fontSize: '11px', marginTop: '2px',
                      color: r.status === 'paid' ? '#059669' : r.status === 'issued' ? '#2563EB' : '#D97706',
                    }}>
                      {r.status === 'pending' ? 'Ready to cash out' : r.status === 'issued' ? 'Payout sent' : 'Paid'}
                    </div>
                  </div>
                </div>
              ))
            )}

            {/* Info note */}
            <div style={{
              fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, textAlign: 'center' as const,
              padding: '16px 0 8px', lineHeight: '1.4',
            }}>
              Brands pay to reach you — we share the revenue. No other grocery app pays you like this. Payouts sent monthly via Venmo.
            </div>
          </div>
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

  // Menu styles moved to NavigationMenu component

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

  // Item row styles moved to ItemRow component

  // Chat bar styles moved to PennyBar component

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

  /* Avatar upload spinner */
  @keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
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

  @keyframes skeleton-shimmer {
    0% { background-position: -200% 0; }
    100% { background-position: 200% 0; }
  }
  .skeleton-shimmer {
    background: linear-gradient(90deg, ${SKELETON} 25%, ${SKELETON_SHIMMER} 50%, ${SKELETON} 75%);
    background-size: 200% 100%;
    animation: skeleton-shimmer 1.8s ease-in-out infinite;
  }
  @media (prefers-reduced-motion: reduce) {
    *, *::before, *::after {
      animation-duration: 0.01ms !important;
      animation-iteration-count: 1 !important;
      transition-duration: 0.01ms !important;
    }
  }

  button, a, [role="button"] { -webkit-tap-highlight-color: transparent; }

  button:focus-visible { outline: 2px solid ${BRAND}; outline-offset: 2px; }

  .sr-only { position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;clip:rect(0,0,0,0);border:0; }

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
