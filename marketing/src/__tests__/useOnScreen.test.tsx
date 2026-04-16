import { render, screen, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useOnScreen } from '../hooks/useOnScreen'

// useOnScreen now defaults to `visible = true` and only flips to `false` (and
// registers an IntersectionObserver) when the element is initially below the
// viewport. This was the fix for the marketing-site whitespace bug where
// elements rendered at the top of the page never received an IO callback and
// the page stayed blank for 15,731px.
//
// JSDOM's default getBoundingClientRect returns {top: 0, ...} — i.e. above the
// fold — so by default the hook short-circuits and never creates an observer.
// To exercise the observer code path we must override the rect mock.

interface MockIOInstance {
  observe: ReturnType<typeof vi.fn>
  unobserve: ReturnType<typeof vi.fn>
  disconnect: ReturnType<typeof vi.fn>
  triggerCallback: (entries: Partial<IntersectionObserverEntry>[]) => void
  options: IntersectionObserverInit | undefined
}

let mockObserverInstances: MockIOInstance[]

function TestComponent({ threshold }: { threshold?: number }) {
  const [ref, visible] = useOnScreen(threshold)
  return (
    <div>
      <div ref={ref as React.RefObject<HTMLDivElement>} data-testid="observed-element">
        Observed Element
      </div>
      <div data-testid="visibility">{visible ? 'visible' : 'hidden'}</div>
    </div>
  )
}

// Helper: stub Element.prototype.getBoundingClientRect to simulate the element
// sitting below the viewport — that's the only path that creates an observer.
function stubBelowFold() {
  const original = Element.prototype.getBoundingClientRect
  Element.prototype.getBoundingClientRect = vi.fn(() => ({
    top: window.innerHeight + 200,
    left: 0,
    right: 0,
    bottom: window.innerHeight + 300,
    width: 0,
    height: 100,
    x: 0,
    y: window.innerHeight + 200,
    toJSON: () => ({}),
  })) as typeof Element.prototype.getBoundingClientRect
  return () => { Element.prototype.getBoundingClientRect = original }
}

describe('useOnScreen hook', () => {
  let originalIO: typeof IntersectionObserver

  beforeEach(() => {
    mockObserverInstances = []
    originalIO = window.IntersectionObserver
    window.IntersectionObserver = class MockIntersectionObserver {
      _observe = vi.fn()
      _unobserve = vi.fn()
      _disconnect = vi.fn()
      _callback: IntersectionObserverCallback
      _options: IntersectionObserverInit | undefined
      constructor(callback: IntersectionObserverCallback, options?: IntersectionObserverInit) {
        this._callback = callback
        this._options = options
        const instance: MockIOInstance = {
          observe: this._observe,
          unobserve: this._unobserve,
          disconnect: this._disconnect,
          triggerCallback: (entries: Partial<IntersectionObserverEntry>[]) => {
            this._callback(entries as IntersectionObserverEntry[], this as unknown as IntersectionObserver)
          },
          options: this._options,
        }
        mockObserverInstances.push(instance)
      }
      observe(...args: Parameters<IntersectionObserver['observe']>) { this._observe(...args) }
      unobserve(...args: Parameters<IntersectionObserver['unobserve']>) { this._unobserve(...args) }
      disconnect() { this._disconnect() }
      takeRecords() { return [] }
      get root() { return null }
      get rootMargin() { return '' }
      get thresholds() { return [] }
    } as unknown as typeof IntersectionObserver
  })

  afterEach(() => {
    window.IntersectionObserver = originalIO
    vi.restoreAllMocks()
  })

  // ===== ABOVE-FOLD DEFAULT (the regression case) =====
  describe('above-fold elements', () => {
    it('REGRESSION: defaults to visible (no fade-in flicker for top-of-page sections)', () => {
      // JSDOM rect.top = 0, which is on-screen. The hook must short-circuit.
      render(<TestComponent />)
      expect(screen.getByTestId('visibility')).toHaveTextContent('visible')
    })

    it('REGRESSION: does not create an IntersectionObserver for on-screen elements', () => {
      // The original bug: every section registered an observer that never
      // fired for elements already on-screen, leaving them invisible forever.
      render(<TestComponent />)
      expect(mockObserverInstances.length).toBe(0)
    })

    it('returns a usable ref object', () => {
      render(<TestComponent />)
      expect(screen.getByTestId('observed-element')).toBeInTheDocument()
    })
  })

  // ===== BELOW-FOLD: observer is registered =====
  describe('below-fold elements', () => {
    let restoreRect: () => void
    beforeEach(() => { restoreRect = stubBelowFold() })
    afterEach(() => restoreRect())

    it('creates an IntersectionObserver', () => {
      render(<TestComponent />)
      expect(mockObserverInstances.length).toBe(1)
    })

    it('starts hidden until observer fires', () => {
      render(<TestComponent />)
      expect(screen.getByTestId('visibility')).toHaveTextContent('hidden')
    })

    it('observes the ref element', () => {
      render(<TestComponent />)
      expect(mockObserverInstances[0].observe).toHaveBeenCalledOnce()
      const observedEl = mockObserverInstances[0].observe.mock.calls[0][0]
      expect(observedEl).toBe(screen.getByTestId('observed-element'))
    })

    it('uses default threshold of 0.05', () => {
      render(<TestComponent />)
      expect(mockObserverInstances[0].options?.threshold).toBe(0.05)
    })

    it('uses custom threshold when provided', () => {
      render(<TestComponent threshold={0.5} />)
      expect(mockObserverInstances[0].options?.threshold).toBe(0.5)
    })

    it('sets rootMargin to 0px 0px -40px 0px', () => {
      render(<TestComponent />)
      expect(mockObserverInstances[0].options?.rootMargin).toBe('0px 0px -40px 0px')
    })

    it('flips to visible when observer reports intersecting', () => {
      render(<TestComponent />)
      expect(screen.getByTestId('visibility')).toHaveTextContent('hidden')
      act(() => {
        mockObserverInstances[0].triggerCallback([{ isIntersecting: true }])
      })
      expect(screen.getByTestId('visibility')).toHaveTextContent('visible')
    })

    it('stays hidden when observer reports not-intersecting', () => {
      render(<TestComponent />)
      act(() => {
        mockObserverInstances[0].triggerCallback([{ isIntersecting: false }])
      })
      expect(screen.getByTestId('visibility')).toHaveTextContent('hidden')
    })

    it('disconnects observer when element becomes visible', () => {
      render(<TestComponent />)
      act(() => {
        mockObserverInstances[0].triggerCallback([{ isIntersecting: true }])
      })
      expect(mockObserverInstances[0].disconnect).toHaveBeenCalledOnce()
    })

    it('disconnects observer on unmount even if not yet visible', () => {
      const { unmount } = render(<TestComponent />)
      unmount()
      expect(mockObserverInstances[0].disconnect).toHaveBeenCalled()
    })
  })

  // ===== INSTANCE INDEPENDENCE (below-fold) =====
  describe('multiple instances (below-fold)', () => {
    let restoreRect: () => void
    beforeEach(() => { restoreRect = stubBelowFold() })
    afterEach(() => restoreRect())

    it('handles multiple instances independently', () => {
      function MultiTest() {
        const [firstRef, firstVisible] = useOnScreen()
        const [secondRef, secondVisible] = useOnScreen()
        return (
          <div>
            <div ref={firstRef as React.RefObject<HTMLDivElement>} data-testid="first">
              {firstVisible ? 'first-visible' : 'first-hidden'}
            </div>
            <div ref={secondRef as React.RefObject<HTMLDivElement>} data-testid="second">
              {secondVisible ? 'second-visible' : 'second-hidden'}
            </div>
          </div>
        )
      }
      render(<MultiTest />)
      expect(mockObserverInstances.length).toBe(2)
      act(() => {
        mockObserverInstances[0].triggerCallback([{ isIntersecting: true }])
      })
      expect(screen.getByTestId('first')).toHaveTextContent('first-visible')
      expect(screen.getByTestId('second')).toHaveTextContent('second-hidden')
    })
  })
})
