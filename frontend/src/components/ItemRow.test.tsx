// A brand swap's savings are not in hand until the shopper buys it: the badge
// says "Saving", never "Saved".

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ItemRow, type ListItem } from './ItemRow'

const item = {
  id: 1, name: 'Cereal', quantity: '1', checked: false, urgent: false,
  created_at: '2020-01-01T00:00:00Z', added_by_name: 'Andy', claimed_by_id: null,
  claimed_by_name: '', swap_savings_cents: 150, emoji: '',
} as unknown as ListItem

function renderRow(overrides: Partial<ListItem> = {}) {
  return render(
    <ItemRow
      item={{ ...item, ...overrides }} isNew={false} currentUserId={1}
      onToggle={vi.fn()} onTap={vi.fn()} onDelete={vi.fn()} onLongPress={vi.fn()}
      onSwapAccept={vi.fn()} onSwapDecline={vi.fn()}
    />,
  )
}

describe('ItemRow swap badge', () => {
  it('says Saving, not Saved', () => {
    renderRow()
    expect(screen.getByText('Saving $1.50')).toBeInTheDocument()
    expect(screen.queryByText(/^Saved/)).not.toBeInTheDocument()
  })

  it('shows no badge without savings', () => {
    renderRow({ swap_savings_cents: 0 })
    expect(screen.queryByText(/Saving/)).not.toBeInTheDocument()
  })
})
