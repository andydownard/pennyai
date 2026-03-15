import { useState, useRef, useEffect } from 'react'
import { fetchAction } from '../api'
import { track } from '../analytics'
import { trackEvent } from '../ab'
import type { ListItem } from '../components/ItemRow'
import type { SwapOffer } from '../components/SwapCard'

// Haptic feedback (matches App.tsx)
const haptic = {
  success: () => navigator.vibrate?.([10, 50, 5, 50, 15]),
  double:  () => navigator.vibrate?.([5, 30, 5]),
}

export type DisambigItem = {
  item_id: number
  product_category: string
  current_brand: string
  alternatives: string[]
  auto_assigned?: boolean
}

export type SkippedItem = {
  parsed: { name: string; brand: string; quantity: number; size: string; department: string; emoji: string }
  existing_id: number
  reason?: string     // "duplicate" (already on list) or "ambiguous" / "needs_clarification" / "not_a_grocery_item"
  raw_input?: string  // original text that couldn't be parsed
}

interface UseSwapStateOptions {
  setItems: React.Dispatch<React.SetStateAction<ListItem[]>>
  sortItems: (arr: ListItem[]) => ListItem[]
  typePennyHome: (msg: string, btns?: { label: string; onClick: () => void }[]) => void
  showToast: (msg: string, type?: 'success' | 'error') => void
  handleSessionExpired: () => void
  setEarningsTotalCents: React.Dispatch<React.SetStateAction<number>>
  setShowFireworks: React.Dispatch<React.SetStateAction<boolean>>
}

export function useSwapState(options: UseSwapStateOptions) {
  // Keep stable refs for callbacks to avoid stale closures
  const optRef = useRef(options)
  optRef.current = options

  // --- Swap offers by item ---
  const [swapsByItemId, setSwapsByItemId] = useState<Record<number, SwapOffer>>({})

  // --- Review state (recently added/skipped items) ---
  const [lastAdded, setLastAdded] = useState<ListItem[]>([])
  const [lastSkipped, setLastSkipped] = useState<SkippedItem[]>([])
  const [showReviewSheet, setShowReviewSheet] = useState(false)
  const hasReviewedRef = useRef(false)

  // --- Brand disambiguation ---
  const [pennyDisambig, setPennyDisambig] = useState<DisambigItem[]>([])
  const [pennyDisambigIndex, setPennyDisambigIndex] = useState(0)

  // --- Post-swap share prompt ---
  const [postSwapShare, setPostSwapShare] = useState<{ brand: string; savings: number } | null>(null)

  const advanceDisambig = () => {
    const next = pennyDisambigIndex + 1
    if (next >= pennyDisambig.length) {
      setPennyDisambig([])
      setPennyDisambigIndex(0)
      optRef.current.typePennyHome("All set! I'll remember those for next time.")
    } else {
      setPennyDisambigIndex(next)
    }
  }

  // Auto-confirm auto-assigned brands after 4s of inactivity
  useEffect(() => {
    if (pennyDisambig.length === 0 || pennyDisambigIndex >= pennyDisambig.length) return
    const d = pennyDisambig[pennyDisambigIndex]
    if (!d?.auto_assigned || !d?.current_brand) return
    const timer = setTimeout(() => {
      handleBrandPick(d.current_brand)
    }, 4000)
    return () => clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pennyDisambigIndex, pennyDisambig])

  const handleBrandPick = async (brand: string) => {
    const d = pennyDisambig[pennyDisambigIndex]
    if (!d) return
    const result = await fetchAction('/api/list/brand-resolve', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ item_id: d.item_id, brand, product_category: d.product_category }),
    })
    if (result.ok) {
      optRef.current.setItems(prev => prev.map(i => i.id === d.item_id ? { ...i, brand } : i))
      setLastAdded(prev => prev.map(i => i.id === d.item_id ? { ...i, brand } : i))
      if (d.auto_assigned) {
        const event = brand === d.current_brand ? 'brand_default_confirmed' : 'brand_default_corrected'
        trackEvent('onboarding_v2', event)
      }
      advanceDisambig()
    } else if (result.status !== 401) {
      optRef.current.showToast('Couldn\'t save brand preference — try again', 'error')
    }
  }

  const handleSwapAccept = async (dealId: number, itemId: number) => {
    haptic.success()
    const swap = swapsByItemId[itemId]
    const offerBrand = swap?.offer_brand || ''
    const discountCents = swap?.discount_cents || 0
    const oldSwaps = { ...swapsByItemId }
    setSwapsByItemId(prev => { const next = { ...prev }; delete next[itemId]; return next })
    optRef.current.typePennyHome('Nice swap!')
    try {
      const res = await fetch('/api/list/parse', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: 'accept_swap', swap_deal_id: dealId, swap_item_id: itemId }),
      })
      if (res.status === 401) { optRef.current.handleSessionExpired(); return }
      if (res.ok) {
        const data = await res.json()
        if (data.updated?.length > 0) {
          optRef.current.setItems(prev => {
            const map = new Map(prev.map(i => [i.id, i]))
            for (const item of data.updated) map.set(item.id, item)
            return optRef.current.sortItems(Array.from(map.values()))
          })
        }
        if (data.swap_credits_cents != null) {
          optRef.current.setEarningsTotalCents(data.swap_credits_cents)
        }
        const firstSwapKey = 'thriftly_first_swap'
        if (!localStorage.getItem(firstSwapKey)) {
          localStorage.setItem(firstSwapKey, '1')
          optRef.current.setShowFireworks(true)
          optRef.current.typePennyHome('Your first swap — nice save!')
          track('first_swap_celebration', { deal_id: dealId, offer_brand: offerBrand })
        }
        if (discountCents > 0) {
          setPostSwapShare({ brand: offerBrand, savings: discountCents })
          track('post_swap_share_shown', { deal_id: dealId, offer_brand: offerBrand })
          setTimeout(() => setPostSwapShare(null), 8000)
        }
      } else {
        setSwapsByItemId(oldSwaps)
        const err = await res.json().catch(() => null)
        optRef.current.showToast(err?.error || 'Couldn\'t accept the swap', 'error')
      }
    } catch {
      setSwapsByItemId(oldSwaps)
      optRef.current.showToast('Connection error — try again', 'error')
    }
  }

  const handleSwapDecline = async (dealId: number, itemId: number) => {
    haptic.double()
    track('swap_declined', { deal_id: dealId })
    setSwapsByItemId(prev => { const next = { ...prev }; delete next[itemId]; return next })
    try {
      await fetch('/api/list/parse', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: 'decline_swap', swap_deal_id: dealId, swap_item_id: itemId }),
      })
    } catch { /* silent — optimistic removal already done */ }
  }

  return {
    // Swap offers
    swapsByItemId,
    setSwapsByItemId,
    // Review state
    lastAdded,
    setLastAdded,
    lastSkipped,
    setLastSkipped,
    showReviewSheet,
    setShowReviewSheet,
    hasReviewedRef,
    // Brand disambiguation
    pennyDisambig,
    setPennyDisambig,
    pennyDisambigIndex,
    setPennyDisambigIndex,
    // Post-swap share
    postSwapShare,
    setPostSwapShare,
    // Handlers
    handleSwapAccept,
    handleSwapDecline,
    handleBrandPick,
    advanceDisambig,
  }
}
