// Settings is its own screen: Penny's settings, under the page header.

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SettingsScreen } from './SettingsScreen'

describe('SettingsScreen', () => {
  it('shows the page header and its content', () => {
    render(
      <SettingsScreen renderHeader={() => <h1>Settings</h1>} transitionStyle={{}}>
        <div>penny-settings</div>
      </SettingsScreen>,
    )
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument()
    expect(screen.getByText('penny-settings')).toBeInTheDocument()
  })
})
