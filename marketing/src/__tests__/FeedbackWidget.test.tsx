import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import FeedbackWidget from '../components/FeedbackWidget'

beforeEach(() => {
  localStorage.clear()
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('FeedbackWidget', () => {
  it('renders the lightbulb FAB', () => {
    render(<FeedbackWidget />)
    expect(screen.getByLabelText('Product feedback')).toBeInTheDocument()
  })

  it('opens the dialog on FAB click', async () => {
    const user = userEvent.setup()
    render(<FeedbackWidget />)
    await user.click(screen.getByLabelText('Product feedback'))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('shows phone verification gate when not authorized', async () => {
    const user = userEvent.setup()
    render(<FeedbackWidget />)
    await user.click(screen.getByLabelText('Product feedback'))
    expect(screen.getByText('Verify Your Number')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('(310) 486-1833')).toBeInTheDocument()
  })

  it('rejects unauthorized phone numbers', async () => {
    const user = userEvent.setup()
    render(<FeedbackWidget />)
    await user.click(screen.getByLabelText('Product feedback'))
    await user.type(screen.getByPlaceholderText('(310) 486-1833'), '5551234567')
    await user.click(screen.getByText('Verify'))
    expect(screen.getByText("That number isn't authorized for feedback.")).toBeInTheDocument()
  })

  it('accepts authorized phone number 3104861833', async () => {
    const user = userEvent.setup()
    render(<FeedbackWidget />)
    await user.click(screen.getByLabelText('Product feedback'))
    await user.type(screen.getByPlaceholderText('(310) 486-1833'), '3104861833')
    await user.click(screen.getByText('Verify'))
    expect(screen.getByText('Product Feedback')).toBeInTheDocument()
    expect(screen.getByPlaceholderText(/What's on your mind/)).toBeInTheDocument()
  })

  it('accepts authorized phone number 6264043580', async () => {
    const user = userEvent.setup()
    render(<FeedbackWidget />)
    await user.click(screen.getByLabelText('Product feedback'))
    await user.type(screen.getByPlaceholderText('(310) 486-1833'), '6264043580')
    await user.click(screen.getByText('Verify'))
    expect(screen.getByText('Product Feedback')).toBeInTheDocument()
  })

  it('accepts formatted phone number with dashes and parens', async () => {
    const user = userEvent.setup()
    render(<FeedbackWidget />)
    await user.click(screen.getByLabelText('Product feedback'))
    await user.type(screen.getByPlaceholderText('(310) 486-1833'), '(310) 486-1833')
    await user.click(screen.getByText('Verify'))
    expect(screen.getByText('Product Feedback')).toBeInTheDocument()
  })

  it('accepts phone number with +1 prefix', async () => {
    const user = userEvent.setup()
    render(<FeedbackWidget />)
    await user.click(screen.getByLabelText('Product feedback'))
    await user.type(screen.getByPlaceholderText('(310) 486-1833'), '+13104861833')
    await user.click(screen.getByText('Verify'))
    expect(screen.getByText('Product Feedback')).toBeInTheDocument()
  })

  it('persists authorization in localStorage', async () => {
    const user = userEvent.setup()
    const { unmount } = render(<FeedbackWidget />)
    await user.click(screen.getByLabelText('Product feedback'))
    await user.type(screen.getByPlaceholderText('(310) 486-1833'), '3104861833')
    await user.click(screen.getByText('Verify'))
    expect(localStorage.getItem('fb_phone')).toBe('3104861833')
    unmount()

    // Re-render — should skip phone gate
    render(<FeedbackWidget />)
    await user.click(screen.getByLabelText('Product feedback'))
    expect(screen.getByText('Product Feedback')).toBeInTheDocument()
  })

  it('shows I insist toggle defaulting to on', async () => {
    localStorage.setItem('fb_phone', '3104861833')
    const user = userEvent.setup()
    render(<FeedbackWidget />)
    await user.click(screen.getByLabelText('Product feedback'))
    expect(screen.getByText('I insist')).toBeInTheDocument()
  })

  it('closes dialog on backdrop click', async () => {
    localStorage.setItem('fb_phone', '3104861833')
    const user = userEvent.setup()
    render(<FeedbackWidget />)
    await user.click(screen.getByLabelText('Product feedback'))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    await user.click(document.querySelector('.fb-backdrop')!)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  // ===== FAB BUTTON =====
  describe('FAB button', () => {
    it('has aria-label "Product feedback"', () => {
      render(<FeedbackWidget />)
      const fab = screen.getByLabelText('Product feedback')
      expect(fab.tagName).toBe('BUTTON')
    })

    it('has title attribute "Product feedback"', () => {
      render(<FeedbackWidget />)
      const fab = screen.getByLabelText('Product feedback')
      expect(fab).toHaveAttribute('title', 'Product feedback')
    })

    it('has fb-fab class for styling', () => {
      render(<FeedbackWidget />)
      const fab = document.querySelector('.fb-fab')
      expect(fab).toBeInTheDocument()
    })

    it('renders a lightbulb SVG icon', () => {
      render(<FeedbackWidget />)
      const fab = document.querySelector('.fb-fab')
      const svg = fab?.querySelector('svg')
      expect(svg).toBeInTheDocument()
      expect(svg?.getAttribute('width')).toBe('16')
      expect(svg?.getAttribute('height')).toBe('16')
    })

    it('FAB is hidden when dialog is open', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(document.querySelector('.fb-fab')).not.toBeInTheDocument()
    })

    it('FAB reappears when dialog is closed', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(document.querySelector('.fb-fab')).not.toBeInTheDocument()
      await user.click(document.querySelector('.fb-backdrop')!)
      expect(document.querySelector('.fb-fab')).toBeInTheDocument()
    })
  })

  // ===== DIALOG ATTRIBUTES =====
  describe('dialog attributes', () => {
    it('dialog has role="dialog"', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      const dialog = screen.getByRole('dialog')
      expect(dialog).toBeInTheDocument()
    })

    it('dialog has aria-modal="true"', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      const dialog = screen.getByRole('dialog')
      expect(dialog).toHaveAttribute('aria-modal', 'true')
    })

    it('dialog has aria-label "Product feedback"', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      const dialog = screen.getByRole('dialog')
      expect(dialog).toHaveAttribute('aria-label', 'Product feedback')
    })

    it('backdrop has fb-backdrop class', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(document.querySelector('.fb-backdrop')).toBeInTheDocument()
    })
  })

  // ===== PHONE VERIFICATION DETAILS =====
  describe('phone verification details', () => {
    it('shows instructional text before phone input', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(screen.getByText('Enter your phone number to submit feedback.')).toBeInTheDocument()
    })

    it('phone input has type="tel"', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      const input = screen.getByPlaceholderText('(310) 486-1833')
      expect(input).toHaveAttribute('type', 'tel')
    })

    it('phone input has fb-phone-input class', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(document.querySelector('.fb-phone-input')).toBeInTheDocument()
    })

    it('verify button has fb-verify-btn class', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(document.querySelector('.fb-verify-btn')).toBeInTheDocument()
    })

    it('clears error when phone input changes', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText('(310) 486-1833'), '1234567890')
      await user.click(screen.getByText('Verify'))
      expect(screen.getByText("That number isn't authorized for feedback.")).toBeInTheDocument()
      await user.type(screen.getByPlaceholderText('(310) 486-1833'), '0')
      expect(screen.queryByText("That number isn't authorized for feedback.")).not.toBeInTheDocument()
    })

    it('submits phone on Enter key press', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      const input = screen.getByPlaceholderText('(310) 486-1833')
      await user.type(input, '3104861833{Enter}')
      expect(screen.getByText('Product Feedback')).toBeInTheDocument()
    })

    it('normalizes phone number by stripping non-digits', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText('(310) 486-1833'), '310-486-1833')
      await user.click(screen.getByText('Verify'))
      expect(screen.getByText('Product Feedback')).toBeInTheDocument()
    })

    it('stores normalized phone number in localStorage', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText('(310) 486-1833'), '(626) 404-3580')
      await user.click(screen.getByText('Verify'))
      expect(localStorage.getItem('fb_phone')).toBe('6264043580')
    })

    it('error message has fb-error class', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText('(310) 486-1833'), '0000000000')
      await user.click(screen.getByText('Verify'))
      const errorEl = document.querySelector('.fb-error')
      expect(errorEl).toBeInTheDocument()
      expect(errorEl?.textContent).toBe("That number isn't authorized for feedback.")
    })
  })

  // ===== DIALOG HEADER =====
  describe('dialog header', () => {
    it('has a close button with aria-label "Close"', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      const closeBtn = screen.getByLabelText('Close')
      expect(closeBtn).toBeInTheDocument()
    })

    it('close button has fb-close class', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      const closeBtn = document.querySelector('.fb-close')
      expect(closeBtn).toBeInTheDocument()
    })

    it('shows title "Verify Your Number" when not authorized', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(screen.getByText('Verify Your Number')).toBeInTheDocument()
    })

    it('shows title "Product Feedback" when authorized', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(screen.getByText('Product Feedback')).toBeInTheDocument()
    })

    it('title has fb-title class', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      const title = document.querySelector('.fb-title')
      expect(title).toBeInTheDocument()
    })

    it('header has fb-header class', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(document.querySelector('.fb-header')).toBeInTheDocument()
    })
  })

  // ===== FEEDBACK FORM =====
  describe('feedback form', () => {
    it('shows textarea when authorized', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(screen.getByPlaceholderText(/What's on your mind/)).toBeInTheDocument()
    })

    it('textarea has maxLength of 10000', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      const textarea = screen.getByPlaceholderText(/What's on your mind/)
      expect(textarea).toHaveAttribute('maxLength', '10000')
    })

    it('textarea has fb-textarea class', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(document.querySelector('.fb-textarea')).toBeInTheDocument()
    })

    it('has a Send button when authorized', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(screen.getByText('Send')).toBeInTheDocument()
    })

    it('Send button is disabled when textarea is empty', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      const sendBtn = screen.getByText('Send')
      expect(sendBtn).toBeDisabled()
    })

    it('Send button is enabled when textarea has content', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText(/What's on your mind/), 'Great product!')
      const sendBtn = screen.getByText('Send')
      expect(sendBtn).not.toBeDisabled()
    })

    it('Send button has fb-send class', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(document.querySelector('.fb-send')).toBeInTheDocument()
    })

    it('fb-body wraps toggle and textarea', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      const fbBody = document.querySelector('.fb-body')
      expect(fbBody).toBeInTheDocument()
      expect(fbBody?.querySelector('.fb-toggle-row')).toBeInTheDocument()
      expect(fbBody?.querySelector('.fb-textarea-wrap')).toBeInTheDocument()
    })
  })

  // ===== I INSIST TOGGLE =====
  describe('I insist toggle', () => {
    it('defaults to "I insist" mode (on)', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(screen.getByText('I insist')).toBeInTheDocument()
    })

    it('toggles to "Normal" when clicked', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByText('I insist'))
      expect(screen.getByText('Normal')).toBeInTheDocument()
    })

    it('toggles back to "I insist" on second click', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByText('I insist'))
      expect(screen.getByText('Normal')).toBeInTheDocument()
      await user.click(screen.getByText('Normal'))
      expect(screen.getByText('I insist')).toBeInTheDocument()
    })

    it('toggle has fb-toggle class', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(document.querySelector('.fb-toggle')).toBeInTheDocument()
    })

    it('toggle knob has fb-toggle-knob class', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(document.querySelector('.fb-toggle-knob')).toBeInTheDocument()
    })

    it('toggle label has fb-toggle-label class', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(document.querySelector('.fb-toggle-label')).toBeInTheDocument()
    })

    it('toggle row has fb-toggle-row class', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(document.querySelector('.fb-toggle-row')).toBeInTheDocument()
    })
  })

  // ===== HISTORY VIEW =====
  describe('history view', () => {
    it('shows "View history" button when authorized', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(screen.getByLabelText('View history')).toBeInTheDocument()
    })

    it('history button has title "Past submissions"', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(screen.getByTitle('Past submissions')).toBeInTheDocument()
    })

    it('clicking history button shows "My Submissions" title', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      // Mock fetch for history endpoint
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => [],
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByLabelText('View history'))
      expect(screen.getByText('My Submissions')).toBeInTheDocument()
    })

    it('shows "No submissions yet." when history is empty', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => [],
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByLabelText('View history'))
      // Wait for loading to finish
      await screen.findByText('No submissions yet.')
      expect(screen.getByText('No submissions yet.')).toBeInTheDocument()
    })

    it('shows "New" button when viewing history', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => [],
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByLabelText('View history'))
      await screen.findByText('No submissions yet.')
      expect(screen.getByText('New')).toBeInTheDocument()
    })

    it('New button has fb-history-new class', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => [],
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByLabelText('View history'))
      await screen.findByText('No submissions yet.')
      expect(document.querySelector('.fb-history-new')).toBeInTheDocument()
    })

    it('clicking New returns to feedback form', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => [],
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByLabelText('View history'))
      await screen.findByText('No submissions yet.')
      await user.click(screen.getByText('New'))
      expect(screen.getByPlaceholderText(/What's on your mind/)).toBeInTheDocument()
    })

    it('renders history items when fetch returns data', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => [
          { id: 1, body: 'Test feedback', urgency: 'normal', status: 'new', admin_notes: '', created_at: '2026-01-01T12:00:00Z' },
        ],
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByLabelText('View history'))
      await screen.findByText('Test feedback')
      expect(screen.getByText('Test feedback')).toBeInTheDocument()
    })

    it('renders status badge from STATUS_LABELS', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => [
          { id: 1, body: 'My idea', urgency: 'normal', status: 'reviewed', admin_notes: '', created_at: '2026-01-01T12:00:00Z' },
        ],
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByLabelText('View history'))
      await screen.findByText('Under Review')
      expect(screen.getByText('Under Review')).toBeInTheDocument()
    })

    it('renders "I insist" badge for critical urgency items', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => [
          { id: 1, body: 'Urgent!', urgency: 'critical', status: 'new', admin_notes: '', created_at: '2026-01-01T12:00:00Z' },
        ],
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByLabelText('View history'))
      await screen.findByText('Urgent!')
      expect(screen.getByText('I insist')).toBeInTheDocument()
    })

    it('renders admin notes when present', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => [
          { id: 1, body: 'Fix this', urgency: 'normal', status: 'roadmapped', admin_notes: 'Working on it!', created_at: '2026-01-01T12:00:00Z' },
        ],
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByLabelText('View history'))
      await screen.findByText('Working on it!')
      expect(screen.getByText('Working on it!')).toBeInTheDocument()
      expect(screen.getByText('Product team:')).toBeInTheDocument()
    })

    it('handles fetch error gracefully (shows empty state)', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network error'))
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByLabelText('View history'))
      await screen.findByText('No submissions yet.')
      expect(screen.getByText('No submissions yet.')).toBeInTheDocument()
    })
  })

  // ===== CLOSE BEHAVIOR =====
  describe('close behavior', () => {
    it('close button closes the dialog', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      expect(screen.getByRole('dialog')).toBeInTheDocument()
      await user.click(screen.getByLabelText('Close'))
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })

    it('FAB reappears after closing dialog', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByLabelText('Close'))
      expect(screen.getByLabelText('Product feedback')).toBeInTheDocument()
    })
  })

  // ===== SUBMIT BEHAVIOR =====
  describe('submit behavior', () => {
    it('sends feedback via POST to API endpoint', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => ({ success: true }),
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText(/What's on your mind/), 'Love it!')
      await user.click(screen.getByText('Send'))
      expect(fetchSpy).toHaveBeenCalledWith(
        '/.netlify/functions/feedback',
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
        })
      )
    })

    it('sends urgency as "critical" when I insist is on', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => ({ success: true }),
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText(/What's on your mind/), 'Urgent matter')
      await user.click(screen.getByText('Send'))
      const lastCall = fetchSpy.mock.calls[fetchSpy.mock.calls.length - 1]
      const body = JSON.parse((lastCall[1] as RequestInit).body as string)
      expect(body.urgency).toBe('critical')
    })

    it('sends urgency as "normal" when toggle is off', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => ({ success: true }),
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.click(screen.getByText('I insist')) // toggle off
      await user.type(screen.getByPlaceholderText(/What's on your mind/), 'Normal feedback')
      await user.click(screen.getByText('Send'))
      const lastCall = fetchSpy.mock.calls[fetchSpy.mock.calls.length - 1]
      const body = JSON.parse((lastCall[1] as RequestInit).body as string)
      expect(body.urgency).toBe('normal')
    })

    it('shows "Escalated to the CEO" on success with I insist on', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => ({ success: true }),
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText(/What's on your mind/), 'CEO feedback')
      await user.click(screen.getByText('Send'))
      await screen.findByText('Escalated to the CEO')
      expect(screen.getByText('Escalated to the CEO')).toBeInTheDocument()
    })

    it('resets isInsist to true after successful send, always shows CEO escalation', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => ({ success: true }),
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      // Toggle to Normal
      const toggle = document.querySelector('.fb-toggle')!
      await user.click(toggle)
      expect(screen.getByText('Normal')).toBeInTheDocument()
      await user.type(screen.getByPlaceholderText(/What's on your mind/), 'Product feedback')
      await user.click(screen.getByText('Send'))
      // After submit, isInsist is reset to true before setSent(true),
      // so the sent state always shows "Escalated to the CEO"
      await screen.findByText('Escalated to the CEO')
      expect(screen.getByText('Escalated to the CEO')).toBeInTheDocument()
    })

    it('shows success message "Your voice shapes what we build next."', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => ({ success: true }),
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText(/What's on your mind/), 'Feedback!')
      await user.click(screen.getByText('Send'))
      await screen.findByText('Your voice shapes what we build next.')
    })

    it('shows error message on API failure', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => ({ success: false, error: 'Server error' }),
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText(/What's on your mind/), 'Test')
      await user.click(screen.getByText('Send'))
      await screen.findByText('Server error')
      expect(screen.getByText('Server error')).toBeInTheDocument()
    })

    it('shows network error on fetch rejection', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('fail'))
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText(/What's on your mind/), 'Test')
      await user.click(screen.getByText('Send'))
      await screen.findByText('Network error. Try again.')
      expect(screen.getByText('Network error. Try again.')).toBeInTheDocument()
    })

    it('does not send when textarea is whitespace only', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText(/What's on your mind/), '   ')
      const fetchSpy = vi.spyOn(globalThis, 'fetch')
      await user.click(screen.getByText('Send'))
      // fetch should not be called for whitespace-only input
      const postCalls = fetchSpy.mock.calls.filter(
        c => typeof c[1] === 'object' && (c[1] as RequestInit).method === 'POST'
      )
      expect(postCalls.length).toBe(0)
    })
  })

  // ===== SENT STATE =====
  describe('sent state', () => {
    it('renders checkmark SVG in sent state', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => ({ success: true }),
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText(/What's on your mind/), 'Done!')
      await user.click(screen.getByText('Send'))
      await screen.findByText('Your voice shapes what we build next.')
      expect(document.querySelector('.fb-sent-icon svg')).toBeInTheDocument()
    })

    it('sent state shows "Sent" title in header', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => ({ success: true }),
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText(/What's on your mind/), 'Sent test')
      await user.click(screen.getByText('Send'))
      await screen.findByText('Sent')
    })

    it('sent state has fb-sent class', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => ({ success: true }),
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText(/What's on your mind/), 'Check class')
      await user.click(screen.getByText('Send'))
      await screen.findByText('Your voice shapes what we build next.')
      expect(document.querySelector('.fb-sent')).toBeInTheDocument()
    })
  })

  // ===== BRAND CONSISTENCY =====
  describe('brand consistency', () => {
    it('does not use corporate jargon in phone gate text', async () => {
      const user = userEvent.setup()
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      const gateText = document.querySelector('.fb-phone-gate')?.textContent || ''
      expect(gateText).not.toMatch(/\bsynergy\b/i)
      expect(gateText).not.toMatch(/\bleverage\b/i)
    })

    it('uses warm language in success message', async () => {
      localStorage.setItem('fb_phone', '3104861833')
      const user = userEvent.setup()
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        json: async () => ({ success: true }),
        ok: true,
      } as Response)
      render(<FeedbackWidget />)
      await user.click(screen.getByLabelText('Product feedback'))
      await user.type(screen.getByPlaceholderText(/What's on your mind/), 'Test')
      await user.click(screen.getByText('Send'))
      await screen.findByText('Your voice shapes what we build next.')
      // Warm, empowering language
      expect(screen.getByText('Your voice shapes what we build next.')).toBeInTheDocument()
    })
  })

  // ===== STYLES =====
  describe('embedded styles', () => {
    it('renders a style element with CSS', () => {
      render(<FeedbackWidget />)
      const styles = document.querySelectorAll('style')
      const fbStyles = Array.from(styles).find(s => s.textContent?.includes('.fb-fab'))
      expect(fbStyles).toBeTruthy()
    })
  })
})
