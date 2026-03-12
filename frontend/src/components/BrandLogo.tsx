import { useState } from 'react'
import { getItemEmoji } from '../styles'

export function BrandLogo({ brand, size = 52, opacity = 1, itemName, department, emoji }: { brand?: string; size?: number; opacity?: number; itemName?: string; department?: string; emoji?: string }) {
  const [failed, setFailed] = useState(false)
  const radius = size > 60 ? '12px' : size > 48 ? '10px' : '8px'
  const pad = size > 60 ? '4px' : size > 48 ? '3px' : '2px'
  const emojiSize = Math.max(16, Math.round(size * 0.5))

  // Emoji fallback priority: stored emoji from AI → static keyword match → department → 🛒
  const fallbackEmoji = emoji || getItemEmoji(itemName || brand || '', department)

  return (
    <div style={{ width: size, height: size, flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      {brand && !failed ? (
        <img
          src={`/api/logo?brand=${encodeURIComponent(brand)}&v=4`}
          alt=""
          width={size}
          height={size}
          loading="lazy"
          style={{ borderRadius: radius, objectFit: 'contain' as const, backgroundColor: 'var(--surface)', border: '1px solid var(--border)', padding: pad, opacity, transition: 'opacity 0.15s' }}
          onError={() => setFailed(true)}
        />
      ) : (
        <div style={{
          width: size, height: size, borderRadius: radius,
          backgroundColor: 'var(--skeleton)', border: '1px solid var(--border)', opacity,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: emojiSize, lineHeight: 1,
        }}>
          {fallbackEmoji}
        </div>
      )}
    </div>
  )
}
