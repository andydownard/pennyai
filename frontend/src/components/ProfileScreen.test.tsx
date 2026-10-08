// The profile no longer adds, links or verifies anything about sign-in, and
// holds no phone number.

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ProfileScreen } from './ProfileScreen'

const fetchMock = vi.fn()
vi.stubGlobal('fetch', fetchMock)

function renderProfile() {
  return render(
    <ProfileScreen
      firstName="Tamika" lastName="Jones" email="t@example.com" address=""
      onFirstNameChange={vi.fn()} onLastNameChange={vi.fn()} onEmailChange={vi.fn()} onAddressChange={vi.fn()}
      onAutoSave={vi.fn()}
      currentUserId={null} isAdmin={false}
      onNavigate={vi.fn()} onLogout={vi.fn()} showToast={vi.fn()} onSessionExpired={vi.fn()}
      placesReady={false} darkMode={false} onToggleDarkMode={vi.fn()}
      renderHeader={() => null} transitionStyle={{}}
    />,
  )
}

describe('ProfileScreen', () => {
  it('shows no phone number and offers no field for one', () => {
    const { container } = renderProfile()
    expect(screen.queryByTestId('profile-phone')).not.toBeInTheDocument()
    expect(screen.queryByText(/phone|text Penny|mobile/i)).not.toBeInTheDocument()
    expect(container.querySelector('input[type=tel]')).toBeNull()
  })

  it('has no link, add or password controls', () => {
    const { container } = renderProfile()
    expect(container.querySelector('input[type=password]')).toBeNull()
    expect(screen.queryByText('Sign-in methods')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^(Add|Link|Send Code|Verify & Link)$/ })).not.toBeInTheDocument()
    expect(screen.queryByText(/password/i)).not.toBeInTheDocument()
  })

  it('keeps the rest of the profile', () => {
    renderProfile()
    expect(screen.getByPlaceholderText('First')).toHaveValue('Tamika')
    expect(screen.getByPlaceholderText('your@email.com')).toHaveValue('t@example.com')
    expect(screen.getByPlaceholderText(/Start typing your address/)).toBeInTheDocument()
    expect(screen.getByText('Need Help?')).toBeInTheDocument()
    expect(screen.getByText('Dark mode')).toBeInTheDocument()
    expect(screen.getByText('Sign Out')).toBeInTheDocument()
  })

  it('makes no network call on render', () => {
    renderProfile()
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
