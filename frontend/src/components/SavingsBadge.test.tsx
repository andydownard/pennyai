import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SavingsBadge } from './SavingsBadge'

describe('SavingsBadge', () => {
  it('shows the amount alone, with no caption', () => {
    render(<SavingsBadge potentialSavingsCents={1240} earningsTotalCents={0} onOpen={() => {}} />)
    const b = screen.getByRole('button')
    expect(b).toHaveTextContent(/^\$12\.40$/)
    expect(b.textContent).not.toMatch(/save|deals/i)
  })
  it('falls back to earned, then $0.00', () => {
    const { rerender } = render(<SavingsBadge potentialSavingsCents={0} earningsTotalCents={570} onOpen={() => {}} />)
    expect(screen.getByRole('button')).toHaveTextContent('$5.70')
    rerender(<SavingsBadge potentialSavingsCents={0} earningsTotalCents={0} onOpen={() => {}} />)
    expect(screen.getByRole('button')).toHaveTextContent('$0.00')
  })
})
