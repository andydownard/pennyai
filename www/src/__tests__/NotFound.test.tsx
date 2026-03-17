import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect } from 'vitest'
import NotFound from '../pages/NotFound'

function renderNotFound() {
  return render(
    <MemoryRouter>
      <NotFound />
    </MemoryRouter>
  )
}

describe('NotFound page', () => {
  // ===== RENDERING =====
  describe('renders without errors', () => {
    it('renders the page without crashing', () => {
      renderNotFound()
      expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
    })

    it('sets the document title to include Page Not Found', () => {
      renderNotFound()
      expect(document.title).toContain('Page Not Found')
    })
  })

  // ===== CONTENT =====
  describe('content', () => {
    it('displays 404 heading', () => {
      renderNotFound()
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('404')
    })

    it('displays helpful message about the page not existing', () => {
      renderNotFound()
      expect(
        screen.getByText("This page doesn't exist. Let's get you back on track.")
      ).toBeInTheDocument()
    })

    it('does not use jargon in the error message', () => {
      renderNotFound()
      const message = screen.getByText(/doesn't exist/)
      expect(message.textContent).not.toMatch(/\berror\b/i)
      expect(message.textContent).not.toMatch(/\bexception\b/i)
    })
  })

  // ===== NAVIGATION LINKS =====
  describe('navigation links', () => {
    it('has a Get Penny Free link pointing to /', () => {
      renderNotFound()
      const homeLink = screen.getByText(/Get Penny Free/)
      expect(homeLink).toHaveAttribute('href', '/')
    })

    it('has a For Brands link pointing to /for-brands', () => {
      renderNotFound()
      const brandsLink = screen.getByText('For Brands')
      expect(brandsLink).toHaveAttribute('href', '/for-brands')
    })

    it('Get Penny Free link uses btn-primary class', () => {
      renderNotFound()
      const homeLink = screen.getByText(/Get Penny Free/)
      expect(homeLink.classList.contains('btn-primary')).toBe(true)
    })

    it('For Brands link uses btn-secondary class', () => {
      renderNotFound()
      const brandsLink = screen.getByText('For Brands')
      expect(brandsLink.classList.contains('btn-secondary')).toBe(true)
    })
  })

  // ===== STYLING =====
  describe('styling', () => {
    it('has centered text alignment', () => {
      renderNotFound()
      const section = document.querySelector('section')
      expect(section?.style.textAlign).toBe('center')
    })

    it('uses brand green color for 404 heading', () => {
      renderNotFound()
      const h1 = screen.getByRole('heading', { level: 1 })
      expect(h1.style.color).toBe('var(--green-600)')
    })

    it('has a large font size for the 404 heading', () => {
      renderNotFound()
      const h1 = screen.getByRole('heading', { level: 1 })
      expect(h1.style.fontSize).toBe('4rem')
    })
  })

  // ===== PAGE STRUCTURE =====
  describe('page structure', () => {
    it('renders a single section', () => {
      renderNotFound()
      const sections = document.querySelectorAll('section')
      expect(sections.length).toBe(1)
    })

    it('renders exactly two navigation links', () => {
      renderNotFound()
      const links = document.querySelectorAll('a')
      expect(links.length).toBe(2)
    })
  })

  // ===== ACCESSIBILITY =====
  describe('accessibility', () => {
    it('has exactly one h1 element', () => {
      renderNotFound()
      const h1s = screen.getAllByRole('heading', { level: 1 })
      expect(h1s.length).toBe(1)
    })

    it('navigation links use descriptive text (not just "click here")', () => {
      renderNotFound()
      const links = document.querySelectorAll('a')
      links.forEach(link => {
        expect(link.textContent).not.toMatch(/^click here$/i)
        expect(link.textContent!.length).toBeGreaterThan(2)
      })
    })

    it('uses clear, friendly error messaging (no technical jargon)', () => {
      renderNotFound()
      const allText = document.body.textContent || ''
      expect(allText).not.toMatch(/\bstack trace\b/i)
      expect(allText).not.toMatch(/\bsegfault\b/i)
      expect(allText).not.toMatch(/\bexception\b/i)
    })
  })

  // ===== USER GUIDANCE =====
  describe('user guidance', () => {
    it('primary action is going home (most likely intent)', () => {
      renderNotFound()
      const links = document.querySelectorAll('a')
      const firstLink = links[0]
      expect(firstLink.getAttribute('href')).toBe('/')
      expect(firstLink.classList.contains('btn-primary')).toBe(true)
    })

    it('secondary action is exploring brands page', () => {
      renderNotFound()
      const links = document.querySelectorAll('a')
      const secondLink = links[1]
      expect(secondLink.getAttribute('href')).toBe('/for-brands')
      expect(secondLink.classList.contains('btn-secondary')).toBe(true)
    })

    it('message reassures the user (not blaming them)', () => {
      renderNotFound()
      const message = screen.getByText(/doesn't exist/)
      expect(message.textContent).toContain("Let's get you back on track")
    })
  })

  // ===== BRAND CONSISTENCY =====
  describe('brand consistency', () => {
    it('uses brand green color for 404 heading', () => {
      renderNotFound()
      const h1 = screen.getByRole('heading', { level: 1 })
      expect(h1.style.color).toBe('var(--green-600)')
    })

    it('does not use corporate jargon', () => {
      renderNotFound()
      const allText = document.body.textContent || ''
      expect(allText).not.toMatch(/\bsynergy\b/i)
      expect(allText).not.toMatch(/\bleverage\b/i)
      expect(allText).not.toMatch(/\boptimize\b/i)
    })

    it('links are styled with btn-primary and btn-secondary classes', () => {
      renderNotFound()
      const links = document.querySelectorAll('a')
      expect(links[0].classList.contains('btn-primary')).toBe(true)
      expect(links[1].classList.contains('btn-secondary')).toBe(true)
    })

    it('uses warm, conversational tone', () => {
      renderNotFound()
      const message = screen.getByText(/doesn't exist/)
      expect(message.textContent).toMatch(/Let's/)
    })
  })

  // ===== LINK ELEMENTS =====
  describe('link elements', () => {
    it('Get Penny Free link uses anchor tag', () => {
      renderNotFound()
      const homeLink = screen.getByText(/Get Penny Free/)
      expect(homeLink.tagName).toBe('A')
    })

    it('For Brands link uses anchor tag', () => {
      renderNotFound()
      const brandsLink = screen.getByText('For Brands')
      expect(brandsLink.tagName).toBe('A')
    })

    it('links have navigation actions wrapper', () => {
      renderNotFound()
      const links = document.querySelectorAll('a')
      expect(links[0].parentElement).toBe(links[1].parentElement)
    })
  })
})
