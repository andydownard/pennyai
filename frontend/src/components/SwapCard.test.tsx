// Tapping a deal must show it is working and stop further taps until the
// server has answered. A second tap while it works would double-accept.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act, fireEvent } from '@testing-library/react'
import { SwapCard, type SwapOffer } from './SwapCard'

vi.mock('../ab', () => ({
  getCachedVariant: () => 'try_it',
  getVariant: () => Promise.resolve('try_it'),
  trackEvent: vi.fn(),
}))
vi.mock('./BrandLogo', () => ({ BrandLogo: () => null }))

const swap: SwapOffer = {
  deal_id: 1, item_id: 2, offer_brand: 'Magic Spoon', offer_product: 'Magic Spoon Cereal',
  discount_cents: 150, offer_price_cents: 598, original_price_cents: 748,
  offer_size: '7 oz', original_size: '18 oz', offer_unit_price: '$1.28/oz', original_unit_price: '$0.33/oz',
  savings_percent: 20, why_bullets: null, deal_type: 'bogo',
}

describe('SwapCard accept', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('spins and goes inactive until the accept has finished', async () => {
    let finish: () => void = () => {}
    const onAccept = vi.fn(() => new Promise<void>(r => { finish = r }))
    render(<SwapCard swap={swap} onAccept={onAccept} onDecline={vi.fn()} />)
    const accept = screen.getByRole('button', { name: /Get BOGO/ })
    expect(accept).not.toBeDisabled()

    fireEvent.click(accept)
    await act(async () => { vi.advanceTimersByTime(500) })

    expect(onAccept).toHaveBeenCalledTimes(1)
    expect(accept).toBeDisabled()
    expect(accept).toHaveAttribute('aria-busy', 'true')
    expect(screen.getByTestId('swap-spinner')).toBeInTheDocument()

    // A second tap while it works does nothing.
    fireEvent.click(accept)
    await act(async () => { vi.advanceTimersByTime(500) })
    expect(onAccept).toHaveBeenCalledTimes(1)

    await act(async () => { finish() })
    expect(accept).not.toBeDisabled()
    expect(screen.queryByTestId('swap-spinner')).not.toBeInTheDocument()
  })

  it('becomes tappable again if the accept fails', async () => {
    const onAccept = vi.fn(() => Promise.reject(new Error('network')))
    render(<SwapCard swap={swap} onAccept={onAccept} onDecline={vi.fn()} />)
    const accept = screen.getByRole('button', { name: /Get BOGO/ })
    fireEvent.click(accept)
    await act(async () => { vi.advanceTimersByTime(500) })
    expect(accept).not.toBeDisabled()
  })
})
