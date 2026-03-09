import { ScreenTransition } from './ScreenTransition'
import { ConfirmModal } from './ConfirmModal'
import { FONT, FONT_2XS, FONT_XS, FONT_BASE, formatPhone, pageStyles, listStyles } from '../styles'

interface SettingsMember {
  name: string
  phone: string
}

interface ShareScreenProps {
  referralCode: string
  referralFriends: { name: string; created_at: string }[]
  settingsMembers: SettingsMember[]
  inviteLoading: boolean
  confirmModal: { title: string; message?: string; confirmLabel?: string; destructive?: boolean } | null
  onInvite: () => void
  onLeaveHousehold: () => void
  onShareList: () => void
  onShareReferral: () => void
  onCopyReferral: () => void
  runConfirmAction: () => void
  closeConfirmModal: () => void
  renderHeader: () => React.ReactNode
  renderToast: () => React.ReactNode
  renderBottomTabs: () => React.ReactNode
  transitionStyle: React.CSSProperties
}

const avatarCircle = (size = 32): React.CSSProperties => ({
  width: `${size}px`, height: `${size}px`, borderRadius: '50%', backgroundColor: 'var(--brand)', color: '#fff',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, flexShrink: 0,
})

export function ShareScreen({
  referralCode, referralFriends, settingsMembers, inviteLoading,
  confirmModal, onInvite, onLeaveHousehold, onShareList, onShareReferral, onCopyReferral,
  runConfirmAction, closeConfirmModal,
  renderHeader, renderToast, renderBottomTabs, transitionStyle,
}: ShareScreenProps) {
  return (
    <ScreenTransition style={transitionStyle}>
    <div style={pageStyles.page}>
      <ConfirmModal open={!!confirmModal} title={confirmModal?.title || ''} message={confirmModal?.message} confirmLabel={confirmModal?.confirmLabel} destructive={confirmModal?.destructive} onConfirm={runConfirmAction} onCancel={closeConfirmModal} />
      {renderHeader()}
      <div style={pageStyles.listArea}>
        <div style={{ padding: '20px', display: 'flex', flexDirection: 'column', gap: '40px' }}>

          {/* Shared List section */}
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
              <span style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 700, color: 'var(--text)', textTransform: 'uppercase' as const, letterSpacing: '0.5px' }}>Shared List</span>
            </div>
            {settingsMembers.length > 0 && settingsMembers.map((m, i) => (
              <div key={i} style={{
                display: 'flex', alignItems: 'center', padding: '10px 0',
                borderBottom: '1px solid var(--border)', gap: '10px',
              }}>
                <div style={avatarCircle()}>
                  {(m.name || m.phone || '?')[0].toUpperCase()}
                </div>
                <div>
                  <div style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text)' }}>
                    {m.name || formatPhone(m.phone)}
                  </div>
                  {m.name && <div style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)' }}>{formatPhone(m.phone)}</div>}
                </div>
              </div>
            ))}
            <button
              onClick={onInvite}
              disabled={inviteLoading}
              style={{
                ...listStyles.sheetButton,
                marginTop: '12px',
                opacity: inviteLoading ? 0.5 : 1,
              }}
            >
              {inviteLoading ? 'Creating link…' : 'Invite someone to your shared grocery list'}
            </button>
            <p style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', marginTop: '8px', lineHeight: 1.5 }}>
              Everyone on your list shops together in real time
            </p>
            {settingsMembers.length > 0 && (<>
              <button
                onClick={onLeaveHousehold}
                style={{
                  marginTop: '8px', padding: 0, fontFamily: FONT, fontSize: FONT_XS, fontWeight: 500,
                  color: 'var(--error)', backgroundColor: 'transparent', border: 'none', cursor: 'pointer',
                  textAlign: 'left' as const,
                }}
              >Leave shared list</button>
              <div style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', marginTop: '4px' }}>You'll still have your own list — you'll just stop sharing with this group.</div>
            </>)}
          </div>

          {/* Share a List (read-only link) */}
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8"/><polyline points="16 6 12 2 8 6"/><line x1="12" y1="2" x2="12" y2="15"/></svg>
              <span style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 700, color: 'var(--text)', textTransform: 'uppercase' as const, letterSpacing: '0.5px' }}>Share a List</span>
            </div>
            <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text)', margin: '0 0 16px 0', lineHeight: 1.6 }}>
              Send a read-only snapshot of your list to anyone — no account needed. Link expires in 7 days.
            </p>
            <button onClick={onShareList} style={listStyles.sheetButton}>
              Share my list for 7 days
            </button>
          </div>

          {/* Refer a Friend */}
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/></svg>
              <span style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 700, color: 'var(--text)', textTransform: 'uppercase' as const, letterSpacing: '0.5px' }}>Refer a Friend</span>
            </div>

            <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text)', margin: '0 0 16px 0', lineHeight: 1.6 }}>
              Introduce your friends to Penny — they'll get their own list, and you'll both benefit as more people join.
            </p>

            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '10px 14px', backgroundColor: 'var(--bg)', borderRadius: '10px', border: '1px solid var(--border)', marginBottom: '12px' }}>
              <span style={{ flex: 1, fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text)', wordBreak: 'break-all' as const }}>{`https://app.thrift.ly/r/${referralCode}`}</span>
              <button
                onClick={onCopyReferral}
                style={{ padding: '6px 12px', fontFamily: FONT, fontSize: FONT_2XS, fontWeight: 600, color: 'var(--brand)', backgroundColor: 'transparent', border: '1px solid var(--brand)', borderRadius: '8px', cursor: 'pointer', whiteSpace: 'nowrap' as const }}
              >Copy</button>
            </div>

            <button onClick={onShareReferral} style={listStyles.sheetButton}>
              Introduce your friends to Penny
            </button>

            {referralFriends.length > 0 && (
              <p style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', marginTop: '8px' }}>
                {referralFriends.length} {referralFriends.length === 1 ? 'friend' : 'friends'} joined
              </p>
            )}
          </div>

        </div>
      </div>
      {renderToast()}
      {renderBottomTabs()}
    </div>
    </ScreenTransition>
  )
}
