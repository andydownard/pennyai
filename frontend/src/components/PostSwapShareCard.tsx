import { FONT, FONT_2XS, FONT_XS, FONT_SM } from '../styles'
import { track } from '../analytics'

interface PostSwapShareCardProps {
  brand: string
  savings: number
  showToast: (message: string) => void
  onDismiss: () => void
}

export function PostSwapShareCard({ brand, savings, showToast, onDismiss }: PostSwapShareCardProps) {
  return (
    <div style={{
      padding: '12px 16px', margin: '8px 16px', borderRadius: '12px',
      backgroundColor: 'var(--success-bg)', border: '1px solid color-mix(in srgb, var(--success) 25%, transparent)',
      display: 'flex', alignItems: 'center', gap: '12px',
      animation: 'fadeInUp 0.3s ease',
    }}>
      <div style={{ fontSize: '24px' }}>🎉</div>
      <div style={{ flex: 1 }}>
        <div style={{ fontFamily: FONT, fontSize: FONT_SM, fontWeight: 600, color: 'var(--text)' }}>
          You saved ${(savings / 100).toFixed(2)}!
        </div>
        <div style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', marginTop: '2px' }}>
          Share your smart swap with friends
        </div>
      </div>
      <button onClick={() => {
        const text = `I just saved $${(savings / 100).toFixed(2)} by swapping to ${brand} on Thriftly! 🔄`
        track('post_swap_share_tapped', { brand, savings })
        if (navigator.share) {
          navigator.share({ text }).catch(() => {})
        } else {
          navigator.clipboard?.writeText(text)
          showToast('Copied to clipboard!')
        }
        onDismiss()
      }} style={{
        backgroundColor: 'var(--brand)', color: '#FFF', border: 'none', borderRadius: '8px',
        fontFamily: FONT, fontSize: FONT_XS, fontWeight: 600, padding: '8px 14px', cursor: 'pointer',
      }}>Share</button>
      <button onClick={onDismiss} style={{
        background: 'none', border: 'none', color: 'var(--text-secondary)', fontSize: '1.125rem',
        cursor: 'pointer', padding: '4px', lineHeight: 1,
      }}>×</button>
    </div>
  )
}
