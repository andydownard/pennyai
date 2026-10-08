// =============================================================================
// clerkToken — the registry between Clerk's React context and plain modules.
//
// api.ts needs a Clerk token, but api.ts is not a component and must not import
// @clerk/clerk-react: Clerk's hooks only work inside its provider, and the
// provider lives in its own lazily-loaded chunk. So ClerkTokenBridge (which IS
// inside the provider) registers the getter here, and everyone else reads it
// through these functions.
//
// This module imports nothing from @clerk/clerk-react. Nothing here ever throws.
// =============================================================================

type TokenGetter = () => Promise<string | null>
type SignOut = () => Promise<void>
type ExchangeHandler = (error?: string) => void
type ExchangeRunner = () => Promise<void>

let tokenGetter: TokenGetter | null = null
let signOutFn: SignOut | null = null
let exchangeHandler: ExchangeHandler | null = null
let exchangeRunner: ExchangeRunner | null = null
let exchangePending = false
const pendingListeners = new Set<() => void>()

// "Resolving" covers the moment after Rally sends the user back: the page has
// loaded but Clerk has not yet said whether there is a session to exchange.
// While it is true the app shows its loading screen instead of the login
// screen, so a returning user goes straight to their list. It starts true only
// when this build carries a Rally key; a cap keeps a failed Clerk load from
// holding the loading screen forever.
const RESOLVE_CAP_MS = 8000
let resolving = Boolean(import.meta.env.VITE_CLERK_PUBLISHABLE_KEY && import.meta.env.VITE_CLERK_SIGN_IN_URL)
const resolvingListeners = new Set<() => void>()
let resolveTimer: ReturnType<typeof setTimeout> | null = resolving
  ? setTimeout(() => setClerkResolving(false), RESOLVE_CAP_MS)
  : null

/** Called once Clerk has decided: no session, nothing to exchange, or the hand-off finished. */
export function setClerkResolving(value: boolean) {
  if (!value && resolveTimer) {
    clearTimeout(resolveTimer)
    resolveTimer = null
  }
  if (resolving === value) return
  resolving = value
  resolvingListeners.forEach(l => {
    try {
      l()
    } catch {
      // A listener throwing must not break the sign-in flow.
    }
  })
}

/** Whether Clerk is still deciding if a returning user is signed in. */
export function clerkResolving(): boolean {
  return resolving
}

/** useSyncExternalStore-shaped subscription to clerkResolving. */
export function subscribeClerkResolving(listener: () => void): () => void {
  resolvingListeners.add(listener)
  return () => {
    resolvingListeners.delete(listener)
  }
}

/**
 * Whether this build carries a Rally (Clerk) key. Checked at the call site
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
 * This matters: without it, signing out of Groflo would leave the Rally session
 * alive, and the next person to tap "Continue with Rally" on a shared device
 * would be signed straight back in as the last one.
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

/** Registered by ClerkSessionExchange while a Clerk session is signed in. */
export function setClerkExchangeRunner(runner: ExchangeRunner | null) {
  exchangeRunner = runner
}

/**
 * Trade the live Clerk session for a Groflo session because the user asked to.
 *
 * The automatic exchange runs once per Clerk session. This is the explicit
 * path for everything after that: a failed exchange, or a Groflo session that
 * expired while the Clerk one is still alive. A no-op when Clerk is not mounted
 * or nobody is signed in. Never throws.
 */
export async function runClerkExchange(): Promise<void> {
  if (!exchangeRunner) return
  try {
    await exchangeRunner()
  } catch {
    // The runner reports its own failures through notifyClerkExchanged.
  }
}

/** Called by ClerkSessionExchange around the request. */
export function setClerkExchangePending(pending: boolean) {
  if (exchangePending === pending) return
  exchangePending = pending
  pendingListeners.forEach(l => {
    try {
      l()
    } catch {
      // A listener throwing must not break the sign-in flow.
    }
  })
}

/** Whether an exchange request is in flight. Pair with subscribeClerkExchangePending. */
export function clerkExchangePending(): boolean {
  return exchangePending
}

/** useSyncExternalStore-shaped subscription to clerkExchangePending. */
export function subscribeClerkExchangePending(listener: () => void): () => void {
  pendingListeners.add(listener)
  return () => {
    pendingListeners.delete(listener)
  }
}

/** Test-only: clear every registration. */
export function _resetClerkTokenRegistry() {
  tokenGetter = null
  signOutFn = null
  exchangeHandler = null
  exchangeRunner = null
  exchangePending = false
  pendingListeners.clear()
  if (resolveTimer) clearTimeout(resolveTimer)
  resolveTimer = null
  resolving = false
  resolvingListeners.clear()
}
