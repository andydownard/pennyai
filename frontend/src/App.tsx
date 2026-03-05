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

import { useState, useRef, useEffect, useCallback } from 'react'

// =============================================================================
// TYPES
// =============================================================================

// The screens the app can show
type Screen = 'phone' | 'code' | 'home' | 'saved' | 'settings' | 'about' | 'invite' | 'settings-profile' | 'settings-family' | 'settings-share' | 'admin'

// Declare gtag for TypeScript
declare function gtag(...args: unknown[]): void

type ListItem = {
  id: number
  name: string
  brand: string
  department: string
  quantity: number
  size: string
  source: string
  checked: boolean
  created_at: string
  added_by_name: string
  added_by_phone: string
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
  'Dairy & Eggs',
  'Meat & Seafood',
  'Bakery',
  'Frozen',
  'Pantry',
  'Beverages',
  'Household',
  'Other',
]

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
      border: size > 40 ? `2px solid ${BORDER}` : 'none',
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

// =============================================================================
// PENNY LOADING MESSAGES
// =============================================================================

const PENNY_STEP1 = ["Reading your list...", "Let me see what you wrote...", "Hmm, looking at this..."]
const PENNY_STEP2 = ["Checking it twice!", "Sorting by aisle...", "Ooh, good choices!", "I know just where these go..."]
const PENNY_STEP3 = ["Adding to your list...", "Almost done!", "Stocking up!"]
const PENNY_STEPS = [PENNY_STEP1, PENNY_STEP2, PENNY_STEP3]

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
    <p style={{ fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: TEXT_SECONDARY, textTransform: 'uppercase', letterSpacing: '0.5px', marginBottom: '12px' }}>{label}</p>
  )
}

function QuantityControl({ value, onChange }: {
  value: number
  onChange: (newValue: number) => void
}) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '0', border: `1.5px solid ${BORDER}`, borderRadius: '10px', overflow: 'hidden', backgroundColor: BG, flexShrink: 0 }}>
      <button
        onClick={() => onChange(Math.max(1, value - 1))}
        style={{ width: '34px', height: '34px', border: 'none', background: 'none', fontSize: '18px', color: BRAND, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
      >−</button>
      <span style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: TEXT, minWidth: '24px', textAlign: 'center' }}>{value}</span>
      <button
        onClick={() => onChange(value + 1)}
        style={{ width: '34px', height: '34px', border: 'none', background: 'none', fontSize: '18px', color: BRAND, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
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
          {message && <div style={{ fontSize: '13px', color: TEXT_SECONDARY, marginTop: '4px' }}>{message}</div>}
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
      <div style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY, marginTop: '12px', opacity: 0.45 }}>
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

export default function App() {
  // --- State ---
  const [screen, setScreenRaw] = useState<Screen>('phone')
  const setScreen = (s: Screen) => {
    setScreenRaw(s)
    if (['home', 'settings', 'about', 'invite', 'saved', 'settings-profile', 'settings-family', 'settings-share', 'admin'].includes(s)) {
      window.location.hash = s === 'home' ? '' : s
    }
    // Track page view in Google Analytics
    if (typeof gtag === 'function') {
      gtag('event', 'page_view', { page_title: s })
    }
  }
  const [phone, setPhone] = useState('')           // raw digits: "5551234567"
  const [code, setCode] = useState(['', '', '', '', '', '']) // 6 digits
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState(true)    // checking existing session on load

  // --- List state ---
  const [items, setItems] = useState<ListItem[]>([])
  const [chatText, setChatText] = useState('')
  const [chatLoading, setChatLoading] = useState(false)
  const [expandedDept, setExpandedDept] = useState<string | null>(null)
  const [selectedItem, setSelectedItem] = useState<ListItem | null>(null)
  const [editName, setEditName] = useState('')
  const [editBrand, setEditBrand] = useState('')
  const [editDept, setEditDept] = useState('')
  const [editQty, setEditQty] = useState(1)
  const [editSize, setEditSize] = useState('')
  const [editLoading, setEditLoading] = useState(false)
  const [itemHistory, setItemHistory] = useState<ItemModification[]>([])
  const deptRefs = useRef<Record<string, HTMLDivElement | null>>({})

  // --- Toast state ---
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // --- Review state (items from last parse) ---
  const [lastAdded, setLastAdded] = useState<ListItem[]>([])
  const [lastSkipped, setLastSkipped] = useState<{ parsed: { name: string; brand: string; quantity: number; size: string; department: string }; existing_id: number }[]>([])
  const [showReviewSheet, setShowReviewSheet] = useState(false)

  // --- Photo state ---
  const [photoBase64, setPhotoBase64] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  // --- Hamburger menu state ---
  const [menuOpen, setMenuOpen] = useState(false)

  // --- Done toast state ---
  const [showDoneToast, setShowDoneToast] = useState(false)

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
  const [settingsName, setSettingsName] = useState('')
  const [settingsEmail, setSettingsEmail] = useState('')
  const [settingsMembers, setSettingsMembers] = useState<{ name: string; phone: string }[]>([])
  const [settingsLoading, setSettingsLoading] = useState(false)
  const [inviteLoading, setInviteLoading] = useState(false)

  // --- Referral state ---
  const [referralCode, setReferralCode] = useState('')
  const [referralFriends, setReferralFriends] = useState<{ name: string; created_at: string }[]>([])

  // --- Admin state ---
  const [isAdmin, setIsAdmin] = useState(false)
  const [adminData, setAdminData] = useState<{
    active_users: number; total_users: number; new_users: number;
    k_factor: number; leaderboard: { phone: string; name: string; points: number; referrals: number }[];
    days: number;
  } | null>(null)
  const [adminDays, setAdminDays] = useState(30)

  // --- SMS opt-in state ---
  const [smsOptIn, setSmsOptIn] = useState(true)

  // --- Penny loading spinner state ---
  const [pennyStatus, setPennyStatus] = useState<string | null>(null)

  // --- Penny chime-in state ---
  const [pennyChimeIn, setPennyChimeIn] = useState<string | null>(null)
  const chimeInTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const prevCheckedRef = useRef(-1)

  const CHIME_IN_MESSAGES = [
    'Looking good!', 'Almost there!', 'Penny approves!', 'Great picks!',
    'Nice work!', 'On a roll!', 'Crushing it!', 'Keep going!',
  ]

  useEffect(() => {
    const checked = items.filter(i => i.checked).length
    if (checked > prevCheckedRef.current && prevCheckedRef.current >= 0 && items.length > 0) {
      // 30% chance Penny chimes in after checking off an item
      if (Math.random() < 0.3) {
        if (chimeInTimer.current) clearTimeout(chimeInTimer.current)
        setPennyChimeIn(pickRandom(CHIME_IN_MESSAGES))
        chimeInTimer.current = setTimeout(() => setPennyChimeIn(null), 4000)
      }
    }
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
          showToast('Joined family list!')
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
    fetch('/auth/me')
      .then(res => {
        if (res.ok) {
          res.clone().json().then((data: { is_admin?: boolean }) => {
            if (data.is_admin) setIsAdmin(true)
          }).catch(() => {})
          tryJoinCode()
          const hash = window.location.hash.replace('#', '') as Screen
          const validScreens: Screen[] = ['home', 'settings', 'about', 'invite', 'saved', 'settings-profile', 'settings-family', 'settings-share', 'admin']
          setScreen(validScreens.includes(hash) ? hash : 'home')
        }
      })
      .catch(() => {
        // Not logged in — that's fine, show the phone screen
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
  }, [screen, checking])

  // =========================================================================
  // LOAD LIST when on home screen
  // =========================================================================

  const sortItems = (arr: ListItem[]) =>
    [...arr].sort((a, b) => a.name.localeCompare(b.name))

  const handleSessionExpired = () => {
    setScreen('phone')
    setPhone('')
    setError('')
    showToast('Session expired — please log in again', 'error')
  }

  const reloadList = async () => {
    const res = await fetch('/api/list')
    if (res.status === 401) { handleSessionExpired(); return }
    if (res.ok) {
      const data = await res.json()
      setItems(sortItems(Array.isArray(data) ? data : []))
    }
  }

  useEffect(() => {
    if (screen !== 'home') return
    reloadList().catch(() => {})
  }, [screen])

  const loadSavedLists = () => {
    fetch('/api/saved-lists')
      .then(res => res.ok ? res.json() : [])
      .then(data => setSavedLists(Array.isArray(data) ? [...data].sort((a: SavedList, b: SavedList) => a.name.localeCompare(b.name)) : []))
      .catch(() => {})
  }

  useEffect(() => {
    if (screen !== 'saved') return
    loadSavedLists()
  }, [screen])

  const loadSettings = () => {
    fetch('/api/settings')
      .then(res => res.ok ? res.json() : null)
      .then(data => {
        if (data) {
          setSettingsName(data.name || '')
          setSettingsEmail(data.email || '')
          setSettingsMembers(data.household_members || [])
        }
      })
      .catch(() => {})
  }

  useEffect(() => {
    if (!['settings', 'about', 'invite', 'settings-profile', 'settings-family'].includes(screen)) return
    loadSettings()
  }, [screen])

  const loadReferralData = () => {
    fetch('/api/referral')
      .then(res => res.ok ? res.json() : null)
      .then(data => {
        if (data) {
          setReferralCode(data.code || '')
          setReferralFriends(data.friends || [])
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
    fetch(`/api/admin/dashboard?days=${adminDays}`)
      .then(res => res.ok ? res.json() : null)
      .then(data => { if (data) setAdminData(data) })
      .catch(() => {})
  }, [screen, adminDays])

  // Redirect legacy invite screen to settings-family
  useEffect(() => {
    if (screen === 'invite') setScreen('settings-family')
  }, [screen])

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    if (toastTimer.current) clearTimeout(toastTimer.current)
    setToast({ message, type })
    toastTimer.current = setTimeout(() => setToast(null), 5000)
  }

  const handlePhotoSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    // Resize large photos to max 1024px to keep request size manageable
    const img = new Image()
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
      setPhotoBase64(canvas.toDataURL('image/jpeg', 0.8))
    }
    img.src = URL.createObjectURL(file)

    // Reset file input so re-selecting same file triggers change
    e.target.value = ''
  }

  const handleToggle = async (id: number) => {
    setItems(prev => {
      const next = prev.map(item => item.id === id ? { ...item, checked: !item.checked } : item)
      // Check if all items are now checked
      if (next.length > 0 && next.every(i => i.checked)) {
        setTimeout(() => setShowDoneToast(true), 300)
      }
      return next
    })
    try {
      const res = await fetch(`/api/list/${id}/toggle`, { method: 'PATCH' })
      if (res.ok) {
        const updated: ListItem = await res.json()
        setItems(prev => {
          const next = prev.map(item => item.id === updated.id ? updated : item)
          if (next.length > 0 && next.every(i => i.checked)) {
            setTimeout(() => setShowDoneToast(true), 300)
          }
          return next
        })
      }
    } catch {}
  }

  const handleSendChat = async () => {
    if ((!chatText.trim() && !photoBase64) || chatLoading) return
    setChatLoading(true)
    setLastAdded([])
    setLastSkipped([])
    try {
      const payload = JSON.stringify({ text: chatText.trim(), image: photoBase64 || '' })
      console.log('[penny] sending parse request, payload size:', payload.length)
      const res = await fetch('/api/list/parse', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload,
      })
      console.log('[penny] parse response status:', res.status)
      if (res.ok) {
        const data = await res.json()
        const addedItems: ListItem[] = data.items || []
        const skippedItems = data.skipped || []
        // Merge new items into list
        setItems(prev => {
          const map = new Map(prev.map(i => [i.id, i]))
          for (const item of addedItems) {
            map.set(item.id, item)
          }
          return sortItems(Array.from(map.values()))
        })
        setLastAdded(addedItems)
        setLastSkipped(skippedItems)
        setChatText('')
        setPhotoBase64(null)
        // Show toast
        const parts: string[] = []
        if (data.added > 0) parts.push(`Added ${data.added} item${data.added === 1 ? '' : 's'}!`)
        if (skippedItems.length > 0) parts.push(`${skippedItems.length} already on your list`)
        if (parts.length > 0) showToast(parts.join(' · '))
        else if (addedItems.length === 0 && skippedItems.length === 0) showToast('No items found', 'error')
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

  const openDetail = (item: ListItem) => {
    setSelectedItem(item)
    setEditName(item.name)
    setEditBrand(item.brand)
    setEditDept(item.department)
    setEditQty(item.quantity || 1)
    setEditSize(item.size)
    setItemHistory([])
    fetch(`/api/list/${item.id}/history`)
      .then(res => res.ok ? res.json() : [])
      .then(data => setItemHistory(Array.isArray(data) ? data : []))
      .catch(() => {})
  }

  const handleEditSave = async () => {
    if (!selectedItem || !editName.trim() || editLoading) return
    setEditLoading(true)
    try {
      const res = await fetch(`/api/list/${selectedItem.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: editName.trim(),
          brand: editBrand.trim(),
          department: editDept,
          quantity: editQty,
          size: editSize.trim(),
        }),
      })
      if (res.ok) {
        const updated: ListItem = await res.json()
        setItems(prev => sortItems(prev.map(item => item.id === updated.id ? updated : item)))
        setSelectedItem(null)
      }
    } catch {}
    finally { setEditLoading(false) }
  }

  const handleEditDelete = async () => {
    if (!selectedItem || editLoading) return
    setEditLoading(true)
    try {
      const res = await fetch(`/api/list/${selectedItem.id}`, { method: 'DELETE' })
      if (res.ok) {
        setItems(prev => prev.filter(item => item.id !== selectedItem.id))
        setSelectedItem(null)
      }
    } catch {}
    finally { setEditLoading(false) }
  }

  const handleClearList = () => {
    fetch('/api/list/clear', { method: 'POST' })
      .then(res => {
        if (res.status === 401) { handleSessionExpired(); return }
        if (!res.ok) { showToast('Failed to clear list', 'error'); return }
        setItems([])
        setShowDoneToast(false)
        setPennyChimeIn(null)
        showToast('List cleared')
      })
      .catch(() => showToast('Failed to clear list', 'error'))
  }

  const handleSaveList = async () => {
    if (!saveNameInput.trim()) return
    try {
      const res = await fetch('/api/saved-lists', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: saveNameInput.trim() }),
      })
      if (res.ok) {
        setShowSaveSheet(false)
        setSaveNameInput('')
        showToast('List saved!')
      } else {
        const data = await res.json().catch(() => null)
        showToast(data?.error || 'Failed to save list', 'error')
      }
    } catch {
      showToast('Failed to save list', 'error')
    }
  }

  const handleLoadSavedList = (id: number, mode: 'merge' | 'overwrite' = 'merge') => {
    const url = mode === 'overwrite'
      ? `/api/saved-lists/${id}/load?mode=overwrite`
      : `/api/saved-lists/${id}/load`
    setSelectedSavedList(null)
    fetch(url, { method: 'POST' })
      .then(res => {
        if (res.status === 401) { handleSessionExpired(); return }
        if (!res.ok) { showToast('Failed to load list', 'error'); return }
        setScreen('home')
        showToast(mode === 'overwrite' ? 'List replaced!' : 'List merged!')
      })
      .catch(() => showToast('Failed to load list', 'error'))
  }

  const handleDeleteSavedList = async (id: number) => {
    try {
      const res = await fetch(`/api/saved-lists/${id}`, { method: 'DELETE' })
      if (res.ok) {
        setSelectedSavedList(null)
        loadSavedLists()
        showToast('List deleted')
      }
    } catch {
      showToast('Failed to delete list', 'error')
    }
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
        tryJoinCode()
        setScreen('home')
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
      const res = await fetch('/api/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: settingsName.trim(), email: settingsEmail.trim() }),
      })
      if (res.ok) {
        showToast('Settings saved!')
      } else {
        const data = await res.json().catch(() => null)
        showToast(data?.error || 'Failed to save', 'error')
      }
    } catch {
      showToast('Network error', 'error')
    } finally {
      setSettingsLoading(false)
    }
  }

  const withInviteLink = async (onLink: (link: string) => void) => {
    if (inviteLoading) return
    setInviteLoading(true)
    try {
      const res = await fetch('/api/settings/invite', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ phone: '' }),
      })
      if (res.ok) {
        const data = await res.json()
        if (data.link) onLink(data.link)
      } else {
        const data = await res.json().catch(() => null)
        showToast(data?.error || 'Failed to create invite', 'error')
      }
    } catch {
      showToast('Network error', 'error')
    } finally {
      setInviteLoading(false)
    }
  }

  const handleTextInviteLink = () => withInviteLink(link => {
    const message = `Join our family shopping list on Thriftly! ${link}`
    window.location.href = `sms:?&body=${encodeURIComponent(message)}`
  })

  // =========================================================================
  // LOGOUT
  // =========================================================================

  const handleLogout = async () => {
    await fetch('/auth/logout', { method: 'POST' })
    setMenuOpen(false)
    setScreen('phone')
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
          <p style={styles.tagline}>
            Penny AI accepts texts, emails, and photos, and organizes a clean, simple list
          </p>

          <div style={styles.inputGroup}>
            <label style={styles.label}>Phone number</label>
            <input
              ref={phoneInputRef}
              type="tel"
              inputMode="numeric"
              autoComplete="tel-national"
              placeholder="(555) 123-4567"
              value={formatPhone(phone)}
              onChange={handlePhoneChange}
              onKeyDown={e => { if (e.key === 'Enter' && isValid) handleSendCode() }}
              style={styles.phoneInput}
              autoFocus
            />
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
            <button style={listStyles.menuItem(active === 'home')} onClick={() => { setMenuOpen(false); setScreen('home') }}>Shopping List</button>
            <button style={listStyles.menuItem(active === 'saved')} onClick={() => { setMenuOpen(false); setScreen('saved') }}>Saved Lists</button>
            <button style={listStyles.menuItem(active === 'about')} onClick={() => { setMenuOpen(false); setScreen('about') }}>How to text Penny</button>
            <button style={listStyles.menuItem(active === 'settings-share')} onClick={() => { setMenuOpen(false); setScreen('settings-share') }}>Share & Earn</button>
            <button style={listStyles.menuItem(active === 'settings' || active === 'settings-profile' as Screen || active === 'settings-family' as Screen)} onClick={() => { setMenuOpen(false); setScreen('settings') }}>Settings</button>
            {isAdmin && (
              <button style={listStyles.menuItem(active === 'admin')} onClick={() => { setMenuOpen(false); setScreen('admin') }}>Dashboard</button>
            )}
            <div style={{ height: '1px', backgroundColor: BORDER, margin: '4px 0' }} />
            <button style={listStyles.menuItem(false)} onClick={handleLogout}>Sign Out</button>
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
      <div style={{ width: '32px', height: '32px', borderRadius: '50%', backgroundColor: BRAND, color: WHITE, display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: FONT, fontSize: '14px', fontWeight: 600, flexShrink: 0 }}>
        {(m.name || m.phone || '?')[0].toUpperCase()}
      </div>
      <div>
        <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT }}>
          {m.name || formatPhone(m.phone)}
        </div>
        {m.name && <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY }}>{formatPhone(m.phone)}</div>}
      </div>
    </div>
  ))

  const renderToast = () => toast ? (
    <div style={{
      position: 'fixed',
      top: '72px',
      left: '16px',
      right: '16px',
      display: 'flex',
      alignItems: 'center',
      gap: '10px',
      backgroundColor: toast.type === 'error' ? '#FEE2E2' : WHITE,
      color: toast.type === 'error' ? '#991B1B' : TEXT,
      padding: '12px 16px',
      borderRadius: '16px',
      fontFamily: FONT,
      fontSize: '15px',
      fontWeight: 500,
      zIndex: 50,
      boxShadow: '0 4px 16px rgba(0,0,0,0.18)',
      border: `1px solid ${toast.type === 'error' ? '#FECACA' : BORDER}`,
    }}>
      <PennyAvatar size={32} />
      {toast.message}
    </div>
  ) : null

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
        }}>{pageName}</span>
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
          onClick={() => setScreen(backScreen)}
          style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '4px 8px 4px 0', color: BRAND, fontSize: '20px', fontWeight: 600, display: 'flex', alignItems: 'center' }}
        >
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
            <path d="M13 4l-6 6 6 6" stroke={BRAND} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
        </button>
        <span style={{ fontFamily: FONT, fontSize: '17px', fontWeight: 600, color: TEXT }}>{title}</span>
        <div style={{ marginLeft: 'auto' }}>{renderMenu(backScreen)}</div>
      </div>
    </div>
  )

  // =========================================================================
  // SETTINGS HUB
  // =========================================================================

  if (screen === 'settings') {
    const settingsRows: { label: string; target: Screen | 'logout' }[] = [
      { label: 'Profile', target: 'settings-profile' },
      { label: 'Family', target: 'settings-family' },
    ]
    return (
      <div style={listStyles.page}>
        {renderPageHeader('Settings', 'settings')}
        <div style={listStyles.listArea}>
          {settingsRows.map(row => (
            <button
              key={row.label}
              onClick={() => setScreen(row.target as Screen)}
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
              <span style={{ flex: 1, fontFamily: FONT, fontSize: '16px', fontWeight: 500, color: TEXT }}>{row.label}</span>
              <span style={{ color: TEXT_SECONDARY, fontSize: '18px' }}>›</span>
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
        {renderToast()}
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
          <div style={{ padding: '20px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <div>
              <label style={listStyles.fieldLabel}>Name</label>
              <input
                type="text"
                value={settingsName}
                onChange={e => setSettingsName(e.target.value)}
                placeholder="Your name"
                style={listStyles.sheetTextInput}
              />
            </div>
            <div>
              <label style={listStyles.fieldLabel}>Email</label>
              <input
                type="email"
                value={settingsEmail}
                onChange={e => setSettingsEmail(e.target.value)}
                placeholder="your@email.com"
                style={listStyles.sheetTextInput}
              />
              <p style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '4px', paddingLeft: '4px' }}>
                Set your email to add items by emailing penny@list.thrift.ly
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
        {renderToast()}
      </div>
    )
  }

  // =========================================================================
  // SETTINGS — FAMILY
  // =========================================================================

  if (screen === 'settings-family') {
    return (
      <div style={listStyles.page}>
        <ConfirmModal open={!!confirmModal} title={confirmModal?.title || ''} message={confirmModal?.message} confirmLabel={confirmModal?.confirmLabel} destructive={confirmModal?.destructive} onConfirm={runConfirmAction} onCancel={closeConfirmModal} />
        {renderSubpageHeader('Family', 'settings')}
        <div style={listStyles.listArea}>
          <div style={{ padding: '20px', display: 'flex', flexDirection: 'column', gap: '20px' }}>
            {settingsMembers.length > 0 && (
              <div>
                <SectionHeader label="Family Members" />
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
                    fontSize: '14px',
                    fontWeight: 500,
                    color: ERROR_RED,
                    backgroundColor: 'transparent',
                    border: `1px solid ${ERROR_RED}`,
                    borderRadius: '10px',
                    cursor: 'pointer',
                    width: '100%',
                  }}
                >Leave Household</button>
              </div>
            )}

            <div>
              <SectionHeader label="Invite" />
              <p style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY, margin: '0 0 12px 0' }}>
                Invite family members to share your shopping list.
              </p>
              <button
                onClick={handleTextInviteLink}
                disabled={inviteLoading}
                style={{
                  ...listStyles.sheetButton,
                  opacity: inviteLoading ? 0.5 : 1,
                }}
              >
                {inviteLoading ? 'Creating...' : 'Text Invite Link'}
              </button>
            </div>
          </div>
        </div>
        {renderToast()}
      </div>
    )
  }

  // =========================================================================
  // SETTINGS — SHARE & EARN
  // =========================================================================

  if (screen === 'settings-share') {
    const handleShare = () => {
      const url = `https://app.thrift.ly/r/${referralCode}`
      const text = `Try Thriftly — Penny AI helps your family organize the grocery list. Just text, email, or snap a photo and she handles the rest! ${url}`
      if (navigator.share) {
        navigator.share({ title: 'Thriftly', text }).catch(() => {})
      } else {
        window.open(`sms:?&body=${encodeURIComponent(text)}`, '_self')
      }
    }


    return (
      <div style={listStyles.page}>
        {renderSubpageHeader('Share & Earn', 'home')}
        <div style={listStyles.listArea}>
          <div style={{ padding: '24px 20px', display: 'flex', flexDirection: 'column', gap: '20px' }}>
            {/* Share info */}
            <div style={{ backgroundColor: BG, borderRadius: '12px', padding: '16px', textAlign: 'center' }}>
              <p style={{ fontFamily: FONT, fontSize: '15px', color: TEXT, margin: 0, lineHeight: 1.6 }}>
                Share Thriftly with friends and family. The more people who use Penny, the better deals everyone gets.
              </p>
            </div>

            {/* Share button */}
            <button onClick={handleShare} style={listStyles.sheetButton}>
              Share Your Link
            </button>

            {/* Referral count */}
            {referralFriends.length > 0 && (
              <div style={{ backgroundColor: BG, borderRadius: '12px', padding: '12px 16px', textAlign: 'center' }}>
                <span style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: BRAND }}>{referralFriends.length}</span>
                <span style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, marginLeft: '8px' }}>{referralFriends.length === 1 ? 'friend joined' : 'friends joined'}</span>
              </div>
            )}

            {/* Brand rebates CTA */}
            <div style={{ backgroundColor: BG, borderRadius: '12px', padding: '16px' }}>
              <p style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: TEXT, margin: '0 0 8px 0' }}>
                Exclusive Rebates — Coming Soon
              </p>
              <p style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, margin: '0 0 12px 0', lineHeight: 1.5 }}>
                We're partnering with brands to offer exclusive rebates on the things you already buy. No points, no gimmicks — real savings on real groceries.
              </p>
              <p style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, margin: '0 0 8px 0', lineHeight: 1.5 }}>
                Are you a product manager at a CPG brand? Get your products in front of engaged shoppers.
              </p>
              <a
                href="mailto:partners@thrift.ly?subject=Create%20a%20brand%20deal"
                style={{ fontFamily: FONT, fontSize: '13px', color: BRAND, textDecoration: 'underline', textUnderlineOffset: '2px' }}
              >
                Create a brand deal
              </a>
            </div>

            {/* Friends list */}
            {referralFriends.length > 0 && (
              <div>
                <SectionHeader label={`Friends (${referralFriends.length})`} />
                {referralFriends.map((f, i) => (
                  <div key={i} style={{
                    display: 'flex',
                    alignItems: 'center',
                    padding: '10px 0',
                    borderBottom: `1px solid ${BORDER}`,
                    gap: '10px',
                  }}>
                    <div style={{ width: '32px', height: '32px', borderRadius: '50%', backgroundColor: BRAND, color: WHITE, display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: FONT, fontSize: '14px', fontWeight: 600, flexShrink: 0 }}>
                      {(f.name || '?')[0].toUpperCase()}
                    </div>
                    <div style={{ flex: 1 }}>
                      <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT }}>{f.name || 'Friend'}</div>
                      <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY }}>{new Date(f.created_at).toLocaleDateString()}</div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
        {renderToast()}
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

        <div style={listStyles.listArea}>
          <div style={{ padding: '30px 20px', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '24px' }}>
            <PennyAvatar size={96} />

            {/* How it works */}
            <div style={{ width: '100%' }}>
              <ol style={{ fontFamily: FONT, fontSize: '15px', color: TEXT, margin: 0, paddingLeft: '24px', lineHeight: '1.8' }}>
                <li style={{ marginBottom: '8px' }}><strong>Add Penny to your contacts</strong> using the button below</li>
                <li style={{ marginBottom: '8px' }}><strong>Text her what you need</strong> — a grocery list, a photo of your fridge, pantry, or a recipe</li>
                <li><strong>She adds it to your list</strong> — works in group chats with your family too</li>
              </ol>
            </div>

            <p style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, textAlign: 'center', margin: 0, lineHeight: '1.5' }}>
              Penny's shy — she won't text back yet. But don't worry, your items will appear on your list!
            </p>

            <a
              href="/api/penny.vcf"
              style={{
                display: 'block',
                width: '100%',
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

            {/* Coming Soon */}
            <div style={{ width: '100%', backgroundColor: BG, borderRadius: '12px', padding: '16px 20px' }}>
              <p style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: TEXT, margin: '0 0 10px 0' }}>
                Coming Soon
              </p>
              <ul style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, margin: 0, paddingLeft: '20px', lineHeight: '1.8' }}>
                <li>Best deals near you</li>
                <li>Best places to shop</li>
                <li>Delivery options</li>
              </ul>
              <a
                href="mailto:support@thrift.ly?subject=Feature%20request"
                style={{ fontFamily: FONT, fontSize: '13px', color: BRAND, marginTop: '12px', display: 'inline-block', textDecoration: 'underline', textUnderlineOffset: '2px' }}
              >
                Request a feature
              </a>
            </div>
          </div>
        </div>

        {renderToast()}
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
        {renderPageHeader('Saved Lists', 'saved')}

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
                onClick={() => setSelectedSavedList(sl)}
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
                  <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, marginTop: '2px' }}>
                    {sl.item_count} item{sl.item_count !== 1 ? 's' : ''} · {new Date(sl.created_at).toLocaleDateString()}
                  </div>
                </div>
                <span style={{ color: TEXT_SECONDARY, fontSize: '18px' }}>›</span>
              </button>
            ))
          )}
        </div>

        {renderToast()}

        {/* Saved list action sheet */}
        {selectedSavedList && (
          <ActionSheet isOpen={true} onClose={() => setSelectedSavedList(null)} title={selectedSavedList.name}>
              <p style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, margin: '0 0 8px 0' }}>
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
        <div style={{ ...listStyles.listArea, padding: '20px 16px' }}>
          {/* Time period selector */}
          <div style={{ display: 'flex', gap: '8px', marginBottom: '20px' }}>
            {dayOptions.map(d => (
              <button key={d} onClick={() => setAdminDays(d)} style={{
                flex: 1, padding: '8px', borderRadius: '8px', border: `1px solid ${BORDER}`,
                backgroundColor: adminDays === d ? BRAND : WHITE, color: adminDays === d ? WHITE : TEXT,
                fontFamily: FONT, fontSize: '14px', fontWeight: 600, cursor: 'pointer',
              }}>{d}d</button>
            ))}
          </div>

          {adminData ? (
            <>
              {/* Metric cards */}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '24px' }}>
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
                    <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, marginBottom: '4px' }}>{m.label}</div>
                    <div style={{ fontFamily: FONT, fontSize: '28px', fontWeight: 700, color: TEXT }}>{m.value}</div>
                  </div>
                ))}
              </div>

              {/* Leaderboard */}
              <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginBottom: '12px' }}>Referral Leaderboard</div>
              {adminData.leaderboard.length === 0 ? (
                <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY }}>No referrals yet.</div>
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
                      {entry.name && <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY }}>{entry.phone}</div>}
                    </div>
                    <div style={{ textAlign: 'right' }}>
                      <div style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: BRAND }}>{entry.points} pts</div>
                      <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY }}>{entry.referrals} referrals</div>
                    </div>
                  </div>
                ))
              )}
            </>
          ) : (
            <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, textAlign: 'center', padding: '40px 0' }}>Loading...</div>
          )}
        </div>
        {renderToast()}
      </div>
    )
  }

  // =========================================================================
  // HOME SCREEN — Shopping list
  // =========================================================================

  const handleDeptClick = (dept: string) => {
    if (expandedDept === dept) {
      setExpandedDept(null)
      return
    }
    setExpandedDept(dept)
    setTimeout(() => {
      deptRefs.current[dept]?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }, 0)
  }

  // Departments that have items, in store-aisle order
  const activeDepts = DEPARTMENTS.filter(d => items.some(i => i.department === d))

  return (
    <div style={listStyles.page}>
      <ConfirmModal open={!!confirmModal} title={confirmModal?.title || ''} message={confirmModal?.message} confirmLabel={confirmModal?.confirmLabel} destructive={confirmModal?.destructive} onConfirm={runConfirmAction} onCancel={closeConfirmModal} />
      {/* Fixed header — compact */}
      <div style={{ flexShrink: 0, backgroundColor: WHITE, borderBottom: `1px solid ${BORDER}` }}>
        <div style={{ display: 'flex', alignItems: 'center', padding: '10px 16px', position: 'relative' }}>
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
          }}>Shopping List</span>
          <div style={{ marginLeft: 'auto' }}>{renderMenu('home')}</div>
        </div>
        {/* Progress bar */}
        {items.length > 0 && (() => {
          const pct = Math.round((items.filter(i => i.checked).length / items.length) * 100)
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

      {/* Scrollable list */}
      <div style={listStyles.listArea}>
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
          activeDepts.map(dept => {
            const deptItems = items.filter(i => i.department === dept)
            const deptChecked = deptItems.filter(i => i.checked).length
            const deptDone = deptChecked === deptItems.length
            const isOpen = expandedDept === dept
            return (
              <div key={dept} ref={el => { deptRefs.current[dept] = el }}>
                <button
                  style={listStyles.deptHeader(isOpen)}
                  onClick={() => handleDeptClick(dept)}
                >
                  {deptDone && <span style={{ color: BRAND, fontSize: '14px', marginRight: '2px' }}>✓</span>}
                  <span style={deptDone ? { textDecoration: 'line-through', opacity: 0.5 } : undefined}>{dept}</span>
                  <span style={listStyles.deptCount}>{deptChecked}/{deptItems.length}</span>
                  <span style={listStyles.deptChevron(isOpen)}>›</span>
                </button>
                {isOpen && deptItems.map(item => (
                  <div key={item.id} style={listStyles.itemRow}>
                    <button onClick={() => handleToggle(item.id)} style={listStyles.itemCheckBtn}>
                      <span style={listStyles.itemCheck(item.checked)}>
                        {item.checked && '✓'}
                      </span>
                    </button>
                    <div style={listStyles.itemCenter} onClick={() => handleToggle(item.id)}>
                      <span style={listStyles.itemName(item.checked)}>
                        {item.brand ? `${item.brand} ` : ''}{item.name}
                      </span>
                      {(item.quantity > 1 || item.size) && (
                        <span style={listStyles.itemMeta}>
                          {item.quantity > 1 ? `×${item.quantity}` : ''}{item.quantity > 1 && item.size ? ' · ' : ''}{item.size}
                        </span>
                      )}
                    </div>
                    <button onClick={() => openDetail(item)} style={listStyles.itemEditBtn}>
                      <span style={{ width: '32px', height: '32px', borderRadius: '50%', backgroundColor: BG, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                          <path d="M11.5 1.5a1.77 1.77 0 0 1 2.5 0 1.77 1.77 0 0 1 0 2.5L5.5 12.5 1.5 14l1.5-4L11.5 1.5z" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"/>
                          <path d="M10 3.5l2.5 2.5" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"/>
                        </svg>
                      </span>
                    </button>
                  </div>
                ))}
              </div>
            )
          })
        )}
      </div>


      {/* "All done" toast — rendered inline below in the penny status bar area */}

      {/* Save/Clear links */}
      {items.length > 0 && (
        <div style={{
          flexShrink: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: '16px',
          padding: '10px 16px',
          borderTop: `1px solid ${BORDER}`,
        }}>
          <button
            onClick={() => { setSaveNameInput(''); setShowSaveSheet(true) }}
            style={{
              fontFamily: FONT,
              fontSize: '14px',
              fontWeight: 500,
              color: TEXT_SECONDARY,
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              textDecoration: 'underline',
              textUnderlineOffset: '2px',
            }}
          >Save List</button>
          <button
            onClick={() => openConfirmModal({
              title: 'Clear List',
              message: 'Remove all items from your list?',
              confirmLabel: 'Clear',
              destructive: true,
              onConfirm: handleClearList,
            })}
            style={{
              fontFamily: FONT,
              fontSize: '14px',
              fontWeight: 500,
              color: TEXT_SECONDARY,
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              textDecoration: 'underline',
              textUnderlineOffset: '2px',
            }}
          >Clear List</button>
        </div>
      )}

      {/* Hidden file input for camera/photo */}
      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        onChange={handlePhotoSelect}
        style={{ display: 'none' }}
      />

      {/* Unified Penny bar — chime-in, loading, result, or "all done" */}
      {(() => {
        const allDone = showDoneToast && items.length > 0 && items.every(i => i.checked)
        const showBar = pennyStatus || toast || allDone || pennyChimeIn
        if (!showBar) return null

        const isDone = allDone && !pennyStatus && !toast && !pennyChimeIn
        const isChimeIn = !!pennyChimeIn && !pennyStatus && !toast && !isDone
        const bgColor = isDone ? BRAND : isChimeIn ? `${BRAND}12` : toast?.type === 'error' ? '#FEE2E2' : '#F5F3EF'
        const textColor = isDone ? WHITE : isChimeIn ? BRAND : toast?.type === 'error' ? '#991B1B' : TEXT

        return (
          <div style={{
            flexShrink: 0,
            display: 'flex',
            alignItems: 'center',
            gap: isChimeIn ? '8px' : '10px',
            padding: isChimeIn ? '8px 16px' : '10px 16px',
            backgroundColor: bgColor,
            borderTop: `1px solid ${isDone ? BRAND : isChimeIn ? 'transparent' : BORDER}`,
            fontFamily: FONT,
            fontSize: '14px',
            color: textColor,
            fontWeight: 500,
          }}>
            <PennyAvatar size={isChimeIn ? 22 : 45} />
            {isDone ? (
              <>
                <div style={{ flex: 1 }}>
                  <div style={{ marginBottom: '6px' }}>You're done!</div>
                  <div style={{ display: 'flex', gap: '12px' }}>
                    <button
                      onClick={() => {
                        setShowDoneToast(false)
                        setSaveNameInput('')
                        setShowSaveSheet(true)
                      }}
                      style={{
                        background: 'rgba(255,255,255,0.2)',
                        border: 'none',
                        color: WHITE,
                        fontFamily: FONT,
                        fontSize: '13px',
                        fontWeight: 600,
                        cursor: 'pointer',
                        padding: '5px 12px',
                        borderRadius: '12px',
                      }}
                    >Save List</button>
                    <button
                      onClick={() => {
                        setShowDoneToast(false)
                        handleClearList()
                      }}
                      style={{
                        background: 'rgba(255,255,255,0.2)',
                        border: 'none',
                        color: WHITE,
                        fontFamily: FONT,
                        fontSize: '13px',
                        fontWeight: 600,
                        cursor: 'pointer',
                        padding: '5px 12px',
                        borderRadius: '12px',
                      }}
                    >Clear List</button>
                  </div>
                </div>
                <button
                  onClick={() => setShowDoneToast(false)}
                  style={{
                    background: 'none',
                    border: 'none',
                    color: WHITE,
                    fontFamily: FONT,
                    fontSize: '16px',
                    cursor: 'pointer',
                    padding: '0',
                    opacity: 0.6,
                    alignSelf: 'flex-start',
                  }}
                >×</button>
              </>
            ) : isChimeIn ? (
              <span style={{ flex: 1, fontSize: '13px' }}>{pennyChimeIn}</span>
            ) : (
              <>
                <span style={{ flex: 1 }}>{pennyStatus || toast?.message}</span>
                {pennyStatus && (
                  <span className="penny-pulse" style={{ width: '6px', height: '6px', borderRadius: '50%', backgroundColor: BRAND, flexShrink: 0 }} />
                )}
                {!pennyStatus && toast && (
                  <>
                    {(lastAdded.length > 0 || lastSkipped.length > 0) && toast.type !== 'error' && (
                      <button
                        onClick={() => {
                          setShowReviewSheet(true)
                          setToast(null)
                          if (toastTimer.current) clearTimeout(toastTimer.current)
                        }}
                        style={{
                          background: BRAND,
                          border: 'none',
                          color: WHITE,
                          fontFamily: FONT,
                          fontSize: '13px',
                          fontWeight: 600,
                          cursor: 'pointer',
                          padding: '4px 12px',
                          borderRadius: '12px',
                          flexShrink: 0,
                        }}
                      >Review</button>
                    )}
                    <button
                      onClick={() => {
                        setToast(null)
                        if (toastTimer.current) clearTimeout(toastTimer.current)
                      }}
                      style={{
                        background: 'none',
                        border: 'none',
                        color: isDone ? WHITE : TEXT_SECONDARY,
                        fontFamily: FONT,
                        fontSize: '18px',
                        cursor: 'pointer',
                        padding: '0 2px',
                        lineHeight: 1,
                        flexShrink: 0,
                      }}
                    >×</button>
                  </>
                )}
              </>
            )}
          </div>
        )
      })()}

      {/* Bottom chat bar — flex item, anchors above keyboard via 100dvh */}
      <div style={listStyles.chatBar}>
        {photoBase64 ? (
          <div style={{ position: 'relative', flexShrink: 0 }}>
            <img src={photoBase64} alt="Photo preview" style={{
              width: '40px',
              height: '40px',
              borderRadius: '8px',
              objectFit: 'cover',
            }} />
            <button
              onClick={() => setPhotoBase64(null)}
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
                fontSize: '11px',
                lineHeight: '18px',
                textAlign: 'center',
                cursor: 'pointer',
                padding: 0,
              }}
            >×</button>
          </div>
        ) : (
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
        )}
        <input
          type="text"
          placeholder="Tell Penny what you need…"
          value={chatText}
          onChange={e => setChatText(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') handleSendChat() }}
          disabled={chatLoading}
          style={listStyles.chatInput}
        />
        <button
          onClick={handleSendChat}
          disabled={(!chatText.trim() && !photoBase64) || chatLoading}
          style={listStyles.chatSendBtn((!!chatText.trim() || !!photoBase64) && !chatLoading)}
        >
          {chatLoading
            ? <span style={{ fontSize: '12px', fontFamily: FONT }}>…</span>
            : <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
                <path d="M2 9h14M10 3l6 6-6 6" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
          }
        </button>
      </div>

      {/* Item detail sheet */}
      {selectedItem && (
        <ActionSheet isOpen={true} onClose={() => setSelectedItem(null)} title="Edit item" maxHeight="75dvh">
            <p style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, margin: '-4px 0 8px 0' }}>
              {selectedItem.added_by_name || (selectedItem.added_by_phone ? formatPhone(selectedItem.added_by_phone) : '')}
              {(selectedItem.added_by_name || selectedItem.added_by_phone) && ' · '}
              {selectedItem.source === 'sms' ? 'Text' : selectedItem.source === 'email' ? 'Email' : 'App'}
              {selectedItem.created_at && ` · ${new Date(selectedItem.created_at).toLocaleDateString()}`}
            </p>

            <div>
              <label style={listStyles.fieldLabel}>Name</label>
              <input
                type="text"
                value={editName}
                onChange={e => setEditName(e.target.value)}
                placeholder="Item name"
                style={listStyles.sheetTextInput}
                autoFocus
              />
            </div>

            <div>
              <label style={listStyles.fieldLabel}>Brand</label>
              <input
                type="text"
                value={editBrand}
                onChange={e => setEditBrand(e.target.value)}
                placeholder="Optional"
                style={listStyles.sheetTextInput}
              />
            </div>

            <div>
              <label style={listStyles.fieldLabel}>Department</label>
              <select
                value={editDept}
                onChange={e => setEditDept(e.target.value)}
                style={listStyles.sheetSelect}
              >
                {DEPARTMENTS.map(d => <option key={d} value={d}>{d}</option>)}
              </select>
            </div>

            <div style={listStyles.sheetRow}>
              <div style={{ flex: 1 }}>
                <label style={listStyles.fieldLabel}>Size</label>
                <input
                  type="text"
                  list="size-suggestions"
                  value={editSize}
                  onChange={e => setEditSize(e.target.value)}
                  placeholder="e.g. 16 oz, 1 lb"
                  style={listStyles.sheetTextInput}
                />
                <datalist id="size-suggestions">
                  {['4 oz','8 oz','12 oz','16 oz','24 oz','32 oz','1 lb','2 lb','5 lb','½ gal','1 gal','Small','Medium','Large','XL'].map(s => (
                    <option key={s} value={s} />
                  ))}
                </datalist>
              </div>
              <div>
                <label style={listStyles.fieldLabel}>Qty</label>
                <QuantityControl value={editQty} onChange={setEditQty} />
              </div>
            </div>

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

            {itemHistory.length > 0 && (
              <div style={{ marginTop: '8px' }}>
                <SectionHeader label="History" />
                {itemHistory.map((mod, i) => (
                  <div key={i} style={{
                    padding: '6px 0',
                    borderBottom: i < itemHistory.length - 1 ? `1px solid ${BORDER}` : 'none',
                    fontFamily: FONT,
                    fontSize: '13px',
                    color: TEXT_SECONDARY,
                  }}>
                    <span style={{ fontWeight: 500, color: TEXT }}>{mod.user_name || 'Unknown'}</span>
                    {' '}{mod.action}{mod.detail ? ` (${mod.detail})` : ''}
                    {' · '}{mod.source === 'sms' ? 'Text' : mod.source === 'email' ? 'Email' : 'App'}
                    {mod.created_at && ` · ${new Date(mod.created_at).toLocaleDateString()}`}
                  </div>
                ))}
              </div>
            )}

            <button onClick={handleEditDelete} disabled={editLoading} style={listStyles.sheetDeleteBtn}>
              Delete item
            </button>
        </ActionSheet>
      )}

      {/* Review sheet — shows items from last parse */}
      {showReviewSheet && (lastAdded.length > 0 || lastSkipped.length > 0) && (
        <ActionSheet isOpen={true} onClose={() => { setShowReviewSheet(false); setLastAdded([]); setLastSkipped([]) }} title={lastAdded.length > 0 ? 'Added' : undefined}>

            {/* Added items section */}
            {lastAdded.length > 0 && (
              <>
                {lastAdded.map(item => (
                  <div key={item.id} style={{
                    display: 'flex',
                    alignItems: 'center',
                    padding: '10px 0',
                    borderBottom: `1px solid ${BORDER}`,
                    gap: '8px',
                  }}>
                    <div style={{ flex: 1 }}>
                      <span style={{ fontFamily: FONT, fontSize: '16px', color: TEXT }}>
                        {item.brand ? `${item.brand} ` : ''}{item.name}
                      </span>
                      {item.size && (
                        <span style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, marginLeft: '6px' }}>
                          {item.size}
                        </span>
                      )}
                      <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '2px' }}>
                        {item.department}
                      </div>
                    </div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '0', border: `1.5px solid ${BORDER}`, borderRadius: '10px', overflow: 'hidden', backgroundColor: BG, flexShrink: 0 }}>
                      <button
                        onClick={async () => {
                          const newQty = Math.max(1, item.quantity - 1)
                          const res = await fetch(`/api/list/${item.id}`, {
                            method: 'PATCH',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ quantity: newQty }),
                          })
                          if (res.ok) {
                            const updated: ListItem = await res.json()
                            setLastAdded(prev => prev.map(i => i.id === updated.id ? updated : i))
                            setItems(prev => sortItems(prev.map(i => i.id === updated.id ? updated : i)))
                          }
                        }}
                        style={{ width: '34px', height: '34px', border: 'none', background: 'none', fontSize: '18px', color: BRAND, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
                      >−</button>
                      <span style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: TEXT, minWidth: '24px', textAlign: 'center' }}>{item.quantity}</span>
                      <button
                        onClick={async () => {
                          const res = await fetch(`/api/list/${item.id}`, {
                            method: 'PATCH',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ quantity: item.quantity + 1 }),
                          })
                          if (res.ok) {
                            const updated: ListItem = await res.json()
                            setLastAdded(prev => prev.map(i => i.id === updated.id ? updated : i))
                            setItems(prev => sortItems(prev.map(i => i.id === updated.id ? updated : i)))
                          }
                        }}
                        style={{ width: '34px', height: '34px', border: 'none', background: 'none', fontSize: '18px', color: BRAND, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
                      >+</button>
                    </div>
                    <button
                      onClick={async () => {
                        await fetch(`/api/list/${item.id}`, { method: 'DELETE' })
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
                        fontSize: '14px',
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
                      gap: '8px',
                    }}>
                      <div style={{ flex: 1 }}>
                        <span style={{ fontFamily: FONT, fontSize: '16px', color: TEXT }}>
                          {skipped.parsed.brand ? `${skipped.parsed.brand} ` : ''}{skipped.parsed.name}
                        </span>
                        <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, marginTop: '2px' }}>
                          Currently: ×{existing?.quantity ?? 1}
                        </div>
                      </div>
                      <button
                        onClick={async () => {
                          const currentQty = existing?.quantity ?? 1
                          const res = await fetch(`/api/list/${skipped.existing_id}`, {
                            method: 'PATCH',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ quantity: currentQty + (skipped.parsed.quantity || 1) }),
                          })
                          if (res.ok) {
                            const updated: ListItem = await res.json()
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
                          fontSize: '14px',
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
                    await fetch(`/api/list/${item.id}`, { method: 'DELETE' })
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
        <ActionSheet isOpen={true} onClose={() => setShowSaveSheet(false)} title="Save List">
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
    margin: '16px 0 0 0',
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
    fontSize: '13px',
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
    fontSize: '14px',
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
    fontSize: '14px',
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
    height: '100svh',
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
    minWidth: '180px',
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

  deptHeader: (open: boolean): React.CSSProperties => ({
    display: 'flex',
    alignItems: 'center',
    width: '100%',
    padding: '13px 20px',
    background: open ? `${BRAND}0D` : 'none',
    border: 'none',
    borderBottom: `1px solid ${BORDER}`,
    cursor: 'pointer',
    textAlign: 'left' as const,
    WebkitTapHighlightColor: 'transparent',
    gap: '8px',
    fontFamily: FONT,
    fontSize: '13px',
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
    fontSize: '18px',
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
    border: `2px solid ${checked ? BRAND : BORDER}`,
    backgroundColor: checked ? BRAND : 'transparent',
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
    fontSize: '16px',
    color: checked ? TEXT_SECONDARY : TEXT,
    textDecoration: checked ? 'line-through' : 'none',
    transition: 'all 0.15s',
  }),

  itemMeta: {
    fontFamily: FONT,
    fontSize: '13px',
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
    alignItems: 'center',
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
    gap: '10px',
    maxHeight: '85dvh',
    overflowY: 'auto' as const,
  } as React.CSSProperties,

  sheetTitle: {
    fontFamily: FONT,
    fontSize: '18px',
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
    fontSize: '14px',
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
  @keyframes penny-pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.2; }
  }
  .penny-pulse {
    animation: penny-pulse 1.2s ease-in-out infinite;
  }
`

// Inject the global styles into the document head.
// This runs once when the module loads.
const styleEl = document.createElement('style')
styleEl.textContent = globalCSS
document.head.appendChild(styleEl)
