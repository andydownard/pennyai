import { BORDER } from '../styles'

export function PennyAvatar({ size = 36 }: { size?: number }) {
  const isLarge = size > 80
  return (
    <div style={{
      width: size,
      height: size,
      borderRadius: '50%',
      overflow: 'hidden',
      flexShrink: 0,
      backgroundColor: '#e8e4df',
      border: isLarge ? `2px solid ${BORDER}` : `1px solid ${BORDER}`,
      boxShadow: isLarge ? '0 4px 20px rgba(0,0,0,0.08)' : 'none',
    }}>
      <img
        src="/penny.png"
        alt="Penny"
        width={size}
        height={size}
        style={{ objectFit: 'cover', objectPosition: 'center 15%', display: 'block' }}
      />
    </div>
  )
}
