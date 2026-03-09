import { FONT, FONT_2XS } from '../styles'

interface SectionHeaderProps {
  label: string
  /** Optional margin-bottom override, default 12px */
  marginBottom?: string
}

export function SectionHeader({ label, marginBottom = '12px' }: SectionHeaderProps) {
  return (
    <p style={{
      fontFamily: FONT,
      fontSize: FONT_2XS,
      fontWeight: 600,
      color: 'var(--text-secondary)',
      textTransform: 'uppercase' as const,
      letterSpacing: '0.5px',
      marginBottom,
      marginTop: 0,
    }}>
      {label}
    </p>
  )
}
