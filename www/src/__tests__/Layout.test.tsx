import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Layout from '../components/Layout'

// We need to mock sub-components that are imported by Layout
// to keep tests focused on Layout's own behavior
vi.mock('../components/Footer', () => ({
  default: () => <footer data-testid="footer">Footer Mock</footer>,
}))

vi.mock('../components/FeedbackWidget', () => ({
  default: () => <div data-testid="feedback-widget">Feedback Mock</div>,
}))

vi.mock('../components/Header', () => ({
  default: () => <header data-testid="header">Header Mock</header>,
}))

function renderLayout(initialRoute = '/') {
  return render(
    <MemoryRouter initialEntries={[initialRoute]}>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<div data-testid="home-page">Home Content</div>} />
          <Route path="/test" element={<div data-testid="test-page">Test Content</div>} />
          <Route path="/hash-test" element={<div data-testid="hash-page">Hash Content</div>} />
        </Route>
      </Routes>
    </MemoryRouter>
  )
}

describe('Layout component', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  // ===== RENDERING =====
  describe('renders without errors', () => {
    it('renders the layout without crashing', () => {
      renderLayout()
      expect(screen.getByTestId('home-page')).toBeInTheDocument()
    })
  })

  // ===== STRUCTURE =====
  describe('structural components', () => {
    it('renders Header component', () => {
      renderLayout()
      expect(screen.getByTestId('header')).toBeInTheDocument()
    })

    it('renders Footer component', () => {
      renderLayout()
      expect(screen.getByTestId('footer')).toBeInTheDocument()
    })

    it('renders FeedbackWidget component', () => {
      renderLayout()
      expect(screen.getByTestId('feedback-widget')).toBeInTheDocument()
    })

    it('renders child route content via Outlet', () => {
      renderLayout()
      expect(screen.getByText('Home Content')).toBeInTheDocument()
    })

    it('wraps child content in a main element', () => {
      renderLayout()
      const main = document.querySelector('main')
      expect(main).toBeInTheDocument()
      expect(main?.textContent).toContain('Home Content')
    })

    it('renders child content inside main between header and footer', () => {
      renderLayout()
      const header = screen.getByTestId('header')
      const main = document.querySelector('main')!
      const footer = screen.getByTestId('footer')

      // Check DOM order: header, then main, then footer
      const allElements = document.querySelectorAll('header, main, footer')
      expect(allElements[0]).toBe(header)
      expect(allElements[1]).toBe(main)
      expect(allElements[2]).toBe(footer)
    })
  })

  // ===== ROUTE RENDERING =====
  describe('route rendering', () => {
    it('renders home page at root route', () => {
      renderLayout('/')
      expect(screen.getByTestId('home-page')).toBeInTheDocument()
    })

    it('renders test page at /test route', () => {
      renderLayout('/test')
      expect(screen.getByTestId('test-page')).toBeInTheDocument()
    })
  })

  // ===== SCROLL TO HASH BEHAVIOR =====
  describe('ScrollToHash behavior', () => {
    it('calls window.scrollTo on route without hash', () => {
      const scrollSpy = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
      renderLayout('/')
      expect(scrollSpy).toHaveBeenCalledWith(0, 0)
      scrollSpy.mockRestore()
    })

    it('attempts to find element and scroll to hash', () => {
      const querySpy = vi.spyOn(document, 'querySelector')
      vi.useFakeTimers()
      renderLayout('/hash-test#section')
      vi.advanceTimersByTime(200)
      // querySelector should have been called with the hash
      const hashCalls = querySpy.mock.calls.filter(c => c[0] === '#section')
      expect(hashCalls.length).toBeGreaterThan(0)
      vi.useRealTimers()
      querySpy.mockRestore()
    })
  })

  // ===== GA4 TRACKING =====
  describe('GA4 page view tracking', () => {
    it('calls gtag page_view event when gtag exists', () => {
      const gtagMock = vi.fn()
      ;(window as unknown as { gtag: typeof gtagMock }).gtag = gtagMock
      renderLayout('/')
      const pageViewCalls = gtagMock.mock.calls.filter(
        c => c[0] === 'event' && c[1] === 'page_view'
      )
      expect(pageViewCalls.length).toBeGreaterThan(0)
      delete (window as unknown as { gtag?: unknown }).gtag
    })

    it('does not throw when gtag is not defined', () => {
      delete (window as unknown as { gtag?: unknown }).gtag
      expect(() => renderLayout('/')).not.toThrow()
    })

    it('includes page_path in gtag event', () => {
      const gtagMock = vi.fn()
      ;(window as unknown as { gtag: typeof gtagMock }).gtag = gtagMock
      renderLayout('/test')
      const pageViewCalls = gtagMock.mock.calls.filter(
        c => c[0] === 'event' && c[1] === 'page_view'
      )
      expect(pageViewCalls.length).toBeGreaterThan(0)
      const eventData = pageViewCalls[0][2]
      expect(eventData.page_path).toBe('/test')
      delete (window as unknown as { gtag?: unknown }).gtag
    })

    it('includes page_title in gtag event', () => {
      const gtagMock = vi.fn()
      ;(window as unknown as { gtag: typeof gtagMock }).gtag = gtagMock
      renderLayout('/')
      const pageViewCalls = gtagMock.mock.calls.filter(
        c => c[0] === 'event' && c[1] === 'page_view'
      )
      const eventData = pageViewCalls[0][2]
      expect(eventData.page_title).toBeDefined()
      delete (window as unknown as { gtag?: unknown }).gtag
    })
  })

  // ===== LAYOUT ORDER =====
  describe('element rendering order', () => {
    it('renders ScrollToHash (no visible output)', () => {
      renderLayout()
      // ScrollToHash returns null, so it doesn't produce DOM.
      // Verifying it doesn't break rendering.
      expect(screen.getByTestId('home-page')).toBeInTheDocument()
    })

    it('header appears before main in DOM', () => {
      renderLayout()
      const header = screen.getByTestId('header')
      const main = document.querySelector('main')!
      // compareDocumentPosition: 4 means header is before main
      expect(header.compareDocumentPosition(main) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    })

    it('main appears before footer in DOM', () => {
      renderLayout()
      const main = document.querySelector('main')!
      const footer = screen.getByTestId('footer')
      expect(main.compareDocumentPosition(footer) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    })

    it('FeedbackWidget is rendered after Footer', () => {
      renderLayout()
      const footer = screen.getByTestId('footer')
      const widget = screen.getByTestId('feedback-widget')
      expect(footer.compareDocumentPosition(widget) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    })
  })

  // ===== HASH VALIDATION =====
  describe('hash validation', () => {
    it('does not attempt to scroll for invalid hash selectors', () => {
      const querySpy = vi.spyOn(document, 'querySelector')
      vi.useFakeTimers()
      // Hash with special characters that aren't valid CSS selectors
      renderLayout('/hash-test#123invalid')
      vi.advanceTimersByTime(200)
      // Should not call querySelector with invalid hash
      const hashCalls = querySpy.mock.calls.filter(c => c[0] === '#123invalid')
      expect(hashCalls.length).toBe(0)
      vi.useRealTimers()
      querySpy.mockRestore()
    })
  })

  // ===== MULTIPLE ROUTE CHANGES =====
  describe('route change behavior', () => {
    it('scrolls to top on each route navigation', () => {
      const scrollSpy = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
      renderLayout('/')
      expect(scrollSpy).toHaveBeenCalledWith(0, 0)
      scrollSpy.mockRestore()
    })

    it('different routes render different page content', () => {
      const { unmount } = renderLayout('/')
      expect(screen.getByTestId('home-page')).toBeInTheDocument()
      unmount()

      renderLayout('/test')
      expect(screen.getByTestId('test-page')).toBeInTheDocument()
    })

    it('layout components persist across routes', () => {
      const { unmount } = renderLayout('/')
      expect(screen.getByTestId('header')).toBeInTheDocument()
      expect(screen.getByTestId('footer')).toBeInTheDocument()
      unmount()

      renderLayout('/test')
      expect(screen.getByTestId('header')).toBeInTheDocument()
      expect(screen.getByTestId('footer')).toBeInTheDocument()
    })
  })

  // ===== GA4 TRACKING DETAILS =====
  describe('GA4 tracking details', () => {
    it('sends page_view event with correct page_path for root', () => {
      const gtagMock = vi.fn()
      ;(window as unknown as { gtag: typeof gtagMock }).gtag = gtagMock
      renderLayout('/')
      const pageViewCalls = gtagMock.mock.calls.filter(
        c => c[0] === 'event' && c[1] === 'page_view'
      )
      expect(pageViewCalls[0][2].page_path).toBe('/')
      delete (window as unknown as { gtag?: unknown }).gtag
    })

    it('sends page_view event with correct page_path for /test', () => {
      const gtagMock = vi.fn()
      ;(window as unknown as { gtag: typeof gtagMock }).gtag = gtagMock
      renderLayout('/test')
      const pageViewCalls = gtagMock.mock.calls.filter(
        c => c[0] === 'event' && c[1] === 'page_view'
      )
      expect(pageViewCalls[0][2].page_path).toBe('/test')
      delete (window as unknown as { gtag?: unknown }).gtag
    })

    it('gtag event uses "event" as first argument', () => {
      const gtagMock = vi.fn()
      ;(window as unknown as { gtag: typeof gtagMock }).gtag = gtagMock
      renderLayout('/')
      const firstCall = gtagMock.mock.calls.find(c => c[1] === 'page_view')
      expect(firstCall?.[0]).toBe('event')
      delete (window as unknown as { gtag?: unknown }).gtag
    })
  })

  // ===== COMPONENT INTEGRATION =====
  describe('component integration', () => {
    it('renders all 4 layout components (Header, main, Footer, FeedbackWidget)', () => {
      renderLayout()
      expect(screen.getByTestId('header')).toBeInTheDocument()
      expect(document.querySelector('main')).toBeInTheDocument()
      expect(screen.getByTestId('footer')).toBeInTheDocument()
      expect(screen.getByTestId('feedback-widget')).toBeInTheDocument()
    })

    it('main element contains the route content', () => {
      renderLayout('/')
      const main = document.querySelector('main')
      expect(main?.querySelector('[data-testid="home-page"]')).toBeInTheDocument()
    })

    it('main element does not contain header or footer', () => {
      renderLayout()
      const main = document.querySelector('main')
      expect(main?.querySelector('[data-testid="header"]')).not.toBeInTheDocument()
      expect(main?.querySelector('[data-testid="footer"]')).not.toBeInTheDocument()
    })

    it('FeedbackWidget is not inside main', () => {
      renderLayout()
      const main = document.querySelector('main')
      expect(main?.querySelector('[data-testid="feedback-widget"]')).not.toBeInTheDocument()
    })
  })

  // ===== EDGE CASES =====
  describe('edge cases', () => {
    it('handles routes with hash fragments', () => {
      renderLayout('/#features')
      expect(screen.getByTestId('home-page')).toBeInTheDocument()
    })

    it('handles routes with query parameters', () => {
      renderLayout('/?utm_source=test')
      expect(screen.getByTestId('home-page')).toBeInTheDocument()
    })

    it('renders without gtag defined (no error)', () => {
      delete (window as unknown as { gtag?: unknown }).gtag
      expect(() => renderLayout('/')).not.toThrow()
    })
  })
})
