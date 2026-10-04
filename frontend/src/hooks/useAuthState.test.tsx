// Sign-in and sign-out, as the app sees them.
//
// Sign-out must end BOTH sessions — Groflo's and Rally's — and leave the user
// on the login screen for good: no automatic exchange afterwards, no loop.
// Sign-in must hand /auth/me to the app so a pending invite is honoured.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act, render, renderHook, waitFor } from '@testing-library/react'

let clerkAuth = { isLoaded: true, isSignedIn: true, sessionId: 'sess_1' }
vi.mock('@clerk/clerk-react', () => ({
  useAuth: () => clerkAuth,
}))

import { useAuthState, type MeResponse } from './useAuthState'
import { ClerkSessionExchange } from '../auth/ClerkSessionExchange'
import {
  _resetClerkTokenRegistry,
  notifyClerkExchanged,
  setClerkSignOut,
} from '../auth/clerkToken'

interface Call { url: string; method: string }
let calls: Call[] = []
let failLogout = false
let me: { status: number; body: unknown } = { status: 401, body: { error: 'Not logged in' } }
let config: unknown = { clerk_enabled: true }

function json(status: number, body: unknown) {
  return { ok: status >= 200 && status < 300, status, json: async () => body } as Response
}

const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
  const url = String(input)
  calls.push({ url, method: init?.method || 'GET' })
  if (url === '/api/config') return json(200, config)
  if (url === '/auth/logout') {
    if (failLogout) throw new Error('offline')
    return json(200, { success: true })
  }
  if (url === '/auth/me') return json(me.status, me.body)
  if (url === '/auth/clerk/exchange') return json(200, { success: true, user_id: 'u1' })
  return json(404, {})
})

const posts = (url: string) => calls.filter(c => c.url === url && c.method === 'POST').length
const settle = () => new Promise(r => setTimeout(r, 10))

function setup() {
  const setScreen = vi.fn()
  const showToast = vi.fn()
  const onSignedIn = vi.fn()
  const hook = renderHook(() => useAuthState({ setScreen, showToast, onSignedIn }))
  return { ...hook, setScreen, showToast, onSignedIn }
}

describe('useAuthState', () => {
  beforeEach(() => {
    calls = []
    failLogout = false
    me = { status: 401, body: { error: 'Not logged in' } }
    config = { clerk_enabled: true }
    clerkAuth = { isLoaded: true, isSignedIn: true, sessionId: 'sess_1' }
    sessionStorage.clear()
    _resetClerkTokenRegistry()
    fetchMock.mockClear()
    vi.stubGlobal('fetch', fetchMock)
    vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', '')
  })
  afterEach(async () => {
    await settle()
    vi.unstubAllGlobals()
    vi.unstubAllEnvs()
  })

  describe('sign-out', () => {
    it('ends the Groflo session, then the Rally session, then shows login', async () => {
      const order: string[] = []
      const { result, setScreen } = setup()
      setClerkSignOut(async () => { order.push(posts('/auth/logout') === 1 ? 'rally-after-logout' : 'rally-too-early') })
      setScreen.mockImplementation((s: string) => { order.push(`screen:${s}`) })

      await act(async () => { await result.current.handleLogout() })

      expect(posts('/auth/logout')).toBe(1)
      expect(order).toEqual(['rally-after-logout', 'screen:login'])
      expect(setScreen).toHaveBeenCalledWith('login', true)
    })

    it('calls none of the retired sign-in endpoints', async () => {
      const { result } = setup()
      await act(async () => { await result.current.handleLogout() })
      await settle()
      for (const { url } of calls) {
        expect(url).not.toMatch(/send-code|verify-code|\/auth\/google|\/auth\/email|auth\/link/)
      }
    })

    it('still reaches the login screen when Rally sign-out fails', async () => {
      const { result, setScreen } = setup()
      setClerkSignOut(async () => { throw new Error('network') })
      await act(async () => { await result.current.handleLogout() })
      expect(setScreen).toHaveBeenCalledWith('login', true)
    })

    it('stays put and says so when the Groflo session could not be ended', async () => {
      failLogout = true
      const signOut = vi.fn(async () => {})
      const { result, setScreen, showToast } = setup()
      setClerkSignOut(signOut)

      await act(async () => { await result.current.handleLogout() })

      expect(signOut).not.toHaveBeenCalled()
      expect(setScreen).not.toHaveBeenCalled()
      expect(showToast).toHaveBeenCalledWith(expect.stringMatching(/sign out/i), 'error')
    })

    it('a double tap signs out once', async () => {
      const signOut = vi.fn(async () => {})
      const { result } = setup()
      setClerkSignOut(signOut)
      await act(async () => {
        await Promise.all([result.current.handleLogout(), result.current.handleLogout()])
      })
      expect(posts('/auth/logout')).toBe(1)
      expect(signOut).toHaveBeenCalledTimes(1)
    })

    it('logout → login is stable: nothing exchanges again and the screen stays on login', async () => {
      // A signed-in Rally session is exchanged once on arrival.
      me = { status: 200, body: { logged_in: true, user_id: 7 } }
      const { result, setScreen, onSignedIn } = setup()
      const exchange = render(<ClerkSessionExchange />)
      await waitFor(() => expect(onSignedIn).toHaveBeenCalledTimes(1))
      expect(posts('/auth/clerk/exchange')).toBe(1)

      // Sign out. Rally reports the session ended.
      setClerkSignOut(async () => {
        clerkAuth = { isLoaded: true, isSignedIn: false, sessionId: '' }
        exchange.rerender(<ClerkSessionExchange />)
      })
      me = { status: 401, body: { error: 'Not logged in' } }
      await act(async () => { await result.current.handleLogout() })
      await settle()

      expect(setScreen).toHaveBeenLastCalledWith('login', true)
      expect(setScreen.mock.calls.filter(c => c[0] === 'login')).toHaveLength(1)
      expect(posts('/auth/clerk/exchange')).toBe(1)
      expect(onSignedIn).toHaveBeenCalledTimes(1)
    })

    it('logout → login is stable even if the Rally session survives a reload', async () => {
      me = { status: 200, body: { logged_in: true, user_id: 7 } }
      const { result, setScreen, onSignedIn } = setup()
      const first = render(<ClerkSessionExchange />)
      await waitFor(() => expect(onSignedIn).toHaveBeenCalledTimes(1))

      // Rally sign-out fails; the same Clerk session is still alive.
      setClerkSignOut(async () => { throw new Error('network') })
      await act(async () => { await result.current.handleLogout() })

      // Reload: the exchange component mounts again with the same session.
      first.unmount()
      render(<ClerkSessionExchange />)
      await settle()

      expect(posts('/auth/clerk/exchange')).toBe(1)
      expect(onSignedIn).toHaveBeenCalledTimes(1)
      expect(setScreen).toHaveBeenLastCalledWith('login', true)
    })
  })

  describe('sign-in', () => {
    it('hands /auth/me to the app once the exchange succeeds', async () => {
      const body: MeResponse = { logged_in: true, user_id: 7, name: 'Tamika Jones', needs_sms_consent: true, phone_number: '2125550101' }
      me = { status: 200, body }
      const { onSignedIn } = setup()

      act(() => notifyClerkExchanged())

      await waitFor(() => expect(onSignedIn).toHaveBeenCalledWith(body))
    })

    it('shows the server\'s message when the exchange fails, and does not enter the app', async () => {
      const { result, onSignedIn } = setup()
      act(() => notifyClerkExchanged('This account is already connected to a different Rally sign-in.'))
      await waitFor(() => expect(result.current.error).toMatch(/already connected/))
      expect(onSignedIn).not.toHaveBeenCalled()
    })

    it('does not enter the app if the session did not take', async () => {
      const { result, onSignedIn } = setup()
      act(() => notifyClerkExchanged())
      await waitFor(() => expect(result.current.error).toMatch(/could not be completed/i))
      expect(onSignedIn).not.toHaveBeenCalled()
    })
  })

  describe('/api/config', () => {
    it('is unknown (null) until the server answers, then true', async () => {
      const { result } = setup()
      expect(result.current.clerkEnabled).toBeNull()
      await waitFor(() => expect(result.current.clerkEnabled).toBe(true))
    })

    it('is false when the server says clerk_enabled: false', async () => {
      config = { clerk_enabled: false }
      const { result } = setup()
      await waitFor(() => expect(result.current.clerkEnabled).toBe(false))
    })

    it('is false when the field is absent', async () => {
      config = {}
      const { result } = setup()
      await waitFor(() => expect(result.current.clerkEnabled).toBe(false))
    })
  })
})
