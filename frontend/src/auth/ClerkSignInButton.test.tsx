// "Continue with Rally" must build its destination through Clerk — never a
// hardcoded URL, because the satellite handshake needs Clerk's own parameters —
// and must round-trip the user back to the page they were on.

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'

let signedIn = false
const buildSignInUrl = vi.fn(() => 'https://primary.example/sign-in?built=1')

vi.mock('@clerk/clerk-react', () => ({
  useClerk: () => ({ buildSignInUrl }),
  SignedOut: ({ children }: { children: React.ReactNode }) => (signedIn ? null : <>{children}</>),
  SignedIn: ({ children }: { children: React.ReactNode }) => (signedIn ? <>{children}</> : null),
}))

import ClerkSignInButton from './ClerkSignInButton'

function setHref(href: string) {
  Object.defineProperty(window, 'location', {
    value: { ...new URL(href), href, assign: vi.fn() },
    writable: true,
    configurable: true,
  })
}

describe('ClerkSignInButton', () => {
  beforeEach(() => {
    signedIn = false
    buildSignInUrl.mockClear()
    setHref('https://app.groflo.io/join/ABC?x=1#frag')
  })

  it('renders a button whose copy names Rally and nothing else', () => {
    render(<ClerkSignInButton />)
    const button = screen.getByRole('button')
    expect(button).toHaveTextContent('Continue with Rally')
    // Penny's vocabulary rules: never "SSO", "bot", "AI", "assistant".
    for (const banned of ['SSO', 'bot', 'AI', 'assistant', 'Clerk']) {
      expect(button.textContent).not.toContain(banned)
    }
  })

  it('navigates to the URL Clerk builds, with the fragment stripped', () => {
    render(<ClerkSignInButton />)
    screen.getByRole('button').click()

    expect(buildSignInUrl).toHaveBeenCalledWith({ redirectUrl: 'https://app.groflo.io/join/ABC?x=1' })
    expect(window.location.assign).toHaveBeenCalledWith('https://primary.example/sign-in?built=1')
  })

  it('renders nothing when already signed in — the exchange is running', () => {
    signedIn = true
    const { container } = render(<ClerkSignInButton />)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(container.textContent).toBe('')
  })
})
