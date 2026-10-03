// ClerkSessionExchange posts to /auth/clerk/exchange exactly once per Clerk
// session, and the guard is sessionStorage — not React state — so StrictMode's
// deliberate double mount cannot produce a second POST. (CLAUDE.md: never rely
// on React state alone for a one-shot action.)

import React from 'react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, waitFor } from '@testing-library/react'

let authState = { isLoaded: true, isSignedIn: true, sessionId: 'sess_1' }

vi.mock('@clerk/clerk-react', () => ({
  useAuth: () => authState,
}))

const notifyClerkExchanged = vi.fn()
vi.mock('./clerkToken', () => ({
  notifyClerkExchanged: (e?: string) => notifyClerkExchanged(e),
}))

const fetchActionMock = vi.fn()
vi.mock('../api', () => ({
  fetchAction: (url: string, options?: RequestInit) => fetchActionMock(url, options),
}))

import { ClerkSessionExchange } from './ClerkSessionExchange'

const GUARD = 'groflo.clerk_exchanged:sess_1'

describe('ClerkSessionExchange', () => {
  beforeEach(() => {
    sessionStorage.clear()
    fetchActionMock.mockReset()
    fetchActionMock.mockResolvedValue({ ok: true, status: 200 })
    notifyClerkExchanged.mockClear()
    authState = { isLoaded: true, isSignedIn: true, sessionId: 'sess_1' }
  })
  afterEach(() => sessionStorage.clear())

  it('posts once and notifies on success', async () => {
    render(<ClerkSessionExchange />)
    await waitFor(() => expect(fetchActionMock).toHaveBeenCalledTimes(1))
    expect(fetchActionMock).toHaveBeenCalledWith('/auth/clerk/exchange', { method: 'POST' })
    await waitFor(() => expect(notifyClerkExchanged).toHaveBeenCalledWith(undefined))
    expect(sessionStorage.getItem(GUARD)).not.toBeNull()
  })

  it('posts exactly once across a StrictMode double mount', async () => {
    render(
      <React.StrictMode>
        <ClerkSessionExchange />
      </React.StrictMode>,
    )
    await waitFor(() => expect(fetchActionMock).toHaveBeenCalledTimes(1))
    // Let any second effect pass run before asserting the count is still one.
    await new Promise(r => setTimeout(r, 10))
    expect(fetchActionMock).toHaveBeenCalledTimes(1)
  })

  it('does not post when the guard is already set', async () => {
    sessionStorage.setItem(GUARD, '1')
    render(<ClerkSessionExchange />)
    await new Promise(r => setTimeout(r, 10))
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
      expect(notifyClerkExchanged).toHaveBeenCalledWith(
        'This account is already connected to a different Rally sign-in.',
      ),
    )
    // Guard removed so a later reload can retry.
    expect(sessionStorage.getItem(GUARD)).toBeNull()
  })

  it('falls back to its own message when the server sends none', async () => {
    fetchActionMock.mockResolvedValue({ ok: false, status: 0 })
    render(<ClerkSessionExchange />)
    await waitFor(() => expect(notifyClerkExchanged).toHaveBeenCalled())
    expect(notifyClerkExchanged.mock.calls[0][0]).toMatch(/could not be completed/i)
  })

  it('does nothing until Clerk has loaded', async () => {
    authState = { isLoaded: false, isSignedIn: false, sessionId: '' }
    render(<ClerkSessionExchange />)
    await new Promise(r => setTimeout(r, 10))
    expect(fetchActionMock).not.toHaveBeenCalled()
  })

  it('does nothing when signed out', async () => {
    authState = { isLoaded: true, isSignedIn: false, sessionId: '' }
    render(<ClerkSessionExchange />)
    await new Promise(r => setTimeout(r, 10))
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
})
