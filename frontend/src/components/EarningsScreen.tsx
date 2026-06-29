import { useState, useEffect, useRef } from 'react'
import { PennyAvatar } from './PennyAvatar'
import { BrandLogo } from './BrandLogo'
import { StatusBadge } from './StatusBadge'
import { fetchAPI } from '../api'
import { FONT, FONT_XS, FONT_BASE, FONT_MD, layout, text } from '../styles'

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
  onViewReceipts?: () => void
  onGoToList?: () => void
  onEarningsLoaded?: (totalCents: number) => void
}

type DaysFilter = 0 | 7 | 30 | 90

export function EarningsScreen({ onViewReceipts, onGoToList, onEarningsLoaded }: EarningsScreenProps) {
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
      const totalCents = (result.total_cents as number) || 0
      setData({
        transactions: (result.transactions as EarningTransaction[]) || [],
        total_cents: totalCents,
        pending_cents: (result.pending_cents as number) || 0,
        paid_cents: (result.paid_cents as number) || 0,
        display_amount: (result.display_amount as string) || '$0.00',
      })
      // Sync header badge with earnings total (unfiltered only)
      if (days === 0 && onEarningsLoaded) {
        onEarningsLoaded(totalCents)
      }
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

  const STATUS_MAP: Record<string, { label: string; variant: 'warning' | 'info' | 'success' }> = {
    pending: { label: 'Pending', variant: 'warning' },
    issued: { label: 'Issued', variant: 'info' },
    paid: { label: 'Paid', variant: 'success' },
  }

  if (loading && !data) {
    return (
      <div style={{ padding: '40px 20px', textAlign: 'center' }} aria-live="polite" aria-busy="true">
        <div style={text.secondary}>Loading earnings...</div>
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
        aria-label="Upload receipt image or PDF"
      />

      {/* Summary cards */}
      <div style={{ ...layout.rowGap, margin: '16px 0' }}>
        {[
          { label: 'Total', cents: data?.total_cents || 0, color: 'var(--text)' },
          { label: 'Pending', cents: data?.pending_cents || 0, color: 'var(--warning)' },
          { label: 'Paid', cents: data?.paid_cents || 0, color: 'var(--success)' },
        ].map(card => (
          <div key={card.label} style={{
            flex: 1, padding: '14px 8px', borderRadius: '10px', backgroundColor: 'var(--surface)',
            textAlign: 'center' as const, border: '1px solid var(--border)',
          }}>
            <div style={{ fontFamily: FONT, fontSize: '1.25rem', fontWeight: 700, color: card.color }}>
              ${(card.cents / 100).toFixed(2)}
            </div>
            <div style={{ fontFamily: FONT, fontSize: '0.6875rem', color: 'var(--text-secondary)', marginTop: '3px' }}>
              {card.label}
            </div>
          </div>
        ))}
      </div>

      {/* Time filter pills */}
      <div style={{ ...layout.rowGap, marginBottom: '16px' }}>
        {([0, 7, 30, 90] as DaysFilter[]).map(d => (
          <button
            key={d}
            onClick={() => handleFilterChange(d)}
            style={{
              flex: 1, padding: '8px 0', borderRadius: '20px', border: '1px solid var(--border)',
              backgroundColor: daysFilter === d ? 'var(--brand)' : 'var(--surface)',
              color: daysFilter === d ? '#fff' : 'var(--text)',
              fontFamily: FONT, fontSize: FONT_XS, fontWeight: 500, cursor: 'pointer',
            }}
          >
            {d === 0 ? 'All' : `${d}d`}
          </button>
        ))}
      </div>

      {/* Empty state */}
      {isEmpty ? (
        <div style={{
          ...layout.columnCenter,
          padding: '40px 20px', gap: '16px',
        }}>
          <PennyAvatar size={48} />
          <div style={{ ...text.heading, fontSize: FONT_MD, textAlign: 'center' }}>
            Try a deal and get paid. No one else does this.
          </div>
          <div style={{ ...text.secondary, textAlign: 'center', lineHeight: '1.5', maxWidth: '280px' }}>
            Brands pay me to reach you. In addition to a rebate, 30% of what they pay me comes back to you — real cash.
          </div>
          {onGoToList && (
            <button
              onClick={onGoToList}
              style={{
                backgroundColor: 'var(--brand)', color: '#fff', border: 'none', borderRadius: '10px',
                padding: '12px 24px', fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, cursor: 'pointer',
                marginTop: '4px',
              }}
            >
              Go to my list
            </button>
          )}
          <div style={{ ...text.caption, textAlign: 'center' }}>
            Deal suggestions appear when you add items to your list
          </div>
        </div>
      ) : (
        /* Transaction cards */
        <div style={layout.columnGap}>
          {data!.transactions.map(tx => (
            <div key={tx.id} style={{
              backgroundColor: 'var(--surface)', borderRadius: '10px', padding: '14px 16px',
              border: '1px solid var(--border)',
            }}>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: '12px' }}>
                <BrandLogo brand={tx.offer_brand} size={44} />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={text.label}>
                    {tx.offer_brand} {tx.offer_product}
                  </div>
                  {tx.competitor_product && (
                    <div style={{ ...text.caption, marginTop: '2px' }}>
                      Switched from {tx.competitor_product}
                    </div>
                  )}
                  <div style={{ fontFamily: FONT, fontSize: '0.6875rem', color: 'var(--text-secondary)', marginTop: '4px' }}>
                    {formatDate(tx.created_at)}
                  </div>
                </div>
                <div style={{ textAlign: 'right', flexShrink: 0 }}>
                  <div style={{ fontFamily: FONT, fontSize: FONT_MD, fontWeight: 700, color: 'var(--brand)' }}>
                    ${(tx.amount_cents / 100).toFixed(2)}
                  </div>
                  <div style={{ marginTop: '4px' }}><StatusBadge label={(STATUS_MAP[tx.status] || STATUS_MAP.pending).label} variant={(STATUS_MAP[tx.status] || STATUS_MAP.pending).variant} /></div>
                </div>
              </div>

              {/* Upload / attachment indicator */}
              <div style={{ ...layout.rowGap, marginTop: '10px' }}>
                {tx.attachment_path ? (
                  <span style={{
                    ...layout.row, ...text.caption, color: 'var(--success)', fontWeight: 500,
                    gap: '4px',
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
                      ...layout.row, ...text.caption, color: 'var(--brand)', fontWeight: 500,
                      background: 'none', border: '1px solid var(--brand)', borderRadius: '6px',
                      padding: '4px 10px', cursor: 'pointer', gap: '4px',
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
          ...text.caption, textAlign: 'center',
          padding: '16px 0 8px', lineHeight: '1.5',
        }}>
          Brands pay me to reach you. In addition to a rebate, 30% of what they pay me comes back to you — real cash. No other grocery app does this. Payouts sent monthly via Venmo.
        </div>
      )}

      {/* View receipts link */}
      {onViewReceipts && (
        <button
          onClick={onViewReceipts}
          style={{
            ...layout.rowCenter, ...text.secondary, fontWeight: 500,
            width: '100%', marginTop: '8px', padding: '12px', borderRadius: '10px',
            border: '1px solid var(--border)', backgroundColor: 'var(--surface)',
            cursor: 'pointer', gap: '8px',
          }}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
            <polyline points="14 2 14 8 20 8"/>
            <line x1="16" y1="13" x2="8" y2="13"/>
            <line x1="16" y1="17" x2="8" y2="17"/>
          </svg>
          View receipts
        </button>
      )}
    </div>
  )
}
