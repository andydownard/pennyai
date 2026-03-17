import { PennyAvatar } from './PennyAvatar'
import { FONT, FONT_BASE, FONT_SM, FONT_2XS } from '../styles'

export type NavScreen = 'home' | 'saved-lists' | 'penny' | 'settings-share' | 'profile' | 'admin'

interface NavMenuProps {
  open: boolean
  active: NavScreen
  isAdmin?: boolean
  onNavigate: (screen: NavScreen) => void
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
  {
    key: 'settings-share', label: 'Share & Earn',
    icon: (c) => <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={c} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/></svg>,
  },
  {
    key: 'profile', label: 'Profile',
    icon: (c) => <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={c} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>,
  },
]

const adminItem: typeof menuItems[0] = {
  key: 'admin', label: 'Dashboard',
  icon: (c) => <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={c} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="18" y="3" width="4" height="18"/><rect x="10" y="8" width="4" height="13"/><rect x="2" y="13" width="4" height="8"/></svg>,
}

export function NavMenu({ open, active, isAdmin, onNavigate, onClose }: NavMenuProps) {
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
        {/* Brand header */}
        <div style={{
          padding: '20px 20px 16px',
          borderBottom: '1px solid var(--border)',
          display: 'flex', alignItems: 'center', gap: '12px',
        }}>
          <svg width="36" height="36" viewBox="0 0 72 72" fill="none" xmlns="http://www.w3.org/2000/svg" aria-label="Thriftly logo" style={{ flexShrink: 0 }}>
            <rect x="12" y="10" width="42" height="52" rx="6" stroke="var(--brand)" strokeWidth="2.5" fill="var(--surface)" />
            <rect x="25" y="4" width="16" height="10" rx="4" fill="var(--brand)" />
            <rect x="29" y="7" width="8" height="4" rx="2" fill="var(--surface)" />
            <path d="M20 27 L23 30 L28 24" stroke="var(--brand)" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
            <line x1="33" y1="27" x2="46" y2="27" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" opacity="0.7" />
            <path d="M20 37 L23 40 L28 34" stroke="var(--brand)" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
            <line x1="33" y1="37" x2="42" y2="37" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" opacity="0.7" />
            <circle cx="24" cy="47" r="2" fill="var(--brand)" opacity="0.3" />
            <line x1="33" y1="47" x2="39" y2="47" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" opacity="0.3" />
            <path d="M54 14 L56 19 L61 21 L56 23 L54 28 L52 23 L47 21 L52 19 Z" fill="var(--brand)" />
            <path d="M60 6 L61 9 L64 10 L61 11 L60 14 L59 11 L56 10 L59 9 Z" fill="var(--brand)" opacity="0.5" />
          </svg>
          <div>
            <span style={{
              fontFamily: FONT, fontSize: '1.25rem', fontWeight: 700,
              color: 'var(--brand)', letterSpacing: '-0.5px',
            }}>thriftly</span>
            <p style={{
              fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)',
              margin: '2px 0 0', lineHeight: 1.3,
            }}>Text Penny. She handles the rest.</p>
          </div>
        </div>

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

        {/* Brand portal link */}
        <a
          href="https://brand.thrift.ly/"
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
        </a>

        {/* Close hint */}
        <div style={{
          padding: '12px 20px',
          borderTop: '1px solid var(--border)',
        }}>
          <span style={{
            fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text-secondary)',
          }}>Tap outside to close</span>
        </div>
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

/** Hamburger icon button for the header */
export function HamburgerButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      aria-label="Open navigation menu"
      style={{
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        minWidth: '44px', minHeight: '44px',
        padding: 0, background: 'none', border: 'none', cursor: 'pointer',
        marginLeft: '-8px', marginRight: '4px',
      }}
    >
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="var(--text)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <line x1="3" y1="6" x2="21" y2="6"/>
        <line x1="3" y1="12" x2="21" y2="12"/>
        <line x1="3" y1="18" x2="21" y2="18"/>
      </svg>
    </button>
  )
}
