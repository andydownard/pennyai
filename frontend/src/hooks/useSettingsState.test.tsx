import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'

vi.mock('../api', () => ({ fetchAPI: vi.fn() }))
vi.mock('../analytics', () => ({ track: vi.fn() }))

import { fetchAPI } from '../api'
import { track } from '../analytics'
import { useSettingsState } from './useSettingsState'

const api = vi.mocked(fetchAPI)
const trackMock = vi.mocked(track)

function setup(screen = 'home', extra: Record<string, unknown> = {}) {
  const opts = { screen, currentUserId: 1, showToast: vi.fn(), onEarningsTotalCents: vi.fn(), onOnboardingFlags: vi.fn(), ...extra }
  const hook = renderHook((p: { screen: string }) => useSettingsState({ ...opts, screen: p.screen } as never), { initialProps: { screen } })
  return { opts, ...hook }
}

const settingsBody = {
  name: 'Ada Lovelace King', email: 'ada@example.com', address: '1 Main St',
  google_places_ready: true, household_members: [{ name: 'Bob' }],
  penny_slider: 70, penny_autotune: false, store_order_enabled: false,
  swap_credits_cents: 1234, list_name: 'Home', onboarding_flags: '{"a":"1"}',
}

function routes(map: Record<string, unknown>) {
  api.mockImplementation((async (url: string) => (url in map ? map[url] : null)) as never)
}

beforeEach(() => {
  api.mockReset(); trackMock.mockReset()
  localStorage.clear(); sessionStorage.clear()
  delete (navigator as { share?: unknown }).share
  delete (navigator as { clipboard?: unknown }).clipboard
})
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

describe('useSettingsState initial state', () => {
  it('has defaults and does not fetch on an unrelated screen', () => {
    const r = setup('home')
    expect(r.result.current.settingsFirstName).toBe('')
    expect(r.result.current.pennySlider).toBe(35)
    expect(r.result.current.pennyAutotune).toBe(true)
    expect(r.result.current.storeOrderEnabled).toBe(true)
    expect(r.result.current.showInviteSheet).toBe(false)
    expect(r.result.current.inviteLoading).toBe(false)
    expect(api).not.toHaveBeenCalled()
  })
})

describe('useSettingsState loading', () => {
  it.each(['settings', 'profile', 'settings-share'])('loads settings on %s', async screen => {
    routes({ '/api/settings': settingsBody, '/api/referral': { code: 'ABC', friends: [] } })
    const r = setup(screen)
    await waitFor(() => expect(r.result.current.settingsEmail).toBe('ada@example.com'))
    expect(api).toHaveBeenCalledWith('/api/settings')
  })

  it('maps every settings field and fires callbacks', async () => {
    routes({ '/api/settings': settingsBody })
    const r = setup('profile')
    await waitFor(() => expect(r.result.current.listName).toBe('Home'))
    const c = r.result.current
    expect(c.settingsFirstName).toBe('Ada')
    expect(c.settingsLastName).toBe('Lovelace King')
    expect(c.settingsAddress).toBe('1 Main St')
    expect(c.placesReady).toBe(true)
    expect(c.settingsMembers).toEqual([{ name: 'Bob' }])
    expect(c.pennySlider).toBe(70)
    expect(c.pennyAutotune).toBe(false)
    expect(c.storeOrderEnabled).toBe(false)
    expect(r.opts.onEarningsTotalCents).toHaveBeenCalledWith(1234)
    expect(r.opts.onOnboardingFlags).toHaveBeenCalledWith({ a: '1' })
  })

  it('keeps defaults when optional fields are absent, and tolerates missing callbacks', async () => {
    routes({ '/api/settings': { name: '' } })
    const r = setup('profile', { onEarningsTotalCents: undefined, onOnboardingFlags: undefined })
    await waitFor(() => expect(api).toHaveBeenCalled())
    await act(async () => {})
    const c = r.result.current
    expect(c.settingsFirstName).toBe('')
    expect(c.settingsMembers).toEqual([])
    expect(c.pennySlider).toBe(35)
    expect(c.pennyAutotune).toBe(true)
    expect(c.placesReady).toBe(false)
  })

  it('zero credits still reaches the callback', async () => {
    routes({ '/api/settings': { name: 'A', swap_credits_cents: 0 } })
    const r = setup('profile')
    await waitFor(() => expect(r.opts.onEarningsTotalCents).toHaveBeenCalledWith(0))
  })

  it('ignores bad onboarding_flags JSON', async () => {
    routes({ '/api/settings': { name: 'A', onboarding_flags: '{nope' } })
    const r = setup('profile')
    await waitFor(() => expect(r.result.current.settingsFirstName).toBe('A'))
    expect(r.opts.onOnboardingFlags).not.toHaveBeenCalled()
  })

  it('ignores null and array settings responses', async () => {
    routes({ '/api/settings': [] })
    const r = setup('profile')
    await act(async () => {})
    expect(r.result.current.settingsFirstName).toBe('')
  })

  it('swallows a settings failure with a warning', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    api.mockRejectedValue(new Error('x'))
    setup('profile')
    await waitFor(() => expect(warn).toHaveBeenCalled())
  })

  it('loads referral data only on the share screen', async () => {
    routes({ '/api/settings': settingsBody, '/api/referral': { code: 'ABC', friends: [{ name: 'Z', created_at: 't' }] } })
    const p = setup('profile')
    await act(async () => {})
    expect(api).not.toHaveBeenCalledWith('/api/referral')
    p.unmount()
    const r = setup('settings-share')
    await waitFor(() => expect(r.result.current.referralCode).toBe('ABC'))
    expect(r.result.current.referralFriends).toEqual([{ name: 'Z', created_at: 't' }])
  })

  it('referral defaults when fields are missing; failure is swallowed', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    routes({ '/api/settings': {}, '/api/referral': {} })
    const r = setup('settings-share')
    await act(async () => {})
    expect(r.result.current.referralCode).toBe('')
    expect(r.result.current.referralFriends).toEqual([])
    api.mockRejectedValue(new Error('x'))
    act(() => r.result.current.loadReferralData())
    await waitFor(() => expect(warn).toHaveBeenCalled())
  })

  it('reloads settings when the screen changes into a settings screen', async () => {
    routes({ '/api/settings': settingsBody })
    const r = setup('home')
    expect(api).not.toHaveBeenCalled()
    r.rerender({ screen: 'settings' })
    await waitFor(() => expect(api).toHaveBeenCalledWith('/api/settings'))
  })
})

describe('useSettingsState auto-save', () => {
  it('debounces to one PATCH with the trimmed full name and address', async () => {
    vi.useFakeTimers()
    api.mockResolvedValue({} as never)
    const r = setup('home')
    act(() => { r.result.current.setSettingsFirstName(' Ada '); r.result.current.setSettingsLastName(' King ') ; r.result.current.setSettingsAddress(' 1 Main ') })
    act(() => { r.result.current.handleSettingsAutoSave(); r.result.current.handleSettingsAutoSave(); r.result.current.handleSettingsAutoSave() })
    expect(api).not.toHaveBeenCalled()
    await act(async () => { vi.advanceTimersByTime(500) })
    expect(api).toHaveBeenCalledTimes(1)
    const [url, init] = api.mock.calls[0] as unknown as [string, { method: string; body: string }]
    expect(url).toBe('/api/settings')
    expect(init.method).toBe('PATCH')
    expect(JSON.parse(init.body)).toEqual({ name: 'Ada King', address: '1 Main' })
  })

  it('omits an empty last name from the saved name', async () => {
    api.mockResolvedValue({} as never)
    const r = setup('home')
    act(() => r.result.current.setSettingsFirstName('Ada'))
    await act(async () => { await r.result.current.doSettingsSave() })
    expect(JSON.parse((api.mock.calls[0][1] as { body: string }).body).name).toBe('Ada')
  })

  it('a save requested mid-flight is queued and sent once afterwards, never in parallel', async () => {
    let release: () => void = () => {}
    api.mockImplementationOnce((() => new Promise(r => { release = () => r({}) })) as never)
    api.mockResolvedValue({} as never)
    const r = setup('home')
    let first: Promise<void> = Promise.resolve()
    act(() => { first = r.result.current.doSettingsSave() })
    await act(async () => { await r.result.current.doSettingsSave(); await r.result.current.doSettingsSave() })
    expect(api).toHaveBeenCalledTimes(1)
    await act(async () => { release(); await first })
    await waitFor(() => expect(api).toHaveBeenCalledTimes(2))
    await act(async () => {})
    expect(api).toHaveBeenCalledTimes(2)
  })

  it('toasts on failure and allows the next save', async () => {
    api.mockRejectedValueOnce(new Error('x'))
    api.mockResolvedValue({} as never)
    const r = setup('home')
    await act(async () => { await r.result.current.doSettingsSave() })
    expect(r.opts.showToast).toHaveBeenCalledWith('Something went wrong. Try again.', 'error')
    await act(async () => { await r.result.current.doSettingsSave() })
    expect(api).toHaveBeenCalledTimes(2)
  })
})

describe('useSettingsState toggles', () => {
  it('toggleAutotune flips and PATCHes the new value', async () => {
    api.mockResolvedValue({} as never)
    const r = setup('home')
    act(() => r.result.current.toggleAutotune())
    expect(r.result.current.pennyAutotune).toBe(false)
    const [url, init] = api.mock.calls[0] as unknown as [string, { method: string; body: string }]
    expect(url).toBe('/api/settings')
    expect(init.method).toBe('PATCH')
    expect(JSON.parse(init.body)).toEqual({ penny_autotune: false })
    await act(async () => {})
    expect(r.opts.showToast).not.toHaveBeenCalled()
  })

  it('toggleAutotune reverts and toasts on failure', async () => {
    api.mockRejectedValue(new Error('x'))
    const r = setup('home')
    act(() => r.result.current.toggleAutotune())
    await waitFor(() => expect(r.opts.showToast).toHaveBeenCalledWith('Something went wrong. Try again.', 'error'))
    expect(r.result.current.pennyAutotune).toBe(true)
  })

  it('toggleStoreOrder flips, PATCHes, and reverts on failure', async () => {
    api.mockResolvedValueOnce({} as never)
    const r = setup('home')
    act(() => r.result.current.toggleStoreOrder())
    expect(r.result.current.storeOrderEnabled).toBe(false)
    expect(JSON.parse((api.mock.calls[0][1] as { body: string }).body)).toEqual({ store_order_enabled: false })
    await act(async () => {})
    api.mockRejectedValueOnce(new Error('x'))
    act(() => r.result.current.toggleStoreOrder())
    expect(r.result.current.storeOrderEnabled).toBe(true)
    await waitFor(() => expect(r.opts.showToast).toHaveBeenCalled())
    expect(r.result.current.storeOrderEnabled).toBe(false)
  })
})

describe('useSettingsState invite', () => {
  it('opens the invite sheet', () => {
    const r = setup('home')
    act(() => r.result.current.handleOpenInviteSheet())
    expect(r.result.current.showInviteSheet).toBe(true)
  })

  it('toasts when no link comes back, and clears loading', async () => {
    for (const bad of [null, [], {}]) {
      api.mockResolvedValueOnce(bad as never)
      const r = setup('home')
      await act(async () => { await r.result.current.handleShareInviteLink() })
      expect(r.opts.showToast).toHaveBeenCalledWith("Couldn't create invite link — please try again", 'error')
      expect(r.result.current.inviteLoading).toBe(false)
    }
  })

  it('toasts a connection error when the request throws', async () => {
    api.mockRejectedValue(new Error('x'))
    const r = setup('home')
    await act(async () => { await r.result.current.handleShareInviteLink() })
    expect(r.opts.showToast).toHaveBeenCalledWith(expect.stringContaining("Couldn't connect"), 'error')
    expect(r.result.current.inviteLoading).toBe(false)
  })

  it('uses native share when available and closes the sheet', async () => {
    api.mockResolvedValue({ link: 'https://g/j/1' } as never)
    const share = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'share', { value: share, configurable: true })
    const r = setup('home')
    act(() => r.result.current.handleOpenInviteSheet())
    await act(async () => { await r.result.current.handleShareInviteLink() })
    expect(api).toHaveBeenCalledWith('/api/settings/invite', expect.objectContaining({ method: 'POST' }))
    expect(share).toHaveBeenCalledWith(expect.objectContaining({ url: 'https://g/j/1', text: expect.stringContaining('https://g/j/1') }))
    expect(trackMock).toHaveBeenCalledWith('invite_shared', { method: 'native_share' })
    expect(trackMock).toHaveBeenCalledTimes(1)
    expect(r.result.current.showInviteSheet).toBe(false)
  })

  it('falls back to the clipboard when native share is cancelled', async () => {
    api.mockResolvedValue({ link: 'L' } as never)
    Object.defineProperty(navigator, 'share', { value: vi.fn().mockRejectedValue(new Error('AbortError')), configurable: true })
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const r = setup('home')
    act(() => r.result.current.handleOpenInviteSheet())
    await act(async () => { await r.result.current.handleShareInviteLink() })
    expect(writeText).toHaveBeenCalledWith('L')
    expect(r.opts.showToast).toHaveBeenCalledWith('Invite link copied!')
    expect(trackMock.mock.calls.map(c => (c[1] as { method: string }).method)).toEqual(['native_share', 'clipboard'])
    expect(r.result.current.showInviteSheet).toBe(false)
  })

  it('copies to the clipboard when share is unavailable', async () => {
    api.mockResolvedValue({ link: 'L' } as never)
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const r = setup('home')
    await act(async () => { await r.result.current.handleShareInviteLink() })
    expect(writeText).toHaveBeenCalledWith('L')
    expect(trackMock).toHaveBeenCalledTimes(1)
    expect(trackMock).toHaveBeenCalledWith('invite_shared', { method: 'clipboard' })
  })

  it('shows a copy prompt when the clipboard is blocked', async () => {
    api.mockResolvedValue({ link: 'L' } as never)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText: vi.fn().mockRejectedValue(new Error('denied')) }, configurable: true })
    const prompt = vi.spyOn(window, 'prompt').mockImplementation(() => null)
    const r = setup('home')
    act(() => r.result.current.handleOpenInviteSheet())
    await act(async () => { await r.result.current.handleShareInviteLink() })
    expect(prompt).toHaveBeenCalledWith('Copy this invite link', 'L')
    expect(r.result.current.showInviteSheet).toBe(false)
  })
})
