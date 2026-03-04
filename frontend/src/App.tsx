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

// The three screens the app can show
type Screen = 'phone' | 'code' | 'home'

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
      // Small delay so the DOM has rendered
      setTimeout(() => phoneInputRef.current?.focus(), 50)
    }
    if (screen === 'code') {
      setTimeout(() => codeInputRefs.current[0]?.focus(), 50)
    }
  }, [screen, checking])

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
  // HOME SCREEN (placeholder — the list/chat UI comes next)
  // =========================================================================

  return (
    <div style={styles.container}>
      <div style={styles.card}>
        <Logo />
        <h1 style={styles.wordmark}>thriftly</h1>
        <p style={styles.tagline}>
          You're in! The shopping list is coming soon.
        </p>
        <button onClick={handleLogout} style={styles.logoutButton}>
          Log out
        </button>
      </div>
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
    minHeight: '100dvh', // dvh = dynamic viewport height, accounts for mobile browser chrome
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: BG,
    fontFamily: FONT,
    padding: '24px 20px',
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
// GLOBAL STYLES
//
// Inject a tiny bit of global CSS for things inline styles can't do:
//   - Focus ring colors on inputs
//   - Remove default body margin
//   - Smooth font rendering
// =============================================================================

const globalCSS = `
  * { margin: 0; padding: 0; box-sizing: border-box; }

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
`

// Inject the global styles into the document head.
// This runs once when the module loads.
const styleEl = document.createElement('style')
styleEl.textContent = globalCSS
document.head.appendChild(styleEl)
