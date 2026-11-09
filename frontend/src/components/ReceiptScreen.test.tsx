import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'

vi.mock('../api', () => ({ fetchAPI: vi.fn() }))
vi.mock('./PennyAvatar', () => ({ PennyAvatar: () => null }))

import { fetchAPI } from '../api'
import { ReceiptScreen } from './ReceiptScreen'

const receipt = (over: Record<string, unknown> = {}) => ({
  id: 7, image_hash: 'h', items_found: 9, items_matched: 6, store_name: 'Kroger',
  transaction_date: '2026-10-30', total_cents: 4321, status: 'done', swaps_verified: 2,
  created_at: '2026-10-30T12:00:00Z', ...over,
})

const item = (id: number, over: Record<string, unknown> = {}) => ({
  id, raw_text: `RAW ${id}`, price_cents: 199, quantity: 1, matched_name: `Match ${id}`, confidence: 'high', ...over,
})

type Handler = (url: string, init?: RequestInit) => unknown
function route(h: Handler) {
  vi.mocked(fetchAPI).mockImplementation(((url: string, init?: RequestInit) => Promise.resolve(h(url, init))) as never)
}

describe('ReceiptScreen', () => {
  beforeEach(() => { vi.mocked(fetchAPI).mockReset() })

  it('shows a loading message first', () => {
    vi.mocked(fetchAPI).mockReturnValue(new Promise(() => {}) as never)
    render(<ReceiptScreen onSnapReceipt={vi.fn()} />)
    expect(screen.getByText('Loading receipts...')).toBeInTheDocument()
  })

  it('loads receipts once on mount', async () => {
    route(() => [])
    render(<ReceiptScreen onSnapReceipt={vi.fn()} />)
    await screen.findByText('No receipts yet')
    expect(fetchAPI).toHaveBeenCalledTimes(1)
    expect(fetchAPI).toHaveBeenCalledWith('/api/receipts')
  })

  it('empty state with swaps offers to snap a receipt, firing once', async () => {
    route(() => [])
    const onSnap = vi.fn()
    render(<ReceiptScreen onSnapReceipt={onSnap} />)
    fireEvent.click(await screen.findByRole('button', { name: 'Snap your receipt' }))
    expect(onSnap).toHaveBeenCalledTimes(1)
  })

  it('empty state without accepted swaps explains why and offers no snap button', async () => {
    route(() => [])
    render(<ReceiptScreen onSnapReceipt={vi.fn()} hasAcceptedSwaps={false} />)
    expect(await screen.findByText('No receipts to snap yet')).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('a non-array response is treated as empty', async () => {
    route(() => ({ error: 'nope' }))
    render(<ReceiptScreen onSnapReceipt={vi.fn()} />)
    expect(await screen.findByText('No receipts yet')).toBeInTheDocument()
  })

  it('lists receipts with store, counts, total and verified swaps', async () => {
    route(() => [receipt(), receipt({ id: 8, store_name: '', total_cents: 0, swaps_verified: 0 })])
    const onSnap = vi.fn()
    render(<ReceiptScreen onSnapReceipt={onSnap} />)
    expect(await screen.findByText('Kroger')).toBeInTheDocument()
    expect(screen.getByText('Receipt')).toBeInTheDocument()
    expect(screen.getByText('$43.21')).toBeInTheDocument()
    expect(screen.getAllByText('9 items · 6 matched')).toHaveLength(2)
    expect(screen.getByText('2 verified')).toBeInTheDocument()
    expect(screen.getAllByText(/verified/)).toHaveLength(1)
    fireEvent.click(screen.getByRole('button', { name: 'Snap new receipt' }))
    expect(onSnap).toHaveBeenCalledTimes(1)
  })

  async function openDetail(items: unknown, r = receipt()) {
    route(url => (url === '/api/receipts' ? [r] : url.endsWith('/items') ? items : {}))
    render(<ReceiptScreen onSnapReceipt={vi.fn()} />)
    fireEvent.click(await screen.findByRole('button', { name: /Kroger/ }))
  }

  it('opens detail with header facts and line items', async () => {
    await openDetail([item(1), item(2, { quantity: 3, confidence: 'medium' }), item(3, { confidence: 'rejected', price_cents: 0 })])
    expect(await screen.findByText('RAW 1')).toBeInTheDocument()
    expect(fetchAPI).toHaveBeenCalledWith('/api/receipts/7/items')
    expect(screen.getByText('9 items found')).toBeInTheDocument()
    expect(screen.getByText('6 matched')).toBeInTheDocument()
    expect(screen.getByText('2 swaps verified')).toBeInTheDocument()
    expect(screen.getByText('Matched: Match 1')).toBeInTheDocument()
    expect(screen.getByText('x3')).toBeInTheDocument()
    expect(screen.getByText('Not a match')).toBeInTheDocument()
    expect(screen.queryByText(/Help Penny learn/)).not.toBeInTheDocument()
  })

  it('singularises one verified swap', async () => {
    await openDetail([item(1)], receipt({ swaps_verified: 1 }))
    expect(await screen.findByText('1 swap verified')).toBeInTheDocument()
  })

  it('explains an empty line-item list', async () => {
    await openDetail([])
    expect(await screen.findByText(/No line items extracted yet/)).toBeInTheDocument()
  })

  it('shows loading while line items are fetched', async () => {
    route(url => (url === '/api/receipts' ? [receipt()] : undefined))
    vi.mocked(fetchAPI).mockImplementation(((url: string) =>
      url === '/api/receipts' ? Promise.resolve([receipt()]) : new Promise(() => {})) as never)
    render(<ReceiptScreen onSnapReceipt={vi.fn()} />)
    fireEvent.click(await screen.findByRole('button', { name: /Kroger/ }))
    expect(await screen.findByText('Loading items...')).toBeInTheDocument()
  })

  it('back returns to the list', async () => {
    await openDetail([item(1)])
    await screen.findByText('RAW 1')
    fireEvent.click(screen.getByRole('button', { name: 'Receipts' }))
    expect(await screen.findByRole('button', { name: 'Snap new receipt' })).toBeInTheDocument()
    expect(screen.queryByText('RAW 1')).not.toBeInTheDocument()
  })

  describe('confirming low-confidence matches', () => {
    const lows = [item(1, { confidence: 'low' }), item(2, { confidence: 'low', matched_name: 'Other' })]

    it('prompts for each unsure match, defaulting to confirmed', async () => {
      await openDetail(lows)
      expect(await screen.findByText(/I found 2 items I'm not sure about/)).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Confirm Match 1' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Reject Other' })).toBeInTheDocument()
    })

    it('ignores low-confidence lines with no match', async () => {
      await openDetail([item(1, { confidence: 'low', matched_name: '' })])
      await screen.findByText('RAW 1')
      expect(screen.queryByText(/Help Penny learn/)).not.toBeInTheDocument()
    })

    it('posts the confirm/reject choices once and reloads the detail', async () => {
      await openDetail(lows)
      fireEvent.click(await screen.findByRole('button', { name: 'Reject Match 1' }))
      fireEvent.click(screen.getByRole('button', { name: 'Confirm matches' }))
      await waitFor(() => {
        const posts = vi.mocked(fetchAPI).mock.calls.filter(c => (c[1] as RequestInit | undefined)?.method === 'POST')
        expect(posts).toHaveLength(1)
      })
      const post = vi.mocked(fetchAPI).mock.calls.find(c => (c[1] as RequestInit | undefined)?.method === 'POST')!
      expect(post[0]).toBe('/api/receipts/7/confirm')
      expect(JSON.parse((post[1] as RequestInit).body as string)).toEqual({
        confirmations: [
          { line_item_id: 1, confirmed: false, matched_name: 'Match 1' },
          { line_item_id: 2, confirmed: true, matched_name: 'Other' },
        ],
      })
      await waitFor(() => {
        const itemFetches = vi.mocked(fetchAPI).mock.calls.filter(c => c[0] === '/api/receipts/7/items')
        expect(itemFetches).toHaveLength(2)
      })
    })

    it('disables the button and shows Saving... while the post is in flight, so it cannot double-submit', async () => {
      let release: () => void = () => {}
      vi.mocked(fetchAPI).mockImplementation(((url: string, init?: RequestInit) => {
        if (url === '/api/receipts') return Promise.resolve([receipt()])
        if (init?.method === 'POST') return new Promise<void>(r => { release = r })
        return Promise.resolve(lows)
      }) as never)
      render(<ReceiptScreen onSnapReceipt={vi.fn()} />)
      fireEvent.click(await screen.findByRole('button', { name: /Kroger/ }))
      fireEvent.click(await screen.findByRole('button', { name: 'Confirm matches' }))
      const saving = await screen.findByRole('button', { name: 'Saving...' })
      expect(saving).toBeDisabled()
      fireEvent.click(saving)
      const posts = () => vi.mocked(fetchAPI).mock.calls.filter(c => (c[1] as RequestInit | undefined)?.method === 'POST')
      expect(posts()).toHaveLength(1)
      release()
      await waitFor(() => expect(screen.queryByRole('button', { name: 'Saving...' })).not.toBeInTheDocument())
    })
  })

  it('visible copy never says bot, AI or assistant', async () => {
    route(() => [])
    render(<ReceiptScreen onSnapReceipt={vi.fn()} />)
    await screen.findByText('No receipts yet')
    expect(document.body.textContent).not.toMatch(/\b(bot|AI|assistant)\b/i)
  })
})
