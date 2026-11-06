import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useSwapState, type DisambigItem } from './useSwapState'
import type { SwapOffer } from '../components/SwapCard'
import type { ListItem } from '../components/ItemRow'

vi.mock('../api', () => ({ fetchAction: vi.fn(), fetchAPI: vi.fn() }))
vi.mock('../analytics', () => ({ track: vi.fn() }))
vi.mock('../ab', () => ({ trackEvent: vi.fn() }))
vi.mock('../haptics', () => ({ haptic: { success: vi.fn(), double: vi.fn() } }))

import { fetchAction } from '../api'
import { track } from '../analytics'
import { trackEvent } from '../ab'

const swap: SwapOffer = {
  deal_id: 7, item_id: 2, offer_brand: 'Magic Spoon', offer_product: 'Magic Spoon Cereal',
  discount_cents: 150, offer_price_cents: 598, original_price_cents: 748,
  offer_size: '7 oz', original_size: '18 oz', offer_unit_price: '$1.28/oz', original_unit_price: '$0.33/oz',
  savings_percent: 20, why_bullets: null, deal_type: 'bogo',
}

const mkItem = (id: number, brand = ''): ListItem => ({ id, name: `item${id}`, brand } as unknown as ListItem)

function setup() {
  const opts = {
    setItems: vi.fn(),
    sortItems: vi.fn((a: ListItem[]) => a),
    typePennyHome: vi.fn(),
    showToast: vi.fn(),
    handleSessionExpired: vi.fn(),
    setEarningsTotalCents: vi.fn(),
    setShowFireworks: vi.fn(),
  }
  const hook = renderHook(() => useSwapState(opts))
  return { opts, ...hook }
}

const jsonRes = (status: number, body: unknown) =>
  ({ status, ok: status >= 200 && status < 300, json: () => Promise.resolve(body) }) as Response

let fetchMock: ReturnType<typeof vi.fn>

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
})
afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('useSwapState initial state', () => {
  it('starts empty', () => {
    const { result } = setup()
    expect(result.current.swapsByItemId).toEqual({})
    expect(result.current.lastAdded).toEqual([])
    expect(result.current.lastSkipped).toEqual([])
    expect(result.current.showReviewSheet).toBe(false)
    expect(result.current.hasReviewedRef.current).toBe(false)
    expect(result.current.pennyDisambig).toEqual([])
    expect(result.current.pennyDisambigIndex).toBe(0)
    expect(result.current.postSwapShare).toBeNull()
  })
})

describe('brand swap accept', () => {
  it('posts the accept, removes the card, and tells Penny', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, { updated: [mkItem(2, 'Magic Spoon')], swap_credits_cents: 150 }))
    const { result, opts } = setup()
    act(() => result.current.setSwapsByItemId({ 2: swap, 3: { ...swap, item_id: 3 } }))

    await act(async () => { await result.current.handleSwapAccept(7, 2) })

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/list/parse')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body)).toEqual({ text: 'accept_swap', swap_deal_id: 7, swap_item_id: 2 })
    expect(Object.keys(result.current.swapsByItemId)).toEqual(['3'])
    expect(opts.typePennyHome).toHaveBeenCalledWith('Nice swap!')
    expect(opts.setEarningsTotalCents).toHaveBeenCalledWith(150)
    expect(opts.setItems).toHaveBeenCalledTimes(1)
    // the updater merges the server's updated item over the existing one
    const updater = opts.setItems.mock.calls[0][0] as (p: ListItem[]) => ListItem[]
    const merged = updater([mkItem(1), mkItem(2, 'Old')])
    expect(merged.find(i => i.id === 2)?.brand).toBe('Magic Spoon')
    expect(opts.sortItems).toHaveBeenCalled()
  })

  it('does not set earnings when the server omits swap_credits_cents', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, {}))
    const { result, opts } = setup()
    await act(async () => { await result.current.handleSwapAccept(7, 2) })
    expect(opts.setEarningsTotalCents).not.toHaveBeenCalled()
    expect(opts.setItems).not.toHaveBeenCalled()
  })

  it('sets earnings even when credits are zero', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, { swap_credits_cents: 0 }))
    const { result, opts } = setup()
    await act(async () => { await result.current.handleSwapAccept(7, 2) })
    expect(opts.setEarningsTotalCents).toHaveBeenCalledWith(0)
  })

  it('celebrates the first swap once and records it', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, {}))
    const { result, opts } = setup()
    await act(async () => { await result.current.handleSwapAccept(7, 2) })
    expect(localStorage.getItem('groflo_first_swap')).toBe('1')
    expect(opts.setShowFireworks).toHaveBeenCalledWith(true)
    expect(opts.typePennyHome).toHaveBeenCalledWith('Your first swap — nice save!')
    expect(track).toHaveBeenCalledWith('first_swap_celebration', { deal_id: 7, offer_brand: '' })

    opts.setShowFireworks.mockClear()
    await act(async () => { await result.current.handleSwapAccept(8, 3) })
    expect(opts.setShowFireworks).not.toHaveBeenCalled()
  })

  it('does not celebrate again when the legacy first-swap key exists', async () => {
    localStorage.setItem('thriftly_first_swap', '1')
    fetchMock.mockResolvedValue(jsonRes(200, {}))
    const { result, opts } = setup()
    await act(async () => { await result.current.handleSwapAccept(7, 2) })
    expect(opts.setShowFireworks).not.toHaveBeenCalled()
    expect(localStorage.getItem('groflo_first_swap')).toBeNull()
  })

  it('shows the post-swap share prompt for 8 seconds when the deal saved money', async () => {
    localStorage.setItem('groflo_first_swap', '1')
    fetchMock.mockResolvedValue(jsonRes(200, {}))
    const { result } = setup()
    act(() => result.current.setSwapsByItemId({ 2: swap }))
    vi.useFakeTimers()
    await act(async () => { await result.current.handleSwapAccept(7, 2) })
    expect(result.current.postSwapShare).toEqual({ brand: 'Magic Spoon', savings: 150 })
    expect(track).toHaveBeenCalledWith('post_swap_share_shown', { deal_id: 7, offer_brand: 'Magic Spoon' })
    act(() => { vi.advanceTimersByTime(8000) })
    expect(result.current.postSwapShare).toBeNull()
  })

  it('shows no share prompt when no swap offer is known for the item', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, {}))
    const { result } = setup()
    await act(async () => { await result.current.handleSwapAccept(7, 99) })
    expect(result.current.postSwapShare).toBeNull()
  })

  it('expires the session on 401 and keeps the card so it can be tapped again', async () => {
    fetchMock.mockResolvedValue(jsonRes(401, {}))
    const { result, opts } = setup()
    act(() => result.current.setSwapsByItemId({ 2: swap }))
    await act(async () => { await result.current.handleSwapAccept(7, 2) })
    expect(opts.handleSessionExpired).toHaveBeenCalledTimes(1)
    expect(result.current.swapsByItemId[2]).toBeDefined()
    expect(opts.typePennyHome).not.toHaveBeenCalled()
    expect(opts.showToast).not.toHaveBeenCalled()
  })

  it('keeps the card and shows the server error on failure', async () => {
    fetchMock.mockResolvedValue(jsonRes(409, { error: 'Deal expired' }))
    const { result, opts } = setup()
    act(() => result.current.setSwapsByItemId({ 2: swap }))
    await act(async () => { await result.current.handleSwapAccept(7, 2) })
    expect(opts.showToast).toHaveBeenCalledWith('Deal expired', 'error')
    expect(result.current.swapsByItemId[2]).toBeDefined()
    expect(opts.typePennyHome).not.toHaveBeenCalled()
    expect(localStorage.getItem('groflo_first_swap')).toBeNull()
    expect(opts.setShowFireworks).not.toHaveBeenCalled()
  })

  it('falls back to a generic message when the error body is not JSON', async () => {
    fetchMock.mockResolvedValue({ status: 500, ok: false, json: () => Promise.reject(new Error('bad')) } as Response)
    const { result, opts } = setup()
    await act(async () => { await result.current.handleSwapAccept(7, 2) })
    expect(opts.showToast).toHaveBeenCalledWith("Couldn't accept the swap", 'error')
  })

  it('reports a connection error and keeps the card when the network fails', async () => {
    fetchMock.mockRejectedValue(new Error('offline'))
    const { result, opts } = setup()
    act(() => result.current.setSwapsByItemId({ 2: swap }))
    await act(async () => { await result.current.handleSwapAccept(7, 2) })
    expect(opts.showToast).toHaveBeenCalledWith('Connection error — try again', 'error')
    expect(result.current.swapsByItemId[2]).toBeDefined()
  })

  it('can be retried after a failure and then succeeds', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonRes(500, { error: 'boom' }))
      .mockResolvedValueOnce(jsonRes(200, {}))
    const { result, opts } = setup()
    act(() => result.current.setSwapsByItemId({ 2: swap }))
    await act(async () => { await result.current.handleSwapAccept(7, 2) })
    expect(result.current.swapsByItemId[2]).toBeDefined()
    await act(async () => { await result.current.handleSwapAccept(7, 2) })
    expect(result.current.swapsByItemId[2]).toBeUndefined()
    expect(opts.typePennyHome).toHaveBeenCalledWith('Nice swap!')
  })
})

describe('brand swap decline', () => {
  it('removes the card at once and tells the server', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, {}))
    const { result } = setup()
    act(() => result.current.setSwapsByItemId({ 2: swap, 3: { ...swap, item_id: 3 } }))
    await act(async () => { await result.current.handleSwapDecline(7, 2) })
    expect(Object.keys(result.current.swapsByItemId)).toEqual(['3'])
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/list/parse')
    expect(JSON.parse(init.body)).toEqual({ text: 'decline_swap', swap_deal_id: 7, swap_item_id: 2 })
    expect(track).toHaveBeenCalledWith('swap_declined', { deal_id: 7 })
  })

  it('stays declined and silent when the network fails', async () => {
    fetchMock.mockRejectedValue(new Error('offline'))
    const { result, opts } = setup()
    act(() => result.current.setSwapsByItemId({ 2: swap }))
    await act(async () => { await result.current.handleSwapDecline(7, 2) })
    expect(result.current.swapsByItemId[2]).toBeUndefined()
    expect(opts.showToast).not.toHaveBeenCalled()
  })

  it('declining a deal that is not on screen is harmless', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, {}))
    const { result } = setup()
    await act(async () => { await result.current.handleSwapDecline(7, 42) })
    expect(result.current.swapsByItemId).toEqual({})
  })
})

const d1: DisambigItem = { item_id: 10, product_category: 'cereal', current_brand: 'Cheerios', alternatives: ['Kix', 'Chex'] }
const d2: DisambigItem = { item_id: 11, product_category: 'soap', current_brand: '', alternatives: ['Dial'] }

describe('brand disambiguation', () => {
  it('advances to the next item, then finishes with Penny', () => {
    const { result, opts } = setup()
    act(() => result.current.setPennyDisambig([d1, d2]))
    act(() => result.current.advanceDisambig())
    expect(result.current.pennyDisambigIndex).toBe(1)
    expect(opts.typePennyHome).not.toHaveBeenCalled()
    act(() => result.current.advanceDisambig())
    expect(result.current.pennyDisambig).toEqual([])
    expect(result.current.pennyDisambigIndex).toBe(0)
    expect(opts.typePennyHome).toHaveBeenCalledWith("All set! I'll remember those for next time.")
  })

  it('does nothing on a brand pick when there is no pending item', async () => {
    const { result } = setup()
    await act(async () => { await result.current.handleBrandPick('Kix') })
    expect(fetchAction).not.toHaveBeenCalled()
  })

  it('saves a brand pick, updates the list and review, and advances', async () => {
    vi.mocked(fetchAction).mockResolvedValue({ ok: true, status: 200 })
    const { result, opts } = setup()
    act(() => {
      result.current.setPennyDisambig([d1, d2])
      result.current.setLastAdded([mkItem(10), mkItem(11)])
    })
    await act(async () => { await result.current.handleBrandPick('Kix') })

    const [url, init] = vi.mocked(fetchAction).mock.calls[0]
    expect(url).toBe('/api/list/brand-resolve')
    expect(JSON.parse(init!.body as string)).toEqual({ item_id: 10, brand: 'Kix', product_category: 'cereal' })
    const updater = opts.setItems.mock.calls[0][0] as (p: ListItem[]) => ListItem[]
    expect(updater([mkItem(10), mkItem(11)]).map(i => i.brand)).toEqual(['Kix', ''])
    expect(result.current.lastAdded.map(i => i.brand)).toEqual(['Kix', ''])
    expect(result.current.pennyDisambigIndex).toBe(1)
    // not auto-assigned: no onboarding event
    expect(trackEvent).not.toHaveBeenCalled()
  })

  it('tracks a corrected default when an auto-assigned brand is changed', async () => {
    vi.mocked(fetchAction).mockResolvedValue({ ok: true, status: 200 })
    const { result } = setup()
    act(() => result.current.setPennyDisambig([{ ...d1, auto_assigned: true }]))
    await act(async () => { await result.current.handleBrandPick('Kix') })
    expect(trackEvent).toHaveBeenCalledWith('onboarding_v2', 'brand_default_corrected')
  })

  it('tracks a confirmed default when an auto-assigned brand is kept', async () => {
    vi.mocked(fetchAction).mockResolvedValue({ ok: true, status: 200 })
    const { result } = setup()
    act(() => result.current.setPennyDisambig([{ ...d1, auto_assigned: true }]))
    await act(async () => { await result.current.handleBrandPick('Cheerios') })
    expect(trackEvent).toHaveBeenCalledWith('onboarding_v2', 'brand_default_confirmed')
  })

  it('toasts and stays on the item when the save fails', async () => {
    vi.mocked(fetchAction).mockResolvedValue({ ok: false, status: 500 })
    const { result, opts } = setup()
    act(() => result.current.setPennyDisambig([d1, d2]))
    await act(async () => { await result.current.handleBrandPick('Kix') })
    expect(opts.showToast).toHaveBeenCalledWith("Couldn't save brand preference — try again", 'error')
    expect(result.current.pennyDisambigIndex).toBe(0)
    expect(opts.setItems).not.toHaveBeenCalled()
  })

  it('stays quiet on 401 (the session handler already ran)', async () => {
    vi.mocked(fetchAction).mockResolvedValue({ ok: false, status: 401 })
    const { result, opts } = setup()
    act(() => result.current.setPennyDisambig([d1]))
    await act(async () => { await result.current.handleBrandPick('Kix') })
    expect(opts.showToast).not.toHaveBeenCalled()
  })

  describe('auto-confirm timer', () => {
    beforeEach(() => vi.useFakeTimers())

    it('confirms an auto-assigned brand after 4 seconds', async () => {
      vi.mocked(fetchAction).mockResolvedValue({ ok: true, status: 200 })
      const { result } = setup()
      act(() => result.current.setPennyDisambig([{ ...d1, auto_assigned: true }]))
      await act(async () => { vi.advanceTimersByTime(3999) })
      expect(fetchAction).not.toHaveBeenCalled()
      await act(async () => { vi.advanceTimersByTime(1) })
      expect(fetchAction).toHaveBeenCalledTimes(1)
      expect(JSON.parse(vi.mocked(fetchAction).mock.calls[0][1]!.body as string).brand).toBe('Cheerios')
      expect(result.current.pennyDisambig).toEqual([])
    })

    it('does not auto-confirm items that were not auto-assigned', async () => {
      const { result } = setup()
      act(() => result.current.setPennyDisambig([d1]))
      await act(async () => { vi.advanceTimersByTime(10000) })
      expect(fetchAction).not.toHaveBeenCalled()
    })

    it('does not auto-confirm an auto-assigned item with no brand', async () => {
      const { result } = setup()
      act(() => result.current.setPennyDisambig([{ ...d2, auto_assigned: true }]))
      await act(async () => { vi.advanceTimersByTime(10000) })
      expect(fetchAction).not.toHaveBeenCalled()
    })

    it('cancels the timer when the user moves on first', async () => {
      const { result } = setup()
      act(() => result.current.setPennyDisambig([{ ...d1, auto_assigned: true }, d2]))
      await act(async () => { vi.advanceTimersByTime(2000) })
      act(() => result.current.advanceDisambig())
      await act(async () => { vi.advanceTimersByTime(10000) })
      expect(fetchAction).not.toHaveBeenCalled()
    })
  })
})
