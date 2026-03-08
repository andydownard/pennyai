import { PennyAvatar } from './PennyAvatar'
import { BRAND, TEXT_SECONDARY, WHITE, FONT, BORDER } from '../styles'

export type TabScreen = 'home' | 'penny' | 'settings-share' | 'profile'

interface BottomTabBarProps {
  active: TabScreen
  onNavigate: (screen: TabScreen) => void
  pennyUnread?: boolean
}

const TAB_HEIGHT = 56
const ICON_SIZE = 22

const icons = {
  list: (color: string) => (
    <svg width={ICON_SIZE} height={ICON_SIZE} viewBox="0 0 24 24" fill="none" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="9" cy="21" r="1"/><circle cx="20" cy="21" r="1"/>
      <path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"/>
    </svg>
  ),
  share: (color: string) => (
    <svg width={ICON_SIZE} height={ICON_SIZE} viewBox="0 0 24 24" fill="none" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/>
      <line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/>
    </svg>
  ),
  profile: (color: string) => (
    <svg width={ICON_SIZE} height={ICON_SIZE} viewBox="0 0 24 24" fill="none" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>
    </svg>
  ),
}

const tabs: { key: TabScreen; label: string; icon: (color: string) => JSX.Element }[] = [
  { key: 'home', label: 'List', icon: icons.list },
  { key: 'penny', label: 'Penny', icon: () => <PennyAvatar size={ICON_SIZE} /> },
  { key: 'settings-share', label: 'Share', icon: icons.share },
  { key: 'profile', label: 'Profile', icon: icons.profile },
]

export function BottomTabBar({ active, onNavigate, pennyUnread }: BottomTabBarProps) {
  return (
    <nav
      style={{
        flexShrink: 0,
        height: `${TAB_HEIGHT}px`,
        paddingBottom: 'env(safe-area-inset-bottom, 0px)',
        backgroundColor: WHITE,
        borderTop: `1px solid ${BORDER}`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-around',
        fontFamily: FONT,
      }}
      aria-label="Main navigation"
    >
      {tabs.map(tab => {
        const isActive = active === tab.key
        const color = isActive ? BRAND : TEXT_SECONDARY
        return (
          <button
            key={tab.key}
            onClick={() => onNavigate(tab.key)}
            aria-label={tab.label}
            aria-current={isActive ? 'page' : undefined}
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '2px',
              minWidth: '44px',
              minHeight: '44px',
              padding: '4px 12px',
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              position: 'relative',
            }}
          >
            {tab.icon(color)}
            {tab.key === 'penny' && pennyUnread && (
              <span style={{
                position: 'absolute',
                top: '4px',
                right: '10px',
                width: '8px',
                height: '8px',
                borderRadius: '50%',
                backgroundColor: '#EF4444',
              }} />
            )}
            <span style={{
              fontSize: '10px',
              fontWeight: isActive ? 600 : 400,
              color,
              lineHeight: 1,
            }}>
              {tab.label}
            </span>
          </button>
        )
      })}
    </nav>
  )
}

/** Bottom padding spacer to prevent content from being hidden behind the tab bar */
export const TAB_BAR_HEIGHT = TAB_HEIGHT
