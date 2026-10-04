// =============================================================================
// pendingLinks — where a visitor was going before they had to sign in.
//
// Sign-in happens on the Rally primary, so the browser leaves Groflo and comes
// back. React state does not survive that trip. Everything a signed-out visitor
// was on their way to is therefore kept in web storage and read back once the
// session exists:
//
//   /join/<code>     household invite   localStorage  groflo_join_code
//   /r/<code>        Share & Earn link  localStorage  referral_code
//   /shared/<code>   shared list invite localStorage  groflo_share_invite (+ _name)
//   any app screen   deep link          sessionStorage groflo_redirect
//
// The same landing rule runs whether the session was found on page load or was
// just created by the Rally exchange, so both paths end in the same place.
// =============================================================================

export const JOIN_CODE_KEY = 'groflo_join_code'
export const REFERRAL_CODE_KEY = 'referral_code'
export const SHARE_INVITE_KEY = 'groflo_share_invite'
export const SHARE_NAME_KEY = 'groflo_share_name'
export const REDIRECT_KEY = 'groflo_redirect'

function safeGet(storage: Storage, key: string): string | null {
  try {
    return storage.getItem(key)
  } catch {
    return null
  }
}

function safeSet(storage: Storage, key: string, value: string) {
  try {
    storage.setItem(key, value)
  } catch {
    // Storage blocked: the link is lost, the app still works.
  }
}

function safeRemove(storage: Storage, key: string) {
  try {
    storage.removeItem(key)
  } catch {
    // Ignore: see safeSet.
  }
}

/**
 * Remember a /join/<code> or /r/<code> link. Returns which kind was stored, or
 * null when the path is neither.
 */
export function stashInviteLink(pathname: string): 'join' | 'referral' | null {
  const join = pathname.match(/^\/join\/(.+)$/)
  if (join) {
    safeSet(localStorage, JOIN_CODE_KEY, join[1])
    return 'join'
  }
  const referral = pathname.match(/^\/r\/(.+)$/)
  if (referral) {
    safeSet(localStorage, REFERRAL_CODE_KEY, referral[1])
    return 'referral'
  }
  return null
}

/** Remember a shared list's household invite for the prompt after sign-in. */
export function stashShareInvite(inviteCode: string, sharerName: string) {
  safeSet(localStorage, SHARE_INVITE_KEY, inviteCode)
  safeSet(localStorage, SHARE_NAME_KEY, sharerName)
}

/**
 * A signed-out visitor asked for `pathname`. If it is a real app screen (not
 * the login screen itself), remember it for after sign-in.
 */
export function rememberDestination<S extends string>(
  pathname: string,
  pathToScreen: Record<string, S>,
  signedOutScreens: readonly S[],
) {
  const resolved = pathToScreen[pathname]
  if (resolved && !signedOutScreens.includes(resolved)) {
    safeSet(sessionStorage, REDIRECT_KEY, pathname)
  }
}

export interface Landing<S extends string> {
  screen: S
  /** Set when a shared-list invite is waiting: show the join prompt on home. */
  joinPrompt?: { code: string; name: string }
}

/**
 * Where a newly signed-in user lands. Consumes what it reads, so calling it
 * twice cannot show the join prompt twice.
 *
 *   1. a pending shared-list invite → home, with the join prompt
 *   2. the URL itself, when it names an app screen
 *   3. the destination remembered before sign-in
 *   4. home
 */
export function consumeLanding<S extends string>(
  pathname: string,
  pathToScreen: Record<string, S>,
  signedOutScreens: readonly S[],
  home: S,
): Landing<S> {
  const shareInvite = safeGet(localStorage, SHARE_INVITE_KEY)
  if (shareInvite) {
    const name = safeGet(localStorage, SHARE_NAME_KEY) || ''
    safeRemove(localStorage, SHARE_INVITE_KEY)
    safeRemove(localStorage, SHARE_NAME_KEY)
    return { screen: home, joinPrompt: { code: shareInvite, name } }
  }

  const here = pathToScreen[pathname]
  if (here && !signedOutScreens.includes(here)) {
    return { screen: here }
  }

  const redirect = safeGet(sessionStorage, REDIRECT_KEY)
  if (redirect) {
    safeRemove(sessionStorage, REDIRECT_KEY)
    return { screen: pathToScreen[redirect] || home }
  }

  return { screen: home }
}
