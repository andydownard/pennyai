import { render, screen, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useOnScreen } from '../hooks/useOnScreen'

// Store mock IntersectionObserver instances for test control
interface MockIOInstance {
  observe: ReturnType<typeof vi.fn>
  unobserve: ReturnType<typeof vi.fn>
  disconnect: ReturnType<typeof vi.fn>
  triggerCallback: (entries: Partial<IntersectionObserverEntry>[]) => void
  options: IntersectionObserverInit | undefined
}

let mockObserverInstances: MockIOInstance[]

// Helper component that uses the hook
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

describe('useOnScreen hook', () => {
  let originalIO: typeof IntersectionObserver

  beforeEach(() => {
    mockObserverInstances = []
    originalIO = window.IntersectionObserver

    // Use a proper class so it can be called with `new`
    // The test-setup already set this property as writable, so direct assignment works
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

  // ===== INITIAL STATE =====
  describe('initial state', () => {
    it('returns visible as false initially', () => {
      render(<TestComponent />)
      expect(screen.getByTestId('visibility')).toHaveTextContent('hidden')
    })

    it('returns a ref object', () => {
      render(<TestComponent />)
      expect(screen.getByTestId('observed-element')).toBeInTheDocument()
    })
  })

  // ===== OBSERVER SETUP =====
  describe('IntersectionObserver setup', () => {
    it('creates an IntersectionObserver', () => {
      render(<TestComponent />)
      expect(mockObserverInstances.length).toBe(1)
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
  })

  // ===== VISIBILITY CHANGE =====
  describe('visibility changes', () => {
    it('sets visible to true when element is intersecting', () => {
      render(<TestComponent />)
      expect(screen.getByTestId('visibility')).toHaveTextContent('hidden')

      act(() => {
        mockObserverInstances[0].triggerCallback([{ isIntersecting: true }])
      })

      expect(screen.getByTestId('visibility')).toHaveTextContent('visible')
    })

    it('does not set visible when element is not intersecting', () => {
      render(<TestComponent />)

      act(() => {
        mockObserverInstances[0].triggerCallback([{ isIntersecting: false }])
      })

      expect(screen.getByTestId('visibility')).toHaveTextContent('hidden')
    })

    it('stays visible once set (does not toggle back)', () => {
      render(<TestComponent />)

      act(() => {
        mockObserverInstances[0].triggerCallback([{ isIntersecting: true }])
      })

      expect(screen.getByTestId('visibility')).toHaveTextContent('visible')
      expect(mockObserverInstances[0].disconnect).toHaveBeenCalled()
    })

    it('disconnects observer when element becomes visible', () => {
      render(<TestComponent />)

      act(() => {
        mockObserverInstances[0].triggerCallback([{ isIntersecting: true }])
      })

      expect(mockObserverInstances[0].disconnect).toHaveBeenCalledOnce()
    })
  })

  // ===== CLEANUP =====
  describe('cleanup', () => {
    it('disconnects observer on unmount', () => {
      const { unmount } = render(<TestComponent />)
      unmount()
      expect(mockObserverInstances[0].disconnect).toHaveBeenCalled()
    })

    it('disconnects observer on unmount even if not yet visible', () => {
      const { unmount } = render(<TestComponent />)
      unmount()
      expect(mockObserverInstances[0].disconnect).toHaveBeenCalled()
    })
  })

  // ===== EDGE CASES =====
  describe('edge cases', () => {
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
      expect(screen.getByTestId('first')).toHaveTextContent('first-hidden')
      expect(screen.getByTestId('second')).toHaveTextContent('second-hidden')

      act(() => {
        mockObserverInstances[0].triggerCallback([{ isIntersecting: true }])
      })

      expect(screen.getByTestId('first')).toHaveTextContent('first-visible')
      expect(screen.getByTestId('second')).toHaveTextContent('second-hidden')
    })

    it('second instance can become visible independently', () => {
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

      act(() => {
        mockObserverInstances[1].triggerCallback([{ isIntersecting: true }])
      })

      expect(screen.getByTestId('first')).toHaveTextContent('first-hidden')
      expect(screen.getByTestId('second')).toHaveTextContent('second-visible')
    })

    it('both instances can become visible', () => {
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

      act(() => {
        mockObserverInstances[0].triggerCallback([{ isIntersecting: true }])
        mockObserverInstances[1].triggerCallback([{ isIntersecting: true }])
      })

      expect(screen.getByTestId('first')).toHaveTextContent('first-visible')
      expect(screen.getByTestId('second')).toHaveTextContent('second-visible')
    })
  })

  // ===== THRESHOLD VARIATIONS =====
  describe('threshold variations', () => {
    it('accepts threshold of 0', () => {
      render(<TestComponent threshold={0} />)
      expect(mockObserverInstances[0].options?.threshold).toBe(0)
    })

    it('accepts threshold of 1', () => {
      render(<TestComponent threshold={1} />)
      expect(mockObserverInstances[0].options?.threshold).toBe(1)
    })

    it('accepts threshold of 0.1', () => {
      render(<TestComponent threshold={0.1} />)
      expect(mockObserverInstances[0].options?.threshold).toBe(0.1)
    })
  })

  // ===== OBSERVER LIFECYCLE =====
  describe('observer lifecycle', () => {
    it('creates exactly one observer per component mount', () => {
      render(<TestComponent />)
      expect(mockObserverInstances.length).toBe(1)
    })

    it('does not create extra observers on re-render', () => {
      const { rerender } = render(<TestComponent threshold={0.1} />)
      expect(mockObserverInstances.length).toBe(1)
      rerender(<TestComponent threshold={0.1} />)
      // Hook deps don't change, so no new observer
      expect(mockObserverInstances.length).toBe(1)
    })

    it('observe is called with the DOM element', () => {
      render(<TestComponent />)
      const observed = mockObserverInstances[0].observe.mock.calls[0][0]
      expect(observed).toBeInstanceOf(HTMLElement)
    })

    it('disconnect is not called before visibility or unmount', () => {
      render(<TestComponent />)
      // Not yet visible, not yet unmounted
      expect(mockObserverInstances[0].disconnect).not.toHaveBeenCalled()
    })
  })
})
