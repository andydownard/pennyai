import { FONT } from '../styles'

/**
 * Row amount chip from the Groflo money display design (board 1).
 * "taken": a swap already made (filled, with a check). "offer": a deal still on offer (outlined).
 */
export function MoneyChip({ cents, kind }: { cents: number; kind: 'taken' | 'offer' }) {
  const taken = kind === 'taken'
  const amount = `$${(cents / 100).toFixed(2)}`
  return (
    <span
      style={{
        display: 'inline-flex', alignItems: 'center', gap: '4px',
        padding: taken ? '4px 12px' : '3px 12px', borderRadius: '999px',
        fontFamily: FONT, fontSize: '0.875rem', fontWeight: 600,
        fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap',
        background: taken ? 'var(--brand-tint, color-mix(in srgb, var(--brand) 8%, transparent))' : 'var(--surface)',
        color: 'var(--brand)',
        border: taken ? 'none' : '1.5px solid var(--brand)',
      }}
    >
      {taken && (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><polyline points="20 6 9 17 4 12" /></svg>
      )}
      {taken ? `Saving ${amount}` : `Deal · save ${amount}`}
    </span>
  )
}
