import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MoneyChip } from './MoneyChip'

describe('MoneyChip', () => {
  it('shows a taken swap as plain text', () => {
    render(<MoneyChip kind="taken" cents={50} />)
    expect(screen.getByText('Saving $0.50')).toBeInTheDocument()
    expect(screen.queryByRole('button')).toBeNull()
  })
  it('makes an offer chip a button that toggles', () => {
    const onToggle = vi.fn()
    render(<MoneyChip kind="offer" cents={40} expanded onToggle={onToggle} />)
    const b = screen.getByRole('button', { name: 'Deal: save $0.40' })
    expect(b).toHaveAttribute('aria-expanded', 'true')
    fireEvent.click(b)
    expect(onToggle).toHaveBeenCalledTimes(1)
  })
})
