import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect } from 'vitest'
import Terms from '../pages/Terms'

function renderTerms() {
  return render(
    <MemoryRouter>
      <Terms />
    </MemoryRouter>
  )
}

describe('Terms page', () => {
  // ===== RENDERING =====
  describe('renders without errors', () => {
    it('renders the page without crashing', () => {
      renderTerms()
      expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
    })

    it('sets the document title to include Terms of Service', () => {
      renderTerms()
      expect(document.title).toContain('Terms of Service')
    })
  })

  // ===== MAIN HEADING =====
  describe('page header', () => {
    it('displays the Terms of Service heading', () => {
      renderTerms()
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Terms of Service')
    })

    it('displays the last updated date', () => {
      renderTerms()
      expect(screen.getByText('Last updated: March 2026')).toBeInTheDocument()
    })

    it('uses the legal-updated class for the date', () => {
      renderTerms()
      const updated = document.querySelector('.legal-updated')
      expect(updated).toBeInTheDocument()
      expect(updated?.textContent).toContain('March 2026')
    })
  })

  // ===== CONTENT SECTIONS =====
  describe('content sections', () => {
    it('has a For shoppers section', () => {
      renderTerms()
      expect(screen.getByText('For shoppers')).toBeInTheDocument()
      expect(screen.getByText(/free to use/)).toBeInTheDocument()
    })

    it('mentions clearly labeled product suggestions for shoppers', () => {
      renderTerms()
      expect(screen.getByText(/clearly labeled product suggestions/)).toBeInTheDocument()
    })

    it('has a For advertisers section', () => {
      renderTerms()
      expect(screen.getByText('For advertisers')).toBeInTheDocument()
      expect(screen.getByText(/truthful product information/)).toBeInTheDocument()
    })

    it('mentions campaign review rights', () => {
      renderTerms()
      expect(screen.getByText(/reserve the right to reject or remove/)).toBeInTheDocument()
    })

    it('has an Advertising guidelines section', () => {
      renderTerms()
      expect(screen.getByText('Advertising guidelines')).toBeInTheDocument()
      expect(screen.getByText(/sponsored content.*must be clearly labeled/)).toBeInTheDocument()
    })

    it('prohibits harmful, misleading, or inappropriate products', () => {
      renderTerms()
      expect(screen.getByText(/harmful, misleading, or inappropriate/)).toBeInTheDocument()
    })

    it('has a Payment terms section', () => {
      renderTerms()
      expect(screen.getByText('Payment terms')).toBeInTheDocument()
      expect(screen.getByText(/CPM or CPC/)).toBeInTheDocument()
    })

    it('mentions the ability to pause or cancel anytime', () => {
      renderTerms()
      expect(screen.getByText(/pause or cancel your campaign at any time/)).toBeInTheDocument()
    })

    it('has a Limitation of liability section', () => {
      renderTerms()
      expect(screen.getByText('Limitation of liability')).toBeInTheDocument()
      expect(screen.getByText(/provided as-is/)).toBeInTheDocument()
    })

    it('disclaims guarantees about specific results', () => {
      renderTerms()
      expect(screen.getByText(/no guarantees about specific savings/)).toBeInTheDocument()
    })

    it('has a Contact section', () => {
      renderTerms()
      expect(screen.getByText('Contact')).toBeInTheDocument()
    })
  })

  // ===== LINKS =====
  describe('contact links', () => {
    it('links to legal@thrift.ly email', () => {
      renderTerms()
      const legalLink = screen.getByText('legal@thrift.ly')
      expect(legalLink).toHaveAttribute('href', 'mailto:legal@thrift.ly')
    })
  })

  // ===== PAGE STRUCTURE =====
  describe('page structure', () => {
    it('wraps content in a legal-page section', () => {
      renderTerms()
      const section = document.querySelector('.legal-page')
      expect(section).toBeInTheDocument()
    })

    it('constrains content width with section-inner', () => {
      renderTerms()
      const inner = document.querySelector('.section-inner')
      expect(inner).toBeInTheDocument()
    })

    it('renders all six h2 sections', () => {
      renderTerms()
      const h2s = screen.getAllByRole('heading', { level: 2 })
      expect(h2s.length).toBe(6)
    })

    it('h2 sections appear in expected order', () => {
      renderTerms()
      const h2s = screen.getAllByRole('heading', { level: 2 })
      const headingTexts = h2s.map(h => h.textContent)
      expect(headingTexts).toEqual([
        'For shoppers',
        'For advertisers',
        'Advertising guidelines',
        'Payment terms',
        'Limitation of liability',
        'Contact',
      ])
    })
  })

  // ===== BRAND CONSISTENCY =====
  describe('brand consistency', () => {
    it('does not use corporate jargon in headings', () => {
      renderTerms()
      const headings = screen.getAllByRole('heading')
      headings.forEach(heading => {
        expect(heading.textContent).not.toMatch(/\bsynergy\b/i)
        expect(heading.textContent).not.toMatch(/\bleverage\b/i)
      })
    })

    it('uses plain language in legal text', () => {
      renderTerms()
      // The terms use accessible language, not legalese
      expect(screen.getByText(/free to use/)).toBeInTheDocument()
      expect(screen.getByText(/pause or cancel/)).toBeInTheDocument()
    })
  })

  // ===== ACCESSIBILITY =====
  describe('accessibility', () => {
    it('has exactly one h1 element', () => {
      renderTerms()
      const h1s = screen.getAllByRole('heading', { level: 1 })
      expect(h1s.length).toBe(1)
    })

    it('email link is a proper anchor element', () => {
      renderTerms()
      const link = screen.getByText('legal@thrift.ly')
      expect(link.tagName).toBe('A')
    })

    it('heading hierarchy is correct (h1 then h2s)', () => {
      renderTerms()
      const allHeadings = screen.getAllByRole('heading')
      expect(allHeadings[0].tagName).toBe('H1')
      allHeadings.slice(1).forEach(h => {
        expect(h.tagName).toBe('H2')
      })
    })
  })

  // ===== CONTENT FLOW =====
  describe('content flow', () => {
    it('each h2 section is followed by a paragraph', () => {
      renderTerms()
      const h2s = document.querySelectorAll('h2')
      h2s.forEach(h2 => {
        const nextSibling = h2.nextElementSibling
        expect(nextSibling?.tagName).toBe('P')
      })
    })

    it('terms page has only the brand logo', () => {
      renderTerms()
      const images = document.querySelectorAll('img')
      expect(images.length).toBe(1)
      expect(images[0].alt).toBe('thriftly')
    })

    it('terms page has no buttons', () => {
      renderTerms()
      const buttons = document.querySelectorAll('button')
      expect(buttons.length).toBe(0)
    })
  })

  // ===== TERMS-SPECIFIC CONTENT =====
  describe('terms-specific content', () => {
    it('mentions non-commercial use for shoppers', () => {
      renderTerms()
      expect(screen.getByText(/non-commercial grocery shopping/)).toBeInTheDocument()
    })

    it('mentions compliance with advertising laws', () => {
      renderTerms()
      expect(screen.getByText(/advertising laws and regulations/)).toBeInTheDocument()
    })

    it('mentions products must be accurately described', () => {
      renderTerms()
      expect(screen.getByText(/accurately described/)).toBeInTheDocument()
    })

    it('mentions CPM or CPC billing models', () => {
      renderTerms()
      expect(screen.getByText(/CPM or CPC/)).toBeInTheDocument()
    })

    it('includes as-is disclaimer', () => {
      renderTerms()
      expect(screen.getByText(/provided as-is/)).toBeInTheDocument()
    })
  })

  // ===== SECTION CONTENT DETAILS =====
  describe('section content details', () => {
    it('For shoppers mentions clearly labeled product suggestions', () => {
      renderTerms()
      expect(screen.getByText(/clearly labeled product suggestions/)).toBeInTheDocument()
    })

    it('For advertisers mentions truthful product information', () => {
      renderTerms()
      expect(screen.getByText(/truthful product information/)).toBeInTheDocument()
    })

    it('Advertising guidelines prohibits harmful products', () => {
      renderTerms()
      expect(screen.getByText(/harmful, misleading, or inappropriate/)).toBeInTheDocument()
    })

    it('Payment terms mentions campaign flexibility', () => {
      renderTerms()
      expect(screen.getByText(/pause or cancel your campaign at any time/)).toBeInTheDocument()
    })

    it('Limitation of liability mentions no specific guarantees', () => {
      renderTerms()
      expect(screen.getByText(/no guarantees about specific savings/)).toBeInTheDocument()
    })

    it('Contact section provides legal email', () => {
      renderTerms()
      const contactSection = screen.getByText('Contact')
      const contactParagraph = contactSection.nextElementSibling
      expect(contactParagraph?.textContent).toContain('legal@thrift.ly')
    })
  })

  // ===== EMAIL LINK DETAILS =====
  describe('email link details', () => {
    it('legal email link uses mailto protocol', () => {
      renderTerms()
      const link = screen.getByText('legal@thrift.ly')
      expect(link.getAttribute('href')).toMatch(/^mailto:/)
    })

    it('email link is inline within paragraph text', () => {
      renderTerms()
      const link = screen.getByText('legal@thrift.ly')
      expect(link.parentElement?.tagName).toBe('P')
    })

    it('exactly one legal email link is rendered', () => {
      renderTerms()
      const links = document.querySelectorAll('a[href="mailto:legal@thrift.ly"]')
      expect(links.length).toBe(1)
    })
  })
})
