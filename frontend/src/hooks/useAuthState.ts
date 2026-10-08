// =============================================================================
// useAuthState — Custom hook for sign-in state, handlers, and effects
//
// Rally (Clerk) is the one way into Groflo. This hook holds:
//   - The server's Rally flag from /api/config
//   - The exchange callback: what happens once Rally sign-in completes
//   - Sign-out (Groflo session, then the Rally session)
//   - Login typewriter effect
//   - Online/offline tracking
// =============================================================================

import { useState, useRef, useEffect, useCallback } from 'react'
import { clerkSignOut, setClerkExchangeHandler, setClerkResolving } from '../auth/clerkToken'

/** GET /auth/me for a signed-in user. */
export interface MeResponse {
  logged_in?: boolean
  user_id?: number
  is_admin?: boolean
  auth_methods?: string[]
  name?: string
  login_count?: number
  can_submit_feedback?: boolean
}

export interface UseAuthStateOptions {
  setScreen: (s: string, replace?: boolean) => void
  showToast: (msg: string, type?: 'success' | 'error') => void
  /**
   * Called with /auth/me once a Rally sign-in has been traded for a Groflo
   * session. The app does here exactly what it does when a page load finds an
   * existing session: load the user, honour any pending invite, pick a screen.
   */
  onSignedIn: (me: MeResponse) => void
}

// Login page typewriter messages
const LOGIN_MESSAGES = [
  "Hi, I'm Penny. I keep your household's shopping list.",
  "Tell me what you need. I'll sort by aisle.",
  "I learn your brand preferences over time.",
  "Share your list with your whole household.",
  "Email me anytime. I'll add it.",
  "Snap a photo of your fridge. I'll spot what's missing.",
]

const SIGN_IN_FAILED = 'Sign-in could not be completed. Try again.'

export function useAuthState(options: UseAuthStateOptions) {
  const { setScreen, showToast, onSignedIn } = options

  const [error, setError] = useState('')
  const [checking, setChecking] = useState(true)
  const [isOffline, setIsOffline] = useState(!navigator.onLine)

  // Server-side Rally flag. null until /api/config answers, so the login screen
  // neither flashes "unavailable" nor offers a button the server would refuse.
  const [clerkEnabled, setClerkEnabled] = useState<boolean | null>(null)

  // --- Login typewriter state ---
  const [loginPennyText, setLoginPennyText] = useState('')
  const loginTypingRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Stable refs for callbacks to avoid stale closures in effects
  const onSignedInRef = useRef(onSignedIn)
  onSignedInRef.current = onSignedIn
  const showToastRef = useRef(showToast)
  showToastRef.current = showToast
  const setScreenRef = useRef(setScreen)
  setScreenRef.current = setScreen

  // =========================================================================
  // CONFIG FETCH
  // =========================================================================

  useEffect(() => {
    fetch('/api/config')
      .then(r => (r.ok ? r.json() : null))
      .then((data: { clerk_enabled?: boolean } | null) => {
        setClerkEnabled(Boolean(data?.clerk_enabled))
      })
      // If the server cannot be reached, the exchange could not succeed either.
      .catch(() => setClerkEnabled(false))
  }, [])

  // =========================================================================
  // RALLY SIGN-IN (CLERK) EXCHANGE CALLBACK
  //
  // ClerkSessionExchange posts to /auth/clerk/exchange on its own, outside the
  // React tree this hook lives in. This is how the result gets back here: on
  // success re-check /auth/me (the session cookie is now set) and hand the user
  // to the app; on failure show the server's message, which is written for them.
  // =========================================================================

  useEffect(() => {
    setClerkExchangeHandler((exchangeError?: string) => {
      if (exchangeError) {
        setError(exchangeError)
        setClerkResolving(false)
        return
      }
      fetch('/auth/me')
        .then(r => (r.ok ? r.json() : null))
        .then((data: MeResponse | null) => {
          if (!data || data.logged_in === false) {
            setError(SIGN_IN_FAILED)
            return
          }
          setError('')
          onSignedInRef.current(data)
        })
        .catch(() => setError(SIGN_IN_FAILED))
        // Only now: the screen has already moved to the list on success.
        .finally(() => setClerkResolving(false))
    })
    return () => setClerkExchangeHandler(null)
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
    if (screen !== 'login') {
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
  // SIGN OUT
  //
  // Order matters. The Groflo session goes first: if that request cannot reach
  // the server the cookie is still good, so stay put and say so. Then the Rally
  // session — without that, the next "Continue with Rally" on a shared device
  // would sign the same person straight back in. Only then the login screen.
  //
  // Nothing here can loop: ClerkSessionExchange exchanges automatically once
  // per Clerk session, and that session is either gone or already spent.
  // =========================================================================

  const loggingOutRef = useRef(false)

  const handleLogout = async () => {
    if (loggingOutRef.current) return
    loggingOutRef.current = true
    try {
      try {
        await fetch('/auth/logout', { method: 'POST' })
      } catch {
        showToastRef.current("Couldn't sign out — check your connection and try again", 'error')
        return
      }
      await clerkSignOut()
      setError('')
      setScreenRef.current('login', true)
    } finally {
      loggingOutRef.current = false
    }
  }

  return {
    error, setError,
    checking, setChecking,
    isOffline,
    clerkEnabled,
    loginPennyText,
    handleLogout,
    startLoginTypewriter,
  }
}
