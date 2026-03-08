import { BrandLogo } from './BrandLogo'
import { SwapCard, type SwapOffer } from './SwapCard'
import {
  BRAND, TEXT, TEXT_SECONDARY, WHITE, FONT, BORDER,
  ERROR_RED, ERROR_RED_BG, SUCCESS_GREEN, SUCCESS_GREEN_BG, SUCCESS_GREEN_LIGHT,
} from '../styles'

export type ListItem = {
  id: number
  name: string
  brand: string
  department: string
  quantity: number
  size: string
  notes: string
  source: string
  checked: boolean
  urgent: boolean
  created_at: string
  added_by_name: string
  added_by_phone: string
  claimed_by_id: number | null
  claimed_by_name: string
  swap_savings_cents: number
}

interface ItemRowProps {
  item: ListItem
  isNew: boolean
  currentUserId: number | null
  swap?: SwapOffer
  onToggle: (id: number) => void
  onTap: (item: ListItem) => void
  onSwapAccept: (dealId: number, itemId: number) => void
  onSwapDecline: (dealId: number, itemId: number) => void
}

function relativeTimestamp(createdAt: string): string | null {
  const created = new Date(createdAt)
  if (isNaN(created.getTime())) return null
  const diffMs = Date.now() - created.getTime()
  if (diffMs < 0 || diffMs >= 3600000) return null
  const diffSec = Math.floor(diffMs / 1000)
  if (diffSec < 60) return 'just now'
  const diffMin = Math.floor(diffSec / 60)
  return `${diffMin} min ago`
}

const styles = {
  row: {
    display: 'flex',
    alignItems: 'center',
    borderBottom: '1px solid var(--border)',
    backgroundColor: 'var(--surface)',
  } as React.CSSProperties,

  checkBtn: {
    padding: '10px 10px 10px 20px',
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    flexShrink: 0,

    display: 'flex',
    alignItems: 'center',
  } as React.CSSProperties,

  check: (checked: boolean): React.CSSProperties => ({
    width: '22px',
    height: '22px',
    borderRadius: '50%',
    border: `2px solid ${checked ? '#C5C0B8' : BORDER}`,
    backgroundColor: checked ? '#C5C0B8' : 'transparent',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
    fontSize: '12px',
    fontWeight: 700,
    color: WHITE,
    transition: 'all 0.15s',
  }),

  center: {
    flex: 1,
    padding: '10px 0',
    cursor: 'pointer',
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '2px',

  } as React.CSSProperties,

  name: (checked: boolean): React.CSSProperties => ({
    fontFamily: FONT,
    fontSize: '17px',
    color: checked ? TEXT_SECONDARY : TEXT,
    textDecoration: checked ? 'line-through' : 'none',
    transition: 'all 0.15s',
  }),

  meta: {
    fontFamily: FONT,
    fontSize: '12px',
    color: TEXT_SECONDARY,
  } as React.CSSProperties,
}

export function ItemRow({ item, isNew, currentUserId, swap, onToggle, onTap, onSwapAccept, onSwapDecline }: ItemRowProps) {
  const ts = relativeTimestamp(item.created_at)

  return (
    <div>
      <div style={{
        ...styles.row,
        position: 'relative' as const,
        ...(item.checked ? { backgroundColor: 'var(--bg)', opacity: 0.7 } : {}),
        ...(isNew ? {
          backgroundColor: `${BRAND}15`,
          transition: 'background-color 1.5s ease',
        } : {}),
        ...(!item.checked && item.urgent ? { backgroundColor: ERROR_RED_BG } : {}),
      }}>
        <button onClick={() => onToggle(item.id)} style={styles.checkBtn} aria-label={`${item.checked ? 'Uncheck' : 'Check off'} ${item.brand && !item.name.toLowerCase().includes(item.brand.toLowerCase()) ? item.brand + ' ' : ''}${item.name}`}>
          <span style={styles.check(item.checked)} aria-hidden="true">
            {item.checked && '✓'}
          </span>
        </button>
        <div style={{ marginRight: '10px' }}>
          <BrandLogo brand={item.brand} size={52} opacity={item.checked ? 0.4 : 1} />
        </div>
        <div style={styles.center} onClick={() => onTap(item)} role="button" tabIndex={0} aria-label={`Edit ${item.brand && !item.name.toLowerCase().includes(item.brand.toLowerCase()) ? item.brand + ' ' : ''}${item.name}`} onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onTap(item) } }}>
          <span style={styles.name(item.checked)}>
            {item.brand && !item.name.toLowerCase().includes(item.brand.toLowerCase()) ? `${item.brand} ` : ''}{item.name}
          </span>
          {(item.quantity > 1 || item.size || item.notes) && (
            <span style={styles.meta}>
              {item.quantity > 1 ? `×${item.quantity}` : ''}{item.quantity > 1 && item.size ? ' · ' : ''}{item.size}{(item.quantity > 1 || item.size) && item.notes ? ' · ' : ''}{item.notes}
            </span>
          )}
          {ts && (
            <span style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY }}>
              {ts}
            </span>
          )}
        </div>
        {(item.urgent || item.claimed_by_id || item.swap_savings_cents > 0) && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', alignItems: 'flex-end', flexShrink: 0, paddingRight: '12px' }}>
            {item.swap_savings_cents > 0 && (
              <span style={{
                backgroundColor: SUCCESS_GREEN_BG, color: SUCCESS_GREEN, border: `1px solid ${SUCCESS_GREEN_LIGHT}`,
                fontSize: '12px', fontWeight: 600, borderRadius: '10px', padding: '2px 8px',
                whiteSpace: 'nowrap',
              }}>Saved ${(item.swap_savings_cents / 100).toFixed(2)}</span>
            )}
            {item.urgent && (
              <span style={{
                backgroundColor: ERROR_RED_BG, color: ERROR_RED, border: `1px solid ${ERROR_RED}40`,
                fontSize: '12px', fontWeight: 600, borderRadius: '10px', padding: '2px 8px',
                whiteSpace: 'nowrap',
              }}>🔥 urgent</span>
            )}
            {item.claimed_by_id && (
              <span style={{
                fontSize: '12px', fontWeight: 600, borderRadius: '10px', padding: '2px 8px',
                whiteSpace: 'nowrap',
                ...(item.claimed_by_id === currentUserId
                  ? { backgroundColor: SUCCESS_GREEN_BG, color: SUCCESS_GREEN, border: `1px solid ${SUCCESS_GREEN_LIGHT}` }
                  : { backgroundColor: 'var(--bg)', color: 'var(--text)', border: '1px solid var(--border)' }),
              }}>
                {item.claimed_by_id === currentUserId ? 'Me' : (item.claimed_by_name?.[0] || '?')}
              </span>
            )}
          </div>
        )}
      </div>
      {swap && !item.checked && (
        <div style={{ padding: '0 8px 4px 8px' }}>
          <SwapCard
            swap={swap}
            onAccept={onSwapAccept}
            onDecline={onSwapDecline}
          />
        </div>
      )}
    </div>
  )
}
