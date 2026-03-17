import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect } from 'vitest'
import Footer from '../components/Footer'

function renderFooter() {
  return render(
    <MemoryRouter>
      <Footer />
    </MemoryRouter>
  )
}

describe('Footer', () => {
  // ===== RENDERING =====
  describe('renders without errors', () => {
    it('renders the footer element', () => {
      renderFooter()
      const footer = document.querySelector('footer')
      expect(footer).toBeInTheDocument()
    })

    it('footer has the footer class', () => {
      renderFooter()
      const footer = document.querySelector('footer')
      expect(footer?.classList.contains('footer')).toBe(true)
    })
  })

  // ===== BRAND ELEMENTS =====
  describe('brand elements', () => {
    it('displays the brand tagline', () => {
      renderFooter()
      expect(
        screen.getByText(/The AI shopping list for families/)
      ).toBeInTheDocument()
    })

    it('displays the one-liner', () => {
      renderFooter()
      expect(
        screen.getByText(/she handles the rest/)
      ).toBeInTheDocument()
    })

    it('renders the white logo image', () => {
      renderFooter()
      const logo = document.querySelector('.logo-img') as HTMLImageElement
      expect(logo).toBeInTheDocument()
      expect(logo.src).toContain('logo-white.svg')
    })

    it('logo has alt text', () => {
      renderFooter()
      const logo = document.querySelector('.logo-img') as HTMLImageElement
      expect(logo.alt).toBe('thriftly')
    })

    it('displays copyright with current year', () => {
      renderFooter()
      const year = new Date().getFullYear()
      expect(screen.getByText(new RegExp(`${year}`))).toBeInTheDocument()
    })

    it('copyright includes Thriftly, Inc.', () => {
      renderFooter()
      expect(screen.getByText(/Thriftly, Inc/)).toBeInTheDocument()
    })
  })

  // ===== PRODUCT COLUMN =====
  describe('Product column', () => {
    it('renders Product heading', () => {
      renderFooter()
      expect(screen.getByText('Product')).toBeInTheDocument()
    })

    it('links to Features section', () => {
      renderFooter()
      const link = document.querySelector('a[href="/#features"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('Features')
    })

    it('links to How It Works section', () => {
      renderFooter()
      const link = document.querySelector('a[href="/#how-it-works"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('How It Works')
    })

    it('links to Meet Penny section', () => {
      renderFooter()
      const link = document.querySelector('a[href="/#penny"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('Meet Penny')
    })

    it('links to web app', () => {
      renderFooter()
      const link = document.querySelector('a[href="https://app.thrift.ly"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('Open Web App')
    })
  })

  // ===== COMPANY COLUMN =====
  describe('Company column', () => {
    it('renders Company heading', () => {
      renderFooter()
      expect(screen.getByText('Company')).toBeInTheDocument()
    })

    it('links to For Brands page', () => {
      renderFooter()
      const link = document.querySelector('a[href="/for-brands"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('For Brands')
    })

    it('links to Pricing page', () => {
      renderFooter()
      const link = document.querySelector('a[href="/pricing"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('Pricing')
    })

    it('links to About page', () => {
      renderFooter()
      const link = document.querySelector('a[href="/about"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('About')
    })

    it('links to Contact email', () => {
      renderFooter()
      const link = document.querySelector('a[href="mailto:hello@thrift.ly"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('Contact')
    })
  })

  // ===== SUPPORT COLUMN =====
  describe('Support column', () => {
    it('renders Support heading', () => {
      renderFooter()
      expect(screen.getAllByText('Support').length).toBeGreaterThan(0)
    })

    it('links to Email Support', () => {
      renderFooter()
      const link = document.querySelector('a[href="mailto:support@thrift.ly"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('Email Support')
    })

    it('links to Email Penny', () => {
      renderFooter()
      const link = document.querySelector('a[href="mailto:penny@list.thrift.ly"]')
      expect(link).toBeInTheDocument()
      expect(link?.textContent).toBe('Email Penny')
    })

    it('links to Contact Us', () => {
      renderFooter()
      const contactLinks = document.querySelectorAll('a[href="/about"]')
      const contactUsLink = Array.from(contactLinks).find(l => l.textContent === 'Contact Us')
      expect(contactUsLink).toBeInTheDocument()
    })
  })

  // ===== BOTTOM BAR =====
  describe('bottom bar', () => {
    it('links to privacy policy', () => {
      renderFooter()
      const bottomLinks = document.querySelector('.footer-bottom-links')
      const privacyLink = bottomLinks?.querySelector('a[href="/privacy"]')
      expect(privacyLink).toBeInTheDocument()
      expect(privacyLink?.textContent).toBe('Privacy')
    })

    it('links to terms', () => {
      renderFooter()
      const bottomLinks = document.querySelector('.footer-bottom-links')
      const termsLink = bottomLinks?.querySelector('a[href="/terms"]')
      expect(termsLink).toBeInTheDocument()
      expect(termsLink?.textContent).toBe('Terms')
    })

    it('renders footer-bottom section', () => {
      renderFooter()
      expect(document.querySelector('.footer-bottom')).toBeInTheDocument()
    })

    it('renders footer-bottom-links section', () => {
      renderFooter()
      expect(document.querySelector('.footer-bottom-links')).toBeInTheDocument()
    })
  })

  // ===== STRUCTURE =====
  describe('page structure', () => {
    it('has a footer-inner wrapper', () => {
      renderFooter()
      expect(document.querySelector('.footer-inner')).toBeInTheDocument()
    })

    it('has a footer-top section', () => {
      renderFooter()
      expect(document.querySelector('.footer-top')).toBeInTheDocument()
    })

    it('renders footer-brand section', () => {
      renderFooter()
      expect(document.querySelector('.footer-brand')).toBeInTheDocument()
    })

    it('renders four footer columns', () => {
      renderFooter()
      const cols = document.querySelectorAll('.footer-col')
      expect(cols.length).toBe(4)
    })

    it('each column has an h4 heading', () => {
      renderFooter()
      const colHeadings = document.querySelectorAll('.footer-col h4')
      expect(colHeadings.length).toBe(4)
    })
  })

  // ===== BRAND CONSISTENCY =====
  describe('brand consistency', () => {
    it('tagline matches brand source: "The AI shopping list for families"', () => {
      renderFooter()
      expect(screen.getByText(/The AI shopping list for families/)).toBeInTheDocument()
    })

    it('one-liner matches brand source: "she handles the rest"', () => {
      renderFooter()
      expect(screen.getByText(/she handles the rest/)).toBeInTheDocument()
    })

    it('does not use corporate jargon in footer text', () => {
      renderFooter()
      const allText = document.querySelector('footer')?.textContent || ''
      expect(allText).not.toMatch(/\bsynergy\b/i)
      expect(allText).not.toMatch(/\bleverage\b/i)
    })
  })
})
