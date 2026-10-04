// The shared list view is read-only and collects nothing. Joining goes through
// the login screen; the invite waits in storage (see auth/pendingLinks).

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SharedListScreen } from './SharedListScreen'

const data = {
  items: [{ name: 'Milk', quantity: 1, checked: false }],
  sharer_name: 'Tamika',
  expires_at: new Date(Date.now() + 3 * 3600_000).toISOString(),
  invite_code: 'INV42',
}

describe('SharedListScreen', () => {
  it('shows the list and one Join button, with no phone or code inputs', () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    const onContinue = vi.fn()
    const { container } = render(
      <SharedListScreen sharedData={data} sharedLoading={false} sharedError={null} onContinue={onContinue} />,
    )
    expect(screen.getByText('Milk')).toBeInTheDocument()
    expect(container.querySelector('input')).toBeNull()

    screen.getByRole('button', { name: 'Join This List' }).click()
    expect(onContinue).toHaveBeenCalledTimes(1)
    expect(fetchMock).not.toHaveBeenCalled()
    vi.unstubAllGlobals()
  })

  it('offers Get Started when the link carries no invite', () => {
    const onContinue = vi.fn()
    render(
      <SharedListScreen sharedData={{ ...data, invite_code: undefined }} sharedLoading={false} sharedError={null} onContinue={onContinue} />,
    )
    screen.getByRole('button', { name: 'Get Started' }).click()
    expect(onContinue).toHaveBeenCalledTimes(1)
  })

  it('still offers a way forward on an expired link', () => {
    const onContinue = vi.fn()
    render(<SharedListScreen sharedData={null} sharedLoading={false} sharedError="expired" onContinue={onContinue} />)
    expect(screen.getByText('This link has expired')).toBeInTheDocument()
    screen.getAllByRole('button', { name: 'Get Started' })[0].click()
    expect(onContinue).toHaveBeenCalled()
  })
})
