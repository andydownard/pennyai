// =============================================================================
// ClerkSignInButton — "Continue with Rally", one more option below Google.
//
// Signed out: a button that sends the browser to the Rally primary's sign-in
// page, built by Clerk (never a hardcoded URL — the satellite handshake needs
// Clerk's own parameters) with a redirect back to where the user already was.
//
// Signed in: nothing. ClerkSessionExchange is already trading the Clerk session
// for a Groflo one, so offering a sign-in button again would only confuse.
// =============================================================================

import { SignedIn, SignedOut, useClerk } from '@clerk/clerk-react'
import { FONT, authStyles } from '../styles'
import { signInRedirectTarget } from './signInRedirect'

export function ClerkSignInButton() {
  const clerk = useClerk()

  const goToSignIn = () => {
    const url = clerk.buildSignInUrl({ redirectUrl: signInRedirectTarget() })
    window.location.assign(url)
  }

  return (
    <>
      <SignedOut>
        <button
          type="button"
          onClick={goToSignIn}
          style={{
            ...authStyles.primaryButton,
            fontFamily: FONT,
            color: 'var(--text)',
            backgroundColor: 'var(--bg)',
            border: '1px solid var(--border)',
            fontWeight: 600,
            padding: '14px',
            marginBottom: '12px',
            cursor: 'pointer',
          }}
        >
          Continue with Rally
        </button>
      </SignedOut>
      <SignedIn>{null}</SignedIn>
    </>
  )
}

export default ClerkSignInButton
