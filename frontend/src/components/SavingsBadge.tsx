import { FONT, FONT_LG, layout } from '../styles'

/**
 * Header total: the amount alone, as in the Groflo prototype's Home header
 * (its label is empty). Potential savings, else earned, else $0.00.
 */
export function SavingsBadge({ potentialSavingsCents, earningsTotalCents, onOpen }: {
  potentialSavingsCents: number
  earningsTotalCents: number
  onOpen: () => void
}) {
  const cents = potentialSavingsCents > 0 ? potentialSavingsCents : earningsTotalCents
  return (
    <button
      type="button"
      onClick={onOpen}
      aria-label={`View savings, $${(cents / 100).toFixed(2)}`}
      style={{
        ...layout.column,
        marginLeft: 'auto',
        alignItems: 'center',
        justifyContent: 'center',
        minWidth: '56px',
        minHeight: '44px',
        lineHeight: 1.1,
        background: 'var(--brand-tint)',
        border: 'none',
        borderRadius: '12px',
        padding: '6px 12px',
        cursor: 'pointer',
        fontFamily: FONT,
      }}
    >
      <b style={{ fontSize: FONT_LG, fontWeight: 600, color: 'var(--brand)', fontVariantNumeric: 'tabular-nums' }}>
        ${(cents / 100).toFixed(2)}
      </b>
    </button>
  )
}
