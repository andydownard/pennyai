import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect } from 'vitest'
import Privacy from '../pages/Privacy'

function renderPrivacy() {
  return render(
    <MemoryRouter>
      <Privacy />
    </MemoryRouter>
  )
}

describe('Privacy page', () => {
  // ===== RENDERING =====
  describe('renders without errors', () => {
    it('renders the page without crashing', () => {
      renderPrivacy()
      expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
    })

    it('sets the document title to include Privacy Policy', () => {
      renderPrivacy()
      expect(document.title).toContain('Privacy Policy')
    })
  })

  // ===== MAIN HEADING =====
  describe('page header', () => {
    it('displays the Privacy Policy heading', () => {
      renderPrivacy()
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Privacy Policy')
    })

    it('displays the last updated date', () => {
      renderPrivacy()
      expect(screen.getByText('Last updated: March 2026')).toBeInTheDocument()
    })

    it('uses the legal-updated class for the date', () => {
      renderPrivacy()
      const updated = document.querySelector('.legal-updated')
      expect(updated).toBeInTheDocument()
      expect(updated?.textContent).toContain('March 2026')
    })
  })

  // ===== CONTENT SECTIONS =====
  describe('content sections', () => {
    it('has a What we collect section', () => {
      renderPrivacy()
      expect(screen.getByText('What we collect')).toBeInTheDocument()
      expect(screen.getByText(/shopping list items, brand preferences/)).toBeInTheDocument()
    })

    it('mentions company and payment info for advertisers', () => {
      renderPrivacy()
      expect(screen.getByText(/company and payment information/)).toBeInTheDocument()
    })

    it('has a How we use your data section', () => {
      renderPrivacy()
      expect(screen.getByText('How we use your data')).toBeInTheDocument()
      expect(screen.getByText(/building your lists, learning your preferences/)).toBeInTheDocument()
    })

    it('states we never sell individual shopping data', () => {
      renderPrivacy()
      expect(screen.getByText(/never sell your individual shopping data/)).toBeInTheDocument()
    })

    it('has a Brand advertising section', () => {
      renderPrivacy()
      expect(screen.getByText('Brand advertising')).toBeInTheDocument()
      expect(screen.getByText(/free for shoppers because brands pay/)).toBeInTheDocument()
    })

    it('mentions brand suggestions are clearly labeled', () => {
      renderPrivacy()
      expect(screen.getByText(/clearly labeled/)).toBeInTheDocument()
    })

    it('states brands only get anonymized, aggregated insights', () => {
      renderPrivacy()
      expect(screen.getByText(/anonymized, aggregated insights/)).toBeInTheDocument()
    })

    it('has a Data storage and security section', () => {
      renderPrivacy()
      expect(screen.getByText('Data storage and security')).toBeInTheDocument()
      expect(screen.getByText(/encrypted in transit and at rest/)).toBeInTheDocument()
    })

    it('has a Your rights section', () => {
      renderPrivacy()
      expect(screen.getByText('Your rights')).toBeInTheDocument()
      expect(screen.getByText(/request access to, correction of, or deletion/)).toBeInTheDocument()
    })

    it('mentions CCPA rights for California residents', () => {
      renderPrivacy()
      expect(screen.getByText(/California residents/)).toBeInTheDocument()
      expect(screen.getByText(/CCPA/)).toBeInTheDocument()
    })

    it('has a Contact section', () => {
      renderPrivacy()
      expect(screen.getByText('Contact')).toBeInTheDocument()
    })
  })

  // ===== LINKS =====
  describe('contact links', () => {
    it('links to privacy@thrift.ly email in Your rights section', () => {
      renderPrivacy()
      const privacyLinks = screen.getAllByText('privacy@thrift.ly')
      expect(privacyLinks.length).toBeGreaterThanOrEqual(1)
      privacyLinks.forEach(link => {
        expect(link).toHaveAttribute('href', 'mailto:privacy@thrift.ly')
      })
    })

    it('links to privacy@thrift.ly email in Contact section', () => {
      renderPrivacy()
      const privacyLinks = screen.getAllByText('privacy@thrift.ly')
      expect(privacyLinks.length).toBe(2)
    })
  })

  // ===== PAGE STRUCTURE =====
  describe('page structure', () => {
    it('wraps content in a legal-page section', () => {
      renderPrivacy()
      const section = document.querySelector('.legal-page')
      expect(section).toBeInTheDocument()
    })

    it('constrains content width with section-inner', () => {
      renderPrivacy()
      const inner = document.querySelector('.section-inner')
      expect(inner).toBeInTheDocument()
    })

    it('renders all six h2 sections', () => {
      renderPrivacy()
      const h2s = screen.getAllByRole('heading', { level: 2 })
      expect(h2s.length).toBe(6)
    })

    it('h2 sections appear in expected order', () => {
      renderPrivacy()
      const h2s = screen.getAllByRole('heading', { level: 2 })
      const headingTexts = h2s.map(h => h.textContent)
      expect(headingTexts).toEqual([
        'What we collect',
        'How we use your data',
        'Brand advertising',
        'Data storage and security',
        'Your rights',
        'Contact',
      ])
    })
  })

  // ===== BRAND CONSISTENCY =====
  describe('brand consistency', () => {
    it('does not use corporate jargon in headings', () => {
      renderPrivacy()
      const headings = screen.getAllByRole('heading')
      headings.forEach(heading => {
        expect(heading.textContent).not.toMatch(/\bsynergy\b/i)
        expect(heading.textContent).not.toMatch(/\bleverage\b/i)
      })
    })

    it('uses plain, transparent language about data practices', () => {
      renderPrivacy()
      // The privacy page uses straightforward language
      expect(screen.getAllByText(/never sell your individual shopping data/).length).toBeGreaterThan(0)
    })
  })

  // ===== ACCESSIBILITY =====
  describe('accessibility', () => {
    it('has exactly one h1 element', () => {
      renderPrivacy()
      const h1s = screen.getAllByRole('heading', { level: 1 })
      expect(h1s.length).toBe(1)
    })

    it('email links are proper anchor elements', () => {
      renderPrivacy()
      const links = screen.getAllByText('privacy@thrift.ly')
      links.forEach(link => {
        expect(link.tagName).toBe('A')
      })
    })

    it('heading hierarchy is correct (h1 then h2s)', () => {
      renderPrivacy()
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
      renderPrivacy()
      const h2s = document.querySelectorAll('h2')
      h2s.forEach(h2 => {
        const nextSibling = h2.nextElementSibling
        expect(nextSibling?.tagName).toBe('P')
      })
    })

    it('privacy page has only the brand logo', () => {
      renderPrivacy()
      const images = document.querySelectorAll('img')
      expect(images.length).toBe(1)
      expect(images[0].alt).toBe('thriftly')
    })

    it('privacy page has no buttons (read-only content)', () => {
      renderPrivacy()
      const buttons = document.querySelectorAll('button')
      expect(buttons.length).toBe(0)
    })
  })

  // ===== PRIVACY-SPECIFIC CONTENT =====
  describe('privacy-specific content', () => {
    it('mentions how data is used for the Thriftly experience', () => {
      renderPrivacy()
      expect(screen.getByText(/power the Thriftly experience/)).toBeInTheDocument()
    })

    it('mentions industry-standard security practices', () => {
      renderPrivacy()
      expect(screen.getByText(/industry-standard practices/)).toBeInTheDocument()
    })

    it('mentions right to request data deletion', () => {
      renderPrivacy()
      expect(screen.getByText(/deletion of your personal data/)).toBeInTheDocument()
    })
  })

  // ===== SECTION CONTENT DETAILS =====
  describe('section content details', () => {
    it('What we collect describes shopper data items', () => {
      renderPrivacy()
      expect(screen.getByText(/shopping list items/)).toBeInTheDocument()
      expect(screen.getByText(/brand preferences/)).toBeInTheDocument()
    })

    it('How we use your data mentions lists and preferences', () => {
      renderPrivacy()
      expect(screen.getByText(/building your lists/)).toBeInTheDocument()
      expect(screen.getByText(/learning your preferences/)).toBeInTheDocument()
    })

    it('Brand advertising explains the business model', () => {
      renderPrivacy()
      expect(screen.getByText(/free for shoppers because brands pay/)).toBeInTheDocument()
    })

    it('Your rights section mentions multiple rights', () => {
      renderPrivacy()
      expect(screen.getByText(/request access to, correction of, or deletion/)).toBeInTheDocument()
    })

    it('Contact section provides clear contact method', () => {
      renderPrivacy()
      const contactSection = screen.getByText('Contact')
      const contactParagraph = contactSection.nextElementSibling
      expect(contactParagraph?.textContent).toContain('privacy@thrift.ly')
    })
  })

  // ===== EMAIL LINK DETAILS =====
  describe('email link details', () => {
    it('all privacy email links use mailto protocol', () => {
      renderPrivacy()
      const links = screen.getAllByText('privacy@thrift.ly')
      links.forEach(link => {
        expect(link.getAttribute('href')).toMatch(/^mailto:/)
      })
    })

    it('email links are inline within paragraph text', () => {
      renderPrivacy()
      const links = screen.getAllByText('privacy@thrift.ly')
      links.forEach(link => {
        expect(link.parentElement?.tagName).toBe('P')
      })
    })
  })
})
