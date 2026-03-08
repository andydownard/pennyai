import { useState } from 'react'
import { SKELETON, TEXT_SECONDARY, FONT } from '../styles'

export function BrandLogo({ brand, size = 52, opacity = 1 }: { brand?: string; size?: number; opacity?: number }) {
  const [failed, setFailed] = useState(false)
  const radius = size > 60 ? '12px' : size > 48 ? '10px' : '8px'
  const pad = size > 60 ? '4px' : size > 48 ? '3px' : '2px'
  const fontSize = Math.max(14, Math.round(size * 0.36))

  // Monogram fallback: first letter of brand
  const monogram = brand ? brand.charAt(0).toUpperCase() : ''

  return (
    <div style={{ width: size, height: size, flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      {brand && !failed ? (
        <img
          src={`/api/logo?brand=${encodeURIComponent(brand)}&v=4`}
          alt=""
          width={size}
          height={size}
          loading="lazy"
          style={{ borderRadius: radius, objectFit: 'contain' as const, backgroundColor: '#FAFAFA', border: '1px solid rgba(0,0,0,0.06)', padding: pad, opacity, transition: 'opacity 0.15s' }}
          onError={() => setFailed(true)}
        />
      ) : (
        <div style={{
          width: size, height: size, borderRadius: radius,
          backgroundColor: SKELETON, border: '1px solid rgba(0,0,0,0.06)', opacity,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontFamily: FONT, fontSize, fontWeight: 600, color: TEXT_SECONDARY,
        }}>
          {monogram}
        </div>
      )}
    </div>
  )
}
