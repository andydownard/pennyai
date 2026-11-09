import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'

vi.mock('../api', () => ({ fetchAPI: vi.fn(), fetchAction: vi.fn() }))
vi.mock('../haptics', () => ({ haptic: { tap: vi.fn(), double: vi.fn(), heavy: vi.fn() } }))

import { fetchAPI, fetchAction } from '../api'
import { useListState } from './useListState'
import type { ListItem } from '../components/ItemRow'

const api = vi.mocked(fetchAPI)
const action = vi.mocked(fetchAction)

function mk(id: number, name: string, extra: Partial<ListItem> = {}): ListItem {
  return { id, name, brand: 'B' + id, department: 'Dairy', quantity: 1, size: '', notes: '', checked: false, urgent: false, claimed_by_id: null, claimed_by_name: '', ...extra } as unknown as ListItem
}

function setup(over: Record<string, unknown> = {}) {
  const opts = {
    currentUserId: 7,
    showToast: vi.fn(), showChimeIn: vi.fn(), typePennyHome: vi.fn(),
    setSwapsByItemId: vi.fn(),
    lastAdded: [] as ListItem[], setLastAdded: vi.fn(),
    lastSkipped: [] as never[], setLastSkipped: vi.fn(),
    setShowReviewSheet: vi.fn(),
    setShowDoneToast: vi.fn(), setShowFireworks: vi.fn(), setShowDoneOverlay: vi.fn(),
    setPennyChimeIn: vi.fn(),
    pushModalState: vi.fn(), popModalState: vi.fn(),
    ...over,
  }
  const hook = renderHook(() => useListState(opts as never))
  return { opts, ...hook }
}

async function load(r: ReturnType<typeof setup>, list: unknown, swaps: unknown = []) {
  api.mockImplementation(async (url: string) => (url === '/api/list' ? list : url === '/api/list/swaps' ? swaps : null) as never)
  await act(async () => { await r.result.current.reloadList() })
}

function setOnline(v: boolean) {
  Object.defineProperty(navigator, 'onLine', { value: v, configurable: true })
}

beforeEach(() => {
  api.mockReset(); action.mockReset()
  localStorage.clear(); sessionStorage.clear()
  setOnline(true)
})
afterEach(() => { vi.useRealTimers(); setOnline(true) })

describe('useListState load', () => {
  it('starts empty and loading', () => {
    const r = setup()
    expect(r.result.current.items).toEqual([])
    expect(r.result.current.listLoading).toBe(true)
    expect(r.result.current.selectedItem).toBeNull()
  })

  it('loads items sorted by name, expands every department, ends loading', async () => {
    const r = setup()
    await load(r, [mk(2, 'Milk'), mk(1, 'Apples', { department: 'Produce' })])
    expect(r.result.current.items.map(i => i.name)).toEqual(['Apples', 'Milk'])
    expect([...r.result.current.expandedDepts].sort()).toEqual(['Dairy', 'Produce'])
    expect(r.result.current.listLoading).toBe(false)
  })

  it("keeps the shopper's expand/collapse choices on reload", async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk')])
    act(() => r.result.current.handleDeptClick('Dairy'))
    act(() => r.result.current.handleDeptClick('Bakery'))
    await load(r, [mk(1, 'Milk')])
    expect([...r.result.current.expandedDepts]).toEqual(['Bakery'])
  })

  it('treats a non-array list as empty', async () => {
    const r = setup()
    await load(r, { oops: true })
    expect(r.result.current.items).toEqual([])
    expect(r.result.current.listLoading).toBe(false)
  })

  it('restores pending deals keyed by item id', async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk')], [{ item_id: 1, deal_id: 9 }, { item_id: 4, deal_id: 10 }])
    expect(r.opts.setSwapsByItemId).toHaveBeenCalledTimes(1)
    const updater = r.opts.setSwapsByItemId.mock.calls[0][0] as unknown as (p: Record<number, unknown>) => Record<number, unknown>
    expect(updater({ 8: { item_id: 8 } })).toEqual({ 8: { item_id: 8 }, 1: { item_id: 1, deal_id: 9 }, 4: { item_id: 4, deal_id: 10 } })
  })

  it('a deals failure never blocks the list', async () => {
    const r = setup()
    api.mockImplementation((url: string) => (url === '/api/list' ? Promise.resolve([mk(1, 'Milk')]) : Promise.reject(new Error('x'))) as never)
    await act(async () => { await r.result.current.reloadList() })
    expect(r.result.current.items).toHaveLength(1)
    expect(r.opts.setSwapsByItemId).not.toHaveBeenCalled()
  })

  it('a null list leaves items alone but stops loading', async () => {
    const r = setup()
    await load(r, null)
    expect(r.result.current.items).toEqual([])
    expect(r.result.current.listLoading).toBe(false)
  })
})

describe('useListState detail sheet', () => {
  it('openDetail fills the form and loads history', async () => {
    const r = setup()
    const item = mk(1, 'Milk', { quantity: 0 as never, notes: undefined as never })
    api.mockResolvedValue([{ id: 1 }] as never)
    await act(async () => { r.result.current.openDetail(item) })
    expect(r.opts.pushModalState).toHaveBeenCalled()
    expect(r.result.current.selectedItem).toEqual(item)
    expect(r.result.current.editForm).toMatchObject({ name: 'Milk', brand: 'B1', dept: 'Dairy', qty: 1, notes: '' })
    expect(r.result.current.displayBrand).toBe('B1')
    await waitFor(() => expect(r.result.current.itemHistory).toEqual([{ id: 1 }]))
    expect(api).toHaveBeenCalledWith('/api/list/1/history')
  })

  it('history that is not an array becomes empty; failure is swallowed', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const r = setup()
    api.mockResolvedValue({ nope: 1 } as never)
    await act(async () => { r.result.current.openDetail(mk(1, 'Milk')) })
    await waitFor(() => expect(api).toHaveBeenCalled())
    expect(r.result.current.itemHistory).toEqual([])
    api.mockRejectedValue(new Error('x'))
    await act(async () => { r.result.current.openDetail(mk(1, 'Milk')) })
    await waitFor(() => expect(warn).toHaveBeenCalled())
    warn.mockRestore()
  })

  it('closeEditSheet pops modal state and clears selection', async () => {
    const r = setup()
    api.mockResolvedValue([] as never)
    await act(async () => { r.result.current.openDetail(mk(1, 'Milk')) })
    act(() => r.result.current.closeEditSheet())
    expect(r.opts.popModalState).toHaveBeenCalled()
    expect(r.result.current.selectedItem).toBeNull()
    expect(r.opts.setShowReviewSheet).not.toHaveBeenCalled()
  })

  it('opening from review hides the sheet; closing reopens it with the edit applied', async () => {
    vi.useFakeTimers()
    const added = [mk(1, 'Milk'), mk(2, 'Eggs')]
    const r = setup({ lastAdded: added })
    api.mockResolvedValue([] as never)
    await act(async () => { r.result.current.openDetailFromReview(added[0]) })
    expect(r.opts.setShowReviewSheet).toHaveBeenCalledWith(false)
    expect(r.result.current.returnToReview).toBe(true)
    const updated = mk(1, 'Milk 2%')
    act(() => r.result.current.closeEditSheet(undefined, updated))
    const upd = r.opts.setLastAdded.mock.calls[0][0] as unknown as (p: ListItem[]) => ListItem[]
    expect(upd(added)).toEqual([updated, added[1]])
    expect(r.result.current.returnToReview).toBe(false)
    act(() => { vi.advanceTimersByTime(100) })
    expect(r.opts.setShowReviewSheet).toHaveBeenLastCalledWith(true)
  })

  it('deleting from review drops the item from added and skipped, and does not reopen when nothing is left', async () => {
    vi.useFakeTimers()
    const added = [mk(1, 'Milk')]
    const r = setup({ lastAdded: added, lastSkipped: [{ existing_id: 1 }] })
    api.mockResolvedValue([] as never)
    await act(async () => { r.result.current.openDetailFromReview(added[0]) })
    r.opts.setShowReviewSheet.mockClear()
    act(() => r.result.current.closeEditSheet(1))
    const a = r.opts.setLastAdded.mock.calls[0][0] as unknown as (p: ListItem[]) => ListItem[]
    expect(a(added)).toEqual([])
    expect(a([])).toEqual([])
    const s = r.opts.setLastSkipped.mock.calls[0][0] as unknown as (p: { existing_id: number }[]) => unknown[]
    expect(s([{ existing_id: 1 }, { existing_id: 2 }])).toEqual([{ existing_id: 2 }])
    act(() => { vi.advanceTimersByTime(200) })
    expect(r.opts.setShowReviewSheet).not.toHaveBeenCalled()
  })

  it('deleting one of several from review reopens the sheet (skipped remain)', async () => {
    vi.useFakeTimers()
    const added = [mk(1, 'Milk')]
    const r = setup({ lastAdded: added, lastSkipped: [{ existing_id: 5 }] })
    api.mockResolvedValue([] as never)
    await act(async () => { r.result.current.openDetailFromReview(added[0]) })
    act(() => r.result.current.closeEditSheet(1))
    act(() => { vi.advanceTimersByTime(100) })
    expect(r.opts.setShowReviewSheet).toHaveBeenLastCalledWith(true)
  })
})

describe('useListState edit save', () => {
  async function open(r: ReturnType<typeof setup>, item: ListItem) {
    api.mockResolvedValue([] as never)
    await act(async () => { r.result.current.openDetail(item) })
  }

  it('ignores save without a selection or with a blank name', async () => {
    const r = setup()
    await act(async () => { await r.result.current.handleEditSave() })
    expect(api).not.toHaveBeenCalled()
    await load(r, [mk(1, 'Milk')])
    await open(r, mk(1, 'Milk'))
    act(() => r.result.current.setEditForm({ ...r.result.current.editForm, name: '   ' }))
    api.mockClear()
    await act(async () => { await r.result.current.handleEditSave() })
    expect(api).not.toHaveBeenCalled()
  })

  it('applies optimistically, PATCHes trimmed fields, then uses the server item', async () => {
    const r = setup()
    const milk = mk(1, 'Milk')
    await load(r, [milk])
    await open(r, milk)
    act(() => r.result.current.setEditForm({ ...r.result.current.editForm, name: ' Oat Milk ', brand: ' Oatly ', qty: 3 }))
    api.mockClear()
    api.mockResolvedValue(mk(1, 'Oat Milk (server)') as never)
    await act(async () => { await r.result.current.handleEditSave() })
    const [url, init] = api.mock.calls[0] as unknown as [string, { method: string; body: string }]
    expect(url).toBe('/api/list/1')
    expect(init.method).toBe('PATCH')
    expect(JSON.parse(init.body)).toMatchObject({ name: 'Oat Milk', brand: 'Oatly', quantity: 3 })
    expect(r.result.current.items[0].name).toBe('Oat Milk (server)')
    expect(r.result.current.selectedItem).toBeNull()
    expect(r.opts.showToast).not.toHaveBeenCalled()
  })

  it('rolls back and toasts when the server answers null', async () => {
    const r = setup()
    const milk = mk(1, 'Milk')
    await load(r, [milk])
    await open(r, milk)
    act(() => r.result.current.setEditForm({ ...r.result.current.editForm, name: 'Changed' }))
    api.mockResolvedValue(null as never)
    await act(async () => { await r.result.current.handleEditSave() })
    expect(r.result.current.items[0].name).toBe('Milk')
    expect(r.opts.showToast).toHaveBeenCalledWith(expect.stringContaining('save changes'), 'error')
  })

  it('rolls back and toasts when the request throws', async () => {
    const r = setup()
    const milk = mk(1, 'Milk')
    await load(r, [milk])
    await open(r, milk)
    act(() => r.result.current.setEditForm({ ...r.result.current.editForm, name: 'Changed' }))
    api.mockRejectedValue(new Error('net'))
    await act(async () => { await r.result.current.handleEditSave() })
    expect(r.result.current.items[0].name).toBe('Milk')
    expect(r.opts.showToast).toHaveBeenCalledWith(expect.stringContaining('save changes'), 'error')
  })
})

describe('useListState delete and clear', () => {
  it('handleEditDelete ignores no selection', async () => {
    const r = setup()
    await act(async () => { await r.result.current.handleEditDelete() })
    expect(action).not.toHaveBeenCalled()
  })

  it('handleEditDelete removes optimistically and keeps it removed on success', async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk'), mk(2, 'Eggs')])
    api.mockResolvedValue([] as never)
    await act(async () => { r.result.current.openDetail(mk(1, 'Milk')) })
    action.mockResolvedValue({ ok: true, status: 200 } as never)
    await act(async () => { await r.result.current.handleEditDelete() })
    expect(action).toHaveBeenCalledWith('/api/list/1', { method: 'DELETE' })
    expect(r.result.current.items.map(i => i.id)).toEqual([2])
    expect(r.opts.showToast).not.toHaveBeenCalled()
  })

  it('handleEditDelete rolls back on failure and throw, but not on 401', async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk')])
    api.mockResolvedValue([] as never)
    await act(async () => { r.result.current.openDetail(mk(1, 'Milk')) })
    action.mockResolvedValue({ ok: false, status: 500 } as never)
    await act(async () => { await r.result.current.handleEditDelete() })
    expect(r.result.current.items).toHaveLength(1)
    expect(r.opts.showToast).toHaveBeenCalledTimes(1)

    await act(async () => { r.result.current.openDetail(mk(1, 'Milk')) })
    action.mockRejectedValue(new Error('x'))
    await act(async () => { await r.result.current.handleEditDelete() })
    expect(r.result.current.items).toHaveLength(1)
    expect(r.opts.showToast).toHaveBeenCalledTimes(2)

    await act(async () => { r.result.current.openDetail(mk(1, 'Milk')) })
    action.mockResolvedValue({ ok: false, status: 401 } as never)
    await act(async () => { await r.result.current.handleEditDelete() })
    expect(r.result.current.items).toHaveLength(0)
    expect(r.opts.showToast).toHaveBeenCalledTimes(2)
  })

  it('handleSwipeDelete removes, rolls back on failure and throw, keeps removal on 401', async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk'), mk(2, 'Eggs')])
    action.mockResolvedValueOnce({ ok: true, status: 200 } as never)
    await act(async () => { await r.result.current.handleSwipeDelete(1) })
    expect(r.result.current.items.map(i => i.id)).toEqual([2])

    action.mockResolvedValueOnce({ ok: false, status: 500 } as never)
    await act(async () => { await r.result.current.handleSwipeDelete(2) })
    expect(r.result.current.items.map(i => i.id)).toEqual([2])
    expect(r.opts.showToast).toHaveBeenCalledTimes(1)

    action.mockRejectedValueOnce(new Error('x'))
    await act(async () => { await r.result.current.handleSwipeDelete(2) })
    expect(r.result.current.items.map(i => i.id)).toEqual([2])
    expect(r.opts.showToast).toHaveBeenCalledTimes(2)

    action.mockResolvedValueOnce({ ok: false, status: 401 } as never)
    await act(async () => { await r.result.current.handleSwipeDelete(2) })
    expect(r.result.current.items).toEqual([])
    expect(r.opts.showToast).toHaveBeenCalledTimes(2)
  })

  it('clear keeping unchecked items only removes checked ones', async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk', { checked: true }), mk(2, 'Eggs')])
    action.mockResolvedValue({ ok: true, status: 200 } as never)
    await act(async () => { await r.result.current.handleClearList() })
    expect(action).toHaveBeenCalledWith('/api/list/clear', { method: 'POST' })
    expect(r.result.current.items.map(i => i.id)).toEqual([2])
    expect(r.result.current.expandedDepts.size).toBe(0)
    expect(r.opts.setShowDoneToast).toHaveBeenCalledWith(false)
    expect(r.opts.setShowFireworks).toHaveBeenCalledWith(false)
    expect(r.opts.setPennyChimeIn).toHaveBeenCalledWith(null)
    expect(r.opts.typePennyHome).toHaveBeenCalledWith(expect.stringContaining('List cleared'))
  })

  it('clear all empties the list; failure restores it', async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk'), mk(2, 'Eggs')])
    action.mockResolvedValueOnce({ ok: true, status: 200 } as never)
    await act(async () => { await r.result.current.handleClearList(true) })
    expect(action).toHaveBeenLastCalledWith('/api/list/clear?all=true', { method: 'POST' })
    expect(r.result.current.items).toEqual([])

    await load(r, [mk(1, 'Milk')])
    action.mockResolvedValueOnce({ ok: false, status: 500 } as never)
    await act(async () => { await r.result.current.handleClearList(true) })
    expect(r.result.current.items).toHaveLength(1)
    expect(r.opts.showToast).toHaveBeenCalledWith(expect.stringContaining('clear your list'), 'error')
  })

  it('clear does not roll back on 401', async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk')])
    action.mockResolvedValueOnce({ ok: false, status: 401 } as never)
    await act(async () => { await r.result.current.handleClearList(true) })
    expect(r.result.current.items).toEqual([])
    expect(r.opts.showToast).not.toHaveBeenCalled()
  })
})

describe('useListState toggle', () => {
  it('flips optimistically then takes the server item', async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk'), mk(2, 'Eggs')])
    api.mockResolvedValue(mk(1, 'Milk', { checked: true, brand: 'Server' }) as never)
    await act(async () => { await r.result.current.handleToggle(1) })
    expect(api).toHaveBeenLastCalledWith('/api/list/1/toggle', { method: 'PATCH' })
    expect(r.result.current.items.find(i => i.id === 1)).toMatchObject({ checked: true, brand: 'Server' })
  })

  it('reverts and toasts on null and on throw', async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk')])
    api.mockResolvedValueOnce(null as never)
    await act(async () => { await r.result.current.handleToggle(1) })
    expect(r.result.current.items[0].checked).toBe(false)
    expect(r.opts.showToast).toHaveBeenCalledTimes(1)
    api.mockRejectedValueOnce(new Error('x'))
    await act(async () => { await r.result.current.handleToggle(1) })
    expect(r.result.current.items[0].checked).toBe(false)
    expect(r.opts.showToast).toHaveBeenCalledTimes(2)
  })

  it('celebrates once the last item is checked', async () => {
    vi.useFakeTimers()
    const r = setup()
    await act(async () => {
      r.result.current.setItems([mk(1, 'Milk', { checked: true }), mk(2, 'Eggs')])
    })
    api.mockResolvedValue(mk(2, 'Eggs', { checked: true }) as never)
    await act(async () => { await r.result.current.handleToggle(2) })
    expect(r.opts.setShowDoneToast).not.toHaveBeenCalled()
    act(() => { vi.advanceTimersByTime(300) })
    expect(r.opts.setShowDoneToast).toHaveBeenCalledWith(true)
    expect(r.opts.setShowFireworks).toHaveBeenCalledWith(true)
    expect(r.opts.setShowDoneOverlay).toHaveBeenCalledWith(true)
  })

  it('checkAllDone does nothing for an empty or partly checked list', () => {
    vi.useFakeTimers()
    const r = setup()
    r.result.current.checkAllDone([])
    r.result.current.checkAllDone([mk(1, 'A', { checked: true }), mk(2, 'B')])
    vi.advanceTimersByTime(500)
    expect(r.opts.setShowDoneToast).not.toHaveBeenCalled()
  })
})

describe('useListState offline queue', () => {
  it('queues toggles offline without calling the server, cancels double toggles', async () => {
    setOnline(false)
    const r = setup()
    await load(r, [mk(1, 'Milk'), mk(2, 'Eggs')])
    api.mockClear()
    await act(async () => { await r.result.current.handleToggle(1) })
    expect(api).not.toHaveBeenCalled()
    expect(r.result.current.pendingToggleCount).toBe(1)
    expect(r.result.current.items.find(i => i.id === 1)?.checked).toBe(true)
    expect(r.opts.typePennyHome).toHaveBeenCalledTimes(1)

    await act(async () => { await r.result.current.handleToggle(2) })
    expect(r.result.current.pendingToggleCount).toBe(2)
    expect(r.opts.typePennyHome).toHaveBeenCalledTimes(1) // no message storm

    await act(async () => { await r.result.current.handleToggle(1) })
    expect(r.result.current.pendingToggleCount).toBe(1)
    expect(r.result.current.items.find(i => i.id === 1)?.checked).toBe(false)
  })

  it('replays the queue once on reconnect, each toggle sent once', async () => {
    setOnline(false)
    const r = setup()
    await load(r, [mk(1, 'Milk'), mk(2, 'Eggs')])
    await act(async () => { await r.result.current.handleToggle(1) })
    await act(async () => { await r.result.current.handleToggle(2) })
    api.mockClear()
    api.mockImplementation(async (url: string) => mk(Number(url.split('/')[3]), 'x', { checked: true }) as never)
    setOnline(true)
    await act(async () => { window.dispatchEvent(new Event('online')) })
    await waitFor(() => expect(r.result.current.pendingToggleCount).toBe(0))
    expect(api).toHaveBeenCalledTimes(2)
    expect(api.mock.calls.map(c => c[0]).sort()).toEqual(['/api/list/1/toggle', '/api/list/2/toggle'])
    expect(r.opts.typePennyHome).toHaveBeenLastCalledWith('All caught up!')
    // a second reconnect with an empty queue sends nothing
    await act(async () => { window.dispatchEvent(new Event('offline')); window.dispatchEvent(new Event('online')) })
    expect(api).toHaveBeenCalledTimes(2)
  })

  it('keeps failed toggles queued and reports them', async () => {
    setOnline(false)
    const r = setup()
    await load(r, [mk(1, 'Milk')])
    await act(async () => { await r.result.current.handleToggle(1) })
    api.mockRejectedValue(new Error('x'))
    await act(async () => { window.dispatchEvent(new Event('online')) })
    await waitFor(() => expect(r.opts.showToast).toHaveBeenCalled())
    expect(r.opts.showToast).toHaveBeenCalledWith(expect.stringContaining("1 change couldn't sync"), 'error')
    expect(r.result.current.pendingToggleCount).toBe(1)
  })

  it('treats a null replay answer as a failure and pluralises', async () => {
    setOnline(false)
    const r = setup()
    await load(r, [mk(1, 'Milk'), mk(2, 'Eggs')])
    await act(async () => { await r.result.current.handleToggle(1) })
    await act(async () => { await r.result.current.handleToggle(2) })
    api.mockResolvedValue(null as never)
    await act(async () => { window.dispatchEvent(new Event('online')) })
    await waitFor(() => expect(r.opts.showToast).toHaveBeenCalled())
    expect(r.opts.showToast).toHaveBeenCalledWith(expect.stringContaining("2 changes couldn't sync"), 'error')
  })
})

describe('useListState urgent and claim', () => {
  it('chimes sensibly even if the server returns no item name', async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk')])
    api.mockResolvedValueOnce(mk(1, '', { urgent: true }) as never)
    await act(async () => { await r.result.current.handleUrgentToggle(1) })
    expect(r.opts.showChimeIn).toHaveBeenLastCalledWith('Marked it as urgent!', 3000)
  })

  it('urgent flips, chimes with the server name, and reverts on failure', async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk')])
    api.mockResolvedValueOnce(mk(1, 'Milk', { urgent: true }) as never)
    await act(async () => { await r.result.current.handleUrgentToggle(1) })
    expect(api).toHaveBeenLastCalledWith('/api/list/1/urgent', { method: 'PATCH' })
    expect(r.result.current.items[0].urgent).toBe(true)
    expect(r.opts.showChimeIn).toHaveBeenLastCalledWith('Marked Milk as urgent!', 3000)

    api.mockResolvedValueOnce(mk(1, 'Milk', { urgent: false }) as never)
    await act(async () => { await r.result.current.handleUrgentToggle(1) })
    expect(r.opts.showChimeIn).toHaveBeenLastCalledWith('Milk back to normal', 3000)

    api.mockResolvedValueOnce(null as never)
    await act(async () => { await r.result.current.handleUrgentToggle(1) })
    expect(r.result.current.items[0].urgent).toBe(false)
    expect(r.opts.showToast).toHaveBeenCalledTimes(1)

    api.mockRejectedValueOnce(new Error('x'))
    await act(async () => { await r.result.current.handleUrgentToggle(1) })
    expect(r.result.current.items[0].urgent).toBe(false)
    expect(r.opts.showToast).toHaveBeenCalledTimes(2)
  })

  it('claim sets the claimer, chimes, and reverts on failure', async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk')])
    api.mockResolvedValueOnce(mk(1, 'Milk', { claimed_by_id: 7, claimed_by_name: 'Me' }) as never)
    await act(async () => { await r.result.current.handleClaim(1) })
    expect(api).toHaveBeenLastCalledWith('/api/list/1/claim', { method: 'PATCH' })
    expect(r.result.current.items[0].claimed_by_id).toBe(7)
    expect(r.opts.showChimeIn).toHaveBeenLastCalledWith('You claimed Milk!', 3000)

    api.mockResolvedValueOnce(mk(1, 'Milk', { claimed_by_id: null }) as never)
    await act(async () => { await r.result.current.handleClaim(1) })
    expect(r.opts.showChimeIn).toHaveBeenLastCalledWith('Unclaimed Milk', 3000)
    expect(r.result.current.items[0].claimed_by_id).toBeNull()

    api.mockResolvedValueOnce(null as never)
    await act(async () => { await r.result.current.handleClaim(1) })
    expect(r.result.current.items[0].claimed_by_id).toBeNull()
    expect(r.opts.showToast).toHaveBeenCalledTimes(1)

    api.mockRejectedValueOnce(new Error('x'))
    await act(async () => { await r.result.current.handleClaim(1) })
    expect(r.result.current.items[0].claimed_by_id).toBeNull()
    expect(r.opts.showToast).toHaveBeenCalledTimes(2)
  })
})

describe('useListState departments and utilities', () => {
  it('handleDeptClick expands and collapses, scrolling to the opened department', () => {
    vi.useFakeTimers()
    const r = setup()
    const scroll = vi.fn()
    r.result.current.deptRefs.current['Dairy'] = { scrollIntoView: scroll } as unknown as HTMLDivElement
    act(() => r.result.current.handleDeptClick('Dairy'))
    expect(r.result.current.expandedDepts.has('Dairy')).toBe(true)
    act(() => { vi.advanceTimersByTime(1) })
    expect(scroll).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' })
    act(() => r.result.current.handleDeptClick('Dairy'))
    expect(r.result.current.expandedDepts.has('Dairy')).toBe(false)
  })

  it('updateItem patches only the matching item; sortItems does not mutate', async () => {
    const r = setup()
    await load(r, [mk(1, 'Milk'), mk(2, 'Eggs')])
    act(() => r.result.current.updateItem(2, { notes: 'free range' }))
    expect(r.result.current.items.find(i => i.id === 2)?.notes).toBe('free range')
    expect(r.result.current.items.find(i => i.id === 1)?.notes).toBe('')
    const input = [mk(1, 'b'), mk(2, 'a')]
    expect(r.result.current.sortItems(input).map(i => i.name)).toEqual(['a', 'b'])
    expect(input[0].name).toBe('b')
  })

  it('stops listening for online/offline after unmount', () => {
    const remove = vi.spyOn(window, 'removeEventListener')
    const r = setup()
    r.unmount()
    expect(remove).toHaveBeenCalledWith('online', expect.any(Function))
    expect(remove).toHaveBeenCalledWith('offline', expect.any(Function))
    remove.mockRestore()
  })
})
