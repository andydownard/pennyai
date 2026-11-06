import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useChatState } from './useChatState'
import type { ListItem } from '../components/ItemRow'

vi.mock('../api', () => ({ fetchAction: vi.fn(), fetchAPI: vi.fn() }))
vi.mock('../analytics', () => ({ track: vi.fn() }))
vi.mock('../ab', () => ({ trackEvent: vi.fn() }))

import { fetchAction, fetchAPI } from '../api'
import { track } from '../analytics'
import { trackEvent } from '../ab'

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Any = any

const mkItem = (id: number, over: Partial<ListItem> = {}): ListItem =>
  ({ id, name: `item${id}`, brand: '', quantity: 1, department: 'Produce', checked: false, ...over } as unknown as ListItem)

function makeOptions(over: Record<string, unknown> = {}) {
  return {
    items: [] as ListItem[],
    setItems: vi.fn(),
    sortItems: vi.fn((a: ListItem[]) => a),
    setExpandedDepts: vi.fn(),
    setNewItemIds: vi.fn(),
    newItemTimerRef: { current: null } as { current: ReturnType<typeof setTimeout> | null },
    deptRefs: { current: {} } as { current: Record<string, HTMLDivElement | null> },
    reloadList: vi.fn(() => Promise.resolve()),
    handleClearList: vi.fn(() => Promise.resolve()),
    setLastAdded: vi.fn(),
    setLastSkipped: vi.fn(),
    hasReviewedRef: { current: true },
    pennyDisambig: [] as Any[],
    setPennyDisambig: vi.fn(),
    pennyDisambigIndex: 0,
    setPennyDisambigIndex: vi.fn(),
    setSwapsByItemId: vi.fn(),
    advanceDisambig: vi.fn(),
    handleBrandPick: vi.fn(() => Promise.resolve()),
    setShowReviewSheet: vi.fn(),
    settingsFirstName: 'Sam',
    setSettingsFirstName: vi.fn(),
    settingsLastName: '',
    setSettingsLastName: vi.fn(),
    settingsEmail: 'sam@example.com',
    handleOpenInviteSheet: vi.fn(),
    loginCount: 5,
    handleSessionExpired: vi.fn(),
    handleLoadSavedList: vi.fn(() => Promise.resolve()),
    showToast: vi.fn(),
    setScreen: vi.fn(),
    pendingImages: [] as string[],
    setPendingImages: vi.fn(),
    setFloatingLabels: vi.fn(),
    setShowDoneToast: vi.fn(),
    setShowFireworks: vi.fn(),
    setPendingReceiptMode: vi.fn(),
    receiptInputRef: { current: null } as Any,
    setEarningsTotalCents: vi.fn(),
    ...over,
  }
}

function setup(over: Record<string, unknown> = {}) {
  const opts = makeOptions(over)
  const hook = renderHook(() => useChatState(opts as Any))
  return { opts, ...hook }
}

const jsonRes = (status: number, body: unknown) =>
  ({ status, ok: status >= 200 && status < 300, json: () => Promise.resolve(body) }) as Response

let fetchMock: ReturnType<typeof vi.fn>

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.useFakeTimers()
  vi.mocked(fetchAPI).mockResolvedValue(null)
  vi.mocked(fetchAction).mockResolvedValue({ ok: true, status: 200 })
  fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const typeOut = (msg: string) => act(() => { vi.advanceTimersByTime(25 * msg.length + 50) })
const run = (fn: () => void) => act(() => { fn() })

async function send(r: { current: ReturnType<typeof useChatState> }, text: string) {
  act(() => r.current.setChatText(text))
  await act(async () => { await r.current.handleSendChat() })
}

describe('initial state', () => {
  it('starts empty and idle', () => {
    const { result } = setup()
    expect(result.current.chatText).toBe('')
    expect(result.current.chatLoading).toBe(false)
    expect(result.current.pendingAdds).toEqual([])
    expect(result.current.homePennyText).toBe('')
    expect(result.current.homePennyTyping).toBe(false)
    expect(result.current.homePennyBtns).toEqual([])
    expect(result.current.pennyStatus).toBeNull()
    expect(result.current.nameInputMode.current).toBe(false)
    expect(result.current.hasGreetedEmpty.current).toBe(false)
    expect(result.current.hasGreetedWithItems.current).toBe(false)
    expect(result.current.onboardingNameAsked.current).toBe(false)
  })

  it('restores onboarding flags from localStorage', () => {
    localStorage.setItem('ob_name', '1')
    localStorage.setItem('ob_share', '1')
    localStorage.setItem('ob_swaps', '1')
    localStorage.setItem('ob_link_email', '1')
    const { result } = setup()
    expect(result.current.onboardingNameAsked.current).toBe(true)
    expect(result.current.onboardingShareMentioned.current).toBe(true)
    expect(result.current.onboardingSwapsMentioned.current).toBe(true)
    expect(result.current.onboardingLinkEmailMentioned.current).toBe(true)
  })

  it('syncs only the set onboarding flags to the server', () => {
    localStorage.setItem('ob_share', '1')
    const { result } = setup()
    run(() => result.current.syncOnboardingFlags())
    const [url, init] = vi.mocked(fetchAPI).mock.calls[0]
    expect(url).toBe('/api/settings')
    expect(init!.method).toBe('PATCH')
    const body = JSON.parse(init!.body as string)
    expect(JSON.parse(body.onboarding_flags)).toEqual({ ob_share: '1' })
  })
})

describe('typePennyHome (typewriter)', () => {
  it('types the message one letter at a time', () => {
    const { result } = setup()
    run(() => result.current.typePennyHome('Hi!'))
    expect(result.current.homePennyTyping).toBe(true)
    expect(result.current.homePennyText).toBe('')
    act(() => { vi.advanceTimersByTime(25) })
    expect(result.current.homePennyText).toBe('H')
    act(() => { vi.advanceTimersByTime(25) })
    expect(result.current.homePennyText).toBe('Hi')
    act(() => { vi.advanceTimersByTime(25) })
    expect(result.current.homePennyText).toBe('Hi!')
    expect(result.current.homePennyTyping).toBe(false)
    expect(result.current.homePennyInterval.current).toBeNull()
  })

  it('does not retype the same message when there are no buttons', () => {
    const { result } = setup()
    run(() => result.current.typePennyHome('Hi!'))
    typeOut('Hi!')
    run(() => result.current.typePennyHome('Hi!'))
    expect(result.current.homePennyTyping).toBe(false)
    expect(result.current.homePennyText).toBe('Hi!')
  })

  it('retypes the same message when buttons are supplied', () => {
    const { result } = setup()
    const onClick = vi.fn()
    run(() => result.current.typePennyHome('Hi!'))
    typeOut('Hi!')
    run(() => result.current.typePennyHome('Hi!', [{ label: 'Go', onClick }]))
    expect(result.current.homePennyTyping).toBe(true)
    expect(result.current.homePennyBtns).toHaveLength(1)
  })

  it('a new message replaces one still being typed', () => {
    const { result } = setup()
    run(() => result.current.typePennyHome('First message'))
    act(() => { vi.advanceTimersByTime(50) })
    run(() => result.current.typePennyHome('Second'))
    typeOut('Second')
    expect(result.current.homePennyText).toBe('Second')
  })
})

describe('sending a message', () => {
  it('ignores empty input with no images', async () => {
    const { result } = setup()
    await send(result, '   ')
    expect(fetchMock).not.toHaveBeenCalled()
    expect(result.current.chatLoading).toBe(false)
  })

  it('sends text to the parse endpoint, clears the box and resets review state', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, { action: 'add', items: [], added: 0, message: 'Done' }))
    const { result, opts } = setup()
    await send(result, ' milk ')
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/list/parse')
    expect(init.method).toBe('POST')
    expect(init.headers['X-Async']).toBe('1')
    expect(JSON.parse(init.body)).toEqual({ text: 'milk', images: [] })
    expect(result.current.chatText).toBe('')
    expect(opts.setPendingImages).toHaveBeenCalledWith([])
    expect(opts.hasReviewedRef.current).toBe(false)
    expect(opts.setPennyDisambig).toHaveBeenCalledWith([])
    expect(opts.setPennyDisambigIndex).toHaveBeenCalledWith(0)
    expect(result.current.chatLoading).toBe(false)
  })

  it('sends photos even with no text, and shows no ghost rows for them', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, { action: 'add', items: [] }))
    const { result } = setup({ pendingImages: ['data:image/png;base64,AAA'] })
    await act(async () => { await result.current.handleSendChat() })
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ text: '', images: ['data:image/png;base64,AAA'] })
    expect(result.current.pendingAdds).toEqual([])
  })

  it('shows ghost rows and loading while the request is in flight, then clears them', async () => {
    let resolve: (r: Response) => void = () => {}
    fetchMock.mockReturnValue(new Promise<Response>(r => { resolve = r }))
    const { result } = setup()
    act(() => result.current.setChatText('milk, eggs'))
    let p: Promise<void> = Promise.resolve()
    await act(async () => { p = result.current.handleSendChat() })
    expect(result.current.chatLoading).toBe(true)
    expect(result.current.pendingAdds).toEqual(['milk', 'eggs'])
    await act(async () => { resolve(jsonRes(200, { action: 'add', items: [] })); await p })
    expect(result.current.chatLoading).toBe(false)
    expect(result.current.pendingAdds).toEqual([])
  })

  it('ignores a second send while the first is loading (no duplicate request)', async () => {
    fetchMock.mockReturnValue(new Promise<Response>(() => {}))
    const { result } = setup()
    act(() => result.current.setChatText('milk'))
    await act(async () => { void result.current.handleSendChat() })
    act(() => result.current.setChatText('milk'))
    await act(async () => { await result.current.handleSendChat() })
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('hands the add result to the list', async () => {
    const milk = mkItem(1, { name: 'Milk', department: 'Dairy & Eggs', quantity: 2 })
    fetchMock.mockResolvedValue(jsonRes(200, { action: 'add', items: [milk], added: 1, skipped: [], message: 'Got it.' }))
    const { result, opts } = setup()
    await send(result, 'milk')
    expect(opts.setLastAdded).toHaveBeenLastCalledWith([milk])
    const updater = opts.setItems.mock.calls[0][0] as (p: ListItem[]) => ListItem[]
    expect(updater([mkItem(9)]).map(i => i.id).sort()).toEqual([1, 9])
    expect(opts.sortItems).toHaveBeenCalled()
  })

  it('asks the session handler on 401 and tells nobody else', async () => {
    fetchMock.mockResolvedValue(jsonRes(401, {}))
    const { result, opts } = setup()
    await send(result, 'milk')
    expect(opts.handleSessionExpired).toHaveBeenCalledTimes(1)
    expect(result.current.homePennyText).toBe('')
    expect(result.current.chatLoading).toBe(false)
  })

  it('shows Penny\'s message for a 202 and does not process a result', async () => {
    fetchMock.mockResolvedValue(jsonRes(202, { message: 'On it...' }))
    const { result, opts } = setup()
    await send(result, 'recipe for lasagna')
    typeOut('On it...')
    expect(result.current.homePennyText).toBe('On it...')
    expect(opts.setItems).not.toHaveBeenCalled()
  })

  it('falls back to a default message for a 202 without one', async () => {
    fetchMock.mockResolvedValue(jsonRes(202, {}))
    const { result } = setup()
    await send(result, 'recipe')
    typeOut('Working on it...')
    expect(result.current.homePennyText).toBe('Working on it...')
  })

  // The comment in the source says chatLoading stays true after a 202 until the
  // SSE result arrives, but the `finally` block clears it. Documented as a bug.
  it.fails('keeps loading after a 202 until the streamed result arrives', async () => {
    fetchMock.mockResolvedValue(jsonRes(202, { message: 'On it...' }))
    const { result } = setup()
    await send(result, 'recipe')
    expect(result.current.chatLoading).toBe(true)
  })

  it('offers to try again after a 429, and the retry sends the same text', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonRes(429, {}))
      .mockResolvedValueOnce(jsonRes(200, { action: 'add', items: [] }))
    const { result, opts } = setup()
    await send(result, 'milk')
    typeOut('Hold on — give it a minute and try again.')
    expect(result.current.homePennyText).toBe('Hold on — give it a minute and try again.')
    expect(result.current.homePennyBtns.map(b => b.label)).toEqual(['Try again'])
    expect(fetchMock).toHaveBeenCalledTimes(1)

    run(() => result.current.homePennyBtns[0].onClick())
    expect(opts.setPendingImages).toHaveBeenLastCalledWith([])
    await act(async () => { vi.advanceTimersByTime(100) })
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(JSON.parse(fetchMock.mock.calls[1][1].body).text).toBe('milk')
  })

  it('shows the server error for other failures and logs it', async () => {
    fetchMock.mockResolvedValue(jsonRes(500, { error: 'Parser down' }))
    const { result } = setup()
    await send(result, 'milk')
    typeOut('Something went wrong — Parser down')
    expect(result.current.homePennyText).toBe('Something went wrong — Parser down')
    expect(result.current.homePennyBtns.map(b => b.label)).toEqual(['Try again'])
    expect(console.error).toHaveBeenCalled()
    expect(result.current.chatLoading).toBe(false)
  })

  it('uses the status code when the error body is not JSON', async () => {
    fetchMock.mockResolvedValue({ status: 502, ok: false, json: () => Promise.reject(new Error('x')) } as Response)
    const { result } = setup()
    await send(result, 'milk')
    typeOut('Something went wrong — Error 502')
    expect(result.current.homePennyText).toBe('Something went wrong — Error 502')
  })

  it('tells the user when the network is down, and offers a retry', async () => {
    fetchMock.mockRejectedValue(new Error('offline'))
    const { result } = setup()
    await send(result, 'milk')
    typeOut("Couldn't connect — check your connection and try again.")
    expect(result.current.homePennyText).toBe("Couldn't connect — check your connection and try again.")
    expect(result.current.homePennyBtns).toHaveLength(1)
    expect(result.current.chatLoading).toBe(false)
  })

  it('nudges after 8 seconds of waiting, and not at all if the answer is quick', async () => {
    let resolve: (r: Response) => void = () => {}
    fetchMock.mockReturnValue(new Promise<Response>(r => { resolve = r }))
    const { result } = setup()
    act(() => result.current.setChatText('milk'))
    let p: Promise<void> = Promise.resolve()
    await act(async () => { p = result.current.handleSendChat() })
    act(() => { vi.advanceTimersByTime(7999) })
    expect(result.current.homePennyBtns).toEqual([])
    act(() => { vi.advanceTimersByTime(1) })
    expect(result.current.homePennyBtns.map(b => b.label)).toEqual(['Try again'])
    await act(async () => { resolve(jsonRes(200, { action: 'add', items: [] })); await p })
  })

  it('cancels the 8 second nudge once the answer arrives', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, { action: 'add', items: [], message: 'Hello' }))
    const { result } = setup()
    await send(result, 'milk')
    typeOut('Hello')
    act(() => { vi.advanceTimersByTime(20000) })
    expect(result.current.homePennyText).toBe('Hello')
  })
})

describe('processParseResult: non-add actions', () => {
  it('removes items and floats the removed names', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action: 'remove', removed: [mkItem(1, { name: 'Milk' }), mkItem(2, { name: 'Eggs' })], message: 'Removed' }))
    const updater = opts.setItems.mock.calls[0][0] as (p: ListItem[]) => ListItem[]
    expect(updater([mkItem(1), mkItem(2), mkItem(3)]).map(i => i.id)).toEqual([3])
    const labels = opts.setFloatingLabels.mock.calls[0][0]
    expect(labels.map((l: { text: string }) => l.text)).toEqual(['−Milk', '−Eggs'])
    expect(new Set(labels.map((l: { key: number }) => l.key)).size).toBe(2)
  })

  it('floats at most five removed names', () => {
    const { result, opts } = setup()
    const removed = Array.from({ length: 8 }, (_, i) => mkItem(i + 1))
    run(() => result.current.processParseResult({ action: 'remove', removed }))
    expect(opts.setFloatingLabels.mock.calls[0][0]).toHaveLength(5)
  })

  it.each(['check', 'uncheck'])('applies the server\'s %s state to the matching items only', (action) => {
    const { result, opts } = setup()
    const flag = action === 'check'
    run(() => result.current.processParseResult({ action, checked: [{ id: 1, checked: flag }] }))
    const updater = opts.setItems.mock.calls[0][0] as (p: ListItem[]) => ListItem[]
    const out = updater([mkItem(1, { checked: !flag }), mkItem(2, { checked: !flag })])
    expect(out.map(i => i.checked)).toEqual([flag, !flag])
  })

  it('summarises a check result when Penny sent no message', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({ action: 'check', checked: [{ id: 1, checked: true }, { id: 2, checked: true }] }))
    typeOut('Checked 2 items')
    expect(result.current.homePennyText).toBe('Checked 2 items')
  })

  it('merges updated items', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action: 'update', updated: [mkItem(1, { brand: 'Kix' })], message: 'Updated' }))
    const updater = opts.setItems.mock.calls[0][0] as (p: ListItem[]) => ListItem[]
    expect(updater([mkItem(1), mkItem(2)]).find(i => i.id === 1)?.brand).toBe('Kix')
  })

  it('share: types the message with an Invite button that opens the invite sheet', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action: 'share', message: 'Invite your household' }))
    typeOut('Invite your household')
    expect(result.current.homePennyText).toBe('Invite your household')
    run(() => result.current.homePennyBtns[0].onClick())
    expect(result.current.homePennyBtns[0].label).toBe('Invite')
    expect(opts.handleOpenInviteSheet).toHaveBeenCalledTimes(1)
  })

  it('share without a message types nothing', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({ action: 'share' }))
    expect(result.current.homePennyTyping).toBe(false)
    expect(result.current.homePennyBtns).toEqual([])
  })

  it.each(['save', 'delete_saved_list'])('%s shows only Penny\'s message', (action) => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action, message: 'Saved it' }))
    typeOut('Saved it')
    expect(result.current.homePennyText).toBe('Saved it')
    expect(opts.setItems).not.toHaveBeenCalled()
  })

  it('load_saved_list with items added reloads the list', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({
      action: 'load_saved_list', added: 3, message: 'Loaded', saved_lists: [{ id: 1, name: 'Weekly', item_count: 3 }],
    }))
    expect(opts.reloadList).toHaveBeenCalledTimes(1)
    typeOut('Loaded')
    expect(result.current.homePennyText).toBe('Loaded')
  })

  it('load_saved_list with several lists offers one button each', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({
      action: 'load_saved_list', added: 0, message: 'Which one?',
      saved_lists: [{ id: 1, name: 'Weekly', item_count: 3 }, { id: 2, name: 'Party', item_count: 9 }],
    }))
    expect(result.current.homePennyBtns.map(b => b.label)).toEqual(['Weekly (3)', 'Party (9)'])
    run(() => result.current.homePennyBtns[1].onClick())
    expect(opts.handleLoadSavedList).toHaveBeenCalledWith(2, 'merge')
    expect(opts.reloadList).not.toHaveBeenCalled()
  })

  it('load_saved_list with no saved lists just shows Penny\'s message', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action: 'load_saved_list', message: 'You have no saved lists.' }))
    typeOut('You have no saved lists.')
    expect(result.current.homePennyText).toBe('You have no saved lists.')
    expect(opts.reloadList).not.toHaveBeenCalled()
  })

  it('clear on an empty list says so', () => {
    const { result, opts } = setup({ items: [] })
    run(() => result.current.processParseResult({ action: 'clear' }))
    typeOut('Your list is already empty!')
    expect(result.current.homePennyText).toBe('Your list is already empty!')
    expect(opts.handleClearList).not.toHaveBeenCalled()
  })

  it('clear with checked and unchecked items offers three choices', () => {
    const items = [mkItem(1, { checked: true }), mkItem(2), mkItem(3)]
    const { result, opts } = setup({ items })
    run(() => result.current.processParseResult({ action: 'clear' }))
    expect(result.current.homePennyBtns.map(b => b.label)).toEqual([
      'Clear 1 checked item', 'Clear entire list (3 items)', 'Cancel',
    ])
    run(() => result.current.homePennyBtns[0].onClick())
    expect(opts.handleClearList).toHaveBeenLastCalledWith(false)
    run(() => result.current.processParseResult({ action: 'clear' }))
    run(() => result.current.homePennyBtns[1].onClick())
    expect(opts.handleClearList).toHaveBeenLastCalledWith(true)
    expect(opts.handleClearList).toHaveBeenCalledTimes(2)
  })

  it('clear with every item checked offers only the checked and cancel choices', () => {
    const items = [mkItem(1, { checked: true }), mkItem(2, { checked: true })]
    const { result } = setup({ items })
    run(() => result.current.processParseResult({ action: 'clear' }))
    expect(result.current.homePennyBtns.map(b => b.label)).toEqual(['Clear 2 checked items', 'Cancel'])
  })

  it('clear with none checked asks to confirm, and Cancel leaves the list alone', () => {
    const { result, opts } = setup({ items: [mkItem(1), mkItem(2)] })
    run(() => result.current.processParseResult({ action: 'clear' }))
    expect(result.current.homePennyBtns.map(b => b.label)).toEqual(['Yes, clear all 2 items', 'Cancel'])
    run(() => result.current.homePennyBtns[1].onClick())
    typeOut('No problem, your list is safe!')
    expect(result.current.homePennyText).toBe('No problem, your list is safe!')
    expect(opts.handleClearList).not.toHaveBeenCalled()
  })

  it('search types Penny\'s message and reloads only when items came back', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action: 'search', message: 'Found it' }))
    expect(opts.reloadList).not.toHaveBeenCalled()
    run(() => result.current.processParseResult({ action: 'search', items: [mkItem(1)], message: 'Found two' }))
    expect(opts.reloadList).toHaveBeenCalledTimes(1)
  })

  it('search without a message uses a default line', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({ action: 'search' }))
    typeOut("Here's what I found!")
    expect(result.current.homePennyText).toBe("Here's what I found!")
  })

  it('done_shopping offers Snap receipt and Skip', () => {
    const click = vi.fn()
    const { result, opts } = setup({ receiptInputRef: { current: { click } } })
    run(() => result.current.processParseResult({ action: 'done_shopping' }))
    expect(result.current.homePennyBtns.map(b => b.label)).toEqual(['Snap receipt', 'Skip'])
    run(() => result.current.homePennyBtns[0].onClick())
    expect(opts.setPendingReceiptMode).toHaveBeenCalledWith(true)
    expect(click).toHaveBeenCalledTimes(1)
  })

  it('done_shopping Skip says goodbye', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({ action: 'done_shopping', message: 'Trip done' }))
    run(() => result.current.homePennyBtns[1].onClick())
    typeOut('All good! Happy shopping!')
    expect(result.current.homePennyText).toBe('All good! Happy shopping!')
  })

  it('snap receipt tolerates a missing file input', () => {
    const { result } = setup({ receiptInputRef: { current: null } })
    run(() => result.current.processParseResult({ action: 'done_shopping' }))
    expect(() => run(() => result.current.homePennyBtns[0].onClick())).not.toThrow()
  })

  it('receipt_processed checks off matched items and leaves receipt mode', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action: 'receipt_processed', checked: [{ id: 1, checked: true }], message: 'Checked' }))
    const updater = opts.setItems.mock.calls[0][0] as (p: ListItem[]) => ListItem[]
    expect(updater([mkItem(1), mkItem(2)]).map(i => i.checked)).toEqual([true, false])
    expect(opts.setPendingReceiptMode).toHaveBeenCalledWith(false)
  })

  it('receipt_processed with nothing matched still leaves receipt mode', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action: 'receipt_processed' }))
    expect(opts.setItems).not.toHaveBeenCalled()
    expect(opts.setPendingReceiptMode).toHaveBeenCalledWith(false)
  })

  it('receipt_duplicate leaves receipt mode and changes no items', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action: 'receipt_duplicate', message: 'Seen that one.' }))
    expect(opts.setPendingReceiptMode).toHaveBeenCalledWith(false)
    expect(opts.setItems).not.toHaveBeenCalled()
  })

  it('swap_accepted updates items and removes the matching deal cards', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action: 'swap_accepted', updated: [mkItem(2, { brand: 'Magic Spoon' })], message: 'Nice' }))
    expect(track).toHaveBeenCalledWith('swap_accepted', { items: 1 })
    const itemsUpdater = opts.setItems.mock.calls[0][0] as (p: ListItem[]) => ListItem[]
    expect(itemsUpdater([mkItem(2)])[0].brand).toBe('Magic Spoon')
    const swapsUpdater = opts.setSwapsByItemId.mock.calls[0][0] as (p: Record<number, unknown>) => Record<number, unknown>
    expect(swapsUpdater({ 2: {}, 3: {} })).toEqual({ 3: {} })
  })

  it('swap_accepted with no updated items leaves the list alone', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action: 'swap_accepted', message: 'Nice' }))
    expect(opts.setItems).not.toHaveBeenCalled()
  })

  it('always clears loading and ghost rows', async () => {
    fetchMock.mockReturnValue(new Promise<Response>(() => {}))
    const { result } = setup()
    act(() => result.current.setChatText('milk'))
    await act(async () => { void result.current.handleSendChat() })
    expect(result.current.chatLoading).toBe(true)
    run(() => result.current.processParseResult({ action: 'add', items: [] }))
    expect(result.current.chatLoading).toBe(false)
    expect(result.current.pendingAdds).toEqual([])
  })
})

describe('processParseResult: add', () => {
  const eggs = mkItem(1, { name: 'Eggs', department: 'Dairy & Eggs', quantity: 2 })
  const apple = mkItem(2, { name: 'Apple', department: 'Produce' })

  it('floats labels, expands the departments, highlights new items and records first add', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action: 'add', items: [eggs, apple], added: 2, message: 'Added' }))
    expect(opts.setFloatingLabels.mock.calls[0][0].map((l: { text: string }) => l.text)).toEqual(['+2 Eggs', '+1 Apple'])
    expect(localStorage.getItem('has_added_items')).toBe('1')
    expect(track).toHaveBeenCalledWith('items_added', { count: 2 })
    expect(trackEvent).toHaveBeenCalledWith('onboarding_v2', 'items_added_day1', 2)
    const expand = opts.setExpandedDepts.mock.calls[0][0] as (p: Set<string>) => Set<string>
    expect([...expand(new Set(['Frozen']))].sort()).toEqual(['Dairy & Eggs', 'Frozen', 'Produce'])
    expect([...(opts.setNewItemIds.mock.calls[0][0] as Set<number>)].sort()).toEqual([1, 2])
    act(() => { vi.advanceTimersByTime(1500) })
    expect([...(opts.setNewItemIds.mock.calls[1][0] as Set<number>)]).toEqual([])
  })

  it('resets the highlight timer if a previous one is still running', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action: 'add', items: [eggs], added: 1, message: 'a' }))
    act(() => { vi.advanceTimersByTime(1000) })
    run(() => result.current.processParseResult({ action: 'add', items: [apple], added: 1, message: 'b' }))
    act(() => { vi.advanceTimersByTime(1000) })
    // first timer (would have fired at 1500) was cancelled: only the latest set is live
    expect(opts.setNewItemIds).toHaveBeenCalledTimes(2)
    act(() => { vi.advanceTimersByTime(500) })
    expect(opts.setNewItemIds).toHaveBeenCalledTimes(3)
  })

  it('scrolls the first department in aisle order into view', () => {
    const scrollIntoView = vi.fn()
    const { result } = setup({ deptRefs: { current: { Produce: { scrollIntoView } } } })
    run(() => result.current.processParseResult({ action: 'add', items: [eggs, apple], added: 2, message: 'a' }))
    act(() => { vi.advanceTimersByTime(100) })
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' })
  })

  it('floats at most five labels', () => {
    const { result, opts } = setup()
    const items = Array.from({ length: 7 }, (_, i) => mkItem(i + 1))
    run(() => result.current.processParseResult({ action: 'add', items, added: 7, message: 'a' }))
    expect(opts.setFloatingLabels.mock.calls[0][0]).toHaveLength(5)
  })

  it('does not record a first add or float labels when nothing was added', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ action: 'add', items: [], added: 0, message: 'a' }))
    expect(localStorage.getItem('has_added_items')).toBeNull()
    expect(opts.setFloatingLabels).not.toHaveBeenCalled()
    expect(track).not.toHaveBeenCalledWith('items_added', expect.anything())
  })

  it('treats a result with no action as an add', () => {
    const { result, opts } = setup()
    run(() => result.current.processParseResult({ items: [eggs], added: 1 }))
    expect(opts.setLastAdded).toHaveBeenCalledWith([eggs])
  })

  it('stores a pending deal offer for the item', () => {
    const { result, opts } = setup()
    const swap = { item_id: 1, deal_id: 9, offer_brand: 'Magic Spoon', discount_cents: 150 }
    run(() => result.current.processParseResult({ action: 'add', items: [eggs], added: 1, message: 'a', pending_swap: swap }))
    const updater = opts.setSwapsByItemId.mock.calls[0][0] as (p: Record<number, unknown>) => Record<number, unknown>
    expect(updater({})).toEqual({ 1: swap })
    expect(track).toHaveBeenCalledWith('swap_offered', { deal_id: 9, offer_brand: 'Magic Spoon', discount_cents: 150 })
  })

  it('warns about household duplicates', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({
      action: 'add', items: [], added: 0, message: 'a',
      duplicate_warnings: [
        { item_name: 'milk', added_by: 'Alex', hours_ago: 3 },
        { item_name: 'eggs', added_by: 'Jo', hours_ago: 0 },
      ],
    }))
    // Penny's own message is typed after the warning and replaces it; the warning
    // is still tracked.
    expect(track).toHaveBeenCalledWith('household_duplicate_warned', { count: 2 })
  })

  it('types the household warning when Penny has no message of her own', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({
      action: 'add', items: [], added: 0, skipped: [{ reason: 'duplicate', parsed: { name: 'milk' } }],
      duplicate_warnings: [
        { item_name: 'milk', added_by: 'Alex', hours_ago: 3 },
        { item_name: 'eggs', added_by: 'Jo', hours_ago: 0 },
      ],
    }))
    // The fallback summary follows and replaces the warning being typed.
    typeOut('1 already on your list')
    expect(result.current.homePennyText).toBe('1 already on your list')
  })

  it('passes skipped items to the review state', () => {
    const { result, opts } = setup()
    const skipped = [{ reason: 'duplicate', existing_id: 4, parsed: { name: 'milk' } }]
    run(() => result.current.processParseResult({ action: 'add', items: [], skipped, message: 'a' }))
    expect(opts.setLastSkipped).toHaveBeenCalledWith(skipped)
  })

  it('refocuses the input shortly after', () => {
    const focus = vi.fn()
    const { result } = setup()
    ;(result.current.chatInputRef as Any).current = { focus, style: {} }
    run(() => result.current.processParseResult({ action: 'add', items: [] }))
    act(() => { vi.advanceTimersByTime(150) })
    expect(focus).toHaveBeenCalled()
  })
})

describe('processParseResult: Penny\'s line after an add', () => {
  const eggs = mkItem(1, { name: 'Eggs' })

  it('uses Penny\'s message verbatim', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({ action: 'add', items: [eggs], added: 1, message: 'Eggs, done.' }))
    typeOut('Eggs, done.')
    expect(result.current.homePennyText).toBe('Eggs, done.')
  })

  it('appends a note on ambiguous skipped items', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({
      action: 'add', items: [eggs], added: 1, message: 'Added eggs.',
      skipped: [{ reason: 'ambiguous', raw_input: 'stuff' }, { reason: 'needs_clarification', parsed: { name: 'thing' } }, { reason: 'x' }],
    }))
    const full = "Added eggs. I wasn't sure about: 'stuff', 'thing', 'unknown' — try being more specific!"
    typeOut(full)
    expect(result.current.homePennyText).toBe(full)
  })

  it('does not append the note when Penny already mentioned it', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({
      action: 'add', items: [eggs], added: 1, message: "I wasn't sure about 'stuff'.",
      skipped: [{ reason: 'ambiguous', raw_input: 'stuff' }],
    }))
    typeOut("I wasn't sure about 'stuff'.")
    expect(result.current.homePennyText).toBe("I wasn't sure about 'stuff'.")
  })

  it('does not append the note for duplicates', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({
      action: 'add', items: [eggs], added: 1, message: 'Added eggs.',
      skipped: [{ reason: 'duplicate', existing_id: 3, parsed: { name: 'milk' } }],
    }))
    typeOut('Added eggs.')
    expect(result.current.homePennyText).toBe('Added eggs.')
  })

  it('builds a summary when there is no message', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({
      action: 'add', items: [eggs, mkItem(2)], added: 2,
      skipped: [{ reason: 'duplicate', existing_id: 3 }, { reason: '' }, { reason: 'ambiguous', raw_input: 'zzz' }],
    }))
    const full = "Added 2 items · 2 already on your list · Wasn't sure about 'zzz'"
    typeOut(full)
    expect(result.current.homePennyText).toBe(full)
  })

  it('uses the singular for one added item', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({ action: 'add', items: [eggs], added: 1 }))
    typeOut('Added 1 item')
    expect(result.current.homePennyText).toBe('Added 1 item')
  })

  it('summarises removed and updated counts', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({ action: 'remove', removed: [mkItem(1)] }))
    typeOut('Removed 1 item')
    expect(result.current.homePennyText).toBe('Removed 1 item')
    run(() => result.current.processParseResult({ action: 'update', updated: [mkItem(1), mkItem(2)] }))
    typeOut('Updated 2 items')
    expect(result.current.homePennyText).toBe('Updated 2 items')
  })

  it('says it found no items when the parse came back empty', () => {
    const { result } = setup()
    run(() => result.current.processParseResult({ action: 'add', items: [], skipped: [] }))
    typeOut("I couldn't find any items in that — try rewording it?")
    expect(result.current.homePennyText).toBe("I couldn't find any items in that — try rewording it?")
  })
})

describe('processParseResult: brand disambiguation', () => {
  const commodity = { item_id: 5, product_category: 'Milk', current_brand: 'Horizon', alternatives: ['Organic Valley'], auto_assigned: true }
  const nonCommodity = { item_id: 6, product_category: 'Granola bars', current_brand: 'Kind', alternatives: ['Nature Valley'], auto_assigned: true }

  it('confirms a commodity auto-assigned brand silently and toasts', async () => {
    const { result, opts } = setup()
    await act(async () => { result.current.processParseResult({ action: 'add', items: [], brand_disambiguations: [commodity] }) })
    const [url, init] = vi.mocked(fetchAction).mock.calls[0]
    expect(url).toBe('/api/list/brand-resolve')
    expect(JSON.parse(init!.body as string)).toEqual({ item_id: 5, brand: 'Horizon', product_category: 'Milk' })
    expect(opts.showToast).toHaveBeenCalledWith('Added Milk (Horizon)', 'success')
    expect(opts.setPennyDisambig).not.toHaveBeenCalled()
    // the add flow calls setItems first; the brand confirmation is the last call
    const itemsUpdater = opts.setItems.mock.calls.at(-1)![0] as (p: ListItem[]) => ListItem[]
    expect(itemsUpdater([mkItem(5), mkItem(6)]).map(i => i.brand)).toEqual(['Horizon', ''])
    const addedUpdater = opts.setLastAdded.mock.calls.at(-1)![0] as (p: ListItem[]) => ListItem[]
    expect(addedUpdater([mkItem(5)])[0].brand).toBe('Horizon')
    expect(trackEvent).toHaveBeenCalledWith('onboarding_v2', 'brand_default_confirmed')
  })

  it('does not change the item when the silent confirmation fails', async () => {
    vi.mocked(fetchAction).mockResolvedValue({ ok: false, status: 500 })
    const { result, opts } = setup()
    await act(async () => { result.current.processParseResult({ action: 'add', items: [], brand_disambiguations: [commodity] }) })
    // only the add flow's own setItems call; no brand-confirmation update
    expect(opts.setItems).toHaveBeenCalledTimes(1)
    expect(trackEvent).not.toHaveBeenCalledWith('onboarding_v2', 'brand_default_confirmed')
  })

  it('asks about non-commodity items and mentions auto-picks for the commodity ones', async () => {
    const { result, opts } = setup()
    await act(async () => { result.current.processParseResult({ action: 'add', items: [], brand_disambiguations: [commodity, nonCommodity] }) })
    expect(opts.showToast).toHaveBeenCalledWith('Auto-picked brands for Milk', 'success')
    expect(opts.setPennyDisambig).toHaveBeenCalledWith([nonCommodity])
    expect(opts.setPennyDisambigIndex).toHaveBeenCalledWith(0)
    expect(fetchAction).toHaveBeenCalledTimes(1)
  })

  it('asks when the commodity brand was not auto-assigned', async () => {
    const asked = { ...commodity, auto_assigned: false }
    const { result, opts } = setup()
    await act(async () => { result.current.processParseResult({ action: 'add', items: [], brand_disambiguations: [asked] }) })
    expect(fetchAction).not.toHaveBeenCalled()
    expect(opts.setPennyDisambig).toHaveBeenCalledWith([asked])
    expect(opts.showToast).not.toHaveBeenCalled()
  })

  it('asks when an auto-assigned commodity has no current brand', async () => {
    const blank = { ...commodity, current_brand: '' }
    const { result, opts } = setup()
    await act(async () => { result.current.processParseResult({ action: 'add', items: [], brand_disambiguations: [blank] }) })
    expect(fetchAction).not.toHaveBeenCalled()
    expect(opts.setPennyDisambig).toHaveBeenCalledWith([blank])
  })
})

describe('processParseResult: onboarding nudges', () => {
  const one = [mkItem(1)]
  const add = (r: { current: ReturnType<typeof useChatState> }, items = one) =>
    run(() => r.current.processParseResult({ action: 'add', items, added: items.length, message: 'ok' }))

  it('asks a first-session user their name once, after a pause', () => {
    const { result } = setup({ loginCount: 1, settingsFirstName: '' })
    add(result)
    expect(localStorage.getItem('ob_name')).toBe('1')
    expect(result.current.onboardingNameAsked.current).toBe(true)
    expect(fetchAPI).toHaveBeenCalledTimes(1)
    expect(result.current.nameInputMode.current).toBe(false)
    act(() => { vi.advanceTimersByTime(2500) })
    expect(result.current.nameInputMode.current).toBe(true)
    typeOut('By the way, what should I call you?')
    expect(result.current.homePennyText).toBe('By the way, what should I call you?')

    // a second add does not ask again
    add(result)
    expect(fetchAPI).toHaveBeenCalledTimes(1)
  })

  it('Skip on the name question leaves name mode', () => {
    const { result } = setup({ loginCount: 1, settingsFirstName: '' })
    add(result)
    act(() => { vi.advanceTimersByTime(2500) })
    run(() => result.current.homePennyBtns[0].onClick())
    expect(result.current.nameInputMode.current).toBe(false)
    typeOut('No worries! Just keep adding items.')
    expect(result.current.homePennyText).toBe('No worries! Just keep adding items.')
  })

  it('does not ask for a name that is already known', () => {
    const { result } = setup({ loginCount: 1, settingsFirstName: 'Sam' })
    add(result)
    expect(localStorage.getItem('ob_name')).toBeNull()
  })

  it('mentions sharing once the list reaches three items', () => {
    const { result, opts } = setup({ items: [mkItem(10), mkItem(11)] })
    add(result)
    expect(localStorage.getItem('ob_share')).toBe('1')
    act(() => { vi.advanceTimersByTime(3000) })
    expect(result.current.homePennyBtns.map(b => b.label)).toEqual(['Yes, invite them', 'Just me for now'])
    run(() => result.current.homePennyBtns[0].onClick())
    expect(opts.handleOpenInviteSheet).toHaveBeenCalledTimes(1)
  })

  it('"Just me for now" answers in kind', () => {
    const { result } = setup({ items: [mkItem(10), mkItem(11)] })
    add(result)
    act(() => { vi.advanceTimersByTime(3000) })
    run(() => result.current.homePennyBtns[1].onClick())
    typeOut('Got it — solo mission!')
    expect(result.current.homePennyText).toBe('Got it — solo mission!')
  })

  it('does not mention sharing for a short list, or twice', () => {
    const { result } = setup({ items: [] })
    add(result)
    expect(localStorage.getItem('ob_share')).toBeNull()
    localStorage.setItem('ob_share', '1')
    const again = setup({ items: [mkItem(10), mkItem(11)] })
    add(again.result)
    expect(fetchAPI).not.toHaveBeenCalled()
  })

  it('mentions deals at five items once sharing has been mentioned', () => {
    localStorage.setItem('ob_share', '1')
    const { result } = setup({ items: [mkItem(10), mkItem(11), mkItem(12), mkItem(13)] })
    add(result)
    expect(localStorage.getItem('ob_swaps')).toBe('1')
    const msg = "By the way — I keep an eye out for deals on your items. If I spot a swap that saves you money, I'll let you know!"
    act(() => { vi.advanceTimersByTime(3499) })
    expect(result.current.homePennyText).not.toBe(msg)
    expect(result.current.homePennyText).not.toBe('')
    act(() => { vi.advanceTimersByTime(1) })
    typeOut(msg)
    expect(result.current.homePennyText).toBe(msg)
  })

  it('asks a returning user without an email to add one', () => {
    localStorage.setItem('ob_share', '1')
    localStorage.setItem('ob_swaps', '1')
    const { result, opts } = setup({ loginCount: 2, settingsEmail: '' })
    add(result)
    expect(localStorage.getItem('ob_link_email')).toBe('1')
    act(() => { vi.advanceTimersByTime(3500) })
    expect(result.current.homePennyBtns.map(b => b.label)).toEqual(['Add email', 'Maybe later'])
    run(() => result.current.homePennyBtns[0].onClick())
    expect(opts.setScreen).toHaveBeenCalledWith('profile')
  })

  it('"Maybe later" on the email prompt answers in kind', () => {
    localStorage.setItem('ob_share', '1')
    localStorage.setItem('ob_swaps', '1')
    const { result } = setup({ loginCount: 2, settingsEmail: '' })
    add(result)
    act(() => { vi.advanceTimersByTime(3500) })
    run(() => result.current.homePennyBtns[1].onClick())
    typeOut("Just let me know when you're ready!")
    expect(result.current.homePennyText).toBe("Just let me know when you're ready!")
  })

  it('does not ask for an email the user already gave', () => {
    localStorage.setItem('ob_share', '1')
    localStorage.setItem('ob_swaps', '1')
    const { result } = setup({ loginCount: 2, settingsEmail: 'a@b.co' })
    add(result)
    expect(localStorage.getItem('ob_link_email')).toBeNull()
  })

  it('shows no nudges when nothing was added', () => {
    const { result } = setup({ loginCount: 1, settingsFirstName: '' })
    run(() => result.current.processParseResult({ action: 'add', items: [], added: 0, message: 'ok' }))
    expect(localStorage.getItem('ob_name')).toBeNull()
  })

  it('shows no nudges for non-add actions', () => {
    const { result } = setup({ loginCount: 1, settingsFirstName: '' })
    run(() => result.current.processParseResult({ action: 'update', updated: one, message: 'ok' }))
    expect(localStorage.getItem('ob_name')).toBeNull()
  })
})

describe('name input mode', () => {
  it('saves a typed name, splitting first and last, and sends no parse request', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, {}))
    const { result, opts } = setup()
    result.current.nameInputMode.current = true
    await send(result, 'Sam Lee')
    expect(opts.setSettingsFirstName).toHaveBeenCalledWith('Sam')
    expect(opts.setSettingsLastName).toHaveBeenCalledWith('Lee')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/settings')
    expect(init.method).toBe('PATCH')
    expect(JSON.parse(init.body)).toEqual({ name: 'Sam Lee' })
    typeOut('Nice to meet you, Sam! Keep adding — I\'ll sort everything by aisle.')
    expect(result.current.homePennyText).toBe("Nice to meet you, Sam! Keep adding — I'll sort everything by aisle.")
    expect(result.current.nameInputMode.current).toBe(false)
    expect(result.current.chatText).toBe('')
  })

  it('survives a failed name save', async () => {
    fetchMock.mockRejectedValue(new Error('offline'))
    const { result } = setup()
    result.current.nameInputMode.current = true
    await send(result, 'Maria')
    typeOut('Nice to meet you, Maria! Keep adding — I\'ll sort everything by aisle.')
    expect(result.current.homePennyText).toContain('Nice to meet you, Maria!')
  })

  it.each(['milk', 'organic milk', 'eggs and bacon', '2 apples', 'large eggs'])(
    'treats "%s" as groceries and parses it instead of saving a name', async (text) => {
      fetchMock.mockResolvedValue(jsonRes(200, { action: 'add', items: [] }))
      const { result, opts } = setup()
      result.current.nameInputMode.current = true
      await send(result, text)
      expect(opts.setSettingsFirstName).not.toHaveBeenCalled()
      expect(fetchMock).toHaveBeenCalledTimes(1)
      expect(fetchMock.mock.calls[0][0]).toBe('/api/list/parse')
      expect(JSON.parse(fetchMock.mock.calls[0][1].body).text).toBe(text)
      expect(result.current.nameInputMode.current).toBe(false)
    })

  it('does not treat a name as a name when photos are pending', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, { action: 'add', items: [] }))
    const { result, opts } = setup({ pendingImages: ['img'] })
    result.current.nameInputMode.current = true
    await send(result, 'Sam')
    expect(opts.setSettingsFirstName).not.toHaveBeenCalled()
    expect(fetchMock.mock.calls[0][0]).toBe('/api/list/parse')
  })
})

describe('brand pick through the chat box', () => {
  const d = { item_id: 1, product_category: 'cereal', current_brand: 'Cheerios', alternatives: ['Kix', 'Chex'] }

  it.each(['skip', 'Next'])('"%s" moves to the next item', async (text) => {
    const { result, opts } = setup({ pennyDisambig: [d] })
    await send(result, text)
    expect(opts.advanceDisambig).toHaveBeenCalledTimes(1)
    expect(fetchMock).not.toHaveBeenCalled()
    expect(result.current.chatText).toBe('')
  })

  it.each(['skip all', 'Just pick for me', 'just pick', 'you decide'])('"%s" dismisses all questions', async (text) => {
    const { result, opts } = setup({ pennyDisambig: [d] })
    await send(result, text)
    expect(opts.setPennyDisambig).toHaveBeenCalledWith([])
    expect(opts.setPennyDisambigIndex).toHaveBeenCalledWith(0)
    typeOut("All set! I'll remember those for next time.")
    expect(result.current.homePennyText).toBe("All set! I'll remember those for next time.")
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('picks a matching alternative, case-insensitively', async () => {
    const { result, opts } = setup({ pennyDisambig: [d] })
    await send(result, 'kix')
    expect(opts.handleBrandPick).toHaveBeenCalledWith('Kix')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('treats short free text as a custom brand, keeping what the user typed', async () => {
    const { result, opts } = setup({ pennyDisambig: [d] })
    await send(result, 'Annie\'s')
    expect(opts.handleBrandPick).toHaveBeenCalledWith("Annie's")
  })

  it.each(['kix and chex', 'bread, milk', 'x'.repeat(40)])('"%s" is parsed as a new request', async (text) => {
    fetchMock.mockResolvedValue(jsonRes(200, { action: 'add', items: [] }))
    const { result, opts } = setup({ pennyDisambig: [d] })
    await send(result, text)
    expect(opts.handleBrandPick).not.toHaveBeenCalled()
    expect(fetchMock.mock.calls[0][0]).toBe('/api/list/parse')
  })

  it('uses the question at the current index', async () => {
    const second = { ...d, item_id: 2, alternatives: ['Dial'] }
    const { result, opts } = setup({ pennyDisambig: [d, second], pennyDisambigIndex: 1 })
    await send(result, 'dial')
    expect(opts.handleBrandPick).toHaveBeenCalledWith('Dial')
  })

  it('falls through to parsing when the index is past the end', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, { action: 'add', items: [] }))
    const { result, opts } = setup({ pennyDisambig: [d], pennyDisambigIndex: 3 })
    await send(result, 'bread')
    expect(opts.handleBrandPick).not.toHaveBeenCalled()
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('does not hijack a message while photos are pending', async () => {
    fetchMock.mockResolvedValue(jsonRes(200, { action: 'add', items: [] }))
    const { result, opts } = setup({ pennyDisambig: [d], pendingImages: ['img'] })
    await send(result, 'kix')
    expect(opts.handleBrandPick).not.toHaveBeenCalled()
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})
