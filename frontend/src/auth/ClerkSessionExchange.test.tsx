// ClerkSessionExchange posts to /auth/clerk/exchange exactly once per Clerk
// session, and the guard is sessionStorage — not React state — so StrictMode's
// deliberate double mount cannot produce a second POST. (CLAUDE.md: never rely
// on React state alone for a one-shot action.)
//
// It is also what keeps sign-out stable: once a Clerk session has been
// exchanged, nothing exchanges it again unless the user asks.

import React from 'react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, waitFor, act } from '@testing-library/react'

let authState = { isLoaded: true, isSignedIn: true, sessionId: 'sess_1' }

vi.mock('@clerk/clerk-react', () => ({
  useAuth: () => authState,
}))

const fetchActionMock = vi.fn()
vi.mock('../api', () => ({
  fetchAction: (url: string, options?: RequestInit) => fetchActionMock(url, options),
}))

import { ClerkSessionExchange } from './ClerkSessionExchange'
import {
  _resetClerkTokenRegistry,
  clerkExchangePending,
  runClerkExchange,
  setClerkExchangeHandler,
  clerkResolving,
  setClerkResolving,
} from './clerkToken'

const GUARD = 'groflo.clerk_exchanged:sess_1'
const notified = vi.fn()
const settle = () => new Promise(r => setTimeout(r, 10))

describe('ClerkSessionExchange', () => {
  beforeEach(() => {
    sessionStorage.clear()
    _resetClerkTokenRegistry()
    setClerkExchangeHandler(notified)
    fetchActionMock.mockReset()
    fetchActionMock.mockResolvedValue({ ok: true, status: 200 })
    notified.mockClear()
    authState = { isLoaded: true, isSignedIn: true, sessionId: 'sess_1' }
  })
  afterEach(async () => {
    await settle() // let any in-flight exchange finish before the next test
    sessionStorage.clear()
  })

  it('posts once and notifies on success', async () => {
    render(<ClerkSessionExchange />)
    await waitFor(() => expect(fetchActionMock).toHaveBeenCalledTimes(1))
    expect(fetchActionMock).toHaveBeenCalledWith('/auth/clerk/exchange', { method: 'POST' })
    await waitFor(() => expect(notified).toHaveBeenCalledWith(undefined))
    expect(sessionStorage.getItem(GUARD)).not.toBeNull()
  })

  it('posts exactly once across a StrictMode double mount, and still notifies', async () => {
    render(
      <React.StrictMode>
        <ClerkSessionExchange />
      </React.StrictMode>,
    )
    await waitFor(() => expect(fetchActionMock).toHaveBeenCalledTimes(1))
    // Let any second effect pass run before asserting the count is still one.
    await settle()
    expect(fetchActionMock).toHaveBeenCalledTimes(1)
    // The first mount's cleanup must not swallow the result.
    expect(notified).toHaveBeenCalledTimes(1)
    expect(notified).toHaveBeenCalledWith(undefined)
  })

  it('does not post when the guard is already set', async () => {
    sessionStorage.setItem(GUARD, '1')
    render(<ClerkSessionExchange />)
    await settle()
    expect(fetchActionMock).not.toHaveBeenCalled()
  })

  it('clears the guard and reports the server message on failure', async () => {
    fetchActionMock.mockResolvedValue({
      ok: false,
      status: 409,
      error: 'This account is already connected to a different Rally sign-in.',
    })
    render(<ClerkSessionExchange />)
    await waitFor(() =>
      expect(notified).toHaveBeenCalledWith(
        'This account is already connected to a different Rally sign-in.',
      ),
    )
    // Guard removed so a later reload can retry.
    expect(sessionStorage.getItem(GUARD)).toBeNull()
  })

  it('falls back to its own message when the server sends none', async () => {
    fetchActionMock.mockResolvedValue({ ok: false, status: 0 })
    render(<ClerkSessionExchange />)
    await waitFor(() => expect(notified).toHaveBeenCalled())
    expect(notified.mock.calls[0][0]).toMatch(/could not be completed/i)
  })

  it('does nothing until Clerk has loaded', async () => {
    authState = { isLoaded: false, isSignedIn: false, sessionId: '' }
    render(<ClerkSessionExchange />)
    await settle()
    expect(fetchActionMock).not.toHaveBeenCalled()
  })

  it('does nothing when signed out', async () => {
    authState = { isLoaded: true, isSignedIn: false, sessionId: '' }
    render(<ClerkSessionExchange />)
    await settle()
    expect(fetchActionMock).not.toHaveBeenCalled()
  })

  it('uses a per-session guard key, so a new Clerk session exchanges again', async () => {
    sessionStorage.setItem('groflo.clerk_exchanged:sess_OLD', '1')
    render(<ClerkSessionExchange />)
    await waitFor(() => expect(fetchActionMock).toHaveBeenCalledTimes(1))
  })

  it('renders nothing', () => {
    const { container } = render(<ClerkSessionExchange />)
    expect(container.firstChild).toBeNull()
  })

  it('reports the request as pending while it is in flight', async () => {
    let finish: (v: unknown) => void = () => {}
    fetchActionMock.mockReturnValue(new Promise(r => { finish = r }))
    render(<ClerkSessionExchange />)
    await waitFor(() => expect(clerkExchangePending()).toBe(true))
    finish({ ok: true, status: 200 })
    await waitFor(() => expect(clerkExchangePending()).toBe(false))
  })

  describe('after sign-out', () => {
    it('does not exchange again once the Clerk session has ended', async () => {
      const { rerender } = render(<ClerkSessionExchange />)
      await waitFor(() => expect(notified).toHaveBeenCalledTimes(1))

      // Sign-out: Clerk reports no session.
      authState = { isLoaded: true, isSignedIn: false, sessionId: '' }
      rerender(<ClerkSessionExchange />)
      await settle()
      expect(fetchActionMock).toHaveBeenCalledTimes(1)

      // Even an explicit request has nothing to trade.
      await runClerkExchange()
      expect(fetchActionMock).toHaveBeenCalledTimes(1)
    })

    it('does not sign the user back in if ending the Clerk session failed', async () => {
      // First mount: the session is exchanged. Then the user signs out of
      // Groflo, Clerk sign-out fails, and the page reloads with the same Clerk
      // session still alive. A second automatic exchange here would be a loop.
      const first = render(<ClerkSessionExchange />)
      await waitFor(() => expect(notified).toHaveBeenCalledTimes(1))
      first.unmount()

      render(<ClerkSessionExchange />)
      await settle()
      expect(fetchActionMock).toHaveBeenCalledTimes(1)
      expect(notified).toHaveBeenCalledTimes(1)
    })

    it('exchanges again only when the user asks', async () => {
      render(<ClerkSessionExchange />)
      await waitFor(() => expect(notified).toHaveBeenCalledTimes(1))

      await act(async () => { await runClerkExchange() })
      expect(fetchActionMock).toHaveBeenCalledTimes(2)
      expect(notified).toHaveBeenCalledTimes(2)
    })

    it('joins a request already in flight instead of posting twice', async () => {
      let finish: (v: unknown) => void = () => {}
      fetchActionMock.mockReturnValue(new Promise(r => { finish = r }))
      render(<ClerkSessionExchange />)
      await waitFor(() => expect(fetchActionMock).toHaveBeenCalledTimes(1))

      const asked = runClerkExchange()
      finish({ ok: true, status: 200 })
      await asked
      expect(fetchActionMock).toHaveBeenCalledTimes(1)
    })
  })
})

// The loading screen stays up while Clerk decides; these pin when it stops.
describe('ClerkSessionExchange resolving signal', () => {
  beforeEach(() => {
    sessionStorage.clear()
    _resetClerkTokenRegistry()
    setClerkExchangeHandler(notified)
    fetchActionMock.mockReset()
    fetchActionMock.mockResolvedValue({ ok: true, status: 200 })
    setClerkResolving(true)
  })
  afterEach(async () => {
    await settle()
    sessionStorage.clear()
  })

  it('stops resolving when Clerk loads with no session', async () => {
    authState = { isLoaded: true, isSignedIn: false, sessionId: '' } as typeof authState
    render(<ClerkSessionExchange />)
    await waitFor(() => expect(clerkResolving()).toBe(false))
    expect(fetchActionMock).not.toHaveBeenCalled()
  })

  it('keeps resolving while Clerk is still loading', async () => {
    authState = { isLoaded: false, isSignedIn: false, sessionId: '' } as typeof authState
    render(<ClerkSessionExchange />)
    await settle()
    expect(clerkResolving()).toBe(true)
  })

  it('keeps resolving during a first exchange (the handler ends it after entering the app)', async () => {
    authState = { isLoaded: true, isSignedIn: true, sessionId: 'sess_1' }
    render(<ClerkSessionExchange />)
    await waitFor(() => expect(fetchActionMock).toHaveBeenCalledTimes(1))
    expect(clerkResolving()).toBe(true)
  })

  it('stops resolving when this session was already exchanged (signed out since)', async () => {
    sessionStorage.setItem(GUARD, '1')
    authState = { isLoaded: true, isSignedIn: true, sessionId: 'sess_1' }
    render(<ClerkSessionExchange />)
    await waitFor(() => expect(clerkResolving()).toBe(false))
    expect(fetchActionMock).not.toHaveBeenCalled()
  })
})
