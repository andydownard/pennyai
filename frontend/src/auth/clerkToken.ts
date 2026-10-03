// =============================================================================
// clerkToken — the registry between Clerk's React context and plain modules.
//
// api.ts needs a Clerk token, but api.ts is not a component and must not import
// @clerk/clerk-react: that import is what would pull Clerk into the main bundle
// for every user, including the ones who will never see a Rally sign-in button.
// So ClerkTokenBridge (which IS inside the provider) registers the getter here,
// and everyone else reads it through these functions.
//
// This module imports nothing from @clerk/clerk-react. Nothing here ever throws.
// =============================================================================

type TokenGetter = () => Promise<string | null>
type SignOut = () => Promise<void>
type ExchangeHandler = (error?: string) => void

let tokenGetter: TokenGetter | null = null
let signOutFn: SignOut | null = null
let exchangeHandler: ExchangeHandler | null = null

/**
 * Whether this build has Clerk configured at all. Checked at the call site
 * rather than cached so tests can stub import.meta.env.
 */
export function clerkActive(): boolean {
  return Boolean(import.meta.env.VITE_CLERK_PUBLISHABLE_KEY)
}

/** Registered by ClerkTokenBridge on mount; pass null on unmount. */
export function setClerkTokenGetter(getter: TokenGetter | null) {
  tokenGetter = getter
}

/**
 * The current Clerk session token, or null. Never throws and never rejects:
 * a failure here must degrade to "no Authorization header", not break the
 * request it was decorating.
 */
export async function getClerkToken(): Promise<string | null> {
  if (!tokenGetter) return null
  try {
    return (await tokenGetter()) ?? null
  } catch {
    return null
  }
}

/** Registered by ClerkTokenBridge on mount; pass null on unmount. */
export function setClerkSignOut(fn: SignOut | null) {
  signOutFn = fn
}

/**
 * End the Clerk session. A no-op when Clerk is not mounted.
 *
 * This matters: without it, logging out of Groflo would leave the Clerk session
 * alive, and the very next request would re-authenticate via the Bearer header.
 * The user would be unable to log out.
 */
export async function clerkSignOut(): Promise<void> {
  if (!signOutFn) return
  try {
    await signOutFn()
  } catch {
    // Nothing to do: the local session is already gone.
  }
}

/** Registered by useAuthState so the app can react to a completed exchange. */
export function setClerkExchangeHandler(handler: ExchangeHandler | null) {
  exchangeHandler = handler
}

/** Called by ClerkSessionExchange: no argument on success, a message on failure. */
export function notifyClerkExchanged(error?: string) {
  if (!exchangeHandler) return
  try {
    exchangeHandler(error)
  } catch {
    // A listener throwing must not break the sign-in flow.
  }
}

/** Test-only: clear every registration. */
export function _resetClerkTokenRegistry() {
  tokenGetter = null
  signOutFn = null
  exchangeHandler = null
}
