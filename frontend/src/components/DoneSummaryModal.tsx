import { fetchAPI } from '../api'
import { FONT, FONT_XS, FONT_BASE, FONT_XL, layout, buttons, text } from '../styles'
import type { ListItem } from './ItemRow'

interface DoneSummaryData {
  checkedCount: number
  totalCount: number
  tripStartedAt: string | null
}

interface DoneSummaryModalProps {
  data: DoneSummaryData
  isFirstTrip: boolean
  items: ListItem[]
  typePennyHome: (msg: string) => void
  showToast: (message: string, type?: 'success' | 'error') => void
  handleCancelTrip: () => Promise<void>
  handleClearList: () => Promise<void>
  onClose: () => void
  onFinish: (isFirstTrip: boolean) => void
  onSnapReceipt: () => void
  hasSwapsToClaim?: boolean
}

export function DoneSummaryModal({
  data, isFirstTrip, items, typePennyHome,
  handleCancelTrip, handleClearList, onClose, onFinish, onSnapReceipt,
  hasSwapsToClaim,
}: DoneSummaryModalProps) {
  const tripDuration = (() => {
    if (!data.tripStartedAt) return null
    const startTime = new Date(data.tripStartedAt)
    const mins = Math.round((Date.now() - startTime.getTime()) / 60000)
    if (mins < 1 || mins > 480) return null
    const hrs = Math.floor(mins / 60)
    const remMins = mins % 60
    const durationText = hrs > 0 ? `${hrs}h ${remMins}m` : `${mins} min${mins !== 1 ? 's' : ''}`
    return data.checkedCount >= data.totalCount
      ? `Everything in ${durationText}!`
      : `${durationText} in the store`
  })()

  return (
    <div role="presentation" style={{
      position: 'fixed', inset: 0, zIndex: 9999,
      display: 'flex', flexDirection: 'column' as const, alignItems: 'center', justifyContent: 'center',
      backgroundColor: 'rgba(0,0,0,0.6)',
      padding: '24px',
    }}>
      <div role="dialog" aria-modal="true" aria-label="Trip summary" style={{
        backgroundColor: 'var(--surface)', borderRadius: '20px', padding: '32px 24px',
        maxWidth: '360px', width: '100%', textAlign: 'center' as const,
      }}>
        <div style={{ fontSize: '48px', marginBottom: '12px' }}>
          {isFirstTrip ? '🎉' : '✓'}
        </div>
        <div style={{ fontFamily: FONT, fontSize: FONT_XL, fontWeight: 700, color: 'var(--text)', marginBottom: '4px' }}>
          {isFirstTrip ? 'First trip complete!' : 'Nice work!'}
        </div>
        <div style={{ ...text.body, color: 'var(--text-secondary)', marginBottom: '4px' }}>
          {data.checkedCount} of {data.totalCount} item{data.totalCount !== 1 ? 's' : ''} checked off
        </div>

        {tripDuration ? (
          <div style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', marginBottom: '16px' }}>
            {tripDuration}
          </div>
        ) : (
          <div style={{ marginBottom: '16px' }} />
        )}

        {/* Clear checked items — auto-merges checked items into master list */}
        <button
          onClick={async () => {
            // Auto-merge checked items into "My Groceries" master list
            if (data.checkedCount > 0) {
              const checkedItems = items.filter(i => i.checked).map(i => ({
                name: i.name, brand: i.brand, department: i.department,
                quantity: i.quantity, size: i.size, notes: i.notes,
              }))
              // Find or create master list
              const lists = await fetchAPI('/api/saved-lists') as Array<{ id: number; name: string }> | null
              const master = lists?.find(l => l.name === 'My Groceries')
              if (master) {
                // Merge into existing master list (skips duplicates)
                await fetchAPI(`/api/saved-lists/${master.id}/merge`, {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify({ items: checkedItems }),
                })
              } else {
                // Create master list with these items
                await fetchAPI('/api/saved-lists', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify({ name: 'My Groceries', items: checkedItems }),
                })
              }
              typePennyHome('Updated your master grocery list — ask me to load it anytime!')
            }
            await handleCancelTrip()
            if (data.checkedCount > 0) {
              await handleClearList()
            }
            onFinish(isFirstTrip)
          }}
          style={{
            ...buttons.primary,
            padding: '14px', borderRadius: '10px', fontSize: FONT_BASE,
            marginBottom: '8px',
          }}
        >
          {data.checkedCount > 0 ? 'Clear checked & finish' : 'Finish trip'}
        </button>

        <button
          onClick={onClose}
          style={{
            ...buttons.reset, ...text.secondary,
            width: '100%', padding: '10px',
          }}
        >
          Keep shopping
        </button>

        {/* Receipt scan button — only show when user has accepted swaps to verify */}
        {hasSwapsToClaim && <button
          onClick={onSnapReceipt}
          style={{
            ...layout.rowCenter, ...text.body,
            width: '100%', marginTop: '16px', padding: '12px', borderRadius: '10px', gap: '8px',
            border: '1px solid var(--brand)', backgroundColor: 'color-mix(in srgb, var(--brand) 3%, transparent)',
            fontWeight: 600, color: 'var(--brand)', cursor: 'pointer',
          }}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
            <circle cx="12" cy="13" r="4"/>
          </svg>
          Snap your receipt
        </button>}
      </div>
    </div>
  )
}
