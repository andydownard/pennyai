import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react'

vi.mock('./PennyAvatar', () => ({ PennyAvatar: () => null }))

import { SavedListsScreen } from './SavedListsScreen'

const iso = (msAgo: number) => new Date(Date.now() - msAgo).toISOString()
const MIN = 60_000

const lists = [
  { id: 1, name: 'Weekly Groceries', store_name: 'Kroger', item_count: 12, is_master: false, created_at: iso(MIN), updated_at: iso(5 * MIN) },
  { id: 2, name: 'Party', item_count: 1, is_master: false, created_at: iso(MIN), updated_at: iso(3 * 60 * MIN) },
]

function resp(ok: boolean, body: unknown) {
  return { ok, json: () => Promise.resolve(body) } as Response
}

function mockFetch(handler: (url: string, init?: RequestInit) => Response | Promise<Response>) {
  const f = vi.fn((url: string, init?: RequestInit) => Promise.resolve(handler(url, init)))
  vi.stubGlobal('fetch', f)
  return f
}

function setup(over: Partial<React.ComponentProps<typeof SavedListsScreen>> = {}) {
  const props = {
    renderHeader: () => <header>HEADER</header>,
    renderToast: () => <div>TOAST</div>,
    transitionStyle: {},
    onShowToast: vi.fn(),
    onNavigateHome: vi.fn(),
    ...over,
  }
  render(<SavedListsScreen {...props} />)
  return props
}

describe('SavedListsScreen', () => {
  afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers() })

  it('renders the header, toast slot and skeleton while loading', () => {
    vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
    setup()
    expect(screen.getByText('HEADER')).toBeInTheDocument()
    expect(screen.getByText('TOAST')).toBeInTheDocument()
    expect(screen.queryByText('No saved lists yet')).not.toBeInTheDocument()
  })

  it('shows the empty state and sends the user home', async () => {
    mockFetch(() => resp(true, []))
    const p = setup()
    expect(await screen.findByText('No saved lists yet')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Go to my list' }))
    expect(p.onNavigateHome).toHaveBeenCalledTimes(1)
  })

  it('treats a null body and a failed request as empty', async () => {
    mockFetch(() => resp(true, null))
    setup()
    expect(await screen.findByText('No saved lists yet')).toBeInTheDocument()
  })

  it('treats a network error as empty, not a crash', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new Error('offline'))))
    setup()
    expect(await screen.findByText('No saved lists yet')).toBeInTheDocument()
  })

  it('lists each saved list with store, count and age', async () => {
    mockFetch(() => resp(true, lists))
    setup()
    expect(await screen.findByText('Weekly Groceries')).toBeInTheDocument()
    expect(screen.getByText('Kroger')).toBeInTheDocument()
    expect(screen.getByText('12 items')).toBeInTheDocument()
    expect(screen.getByText('1 item')).toBeInTheDocument()
    expect(screen.getByText('5m ago')).toBeInTheDocument()
    expect(screen.getByText('3h ago')).toBeInTheDocument()
    expect(screen.getByText('Long-press a list to delete it')).toBeInTheDocument()
  })

  it('confirms before loading, then loads once, toasts and goes home', async () => {
    const f = mockFetch(url => (url === '/api/saved-lists' ? resp(true, lists) : resp(true, {})))
    const p = setup()
    fireEvent.click(await screen.findByRole('button', { name: /Weekly Groceries/ }))
    expect(screen.getByRole('alertdialog', { name: 'Load Weekly Groceries?' })).toBeInTheDocument()
    expect(f).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByRole('button', { name: 'Load' }))
    await waitFor(() => expect(p.onNavigateHome).toHaveBeenCalledTimes(1))
    expect(f).toHaveBeenCalledWith('/api/saved-lists/1/load', { method: 'POST' })
    expect(f.mock.calls.filter(c => String(c[0]).endsWith('/load'))).toHaveLength(1)
    expect(p.onShowToast).toHaveBeenCalledWith('Loaded Weekly Groceries!')
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('cancelling the load confirm sends nothing', async () => {
    const f = mockFetch(() => resp(true, lists))
    const p = setup()
    fireEvent.click(await screen.findByRole('button', { name: /Weekly Groceries/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(f).toHaveBeenCalledTimes(1)
    expect(p.onNavigateHome).not.toHaveBeenCalled()
  })

  it('Escape dismisses the confirm', async () => {
    mockFetch(() => resp(true, lists))
    setup()
    fireEvent.click(await screen.findByRole('button', { name: /Weekly Groceries/ }))
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('shows the server error when a load fails, and stays on the screen', async () => {
    mockFetch(url => (url === '/api/saved-lists' ? resp(true, lists) : resp(false, { error: 'List is empty' })))
    const p = setup()
    fireEvent.click(await screen.findByRole('button', { name: /Weekly Groceries/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Load' }))
    await waitFor(() => expect(p.onShowToast).toHaveBeenCalledWith('List is empty', 'error'))
    expect(p.onNavigateHome).not.toHaveBeenCalled()
  })

  it('falls back to a generic message when the error body is unreadable', async () => {
    vi.stubGlobal('fetch', vi.fn((url: string) => (url === '/api/saved-lists'
      ? Promise.resolve(resp(true, lists))
      : Promise.resolve({ ok: false, json: () => Promise.reject(new Error('bad')) } as Response))))
    const p = setup()
    fireEvent.click(await screen.findByRole('button', { name: /Weekly Groceries/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Load' }))
    await waitFor(() => expect(p.onShowToast).toHaveBeenCalledWith('Could not load list', 'error'))
  })

  it('reports a connection error when loading throws', async () => {
    vi.stubGlobal('fetch', vi.fn((url: string) => (url === '/api/saved-lists'
      ? Promise.resolve(resp(true, lists)) : Promise.reject(new Error('offline')))))
    const p = setup()
    fireEvent.click(await screen.findByRole('button', { name: /Weekly Groceries/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Load' }))
    await waitFor(() => expect(p.onShowToast).toHaveBeenCalledWith('Connection error', 'error'))
  })

  it('context menu opens a destructive delete confirm; confirming deletes once and removes the row', async () => {
    const f = mockFetch(url => (url === '/api/saved-lists' ? resp(true, lists) : resp(true, {})))
    const p = setup()
    fireEvent.contextMenu(await screen.findByRole('button', { name: /Weekly Groceries/ }))
    expect(screen.getByRole('alertdialog', { name: 'Delete Weekly Groceries?' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(screen.queryByText('Weekly Groceries')).not.toBeInTheDocument())
    expect(f).toHaveBeenCalledWith('/api/saved-lists/1', { method: 'DELETE' })
    expect(f.mock.calls.filter(c => (c[1] as RequestInit | undefined)?.method === 'DELETE')).toHaveLength(1)
    expect(p.onShowToast).toHaveBeenCalledWith('Deleted Weekly Groceries')
    expect(screen.getByText('Party')).toBeInTheDocument()
  })

  it('keeps the row and toasts an error when delete fails', async () => {
    mockFetch(url => (url === '/api/saved-lists' ? resp(true, lists) : resp(false, {})))
    const p = setup()
    fireEvent.contextMenu(await screen.findByRole('button', { name: /Weekly Groceries/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(p.onShowToast).toHaveBeenCalledWith('Could not delete list', 'error'))
    expect(screen.getByText('Weekly Groceries')).toBeInTheDocument()
  })

  it('reports a connection error when delete throws', async () => {
    vi.stubGlobal('fetch', vi.fn((url: string) => (url === '/api/saved-lists'
      ? Promise.resolve(resp(true, lists)) : Promise.reject(new Error('offline')))))
    const p = setup()
    fireEvent.contextMenu(await screen.findByRole('button', { name: /Weekly Groceries/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(p.onShowToast).toHaveBeenCalledWith('Connection error', 'error'))
  })

  it('a 500ms long press opens delete and the lifting tap does not open load', async () => {
    mockFetch(() => resp(true, lists))
    setup()
    const row = await screen.findByRole('button', { name: /Weekly Groceries/ })
    vi.useFakeTimers()
    fireEvent.touchStart(row)
    act(() => { vi.advanceTimersByTime(500) })
    expect(screen.getByRole('alertdialog', { name: 'Delete Weekly Groceries?' })).toBeInTheDocument()
    fireEvent.touchEnd(row)
    fireEvent.click(row)
    expect(screen.queryByRole('alertdialog', { name: /^Load/ })).not.toBeInTheDocument()
  })

  it('a short touch cancels the long press', async () => {
    mockFetch(() => resp(true, lists))
    setup()
    const row = await screen.findByRole('button', { name: /Weekly Groceries/ })
    vi.useFakeTimers()
    fireEvent.touchStart(row)
    act(() => { vi.advanceTimersByTime(200) })
    fireEvent.touchEnd(row)
    act(() => { vi.advanceTimersByTime(1000) })
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('never says bot, AI or assistant', async () => {
    mockFetch(() => resp(true, []))
    setup()
    await screen.findByText('No saved lists yet')
    expect(document.body.textContent).not.toMatch(/\b(bot|AI|assistant)\b/i)
  })

  beforeEach(() => { vi.useRealTimers() })
})
