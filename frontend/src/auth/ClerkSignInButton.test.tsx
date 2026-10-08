// "Continue with Rally" must build its destination through Clerk — never a
// hand-built URL, because Clerk adds what the current mode needs (first-party
// or satellite) — and must round-trip the user back to the page they were on.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'

let authState = { isLoaded: true, isSignedIn: false }
const buildSignInUrl = vi.fn((_opts?: unknown) => 'https://primary.example/sign-in?built=1')

vi.mock('@clerk/clerk-react', () => ({
  useClerk: () => ({ buildSignInUrl }),
  useAuth: () => authState,
}))

import ClerkSignInButton, { RALLY_LOAD_TIMEOUT_MS } from './ClerkSignInButton'
import {
  _resetClerkTokenRegistry,
  setClerkExchangePending,
  setClerkExchangeRunner,
  setClerkSignOut,
} from './clerkToken'

function setHref(href: string) {
  Object.defineProperty(window, 'location', {
    value: { ...new URL(href), href, assign: vi.fn() },
    writable: true,
    configurable: true,
  })
}

describe('ClerkSignInButton', () => {
  beforeEach(() => {
    authState = { isLoaded: true, isSignedIn: false }
    buildSignInUrl.mockClear()
    _resetClerkTokenRegistry()
    setHref('https://app.groflo.io/join/ABC?x=1#frag')
  })
  afterEach(() => vi.useRealTimers())

  it('renders a button whose copy names Rally and nothing else', () => {
    render(<ClerkSignInButton />)
    const button = screen.getByRole('button')
    expect(button).toHaveTextContent('Continue with Rally')
    // Penny's vocabulary rules: never "SSO", "bot", "AI", "assistant".
    for (const banned of ['SSO', 'bot', 'AI', 'assistant', 'Clerk']) {
      expect(button.textContent).not.toContain(banned)
    }
  })

  it('is a thumb-sized target', () => {
    render(<ClerkSignInButton />)
    const button = screen.getByRole('button')
    expect(parseInt(button.style.minHeight, 10)).toBeGreaterThanOrEqual(48)
    expect(button.style.width).toBe('100%')
  })

  it('navigates to the URL Clerk builds, with the fragment stripped', () => {
    render(<ClerkSignInButton />)
    screen.getByRole('button').click()

    expect(buildSignInUrl).toHaveBeenCalledWith({ redirectUrl: 'https://app.groflo.io/join/ABC?x=1' })
    expect(window.location.assign).toHaveBeenCalledWith('https://primary.example/sign-in?built=1')
  })

  describe('while Rally is loading', () => {
    beforeEach(() => {
      authState = { isLoaded: false, isSignedIn: false }
    })

    it('shows the button disabled, so the screen is never blank', () => {
      render(<ClerkSignInButton />)
      const button = screen.getByRole('button', { name: 'Continue with Rally' })
      expect(button).toBeDisabled()
      button.click()
      expect(buildSignInUrl).not.toHaveBeenCalled()
    })

    it('says sign-in is unavailable if Rally never loads', () => {
      vi.useFakeTimers()
      render(<ClerkSignInButton />)
      act(() => { vi.advanceTimersByTime(RALLY_LOAD_TIMEOUT_MS + 1) })
      expect(screen.getByText('Sign-in is briefly unavailable.')).toBeInTheDocument()
      expect(screen.queryByRole('button')).not.toBeInTheDocument()
    })
  })

  describe('signed in to Rally, not yet to Groflo', () => {
    beforeEach(() => {
      authState = { isLoaded: true, isSignedIn: true }
    })

    it('says so while the session is being traded', () => {
      setClerkExchangePending(true)
      render(<ClerkSignInButton />)
      const button = screen.getByRole('button')
      expect(button).toHaveTextContent('Signing you in…')
      expect(button).toBeDisabled()
    })

    it('retries the trade on tap, without a trip to Rally', () => {
      const runner = vi.fn(async () => {})
      setClerkExchangeRunner(runner)
      render(<ClerkSignInButton />)
      screen.getByRole('button', { name: 'Continue with Rally' }).click()
      expect(runner).toHaveBeenCalledTimes(1)
      expect(buildSignInUrl).not.toHaveBeenCalled()
      expect(window.location.assign).not.toHaveBeenCalled()
    })

    it('lets a different person sign in by ending the Rally session', () => {
      const signOut = vi.fn(async () => {})
      setClerkSignOut(signOut)
      render(<ClerkSignInButton />)
      screen.getByRole('button', { name: 'Use a different account' }).click()
      expect(signOut).toHaveBeenCalledTimes(1)
    })
  })
})
