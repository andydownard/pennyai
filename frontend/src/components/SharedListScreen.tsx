import { PennyAvatar } from './PennyAvatar'
import { FONT, FONT_XS, FONT_SM, FONT_BASE } from '../styles'

interface SharedItem {
  name: string
  quantity: number
  checked: boolean
}

interface SharedData {
  items: SharedItem[]
  sharer_name: string
  expires_at: string
  invite_code?: string
}

interface SharedListScreenProps {
  sharedData: SharedData | null
  sharedLoading: boolean
  sharedError: string | null
  /** Whether the visitor already has a Groflo session. */
  signedIn?: boolean
  /**
   * Leave this read-only view. Signed-out visitors go to the login screen; the
   * household invite waits in storage and becomes the join prompt after sign-in.
   */
  onContinue: () => void
}

export function SharedListScreen({
  sharedData, sharedLoading, sharedError, signedIn = false, onContinue,
}: SharedListScreenProps) {
  const timeLeft = sharedData ? (() => {
    const ms = new Date(sharedData.expires_at).getTime() - Date.now()
    if (ms <= 0) return 'expired'
    const hours = Math.floor(ms / 3600000)
    if (hours > 0) return `${hours} hour${hours !== 1 ? 's' : ''}`
    const mins = Math.floor(ms / 60000)
    return `${mins} minute${mins !== 1 ? 's' : ''}`
  })() : null

  const canJoin = Boolean(sharedData?.invite_code)
  const sharerList = sharedData?.sharer_name ? `${sharedData.sharer_name}'s` : 'this'

  return (
    <div style={{ minHeight: '100vh', backgroundColor: 'var(--bg)', display: 'flex', flexDirection: 'column' }}>
      {/* Header */}
      <div style={{ backgroundColor: 'var(--surface)', borderBottom: '1px solid var(--border)', padding: '20px', textAlign: 'center' }}>
        <div style={{ display: 'flex', justifyContent: 'center', marginBottom: '12px' }}>
          <PennyAvatar size={48} />
        </div>
        <div style={{ fontFamily: FONT, fontSize: '1.25rem', fontWeight: 700, color: 'var(--text)' }}>
          {sharedData ? (sharedData.sharer_name ? `${sharedData.sharer_name}'s Shopping List` : 'Shopping List') : 'Shopping List'}
        </div>
        <div style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--brand)', fontWeight: 600, letterSpacing: '-0.3px', marginTop: '4px' }}>Groflo</div>
      </div>

      {/* Expiry banner */}
      {sharedData && timeLeft && timeLeft !== 'expired' && (
        <div style={{ backgroundColor: 'var(--warning-bg)', padding: '10px 20px', fontFamily: FONT, fontSize: FONT_XS, color: 'var(--warning)', textAlign: 'center' }}>
          This link expires in {timeLeft}
        </div>
      )}

      {/* Content */}
      <div style={{ flex: 1, padding: '16px 20px', overflowY: 'auto' }}>
        {sharedLoading && (
          <div style={{ textAlign: 'center', padding: '40px 0' }}>
            <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text-secondary)' }}>Loading list...</p>
          </div>
        )}

        {sharedError === 'expired' && (
          <div style={{ textAlign: 'center', padding: '40px 20px' }}>
            <div style={{ fontSize: '48px', marginBottom: '16px' }}>⏰</div>
            <h2 style={{ fontFamily: FONT, fontSize: '1.25rem', color: 'var(--text)', margin: '0 0 8px' }}>This link has expired</h2>
            <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text-secondary)', margin: '0 0 24px' }}>
              Share links are valid for 48 hours. Ask the sender to share again.
            </p>
            <button
              onClick={onContinue}
              style={{ backgroundColor: 'var(--brand)', color: '#fff', border: 'none', borderRadius: '10px', padding: '14px 28px', fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, cursor: 'pointer' }}
            >
              {signedIn ? 'Open My List' : 'Get Started'}
            </button>
          </div>
        )}

        {sharedError === 'not_found' && (
          <div style={{ textAlign: 'center', padding: '40px 20px' }}>
            <div style={{ fontSize: '48px', marginBottom: '16px' }}>🔗</div>
            <h2 style={{ fontFamily: FONT, fontSize: '1.25rem', color: 'var(--text)', margin: '0 0 8px' }}>Link not found</h2>
            <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text-secondary)', margin: '0 0 24px' }}>
              This share link doesn't exist. Check the URL and try again.
            </p>
            <button
              onClick={onContinue}
              style={{ backgroundColor: 'var(--brand)', color: '#fff', border: 'none', borderRadius: '10px', padding: '14px 28px', fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, cursor: 'pointer' }}
            >
              {signedIn ? 'Open My List' : 'Get Started'}
            </button>
          </div>
        )}

        {sharedData && !sharedError && (
          <>
            {sharedData.items.length === 0 ? (
              <div style={{ textAlign: 'center', padding: '40px 20px' }}>
                <div style={{ fontSize: '48px', marginBottom: '16px' }}>📝</div>
                <h2 style={{ fontFamily: FONT, fontSize: '1.125rem', color: 'var(--text)', margin: '0 0 8px' }}>
                  Nothing on this list yet
                </h2>
                <p style={{ fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text-secondary)', margin: 0, lineHeight: '1.5' }}>
                  {sharedData.sharer_name || 'The owner'} hasn't added any items yet. Check back soon!
                </p>
              </div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                {sharedData.items.map((item, i) => (
                  <div key={i} style={{
                    display: 'flex', alignItems: 'center', gap: '12px',
                    padding: '14px 16px', backgroundColor: 'var(--surface)', borderRadius: '10px',
                    opacity: item.checked ? 0.5 : 1,
                  }}>
                    <div style={{
                      width: '22px', height: '22px', borderRadius: '6px', flexShrink: 0,
                      border: item.checked ? 'none' : '2px solid var(--border)',
                      backgroundColor: item.checked ? 'var(--brand)' : 'transparent',
                      display: 'flex', alignItems: 'center', justifyContent: 'center',
                    }}>
                      {item.checked && <span style={{ color: '#fff', fontSize: '14px', fontWeight: 700 }}>✓</span>}
                    </div>
                    <span style={{
                      fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text)', flex: 1,
                      textDecoration: item.checked ? 'line-through' : 'none',
                    }}>
                      {item.quantity > 1 ? `${item.quantity}x ` : ''}{item.name}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </div>

      {/* Join CTA — sign-in itself happens on the login screen */}
      <div style={{ padding: '20px', backgroundColor: 'var(--surface)', borderTop: '1px solid var(--border)', textAlign: 'center' }}>
        <p style={{ fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text-secondary)', margin: '0 0 12px' }}>
          {canJoin
            ? (signedIn ? `Join ${sharerList} shopping list.` : `Join ${sharerList} shopping list. Sign in to get started.`)
            : (signedIn ? 'Back to your own shopping list.' : 'Sign in to Groflo to make your own shopping list.')}
        </p>
        <button
          onClick={onContinue}
          style={{ backgroundColor: 'var(--brand)', color: '#fff', border: 'none', borderRadius: '10px', padding: '14px 28px', minHeight: '48px', fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, cursor: 'pointer', width: '100%', maxWidth: '320px' }}
        >
          {canJoin ? 'Join This List' : (signedIn ? 'Open My List' : 'Get Started')}
        </button>
      </div>
    </div>
  )
}
