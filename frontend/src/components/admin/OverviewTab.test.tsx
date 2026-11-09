import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { OverviewTab } from './OverviewTab'
import type { AdminData, AnalyticsData, Deal, Payout } from './types'

const admin = (over: Partial<AdminData> = {}): AdminData => ({
  active_users: 12, total_users: 340, new_users: 7, k_factor: 1.256,
  leaderboard: [{ name: 'Dana', points: 90, referrals: 3 }, { name: '', points: 10, referrals: 1 }],
  nps_responses: [{ score: 9, comment: 'Love it', user_name: 'Sam', created_at: '2026-10-01T00:00:00Z' }, { score: 3, comment: '', user_name: '', created_at: '2026-10-02T00:00:00Z' }],
  nps_score: 42.4, nps_total: 2, nps_promoters: 1, nps_detractors: 1,
  ...over,
})

const deal = (over: Partial<Deal> = {}): Deal => ({
  id: 1, competitor_product: 'Brand X Milk', offer_brand: 'Kroger', offer_product: 'Milk',
  discount_cents: 50, daily_budget_cents: 10000, budget_spent_cents: 2500, status: 'active',
  starts_at: '', ends_at: '', created_at: '', offer_price_cents: 0, competitor_price_cents: 0,
  offer_size: '', competitor_size: '', offer_unit_price: '', competitor_unit_price: '',
  why_bullets: '[]', allergen_notes: '', deal_type: 'sku_swap', quantity_offered: 1,
  trial_product_name: '', trial_size: '', campaign_tag: '', ...over,
})

const payout = (over: Partial<Payout> = {}): Payout => ({
  user_id: 9, name: 'Pat', email: 'pat@example.com', total_cents: 1250, swap_count: 1, oldest_swap: '2026-09-01T00:00:00Z', ...over,
})

function setup(over: Partial<React.ComponentProps<typeof OverviewTab>> = {}) {
  const props: React.ComponentProps<typeof OverviewTab> = {
    adminData: admin(), analyticsData: null, deals: [], payouts: [],
    onDealsChange: vi.fn(), onPayoutsChange: vi.fn(), onDeepDive: vi.fn(), showToast: vi.fn(),
    ...over,
  }
  render(<OverviewTab {...props} />)
  return props
}

function stubFetch(handler: (url: string, init?: RequestInit) => { ok: boolean; body?: unknown }) {
  const f = vi.fn((url: string, init?: RequestInit) => {
    const r = handler(url, init)
    return Promise.resolve({ ok: r.ok, json: () => Promise.resolve(r.body) } as Response)
  })
  vi.stubGlobal('fetch', f)
  return f
}

afterEach(() => { vi.unstubAllGlobals() })

describe('OverviewTab', () => {
  it('shows Loading... without admin data', () => {
    setup({ adminData: null })
    expect(screen.getByText('Loading...')).toBeInTheDocument()
    expect(screen.queryByText('Referral Leaderboard')).not.toBeInTheDocument()
  })

  it('renders the headline metric cards', () => {
    setup()
    for (const [label, value] of [['Active Users', '12'], ['Total Users', '340'], ['New Users', '7'], ['K-Factor', '1.26']]) {
      expect(screen.getByText(label).nextElementSibling).toHaveTextContent(value)
    }
  })

  it('renders the leaderboard, with an Anonymous fallback, or its empty message', () => {
    setup()
    expect(screen.getByText('Dana')).toBeInTheDocument()
    expect(screen.getByText('90 pts')).toBeInTheDocument()
    expect(screen.getByText('3 referrals')).toBeInTheDocument()
    expect(screen.getAllByText('Anonymous').length).toBeGreaterThan(0)
  })

  it('leaderboard empty message', () => {
    setup({ adminData: admin({ leaderboard: [] }) })
    expect(screen.getByText('No referrals yet.')).toBeInTheDocument()
  })

  it('renders the NPS heading and responses', () => {
    setup()
    expect(screen.getByText('NPS: 42 (2 responses, 1 promoters, 1 detractors)')).toBeInTheDocument()
    expect(screen.getByText('Score: 9')).toBeInTheDocument()
    expect(screen.getByText('Love it')).toBeInTheDocument()
    expect(screen.getByText('Score: 3')).toBeInTheDocument()
  })

  it('NPS empty message', () => {
    setup({ adminData: admin({ nps_responses: [] }) })
    expect(screen.getByText('No NPS responses yet.')).toBeInTheDocument()
  })

  it('shows swap deal stats only when something was offered', () => {
    const { unmount } = render(<div />)
    unmount()
    setup({ adminData: admin({ swap_offered: 100, swap_accepted: 40, swap_declined: 60, swap_accept_rate: 40 }) })
    expect(screen.getByText('Swap Deals')).toBeInTheDocument()
    expect(screen.getByText('40.0%')).toBeInTheDocument()
    expect(screen.getByText('Offered').nextElementSibling).toHaveTextContent('100')
  })

  it('hides swap deal stats when none offered', () => {
    setup({ adminData: admin({ swap_offered: 0 }) })
    expect(screen.queryByText('Swap Deals')).not.toBeInTheDocument()
  })

  describe('payouts', () => {
    it('lists pending payouts with pluralised swap counts', () => {
      setup({ payouts: [payout(), payout({ user_id: 10, name: '', email: '', swap_count: 3 })] })
      expect(screen.getByText('Pending Payouts')).toBeInTheDocument()
      expect(screen.getByText('Pat — $12.50')).toBeInTheDocument()
      expect(screen.getByText(/pat@example.com · 1 swap · since/)).toBeInTheDocument()
      expect(screen.getByText(/No contact · 3 swaps · since/)).toBeInTheDocument()
      expect(screen.getByText('No name — $12.50')).toBeInTheDocument()
    })

    it('has no payouts section when empty', () => {
      setup()
      expect(screen.queryByText('Pending Payouts')).not.toBeInTheDocument()
    })

    it('Mark Paid posts the user once, removes the row and toasts', async () => {
      const f = stubFetch(() => ({ ok: true }))
      const p = setup({ payouts: [payout()] })
      fireEvent.click(screen.getByRole('button', { name: 'Mark Paid' }))
      await waitFor(() => expect(p.onPayoutsChange).toHaveBeenCalledTimes(1))
      expect(f).toHaveBeenCalledTimes(1)
      expect(f).toHaveBeenCalledWith('/api/admin/payouts', expect.objectContaining({ method: 'POST', body: JSON.stringify({ user_id: 9 }) }))
      const updater = vi.mocked(p.onPayoutsChange).mock.calls[0][0]
      expect(updater([payout(), payout({ user_id: 10 })])).toEqual([payout({ user_id: 10 })])
      expect(p.showToast).toHaveBeenCalledWith('Marked $12.50 as paid for Pat')
    })

    it('a double-click on Mark Paid sends one request', async () => {
      let release: (v: Response) => void = () => {}
      const f = vi.fn(() => new Promise<Response>(r => { release = r }))
      vi.stubGlobal('fetch', f)
      setup({ payouts: [payout()] })
      const btn = screen.getByRole('button', { name: 'Mark Paid' })
      fireEvent.click(btn)
      fireEvent.click(btn)
      expect(f).toHaveBeenCalledTimes(1)
      expect(screen.getByRole('button', { name: 'Marking…' })).toBeDisabled()
      release({ ok: true } as Response)
      await waitFor(() => expect(screen.queryByRole('button', { name: 'Marking…' })).toBeNull())
    })

    it('a failed Mark Paid toasts an error and keeps the row', async () => {
      stubFetch(() => ({ ok: false }))
      const p = setup({ payouts: [payout()] })
      fireEvent.click(screen.getByRole('button', { name: 'Mark Paid' }))
      await waitFor(() => expect(p.showToast).toHaveBeenCalledWith('Failed to mark as paid', 'error'))
      expect(p.onPayoutsChange).not.toHaveBeenCalled()
    })
  })

  describe('deal list', () => {
    it('shows each deal with money formatted from cents and its status', () => {
      setup({ deals: [deal()] })
      expect(screen.getByText(/Brand X Milk → Kroger Milk/)).toBeInTheDocument()
      expect(screen.getByText('$0.50 off · Budget: $100.00 · Spent: $25.00 · active')).toBeInTheDocument()
    })

    it('badges non-default deal types only', () => {
      setup({ deals: [deal({ id: 1 }), deal({ id: 2, deal_type: 'bogo' })] })
      expect(screen.getByText('bogo')).toBeInTheDocument()
      expect(screen.queryByText('sku_swap')).not.toBeInTheDocument()
    })

    it('Deep Dive passes the deal id once', () => {
      const p = setup({ deals: [deal({ id: 4 })] })
      fireEvent.click(screen.getByRole('button', { name: 'Deep Dive' }))
      expect(p.onDeepDive).toHaveBeenCalledTimes(1)
      expect(p.onDeepDive).toHaveBeenCalledWith(4)
    })

    it('Pause sends the flipped status then refreshes the list', async () => {
      const refreshed = [deal({ status: 'paused' })]
      const f = stubFetch((_u, init) => ({ ok: true, body: init?.method === 'PUT' ? {} : refreshed }))
      const p = setup({ deals: [deal()] })
      fireEvent.click(screen.getByRole('button', { name: 'Pause' }))
      await waitFor(() => expect(p.onDealsChange).toHaveBeenCalledWith(refreshed))
      const put = f.mock.calls.find(c => c[1]?.method === 'PUT')!
      expect(JSON.parse(put[1]!.body as string).status).toBe('paused')
      expect(p.onDealsChange).toHaveBeenCalledTimes(1)
    })

    it('a paused deal offers Resume and sends active', async () => {
      const f = stubFetch(() => ({ ok: true, body: [] }))
      setup({ deals: [deal({ status: 'paused' })] })
      fireEvent.click(screen.getByRole('button', { name: 'Resume' }))
      await waitFor(() => expect(f).toHaveBeenCalledTimes(2))
      expect(JSON.parse(f.mock.calls[0][1]!.body as string).status).toBe('active')
    })

    it('Del asks first, then sends the delete once and refreshes', async () => {
      const f = stubFetch(() => ({ ok: true, body: [] }))
      const p = setup({ deals: [deal({ id: 8 })] })
      fireEvent.click(screen.getByRole('button', { name: 'Del' }))
      expect(f.mock.calls.filter(c => c[1]?.method === 'DELETE')).toHaveLength(0)
      const confirm = screen.getByRole('button', { name: 'Confirm delete' })
      fireEvent.click(confirm)
      fireEvent.click(confirm)
      await waitFor(() => expect(p.onDealsChange).toHaveBeenCalledWith([]))
      const del = f.mock.calls.filter(c => c[1]?.method === 'DELETE')
      expect(del).toHaveLength(1)
      expect(JSON.parse(del[0][1]!.body as string)).toEqual({ id: 8 })
    })

    it('Cancel after Del keeps the deal and sends nothing', () => {
      const f = stubFetch(() => ({ ok: true, body: [] }))
      setup({ deals: [deal({ id: 8 })] })
      fireEvent.click(screen.getByRole('button', { name: 'Del' }))
      fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
      expect(screen.getByRole('button', { name: 'Del' })).toBeInTheDocument()
      expect(f).not.toHaveBeenCalled()
    })

    it('does not replace the list when the refresh fails', async () => {
      stubFetch((_u, init) => ({ ok: init?.method === 'PUT', body: [] }))
      const p = setup({ deals: [deal()] })
      fireEvent.click(screen.getByRole('button', { name: 'Pause' }))
      await new Promise(r => setTimeout(r, 20))
      expect(p.onDealsChange).not.toHaveBeenCalled()
    })
  })

  describe('deal form', () => {
    it('is hidden until + Add, and Cancel hides it', () => {
      setup()
      expect(screen.queryByText('Deal Type')).not.toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: '+ Add' }))
      expect(screen.getByText('Deal Type')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
      expect(screen.queryByText('Deal Type')).not.toBeInTheDocument()
    })

    it('shows type-specific fields', () => {
      setup()
      fireEvent.click(screen.getByRole('button', { name: '+ Add' }))
      const select = screen.getByRole('combobox')
      expect(screen.queryByText('Quantity Offered')).not.toBeInTheDocument()
      fireEvent.change(select, { target: { value: 'bogo' } })
      expect(screen.getByText('Quantity Offered')).toBeInTheDocument()
      fireEvent.change(select, { target: { value: 'trial' } })
      expect(screen.getByText('Trial Product Name')).toBeInTheDocument()
      expect(screen.getByText('Trial Size')).toBeInTheDocument()
      fireEvent.change(select, { target: { value: 'seasonal' } })
      expect(screen.getByText('Campaign Tag')).toBeInTheDocument()
    })

    it('Create posts the form, refreshes the list, closes and toasts', async () => {
      const refreshed = [deal()]
      const f = stubFetch((_u, init) => ({ ok: true, body: init?.method === 'POST' ? {} : refreshed }))
      const p = setup()
      fireEvent.click(screen.getByRole('button', { name: '+ Add' }))
      const field = (label: string) => screen.getByText(label).nextElementSibling as HTMLInputElement
      fireEvent.change(field('Offer Brand'), { target: { value: 'Kroger' } })
      fireEvent.change(field('Discount (cents)'), { target: { value: '75' } })
      fireEvent.click(screen.getByRole('button', { name: 'Create' }))
      await waitFor(() => expect(p.showToast).toHaveBeenCalledWith('Deal created'))
      const post = f.mock.calls.find(c => c[1]?.method === 'POST')!
      expect(f.mock.calls.filter(c => c[1]?.method === 'POST')).toHaveLength(1)
      expect(JSON.parse(post[1]!.body as string)).toMatchObject({ offer_brand: 'Kroger', discount_cents: 75, deal_type: 'sku_swap' })
      expect(p.onDealsChange).toHaveBeenCalledWith(refreshed)
      expect(screen.queryByText('Deal Type')).not.toBeInTheDocument()
    })

    it('a failed save toasts an error and keeps the form open', async () => {
      stubFetch(() => ({ ok: false }))
      const p = setup()
      fireEvent.click(screen.getByRole('button', { name: '+ Add' }))
      fireEvent.click(screen.getByRole('button', { name: 'Create' }))
      await waitFor(() => expect(p.showToast).toHaveBeenCalledWith('Failed to save deal', 'error'))
      expect(p.onDealsChange).not.toHaveBeenCalled()
      expect(screen.getByText('Deal Type')).toBeInTheDocument()
    })

    it('Edit pre-fills the form and Update sends a PUT with the id', async () => {
      const f = stubFetch(() => ({ ok: true, body: [] }))
      const p = setup({ deals: [deal({ id: 3, offer_brand: 'Aldi' })] })
      fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
      const brand = screen.getByText('Offer Brand').nextElementSibling as HTMLInputElement
      expect(brand).toHaveValue('Aldi')
      fireEvent.click(screen.getByRole('button', { name: 'Update' }))
      await waitFor(() => expect(p.showToast).toHaveBeenCalledWith('Deal updated'))
      const put = f.mock.calls.find(c => c[1]?.method === 'PUT')!
      expect(JSON.parse(put[1]!.body as string)).toMatchObject({ id: 3, offer_brand: 'Aldi' })
    })

    it('+ Add after Edit clears the form', () => {
      setup({ deals: [deal({ offer_brand: 'Aldi' })] })
      fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
      fireEvent.click(screen.getByRole('button', { name: '+ Add' }))
      expect((screen.getByText('Offer Brand').nextElementSibling as HTMLInputElement).value).toBe('')
      expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument()
    })
  })

  describe('item analytics', () => {
    const analytics: AnalyticsData = {
      items_created: 55,
      items_by_source: { web: 5, email: 20 },
      items_by_department: Object.fromEntries(Array.from({ length: 12 }, (_, i) => [`Dept${i}`, i + 1])),
      events_by_action: { add: 3, remove: 7 },
      events_by_day: Array.from({ length: 20 }, (_, i) => ({ date: `2026-10-${String(i + 1).padStart(2, '0')}`, count: i })),
    }

    it('is absent without analytics data', () => {
      setup()
      expect(screen.queryByText('Item Analytics')).not.toBeInTheDocument()
    })

    it('shows totals, sorted breakdowns, top 10 departments and the last 14 days', () => {
      setup({ analyticsData: analytics })
      expect(screen.getByText('Items Created').nextElementSibling).toHaveTextContent('55')
      expect(screen.getByText('Total Events').nextElementSibling).toHaveTextContent('10')
      const bySource = screen.getByText('By Source').parentElement!
      const rows = within(bySource).getAllByText(/^(web|email)$/).map(e => e.textContent)
      expect(rows).toEqual(['email', 'web'])
      expect(screen.getByText('Dept11')).toBeInTheDocument()
      expect(screen.queryByText('Dept1')).not.toBeInTheDocument()
      expect(screen.queryByText('2026-10-06')).not.toBeInTheDocument()
      expect(screen.getByText('2026-10-07')).toBeInTheDocument()
      expect(screen.getByText('2026-10-20')).toBeInTheDocument()
    })

    it('omits empty breakdown sections', () => {
      setup({ analyticsData: { items_created: 0, items_by_source: {}, items_by_department: {}, events_by_action: {}, events_by_day: [] } })
      expect(screen.getByText('Item Analytics')).toBeInTheDocument()
      for (const t of ['By Source', 'Top Departments', 'Events by Action', 'Daily Activity']) {
        expect(screen.queryByText(t)).not.toBeInTheDocument()
      }
    })
  })
})
