// =============================================================================
// signInRedirect — where Clerk should send the user back to.
//
// The current href with any #fragment stripped. Groflo's deep links
// (/join/<code>, /r/<code>) must round-trip through sign-in, so the path and
// query are preserved; the fragment is dropped so nothing after # leaks into a
// redirect URL that gets logged on the primary.
//
// The same target serves both modes (first-party and satellite): it is always
// an address on this app's own origin.
// =============================================================================

export function signInRedirectTarget(): string {
  const { href, origin, pathname } = window.location
  // The login screen is not a destination: coming back to /login would leave a
  // signed-in user on it. Send them to the app root instead.
  if (pathname === '/login') return `${origin}/`
  const hash = href.indexOf('#')
  return hash === -1 ? href : href.slice(0, hash)
}
