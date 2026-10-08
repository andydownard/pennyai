import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import { NavMenu, AvatarMenuButton } from './NavMenu'

function renderMenu(over: Partial<React.ComponentProps<typeof NavMenu>> = {}) {
  const props = {
    open: true, active: 'home' as const, isAdmin: false, userId: 7,
    initials: 'TJ', displayName: 'Tamika Jones',
    onNavigate: vi.fn(), onLogout: vi.fn(), onClose: vi.fn(), ...over,
  }
  render(<NavMenu {...props} />)
  return props
}

describe('AvatarMenuButton', () => {
  it('is an accessible menu button showing initials when the photo 404s', () => {
    render(<AvatarMenuButton onClick={vi.fn()} expanded={false} userId={7} initials="TJ" />)
    const btn = screen.getByRole('button', { name: 'Open menu' })
    expect(btn).toHaveAttribute('aria-haspopup', 'true')
    expect(btn).toHaveAttribute('aria-expanded', 'false')
    const img = btn.querySelector('img')!
    expect(img).toHaveStyle({ opacity: '0' })
    fireEvent.error(img)
    expect(btn.querySelector('img')).toBeNull()
    expect(btn).toHaveTextContent('TJ')
  })

  it('shows the photo once it loads', () => {
    render(<AvatarMenuButton onClick={vi.fn()} expanded userId={7} initials="TJ" />)
    const img = screen.getByRole('button', { name: 'Open menu' }).querySelector('img')!
    expect(img.getAttribute('src')).toMatch(/^\/api\/avatar\/7\?v=/)
    fireEvent.load(img)
    expect(img).toHaveStyle({ opacity: '1' })
  })

  it('falls back to a person glyph with no name', () => {
    render(<AvatarMenuButton onClick={vi.fn()} expanded={false} userId={null} initials="" />)
    expect(screen.getByRole('button', { name: 'Open menu' }).querySelector('svg')).not.toBeNull()
  })
})

describe('NavMenu', () => {
  it('puts Profile first and Sign out last', () => {
    renderMenu({ isAdmin: true })
    const labels = within(screen.getByRole('navigation')).getAllByRole('button').map(b => b.textContent || '')
    expect(labels[0]).toContain('Profile')
    expect(labels[0]).toContain('Tamika Jones')
    expect(labels.slice(1, -1)).toEqual(['Shopping List', 'My Lists', 'Penny', 'Share & Earn', 'Dashboard'])
    expect(labels[labels.length - 1]).toBe('Sign out')
  })

  it('calls the logout handler once', () => {
    const p = renderMenu()
    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
    expect(p.onLogout).toHaveBeenCalledTimes(1)
    expect(p.onClose).toHaveBeenCalledTimes(1)
  })
})

describe('NavMenu brand portal link', () => {
  it('is hidden when no brand portal URL is configured', () => {
    renderMenu()
    expect(screen.queryByText('For Brands')).toBeNull()
    expect(screen.getByRole('button', { name: /sign out/i })).toBeInTheDocument()
  })
})
