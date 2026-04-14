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

  describe('page header', () => {
    it('displays the Terms of Service heading', () => {
      renderTerms()
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Terms of Service')
    })

    it('displays the effective date', () => {
      renderTerms()
      expect(screen.getByText(/Effective:/)).toBeInTheDocument()
    })
  })

  describe('content sections', () => {
    it('introduces Groflo and Penny in the lead paragraph', () => {
      renderTerms()
      expect(screen.getAllByText(/Groflo/).length).toBeGreaterThan(0)
      expect(screen.getAllByText(/Penny/).length).toBeGreaterThan(0)
    })

    it('has a service description section', () => {
      renderTerms()
      expect(
        screen.getByRole('heading', { level: 2, name: /The service/ })
      ).toBeInTheDocument()
    })

    it('discloses the Rally ecosystem account model', () => {
      renderTerms()
      expect(screen.getByText(/Your account and the Rally ecosystem/)).toBeInTheDocument()
    })

    it('has an SMS messaging program section with Twilio-required disclosures', () => {
      renderTerms()
      expect(screen.getByText(/SMS messaging program/)).toBeInTheDocument()
      expect(screen.getByText(/Message frequency varies/)).toBeInTheDocument()
      expect(screen.getByText(/Message and data rates may apply\./)).toBeInTheDocument()
      expect(screen.getByText(/STOP/)).toBeInTheDocument()
      expect(screen.getByText(/HELP/)).toBeInTheDocument()
    })

    it('promises frequency control in Settings', () => {
      renderTerms()
      expect(screen.getByText(/change how often Penny texts you/i)).toBeInTheDocument()
    })

    it('has a Your content section', () => {
      renderTerms()
      expect(
        screen.getByRole('heading', { level: 2, name: /Your content/ })
      ).toBeInTheDocument()
    })

    it('has a brand suggestions and swap rebates section', () => {
      renderTerms()
      expect(screen.getByText(/Brand suggestions and swap rebates/)).toBeInTheDocument()
    })

    it('has an acceptable use section', () => {
      renderTerms()
      expect(
        screen.getByRole('heading', { level: 2, name: /Acceptable use/ })
      ).toBeInTheDocument()
    })

    it('has an eligibility section that names US mobile phones', () => {
      renderTerms()
      expect(
        screen.getByRole('heading', { level: 2, name: /Eligibility/ })
      ).toBeInTheDocument()
      expect(screen.getByText(/at least 13 years old/)).toBeInTheDocument()
      expect(screen.getByText(/US mobile phone/)).toBeInTheDocument()
    })

    it('has a termination section', () => {
      renderTerms()
      expect(
        screen.getByRole('heading', { level: 2, name: /Termination/ })
      ).toBeInTheDocument()
    })

    it('has Disclaimers and Limitation of liability sections', () => {
      renderTerms()
      expect(
        screen.getByRole('heading', { level: 2, name: /Disclaimers/ })
      ).toBeInTheDocument()
      expect(
        screen.getByRole('heading', { level: 2, name: /Limitation of liability/ })
      ).toBeInTheDocument()
    })

    it('names governing law as Delaware', () => {
      renderTerms()
      expect(screen.getByText(/Governing law and disputes/)).toBeInTheDocument()
      expect(screen.getByText(/Delaware/)).toBeInTheDocument()
    })

    it('has a Changes section', () => {
      renderTerms()
      expect(
        screen.getByRole('heading', { level: 2, name: /Changes/ })
      ).toBeInTheDocument()
    })

    it('has a Contact section', () => {
      renderTerms()
      expect(
        screen.getByRole('heading', { level: 2, name: /^\d+\.\s*Contact$/ })
      ).toBeInTheDocument()
    })
  })

  describe('contact links', () => {
    it('uses legal@groflo.io for legal contact', () => {
      renderTerms()
      const link = screen.getByText('legal@groflo.io')
      expect(link).toHaveAttribute('href', 'mailto:legal@groflo.io')
    })

    it('uses support@groflo.io for support', () => {
      renderTerms()
      const links = screen.getAllByText('support@groflo.io')
      expect(links.length).toBeGreaterThanOrEqual(1)
    })

    it('uses privacy@groflo.io for account deletion', () => {
      renderTerms()
      const links = screen.getAllByText('privacy@groflo.io')
      expect(links.length).toBeGreaterThanOrEqual(1)
    })
  })

  describe('page structure', () => {
    it('wraps content in a legal-page section', () => {
      renderTerms()
      const section = document.querySelector('.legal-page')
      expect(section).toBeInTheDocument()
    })

    it('renders all thirteen numbered h2 sections', () => {
      renderTerms()
      const h2s = screen.getAllByRole('heading', { level: 2 })
      expect(h2s.length).toBe(13)
    })
  })

  describe('brand consistency', () => {
    it('does not contain stale Thriftly references', () => {
      renderTerms()
      const body = document.body.textContent || ''
      expect(body).not.toMatch(/Thriftly/i)
      expect(body).not.toMatch(/thrift\.ly/)
    })

    it('uses the Groflo logo alt text', () => {
      renderTerms()
      const images = document.querySelectorAll('img')
      expect(images.length).toBe(1)
      expect(images[0].alt).toBe('Groflo')
    })

    it('refers to the company as Groflo, Inc.', () => {
      renderTerms()
      expect(screen.getAllByText(/Groflo, Inc\./).length).toBeGreaterThan(0)
    })
  })

  describe('accessibility', () => {
    it('has exactly one h1 element', () => {
      renderTerms()
      const h1s = screen.getAllByRole('heading', { level: 1 })
      expect(h1s.length).toBe(1)
    })

    it('email links are anchor elements', () => {
      renderTerms()
      const link = screen.getByText('legal@groflo.io')
      expect(link.tagName).toBe('A')
    })

    it('heading hierarchy is h1 then h2s', () => {
      renderTerms()
      const allHeadings = screen.getAllByRole('heading')
      expect(allHeadings[0].tagName).toBe('H1')
      allHeadings.slice(1).forEach(h => {
        expect(h.tagName).toBe('H2')
      })
    })
  })
})
