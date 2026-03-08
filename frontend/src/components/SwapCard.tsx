import { useState, useEffect, useCallback } from 'react'
import { BRAND, TEXT, TEXT_SECONDARY, BORDER, WHITE, ERROR_RED, FONT } from '../styles'
import { BrandLogo } from './BrandLogo'
import type { DealType } from '../types'

// Matches the expanded PendingSwap from the backend
export interface SwapOffer {
  deal_id: number
  item_id: number
  offer_brand: string
  offer_product: string
  discount_cents: number
  offer_price_cents: number
  original_price_cents: number
  offer_size: string
  original_size: string
  offer_unit_price: string
  original_unit_price: string
  savings_percent: number
  why_bullets: string[] | null
  allergen_warning?: string
  deal_type?: DealType
  quantity_offered?: number
  trial_product_name?: string
  trial_size?: string
  campaign_tag?: string
  ends_at?: string
}

interface SwapCardProps {
  swap: SwapOffer
  onAccept: (dealId: number, itemId: number) => void
  onDecline: (dealId: number, itemId: number) => void
}

const prefersReducedMotion = () =>
  typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

function formatPrice(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`
}

export function SwapCard({ swap, onAccept, onDecline }: SwapCardProps) {
  const [whyExpanded, setWhyExpanded] = useState(true)
  const [animState, setAnimState] = useState<'idle' | 'accepting' | 'dismissing'>('idle')

  const dealType = swap.deal_type || 'sku_swap'
  const isBogo = dealType === 'bogo'
  const isTrial = dealType === 'trial'
  const isSeasonal = dealType === 'seasonal'

  const hasPriceData = swap.offer_price_cents > 0
  const hasOriginalPrice = swap.original_price_cents > 0
  const hasUnitPrice = !!swap.offer_unit_price
  const hasOriginalUnitPrice = !!swap.original_unit_price
  const hasWhyBullets = swap.why_bullets && swap.why_bullets.length > 0
  const hasSize = !!swap.offer_size
  const hasAllergen = !!swap.allergen_warning

  // Seasonal deadline
  const deadlineText = isSeasonal && swap.ends_at ? (() => {
    const end = new Date(swap.ends_at)
    const now = new Date()
    const daysLeft = Math.ceil((end.getTime() - now.getTime()) / (1000 * 60 * 60 * 24))
    if (daysLeft <= 0) return 'Last day!'
    if (daysLeft <= 3) return `${daysLeft} day${daysLeft === 1 ? '' : 's'} left!`
    return `Ends ${end.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}`
  })() : null

  const handleAccept = useCallback((e: React.MouseEvent) => {
    e.stopPropagation()
    if (animState !== 'idle') return
    if (prefersReducedMotion()) {
      onAccept(swap.deal_id, swap.item_id)
      return
    }
    setAnimState('accepting')
  }, [animState, onAccept, swap.deal_id, swap.item_id])

  const handleDecline = useCallback((e: React.MouseEvent) => {
    e.stopPropagation()
    if (animState !== 'idle') return
    if (prefersReducedMotion()) {
      onDecline(swap.deal_id, swap.item_id)
      return
    }
    setAnimState('dismissing')
  }, [animState, onDecline, swap.deal_id, swap.item_id])

  // Animation completion — only re-run when animState changes.
  // Capture dealId/itemId/callbacks at effect time to avoid re-triggering on parent re-renders.
  useEffect(() => {
    if (animState === 'accepting') {
      const t = setTimeout(() => onAccept(swap.deal_id, swap.item_id), 400)
      return () => clearTimeout(t)
    }
    if (animState === 'dismissing') {
      const t = setTimeout(() => onDecline(swap.deal_id, swap.item_id), 300)
      return () => clearTimeout(t)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [animState])

  // Build price line: "20oz · $4.49 ($0.22/oz)"
  const offerPriceLine = hasPriceData
    ? [hasSize && swap.offer_size, formatPrice(swap.offer_price_cents), hasUnitPrice && `(${swap.offer_unit_price})`]
        .filter(Boolean).join(' · ').replace(' · (', ' (')
    : null

  // Build "You pay" line: "You pay $5.99 ($0.30/oz)"
  const originalPriceLine = hasOriginalPrice
    ? ['You pay', swap.original_size || null, formatPrice(swap.original_price_cents), hasOriginalUnitPrice && `(${swap.original_unit_price})`]
        .filter(Boolean).join(' ')
    : null

  return (
    <div
      onClick={e => e.stopPropagation()}
      style={{
        margin: '0 0 4px 0',
        borderRadius: '10px',
        border: `1px solid ${BORDER}`,
        backgroundColor: WHITE,
        overflow: 'hidden',
        transition: animState !== 'idle' ? 'opacity 0.3s ease, max-height 0.3s ease' : undefined,
        opacity: animState === 'dismissing' ? 0 : 1,
        maxHeight: animState === 'dismissing' ? '0px' : '500px',
        ...(animState === 'accepting' ? { backgroundColor: `${BRAND}15` } : {}),
      }}
    >
      {/* Brand Partner disclosure + deal type badge */}
      <div style={{
        padding: '8px 12px 0 12px',
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
      }}>
        <span style={{
          fontFamily: FONT,
          fontSize: '11px',
          fontWeight: 500,
          color: TEXT_SECONDARY,
          letterSpacing: '0.3px',
          textTransform: 'uppercase' as const,
        }}>
          Brand Partner
        </span>
        {isBogo && (
          <span style={{
            fontFamily: FONT, fontSize: '10px', fontWeight: 700,
            color: WHITE, backgroundColor: '#E67E22',
            padding: '2px 6px', borderRadius: '4px',
            letterSpacing: '0.3px', textTransform: 'uppercase' as const,
          }}>2x</span>
        )}
        {isTrial && (
          <span style={{
            fontFamily: FONT, fontSize: '10px', fontWeight: 700,
            color: WHITE, backgroundColor: '#27AE60',
            padding: '2px 6px', borderRadius: '4px',
            letterSpacing: '0.3px', textTransform: 'uppercase' as const,
          }}>Free Trial</span>
        )}
        {deadlineText && (
          <span style={{
            fontFamily: FONT, fontSize: '10px', fontWeight: 700,
            color: WHITE, backgroundColor: '#E74C3C',
            padding: '2px 6px', borderRadius: '4px',
            letterSpacing: '0.3px',
          }}>{deadlineText}</span>
        )}
      </div>

      {/* Card content */}
      <div style={{ padding: '8px 12px 12px 12px' }}>
        {/* Product row: logo + info */}
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: '10px' }}>
          <BrandLogo brand={swap.offer_brand} size={52} />
          <div style={{ flex: 1, minWidth: 0 }}>
            {/* Product name */}
            <div style={{
              fontFamily: FONT,
              fontSize: '15px',
              fontWeight: 600,
              color: TEXT,
              lineHeight: 1.3,
            }}>
              {isBogo
                ? `${swap.offer_brand} ${swap.offer_product} — Get ${swap.quantity_offered || 2} for 1`
                : isTrial && swap.trial_product_name
                  ? `Free: ${swap.offer_brand} ${swap.trial_product_name}`
                  : `${swap.offer_brand} ${swap.offer_product}`
              }
            </div>
            {isTrial && swap.trial_size && (
              <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '2px' }}>
                {swap.trial_size} — keep your original item
              </div>
            )}

            {/* Offer price line */}
            {offerPriceLine && (
              <div style={{
                fontFamily: FONT,
                fontSize: '13px',
                color: TEXT,
                marginTop: '2px',
              }}>
                {offerPriceLine}
              </div>
            )}

            {/* Original price + Why? row */}
            {(originalPriceLine || hasWhyBullets) && (
              <div style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                marginTop: '2px',
              }}>
                {originalPriceLine && (
                  <span style={{
                    fontFamily: FONT,
                    fontSize: '13px',
                    color: TEXT_SECONDARY,
                  }}>
                    {originalPriceLine}
                  </span>
                )}
                {hasWhyBullets && (
                  <button
                    onClick={(e) => { e.stopPropagation(); setWhyExpanded(!whyExpanded) }}
                    aria-expanded={whyExpanded}
                    aria-label={whyExpanded ? 'Hide why Penny picked this' : 'Show why Penny picked this'}
                    style={{
                      background: 'none',
                      border: 'none',
                      color: BRAND,
                      fontFamily: FONT,
                      fontSize: '13px',
                      fontWeight: 600,
                      cursor: 'pointer',
                      padding: '2px 0',
                      marginLeft: '8px',
                      whiteSpace: 'nowrap' as const,
                      flexShrink: 0,
                    }}
                  >
                    Why? {whyExpanded ? '\u25B4' : '\u25BE'}
                  </button>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Why? expanded panel */}
        <div style={{
          maxHeight: whyExpanded ? '200px' : '0px',
          opacity: whyExpanded ? 1 : 0,
          overflow: 'hidden',
          transition: 'max-height 0.25s ease, opacity 0.2s ease',
        }}>
          <div style={{
            marginTop: '8px',
            padding: '10px 12px',
            backgroundColor: '#F7F6F3',
            borderRadius: '8px',
          }}>
            <div style={{
              fontFamily: FONT,
              fontSize: '12px',
              fontWeight: 600,
              color: TEXT_SECONDARY,
              marginBottom: '6px',
            }}>
              Why Penny picked this:
            </div>
            {swap.why_bullets?.map((bullet, i) => (
              <div key={i} style={{
                fontFamily: FONT,
                fontSize: '13px',
                color: TEXT,
                lineHeight: 1.5,
                paddingLeft: '12px',
                textIndent: '-12px',
              }}>
                &middot; {bullet}
              </div>
            ))}
            {hasAllergen && (
              <div role="alert" style={{
                fontFamily: FONT,
                fontSize: '12px',
                fontWeight: 600,
                color: ERROR_RED,
                marginTop: '6px',
              }}>
                {swap.allergen_warning}
              </div>
            )}
          </div>
        </div>

        {/* Action buttons */}
        <div style={{
          display: 'flex',
          gap: '8px',
          marginTop: '10px',
        }}>
          <button
            onClick={handleAccept}
            aria-label={`${isTrial ? 'Add Free Trial' : isBogo ? 'Get BOGO' : 'Swap'}: ${swap.offer_brand} ${swap.offer_product}, save ${formatPrice(swap.discount_cents)}`}
            style={{
              flex: 1,
              minHeight: '44px',
              backgroundColor: BRAND,
              color: WHITE,
              border: 'none',
              borderRadius: '10px',
              fontFamily: FONT,
              fontSize: '14px',
              fontWeight: 600,
              cursor: 'pointer',
              display: 'flex',
              flexDirection: 'column' as const,
              alignItems: 'center',
              justifyContent: 'center',
              padding: '6px 12px',
            }}
          >
            <span>{isTrial ? 'Add Free Trial' : isBogo ? 'Get BOGO' : `Try it — save ${formatPrice(swap.discount_cents)}`}</span>
            {!isTrial && !isBogo && swap.savings_percent > 0 && (
              <span style={{ fontSize: '12px', fontWeight: 400, opacity: 0.9 }}>
                {swap.savings_percent}% less than {swap.original_size || 'yours'}
              </span>
            )}
            {(isTrial || isBogo) && (
              <span style={{ fontSize: '12px', fontWeight: 400, opacity: 0.9 }}>
                Save {formatPrice(swap.discount_cents)}
              </span>
            )}
          </button>
          <button
            onClick={handleDecline}
            aria-label={`Decline swap for ${swap.offer_brand} ${swap.offer_product}`}
            style={{
              flexShrink: 0,
              width: '44px',
              minHeight: '44px',
              backgroundColor: 'transparent',
              color: TEXT_SECONDARY,
              border: `1px solid ${BORDER}`,
              borderRadius: '10px',
              fontFamily: FONT,
              fontSize: '18px',
              cursor: 'pointer',
              padding: 0,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            ✕
          </button>
        </div>
      </div>
    </div>
  )
}
