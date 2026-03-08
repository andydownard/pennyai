import { useState, useEffect, useRef } from 'react'
import { PennyAvatar } from './PennyAvatar'
import { FONT } from '../styles'
import { type ReceiptSummary, type ReceiptLineItem } from '../types'

interface ReceiptScreenProps {
  fetchAPI: (url: string, options?: RequestInit) => Promise<Record<string, unknown> | unknown[] | null>
  onSnapReceipt: () => void
}

export function ReceiptScreen({ fetchAPI, onSnapReceipt }: ReceiptScreenProps) {
  const [receipts, setReceipts] = useState<ReceiptSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [selectedReceipt, setSelectedReceipt] = useState<ReceiptSummary | null>(null)
  const [lineItems, setLineItems] = useState<ReceiptLineItem[]>([])
  const [lineItemsLoading, setLineItemsLoading] = useState(false)
  const [confirmingItems, setConfirmingItems] = useState<Record<number, boolean>>({})
  const [submitting, setSubmitting] = useState(false)
  const loadedRef = useRef(false)

  useEffect(() => {
    if (loadedRef.current) return
    loadedRef.current = true
    loadReceipts()
  }, [])

  const loadReceipts = async () => {
    setLoading(true)
    const result = await fetchAPI('/api/receipts')
    if (Array.isArray(result)) {
      setReceipts(result as ReceiptSummary[])
    }
    setLoading(false)
  }

  const openReceipt = async (receipt: ReceiptSummary) => {
    setSelectedReceipt(receipt)
    setLineItemsLoading(true)
    const result = await fetchAPI(`/api/receipts/${receipt.id}/items`)
    if (Array.isArray(result)) {
      setLineItems(result as ReceiptLineItem[])
      // Pre-set confirmations for low-confidence items
      const initial: Record<number, boolean> = {}
      for (const item of result as ReceiptLineItem[]) {
        if (item.confidence === 'low' && item.matched_name) {
          initial[item.id] = true // default to confirmed
        }
      }
      setConfirmingItems(initial)
    }
    setLineItemsLoading(false)
  }

  const handleConfirmSubmit = async () => {
    if (!selectedReceipt) return
    setSubmitting(true)

    const confirmations = Object.entries(confirmingItems).map(([id, confirmed]) => {
      const item = lineItems.find(li => li.id === Number(id))
      return {
        line_item_id: Number(id),
        confirmed,
        matched_name: item?.matched_name || '',
      }
    })

    await fetchAPI(`/api/receipts/${selectedReceipt.id}/confirm`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ confirmations }),
    })

    // Reload receipt detail
    await openReceipt(selectedReceipt)
    setSubmitting(false)
  }

  const formatDate = (dateStr: string) => {
    if (!dateStr) return ''
    const d = new Date(dateStr.includes('T') ? dateStr : dateStr + 'T00:00:00')
    return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
  }

  const formatPrice = (cents: number) => {
    return `$${(cents / 100).toFixed(2)}`
  }

  const lowConfidenceItems = lineItems.filter(li => li.confidence === 'low' && li.matched_name)
  const hasUnconfirmed = lowConfidenceItems.length > 0

  // Receipt detail view
  if (selectedReceipt) {
    return (
      <div style={{ padding: '0 16px 20px' }}>
        {/* Back button */}
        <button
          onClick={() => { setSelectedReceipt(null); setLineItems([]) }}
          style={{
            background: 'none', border: 'none', padding: '12px 0', cursor: 'pointer',
            fontFamily: FONT, fontSize: '14px', color: 'var(--brand)', fontWeight: 500,
            display: 'flex', alignItems: 'center', gap: '4px',
          }}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="15 18 9 12 15 6"/>
          </svg>
          Receipts
        </button>

        {/* Receipt header */}
        <div style={{
          backgroundColor: 'var(--surface)', borderRadius: '10px', padding: '16px',
          border: '1px solid var(--border)', marginBottom: '12px',
        }}>
          <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: 'var(--text)' }}>
            {selectedReceipt.store_name || 'Receipt'}
          </div>
          <div style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text-secondary)', marginTop: '4px' }}>
            {formatDate(selectedReceipt.transaction_date || selectedReceipt.created_at)}
          </div>
          <div style={{ display: 'flex', gap: '16px', marginTop: '10px' }}>
            <div style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text-secondary)' }}>
              {selectedReceipt.items_found} items found
            </div>
            <div style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text-secondary)' }}>
              {selectedReceipt.items_matched} matched
            </div>
            {selectedReceipt.total_cents > 0 && (
              <div style={{ fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                {formatPrice(selectedReceipt.total_cents)}
              </div>
            )}
          </div>
          {selectedReceipt.swaps_verified > 0 && (
            <div style={{
              marginTop: '10px', padding: '6px 10px', borderRadius: '8px',
              backgroundColor: '#D1FAE5', display: 'inline-flex', alignItems: 'center', gap: '6px',
            }}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#059669" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="20 6 9 17 4 12"/>
              </svg>
              <span style={{ fontFamily: FONT, fontSize: '12px', fontWeight: 600, color: '#059669' }}>
                {selectedReceipt.swaps_verified} swap{selectedReceipt.swaps_verified !== 1 ? 's' : ''} verified
              </span>
            </div>
          )}
        </div>

        {/* Confirmation prompt for low-confidence items */}
        {hasUnconfirmed && (
          <div style={{
            backgroundColor: '#FFFBEB', borderRadius: '10px', padding: '14px 16px',
            border: '1px solid #FDE68A', marginBottom: '12px',
          }}>
            <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: '#92400E', marginBottom: '8px' }}>
              Help Penny learn
            </div>
            <div style={{ fontFamily: FONT, fontSize: '13px', color: '#78350F', lineHeight: '1.5', marginBottom: '12px' }}>
              I found {lowConfidenceItems.length} item{lowConfidenceItems.length !== 1 ? 's' : ''} I'm not sure about. Can you confirm?
            </div>
            {lowConfidenceItems.map(item => (
              <div key={item.id} style={{
                display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                padding: '8px 0', borderTop: '1px solid #FDE68A',
              }}>
                <div style={{ flex: 1 }}>
                  <div style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text)', fontWeight: 500 }}>
                    "{item.raw_text}"
                  </div>
                  <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginTop: '2px' }}>
                    = {item.matched_name}?
                  </div>
                </div>
                <div style={{ display: 'flex', gap: '6px', flexShrink: 0, marginLeft: '12px' }}>
                  <button
                    onClick={() => setConfirmingItems(prev => ({ ...prev, [item.id]: true }))}
                    style={{
                      width: '32px', height: '32px', borderRadius: '16px', border: 'none',
                      backgroundColor: confirmingItems[item.id] ? '#059669' : '#E5E7EB',
                      color: confirmingItems[item.id] ? '#fff' : 'var(--text-secondary)',
                      cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center',
                    }}
                    aria-label={`Confirm ${item.matched_name}`}
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                      <polyline points="20 6 9 17 4 12"/>
                    </svg>
                  </button>
                  <button
                    onClick={() => setConfirmingItems(prev => ({ ...prev, [item.id]: false }))}
                    style={{
                      width: '32px', height: '32px', borderRadius: '16px', border: 'none',
                      backgroundColor: confirmingItems[item.id] === false ? '#DC2626' : '#E5E7EB',
                      color: confirmingItems[item.id] === false ? '#fff' : 'var(--text-secondary)',
                      cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center',
                    }}
                    aria-label={`Reject ${item.matched_name}`}
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                      <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                    </svg>
                  </button>
                </div>
              </div>
            ))}
            <button
              onClick={handleConfirmSubmit}
              disabled={submitting}
              style={{
                width: '100%', marginTop: '12px', padding: '10px', borderRadius: '8px',
                border: 'none', backgroundColor: 'var(--brand)', color: '#fff',
                fontFamily: FONT, fontSize: '14px', fontWeight: 600,
                cursor: submitting ? 'default' : 'pointer',
                opacity: submitting ? 0.6 : 1,
              }}
            >
              {submitting ? 'Saving...' : 'Confirm matches'}
            </button>
          </div>
        )}

        {/* Line items list */}
        {lineItemsLoading ? (
          <div style={{ padding: '20px', textAlign: 'center' }}>
            <div style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text-secondary)' }}>
              Loading items...
            </div>
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
            {lineItems.map(item => (
              <div key={item.id} style={{
                backgroundColor: 'var(--surface)', padding: '10px 14px',
                borderBottom: '1px solid var(--border)',
                display: 'flex', alignItems: 'center', justifyContent: 'space-between',
              }}>
                <div style={{ flex: 1 }}>
                  <div style={{
                    fontFamily: FONT, fontSize: '14px', color: 'var(--text)', fontWeight: 500,
                    display: 'flex', alignItems: 'center', gap: '6px',
                  }}>
                    {item.raw_text}
                    {item.quantity > 1 && (
                      <span style={{ fontFamily: FONT, fontSize: '11px', color: 'var(--text-secondary)' }}>
                        x{item.quantity}
                      </span>
                    )}
                  </div>
                  {item.matched_name && (
                    <div style={{
                      fontFamily: FONT, fontSize: '12px', marginTop: '2px',
                      color: item.confidence === 'high' ? '#059669' :
                             item.confidence === 'medium' ? '#D97706' :
                             item.confidence === 'rejected' ? '#DC2626' : 'var(--text-secondary)',
                    }}>
                      {item.confidence === 'rejected' ? 'Not a match' :
                       `Matched: ${item.matched_name}`}
                    </div>
                  )}
                </div>
                <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)', flexShrink: 0, marginLeft: '12px' }}>
                  {item.price_cents > 0 ? formatPrice(item.price_cents) : ''}
                </div>
              </div>
            ))}
            {lineItems.length === 0 && (
              <div style={{ padding: '20px', textAlign: 'center' }}>
                <div style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text-secondary)' }}>
                  No line items extracted yet. Processing may still be in progress.
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    )
  }

  // Receipt list view
  if (loading) {
    return (
      <div style={{ padding: '40px 20px', textAlign: 'center' }}>
        <div style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text-secondary)' }}>Loading receipts...</div>
      </div>
    )
  }

  if (receipts.length === 0) {
    return (
      <div style={{
        display: 'flex', flexDirection: 'column', alignItems: 'center',
        padding: '40px 20px', gap: '16px',
      }}>
        <PennyAvatar size={48} />
        <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: 'var(--text)', textAlign: 'center' }}>
          No receipts yet
        </div>
        <div style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text-secondary)', textAlign: 'center', lineHeight: '1.5', maxWidth: '280px' }}>
          Snap a photo of your grocery receipt and I'll match the items to your list and verify your swaps.
        </div>
        <button
          onClick={onSnapReceipt}
          style={{
            marginTop: '8px', padding: '12px 24px', borderRadius: '10px',
            border: 'none', backgroundColor: 'var(--brand)', color: '#fff',
            fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer',
            display: 'flex', alignItems: 'center', gap: '8px',
          }}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
            <circle cx="12" cy="13" r="4"/>
          </svg>
          Snap your receipt
        </button>
      </div>
    )
  }

  return (
    <div style={{ padding: '0 16px 20px' }}>
      {/* Snap receipt button at top */}
      <button
        onClick={onSnapReceipt}
        style={{
          width: '100%', margin: '12px 0', padding: '12px', borderRadius: '10px',
          border: '1px solid var(--brand)', backgroundColor: 'color-mix(in srgb, var(--brand) 3%, transparent)',
          fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: 'var(--brand)',
          cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px',
        }}
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
          <circle cx="12" cy="13" r="4"/>
        </svg>
        Snap new receipt
      </button>

      {/* Receipt list */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {receipts.map(receipt => (
          <button
            key={receipt.id}
            onClick={() => openReceipt(receipt)}
            style={{
              width: '100%', backgroundColor: 'var(--surface)', borderRadius: '10px', padding: '14px 16px',
              border: '1px solid var(--border)', cursor: 'pointer', textAlign: 'left' as const,
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
              <div style={{ flex: 1 }}>
                <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)' }}>
                  {receipt.store_name || 'Receipt'}
                </div>
                <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginTop: '3px' }}>
                  {formatDate(receipt.transaction_date || receipt.created_at)}
                </div>
                <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginTop: '2px' }}>
                  {receipt.items_found} items · {receipt.items_matched} matched
                </div>
              </div>
              <div style={{ textAlign: 'right', flexShrink: 0, marginLeft: '12px' }}>
                {receipt.total_cents > 0 && (
                  <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                    {formatPrice(receipt.total_cents)}
                  </div>
                )}
                {receipt.swaps_verified > 0 && (
                  <div style={{
                    marginTop: '4px', padding: '2px 8px', borderRadius: '10px',
                    backgroundColor: '#D1FAE5', display: 'inline-flex', alignItems: 'center', gap: '4px',
                  }}>
                    <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="#059669" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                      <polyline points="20 6 9 17 4 12"/>
                    </svg>
                    <span style={{ fontFamily: FONT, fontSize: '11px', fontWeight: 600, color: '#059669' }}>
                      {receipt.swaps_verified} verified
                    </span>
                  </div>
                )}
              </div>
            </div>
          </button>
        ))}
      </div>
    </div>
  )
}
