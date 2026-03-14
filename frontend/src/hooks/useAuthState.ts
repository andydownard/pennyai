// =============================================================================
// useAuthState — Custom hook for auth-related state, handlers, and effects
//
// Extracted from App.tsx to reduce file size. Contains:
//   - Auth form state (phone, code, email, password, etc.)
//   - Auth handlers (login, register, verify, logout, etc.)
//   - Login typewriter effect
//   - Google config fetch
//   - Online/offline tracking
// =============================================================================

import { useState, useRef, useEffect, useCallback } from 'react'

// Auth success data shape — passed to the parent when authentication succeeds
export interface AuthSuccessData {
  user_id?: number
  is_admin?: boolean
  auth_methods?: string[]
  name?: string
  last_name?: string
  phone?: string
  email?: string
  login_count?: number
  can_submit_feedback?: boolean
  settings?: Record<string, unknown>
  account_merged?: boolean
  merge_message?: string
}

export interface UseAuthStateOptions {
  setScreen: (s: string, replace?: boolean) => void
  showToast: (msg: string, type?: 'success' | 'error') => void
  onAuthSuccess: (data: AuthSuccessData) => void
  /** Called after successful verify-code to try joining a household */
  tryJoinCode: () => void
  /** Map of URL paths to screen names, for post-login redirects */
  pathToScreen: Record<string, string>
}

// Login page typewriter messages
const LOGIN_MESSAGES = [
  "Hi, I'm Penny — your household's AI shopping assistant.",
  "Tell me what you need and I'll organize your list by aisle.",
  "I learn your brand preferences over time.",
  "Share your list with your whole household.",
  "Text or email me items anytime — I'll add them for you.",
  "Snap a photo of your fridge and I'll spot what's missing.",
]

export function useAuthState(options: UseAuthStateOptions) {
  const { setScreen, showToast, onAuthSuccess, tryJoinCode, pathToScreen } = options

  // --- Auth form state ---
  const [phone, setPhone] = useState('')
  const [code, setCode] = useState(['', '', '', '', '', ''])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState(true)
  const [isOffline, setIsOffline] = useState(!navigator.onLine)

  // --- Email/Google auth state ---
  const [emailInput, setEmailInput] = useState('')
  const [passwordInput, setPasswordInput] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [nameInput, setNameInput] = useState('')
  const [lastNameInput, setLastNameInput] = useState('')
  const [googleClientId, setGoogleClientId] = useState('')
  const [resetToken, setResetToken] = useState('')
  const [resetSent, setResetSent] = useState(false)
  const [magicLinkSent, setMagicLinkSent] = useState(false)

  // --- SMS opt-in state ---
  const [smsOptIn, setSmsOptIn] = useState(true)

  // --- Login typewriter state ---
  const [loginPennyText, setLoginPennyText] = useState('')
  const loginTypingRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // --- Refs ---
  const phoneInputRef = useRef<HTMLInputElement>(null)
  const codeInputRefs = useRef<(HTMLInputElement | null)[]>([])
  const verifyingRef = useRef(false)

  // Stable refs for callbacks to avoid stale closures in effects
  const tryJoinCodeRef = useRef(tryJoinCode)
  tryJoinCodeRef.current = tryJoinCode
  const onAuthSuccessRef = useRef(onAuthSuccess)
  onAuthSuccessRef.current = onAuthSuccess
  const showToastRef = useRef(showToast)
  showToastRef.current = showToast
  const setScreenRef = useRef(setScreen)
  setScreenRef.current = setScreen
  const pathToScreenRef = useRef(pathToScreen)
  pathToScreenRef.current = pathToScreen

  // =========================================================================
  // GOOGLE CONFIG FETCH + RESET TOKEN EXTRACTION
  // =========================================================================

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

  // =========================================================================
  // ONLINE / OFFLINE TRACKING
  // =========================================================================

  useEffect(() => {
    const goOnline = () => setIsOffline(false)
    const goOffline = () => setIsOffline(true)
    window.addEventListener('online', goOnline)
    window.addEventListener('offline', goOffline)
    return () => {
      window.removeEventListener('online', goOnline)
      window.removeEventListener('offline', goOffline)
    }
  }, [])

  // =========================================================================
  // LOGIN TYPEWRITER EFFECT
  // =========================================================================

  const startLoginTypewriter = useCallback((screen: string) => {
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
  }, [])

  // =========================================================================
  // AUTH HANDLERS
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
        if (data.account_merged && data.merge_message) {
          sessionStorage.setItem('thriftly_merge_msg', data.merge_message)
        }
        setScreenRef.current('home', true)
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
        setScreenRef.current('home', true)
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
        showToastRef.current('Password reset! Please sign in.')
        setPasswordInput('')
        setConfirmPassword('')
        setScreenRef.current('email-login')
      } else {
        setError(data.error || 'Failed to reset password')
      }
    } catch { setError('Failed to reset password') }
    finally { setLoading(false) }
  }

  const handlePhoneChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    let raw = e.target.value.replace(/\D/g, '')
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
        setScreenRef.current('code')
        setCode(['', '', '', '', '', ''])
      } else {
        setError(data.error || 'Failed to send code')
      }
    } catch {
      setError('Network error. Please try again.')
    } finally {
      setLoading(false)
    }
  }

  const verifyCode = useCallback(async (fullCode: string) => {
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
        onAuthSuccessRef.current({
          is_admin: data.is_admin as boolean | undefined,
          user_id: data.user_id as number | undefined,
          auth_methods: data.auth_methods as string[] | undefined,
        })
        tryJoinCodeRef.current()
        // Check for post-login redirect (deep link before auth)
        const redirect = sessionStorage.getItem('thriftly_redirect')
        if (redirect) {
          sessionStorage.removeItem('thriftly_redirect')
          const target = pathToScreenRef.current[redirect]
          setScreenRef.current((target || 'home') as string, true)
        } else {
          setScreenRef.current('home', true)
        }
      } else {
        setError((data.error as string) || 'Invalid code')
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

  const handleCodeChange = (index: number, value: string) => {
    const digits = value.replace(/\D/g, '')

    // iOS autofill may put the full code into one input — distribute across all boxes
    if (digits.length > 1) {
      const newCode = [...code]
      for (let i = 0; i < digits.length && i < 6; i++) {
        newCode[i] = digits[i]
      }
      setCode(newCode)
      setError('')
      const nextEmpty = newCode.findIndex(d => !d)
      if (nextEmpty >= 0) {
        codeInputRefs.current[nextEmpty]?.focus()
      } else {
        codeInputRefs.current[5]?.focus()
        verifyCode(newCode.join(''))
      }
      return
    }

    const digit = digits.slice(-1)
    const newCode = [...code]
    newCode[index] = digit
    setCode(newCode)
    setError('')

    if (digit && index < 5) {
      codeInputRefs.current[index + 1]?.focus()
    }

    // Auto-submit when all 6 digits are filled
    if (digit && newCode.join('').length === 6) {
      verifyCode(newCode.join(''))
    }
  }

  const handleCodeKeyDown = (index: number, e: React.KeyboardEvent) => {
    if (e.key === 'Backspace' && !code[index] && index > 0) {
      codeInputRefs.current[index - 1]?.focus()
    }
  }

  const handleCodePaste = (e: React.ClipboardEvent) => {
    e.preventDefault()
    const pasted = e.clipboardData.getData('text').replace(/\D/g, '').slice(0, 6)
    if (pasted.length === 0) return

    const newCode = [...code]
    for (let i = 0; i < pasted.length && i < 6; i++) {
      newCode[i] = pasted[i]
    }
    setCode(newCode)

    const nextEmpty = newCode.findIndex(d => !d)
    if (nextEmpty >= 0) {
      codeInputRefs.current[nextEmpty]?.focus()
    } else {
      codeInputRefs.current[5]?.focus()
      verifyCode(newCode.join(''))
    }
  }

  const handleLogout = async () => {
    await fetch('/auth/logout', { method: 'POST' })
    setScreenRef.current('phone', true)
    setPhone('')
    setCode(['', '', '', '', '', ''])
    setError('')
  }

  // =========================================================================
  // iOS AUTOFILL EFFECT
  // =========================================================================

  // iOS autofill fallback: native `input` event listener on the first code box.
  // iOS may set .value directly (bypassing React onChange), so we detect multi-digit
  // values via a native listener and distribute across all boxes.
  useEffect(() => {
    const el = codeInputRefs.current[0]
    if (!el) return
    const handler = () => {
      const digits = (el.value || '').replace(/\D/g, '')
      if (digits.length > 1) {
        handleCodeChange(0, digits)
      }
    }
    el.addEventListener('input', handler)
    return () => el.removeEventListener('input', handler)
  })

  return {
    // State
    phone, setPhone,
    code, setCode,
    error, setError,
    loading, setLoading,
    checking, setChecking,
    isOffline, setIsOffline,
    emailInput, setEmailInput,
    passwordInput, setPasswordInput,
    confirmPassword, setConfirmPassword,
    nameInput, setNameInput,
    lastNameInput, setLastNameInput,
    googleClientId, setGoogleClientId,
    resetToken, setResetToken,
    resetSent, setResetSent,
    magicLinkSent, setMagicLinkSent,
    smsOptIn, setSmsOptIn,
    loginPennyText,

    // Refs
    phoneInputRef,
    codeInputRefs,

    // Handlers
    handlePhoneChange,
    handleSendCode,
    verifyCode,
    handleCodeChange,
    handleCodeKeyDown,
    handleCodePaste,
    handleEmailRegister,
    handleEmailLogin,
    handleForgotPassword,
    handleMagicLink,
    handleResetPassword,
    handleLogout,

    // Effects that need screen param
    startLoginTypewriter,
  }
}
