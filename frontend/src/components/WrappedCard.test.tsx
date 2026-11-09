import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'

vi.mock('../api', () => ({ fetchAPI: vi.fn() }))
vi.mock('../analytics', () => ({ track: vi.fn() }))

import { fetchAPI } from '../api'
import { track } from '../analytics'
import { WrappedCard } from './WrappedCard'

const wrapped = {
  month: 'October 2026', total_savings_cents: 1234, swaps_count: 5, items_added: 22,
  top_swap_brand: 'Kroger', top_category: 'Dairy', streak_days: 3,
  share_text: 'I saved big', user_name: 'Dana',
}

function load(payload: unknown) {
  vi.mocked(fetchAPI).mockResolvedValue(payload as never)
}

describe('WrappedCard', () => {
  beforeEach(() => {
    vi.mocked(fetchAPI).mockReset()
    vi.mocked(track).mockReset()
  })
  afterEach(() => { vi.unstubAllGlobals() })

  it('shows a loading message while the recap is fetched', () => {
    vi.mocked(fetchAPI).mockReturnValue(new Promise(() => {}) as never)
    render(<WrappedCard onClose={vi.fn()} />)
    expect(screen.getByText('Loading your month...')).toBeInTheDocument()
  })

  it('renders the populated recap with every stat', async () => {
    load(wrapped)
    render(<WrappedCard onClose={vi.fn()} />)
    expect(await screen.findByText('October 2026')).toBeInTheDocument()
    expect(screen.getByText('Dana')).toBeInTheDocument()
    expect(screen.getByText('$12.34')).toBeInTheDocument()
    expect(screen.getByText('Smart Swaps')).toBeInTheDocument()
    expect(screen.getByText('22')).toBeInTheDocument()
    expect(screen.getByText('Day Streak')).toBeInTheDocument()
    expect(screen.getByText('Dairy')).toBeInTheDocument()
    expect(screen.getByText('Kroger')).toBeInTheDocument()
    expect(fetchAPI).toHaveBeenCalledWith('/api/wrapped')
    expect(track).toHaveBeenCalledWith('wrapped_viewed', { month: 'October 2026' })
  })

  it('hides optional stats when absent', async () => {
    load({ ...wrapped, streak_days: 0, top_category: '', top_swap_brand: '', user_name: '' })
    render(<WrappedCard onClose={vi.fn()} />)
    await screen.findByText('October 2026')
    expect(screen.queryByText('Day Streak')).not.toBeInTheDocument()
    expect(screen.queryByText('Top Category')).not.toBeInTheDocument()
    expect(screen.queryByText(/favorite swap/i)).not.toBeInTheDocument()
  })

  it('shows the empty state when there is no activity, and Close fires once', async () => {
    load({ ...wrapped, total_savings_cents: 0, swaps_count: 0, items_added: 0 })
    const onClose = vi.fn()
    render(<WrappedCard onClose={onClose} />)
    expect(await screen.findByText('No activity yet this month')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Share My Wrapped' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('shows the empty state when the request fails', async () => {
    vi.mocked(fetchAPI).mockRejectedValue(new Error('boom'))
    render(<WrappedCard onClose={vi.fn()} />)
    expect(await screen.findByText('No activity yet this month')).toBeInTheDocument()
    expect(track).not.toHaveBeenCalled()
  })

  it('treats an array payload as no data', async () => {
    load([])
    render(<WrappedCard onClose={vi.fn()} />)
    expect(await screen.findByText('No activity yet this month')).toBeInTheDocument()
  })

  it('Close on the populated card fires onClose once', async () => {
    load(wrapped)
    const onClose = vi.fn()
    render(<WrappedCard onClose={onClose} />)
    await screen.findByText('October 2026')
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  describe('sharing', () => {
    const fakeCtx = new Proxy({}, {
      get: (_t, p) => (p === 'createLinearGradient' ? () => ({ addColorStop: () => {} }) : () => {}),
      set: () => true,
    })
    beforeEach(() => {
      vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(fakeCtx as never)
      vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation(cb => cb(new Blob(['x'], { type: 'image/png' })))
      URL.createObjectURL = vi.fn(() => 'blob:x')
      URL.revokeObjectURL = vi.fn()
    })
    afterEach(() => vi.restoreAllMocks())

    it('downloads the image when native file share is unavailable', async () => {
      load(wrapped)
      const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
      render(<WrappedCard onClose={vi.fn()} />)
      fireEvent.click(await screen.findByRole('button', { name: 'Share My Wrapped' }))
      await waitFor(() => expect(click).toHaveBeenCalledTimes(1))
      expect(track).toHaveBeenCalledWith('wrapped_shared', { month: 'October 2026', savings: 1234 })
      expect(URL.createObjectURL).toHaveBeenCalledTimes(1)
    })

    it('uses native share with the file when supported', async () => {
      load(wrapped)
      const share = vi.fn().mockResolvedValue(undefined)
      vi.stubGlobal('navigator', { ...navigator, share, canShare: () => true })
      render(<WrappedCard onClose={vi.fn()} />)
      fireEvent.click(await screen.findByRole('button', { name: 'Share My Wrapped' }))
      await waitFor(() => expect(share).toHaveBeenCalledTimes(1))
      expect(share.mock.calls[0][0].text).toBe('I saved big')
      expect(share.mock.calls[0][0].files[0].name).toBe('groflo-wrapped.png')
    })

    it('falls back to text sharing when the image cannot be built', async () => {
      load(wrapped)
      vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation(cb => cb(null))
      const writeText = vi.fn()
      vi.stubGlobal('navigator', { clipboard: { writeText } })
      render(<WrappedCard onClose={vi.fn()} />)
      fireEvent.click(await screen.findByRole('button', { name: 'Share My Wrapped' }))
      await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1))
      expect(writeText.mock.calls[0][0]).toContain('I saved big')
      expect(writeText.mock.calls[0][0]).toContain(window.location.origin)
    })
  })
})
