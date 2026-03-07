import { useMemo, useEffect, useRef } from 'react'
import { CONFETTI_COLORS } from '../styles'

export function Confetti({ onComplete }: { onComplete: () => void }) {
  const particles = useMemo(() =>
    Array.from({ length: 120 }, (_, i) => ({
      id: i,
      color: CONFETTI_COLORS[i % CONFETTI_COLORS.length],
      left: Math.random() * 100,
      size: 6 + Math.random() * 4,
      delay: Math.random() * 1.2,
      duration: 2 + Math.random() * 1.5,
    })), [])

  const onCompleteRef = useRef(onComplete)
  onCompleteRef.current = onComplete
  useEffect(() => {
    const timer = setTimeout(() => onCompleteRef.current(), 5000)
    return () => clearTimeout(timer)
  }, [])

  return (
    <div style={{ position: 'fixed', inset: 0, pointerEvents: 'none', zIndex: 9999, overflow: 'hidden' }}>
      {particles.map((p: { id: number; color: string; left: number; size: number; delay: number; duration: number }) => (
        <div key={p.id} style={{
          position: 'absolute',
          top: '-10px',
          left: `${p.left}%`,
          width: `${p.size}px`,
          height: `${p.size}px`,
          backgroundColor: p.color,
          borderRadius: p.id % 3 === 0 ? '50%' : '2px',
          animation: `confetti-fall ${p.duration}s ease-in ${p.delay}s forwards`,
          opacity: 0,
        }} />
      ))}
    </div>
  )
}
