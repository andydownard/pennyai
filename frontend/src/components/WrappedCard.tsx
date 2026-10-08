import { useState, useEffect, useRef } from 'react'
import { appOrigin } from '../appUrl'
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

  const generateShareImage = async (): Promise<Blob | null> => {
    if (!data) return null
    const W = 1080, H = 1350 // Instagram portrait ratio
    const canvas = document.createElement('canvas')
    canvas.width = W
    canvas.height = H
    const ctx = canvas.getContext('2d')
    if (!ctx) return null

    // Background gradient
    const grad = ctx.createLinearGradient(0, 0, 0, H)
    grad.addColorStop(0, '#35644F')
    grad.addColorStop(1, '#2A4F3E')
    ctx.fillStyle = grad
    ctx.fillRect(0, 0, W, H)

    // Rounded card area
    const cardX = 60, cardY = 120, cardW = W - 120, cardH = H - 240
    ctx.fillStyle = '#FFFFFF'
    const r = 32
    ctx.beginPath()
    ctx.moveTo(cardX + r, cardY)
    ctx.lineTo(cardX + cardW - r, cardY)
    ctx.arcTo(cardX + cardW, cardY, cardX + cardW, cardY + r, r)
    ctx.lineTo(cardX + cardW, cardY + cardH - r)
    ctx.arcTo(cardX + cardW, cardY + cardH, cardX + cardW - r, cardY + cardH, r)
    ctx.lineTo(cardX + r, cardY + cardH)
    ctx.arcTo(cardX, cardY + cardH, cardX, cardY + cardH - r, r)
    ctx.lineTo(cardX, cardY + r)
    ctx.arcTo(cardX, cardY, cardX + r, cardY, r)
    ctx.closePath()
    ctx.fill()

    // Month header
    ctx.textAlign = 'center'
    ctx.fillStyle = '#35644F'
    ctx.font = '600 28px Avenir, sans-serif'
    ctx.fillText('MY MONTH IN REVIEW', W / 2, cardY + 80)
    ctx.fillStyle = '#2A2A2A'
    ctx.font = '700 56px Avenir, sans-serif'
    ctx.fillText(data.month, W / 2, cardY + 150)

    if (data.user_name) {
      ctx.fillStyle = '#525252'
      ctx.font = '400 28px Avenir, sans-serif'
      ctx.fillText(data.user_name, W / 2, cardY + 190)
    }

    // Big savings number — with green badge
    const savingsY = cardY + 300
    ctx.fillStyle = '#F0FFF4'
    ctx.beginPath()
    ctx.roundRect(cardX + 60, savingsY - 70, cardW - 120, 160, 20)
    ctx.fill()
    ctx.fillStyle = '#35644F'
    ctx.font = '700 96px Avenir, sans-serif'
    ctx.fillText(`$${(data.total_savings_cents / 100).toFixed(2)}`, W / 2, savingsY + 35)
    ctx.fillStyle = '#525252'
    ctx.font = '500 28px Avenir, sans-serif'
    ctx.fillText('saved this month', W / 2, savingsY + 75)

    // Stats grid
    const statsY = savingsY + 170
    const statData = [
      { emoji: '🔄', label: 'Smart Swaps', value: String(data.swaps_count) },
      { emoji: '📝', label: 'Items Added', value: String(data.items_added) },
    ]
    if (data.streak_days > 0) statData.push({ emoji: '🔥', label: 'Day Streak', value: String(data.streak_days) })
    if (data.top_category) statData.push({ emoji: '🏷️', label: 'Top Category', value: data.top_category })

    const cols = 2
    const boxW = (cardW - 120 - 20) / cols
    const boxH = 120
    statData.forEach((stat, i) => {
      const col = i % cols
      const row = Math.floor(i / cols)
      const x = cardX + 60 + col * (boxW + 20)
      const y = statsY + row * (boxH + 16)

      ctx.fillStyle = '#F7F6F3'
      ctx.beginPath()
      ctx.roundRect(x, y, boxW, boxH, 14)
      ctx.fill()

      ctx.font = '32px sans-serif'
      ctx.fillText(stat.emoji, x + boxW / 2, y + 38)
      ctx.fillStyle = '#2A2A2A'
      ctx.font = '700 36px Avenir, sans-serif'
      ctx.fillText(stat.value, x + boxW / 2, y + 76)
      ctx.fillStyle = '#525252'
      ctx.font = '400 20px Avenir, sans-serif'
      ctx.fillText(stat.label, x + boxW / 2, y + 102)
    })

    // Favorite swap
    if (data.top_swap_brand) {
      const favY = statsY + Math.ceil(statData.length / cols) * (boxH + 16) + 20
      ctx.fillStyle = '#525252'
      ctx.font = '400 28px Avenir, sans-serif'
      ctx.fillText(`Favorite swap: ${data.top_swap_brand}`, W / 2, favY)
    }

    // Footer branding
    ctx.fillStyle = '#FFFFFF'
    ctx.font = '600 32px Avenir, sans-serif'
    ctx.fillText('groflo.io', W / 2, H - 80)
    ctx.fillStyle = 'rgba(255,255,255,0.6)'
    ctx.font = '400 22px Avenir, sans-serif'
    ctx.fillText('Smart grocery savings', W / 2, H - 45)

    return new Promise(resolve => canvas.toBlob(blob => resolve(blob), 'image/png'))
  }

  const handleShare = async () => {
    if (!data) return
    track('wrapped_shared', { month: data.month, savings: data.total_savings_cents })

    const blob = await generateShareImage()
    if (blob && navigator.share && navigator.canShare?.({ files: [new File([blob], 'groflo-wrapped.png', { type: 'image/png' })] })) {
      const file = new File([blob], 'groflo-wrapped.png', { type: 'image/png' })
      navigator.share({ files: [file], text: data.share_text }).catch(() => {})
    } else if (blob) {
      // Fallback: download the image
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'groflo-wrapped.png'
      a.click()
      URL.revokeObjectURL(url)
    } else {
      // Ultimate fallback: text-only sharing
      const shareText = data.share_text + `\n\nGet Groflo: ${appOrigin()}`
      if (navigator.share) {
        navigator.share({ text: shareText }).catch(() => {})
      } else {
        navigator.clipboard?.writeText(shareText)
      }
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

        {/* Groflo branding */}
        <div style={{ ...text.caption, textAlign: 'center', marginBottom: '16px' }}>
          groflo.io
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
