import { useState } from 'react'
import { PennyAvatar } from './PennyAvatar'
import { BRAND, TEXT, WHITE, FONT, BORDER } from '../styles'

export type Screen = 'phone' | 'code' | 'home' | 'penny' | 'profile' | 'settings-share' | 'admin' | 'email-login' | 'email-register' | 'forgot-password' | 'reset-password' | 'shared-list' | 'help'

interface NavigationMenuProps {
  active: Screen
  isAdmin: boolean
  firstName: string
  lastName: string
  userId: number | null
  avatarVersion: number
  onNavigate: (screen: Screen) => void
}

const avatarCircleStyle = (size = 32): React.CSSProperties => ({
  width: `${size}px`, height: `${size}px`, borderRadius: '50%', backgroundColor: BRAND, color: WHITE,
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  fontFamily: FONT, fontSize: '15px', fontWeight: 600, flexShrink: 0,
})

const styles = {
  overlay: {
    position: 'fixed' as const,
    inset: 0,
    zIndex: 19,
  } as React.CSSProperties,
  dropdown: {
    position: 'absolute' as const,
    top: '44px',
    right: 0,
    backgroundColor: WHITE,
    borderRadius: '12px',
    boxShadow: '0 4px 20px rgba(0,0,0,0.15)',
    padding: '6px 0',
    minWidth: '220px',
    whiteSpace: 'nowrap' as const,
    zIndex: 20,
  } as React.CSSProperties,
  item: (active: boolean): React.CSSProperties => ({
    display: 'flex',
    alignItems: 'center',
    gap: '10px',
    width: '100%',
    padding: '10px 18px',
    fontFamily: FONT,
    fontSize: '15px',
    fontWeight: active ? 600 : 400,
    color: active ? BRAND : TEXT,
    background: 'none',
    border: 'none',
    textAlign: 'left' as const,
    cursor: 'pointer',

  }),
}

// SVG icons for menu items (18x18, stroke-based)
const icons = {
  cart: <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="9" cy="21" r="1"/><circle cx="20" cy="21" r="1"/><path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"/></svg>,
  share: <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/></svg>,
  profile: <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>,
  dashboard: <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="18" y="3" width="4" height="18"/><rect x="10" y="8" width="4" height="13"/><rect x="2" y="13" width="4" height="8"/></svg>,
}

export function NavigationMenu({ active, isAdmin, firstName, lastName, userId, avatarVersion, onNavigate }: NavigationMenuProps) {
  const [open, setOpen] = useState(false)
  const [avatarError, setAvatarError] = useState(false)

  const initials = [firstName, lastName]
    .map(n => (n || '')[0]?.toUpperCase() || '')
    .filter(Boolean)
    .join('')

  const avatarUrl = userId ? `/api/avatar/${userId}?v=${avatarVersion}` : null

  const nav = (screen: Screen) => {
    setOpen(false)
    onNavigate(screen)
  }

  return (
    <div style={{ position: 'relative' }}>
      <button
        onClick={() => setOpen(!open)}
        style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0, display: 'flex', alignItems: 'center' }}
        aria-label="Navigation menu"
        aria-expanded={open}
        aria-haspopup="true"
      >
        <div style={{ ...avatarCircleStyle(30), fontSize: '12px', overflow: 'hidden' }}>
          {avatarUrl && !avatarError ? (
            <img
              key={avatarVersion}
              src={avatarUrl}
              alt="Profile"
              onError={() => setAvatarError(true)}
              style={{ width: '100%', height: '100%', objectFit: 'cover' }}
            />
          ) : (
            initials || icons.profile
          )}
        </div>
      </button>
      {open && (
        <>
          <div style={styles.overlay} onClick={() => setOpen(false)} aria-hidden="true" />
          <div style={styles.dropdown} role="menu" aria-label="Navigation">
            <button style={styles.item(active === 'home')} onClick={() => nav('home')} role="menuitem">
              {icons.cart} Shopping List
            </button>
            <button style={styles.item(active === 'settings-share')} onClick={() => nav('settings-share')} role="menuitem">
              {icons.share} Share & Earn
            </button>
            <button style={styles.item(active === 'penny')} onClick={() => nav('penny')} role="menuitem">
              <PennyAvatar size={18} /> Penny
            </button>
            <div style={{ height: '1px', backgroundColor: BORDER, margin: '4px 0' }} role="separator" />
            <button style={styles.item(active === 'profile')} onClick={() => nav('profile')} role="menuitem">
              {icons.profile} Profile
            </button>
            {isAdmin && (
              <button style={styles.item(active === 'admin')} onClick={() => nav('admin')} role="menuitem">
                {icons.dashboard} Dashboard
              </button>
            )}
          </div>
        </>
      )}
    </div>
  )
}
