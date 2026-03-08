import { FONT } from '../styles'
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
  fetchAPI: (url: string, options?: RequestInit) => Promise<Record<string, unknown> | unknown[] | null>
  typePennyHome: (msg: string) => void
  showToast: (message: string, type?: 'success' | 'error') => void
  handleCancelTrip: () => Promise<void>
  handleClearList: () => Promise<void>
  onClose: () => void
  onFinish: (isFirstTrip: boolean) => void
  onSnapReceipt: () => void
}

export function DoneSummaryModal({
  data, isFirstTrip, items, fetchAPI, typePennyHome, showToast,
  handleCancelTrip, handleClearList, onClose, onFinish, onSnapReceipt,
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
        <div style={{ fontFamily: FONT, fontSize: '22px', fontWeight: 700, color: 'var(--text)', marginBottom: '4px' }}>
          {isFirstTrip ? 'First trip complete!' : 'Nice work!'}
        </div>
        <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', marginBottom: '4px' }}>
          {data.checkedCount} of {data.totalCount} item{data.totalCount !== 1 ? 's' : ''} checked off
        </div>

        {tripDuration ? (
          <div style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text-secondary)', marginBottom: '16px' }}>
            {tripDuration}
          </div>
        ) : (
          <div style={{ marginBottom: '16px' }} />
        )}

        {/* Save as list option */}
        {data.checkedCount > 0 && (
          <button
            onClick={async () => {
              const listName = `Trip ${new Date().toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}`
              const res = await fetchAPI('/api/saved-lists', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name: listName, items: items.filter(i => i.checked).map(i => ({ name: i.name, brand: i.brand, department: i.department, quantity: i.quantity, size: i.size, notes: i.notes })) }),
              })
              if (res) typePennyHome(`Saved as "${listName}" — you can load it anytime!`)
              else showToast('Could not save list', 'error')
            }}
            style={{
              width: '100%', padding: '12px', borderRadius: '10px',
              border: '1px solid var(--border)', backgroundColor: 'var(--bg)',
              fontFamily: FONT, fontSize: '14px', color: 'var(--text)', cursor: 'pointer',
              marginBottom: '8px',
            }}
          >
            Save this list for next time
          </button>
        )}

        {/* Clear checked items */}
        <button
          onClick={async () => {
            await handleCancelTrip()
            if (data.checkedCount > 0) {
              await handleClearList()
            }
            onFinish(isFirstTrip)
          }}
          style={{
            width: '100%', padding: '14px', borderRadius: '10px',
            backgroundColor: 'var(--brand)', color: '#fff', border: 'none',
            fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer',
            marginBottom: '8px',
          }}
        >
          {data.checkedCount > 0 ? 'Clear checked & finish' : 'Finish trip'}
        </button>

        <button
          onClick={onClose}
          style={{
            width: '100%', padding: '10px', background: 'none', border: 'none',
            fontFamily: FONT, fontSize: '14px', color: 'var(--text-secondary)', cursor: 'pointer',
          }}
        >
          Keep shopping
        </button>

        {/* Receipt scan button */}
        <button
          onClick={onSnapReceipt}
          style={{
            width: '100%', marginTop: '16px', padding: '12px', borderRadius: '10px',
            border: '1px solid var(--brand)', backgroundColor: 'color-mix(in srgb, var(--brand) 3%, transparent)',
            fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: 'var(--brand)',
            cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px',
          }}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
            <circle cx="12" cy="13" r="4"/>
          </svg>
          Snap your receipt
        </button>
      </div>
    </div>
  )
}
