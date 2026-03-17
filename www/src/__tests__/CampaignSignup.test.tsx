import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import CampaignSignup from '../pages/CampaignSignup'

// Mock Stripe modules
vi.mock('@stripe/stripe-js', () => ({
  loadStripe: vi.fn(() => null),
}))

vi.mock('@stripe/react-stripe-js', () => ({
  Elements: ({ children }: { children: React.ReactNode }) => <div data-testid="stripe-elements">{children}</div>,
  CardElement: () => <div data-testid="card-element">Card Element Mock</div>,
  useStripe: () => null,
  useElements: () => null,
}))

function renderCampaignSignup() {
  return render(
    <MemoryRouter>
      <CampaignSignup />
    </MemoryRouter>
  )
}

describe('CampaignSignup page', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.useFakeTimers({ shouldAdvanceTime: true })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

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
    it('renders all four step labels', () => {
      renderCampaignSignup()
      expect(screen.getByText('Account')).toBeInTheDocument()
      expect(screen.getByText('Campaign')).toBeInTheDocument()
      expect(screen.getByText('Payment')).toBeInTheDocument()
      expect(screen.getByText('Confirmation')).toBeInTheDocument()
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

    it('renders Full name field', () => {
      renderCampaignSignup()
      expect(screen.getByLabelText('Full name')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('Jane Smith')).toBeInTheDocument()
    })

    it('renders Company name field', () => {
      renderCampaignSignup()
      expect(screen.getByLabelText('Company name')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('Acme Foods Inc.')).toBeInTheDocument()
    })

    it('renders Work email field', () => {
      renderCampaignSignup()
      expect(screen.getByLabelText('Work email')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('jane@acmefoods.com')).toBeInTheDocument()
    })

    it('renders Company website field marked as optional', () => {
      renderCampaignSignup()
      expect(screen.getByLabelText(/Company website/)).toBeInTheDocument()
      expect(screen.getByText('(optional)')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('acmefoods.com')).toBeInTheDocument()
    })

    it('renders Continue button', () => {
      renderCampaignSignup()
      expect(screen.getByText(/Continue/)).toBeInTheDocument()
    })

    it('updates input fields on user type', async () => {
      vi.useRealTimers()
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
    async function fillAndSubmitStep1() {
      vi.useRealTimers()
      const user = userEvent.setup()
      renderCampaignSignup()

      await user.type(screen.getByLabelText('Full name'), 'Jane Smith')
      await user.type(screen.getByLabelText('Company name'), 'Acme Foods')
      await user.type(screen.getByLabelText('Work email'), 'jane@acme.com')
      await user.click(screen.getByText(/Continue/))
      return user
    }

    it('advances to Step 2 on form submission', async () => {
      await fillAndSubmitStep1()
      expect(screen.getByText('Configure your campaign')).toBeInTheDocument()
    })

    it('shows Step 2 as active in step indicator', async () => {
      await fillAndSubmitStep1()
      const activeStep = document.querySelector('.step-dot-wrapper.active')
      const label = activeStep?.querySelector('.step-dot-label')
      expect(label?.textContent).toBe('Campaign')
    })

    it('marks Step 1 as completed with checkmark', async () => {
      await fillAndSubmitStep1()
      const completedStep = document.querySelector('.step-dot-wrapper.completed')
      const dot = completedStep?.querySelector('.step-dot')
      expect(dot?.textContent).toContain('\u2713')
    })
  })

  // ===== STEP 2: CAMPAIGN =====
  describe('Step 2: Campaign', () => {
    async function goToStep2() {
      vi.useRealTimers()
      const user = userEvent.setup()
      renderCampaignSignup()
      await user.type(screen.getByLabelText('Full name'), 'Jane Smith')
      await user.type(screen.getByLabelText('Company name'), 'Acme Foods')
      await user.type(screen.getByLabelText('Work email'), 'jane@acme.com')
      await user.click(screen.getByText(/Continue/))
      return user
    }

    it('displays the Configure your campaign heading', async () => {
      await goToStep2()
      expect(screen.getByText('Configure your campaign')).toBeInTheDocument()
    })

    it('displays the step description', async () => {
      await goToStep2()
      expect(screen.getByText('Choose your campaign type, targeting, and budget.')).toBeInTheDocument()
    })

    it('renders campaign type radio options', async () => {
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

    it('renders budget type toggle with Daily and Total Campaign', async () => {
      await goToStep2()
      expect(screen.getByText('Daily')).toBeInTheDocument()
      expect(screen.getByText('Total Campaign')).toBeInTheDocument()
    })

    it('defaults to daily budget type', async () => {
      await goToStep2()
      const dailyBtn = screen.getByText('Daily')
      expect(dailyBtn.classList.contains('active')).toBe(true)
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

    it('renders start and end date inputs', async () => {
      await goToStep2()
      expect(screen.getByLabelText('Start date')).toBeInTheDocument()
      expect(screen.getByLabelText('End date')).toBeInTheDocument()
    })

    it('renders product/brand name field', async () => {
      await goToStep2()
      expect(screen.getByLabelText('Product / brand name')).toBeInTheDocument()
      expect(screen.getByPlaceholderText(/Organic Valley/)).toBeInTheDocument()
    })

    it('renders short description textarea with character counter', async () => {
      await goToStep2()
      expect(screen.getByLabelText(/Short description/)).toBeInTheDocument()
      expect(screen.getByText('0/140')).toBeInTheDocument()
    })

    it('updates character count on description input', async () => {
      const user = await goToStep2()
      const textarea = screen.getByLabelText(/Short description/)
      await user.type(textarea, 'Hello world')
      expect(screen.getByText('11/140')).toBeInTheDocument()
    })

    it('has a Back button to return to Step 1', async () => {
      const user = await goToStep2()
      expect(screen.getByText(/Back/)).toBeInTheDocument()
      await user.click(screen.getByText(/← Back/))
      expect(screen.getByText('Create your brand account')).toBeInTheDocument()
    })

    it('has a Continue to Payment button', async () => {
      await goToStep2()
      expect(screen.getByText(/Continue to Payment/)).toBeInTheDocument()
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
  })

  // ===== STEP 2 VALIDATION =====
  describe('Step 2 validation', () => {
    async function goToStep2WithData() {
      vi.useRealTimers()
      const user = userEvent.setup()
      renderCampaignSignup()
      await user.type(screen.getByLabelText('Full name'), 'Jane')
      await user.type(screen.getByLabelText('Company name'), 'Acme')
      await user.type(screen.getByLabelText('Work email'), 'j@a.com')
      await user.click(screen.getByText(/Continue/))
      return user
    }

    it('shows validation error when no categories selected', async () => {
      const user = await goToStep2WithData()

      // Fill required fields but skip categories
      const today = new Date().toISOString().split('T')[0]
      const tomorrow = new Date(Date.now() + 86400000).toISOString().split('T')[0]

      // We need to use fireEvent for date inputs since userEvent doesn't handle date well
      const startDate = screen.getByLabelText('Start date')
      const endDate = screen.getByLabelText('End date')
      await user.clear(startDate)
      await user.type(startDate, today)
      await user.clear(endDate)
      await user.type(endDate, tomorrow)
      await user.type(screen.getByLabelText('Product / brand name'), 'Test Product')
      await user.type(screen.getByLabelText(/Short description/), 'Test desc')
      await user.click(screen.getByText(/Continue to Payment/))

      expect(screen.getByText('Please select at least one target category.')).toBeInTheDocument()
    })
  })

  // ===== STEP 3: PAYMENT (Mock) =====
  describe('Step 3: Payment/Review', () => {
    async function goToStep3() {
      vi.useRealTimers()
      const user = userEvent.setup()
      renderCampaignSignup()

      // Step 1
      await user.type(screen.getByLabelText('Full name'), 'Jane Smith')
      await user.type(screen.getByLabelText('Company name'), 'Acme Foods')
      await user.type(screen.getByLabelText('Work email'), 'jane@acme.com')
      await user.click(screen.getByText(/Continue/))

      // Step 2 - fill all required fields
      const dairyLabel = screen.getByText('Dairy').closest('label')!
      await user.click(dairyLabel)

      const today = new Date()
      const todayStr = today.toISOString().split('T')[0]
      const future = new Date(today.getTime() + 7 * 86400000)
      const futureStr = future.toISOString().split('T')[0]

      const startDate = screen.getByLabelText('Start date')
      const endDate = screen.getByLabelText('End date')
      await user.clear(startDate)
      await user.type(startDate, todayStr)
      await user.clear(endDate)
      await user.type(endDate, futureStr)
      await user.type(screen.getByLabelText('Product / brand name'), 'Organic Valley Milk')
      await user.type(screen.getByLabelText(/Short description/), 'Farm-fresh organic milk')
      await user.click(screen.getByText(/Continue to Payment/))
      return user
    }

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
      expect(screen.getByText(/I agree to Thriftly's/)).toBeInTheDocument()
    })

    it('links to Terms of Service and Privacy Policy', async () => {
      await goToStep3()
      expect(screen.getByText('Terms of Service')).toHaveAttribute('href', '/terms')
      expect(screen.getByText('Privacy Policy')).toHaveAttribute('href', '/privacy')
    })

    it('does not show payment form until terms are agreed to', async () => {
      await goToStep3()
      expect(screen.queryByText('Submit Campaign for Review')).not.toBeInTheDocument()
    })

    it('shows mock payment form when terms are agreed (no Stripe key)', async () => {
      const user = await goToStep3()
      const checkbox = screen.getByRole('checkbox')
      await user.click(checkbox)
      expect(screen.getByText('Submit Campaign for Review')).toBeInTheDocument()
      expect(screen.getByText(/Payment setup coming soon/)).toBeInTheDocument()
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
  })

  // ===== STEP 4: CONFIRMATION (via mock payment) =====
  // TODO: Step 4 tests require deeper Stripe mock (payment intent confirmation)
  describe.skip('Step 4: Confirmation', () => {
    async function goToStep4() {
      vi.useRealTimers()
      vi.useFakeTimers({ shouldAdvanceTime: true })
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      renderCampaignSignup()

      // Step 1
      await user.type(screen.getByLabelText('Full name'), 'Jane Smith')
      await user.type(screen.getByLabelText('Company name'), 'Acme Foods')
      await user.type(screen.getByLabelText('Work email'), 'jane@acme.com')
      await user.click(screen.getByText(/Continue/))

      // Step 2
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
      await user.click(screen.getByText(/Continue to Payment/))

      // Step 3
      const checkbox = screen.getByRole('checkbox')
      await user.click(checkbox)
      await user.click(screen.getByText('Submit Campaign for Review'))

      // Advance past mock processing timeout (1500ms)
      vi.advanceTimersByTime(2000)

      return user
    }

    it('shows confirmation page after successful payment', async () => {
      await goToStep4()
      expect(screen.getByText('Campaign submitted for review')).toBeInTheDocument()
    })

    it('displays confirmation icon', async () => {
      await goToStep4()
      const icon = document.querySelector('.confirmation-icon')
      expect(icon).toBeInTheDocument()
    })

    it('displays the product name in confirmation', async () => {
      await goToStep4()
      expect(screen.getByText('Organic Valley Milk')).toBeInTheDocument()
    })

    it('mentions the confirmation email address', async () => {
      await goToStep4()
      expect(screen.getByText('jane@acme.com')).toBeInTheDocument()
    })

    it('displays campaign details summary', async () => {
      await goToStep4()
      expect(screen.getByText('Campaign Details')).toBeInTheDocument()
    })

    it('shows Under Review status badge', async () => {
      await goToStep4()
      expect(screen.getByText('Under Review')).toBeInTheDocument()
    })

    it('has Return to Home link', async () => {
      await goToStep4()
      const homeLink = screen.getByText(/Return to Home/)
      expect(homeLink).toHaveAttribute('href', '/')
    })

    it('has Learn More About Ads link', async () => {
      await goToStep4()
      const learnLink = screen.getByText('Learn More About Ads')
      expect(learnLink).toHaveAttribute('href', '/for-brands')
    })

    it('stores campaign data in localStorage', async () => {
      await goToStep4()
      const stored = localStorage.getItem('thriftly_campaign')
      expect(stored).not.toBeNull()
      const parsed = JSON.parse(stored!)
      expect(parsed.accountData.companyName).toBe('Acme Foods')
      expect(parsed.campaignData.creativeName).toBe('Organic Valley Milk')
      expect(parsed.createdAt).toBeDefined()
    })
  })

  // ===== STEP 2: CAMPAIGN TYPE DESCRIPTIONS =====
  describe('Step 2: Campaign type descriptions', () => {
    async function goToStep2() {
      vi.useRealTimers()
      const user = userEvent.setup()
      renderCampaignSignup()
      await user.type(screen.getByLabelText('Full name'), 'Jane')
      await user.type(screen.getByLabelText('Company name'), 'Acme')
      await user.type(screen.getByLabelText('Work email'), 'j@a.com')
      await user.click(screen.getByText(/Continue/))
      return user
    }

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
      // Free Product Trial should no longer be checked
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

      const dairyCheckbox = within(dairyLabel).getByRole('checkbox') as HTMLInputElement
      const snacksCheckbox = within(snacksLabel).getByRole('checkbox') as HTMLInputElement
      const beveragesCheckbox = within(beveragesLabel).getByRole('checkbox') as HTMLInputElement

      expect(dairyCheckbox.checked).toBe(true)
      expect(snacksCheckbox.checked).toBe(true)
      expect(beveragesCheckbox.checked).toBe(true)
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

    it('budget amount updates when switching from daily to total', async () => {
      const user = await goToStep2()
      // Default is daily with 25
      const budgetInput = screen.getByLabelText(/budget/) as HTMLInputElement
      expect(budgetInput.value).toBe('25')

      await user.click(screen.getByText('Total Campaign'))
      // Should be at least 100 when switching to total
      const updatedInput = screen.getByLabelText(/budget/) as HTMLInputElement
      expect(Number(updatedInput.value)).toBeGreaterThanOrEqual(100)
    })

    it('has $ prefix on budget input', async () => {
      await goToStep2()
      expect(document.querySelector('.budget-currency')).toBeInTheDocument()
      expect(document.querySelector('.budget-currency')?.textContent).toBe('$')
    })

    it('renders all 10 checkbox options for categories', async () => {
      await goToStep2()
      const checkboxes = document.querySelectorAll('.checkbox-option')
      expect(checkboxes.length).toBe(10)
    })

    it('renders 6 geography options', async () => {
      await goToStep2()
      const select = screen.getByLabelText('Geographic targeting') as HTMLSelectElement
      expect(select.options.length).toBe(6)
    })

    it('description textarea has maxLength of 140', async () => {
      await goToStep2()
      const textarea = screen.getByLabelText(/Short description/) as HTMLTextAreaElement
      expect(textarea.maxLength).toBe(140)
    })
  })

  // ===== STEP 1: FORM FIELD ATTRIBUTES =====
  describe('Step 1: Form field attributes', () => {
    it('Full name input has type text', () => {
      renderCampaignSignup()
      const input = screen.getByLabelText('Full name') as HTMLInputElement
      expect(input.type).toBe('text')
      expect(input.required).toBe(true)
    })

    it('Company name input has type text and is required', () => {
      renderCampaignSignup()
      const input = screen.getByLabelText('Company name') as HTMLInputElement
      expect(input.type).toBe('text')
      expect(input.required).toBe(true)
    })

    it('Work email input has type email and is required', () => {
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

    it('Continue button has type submit', () => {
      renderCampaignSignup()
      const btn = screen.getByText(/Continue/) as HTMLButtonElement
      expect(btn.type).toBe('submit')
    })

    it('Full name input starts empty', () => {
      renderCampaignSignup()
      expect(screen.getByLabelText('Full name')).toHaveValue('')
    })

    it('Company name input starts empty', () => {
      renderCampaignSignup()
      expect(screen.getByLabelText('Company name')).toHaveValue('')
    })

    it('Work email input starts empty', () => {
      renderCampaignSignup()
      expect(screen.getByLabelText('Work email')).toHaveValue('')
    })

    it('Company website input starts empty', () => {
      renderCampaignSignup()
      expect(screen.getByLabelText(/Company website/)).toHaveValue('')
    })
  })

  // ===== STEP 3: SUMMARY ROW CONTENT =====
  describe('Step 3: Campaign summary rows', () => {
    async function goToStep3() {
      vi.useRealTimers()
      const user = userEvent.setup()
      renderCampaignSignup()

      await user.type(screen.getByLabelText('Full name'), 'Jane Smith')
      await user.type(screen.getByLabelText('Company name'), 'Acme Foods')
      await user.type(screen.getByLabelText('Work email'), 'jane@acme.com')
      await user.click(screen.getByText(/Continue/))

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
      await user.click(screen.getByText(/Continue to Payment/))
      return user
    }

    it('shows Company row in summary', async () => {
      await goToStep3()
      expect(screen.getByText('Company')).toBeInTheDocument()
    })

    it('shows Campaign type row in summary', async () => {
      await goToStep3()
      expect(screen.getByText('Campaign type')).toBeInTheDocument()
    })

    it('shows Categories row in summary', async () => {
      await goToStep3()
      expect(screen.getByText('Categories')).toBeInTheDocument()
    })

    it('shows Geography row in summary', async () => {
      await goToStep3()
      expect(screen.getByText('Geography')).toBeInTheDocument()
    })

    it('shows Budget row in summary', async () => {
      await goToStep3()
      expect(screen.getByText('Budget')).toBeInTheDocument()
    })

    it('shows Dates row in summary', async () => {
      await goToStep3()
      expect(screen.getByText('Dates')).toBeInTheDocument()
    })

    it('shows Product row in summary', async () => {
      await goToStep3()
      expect(screen.getByText('Product')).toBeInTheDocument()
    })

    it('renders 7 summary rows', async () => {
      await goToStep3()
      const rows = document.querySelectorAll('.summary-row')
      expect(rows.length).toBe(7)
    })

    it('terms links open in new tab', async () => {
      await goToStep3()
      const tosLink = screen.getByText('Terms of Service')
      expect(tosLink).toHaveAttribute('target', '_blank')
      const privacyLink = screen.getByText('Privacy Policy')
      expect(privacyLink).toHaveAttribute('target', '_blank')
    })

    it('shows budget with /day for daily budget type', async () => {
      await goToStep3()
      expect(screen.getByText('$25/day')).toBeInTheDocument()
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

    it('uses clear, action-oriented button labels', () => {
      renderCampaignSignup()
      expect(screen.getByText(/Continue/)).toBeInTheDocument()
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
    it('all form inputs have associated labels', () => {
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
      const labels = document.querySelectorAll('label.form-label')
      expect(labels.length).toBeGreaterThanOrEqual(3)
    })

    it('form inputs have form-input class', () => {
      renderCampaignSignup()
      const inputs = document.querySelectorAll('.form-input')
      expect(inputs.length).toBeGreaterThanOrEqual(3)
    })
  })

  // ===== PAGE STRUCTURE =====
  describe('page structure', () => {
    it('wraps content in signup-section', () => {
      renderCampaignSignup()
      expect(document.querySelector('.signup-section')).toBeInTheDocument()
    })

    it('wraps content in signup-container', () => {
      renderCampaignSignup()
      expect(document.querySelector('.signup-container')).toBeInTheDocument()
    })

    it('has step-indicator at top', () => {
      renderCampaignSignup()
      expect(document.querySelector('.step-indicator')).toBeInTheDocument()
    })

    it('renders four step dot wrappers', () => {
      renderCampaignSignup()
      const wrappers = document.querySelectorAll('.step-dot-wrapper')
      expect(wrappers.length).toBe(4)
    })

    it('renders signup-step wrapper on Step 1', () => {
      renderCampaignSignup()
      expect(document.querySelector('.signup-step')).toBeInTheDocument()
    })

    it('has exactly one section element', () => {
      renderCampaignSignup()
      const sections = document.querySelectorAll('section')
      expect(sections.length).toBe(1)
    })
  })

  // ===== STEP INDICATOR DETAILED =====
  describe('step indicator details', () => {
    it('no steps are completed initially', () => {
      renderCampaignSignup()
      const completed = document.querySelectorAll('.step-dot-wrapper.completed')
      expect(completed.length).toBe(0)
    })

    it('no steps other than first are active initially', () => {
      renderCampaignSignup()
      const activeSteps = document.querySelectorAll('.step-dot-wrapper.active')
      expect(activeSteps.length).toBe(1)
    })

    it('step labels appear in order: Account, Campaign, Payment, Confirmation', () => {
      renderCampaignSignup()
      const labels = document.querySelectorAll('.step-dot-label')
      const texts = Array.from(labels).map(l => l.textContent)
      expect(texts).toEqual(['Account', 'Campaign', 'Payment', 'Confirmation'])
    })
  })

  // ===== STEP 2: GEOGRAPHY OPTIONS =====
  describe('Step 2: Geography options detail', () => {
    async function goToStep2() {
      vi.useRealTimers()
      const user = userEvent.setup()
      renderCampaignSignup()
      await user.type(screen.getByLabelText('Full name'), 'Jane')
      await user.type(screen.getByLabelText('Company name'), 'Acme')
      await user.type(screen.getByLabelText('Work email'), 'j@a.com')
      await user.click(screen.getByText(/Continue/))
      return user
    }

    it('geography select has value "national" by default', async () => {
      await goToStep2()
      const select = screen.getByLabelText('Geographic targeting') as HTMLSelectElement
      expect(select.value).toBe('national')
    })

    it('can change to southeast', async () => {
      const user = await goToStep2()
      await user.selectOptions(screen.getByLabelText('Geographic targeting'), 'southeast')
      expect(screen.getByLabelText('Geographic targeting')).toHaveValue('southeast')
    })

    it('can change to midwest', async () => {
      const user = await goToStep2()
      await user.selectOptions(screen.getByLabelText('Geographic targeting'), 'midwest')
      expect(screen.getByLabelText('Geographic targeting')).toHaveValue('midwest')
    })

    it('can change to southwest', async () => {
      const user = await goToStep2()
      await user.selectOptions(screen.getByLabelText('Geographic targeting'), 'southwest')
      expect(screen.getByLabelText('Geographic targeting')).toHaveValue('southwest')
    })

    it('budget input has type number', async () => {
      await goToStep2()
      const input = screen.getByLabelText(/budget/) as HTMLInputElement
      expect(input.type).toBe('number')
    })

    it('budget input has required attribute', async () => {
      await goToStep2()
      const input = screen.getByLabelText(/budget/) as HTMLInputElement
      expect(input.required).toBe(true)
    })

    it('start date input has type date', async () => {
      await goToStep2()
      const input = screen.getByLabelText('Start date') as HTMLInputElement
      expect(input.type).toBe('date')
    })

    it('end date input has type date', async () => {
      await goToStep2()
      const input = screen.getByLabelText('End date') as HTMLInputElement
      expect(input.type).toBe('date')
    })

    it('product name input is required', async () => {
      await goToStep2()
      const input = screen.getByLabelText('Product / brand name') as HTMLInputElement
      expect(input.required).toBe(true)
    })

    it('description textarea is required', async () => {
      await goToStep2()
      const textarea = screen.getByLabelText(/Short description/) as HTMLTextAreaElement
      expect(textarea.required).toBe(true)
    })

    it('description textarea has 3 rows', async () => {
      await goToStep2()
      const textarea = screen.getByLabelText(/Short description/) as HTMLTextAreaElement
      expect(textarea.rows).toBe(3)
    })

    it('form has signup-actions section with Back and Continue buttons', async () => {
      await goToStep2()
      const actions = document.querySelector('.signup-actions')
      expect(actions).toBeInTheDocument()
      const buttons = actions?.querySelectorAll('button')
      expect(buttons?.length).toBe(2)
    })
  })

  // ===== STEP 2: BUDGET TYPE SWITCHING =====
  describe('Step 2: Budget type switching', () => {
    async function goToStep2() {
      vi.useRealTimers()
      const user = userEvent.setup()
      renderCampaignSignup()
      await user.type(screen.getByLabelText('Full name'), 'Jane')
      await user.type(screen.getByLabelText('Company name'), 'Acme')
      await user.type(screen.getByLabelText('Work email'), 'j@a.com')
      await user.click(screen.getByText(/Continue/))
      return user
    }

    it('Daily button has active class initially', async () => {
      await goToStep2()
      const dailyBtn = screen.getByText('Daily')
      expect(dailyBtn.classList.contains('active')).toBe(true)
    })

    it('Total Campaign button does not have active class initially', async () => {
      await goToStep2()
      const totalBtn = screen.getByText('Total Campaign')
      expect(totalBtn.classList.contains('active')).toBe(false)
    })

    it('switching to Total makes Total active and Daily inactive', async () => {
      const user = await goToStep2()
      await user.click(screen.getByText('Total Campaign'))
      const totalBtn = screen.getByText('Total Campaign')
      const dailyBtn = screen.getByText('Daily')
      expect(totalBtn.classList.contains('active')).toBe(true)
      expect(dailyBtn.classList.contains('active')).toBe(false)
    })

    it('switching back to Daily restores Daily as active', async () => {
      const user = await goToStep2()
      await user.click(screen.getByText('Total Campaign'))
      await user.click(screen.getByText('Daily'))
      const dailyBtn = screen.getByText('Daily')
      expect(dailyBtn.classList.contains('active')).toBe(true)
    })

    it('budget toggle buttons are type=button (not submit)', async () => {
      await goToStep2()
      const dailyBtn = screen.getByText('Daily') as HTMLButtonElement
      const totalBtn = screen.getByText('Total Campaign') as HTMLButtonElement
      expect(dailyBtn.type).toBe('button')
      expect(totalBtn.type).toBe('button')
    })

    it('budget toggle wrapper has budget-toggle class', async () => {
      await goToStep2()
      expect(document.querySelector('.budget-toggle')).toBeInTheDocument()
    })
  })

  // ===== STEP 2: CATEGORY CHECKBOXES DETAIL =====
  describe('Step 2: Category checkbox details', () => {
    async function goToStep2() {
      vi.useRealTimers()
      const user = userEvent.setup()
      renderCampaignSignup()
      await user.type(screen.getByLabelText('Full name'), 'Jane')
      await user.type(screen.getByLabelText('Company name'), 'Acme')
      await user.type(screen.getByLabelText('Work email'), 'j@a.com')
      await user.click(screen.getByText(/Continue/))
      return user
    }

    it('all categories are unchecked by default', async () => {
      await goToStep2()
      const checkboxes = screen.getAllByRole('checkbox') as HTMLInputElement[]
      checkboxes.forEach(cb => {
        expect(cb.checked).toBe(false)
      })
    })

    it('clicking a category adds selected class to its label', async () => {
      const user = await goToStep2()
      const produceLabel = screen.getByText('Produce').closest('label')!
      expect(produceLabel.classList.contains('selected')).toBe(false)
      await user.click(produceLabel)
      expect(produceLabel.classList.contains('selected')).toBe(true)
    })

    it('unclicking a category removes selected class', async () => {
      const user = await goToStep2()
      const snacksLabel = screen.getByText('Snacks').closest('label')!
      await user.click(snacksLabel)
      expect(snacksLabel.classList.contains('selected')).toBe(true)
      await user.click(snacksLabel)
      expect(snacksLabel.classList.contains('selected')).toBe(false)
    })

    it('checkbox group has checkbox-group class', async () => {
      await goToStep2()
      expect(document.querySelector('.checkbox-group')).toBeInTheDocument()
    })

    it('each category option has checkbox-option class', async () => {
      await goToStep2()
      const options = document.querySelectorAll('.checkbox-option')
      expect(options.length).toBe(10)
    })
  })

  // ===== STEP 2: CAMPAIGN TYPE RADIO DETAIL =====
  describe('Step 2: Campaign type radio details', () => {
    async function goToStep2() {
      vi.useRealTimers()
      const user = userEvent.setup()
      renderCampaignSignup()
      await user.type(screen.getByLabelText('Full name'), 'Jane')
      await user.type(screen.getByLabelText('Company name'), 'Acme')
      await user.type(screen.getByLabelText('Work email'), 'j@a.com')
      await user.click(screen.getByText(/Continue/))
      return user
    }

    it('Free Product Trial radio is selected by default', async () => {
      await goToStep2()
      const radio = screen.getByDisplayValue('free-trial') as HTMLInputElement
      expect(radio.checked).toBe(true)
    })

    it('Sponsored Brand radio is not selected by default', async () => {
      await goToStep2()
      const radio = screen.getByDisplayValue('sponsored-brand') as HTMLInputElement
      expect(radio.checked).toBe(false)
    })

    it('List Placement radio is not selected by default', async () => {
      await goToStep2()
      const radio = screen.getByDisplayValue('list-placement') as HTMLInputElement
      expect(radio.checked).toBe(false)
    })

    it('selecting Sponsored Brand deselects Free Product Trial', async () => {
      const user = await goToStep2()
      await user.click(screen.getByDisplayValue('sponsored-brand'))
      expect((screen.getByDisplayValue('sponsored-brand') as HTMLInputElement).checked).toBe(true)
      expect((screen.getByDisplayValue('free-trial') as HTMLInputElement).checked).toBe(false)
    })

    it('radio group has radio-group class', async () => {
      await goToStep2()
      expect(document.querySelector('.radio-group')).toBeInTheDocument()
    })

    it('each radio option has radio-option class', async () => {
      await goToStep2()
      const options = document.querySelectorAll('.radio-option')
      expect(options.length).toBe(5)
    })

    it('selected radio option has selected class', async () => {
      await goToStep2()
      const freeTrialLabel = screen.getByDisplayValue('free-trial').closest('label')!
      expect(freeTrialLabel.classList.contains('selected')).toBe(true)
    })

    it('non-selected radio options do not have selected class', async () => {
      await goToStep2()
      const sponsoredLabel = screen.getByDisplayValue('sponsored-brand').closest('label')!
      expect(sponsoredLabel.classList.contains('selected')).toBe(false)
    })
  })
})
