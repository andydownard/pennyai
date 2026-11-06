// Edit item is a short form: brand leads, name/size/qty follow, then one quiet
// row per setting. Notes hide under "More details"; priority is a plain select.

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ItemEditSheet } from './ItemEditSheet'
import { type ListItem } from './ItemRow'

const item = {
  id: 7, name: 'Whole milk', quantity: '1', checked: false, urgent: false,
  created_at: '2020-01-01T00:00:00Z', added_by_name: 'Andy', claimed_by_id: null,
  claimed_by_name: '', swap_savings_cents: 0, emoji: '🥛',
} as unknown as ListItem

const form = { name: 'Whole milk', brand: 'Horizon', dept: 'Dairy & Eggs', size: '1 gal', qty: 1, notes: '' }

function renderSheet(overrides: Partial<ListItem> = {}, handlers: Record<string, unknown> = {}) {
  const props = {
    item: { ...item, ...overrides }, editForm: form, setEditForm: vi.fn(),
    displayBrand: 'Horizon', setDisplayBrand: vi.fn(), editLoading: false,
    currentUserId: 1, itemHistory: [], onSave: vi.fn(), onDelete: vi.fn(), onClose: vi.fn(),
    onUrgentToggle: vi.fn(), onClaim: vi.fn(), setSelectedItem: vi.fn(), openConfirmModal: vi.fn(),
    listStyles: { overlay: {}, floatingInput: {}, floatingSelect: {}, sheetRow: {}, sheetButton: {}, sheetDeleteBtn: {} },
    ToggleSwitch: () => <button role="switch" aria-label="Claim this item" />,
    QuantityControl: () => <span>qty</span>,
    ...handlers,
  }
  render(<ItemEditSheet {...(props as React.ComponentProps<typeof ItemEditSheet>)} />)
  return props
}

describe('ItemEditSheet short form', () => {
  it('leads with the brand name as text', () => {
    renderSheet()
    expect(screen.getByText('Horizon')).toBeInTheDocument()
  })

  it('hides Notes until More details is opened, then hides it again', () => {
    renderSheet()
    expect(screen.queryByLabelText('Notes')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /more details/i }))
    expect(screen.getByLabelText('Notes')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /fewer details/i }))
    expect(screen.queryByLabelText('Notes')).not.toBeInTheDocument()
  })

  it('shows Department and Priority as labelled selects', () => {
    renderSheet()
    expect(screen.getByRole('combobox', { name: 'Department' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Priority' })).toHaveValue('normal')
  })

  it('makes an item urgent from the Priority select', () => {
    const p = renderSheet()
    fireEvent.change(screen.getByRole('combobox', { name: 'Priority' }), { target: { value: 'urgent' } })
    expect(p.onUrgentToggle).toHaveBeenCalledWith(7)
  })

  it('does not toggle when the priority is unchanged', () => {
    const p = renderSheet({ urgent: true })
    expect(screen.getByRole('combobox', { name: 'Priority' })).toHaveValue('urgent')
    fireEvent.change(screen.getByRole('combobox', { name: 'Priority' }), { target: { value: 'urgent' } })
    expect(p.onUrgentToggle).not.toHaveBeenCalled()
  })

  it('shows no History heading when there is no history', () => {
    renderSheet()
    expect(screen.queryByText(/history/i)).not.toBeInTheDocument()
  })

  it('keeps Save and Delete', () => {
    renderSheet()
    expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Delete item' })).toBeInTheDocument()
  })
})
