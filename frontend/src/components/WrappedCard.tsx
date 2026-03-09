import { useState, useEffect, useRef } from 'react'
import { track } from '../analytics'
import { fetchAPI } from '../api'
import { FONT, FONT_XS, FONT_BASE, FONT_MD, layout, text } from '../styles'
// Uses CSS custom properties for dark mode reactivity

interface WrappedData {
  month: string
  total_savings_cents: number
  swaps_count: number
  items_added: number
  top_swap_brand: string
  top_category: string
  streak_days: number
  share_text: string
  user_name: string
}

interface WrappedCardProps {
  onClose: () => void
}

export function WrappedCard({ onClose }: WrappedCardProps) {
  const [data, setData] = useState<WrappedData | null>(null)
  const [loading, setLoading] = useState(true)
  const cardRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    fetchAPI('/api/wrapped')
      .then(d => {
        if (d && !Array.isArray(d)) {
          const wrapped = d as unknown as WrappedData
          setData(wrapped)
          track('wrapped_viewed', { month: wrapped.month })
        }
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [fetchAPI])

  const handleShare = () => {
    if (!data) return
    track('wrapped_shared', { month: data.month, savings: data.total_savings_cents })
    const text = data.share_text + '\n\nGet Thriftly: https://app.thrift.ly'
    if (navigator.share) {
      navigator.share({ text }).catch(() => {})
    } else {
      navigator.clipboard?.writeText(text)
    }
  }

  if (loading) {
    return (
      <div style={overlayStyle}>
        <div style={{ ...cardStyle, textAlign: 'center', padding: '48px 24px' }}>
          <div style={{ fontFamily: FONT, fontSize: FONT_MD, color: 'var(--text-secondary)' }}>Loading your month...</div>
        </div>
      </div>
    )
  }

  if (!data || data.total_savings_cents === 0 && data.swaps_count === 0 && data.items_added === 0) {
    return (
      <div style={overlayStyle}>
        <div style={{ ...cardStyle, textAlign: 'center', padding: '48px 24px' }}>
          <div style={{ fontSize: '48px', marginBottom: '12px' }}>📋</div>
          <div style={{ fontFamily: FONT, fontSize: '1.125rem', fontWeight: 600, color: 'var(--text)' }}>No activity yet this month</div>
          <div style={{ ...text.secondary, marginTop: '8px' }}>Add items and accept swaps to see your monthly recap!</div>
          <button onClick={onClose} style={closeBtnStyle}>Close</button>
        </div>
      </div>
    )
  }

  const savings = `$${(data.total_savings_cents / 100).toFixed(2)}`

  return (
    <div style={overlayStyle}>
      <div ref={cardRef} style={cardStyle}>
        {/* Header */}
        <div style={{ textAlign: 'center', marginBottom: '24px' }}>
          <div style={{ fontFamily: FONT, fontSize: FONT_XS, fontWeight: 600, color: 'var(--brand)', textTransform: 'uppercase', letterSpacing: '2px' }}>Your Month in Review</div>
          <div style={{ fontFamily: FONT, fontSize: '1.75rem', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>{data.month}</div>
          {data.user_name && <div style={{ ...text.secondary, marginTop: '4px' }}>{data.user_name}</div>}
        </div>

        {/* Big savings number */}
        <div style={{ textAlign: 'center', padding: '20px', backgroundColor: 'var(--success-bg, #F0FFF0)', borderRadius: '16px', marginBottom: '20px' }}>
          <div style={{ fontFamily: FONT, fontSize: '3rem', fontWeight: 700, color: 'var(--brand)' }}>{savings}</div>
          <div style={text.secondary}>saved this month</div>
        </div>

        {/* Stats grid */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '20px' }}>
          <StatBox emoji="🔄" label="Smart Swaps" value={String(data.swaps_count)} />
          <StatBox emoji="📝" label="Items Added" value={String(data.items_added)} />
          {data.streak_days > 0 && <StatBox emoji="🔥" label="Day Streak" value={String(data.streak_days)} />}
          {data.top_category && <StatBox emoji="🏷️" label="Top Category" value={data.top_category} />}
        </div>

        {/* Top brand */}
        {data.top_swap_brand && (
          <div style={{ ...text.secondary, textAlign: 'center', marginBottom: '20px' }}>
            Your favorite swap: <b style={{ color: 'var(--text)' }}>{data.top_swap_brand}</b>
          </div>
        )}

        {/* Thriftly branding */}
        <div style={{ ...text.caption, textAlign: 'center', marginBottom: '16px' }}>
          thrift.ly
        </div>

        {/* Actions */}
        <div style={{ ...layout.row, gap: '12px' }}>
          <button onClick={handleShare} style={{
            flex: 1, backgroundColor: 'var(--brand)', color: '#fff', border: 'none', borderRadius: '12px',
            fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, padding: '14px', cursor: 'pointer',
          }}>Share My Wrapped</button>
          <button onClick={onClose} style={closeBtnStyle}>Close</button>
        </div>
      </div>
    </div>
  )
}

function StatBox({ emoji, label, value }: { emoji: string; label: string; value: string }) {
  return (
    <div style={{ padding: '14px', backgroundColor: 'var(--bg)', borderRadius: '12px', textAlign: 'center' }}>
      <div style={{ fontSize: '20px' }}>{emoji}</div>
      <div style={{ fontFamily: FONT, fontSize: '1.25rem', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>{value}</div>
      <div style={text.caption}>{label}</div>
    </div>
  )
}

const overlayStyle: React.CSSProperties = {
  position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
  backgroundColor: 'rgba(0,0,0,0.5)', display: 'flex',
  alignItems: 'center', justifyContent: 'center', padding: '24px', zIndex: 1000,
}

const cardStyle: React.CSSProperties = {
  backgroundColor: 'var(--surface)', borderRadius: '20px', padding: '28px',
  maxWidth: '380px', width: '100%', maxHeight: '90vh', overflowY: 'auto',
}

const closeBtnStyle: React.CSSProperties = {
  backgroundColor: 'var(--bg)', color: 'var(--text-secondary)', border: 'none', borderRadius: '12px',
  fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 500, padding: '14px 20px', cursor: 'pointer',
}
