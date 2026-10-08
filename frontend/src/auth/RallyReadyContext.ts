// =============================================================================
// RallyReadyContext — true only beneath a mounted <ClerkProvider>.
//
// Clerk's hooks throw outside their provider. The login screen reads this
// before it renders anything that uses them, so a build with a missing or
// half-set Clerk configuration shows "sign-in is unavailable" and never crashes.
//
// This module imports nothing from @clerk/clerk-react.
// =============================================================================

import { createContext } from 'react'

export const RallyReadyContext = createContext(false)
