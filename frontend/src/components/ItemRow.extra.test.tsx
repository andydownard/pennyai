import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'

vi.mock('./BrandLogo', () => ({ BrandLogo: ({ brand }: { brand: string }) => <span data-testid="logo">{brand}</span> }))
vi.mock('./SwapCard', () => ({
  SwapCard: ({ swap, onAccept, onDecline }: { swap: { deal_id: number }; onAccept: (d: number, i: number) => void; onDecline: (d: number, i: number) => void }) => (
    <div data-testid="swap">
      <button onClick={() => onAccept(swap.deal_id, 1)}>mock-accept</button>
      <button onClick={() => onDecline(swap.deal_id, 1)}>mock-decline</button>
    </div>
  ),
}))
vi.mock('../haptics', () => ({
  haptic: { tap: vi.fn(), medium: vi.fn(), heavy: vi.fn(), light: vi.fn(), success: vi.fn(), error: vi.fn(), double: vi.fn() },
}))

import { haptic } from '../haptics'
import { ItemRow, type ListItem } from './ItemRow'

const base: ListItem = {
  id: 1, name: 'Milk', brand: '', department: 'Dairy', quantity: 1, size: '', notes: '', source: 'web',
  checked: false, urgent: false, created_at: '2020-01-01T00:00:00Z', added_by_name: 'Andy',
  claimed_by_id: null, claimed_by_name: '', swap_savings_cents: 0, emoji: '',
}

function setup(over: Partial<ListItem> = {}, extra: Record<string, unknown> = {}) {
  const props = {
    isNew: false, currentUserId: 1 as number | null,
    onToggle: vi.fn(), onTap: vi.fn(), onDelete: vi.fn(), onLongPress: vi.fn(),
    onSwapAccept: vi.fn(), onSwapDecline: vi.fn(), ...extra,
  }
  const view = render(<ItemRow item={{ ...base, ...over }} {...props} />)
  // The swipeable surface is the row that holds the check button.
  const surface = () => screen.getByRole('button', { name: /^(Check off|Uncheck)/ }).parentElement!
  return { props, surface, ...view }
}

const touch = (x: number, y = 0) => ({ touches: [{ clientX: x, clientY: y }] })

beforeEach(() => { vi.mocked(haptic.tap).mockClear(); vi.mocked(haptic.medium).mockClear(); vi.mocked(haptic.heavy).mockClear() })
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

describe('ItemRow naming and meta', () => {
  it('prefixes the brand unless the name already contains it', () => {
    const { unmount } = setup({ brand: 'Horizon', name: 'Milk' })
    expect(screen.getByText('Horizon Milk')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Check off Horizon Milk' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Edit Horizon Milk' })).toBeInTheDocument()
    unmount()
    setup({ brand: 'horizon', name: 'Horizon Milk' })
    expect(screen.getByText('Horizon Milk')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Check off Horizon Milk' })).toBeInTheDocument()
  })

  it('shows quantity, size and notes joined by dots', () => {
    setup({ quantity: 3, size: '1 gal', notes: 'organic' })
    expect(screen.getByText('×3 · 1 gal · organic')).toBeInTheDocument()
  })

  it('shows only the meta parts that exist', () => {
    const { unmount } = setup({ quantity: 2 })
    expect(screen.getByText('×2')).toBeInTheDocument()
    unmount()
    setup({ notes: 'ripe' })
    expect(screen.getByText('ripe')).toBeInTheDocument()
  })

  it('shows no meta line for a bare item', () => {
    setup()
    expect(screen.queryByText(/×/)).not.toBeInTheDocument()
  })

  it('shows "just now" for a new item and "N min ago" within the hour', () => {
    const { unmount } = setup({ created_at: new Date(Date.now() - 10_000).toISOString() })
    expect(screen.getByText('just now')).toBeInTheDocument()
    unmount()
    setup({ created_at: new Date(Date.now() - 5 * 60_000 - 5000).toISOString() })
    expect(screen.getByText('5 min ago')).toBeInTheDocument()
  })

  it('shows no timestamp for old, future or invalid dates', () => {
    for (const created_at of [new Date(Date.now() - 2 * 3600_000).toISOString(), new Date(Date.now() + 60_000).toISOString(), 'garbage']) {
      const { unmount } = setup({ created_at })
      expect(screen.queryByText(/ago|just now/)).not.toBeInTheDocument()
      unmount()
    }
  })
})

describe('ItemRow state and badges', () => {
  it('checked items read "Uncheck", are struck through and hide the swap card', () => {
    setup({ checked: true }, { swap: { deal_id: 9 } })
    expect(screen.getByRole('button', { name: 'Uncheck Milk' })).toBeInTheDocument()
    expect(screen.getByText('Milk')).toHaveStyle({ textDecoration: 'line-through' })
    expect(screen.queryByTestId('swap')).not.toBeInTheDocument()
  })

  it('shows the urgent badge', () => {
    setup({ urgent: true })
    expect(screen.getByText(/Urgent/)).toBeInTheDocument()
  })

  it('shows "Me" when the current user claimed it', () => {
    setup({ claimed_by_id: 1, claimed_by_name: 'Andy' })
    expect(screen.getByText('Me')).toBeInTheDocument()
  })

  it('shows the claimer initial for someone else, or ? with no name', () => {
    const { unmount } = setup({ claimed_by_id: 2, claimed_by_name: 'Sam' })
    expect(screen.getByText('S')).toBeInTheDocument()
    unmount()
    setup({ claimed_by_id: 2, claimed_by_name: '' })
    expect(screen.getByText('?')).toBeInTheDocument()
  })

  it('shows no badge column for a plain item', () => {
    setup()
    expect(screen.queryByText(/Urgent|Saving|Me/)).not.toBeInTheDocument()
  })
})

describe('ItemRow taps and keyboard', () => {
  it('check button toggles once', () => {
    const { props } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Check off Milk' }))
    expect(props.onToggle).toHaveBeenCalledTimes(1)
    expect(props.onToggle).toHaveBeenCalledWith(1)
    expect(props.onTap).not.toHaveBeenCalled()
  })

  it('tapping the name opens edit once, with the item', () => {
    const { props } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Edit Milk' }))
    expect(props.onTap).toHaveBeenCalledTimes(1)
    expect(props.onTap).toHaveBeenCalledWith(expect.objectContaining({ id: 1, name: 'Milk' }))
  })

  it('Enter and Space on the name open edit; other keys do not', () => {
    const { props } = setup()
    const edit = screen.getByRole('button', { name: 'Edit Milk' })
    fireEvent.keyDown(edit, { key: 'Enter' })
    fireEvent.keyDown(edit, { key: ' ' })
    fireEvent.keyDown(edit, { key: 'a' })
    expect(props.onTap).toHaveBeenCalledTimes(2)
  })

  it('renders the swap card for an unchecked item and forwards accept/decline', () => {
    const { props } = setup({}, { swap: { deal_id: 9 } })
    fireEvent.click(screen.getByText('mock-accept'))
    fireEvent.click(screen.getByText('mock-decline'))
    expect(props.onSwapAccept).toHaveBeenCalledTimes(1)
    expect(props.onSwapAccept).toHaveBeenCalledWith(9, 1)
    expect(props.onSwapDecline).toHaveBeenCalledTimes(1)
  })

  it('has no swap card without an offer', () => {
    setup()
    expect(screen.queryByTestId('swap')).not.toBeInTheDocument()
  })
})

describe('ItemRow gestures', () => {
  it('a left swipe past the threshold deletes once, with haptics', () => {
    const { props, surface } = setup()
    const row = surface()
    fireEvent.touchStart(row, touch(200))
    fireEvent.touchMove(row, touch(100))
    expect(haptic.medium).toHaveBeenCalledTimes(1)
    fireEvent.touchEnd(row)
    expect(props.onDelete).toHaveBeenCalledTimes(1)
    expect(props.onDelete).toHaveBeenCalledWith(1)
    expect(haptic.heavy).toHaveBeenCalledTimes(1)
    expect(props.onToggle).not.toHaveBeenCalled()
  })

  it('a short left swipe does nothing', () => {
    const { props, surface } = setup()
    const row = surface()
    fireEvent.touchStart(row, touch(200))
    fireEvent.touchMove(row, touch(150))
    fireEvent.touchEnd(row)
    expect(props.onDelete).not.toHaveBeenCalled()
    expect(props.onToggle).not.toHaveBeenCalled()
  })

  it('a right swipe past the threshold toggles once', () => {
    const { props, surface } = setup()
    const row = surface()
    fireEvent.touchStart(row, touch(100))
    fireEvent.touchMove(row, touch(170))
    expect(haptic.tap).toHaveBeenCalledTimes(1)
    fireEvent.touchEnd(row)
    expect(props.onToggle).toHaveBeenCalledTimes(1)
    expect(props.onToggle).toHaveBeenCalledWith(1)
    expect(props.onDelete).not.toHaveBeenCalled()
  })

  it('buzzes once per crossing, not on every move frame', () => {
    const { surface } = setup()
    const row = surface()
    fireEvent.touchStart(row, touch(200))
    fireEvent.touchMove(row, touch(110))
    fireEvent.touchMove(row, touch(100))
    fireEvent.touchMove(row, touch(95))
    expect(haptic.medium).toHaveBeenCalledTimes(1)
    fireEvent.touchMove(row, touch(180))
    fireEvent.touchMove(row, touch(100))
    expect(haptic.medium).toHaveBeenCalledTimes(2)
  })

  it('a vertical gesture is a scroll, not a swipe', () => {
    const { props, surface } = setup()
    const row = surface()
    fireEvent.touchStart(row, touch(100, 100))
    fireEvent.touchMove(row, touch(105, 200))
    fireEvent.touchEnd(row)
    expect(props.onDelete).not.toHaveBeenCalled()
    expect(props.onToggle).not.toHaveBeenCalled()
  })

  it('a touch with no movement does nothing', () => {
    const { props, surface } = setup()
    const row = surface()
    fireEvent.touchStart(row, touch(100))
    fireEvent.touchEnd(row)
    expect(props.onDelete).not.toHaveBeenCalled()
    expect(props.onToggle).not.toHaveBeenCalled()
    expect(props.onLongPress).not.toHaveBeenCalled()
  })

  it('reveals the delete and check backdrops while swiping, then clears them', () => {
    const { container, surface } = setup()
    const row = surface()
    fireEvent.touchStart(row, touch(200))
    fireEvent.touchMove(row, touch(150))
    expect(container.querySelector('[aria-hidden="true"][style*="var(--error)"]')).toBeInTheDocument()
    fireEvent.touchEnd(row)
    expect(container.querySelector('[style*="var(--error)"]')).toBeNull()
    fireEvent.touchStart(row, touch(100))
    fireEvent.touchMove(row, touch(140))
    expect(container.querySelector('[aria-hidden="true"][style*="var(--success"]')).toBeInTheDocument()
    fireEvent.touchEnd(row)
  })

  it('holding for 500ms fires the long press once and the release does not swipe or delete', () => {
    vi.useFakeTimers()
    const { props, surface } = setup()
    const row = surface()
    fireEvent.touchStart(row, touch(100))
    act(() => { vi.advanceTimersByTime(499) })
    expect(props.onLongPress).not.toHaveBeenCalled()
    act(() => { vi.advanceTimersByTime(1) })
    expect(props.onLongPress).toHaveBeenCalledTimes(1)
    expect(props.onLongPress).toHaveBeenCalledWith(1)
    fireEvent.touchEnd(row)
    act(() => { vi.advanceTimersByTime(2000) })
    expect(props.onLongPress).toHaveBeenCalledTimes(1)
    expect(props.onDelete).not.toHaveBeenCalled()
    expect(props.onToggle).not.toHaveBeenCalled()
  })

  it('moving more than 5px cancels the long press', () => {
    vi.useFakeTimers()
    const { props, surface } = setup()
    const row = surface()
    fireEvent.touchStart(row, touch(100))
    fireEvent.touchMove(row, touch(108))
    act(() => { vi.advanceTimersByTime(1000) })
    expect(props.onLongPress).not.toHaveBeenCalled()
  })

  it('lifting early cancels the long press', () => {
    vi.useFakeTimers()
    const { props, surface } = setup()
    const row = surface()
    fireEvent.touchStart(row, touch(100))
    act(() => { vi.advanceTimersByTime(200) })
    fireEvent.touchEnd(row)
    act(() => { vi.advanceTimersByTime(1000) })
    expect(props.onLongPress).not.toHaveBeenCalled()
  })
})
