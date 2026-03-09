import { ActionSheet } from './ActionSheet'
import { BrandLogo } from './BrandLogo'
import { type ListItem } from './ItemRow'
import { FONT, FONT_2XS, FONT_BASE, FONT_MD } from '../styles'

type SkippedItem = {
  parsed: { name: string; brand: string; quantity: number; size: string; department: string }
  existing_id: number
}

interface ReviewSheetProps {
  lastAdded: ListItem[]
  lastSkipped: SkippedItem[]
  items: ListItem[]
  onClose: () => void
  onUpdateQuantity: (item: ListItem, delta: number) => Promise<void>
  onDeleteItem: (item: ListItem) => Promise<void>
  onUndoAll: () => Promise<void>
  onAddSkipped: (skipped: SkippedItem, existing: ListItem | undefined) => Promise<void>
  onOpenDetail: (item: ListItem) => void
  listStyles: {
    sheetTitle: React.CSSProperties
    sheetButton: React.CSSProperties
    sheetDeleteBtn: React.CSSProperties
  }
}

export function ReviewSheet({
  lastAdded, lastSkipped, items, onClose,
  onUpdateQuantity, onDeleteItem, onUndoAll, onAddSkipped, onOpenDetail,
  listStyles,
}: ReviewSheetProps) {
  return (
    <ActionSheet isOpen={true} onClose={onClose} title={lastAdded.length > 0 ? 'Added' : undefined}>

      {/* Added items section */}
      {lastAdded.length > 0 && (
        <>
          {lastAdded.map(item => (
            <div key={item.id} style={{
              display: 'flex',
              alignItems: 'center',
              padding: '10px 0',
              borderBottom: '1px solid var(--border)',
              gap: '10px',
            }}>
              <BrandLogo brand={item.brand} size={52} />
              <div role="button" tabIndex={0} style={{ flex: 1, cursor: 'pointer' }} onClick={() => onOpenDetail(item)} onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onOpenDetail(item) } }} aria-label={`Edit ${item.brand ? item.brand + ' ' : ''}${item.name}`}>
                <span style={{ fontFamily: FONT, fontSize: FONT_MD, color: 'var(--text)' }}>
                  {item.brand ? `${item.brand} ` : ''}{item.name}
                </span>
                {item.size && (
                  <span style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', marginLeft: '6px' }}>
                    {item.size}
                  </span>
                )}
                <div style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', marginTop: '2px' }}>
                  {item.department} · <span style={{ color: 'var(--brand)', fontSize: FONT_2XS }}>edit</span>
                </div>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0', border: '1.5px solid var(--border)', borderRadius: '10px', overflow: 'hidden', backgroundColor: 'var(--bg)', flexShrink: 0 }}>
                <button
                  onClick={() => onUpdateQuantity(item, -1)}
                  aria-label={`Decrease quantity of ${item.name}, currently ${item.quantity}`}
                  style={{ width: '34px', height: '34px', border: 'none', background: 'none', fontSize: '17px', color: 'var(--brand)', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
                >−</button>
                <span aria-live="polite" aria-atomic="true" style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, color: 'var(--text)', minWidth: '24px', textAlign: 'center' }}>{item.quantity}</span>
                <button
                  onClick={() => onUpdateQuantity(item, 1)}
                  aria-label={`Increase quantity of ${item.name}, currently ${item.quantity}`}
                  style={{ width: '34px', height: '34px', border: 'none', background: 'none', fontSize: '17px', color: 'var(--brand)', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
                >+</button>
              </div>
              <button
                onClick={() => onDeleteItem(item)}
                aria-label={`Remove ${item.name} from list`}
                style={{
                  flexShrink: 0,
                  width: '28px',
                  height: '28px',
                  borderRadius: '50%',
                  border: '1px solid var(--border)',
                  background: 'none',
                  color: 'var(--text-secondary)',
                  fontSize: '15px',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                }}
              >×</button>
            </div>
          ))}
        </>
      )}

      {/* Skipped items section */}
      {lastSkipped.length > 0 && (
        <>
          <p style={{ ...listStyles.sheetTitle, marginTop: lastAdded.length > 0 ? '16px' : '0' } as React.CSSProperties}>Already on your list</p>
          {lastSkipped.map(skipped => {
            const existing = items.find(i => i.id === skipped.existing_id)
            return (
              <div key={skipped.existing_id} style={{
                display: 'flex',
                alignItems: 'center',
                padding: '10px 0',
                borderBottom: '1px solid var(--border)',
                gap: '10px',
              }}>
                <BrandLogo brand={skipped.parsed.brand} size={52} />
                <div role={existing ? 'button' : undefined} tabIndex={existing ? 0 : undefined} style={{ flex: 1, cursor: existing ? 'pointer' : undefined }} onClick={() => existing && onOpenDetail(existing)} onKeyDown={e => { if (existing && (e.key === 'Enter' || e.key === ' ')) { e.preventDefault(); onOpenDetail(existing) } }} aria-label={existing ? `Edit ${existing.brand ? existing.brand + ' ' : ''}${existing.name}` : undefined}>
                  <span style={{ fontFamily: FONT, fontSize: FONT_MD, color: 'var(--text)' }}>
                    {skipped.parsed.brand ? `${skipped.parsed.brand} ` : ''}{skipped.parsed.name}
                  </span>
                  <div style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', marginTop: '2px' }}>
                    On list: {existing?.quantity ?? 1}{existing && <> · <span style={{ color: 'var(--brand)', fontSize: FONT_2XS }}>edit</span></>}
                  </div>
                </div>
                <button
                  onClick={() => onAddSkipped(skipped, existing)}
                  aria-label={`Add ${skipped.parsed.quantity || 1} more ${skipped.parsed.name}`}
                  style={{
                    flexShrink: 0,
                    padding: '6px 14px',
                    borderRadius: '16px',
                    border: '1.5px solid var(--brand)',
                    background: 'none',
                    color: 'var(--brand)',
                    fontFamily: FONT,
                    fontSize: FONT_BASE,
                    fontWeight: 600,
                    cursor: 'pointer',
                  }}
                >+{skipped.parsed.quantity || 1}</button>
              </div>
            )
          })}
        </>
      )}

      {lastAdded.length > 0 && (
        <button
          onClick={onUndoAll}
          style={listStyles.sheetDeleteBtn}
        >
          Undo all
        </button>
      )}

      <button
        onClick={onClose}
        style={listStyles.sheetButton}
      >
        Done
      </button>
    </ActionSheet>
  )
}
