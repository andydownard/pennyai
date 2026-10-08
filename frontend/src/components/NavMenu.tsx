import { PennyAvatar } from './PennyAvatar'
import { UserAvatar } from './UserAvatar'
import { FONT, FONT_BASE, FONT_SM } from '../styles'

const BRAND_PORTAL_URL = import.meta.env.VITE_BRAND_PORTAL_URL || ''

export type NavScreen = 'home' | 'saved-lists' | 'penny' | 'settings-share' | 'profile' | 'admin'

interface NavMenuProps {
  open: boolean
  active: NavScreen
  isAdmin?: boolean
  userId: number | null
  initials: string
  displayName: string
  email?: string
  onNavigate: (screen: NavScreen) => void
  onLogout: () => void
  onClose: () => void
}

const menuItems: { key: NavScreen; label: string; icon: (color: string) => JSX.Element }[] = [
  {
    key: 'home', label: 'Shopping List',
    icon: (c) => <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={c} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="9" cy="21" r="1"/><circle cx="20" cy="21" r="1"/><path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"/></svg>,
  },
  {
    key: 'saved-lists', label: 'My Lists',
    icon: (c) => <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={c} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>,
  },
  {
    key: 'penny', label: 'Penny',
    icon: () => <PennyAvatar size={20} />,
  },
]

const adminItem: typeof menuItems[0] = {
  key: 'admin', label: 'Dashboard',
  icon: (c) => <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={c} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="18" y="3" width="4" height="18"/><rect x="10" y="8" width="4" height="13"/><rect x="2" y="13" width="4" height="8"/></svg>,
}

export function NavMenu({ open, active, isAdmin, userId, initials, displayName, email = '', onNavigate, onLogout, onClose }: NavMenuProps) {
  if (!open) return null

  const items = isAdmin ? [...menuItems, adminItem] : menuItems

  return (
    <div
      style={{
        position: 'fixed', inset: 0, zIndex: 9500,
        backgroundColor: 'rgba(0,0,0,0.3)',
        transition: 'opacity 150ms ease',
      }}
      onClick={onClose}
    >
      <nav
        style={{
          position: 'absolute', top: 0, left: 0, bottom: 0,
          width: '260px', maxWidth: '75vw',
          backgroundColor: 'var(--surface)',
          boxShadow: '4px 0 20px rgba(0,0,0,0.15)',
          padding: 'env(safe-area-inset-top, 16px) 0 env(safe-area-inset-bottom, 16px)',
          display: 'flex', flexDirection: 'column',
          animation: 'slideInMenu 200ms ease',
        }}
        onClick={e => e.stopPropagation()}
        aria-label="Main navigation"
      >
        {/* Profile: first. The arrow says the name row opens Profile. */}
        <button
          onClick={() => { onNavigate('profile'); onClose() }}
          aria-current={active === 'profile' ? 'page' : undefined}
          aria-label={displayName ? `Profile, ${displayName}` : 'Profile'}
          style={{
            display: 'flex', alignItems: 'center', gap: '14px',
            // Tighter right edge and gap so the arrow costs the name little room
            // (names are rem-sized; the menu is a fixed 260px).
            width: '100%', padding: '16px 12px 16px 20px',
            fontFamily: FONT, border: 'none', cursor: 'pointer', textAlign: 'left',
            color: 'var(--text)',
            backgroundColor: active === 'profile' ? 'var(--bg)' : 'transparent',
            borderLeft: active === 'profile' ? '3px solid var(--brand)' : '3px solid transparent',
            borderBottom: '1px solid var(--border)',
          }}
        >
          <UserAvatar userId={userId} initials={initials} size={48} />
          <span data-testid="profile-text" style={{ display: 'flex', flexDirection: 'column', flex: 1, minWidth: 0 }}>
            <span style={{ fontSize: FONT_BASE, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{displayName || 'Profile'}</span>
            {email && (
              <span style={{ fontSize: FONT_SM, color: 'var(--text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{email}</span>
            )}
          </span>
          <svg data-testid="profile-arrow" aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0, marginLeft: '-8px' }}><polyline points="9 18 15 12 9 6"/></svg>
        </button>

        {/* Nav items */}
        <div style={{ flex: 1, padding: '8px 0', overflowY: 'auto' }}>
          {items.map(item => {
            const isActive = active === item.key
            return (
              <button
                key={item.key}
                onClick={() => { onNavigate(item.key); onClose() }}
                aria-current={isActive ? 'page' : undefined}
                style={{
                  display: 'flex', alignItems: 'center', gap: '14px',
                  width: '100%', padding: '14px 20px',
                  fontFamily: FONT, fontSize: FONT_BASE, fontWeight: isActive ? 600 : 400,
                  color: isActive ? 'var(--brand)' : 'var(--text)',
                  backgroundColor: isActive ? 'var(--bg)' : 'transparent',
                  border: 'none', cursor: 'pointer',
                  textAlign: 'left',
                  borderLeft: isActive ? '3px solid var(--brand)' : '3px solid transparent',
                }}
              >
                {item.icon(isActive ? 'var(--brand)' : 'var(--text-secondary)')}
                {item.label}
              </button>
            )
          })}
        </div>

        {/* Brand portal link: shown only when the portal has a host
            (VITE_BRAND_PORTAL_URL at build time). */}
        {BRAND_PORTAL_URL && <a
          href={BRAND_PORTAL_URL}
          target="_blank"
          rel="noopener noreferrer"
          style={{
            display: 'flex', alignItems: 'center', gap: '10px',
            padding: '12px 20px',
            fontFamily: FONT, fontSize: FONT_SM, fontWeight: 500,
            color: 'var(--text-secondary)',
            textDecoration: 'none',
            borderTop: '1px solid var(--border)',
          }}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <rect x="2" y="7" width="20" height="14" rx="2" ry="2"/><path d="M16 21V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"/>
          </svg>
          For Brands
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ marginLeft: 'auto', opacity: 0.5 }}>
            <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/>
          </svg>
        </a>}

        {/* Sign out: last */}
        <button
          onClick={() => { onClose(); onLogout() }}
          style={{
            display: 'flex', alignItems: 'center', gap: '14px',
            width: '100%', minHeight: '44px', padding: '14px 20px',
            fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 500,
            color: 'var(--error)', backgroundColor: 'transparent',
            border: 'none', borderTop: '1px solid var(--border)', cursor: 'pointer', textAlign: 'left',
          }}
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
          Sign out
        </button>
      </nav>

      {/* Slide-in animation */}
      <style>{`
        @keyframes slideInMenu {
          from { transform: translateX(-100%); }
          to { transform: translateX(0); }
        }
      `}</style>
    </div>
  )
}

/** Header menu trigger: the signed-in user's avatar. */
export function AvatarMenuButton({ onClick, expanded, userId, initials }: {
  onClick: () => void; expanded: boolean; userId: number | null; initials: string
}) {
  return (
    <button
      onClick={onClick}
      aria-label="Open menu"
      aria-haspopup="true"
      aria-expanded={expanded}
      style={{
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        minWidth: '44px', minHeight: '44px',
        padding: 0, background: 'none', border: 'none', cursor: 'pointer',
        marginLeft: '-8px', marginRight: '4px',
      }}
    >
      <UserAvatar userId={userId} initials={initials} size={36} />
    </button>
  )
}
