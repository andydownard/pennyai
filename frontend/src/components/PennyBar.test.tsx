// Penny's face always sits in her bar, beside her words, including on an empty list.

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { createRef } from 'react'
import { PennyBar } from './PennyBar'

function renderBar(extra: Partial<React.ComponentProps<typeof PennyBar>> = {}) {
  return render(
    <PennyBar
      pennyText="What can I add?" pennyTyping={false} buttons={[]} toastError={false}
      keyboardOpen={false} disambig={[]} disambigIndex={0}
      onBrandPick={vi.fn()} onSkipDisambig={vi.fn()}
      pendingImages={[]} onRemoveImage={vi.fn()}
      chatText="" onChatTextChange={vi.fn()} chatLoading={false} onSend={vi.fn()}
      onPaste={vi.fn()} onAddPhoto={vi.fn()} onOpenCamera={vi.fn()}
      chatInputRef={createRef<HTMLTextAreaElement>()}
      {...extra}
    />,
  )
}

describe('PennyBar face', () => {
  it('shows her face by default', () => {
    renderBar()
    expect(screen.getByAltText('Penny')).toBeInTheDocument()
  })

  it('shows her words beside her face', () => {
    renderBar()
    expect(screen.getAllByText('What can I add?').length).toBeGreaterThan(0)
  })
})
