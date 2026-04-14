import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import ErrorBoundary from '../components/ErrorBoundary'

// A component that throws an error when shouldThrow is true
function ThrowingComponent({ shouldThrow }: { shouldThrow: boolean }) {
  if (shouldThrow) {
    throw new Error('Test error')
  }
  return <div data-testid="child-content">Child works fine</div>
}

describe('ErrorBoundary component', () => {
  beforeEach(() => {
    // Suppress React error boundary console errors in test output
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  // ===== RENDERING =====
  describe('renders without errors', () => {
    it('renders children when no error occurs', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={false} />
        </ErrorBoundary>
      )
      expect(screen.getByTestId('child-content')).toBeInTheDocument()
      expect(screen.getByText('Child works fine')).toBeInTheDocument()
    })

    it('does not show error UI when children render normally', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={false} />
        </ErrorBoundary>
      )
      expect(screen.queryByText('Something went wrong')).not.toBeInTheDocument()
    })
  })

  // ===== ERROR STATE =====
  describe('error state', () => {
    it('catches errors and displays error UI', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      expect(screen.getByText('Something went wrong')).toBeInTheDocument()
    })

    it('displays helpful error message', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      expect(
        screen.getByText('We hit an unexpected error. Please try refreshing the page.')
      ).toBeInTheDocument()
    })

    it('displays a Refresh Page button', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      const refreshBtn = screen.getByText('Refresh Page')
      expect(refreshBtn).toBeInTheDocument()
      expect(refreshBtn.classList.contains('btn-primary')).toBe(true)
    })

    it('does not render children when in error state', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      expect(screen.queryByTestId('child-content')).not.toBeInTheDocument()
    })

    it('renders error heading with brand green color', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      const heading = screen.getByText('Something went wrong')
      expect(heading.style.color).toBe('var(--green-600)')
    })

    it('centers the error content', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      const wrapper = screen.getByText('Something went wrong').parentElement
      expect(wrapper?.style.textAlign).toBe('center')
    })
  })

  // ===== REFRESH BUTTON =====
  describe('refresh button behavior', () => {
    it('calls window.location.reload on click', () => {
      const reloadMock = vi.fn()
      Object.defineProperty(window, 'location', {
        value: { ...window.location, reload: reloadMock },
        writable: true,
      })

      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )

      const refreshBtn = screen.getByText('Refresh Page')
      refreshBtn.click()
      expect(reloadMock).toHaveBeenCalledOnce()
    })
  })

  // ===== EDGE CASES =====
  describe('edge cases', () => {
    it('renders multiple children without error', () => {
      render(
        <ErrorBoundary>
          <div data-testid="child-1">First</div>
          <div data-testid="child-2">Second</div>
        </ErrorBoundary>
      )
      expect(screen.getByTestId('child-1')).toBeInTheDocument()
      expect(screen.getByTestId('child-2')).toBeInTheDocument()
    })

    it('renders with no children without crashing', () => {
      render(<ErrorBoundary>{null}</ErrorBoundary>)
      expect(screen.queryByText('Something went wrong')).not.toBeInTheDocument()
    })

    it('renders with text children', () => {
      render(<ErrorBoundary>Just some text</ErrorBoundary>)
      expect(screen.getByText('Just some text')).toBeInTheDocument()
    })
  })

  // ===== BRAND CONSISTENCY =====
  describe('brand consistency', () => {
    it('uses plain language in the error message (no jargon)', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      const heading = screen.getByText('Something went wrong')
      expect(heading.textContent).not.toMatch(/\bexception\b/i)
      expect(heading.textContent).not.toMatch(/\bfatal\b/i)

      const message = screen.getByText(/unexpected error/)
      expect(message.textContent).not.toMatch(/\bstack trace\b/i)
    })

    it('does not expose the actual error message to users', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      expect(screen.queryByText('Test error')).not.toBeInTheDocument()
    })
  })

  // ===== STYLING =====
  describe('error UI styling', () => {
    it('has appropriate padding', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      const wrapper = screen.getByText('Something went wrong').parentElement!
      expect(wrapper.style.padding).toBe('10rem 2rem 6rem')
    })

    it('heading has 2rem font size', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      const heading = screen.getByText('Something went wrong')
      expect(heading.style.fontSize).toBe('2rem')
    })

    it('heading has font weight 700', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      const heading = screen.getByText('Something went wrong')
      expect(heading.style.fontWeight).toBe('700')
    })

    it('description text uses gray-600 color', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      const description = screen.getByText(/unexpected error/)
      expect(description.style.color).toBe('var(--gray-600)')
    })

    it('description has bottom margin for spacing', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      const description = screen.getByText(/unexpected error/)
      expect(description.style.marginBottom).toBe('0.5rem')
    })
  })

  // ===== CLASS-BASED COMPONENT BEHAVIOR =====
  describe('class component lifecycle', () => {
    it('getDerivedStateFromError sets hasError to true', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      // If it renders the error UI, getDerivedStateFromError worked
      expect(screen.getByText('Something went wrong')).toBeInTheDocument()
    })

    it('stores the error object in state', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      // The error is captured (evidenced by rendering error UI instead of child)
      expect(screen.queryByTestId('child-content')).not.toBeInTheDocument()
    })
  })

  // ===== ERROR UI STRUCTURE =====
  describe('error UI structure', () => {
    it('error heading is an h1 element', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      const heading = screen.getByText('Something went wrong')
      expect(heading.tagName).toBe('H1')
    })

    it('refresh button is a button element', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      const btn = screen.getByText('Refresh Page')
      expect(btn.tagName).toBe('BUTTON')
    })

    it('error description is a p element', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      const desc = screen.getByText(/unexpected error/)
      expect(desc.tagName).toBe('P')
    })

    it('all elements are inside a single wrapper div', () => {
      render(
        <ErrorBoundary>
          <ThrowingComponent shouldThrow={true} />
        </ErrorBoundary>
      )
      const heading = screen.getByText('Something went wrong')
      const desc = screen.getByText(/unexpected error/)
      const btn = screen.getByText('Refresh Page')
      expect(heading.parentElement).toBe(desc.parentElement)
      expect(desc.parentElement).toBe(btn.parentElement)
    })
  })

  // ===== NORMAL RENDERING EDGE CASES =====
  describe('normal rendering edge cases', () => {
    it('renders deeply nested children without error', () => {
      render(
        <ErrorBoundary>
          <div>
            <div>
              <div data-testid="deep-child">Deep</div>
            </div>
          </div>
        </ErrorBoundary>
      )
      expect(screen.getByTestId('deep-child')).toBeInTheDocument()
    })

    it('renders fragment children without error', () => {
      render(
        <ErrorBoundary>
          <>
            <div data-testid="frag-1">Fragment 1</div>
            <div data-testid="frag-2">Fragment 2</div>
          </>
        </ErrorBoundary>
      )
      expect(screen.getByTestId('frag-1')).toBeInTheDocument()
      expect(screen.getByTestId('frag-2')).toBeInTheDocument()
    })

    it('does not interfere with normal rendering', () => {
      const { container } = render(
        <ErrorBoundary>
          <div className="my-content">Normal Content</div>
        </ErrorBoundary>
      )
      expect(container.querySelector('.my-content')).toBeInTheDocument()
    })
  })
})
