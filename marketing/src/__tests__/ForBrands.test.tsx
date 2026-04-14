import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect } from 'vitest'
import ForBrands from '../pages/ForBrands'

function renderForBrands() {
  return render(
    <MemoryRouter>
      <ForBrands />
    </MemoryRouter>
  )
}

describe('ForBrands page', () => {
  // ===== RENDERING =====
  describe('renders without errors', () => {
    it('renders the page without crashing', () => {
      renderForBrands()
      expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
    })

    it('sets the document title to include For Brands', () => {
      renderForBrands()
      expect(document.title).toContain('For Brands')
    })
  })

  // ===== HERO SECTION =====
  describe('hero section', () => {
    it('displays the section label', () => {
      renderForBrands()
      expect(screen.getByText('Groflo for Brands')).toBeInTheDocument()
    })

    it('displays the hero headline about free deals', () => {
      renderForBrands()
      const h1 = screen.getByRole('heading', { level: 1 })
      expect(h1).toHaveTextContent(/Put your product in their cart/)
    })

    it('displays the hero subtitle about free trials', () => {
      renderForBrands()
      expect(
        screen.getByText(/free trials that get your product into their hands/)
      ).toBeInTheDocument()
    })

    it('has a CTA link to campaign signup', () => {
      renderForBrands()
      const ctaLinks = screen.getAllByText(/Create a Free Deal/)
      expect(ctaLinks.length).toBeGreaterThanOrEqual(1)
      const heroLink = ctaLinks[0]
      expect(heroLink).toHaveAttribute('href', '/campaign/signup')
    })

    it('has a secondary link to pricing page', () => {
      renderForBrands()
      const pricingLinks = screen.getAllByText('See Pricing')
      expect(pricingLinks.length).toBeGreaterThanOrEqual(1)
      expect(pricingLinks[0]).toHaveAttribute('href', '/pricing')
    })

    it('does not use jargon in hero headline', () => {
      renderForBrands()
      const h1 = screen.getByRole('heading', { level: 1 })
      expect(h1.textContent).not.toMatch(/\bsynergize\b/i)
      expect(h1.textContent).not.toMatch(/\borchestrate\b/i)
      expect(h1.textContent).not.toMatch(/\bleverage\b/i)
    })

    it('displays hero stats about in-store market', () => {
      renderForBrands()
      expect(screen.getAllByText('85%').length).toBeGreaterThanOrEqual(1)
      expect(screen.getByText(/of grocery shopping is still in-store/)).toBeInTheDocument()
    })
  })

  // ===== HOW SWAPS WORK SECTION =====
  describe('How It Works section', () => {
    it('displays the How It Works heading', () => {
      renderForBrands()
      expect(screen.getByText('How It Works')).toBeInTheDocument()
    })

    it('displays the section title about swaps', () => {
      renderForBrands()
      expect(screen.getByText(/Your product\. Their list\. Zero friction\./)).toBeInTheDocument()
    })

    it('mentions Penny matching deals', () => {
      renderForBrands()
      expect(screen.getByText(/Penny matches your deal to shoppers/)).toBeInTheDocument()
    })

    it('renders step 1: Create your deal', () => {
      renderForBrands()
      expect(screen.getByText('Create your deal')).toBeInTheDocument()
      expect(screen.getByText(/free trial, a targeted swap, or a discount/)).toBeInTheDocument()
    })

    it('renders step 2: Penny matches it', () => {
      renderForBrands()
      expect(screen.getByText('Penny matches it')).toBeInTheDocument()
      expect(screen.getAllByText(/adds a competitor's product/).length).toBeGreaterThanOrEqual(1)
    })

    it('renders step 3: They try your product', () => {
      renderForBrands()
      expect(screen.getByText('They try your product')).toBeInTheDocument()
      expect(screen.getByText(/you pay only for the trial/)).toBeInTheDocument()
    })

    it('renders all three step numbers', () => {
      renderForBrands()
      const stepNumbers = document.querySelectorAll('.step-number')
      expect(stepNumbers.length).toBe(3)
      expect(stepNumbers[0]).toHaveTextContent('1')
      expect(stepNumbers[1]).toHaveTextContent('2')
      expect(stepNumbers[2]).toHaveTextContent('3')
    })
  })

  // ===== CAMPAIGN TYPES SECTION =====
  describe('Campaign Types section', () => {
    it('displays the Campaign Types label', () => {
      renderForBrands()
      expect(screen.getByText('Campaign Types')).toBeInTheDocument()
    })

    it('displays the section title about three ways to get into the cart', () => {
      renderForBrands()
      expect(screen.getByText('Three ways to get into the cart')).toBeInTheDocument()
    })

    it('renders the Free Product Trials card', () => {
      renderForBrands()
      expect(screen.getByText('Free Product Trials')).toBeInTheDocument()
      expect(screen.getByText(/Cover the full cost of your product/)).toBeInTheDocument()
      expect(screen.getByText('Best for emerging brands')).toBeInTheDocument()
    })

    it('renders the Targeted Swaps card', () => {
      renderForBrands()
      expect(screen.getByText('Targeted Swaps')).toBeInTheDocument()
      expect(screen.getByText(/Offer a discount to switch from a competitor/)).toBeInTheDocument()
      expect(screen.getByText('Best for category challengers')).toBeInTheDocument()
    })

    it('renders the Promoted Suggestions card', () => {
      renderForBrands()
      expect(screen.getByText('Promoted Suggestions')).toBeInTheDocument()
      expect(screen.getByText(/Penny mentions your product by name/)).toBeInTheDocument()
      expect(screen.getByText('Best for brand awareness')).toBeInTheDocument()
    })

    it('renders three format cards total', () => {
      renderForBrands()
      const formatCards = document.querySelectorAll('.format-card')
      expect(formatCards.length).toBe(3)
    })

    it('highlights Free Product Trials as featured', () => {
      renderForBrands()
      const featured = document.querySelector('.format-card-featured')
      expect(featured).toBeInTheDocument()
    })
  })

  // ===== BENEFITS SECTION =====
  describe('Benefits / Why Groflo section', () => {
    it('displays the Why Groflo label', () => {
      renderForBrands()
      expect(screen.getByText('Why Groflo')).toBeInTheDocument()
    })

    it('displays the benefits headline about in-store channel', () => {
      renderForBrands()
      expect(
        screen.getByText(/The in-store channel that doesn't exist yet/)
      ).toBeInTheDocument()
    })

    it('displays the subtitle about Instacart comparison', () => {
      renderForBrands()
      expect(
        screen.getByText(/Instacart proved data-driven targeting works/)
      ).toBeInTheDocument()
    })

    it('renders the moment-of-intent benefit', () => {
      renderForBrands()
      expect(screen.getByText('Moment-of-intent targeting')).toBeInTheDocument()
      expect(
        screen.getAllByText(/adds a competitor's product/).length
      ).toBeGreaterThanOrEqual(1)
    })

    it('renders the pay-for-trials benefit', () => {
      renderForBrands()
      expect(screen.getByText('Pay for trials, not impressions')).toBeInTheDocument()
    })

    it('renders the swap analytics benefit', () => {
      renderForBrands()
      expect(screen.getByText('Real-time swap analytics')).toBeInTheDocument()
      expect(
        screen.getByText(/swap acceptance rates, repeat purchases/)
      ).toBeInTheDocument()
    })

    it('renders the no shelf space benefit', () => {
      renderForBrands()
      expect(screen.getByText('No shelf-space required')).toBeInTheDocument()
      expect(screen.getByText(/skip the shelf and go straight to the shopping list/)).toBeInTheDocument()
    })

    it('renders four benefit items total', () => {
      renderForBrands()
      const benefitItems = document.querySelectorAll('.benefit-item')
      expect(benefitItems.length).toBe(4)
    })
  })

  // ===== TAM SECTION =====
  describe('TAM / Opportunity section', () => {
    it('displays the opportunity headline', () => {
      renderForBrands()
      expect(screen.getByText(/The in-store market is 7x larger than delivery/)).toBeInTheDocument()
    })

    it('shows the Instacart comparison numbers', () => {
      renderForBrands()
      expect(screen.getByText('$1.2B')).toBeInTheDocument()
      expect(screen.getByText('$8.4B+')).toBeInTheDocument()
    })

    it('labels delivery vs in-store', () => {
      renderForBrands()
      expect(screen.getByText(/Delivery ads \(Instacart\)/)).toBeInTheDocument()
      expect(screen.getByText(/In-store opportunity \(Penny\)/)).toBeInTheDocument()
    })
  })

  // ===== EARLY ACCESS SECTION =====
  describe('Early Access section', () => {
    it('displays the Early Access label', () => {
      renderForBrands()
      expect(screen.getByText('Early Access')).toBeInTheDocument()
    })

    it('displays the emerging brands headline', () => {
      renderForBrands()
      expect(screen.getByText('Ideal for emerging brands')).toBeInTheDocument()
    })

    it('mentions slotting fees barrier', () => {
      renderForBrands()
      expect(
        screen.getByText(/Can't afford slotting fees/)
      ).toBeInTheDocument()
    })

    it('renders the free trial campaigns perk', () => {
      renderForBrands()
      expect(screen.getByText('Free trial campaigns')).toBeInTheDocument()
    })

    it('renders the no minimums perk', () => {
      renderForBrands()
      expect(screen.getByText('No minimums to start')).toBeInTheDocument()
    })

    it('renders the direct-to-list distribution perk', () => {
      renderForBrands()
      expect(screen.getByText('Direct-to-list distribution')).toBeInTheDocument()
    })

    it('renders three early access perks total', () => {
      renderForBrands()
      const perks = document.querySelectorAll('.early-access-perk')
      expect(perks.length).toBe(3)
    })
  })

  // ===== CTA SECTION =====
  describe('CTA section', () => {
    it('displays the CTA headline', () => {
      renderForBrands()
      expect(screen.getByText('Get your product on their list')).toBeInTheDocument()
    })

    it('displays the CTA body text', () => {
      renderForBrands()
      expect(
        screen.getByText(/Create a free deal or targeted swap today/)
      ).toBeInTheDocument()
    })

    it('has a primary CTA linking to campaign signup', () => {
      renderForBrands()
      const ctaLinks = screen.getAllByText(/Create a Free Deal/)
      const ctaLink = ctaLinks[ctaLinks.length - 1]
      expect(ctaLink).toHaveAttribute('href', '/campaign/signup')
    })

    it('has a secondary CTA linking to pricing', () => {
      renderForBrands()
      const pricingLinks = screen.getAllByText('See Pricing')
      const ctaLink = pricingLinks[pricingLinks.length - 1]
      expect(ctaLink).toHaveAttribute('href', '/pricing')
    })
  })

  // ===== BRAND CONSISTENCY =====
  describe('brand consistency', () => {
    it('uses brand green (#35644F) in SVG icons', () => {
      renderForBrands()
      const svgPaths = document.querySelectorAll('svg path[fill="#35644F"]')
      expect(svgPaths.length).toBeGreaterThan(0)
    })

    it('uses section-label CSS class consistently', () => {
      renderForBrands()
      const labels = document.querySelectorAll('.section-label')
      expect(labels.length).toBeGreaterThan(0)
    })

    it('does not contain corporate jargon in headings', () => {
      renderForBrands()
      const headings = screen.getAllByRole('heading')
      const jargonTerms = [
        /\bsynergy\b/i, /\bparadigm\b/i, /\bdisrupt\b/i,
        /\blocomotion\b/i, /\bholistic\b/i, /\bscalable\b/i,
      ]
      headings.forEach(heading => {
        jargonTerms.forEach(term => {
          expect(heading.textContent).not.toMatch(term)
        })
      })
    })

    it('uses warm, clear language in copy', () => {
      renderForBrands()
      expect(screen.getAllByText(/\$25\/day/).length).toBeGreaterThanOrEqual(1)
    })
  })

  // ===== FADE ANIMATIONS =====
  describe('fade-in animation sections', () => {
    it('applies fade-section class to How It Works section', () => {
      renderForBrands()
      const section = document.querySelector('.brands-how-section')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })

    it('applies fade-section class to Campaign Types section', () => {
      renderForBrands()
      const section = document.querySelector('.brands-formats')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })

    it('applies fade-section class to Benefits section', () => {
      renderForBrands()
      const section = document.querySelector('.brands-benefits-section')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })

    it('applies fade-section class to Early Access section', () => {
      renderForBrands()
      const section = document.querySelector('.brands-early-access')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })

    it('applies fade-section class to CTA section', () => {
      renderForBrands()
      const section = document.querySelector('.brands-cta-section')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })
  })

  // ===== LINK TARGETS =====
  describe('navigation links', () => {
    it('all campaign signup links point to /campaign/signup', () => {
      renderForBrands()
      const links = screen.getAllByText(/Create a Free Deal/)
      links.forEach(link => {
        expect(link).toHaveAttribute('href', '/campaign/signup')
      })
    })

    it('all pricing links point to /pricing', () => {
      renderForBrands()
      const links = screen.getAllByText('See Pricing')
      links.forEach(link => {
        expect(link).toHaveAttribute('href', '/pricing')
      })
    })
  })

  // ===== STRUCTURE =====
  describe('page structure', () => {
    it('renders multiple sections', () => {
      renderForBrands()
      const sections = document.querySelectorAll('section')
      expect(sections.length).toBeGreaterThanOrEqual(5)
    })

    it('contains a hero section with brands-hero class', () => {
      renderForBrands()
      expect(document.querySelector('.brands-hero')).toBeInTheDocument()
    })

    it('wraps content in section-inner containers', () => {
      renderForBrands()
      const innerContainers = document.querySelectorAll('.section-inner')
      expect(innerContainers.length).toBeGreaterThanOrEqual(4)
    })

    it('renders exactly 12 sections', () => {
      renderForBrands()
      const sections = document.querySelectorAll('section')
      expect(sections.length).toBe(12)
    })
  })

  // ===== ACCESSIBILITY =====
  describe('accessibility', () => {
    it('all SVG icons are marked aria-hidden (except flywheel arrows)', () => {
      renderForBrands()
      const svgs = document.querySelectorAll('svg')
      svgs.forEach(svg => {
        const isFlywheelArrow = svg.closest('.flywheel-arrow[aria-hidden="true"]')
        if (!isFlywheelArrow) {
          expect(svg.getAttribute('aria-hidden')).toBe('true')
        }
      })
    })

    it('hero has exactly one h1 element', () => {
      renderForBrands()
      const h1s = screen.getAllByRole('heading', { level: 1 })
      expect(h1s.length).toBe(1)
    })

    it('all format cards have h3 headings', () => {
      renderForBrands()
      const formatCards = document.querySelectorAll('.format-card h3')
      expect(formatCards.length).toBe(3)
    })

    it('all benefit items have h4 headings', () => {
      renderForBrands()
      const benefitHeadings = document.querySelectorAll('.benefit-item h4')
      expect(benefitHeadings.length).toBe(4)
    })

    it('CTA buttons use semantic link elements', () => {
      renderForBrands()
      const ctaLinks = screen.getAllByText(/Create a Free Deal/)
      ctaLinks.forEach(link => {
        expect(link.tagName).toBe('A')
      })
    })
  })

  // ===== SECTION ORDERING =====
  describe('section ordering', () => {
    it('hero comes before how it works', () => {
      renderForBrands()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const heroIdx = classNames.indexOf('brands-hero')
      const howIdx = classNames.findIndex(c => c === 'brands-how-section')
      expect(heroIdx).toBeLessThan(howIdx)
    })

    it('how it works comes before campaign types', () => {
      renderForBrands()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const howIdx = classNames.findIndex(c => c === 'brands-how-section')
      const formatsIdx = classNames.findIndex(c => c === 'brands-formats')
      expect(howIdx).toBeLessThan(formatsIdx)
    })

    it('campaign types comes before benefits', () => {
      renderForBrands()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const formatsIdx = classNames.findIndex(c => c === 'brands-formats')
      const benefitsIdx = classNames.findIndex(c => c === 'brands-benefits-section')
      expect(formatsIdx).toBeLessThan(benefitsIdx)
    })

    it('benefits comes before TAM section', () => {
      renderForBrands()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const benefitsIdx = classNames.findIndex(c => c === 'brands-benefits-section')
      const tamIdx = classNames.findIndex(c => c === 'brands-tam')
      expect(benefitsIdx).toBeLessThan(tamIdx)
    })

    it('TAM comes before early access', () => {
      renderForBrands()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const tamIdx = classNames.findIndex(c => c === 'brands-tam')
      const earlyIdx = classNames.findIndex(c => c === 'brands-early-access')
      expect(tamIdx).toBeLessThan(earlyIdx)
    })

    it('early access comes before final CTA', () => {
      renderForBrands()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const earlyIdx = classNames.findIndex(c => c === 'brands-early-access')
      const ctaIdx = classNames.findIndex(c => c === 'brands-cta-section')
      expect(earlyIdx).toBeLessThan(ctaIdx)
    })
  })

  // ===== CONTENT COMPLETENESS =====
  describe('content completeness', () => {
    it('mentions starting budget of $25/day', () => {
      renderForBrands()
      expect(screen.getAllByText(/\$25\/day/).length).toBeGreaterThanOrEqual(1)
    })

    it('format tags use format-tag class', () => {
      renderForBrands()
      const tags = document.querySelectorAll('.format-tag')
      expect(tags.length).toBe(3)
    })

    it('benefits section has a dark background (brands-benefits-section)', () => {
      renderForBrands()
      const section = document.querySelector('.brands-benefits-section')
      expect(section).toBeInTheDocument()
    })

    it('benefit icons use white fill color', () => {
      renderForBrands()
      const benefitIcons = document.querySelectorAll('.benefit-icon svg path[fill="white"]')
      expect(benefitIcons.length).toBeGreaterThan(0)
    })

    it('CTA actions are wrapped in cta-actions div', () => {
      renderForBrands()
      const ctaActions = document.querySelectorAll('.cta-actions')
      expect(ctaActions.length).toBeGreaterThanOrEqual(1)
    })

    it('highlights free deals as the primary value prop', () => {
      renderForBrands()
      expect(screen.getByText('Free Product Trials')).toBeInTheDocument()
      expect(screen.getByText('Targeted Swaps')).toBeInTheDocument()
    })
  })
})
