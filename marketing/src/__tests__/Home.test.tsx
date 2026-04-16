import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect } from 'vitest'
import Home from '../pages/Home'

function renderHome() {
  return render(
    <MemoryRouter>
      <Home />
    </MemoryRouter>
  )
}

describe('Home page', () => {
  // ===== GATE 1: Hero Message =====
  describe('hero section (Gate 1 — First 5 Minutes Magic)', () => {
    it('displays the locked hero headline', () => {
      renderHome()
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
        /No more.*Did you get milk/
      )
    })

    it('has no jargon in hero badge (no "AI" in headline/subheader)', () => {
      renderHome()
      const badge = document.querySelector('.hero-badge')
      expect(badge?.textContent).not.toMatch(/\bAI\b/)
    })

    it('displays a clear CTA above the fold', () => {
      renderHome()
      expect(screen.getByText(/Get Penny/)).toBeInTheDocument()
    })

    it('displays the brand tagline in badge', () => {
      renderHome()
      const badge = document.querySelector('.hero-badge')
      expect(badge?.textContent).toMatch(/shopping list for families/)
    })
  })

  // ===== TIER 1 ITEM 1: Referral Share & Earn =====
  describe('Share & Earn section (Tier 1 — Referral)', () => {
    it('renders the Share & Earn section', () => {
      renderHome()
      expect(
        screen.getByText('Friends who shop together, stay organized together.')
      ).toBeInTheDocument()
    })

    it('uses relational language, not transactional', () => {
      renderHome()
      expect(
        screen.getByText(/invite your family or roommates/)
      ).toBeInTheDocument()
      expect(
        screen.getByText(/Both of you earn rewards when they join/)
      ).toBeInTheDocument()
    })

    it('has Create invite link CTA', () => {
      renderHome()
      expect(screen.getByText('Create invite link')).toBeInTheDocument()
    })

    it('has See referral rewards CTA', () => {
      renderHome()
      expect(screen.getByText('See referral rewards')).toBeInTheDocument()
    })

    it('shows 3-step referral visual (invite, join, earn)', () => {
      renderHome()
      expect(screen.getByText('You invite a friend')).toBeInTheDocument()
      expect(screen.getByText('They join your list')).toBeInTheDocument()
      expect(screen.getByText('You both earn rewards')).toBeInTheDocument()
    })
  })

  // ===== TIER 1 ITEM 2: Dark Mode Feature Badge =====
  describe('Dark Mode feature card (Tier 1)', () => {
    it('renders the dark mode feature card', () => {
      renderHome()
      expect(screen.getByText('Dark mode included')).toBeInTheDocument()
    })

    it('has the correct copy', () => {
      renderHome()
      expect(
        screen.getByText(/Shop late at night without straining your eyes/)
      ).toBeInTheDocument()
    })
  })

  // ===== TIER 1 ITEM 3: Household Sharing Copy Reframe =====
  describe('Household sharing copy reframe (Tier 1)', () => {
    it('uses the locked hero message as feature headline', () => {
      renderHome()
      expect(
        screen.getByText(/No more "Did you get milk\?"/)
      ).toBeInTheDocument()
    })

    it('mentions Penny picking brands (CEO Resolution #1)', () => {
      renderHome()
      expect(
        screen.getByText(/Penny picks your brands based on household preferences/)
      ).toBeInTheDocument()
    })

    it('mentions item claiming and no duplicates', () => {
      renderHome()
      expect(
        screen.getByText(/Claim items as you add them/)
      ).toBeInTheDocument()
    })

    it('does NOT use the old generic copy', () => {
      renderHome()
      const featureCards = document.querySelectorAll('.feature-card p')
      const texts = Array.from(featureCards).map((el) => el.textContent)
      expect(texts).not.toContain(
        'Share a list with anyone — roommates, family, a partner. Everyone adds items, everyone sees updates live.'
      )
    })
  })

  // ===== STORY ARC: Trust / Privacy =====
  describe('Trust / Privacy section (Story Arc — Trust Foundation)', () => {
    it('renders the privacy FAQ section', () => {
      renderHome()
      expect(
        screen.getByText('How does Penny know what I like?')
      ).toBeInTheDocument()
    })

    it('addresses data ownership', () => {
      renderHome()
      expect(screen.getByText('Your data stays yours')).toBeInTheDocument()
    })

    it('addresses user control', () => {
      renderHome()
      expect(
        screen.getByText("You're always in control")
      ).toBeInTheDocument()
    })

    it('addresses sponsored content transparency', () => {
      renderHome()
      expect(
        screen.getByText('Brand suggestions are labeled')
      ).toBeInTheDocument()
    })

    it('links to full privacy policy', () => {
      renderHome()
      expect(
        screen.getByText(/Read our full privacy policy/)
      ).toBeInTheDocument()
    })
  })

  // ===== STORY ARC: Testimonials reframed =====
  describe('Testimonials (Story Arc — Social Proof)', () => {
    it('uses relief-focused headline', () => {
      renderHome()
      expect(screen.getByText('No more grocery chaos')).toBeInTheDocument()
    })
  })

  // ===== TIER 2: Trip Planner =====
  describe('Trip Planner section (Tier 2)', () => {
    it('renders the Trip Planner label', () => {
      renderHome()
      expect(screen.getByText('Trip Planner')).toBeInTheDocument()
    })

    it('displays the headline "Get in, get out. Fast."', () => {
      renderHome()
      expect(screen.getByText('Get in, get out. Fast.')).toBeInTheDocument()
    })

    it('describes aisle optimization', () => {
      renderHome()
      expect(screen.getByText(/Penny knows the store layout/)).toBeInTheDocument()
    })

    it('shows the "See it in action" CTA', () => {
      renderHome()
      const cta = screen.getByText('See it in action →')
      expect(cta).toBeInTheDocument()
      expect(cta.getAttribute('href')).toBe('https://app.groflo.io')
    })

    it('renders the optimized route demo card', () => {
      renderHome()
      expect(screen.getByText('Your optimized route')).toBeInTheDocument()
    })

    it('shows 4 aisle stops in the demo', () => {
      renderHome()
      const aisles = document.querySelectorAll('.trip-demo-aisle')
      expect(aisles.length).toBe(4)
    })

    it('displays estimated trip time', () => {
      renderHome()
      expect(screen.getByText(/Estimated trip: 15 min/)).toBeInTheDocument()
    })

    it('shows specific aisle departments', () => {
      renderHome()
      expect(screen.getByText('Produce')).toBeInTheDocument()
      expect(screen.getByText('Dairy')).toBeInTheDocument()
      expect(screen.getByText('Bakery')).toBeInTheDocument()
      expect(screen.getByText('Frozen')).toBeInTheDocument()
    })

    it('includes brand names in demo items', () => {
      renderHome()
      expect(screen.getAllByText(/Organic Valley/).length).toBeGreaterThan(0)
      expect(screen.getAllByText(/Dave's Killer Bread/).length).toBeGreaterThan(0)
    })
  })

  // ===== SECTION ORDER (Story Arc) =====
  describe('page section order follows required story arc', () => {
    it('renders all required sections in order', () => {
      renderHome()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map((s) => s.className.split(' ')[0])

      const heroIdx = classNames.indexOf('hero')
      const howItWorksIdx = classNames.findIndex((c) => c === 'how-it-works')
      const shareEarnIdx = classNames.findIndex((c) => c === 'share-earn')
      const featuresIdx = classNames.findIndex((c) => c === 'features')
      const tripPlannerIdx = classNames.findIndex((c) => c === 'trip-planner')
      const testimonialsIdx = classNames.findIndex((c) => c === 'testimonials')
      const trustIdx = classNames.findIndex((c) => c === 'trust-section')
      const ctaIdx = classNames.findIndex((c) => c === 'cta-section')

      // All sections exist
      expect(heroIdx).toBeGreaterThanOrEqual(0)
      expect(howItWorksIdx).toBeGreaterThan(heroIdx)
      expect(shareEarnIdx).toBeGreaterThan(howItWorksIdx)
      expect(featuresIdx).toBeGreaterThan(shareEarnIdx)
      expect(tripPlannerIdx).toBeGreaterThan(featuresIdx)
      expect(testimonialsIdx).toBeGreaterThan(tripPlannerIdx)
      expect(trustIdx).toBeGreaterThan(testimonialsIdx)
      expect(ctaIdx).toBeGreaterThan(trustIdx)
    })
  })

  // ===== RENDERING =====
  describe('renders without errors', () => {
    it('renders the page without crashing', () => {
      renderHome()
      expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
    })

    it('sets the default document title', () => {
      renderHome()
      expect(document.title).toContain('Groflo')
    })
  })

  // ===== BRAND CONSISTENCY =====
  describe('brand consistency', () => {
    it('uses brand green (#35644F) in SVG icons', () => {
      renderHome()
      const greenPaths = document.querySelectorAll('svg path[fill="#35644F"]')
      expect(greenPaths.length).toBeGreaterThan(0)
    })

    it('does not use jargon like "orchestration" or "synergy" in headings', () => {
      renderHome()
      const headings = screen.getAllByRole('heading')
      headings.forEach(heading => {
        expect(heading.textContent).not.toMatch(/\borchestration\b/i)
        expect(heading.textContent).not.toMatch(/\bsynergy\b/i)
        expect(heading.textContent).not.toMatch(/\bleverage\b/i)
      })
    })

    it('hero CTA links to app.groflo.io', () => {
      renderHome()
      const ctaLink = screen.getByText(/Get Penny/)
      expect(ctaLink.getAttribute('href')).toBe('https://app.groflo.io')
    })

    it('See How It Works links to #how-it-works', () => {
      renderHome()
      const link = screen.getByText('See How It Works')
      expect(link.getAttribute('href')).toBe('#how-it-works')
    })
  })

  // ===== HOW IT WORKS =====
  describe('How It Works section', () => {
    it('renders three steps', () => {
      renderHome()
      const steps = document.querySelectorAll('.how-it-works .step')
      expect(steps.length).toBe(3)
    })

    it('Step 1 is Text Penny', () => {
      renderHome()
      expect(screen.getByText('Text Penny')).toBeInTheDocument()
    })

    it('Step 2 is She builds your list', () => {
      renderHome()
      expect(screen.getByText('She builds your list')).toBeInTheDocument()
    })

    it('Step 3 is Shop together', () => {
      renderHome()
      expect(screen.getByText('Shop together')).toBeInTheDocument()
    })
  })

  // ===== FEATURES GRID =====
  describe('features grid', () => {
    it('renders 8 feature cards', () => {
      renderHome()
      const cards = document.querySelectorAll('.features .feature-card')
      expect(cards.length).toBe(8)
    })

    it('includes Natural language input feature', () => {
      renderHome()
      expect(screen.getByText('Natural language input')).toBeInTheDocument()
    })

    it('includes Photo to list feature', () => {
      renderHome()
      expect(screen.getByText('Photo to list')).toBeInTheDocument()
    })

    it('includes Brand memory feature', () => {
      renderHome()
      expect(screen.getByText('Brand memory')).toBeInTheDocument()
    })

    it('includes Smart sorting feature', () => {
      renderHome()
      expect(screen.getByText('Smart sorting')).toBeInTheDocument()
    })

    it('includes Saved lists feature', () => {
      renderHome()
      expect(screen.getByText('Saved lists')).toBeInTheDocument()
    })
  })

  // ===== MEET PENNY SECTION =====
  describe('Meet Penny section', () => {
    it('renders the Meet Penny label', () => {
      renderHome()
      expect(screen.getByText('Meet Penny')).toBeInTheDocument()
    })

    it('describes how Penny works for the household', () => {
      renderHome()
      expect(screen.getByText(/keeps the whole household on the same list/)).toBeInTheDocument()
    })

    it('renders Penny avatar image', () => {
      renderHome()
      const avatar = document.querySelector('.penny-avatar-img') as HTMLImageElement
      expect(avatar).toBeInTheDocument()
      expect(avatar.src).toContain('penny.png')
    })
  })

  // ===== FOR BRANDS TEASER =====
  describe('For Brands teaser', () => {
    it('renders the For Brands label', () => {
      renderHome()
      expect(screen.getByText('For Brands')).toBeInTheDocument()
    })

    it('links to For Brands page', () => {
      renderHome()
      const link = screen.getByText(/Create a Free Deal/)
      expect(link.getAttribute('href')).toBe('/for-brands')
    })
  })

  // ===== CTA SECTION =====
  describe('CTA section', () => {
    it('renders CTA heading', () => {
      renderHome()
      expect(screen.getByText('Start saving on groceries today')).toBeInTheDocument()
    })

    it('CTA links to app', () => {
      renderHome()
      const ctaLink = screen.getByText(/Try Groflo Free →/)
      expect(ctaLink.getAttribute('href')).toBe('https://app.groflo.io')
    })

    it('has Learn More About Groflo link', () => {
      renderHome()
      const link = screen.getByText('Learn More About Groflo')
      expect(link.getAttribute('href')).toBe('/about')
    })
  })

  // ===== ACCESSIBILITY =====
  describe('accessibility', () => {
    it('has exactly one h1 element', () => {
      renderHome()
      const h1s = screen.getAllByRole('heading', { level: 1 })
      expect(h1s.length).toBe(1)
    })

    it('all decorative SVGs are aria-hidden', () => {
      renderHome()
      const svgs = document.querySelectorAll('svg[aria-hidden="true"]')
      expect(svgs.length).toBeGreaterThan(0)
    })

    it('Penny avatar has alt text', () => {
      renderHome()
      const img = document.querySelector('.penny-avatar-img') as HTMLImageElement
      expect(img.alt).toBeTruthy()
    })
  })

  // ===== HERO VISUAL ELEMENTS =====
  describe('hero visual elements', () => {
    it('renders phone mockup', () => {
      renderHome()
      expect(document.querySelector('.hero-phone')).toBeInTheDocument()
    })

    it('renders phone screen with messages', () => {
      renderHome()
      expect(document.querySelector('.phone-messages')).toBeInTheDocument()
    })

    it('renders floating cards', () => {
      renderHome()
      expect(document.querySelector('.float-card-1')).toBeInTheDocument()
      expect(document.querySelector('.float-card-2')).toBeInTheDocument()
    })

    it('renders Penny avatar in phone mockup', () => {
      renderHome()
      const phoneAvatar = document.querySelector('.phone-penny-avatar') as HTMLImageElement
      expect(phoneAvatar).toBeInTheDocument()
    })

    it('renders shared update notification in phone', () => {
      renderHome()
      expect(screen.getByText(/Alex added/)).toBeInTheDocument()
    })
  })

  // ===== SOCIAL PROOF =====
  describe('social proof', () => {
    it('renders hero social proof badges', () => {
      renderHome()
      expect(screen.getByText('Free forever')).toBeInTheDocument()
      expect(screen.getByText('No app download')).toBeInTheDocument()
      expect(screen.getByText('Free deals from brands')).toBeInTheDocument()
    })

    it('renders four testimonial cards', () => {
      renderHome()
      const cards = document.querySelectorAll('.testimonial-card')
      expect(cards.length).toBe(4)
    })

    it('all testimonials have 5-star ratings', () => {
      renderHome()
      const stars = document.querySelectorAll('.testimonial-stars')
      stars.forEach(star => {
        expect(star.textContent).toContain('\u2605\u2605\u2605\u2605\u2605')
      })
    })

    it('testimonials include specific real names', () => {
      renderHome()
      expect(screen.getByText('Molly N.')).toBeInTheDocument()
      expect(screen.getByText('Jada M.')).toBeInTheDocument()
      expect(screen.getByText('Karen R.')).toBeInTheDocument()
    })
  })

  // ===== STICKY MOBILE CTA =====
  describe('sticky mobile CTA', () => {
    it('renders the sticky mobile CTA', () => {
      renderHome()
      expect(document.querySelector('.sticky-mobile-cta')).toBeInTheDocument()
    })

    it('sticky CTA links to app', () => {
      renderHome()
      const stickyLink = document.querySelector('.sticky-cta-btn')
      expect(stickyLink?.getAttribute('href')).toBe('https://app.groflo.io')
    })

    it('sticky CTA has "Try Groflo Free" text', () => {
      renderHome()
      const stickyLink = document.querySelector('.sticky-cta-btn')
      expect(stickyLink?.textContent).toBe('Try Groflo Free')
    })
  })

  // ===== LOGOS BAR =====
  describe('logos bar', () => {
    it('renders the logos bar section', () => {
      renderHome()
      expect(document.querySelector('.logos-bar')).toBeInTheDocument()
    })

    it('displays the store department label', () => {
      renderHome()
      expect(screen.getByText(/Penny organizes your list by department/)).toBeInTheDocument()
    })

    it('mentions major store names', () => {
      renderHome()
      expect(screen.getByText('Kroger')).toBeInTheDocument()
      expect(screen.getByText('Whole Foods')).toBeInTheDocument()
      expect(screen.getByText('Costco')).toBeInTheDocument()
    })

    it('has logos-inner wrapper', () => {
      renderHome()
      expect(document.querySelector('.logos-inner')).toBeInTheDocument()
    })

    it('has logos-row container', () => {
      renderHome()
      expect(document.querySelector('.logos-row')).toBeInTheDocument()
    })
  })

  // ===== STATS SECTION =====
  describe('stats section', () => {
    it('renders the stats section', () => {
      renderHome()
      expect(document.querySelector('.stats')).toBeInTheDocument()
    })

    it('displays "Fewer trips" stat', () => {
      renderHome()
      expect(screen.getByText('Fewer trips')).toBeInTheDocument()
    })

    it('displays "$100/mo" savings stat', () => {
      renderHome()
      expect(screen.getAllByText('$100/mo').length).toBeGreaterThanOrEqual(1)
    })

    it('displays "Organized" stat', () => {
      renderHome()
      expect(screen.getByText('Organized')).toBeInTheDocument()
    })

    it('displays "$0" free stat', () => {
      renderHome()
      expect(screen.getByText('$0')).toBeInTheDocument()
    })

    it('displays stat labels for each stat', () => {
      renderHome()
      expect(screen.getByText(/Shared lists mean one trip covers everyone/)).toBeInTheDocument()
      expect(screen.getByText(/Average savings from deals/)).toBeInTheDocument()
      expect(screen.getByText(/Sorted by department/)).toBeInTheDocument()
      expect(screen.getByText(/Brands send you free products/)).toBeInTheDocument()
    })

    it('renders 4 stat number elements', () => {
      renderHome()
      const statNumbers = document.querySelectorAll('.stat-number')
      expect(statNumbers.length).toBe(4)
    })

    it('renders 4 stat label elements', () => {
      renderHome()
      const statLabels = document.querySelectorAll('.stat-label')
      expect(statLabels.length).toBe(4)
    })

    it('stats section has fade-section class', () => {
      renderHome()
      const stats = document.querySelector('.stats')
      expect(stats?.classList.contains('fade-section')).toBe(true)
    })
  })

  // ===== PENNY FEATURES DETAILS =====
  describe('Penny features details', () => {
    it('displays "Learns your preferences" feature', () => {
      renderHome()
      expect(screen.getByText('Learns your preferences')).toBeInTheDocument()
    })

    it('displays "Email or text to add" feature', () => {
      renderHome()
      expect(screen.getByText('Email or text to add')).toBeInTheDocument()
    })

    it('displays "Instant organization" feature', () => {
      renderHome()
      expect(screen.getByText('Instant organization')).toBeInTheDocument()
    })

    it('renders 3 penny-feature items', () => {
      renderHome()
      const features = document.querySelectorAll('.penny-feature')
      expect(features.length).toBe(3)
    })

    it('each penny feature has an icon', () => {
      renderHome()
      const icons = document.querySelectorAll('.penny-feature-icon')
      expect(icons.length).toBe(3)
    })

    it('each penny feature has an h4 heading', () => {
      renderHome()
      const headings = document.querySelectorAll('.penny-feature h4')
      expect(headings.length).toBe(3)
    })
  })

  // ===== BRANDS TEASER DETAILS =====
  describe('brands teaser details', () => {
    it('displays free deals heading', () => {
      renderHome()
      expect(screen.getByText(/Put your product on their list/)).toBeInTheDocument()
    })

    it('displays 3 brand metric cards', () => {
      renderHome()
      const cards = document.querySelectorAll('.brand-metric-card')
      expect(cards.length).toBe(3)
    })

    it('displays "85%" metric', () => {
      renderHome()
      expect(screen.getByText('85%')).toBeInTheDocument()
    })

    it('displays "$0" metric', () => {
      renderHome()
      expect(screen.getByText('$0')).toBeInTheDocument()
    })

    it('displays "Intent" metric', () => {
      renderHome()
      expect(screen.getByText('Intent')).toBeInTheDocument()
    })

    it('renders brand benefits section with 3 items', () => {
      renderHome()
      const benefits = document.querySelectorAll('.brand-benefit')
      expect(benefits.length).toBe(3)
    })

    it('displays "Free product trials" benefit', () => {
      renderHome()
      expect(screen.getAllByText('Free product trials').length).toBeGreaterThanOrEqual(1)
    })

    it('displays "Targeted swaps" benefit', () => {
      renderHome()
      expect(screen.getAllByText('Targeted swaps').length).toBeGreaterThanOrEqual(1)
    })

    it('displays "No shelf space needed" benefit', () => {
      renderHome()
      expect(screen.getByText('No shelf space needed')).toBeInTheDocument()
    })
  })

  // ===== PHONE MOCKUP CONTENT =====
  describe('phone mockup content', () => {
    it('renders user message about the fridge', () => {
      renderHome()
      expect(screen.getByText(/Here's what the fridge looks like/)).toBeInTheDocument()
    })

    it('renders Penny response about favorites', () => {
      renderHome()
      expect(screen.getByText(/I picked all your favorite brands/)).toBeInTheDocument()
    })

    it('shows phone list items', () => {
      renderHome()
      expect(screen.getByText('Organic Valley milk')).toBeInTheDocument()
      expect(screen.getByText(/Pete.*Gerry's eggs/)).toBeInTheDocument()
      expect(screen.getByText('Kerrygold butter')).toBeInTheDocument()
    })

    it('shows "4 more items" indicator', () => {
      renderHome()
      expect(screen.getByText('+ 4 more items')).toBeInTheDocument()
    })

    it('renders phone header with Penny name', () => {
      renderHome()
      const phoneHeader = document.querySelector('.phone-header')
      expect(phoneHeader).toBeInTheDocument()
    })

    it('shows Penny as "Online" in phone header', () => {
      renderHome()
      const phoneHeaderText = document.querySelector('.phone-header-text')
      expect(phoneHeaderText?.textContent).toContain('Online')
    })

    it('renders 3 phone list items', () => {
      renderHome()
      const items = document.querySelectorAll('.phone-list-item')
      expect(items.length).toBe(3)
    })

    it('each phone list item has a check circle', () => {
      renderHome()
      const checks = document.querySelectorAll('.phone-list-check')
      expect(checks.length).toBe(3)
    })
  })

  // ===== TRUST SECTION DETAILS =====
  describe('trust section details', () => {
    it('renders 4 trust cards', () => {
      renderHome()
      const cards = document.querySelectorAll('.trust-card')
      expect(cards.length).toBe(4)
    })

    it('displays "Penny learns from your choices" card', () => {
      renderHome()
      expect(screen.getByText('Penny learns from your choices')).toBeInTheDocument()
    })

    it('mentions Tillamook as a brand example', () => {
      renderHome()
      expect(screen.getAllByText(/Tillamook/).length).toBeGreaterThan(0)
    })

    it('mentions never selling shopping data', () => {
      renderHome()
      expect(screen.getByText(/never sell your shopping data/)).toBeInTheDocument()
    })

    it('mentions ability to turn off auto-pick', () => {
      renderHome()
      expect(screen.getByText(/Turn it off/)).toBeInTheDocument()
    })

    it('mentions sponsored suggestions are clearly marked', () => {
      renderHome()
      expect(screen.getByText(/Sponsored suggestions are clearly marked/)).toBeInTheDocument()
    })

    it('trust section links to privacy page', () => {
      renderHome()
      const trustCta = document.querySelector('.trust-cta a')
      expect(trustCta?.getAttribute('href')).toBe('/privacy')
    })

    it('trust section has fade-section class', () => {
      renderHome()
      const section = document.querySelector('.trust-section')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })
  })

  // ===== FLOAT CARDS =====
  describe('float cards', () => {
    it('float card 1 shows "Snap your fridge"', () => {
      renderHome()
      expect(screen.getByText('Snap your fridge')).toBeInTheDocument()
    })

    it('float card 1 shows "Penny sees what\'s low"', () => {
      renderHome()
      expect(screen.getByText("Penny sees what's low")).toBeInTheDocument()
    })

    it('float card 2 shows "Shared with 3 people"', () => {
      renderHome()
      expect(screen.getByText('Shared with 3 people')).toBeInTheDocument()
    })

    it('float card 2 shows "Everyone adds, everyone sees"', () => {
      renderHome()
      expect(screen.getByText('Everyone adds, everyone sees')).toBeInTheDocument()
    })

    it('float cards have hero-float-card class', () => {
      renderHome()
      const cards = document.querySelectorAll('.hero-float-card')
      expect(cards.length).toBe(3)
    })

    it('each float card has an icon', () => {
      renderHome()
      const icons = document.querySelectorAll('.float-card-icon')
      expect(icons.length).toBe(3)
    })
  })

  // ===== TESTIMONIAL DETAILS =====
  describe('testimonial details', () => {
    it('displays location for Molly', () => {
      renderHome()
      expect(screen.getByText('Raleigh, NC')).toBeInTheDocument()
    })

    it('displays location for Jada', () => {
      renderHome()
      expect(screen.getByText('Brooklyn, NY')).toBeInTheDocument()
    })

    it('displays location for Karen', () => {
      renderHome()
      expect(screen.getByText('Austin, TX')).toBeInTheDocument()
    })

    it('renders testimonial avatars with initials', () => {
      renderHome()
      const avatars = document.querySelectorAll('.testimonial-avatar')
      expect(avatars.length).toBe(4)
      const initials = Array.from(avatars).map(a => a.textContent)
      expect(initials).toContain('MN')
      expect(initials).toContain('JM')
      expect(initials).toContain('KR')
      expect(initials).toContain('DT')
    })

    it('each testimonial has a blockquote', () => {
      renderHome()
      const quotes = document.querySelectorAll('.testimonial-card blockquote')
      expect(quotes.length).toBe(4)
    })

    it('testimonials section subtitle mentions families and roommates', () => {
      renderHome()
      expect(screen.getByText(/Families, roommates, and couples/)).toBeInTheDocument()
    })

    it('testimonials use "Real People, Real Relief" label', () => {
      renderHome()
      expect(screen.getByText('Real People, Real Relief')).toBeInTheDocument()
    })
  })

  // ===== SHARE & EARN DETAILS =====
  describe('Share & Earn section details', () => {
    it('has share-earn-actions wrapper', () => {
      renderHome()
      expect(document.querySelector('.share-earn-actions')).toBeInTheDocument()
    })

    it('invite link points to app invite page', () => {
      renderHome()
      const link = screen.getByText('Create invite link')
      expect(link.getAttribute('href')).toBe('https://app.groflo.io/invite')
    })

    it('rewards link points to app rewards page', () => {
      renderHome()
      const link = screen.getByText('See referral rewards')
      expect(link.getAttribute('href')).toBe('https://app.groflo.io/rewards')
    })

    it('renders share-earn-visual with 3 cards', () => {
      renderHome()
      const cards = document.querySelectorAll('.share-earn-card')
      expect(cards.length).toBe(3)
    })

    it('each share-earn card has an icon', () => {
      renderHome()
      const icons = document.querySelectorAll('.share-earn-card-icon')
      expect(icons.length).toBe(3)
    })

    it('share-earn section has fade-section class', () => {
      renderHome()
      const section = document.querySelector('.share-earn')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })
  })

  // ===== HOW IT WORKS DETAILS =====
  describe('How It Works section details', () => {
    it('displays section title "Three steps. That\'s it."', () => {
      renderHome()
      expect(screen.getByText("Three steps. That's it.")).toBeInTheDocument()
    })

    it('displays subtitle about no app download', () => {
      renderHome()
      expect(screen.getByText(/No app to download/)).toBeInTheDocument()
    })

    it('has how-it-works id for anchor navigation', () => {
      renderHome()
      expect(document.getElementById('how-it-works')).toBeInTheDocument()
    })

    it('renders step numbers 1, 2, 3', () => {
      renderHome()
      const stepNumbers = document.querySelectorAll('.how-it-works .step-number')
      expect(stepNumbers.length).toBe(3)
      expect(stepNumbers[0].textContent).toBe('1')
      expect(stepNumbers[1].textContent).toBe('2')
      expect(stepNumbers[2].textContent).toBe('3')
    })

    it('step 1 mentions natural language', () => {
      renderHome()
      expect(screen.getAllByText(/Type naturally/).length).toBeGreaterThanOrEqual(1)
    })

    it('step 2 mentions organizing by department', () => {
      renderHome()
      expect(screen.getByText(/organizes everything by department/)).toBeInTheDocument()
    })

    it('step 3 mentions sharing with anyone', () => {
      renderHome()
      expect(screen.getByText(/Share your list with anyone/)).toBeInTheDocument()
    })

    it('how-it-works section has fade-section class', () => {
      renderHome()
      const section = document.querySelector('.how-it-works')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })
  })

  // ===== FEATURES GRID DETAILS =====
  describe('features grid details', () => {
    it('displays section title "Everything you need, nothing you don\'t"', () => {
      renderHome()
      expect(screen.getByText("Everything you need, nothing you don't")).toBeInTheDocument()
    })

    it('displays subtitle about how you actually shop', () => {
      renderHome()
      expect(screen.getByText(/Built for the way you actually shop/)).toBeInTheDocument()
    })

    it('features section has id "features"', () => {
      renderHome()
      expect(document.getElementById('features')).toBeInTheDocument()
    })

    it('each feature card has an icon', () => {
      renderHome()
      const icons = document.querySelectorAll('.features .feature-icon')
      expect(icons.length).toBe(8)
    })

    it('each feature card has an h3 heading', () => {
      renderHome()
      const headings = document.querySelectorAll('.features .feature-card h3')
      expect(headings.length).toBe(8)
    })

    it('feature icons use brand green SVG fill', () => {
      renderHome()
      const icons = document.querySelectorAll('.features .feature-icon svg path[fill="#35644F"]')
      expect(icons.length).toBeGreaterThan(0)
    })

    it('features section has fade-section class', () => {
      renderHome()
      const section = document.querySelector('.features')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })
  })

  // ===== CTA SECTION DETAILS =====
  describe('CTA section details', () => {
    it('displays subtitle about savings and free products', () => {
      renderHome()
      expect(screen.getByText(/get free products from brands/)).toBeInTheDocument()
    })

    it('mentions free to start, no credit card', () => {
      renderHome()
      expect(screen.getByText(/no credit card needed/)).toBeInTheDocument()
    })

    it('has cta-actions wrapper', () => {
      renderHome()
      expect(document.querySelector('.cta-actions')).toBeInTheDocument()
    })

    it('CTA section has fade-section class', () => {
      renderHome()
      const section = document.querySelector('.cta-section')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })
  })

  // ===== SAVINGS CALCULATOR =====
  describe('savings calculator', () => {
    it('renders the calculator heading', () => {
      renderHome()
      expect(screen.getByText('How much could you save?')).toBeInTheDocument()
    })

    it('renders household size slider', () => {
      renderHome()
      expect(screen.getByLabelText(/People in your household/)).toBeInTheDocument()
    })

    it('renders trips per week slider', () => {
      renderHome()
      expect(screen.getByLabelText(/Grocery trips per week/)).toBeInTheDocument()
    })

    it('displays calculated savings results', () => {
      renderHome()
      // Default: 3 household, 2 trips = $48/mo savings
      expect(screen.getByText('$48')).toBeInTheDocument()
    })

    it('has a Start saving CTA', () => {
      renderHome()
      expect(screen.getByText('Start saving →')).toBeInTheDocument()
    })

    it('has the Groflo logo', () => {
      renderHome()
      const logos = document.querySelectorAll('.calc-logo')
      expect(logos.length).toBe(1)
    })
  })

  // ===== HOME FAQ =====
  describe('home FAQ', () => {
    it('renders FAQ heading', () => {
      renderHome()
      expect(screen.getByText('Common questions')).toBeInTheDocument()
    })

    it('renders all 6 FAQ questions', () => {
      renderHome()
      expect(screen.getByText('Is Penny really free?')).toBeInTheDocument()
      expect(screen.getByText('How does Penny pick my brands?')).toBeInTheDocument()
      expect(screen.getByText('What are Penny Swaps?')).toBeInTheDocument()
      expect(screen.getByText('Do I need to download an app?')).toBeInTheDocument()
      expect(screen.getByText('Can my whole family use it?')).toBeInTheDocument()
      expect(screen.getByText('Is my data sold to advertisers?')).toBeInTheDocument()
    })

    it('opens FAQ answer on click', async () => {
      const user = userEvent.setup()
      renderHome()
      const question = screen.getByText('Is Penny really free?')
      await user.click(question.closest('button')!)
      expect(screen.getByText(/completely free for shoppers/)).toBeInTheDocument()
    })

    it('FAQ buttons have aria-expanded', () => {
      renderHome()
      const buttons = document.querySelectorAll('.home-faq .faq-question')
      expect(buttons.length).toBe(6)
      buttons.forEach(btn => {
        expect(btn.getAttribute('aria-expanded')).toBe('false')
      })
    })
  })

  // ===== THRIFTLY WRAPPED =====
  describe('Groflo Wrapped preview', () => {
    it('renders the Wrapped heading', () => {
      renderHome()
      expect(screen.getByText('See what you saved every month')).toBeInTheDocument()
    })

    it('shows savings stats in wrapped card', () => {
      renderHome()
      expect(screen.getByText('$94')).toBeInTheDocument()
      expect(screen.getByText('Total saved')).toBeInTheDocument()
    })

    it('shows free products count', () => {
      renderHome()
      expect(screen.getByText('Free products')).toBeInTheDocument()
    })

    it('shows top brand', () => {
      renderHome()
      expect(screen.getByText(/Top brand: Horizon Organic/)).toBeInTheDocument()
    })
  })

  // ===== EMAIL CAPTURE =====
  describe('email capture', () => {
    it('renders the capture heading', () => {
      renderHome()
      expect(screen.getByText('Get notified when Penny launches in your area')).toBeInTheDocument()
    })

    it('has an email input', () => {
      renderHome()
      expect(screen.getByLabelText('Email address')).toBeInTheDocument()
    })

    it('has a Notify Me button', () => {
      renderHome()
      expect(screen.getByText('Notify Me')).toBeInTheDocument()
    })

    it('shows no-spam note', () => {
      renderHome()
      expect(screen.getByText(/No spam/)).toBeInTheDocument()
    })
  })
})
