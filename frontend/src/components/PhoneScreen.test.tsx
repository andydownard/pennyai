// The login screens gain exactly one thing: an optional button below Google.
//
// With clerkEnabled false — the default, and what the server reports while Rally
// sign-in is dark — the markup must be what it is today. Nothing moves, nothing
// is renamed, and the e2e selectors (#google-signin-btn, the phone input, the
// Continue button) still match.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { PhoneScreen } from './PhoneScreen'
import { EmailLoginScreen } from './EmailLoginScreen'

vi.mock('../auth/ClerkSignInButton', () => ({
  default: () => <button type="button">Continue with Rally</button>,
}))

const phoneProps = {
  phone: '5551234567',
  smsOptIn: true,
  loading: false,
  error: '',
  loginPennyText: 'Hi.',
  googleClientId: 'gcid',
  phoneInputRef: { current: null },
  onPhoneChange: vi.fn(),
  onSmsOptInChange: vi.fn(),
  onSendCode: vi.fn(),
  onNavigate: vi.fn(),
  setError: vi.fn(),
}

const emailProps = {
  emailInput: '',
  passwordInput: '',
  loading: false,
  error: '',
  magicLinkSent: false,
  loginPennyText: 'Hi.',
  googleClientId: 'gcid',
  onEmailChange: vi.fn(),
  onPasswordChange: vi.fn(),
  onLogin: vi.fn(),
  onMagicLink: vi.fn(),
  onNavigate: vi.fn(),
  setError: vi.fn(),
  setMagicLinkSent: vi.fn(),
}

describe('PhoneScreen Rally sign-in button', () => {
  afterEach(() => vi.unstubAllEnvs())

  it('renders no Rally button by default', () => {
    vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', '')
    render(<PhoneScreen {...phoneProps} />)
    expect(screen.queryByText('Continue with Rally')).not.toBeInTheDocument()
  })

  it('renders no Rally button when the server says the feature is off', () => {
    vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', 'pk_test_abc')
    render(<PhoneScreen {...phoneProps} clerkEnabled={false} />)
    expect(screen.queryByText('Continue with Rally')).not.toBeInTheDocument()
  })

  it('renders no Rally button when the build has no key, even if the server says on', () => {
    vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', '')
    render(<PhoneScreen {...phoneProps} clerkEnabled />)
    expect(screen.queryByText('Continue with Rally')).not.toBeInTheDocument()
  })

  it('produces identical markup with the feature off and with it absent', () => {
    vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', '')
    const withoutProp = render(<PhoneScreen {...phoneProps} />).container.innerHTML
    const withPropFalse = render(<PhoneScreen {...phoneProps} clerkEnabled={false} />).container.innerHTML
    expect(withPropFalse).toBe(withoutProp)
  })

  it('keeps every existing element when the Rally button is shown', async () => {
    vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', 'pk_test_abc')
    const { container } = render(<PhoneScreen {...phoneProps} clerkEnabled />)

    // The Google mount point, the phone field and the Continue button all stay.
    expect(container.querySelector('#google-signin-btn')).not.toBeNull()
    expect(screen.getByPlaceholderText('(555) 123-4567')).toBeInTheDocument()
    expect(screen.getByText(/Continue →/)).toBeInTheDocument()
    expect(screen.getByText('Use email instead')).toBeInTheDocument()
    // And the Rally button appears (lazy, so await it).
    expect(await screen.findByText('Continue with Rally')).toBeInTheDocument()
  })
})

describe('EmailLoginScreen Rally sign-in button', () => {
  afterEach(() => vi.unstubAllEnvs())

  it('renders no Rally button by default', () => {
    vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', '')
    render(<EmailLoginScreen {...emailProps} />)
    expect(screen.queryByText('Continue with Rally')).not.toBeInTheDocument()
  })

  it('shows it when enabled, alongside the Google mount point', async () => {
    vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', 'pk_test_abc')
    const { container } = render(<EmailLoginScreen {...emailProps} clerkEnabled />)
    expect(await screen.findByText('Continue with Rally')).toBeInTheDocument()
    expect(container.querySelector('#google-signin-btn')).not.toBeNull()
    expect(screen.getByText('Use phone number instead')).toBeInTheDocument()
  })
})
