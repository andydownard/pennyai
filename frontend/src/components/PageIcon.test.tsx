import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { PageIcon } from './PageIcon'
import { NavMenu } from './NavMenu'

// Hard rule: the nav and the page title draw the same icon for a page.
describe('PageIcon', () => {
  it('draws the settings gear, not a star', () => {
    const { container } = render(<PageIcon name="settings" />)
    expect(container.querySelector('circle')).toBeTruthy()
    expect(container.innerHTML).not.toContain('M12 2l1.09 3.26')
  })

  it('nav Settings uses the same icon as the page title', () => {
    const title = render(<PageIcon name="settings" size={20} />).container.innerHTML
    const nav = render(
      <NavMenu open active="home" isAdmin={false} userId={1} initials="AD" displayName="A D" onNavigate={() => {}} onLogout={() => {}} onClose={() => {}} />,
    ).container
    const btn = Array.from(nav.querySelectorAll('button')).find(b => b.textContent === 'Settings')!
    expect(btn.querySelector('svg')!.outerHTML.replace(/ stroke="[^"]*"/, '')).toBe(title.replace(/ stroke="[^"]*"/, ''))
  })
})
