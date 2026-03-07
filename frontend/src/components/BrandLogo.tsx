export function BrandLogo({ brand, size = 52, opacity = 1 }: { brand?: string; size?: number; opacity?: number }) {
  const radius = size > 60 ? '12px' : size > 48 ? '10px' : '8px'
  const pad = size > 60 ? '4px' : size > 48 ? '3px' : '2px'
  return (
    <div style={{ width: size, height: size, flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      {brand ? (
        <img
          src={`/api/logo?brand=${encodeURIComponent(brand)}&v=4`}
          alt=""
          width={size}
          height={size}
          loading="lazy"
          style={{ borderRadius: radius, objectFit: 'contain' as const, backgroundColor: '#FAFAFA', border: '1px solid rgba(0,0,0,0.06)', padding: pad, opacity, transition: 'opacity 0.15s' }}
          onError={e => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
      ) : (
        <div style={{ width: size, height: size, borderRadius: radius, backgroundColor: '#FAFAFA', border: '1px solid rgba(0,0,0,0.06)', opacity }} />
      )}
    </div>
  )
}
