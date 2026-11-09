import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act, fireEvent } from '@testing-library/react'

vi.mock('./PennyAvatar', () => ({ PennyAvatar: () => null }))

import { NPSSurveyModal } from './NPSSurveyModal'

const Q1 = 'How likely are you to recommend Groflo to a friend?'

async function typeOut() {
  await act(async () => { await vi.advanceTimersByTimeAsync(25 * 120) })
}

// The modal is controlled by its parent (onClose / onSubmit); it holds no
// dismissal state of its own, so persistence is the caller's job. These tests
// pin the contract the caller relies on: exactly one callback per action.
describe('NPSSurveyModal', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('is a named modal dialog and types the question out before showing scores', async () => {
    render(<NPSSurveyModal onClose={vi.fn()} onSubmit={vi.fn()} />)
    expect(screen.getByRole('dialog', { name: 'NPS Survey' })).toHaveAttribute('aria-modal', 'true')
    expect(screen.queryByRole('button', { name: /Score 0/ })).not.toBeInTheDocument()
    await typeOut()
    expect(screen.getByText(Q1)).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /^Score \d+ out of 10$/ })).toHaveLength(11)
    expect(screen.getByText('Not likely')).toBeInTheDocument()
    expect(screen.getByText('Very likely')).toBeInTheDocument()
  })

  it('moves to the comment step with the chosen score and submits it once', async () => {
    const onSubmit = vi.fn()
    render(<NPSSurveyModal onClose={vi.fn()} onSubmit={onSubmit} />)
    await typeOut()
    fireEvent.click(screen.getByRole('button', { name: 'Score 9 out of 10' }))
    await typeOut()
    expect(screen.getByText(/You picked 9!/)).toBeInTheDocument()
    const box = screen.getByRole('textbox', { name: /behind your score/i })
    expect(box).toHaveAttribute('placeholder', "What's behind your score of 9? (optional)")
    fireEvent.change(box, { target: { value: 'Shared list is great' } })
    fireEvent.click(screen.getByRole('button', { name: 'Submit' }))
    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(onSubmit).toHaveBeenCalledWith(9, 'Shared list is great')
  })

  it('accepts a score of 0 and an empty comment', async () => {
    const onSubmit = vi.fn()
    render(<NPSSurveyModal onClose={vi.fn()} onSubmit={onSubmit} />)
    await typeOut()
    fireEvent.click(screen.getByRole('button', { name: 'Score 0 out of 10' }))
    await typeOut()
    fireEvent.click(screen.getByRole('button', { name: 'Submit' }))
    expect(onSubmit).toHaveBeenCalledWith(0, '')
  })

  it('does not submit before a score is chosen', async () => {
    const onSubmit = vi.fn()
    render(<NPSSurveyModal onClose={vi.fn()} onSubmit={onSubmit} />)
    await typeOut()
    expect(screen.queryByRole('button', { name: 'Submit' })).not.toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('closes once via the close button without submitting', () => {
    const onClose = vi.fn()
    const onSubmit = vi.fn()
    render(<NPSSurveyModal onClose={onClose} onSubmit={onSubmit} />)
    fireEvent.click(screen.getByRole('button', { name: 'Close survey' }))
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('closes on overlay click but not on dialog click', () => {
    const onClose = vi.fn()
    render(<NPSSurveyModal onClose={onClose} onSubmit={vi.fn()} />)
    fireEvent.click(screen.getByRole('dialog'))
    expect(onClose).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('presentation'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('restores focus to the trigger on unmount', () => {
    const trigger = document.createElement('button')
    document.body.appendChild(trigger)
    trigger.focus()
    const { unmount } = render(<NPSSurveyModal onClose={vi.fn()} onSubmit={vi.fn()} />)
    trigger.blur()
    unmount()
    expect(trigger).toHaveFocus()
    trigger.remove()
  })

  it('never says bot, AI or assistant', async () => {
    render(<NPSSurveyModal onClose={vi.fn()} onSubmit={vi.fn()} />)
    await typeOut()
    expect(screen.getByRole('dialog').textContent).not.toMatch(/\b(bot|AI|assistant)\b/i)
  })
})
