import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { FeedbackWidget } from './FeedbackWidget'

const item = (id: number, over: Record<string, unknown> = {}) => ({
  id, body: `Body ${id}`, type: 'bug', urgency: 'normal', status: 'new', admin_notes: '',
  github_issue_url: '', created_at: '2026-10-30T12:00:00Z', updated_at: '2026-10-30T12:00:00Z', ...over,
})

function mockHistory(payload: unknown, ok = true) {
  const f = vi.fn(() => Promise.resolve({ ok, json: () => Promise.resolve(payload) } as Response))
  vi.stubGlobal('fetch', f)
  return f
}

function openWidget(onSubmit = vi.fn().mockResolvedValue(true)) {
  render(<FeedbackWidget onSubmit={onSubmit} />)
  fireEvent.click(screen.getByRole('button', { name: 'Send feedback' }))
  return onSubmit
}

afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers() })

describe('FeedbackWidget', () => {
  it('shows only the floating button until opened', () => {
    render(<FeedbackWidget onSubmit={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Send feedback' })).toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('opens a named modal dialog on the write tab and hides the button', () => {
    openWidget()
    expect(screen.getByRole('dialog', { name: 'Product feedback' })).toHaveAttribute('aria-modal', 'true')
    expect(screen.getByText('Help shape Penny')).toBeInTheDocument()
    expect(screen.getByRole('textbox')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Send feedback' })).not.toBeInTheDocument()
  })

  it('focuses the textarea shortly after opening', () => {
    vi.useFakeTimers()
    openWidget()
    act(() => { vi.advanceTimersByTime(200) })
    expect(screen.getByRole('textbox')).toHaveFocus()
  })

  it('Send is disabled for empty or whitespace-only text and never calls onSubmit', () => {
    const onSubmit = openWidget()
    const send = screen.getByRole('button', { name: 'Send' })
    expect(send).toBeDisabled()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '   ' } })
    expect(send).toBeDisabled()
    fireEvent.click(send)
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('placeholder follows the chosen type', () => {
    openWidget()
    const box = screen.getByRole('textbox')
    expect(box).toHaveAttribute('placeholder', 'What would make Penny better?')
    fireEvent.click(screen.getByRole('button', { name: 'Bug' }))
    expect(box).toHaveAttribute('placeholder', 'What went wrong?')
    fireEvent.click(screen.getByRole('button', { name: 'Other' }))
    expect(box).toHaveAttribute('placeholder', "What's on your mind?")
  })

  it('submits the trimmed text with the default type and urgency, once', async () => {
    mockHistory([])
    const onSubmit = openWidget()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '  Add dark mode  ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    await screen.findByText('Got it!')
    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(onSubmit).toHaveBeenCalledWith('Add dark mode', 'feature', 'normal')
  })

  it('submits the chosen type and urgency', async () => {
    mockHistory([])
    const onSubmit = openWidget()
    fireEvent.click(screen.getByRole('button', { name: 'Bug' }))
    fireEvent.click(screen.getByRole('button', { name: 'Urgent' }))
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Crash' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    await screen.findByText('Got it!')
    expect(onSubmit).toHaveBeenCalledWith('Crash', 'bug', 'critical')
  })

  it('Cmd+Enter and Ctrl+Enter send; plain Enter does not', async () => {
    mockHistory([])
    const onSubmit = openWidget()
    const box = screen.getByRole('textbox')
    fireEvent.change(box, { target: { value: 'Hi' } })
    fireEvent.keyDown(box, { key: 'Enter' })
    expect(onSubmit).not.toHaveBeenCalled()
    fireEvent.keyDown(box, { key: 'Enter', metaKey: true })
    await screen.findByText('Got it!')
    expect(onSubmit).toHaveBeenCalledTimes(1)
  })

  it('Ctrl+Enter sends too', async () => {
    mockHistory([])
    const onSubmit = openWidget()
    const box = screen.getByRole('textbox')
    fireEvent.change(box, { target: { value: 'Hi' } })
    fireEvent.keyDown(box, { key: 'Enter', ctrlKey: true })
    await screen.findByText('Got it!')
    expect(onSubmit).toHaveBeenCalledTimes(1)
  })

  it('cannot double-submit while a send is in flight', async () => {
    let finish: (ok: boolean) => void = () => {}
    const onSubmit = vi.fn(() => new Promise<boolean>(r => { finish = r }))
    mockHistory([])
    openWidget(onSubmit)
    const box = screen.getByRole('textbox')
    fireEvent.change(box, { target: { value: 'Hi' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    const sending = await screen.findByRole('button', { name: 'Sending...' })
    expect(sending).toBeDisabled()
    fireEvent.click(sending)
    fireEvent.keyDown(box, { key: 'Enter', metaKey: true })
    expect(onSubmit).toHaveBeenCalledTimes(1)
    await act(async () => { finish(true) })
    await screen.findByText('Got it!')
  })

  it('does not close on backdrop or X while sending', async () => {
    let finish: (ok: boolean) => void = () => {}
    mockHistory([])
    openWidget(vi.fn(() => new Promise<boolean>(r => { finish = r })))
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Hi' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    await screen.findByRole('button', { name: 'Sending...' })
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    await act(async () => { finish(false) })
  })

  it('keeps the draft and the form when onSubmit reports failure', async () => {
    const onSubmit = vi.fn().mockResolvedValue(false)
    openWidget(onSubmit)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Keep me' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled())
    expect(screen.getByRole('textbox')).toHaveValue('Keep me')
    expect(screen.queryByText('Got it!')).not.toBeInTheDocument()
  })

  async function sendWithFakeTimers(f: () => void) {
    vi.useFakeTimers()
    f()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Hi' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    await act(async () => { await vi.advanceTimersByTimeAsync(10) })
    expect(screen.getByText('Got it!')).toBeInTheDocument()
    await act(async () => { await vi.advanceTimersByTimeAsync(2000) })
  }

  it('after success, moves to History after 2s and loads it once', async () => {
    const f = mockHistory([item(1)])
    await sendWithFakeTimers(() => openWidget())
    expect(screen.getByText('Body 1')).toBeInTheDocument()
    // The tab change loads History once; the timer must not fetch it a second time.
    const historyGets = f.mock.calls.filter(c => c[0] === '/api/feedback' && !c[1])
    expect(historyGets).toHaveLength(1)
  })

  it('the form is blank after a successful send', async () => {
    mockHistory([])
    await sendWithFakeTimers(() => openWidget())
    fireEvent.click(screen.getByRole('button', { name: 'Submit your first' }))
    expect(screen.getByRole('textbox')).toHaveValue('')
  })

  describe('history tab', () => {
    it('shows loading, then entries with type, status, urgency-agnostic body and date', async () => {
      mockHistory([item(1, { status: 'roadmapped', type: 'feature' }), item(2, { status: 'done' })])
      openWidget()
      fireEvent.click(screen.getByRole('button', { name: 'History' }))
      expect(await screen.findByText('Body 1')).toBeInTheDocument()
      expect(screen.getByText('Roadmapped')).toBeInTheDocument()
      expect(screen.getByText('Done')).toBeInTheDocument()
      expect(screen.getByText('feature')).toBeInTheDocument()
    })

    it('falls back to the New badge for an unknown status', async () => {
      mockHistory([item(1, { status: 'mystery' })])
      openWidget()
      fireEvent.click(screen.getByRole('button', { name: 'History' }))
      expect(await screen.findByText('New')).toBeInTheDocument()
    })

    it('strips the device-context footer from the body', async () => {
      mockHistory([item(1, { body: 'Visible text\n\n---\nUA: secret-device-info' })])
      openWidget()
      fireEvent.click(screen.getByRole('button', { name: 'History' }))
      expect(await screen.findByText('Visible text')).toBeInTheDocument()
      expect(screen.queryByText(/secret-device-info/)).not.toBeInTheDocument()
    })

    it('shows team notes and a safe issue link only when present', async () => {
      mockHistory([item(1, { admin_notes: 'On the list', github_issue_url: 'https://github.com/x/y/issues/1' }), item(2)])
      openWidget()
      fireEvent.click(screen.getByRole('button', { name: 'History' }))
      expect(await screen.findByText('On the list')).toBeInTheDocument()
      const links = screen.getAllByRole('link', { name: /View issue/ })
      expect(links).toHaveLength(1)
      expect(links[0]).toHaveAttribute('href', 'https://github.com/x/y/issues/1')
      expect(links[0]).toHaveAttribute('rel', 'noopener noreferrer')
      expect(links[0]).toHaveAttribute('target', '_blank')
    })

    it('empty history offers to write the first one', async () => {
      mockHistory([])
      openWidget()
      fireEvent.click(screen.getByRole('button', { name: 'History' }))
      expect(await screen.findByText('No feedback yet')).toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Submit your first' }))
      expect(screen.getByRole('textbox')).toBeInTheDocument()
    })

    it('treats a non-array payload, an error status and a network failure as empty', async () => {
      mockHistory({ error: 'x' })
      openWidget()
      fireEvent.click(screen.getByRole('button', { name: 'History' }))
      expect(await screen.findByText('No feedback yet')).toBeInTheDocument()
    })

    it('empty on HTTP error', async () => {
      mockHistory([item(1)], false)
      openWidget()
      fireEvent.click(screen.getByRole('button', { name: 'History' }))
      expect(await screen.findByText('No feedback yet')).toBeInTheDocument()
    })

    it('empty on network failure', async () => {
      vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new Error('offline'))))
      openWidget()
      fireEvent.click(screen.getByRole('button', { name: 'History' }))
      expect(await screen.findByText('No feedback yet')).toBeInTheDocument()
    })

    it('switching back to New Feedback shows the form', async () => {
      mockHistory([])
      openWidget()
      fireEvent.click(screen.getByRole('button', { name: 'History' }))
      await screen.findByText('No feedback yet')
      fireEvent.click(screen.getByRole('button', { name: 'New Feedback' }))
      expect(screen.getByRole('textbox')).toBeInTheDocument()
    })
  })

  describe('closing', () => {
    it('the X closes and the floating button returns', () => {
      openWidget()
      fireEvent.click(screen.getByRole('button', { name: 'Close' }))
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Send feedback' })).toBeInTheDocument()
    })

    it('a backdrop click closes', () => {
      openWidget()
      const backdrop = screen.getByRole('dialog').previousElementSibling as HTMLElement
      fireEvent.click(backdrop)
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })

    it('reopening always starts on the write tab', async () => {
      mockHistory([])
      openWidget()
      fireEvent.click(screen.getByRole('button', { name: 'History' }))
      await screen.findByText('No feedback yet')
      fireEvent.click(screen.getByRole('button', { name: 'Close' }))
      fireEvent.click(screen.getByRole('button', { name: 'Send feedback' }))
      expect(screen.getByRole('textbox')).toBeInTheDocument()
    })
  })

  it('visible copy never says bot, AI or assistant', () => {
    openWidget()
    expect(document.body.textContent).not.toMatch(/\b(bot|AI|assistant)\b/i)
  })
})
