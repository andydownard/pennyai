import { useState, useEffect, useRef } from 'react'
import { PennyAvatar } from './PennyAvatar'
import { BRAND, TEXT, TEXT_SECONDARY, BORDER, WHITE, FONT } from '../styles'

interface EarningTransaction {
  id: number
  offer_brand: string
  offer_product: string
  competitor_product: string
  original_item_name: string
  amount_cents: number
  status: string
  created_at: string
  attachment_path: string
}

interface EarningsData {
  transactions: EarningTransaction[]
  total_cents: number
  pending_cents: number
  paid_cents: number
  display_amount: string
}

interface EarningsScreenProps {
  onBack: () => void
  fetchAPI: (url: string, options?: RequestInit) => Promise<Record<string, unknown> | unknown[] | null>
}

type DaysFilter = 0 | 7 | 30 | 90

export function EarningsScreen({ fetchAPI }: EarningsScreenProps) {
  const [data, setData] = useState<EarningsData | null>(null)
  const [loading, setLoading] = useState(true)
  const [daysFilter, setDaysFilter] = useState<DaysFilter>(0)
  const [uploadingId, setUploadingId] = useState<number | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const uploadTargetId = useRef<number | null>(null)

  const loadEarnings = async (days: DaysFilter) => {
    setLoading(true)
    const url = days > 0 ? `/api/earnings?days=${days}` : '/api/earnings'
    const result = await fetchAPI(url)
    if (result && !Array.isArray(result)) {
      setData({
        transactions: (result.transactions as EarningTransaction[]) || [],
        total_cents: (result.total_cents as number) || 0,
        pending_cents: (result.pending_cents as number) || 0,
        paid_cents: (result.paid_cents as number) || 0,
        display_amount: (result.display_amount as string) || '$0.00',
      })
    }
    setLoading(false)
  }

  useEffect(() => {
    loadEarnings(daysFilter)
  }, [daysFilter]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleFilterChange = (days: DaysFilter) => {
    setDaysFilter(days)
  }

  const handleUploadClick = (id: number) => {
    uploadTargetId.current = id
    fileInputRef.current?.click()
  }

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    const targetId = uploadTargetId.current
    if (!file || !targetId) return

    setUploadingId(targetId)
    const formData = new FormData()
    formData.append('file', file)

    try {
      const res = await fetch(`/api/earnings/${targetId}/upload`, {
        method: 'POST',
        body: formData,
      })
      if (res.ok) {
        // Refresh data to show updated attachment
        await loadEarnings(daysFilter)
      }
    } catch {
      // silently fail
    } finally {
      setUploadingId(null)
      uploadTargetId.current = null
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
  }

  const formatDate = (dateStr: string) => {
    const d = new Date(dateStr + 'Z')
    return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
  }

  const statusBadge = (status: string) => {
    const config: Record<string, { label: string; bg: string; color: string }> = {
      pending: { label: 'Pending', bg: '#FEF3C7', color: '#D97706' },
      issued: { label: 'Issued', bg: '#DBEAFE', color: '#2563EB' },
      paid: { label: 'Paid', bg: '#D1FAE5', color: '#059669' },
    }
    const c = config[status] || config.pending
    return (
      <span style={{
        fontFamily: FONT, fontSize: '11px', fontWeight: 600,
        padding: '2px 8px', borderRadius: '10px',
        backgroundColor: c.bg, color: c.color,
      }}>
        {c.label}
      </span>
    )
  }

  if (loading && !data) {
    return (
      <div style={{ padding: '40px 20px', textAlign: 'center' }}>
        <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY }}>Loading earnings...</div>
      </div>
    )
  }

  const isEmpty = !data || data.transactions.length === 0

  return (
    <div style={{ padding: '0 16px 20px' }}>
      {/* Hidden file input for uploads */}
      <input
        ref={fileInputRef}
        type="file"
        accept="image/*,.pdf"
        onChange={handleFileChange}
        style={{ display: 'none' }}
      />

      {/* Summary cards */}
      <div style={{ display: 'flex', gap: '8px', margin: '16px 0' }}>
        {[
          { label: 'Total', cents: data?.total_cents || 0, color: TEXT },
          { label: 'Pending', cents: data?.pending_cents || 0, color: '#D97706' },
          { label: 'Paid', cents: data?.paid_cents || 0, color: '#059669' },
        ].map(card => (
          <div key={card.label} style={{
            flex: 1, padding: '14px 8px', borderRadius: '10px', backgroundColor: WHITE,
            textAlign: 'center' as const, border: `1px solid ${BORDER}`,
          }}>
            <div style={{ fontFamily: FONT, fontSize: '20px', fontWeight: 700, color: card.color }}>
              ${(card.cents / 100).toFixed(2)}
            </div>
            <div style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY, marginTop: '3px' }}>
              {card.label}
            </div>
          </div>
        ))}
      </div>

      {/* Time filter pills */}
      <div style={{ display: 'flex', gap: '8px', marginBottom: '16px' }}>
        {([0, 7, 30, 90] as DaysFilter[]).map(d => (
          <button
            key={d}
            onClick={() => handleFilterChange(d)}
            style={{
              flex: 1, padding: '8px 0', borderRadius: '20px', border: `1px solid ${BORDER}`,
              backgroundColor: daysFilter === d ? BRAND : WHITE,
              color: daysFilter === d ? WHITE : TEXT,
              fontFamily: FONT, fontSize: '13px', fontWeight: 500, cursor: 'pointer',
            }}
          >
            {d === 0 ? 'All' : `${d}d`}
          </button>
        ))}
      </div>

      {/* Empty state */}
      {isEmpty ? (
        <div style={{
          display: 'flex', flexDirection: 'column', alignItems: 'center',
          padding: '40px 20px', gap: '16px',
        }}>
          <PennyAvatar size={48} />
          <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, textAlign: 'center' }}>
            We pay you. No one else does.
          </div>
          <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, textAlign: 'center', lineHeight: '1.5', maxWidth: '280px' }}>
            Brands pay to reach you. We share that revenue with you — real money, every time you try something new.
          </div>
          <div style={{ fontFamily: FONT, fontSize: '13px', color: BRAND, textAlign: 'center', fontWeight: 500 }}>
            Accept a swap to start earning.
          </div>
        </div>
      ) : (
        /* Transaction cards */
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {data!.transactions.map(tx => (
            <div key={tx.id} style={{
              backgroundColor: WHITE, borderRadius: '10px', padding: '14px 16px',
              border: `1px solid ${BORDER}`,
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <div style={{ flex: 1 }}>
                  <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT }}>
                    {tx.offer_brand} {tx.offer_product}
                  </div>
                  {tx.competitor_product && (
                    <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '2px' }}>
                      Swapped from {tx.competitor_product}
                    </div>
                  )}
                  <div style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY, marginTop: '4px' }}>
                    {formatDate(tx.created_at)}
                  </div>
                </div>
                <div style={{ textAlign: 'right', flexShrink: 0, marginLeft: '12px' }}>
                  <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 700, color: BRAND }}>
                    ${(tx.amount_cents / 100).toFixed(2)}
                  </div>
                  <div style={{ marginTop: '4px' }}>{statusBadge(tx.status)}</div>
                </div>
              </div>

              {/* Upload / attachment indicator */}
              <div style={{ marginTop: '10px', display: 'flex', alignItems: 'center', gap: '8px' }}>
                {tx.attachment_path ? (
                  <span style={{
                    fontFamily: FONT, fontSize: '12px', color: '#059669', fontWeight: 500,
                    display: 'flex', alignItems: 'center', gap: '4px',
                  }}>
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <polyline points="20 6 9 17 4 12"/>
                    </svg>
                    Receipt uploaded
                  </span>
                ) : (
                  <button
                    onClick={() => handleUploadClick(tx.id)}
                    disabled={uploadingId === tx.id}
                    style={{
                      fontFamily: FONT, fontSize: '12px', color: BRAND, fontWeight: 500,
                      background: 'none', border: `1px solid ${BRAND}`, borderRadius: '6px',
                      padding: '4px 10px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '4px',
                      opacity: uploadingId === tx.id ? 0.5 : 1,
                    }}
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                      <polyline points="17 8 12 3 7 8"/>
                      <line x1="12" y1="3" x2="12" y2="15"/>
                    </svg>
                    {uploadingId === tx.id ? 'Uploading...' : 'Upload receipt'}
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Info note */}
      {!isEmpty && (
        <div style={{
          fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, textAlign: 'center',
          padding: '16px 0 8px', lineHeight: '1.5',
        }}>
          Brands pay to reach you — we share the revenue. No other grocery app does this. Payouts sent monthly via Venmo.
        </div>
      )}
    </div>
  )
}
