import { FONT } from '../styles'

type BadgeVariant = 'success' | 'warning' | 'error' | 'info' | 'neutral'

const VARIANT_STYLES: Record<BadgeVariant, { bg: string; color: string }> = {
  success: { bg: 'var(--success-bg)', color: 'var(--success)' },
  warning: { bg: 'var(--warning-bg)', color: 'var(--warning)' },
  error: { bg: 'var(--error-bg)', color: 'var(--error)' },
  info: { bg: 'var(--info-bg)', color: 'var(--info)' },
  neutral: { bg: 'var(--neutral-bg)', color: 'var(--text-secondary)' },
}

interface StatusBadgeProps {
  label: string
  variant: BadgeVariant
  /** Optional border in the variant color at 25% opacity */
  bordered?: boolean
}

export function StatusBadge({ label, variant, bordered }: StatusBadgeProps) {
  const v = VARIANT_STYLES[variant]
  return (
    <span style={{
      fontFamily: FONT,
      fontSize: '11px',
      fontWeight: 600,
      padding: '2px 8px',
      borderRadius: '10px',
      backgroundColor: v.bg,
      color: v.color,
      whiteSpace: 'nowrap',
      ...(bordered ? { border: `1px solid color-mix(in srgb, ${v.color} 25%, transparent)` } : {}),
    }}>
      {label}
    </span>
  )
}
