import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, act, within } from '@testing-library/react'
import { createRef } from 'react'

vi.mock('./PennyAvatar', () => ({
  PennyAvatar: ({ size, mood }: { size?: number; mood?: string }) => <img alt="Penny" data-size={size} data-mood={mood} />,
}))
vi.mock('../haptics', () => ({ haptic: { tap: vi.fn(), medium: vi.fn(), heavy: vi.fn(), light: vi.fn(), success: vi.fn(), error: vi.fn(), double: vi.fn() } }))

import { haptic } from '../haptics'
import { PennyBar, type DisambigItem } from './PennyBar'

type Props = React.ComponentProps<typeof PennyBar>

function makeProps(extra: Partial<Props> = {}): Props {
  return {
    pennyText: 'What can I add?', pennyTyping: false, buttons: [], toastError: false,
    keyboardOpen: false, disambig: [], disambigIndex: 0,
    onBrandPick: vi.fn(), onSkipDisambig: vi.fn(),
    pendingImages: [], onRemoveImage: vi.fn(),
    chatText: '', onChatTextChange: vi.fn(), chatLoading: false, onSend: vi.fn(),
    onPaste: vi.fn(), onAddPhoto: vi.fn(), onOpenCamera: vi.fn(),
    chatInputRef: createRef<HTMLTextAreaElement>(),
    ...extra,
  }
}

function setup(extra: Partial<Props> = {}) {
  const props = makeProps(extra)
  const view = render(<PennyBar {...props} />)
  return { props, ...view, rerenderWith: (more: Partial<Props>) => view.rerender(<PennyBar {...props} {...more} />) }
}

const d = (over: Partial<DisambigItem> = {}): DisambigItem => ({
  product_category: 'Milk', current_brand: 'Horizon', alternatives: ['Horizon', 'Organic Valley'], ...over,
})

afterEach(() => { vi.useRealTimers(); vi.mocked(haptic.tap).mockClear() })

describe('PennyBar text and buttons', () => {
  it('is a labelled region showing Penny\'s words', () => {
    setup()
    expect(screen.getByRole('region', { name: 'Penny' })).toBeInTheDocument()
    expect(screen.getAllByText('What can I add?').length).toBeGreaterThan(0)
  })

  it('shows a thinking face while typing, and the cursor, and no screen-reader duplicate', () => {
    const { container } = setup({ pennyTyping: true })
    expect(screen.getByAltText('Penny')).toHaveAttribute('data-mood', 'thinking')
    expect(container.querySelector('.cursor-blink')).toBeInTheDocument()
    expect(container.querySelector('.sr-only')).toBeNull()
  })

  it('announces the full text to screen readers once typing is done', () => {
    const { container } = setup()
    const live = container.querySelector('.sr-only')!
    expect(live).toHaveTextContent('What can I add?')
    expect(live).toHaveAttribute('aria-live', 'polite')
  })

  it('face is smaller with the keyboard open', () => {
    setup({ keyboardOpen: true, pennyTyping: true })
    expect(screen.getByAltText('Penny')).toHaveAttribute('data-size', '48')
  })

  it('face is 56 with the keyboard closed', () => {
    setup()
    expect(screen.getByAltText('Penny')).toHaveAttribute('data-size', '56')
  })

  it('hides her words while the keyboard is open and she is idle', () => {
    setup({ keyboardOpen: true })
    expect(screen.queryByText('What can I add?')).not.toBeInTheDocument()
  })

  it('shows her words with the keyboard open while she is typing', () => {
    setup({ keyboardOpen: true, pennyTyping: true })
    expect(screen.getByText(/What can I add\?/)).toBeInTheDocument()
  })

  it('renders action buttons that fire once each', () => {
    const a = vi.fn(), b = vi.fn()
    setup({ buttons: [{ label: 'Undo', onClick: a }, { label: 'Share', onClick: b }] })
    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
    expect(a).toHaveBeenCalledTimes(1)
    expect(b).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Share' })).toBeInTheDocument()
  })

  it('hides action buttons while typing or with the keyboard open', () => {
    const buttons = [{ label: 'Undo', onClick: vi.fn() }]
    const { unmount } = setup({ buttons, pennyTyping: true })
    expect(screen.queryByRole('button', { name: 'Undo' })).not.toBeInTheDocument()
    unmount()
    setup({ buttons, keyboardOpen: true, pennyTyping: true })
    expect(screen.queryByRole('button', { name: 'Undo' })).not.toBeInTheDocument()
  })

  it('uses the error colour on a toast error', () => {
    setup({ toastError: true })
    expect(screen.getByRole('region', { name: 'Penny' })).toHaveStyle({ backgroundColor: 'var(--error-bg, #FEE2E2)' })
  })
})

describe('PennyBar chat input', () => {
  it('has a labelled input with the default placeholder, or a custom one', () => {
    const { unmount } = setup()
    expect(screen.getByRole('textbox', { name: 'Add items or tell Penny anything' })).toHaveAttribute('placeholder', 'Add, edit, or ask Penny…')
    unmount()
    setup({ placeholder: 'Name your list' })
    expect(screen.getByRole('textbox')).toHaveAttribute('placeholder', 'Name your list')
  })

  it('reports typing through onChatTextChange', () => {
    const { props } = setup()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'eggs' } })
    expect(props.onChatTextChange).toHaveBeenCalledWith('eggs')
  })

  it('Enter sends once; Shift+Enter does not', () => {
    const { props } = setup({ chatText: 'two dozen eggs please now' })
    const box = screen.getByRole('textbox')
    fireEvent.keyDown(box, { key: 'Enter', shiftKey: true })
    expect(props.onSend).not.toHaveBeenCalled()
    fireEvent.keyDown(box, { key: 'Enter' })
    expect(props.onSend).toHaveBeenCalledTimes(1)
  })

  it('Send is disabled when empty or loading, enabled with text or a photo', () => {
    const { unmount } = setup()
    expect(screen.getByRole('button', { name: 'Send message' })).toBeDisabled()
    unmount()
    const l = setup({ chatText: 'milk', chatLoading: true })
    expect(screen.getByRole('button', { name: 'Send message' })).toBeDisabled()
    expect(screen.getByRole('textbox')).toBeDisabled()
    l.unmount()
    const t = setup({ chatText: 'a b c d e' })
    expect(screen.getByRole('button', { name: 'Send message' })).toBeEnabled()
    t.unmount()
    setup({ pendingImages: ['data:x'] })
    expect(screen.getByRole('button', { name: 'Send message' })).toBeEnabled()
  })

  it('the Send button sends once', () => {
    const { props } = setup({ chatText: 'a b c d e' })
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }))
    expect(props.onSend).toHaveBeenCalledTimes(1)
  })

  it('camera and library buttons fire once', () => {
    const { props } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Take photo with camera' }))
    fireEvent.click(screen.getByRole('button', { name: 'Add from photo library' }))
    expect(props.onOpenCamera).toHaveBeenCalledTimes(1)
    expect(props.onAddPhoto).toHaveBeenCalledTimes(1)
  })

  it('forwards paste', () => {
    const { props } = setup()
    fireEvent.paste(screen.getByRole('textbox'))
    expect(props.onPaste).toHaveBeenCalledTimes(1)
  })

  it('shows photo previews and removes by index', () => {
    const { props } = setup({ pendingImages: ['data:a', 'data:b'] })
    expect(screen.getByAltText('Preview 1')).toHaveAttribute('src', 'data:a')
    fireEvent.click(screen.getByRole('button', { name: 'Remove image 2' }))
    expect(props.onRemoveImage).toHaveBeenCalledTimes(1)
    expect(props.onRemoveImage).toHaveBeenCalledWith(1)
  })
})

describe('PennyBar auto-submit', () => {
  const run = (ms: number) => act(() => { vi.advanceTimersByTime(ms) })

  it('sends a simple item once, after 1.5s, with a haptic pulse', () => {
    vi.useFakeTimers()
    const { props } = setup({ chatText: 'milk' })
    run(1499)
    expect(props.onSend).not.toHaveBeenCalled()
    run(1)
    expect(props.onSend).toHaveBeenCalledTimes(1)
    expect(haptic.tap).toHaveBeenCalledTimes(1)
    run(5000)
    expect(props.onSend).toHaveBeenCalledTimes(1)
  })

  it('shows the countdown ring while waiting and removes it after sending', () => {
    vi.useFakeTimers()
    const { container } = setup({ chatText: 'milk' })
    expect(container.querySelector('circle[r="15"]')).toBeInTheDocument()
    run(1500)
    expect(container.querySelector('circle[r="15"]')).toBeNull()
  })

  it('typing more restarts the countdown', () => {
    vi.useFakeTimers()
    const { props, rerenderWith } = setup({ chatText: 'milk' })
    run(1000)
    rerenderWith({ chatText: 'milk 2%' })
    run(1000)
    expect(props.onSend).not.toHaveBeenCalled()
    run(500)
    expect(props.onSend).toHaveBeenCalledTimes(1)
  })

  it('pressing Enter sends and cancels the pending auto-send, so it cannot double-send', () => {
    vi.useFakeTimers()
    const { props } = setup({ chatText: 'milk' })
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter' })
    run(5000)
    expect(props.onSend).toHaveBeenCalledTimes(1)
  })

  it('clicking Send cancels the pending auto-send', () => {
    vi.useFakeTimers()
    const { props } = setup({ chatText: 'milk' })
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }))
    run(5000)
    expect(props.onSend).toHaveBeenCalledTimes(1)
  })

  it('clearing the text cancels it', () => {
    vi.useFakeTimers()
    const { props, rerenderWith } = setup({ chatText: 'milk' })
    rerenderWith({ chatText: '' })
    run(5000)
    expect(props.onSend).not.toHaveBeenCalled()
  })

  it('unmounting cancels it', () => {
    vi.useFakeTimers()
    const { props, unmount } = setup({ chatText: 'milk' })
    unmount()
    run(5000)
    expect(props.onSend).not.toHaveBeenCalled()
  })

  it.each([
    ['a command word', 'remove milk'],
    ['a question word', 'what is on my list'],
    ['a question mark', 'milk?'],
    ['more than four words', 'one two three four five'],
    ['a comma-separated list', 'milk, eggs'],
    ['a meal word', 'tacos tonight'],
    ['only whitespace', '   '],
  ])('does not auto-send %s', (_label, chatText) => {
    vi.useFakeTimers()
    const { props } = setup({ chatText })
    run(5000)
    expect(props.onSend).not.toHaveBeenCalled()
  })

  it('does not auto-send when disabled, loading, with photos, or during brand choice', () => {
    vi.useFakeTimers()
    for (const extra of [
      { autoSubmitEnabled: false },
      { chatLoading: true },
      { pendingImages: ['data:x'] },
      { disambig: [d()] },
    ] as Partial<Props>[]) {
      const { props, unmount } = setup({ chatText: 'milk', ...extra })
      run(5000)
      expect(props.onSend).not.toHaveBeenCalled()
      unmount()
    }
  })

  it('calls the latest onSend, not a stale one', () => {
    vi.useFakeTimers()
    const first = vi.fn(), second = vi.fn()
    const props = makeProps({ chatText: 'milk', onSend: first })
    const { rerender } = render(<PennyBar {...props} />)
    rerender(<PennyBar {...props} onSend={second} />)
    run(1500)
    expect(first).not.toHaveBeenCalled()
    expect(second).toHaveBeenCalledTimes(1)
  })
})

describe('PennyBar brand disambiguation', () => {
  it('asks which brand, with choices, a Skip, and a typing hint', () => {
    const { props } = setup({ disambig: [d()] })
    expect(screen.getByText('Milk (Horizon) — which brand?')).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Type a brand name' })).toHaveAttribute('placeholder', 'Or type a brand name…')
    fireEvent.click(screen.getByRole('button', { name: 'Organic Valley' }))
    expect(props.onBrandPick).toHaveBeenCalledTimes(1)
    expect(props.onBrandPick).toHaveBeenCalledWith('Organic Valley')
    fireEvent.click(screen.getByRole('button', { name: 'Skip brand selection' }))
    expect(props.onSkipDisambig).toHaveBeenCalledTimes(1)
  })

  it('omits the brand from the question when there is none', () => {
    setup({ disambig: [d({ current_brand: '' })] })
    expect(screen.getByText('Milk — which brand?')).toBeInTheDocument()
  })

  it('offers the current brand as its own choice when it is not among the alternatives', () => {
    const { props } = setup({ disambig: [d({ current_brand: 'Store Brand', alternatives: ['Horizon', ''] })] })
    fireEvent.click(screen.getByRole('button', { name: 'Store Brand' }))
    expect(props.onBrandPick).toHaveBeenCalledWith('Store Brand')
    const labels = within(screen.getByRole('region', { name: 'Penny' })).getAllByRole('button').map(b => b.textContent)
    expect(labels).toEqual(['Store Brand', 'Horizon', 'Skip'])
  })

  it('shows a position counter for several items', () => {
    setup({ disambig: [d(), d({ product_category: 'Eggs' })], disambigIndex: 1 })
    expect(screen.getByText('Eggs (Horizon) — which brand?')).toBeInTheDocument()
    expect(screen.getByText('2/2')).toBeInTheDocument()
  })

  it('treats an index past the end as no disambiguation', () => {
    setup({ disambig: [d()], disambigIndex: 1 })
    expect(screen.queryByText(/which brand/)).not.toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Add items or tell Penny anything' })).toBeInTheDocument()
  })

  it('auto-assigned: says what she picked; OK confirms it once', () => {
    const { props } = setup({ disambig: [d({ auto_assigned: true })] })
    expect(screen.getByText('I picked Horizon for Milk')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'OK' }))
    expect(props.onBrandPick).toHaveBeenCalledTimes(1)
    expect(props.onBrandPick).toHaveBeenCalledWith('Horizon')
  })

  it('auto-assigned: Change reveals the alternatives', () => {
    const { props } = setup({ disambig: [d({ auto_assigned: true })] })
    expect(screen.queryByRole('button', { name: 'Organic Valley' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Change' }))
    fireEvent.click(screen.getByRole('button', { name: 'Organic Valley' }))
    expect(props.onBrandPick).toHaveBeenCalledWith('Organic Valley')
    expect(props.onBrandPick).toHaveBeenCalledTimes(1)
  })

  it('compact layout with the keyboard open still shows choices and the counter beside the text', () => {
    setup({ keyboardOpen: true, disambig: [d(), d()], disambigIndex: 0 })
    expect(screen.getByText('Milk (Horizon) — which brand?')).toBeInTheDocument()
    expect(screen.getAllByText('1/2')).toHaveLength(1)
    expect(screen.getByRole('button', { name: 'Skip brand selection' })).toBeInTheDocument()
  })

  it('without a face the text still renders', () => {
    setup({ showFace: false, disambig: [d()] })
    expect(screen.queryByAltText('Penny')).not.toBeInTheDocument()
    expect(screen.getByText('Milk (Horizon) — which brand?')).toBeInTheDocument()
  })
})

describe('PennyBar copy', () => {
  it('never says bot, AI or assistant', () => {
    setup({ disambig: [d()] })
    expect(document.body.textContent).not.toMatch(/\b(bot|AI|assistant)\b/i)
  })
})
