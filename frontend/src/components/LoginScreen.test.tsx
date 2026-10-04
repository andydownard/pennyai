// The login screen is the one way in: Penny, "Continue with Rally", legal links.
// It collects no credentials, and it never renders blank — a build or server
// without Rally sign-in shows a calm message instead.

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { LoginScreen } from './LoginScreen'
import { RallyReadyContext } from '../auth/RallyReadyContext'

const signInButtonRenders = vi.fn()
vi.mock('../auth/ClerkSignInButton', () => ({
  default: () => {
    signInButtonRenders()
    return <button type="button">Continue with Rally</button>
  },
}))

function renderLogin(opts: { rallyReady?: boolean; clerkEnabled?: boolean | null; error?: string } = {}) {
  const { rallyReady = true, clerkEnabled = true, error } = opts
  return render(
    <RallyReadyContext.Provider value={rallyReady}>
      <LoginScreen loginPennyText="Hi, I'm Penny." clerkEnabled={clerkEnabled} error={error} />
    </RallyReadyContext.Provider>,
  )
}

const UNAVAILABLE = 'Sign-in is briefly unavailable.'

describe('LoginScreen', () => {
  it('renders Penny, the Rally button and the legal links', async () => {
    renderLogin()
    expect(await screen.findByRole('button', { name: 'Continue with Rally' })).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Penny' })).toBeInTheDocument()
    expect(screen.getByText(/Hi, I'm Penny\./)).toBeInTheDocument()
    for (const link of screen.getAllByRole('link', { name: /^Privacy/ })) expect(link).toHaveAttribute('href', '/privacy')
    for (const link of screen.getAllByRole('link', { name: 'Terms' })) expect(link).toHaveAttribute('href', '/terms')
    expect(screen.getByRole('link', { name: 'Privacy' })).toBeInTheDocument()
    expect(screen.queryByText(UNAVAILABLE)).not.toBeInTheDocument()
  })

  it('offers exactly one action', async () => {
    renderLogin()
    await screen.findByRole('button', { name: 'Continue with Rally' })
    expect(screen.getAllByRole('button')).toHaveLength(1)
  })

  it('collects no credentials', async () => {
    const { container } = renderLogin()
    await screen.findByRole('button', { name: 'Continue with Rally' })
    for (const selector of ['input[type=tel]', 'input[type=email]', 'input[type=password]', 'input', 'form', '#google-signin-btn']) {
      expect(container.querySelector(selector), selector).toBeNull()
    }
  })

  it('keeps SMS consent wording off the screen', async () => {
    const { container } = renderLogin()
    await screen.findByRole('button', { name: 'Continue with Rally' })
    expect(container.textContent).not.toMatch(/SMS|STOP|Msg &|data rates/i)
  })

  it('follows the vocabulary rules', async () => {
    const { container } = renderLogin()
    await screen.findByRole('button', { name: 'Continue with Rally' })
    expect(container.textContent).not.toMatch(/\b(bot|AI|assistant|chatbot|Clerk|SSO|Thriftly)\b/)
  })

  describe('when Rally sign-in is not available', () => {
    it('shows the unavailable message in a build with no key (no provider mounted)', () => {
      signInButtonRenders.mockClear()
      const { container } = renderLogin({ rallyReady: false, clerkEnabled: true })
      expect(screen.getByText(UNAVAILABLE)).toBeInTheDocument()
      expect(screen.queryByRole('button')).not.toBeInTheDocument()
      // The button uses Clerk hooks, which throw outside the provider.
      expect(signInButtonRenders).not.toHaveBeenCalled()
      expect(container.querySelector('input')).toBeNull()
    })

    it('shows the unavailable message without any provider at all', () => {
      render(<LoginScreen loginPennyText="" clerkEnabled />)
      expect(screen.getByText(UNAVAILABLE)).toBeInTheDocument()
    })

    it('shows the unavailable message when the server says clerk_enabled: false', () => {
      renderLogin({ rallyReady: true, clerkEnabled: false })
      expect(screen.getByText(UNAVAILABLE)).toBeInTheDocument()
      expect(screen.queryByRole('button')).not.toBeInTheDocument()
    })

    it('still shows Penny and the legal links', () => {
      renderLogin({ rallyReady: false })
      expect(screen.getByRole('img', { name: 'Penny' })).toBeInTheDocument()
      expect(screen.getByRole('link', { name: 'Privacy' })).toBeInTheDocument()
    })
  })

  it('shows neither the button nor the unavailable message while config is loading', () => {
    renderLogin({ clerkEnabled: null })
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.queryByText(UNAVAILABLE)).not.toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Penny' })).toBeInTheDocument()
  })

  it('shows a failed sign-in in the server\'s words, with the button still there', async () => {
    renderLogin({ error: 'This account is already connected to a different Rally sign-in.' })
    expect(screen.getByRole('alert')).toHaveTextContent('already connected to a different Rally sign-in')
    expect(await screen.findByRole('button', { name: 'Continue with Rally' })).toBeInTheDocument()
  })
})
