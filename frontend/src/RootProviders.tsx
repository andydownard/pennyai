// =============================================================================
// RootProviders — the one place that decides whether Clerk loads at all.
//
// With VITE_CLERK_PUBLISHABLE_KEY unset this renders exactly what main.tsx used
// to render: <React.StrictMode>{children}</React.StrictMode>. The dynamic import
// is never evaluated, so @clerk/clerk-react never reaches the entry bundle and
// index.html never preloads it.
// =============================================================================

import React, { Suspense, lazy, type ReactNode } from 'react'

const ClerkAuthProvider = lazy(() => import('./auth/ClerkAuthProvider'))

interface RootProvidersProps {
  children: ReactNode
}

export function RootProviders({ children }: RootProvidersProps) {
  const clerkKey = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY

  if (!clerkKey) {
    return <React.StrictMode>{children}</React.StrictMode>
  }

  return (
    <React.StrictMode>
      <Suspense fallback={null}>
        <ClerkAuthProvider>{children}</ClerkAuthProvider>
      </Suspense>
    </React.StrictMode>
  )
}

export default RootProviders
