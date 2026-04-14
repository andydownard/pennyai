import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect } from 'vitest'
import Pricing from '../pages/Pricing'

function renderPricing() {
  return render(
    <MemoryRouter>
      <Pricing />
    </MemoryRouter>
  )
}

describe('Pricing page', () => {
  // ===== RENDERING =====
  describe('renders without errors', () => {
    it('renders the page without crashing', () => {
      renderPricing()
      expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
    })

    it('sets the document title to include Pricing', () => {
      renderPricing()
      expect(document.title).toContain('Pricing')
    })
  })

  // ===== HERO SECTION =====
  describe('hero section', () => {
    it('displays the Pricing label', () => {
      renderPricing()
      expect(screen.getByText('Pricing')).toBeInTheDocument()
    })

    it('displays the hero headline', () => {
      renderPricing()
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
        'Pay for trials, not impressions'
      )
    })

    it('displays the subtitle mentioning free trials, targeted swaps, and $25/day', () => {
      renderPricing()
      expect(
        screen.getByText(/free trials, targeted swaps/)
      ).toBeInTheDocument()
      expect(
        screen.getByText(/Start with \$25\/day/)
      ).toBeInTheDocument()
    })

    it('does not use jargon in hero headline', () => {
      renderPricing()
      const h1 = screen.getByRole('heading', { level: 1 })
      expect(h1.textContent).not.toMatch(/\bsynergy\b/i)
      expect(h1.textContent).not.toMatch(/\borchestration\b/i)
    })
  })

  // ===== DEAL CAMPAIGNS =====
  describe('deal campaign cards', () => {
    it('renders the Free Trial card', () => {
      renderPricing()
      expect(screen.getByText('Free Trial')).toBeInTheDocument()
      expect(screen.getByText('Cost Per Accepted Trial')).toBeInTheDocument()
    })

    it('renders the Targeted Swap card', () => {
      renderPricing()
      expect(screen.getByText('Targeted Swap')).toBeInTheDocument()
      expect(screen.getByText('Cost Per Accepted Swap')).toBeInTheDocument()
    })

    it('shows Most Popular badge on Free Trial card', () => {
      renderPricing()
      expect(screen.getByText('Most Popular')).toBeInTheDocument()
    })

    it('shows Challenger badge on Targeted Swap card', () => {
      renderPricing()
      expect(screen.getByText('Challenger')).toBeInTheDocument()
    })

    it('applies featured styling to Free Trial card', () => {
      renderPricing()
      const featuredCard = document.querySelector('.pricing-card-featured')
      expect(featuredCard).toBeInTheDocument()
    })

    it('renders Free Trial features list', () => {
      renderPricing()
      expect(screen.getByText('Cover the full cost of your product for the shopper')).toBeInTheDocument()
      expect(screen.getByText('Pay only when a shopper accepts your free trial')).toBeInTheDocument()
      expect(screen.getByText('Best for emerging brands seeking first-time customers')).toBeInTheDocument()
      expect(screen.getByText('Track repeat purchases after the free trial')).toBeInTheDocument()
    })

    it('renders Targeted Swap features list', () => {
      renderPricing()
      expect(screen.getByText(/Offer a discount when someone adds a competitor/)).toBeInTheDocument()
      expect(screen.getByText('Pay only when a shopper swaps to your brand')).toBeInTheDocument()
      expect(screen.getByText('Best for brands competing in established categories')).toBeInTheDocument()
      expect(screen.getByText('Precision targeting at the moment of purchase decision')).toBeInTheDocument()
    })

    it('has deal campaign CTAs linking to campaign signup', () => {
      renderPricing()
      const freeTrialLinks = screen.getAllByText(/Create a Free Deal/)
      expect(freeTrialLinks.length).toBeGreaterThanOrEqual(1)
      expect(freeTrialLinks[0]).toHaveAttribute('href', '/campaign/signup')
      const swapLink = screen.getByText(/Create a Swap/)
      expect(swapLink).toHaveAttribute('href', '/campaign/signup')
    })
  })

  // ===== AD CAMPAIGNS =====
  describe('ad campaign cards', () => {
    it('renders the CPM card', () => {
      renderPricing()
      expect(screen.getByText('CPM')).toBeInTheDocument()
      expect(screen.getByText('Cost Per Thousand Impressions')).toBeInTheDocument()
    })

    it('renders the CPC card', () => {
      renderPricing()
      expect(screen.getByText('CPC')).toBeInTheDocument()
      expect(screen.getByText('Cost Per Click')).toBeInTheDocument()
    })

    it('displays CPM starting price of $6.00', () => {
      renderPricing()
      expect(screen.getByText('$6.00')).toBeInTheDocument()
      expect(screen.getAllByText('starting CPM').length).toBe(1)
    })

    it('displays CPC starting price of $0.50', () => {
      renderPricing()
      expect(screen.getByText('$0.50')).toBeInTheDocument()
      expect(screen.getAllByText('starting CPC').length).toBe(1)
    })

    it('shows Awareness badge on CPM card', () => {
      renderPricing()
      expect(screen.getByText('Awareness')).toBeInTheDocument()
    })

    it('shows Performance badge on CPC card', () => {
      renderPricing()
      expect(screen.getByText('Performance')).toBeInTheDocument()
    })

    it('renders CPM features list', () => {
      renderPricing()
      expect(screen.getByText('Your product shown to shoppers building lists')).toBeInTheDocument()
      expect(screen.getByText('Pay per 1,000 impressions delivered')).toBeInTheDocument()
      expect(screen.getByText('Best for brand awareness campaigns')).toBeInTheDocument()
      expect(screen.getByText('Reach shoppers across all categories')).toBeInTheDocument()
    })

    it('renders CPC features list', () => {
      renderPricing()
      expect(screen.getByText(/Pay only when shoppers click/)).toBeInTheDocument()
      expect(screen.getByText('Best for driving product consideration')).toBeInTheDocument()
      expect(screen.getByText(/Optimize bids based on real-time performance/)).toBeInTheDocument()
    })

    it('has Get Started links in ad cards pointing to campaign signup', () => {
      renderPricing()
      const getStartedLinks = screen.getAllByText(/Get Started/)
      expect(getStartedLinks.length).toBe(2)
      getStartedLinks.forEach(link => {
        expect(link).toHaveAttribute('href', '/campaign/signup')
      })
    })

    it('renders the pricing-ads section', () => {
      renderPricing()
      expect(document.querySelector('.pricing-ads')).toBeInTheDocument()
    })
  })

  // ===== ALL PRICING CARDS =====
  describe('all pricing cards combined', () => {
    it('renders exactly four pricing cards', () => {
      renderPricing()
      const cards = document.querySelectorAll('.pricing-card')
      expect(cards.length).toBe(4)
    })
  })

  // ===== BILLING DETAILS SECTION =====
  describe('billing details section', () => {
    it('displays the How billing works heading', () => {
      renderPricing()
      expect(screen.getByText('How billing works')).toBeInTheDocument()
    })

    it('displays the billing subtitle', () => {
      renderPricing()
      expect(
        screen.getByText('Simple, transparent, and completely in your control.')
      ).toBeInTheDocument()
    })

    it('renders billing step 1: Set a budget', () => {
      renderPricing()
      expect(screen.getByText('Set a budget')).toBeInTheDocument()
      expect(screen.getByText(/min \$25\/day/)).toBeInTheDocument()
    })

    it('renders billing step 2: Your ads run', () => {
      renderPricing()
      expect(screen.getByText('Your ads run')).toBeInTheDocument()
      expect(screen.getByText(/charged per impression.*or per click/)).toBeInTheDocument()
    })

    it('renders billing step 3: Adjust anytime', () => {
      renderPricing()
      expect(screen.getByText('Adjust anytime')).toBeInTheDocument()
      expect(screen.getByText(/no penalties, no contracts/)).toBeInTheDocument()
    })

    it('renders three billing step numbers', () => {
      renderPricing()
      const stepNumbers = document.querySelectorAll('.billing-step-number')
      expect(stepNumbers.length).toBe(3)
    })
  })

  // ===== FAQ SECTION =====
  describe('FAQ section', () => {
    it('displays the FAQ heading', () => {
      renderPricing()
      expect(screen.getByText('Frequently asked questions')).toBeInTheDocument()
    })

    it('renders all 8 FAQ questions', () => {
      renderPricing()
      expect(screen.getByText("What's the minimum budget?")).toBeInTheDocument()
      expect(screen.getByText('How does billing work?')).toBeInTheDocument()
      expect(screen.getByText('Can I pause my campaign?')).toBeInTheDocument()
      expect(screen.getByText('What payment methods do you accept?')).toBeInTheDocument()
      expect(screen.getByText('Are there any contracts or commitments?')).toBeInTheDocument()
      expect(screen.getByText('What analytics do I get?')).toBeInTheDocument()
      expect(screen.getByText('How do free trial deals work?')).toBeInTheDocument()
      expect(screen.getByText("What's a targeted swap?")).toBeInTheDocument()
    })

    it('all FAQ questions are buttons with aria-expanded', () => {
      renderPricing()
      const faqButtons = document.querySelectorAll('.faq-question')
      expect(faqButtons.length).toBe(8)
      faqButtons.forEach(btn => {
        expect(btn.getAttribute('aria-expanded')).toBe('false')
      })
    })

    it('shows + toggle indicator for closed FAQs', () => {
      renderPricing()
      const toggles = document.querySelectorAll('.faq-toggle')
      toggles.forEach(toggle => {
        expect(toggle.textContent).toBe('+')
      })
    })

    it('does not show any FAQ answers by default', () => {
      renderPricing()
      const answers = document.querySelectorAll('.faq-answer')
      expect(answers.length).toBe(0)
    })

    it('opens a FAQ answer on question click', async () => {
      const user = userEvent.setup()
      renderPricing()
      const firstQuestion = screen.getByText("What's the minimum budget?")
      await user.click(firstQuestion.closest('button')!)
      expect(screen.getByText(/Daily budgets start at \$25\/day/)).toBeInTheDocument()
    })

    it('sets aria-expanded to true when FAQ is opened', async () => {
      const user = userEvent.setup()
      renderPricing()
      const firstQuestion = screen.getByText("What's the minimum budget?")
      const btn = firstQuestion.closest('button')!
      await user.click(btn)
      expect(btn).toHaveAttribute('aria-expanded', 'true')
    })

    it('shows minus toggle for open FAQ', async () => {
      const user = userEvent.setup()
      renderPricing()
      const firstQuestion = screen.getByText("What's the minimum budget?")
      await user.click(firstQuestion.closest('button')!)
      const faqItem = document.querySelector('.faq-item.open')
      const toggle = faqItem?.querySelector('.faq-toggle')
      expect(toggle?.textContent).toContain('\u2212')
    })

    it('closes an open FAQ when clicking on it again', async () => {
      const user = userEvent.setup()
      renderPricing()
      const firstQuestion = screen.getByText("What's the minimum budget?")
      const btn = firstQuestion.closest('button')!
      await user.click(btn)
      expect(screen.getByText(/Daily budgets start at \$25\/day/)).toBeInTheDocument()
      await user.click(btn)
      expect(screen.queryByText(/Daily budgets start at \$25\/day/)).not.toBeInTheDocument()
    })

    it('only shows one FAQ answer at a time', async () => {
      const user = userEvent.setup()
      renderPricing()
      const firstQuestion = screen.getByText("What's the minimum budget?")
      await user.click(firstQuestion.closest('button')!)
      expect(screen.getByText(/Daily budgets start at \$25\/day/)).toBeInTheDocument()

      const secondQuestion = screen.getByText('How does billing work?')
      await user.click(secondQuestion.closest('button')!)
      expect(screen.queryByText(/Daily budgets start at \$25\/day/)).not.toBeInTheDocument()
      expect(screen.getByText(/Submitting your brief here doesn't collect a card/)).toBeInTheDocument()
      // Marketing site must NOT claim it authorizes a card — that happens in the brand portal
      expect(screen.queryByText(/Your card is authorized/)).not.toBeInTheDocument()
    })

    it('renders FAQ answer with region role', async () => {
      const user = userEvent.setup()
      renderPricing()
      const firstQuestion = screen.getByText("What's the minimum budget?")
      await user.click(firstQuestion.closest('button')!)
      const answer = document.querySelector('.faq-answer')
      expect(answer?.getAttribute('role')).toBe('region')
    })

    it('renders FAQ answers with proper id for aria-controls', async () => {
      const user = userEvent.setup()
      renderPricing()
      const firstQuestion = screen.getByText("What's the minimum budget?")
      const btn = firstQuestion.closest('button')!
      await user.click(btn)
      expect(btn).toHaveAttribute('aria-controls', 'faq-answer-0')
      expect(document.getElementById('faq-answer-0')).toBeInTheDocument()
    })

    it('FAQ about payment methods mentions Stripe', async () => {
      const user = userEvent.setup()
      renderPricing()
      const question = screen.getByText('What payment methods do you accept?')
      await user.click(question.closest('button')!)
      expect(screen.getByText(/Stripe-powered payment system/)).toBeInTheDocument()
    })

    it('FAQ about contracts mentions self-serve and no commitments', async () => {
      const user = userEvent.setup()
      renderPricing()
      const question = screen.getByText('Are there any contracts or commitments?')
      await user.click(question.closest('button')!)
      expect(screen.getByText(/completely self-serve/)).toBeInTheDocument()
      expect(screen.getByText(/no minimum commitments/)).toBeInTheDocument()
    })

    it('FAQ about pausing confirms you can pause anytime', async () => {
      const user = userEvent.setup()
      renderPricing()
      const question = screen.getByText('Can I pause my campaign?')
      await user.click(question.closest('button')!)
      expect(screen.getByText(/pause, resume, or stop your campaign at any time/)).toBeInTheDocument()
    })

    it('FAQ about analytics mentions impressions, clicks, list adds', async () => {
      const user = userEvent.setup()
      renderPricing()
      const question = screen.getByText('What analytics do I get?')
      await user.click(question.closest('button')!)
      expect(screen.getByText(/Track impressions, clicks, list adds/)).toBeInTheDocument()
    })
  })

  // ===== CTA SECTION =====
  describe('CTA section', () => {
    it('displays the CTA headline', () => {
      renderPricing()
      expect(screen.getByText('Ready to get your product on their list?')).toBeInTheDocument()
    })

    it('displays the CTA body text', () => {
      renderPricing()
      expect(
        screen.getByText(/Create a free deal or targeted swap today/)
      ).toBeInTheDocument()
    })

    it('has a primary CTA linking to campaign signup', () => {
      renderPricing()
      const ctaSection = document.querySelector('.cta-section')!
      const ctaLink = ctaSection.querySelector('.btn-cta-primary')!
      expect(ctaLink).toHaveTextContent(/Create a Free Deal/)
      expect(ctaLink).toHaveAttribute('href', '/campaign/signup')
    })

    it('has a secondary Learn More link to for-brands page', () => {
      renderPricing()
      expect(screen.getByText('Learn More')).toHaveAttribute('href', '/for-brands')
    })
  })

  // ===== BRAND CONSISTENCY =====
  describe('brand consistency', () => {
    it('does not use corporate jargon in headlines', () => {
      renderPricing()
      const headings = screen.getAllByRole('heading')
      const jargonTerms = [
        /\bsynergy\b/i, /\bparadigm\b/i, /\bdisrupt\b/i,
        /\bleverage\b/i, /\bholistic\b/i
      ]
      headings.forEach(heading => {
        jargonTerms.forEach(term => {
          expect(heading.textContent).not.toMatch(term)
        })
      })
    })

    it('uses clear, jargon-free language throughout', () => {
      renderPricing()
      // Good phrases that pass "text to a friend" test
      expect(screen.getByText(/Set a budget/)).toBeInTheDocument()
      expect(screen.getByText(/Pay for trials, not impressions/)).toBeInTheDocument()
    })
  })

  // ===== FADE ANIMATIONS =====
  describe('fade-in animation sections', () => {
    it('applies fade-section class to pricing models section', () => {
      renderPricing()
      const section = document.querySelector('.pricing-models')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })

    it('applies fade-section class to pricing ads section', () => {
      renderPricing()
      const section = document.querySelector('.pricing-ads')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })

    it('applies fade-section class to pricing details section', () => {
      renderPricing()
      const section = document.querySelector('.pricing-details')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })

    it('applies fade-section class to FAQ section', () => {
      renderPricing()
      const section = document.querySelector('.pricing-faq')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })

    it('applies fade-section class to CTA section', () => {
      renderPricing()
      const section = document.querySelector('.cta-section')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })
  })

  // ===== PAGE STRUCTURE =====
  describe('page structure', () => {
    it('renders multiple sections', () => {
      renderPricing()
      const sections = document.querySelectorAll('section')
      expect(sections.length).toBeGreaterThanOrEqual(4)
    })

    it('contains a hero section with pricing-hero class', () => {
      renderPricing()
      expect(document.querySelector('.pricing-hero')).toBeInTheDocument()
    })

    it('renders exactly 6 sections', () => {
      renderPricing()
      const sections = document.querySelectorAll('section')
      expect(sections.length).toBe(6)
    })

    it('wraps content in section-inner containers', () => {
      renderPricing()
      const innerContainers = document.querySelectorAll('.section-inner')
      expect(innerContainers.length).toBeGreaterThanOrEqual(3)
    })
  })

  // ===== ACCESSIBILITY =====
  describe('accessibility', () => {
    it('has exactly one h1 element', () => {
      renderPricing()
      const h1s = screen.getAllByRole('heading', { level: 1 })
      expect(h1s.length).toBe(1)
    })

    it('FAQ buttons are keyboard accessible', () => {
      renderPricing()
      const faqButtons = document.querySelectorAll('.faq-question')
      faqButtons.forEach(btn => {
        expect(btn.tagName).toBe('BUTTON')
      })
    })

    it('FAQ items use aria-controls linking question to answer', () => {
      renderPricing()
      const faqButtons = document.querySelectorAll('.faq-question')
      faqButtons.forEach((btn, i) => {
        expect(btn.getAttribute('aria-controls')).toBe(`faq-answer-${i}`)
      })
    })

    it('pricing card features use list semantics', () => {
      renderPricing()
      const lists = document.querySelectorAll('.pricing-card-features')
      expect(lists.length).toBe(4)
      lists.forEach(list => {
        expect(list.tagName).toBe('UL')
      })
    })
  })

  // ===== SECTION ORDERING =====
  describe('section ordering', () => {
    it('hero comes before pricing models', () => {
      renderPricing()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const heroIdx = classNames.indexOf('pricing-hero')
      const modelsIdx = classNames.findIndex(c => c === 'pricing-models')
      expect(heroIdx).toBeLessThan(modelsIdx)
    })

    it('pricing models comes before pricing ads', () => {
      renderPricing()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const modelsIdx = classNames.findIndex(c => c === 'pricing-models')
      const adsIdx = classNames.findIndex(c => c === 'pricing-ads')
      expect(modelsIdx).toBeLessThan(adsIdx)
    })

    it('pricing ads comes before billing details', () => {
      renderPricing()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const adsIdx = classNames.findIndex(c => c === 'pricing-ads')
      const detailsIdx = classNames.findIndex(c => c === 'pricing-details')
      expect(adsIdx).toBeLessThan(detailsIdx)
    })

    it('billing details comes before FAQ', () => {
      renderPricing()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const detailsIdx = classNames.findIndex(c => c === 'pricing-details')
      const faqIdx = classNames.findIndex(c => c === 'pricing-faq')
      expect(detailsIdx).toBeLessThan(faqIdx)
    })

    it('FAQ comes before CTA', () => {
      renderPricing()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const faqIdx = classNames.findIndex(c => c === 'pricing-faq')
      const ctaIdx = classNames.findIndex(c => c === 'cta-section')
      expect(faqIdx).toBeLessThan(ctaIdx)
    })
  })

  // ===== CONTENT DETAILS =====
  describe('pricing content details', () => {
    it('all four pricing cards have 4 feature items each', () => {
      renderPricing()
      const cards = document.querySelectorAll('.pricing-card')
      expect(cards.length).toBe(4)
      cards.forEach(card => {
        const features = card.querySelectorAll('.pricing-card-features li')
        expect(features.length).toBe(4)
      })
    })

    it('FAQ toggle icons all start as plus', () => {
      renderPricing()
      const toggles = document.querySelectorAll('.faq-toggle')
      expect(toggles.length).toBe(8)
      toggles.forEach(toggle => {
        expect(toggle.textContent).toBe('+')
      })
    })

    it('billing steps have sequential numbers 1-3', () => {
      renderPricing()
      const steps = document.querySelectorAll('.billing-step-number')
      expect(steps[0].textContent).toBe('1')
      expect(steps[1].textContent).toBe('2')
      expect(steps[2].textContent).toBe('3')
    })

    it('pricing card badges differentiate between card types', () => {
      renderPricing()
      const badges = document.querySelectorAll('.pricing-card-badge')
      expect(badges.length).toBe(4)
      const badgeTexts = Array.from(badges).map(b => b.textContent)
      expect(badgeTexts).toContain('Most Popular')
      expect(badgeTexts).toContain('Challenger')
      expect(badgeTexts).toContain('Awareness')
      expect(badgeTexts).toContain('Performance')
    })
  })

  // ===== FAQ COMPREHENSIVE COVERAGE =====
  describe('FAQ answer content', () => {
    it('billing FAQ mentions $500 threshold', async () => {
      const user = userEvent.setup()
      renderPricing()
      const question = screen.getByText('How does billing work?')
      await user.click(question.closest('button')!)
      expect(screen.getByText(/\$500/)).toBeInTheDocument()
    })

    it('minimum budget FAQ mentions $100 for total', async () => {
      const user = userEvent.setup()
      renderPricing()
      const question = screen.getByText("What's the minimum budget?")
      await user.click(question.closest('button')!)
      expect(screen.getByText(/minimum is \$100/)).toBeInTheDocument()
    })

    it('payment FAQ mentions all 4 card brands', async () => {
      const user = userEvent.setup()
      renderPricing()
      const question = screen.getByText('What payment methods do you accept?')
      await user.click(question.closest('button')!)
      expect(screen.getByText(/Visa/)).toBeInTheDocument()
      expect(screen.getByText(/Mastercard/)).toBeInTheDocument()
      expect(screen.getByText(/American Express/)).toBeInTheDocument()
      expect(screen.getByText(/Discover/)).toBeInTheDocument()
    })

    it('free trial FAQ explains how trials work', async () => {
      const user = userEvent.setup()
      renderPricing()
      const question = screen.getByText('How do free trial deals work?')
      await user.click(question.closest('button')!)
      expect(screen.getByText(/retail value of your product/)).toBeInTheDocument()
      expect(screen.getByText(/shopper picks it up in-store at no cost/)).toBeInTheDocument()
    })

    it('targeted swap FAQ explains what a swap is', async () => {
      const user = userEvent.setup()
      renderPricing()
      const question = screen.getByText("What's a targeted swap?")
      await user.click(question.closest('button')!)
      expect(screen.getByText(/offer a discount when a shopper adds a competitor/)).toBeInTheDocument()
      expect(screen.getByText(/they buy your product instead/)).toBeInTheDocument()
    })
  })
})
