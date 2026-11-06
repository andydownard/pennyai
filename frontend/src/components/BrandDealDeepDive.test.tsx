import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { BrandDealDeepDive } from './BrandDealDeepDive'

const day = (date: string, impressions: number, accepts: number) => (
  { date, impressions, accepts, declines: impressions - accepts, revenue_cents: 0, verified_count: 0 }
)

const perf = { deal_id: 5, performance: [day('2026-10-01', 100, 40), day('2026-10-02', 50, 10)] }
const opt = {
  deal_id: 5,
  metrics: { offered: 150, accepted: 50, declined: 100, accept_rate: 33.333, budget_utilization: 61.6, category_avg_rate: 25 },
  decline_reasons: [{ reason: 'Too pricey', count: 8 }, { reason: 'Prefer my brand', count: 4 }],
  recommendations: [
    { priority: 'high', category: 'Price', title: 'Lower the rebate floor', description: 'Shoppers drop off.' },
    { priority: 'low', category: 'Copy', title: 'Tighten the pitch', description: 'Shorter is better.' },
  ],
}

function resp(ok: boolean, body: unknown, status = ok ? 200 : 500) {
  return Promise.resolve({ ok, status, json: () => Promise.resolve(body) } as Response)
}

function mockApi(p: () => Promise<Response>, o: () => Promise<Response>) {
  const f = vi.fn((url: string) => (url.startsWith('/brand/deal-performance/') ? p() : o()))
  vi.stubGlobal('fetch', f)
  return f
}

describe('BrandDealDeepDive', () => {
  afterEach(() => { vi.unstubAllGlobals() })

  it('shows a loading message while both requests are pending', () => {
    mockApi(() => new Promise(() => {}), () => new Promise(() => {}))
    render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    expect(screen.getByText('Loading deal analytics...')).toBeInTheDocument()
    expect(screen.queryByText('Key Metrics')).not.toBeInTheDocument()
  })

  it('fetches performance and optimize data for the deal', async () => {
    const f = mockApi(() => resp(true, perf), () => resp(true, opt))
    render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    await screen.findByText('Key Metrics')
    expect(f).toHaveBeenCalledWith('/brand/deal-performance/5')
    expect(f).toHaveBeenCalledWith('/brand/optimize/5')
    expect(f).toHaveBeenCalledTimes(2)
  })

  it('renders metrics exactly as the server reports them', async () => {
    mockApi(() => resp(true, perf), () => resp(true, opt))
    render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    expect(await screen.findByText('33.3%')).toBeInTheDocument()
    expect(screen.getByText('50 of 150')).toBeInTheDocument()
    expect(screen.getByText('62%')).toBeInTheDocument()
    expect(screen.getByText('25.0%')).toBeInTheDocument()
    expect(screen.getByText('100')).toBeInTheDocument()
    expect(screen.getByText('Accept Rate')).toBeInTheDocument()
    expect(screen.getByText('Budget Used')).toBeInTheDocument()
  })

  it('colours the accept rate green at or above the category average, red below', async () => {
    mockApi(() => resp(true, perf), () => resp(true, opt))
    const { unmount } = render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    expect(await screen.findByText('33.3%')).toHaveStyle({ color: 'var(--success)' })
    unmount()
    mockApi(() => resp(true, perf), () => resp(true, { ...opt, metrics: { ...opt.metrics, accept_rate: 10 } }))
    render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    expect(await screen.findByText('10.0%')).toHaveStyle({ color: 'var(--error)' })
  })

  it('draws the daily chart with a legend and date labels', async () => {
    mockApi(() => resp(true, perf), () => resp(true, opt))
    const { container } = render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    await screen.findByText('Daily Performance (last 30 days)')
    expect(container.querySelectorAll('svg rect')).toHaveLength(4)
    expect(screen.getByText('10-01')).toBeInTheDocument()
    expect(screen.getByText('Impressions')).toBeInTheDocument()
    expect(screen.getByText('Accepts')).toBeInTheDocument()
  })

  it('shows only the last 30 days of a longer series', async () => {
    const many = Array.from({ length: 45 }, (_, i) => day(`2026-09-${String((i % 28) + 1).padStart(2, '0')}-${i}`, 10, 5))
    mockApi(() => resp(true, { deal_id: 5, performance: many }), () => resp(true, opt))
    const { container } = render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    await screen.findByText('Key Metrics')
    expect(container.querySelectorAll('svg rect')).toHaveLength(60)
  })

  it('lists decline reasons and recommendations with priority badges', async () => {
    mockApi(() => resp(true, perf), () => resp(true, opt))
    render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    expect(await screen.findByText('Too pricey')).toBeInTheDocument()
    expect(screen.getByText('Prefer my brand')).toBeInTheDocument()
    expect(screen.getByText('Lower the rebate floor')).toBeInTheDocument()
    expect(screen.getByText('Shoppers drop off.')).toBeInTheDocument()
    expect(screen.getByText('high')).toHaveStyle({ color: 'var(--error)' })
    expect(screen.getByText('low')).toHaveStyle({ color: 'var(--info)' })
    expect(screen.getByText('Price')).toBeInTheDocument()
  })

  it('falls back to the low badge colour for an unknown priority', async () => {
    const odd = { ...opt, recommendations: [{ priority: 'urgent', category: 'X', title: 'T', description: 'D' }] }
    mockApi(() => resp(true, perf), () => resp(true, odd))
    render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    expect(await screen.findByText('urgent')).toHaveStyle({ color: 'var(--info)' })
  })

  it('shows an empty message for each empty section', async () => {
    mockApi(() => resp(true, { deal_id: 5, performance: [] }), () => resp(true, { ...opt, decline_reasons: [], recommendations: [] }))
    render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    expect(await screen.findByText('No performance data yet.')).toBeInTheDocument()
    expect(screen.getByText('No decline data recorded.')).toBeInTheDocument()
    expect(screen.getByText('No recommendations at this time.')).toBeInTheDocument()
  })

  it('shows the error when the performance request fails, with no stale sections', async () => {
    mockApi(() => resp(false, {}, 404), () => resp(true, opt))
    render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    expect(await screen.findByText('Performance API returned 404')).toBeInTheDocument()
    expect(screen.queryByText('Key Metrics')).not.toBeInTheDocument()
    expect(screen.queryByText('Loading deal analytics...')).not.toBeInTheDocument()
  })

  it('shows the error when the optimize request fails', async () => {
    mockApi(() => resp(true, perf), () => resp(false, {}, 500))
    render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    expect(await screen.findByText('Optimize API returned 500')).toBeInTheDocument()
  })

  it('shows a network error message', async () => {
    mockApi(() => Promise.reject(new Error('Failed to fetch')), () => resp(true, opt))
    render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    expect(await screen.findByText('Failed to fetch')).toBeInTheDocument()
  })

  it('uses a generic message when the error has no text', async () => {
    mockApi(() => Promise.reject(new Error('')), () => resp(true, opt))
    render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    expect(await screen.findByText('Failed to load deal data')).toBeInTheDocument()
  })

  it('refetches when the deal changes', async () => {
    const f = mockApi(() => resp(true, perf), () => resp(true, opt))
    const { rerender } = render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    await screen.findByText('Key Metrics')
    rerender(<BrandDealDeepDive dealId={6} onClose={vi.fn()} />)
    await waitFor(() => expect(f).toHaveBeenCalledWith('/brand/optimize/6'))
    expect(f).toHaveBeenCalledTimes(4)
  })

  it('does not update after unmount', async () => {
    let release: (r: Response) => void = () => {}
    mockApi(() => new Promise<Response>(r => { release = r }), () => resp(true, opt))
    const err = vi.spyOn(console, 'error').mockImplementation(() => {})
    const { unmount } = render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    unmount()
    release({ ok: true, status: 200, json: () => Promise.resolve(perf) } as Response)
    await new Promise(r => setTimeout(r, 10))
    expect(err).not.toHaveBeenCalled()
    err.mockRestore()
  })

  it('close button fires onClose once', () => {
    mockApi(() => new Promise(() => {}), () => new Promise(() => {}))
    const onClose = vi.fn()
    render(<BrandDealDeepDive dealId={5} onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Close deep dive' }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('backdrop click closes, clicking inside the panel does not', () => {
    mockApi(() => new Promise(() => {}), () => new Promise(() => {}))
    const onClose = vi.fn()
    render(<BrandDealDeepDive dealId={5} onClose={onClose} />)
    fireEvent.click(screen.getByText('Deal Deep Dive'))
    expect(onClose).not.toHaveBeenCalled()
    fireEvent.click(screen.getByText('Deal Deep Dive').parentElement!.parentElement!.parentElement!)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('visible copy never says bot, AI or assistant', async () => {
    mockApi(() => resp(true, perf), () => resp(true, opt))
    render(<BrandDealDeepDive dealId={5} onClose={vi.fn()} />)
    await screen.findByText('Key Metrics')
    expect(document.body.textContent).not.toMatch(/\b(bot|AI|assistant)\b/i)
  })
})
