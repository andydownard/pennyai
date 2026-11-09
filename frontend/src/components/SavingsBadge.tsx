import { FONT, FONT_LG, layout } from '../styles'

/**
 * Header total: a 17px amount over a 12px caption, as in the Groflo money
 * display design (board 1). Potential savings, else earned, else "deals".
 */
export function SavingsBadge({ potentialSavingsCents, earningsTotalCents, onOpen }: {
  potentialSavingsCents: number
  earningsTotalCents: number
  onOpen: () => void
}) {
  const cents = potentialSavingsCents > 0 ? potentialSavingsCents : earningsTotalCents
  const label = potentialSavingsCents > 0 ? 'save' : earningsTotalCents > 0 ? 'saved' : 'deals'
  return (
    <button
      type="button"
      onClick={onOpen}
      aria-label="View savings"
      style={{
        ...layout.column,
        marginLeft: 'auto',
        alignItems: 'center',
        minWidth: '56px',
        lineHeight: 1.1,
        background: 'var(--brand-tint, color-mix(in srgb, var(--brand) 8%, transparent))',
        border: 'none',
        borderRadius: '12px',
        padding: '6px 12px',
        cursor: 'pointer',
        fontFamily: FONT,
      }}
    >
      {cents > 0 && (
        <b style={{ fontSize: FONT_LG, fontWeight: 600, color: 'var(--brand)', fontVariantNumeric: 'tabular-nums' }}>
          ${(cents / 100).toFixed(2)}
        </b>
      )}
      <span style={{ fontSize: '0.75rem', color: cents > 0 ? 'var(--text-secondary)' : 'var(--brand)', fontWeight: cents > 0 ? 400 : 600 }}>{label}</span>
    </button>
  )
}
