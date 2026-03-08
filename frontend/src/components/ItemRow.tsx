import { useState, useRef, useCallback } from 'react'
import { BrandLogo } from './BrandLogo'
import { SwapCard, type SwapOffer } from './SwapCard'
import { FONT, DEPT_ICONS } from '../styles'

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
  onDelete: (id: number) => void
  onLongPress: (id: number) => void
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
    padding: '12px 8px 12px 14px',
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    flexShrink: 0,
    minHeight: '44px',
    display: 'flex',
    alignItems: 'center',
  } as React.CSSProperties,

  check: (checked: boolean): React.CSSProperties => ({
    width: '20px',
    height: '20px',
    borderRadius: '50%',
    border: `2px solid ${checked ? 'var(--brand)' : 'var(--text-secondary)'}`,
    backgroundColor: checked ? 'var(--brand)' : 'transparent',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
    fontSize: '12px',
    fontWeight: 700,
    color: '#fff',
    transition: 'all 0.2s ease',
  }),

  center: {
    flex: 1,
    padding: '7px 0',
    cursor: 'pointer',
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '1px',

  } as React.CSSProperties,

  name: (checked: boolean): React.CSSProperties => ({
    fontFamily: FONT,
    fontSize: '15px',
    color: checked ? 'var(--text-secondary)' : 'var(--text)',
    textDecoration: checked ? 'line-through' : 'none',
    transition: 'all 0.15s',
  }),

  meta: {
    fontFamily: FONT,
    fontSize: '11px',
    color: 'var(--text-secondary)',
  } as React.CSSProperties,
}

const DELETE_THRESHOLD = 80
const REDUCED_MOTION = typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

export function ItemRow({ item, isNew, currentUserId, swap, onToggle, onTap, onDelete, onLongPress, onSwapAccept, onSwapDecline }: ItemRowProps) {
  const ts = relativeTimestamp(item.created_at)
  const [swipeX, setSwipeX] = useState(0)
  const touchRef = useRef<{ startX: number; startY: number; swiping: boolean } | null>(null)
  const longPressTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const longPressFired = useRef(false)

  const onTouchStart = useCallback((e: React.TouchEvent) => {
    const t = e.touches[0]
    touchRef.current = { startX: t.clientX, startY: t.clientY, swiping: false }
    longPressFired.current = false
    longPressTimer.current = setTimeout(() => {
      longPressFired.current = true
      touchRef.current = null // prevent swipe after long-press
      onLongPress(item.id)
    }, 500)
  }, [onLongPress, item.id])

  const onTouchMove = useCallback((e: React.TouchEvent) => {
    if (!touchRef.current) return
    const t = e.touches[0]
    const dx = t.clientX - touchRef.current.startX
    const dy = t.clientY - touchRef.current.startY

    // Cancel long-press on any movement
    if (Math.abs(dx) > 5 || Math.abs(dy) > 5) {
      if (longPressTimer.current) { clearTimeout(longPressTimer.current); longPressTimer.current = null }
    }

    // Decide direction on first significant move
    if (!touchRef.current.swiping && Math.abs(dx) < 10 && Math.abs(dy) < 10) return
    if (!touchRef.current.swiping) {
      if (Math.abs(dy) > Math.abs(dx)) { touchRef.current = null; return } // vertical scroll
      touchRef.current.swiping = true
    }

    // Only allow left swipe (negative dx), clamp to -DELETE_THRESHOLD*1.3
    const clamped = Math.max(dx, -DELETE_THRESHOLD * 1.3)
    setSwipeX(Math.min(0, clamped))
  }, [])

  const onTouchEnd = useCallback(() => {
    if (longPressTimer.current) { clearTimeout(longPressTimer.current); longPressTimer.current = null }
    if (longPressFired.current) { longPressFired.current = false; return }
    if (!touchRef.current?.swiping) { touchRef.current = null; return }
    touchRef.current = null
    if (swipeX <= -DELETE_THRESHOLD) {
      onDelete(item.id)
    }
    setSwipeX(0)
  }, [swipeX, onDelete, item.id])

  const isRevealed = swipeX < -10
  const deleteOpacity = Math.min(1, Math.abs(swipeX) / DELETE_THRESHOLD)

  return (
    <div style={{ position: 'relative', overflow: 'hidden' }}>
      {/* Delete action behind the row */}
      {isRevealed && (
        <div
          style={{
            position: 'absolute', right: 0, top: 0, bottom: 0,
            width: `${Math.abs(swipeX)}px`,
            backgroundColor: 'var(--error)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            color: '#fff', fontFamily: FONT, fontSize: '13px', fontWeight: 600,
            opacity: deleteOpacity,
          }}
          aria-hidden="true"
        >
          {Math.abs(swipeX) > 40 && (
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
            </svg>
          )}
        </div>
      )}
      <div
        onTouchStart={onTouchStart}
        onTouchMove={onTouchMove}
        onTouchEnd={onTouchEnd}
        style={{
          ...styles.row,
          position: 'relative' as const,
          transform: swipeX !== 0 ? `translateX(${swipeX}px)` : undefined,
          transition: !touchRef.current?.swiping && !REDUCED_MOTION ? 'transform 0.2s ease' : undefined,
          ...(item.checked ? { backgroundColor: 'var(--bg)', opacity: 0.7 } : {}),
          ...(isNew ? {
            backgroundColor: 'color-mix(in srgb, var(--brand) 8%, transparent)',
            transition: 'background-color 1.5s ease',
          } : {}),
          ...(!item.checked && item.urgent ? { backgroundColor: 'var(--error-bg)' } : {}),
        }}>
        <button onClick={() => onToggle(item.id)} style={styles.checkBtn} aria-label={`${item.checked ? 'Uncheck' : 'Check off'} ${item.brand && !item.name.toLowerCase().includes(item.brand.toLowerCase()) ? item.brand + ' ' : ''}${item.name}`}>
          <span style={styles.check(item.checked)} aria-hidden="true">
            {item.checked && <svg width="12" height="12" viewBox="0 0 12 12" fill="none"><path d="M2.5 6L5 8.5L9.5 3.5" stroke="#fff" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/></svg>}
          </span>
        </button>
        <div style={{ marginRight: '8px' }}>
          {item.brand ? (
            <BrandLogo brand={item.brand} size={36} opacity={item.checked ? 0.4 : 1} />
          ) : (
            <div style={{
              width: 36, height: 36, borderRadius: '8px', flexShrink: 0,
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: '20px', opacity: item.checked ? 0.4 : 1,
              transition: 'opacity 0.15s',
            }}>
              {DEPT_ICONS[item.department] || '🛒'}
            </div>
          )}
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
            <span style={{ fontFamily: FONT, fontSize: '10px', color: 'var(--text-secondary)' }}>
              {ts}
            </span>
          )}
        </div>
        {(item.urgent || item.claimed_by_id || item.swap_savings_cents > 0) && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', alignItems: 'flex-end', flexShrink: 0, paddingRight: '12px' }}>
            {item.swap_savings_cents > 0 && (
              <span style={{
                backgroundColor: 'var(--success-bg)', color: 'var(--success)', border: '1px solid color-mix(in srgb, var(--success) 25%, transparent)',
                fontSize: '12px', fontWeight: 600, borderRadius: '10px', padding: '2px 8px',
                whiteSpace: 'nowrap',
              }}>Saved ${(item.swap_savings_cents / 100).toFixed(2)}</span>
            )}
            {item.urgent && (
              <span style={{
                backgroundColor: 'var(--error-bg)', color: 'var(--error)', border: '1px solid color-mix(in srgb, var(--error) 25%, transparent)',
                fontSize: '12px', fontWeight: 600, borderRadius: '10px', padding: '2px 8px',
                whiteSpace: 'nowrap',
              }}>🔥 urgent</span>
            )}
            {item.claimed_by_id && (
              <span style={{
                fontSize: '12px', fontWeight: 600, borderRadius: '10px', padding: '2px 8px',
                whiteSpace: 'nowrap',
                ...(item.claimed_by_id === currentUserId
                  ? { backgroundColor: 'var(--success-bg)', color: 'var(--success)', border: '1px solid color-mix(in srgb, var(--success) 25%, transparent)' }
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
