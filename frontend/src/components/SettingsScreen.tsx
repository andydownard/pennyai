import { ScreenTransition } from './ScreenTransition'
import { pageStyles } from '../styles'

interface SettingsScreenProps {
  renderHeader: () => React.ReactNode
  transitionStyle: React.CSSProperties
  children: React.ReactNode
}

/** Settings: a page of preferences. Today that is Penny's settings. */
export function SettingsScreen({ renderHeader, transitionStyle, children }: SettingsScreenProps) {
  return (
    <ScreenTransition style={transitionStyle}>
      <div style={pageStyles.page}>
        {renderHeader()}
        <div style={pageStyles.listArea}>
          <div style={{ padding: '24px 20px', display: 'flex', flexDirection: 'column', gap: '24px' }}>
            {children}
          </div>
        </div>
      </div>
    </ScreenTransition>
  )
}
