// =============================================================================
// Centralized API layer
//
// Exports fetchAPI (returns JSON or null) and fetchAction (returns {ok, status})
// with automatic 401 session expiry handling.
// =============================================================================

import { clerkActive, getClerkToken } from './auth/clerkToken'

type SessionExpiredHandler = () => void

let onSessionExpired: SessionExpiredHandler = () => {}

/** Register a callback for 401 session expiry. Call once at app init. */
export function setSessionExpiredHandler(handler: SessionExpiredHandler) {
  onSessionExpired = handler
}

/**
 * Attach the Rally (Clerk) session token as a Bearer header when a token is
 * available.
 *
 * In a build without Rally configured this returns the caller's options object
 * UNCHANGED — not a copy — and never awaits a getter. A caller-supplied
 * Authorization header is never overwritten.
 */
async function withAuthHeader(options?: RequestInit): Promise<RequestInit | undefined> {
  if (!clerkActive()) return options

  const headers = new Headers(options?.headers)
  if (headers.has('Authorization')) return options

  const token = await getClerkToken()
  if (!token) return options

  headers.set('Authorization', `Bearer ${token}`)
  return { ...options, headers }
}

/**
 * Fetch wrapper that returns parsed JSON on success, or null on failure.
 * Automatically calls onSessionExpired on 401.
 */
export async function fetchAPI<T = Record<string, unknown>>(
  url: string,
  options?: RequestInit,
): Promise<T | null> {
  let res: Response
  try {
    res = await fetch(url, await withAuthHeader(options))
  } catch {
    return null // network error
  }
  if (res.status === 401) { onSessionExpired(); return null }
  if (!res.ok) return null
  try {
    return await res.json()
  } catch {
    return null // invalid JSON
  }
}

export interface ActionResult {
  ok: boolean
  status: number
  error?: string
}

/**
 * Fetch wrapper for actions that don't need a JSON body back (delete, clear, etc.).
 * Returns { ok, status, error? }. Automatically calls onSessionExpired on 401.
 */
export async function fetchAction(
  url: string,
  options?: RequestInit,
): Promise<ActionResult> {
  let res: Response
  try {
    res = await fetch(url, await withAuthHeader(options))
  } catch {
    return { ok: false, status: 0, error: 'Network error' }
  }
  if (res.status === 401) { onSessionExpired(); return { ok: false, status: 401 } }
  if (!res.ok) {
    const errData = await res.json().catch(() => null)
    return { ok: false, status: res.status, error: errData?.error || `Error ${res.status}` }
  }
  return { ok: true, status: res.status }
}
