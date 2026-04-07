import { useState } from 'react'
import { track } from '../analytics'
import { fetchAPI } from '../api'
import { FONT, FONT_2XS, FONT_XS, FONT_BASE, FONT_MD } from '../styles'
// Uses CSS custom properties for dark mode reactivity

interface MilestoneData {
  cents: number
  dollars: string
  share_text: string
  total_cents: number
}

interface MilestoneCardProps {
  milestone: MilestoneData
  onClose: () => void
}

export function MilestoneCard({ milestone, onClose }: MilestoneCardProps) {
  const [shared, setShared] = useState(false)

  const handleShare = () => {
    track('milestone_shared', { cents: milestone.cents })
    setShared(true)
    const nav = navigator as Navigator & { share?: (d: { text: string }) => Promise<void> }
    if (nav.share) {
      nav.share({ text: milestone.share_text }).catch(() => {})
    } else {
      navigator.clipboard?.writeText(milestone.share_text)
    }
  }

  const handleClose = () => {
    // Acknowledge the milestone so it doesn't show again
    fetchAPI('/api/milestones/ack', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cents: milestone.cents }),
    }).catch(() => {})
    track('milestone_dismissed', { cents: milestone.cents })
    onClose()
  }

  const emoji = milestone.cents >= 10000 ? '🏆' : milestone.cents >= 5000 ? '🌟' : milestone.cents >= 2500 ? '🎯' : '🎉'

  return (
    <div style={overlayStyle}>
      <div style={cardStyle}>
        {/* Celebration header */}
        <div style={{ textAlign: 'center', marginBottom: '20px' }}>
          <div style={{ fontSize: '56px', marginBottom: '8px' }}>{emoji}</div>
          <div style={{ fontFamily: FONT, fontSize: FONT_XS, fontWeight: 600, color: 'var(--brand)', textTransform: 'uppercase' as const, letterSpacing: '2px' }}>
            Savings Milestone
          </div>
        </div>

        {/* Big milestone number */}
        <div style={{
          textAlign: 'center', padding: '24px', borderRadius: '16px', marginBottom: '20px',
          background: 'linear-gradient(135deg, var(--success-bg) 0%, color-mix(in srgb, var(--success) 10%, var(--bg)) 100%)',
        }}>
          <div style={{ fontFamily: FONT, fontSize: '3.5rem', fontWeight: 700, color: 'var(--brand)' }}>
            {milestone.dollars}
          </div>
          <div style={{ fontFamily: FONT, fontSize: FONT_MD, color: 'var(--text-secondary)', marginTop: '4px' }}>
            in total savings!
          </div>
        </div>

        {/* Motivational message */}
        <div style={{
          fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text)', textAlign: 'center',
          lineHeight: '1.5', marginBottom: '24px', padding: '0 8px',
        }}>
          {milestone.cents >= 10000
            ? "You're a savings pro! You've earned over $100 just by shopping smarter."
            : milestone.cents >= 5000
            ? "Incredible milestone! Brands are literally paying you to shop."
            : milestone.cents >= 2500
            ? "You're on a roll! Keep swapping and watch your savings grow."
            : "You're off to a great start! Every swap puts money back in your pocket."}
        </div>

        {/* Groflo branding */}
        <div style={{ textAlign: 'center', fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', marginBottom: '16px' }}>
          groflo.io
        </div>

        {/* Actions */}
        <div style={{ display: 'flex', gap: '12px' }}>
          <button onClick={handleShare} style={{
            flex: 1, backgroundColor: 'var(--brand)', color: '#fff', border: 'none', borderRadius: '12px',
            fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, padding: '14px', cursor: 'pointer',
          }}>
            {shared ? 'Shared!' : 'Share Achievement'}
          </button>
          <button onClick={handleClose} style={closeBtnStyle}>
            Nice!
          </button>
        </div>
      </div>
    </div>
  )
}

const overlayStyle: React.CSSProperties = {
  position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
  backgroundColor: 'rgba(0,0,0,0.5)', display: 'flex',
  alignItems: 'center', justifyContent: 'center', padding: '24px', zIndex: 1001,
}

const cardStyle: React.CSSProperties = {
  backgroundColor: 'var(--surface)', borderRadius: '20px', padding: '28px',
  maxWidth: '380px', width: '100%', maxHeight: '90vh', overflowY: 'auto',
}

const closeBtnStyle: React.CSSProperties = {
  backgroundColor: 'var(--bg)', color: 'var(--text-secondary)', border: 'none', borderRadius: '12px',
  fontFamily: FONT,
  fontSize: FONT_BASE, fontWeight: 500, padding: '14px 20px', cursor: 'pointer',
}
