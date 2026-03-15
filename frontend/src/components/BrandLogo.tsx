import { useState } from 'react'
import { getItemEmoji, DEPT_ICONS } from '../styles'

export function BrandLogo({ brand, size = 52, opacity = 1, itemName, department, emoji }: { brand?: string; size?: number; opacity?: number; itemName?: string; department?: string; emoji?: string }) {
  const [failed, setFailed] = useState(false)
  const radius = size > 60 ? '12px' : size > 48 ? '10px' : '8px'
  const pad = size > 60 ? '4px' : size > 48 ? '3px' : '2px'
  const emojiSize = Math.max(16, Math.round(size * 0.5))

  // Emoji fallback priority: item-specific keyword match → stored AI emoji → department → 🛒
  // Always compute from item name first — the keyword list is more granular than
  // what the backend may have stored (which can be a department-level emoji).
  const computed = getItemEmoji(itemName || brand || '', department)
  const deptEmoji = department ? DEPT_ICONS[department] : undefined
  const isComputedSpecific = computed !== deptEmoji && computed !== '🛒'
  const fallbackEmoji = isComputedSpecific ? computed : (emoji || computed)

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
          backgroundColor: 'transparent', border: '1px solid color-mix(in srgb, var(--border) 50%, transparent)', opacity,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: emojiSize, lineHeight: 1,
        }}>
          {fallbackEmoji}
        </div>
      )}
    </div>
  )
}
