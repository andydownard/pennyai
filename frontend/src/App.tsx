// =============================================================================
// Groflo — App.tsx (formerly Thriftly)
//
// Screens: phone → code → home, penny settings, profile, share, admin
// All styles inline. All API calls go to the Go backend.
// =============================================================================

import React, { useState, useRef, useEffect, useCallback, Suspense, lazy } from 'react'

// One-shot migration: copy any legacy `thriftly_*` keys to `groflo_*` so
// existing users keep their dismissed prompts, redirect intents, and join
// codes after the rebrand. Runs once at module load. Idempotent (a second run
// finds the new keys already present and removes the legacy ones).
;(function migrateLegacyGrofloStorage() {
  if (typeof window === 'undefined') return
  try {
    const stores: Storage[] = [window.localStorage, window.sessionStorage]
    for (const store of stores) {
      const legacyKeys: string[] = []
      for (let i = 0; i < store.length; i++) {
        const k = store.key(i)
        if (k && k.startsWith('groflo_')) legacyKeys.push(k)
      }
      for (const legacyKey of legacyKeys) {
        const newKey = 'groflo_' + legacyKey.slice('groflo_'.length)
        const val = store.getItem(legacyKey)
        if (val !== null && store.getItem(newKey) === null) {
          store.setItem(newKey, val)
        }
        store.removeItem(legacyKey)
      }
    }
  } catch { /* private browsing or storage disabled — ignore */ }
})()

import { QRCodeCanvas } from 'qrcode.react'
import { fetchAPI, fetchAction, setSessionExpiredHandler } from './api'
import { useToast } from './hooks/useToast'
import { useConfirmModal } from './hooks/useConfirmModal'
import { useKeyboardOpen } from './hooks/useKeyboardOpen'
import { useAuthState } from './hooks/useAuthState'
import { useSettingsState } from './hooks/useSettingsState'
import { useSwapState, type SkippedItem } from './hooks/useSwapState'
import { useListState } from './hooks/useListState'
import { useChatState } from './hooks/useChatState'

import { PennyAvatar } from './components/PennyAvatar'
import { ActionSheet } from './components/ActionSheet'
import { ConfirmModal } from './components/ConfirmModal'
import { Confetti } from './components/Confetti'
import { ErrorBoundary } from './components/ErrorBoundary'
import { type SwapOffer } from './components/SwapCard'
import { ItemRow, type ListItem } from './components/ItemRow'
import { PennyBar } from './components/PennyBar'
import { NPSSurveyModal } from './components/NPSSurveyModal'
import { track, identify } from './analytics'
import { TripPlannerSheet, type ShoppingTrip } from './components/TripPlannerSheet'
import { ItemEditSheet } from './components/ItemEditSheet'
import { ReviewSheet } from './components/ReviewSheet'
import { WrappedCard } from './components/WrappedCard'
import { MilestoneCard } from './components/MilestoneCard'
import { NavMenu, HamburgerButton, type NavScreen } from './components/NavMenu'
import { ScreenTransition, useScreenTransition } from './components/ScreenTransition'
import { PhoneScreen } from './components/PhoneScreen'
import { CodeScreen } from './components/CodeScreen'
import { FeedbackWidget } from './components/FeedbackWidget'
import { DoneSummaryModal } from './components/DoneSummaryModal'
import { OnboardingOverlay } from './components/OnboardingOverlay'
import { JoinPrompt } from './components/JoinPrompt'
import { PostSwapShareCard } from './components/PostSwapShareCard'
import { ToggleSwitch } from './components/ToggleSwitch'
import { WebcamCapture } from './components/WebcamCapture'

// Lazy-loaded screens (code-split for faster initial load)
const EarningsScreen = lazy(() => import('./components/EarningsScreen').then(m => ({ default: m.EarningsScreen })))
const ReceiptScreen = lazy(() => import('./components/ReceiptScreen').then(m => ({ default: m.ReceiptScreen })))
const ProfileScreen = lazy(() => import('./components/ProfileScreen').then(m => ({ default: m.ProfileScreen })))
const HelpScreen = lazy(() => import('./components/HelpScreen').then(m => ({ default: m.HelpScreen })))
const PennySettingsScreen = lazy(() => import('./components/PennySettingsScreen').then(m => ({ default: m.PennySettingsScreen })))
const EmailLoginScreen = lazy(() => import('./components/EmailLoginScreen').then(m => ({ default: m.EmailLoginScreen })))
const EmailRegisterScreen = lazy(() => import('./components/EmailRegisterScreen').then(m => ({ default: m.EmailRegisterScreen })))
const ForgotPasswordScreen = lazy(() => import('./components/ForgotPasswordScreen').then(m => ({ default: m.ForgotPasswordScreen })))
const ResetPasswordScreen = lazy(() => import('./components/ResetPasswordScreen').then(m => ({ default: m.ResetPasswordScreen })))
const ShareScreen = lazy(() => import('./components/ShareScreen').then(m => ({ default: m.ShareScreen })))
const AdminScreen = lazy(() => import('./components/AdminScreen').then(m => ({ default: m.AdminScreen })))
const SharedListScreen = lazy(() => import('./components/SharedListScreen').then(m => ({ default: m.SharedListScreen })))
const SavedListsScreen = lazy(() => import('./components/SavedListsScreen').then(m => ({ default: m.SavedListsScreen })))
const EmbedView = lazy(() => import('./components/EmbedView').then(m => ({ default: m.EmbedView })))
// ItemModification type — extracted to useListState hook
import {
  FONT, DEPARTMENTS, DEPT_ICONS,
  FONT_2XS, FONT_XS, FONT_SM, FONT_BASE, FONT_MD, FONT_LG, FONT_XL,
  THEME_CSS, initTheme, getTheme, setTheme,
  layout, buttons, text,
} from './styles'

// COMMODITY_KEYWORDS, isCommodityItem — extracted to useChatState hook

// =============================================================================
// TYPES
// =============================================================================

// The screens the app can show
type Screen = 'phone' | 'code' | 'home' | 'penny' | 'profile' | 'settings-share' | 'admin' | 'email-login' | 'email-register' | 'forgot-password' | 'reset-password' | 'shared-list' | 'help' | 'earnings' | 'receipts' | 'saved-lists'

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
  'saved-lists': '/saved-lists',
}

const SCREEN_TITLES: Record<Screen, string> = {
  phone: 'Sign In — Groflo',
  code: 'Verify — Groflo',
  home: 'Shopping List — Groflo',
  penny: 'Penny — Groflo',
  profile: 'Profile — Groflo',
  'settings-share': 'Share & Earn — Groflo',
  admin: 'Admin — Groflo',
  'email-login': 'Sign In — Groflo',
  'email-register': 'Register — Groflo',
  'forgot-password': 'Reset Password — Groflo',
  'reset-password': 'Reset Password — Groflo',
  'shared-list': 'Shared List — Groflo',
  help: 'Help — Groflo',
  earnings: 'Earnings — Groflo',
  receipts: 'Receipts — Groflo',
  'saved-lists': 'My Lists — Groflo',
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
  '/saved-lists': 'saved-lists',
}

const AUTH_SCREENS: Screen[] = ['phone', 'code', 'email-login', 'email-register', 'forgot-password', 'reset-password']

// =============================================================================
// CONSTANTS
// =============================================================================


// Relative timestamp for recently-added items (< 1 hour). Returns null for older items.
function DeptIcon({ dept }: { dept: string }) {
  const icon = DEPT_ICONS[dept]
  if (!icon) return null
  return <span style={{ fontSize: FONT_MD, lineHeight: 1, flexShrink: 0 }}>{icon}</span>
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
      aria-label="Groflo logo"
    >
      {/* Clipboard body */}
      <rect x="12" y="10" width="42" height="52" rx="6" stroke="var(--brand)" strokeWidth="2.5" fill="var(--surface)" />

      {/* Clip tab at top center */}
      <rect x="25" y="4" width="16" height="10" rx="4" fill="var(--brand)" />
      <rect x="29" y="7" width="8" height="4" rx="2" fill="var(--surface)" />

      {/* List item 1: checked */}
      <path d="M20 27 L23 30 L28 24" stroke="var(--brand)" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
      <line x1="33" y1="27" x2="46" y2="27" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" opacity="0.7" />

      {/* List item 2: checked */}
      <path d="M20 37 L23 40 L28 34" stroke="var(--brand)" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
      <line x1="33" y1="37" x2="42" y2="37" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" opacity="0.7" />

      {/* List item 3: being written (partial, lighter) */}
      <circle cx="24" cy="47" r="2" fill="var(--brand)" opacity="0.3" />
      <line x1="33" y1="47" x2="39" y2="47" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" opacity="0.3" />

      {/* AI sparkle — large */}
      <path
        d="M54 14 L56 19 L61 21 L56 23 L54 28 L52 23 L47 21 L52 19 Z"
        fill="var(--brand)"
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
        fill="var(--brand)"
        opacity="0.5"
      />
    </svg>
  )
}


// Toggle switch — used for urgent and claim toggles
// Avatar circle for household members and friends

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
    <div style={{ ...layout.row, border: '1.5px solid var(--border)', borderRadius: '12px', backgroundColor: 'var(--surface)', flexShrink: 0 }}>
      <button
        onClick={() => onChange(Math.max(1, value - 1))}
        aria-label={`Decrease quantity, currently ${value}`}
        style={{ ...buttons.reset, ...layout.rowCenter, width: '38px', height: '42px', fontSize: FONT_LG, color: 'var(--brand)' }}
      >−</button>
      <span aria-live="polite" aria-atomic="true" style={{ fontFamily: FONT, fontSize: FONT_MD, fontWeight: 600, color: 'var(--text)', minWidth: '24px', textAlign: 'center' }}>{value}</span>
      <button
        onClick={() => onChange(value + 1)}
        aria-label={`Increase quantity, currently ${value}`}
        style={{ ...buttons.reset, ...layout.rowCenter, width: '38px', height: '42px', fontSize: FONT_LG, color: 'var(--brand)' }}
      >+</button>
    </div>
  )
}

// LegalLinks extracted to ./components/LegalLinks.tsx

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
  const [navMenuOpen, setNavMenuOpen] = useState(false)
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

  // Update page title on screen change
  useEffect(() => { document.title = SCREEN_TITLES[screen] || 'Groflo' }, [screen])

  // Screen transition animation styles (must be called before any conditional returns)
  const transitionStyle = useScreenTransition(screen)

  // Auth state extracted to useAuthState hook (initialized below after setScreen & showToast are available)

  const [installPrompt, setInstallPrompt] = useState<any>(null)
  const [showInstallBanner, setShowInstallBanner] = useState(false)

  // One-time Groflo rebrand banner — shown to existing Thriftly users on
  // their next visit so the name change isn't a surprise. Per panel R7
  // (Nadia): "Penny is now part of Groflo. Same Penny, same list, same
  // household." Dismissed once and persisted; never re-shown.
  const [showRebrandBanner, setShowRebrandBanner] = useState(() => {
    if (typeof window === 'undefined') return false
    return localStorage.getItem('groflo_rebrand_banner_dismissed') !== '1'
  })

  // Shared list deep link state
  const [sharedCode, setSharedCode] = useState<string | null>(null)
  const [sharedData, setSharedData] = useState<{ items: { name: string; quantity: number; checked: boolean }[]; sharer_name: string; expires_at: string; invite_code?: string } | null>(null)
  const [sharedError, setSharedError] = useState<string | null>(null)
  const [sharedLoading, setSharedLoading] = useState(false)
  // Post-auth join prompt
  const [showJoinPrompt, setShowJoinPrompt] = useState(false)
  const [joinPromptName, setJoinPromptName] = useState('')
  const [joinPromptCode, setJoinPromptCode] = useState('')


  // Penny/chat state — extracted to useChatState hook

  // --- Pull-to-refresh state (stays in App — depends on screen) ---
  const [pullRefreshing, setPullRefreshing] = useState(false)
  const pullStartY = useRef(0)
  const [pullDistance, setPullDistance] = useState(0)
  const isPulling = useRef(false)

  // --- Toast state ---
  const { toast, showToast: _showToast, clearToast } = useToast()

  // --- Photo state ---
  const [pendingImages, setPendingImages] = useState<string[]>([])
  const fileInputRef = useRef<HTMLInputElement>(null)
  const cameraInputRef = useRef<HTMLInputElement>(null)
  const receiptInputRef = useRef<HTMLInputElement>(null)

  // --- Continuous camera mode ---
  const cameraModeRef = useRef(false)
  const cameraPhotoTakenRef = useRef(false) // set true when photo selected, prevents cancel detection
  const [cameraConfirmation, setCameraConfirmation] = useState<string | null>(null)
  const [showWebcam, setShowWebcam] = useState(false)
  const isMobile = /Mobi|Android/i.test(navigator.userAgent)

  // Menu state is now internal to NavigationMenu component

  // --- Penny chime-in state (must be before useListState) ---
  const [pennyChimeIn, setPennyChimeIn] = useState<string | null>(null)
  const chimeInTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

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
  const [tripSuggestionDismissed, setTripSuggestionDismissed] = useState(() => sessionStorage.getItem('groflo_trip_dismissed') === '1')

  // --- Saved lists state ---
  const [showSaveSheet, setShowSaveSheet] = useState(false)
  const [saveNameInput, setSaveNameInput] = useState('')
  const [saveStoreInput, setSaveStoreInput] = useState('')
  const [storeLists, setStoreLists] = useState<Array<{ id: number; name: string; store_name: string; item_count: number; is_master?: boolean }>>([])
  const [activeStore, setActiveStore] = useState<string | null>(null)
  const [restockNudges, setRestockNudges] = useState<Array<{ item_name: string; days_overdue: number }>>([])

  // --- Confirm modal state ---
  const { confirmModal, openConfirmModal, closeConfirmModal, runConfirmAction } = useConfirmModal()

  // --- Theme state ---
  const [darkMode, setDarkMode] = useState(() => getTheme() === 'dark')
  const toggleDarkMode = () => {
    const newTheme = darkMode ? 'light' : 'dark'
    setDarkMode(!darkMode)
    setTheme(newTheme)
  }

  // --- Auth/session state ---
  const [loginCount, setLoginCount] = useState(0)
  // Track whether this is a first-time user (never added items before)
  const [hasEverAddedItems, setHasEverAddedItems] = useState(() => {
    try { return localStorage.getItem('has_added_items') === '1' } catch { return false }
  })
  // Onboarding refs + syncOnboardingFlags — extracted to useChatState hook

  const [pendingReceiptMode, setPendingReceiptMode] = useState(false)

  // --- Admin state ---
  const [isAdmin, setIsAdmin] = useState(false)
  const [canSubmitFeedback, setCanSubmitFeedback] = useState(false)
  const [currentUserId, setCurrentUserId] = useState<number | null>(null)
  const [authMethods, setAuthMethods] = useState<string[]>([])

  // --- NPS state ---
  const [showNPS, setShowNPS] = useState(false)

  // --- Onboarding overlay state ---
  const [showOnboarding, setShowOnboarding] = useState(() => !localStorage.getItem('onboarding_complete'))

  // SMS opt-in, email/Google auth state — extracted to useAuthState hook

  // New item highlight state — extracted to useListState hook

  // pennyStatus — extracted to useChatState hook

  // Brand disambiguation state — extracted to useSwapState hook

  // --- Store department order (learned from shopping trips) ---
  const [storeDeptOrder, setStoreDeptOrder] = useState<string[] | null>(null)
  const [storeDeptOrderLearned, setStoreDeptOrderLearned] = useState(false)
  const [deptListOpacity, setDeptListOpacity] = useState(1)

  // --- Earnings state ---
  const [earningsTotalCents, setEarningsTotalCents] = useState(0)
  const keyboardOpen = useKeyboardOpen()

  // --- Settings state (extracted to useSettingsState hook) ---
  const {
    settingsFirstName, setSettingsFirstName,
    settingsLastName, setSettingsLastName,
    settingsEmail, setSettingsEmail,
    settingsAddress, setSettingsAddress,
    placesReady,
    settingsMembers,
    listName, setListName,
    editingListName, setEditingListName,
    listNameDraft, setListNameDraft,
    settingsPennyPhone,
    settingsUserPhone, setSettingsUserPhone,
    inviteLoading,
    showInviteSheet, setShowInviteSheet,
    invitePhone, setInvitePhone,
    inviteSent,
    referralCode,
    referralFriends,
    pennySlider, setPennySlider,
    pennyAutotune,
    pennyDeals,
    storeOrderEnabled,
    dailyDigestEnabled,
    loadSettings,
    handleSettingsAutoSave,
    handleOpenInviteSheet,
    handleSendSMSInvite,
    handleShareInviteLink,
    toggleAutotune,
    toggleDeals,
    toggleStoreOrder,
    toggleDailyDigest,
  } = useSettingsState({
    screen,
    currentUserId,
    showToast: (msg, type) => {
      if (type === 'error') haptic.error()
      _showToast(msg, type)
    },
    onEarningsTotalCents: (cents) => setEarningsTotalCents(cents),
    onOnboardingFlags: (flags) => updateOnboardingFlagsRef.current(flags),
  })

  // --- Stable refs for callbacks defined later (circular dependency resolution) ---
  const updateOnboardingFlagsRef = useRef<(flags: Record<string, string>) => void>(() => {})
  const typePennyHomeRef = useRef<(msg: string, btns?: { label: string; onClick: () => void }[]) => void>(() => {})
  const handleSessionExpiredRef = useRef<() => void>(() => {})
  const showChimeInRef = useRef<(msg: string, duration?: number) => void>(() => {})
  // Refs for swap state (useListState needs swap setters, useSwapState needs list setters)
  const setSwapsByItemIdRef = useRef<React.Dispatch<React.SetStateAction<Record<number, SwapOffer>>>>(() => {})
  const lastAddedRef = useRef<ListItem[]>([])
  const setLastAddedRef = useRef<React.Dispatch<React.SetStateAction<ListItem[]>>>(() => {})
  const lastSkippedRef = useRef<SkippedItem[]>([])
  const setLastSkippedRef = useRef<React.Dispatch<React.SetStateAction<SkippedItem[]>>>(() => {})
  const setShowReviewSheetRef = useRef<React.Dispatch<React.SetStateAction<boolean>>>(() => {})

  // --- List state (extracted) ---
  const {
    items, setItems,
    listLoading, setListLoading,
    expandedDepts, setExpandedDepts,
    selectedItem, setSelectedItem,
    editForm, setEditForm,
    displayBrand, setDisplayBrand,
    editLoading,
    itemHistory,
    newItemIds, setNewItemIds,
    newItemTimerRef,
    deptRefs,
    pendingToggleCount,
    sortItems,
    reloadList, openDetail, openDetailFromReview, closeEditSheet,
    handleEditSave, handleEditDelete,
    handleToggle, handleUrgentToggle, handleClaim,
    handleSwipeDelete, handleClearList, handleDeptClick,
  } = useListState({
    currentUserId,
    showToast: (msg, type) => { if (type === 'error') haptic.error(); _showToast(msg, type) },
    showChimeIn: (...args) => showChimeInRef.current(...args),
    typePennyHome: (...args) => typePennyHomeRef.current(...args),
    setSwapsByItemId: (...args) => setSwapsByItemIdRef.current(...args),
    lastAdded: lastAddedRef.current,
    setLastAdded: (...args) => setLastAddedRef.current(...args),
    lastSkipped: lastSkippedRef.current,
    setLastSkipped: (...args) => setLastSkippedRef.current(...args),
    setShowReviewSheet: (...args) => setShowReviewSheetRef.current(...args),
    setShowDoneToast,
    setShowFireworks,
    setShowDoneOverlay,
    setPennyChimeIn,
    pushModalState: () => {
      const path = SCREEN_PATHS[screen] || '/'
      window.history.pushState({ screen, modal: true }, '', path)
    },
    popModalState: () => {
      if (window.history.state?.modal) window.history.back()
    },
  })

  // --- Swap, review, and disambiguation state (extracted) ---
  const {
    swapsByItemId, setSwapsByItemId,
    lastAdded, setLastAdded,
    lastSkipped, setLastSkipped,
    showReviewSheet, setShowReviewSheet,
    hasReviewedRef,
    pennyDisambig, setPennyDisambig,
    pennyDisambigIndex, setPennyDisambigIndex,
    postSwapShare, setPostSwapShare,
    handleSwapAccept, handleSwapDecline,
    handleBrandPick, advanceDisambig,
  } = useSwapState({
    setItems,
    sortItems,
    typePennyHome: (...args) => typePennyHomeRef.current(...args),
    showToast: (msg, type) => _showToast(msg, type),
    handleSessionExpired: () => handleSessionExpiredRef.current(),
    setEarningsTotalCents,
    setShowFireworks,
  })

  // Wire refs after list + swap hooks are initialized
  setSwapsByItemIdRef.current = setSwapsByItemId
  lastAddedRef.current = lastAdded
  setLastAddedRef.current = setLastAdded
  lastSkippedRef.current = lastSkipped
  setLastSkippedRef.current = setLastSkipped
  setShowReviewSheetRef.current = setShowReviewSheet

  // Potential savings from active swaps on the list
  const potentialSavingsCents = Object.values(swapsByItemId).reduce((s, sw) => s + (sw.discount_cents || 0), 0)

  // Ref for handleLoadSavedList (defined later but only called from effects/handlers)
  const handleLoadSavedListRef = useRef<(id: number, mode: 'merge' | 'overwrite') => Promise<void>>(async () => {})

  // --- Chat, Penny, and onboarding state (extracted) ---
  const {
    chatText, setChatText,
    chatLoading, setChatLoading,
    chatInputRef,
    homePennyText,
    homePennyTyping,
    homePennyBtns,
    homePennyInterval,
    idleTimer,
    hasGreetedEmpty, hasGreetedWithItems,
    pennyStatus, setPennyStatus,
    onboardingNameAsked, onboardingShareMentioned,
    onboardingTextMentioned, onboardingSwapsMentioned,
    onboardingLinkPhoneMentioned, onboardingLinkEmailMentioned,
    nameInputMode,
    typePennyHome,
    handleSendChat,
    processParseResult,
  } = useChatState({
    // List state
    items, setItems, sortItems,
    setExpandedDepts,
    setNewItemIds, newItemTimerRef, deptRefs,
    reloadList, handleClearList,
    // Swap state
    setLastAdded, setLastSkipped, hasReviewedRef,
    pennyDisambig, setPennyDisambig,
    pennyDisambigIndex, setPennyDisambigIndex,
    setSwapsByItemId,
    advanceDisambig, handleBrandPick,
    setShowReviewSheet,
    // Settings state
    settingsFirstName, setSettingsFirstName,
    settingsLastName: settingsLastName ?? '',
    setSettingsLastName,
    settingsEmail, settingsUserPhone,
    handleOpenInviteSheet,
    // App state
    loginCount, authMethods,
    handleSessionExpired: () => handleSessionExpiredRef.current(),
    handleLoadSavedList: (...args) => handleLoadSavedListRef.current(...args),
    showToast: (msg, type) => { if (type === 'error') haptic.error(); _showToast(msg, type) },
    setScreen: (s) => setScreen(s as Screen),
    // UI state
    pendingImages, setPendingImages,
    setFloatingLabels,
    setShowDoneToast, setShowFireworks,
    setPendingReceiptMode,
    receiptInputRef,
    setEarningsTotalCents,
  })

  // Wire refs for chat state
  typePennyHomeRef.current = typePennyHome
  updateOnboardingFlagsRef.current = (flags) => {
    if (flags.ob_name) { onboardingNameAsked.current = true; localStorage.setItem('ob_name', '1') }
    if (flags.ob_share) { onboardingShareMentioned.current = true; localStorage.setItem('ob_share', '1') }
    if (flags.ob_text) { onboardingTextMentioned.current = true; localStorage.setItem('ob_text', '1') }
    if (flags.ob_swaps) { onboardingSwapsMentioned.current = true; localStorage.setItem('ob_swaps', '1') }
    if (flags.ob_link_phone) { onboardingLinkPhoneMentioned.current = true; localStorage.setItem('ob_link_phone', '1') }
    if (flags.ob_link_email) { onboardingLinkEmailMentioned.current = true; localStorage.setItem('ob_link_email', '1') }
  }

  const [showWrapped, setShowWrapped] = useState(false)
  const [milestoneData, setMilestoneData] = useState<{ cents: number; dollars: string; share_text: string; total_cents: number } | null>(null)

  // pennyChimeIn, chimeInTimer — moved before useListState
  const prevCheckedRef = useRef(-1)

  const CHIME_IN_MESSAGES = [
    'Looking good!', 'Almost there!', 'Nice work!', 'On a roll!', 'Keep going!',
    'You got this!', 'Crushing it!', 'Making great progress!', 'Speedy shopper!',
    'Halfway hero!', 'Cart\'s filling up nicely!',
  ]

  const FIRST_VISIT_GREETINGS = [
    "Hey! I'm Penny. What do you need from the store?",
    "Hi there! I'm Penny, your grocery sidekick. What are we shopping for?",
    "Hey! I'm Penny. Tell me what you need and I'll get your list started.",
    "Welcome! I'm Penny. Type, paste, or snap a photo of what you need.",
  ]

  const RETURNING_GREETINGS = (() => {
    const day = new Date().getDay()
    const base = [
      "What are we shopping for today?",
      "Ready when you are — just tell me what you need!",
      "What can I help you find at the store?",
      "Back for more? Let's build your list!",
    ]
    // Weekend-specific greetings
    if (day === 0 || day === 6) base.push("Weekend shopping? Let's make it quick!")
    // Weekday-specific
    if (day >= 1 && day <= 5) base.push("Hey! What's on the menu this week?")
    return base
  })()

  const EXISTING_LIST_GREETINGS = [
    "What can I add to your list?",
    "Need anything else from the store?",
    "Ready to add more — just type or snap a photo!",
  ]

  const IDLE_MESSAGES = [
    "Still here if you need anything!",
    "I'm not going anywhere — add items whenever you're ready.",
    "Need help planning a meal? Just ask!",
    "Tip: you can paste a recipe and I'll pull out the ingredients.",
    "You can also snap a photo of a recipe or your fridge!",
    "Try listing a few items — like 'chicken, rice, broccoli'.",
    "You can share this list with your household — tap Share!",
  ]

  const DEPT_DONE_MESSAGES = [
    'done with {dept}!', '{dept} — checked!', '{dept} complete!', 'Finished {dept}!',
    '{dept} — all set!', 'That\'s {dept} done!',
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
        haptic.success()
        setTimeout(() => {
          setExpandedDepts(prev => {
            const next = new Set(prev)
            newlyDone.forEach(d => next.delete(d))
            return next
          })
          // Auto-scroll to next unchecked department after collapse
          if (!allChecked) {
            setTimeout(() => {
              const nextDept = DEPARTMENTS.find(d => !nowDone.has(d) && items.some(i => i.department === d))
              if (nextDept && deptRefs.current[nextDept]) {
                deptRefs.current[nextDept]?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
              }
            }, 100)
          }
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
    // Auto-expand departments that were done but now have unchecked items (user unchecked something)
    if (checked < prevCheckedRef.current && prevCheckedRef.current > 0) {
      const reopened = [...prevDeptDoneRef.current].filter(d => !nowDone.has(d))
      if (reopened.length > 0) {
        setExpandedDepts(prev => {
          const next = new Set(prev)
          reopened.forEach(d => next.add(d))
          return next
        })
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

  // Clean up all timers on unmount
  useEffect(() => {
    return () => {
      if (homePennyInterval.current) clearInterval(homePennyInterval.current)
      if (chimeInTimer.current) clearTimeout(chimeInTimer.current)
      if (idleTimer.current) clearTimeout(idleTimer.current)
      if (newItemTimerRef.current) clearTimeout(newItemTimerRef.current)
    }
  }, [])

  // Check for savings milestones when earnings screen opens
  useEffect(() => {
    if (screen !== 'earnings') return
    fetchAPI('/api/milestones/check').then(d => {
      if (d && !Array.isArray(d) && d.milestone) {
        const m = d.milestone as { cents: number; dollars: string; share_text: string; total_cents: number }
        setMilestoneData(m)
        track('milestone_reached', { cents: m.cents, total_cents: m.total_cents })
      }
    }).catch(() => {})
  }, [screen]) // eslint-disable-line react-hooks/exhaustive-deps

  // phoneInputRef and codeInputRefs — extracted to useAuthState hook

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
      localStorage.setItem('groflo_join_code', joinMatch[1])
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
      sessionStorage.setItem('groflo_404', '1')
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
            localStorage.setItem('groflo_share_invite', data.invite_code)
            localStorage.setItem('groflo_share_name', data.sharer_name || '')
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
    const joinCode = localStorage.getItem('groflo_join_code')
    if (joinCode) {
      try {
        const res = await fetch('/api/join', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ code: joinCode }),
        })
        const data = await res.json().catch(() => null)
        if (res.ok) {
          localStorage.removeItem('groflo_join_code')
          showToast(data?.penny_message || 'You joined the shared list!')
        } else {
          localStorage.removeItem('groflo_join_code')
          if (data?.error) showToast(data.error, 'error')
        }
      } catch {
        localStorage.removeItem('groflo_join_code')
        showToast('Something went wrong. Try again.', 'error')
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

  // =========================================================================
  // AUTH STATE HOOK — phone, code, email, password, handlers, typewriter, etc.
  // =========================================================================

  const auth = useAuthState({
    setScreen: (s: string, replace?: boolean) => setScreen(s as Screen, replace),
    showToast: _showToast,
    onAuthSuccess: (data) => {
      if (data.is_admin) setIsAdmin(true)
      if (data.user_id) setCurrentUserId(data.user_id)
      if (data.auth_methods) setAuthMethods(data.auth_methods)
    },
    tryJoinCode,
    pathToScreen: PATH_TO_SCREEN,
  })

  // Destructure auth state for use in JSX and handlers
  const {
    phone, setPhone, code, error, setError, loading, setLoading,
    checking, setChecking, isOffline,
    emailInput, setEmailInput, passwordInput, setPasswordInput,
    confirmPassword, setConfirmPassword, nameInput, setNameInput,
    lastNameInput, setLastNameInput, googleClientId,
    resetSent, setResetSent, magicLinkSent, setMagicLinkSent,
    smsOptIn, setSmsOptIn, loginPennyText,
    phoneInputRef, codeInputRefs,
    handlePhoneChange, handleSendCode,
    handleCodeChange, handleCodeKeyDown, handleCodePaste,
    handleEmailRegister, handleEmailLogin, handleForgotPassword,
    handleMagicLink, handleResetPassword, handleLogout,
    startLoginTypewriter,
  } = auth

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
          res.clone().json().then((data: { is_admin?: boolean; user_id?: number; login_count?: number; name?: string; auth_methods?: string[]; can_submit_feedback?: boolean }) => {
            if (data.is_admin) setIsAdmin(true)
            if (data.can_submit_feedback) setCanSubmitFeedback(true)
            if (data.user_id) { setCurrentUserId(data.user_id); identify(data.user_id) }
            if (data.login_count != null) setLoginCount(data.login_count)
            if (data.auth_methods) setAuthMethods(data.auth_methods)
            if (data.name) {
              const parts = data.name.split(' ')
              setSettingsFirstName(parts[0] || '')
              setSettingsLastName(parts.slice(1).join(' '))
            }
          }).catch(() => {})
          tryJoinCode()
          loadSettings() // Load settings once at auth
          // Check for post-auth join prompt from shared list link
          const shareInvite = localStorage.getItem('groflo_share_invite')
          const shareName = localStorage.getItem('groflo_share_name')
          if (shareInvite) {
            localStorage.removeItem('groflo_share_invite')
            localStorage.removeItem('groflo_share_name')
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
            const redirect = sessionStorage.getItem('groflo_redirect')
            if (redirect) {
              sessionStorage.removeItem('groflo_redirect')
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
            sessionStorage.setItem('groflo_redirect', path)
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
        if (sessionStorage.getItem('groflo_404')) {
          sessionStorage.removeItem('groflo_404')
          showToast("Page not found — here's your list")
        }
      })
  }, [])

  // Google config fetch and reset token extraction — moved to useAuthState hook

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
            if (data.account_merged && data.merge_message) {
              sessionStorage.setItem('groflo_merge_msg', data.merge_message)
            }
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

  // Login typewriter effect — managed by useAuthState.startLoginTypewriter
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => startLoginTypewriter(screen), [screen])


  // =========================================================================
  // LOAD LIST when on home screen
  // =========================================================================

  // sortItems — extracted to useListState hook

  const handleSessionExpired = useCallback(() => {
    setChatLoading(false)
    setScreen('phone', true)
    setPhone('')
    setError('')
    _showToast('Session expired — please log in again', 'error')
  }, [_showToast])

  // Register session expiry handler once
  handleSessionExpiredRef.current = handleSessionExpired
  useEffect(() => {
    setSessionExpiredHandler(handleSessionExpired)
  }, [handleSessionExpired])

  // updateItem, reloadList — extracted to useListState hook

  // Show a Penny chime-in message with auto-clear
  const showChimeIn = (msg: string, duration = 4000) => {
    if (chimeInTimer.current) clearTimeout(chimeInTimer.current)
    setPennyChimeIn(msg)
    chimeInTimer.current = setTimeout(() => setPennyChimeIn(null), duration)
  }
  showChimeInRef.current = showChimeIn

  const loadTrips = async () => {
    const data = await fetchAPI('/api/trips')
    if (data) setTrips(Array.isArray(data) ? data as ShoppingTrip[] : [])
  }

  const loadStoreLists = async () => {
    const data = await fetchAPI('/api/saved-lists')
    if (Array.isArray(data)) {
      const withStore = data.filter((l: { store_name?: string }) => l.store_name && l.store_name.trim() !== '')
      setStoreLists(withStore as typeof storeLists)
    }
  }

  const loadRestockNudges = async () => {
    const data = await fetchAPI('/api/penny/nudges')
    if (data && !Array.isArray(data) && Array.isArray(data.nudges)) {
      setRestockNudges(data.nudges.slice(0, 5))
    }
  }

  const loadStoreDeptOrder = async (store: string | null) => {
    if (!storeOrderEnabled || !store) {
      setStoreDeptOrder(null)
      setStoreDeptOrderLearned(false)
      return
    }
    try {
      const data = await fetchAPI(`/api/list/department-order?store=${encodeURIComponent(store)}`)
      if (data && !Array.isArray(data)) {
        const departments = data.departments as string[] | undefined
        const learned = data.learned as boolean
        setStoreDeptOrder(departments || null)
        setStoreDeptOrderLearned(learned)
        // One-time chime-in when Penny first learns a store's order
        if (learned && departments && departments.length > 0) {
          const key = `storeOrderNotified_${store}`
          if (!localStorage.getItem(key)) {
            localStorage.setItem(key, '1')
            showChimeIn(`I reordered your list to match how you shop at ${store}`, 5000)
          }
        }
      }
    } catch (e) {
      console.warn('[load] department order failed:', e)
    }
  }

  const npsCheckedRef = useRef(false)
  useEffect(() => {
    if (screen !== 'home') return
    reloadList().catch(e => { console.warn('[load] list failed:', e); setListLoading(false) })
    loadTrips().catch(e => console.warn('[load] trips failed:', e))
    loadStoreLists().catch(e => console.warn('[load] store lists failed:', e))
    loadRestockNudges().catch(e => console.warn('[load] nudges failed:', e))
    loadStoreDeptOrder(activeStore).catch(e => console.warn('[load] dept order failed:', e))
    // Show account merge message if auth just linked accounts
    const mergeMsg = sessionStorage.getItem('groflo_merge_msg')
    if (mergeMsg) {
      sessionStorage.removeItem('groflo_merge_msg')
      setTimeout(() => typePennyHome(mergeMsg), 500)
    }
    // Only check NPS once per session — dismiss expires after 90 days
    if (!npsCheckedRef.current) {
      const dismissedAt = localStorage.getItem('nps_dismissed_at')
      const dismissDays = 90
      const isRecentlyDismissed = dismissedAt && (Date.now() - Number(dismissedAt)) < dismissDays * 86400000
      if (!isRecentlyDismissed) {
        npsCheckedRef.current = true
        localStorage.removeItem('nps_dismissed_at')
        localStorage.removeItem('nps_dismissed')
        fetchAPI('/api/nps/check')
          .then(data => { if (data && !Array.isArray(data) && data.show_nps) setShowNPS(true) })
          .catch(e => console.warn('[load] nps check failed:', e))
      } else {
        npsCheckedRef.current = true
      }
    }
  }, [screen])

  // Reload department order when active store changes
  useEffect(() => {
    if (screen !== 'home') return
    // Animate opacity on store change
    setDeptListOpacity(0.7)
    loadStoreDeptOrder(activeStore).catch(() => {})
    const t = setTimeout(() => setDeptListOpacity(1), 150)
    return () => clearTimeout(t)
  }, [activeStore, storeOrderEnabled])

  // Online/offline state tracking is handled by useAuthState hook.
  // Auto-refresh list when coming back online (delayed slightly to let toggle replay finish first):
  useEffect(() => {
    const goOnline = () => {
      if (screen === 'home') {
        // Delay reload to give the offline toggle queue time to replay first.
        // The replay effect fires on the React state transition (isOffline -> online),
        // which runs after this event handler. A 2s delay is enough for typical queues.
        setTimeout(() => reloadList().catch(() => {}), 2000)
      }
    }
    window.addEventListener('online', goOnline)
    return () => window.removeEventListener('online', goOnline)
  }, [screen])

  // PWA install prompt — capture the browser event and show a banner after the user has items
  useEffect(() => {
    if (localStorage.getItem('groflo_install_dismissed')) return
    const handler = (e: Event) => {
      e.preventDefault()
      setInstallPrompt(e)
    }
    window.addEventListener('beforeinstallprompt', handler)
    return () => window.removeEventListener('beforeinstallprompt', handler)
  }, [])

  // Show install banner after user has 5+ items (engaged enough to find the app useful)
  useEffect(() => {
    if (installPrompt && items.length >= 5 && screen === 'home' && !showInstallBanner) {
      setShowInstallBanner(true)
    }
  }, [installPrompt, items.length, screen])

  // Refresh list when user returns to the app (handles household changes while away)
  useEffect(() => {
    const onVisible = () => {
      if (document.visibilityState === 'visible' && screen === 'home') {
        reloadList().catch(() => {})
      }
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => document.removeEventListener('visibilitychange', onVisible)
  }, [screen])

  // SSE — Real-time list synchronization via Server-Sent Events.
  // Connects when on the home screen, falls back to polling on failure.
  const sseRef = useRef<EventSource | null>(null)
  const sseDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => {
    if (screen !== 'home') {
      // Close SSE when navigating away from home
      if (sseRef.current) {
        sseRef.current.close()
        sseRef.current = null
      }
      return
    }

    // Try to connect to SSE stream
    let es: EventSource
    try {
      es = new EventSource('/api/list/stream')
    } catch {
      return // SSE not supported — polling fallback handles it
    }
    sseRef.current = es

    es.addEventListener('list-update', () => {
      // Debounce: rapid mutations shouldn't cause excessive re-fetches
      if (sseDebounceRef.current) clearTimeout(sseDebounceRef.current)
      sseDebounceRef.current = setTimeout(() => {
        reloadList().catch(() => {})
      }, 500)
    })

    // Async command results — pushed by the server after slow-path parsing completes
    es.addEventListener('command-result', (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data)
        processParseResult(data)
      } catch (err) {
        console.error('[sse] failed to parse command-result:', err)
      }
    })

    es.onerror = () => {
      // On error, close and let the existing polling/visibility fallback handle it.
      // The browser will NOT auto-reconnect since we close explicitly on error.
      if (sseRef.current) {
        sseRef.current.close()
        sseRef.current = null
      }
    }

    return () => {
      if (sseDebounceRef.current) clearTimeout(sseDebounceRef.current)
      es.close()
      sseRef.current = null
    }
  }, [screen])

  // Preload adjacent screen chunks after home loads (improves tab switching)
  const preloadedRef = useRef(false)
  useEffect(() => {
    if (screen !== 'home' || preloadedRef.current) return
    preloadedRef.current = true
    const idle = window.requestIdleCallback || ((cb: () => void) => setTimeout(cb, 2000))
    idle(() => {
      import('./components/PennySettingsScreen')
      import('./components/ShareScreen')
      import('./components/ProfileScreen')
    })
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
    typePennyHome('Trip added! Everyone will see it.')
  }

  const handleCancelTrip = async () => {
    const data = await fetchAPI('/api/trips', { method: 'DELETE' })
    if (!data) { showToast('Couldn\'t cancel your trip — please try again', 'error'); return }
    loadTrips()
    typePennyHome('Trip cancelled.')
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

  // Show Penny chime-in when a household member has a trip planned (only for users who have added items)
  const tripChimeShown = useRef(false)
  useEffect(() => {
    if (tripChimeShown.current || !hasEverAddedItems) return
    const othersTrips = activeTrips.filter(t => !t.is_mine)
    if (othersTrips.length > 0) {
      tripChimeShown.current = true
      const t = othersTrips[0]
      const who = t.user_name || 'Someone'
      const when = formatTripTime(t)
      showChimeIn(`${who} is heading to the store ${when.toLowerCase()} — add what you need!`, 6000)
    }
  }, [trips])


  // loadSettings, loadReferralData, and their effects are now in useSettingsState hook

  const lastErrorToastRef = useRef(0)
  const showToast = (message: string, type: 'success' | 'error' = 'success', action?: { label: string; onClick: () => void }) => {
    if (type === 'error') {
      const now = Date.now()
      if (now - lastErrorToastRef.current < 3000) return
      lastErrorToastRef.current = now
      haptic.error()
    }
    _showToast(message, type, action)
  }

  // typePennyHome — extracted to useChatState hook

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
      if (toast.action) btns.push({ label: toast.action.label, onClick: () => { toast.action!.onClick(); clearToast() } })
      else if ((lastAdded.length > 0 || lastSkipped.length > 0) && toast.type !== 'error' && !hasReviewedRef.current) btns.push({ label: 'Review', onClick: () => { hasReviewedRef.current = true; pushModalState(); setShowReviewSheet(true); clearToast() } })
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
        // We know their name — time-aware personal greeting
        const hour = new Date().getHours()
        const timeGreeting = hour < 12 ? 'Good morning' : hour < 17 ? 'Hey' : 'Good evening'
        const timeSuffix = hour >= 20 ? ' Late-night shopping?' : ''
        typePennyHome(`${timeGreeting}, ${settingsFirstName}! What do we need from the store?${timeSuffix}`)
      } else {
        typePennyHome(pickRandom(RETURNING_GREETINGS))
      }
    } else if (items.length > 0 && !hasGreetedWithItems.current) {
      hasGreetedWithItems.current = true
      if (settingsFirstName) {
        const hour = new Date().getHours()
        const timeGreeting = hour < 12 ? 'Good morning' : hour < 17 ? 'Hey' : 'Good evening'
        typePennyHome(`${timeGreeting}, ${settingsFirstName}! What can I add to your list?`)
      } else {
        typePennyHome(pickRandom(EXISTING_LIST_GREETINGS))
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen, pennyStatus, toast?.message, pennyChimeIn, showDoneToast, items.length])

  // Reset empty greeting when items are cleared so it greets again next time
  // Also mark that the user has added items at least once (for first-time user detection)
  useEffect(() => {
    if (items.length > 0) {
      hasGreetedEmpty.current = false
      if (!hasEverAddedItems) setHasEverAddedItems(true)
    }
  }, [items.length, hasEverAddedItems])

  // Repurchase nudge — when list is empty and user returns, suggest items due for restock
  const nudgeShownRef = useRef(false)
  useEffect(() => {
    if (screen !== 'home' || items.length > 0 || nudgeShownRef.current || loginCount <= 1) return
    nudgeShownRef.current = true
    const timer = setTimeout(async () => {
      try {
        const data = await fetchAPI('/api/penny/nudges')
        const nudges = (data as { nudges?: Array<{ item_name: string; days_overdue: number }> })?.nudges
        if (nudges && nudges.length > 0) {
          const top = nudges.slice(0, 3)
          const names = top.map(n => n.item_name).join(', ')
          typePennyHome(`Running low on ${names}?`, top.map(n => ({
            label: `+ ${n.item_name}`,
            onClick: () => {
              fetch('/api/list/parse', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ text: n.item_name }),
              }).then(r => r.ok ? r.json() : null).then(d => {
                if (d?.items?.length > 0) {
                  try { localStorage.setItem('has_added_items', '1') } catch { /* localStorage unavailable */ }
                  setItems(prev => sortItems([...prev, ...d.items]))
                  showToast(`Added ${n.item_name}`)
                }
              }).catch(() => {})
            },
          })))
        }
      } catch { /* nudges are optional */ }
    }, 2000)
    return () => clearTimeout(timer)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen, items.length])

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
        "Want your family on this list too? It only takes a sec.",
        "Families that share lists never wonder 'did you get milk?' again.",
        "Quick thought — want to invite someone to shop with you?",
        "Last thought on this — sharing your list takes about 10 seconds.",
      ]
      typePennyHome(messages[Math.min(promptCount, messages.length - 1)], [{
        label: 'Sure, invite them',
        onClick: () => { setScreen('settings-share'); track('penny_invite_tapped', { prompt_count: promptCount }) },
      }])
    }, 12000)

    return () => clearTimeout(timer)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen, settingsMembers.length])

  const resizeAndSetPhoto = (file: Blob) => {
    const reader = new FileReader()
    reader.onload = () => {
      const dataUrl = reader.result as string
      const img = new Image()
      img.onload = () => {
        try {
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
          if (!ctx) { showToast('Could not process photo', 'error'); return }
          ctx.drawImage(img, 0, 0, w, h)
          setPendingImages(prev => [...prev, canvas.toDataURL('image/jpeg', 0.8)])
        } catch (err) {
          console.error('[photo] resize error:', err)
          showToast('Could not process photo — try again', 'error')
        }
      }
      img.onerror = () => showToast('Could not load photo — try a different image', 'error')
      img.src = dataUrl
    }
    reader.onerror = () => showToast('Could not read photo file', 'error')
    reader.readAsDataURL(file)
  }

  // Continuous camera: resize, send to parse API, show confirmation, re-open camera
  const submitCameraPhoto = (file: Blob) => {
    const reader = new FileReader()
    reader.onload = () => {
      const dataUrl = reader.result as string
      const img = new Image()
      img.onload = async () => {
        try {
          const MAX = 1024
          let w = img.width, h = img.height
          if (w > MAX || h > MAX) {
            if (w > h) { h = Math.round(h * MAX / w); w = MAX }
            else { w = Math.round(w * MAX / h); h = MAX }
          }
          const canvas = document.createElement('canvas')
          canvas.width = w; canvas.height = h
          const ctx = canvas.getContext('2d')
          if (!ctx) return
          ctx.drawImage(img, 0, 0, w, h)
          const resized = canvas.toDataURL('image/jpeg', 0.8)

          // Show processing state
          setCameraConfirmation('Reading...')

          // Send directly to parse API
          const res = await fetch('/api/list/parse', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ text: '', images: [resized] }),
          })
          if (res.status === 401) { cameraModeRef.current = false; handleSessionExpired(); return }
          if (res.ok) {
            const data = await res.json()
            const addedItems: ListItem[] = data.items || []
            const skippedItems = data.skipped || []
            const pennyMsg: string = data.message || ''

            if (addedItems.length > 0) {
              try { localStorage.setItem('has_added_items', '1') } catch { /* localStorage unavailable */ }
              setItems(prev => sortItems([...prev, ...addedItems]))
              // Auto-expand departments for new items
              const newDepts = new Set(addedItems.map((i: ListItem) => i.department))
              setExpandedDepts(prev => {
                const next = new Set(prev)
                newDepts.forEach(d => next.add(d))
                return next
              })
              const names = addedItems.map(i => i.name).join(', ')
              const skippedNote = skippedItems.length > 0 ? ` (${skippedItems.length} already on list)` : ''
              setCameraConfirmation(`+ ${names}${skippedNote}`)
            } else if (skippedItems.length > 0) {
              setCameraConfirmation(`Already on your list`)
            } else if (pennyMsg) {
              setCameraConfirmation(pennyMsg)
            } else {
              setCameraConfirmation("Couldn't read that — try again")
            }
          } else {
            setCameraConfirmation("Couldn't read that — try again")
          }

          // Clear confirmation and re-open camera after delay
          setTimeout(() => {
            setCameraConfirmation(null)
            if (cameraModeRef.current && cameraInputRef.current) {
              cameraPhotoTakenRef.current = false
              cameraInputRef.current.click()
              // Detect cancel on re-open
              const detectCancel = () => {
                setTimeout(() => {
                  if (cameraModeRef.current && !cameraPhotoTakenRef.current) {
                    cameraModeRef.current = false
                  }
                  window.removeEventListener('focus', detectCancel)
                }, 500)
              }
              window.addEventListener('focus', detectCancel)
            }
          }, 1800)
        } catch {
          setCameraConfirmation(null)
          cameraModeRef.current = false
          showToast('Something went wrong. Try again.', 'error')
        }
      }
      img.src = dataUrl
    }
    reader.readAsDataURL(file)
  }

  const handlePhotoSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files
    if (!files) return
    const isCamera = e.target === cameraInputRef.current

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

    // Continuous camera mode: auto-submit and re-open
    if (isCamera && cameraModeRef.current && files.length > 0) {
      cameraPhotoTakenRef.current = true
      submitCameraPhoto(files[0])
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

  // checkAllDone — extracted to useListState hook
  // Swap accept/decline/brand-pick/disambig — extracted to useSwapState hook

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
      } else {
        typePennyHome('Hmm, I couldn\'t read that receipt. Try a clearer photo?')
      }
    } catch {
      typePennyHome('Connection hiccup — try again in a moment.')
    }
    finally {
      setChatLoading(false)
      setPendingReceiptMode(false)
    }
  }

  // handleToggle, handleUrgentToggle, handleClaim — extracted to useListState hook
  // advanceDisambig, auto-confirm effect, handleBrandPick — extracted to useSwapState hook

  // handleSendChat — extracted to useChatState hook

  const handleNPSSubmit = async (score: number, comment: string) => {
    const result = await fetchAction('/api/nps', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ score, comment }),
    })
    if (result.ok) {
      showToast('Thanks for your feedback!')
      setShowNPS(false)
      localStorage.setItem('nps_dismissed_at', Date.now().toString())
    } else if (result.status !== 401) {
      showToast('Couldn\'t save feedback — please try again', 'error')
    }
  }

  const closeNPS = () => {
    setShowNPS(false)
    localStorage.setItem('nps_dismissed_at', Date.now().toString())
    // Server-side dismiss guard — survives localStorage clears
    fetchAPI('/api/nps/dismiss', { method: 'POST' }).catch(() => {})
  }

  const handleFeedbackSubmit = async (body: string, type: string, urgency: string): Promise<boolean> => {
    try {
      // Append device context for debugging
      const ctx = [
        `Screen: ${screen}`,
        `Viewport: ${window.innerWidth}x${window.innerHeight}`,
        `UA: ${navigator.userAgent}`,
        `Dark: ${document.documentElement.getAttribute('data-theme')}`,
        `Items: ${items.length}`,
      ].join(' | ')
      const enrichedBody = `${body}\n\n---\n_Device: ${ctx}_`

      const res = await fetch('/api/feedback', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ body: enrichedBody, type, urgency }),
      })
      if (res.ok) {
        showToast('Got it — your feedback shapes what we build next')
        return true
      } else if (res.status === 429) {
        showToast('Easy — max 10 per hour. Take a breather.', 'error')
      } else {
        showToast('Couldn\'t send feedback — try again', 'error')
      }
      return false
    } catch {
      showToast('No connection — try again later', 'error')
      return false
    }
  }


  // Push a modal history entry so browser back closes the modal
  const pushModalState = () => {
    const path = SCREEN_PATHS[screen] || '/'
    window.history.pushState({ screen, modal: true }, '', path)
  }

  // openDetail, openDetailFromReview — extracted to useListState hook

  const popModalState = () => {
    // Pop the modal history entry (only if we're not already handling popstate)
    if (window.history.state?.modal) {
      window.history.back()
    }
  }

  // closeEditSheet — extracted to useListState hook

  // --- Escape key closes topmost modal/sheet ---
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      if (showDoneSummary) { setShowDoneSummary(false); return }
      if (selectedItem) { closeEditSheet(); return }
      if (showTripSheet) { popModalState(); setShowTripSheet(false); return }
      if (showReviewSheet) { popModalState(); setShowReviewSheet(false); setLastAdded([]); setLastSkipped([]); return }
      if (showSaveSheet) { popModalState(); setShowSaveSheet(false); return }
    }
    window.addEventListener('keydown', handleEscape)
    return () => window.removeEventListener('keydown', handleEscape)
  })

  // handleEditSave, handleEditDelete, handleSwipeDelete, handleClearList — extracted to useListState hook

  const handleSaveList = async () => {
    if (!saveNameInput.trim()) return
    try {
      const payload: { name: string; store_name?: string } = { name: saveNameInput.trim() }
      if (saveStoreInput.trim()) payload.store_name = saveStoreInput.trim()
      const data = await fetchAPI('/api/saved-lists', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      if (data) {
        setShowSaveSheet(false); setSaveNameInput(''); setSaveStoreInput('')
        loadStoreLists().catch(() => {})
        handleClearList(); typePennyHome('List saved! You can load it anytime.')
      }
      else showToast('Couldn\'t save your list — please try again', 'error')
    } catch { showToast('Couldn\'t save your list — please try again', 'error') }
  }

  const handleLoadSavedList = async (id: number, mode: 'merge' | 'overwrite' = 'merge') => {
    const url = mode === 'overwrite' ? `/api/saved-lists/${id}/load?mode=overwrite` : `/api/saved-lists/${id}/load`
    const result = await fetchAction(url, { method: 'POST' })
    if (result.ok) {
      setScreen('home')
      reloadList()
      typePennyHome('List loaded — let\'s get shopping!')
    } else if (result.status !== 401) { showToast('Couldn\'t load that list — please try again', 'error') }
  }
  handleLoadSavedListRef.current = handleLoadSavedList

  // Auth handlers (handleEmailRegister, handleEmailLogin, handleForgotPassword,
  // handleMagicLink, handleResetPassword, handlePhoneChange, handleSendCode,
  // verifyCode, handleCodeChange, handleCodeKeyDown, handleCodePaste)
  // — all extracted to useAuthState hook

  // =========================================================================
  // Settings handlers (doSettingsSave, handleSettingsAutoSave, invite handlers) — extracted to useSettingsState hook

  // =========================================================================
  // LOADING STATE (checking existing session)
  // =========================================================================

  // =========================================================================
  // SHARED LIST SCREEN (standalone, no auth required)
  // =========================================================================

  if (sharedCode) {
    const navigateToSignUp = () => { setSharedCode(null); setSharedError(null); window.history.replaceState({}, '', '/'); setScreen('phone', true) }
    return (
      <SharedListScreen
        sharedData={sharedData}
        sharedLoading={sharedLoading}
        sharedError={sharedError}
        onSignUp={navigateToSignUp}
        onSignUpClear={() => { setSharedCode(null); window.history.replaceState({}, '', '/'); setScreen('phone', true) }}
      />
    )
  }

  if (checking) {
    return (
      <div style={styles.container}>
        <div style={{ ...styles.card, ...layout.columnCenter, gap: '16px' }}>
          <Logo />
          <p style={{ ...text.secondary, fontSize: FONT_BASE, margin: 0 }}>Loading…</p>
        </div>
      </div>
    )
  }

  // =========================================================================
  // PHONE SCREEN
  // =========================================================================

  if (screen === 'phone') {
    return (
      <PhoneScreen
        phone={phone} smsOptIn={smsOptIn} loading={loading} error={error}
        loginPennyText={loginPennyText} googleClientId={googleClientId}
        phoneInputRef={phoneInputRef}
        onPhoneChange={handlePhoneChange}
        onSmsOptInChange={v => setSmsOptIn(v)}
        onSendCode={() => handleSendCode()}
        onNavigate={s => setScreen(s as Screen)}
        setError={setError}
      />
    )
  }

  // =========================================================================
  // EMAIL LOGIN SCREEN
  // =========================================================================

  if (screen === 'email-login') {
    return (
      <EmailLoginScreen
        emailInput={emailInput} passwordInput={passwordInput}
        loading={loading} error={error} magicLinkSent={magicLinkSent}
        loginPennyText={loginPennyText} googleClientId={googleClientId}
        onEmailChange={v => setEmailInput(v)} onPasswordChange={v => setPasswordInput(v)}
        onLogin={handleEmailLogin} onMagicLink={handleMagicLink}
        onNavigate={s => setScreen(s as Screen)}
        setError={setError} setMagicLinkSent={setMagicLinkSent}
      />
    )
  }

  // =========================================================================
  // EMAIL REGISTER SCREEN
  // =========================================================================

  if (screen === 'email-register') {
    return (
      <EmailRegisterScreen
        nameInput={nameInput} lastNameInput={lastNameInput}
        emailInput={emailInput} passwordInput={passwordInput} confirmPassword={confirmPassword}
        loading={loading} error={error} loginPennyText={loginPennyText}
        onNameChange={v => setNameInput(v)} onLastNameChange={v => setLastNameInput(v)}
        onEmailChange={v => setEmailInput(v)} onPasswordChange={v => setPasswordInput(v)}
        onConfirmPasswordChange={v => setConfirmPassword(v)}
        onRegister={handleEmailRegister}
        onNavigate={s => setScreen(s as Screen)}
        setError={setError}
      />
    )
  }

  // =========================================================================
  // FORGOT PASSWORD SCREEN
  // =========================================================================

  if (screen === 'forgot-password') {
    return (
      <ForgotPasswordScreen
        emailInput={emailInput} loading={loading} error={error} resetSent={resetSent}
        onEmailChange={v => setEmailInput(v)} onSubmit={handleForgotPassword}
        onNavigate={s => setScreen(s as Screen)}
        setError={setError} setResetSent={setResetSent}
      />
    )
  }

  // =========================================================================
  // RESET PASSWORD SCREEN
  // =========================================================================

  if (screen === 'reset-password') {
    return (
      <ResetPasswordScreen
        passwordInput={passwordInput} confirmPassword={confirmPassword}
        loading={loading} error={error}
        onPasswordChange={v => setPasswordInput(v)} onConfirmPasswordChange={v => setConfirmPassword(v)}
        onSubmit={handleResetPassword}
        onNavigate={s => setScreen(s as Screen)}
        setError={setError}
      />
    )
  }

  // =========================================================================
  // CODE SCREEN
  // =========================================================================

  if (screen === 'code') {
    return (
      <CodeScreen
        phone={phone} code={code} loading={loading} error={error}
        codeInputRefs={codeInputRefs}
        onCodeChange={handleCodeChange} onCodeKeyDown={handleCodeKeyDown} onCodePaste={handleCodePaste}
        onResendCode={(p) => handleSendCode(p)}
        onNavigate={s => setScreen(s as Screen)}
        setError={setError}
      />
    )
  }

  // =========================================================================
  // SHARED HELPERS — menu, toast, feedback FAB (extracted to avoid duplication)
  // =========================================================================

  // Wrap any authenticated screen with the FeedbackWidget (position:fixed overlay)
  const withFeedbackFAB = (content: React.ReactNode) => (
    <>
      {content}
      {canSubmitFeedback && <FeedbackWidget onSubmit={handleFeedbackSubmit} />}
    </>
  )

  const getActiveNav = (s: Screen): NavScreen => {
    switch (s) {
      case 'home': case 'earnings': case 'receipts': return 'home'
      case 'saved-lists': return 'saved-lists'
      case 'penny': return 'penny'
      case 'settings-share': return 'settings-share'
      case 'admin': return 'admin'
      default: return 'profile'
    }
  }

  const renderNavMenu = () => (
    <NavMenu
      open={navMenuOpen}
      active={getActiveNav(screen)}
      isAdmin={isAdmin}
      onNavigate={(s) => setScreen(s as Screen)}
      onClose={() => setNavMenuOpen(false)}
    />
  )


  const renderPennyToast = () => toast ? (
    <div role={toast.type === 'error' ? 'alert' : 'status'} style={{
      position: 'fixed' as const,
      top: '16px',
      left: '16px',
      right: '16px',
      zIndex: 9999,
      display: 'flex',
      alignItems: 'center',
      gap: '10px',
      padding: '12px 16px',
      backgroundColor: toast.type === 'error' ? 'var(--error-bg)' : 'var(--surface)',
      color: toast.type === 'error' ? 'var(--error)' : 'var(--text)',
      borderRadius: '14px',
      border: `1px solid ${toast.type === 'error' ? 'var(--error)' : 'var(--border)'}`,
      boxShadow: '0 4px 20px rgba(0,0,0,0.12)',
      fontFamily: FONT,
      fontSize: FONT_SM,
      fontWeight: 500,
      animation: 'feedbackSlideDown 0.25s ease-out',
    }}>
      <span style={{ flex: 1 }}>{toast.message}</span>
      {toast.action && (
        <button
          onClick={() => { toast.action!.onClick(); clearToast() }}
          style={{ background: 'var(--brand)', border: 'none', color: '#fff', fontFamily: FONT, fontSize: FONT_2XS, fontWeight: 600, cursor: 'pointer', padding: '5px 12px', borderRadius: '12px', flexShrink: 0 }}
        >{toast.action.label}</button>
      )}
      <button
        onClick={() => { clearToast() }}
        aria-label="Dismiss"
        style={{ ...buttons.reset, color: 'var(--text-secondary)', fontSize: FONT_MD, padding: '4px', lineHeight: 1, flexShrink: 0, opacity: 0.6 }}
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
    'My Lists': <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>,
    'Profile': <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>,
    'Savings': <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/></svg>,
  }

  const SUB_SCREENS: Screen[] = ['earnings', 'receipts', 'help', 'admin']

  const renderPageHeader = (pageName: string, active: Screen) => {
    const isSubScreen = SUB_SCREENS.includes(active)
    return (
      <div style={{ flexShrink: 0, backgroundColor: 'var(--surface)', borderBottom: '1px solid var(--border)' }}>
        <div style={{ ...layout.row, padding: '12px 20px' }}>
          {isSubScreen ? (
            <button
              onClick={() => setScreen(active === 'earnings' || active === 'receipts' ? 'home' : active === 'admin' ? 'profile' : 'profile')}
              aria-label="Back"
              style={{ ...buttons.icon, padding: '0 4px', marginRight: '8px' }}
            >
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="var(--text)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="15 18 9 12 15 6"/>
              </svg>
            </button>
          ) : (
            <HamburgerButton onClick={() => setNavMenuOpen(true)} />
          )}
          <span style={{
            fontFamily: FONT,
            fontSize: FONT_BASE,
            fontWeight: 600,
            color: 'var(--text)',
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
          }}>{pageIcons[pageName]}{pageName}</span>
          {/* Savings badge — upper right */}
          <span
            style={{
              ...layout.column,
              marginLeft: 'auto',
              cursor: 'pointer',
              alignItems: 'center',
              lineHeight: 1,
              backgroundColor: 'color-mix(in srgb, var(--brand) 8%, transparent)',
              borderRadius: '12px',
              padding: '4px 10px',
            }}
            onClick={() => setScreen('earnings')}
            aria-label="View savings"
          >
            {potentialSavingsCents > 0 ? (
              <>
                <span style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, color: 'var(--brand)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/></svg>
                  ${(potentialSavingsCents / 100).toFixed(2)}
                </span>
                <span style={{ fontFamily: FONT, fontSize: '0.625rem', color: 'var(--text-secondary)', marginTop: '1px' }}>save</span>
              </>
            ) : earningsTotalCents > 0 ? (
              <>
                <span style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, color: 'var(--brand)', opacity: 0.6, display: 'flex', alignItems: 'center', gap: '4px' }}>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/></svg>
                  ${(earningsTotalCents / 100).toFixed(2)}
                </span>
                <span style={{ fontFamily: FONT, fontSize: '0.625rem', color: 'var(--text-secondary)', marginTop: '1px' }}>saved</span>
              </>
            ) : (
              <>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/></svg>
                <span style={{ fontFamily: FONT, fontSize: '0.625rem', color: 'var(--text-secondary)', marginTop: '1px' }}>deals</span>
              </>
            )}
          </span>
        </div>
      </div>
    )
  }

  // =========================================================================
  // PENNY SETTINGS (merged: Penny + Profile + Saved Lists + Sign Out)
  // =========================================================================

  if (screen === 'penny') {
    return withFeedbackFAB(
      <><PennySettingsScreen
        pennyPhone={settingsPennyPhone}
        pennySlider={pennySlider}
        pennyAutotune={pennyAutotune}
        pennyDeals={pennyDeals}
        storeOrderEnabled={storeOrderEnabled}
        dailyDigestEnabled={dailyDigestEnabled}
        onSliderChange={setPennySlider}
        onAutotuneToggle={toggleAutotune}
        onDealsToggle={toggleDeals}
        onStoreOrderToggle={toggleStoreOrder}
        onDailyDigestToggle={toggleDailyDigest}

        track={track}
        renderHeader={() => renderPageHeader('Penny', 'penny')}
        renderToast={renderPennyToast}

        transitionStyle={transitionStyle}
      />
      {renderNavMenu()}
      </>
    )
  }

  // =========================================================================
  // PROFILE PAGE
  // =========================================================================

  if (screen === 'profile') {
    return withFeedbackFAB(
      <><ProfileScreen
        firstName={settingsFirstName}
        lastName={settingsLastName}
        email={settingsEmail}
        phone={settingsUserPhone}
        address={settingsAddress}
        onFirstNameChange={setSettingsFirstName}
        onLastNameChange={setSettingsLastName}
        onEmailChange={setSettingsEmail}
        onPhoneChange={setSettingsUserPhone}
        onAddressChange={setSettingsAddress}
        onAutoSave={handleSettingsAutoSave}
        authMethods={authMethods}
        onAuthMethodAdded={(method: string) => setAuthMethods(prev => prev.includes(method) ? prev : [...prev, method])}
        currentUserId={currentUserId}
        isAdmin={isAdmin}
        onNavigate={(s: string) => setScreen(s as Screen)}
        onLogout={handleLogout}
        showToast={showToast}
        onSessionExpired={handleSessionExpired}

        placesReady={placesReady}
        darkMode={darkMode}
        onToggleDarkMode={toggleDarkMode}
        renderHeader={() => renderPageHeader('Profile', 'profile')}

        transitionStyle={transitionStyle}
      />
      {renderNavMenu()}
      </>
    )
  }

  // =========================================================================
  // HELP / FAQ / BUG REPORT PAGE
  // =========================================================================

  if (screen === 'help') {
    return withFeedbackFAB(
      <><HelpScreen
        showToast={showToast}
        onSessionExpired={handleSessionExpired}
        renderHeader={() => renderPageHeader('Help', 'help')}

        transitionStyle={transitionStyle}
      />
      {renderNavMenu()}
      </>
    )
  }

  // =========================================================================
  // SAVED LISTS SCREEN
  // =========================================================================

  if (screen === 'saved-lists') {
    return withFeedbackFAB(
      <><SavedListsScreen
        renderHeader={() => renderPageHeader('My Lists', 'saved-lists')}
        renderToast={renderPennyToast}
        transitionStyle={transitionStyle}
        onShowToast={showToast}
        onNavigateHome={() => setScreen('home')}
      />
      {renderNavMenu()}
      </>
    )
  }

  if (screen === 'settings-share') {
    return withFeedbackFAB(
      <><ShareScreen
        referralCode={referralCode}
        referralFriends={referralFriends}
        settingsMembers={settingsMembers}
        inviteLoading={inviteLoading}
        confirmModal={confirmModal}
        onInvite={handleOpenInviteSheet}
        onLeaveHousehold={() => openConfirmModal({
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
        onShareList={async () => {
          try {
            const res = await fetch('/api/list/share', { method: 'POST' })
            if (res.status === 401) { handleSessionExpired(); return }
            if (res.ok) {
              const data = await res.json()
              const url = `${window.location.origin}${data.url}`
              track('list_shared', { method: 'share' in navigator ? 'native' : 'clipboard' })
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
        onShareReferral={() => {
          const url = `https://app.groflo.io/r/${referralCode}`
          const text = `Try Groflo — Penny AI helps your household organize the grocery list. Just text, email, or snap a photo and she handles the rest! ${url}`
          track('referral_shared', { method: 'share' in navigator ? 'native' : 'sms' })
          if (navigator.share) {
            navigator.share({ title: 'Groflo', text }).catch(() => {})
          } else {
            window.open(`sms:?&body=${encodeURIComponent(text)}`, '_self')
          }
        }}
        onCopyReferral={() => {
          track('referral_copied')
          navigator.clipboard.writeText(`https://app.groflo.io/r/${referralCode}`).then(() => showToast('Link copied!')).catch(() => showToast('Could not copy', 'error'))
        }}
        onShowToast={showToast}
        runConfirmAction={runConfirmAction}
        closeConfirmModal={closeConfirmModal}
        renderHeader={() => renderPageHeader('Share & Earn', 'settings-share')}
        renderToast={renderPennyToast}

        transitionStyle={transitionStyle}
      />
      {showInviteSheet && (
        <div style={{ position: 'fixed', inset: 0, zIndex: 9000, backgroundColor: 'rgba(0,0,0,0.4)', display: 'flex', alignItems: 'flex-end', justifyContent: 'center' }} onClick={() => setShowInviteSheet(false)}>
          <div style={{ width: '100%', maxWidth: '480px', backgroundColor: 'var(--surface)', borderRadius: '20px 20px 0 0', padding: '20px 20px 40px' }} onClick={e => e.stopPropagation()}>
            <div style={{ width: '36px', height: '4px', borderRadius: '2px', backgroundColor: 'var(--border)', margin: '0 auto 16px' }} />
            <h3 style={{ fontFamily: 'Avenir, Avenir Next, sans-serif', fontSize: '18px', fontWeight: 700, color: 'var(--text)', margin: '0 0 16px' }}>Add your family to this list</h3>

            {/* SMS invite */}
            <label style={{ fontFamily: 'Avenir, Avenir Next, sans-serif', fontSize: '14px', color: 'var(--text-secondary)', display: 'block', marginBottom: '6px' }}>Their phone number</label>
            <input
              type="tel"
              placeholder="(555) 123-4567"
              value={invitePhone}
              onChange={e => setInvitePhone(e.target.value)}
              autoFocus
              style={{ width: '100%', padding: '12px 16px', fontSize: '16px', fontFamily: 'Avenir, Avenir Next, sans-serif', color: 'var(--text)', backgroundColor: 'var(--bg)', border: '1.5px solid var(--border)', borderRadius: '12px', outline: 'none', boxSizing: 'border-box' }}
            />
            <p style={{ fontFamily: 'Avenir, Avenir Next, sans-serif', fontSize: '12px', color: 'var(--text-secondary)', margin: '6px 0 12px' }}>
              We'll text them a link to join your shared grocery list
            </p>
            <button
              onClick={handleSendSMSInvite}
              disabled={inviteLoading || inviteSent || invitePhone.replace(/\D/g, '').length < 10}
              style={{
                width: '100%', padding: '14px', fontFamily: 'Avenir, Avenir Next, sans-serif', fontSize: '16px', fontWeight: 600,
                color: '#fff', backgroundColor: 'var(--brand)', border: 'none', borderRadius: '12px', cursor: 'pointer',
                opacity: (inviteLoading || inviteSent || invitePhone.replace(/\D/g, '').length < 10) ? 0.5 : 1,
              }}
            >
              {inviteSent ? '✓ Sent!' : inviteLoading ? 'Sending…' : 'Send SMS invite'}
            </button>

            {/* Divider */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px', margin: '20px 0' }}>
              <div style={{ flex: 1, height: '1px', backgroundColor: 'var(--border)' }} />
              <span style={{ fontFamily: 'Avenir, Avenir Next, sans-serif', fontSize: '13px', color: 'var(--text-secondary)' }}>or scan to join</span>
              <div style={{ flex: 1, height: '1px', backgroundColor: 'var(--border)' }} />
            </div>

            {/* QR code for sharing */}
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', padding: '8px 0 16px' }}>
              <QRCodeCanvas
                value={`https://app.groflo.io/r/${referralCode}?via=qr`}
                size={160}
                level="M"
                marginSize={2}
                fgColor="#35644F"
                bgColor="transparent"
              />
              <p style={{ fontFamily: 'Avenir, Avenir Next, sans-serif', fontSize: '11px', color: 'var(--text-secondary)', marginTop: '8px', textAlign: 'center' }}>
                Scan with phone camera to join your list
              </p>
            </div>

            {/* Share link fallback */}
            <button
              onClick={handleShareInviteLink}
              disabled={inviteLoading}
              style={{
                width: '100%', padding: '14px', fontFamily: 'Avenir, Avenir Next, sans-serif', fontSize: '16px', fontWeight: 600,
                color: 'var(--brand)', backgroundColor: 'transparent', border: '1.5px solid var(--brand)', borderRadius: '12px', cursor: 'pointer',
                opacity: inviteLoading ? 0.5 : 1,
              }}
            >
              Share link instead
            </button>
          </div>
        </div>
      )}
      {renderNavMenu()}
      </>
    )
  }


  // =========================================================================
  // ADMIN DASHBOARD SCREEN
  // =========================================================================

  if (screen === 'admin') {
    return withFeedbackFAB(
      <><AdminScreen

        showToast={showToast}
        renderHeader={() => renderPageHeader('Dashboard', 'admin')}
        renderToast={renderPennyToast}

        transitionStyle={transitionStyle}
      />
      {renderNavMenu()}
      </>
    )
  }

  // =========================================================================
  // EARNINGS SCREEN
  // =========================================================================

  if (screen === 'earnings') {
    return withFeedbackFAB(
      <ScreenTransition style={transitionStyle}>
      <div style={listStyles.page}>
        {renderPageHeader('Savings', 'earnings')}
        <div style={{ flex: 1, overflowY: 'auto', overscrollBehavior: 'none', WebkitOverflowScrolling: 'touch' }}>
          {/* Groflo Wrapped CTA */}
          <div
            onClick={() => setShowWrapped(true)}
            style={{
              margin: '12px 16px', padding: '14px 16px', borderRadius: '12px', cursor: 'pointer',
              background: 'linear-gradient(135deg, #35644F 0%, #2D8B5E 100%)',
              display: 'flex', alignItems: 'center', gap: '12px',
            }}
          >
            <span style={{ fontSize: '24px' }}>🎁</span>
            <div style={{ flex: 1 }}>
              <div style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, color: '#FFF' }}>Your Monthly Wrapped</div>
              <div style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'rgba(255,255,255,0.8)' }}>See your savings recap — share it!</div>
            </div>
            <span style={{ color: 'rgba(255,255,255,0.7)', fontSize: '1.25rem' }}>→</span>
          </div>
          <EarningsScreen
            onBack={() => setScreen('home')}

            onViewReceipts={() => setScreen('receipts')}
            onGoToList={() => setScreen('home')}
            onEarningsLoaded={(cents) => setEarningsTotalCents(cents)}
          />
        </div>
        {renderNavMenu()}
      </div>
      {showWrapped && <WrappedCard onClose={() => setShowWrapped(false)} />}
      {milestoneData && <MilestoneCard milestone={milestoneData} onClose={() => setMilestoneData(null)} />}
      </ScreenTransition>
    )
  }

  // =========================================================================
  // RECEIPTS SCREEN
  // =========================================================================

  if (screen === 'receipts') {
    return withFeedbackFAB(
      <ScreenTransition style={transitionStyle}>
      <div style={listStyles.page}>
        {renderPageHeader('Receipts', 'receipts')}
        <div style={{ flex: 1, overflowY: 'auto', overscrollBehavior: 'none', WebkitOverflowScrolling: 'touch' }}>
          <ReceiptScreen
            hasAcceptedSwaps={Object.keys(swapsByItemId).length > 0 || earningsTotalCents > 0}
            onSnapReceipt={() => {
              setPendingReceiptMode(true)
              receiptInputRef.current?.click()
            }}
          />
        </div>
        {renderNavMenu()}
      </div>
      </ScreenTransition>
    )
  }

  // =========================================================================
  // HOME SCREEN — Shopping list
  // =========================================================================

  // handleDeptClick — extracted to useListState hook

  // Departments that have items, in store-aisle order (+ any unknown depts appended)
  // Use learned store order when available, fall back to default DEPARTMENTS order
  const deptOrderBase = (storeDeptOrderLearned && storeDeptOrder && storeOrderEnabled)
    ? storeDeptOrder
    : DEPARTMENTS
  const activeDepts = deptOrderBase.filter(d => items.some(i => i.department === d))
  // Append any departments not in the ordering source, alphabetically
  const allItemDepts = [...new Set(items.map(i => i.department))]
  const extraDepts = allItemDepts
    .filter(d => !deptOrderBase.includes(d))
    .sort((a, b) => a.localeCompare(b))
  if (extraDepts.length > 0) activeDepts.push(...extraDepts)

  return (
    <ScreenTransition style={transitionStyle}>
    <div style={listStyles.page}>
      {/* Skip navigation for keyboard/screen reader users */}
      <a href="#main-list" style={{ position: 'absolute', left: '-9999px', top: 'auto', width: '1px', height: '1px', overflow: 'hidden', zIndex: 10000 }} onFocus={e => { e.currentTarget.style.position = 'fixed'; e.currentTarget.style.left = '16px'; e.currentTarget.style.top = '16px'; e.currentTarget.style.width = 'auto'; e.currentTarget.style.height = 'auto'; e.currentTarget.style.padding = '12px 24px'; e.currentTarget.style.backgroundColor = 'var(--brand)'; e.currentTarget.style.color = '#fff'; e.currentTarget.style.borderRadius = '8px'; e.currentTarget.style.fontFamily = FONT; e.currentTarget.style.fontSize = FONT_BASE; e.currentTarget.style.fontWeight = '600'; e.currentTarget.style.textDecoration = 'none'; }} onBlur={e => { e.currentTarget.style.position = 'absolute'; e.currentTarget.style.left = '-9999px'; e.currentTarget.style.width = '1px'; e.currentTarget.style.height = '1px'; }}>Skip to shopping list</a>
      <ConfirmModal open={!!confirmModal} title={confirmModal?.title || ''} message={confirmModal?.message} confirmLabel={confirmModal?.confirmLabel} destructive={confirmModal?.destructive} onConfirm={runConfirmAction} onCancel={closeConfirmModal} />
      {/* Fixed header — compact */}
      <div style={{ flexShrink: 0, backgroundColor: 'var(--surface)', borderBottom: '1px solid var(--border)' }}>
        <div style={{ ...layout.row, padding: '12px 20px' }}>
          <HamburgerButton onClick={() => setNavMenuOpen(true)} />
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
                fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, color: 'var(--text)',
                background: 'transparent', border: 'none', borderBottom: '1.5px solid var(--brand)',
                outline: 'none', width: '160px', padding: '2px 0',
              }}
            />
          ) : (
            <span
              onClick={() => { setListNameDraft(listName); setEditingListName(true) }}
              style={{
                fontFamily: FONT,
                fontSize: FONT_BASE,
                fontWeight: 600,
                color: 'var(--text)',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                cursor: 'pointer',
              }}
            >{pageIcons['Shopping List']} {listName || 'Shopping List'}</span>
          )}
          {/* Savings badge — upper right */}
          <span
            style={{
              ...layout.column,
              marginLeft: 'auto',
              cursor: 'pointer',
              alignItems: 'center',
              lineHeight: 1,
              backgroundColor: 'color-mix(in srgb, var(--brand) 8%, transparent)',
              borderRadius: '12px',
              padding: '4px 10px',
            }}
            onClick={() => setScreen('earnings')}
            aria-label="View savings"
          >
            {potentialSavingsCents > 0 ? (
              <>
                <span style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, color: 'var(--brand)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/></svg>
                  ${(potentialSavingsCents / 100).toFixed(2)}
                </span>
                <span style={{ fontFamily: FONT, fontSize: '0.625rem', color: 'var(--text-secondary)', marginTop: '1px' }}>save</span>
              </>
            ) : earningsTotalCents > 0 ? (
              <>
                <span style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, color: 'var(--brand)', opacity: 0.6, display: 'flex', alignItems: 'center', gap: '4px' }}>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/></svg>
                  ${(earningsTotalCents / 100).toFixed(2)}
                </span>
                <span style={{ fontFamily: FONT, fontSize: '0.625rem', color: 'var(--text-secondary)', marginTop: '1px' }}>saved</span>
              </>
            ) : (
              <>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/></svg>
                <span style={{ fontFamily: FONT, fontSize: '0.625rem', color: 'var(--text-secondary)', marginTop: '1px' }}>deals</span>
              </>
            )}
          </span>
        </div>
        {/* Progress bar */}
        {items.length > 0 && (() => {
          const totalQty = items.reduce((s, i) => s + (i.quantity || 1), 0)
          const checkedQty = items.filter(i => i.checked).reduce((s, i) => s + (i.quantity || 1), 0)
          const pct = Math.round((checkedQty / totalQty) * 100)
          return (
            <div role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100} aria-label={`Shopping list ${pct}% complete`} style={{ height: '3px', backgroundColor: 'var(--border)' }}>
              <div style={{
                height: '100%',
                width: `${pct}%`,
                backgroundColor: 'var(--brand)',
                borderRadius: '0 2px 2px 0',
                transition: 'width 0.3s ease',
              }} />
            </div>
          )
        })()}
      </div>

      {/* Store filter pills — only show when user has store-tagged saved lists */}
      {storeLists.length > 0 && (
        <div style={{
          flexShrink: 0, display: 'flex', gap: '8px', padding: '8px 16px',
          overflowX: 'auto', WebkitOverflowScrolling: 'touch',
          borderBottom: '1px solid var(--border)', backgroundColor: 'var(--surface)',
          msOverflowStyle: 'none', scrollbarWidth: 'none',
        }}>
          <button
            onClick={() => { setActiveStore(null); reloadList() }}
            style={{
              ...buttons.reset, flexShrink: 0, padding: '5px 14px', borderRadius: '16px',
              fontFamily: FONT, fontSize: FONT_2XS, fontWeight: 600,
              backgroundColor: activeStore === null ? 'var(--brand)' : 'var(--bg)',
              color: activeStore === null ? '#fff' : 'var(--text-secondary)',
              border: activeStore === null ? 'none' : '1px solid var(--border)',
              transition: 'all 0.2s ease',
            }}
          >My List</button>
          {[...new Set(storeLists.map(l => l.store_name))].map(store => {
            const storeList = storeLists.find(l => l.store_name === store)
            const count = storeList?.item_count || 0
            return (
              <button
                key={store}
                onClick={async () => {
                  if (storeList) {
                    setActiveStore(store)
                    await handleLoadSavedList(storeList.id, 'merge')
                  }
                }}
                style={{
                  ...buttons.reset, flexShrink: 0, padding: '5px 14px', borderRadius: '16px',
                  fontFamily: FONT, fontSize: FONT_2XS, fontWeight: 600,
                  backgroundColor: activeStore === store ? 'var(--brand)' : 'var(--bg)',
                  color: activeStore === store ? '#fff' : 'var(--text-secondary)',
                  border: activeStore === store ? 'none' : '1px solid var(--border)',
                  transition: 'all 0.2s ease',
                }}
              >{store}{count > 0 ? ` (${count})` : ''}</button>
            )
          })}
        </div>
      )}

      {/* Offline indicator */}
      {isOffline && (
        <div style={{
          flexShrink: 0, padding: '6px 16px', backgroundColor: 'var(--warning-bg, #FEF3C7)',
          textAlign: 'center', fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--warning, #92400E)',
          fontWeight: 500,
        }}>
          {pendingToggleCount > 0
            ? `You're offline — ${pendingToggleCount} change${pendingToggleCount > 1 ? 's' : ''} will sync when you're back`
            : "You're offline — changes will sync when you're back"}
        </div>
      )}

      {/* One-time rebrand announcement (panel R7 — Nadia) */}
      {showRebrandBanner && screen === 'home' && (
        <div role="status" style={{
          ...layout.row, flexShrink: 0, padding: '10px 16px', gap: '12px',
          backgroundColor: 'color-mix(in srgb, var(--brand) 8%, transparent)',
          borderBottom: '1px solid var(--border)',
        }}>
          <span style={{ flex: 1, fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text)', lineHeight: 1.45 }}>
            <strong>Penny is now part of Groflo.</strong> Same Penny, same list, same household.
          </span>
          <button
            aria-label="Dismiss rebrand announcement"
            onClick={() => {
              setShowRebrandBanner(false)
              try { localStorage.setItem('groflo_rebrand_banner_dismissed', '1') } catch {}
              track('rebrand_banner_dismissed')
            }}
            style={{ ...buttons.reset, color: 'var(--text-secondary)', fontSize: FONT_SM, padding: '4px 8px' }}
          >×</button>
        </div>
      )}

      {/* PWA install prompt */}
      {showInstallBanner && (
        <div style={{
          ...layout.row, flexShrink: 0, padding: '8px 16px', gap: '12px',
          backgroundColor: 'var(--success-bg, #F0FFF4)', borderBottom: '1px solid var(--border)',
        }}>
          <span style={{ flex: 1, fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text)' }}>
            Add Groflo to your home screen for quick access
          </span>
          <button onClick={async () => {
            if (installPrompt?.prompt) {
              installPrompt.prompt()
              const result = await installPrompt.userChoice
              track('pwa_install_prompt', { outcome: result?.outcome })
            }
            setShowInstallBanner(false)
            setInstallPrompt(null)
          }} style={{
            ...buttons.reset, fontFamily: FONT, fontSize: FONT_2XS, fontWeight: 600,
            color: 'var(--brand)', padding: '4px 10px', backgroundColor: 'color-mix(in srgb, var(--brand) 10%, transparent)',
            borderRadius: '8px',
          }}>Install</button>
          <button onClick={() => {
            setShowInstallBanner(false)
            localStorage.setItem('groflo_install_dismissed', '1')
          }} style={{ ...buttons.reset, color: 'var(--text-secondary)', fontSize: FONT_SM, padding: '4px' }}>×</button>
        </div>
      )}

      {/* Restock nudges — 1-tap quick-add for items due for restock */}
      {restockNudges.length > 0 && items.filter(i => !i.checked).length < 20 && (
        <div style={{
          flexShrink: 0, padding: '8px 16px',
          borderBottom: '1px solid var(--border)', backgroundColor: 'var(--surface)',
        }}>
          <div style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', fontWeight: 600, marginBottom: '6px' }}>
            Time to restock
          </div>
          <div style={{
            display: 'flex', gap: '6px', flexWrap: 'wrap',
          }}>
            {restockNudges.map(nudge => (
              <button
                key={nudge.item_name}
                onClick={async () => {
                  const result = await fetchAPI('/api/list/parse', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ text: nudge.item_name }),
                  })
                  if (result) {
                    reloadList()
                    setRestockNudges(prev => prev.filter(n => n.item_name !== nudge.item_name))
                    showToast(`Added ${nudge.item_name}`)
                  }
                }}
                style={{
                  ...buttons.reset, padding: '4px 12px', borderRadius: '14px',
                  fontFamily: FONT, fontSize: FONT_2XS, fontWeight: 500,
                  backgroundColor: 'color-mix(in srgb, var(--brand) 8%, transparent)',
                  color: 'var(--brand)', border: '1px solid color-mix(in srgb, var(--brand) 20%, transparent)',
                  display: 'flex', alignItems: 'center', gap: '4px',
                }}
              >
                <span style={{ fontSize: '0.75rem' }}>+</span> {nudge.item_name}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Trip banner */}
      {activeTrips.length > 0 && (
        <div style={{ flexShrink: 0, padding: '8px 16px', backgroundColor: 'var(--surface)', borderBottom: '1px solid var(--border)' }}>
          {activeTrips.map(trip => (
            <div key={trip.id} style={{
              display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px',
              borderLeft: `3px solid ${trip.is_mine ? 'var(--brand)' : 'var(--border)'}`, borderRadius: '4px',
              backgroundColor: 'var(--bg)', marginBottom: activeTrips.indexOf(trip) < activeTrips.length - 1 ? '4px' : 0,
            }}>
              {isTripInProgress(trip) && <span style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: 'var(--success, #276749)', flexShrink: 0 }} />}
              <span style={{ flex: 1, fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text)' }}>
                <strong>{trip.is_mine ? 'You' : (trip.user_name || 'Someone')}</strong>
                {' — '}
                {isTripInProgress(trip) ? 'Shopping now!' : formatTripTime(trip)}
              </span>
              {trip.is_mine && (
                <div style={layout.rowGap}>
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
                        fontFamily: FONT, fontSize: FONT_2XS, fontWeight: 600, color: '#fff',
                        backgroundColor: 'var(--brand)', border: 'none', borderRadius: '12px',
                        padding: '4px 12px', cursor: 'pointer', whiteSpace: 'nowrap' as const,
                      }}
                    >Done</button>
                  )}
                  <button
                    onClick={handleCancelTrip}
                    aria-label="Cancel trip"
                    style={{ ...buttons.reset, padding: '4px', color: 'var(--text-secondary)', display: 'flex' }}
                  >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true"><path d="M1 1l12 12M13 1L1 13" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/></svg>
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Scrollable list — tap or scroll to dismiss keyboard */}
      <div id="main-list" ref={listAreaRef} role="main" aria-label="Shopping list" style={{
        ...listStyles.listArea,
        ...(pullDistance > 0 ? { transform: `translateY(${pullDistance}px)`, transition: isPulling.current ? 'none' : 'transform 0.2s ease' } : {}),
      }} onClick={e => {
        if (e.target === e.currentTarget || !(e.target as HTMLElement).closest('button, a, input, textarea')) {
          chatInputRef.current?.blur()
        }
      }} onScroll={() => {
        if (document.activeElement === chatInputRef.current) {
          chatInputRef.current?.blur()
        }
      }}>
        {/* Pull-to-refresh indicator */}
        {(pullDistance > 0 || pullRefreshing) && (
          <div style={{
            ...layout.rowCenter, position: 'absolute', top: -40, left: 0, right: 0,
            height: '40px', fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)',
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
                borderBottom: '1px solid var(--border)', backgroundColor: 'var(--surface)',
              }}>
                <div className="skeleton-shimmer" style={{
                  width: '22px', height: '22px', borderRadius: '50%',
                  backgroundColor: 'var(--skeleton)', marginRight: '12px', flexShrink: 0,
                }} />
                <div className="skeleton-shimmer" style={{
                  width: '44px', height: '44px', borderRadius: '8px',
                  backgroundColor: 'var(--skeleton)', marginRight: '10px', flexShrink: 0,
                }} />
                <div style={{ ...layout.column, flex: 1, gap: '6px' }}>
                  <div className="skeleton-shimmer" style={{
                    height: '14px', borderRadius: '4px', backgroundColor: 'var(--skeleton)',
                    width: `${50 + i * 10}%`,
                  }} />
                  <div className="skeleton-shimmer" style={{
                    height: '10px', borderRadius: '4px', backgroundColor: 'var(--skeleton)',
                    width: `${30 + i * 5}%`,
                  }} />
                </div>
              </div>
            ))}
          </div>
        ) : items.length === 0 && !chatLoading ? (
          <div style={listStyles.emptyState}>
            {!hasEverAddedItems && loginCount <= 1 ? (
              <>
                <PennyAvatar size={64} />
                <p style={{ ...listStyles.emptyText, marginTop: '14px', fontSize: FONT_XL, fontWeight: 600 }}>Hey! I'm Penny</p>
                <p style={listStyles.emptySubtext}>Type what you need — I'll organize it by aisle</p>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginTop: '16px', justifyContent: 'center', maxWidth: '320px' }}>
                  {['milk eggs bread', 'bananas chicken rice', 'coffee creamer butter'].map(example => (
                    <button key={example} onClick={() => { setChatText(example); chatInputRef.current?.focus() }} style={{
                      fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', backgroundColor: 'var(--bg)',
                      border: '1px solid var(--border)', borderRadius: '16px', padding: '6px 14px', cursor: 'pointer',
                    }}>{example}</button>
                  ))}
                </div>
                {settingsUserPhone && settingsPennyPhone && (
                  <p style={{ ...listStyles.emptySubtext, marginTop: '18px', fontSize: FONT_XS, opacity: 0.7 }}>
                    or text me at {settingsPennyPhone}
                  </p>
                )}
              </>
            ) : (
              <>
                <PennyAvatar size={56} />
                <p style={{ ...listStyles.emptyText, marginTop: '12px' }}>What can I add to your list?</p>
                <p style={listStyles.emptySubtext}>Type, paste a list, or snap a photo</p>
              </>
            )}
          </div>
        ) : (
          <div style={{ opacity: deptListOpacity, transition: 'opacity 0.15s ease-out' }}>
          {activeDepts.map((dept, deptIdx) => {
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
                  <span style={{ ...layout.rowCenter, width: '32px', flexShrink: 0, paddingLeft: '6px', boxSizing: 'border-box' }} aria-hidden="true">
                    {deptDone ? <span style={{ color: 'var(--brand)', fontSize: FONT_XS }}>✓</span> : <DeptIcon dept={dept} />}
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
                    onDelete={handleSwipeDelete}
                    onLongPress={handleUrgentToggle}
                    onSwapAccept={handleSwapAccept}
                    onSwapDecline={handleSwapDecline}
                  />
                ))}
              </div>
            )
          })}
          </div>
        )}
      </div>


      {/* Trip planner suggestion — once per session, 5+ unchecked items, no active trip, not for brand-new users */}
      {hasEverAddedItems && !tripSuggestionDismissed && activeTrips.length === 0 && items.filter(i => !i.checked).length >= 5 && (
        <div style={{
          flexShrink: 0, padding: '8px 16px', backgroundColor: 'var(--surface)',
          borderTop: '1px solid var(--border)',
          display: 'flex', alignItems: 'center', gap: '10px',
        }}>
          <span style={{ flex: 1, fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text)' }}>
            Ready to shop? <button
              onClick={() => { pushModalState(); setShowTripSheet(true) }}
              style={{
                background: 'none', border: 'none', color: 'var(--brand)', fontFamily: FONT,
                fontSize: FONT_SM, fontWeight: 600, cursor: 'pointer', padding: 0,
                textDecoration: 'underline', textUnderlineOffset: '2px',
              }}
            >Plan a trip</button>
          </span>
          <button
            onClick={() => { setTripSuggestionDismissed(true); sessionStorage.setItem('groflo_trip_dismissed', '1') }}
            aria-label="Dismiss trip suggestion"
            style={{
              background: 'none', border: 'none', cursor: 'pointer', padding: '4px',
              color: 'var(--text-secondary)', display: 'flex', minWidth: '44px', minHeight: '44px',
              alignItems: 'center', justifyContent: 'center',
            }}
          >
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
              <path d="M1 1l10 10M11 1L1 11" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
            </svg>
          </button>
        </div>
      )}

      {/* Confetti on list completion */}
      {showFireworks && <Confetti onComplete={() => setShowFireworks(false)} />}

      {/* Done Shopping Summary */}
      {showDoneSummary && doneSummaryData && (
        <DoneSummaryModal
          data={doneSummaryData}
          isFirstTrip={isFirstTrip}
          items={items}
  
          typePennyHome={typePennyHome}
          showToast={showToast}
          handleCancelTrip={handleCancelTrip}
          handleClearList={handleClearList}
          onClose={() => { setShowDoneSummary(false); setDoneSummaryData(null) }}
          onFinish={(firstTrip) => {
            setShowDoneSummary(false)
            setDoneSummaryData(null)
            if (firstTrip) setShowFireworks(true)
            typePennyHome('Trip complete — great job!')
          }}
          onSnapReceipt={() => {
            setPendingReceiptMode(true)
            receiptInputRef.current?.click()
          }}
          hasSwapsToClaim={earningsTotalCents > 0}
        />
      )}

      {/* "You're done!" overlay */}
      {showDoneOverlay && (() => {
        const tripSavings = items.reduce((s, i) => s + (i.swap_savings_cents || 0), 0)
        return (
          <div
            style={{
              position: 'fixed', inset: 0, zIndex: 9998,
              display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: '8px',
              backgroundColor: 'rgba(0,0,0,0.3)',
              animation: 'done-overlay 3s ease forwards',
              pointerEvents: 'none',
            }}
            onAnimationEnd={() => setShowDoneOverlay(false)}
          >
            <span style={{ fontSize: '2.25rem', fontWeight: 700, color: '#fff', fontFamily: FONT, textShadow: '0 2px 12px rgba(0,0,0,0.3)' }}>
              You're done! 🎉
            </span>
            {tripSavings > 0 && (
              <span style={{ fontSize: '1.125rem', fontWeight: 600, color: '#fff', fontFamily: FONT, textShadow: '0 2px 8px rgba(0,0,0,0.3)', opacity: 0.9 }}>
                Saved ${(tripSavings / 100).toFixed(2)} this trip
              </span>
            )}
          </div>
        )
      })()}

      {/* Floating "+N ItemName" drift animations */}
      {floatingLabels.map((label, i) => (
        <div key={label.key} style={{
          position: 'fixed', bottom: '100px', left: '50%', transform: 'translateX(-50%)',
          animation: 'float-up 3000ms cubic-bezier(0.2, 0.6, 0.3, 1) forwards',
          animationDelay: `${i * 600}ms`,
          fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, color: 'var(--brand)',
          backgroundColor: 'var(--surface)', padding: '6px 14px', borderRadius: '20px',
          boxShadow: '0 2px 8px rgba(0,0,0,0.12)', zIndex: 40,
          pointerEvents: 'none', opacity: 0,
        }} onAnimationEnd={() => setFloatingLabels(prev => prev.filter(l => l.key !== label.key))}>
          {label.text}
        </div>
      ))}


      {/* Camera mode confirmation overlay */}
      {cameraConfirmation && (
        <div style={{
          position: 'fixed', top: '50%', left: '50%', transform: 'translate(-50%, -50%)',
          zIndex: 10000, backgroundColor: 'rgba(0,0,0,0.8)', color: '#fff',
          padding: '16px 24px', borderRadius: '16px', maxWidth: '280px', textAlign: 'center',
          fontFamily: FONT, fontSize: FONT_MD, fontWeight: 600,
          animation: 'fade-in 200ms ease',
          pointerEvents: 'none',
        }}>
          {cameraConfirmation === 'Reading...' ? (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px' }}>
              <span style={{ fontSize: FONT_SM }}>...</span> Reading photo
            </div>
          ) : (
            <div>{cameraConfirmation}</div>
          )}
        </div>
      )}

      {/* Hidden file input for photo library */}
      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        multiple
        onChange={handlePhotoSelect}
        style={{ display: 'none' }}
        aria-label="Upload photos"
      />
      {/* Hidden file input for camera capture */}
      <input
        ref={cameraInputRef}
        type="file"
        accept="image/*"
        capture="environment"
        onChange={handlePhotoSelect}
        style={{ display: 'none' }}
        aria-label="Take photo"
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

      {/* Desktop webcam capture modal */}
      {showWebcam && (
        <WebcamCapture
          onCapture={(dataUrl) => {
            setShowWebcam(false)
            setPendingImages(prev => [...prev, dataUrl])
          }}
          onClose={() => setShowWebcam(false)}
        />
      )}

      {/* Post-swap share prompt — floating card after swap accept */}
      {postSwapShare && (
        <PostSwapShareCard
          brand={postSwapShare.brand}
          savings={postSwapShare.savings}
          showToast={showToast}
          onDismiss={() => setPostSwapShare(null)}
        />
      )}

      {/* Penny text + disambiguation + chat input */}
      <PennyBar
        pennyText={homePennyText}
        pennyTyping={homePennyTyping}
        buttons={homePennyBtns}
        toastError={toast?.type === 'error'}
        keyboardOpen={keyboardOpen}
        disambig={pennyDisambig.map(d => ({ product_category: d.product_category, current_brand: d.current_brand, alternatives: d.alternatives, auto_assigned: d.auto_assigned }))}
        disambigIndex={pennyDisambigIndex}
        onBrandPick={handleBrandPick}
        onSkipDisambig={advanceDisambig}
        pendingImages={pendingImages}
        onRemoveImage={(idx) => setPendingImages(prev => prev.filter((_, i) => i !== idx))}
        chatText={chatText}
        onChatTextChange={setChatText}
        chatLoading={chatLoading}
        onSend={handleSendChat}
        autoSubmitEnabled={!nameInputMode.current}
        placeholder={!hasEverAddedItems && loginCount <= 1 ? "Type items like 'milk, eggs, bread'..." : "Add items or ask Penny anything..."}
        onPaste={handlePaste}
        onAddPhoto={() => fileInputRef.current?.click()}
        onOpenCamera={() => {
          if (!isMobile) {
            // Desktop: open webcam modal
            setShowWebcam(true)
            return
          }
          // Mobile: use native camera input
          cameraModeRef.current = true
          cameraPhotoTakenRef.current = false
          cameraInputRef.current?.click()
          // Detect when user cancels the camera picker (window regains focus without a file change)
          const detectCancel = () => {
            setTimeout(() => {
              if (cameraModeRef.current && !cameraPhotoTakenRef.current) {
                cameraModeRef.current = false
              }
              window.removeEventListener('focus', detectCancel)
            }, 500)
          }
          window.addEventListener('focus', detectCancel)
        }}
        chatInputRef={chatInputRef}
      />

      {renderNavMenu()}

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
        <JoinPrompt
          name={joinPromptName}
          code={joinPromptCode}
          onJoined={(msg) => typePennyHome(msg)}
          onError={(msg) => showToast(msg, 'error')}
          onClose={() => setShowJoinPrompt(false)}
        />
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
            <input
              type="text"
              value={saveStoreInput}
              onChange={e => setSaveStoreInput(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter' && saveNameInput.trim()) handleSaveList() }}
              placeholder="Store (optional — e.g. Costco, Whole Foods)"
              style={{ ...listStyles.sheetTextInput, marginTop: '8px' }}
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

      {/* Onboarding walkthrough — new users with empty list */}
      {showOnboarding && screen === 'home' && items.filter(i => !i.checked).length === 0 && (
        <OnboardingOverlay onDismiss={() => { setShowOnboarding(false); localStorage.setItem('onboarding_complete', '1') }} />
      )}

      {/* Feedback FAB — approved phone numbers only, visible on every page */}
      {canSubmitFeedback && screen !== 'shared-list' && (
        <FeedbackWidget onSubmit={handleFeedbackSubmit} />
      )}
    </div>
    </ScreenTransition>
  )
}

export default function App() {
  // Embed mode — standalone, no-auth demo for iframe embedding (vibeflourishing pitch deck)
  const path = window.location.pathname
  if (path === '/embed' || path === '/embed/') {
    return (
      <ErrorBoundary>
        <Suspense fallback={null}>
          <EmbedView />
        </Suspense>
      </ErrorBoundary>
    )
  }

  return (
    <ErrorBoundary>
      <Suspense fallback={
        <div style={{ ...layout.rowCenter, height: '100dvh', backgroundColor: 'var(--bg)', fontFamily: FONT }}>
          <PennyAvatar size={40} />
        </div>
      }>
        <AppInner />
      </Suspense>
    </ErrorBoundary>
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
    backgroundColor: 'var(--bg)',
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
    fontSize: FONT_BASE,
    color: 'var(--error)',
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
    backgroundColor: 'var(--bg)',
    fontFamily: FONT,
    overflow: 'hidden',
  } as React.CSSProperties,

  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '16px 20px',
    backgroundColor: 'var(--surface)',
  } as React.CSSProperties,

  headerWordmark: {
    fontFamily: FONT,
    fontSize: FONT_XL,
    fontWeight: 700,
    color: 'var(--brand)',
    letterSpacing: '-0.5px',
  } as React.CSSProperties,

  // Menu styles moved to NavigationMenu component

  listArea: {
    flex: 1,
    overflowY: 'auto' as const,
    overscrollBehavior: 'none',
    WebkitOverflowScrolling: 'touch',
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
    fontSize: FONT_LG,
    fontWeight: 500,
    color: 'var(--text)',
  } as React.CSSProperties,

  emptySubtext: {
    fontFamily: FONT,
    fontSize: FONT_BASE,
    color: 'var(--text-secondary)',
  } as React.CSSProperties,

  deptHeader: (open: boolean, isFirst = false): React.CSSProperties => ({
    display: 'flex',
    alignItems: 'center',
    width: '100%',
    padding: '4px 12px 4px 0',
    marginTop: isFirst ? 0 : '1px',
    background: open ? 'color-mix(in srgb, var(--brand) 9%, transparent)' : 'var(--bg)',
    border: 'none',
    borderTop: isFirst ? 'none' : '1px solid var(--border)',
    borderBottom: '1px solid var(--border)',
    borderLeft: open ? '3px solid var(--brand)' : '3px solid transparent',
    cursor: 'pointer',
    textAlign: 'left' as const,
    gap: '0px',
    fontFamily: FONT,
    fontSize: '0.6875rem',
    fontWeight: 700,
    color: open ? 'var(--brand)' : 'var(--text-secondary)',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.6px',
  }),

  deptCount: {
    fontSize: '0.625rem',
    fontWeight: 600,
    color: 'var(--brand)',
    marginLeft: 'auto',
    backgroundColor: 'color-mix(in srgb, var(--brand) 8%, transparent)',
    borderRadius: '8px',
    padding: '1px 6px',
  } as React.CSSProperties,

  deptChevron: (open: boolean): React.CSSProperties => ({
    fontSize: FONT_LG,
    color: open ? 'var(--brand)' : 'var(--text-secondary)',
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
    backgroundColor: 'var(--surface)',
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
    fontSize: FONT_LG,
    fontWeight: 600,
    color: 'var(--text)',
  } as React.CSSProperties,

  sheetRow: {
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
  } as React.CSSProperties,

  sheetTextInput: {
    width: '100%',
    padding: '13px 16px',
    fontSize: FONT_MD,
    fontFamily: FONT,
    color: 'var(--text)',
    backgroundColor: 'var(--bg)',
    border: '1.5px solid var(--border)',
    borderRadius: '12px',
    outline: 'none',
    boxSizing: 'border-box' as const,
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

  floatingSelect: {
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
    fontSize: FONT_MD,
    fontFamily: FONT,
    fontWeight: 600,
    color: '#fff',
    backgroundColor: 'var(--brand)',
    border: 'none',
    borderRadius: '12px',
    cursor: 'pointer',
  } as React.CSSProperties,

  sheetDeleteBtn: {
    width: '100%',
    padding: '13px',
    fontSize: FONT_BASE,
    fontFamily: FONT,
    fontWeight: 500,
    color: 'var(--error)',
    backgroundColor: 'transparent',
    border: '1px solid color-mix(in srgb, var(--error) 25%, transparent)',
    borderRadius: '12px',
    cursor: 'pointer',
  } as React.CSSProperties,

  // Shared card container (used for toggle rows, settings cards)
  settingsCard: {
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
    padding: '12px 16px',
    backgroundColor: 'var(--bg)',
    borderRadius: '12px',
    cursor: 'pointer',
  } as React.CSSProperties,

  // Subtle text link (used for "wrong number?", "resend code", etc.)
  subtleLink: {
    fontFamily: FONT,
    fontSize: FONT_XS,
    color: 'var(--text-secondary)',
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    padding: 0,
    textDecoration: 'underline',
    textDecorationColor: 'color-mix(in srgb, var(--text-secondary) 25%, transparent)',
    textUnderlineOffset: '2px',
  } as React.CSSProperties,

  // Divider line
  divider: {
    height: '1px',
    backgroundColor: 'var(--border)',
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
    height: 100dvh;
    height: 100%;
    overflow: hidden;
    overscroll-behavior: none;
  }

  body {
    position: fixed;
    width: 100%;
    background-color: var(--bg);
    color: var(--text);
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
    -webkit-user-select: none;
    user-select: none;
    -webkit-touch-callout: none;
    touch-action: manipulation;
  }

  /* Allow text selection in inputs and textareas + fix iOS styling */
  input, textarea, [contenteditable] {
    -webkit-user-select: text;
    user-select: text;
  }

  /* Remove iOS default input styling (not selects — they need native picker) */
  input:not([type="checkbox"]):not([type="radio"]):not([type="file"]),
  textarea {
    -webkit-appearance: none;
    appearance: none;
  }

  /* Fix iOS autofill background flash */
  input:-webkit-autofill,
  input:-webkit-autofill:hover,
  input:-webkit-autofill:focus {
    -webkit-text-fill-color: var(--text);
    -webkit-box-shadow: 0 0 0px 1000px var(--surface) inset;
    transition: background-color 5000s ease-in-out 0s;
  }

  input:focus {
    border-color: var(--brand) !important;
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--brand) 9%, transparent);
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
    background-color: color-mix(in srgb, var(--brand) 19%, transparent);
  }

  /* Avatar upload spinner */
  @keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
  }

  @keyframes fade-in {
    from { opacity: 0; transform: translate(-50%, -50%) scale(0.9); }
    to { opacity: 1; transform: translate(-50%, -50%) scale(1); }
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
    background: var(--brand);
    border: 2px solid white;
    box-shadow: 0 1px 4px rgba(0,0,0,0.2);
    cursor: pointer;
  }
  input[type="range"]::-moz-range-thumb {
    width: 22px;
    height: 22px;
    border-radius: 50%;
    background: var(--brand);
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

  @keyframes feedbackSlideUp {
    from { transform: translateY(100%); }
    to { transform: translateY(0); }
  }

  @keyframes feedbackSlideDown {
    from { transform: translateY(-100%); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
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
    background: linear-gradient(90deg, var(--skeleton) 25%, var(--skeleton-shimmer) 50%, var(--skeleton) 75%);
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

  /* Dark mode overrides for inline-styled elements */
  [data-theme="dark"] body { background-color: var(--bg); color: var(--text); }
  [data-theme="dark"] input, [data-theme="dark"] textarea {
    background-color: var(--surface) !important;
    color: var(--text) !important;
    border-color: var(--border) !important;
  }
  [data-theme="dark"] input:focus {
    border-color: var(--brand) !important;
    box-shadow: 0 0 0 3px rgba(74, 148, 112, 0.25) !important;
  }

  button:focus-visible { outline: 2px solid var(--brand); outline-offset: 2px; }

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
styleEl.textContent = THEME_CSS + globalCSS
document.head.appendChild(styleEl)

// Initialize theme (light/dark) from user preference
initTheme()
