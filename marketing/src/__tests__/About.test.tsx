import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import About from '../pages/About'

function renderAbout() {
  return render(
    <MemoryRouter>
      <About />
    </MemoryRouter>
  )
}

describe('About page', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  // ===== RENDERING =====
  describe('renders without errors', () => {
    it('renders the page without crashing', () => {
      renderAbout()
      expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
    })

    it('sets the document title to include About', () => {
      renderAbout()
      expect(document.title).toContain('About')
    })
  })

  // ===== HERO SECTION =====
  describe('hero section', () => {
    it('displays the About Groflo label', () => {
      renderAbout()
      expect(screen.getByText('About Groflo')).toBeInTheDocument()
    })

    it('displays the hero headline', () => {
      renderAbout()
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
        'Making grocery shopping effortless'
      )
    })

    it('displays the hero subtitle', () => {
      renderAbout()
      expect(
        screen.getByText(/grocery shopping should be simple, smart, and collaborative/)
      ).toBeInTheDocument()
    })

    it('does not use jargon in hero headline', () => {
      renderAbout()
      const h1 = screen.getByRole('heading', { level: 1 })
      expect(h1.textContent).not.toMatch(/\bsynergy\b/i)
      expect(h1.textContent).not.toMatch(/\borchestrate\b/i)
    })
  })

  // ===== STORY SECTION =====
  describe('story section', () => {
    it('displays the Why we built Groflo heading', () => {
      renderAbout()
      expect(screen.getByText('Why we built Groflo')).toBeInTheDocument()
    })

    it('explains the problem (messy lists, duplicates)', () => {
      renderAbout()
      expect(screen.getByText(/Shared lists get messy/)).toBeInTheDocument()
      expect(screen.getByText(/buying duplicates/)).toBeInTheDocument()
    })

    it('mentions Penny as the AI assistant', () => {
      renderAbout()
      expect(screen.getByText(/Penny, our AI assistant/)).toBeInTheDocument()
    })

    it('mentions the marketplace model', () => {
      renderAbout()
      expect(
        screen.getByText(/free trials and targeted swaps/)
      ).toBeInTheDocument()
    })

    it('renders the Web-based stat', () => {
      renderAbout()
      expect(screen.getByText('Web-based')).toBeInTheDocument()
      expect(screen.getByText(/No download needed/)).toBeInTheDocument()
    })

    it('renders the Shared stat', () => {
      renderAbout()
      expect(screen.getByText('Shared')).toBeInTheDocument()
      expect(screen.getByText(/Real-time lists/)).toBeInTheDocument()
    })

    it('renders the savings stat', () => {
      renderAbout()
      expect(screen.getByText('$100/mo')).toBeInTheDocument()
      expect(screen.getByText(/Average savings from free products/)).toBeInTheDocument()
    })
  })

  // ===== VALUES SECTION =====
  describe('values section', () => {
    it('displays the What we believe heading', () => {
      renderAbout()
      expect(screen.getByText('What we believe')).toBeInTheDocument()
    })

    it('renders Simplicity first value', () => {
      renderAbout()
      expect(screen.getByText('Simplicity first')).toBeInTheDocument()
      expect(screen.getByText(/best tools don't need a manual/)).toBeInTheDocument()
    })

    it('renders Built for real life value', () => {
      renderAbout()
      expect(screen.getByText('Built for real life')).toBeInTheDocument()
      expect(screen.getByText(/messy reality of how people actually shop/)).toBeInTheDocument()
    })

    it('renders Helpful, not intrusive value', () => {
      renderAbout()
      expect(screen.getByText('Helpful, not intrusive')).toBeInTheDocument()
      expect(screen.getByText(/Free trials and swaps are always optional/)).toBeInTheDocument()
    })

    it('renders three value cards', () => {
      renderAbout()
      const valueCards = document.querySelectorAll('.value-card')
      expect(valueCards.length).toBe(3)
    })
  })

  // ===== TEAM SECTION =====
  describe('team section', () => {
    it('displays the team heading', () => {
      renderAbout()
      expect(screen.getByText('The team behind Groflo')).toBeInTheDocument()
    })

    it('displays the team description', () => {
      renderAbout()
      expect(
        screen.getByText(/small, focused team of engineers and designers/)
      ).toBeInTheDocument()
    })

    it('displays Founded info', () => {
      renderAbout()
      expect(screen.getByText('Founded')).toBeInTheDocument()
      expect(screen.getByText('2024')).toBeInTheDocument()
    })

    it('displays Mission info', () => {
      renderAbout()
      expect(screen.getByText('Mission')).toBeInTheDocument()
      expect(screen.getByText(/grocery shopping effortless for every household/)).toBeInTheDocument()
    })

    it('displays Approach info', () => {
      renderAbout()
      expect(screen.getByText('Approach')).toBeInTheDocument()
      expect(screen.getByText(/AI-first, privacy-conscious/)).toBeInTheDocument()
    })
  })

  // ===== CONTACT SECTION =====
  describe('contact section', () => {
    it('displays the Get in touch heading', () => {
      renderAbout()
      expect(screen.getByText('Get in touch')).toBeInTheDocument()
    })

    it('displays the contact subtitle', () => {
      renderAbout()
      expect(screen.getByText(/question, partnership idea/)).toBeInTheDocument()
    })

    it('renders the contact form with all fields', () => {
      renderAbout()
      expect(screen.getByLabelText('Name')).toBeInTheDocument()
      expect(screen.getByLabelText('Email')).toBeInTheDocument()
      expect(screen.getByLabelText('Subject')).toBeInTheDocument()
      expect(screen.getByLabelText('Message')).toBeInTheDocument()
    })

    it('renders name input with placeholder', () => {
      renderAbout()
      expect(screen.getByPlaceholderText('Your name')).toBeInTheDocument()
    })

    it('renders email input with placeholder', () => {
      renderAbout()
      expect(screen.getByPlaceholderText('you@company.com')).toBeInTheDocument()
    })

    it('renders message textarea with placeholder', () => {
      renderAbout()
      expect(screen.getByPlaceholderText('How can we help?')).toBeInTheDocument()
    })

    it('renders subject select with all options', () => {
      renderAbout()
      const select = screen.getByLabelText('Subject') as HTMLSelectElement
      const options = Array.from(select.options).map(o => o.textContent)
      expect(options).toContain('General Inquiry')
      expect(options).toContain('Brand Partnerships')
      expect(options).toContain('Press')
      expect(options).toContain('Support')
      expect(options).toContain('Careers')
    })

    it('defaults subject to General Inquiry', () => {
      renderAbout()
      const select = screen.getByLabelText('Subject') as HTMLSelectElement
      expect(select.value).toBe('general')
    })

    it('renders the Send Message button', () => {
      renderAbout()
      expect(screen.getByText(/Send Message/)).toBeInTheDocument()
    })

    it('renders contact email addresses', () => {
      renderAbout()
      expect(screen.getByText('hello@groflo.io')).toBeInTheDocument()
      expect(screen.getByText('brands@groflo.io')).toBeInTheDocument()
      expect(screen.getByText('press@groflo.io')).toBeInTheDocument()
      expect(screen.getByText('support@groflo.io')).toBeInTheDocument()
      expect(screen.getByText('careers@groflo.io')).toBeInTheDocument()
    })

    it('renders contact email links with mailto hrefs', () => {
      renderAbout()
      const helloLink = screen.getByText('hello@groflo.io')
      expect(helloLink).toHaveAttribute('href', 'mailto:hello@groflo.io')

      const brandsLink = screen.getByText('brands@groflo.io')
      expect(brandsLink).toHaveAttribute('href', 'mailto:brands@groflo.io')

      const pressLink = screen.getByText('press@groflo.io')
      expect(pressLink).toHaveAttribute('href', 'mailto:press@groflo.io')

      const supportLink = screen.getByText('support@groflo.io')
      expect(supportLink).toHaveAttribute('href', 'mailto:support@groflo.io')

      const careersLink = screen.getByText('careers@groflo.io')
      expect(careersLink).toHaveAttribute('href', 'mailto:careers@groflo.io')
    })

    it('renders contact info category labels', () => {
      renderAbout()
      expect(screen.getAllByText('General').length).toBeGreaterThan(0)
      expect(screen.getAllByText('Brand Partnerships').length).toBeGreaterThan(0)
      expect(screen.getAllByText('Press').length).toBeGreaterThan(0)
      expect(screen.getAllByText('Support').length).toBeGreaterThan(0)
      expect(screen.getAllByText('Careers').length).toBeGreaterThan(0)
    })
  })

  // ===== CONTACT FORM INTERACTION =====
  describe('contact form interaction', () => {
    it('updates name field on user input', async () => {
      const user = userEvent.setup()
      renderAbout()
      const nameInput = screen.getByLabelText('Name')
      await user.type(nameInput, 'Jane Doe')
      expect(nameInput).toHaveValue('Jane Doe')
    })

    it('updates email field on user input', async () => {
      const user = userEvent.setup()
      renderAbout()
      const emailInput = screen.getByLabelText('Email')
      await user.type(emailInput, 'jane@example.com')
      expect(emailInput).toHaveValue('jane@example.com')
    })

    it('updates subject selection', async () => {
      const user = userEvent.setup()
      renderAbout()
      const select = screen.getByLabelText('Subject')
      await user.selectOptions(select, 'brands')
      expect(select).toHaveValue('brands')
    })

    it('updates message field on user input', async () => {
      const user = userEvent.setup()
      renderAbout()
      const textarea = screen.getByLabelText('Message')
      await user.type(textarea, 'Hello from tests')
      expect(textarea).toHaveValue('Hello from tests')
    })

    it('submits the form and shows success message', async () => {
      const user = userEvent.setup()
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
      renderAbout()

      await user.type(screen.getByLabelText('Name'), 'Jane Doe')
      await user.type(screen.getByLabelText('Email'), 'jane@example.com')
      await user.type(screen.getByLabelText('Message'), 'Hello from tests')
      await user.click(screen.getByText(/Send Message/))

      expect(screen.getByText('Almost there!')).toBeInTheDocument()
      expect(
        screen.getByText(/Your email client should have opened/)
      ).toBeInTheDocument()
      openSpy.mockRestore()
    })

    it('calls window.open with mailto link on submit', async () => {
      const user = userEvent.setup()
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
      renderAbout()

      await user.type(screen.getByLabelText('Name'), 'Jane')
      await user.type(screen.getByLabelText('Email'), 'jane@test.com')
      await user.type(screen.getByLabelText('Message'), 'Test message')
      await user.click(screen.getByText(/Send Message/))

      expect(openSpy).toHaveBeenCalledOnce()
      const mailtoUrl = openSpy.mock.calls[0][0] as string
      expect(mailtoUrl).toContain('mailto:hello@groflo.io')
      expect(mailtoUrl).toContain('General%20Inquiry')
      openSpy.mockRestore()
    })

    it('uses correct subject for Brand Partnerships', async () => {
      const user = userEvent.setup()
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
      renderAbout()

      await user.type(screen.getByLabelText('Name'), 'Jane')
      await user.type(screen.getByLabelText('Email'), 'jane@test.com')
      await user.selectOptions(screen.getByLabelText('Subject'), 'brands')
      await user.type(screen.getByLabelText('Message'), 'Partner inquiry')
      await user.click(screen.getByText(/Send Message/))

      const mailtoUrl = openSpy.mock.calls[0][0] as string
      expect(mailtoUrl).toContain('Brand%20Partnership%20Inquiry')
      openSpy.mockRestore()
    })

    it('uses correct subject for Press', async () => {
      const user = userEvent.setup()
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
      renderAbout()

      await user.type(screen.getByLabelText('Name'), 'Jane')
      await user.type(screen.getByLabelText('Email'), 'jane@test.com')
      await user.selectOptions(screen.getByLabelText('Subject'), 'press')
      await user.type(screen.getByLabelText('Message'), 'Press inquiry')
      await user.click(screen.getByText(/Send Message/))

      const mailtoUrl = openSpy.mock.calls[0][0] as string
      expect(mailtoUrl).toContain('Press%20Inquiry')
      openSpy.mockRestore()
    })

    it('uses correct subject for Support', async () => {
      const user = userEvent.setup()
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
      renderAbout()

      await user.type(screen.getByLabelText('Name'), 'Jane')
      await user.type(screen.getByLabelText('Email'), 'jane@test.com')
      await user.selectOptions(screen.getByLabelText('Subject'), 'support')
      await user.type(screen.getByLabelText('Message'), 'Support request')
      await user.click(screen.getByText(/Send Message/))

      const mailtoUrl = openSpy.mock.calls[0][0] as string
      expect(mailtoUrl).toContain('Support%20Request')
      openSpy.mockRestore()
    })

    it('uses correct subject for Careers', async () => {
      const user = userEvent.setup()
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
      renderAbout()

      await user.type(screen.getByLabelText('Name'), 'Jane')
      await user.type(screen.getByLabelText('Email'), 'jane@test.com')
      await user.selectOptions(screen.getByLabelText('Subject'), 'careers')
      await user.type(screen.getByLabelText('Message'), 'Careers inquiry')
      await user.click(screen.getByText(/Send Message/))

      const mailtoUrl = openSpy.mock.calls[0][0] as string
      expect(mailtoUrl).toContain('Careers%20Inquiry')
      openSpy.mockRestore()
    })

    it('shows Send another message button after submission', async () => {
      const user = userEvent.setup()
      vi.spyOn(window, 'open').mockImplementation(() => null)
      renderAbout()

      await user.type(screen.getByLabelText('Name'), 'Jane')
      await user.type(screen.getByLabelText('Email'), 'jane@test.com')
      await user.type(screen.getByLabelText('Message'), 'Hello')
      await user.click(screen.getByText(/Send Message/))

      expect(screen.getByText('Send another message')).toBeInTheDocument()
    })

    it('resets to form when clicking Send another message', async () => {
      const user = userEvent.setup()
      vi.spyOn(window, 'open').mockImplementation(() => null)
      renderAbout()

      await user.type(screen.getByLabelText('Name'), 'Jane')
      await user.type(screen.getByLabelText('Email'), 'jane@test.com')
      await user.type(screen.getByLabelText('Message'), 'Hello')
      await user.click(screen.getByText(/Send Message/))

      await user.click(screen.getByText('Send another message'))
      expect(screen.getByLabelText('Name')).toBeInTheDocument()
      expect(screen.getByText(/Send Message/)).toBeInTheDocument()
    })

    it('success message includes fallback email link', async () => {
      const user = userEvent.setup()
      vi.spyOn(window, 'open').mockImplementation(() => null)
      renderAbout()

      await user.type(screen.getByLabelText('Name'), 'Jane')
      await user.type(screen.getByLabelText('Email'), 'jane@test.com')
      await user.type(screen.getByLabelText('Message'), 'Hello')
      await user.click(screen.getByText(/Send Message/))

      const fallbackLink = screen.getByText('hello@groflo.io', { selector: '.contact-success a' })
      expect(fallbackLink).toHaveAttribute('href', 'mailto:hello@groflo.io')
    })
  })

  // ===== BRAND CONSISTENCY =====
  describe('brand consistency', () => {
    it('does not use corporate jargon in headings', () => {
      renderAbout()
      const headings = screen.getAllByRole('heading')
      const jargonTerms = [
        /\bsynergy\b/i, /\bparadigm\b/i, /\bdisrupt\b/i,
        /\bleverage\b/i, /\bholistic\b/i, /\bscalable\b/i,
      ]
      headings.forEach(heading => {
        jargonTerms.forEach(term => {
          expect(heading.textContent).not.toMatch(term)
        })
      })
    })

    it('uses warm, family-oriented language', () => {
      renderAbout()
      expect(screen.getAllByText(/families everywhere/).length).toBeGreaterThan(0)
      expect(screen.getAllByText(/household/).length).toBeGreaterThan(0)
    })
  })

  // ===== FADE ANIMATIONS =====
  describe('fade-in animation sections', () => {
    it('applies fade-section class to story section', () => {
      renderAbout()
      const section = document.querySelector('.about-story')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })

    it('applies fade-section class to values section', () => {
      renderAbout()
      const section = document.querySelector('.about-values')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })

    it('applies fade-section class to team section', () => {
      renderAbout()
      const section = document.querySelector('.about-team')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })

    it('applies fade-section class to contact section', () => {
      renderAbout()
      const section = document.querySelector('.about-contact')
      expect(section?.classList.contains('fade-section')).toBe(true)
    })
  })

  // ===== PAGE STRUCTURE =====
  describe('page structure', () => {
    it('renders multiple sections', () => {
      renderAbout()
      const sections = document.querySelectorAll('section')
      expect(sections.length).toBeGreaterThanOrEqual(4)
    })

    it('contains an about-hero section', () => {
      renderAbout()
      expect(document.querySelector('.about-hero')).toBeInTheDocument()
    })

    it('wraps content in section-inner containers', () => {
      renderAbout()
      const innerContainers = document.querySelectorAll('.section-inner')
      expect(innerContainers.length).toBeGreaterThanOrEqual(4)
    })

    it('renders exactly 5 sections', () => {
      renderAbout()
      const sections = document.querySelectorAll('section')
      expect(sections.length).toBe(5)
    })
  })

  // ===== ACCESSIBILITY =====
  describe('accessibility', () => {
    it('has exactly one h1 element', () => {
      renderAbout()
      const h1s = screen.getAllByRole('heading', { level: 1 })
      expect(h1s.length).toBe(1)
    })

    it('form inputs have associated labels via htmlFor', () => {
      renderAbout()
      const labels = document.querySelectorAll('label.form-label')
      labels.forEach(label => {
        const forAttr = label.getAttribute('for')
        if (forAttr) {
          const input = document.getElementById(forAttr)
          expect(input).toBeInTheDocument()
        }
      })
    })

    it('name input has id matching label for attribute', () => {
      renderAbout()
      const label = document.querySelector('label[for="name"]')
      expect(label).toBeInTheDocument()
      expect(document.getElementById('name')).toBeInTheDocument()
    })

    it('email input has id matching label for attribute', () => {
      renderAbout()
      const label = document.querySelector('label[for="email"]')
      expect(label).toBeInTheDocument()
      expect(document.getElementById('email')).toBeInTheDocument()
    })

    it('subject select has id matching label for attribute', () => {
      renderAbout()
      const label = document.querySelector('label[for="subject"]')
      expect(label).toBeInTheDocument()
      expect(document.getElementById('subject')).toBeInTheDocument()
    })

    it('message textarea has id matching label for attribute', () => {
      renderAbout()
      const label = document.querySelector('label[for="message"]')
      expect(label).toBeInTheDocument()
      expect(document.getElementById('message')).toBeInTheDocument()
    })

    it('required fields have required attribute', () => {
      renderAbout()
      expect((screen.getByLabelText('Name') as HTMLInputElement).required).toBe(true)
      expect((screen.getByLabelText('Email') as HTMLInputElement).required).toBe(true)
      expect((screen.getByLabelText('Message') as HTMLTextAreaElement).required).toBe(true)
    })

    it('email input has type email for validation', () => {
      renderAbout()
      const emailInput = screen.getByLabelText('Email') as HTMLInputElement
      expect(emailInput.type).toBe('email')
    })
  })

  // ===== SECTION ORDERING =====
  describe('section ordering', () => {
    it('hero comes before story', () => {
      renderAbout()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const heroIdx = classNames.indexOf('about-hero')
      const storyIdx = classNames.findIndex(c => c === 'about-story')
      expect(heroIdx).toBeLessThan(storyIdx)
    })

    it('story comes before values', () => {
      renderAbout()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const storyIdx = classNames.findIndex(c => c === 'about-story')
      const valuesIdx = classNames.findIndex(c => c === 'about-values')
      expect(storyIdx).toBeLessThan(valuesIdx)
    })

    it('values comes before team', () => {
      renderAbout()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const valuesIdx = classNames.findIndex(c => c === 'about-values')
      const teamIdx = classNames.findIndex(c => c === 'about-team')
      expect(valuesIdx).toBeLessThan(teamIdx)
    })

    it('team comes before contact', () => {
      renderAbout()
      const sections = document.querySelectorAll('section')
      const classNames = Array.from(sections).map(s => s.className.split(' ')[0])
      const teamIdx = classNames.findIndex(c => c === 'about-team')
      const contactIdx = classNames.findIndex(c => c === 'about-contact')
      expect(teamIdx).toBeLessThan(contactIdx)
    })
  })

  // ===== CONTACT INFO STRUCTURE =====
  describe('contact info structure', () => {
    it('renders 5 contact info items', () => {
      renderAbout()
      const items = document.querySelectorAll('.contact-info-item')
      expect(items.length).toBe(5)
    })

    it('all contact info items have mailto links', () => {
      renderAbout()
      const items = document.querySelectorAll('.contact-info-item a')
      items.forEach(link => {
        expect(link.getAttribute('href')).toMatch(/^mailto:/)
      })
    })

    it('contact form uses form-group wrappers', () => {
      renderAbout()
      const groups = document.querySelectorAll('.contact-form .form-group')
      expect(groups.length).toBe(4)
    })

    it('submit button spans full width', () => {
      renderAbout()
      const submitBtn = screen.getByText(/Send Message/)
      expect(submitBtn.style.width).toBe('100%')
    })
  })

  // ===== STAT CARDS =====
  describe('stat cards in story section', () => {
    it('renders 3 story stats', () => {
      renderAbout()
      const stats = document.querySelectorAll('.story-stat')
      expect(stats.length).toBe(3)
    })

    it('each stat has a number and label', () => {
      renderAbout()
      const statNumbers = document.querySelectorAll('.story-stat-number')
      const statLabels = document.querySelectorAll('.story-stat-label')
      expect(statNumbers.length).toBe(3)
      expect(statLabels.length).toBe(3)
    })
  })

  // ===== TEAM DETAILS =====
  describe('team details', () => {
    it('renders 3 team detail items', () => {
      renderAbout()
      const items = document.querySelectorAll('.team-detail-item')
      expect(items.length).toBe(3)
    })

    it('each team detail has an h4 heading', () => {
      renderAbout()
      const headings = document.querySelectorAll('.team-detail-item h4')
      expect(headings.length).toBe(3)
    })
  })
})
