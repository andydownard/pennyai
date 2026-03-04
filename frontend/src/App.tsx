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
type Screen = 'phone' | 'code' | 'home' | 'saved'

type ListItem = {
  id: number
  name: string
  brand: string
  department: string
  quantity: number
  size: string
  checked: boolean
  created_at: string
}

type SavedList = {
  id: number
  name: string
  item_count: number
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
    <svg
      width={size}
      height={size}
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      style={{ flexShrink: 0 }}
      aria-label="Penny"
    >
      {/* Background circle */}
      <circle cx="24" cy="24" r="24" fill={BRAND} />
      {/* Face */}
      <circle cx="24" cy="22" r="13" fill="#FBE8D3" />
      {/* Eyes */}
      <circle cx="19.5" cy="20.5" r="2" fill="#2A2A2A" />
      <circle cx="28.5" cy="20.5" r="2" fill="#2A2A2A" />
      {/* Eye shine */}
      <circle cx="20.2" cy="19.8" r="0.7" fill={WHITE} />
      <circle cx="29.2" cy="19.8" r="0.7" fill={WHITE} />
      {/* Smile */}
      <path d="M20 26.5Q24 30 28 26.5" stroke="#2A2A2A" strokeWidth="1.5" strokeLinecap="round" fill="none" />
      {/* Blush */}
      <ellipse cx="17" cy="25" rx="2.5" ry="1.5" fill="#F5B8A0" opacity="0.5" />
      <ellipse cx="31" cy="25" rx="2.5" ry="1.5" fill="#F5B8A0" opacity="0.5" />
      {/* Hair */}
      <path d="M11 20C11 12.8 16.8 9 24 9C31.2 9 37 12.8 37 20" stroke={BRAND} strokeWidth="0" fill="#5B3A29" />
      <path d="M11.5 21C11 15 16 8 24 8C32 8 37 15 36.5 21C36.5 21 35 13 24 13C13 13 11.5 21 11.5 21Z" fill="#5B3A29" />
      {/* Sparkle */}
      <path d="M38 10L39 13L42 14L39 15L38 18L37 15L34 14L37 13Z" fill="#FFD700" />
    </svg>
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
// MAIN APP COMPONENT
// =============================================================================

export default function App() {
  // --- State ---
  const [screen, setScreen] = useState<Screen>('phone')
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
  const [confirmDeleteSavedList, setConfirmDeleteSavedList] = useState(false)

  // --- Penny loading spinner state ---
  const [pennyStatus, setPennyStatus] = useState<string | null>(null)

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

  useEffect(() => {
    fetch('/auth/me')
      .then(res => {
        if (res.ok) {
          setScreen('home')
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

  useEffect(() => {
    if (screen !== 'home') return
    fetch('/api/list')
      .then(res => res.ok ? res.json() : [])
      .then(data => setItems(sortItems(Array.isArray(data) ? data : [])))
      .catch(() => {})
  }, [screen])

  const loadSavedLists = () => {
    fetch('/api/saved-lists')
      .then(res => res.ok ? res.json() : [])
      .then(data => setSavedLists(Array.isArray(data) ? data : []))
      .catch(() => {})
  }

  useEffect(() => {
    if (screen !== 'saved') return
    loadSavedLists()
  }, [screen])

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    if (toastTimer.current) clearTimeout(toastTimer.current)
    setToast({ message, type })
    toastTimer.current = setTimeout(() => setToast(null), 3000)
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

  const handleClearList = async () => {
    try {
      const res = await fetch('/api/list/clear', { method: 'DELETE' })
      if (res.ok) {
        setItems([])
        showToast('List cleared')
      }
    } catch {
      showToast('Failed to clear list', 'error')
    }
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

  const handleLoadSavedList = async (id: number, mode: 'merge' | 'overwrite' = 'merge') => {
    try {
      const url = mode === 'overwrite'
        ? `/api/saved-lists/${id}/load?mode=overwrite`
        : `/api/saved-lists/${id}/load`
      const res = await fetch(url, { method: 'POST' })
      if (res.ok) {
        setSelectedSavedList(null)
        setScreen('home')
        // Reload active list
        const listRes = await fetch('/api/list')
        if (listRes.ok) {
          const data = await listRes.json()
          setItems(sortItems(Array.isArray(data) ? data : []))
        }
        showToast(mode === 'overwrite' ? 'List replaced!' : 'List merged!')
      }
    } catch {
      showToast('Failed to load list', 'error')
    }
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

          <h1 style={styles.wordmark}>thriftly</h1>
          <p style={styles.tagline}>
            Text Penny AI to build your family's shopping list
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
          </div>

          {error && <p style={styles.error}>{error}</p>}

          <button
            onClick={handleSendCode}
            disabled={!isValid || loading}
            style={{
              ...styles.button,
              opacity: isValid && !loading ? 1 : 0.4,
              cursor: isValid && !loading ? 'pointer' : 'default',
            }}
          >
            {loading ? 'Sending…' : 'Send Code'}
          </button>
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

          <h1 style={styles.wordmark}>thriftly</h1>

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
        </div>
      </div>
    )
  }

  // =========================================================================
  // SAVED LISTS SCREEN
  // =========================================================================

  if (screen === 'saved') {
    return (
      <div style={listStyles.page}>
        {/* Header with hamburger */}
        <div style={{ flexShrink: 0, backgroundColor: WHITE, borderBottom: `1px solid ${BORDER}` }}>
        <div style={listStyles.header}>
          <span style={listStyles.headerWordmark}>thriftly</span>
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
                  <button
                    style={listStyles.menuItem(false)}
                    onClick={() => { setMenuOpen(false); setScreen('home') }}
                  >My List</button>
                  <button
                    style={listStyles.menuItem(true)}
                    onClick={() => { setMenuOpen(false); setScreen('saved') }}
                  >Saved Lists</button>
                  <div style={{ height: '1px', backgroundColor: BORDER, margin: '4px 0' }} />
                  <button
                    style={listStyles.menuItem(false)}
                    onClick={handleLogout}
                  >Sign Out</button>
                </div>
              </>
            )}
          </div>
        </div>
        </div>

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

        {/* Toast */}
        {toast && (
          <div style={{
            position: 'fixed',
            top: '72px',
            left: '50%',
            transform: 'translateX(-50%)',
            backgroundColor: toast.type === 'error' ? ERROR_RED : BRAND,
            color: WHITE,
            padding: '8px 18px',
            borderRadius: '20px',
            fontFamily: FONT,
            fontSize: '14px',
            fontWeight: 500,
            zIndex: 50,
            boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
            whiteSpace: 'nowrap',
          }}>
            {toast.message}
          </div>
        )}

        {/* Saved list action sheet */}
        {selectedSavedList && (
          <div style={listStyles.overlay} onClick={() => { setSelectedSavedList(null); setConfirmDeleteSavedList(false) }}>
            <div style={listStyles.sheet} onClick={e => e.stopPropagation()}>
              <p style={listStyles.sheetTitle}>{selectedSavedList.name}</p>
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
              {!confirmDeleteSavedList ? (
                <button
                  onClick={() => setConfirmDeleteSavedList(true)}
                  style={listStyles.sheetDeleteBtn}
                >Delete List</button>
              ) : (
                <button
                  onClick={() => {
                    setConfirmDeleteSavedList(false)
                    handleDeleteSavedList(selectedSavedList.id)
                  }}
                  style={{
                    ...listStyles.sheetDeleteBtn,
                    backgroundColor: ERROR_RED,
                    color: WHITE,
                    border: 'none',
                  }}
                >Tap again to confirm delete</button>
              )}
            </div>
          </div>
        )}
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
      {/* Fixed header */}
      <div style={{ flexShrink: 0, backgroundColor: WHITE, borderBottom: `1px solid ${BORDER}` }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '16px 20px' }}>
          <span style={listStyles.headerWordmark}>thriftly</span>
          {items.length > 0 && (
            <span style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, fontWeight: 500 }}>
              {items.filter(i => i.checked).length}/{items.length}
            </span>
          )}
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
                <button
                  style={listStyles.menuItem(true)}
                  onClick={() => { setMenuOpen(false); setScreen('home') }}
                >My List</button>
                <button
                  style={listStyles.menuItem(false)}
                  onClick={() => { setMenuOpen(false); setScreen('saved') }}
                >Saved Lists</button>
                <div style={{ height: '1px', backgroundColor: BORDER, margin: '4px 0' }} />
                <button
                  style={listStyles.menuItem(false)}
                  onClick={handleLogout}
                >Sign Out</button>
              </div>
            </>
          )}
          </div>
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

      {/* Toast notification */}
      {toast && (
        <div style={{
          position: 'fixed',
          top: '72px',
          left: '16px',
          right: '16px',
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          backgroundColor: toast.type === 'error' ? ERROR_RED : BRAND,
          color: WHITE,
          padding: '10px 16px',
          borderRadius: '16px',
          fontFamily: FONT,
          fontSize: '14px',
          fontWeight: 500,
          zIndex: 50,
          boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
        }}>
          <PennyAvatar size={28} />
          <span style={{ flex: 1 }}>{toast.message}</span>
          {(lastAdded.length > 0 || lastSkipped.length > 0) && toast.type !== 'error' && (
            <>
              <span style={{ opacity: 0.5 }}>·</span>
              <button
                onClick={() => {
                  setShowReviewSheet(true)
                  setToast(null)
                  if (toastTimer.current) clearTimeout(toastTimer.current)
                }}
                style={{
                  background: 'none',
                  border: 'none',
                  color: WHITE,
                  fontFamily: FONT,
                  fontSize: '14px',
                  fontWeight: 600,
                  textDecoration: 'underline',
                  textUnderlineOffset: '2px',
                  cursor: 'pointer',
                  padding: 0,
                }}
              >Review</button>
            </>
          )}
        </div>
      )}

      {/* "All done" toast */}
      {showDoneToast && items.length > 0 && items.every(i => i.checked) && (
        <div style={{
          position: 'fixed',
          top: '72px',
          left: '16px',
          right: '16px',
          backgroundColor: BRAND,
          color: WHITE,
          padding: '12px 16px',
          borderRadius: '16px',
          fontFamily: FONT,
          fontSize: '14px',
          fontWeight: 500,
          zIndex: 50,
          boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
        }}>
          <PennyAvatar size={28} />
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
        </div>
      )}

      {/* Save/Clear buttons */}
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
              fontWeight: 600,
              color: WHITE,
              backgroundColor: BRAND,
              border: 'none',
              borderRadius: '20px',
              padding: '8px 18px',
              cursor: 'pointer',
            }}
          >Save List</button>
          <button
            onClick={handleClearList}
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

      {/* Penny loading spinner */}
      {pennyStatus && (
        <div style={{
          flexShrink: 0,
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          padding: '10px 16px',
          backgroundColor: `${BRAND}0A`,
          borderTop: `1px solid ${BORDER}`,
          fontFamily: FONT,
          fontSize: '14px',
          color: BRAND,
          fontWeight: 500,
        }}>
          <PennyAvatar size={28} />
          <span>{pennyStatus}</span>
          <span className="penny-pulse" style={{ width: '6px', height: '6px', borderRadius: '50%', backgroundColor: BRAND, flexShrink: 0 }} />
        </div>
      )}

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
        <div style={listStyles.overlay} onClick={() => setSelectedItem(null)}>
          <div style={{ ...listStyles.sheet, maxHeight: '60dvh' }} onClick={e => e.stopPropagation()}>
            <p style={listStyles.sheetTitle}>Edit item</p>

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
                <div style={listStyles.qtyControl}>
                  <button
                    onClick={() => setEditQty(q => Math.max(1, q - 1))}
                    style={listStyles.qtyBtn}
                  >−</button>
                  <span style={listStyles.qtyValue}>{editQty}</span>
                  <button
                    onClick={() => setEditQty(q => q + 1)}
                    style={listStyles.qtyBtn}
                  >+</button>
                </div>
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

            <button onClick={handleEditDelete} disabled={editLoading} style={listStyles.sheetDeleteBtn}>
              Delete item
            </button>
          </div>
        </div>
      )}

      {/* Review sheet — shows items from last parse */}
      {showReviewSheet && (lastAdded.length > 0 || lastSkipped.length > 0) && (
        <div style={listStyles.overlay} onClick={() => { setShowReviewSheet(false); setLastAdded([]); setLastSkipped([]) }}>
          <div style={listStyles.sheet} onClick={e => e.stopPropagation()}>

            {/* Added items section */}
            {lastAdded.length > 0 && (
              <>
                <p style={listStyles.sheetTitle}>Added</p>
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
          </div>
        </div>
      )}

      {/* Save list sheet */}
      {showSaveSheet && (
        <div style={listStyles.overlay} onClick={() => setShowSaveSheet(false)}>
          <div style={listStyles.sheet} onClick={e => e.stopPropagation()}>
            <p style={listStyles.sheetTitle}>Save List</p>
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
          </div>
        </div>
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
