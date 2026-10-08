// =============================================================================
// ClerkAuthProvider — the ONLY module that mounts @clerk/clerk-react's provider.
//
// Rally sign-in is the one way into Groflo. RootProviders loads this file
// lazily, so Clerk stays in its own chunk and off the first paint.
//
// Sign-in is always hosted on the Rally primary (VITE_CLERK_SIGN_IN_URL). There
// is no embedded <SignIn/> and no passkey handling here — the primary owns the
// credential UI. How this app relates to the primary is chosen at build time:
//
//   VITE_CLERK_DOMAIN unset  → FIRST-PARTY. The app is served from the same
//     apex as the Rally Clerk instance (groflo.rallytasks.com), so it shares
//     the primary's session directly. No isSatellite, no domain. This is the
//     launch configuration.
//
//   VITE_CLERK_DOMAIN set    → SATELLITE. The app is on a different apex
//     (app.groflo.io) and syncs its session from the primary.
//
// Either way the user comes back to the page they left: ClerkSignInButton asks
// Clerk for the sign-in URL with a redirect to the current address.
//
// A half-configured build fails SAFE: if the key or the sign-in URL is missing,
// children render WITHOUT Clerk and RallyReadyContext stays false, so the login
// screen says sign-in is briefly unavailable. A misconfiguration must never
// crash the app or show a blank page.
// =============================================================================

import { useEffect, type ReactNode } from 'react'
import { ClerkProvider } from '@clerk/clerk-react'
import { ClerkTokenBridge } from './ClerkTokenBridge'
import { ClerkSessionExchange } from './ClerkSessionExchange'
import { RallyReadyContext } from './RallyReadyContext'

interface ClerkAuthProviderProps {
  children: ReactNode
}

let warned = false

export function ClerkAuthProvider({ children }: ClerkAuthProviderProps) {
  const publishableKey = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY
  const signInUrl = import.meta.env.VITE_CLERK_SIGN_IN_URL
  const domain = import.meta.env.VITE_CLERK_DOMAIN

  const misconfigured = !publishableKey || !signInUrl

  useEffect(() => {
    if (misconfigured && !warned) {
      warned = true
      // eslint-disable-next-line no-console
      console.warn(
        'Rally sign-in is not fully configured (VITE_CLERK_PUBLISHABLE_KEY and VITE_CLERK_SIGN_IN_URL are both required). Sign-in is unavailable until both are set.',
      )
    }
  }, [misconfigured])

  if (misconfigured) {
    return <>{children}</>
  }

  const allowedRedirectOrigins = (import.meta.env.VITE_ALLOWED_REDIRECT_ORIGINS || '')
    .split(',')
    .map(o => o.trim())
    .filter(Boolean)
  const origins = allowedRedirectOrigins.length > 0 ? allowedRedirectOrigins : [window.location.origin]

  const inner = (
    <>
      <ClerkTokenBridge />
      <ClerkSessionExchange />
      <RallyReadyContext.Provider value={true}>{children}</RallyReadyContext.Provider>
    </>
  )

  if (domain) {
    // Satellite: a different apex from the Rally primary.
    return (
      <ClerkProvider
        publishableKey={publishableKey}
        isSatellite
        domain={domain}
        signInUrl={signInUrl}
        allowedRedirectOrigins={origins}
        signInForceRedirectUrl={window.location.origin}
        afterSignInUrl={window.location.origin}
      >
        {inner}
      </ClerkProvider>
    )
  }

  // First-party: same apex as the Rally primary. The redirect the sign-in
  // button asks for (the page the user was on) wins; this origin is only the
  // fallback when a flow arrives without one.
  return (
    <ClerkProvider
      publishableKey={publishableKey}
      signInUrl={signInUrl}
      allowedRedirectOrigins={origins}
      signInFallbackRedirectUrl={window.location.origin}
      signUpFallbackRedirectUrl={window.location.origin}
    >
      {inner}
    </ClerkProvider>
  )
}

export default ClerkAuthProvider
