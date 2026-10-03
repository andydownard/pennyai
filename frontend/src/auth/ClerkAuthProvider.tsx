// =============================================================================
// ClerkAuthProvider — the ONLY top-level importer of @clerk/clerk-react.
//
// Keeping the import in one lazily-loaded module is what makes Rally sign-in
// free for everyone else: RootProviders only imports this file when
// VITE_CLERK_PUBLISHABLE_KEY is set, so with the key unset Clerk is not in the
// entry bundle at all.
//
// pennyai is a SATELLITE: isSatellite, with sign-in hosted on the Rally primary.
// There is no embedded <SignIn/> and no passkey handling here — the primary owns
// the credential UI.
//
// A half-configured build fails SAFE: if the key is set but the domain or the
// sign-in URL is missing, children render WITHOUT Clerk and the app behaves
// exactly as it does today. A misconfiguration must never lock anyone out.
// =============================================================================

import { useEffect, type ReactNode } from 'react'
import { ClerkProvider } from '@clerk/clerk-react'
import { ClerkTokenBridge } from './ClerkTokenBridge'
import { ClerkSessionExchange } from './ClerkSessionExchange'

interface ClerkAuthProviderProps {
  children: ReactNode
}

let warned = false

export function ClerkAuthProvider({ children }: ClerkAuthProviderProps) {
  const publishableKey = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY
  const domain = import.meta.env.VITE_CLERK_DOMAIN
  const signInUrl = import.meta.env.VITE_CLERK_SIGN_IN_URL

  const misconfigured = Boolean(publishableKey) && (!domain || !signInUrl)

  useEffect(() => {
    if (misconfigured && !warned) {
      warned = true
      // eslint-disable-next-line no-console
      console.warn(
        'Rally sign-in is not fully configured (VITE_CLERK_DOMAIN and VITE_CLERK_SIGN_IN_URL are both required). Continuing without it.',
      )
    }
  }, [misconfigured])

  if (!publishableKey || misconfigured) {
    return <>{children}</>
  }

  const allowedRedirectOrigins = (import.meta.env.VITE_ALLOWED_REDIRECT_ORIGINS || '')
    .split(',')
    .map(o => o.trim())
    .filter(Boolean)

  return (
    <ClerkProvider
      publishableKey={publishableKey}
      isSatellite
      domain={domain as string}
      signInUrl={signInUrl as string}
      allowedRedirectOrigins={
        allowedRedirectOrigins.length > 0 ? allowedRedirectOrigins : [window.location.origin]
      }
      signInForceRedirectUrl={window.location.origin}
      afterSignInUrl={window.location.origin}
    >
      <ClerkTokenBridge />
      <ClerkSessionExchange />
      {children}
    </ClerkProvider>
  )
}

export default ClerkAuthProvider
