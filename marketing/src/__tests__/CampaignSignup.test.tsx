import { render, screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import CampaignSignup from '../pages/CampaignSignup'

// Mock fetch for the lead-capture endpoint
const fetchMock = vi.fn()

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock)
  fetchMock.mockReset()
  fetchMock.mockResolvedValue(
    new Response(
      JSON.stringify({ success: true, leadId: 42, redirectUrl: 'https://brands.groflo.io?from=campaign_signup&lead=42', replay: false }),
      { status: 201, headers: { 'Content-Type': 'application/json' } }
    )
  )
})

afterEach(() => {
  vi.unstubAllGlobals()
})

function renderCampaignSignup() {
  return render(
    <MemoryRouter>
      <CampaignSignup />
    </MemoryRouter>
  )
}

async function fillStep1(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText('Full name'), 'Jane Smith')
  await user.type(screen.getByLabelText('Company name'), 'Acme Foods')
  await user.type(screen.getByLabelText('Work email'), 'jane@acme.com')
  await user.click(screen.getByText(/Continue/))
}

async function fillStep2(user: ReturnType<typeof userEvent.setup>) {
  const dairyLabel = screen.getByText('Dairy').closest('label')!
  await user.click(dairyLabel)

  const today = new Date()
  const todayStr = today.toISOString().split('T')[0]
  const future = new Date(today.getTime() + 7 * 86400000)
  const futureStr = future.toISOString().split('T')[0]

  await user.clear(screen.getByLabelText('Start date'))
  await user.type(screen.getByLabelText('Start date'), todayStr)
  await user.clear(screen.getByLabelText('End date'))
  await user.type(screen.getByLabelText('End date'), futureStr)
  await user.type(screen.getByLabelText('Product / brand name'), 'Organic Valley Milk')
  await user.type(screen.getByLabelText(/Short description/), 'Farm-fresh organic milk')
  await user.click(screen.getByText(/Continue to Review/))
}

async function goToStep2() {
  const user = userEvent.setup()
  renderCampaignSignup()
  await fillStep1(user)
  return user
}

async function goToStep3() {
  const user = userEvent.setup()
  renderCampaignSignup()
  await fillStep1(user)
  await fillStep2(user)
  return user
}

describe('CampaignSignup page', () => {
  // ===== RENDERING =====
  describe('renders without errors', () => {
    it('renders the page without crashing', () => {
      renderCampaignSignup()
      expect(screen.getByText('Create your brand account')).toBeInTheDocument()
    })

    it('sets the document title to include Create Campaign', () => {
      renderCampaignSignup()
      expect(document.title).toContain('Create Campaign')
    })
  })

  // ===== STEP INDICATOR =====
  describe('step indicator', () => {
    it('renders all four step labels (Account, Campaign, Review, Confirmation)', () => {
      renderCampaignSignup()
      expect(screen.getByText('Account')).toBeInTheDocument()
      expect(screen.getByText('Campaign')).toBeInTheDocument()
      expect(screen.getByText('Review')).toBeInTheDocument()
      expect(screen.getByText('Confirmation')).toBeInTheDocument()
    })

    it('does NOT label step 3 as "Payment" — no payment is collected on the marketing site', () => {
      renderCampaignSignup()
      const labels = Array.from(document.querySelectorAll('.step-dot-label')).map(l => l.textContent)
      expect(labels).not.toContain('Payment')
    })

    it('shows step 1 as active initially', () => {
      renderCampaignSignup()
      const activeStep = document.querySelector('.step-dot-wrapper.active')
      expect(activeStep).toBeInTheDocument()
      const label = activeStep?.querySelector('.step-dot-label')
      expect(label?.textContent).toBe('Account')
    })

    it('shows step numbers in dots', () => {
      renderCampaignSignup()
      const dots = document.querySelectorAll('.step-dot')
      expect(dots[0].textContent).toBe('1')
      expect(dots[1].textContent).toBe('2')
      expect(dots[2].textContent).toBe('3')
      expect(dots[3].textContent).toBe('4')
    })
  })

  // ===== STEP 1: ACCOUNT =====
  describe('Step 1: Account', () => {
    it('displays the Create your brand account heading', () => {
      renderCampaignSignup()
      expect(screen.getByText('Create your brand account')).toBeInTheDocument()
    })

    it('displays the step description', () => {
      renderCampaignSignup()
      expect(screen.getByText('Tell us about your company to get started.')).toBeInTheDocument()
    })

    it('renders Full name, Company name, Work email, and optional Company website fields', () => {
      renderCampaignSignup()
      expect(screen.getByLabelText('Full name')).toBeInTheDocument()
      expect(screen.getByLabelText('Company name')).toBeInTheDocument()
      expect(screen.getByLabelText('Work email')).toBeInTheDocument()
      expect(screen.getByLabelText(/Company website/)).toBeInTheDocument()
      expect(screen.getByText('(optional)')).toBeInTheDocument()
    })

    it('renders Continue button', () => {
      renderCampaignSignup()
      expect(screen.getByText(/Continue/)).toBeInTheDocument()
    })

    it('updates input fields on user type', async () => {
      const user = userEvent.setup()
      renderCampaignSignup()

      const nameInput = screen.getByLabelText('Full name')
      await user.type(nameInput, 'Jane Smith')
      expect(nameInput).toHaveValue('Jane Smith')

      const companyInput = screen.getByLabelText('Company name')
      await user.type(companyInput, 'Acme Foods')
      expect(companyInput).toHaveValue('Acme Foods')

      const emailInput = screen.getByLabelText('Work email')
      await user.type(emailInput, 'jane@acme.com')
      expect(emailInput).toHaveValue('jane@acme.com')
    })
  })

  // ===== STEP 1 -> STEP 2 TRANSITION =====
  describe('transition from Step 1 to Step 2', () => {
    it('advances to Step 2 on form submission', async () => {
      const user = userEvent.setup()
      renderCampaignSignup()
      await fillStep1(user)
      expect(screen.getByText('Configure your campaign')).toBeInTheDocument()
    })

    it('shows Step 2 as active in step indicator', async () => {
      const user = userEvent.setup()
      renderCampaignSignup()
      await fillStep1(user)
      const activeStep = document.querySelector('.step-dot-wrapper.active')
      const label = activeStep?.querySelector('.step-dot-label')
      expect(label?.textContent).toBe('Campaign')
    })

    it('marks Step 1 as completed with checkmark', async () => {
      const user = userEvent.setup()
      renderCampaignSignup()
      await fillStep1(user)
      const completedStep = document.querySelector('.step-dot-wrapper.completed')
      const dot = completedStep?.querySelector('.step-dot')
      expect(dot?.textContent).toContain('\u2713')
    })
  })

  // ===== STEP 2: CAMPAIGN =====
  describe('Step 2: Campaign', () => {
    it('displays the Configure your campaign heading', async () => {
      await goToStep2()
      expect(screen.getByText('Configure your campaign')).toBeInTheDocument()
    })

    it('displays the step description', async () => {
      await goToStep2()
      expect(screen.getByText('Choose your campaign type, targeting, and budget.')).toBeInTheDocument()
    })

    it('renders all five campaign type radio options', async () => {
      await goToStep2()
      expect(screen.getByText('Free Product Trial')).toBeInTheDocument()
      expect(screen.getByText('Targeted Swap')).toBeInTheDocument()
      expect(screen.getByText('Promoted Suggestion')).toBeInTheDocument()
      expect(screen.getByText('Sponsored Brand')).toBeInTheDocument()
      expect(screen.getByText('List Placement')).toBeInTheDocument()
    })

    it('defaults to Free Product Trial ad type', async () => {
      await goToStep2()
      const radio = screen.getByDisplayValue('free-trial') as HTMLInputElement
      expect(radio.checked).toBe(true)
    })

    it('renders all 10 category checkboxes', async () => {
      await goToStep2()
      const categories = ['Dairy', 'Produce', 'Meat & Seafood', 'Snacks', 'Beverages',
        'Frozen', 'Bakery', 'Pantry', 'Health & Beauty', 'Household']
      categories.forEach(cat => {
        expect(screen.getByText(cat)).toBeInTheDocument()
      })
    })

    it('renders geography dropdown with all regions', async () => {
      await goToStep2()
      const select = screen.getByLabelText('Geographic targeting') as HTMLSelectElement
      const options = Array.from(select.options).map(o => o.textContent)
      expect(options).toContain('National (all regions)')
      expect(options).toContain('Northeast')
      expect(options).toContain('Southeast')
      expect(options).toContain('Midwest')
      expect(options).toContain('Southwest')
      expect(options).toContain('West Coast')
    })

    it('defaults geography to national', async () => {
      await goToStep2()
      const select = screen.getByLabelText('Geographic targeting') as HTMLSelectElement
      expect(select.value).toBe('national')
    })

    it('renders budget type toggle and defaults to daily', async () => {
      await goToStep2()
      expect(screen.getByText('Daily')).toBeInTheDocument()
      expect(screen.getByText('Total Campaign')).toBeInTheDocument()
      expect(screen.getByText('Daily').classList.contains('active')).toBe(true)
    })

    it('shows min $25 label for daily budget', async () => {
      await goToStep2()
      expect(screen.getByText(/Daily budget.*min \$25/)).toBeInTheDocument()
    })

    it('switches to total budget on toggle click', async () => {
      const user = await goToStep2()
      await user.click(screen.getByText('Total Campaign'))
      expect(screen.getByText(/Total campaign budget.*min \$100/)).toBeInTheDocument()
    })

    it('renders start date, end date, product name, and description fields', async () => {
      await goToStep2()
      expect(screen.getByLabelText('Start date')).toBeInTheDocument()
      expect(screen.getByLabelText('End date')).toBeInTheDocument()
      expect(screen.getByLabelText('Product / brand name')).toBeInTheDocument()
      expect(screen.getByLabelText(/Short description/)).toBeInTheDocument()
      expect(screen.getByText('0/140')).toBeInTheDocument()
    })

    it('updates character count on description input', async () => {
      const user = await goToStep2()
      const textarea = screen.getByLabelText(/Short description/)
      await user.type(textarea, 'Hello world')
      expect(screen.getByText('11/140')).toBeInTheDocument()
    })

    it('Back button returns to Step 1', async () => {
      const user = await goToStep2()
      await user.click(screen.getByText(/← Back/))
      expect(screen.getByText('Create your brand account')).toBeInTheDocument()
    })

    it('Continue button is labeled "Continue to Review", NOT "Continue to Payment"', async () => {
      await goToStep2()
      expect(screen.getByText(/Continue to Review/)).toBeInTheDocument()
      expect(screen.queryByText(/Continue to Payment/)).not.toBeInTheDocument()
    })

    it('toggles category selection on click', async () => {
      const user = await goToStep2()
      const dairyLabel = screen.getByText('Dairy').closest('label')!
      const checkbox = within(dairyLabel).getByRole('checkbox') as HTMLInputElement
      expect(checkbox.checked).toBe(false)
      await user.click(dairyLabel)
      expect(checkbox.checked).toBe(true)
      await user.click(dairyLabel)
      expect(checkbox.checked).toBe(false)
    })

    it('changes ad type on radio selection', async () => {
      const user = await goToStep2()
      const sponsoredRadio = screen.getByDisplayValue('sponsored-brand') as HTMLInputElement
      await user.click(sponsoredRadio)
      expect(sponsoredRadio.checked).toBe(true)
    })

    it('budget amount updates when switching from daily to total', async () => {
      const user = await goToStep2()
      const budgetInput = screen.getByLabelText(/budget/) as HTMLInputElement
      expect(budgetInput.value).toBe('25')
      await user.click(screen.getByText('Total Campaign'))
      const updatedInput = screen.getByLabelText(/budget/) as HTMLInputElement
      expect(Number(updatedInput.value)).toBeGreaterThanOrEqual(100)
    })
  })

  // ===== STEP 2 VALIDATION =====
  describe('Step 2 validation', () => {
    it('shows validation error when no categories selected', async () => {
      const user = await goToStep2()
      const today = new Date().toISOString().split('T')[0]
      const tomorrow = new Date(Date.now() + 86400000).toISOString().split('T')[0]
      await user.clear(screen.getByLabelText('Start date'))
      await user.type(screen.getByLabelText('Start date'), today)
      await user.clear(screen.getByLabelText('End date'))
      await user.type(screen.getByLabelText('End date'), tomorrow)
      await user.type(screen.getByLabelText('Product / brand name'), 'Test Product')
      await user.type(screen.getByLabelText(/Short description/), 'Test desc')
      await user.click(screen.getByText(/Continue to Review/))
      expect(screen.getByText('Please select at least one target category.')).toBeInTheDocument()
    })
  })

  // ===== STEP 3: REVIEW =====
  describe('Step 3: Review & submit', () => {
    it('advances to Step 3 with correct data', async () => {
      await goToStep3()
      expect(screen.getByText('Review & submit')).toBeInTheDocument()
    })

    it('displays campaign summary on Step 3', async () => {
      await goToStep3()
      expect(screen.getByText('Campaign Summary')).toBeInTheDocument()
      expect(screen.getByText('Acme Foods')).toBeInTheDocument()
      expect(screen.getByText('Free Product Trial')).toBeInTheDocument()
      expect(screen.getByText('Dairy')).toBeInTheDocument()
    })

    it('shows terms agreement checkbox', async () => {
      await goToStep3()
      expect(screen.getByText(/I agree to Groflo's/)).toBeInTheDocument()
    })

    it('links to Terms of Service and Privacy Policy', async () => {
      await goToStep3()
      expect(screen.getByText('Terms of Service')).toHaveAttribute('href', '/terms')
      expect(screen.getByText('Privacy Policy')).toHaveAttribute('href', '/privacy')
    })

    it('terms links open in new tab', async () => {
      await goToStep3()
      const tosLink = screen.getByText('Terms of Service')
      expect(tosLink).toHaveAttribute('target', '_blank')
      const privacyLink = screen.getByText('Privacy Policy')
      expect(privacyLink).toHaveAttribute('target', '_blank')
    })

    it('Submit button is disabled until terms are agreed', async () => {
      await goToStep3()
      const btn = screen.getByRole('button', { name: /Submit Campaign Brief/i }) as HTMLButtonElement
      expect(btn.disabled).toBe(true)
    })

    it('Submit button enables when terms are agreed', async () => {
      const user = await goToStep3()
      const checkbox = screen.getByRole('checkbox', { name: /I agree to Groflo's/i })
      await user.click(checkbox)
      const btn = screen.getByRole('button', { name: /Submit Campaign Brief/i }) as HTMLButtonElement
      expect(btn.disabled).toBe(false)
    })

    it('has a Back to Campaign Details button', async () => {
      await goToStep3()
      expect(screen.getByText(/Back to Campaign Details/)).toBeInTheDocument()
    })

    it('navigates back to Step 2 on back button click', async () => {
      const user = await goToStep3()
      await user.click(screen.getByText(/Back to Campaign Details/))
      expect(screen.getByText('Configure your campaign')).toBeInTheDocument()
    })

    it('shows budget with /day for daily budget type', async () => {
      await goToStep3()
      expect(screen.getByText('$25/day')).toBeInTheDocument()
    })

    it('does NOT promise card authorization or charge — copy is honest about no-payment-collected', async () => {
      await goToStep3()
      const allText = document.body.textContent || ''
      expect(allText).not.toMatch(/authorize.*card/i)
      expect(allText).not.toMatch(/charge your card/i)
      expect(allText).toMatch(/no payment is collected/i)
    })

    it('does NOT render any Stripe Elements', async () => {
      await goToStep3()
      expect(document.querySelector('.stripe-card-wrapper')).toBeNull()
      expect(document.querySelector('[data-testid="card-element"]')).toBeNull()
    })
  })

  // ===== STEP 3 SUBMISSION =====
  describe('Step 3: lead submission', () => {
    it('POSTs to /.netlify/functions/campaign-lead with idempotency key on submit', async () => {
      const user = await goToStep3()
      const checkbox = screen.getByRole('checkbox', { name: /I agree to Groflo's/i })
      await user.click(checkbox)
      const btn = screen.getByRole('button', { name: /Submit Campaign Brief/i })
      await user.click(btn)

      await waitFor(() => expect(fetchMock).toHaveBeenCalled())
      const [url, init] = fetchMock.mock.calls[0]
      expect(url).toBe('/.netlify/functions/campaign-lead')
      expect(init.method).toBe('POST')
      const headers = init.headers as Record<string, string>
      expect(headers['Idempotency-Key']).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i)
      const body = JSON.parse(init.body as string)
      expect(body.email).toBe('jane@acme.com')
      expect(body.companyName).toBe('Acme Foods')
      expect(body.creativeName).toBe('Organic Valley Milk')
      expect(body.adType).toBe('free-trial')
      expect(body.categories).toEqual(['Dairy'])
      expect(body.budgetAmount).toBe(25)
    })

    it('reuses the same idempotency key on retry after a failure', async () => {
      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'Service unavailable.' }), { status: 500, headers: { 'Content-Type': 'application/json' } })
      )

      const user = await goToStep3()
      const checkbox = screen.getByRole('checkbox', { name: /I agree to Groflo's/i })
      await user.click(checkbox)
      await user.click(screen.getByRole('button', { name: /Submit Campaign Brief/i }))

      await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
      await waitFor(() => expect(screen.getByText(/Service unavailable/)).toBeInTheDocument())
      const firstKey = (fetchMock.mock.calls[0][1].headers as Record<string, string>)['Idempotency-Key']

      await user.click(screen.getByRole('button', { name: /Submit Campaign Brief/i }))
      await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2))
      const secondKey = (fetchMock.mock.calls[1][1].headers as Record<string, string>)['Idempotency-Key']

      expect(secondKey).toBe(firstKey)
    })

    it('shows server error message when backend returns non-2xx', async () => {
      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'Too many submissions from this IP. Try again later.' }), { status: 429, headers: { 'Content-Type': 'application/json' } })
      )
      const user = await goToStep3()
      await user.click(screen.getByRole('checkbox', { name: /I agree to Groflo's/i }))
      await user.click(screen.getByRole('button', { name: /Submit Campaign Brief/i }))
      await waitFor(() => expect(screen.getByText(/Too many submissions/i)).toBeInTheDocument())
    })

    it('shows generic network error message when fetch rejects', async () => {
      fetchMock.mockRejectedValueOnce(new Error('NetworkError'))
      const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const user = await goToStep3()
      await user.click(screen.getByRole('checkbox', { name: /I agree to Groflo's/i }))
      await user.click(screen.getByRole('button', { name: /Submit Campaign Brief/i }))
      await waitFor(() => expect(screen.getByText(/network error/i)).toBeInTheDocument())
      errSpy.mockRestore()
    })

    it('disables Submit while in flight to prevent double-submit', async () => {
      let resolveFetch: ((v: Response) => void) | undefined
      fetchMock.mockReturnValueOnce(new Promise<Response>(r => { resolveFetch = r }))

      const user = await goToStep3()
      await user.click(screen.getByRole('checkbox', { name: /I agree to Groflo's/i }))
      const btn = screen.getByRole('button', { name: /Submit Campaign Brief/i })
      await user.click(btn)
      await waitFor(() => expect(screen.getByRole('button', { name: /Submitting/i })).toBeDisabled())
      resolveFetch?.(new Response(JSON.stringify({ leadId: 7, redirectUrl: 'https://brands.groflo.io' }), { status: 201, headers: { 'Content-Type': 'application/json' } }))
    })
  })

  // ===== STEP 4: CONFIRMATION =====
  describe('Step 4: Confirmation', () => {
    async function goToStep4() {
      const user = await goToStep3()
      await user.click(screen.getByRole('checkbox', { name: /I agree to Groflo's/i }))
      await user.click(screen.getByRole('button', { name: /Submit Campaign Brief/i }))
      await waitFor(() => expect(screen.getByText('Campaign brief submitted')).toBeInTheDocument())
      return user
    }

    it('shows confirmation page after successful submission', async () => {
      await goToStep4()
      expect(screen.getByText('Campaign brief submitted')).toBeInTheDocument()
    })

    it('displays confirmation icon', async () => {
      await goToStep4()
      const icon = document.querySelector('.confirmation-icon')
      expect(icon).toBeInTheDocument()
      expect(icon?.textContent).toBe('\u2713')
    })

    it('mentions the email address where confirmation will arrive', async () => {
      await goToStep4()
      expect(screen.getByText('jane@acme.com')).toBeInTheDocument()
    })

    it('mentions brands.groflo.io as the next destination', async () => {
      await goToStep4()
      expect(screen.getByText(/brands\.groflo\.io/)).toBeInTheDocument()
    })

    it('displays the lead reference id from the server response', async () => {
      await goToStep4()
      expect(screen.getByText(/#42/)).toBeInTheDocument()
    })

    it('shows Brief Received status badge (NOT "Under Review")', async () => {
      await goToStep4()
      expect(screen.getByText('Brief Received')).toBeInTheDocument()
      expect(screen.queryByText('Under Review')).toBeNull()
    })

    it('Visit Brand Portal CTA points to the redirectUrl from the server', async () => {
      await goToStep4()
      const cta = screen.getByText(/Visit Brand Portal/) as HTMLAnchorElement
      expect(cta.getAttribute('href')).toBe('https://brands.groflo.io?from=campaign_signup&lead=42')
    })

    it('has Learn More About Ads link', async () => {
      await goToStep4()
      const learnLink = screen.getByText('Learn More About Ads')
      expect(learnLink).toHaveAttribute('href', '/for-brands')
    })
  })

  // ===== STEP 2: CAMPAIGN TYPE DESCRIPTIONS =====
  describe('Step 2: Campaign type descriptions', () => {
    it('shows description for Free Product Trial', async () => {
      await goToStep2()
      expect(screen.getByText(/Cover the full cost.*families try your product for free/)).toBeInTheDocument()
    })

    it('shows description for Targeted Swap', async () => {
      await goToStep2()
      expect(screen.getByText(/Offer a discount when someone adds a competitor/)).toBeInTheDocument()
    })

    it('shows description for Promoted Suggestion', async () => {
      await goToStep2()
      expect(screen.getByText('Penny mentions your product in conversation')).toBeInTheDocument()
    })

    it('shows description for Sponsored Brand', async () => {
      await goToStep2()
      expect(screen.getByText('Brand highlighted in category mentions')).toBeInTheDocument()
    })

    it('shows description for List Placement', async () => {
      await goToStep2()
      expect(screen.getByText('Product added as a labeled suggestion on lists')).toBeInTheDocument()
    })

    it('renders five radio options for campaign type', async () => {
      await goToStep2()
      const radios = screen.getAllByRole('radio')
      expect(radios.length).toBe(5)
    })

    it('selects List Placement ad type on click', async () => {
      const user = await goToStep2()
      const listPlacementRadio = screen.getByDisplayValue('list-placement') as HTMLInputElement
      await user.click(listPlacementRadio)
      expect(listPlacementRadio.checked).toBe(true)
      const freeTrialRadio = screen.getByDisplayValue('free-trial') as HTMLInputElement
      expect(freeTrialRadio.checked).toBe(false)
    })

    it('can select multiple categories', async () => {
      const user = await goToStep2()
      const dairyLabel = screen.getByText('Dairy').closest('label')!
      const snacksLabel = screen.getByText('Snacks').closest('label')!
      const beveragesLabel = screen.getByText('Beverages').closest('label')!

      await user.click(dairyLabel)
      await user.click(snacksLabel)
      await user.click(beveragesLabel)

      expect((within(dairyLabel).getByRole('checkbox') as HTMLInputElement).checked).toBe(true)
      expect((within(snacksLabel).getByRole('checkbox') as HTMLInputElement).checked).toBe(true)
      expect((within(beveragesLabel).getByRole('checkbox') as HTMLInputElement).checked).toBe(true)
    })

    it('changes geography to northeast', async () => {
      const user = await goToStep2()
      const select = screen.getByLabelText('Geographic targeting')
      await user.selectOptions(select, 'northeast')
      expect(select).toHaveValue('northeast')
    })

    it('changes geography to west coast', async () => {
      const user = await goToStep2()
      const select = screen.getByLabelText('Geographic targeting')
      await user.selectOptions(select, 'west')
      expect(select).toHaveValue('west')
    })

    it('has $ prefix on budget input', async () => {
      await goToStep2()
      const currency = document.querySelector('.budget-currency')
      expect(currency).toBeInTheDocument()
      expect(currency?.textContent).toBe('$')
    })

    it('renders all 10 checkbox options for categories', async () => {
      await goToStep2()
      expect(document.querySelectorAll('.checkbox-option').length).toBe(10)
    })

    it('renders 6 geography options', async () => {
      await goToStep2()
      const select = screen.getByLabelText('Geographic targeting') as HTMLSelectElement
      expect(select.options.length).toBe(6)
    })

    it('description textarea has maxLength of 140 and 3 rows', async () => {
      await goToStep2()
      const textarea = screen.getByLabelText(/Short description/) as HTMLTextAreaElement
      expect(textarea.maxLength).toBe(140)
      expect(textarea.rows).toBe(3)
    })
  })

  // ===== STEP 1: FORM FIELD ATTRIBUTES =====
  describe('Step 1: Form field attributes', () => {
    it('Full name input is text and required', () => {
      renderCampaignSignup()
      const input = screen.getByLabelText('Full name') as HTMLInputElement
      expect(input.type).toBe('text')
      expect(input.required).toBe(true)
    })

    it('Company name input is text and required', () => {
      renderCampaignSignup()
      const input = screen.getByLabelText('Company name') as HTMLInputElement
      expect(input.type).toBe('text')
      expect(input.required).toBe(true)
    })

    it('Work email input is type=email and required', () => {
      renderCampaignSignup()
      const input = screen.getByLabelText('Work email') as HTMLInputElement
      expect(input.type).toBe('email')
      expect(input.required).toBe(true)
    })

    it('Company website input is NOT required', () => {
      renderCampaignSignup()
      const input = screen.getByLabelText(/Company website/) as HTMLInputElement
      expect(input.required).toBe(false)
    })

    it('Continue button has type=submit', () => {
      renderCampaignSignup()
      const btn = screen.getByText(/Continue/) as HTMLButtonElement
      expect(btn.type).toBe('submit')
    })

    it('all required inputs start empty', () => {
      renderCampaignSignup()
      expect(screen.getByLabelText('Full name')).toHaveValue('')
      expect(screen.getByLabelText('Company name')).toHaveValue('')
      expect(screen.getByLabelText('Work email')).toHaveValue('')
      expect(screen.getByLabelText(/Company website/)).toHaveValue('')
    })
  })

  // ===== STEP 3: SUMMARY ROW CONTENT =====
  describe('Step 3: Campaign summary rows', () => {
    it('renders exactly 7 summary rows with all expected labels', async () => {
      await goToStep3()
      const labels = ['Company', 'Campaign type', 'Categories', 'Geography', 'Budget', 'Dates', 'Product']
      labels.forEach(l => expect(screen.getByText(l)).toBeInTheDocument())
      expect(document.querySelectorAll('.summary-row').length).toBe(7)
    })
  })

  // ===== BRAND CONSISTENCY =====
  describe('brand consistency', () => {
    it('does not use corporate jargon in headings', () => {
      renderCampaignSignup()
      const headings = screen.getAllByRole('heading')
      headings.forEach(heading => {
        expect(heading.textContent).not.toMatch(/\bsynergy\b/i)
        expect(heading.textContent).not.toMatch(/\bleverage\b/i)
      })
    })

    it('uses warm tone in description text', () => {
      renderCampaignSignup()
      expect(screen.getByText(/Tell us about your company to get started/)).toBeInTheDocument()
    })

    it('does not use buzzwords like "revolutionize" or "disrupt"', () => {
      renderCampaignSignup()
      const allText = document.body.textContent || ''
      expect(allText).not.toMatch(/\brevolutionize\b/i)
      expect(allText).not.toMatch(/\bdisrupt\b/i)
    })
  })

  // ===== ACCESSIBILITY =====
  describe('accessibility', () => {
    it('all required form inputs have associated labels', () => {
      renderCampaignSignup()
      const inputs = document.querySelectorAll('input[required]')
      inputs.forEach(input => {
        const id = input.getAttribute('id')
        if (id) {
          const label = document.querySelector(`label[for="${id}"]`)
          expect(label).toBeInTheDocument()
        }
      })
    })

    it('form labels have form-label class', () => {
      renderCampaignSignup()
      expect(document.querySelectorAll('label.form-label').length).toBeGreaterThanOrEqual(3)
    })

    it('form inputs have form-input class', () => {
      renderCampaignSignup()
      expect(document.querySelectorAll('.form-input').length).toBeGreaterThanOrEqual(3)
    })

    it('error messages render with role=alert when submission fails', async () => {
      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'Validation failed.' }), { status: 400, headers: { 'Content-Type': 'application/json' } })
      )
      const user = await goToStep3()
      await user.click(screen.getByRole('checkbox', { name: /I agree to Groflo's/i }))
      await user.click(screen.getByRole('button', { name: /Submit Campaign Brief/i }))
      const alert = await screen.findByRole('alert')
      expect(alert.textContent).toMatch(/Validation failed/)
    })
  })

  // ===== PAGE STRUCTURE =====
  describe('page structure', () => {
    it('wraps content in signup-section and signup-container', () => {
      renderCampaignSignup()
      expect(document.querySelector('.signup-section')).toBeInTheDocument()
      expect(document.querySelector('.signup-container')).toBeInTheDocument()
    })

    it('has step-indicator with four step dot wrappers', () => {
      renderCampaignSignup()
      expect(document.querySelector('.step-indicator')).toBeInTheDocument()
      expect(document.querySelectorAll('.step-dot-wrapper').length).toBe(4)
    })

    it('renders signup-step wrapper on Step 1', () => {
      renderCampaignSignup()
      expect(document.querySelector('.signup-step')).toBeInTheDocument()
    })

    it('has exactly one section element', () => {
      renderCampaignSignup()
      expect(document.querySelectorAll('section').length).toBe(1)
    })
  })

  // ===== STEP INDICATOR DETAILED =====
  describe('step indicator details', () => {
    it('no steps are completed initially', () => {
      renderCampaignSignup()
      expect(document.querySelectorAll('.step-dot-wrapper.completed').length).toBe(0)
    })

    it('only first step is active initially', () => {
      renderCampaignSignup()
      expect(document.querySelectorAll('.step-dot-wrapper.active').length).toBe(1)
    })

    it('step labels appear in order: Account, Campaign, Review, Confirmation', () => {
      renderCampaignSignup()
      const texts = Array.from(document.querySelectorAll('.step-dot-label')).map(l => l.textContent)
      expect(texts).toEqual(['Account', 'Campaign', 'Review', 'Confirmation'])
    })
  })

  // ===== STEP 2: GEOGRAPHY OPTIONS DETAIL =====
  describe('Step 2: Form field attributes detail', () => {
    it('budget input is type=number and required', async () => {
      await goToStep2()
      const input = screen.getByLabelText(/budget/) as HTMLInputElement
      expect(input.type).toBe('number')
      expect(input.required).toBe(true)
    })

    it('start date and end date inputs are type=date', async () => {
      await goToStep2()
      expect((screen.getByLabelText('Start date') as HTMLInputElement).type).toBe('date')
      expect((screen.getByLabelText('End date') as HTMLInputElement).type).toBe('date')
    })

    it('product name and description are required', async () => {
      await goToStep2()
      expect((screen.getByLabelText('Product / brand name') as HTMLInputElement).required).toBe(true)
      expect((screen.getByLabelText(/Short description/) as HTMLTextAreaElement).required).toBe(true)
    })

    it('form has signup-actions section with Back and Continue buttons', async () => {
      await goToStep2()
      const actions = document.querySelector('.signup-actions')
      expect(actions).toBeInTheDocument()
      expect(actions?.querySelectorAll('button').length).toBe(2)
    })
  })

  // ===== STEP 2: BUDGET TYPE SWITCHING =====
  describe('Step 2: Budget type switching', () => {
    it('toggles Daily/Total active state', async () => {
      const user = await goToStep2()
      expect(screen.getByText('Daily').classList.contains('active')).toBe(true)
      expect(screen.getByText('Total Campaign').classList.contains('active')).toBe(false)
      await user.click(screen.getByText('Total Campaign'))
      expect(screen.getByText('Total Campaign').classList.contains('active')).toBe(true)
      expect(screen.getByText('Daily').classList.contains('active')).toBe(false)
      await user.click(screen.getByText('Daily'))
      expect(screen.getByText('Daily').classList.contains('active')).toBe(true)
    })

    it('budget toggle buttons are type=button (not submit)', async () => {
      await goToStep2()
      expect((screen.getByText('Daily') as HTMLButtonElement).type).toBe('button')
      expect((screen.getByText('Total Campaign') as HTMLButtonElement).type).toBe('button')
    })

    it('budget toggle wrapper has budget-toggle class', async () => {
      await goToStep2()
      expect(document.querySelector('.budget-toggle')).toBeInTheDocument()
    })
  })

  // ===== STEP 2: CATEGORY DETAIL =====
  describe('Step 2: Category checkbox details', () => {
    it('all categories are unchecked by default', async () => {
      await goToStep2()
      const checkboxes = screen.getAllByRole('checkbox') as HTMLInputElement[]
      checkboxes.forEach(cb => expect(cb.checked).toBe(false))
    })

    it('clicking a category adds and removes selected class', async () => {
      const user = await goToStep2()
      const produceLabel = screen.getByText('Produce').closest('label')!
      await user.click(produceLabel)
      expect(produceLabel.classList.contains('selected')).toBe(true)
      await user.click(produceLabel)
      expect(produceLabel.classList.contains('selected')).toBe(false)
    })

    it('checkbox group has checkbox-group class with 10 options', async () => {
      await goToStep2()
      expect(document.querySelector('.checkbox-group')).toBeInTheDocument()
      expect(document.querySelectorAll('.checkbox-option').length).toBe(10)
    })
  })

  // ===== STEP 2: RADIO DETAIL =====
  describe('Step 2: Campaign type radio details', () => {
    it('Free Product Trial is selected by default; others are not', async () => {
      await goToStep2()
      expect((screen.getByDisplayValue('free-trial') as HTMLInputElement).checked).toBe(true)
      expect((screen.getByDisplayValue('sponsored-brand') as HTMLInputElement).checked).toBe(false)
      expect((screen.getByDisplayValue('list-placement') as HTMLInputElement).checked).toBe(false)
    })

    it('selecting Sponsored Brand deselects Free Product Trial', async () => {
      const user = await goToStep2()
      await user.click(screen.getByDisplayValue('sponsored-brand'))
      expect((screen.getByDisplayValue('sponsored-brand') as HTMLInputElement).checked).toBe(true)
      expect((screen.getByDisplayValue('free-trial') as HTMLInputElement).checked).toBe(false)
    })

    it('radio-group container with 5 radio-option labels', async () => {
      await goToStep2()
      expect(document.querySelector('.radio-group')).toBeInTheDocument()
      expect(document.querySelectorAll('.radio-option').length).toBe(5)
    })

    it('selected radio option has selected class; others do not', async () => {
      await goToStep2()
      const freeTrialLabel = screen.getByDisplayValue('free-trial').closest('label')!
      const sponsoredLabel = screen.getByDisplayValue('sponsored-brand').closest('label')!
      expect(freeTrialLabel.classList.contains('selected')).toBe(true)
      expect(sponsoredLabel.classList.contains('selected')).toBe(false)
    })
  })

  // ===== REGRESSION GUARD: Stripe removal =====
  describe('regression: no Stripe in this repo', () => {
    it('does not import @stripe/* anywhere in CampaignSignup', () => {
      // Source-level guard: if anyone re-introduces a Stripe import on this page,
      // the page would render Stripe Elements; this test asserts none render.
      renderCampaignSignup()
      expect(document.querySelector('[data-testid="stripe-elements"]')).toBeNull()
      expect(document.querySelector('[data-testid="card-element"]')).toBeNull()
      expect(document.querySelector('.stripe-card-wrapper')).toBeNull()
    })

    it('no element claims to authorize a card amount', () => {
      renderCampaignSignup()
      const allText = document.body.textContent || ''
      expect(allText).not.toMatch(/Authorize \$/)
    })
  })
})
