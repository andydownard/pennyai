import type { ReactNode } from 'react'
import { FONT, FONT_2XS } from '../styles'

export function FloatingLabelField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label style={{ textAlign: 'left' as const, display: 'block' }}>
      <span style={{
        fontSize: FONT_2XS, fontWeight: 600, color: 'var(--text-secondary)',
        textTransform: 'uppercase' as const, letterSpacing: '0.5px', fontFamily: FONT,
        marginBottom: '6px', display: 'block',
      }}>{label}</span>
      {children}
    </label>
  )
}
