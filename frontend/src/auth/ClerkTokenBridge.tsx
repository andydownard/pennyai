// =============================================================================
// ClerkTokenBridge — publishes Clerk's hooks to the plain-module registry.
//
// Lives inside <ClerkProvider>, renders nothing. Its whole job is to hand
// useAuth().getToken and useClerk().signOut to clerkToken.ts so api.ts and
// useAuthState can reach them without importing @clerk/clerk-react.
// =============================================================================

import { useEffect } from 'react'
import { useAuth, useClerk } from '@clerk/clerk-react'
import { setClerkSignOut, setClerkTokenGetter } from './clerkToken'

export function ClerkTokenBridge() {
  const { getToken } = useAuth()
  const clerk = useClerk()

  useEffect(() => {
    setClerkTokenGetter(() => getToken())
    return () => setClerkTokenGetter(null)
  }, [getToken])

  useEffect(() => {
    setClerkSignOut(() => clerk.signOut())
    return () => setClerkSignOut(null)
  }, [clerk])

  return null
}

export default ClerkTokenBridge
