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

  describe('page header', () => {
    it('displays the Privacy Policy heading', () => {
      renderPrivacy()
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Privacy Policy')
    })

    it('displays the effective date', () => {
      renderPrivacy()
      expect(screen.getByText(/Effective:/)).toBeInTheDocument()
    })
  })

  describe('content sections', () => {
    it('discloses the Rally ecosystem in section 1', () => {
      renderPrivacy()
      expect(screen.getByText(/Account and the Rally ecosystem/)).toBeInTheDocument()
      expect(screen.getByText(/Your account is shared\. Your shopping list is not\./)).toBeInTheDocument()
    })

    it('discloses SMS messaging consent and frequency control', () => {
      renderPrivacy()
      expect(
        screen.getByRole('heading', { level: 2, name: /SMS messages and your consent/ })
      ).toBeInTheDocument()
      expect(screen.getByText(/Message frequency varies/)).toBeInTheDocument()
      expect(screen.getByText(/change how often Penny texts you in Settings/i)).toBeInTheDocument()
    })

    it('includes the Twilio-required mobile information disclosure verbatim', () => {
      renderPrivacy()
      expect(screen.getByText(/We will not share your mobile information with third parties or affiliates for marketing or promotional purposes/)).toBeInTheDocument()
    })

    it('explains STOP and HELP keywords', () => {
      renderPrivacy()
      expect(screen.getAllByText(/STOP/).length).toBeGreaterThan(0)
      expect(screen.getAllByText(/HELP/).length).toBeGreaterThan(0)
    })

    it('lists information collected', () => {
      renderPrivacy()
      expect(screen.getByText(/Information we collect/)).toBeInTheDocument()
      expect(screen.getByText(/Phone number/)).toBeInTheDocument()
      expect(screen.getByText(/Shopping list data/)).toBeInTheDocument()
      expect(screen.getByText(/Brand preferences and dietary attributes/)).toBeInTheDocument()
    })

    it('names service providers explicitly', () => {
      renderPrivacy()
      expect(
        screen.getByRole('heading', { level: 2, name: /Service providers/ })
      ).toBeInTheDocument()
      expect(screen.getByText('Twilio')).toBeInTheDocument()
      expect(screen.getByText('Anthropic')).toBeInTheDocument()
      expect(screen.getAllByText(/Stripe/).length).toBeGreaterThan(0)
    })

    it('has a brand partners and rebates section', () => {
      renderPrivacy()
      expect(
        screen.getByRole('heading', { level: 2, name: /Brand partners and rebates/ })
      ).toBeInTheDocument()
    })

    it('has a data retention section with deletion path', () => {
      renderPrivacy()
      expect(
        screen.getByRole('heading', { level: 2, name: /Data retention/ })
      ).toBeInTheDocument()
      expect(screen.getAllByText(/within 30 days/).length).toBeGreaterThan(0)
    })

    it('has a security section', () => {
      renderPrivacy()
      expect(
        screen.getByRole('heading', { level: 2, name: /Security/ })
      ).toBeInTheDocument()
    })

    it('has a Your rights section with CCPA mention', () => {
      renderPrivacy()
      expect(
        screen.getByRole('heading', { level: 2, name: /Your rights/ })
      ).toBeInTheDocument()
      expect(screen.getByText(/California residents/)).toBeInTheDocument()
      expect(screen.getByText(/CCPA/)).toBeInTheDocument()
    })

    it('has a Children section', () => {
      renderPrivacy()
      expect(
        screen.getByRole('heading', { level: 2, name: /Children/ })
      ).toBeInTheDocument()
    })

    it('has a Contact section', () => {
      renderPrivacy()
      expect(
        screen.getByRole('heading', { level: 2, name: /^\d+\.\s*Contact$/ })
      ).toBeInTheDocument()
    })
  })

  describe('contact links', () => {
    it('uses privacy@groflo.io for the contact email', () => {
      renderPrivacy()
      const links = screen.getAllByText('privacy@groflo.io')
      expect(links.length).toBeGreaterThanOrEqual(1)
      links.forEach(link => {
        expect(link).toHaveAttribute('href', 'mailto:privacy@groflo.io')
      })
    })
  })

  describe('page structure', () => {
    it('wraps content in a legal-page section', () => {
      renderPrivacy()
      const section = document.querySelector('.legal-page')
      expect(section).toBeInTheDocument()
    })

    it('renders all twelve numbered h2 sections', () => {
      renderPrivacy()
      const h2s = screen.getAllByRole('heading', { level: 2 })
      expect(h2s.length).toBe(12)
    })
  })

  describe('brand consistency', () => {
    it('refers to the company as Groflo, Inc.', () => {
      renderPrivacy()
      expect(screen.getByText(/Groflo, Inc\./)).toBeInTheDocument()
    })

    it('does not contain stale Thriftly references', () => {
      renderPrivacy()
      const body = document.body.textContent || ''
      expect(body).not.toMatch(/Thriftly/i)
      expect(body).not.toMatch(/thrift\.ly/)
    })

    it('uses the Groflo logo alt text', () => {
      renderPrivacy()
      const images = document.querySelectorAll('img')
      expect(images.length).toBe(1)
      expect(images[0].alt).toBe('Groflo')
    })
  })

  describe('accessibility', () => {
    it('has exactly one h1 element', () => {
      renderPrivacy()
      const h1s = screen.getAllByRole('heading', { level: 1 })
      expect(h1s.length).toBe(1)
    })

    it('email links are anchor elements', () => {
      renderPrivacy()
      const links = screen.getAllByText('privacy@groflo.io')
      links.forEach(link => {
        expect(link.tagName).toBe('A')
      })
    })

    it('heading hierarchy is h1 then h2s', () => {
      renderPrivacy()
      const allHeadings = screen.getAllByRole('heading')
      expect(allHeadings[0].tagName).toBe('H1')
      allHeadings.slice(1).forEach(h => {
        expect(h.tagName).toBe('H2')
      })
    })
  })
})
