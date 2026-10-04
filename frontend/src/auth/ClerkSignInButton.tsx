// =============================================================================
// ClerkSignInButton — "Continue with Rally", the one way into Groflo.
//
// Signed out of Rally: the button sends the browser to the Rally primary's
// sign-in page. Clerk builds the URL (never a hand-built one): it knows the
// configured sign-in URL and adds whatever the current mode needs — a plain
// redirect back in first-party mode, the sync parameters too as a satellite.
// The redirect is the page the user was on.
//
// Signed in to Rally but not yet to Groflo: ClerkSessionExchange is trading the
// session, so the button reads "Signing you in…". If that trade failed, or the
// Groflo session expired later, the same button retries it on tap — no round
// trip to Rally — and a second control lets a different person sign in.
//
// Rally not loaded after a fair wait: the calm unavailable message.
// =============================================================================

import { useEffect, useState, useSyncExternalStore } from 'react'
import { useAuth, useClerk } from '@clerk/clerk-react'
import { authStyles } from '../styles'
import { signInRedirectTarget } from './signInRedirect'
import {
  clerkExchangePending,
  clerkSignOut,
  runClerkExchange,
  subscribeClerkExchangePending,
} from './clerkToken'
import { SignInUnavailable } from '../components/SignInUnavailable'

/** How long Rally may take to load before we say sign-in is unavailable. */
export const RALLY_LOAD_TIMEOUT_MS = 10_000

const buttonStyle: React.CSSProperties = {
  ...authStyles.primaryButton,
  minHeight: '56px',
  marginBottom: '12px',
  cursor: 'pointer',
}

const busyStyle: React.CSSProperties = { ...buttonStyle, opacity: 0.6, cursor: 'default' }

export function ClerkSignInButton() {
  const clerk = useClerk()
  const { isLoaded, isSignedIn } = useAuth()
  const exchanging = useSyncExternalStore(subscribeClerkExchangePending, clerkExchangePending)
  const [timedOut, setTimedOut] = useState(false)

  useEffect(() => {
    if (isLoaded) {
      setTimedOut(false)
      return
    }
    const timer = setTimeout(() => setTimedOut(true), RALLY_LOAD_TIMEOUT_MS)
    return () => clearTimeout(timer)
  }, [isLoaded])

  const goToSignIn = () => {
    const url = clerk.buildSignInUrl({ redirectUrl: signInRedirectTarget() })
    window.location.assign(url)
  }

  if (!isLoaded) {
    if (timedOut) return <SignInUnavailable />
    return (
      <button type="button" disabled aria-busy="true" style={busyStyle}>
        Continue with Rally
      </button>
    )
  }

  if (isSignedIn) {
    if (exchanging) {
      return (
        <button type="button" disabled aria-busy="true" style={busyStyle}>
          Signing you in…
        </button>
      )
    }
    return (
      <>
        <button type="button" onClick={() => void runClerkExchange()} style={buttonStyle}>
          Continue with Rally
        </button>
        <button
          type="button"
          onClick={() => void clerkSignOut()}
          style={{ ...authStyles.subtleLink, minHeight: '44px', padding: '8px 12px' }}
        >
          Use a different account
        </button>
      </>
    )
  }

  return (
    <button type="button" onClick={goToSignIn} style={buttonStyle}>
      Continue with Rally
    </button>
  )
}

export default ClerkSignInButton
