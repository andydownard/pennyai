import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, renderHook, act } from '@testing-library/react'
import { ScreenTransition, useScreenTransition } from './ScreenTransition'

type S = Parameters<typeof useScreenTransition>[0]

function mockMotion(reduced: boolean) {
  window.matchMedia = vi.fn().mockImplementation((q: string) => ({
    matches: reduced, media: q, addEventListener: vi.fn(), removeEventListener: vi.fn(),
    addListener: vi.fn(), removeListener: vi.fn(), onchange: null, dispatchEvent: vi.fn(),
  })) as never
}

function run(from: S, to: S) {
  const hook = renderHook(({ s }) => useScreenTransition(s), { initialProps: { s: from } })
  hook.rerender({ s: to })
  return hook
}

describe('ScreenTransition component', () => {
  it('renders children in a full-size wrapper carrying the style', () => {
    render(<ScreenTransition style={{ opacity: 0.5 }}><p>hello</p></ScreenTransition>)
    const wrapper = screen.getByText('hello').parentElement!
    expect(wrapper).toHaveStyle({ opacity: '0.5', height: '100%', width: '100%' })
  })
})

describe('useScreenTransition', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockMotion(false)
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => setTimeout(() => cb(0), 0))
  })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  it('is static on first render', () => {
    const { result } = renderHook(() => useScreenTransition('home'))
    expect(result.current).toMatchObject({ opacity: 1 })
    expect(result.current.transition).toBeUndefined()
  })

  it('is static when the screen does not change', () => {
    const { result } = run('home', 'home')
    expect(result.current.transition).toBeUndefined()
  })

  it('fades between tab screens: starts transparent, ends opaque, then resets', () => {
    const { result } = run('home', 'profile')
    expect(result.current.opacity).toBe(0)
    expect(String(result.current.transition)).toContain('opacity 200ms')
    act(() => { vi.advanceTimersByTime(1) })
    expect(result.current.opacity).toBe(1)
    act(() => { vi.advanceTimersByTime(200) })
    expect(result.current.transition).toBeUndefined()
  })

  it('slides in from the right when drilling into a sub-screen', () => {
    const { result } = run('home', 'earnings')
    expect(result.current.transform).toBe('translate3d(30%, 0, 0)')
    expect(String(result.current.transition)).toContain('transform 250ms')
    act(() => { vi.advanceTimersByTime(1) })
    expect(result.current.transform).toBe('translate3d(0, 0, 0)')
  })

  it('slides in from the left when going back to the parent', () => {
    const { result } = run('earnings', 'home')
    expect(result.current.transform).toBe('translate3d(-30%, 0, 0)')
  })

  it('slides in between sibling sub-screens', () => {
    const { result } = run('earnings', 'receipts')
    expect(result.current.transform).toBe('translate3d(30%, 0, 0)')
  })

  it('fades from a sub-screen to an unrelated tab', () => {
    const { result } = run('help', 'home')
    expect(String(result.current.transition)).toContain('opacity 200ms')
    expect(result.current.transform).toBe('translate3d(0, 0, 0)')
  })

  it('fades for non-tab screens', () => {
    const { result } = run('login', 'saved-lists')
    expect(result.current.opacity).toBe(0)
  })

  it('skips animation when reduced motion is preferred', () => {
    mockMotion(true)
    const { result } = run('home', 'earnings')
    expect(result.current).toMatchObject({ opacity: 1, transform: 'translate3d(0, 0, 0)' })
    expect(result.current.transition).toBeUndefined()
  })
})
