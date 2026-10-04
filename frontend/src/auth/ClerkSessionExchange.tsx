// =============================================================================
// ClerkSessionExchange — trades a Clerk session for a Groflo session, once.
//
// Renders nothing. When Clerk reports a signed-in session this posts to
// /auth/clerk/exchange exactly once per Clerk session id.
//
// "Exactly once" is enforced with a sessionStorage key, NOT React state:
//   - React state does not survive StrictMode's deliberate double mount, so a
//     state-only guard would post twice in development
//   - CLAUDE.md forbids relying on React state alone for a one-shot prompt
//
// The key is written BEFORE the request, so a double mount in the same tick
// cannot slip a second POST through. On failure the key is removed so a later
// reload can retry.
// =============================================================================

import { useEffect } from 'react'
import { useAuth } from '@clerk/clerk-react'
import { fetchAction } from '../api'
import { notifyClerkExchanged } from './clerkToken'

const GUARD_PREFIX = 'groflo.clerk_exchanged:'

function readGuard(key: string): boolean {
  try {
    return sessionStorage.getItem(key) !== null
  } catch {
    // Private mode / blocked storage: treat as "not yet exchanged". Worst case
    // is a repeat POST, which the server handles idempotently.
    return false
  }
}

function writeGuard(key: string) {
  try {
    sessionStorage.setItem(key, String(Date.now()))
  } catch {
    // Ignore: see readGuard.
  }
}

function clearGuard(key: string) {
  try {
    sessionStorage.removeItem(key)
  } catch {
    // Ignore: see readGuard.
  }
}

export function ClerkSessionExchange() {
  const { isLoaded, isSignedIn, sessionId } = useAuth()

  useEffect(() => {
    if (!isLoaded || !isSignedIn || !sessionId) return

    const guardKey = GUARD_PREFIX + sessionId
    if (readGuard(guardKey)) return
    writeGuard(guardKey) // claim the slot first — defeats the double mount

    let cancelled = false
    ;(async () => {
      // fetchAction attaches the Clerk Bearer header and reports the server's
      // own error text, which the 409 copy is written for.
      const result = await fetchAction('/auth/clerk/exchange', { method: 'POST' })
      if (cancelled) return

      if (result.ok) {
        notifyClerkExchanged()
        return
      }
      // Let a later reload try again.
      clearGuard(guardKey)
      notifyClerkExchanged(result.error || 'Sign-in could not be completed. Try again.')
    })()

    return () => {
      cancelled = true
    }
  }, [isLoaded, isSignedIn, sessionId])

  return null
}

export default ClerkSessionExchange
