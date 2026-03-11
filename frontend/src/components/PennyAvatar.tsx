// Uses CSS custom properties for dark mode reactivity

export function PennyAvatar({ size = 36 }: { size?: number }) {
  const isLarge = size > 80
  return (
    <div style={{
      width: size,
      height: size,
      borderRadius: '50%',
      overflow: 'hidden',
      flexShrink: 0,
      backgroundColor: 'var(--skeleton)',
      border: isLarge ? '2px solid var(--border)' : '1px solid var(--border)',
      boxShadow: isLarge ? '0 4px 20px rgba(0,0,0,0.08)' : 'none',
    }}>
      <img
        src="/penny.png?v=2"
        alt="Penny"
        width={size}
        height={size}
        style={{ objectFit: 'cover', objectPosition: 'center 20%', display: 'block' }}
      />
    </div>
  )
}
