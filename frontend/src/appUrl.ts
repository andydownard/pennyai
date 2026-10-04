// =============================================================================
// appUrl — the address this app is being served from.
//
// Share links, referral links and QR codes must point at wherever the user is
// actually using Groflo, so they are built from the live origin, never a
// hard-coded host.
// =============================================================================

/** e.g. "https://groflo.rallytasks.com" — no trailing slash. */
export function appOrigin(): string {
  return window.location.origin
}

/** e.g. "groflo.rallytasks.com" — for display where the scheme is noise. */
export function appHost(): string {
  return window.location.host
}

/** The Share & Earn link for a referral code. */
export function referralLink(code: string, via?: string): string {
  return `${appOrigin()}/r/${code}${via ? `?via=${via}` : ''}`
}
