import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, act, fireEvent } from '@testing-library/react'
import { BottomSheet } from './BottomSheet'

afterEach(() => { vi.useRealTimers() })

describe('BottomSheet', () => {
  it('renders nothing when closed', () => {
    render(<BottomSheet open={false} onClose={vi.fn()}><p>body</p></BottomSheet>)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('renders a modal dialog named by its title with children', () => {
    render(<BottomSheet open onClose={vi.fn()} title="Pick one"><p>body</p></BottomSheet>)
    const dialog = screen.getByRole('dialog', { name: 'Pick one' })
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(screen.getByText('body')).toBeInTheDocument()
    expect(screen.getByText('Pick one')).toBeInTheDocument()
  })

  it('prefers ariaLabel and falls back to "Dialog"', () => {
    const { rerender } = render(<BottomSheet open onClose={vi.fn()} title="T" ariaLabel="Custom"><p>x</p></BottomSheet>)
    expect(screen.getByRole('dialog', { name: 'Custom' })).toBeInTheDocument()
    rerender(<BottomSheet open onClose={vi.fn()}><p>x</p></BottomSheet>)
    expect(screen.getByRole('dialog', { name: 'Dialog' })).toBeInTheDocument()
  })

  it('applies maxHeight and sheetStyle to the panel', () => {
    render(<BottomSheet open onClose={vi.fn()} maxHeight="50vh" sheetStyle={{ padding: '7px' }}><p>x</p></BottomSheet>)
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveStyle({ maxHeight: '50vh', padding: '7px' })
  })

  it('closes once on Escape, and ignores other keys', () => {
    const onClose = vi.fn()
    render(<BottomSheet open onClose={onClose}><button>a</button></BottomSheet>)
    fireEvent.keyDown(document, { key: 'Enter' })
    expect(onClose).not.toHaveBeenCalled()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does not listen for Escape while closed', () => {
    const onClose = vi.fn()
    render(<BottomSheet open={false} onClose={onClose}><p>x</p></BottomSheet>)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onClose).not.toHaveBeenCalled()
  })

  it('closes on overlay click but not on panel click', () => {
    const onClose = vi.fn()
    render(<BottomSheet open onClose={onClose}><p>inside</p></BottomSheet>)
    fireEvent.click(screen.getByText('inside'))
    expect(onClose).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('presentation'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  // KNOWN BUG (reported, not fixed): useFocusTrap's effect depends only on isActive and runs
  // before the panel is mounted (BottomSheet mounts it one render later), so ref.current is
  // null and the trap never engages. it.fails flips to a failure once the bug is fixed.
  it.fails('moves focus into the sheet and traps Tab at both ends', () => {
    render(<BottomSheet open onClose={vi.fn()}><button>first</button><button>last</button></BottomSheet>)
    const first = screen.getByRole('button', { name: 'first' })
    const last = screen.getByRole('button', { name: 'last' })
    expect(first).toHaveFocus()
    fireEvent.keyDown(document, { key: 'Tab', shiftKey: true })
    expect(last).toHaveFocus()
    fireEvent.keyDown(document, { key: 'Tab' })
    expect(first).toHaveFocus()
  })

  it.fails('restores focus to the trigger when it unmounts (trap never engaged, see above)', () => {
    const trigger = document.createElement('button')
    document.body.appendChild(trigger)
    trigger.focus()
    const { unmount } = render(<BottomSheet open onClose={vi.fn()}><button>in</button></BottomSheet>)
    expect(trigger).not.toHaveFocus()
    unmount()
    expect(trigger).toHaveFocus()
    trigger.remove()
  })

  it('hides the drag handle when showHandle is false', () => {
    const { container, rerender } = render(<BottomSheet open onClose={vi.fn()}><p>x</p></BottomSheet>)
    expect(screen.getByRole('dialog').children.length).toBe(2)
    rerender(<BottomSheet open showHandle={false} onClose={vi.fn()}><p>x</p></BottomSheet>)
    expect(container.querySelector('[role="dialog"]')!.children.length).toBe(1)
  })

  it('stays mounted for the slide-down, then unmounts after 250ms', () => {
    vi.useFakeTimers()
    const { rerender } = render(<BottomSheet open onClose={vi.fn()}><p>x</p></BottomSheet>)
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    rerender(<BottomSheet open={false} onClose={vi.fn()}><p>x</p></BottomSheet>)
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    act(() => { vi.advanceTimersByTime(250) })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
