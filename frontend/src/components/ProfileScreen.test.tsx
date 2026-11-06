// The profile no longer adds, links or verifies anything about sign-in, and
// holds no phone number.

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ProfileScreen } from './ProfileScreen'

const fetchMock = vi.fn()
vi.stubGlobal('fetch', fetchMock)

function renderProfile(extra: Partial<React.ComponentProps<typeof ProfileScreen>> = {}) {
  return render(
    <ProfileScreen
      firstName="Tamika" lastName="Jones" address=""
      onFirstNameChange={vi.fn()} onLastNameChange={vi.fn()} onAddressChange={vi.fn()}
      onAutoSave={vi.fn()}
      currentUserId={null} isAdmin={false}
      onNavigate={vi.fn()} showToast={vi.fn()} onSessionExpired={vi.fn()}
      placesReady={false}
      renderHeader={() => null} transitionStyle={{}}
      {...extra}
    />,
  )
}

describe('ProfileScreen', () => {
  it('shows Penny\'s settings when given them, and nothing when not', () => {
    const { unmount } = renderProfile({ pennySettings: <div>penny-settings-slot</div> })
    expect(screen.getByText('penny-settings-slot')).toBeInTheDocument()
    unmount()
    renderProfile()
    expect(screen.queryByText('penny-settings-slot')).not.toBeInTheDocument()
  })

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
    // Email comes from the Rally account; there is no field for it.
    expect(screen.queryByPlaceholderText('your@email.com')).toBeNull()
    expect(screen.getByPlaceholderText(/Start typing your address/)).toBeInTheDocument()
    expect(screen.getByText('Need Help?')).toBeInTheDocument()
    expect(screen.queryByText('Dark mode')).not.toBeInTheDocument()
    expect(screen.queryByText(/sign out/i)).not.toBeInTheDocument()
  })

  it('makes no network call on render', () => {
    renderProfile()
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
