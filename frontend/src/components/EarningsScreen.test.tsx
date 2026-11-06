// Balance display: label above a brand-green total; money not yet verified is
// "Coming" (quiet, clock) rather than amber "Pending"; paid is green with a check.

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'

vi.mock('../api', () => ({ fetchAPI: vi.fn() }))
vi.mock('./BrandLogo', () => ({ BrandLogo: () => null }))
vi.mock('./PennyAvatar', () => ({ PennyAvatar: () => null }))

import { fetchAPI } from '../api'
import { EarningsScreen } from './EarningsScreen'

const tx = (id: number, status: string, cents: number) => ({
  id, offer_brand: 'Kroger', offer_product: 'eggs', competitor_product: '', original_item_name: 'eggs',
  amount_cents: cents, status, created_at: '2026-11-03 10:00:00', attachment_path: '',
})

function load(payload: Record<string, unknown>) {
  vi.mocked(fetchAPI).mockResolvedValue(payload as never)
  render(<EarningsScreen onBack={vi.fn()} />)
}

describe('EarningsScreen', () => {
  beforeEach(() => vi.mocked(fetchAPI).mockReset())

  it('labels the money Coming and Paid, never Pending', async () => {
    load({ transactions: [tx(1, 'pending', 40), tx(2, 'paid', 50)], total_cents: 90, pending_cents: 40, paid_cents: 50 })
    expect(await screen.findAllByText('Coming')).not.toHaveLength(0)
    expect(screen.getAllByText('Paid').length).toBeGreaterThan(0)
    expect(screen.queryByText('Pending')).not.toBeInTheDocument()
  })

  it('shows the total with its label above it, in brand green', async () => {
    load({ transactions: [tx(1, 'paid', 50)], total_cents: 50, pending_cents: 0, paid_cents: 50 })
    const total = await screen.findByText('$0.50', { selector: '[data-total]' })
    expect(total).toHaveStyle({ color: 'var(--brand)' })
    const label = screen.getByText('Total earned')
    expect(label.compareDocumentPosition(total) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('states the payout terms once and promises no share of brand revenue', async () => {
    load({ transactions: [tx(1, 'paid', 50)], total_cents: 50, pending_cents: 0, paid_cents: 50 })
    expect(await screen.findByText(/Paid monthly by Venmo/)).toBeInTheDocument()
    expect(screen.queryByText(/30%/)).not.toBeInTheDocument()
  })
})
