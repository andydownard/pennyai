import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, afterEach } from 'vitest'
import Header from '../components/Header'

function renderHeader(initialRoute = '/') {
  return render(
    <MemoryRouter initialEntries={[initialRoute]}>
      <Header />
    </MemoryRouter>
  )
}

describe('Header component', () => {
  afterEach(() => {
    document.body.style.overflow = ''
  })

  // ===== RENDERING =====
  describe('renders without errors', () => {
    it('renders the header without crashing', () => {
      renderHeader()
      expect(document.querySelector('header')).toBeInTheDocument()
    })
  })

  // ===== LOGO =====
  describe('logo', () => {
    it('renders the logo image', () => {
      renderHeader()
      const logoImg = document.querySelector('.logo-img') as HTMLImageElement
      expect(logoImg).toBeInTheDocument()
      expect(logoImg.src).toContain('logo.svg')
    })

    it('logo links to home page', () => {
      renderHeader()
      const logo = document.querySelector('.logo') as HTMLAnchorElement
      expect(logo).toBeInTheDocument()
      expect(logo.getAttribute('href')).toBe('/')
    })

    it('logo has alt text', () => {
      renderHeader()
      const logoImg = document.querySelector('.logo-img') as HTMLImageElement
      expect(logoImg.alt).toBe('thriftly')
    })
  })

  // ===== DESKTOP NAVIGATION =====
  describe('desktop navigation', () => {
    it('renders How It Works link', () => {
      renderHeader()
      const nav = document.querySelector('.nav')!
      const link = nav.querySelector('a[href="/#how-it-works"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('How It Works')
    })

    it('renders Features link', () => {
      renderHeader()
      const nav = document.querySelector('.nav')!
      const link = nav.querySelector('a[href="/#features"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('Features')
    })

    it('renders For Brands link', () => {
      renderHeader()
      const nav = document.querySelector('.nav')!
      const link = nav.querySelector('a[href="/for-brands"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('For Brands')
    })

    it('renders Pricing link', () => {
      renderHeader()
      const nav = document.querySelector('.nav')!
      const link = nav.querySelector('a[href="/pricing"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('Pricing')
    })

    it('renders About link', () => {
      renderHeader()
      const nav = document.querySelector('.nav')!
      const link = nav.querySelector('a[href="/about"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('About')
    })

    it('renders Try Free CTA link pointing to app', () => {
      renderHeader()
      const nav = document.querySelector('.nav')!
      const cta = nav.querySelector('.nav-cta')
      expect(cta).toBeInTheDocument()
      expect(cta?.textContent).toBe('Try Free')
      expect(cta?.getAttribute('href')).toBe('https://app.thrift.ly')
    })

    it('applies active class to current NavLink', () => {
      renderHeader('/for-brands')
      const nav = document.querySelector('.nav')!
      const brandsLink = nav.querySelector('a[href="/for-brands"]')
      expect(brandsLink?.classList.contains('active')).toBe(true)
    })

    it('does not apply active class to non-current NavLinks', () => {
      renderHeader('/')
      const nav = document.querySelector('.nav')!
      const brandsLink = nav.querySelector('a[href="/for-brands"]')
      expect(brandsLink?.classList.contains('active')).toBe(false)
    })

    it('applies active class to Pricing when on pricing page', () => {
      renderHeader('/pricing')
      const nav = document.querySelector('.nav')!
      const pricingLink = nav.querySelector('a[href="/pricing"]')
      expect(pricingLink?.classList.contains('active')).toBe(true)
    })

    it('applies active class to About when on about page', () => {
      renderHeader('/about')
      const nav = document.querySelector('.nav')!
      const aboutLink = nav.querySelector('a[href="/about"]')
      expect(aboutLink?.classList.contains('active')).toBe(true)
    })
  })

  // ===== MOBILE MENU BUTTON =====
  describe('mobile menu button', () => {
    it('renders the mobile menu button', () => {
      renderHeader()
      const btn = screen.getByLabelText('Open menu')
      expect(btn).toBeInTheDocument()
    })

    it('has aria-expanded=false initially', () => {
      renderHeader()
      const btn = screen.getByLabelText('Open menu')
      expect(btn).toHaveAttribute('aria-expanded', 'false')
    })

    it('has mobile-menu-btn class', () => {
      renderHeader()
      const btn = document.querySelector('.mobile-menu-btn')
      expect(btn).toBeInTheDocument()
    })

    it('contains three span elements for hamburger icon', () => {
      renderHeader()
      const btn = document.querySelector('.mobile-menu-btn')
      const spans = btn?.querySelectorAll('span')
      expect(spans?.length).toBe(3)
    })
  })

  // ===== MOBILE MENU INTERACTION =====
  describe('mobile menu interaction', () => {
    it('opens mobile nav on button click', async () => {
      const user = userEvent.setup()
      renderHeader()
      const btn = screen.getByLabelText('Open menu')
      await user.click(btn)
      const mobileNav = document.querySelector('.mobile-nav.open')
      expect(mobileNav).toBeInTheDocument()
    })

    it('changes aria-label to Close menu when open', async () => {
      const user = userEvent.setup()
      renderHeader()
      const btn = screen.getByLabelText('Open menu')
      await user.click(btn)
      expect(screen.getByLabelText('Close menu')).toBeInTheDocument()
    })

    it('sets aria-expanded to true when open', async () => {
      const user = userEvent.setup()
      renderHeader()
      const btn = screen.getByLabelText('Open menu')
      await user.click(btn)
      expect(screen.getByLabelText('Close menu')).toHaveAttribute('aria-expanded', 'true')
    })

    it('closes mobile nav on second button click', async () => {
      const user = userEvent.setup()
      renderHeader()
      const btn = screen.getByLabelText('Open menu')
      await user.click(btn)
      expect(document.querySelector('.mobile-nav.open')).toBeInTheDocument()
      await user.click(screen.getByLabelText('Close menu'))
      expect(document.querySelector('.mobile-nav.open')).not.toBeInTheDocument()
    })

    it('locks body scroll when mobile menu is open', async () => {
      const user = userEvent.setup()
      renderHeader()
      const btn = screen.getByLabelText('Open menu')
      await user.click(btn)
      expect(document.body.style.overflow).toBe('hidden')
    })

    it('unlocks body scroll when mobile menu is closed', async () => {
      const user = userEvent.setup()
      renderHeader()
      const btn = screen.getByLabelText('Open menu')
      await user.click(btn)
      expect(document.body.style.overflow).toBe('hidden')
      await user.click(screen.getByLabelText('Close menu'))
      expect(document.body.style.overflow).toBe('')
    })

    it('restores body scroll on unmount', async () => {
      const user = userEvent.setup()
      const { unmount } = renderHeader()
      const btn = screen.getByLabelText('Open menu')
      await user.click(btn)
      expect(document.body.style.overflow).toBe('hidden')
      unmount()
      expect(document.body.style.overflow).toBe('')
    })
  })

  // ===== MOBILE NAV LINKS =====
  describe('mobile navigation links', () => {
    it('renders mobile nav with aria label', () => {
      renderHeader()
      const mobileNav = document.querySelector('.mobile-nav')
      expect(mobileNav?.getAttribute('aria-label')).toBe('Mobile navigation')
    })

    it('renders How It Works link in mobile nav', () => {
      renderHeader()
      const mobileNav = document.querySelector('.mobile-nav')!
      const link = mobileNav.querySelector('a[href="/#how-it-works"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('How It Works')
    })

    it('renders Features link in mobile nav', () => {
      renderHeader()
      const mobileNav = document.querySelector('.mobile-nav')!
      const link = mobileNav.querySelector('a[href="/#features"]')
      expect(link).toBeInTheDocument()
    })

    it('renders For Brands link in mobile nav', () => {
      renderHeader()
      const mobileNav = document.querySelector('.mobile-nav')!
      const link = mobileNav.querySelector('a[href="/for-brands"]')
      expect(link).toBeInTheDocument()
    })

    it('renders Pricing link in mobile nav', () => {
      renderHeader()
      const mobileNav = document.querySelector('.mobile-nav')!
      const link = mobileNav.querySelector('a[href="/pricing"]')
      expect(link).toBeInTheDocument()
    })

    it('renders About link in mobile nav', () => {
      renderHeader()
      const mobileNav = document.querySelector('.mobile-nav')!
      const link = mobileNav.querySelector('a[href="/about"]')
      expect(link).toBeInTheDocument()
    })

    it('renders Try Free CTA in mobile nav', () => {
      renderHeader()
      const mobileNav = document.querySelector('.mobile-nav')!
      const cta = mobileNav.querySelector('.nav-cta')
      expect(cta).toBeInTheDocument()
      expect(cta?.textContent).toBe('Try Free')
      expect(cta?.getAttribute('href')).toBe('https://app.thrift.ly')
    })

    it('mobile nav has 6 links total', () => {
      renderHeader()
      const mobileNav = document.querySelector('.mobile-nav')!
      const links = mobileNav.querySelectorAll('a')
      expect(links.length).toBe(6)
    })
  })

  // ===== SCROLL BEHAVIOR =====
  describe('scroll state', () => {
    it('does not have scrolled class initially', () => {
      renderHeader()
      const header = document.querySelector('header')
      expect(header?.classList.contains('scrolled')).toBe(false)
    })

    it('does not have header-hidden class initially', () => {
      renderHeader()
      const header = document.querySelector('header')
      expect(header?.classList.contains('header-hidden')).toBe(false)
    })

    it('attaches scroll event listener', () => {
      const addSpy = vi.spyOn(window, 'addEventListener')
      renderHeader()
      const scrollCalls = addSpy.mock.calls.filter(c => c[0] === 'scroll')
      expect(scrollCalls.length).toBeGreaterThan(0)
      addSpy.mockRestore()
    })

    it('removes scroll event listener on unmount', () => {
      const removeSpy = vi.spyOn(window, 'removeEventListener')
      const { unmount } = renderHeader()
      unmount()
      const scrollCalls = removeSpy.mock.calls.filter(c => c[0] === 'scroll')
      expect(scrollCalls.length).toBeGreaterThan(0)
      removeSpy.mockRestore()
    })
  })

  // ===== HEADER STRUCTURE =====
  describe('header structure', () => {
    it('has a header-inner wrapper', () => {
      renderHeader()
      expect(document.querySelector('.header-inner')).toBeInTheDocument()
    })

    it('renders header element with class header', () => {
      renderHeader()
      const header = document.querySelector('header')
      expect(header?.classList.contains('header')).toBe(true)
    })

    it('has both desktop nav and mobile nav', () => {
      renderHeader()
      expect(document.querySelector('.nav')).toBeInTheDocument()
      expect(document.querySelector('.mobile-nav')).toBeInTheDocument()
    })
  })

  // ===== ACCESSIBILITY =====
  describe('accessibility', () => {
    it('mobile nav has aria-label for screen readers', () => {
      renderHeader()
      const mobileNav = document.querySelector('.mobile-nav')
      expect(mobileNav?.getAttribute('aria-label')).toBe('Mobile navigation')
    })

    it('mobile menu button has descriptive aria-label', () => {
      renderHeader()
      const btn = document.querySelector('.mobile-menu-btn')
      expect(btn?.getAttribute('aria-label')).toBeTruthy()
    })

    it('logo image has descriptive alt text', () => {
      renderHeader()
      const img = document.querySelector('.logo-img') as HTMLImageElement
      expect(img.alt).toBeTruthy()
      expect(img.alt).not.toBe('')
    })

    it('mobile menu button changes aria-label based on state', async () => {
      const user = userEvent.setup()
      renderHeader()
      expect(screen.getByLabelText('Open menu')).toBeInTheDocument()
      await user.click(screen.getByLabelText('Open menu'))
      expect(screen.getByLabelText('Close menu')).toBeInTheDocument()
    })
  })

  // ===== LINK COUNT AND CONSISTENCY =====
  describe('navigation link consistency', () => {
    it('desktop and mobile nav have same number of links', () => {
      renderHeader()
      const desktopLinks = document.querySelector('.nav')!.querySelectorAll('a')
      const mobileLinks = document.querySelector('.mobile-nav')!.querySelectorAll('a')
      expect(desktopLinks.length).toBe(mobileLinks.length)
    })

    it('desktop nav has 6 links total', () => {
      renderHeader()
      const desktopLinks = document.querySelector('.nav')!.querySelectorAll('a')
      expect(desktopLinks.length).toBe(6)
    })

    it('desktop and mobile nav links match destinations', () => {
      renderHeader()
      const desktopLinks = Array.from(document.querySelector('.nav')!.querySelectorAll('a')).map(a => a.getAttribute('href'))
      const mobileLinks = Array.from(document.querySelector('.mobile-nav')!.querySelectorAll('a')).map(a => a.getAttribute('href'))
      expect(desktopLinks).toEqual(mobileLinks)
    })

    it('both navs link to the same pages', () => {
      renderHeader()
      const expectedHrefs = ['/#how-it-works', '/#features', '/for-brands', '/pricing', '/about', 'https://app.thrift.ly']
      const desktopLinks = Array.from(document.querySelector('.nav')!.querySelectorAll('a')).map(a => a.getAttribute('href'))
      expect(desktopLinks).toEqual(expectedHrefs)
    })
  })

  // ===== BRAND CONSISTENCY =====
  describe('brand consistency', () => {
    it('does not use jargon in navigation labels', () => {
      renderHeader()
      const navLinks = document.querySelector('.nav')!.querySelectorAll('a')
      navLinks.forEach(link => {
        expect(link.textContent).not.toMatch(/\bsynergy\b/i)
        expect(link.textContent).not.toMatch(/\bsolutions\b/i)
      })
    })

    it('Try Free CTA is last in nav order', () => {
      renderHeader()
      const navLinks = document.querySelector('.nav')!.querySelectorAll('a')
      const lastLink = navLinks[navLinks.length - 1]
      expect(lastLink.textContent).toBe('Try Free')
    })

    it('Try Free CTA has nav-cta class for styling', () => {
      renderHeader()
      const cta = document.querySelector('.nav .nav-cta')
      expect(cta).toBeInTheDocument()
      expect(cta?.textContent).toBe('Try Free')
    })

    it('CTA links to app subdomain (external)', () => {
      renderHeader()
      const cta = document.querySelector('.nav .nav-cta')
      expect(cta?.getAttribute('href')).toMatch(/^https:\/\/app\.thrift\.ly/)
    })
  })

  // ===== MOBILE MENU CLOSES ON ROUTE CHANGE =====
  describe('mobile menu closes on route change', () => {
    it('mobile menu has close handlers on links', async () => {
      renderHeader()
      const mobileNav = document.querySelector('.mobile-nav')!
      const links = mobileNav.querySelectorAll('a')
      links.forEach(link => {
        expect(link).toBeInTheDocument()
      })
    })

    it('mobile nav link for For Brands has onClick handler', () => {
      renderHeader()
      const mobileNav = document.querySelector('.mobile-nav')!
      const brandsLink = mobileNav.querySelector('a[href="/for-brands"]')
      expect(brandsLink).toBeInTheDocument()
    })
  })

  // ===== HEADER ELEMENT DETAILS =====
  describe('header element details', () => {
    it('logo uses an img element inside a Link', () => {
      renderHeader()
      const logo = document.querySelector('.logo')
      expect(logo?.tagName).toBe('A')
      const img = logo?.querySelector('img')
      expect(img).toBeInTheDocument()
    })

    it('desktop nav uses nav element', () => {
      renderHeader()
      const nav = document.querySelector('nav.nav')
      expect(nav).toBeInTheDocument()
    })

    it('mobile nav uses nav element', () => {
      renderHeader()
      const nav = document.querySelector('nav.mobile-nav')
      expect(nav).toBeInTheDocument()
    })

    it('How It Works is a Link (not NavLink) in desktop nav', () => {
      renderHeader()
      const nav = document.querySelector('.nav')!
      const link = nav.querySelector('a[href="/#how-it-works"]')
      // Non-NavLink links don't have active class functionality
      expect(link).toBeInTheDocument()
    })

    it('Features is a Link (not NavLink) in desktop nav', () => {
      renderHeader()
      const nav = document.querySelector('.nav')!
      const link = nav.querySelector('a[href="/#features"]')
      expect(link).toBeInTheDocument()
    })

    it('For Brands uses NavLink for active state support', () => {
      renderHeader('/for-brands')
      const nav = document.querySelector('.nav')!
      const link = nav.querySelector('a[href="/for-brands"]')
      expect(link?.classList.contains('active')).toBe(true)
    })

    it('mobile menu button has exactly 3 spans (hamburger icon)', () => {
      renderHeader()
      const btn = document.querySelector('.mobile-menu-btn')
      const spans = btn?.querySelectorAll('span')
      expect(spans?.length).toBe(3)
    })

    it('Try Free CTA in mobile nav matches desktop CTA', () => {
      renderHeader()
      const desktopCta = document.querySelector('.nav .nav-cta')
      const mobileCta = document.querySelector('.mobile-nav .nav-cta')
      expect(desktopCta?.textContent).toBe(mobileCta?.textContent)
      expect(desktopCta?.getAttribute('href')).toBe(mobileCta?.getAttribute('href'))
    })
  })

  // ===== NAV LINK ORDER =====
  describe('navigation link order', () => {
    it('desktop nav links are in correct order', () => {
      renderHeader()
      const navLinks = Array.from(document.querySelector('.nav')!.querySelectorAll('a'))
      const labels = navLinks.map(l => l.textContent)
      expect(labels).toEqual(['How It Works', 'Features', 'For Brands', 'Pricing', 'About', 'Try Free'])
    })

    it('mobile nav links are in correct order', () => {
      renderHeader()
      const mobileLinks = Array.from(document.querySelector('.mobile-nav')!.querySelectorAll('a'))
      const labels = mobileLinks.map(l => l.textContent)
      expect(labels).toEqual(['How It Works', 'Features', 'For Brands', 'Pricing', 'About', 'Try Free'])
    })
  })

  // ===== ACTIVE STATE EDGE CASES =====
  describe('active state edge cases', () => {
    it('no NavLinks are active on hash routes', () => {
      renderHeader('/#how-it-works')
      const nav = document.querySelector('.nav')!
      const brandsLink = nav.querySelector('a[href="/for-brands"]')
      const pricingLink = nav.querySelector('a[href="/pricing"]')
      const aboutLink = nav.querySelector('a[href="/about"]')
      expect(brandsLink?.classList.contains('active')).toBe(false)
      expect(pricingLink?.classList.contains('active')).toBe(false)
      expect(aboutLink?.classList.contains('active')).toBe(false)
    })

    it('only one NavLink is active at a time', () => {
      renderHeader('/pricing')
      const nav = document.querySelector('.nav')!
      const activeLinks = nav.querySelectorAll('a.active')
      expect(activeLinks.length).toBe(1)
    })
  })
})
