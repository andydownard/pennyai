import type { ReactNode } from 'react'
import { FONT, TEXT_SECONDARY } from '../styles'

export function FloatingLabelField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div style={{ textAlign: 'left' as const }}>
      <div style={{
        fontSize: '11px', fontWeight: 600, color: TEXT_SECONDARY,
        textTransform: 'uppercase' as const, letterSpacing: '0.5px', fontFamily: FONT,
        marginBottom: '6px',
      }}>{label}</div>
      {children}
    </div>
  )
}
