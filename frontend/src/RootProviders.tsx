// =============================================================================
// RootProviders — wraps the app in the Rally (Clerk) sign-in provider.
//
// Rally is the one way into Groflo, so every production build carries
// VITE_CLERK_PUBLISHABLE_KEY. The provider is still loaded lazily: Clerk stays
// in its own chunk, off the entry bundle.
//
// Two things can go wrong here, and neither may blank the page:
//   - no key at build time      → render the app without Clerk
//   - no sign-in URL            → same (ClerkAuthProvider declines to mount)
//   - the provider chunk fails  → same (ProviderBoundary catches it)
// Either way RallyReadyContext stays false and the login screen shows a calm
// "sign-in is briefly unavailable" message.
// =============================================================================

import React, { Component, Suspense, lazy, type ReactNode } from 'react'

const ClerkAuthProvider = lazy(() => import('./auth/ClerkAuthProvider'))

interface RootProvidersProps {
  children: ReactNode
}

interface ProviderBoundaryProps {
  fallback: ReactNode
  children: ReactNode
}

/** Renders `fallback` (the bare app) if the sign-in provider cannot load. */
class ProviderBoundary extends Component<ProviderBoundaryProps, { failed: boolean }> {
  state = { failed: false }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  componentDidCatch(error: unknown) {
    // eslint-disable-next-line no-console
    console.warn('Rally sign-in failed to load. Continuing without it.', error)
  }

  render() {
    return this.state.failed ? this.props.fallback : this.props.children
  }
}

export function RootProviders({ children }: RootProvidersProps) {
  const clerkKey = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY

  if (!clerkKey) {
    return <React.StrictMode>{children}</React.StrictMode>
  }

  return (
    <React.StrictMode>
      <ProviderBoundary fallback={children}>
        <Suspense fallback={null}>
          <ClerkAuthProvider>{children}</ClerkAuthProvider>
        </Suspense>
      </ProviderBoundary>
    </React.StrictMode>
  )
}

export default RootProviders
