// =============================================================================
// signInRedirect — where Clerk should send the user back to.
//
// The current href with any #fragment stripped. Groflo's deep links
// (/join/<code>, /r/<code>) must round-trip through sign-in, so the path and
// query are preserved; the fragment is dropped so nothing after # leaks into a
// redirect URL that gets logged on the primary.
// =============================================================================

export function signInRedirectTarget(): string {
  const { href } = window.location
  const hash = href.indexOf('#')
  return hash === -1 ? href : href.slice(0, hash)
}
