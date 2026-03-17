import { useState, useRef } from 'react'
import { Link } from 'react-router-dom'
import { loadStripe } from '@stripe/stripe-js'
import { Elements, CardElement, useStripe, useElements } from '@stripe/react-stripe-js'
import { useDocTitle } from '../hooks/useDocTitle'
import '../styles/CampaignSignup.css'

interface AccountData {
  fullName: string
  companyName: string
  email: string
  website: string
}

interface CampaignData {
  adType: 'free-trial' | 'targeted-swap' | 'promoted-suggestion' | 'sponsored-brand' | 'list-placement'
  categories: string[]
  geography: string
  budgetType: 'daily' | 'total'
  budgetAmount: number
  startDate: string
  endDate: string
  creativeName: string
  creativeDescription: string
}

const stripeKey = import.meta.env.VITE_STRIPE_PUBLISHABLE_KEY as string | undefined
const stripePromise = stripeKey ? loadStripe(stripeKey) : null

const CATEGORIES = [
  'Dairy', 'Produce', 'Meat & Seafood', 'Snacks', 'Beverages',
  'Frozen', 'Bakery', 'Pantry', 'Health & Beauty', 'Household'
]

const AD_TYPES = [
  { value: 'free-trial', label: 'Free Product Trial', desc: 'Cover the full cost — families try your product for free' },
  { value: 'targeted-swap', label: 'Targeted Swap', desc: 'Offer a discount when someone adds a competitor\'s product' },
  { value: 'promoted-suggestion', label: 'Promoted Suggestion', desc: 'Penny mentions your product in conversation' },
  { value: 'sponsored-brand', label: 'Sponsored Brand', desc: 'Brand highlighted in category mentions' },
  { value: 'list-placement', label: 'List Placement', desc: 'Product added as a labeled suggestion on lists' },
] as const

function PaymentForm({ onSuccess, amount }: { onSuccess: () => void; amount: number }) {
  const stripe = useStripe()
  const elements = useElements()
  const [processing, setProcessing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const processingRef = useRef(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!stripe || !elements || processingRef.current) return

    processingRef.current = true
    setProcessing(true)
    setError(null)

    // TODO: When backend is ready, replace this with:
    // const res = await fetch('/api/create-payment-intent', {
    //   method: 'POST',
    //   headers: { 'Content-Type': 'application/json' },
    //   body: JSON.stringify({ amount: Math.round(amount * 100) })
    // })
    // const { clientSecret } = await res.json()
    // const { error, paymentIntent } = await stripe.confirmCardPayment(clientSecret, {
    //   payment_method: { card: elements.getElement(CardElement)! }
    // })
    // if (error) { setError(error.message || 'Payment failed'); setProcessing(false); processingRef.current = false; return }
    // if (paymentIntent?.status === 'succeeded') onSuccess()

    // Mock success for now:
    setTimeout(() => {
      setProcessing(false)
      processingRef.current = false
      onSuccess()
    }, 1500)
  }

  return (
    <form onSubmit={handleSubmit}>
      <div className="form-group">
        <label className="form-label">Credit or debit card</label>
        <div className="stripe-card-wrapper">
          <CardElement
            options={{
              style: {
                base: {
                  fontSize: '16px',
                  fontFamily: 'Inter, system-ui, sans-serif',
                  color: '#1a1a1a',
                  '::placeholder': { color: '#8f8f8f' },
                },
              },
            }}
          />
        </div>
      </div>
      {error && <div className="payment-error">{error}</div>}
      <button type="submit" disabled={!stripe || processing} className="btn-primary signup-submit-btn">
        {processing ? 'Processing...' : `Authorize $${amount.toFixed(2)} & Submit for Review`}
      </button>
      <p className="payment-note">
        Your card will be authorized but not charged until your campaign is approved and begins running. You can pause or cancel anytime.
      </p>
    </form>
  )
}

function MockPaymentForm({ onSuccess }: { onSuccess: () => void }) {
  const [processing, setProcessing] = useState(false)
  const processingRef = useRef(false)

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (processingRef.current) return
    processingRef.current = true
    setProcessing(true)
    setTimeout(() => {
      setProcessing(false)
      processingRef.current = false
      onSuccess()
    }, 1500)
  }

  return (
    <form onSubmit={handleSubmit}>
      <div className="stripe-notice">
        <p><strong>Payment setup coming soon.</strong> Submit your campaign details and we'll contact you to finalize payment.</p>
      </div>
      <button type="submit" disabled={processing} className="btn-primary signup-submit-btn">
        {processing ? 'Submitting...' : 'Submit Campaign for Review'}
      </button>
    </form>
  )
}

export default function CampaignSignup() {
  useDocTitle('Create Campaign', 'Create a free deal or targeted swap campaign on Thriftly. Reach families at the moment they build their grocery list.')
  const [currentStep, setCurrentStep] = useState(1)
  const [validationError, setValidationError] = useState<string | null>(null)
  const [agreedToTerms, setAgreedToTerms] = useState(false)
  const [accountData, setAccountData] = useState<AccountData>({
    fullName: '', companyName: '', email: '', website: ''
  })
  const [campaignData, setCampaignData] = useState<CampaignData>({
    adType: 'free-trial',
    categories: [],
    geography: 'national',
    budgetType: 'daily',
    budgetAmount: 25,
    startDate: '',
    endDate: '',
    creativeName: '',
    creativeDescription: ''
  })

  const toggleCategory = (cat: string) => {
    setCampaignData(d => ({
      ...d,
      categories: d.categories.includes(cat)
        ? d.categories.filter(c => c !== cat)
        : [...d.categories, cat]
    }))
  }

  const todayStr = new Date().toISOString().split('T')[0]

  const handleAccountSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setCurrentStep(2)
    window.scrollTo({ top: 0, behavior: 'instant' as ScrollBehavior })
  }

  const handleCampaignSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setValidationError(null)

    // Validate budget
    const minBudget = campaignData.budgetType === 'daily' ? 25 : 100
    if (campaignData.budgetAmount < minBudget || isNaN(campaignData.budgetAmount)) {
      setValidationError(`Budget must be at least $${minBudget}.`)
      return
    }

    // Validate dates
    if (campaignData.startDate < todayStr) {
      setValidationError('Start date must be today or later.')
      return
    }
    if (campaignData.endDate <= campaignData.startDate) {
      setValidationError('End date must be after the start date.')
      return
    }

    // Validate categories
    if (campaignData.categories.length === 0) {
      setValidationError('Please select at least one target category.')
      return
    }

    setCurrentStep(3)
    window.scrollTo({ top: 0, behavior: 'instant' as ScrollBehavior })
  }

  const handlePaymentSuccess = () => {
    localStorage.setItem('thriftly_campaign', JSON.stringify({
      accountData,
      campaignData,
      createdAt: new Date().toISOString()
    }))
    setCurrentStep(4)
    window.scrollTo({ top: 0, behavior: 'instant' as ScrollBehavior })
  }

  const steps = ['Account', 'Campaign', 'Payment', 'Confirmation']

  return (
    <section className="signup-section">
      <div className="signup-container">
        {/* Step indicator */}
        <div className="step-indicator">
          {steps.map((label, i) => (
            <div key={label} className={`step-dot-wrapper${currentStep > i + 1 ? ' completed' : ''}${currentStep === i + 1 ? ' active' : ''}`}>
              <div className="step-dot">
                {currentStep > i + 1 ? '✓' : i + 1}
              </div>
              <span className="step-dot-label">{label}</span>
            </div>
          ))}
        </div>

        {/* Step 1: Account */}
        {currentStep === 1 && (
          <div className="signup-step">
            <h2>Create your brand account</h2>
            <p className="signup-step-desc">Tell us about your company to get started.</p>
            <form onSubmit={handleAccountSubmit}>
              <div className="form-group">
                <label className="form-label" htmlFor="fullName">Full name</label>
                <input
                  className="form-input"
                  id="fullName"
                  type="text"
                  required
                  value={accountData.fullName}
                  onChange={e => setAccountData({ ...accountData, fullName: e.target.value })}
                  placeholder="Jane Smith"
                />
              </div>
              <div className="form-group">
                <label className="form-label" htmlFor="companyName">Company name</label>
                <input
                  className="form-input"
                  id="companyName"
                  type="text"
                  required
                  value={accountData.companyName}
                  onChange={e => setAccountData({ ...accountData, companyName: e.target.value })}
                  placeholder="Acme Foods Inc."
                />
              </div>
              <div className="form-group">
                <label className="form-label" htmlFor="email">Work email</label>
                <input
                  className="form-input"
                  id="email"
                  type="email"
                  required
                  value={accountData.email}
                  onChange={e => setAccountData({ ...accountData, email: e.target.value })}
                  placeholder="jane@acmefoods.com"
                />
              </div>
              <div className="form-group">
                <label className="form-label" htmlFor="website">Company website <span style={{ fontWeight: 400, color: 'var(--gray-500)' }}>(optional)</span></label>
                <input
                  className="form-input"
                  id="website"
                  type="text"
                  value={accountData.website}
                  onChange={e => setAccountData({ ...accountData, website: e.target.value })}
                  placeholder="acmefoods.com"
                />
              </div>
              <button type="submit" className="btn-primary signup-submit-btn">
                Continue →
              </button>
            </form>
          </div>
        )}

        {/* Step 2: Campaign */}
        {currentStep === 2 && (
          <div className="signup-step">
            <h2>Configure your campaign</h2>
            <p className="signup-step-desc">Choose your campaign type, targeting, and budget.</p>
            <form onSubmit={handleCampaignSubmit}>
              {validationError && (
                <div className="payment-error" style={{ marginBottom: '1rem' }}>{validationError}</div>
              )}

              {/* Ad Type */}
              <div className="form-group">
                <label className="form-label">Campaign type</label>
                <div className="radio-group">
                  {AD_TYPES.map(type => (
                    <label key={type.value} className={`radio-option${campaignData.adType === type.value ? ' selected' : ''}`}>
                      <input
                        type="radio"
                        name="adType"
                        value={type.value}
                        checked={campaignData.adType === type.value}
                        onChange={e => setCampaignData({ ...campaignData, adType: e.target.value as CampaignData['adType'] })}
                      />
                      <div>
                        <strong>{type.label}</strong>
                        <span>{type.desc}</span>
                      </div>
                    </label>
                  ))}
                </div>
              </div>

              {/* Categories */}
              <div className="form-group">
                <label className="form-label">Target categories <span style={{ fontWeight: 400, color: 'var(--gray-500)' }}>(select at least one)</span></label>
                <div className="checkbox-group">
                  {CATEGORIES.map(cat => (
                    <label key={cat} className={`checkbox-option${campaignData.categories.includes(cat) ? ' selected' : ''}`}>
                      <input
                        type="checkbox"
                        checked={campaignData.categories.includes(cat)}
                        onChange={() => toggleCategory(cat)}
                      />
                      <span>{cat}</span>
                    </label>
                  ))}
                </div>
              </div>

              {/* Geography */}
              <div className="form-group">
                <label className="form-label" htmlFor="geography">Geographic targeting</label>
                <select
                  className="form-input"
                  id="geography"
                  value={campaignData.geography}
                  onChange={e => setCampaignData({ ...campaignData, geography: e.target.value })}
                >
                  <option value="national">National (all regions)</option>
                  <option value="northeast">Northeast</option>
                  <option value="southeast">Southeast</option>
                  <option value="midwest">Midwest</option>
                  <option value="southwest">Southwest</option>
                  <option value="west">West Coast</option>
                </select>
              </div>

              {/* Budget */}
              <div className="form-group">
                <label className="form-label">Budget type</label>
                <div className="budget-toggle">
                  <button
                    type="button"
                    className={`budget-toggle-btn${campaignData.budgetType === 'daily' ? ' active' : ''}`}
                    onClick={() => setCampaignData(d => ({ ...d, budgetType: 'daily', budgetAmount: Math.max(d.budgetAmount, 25) }))}
                  >
                    Daily
                  </button>
                  <button
                    type="button"
                    className={`budget-toggle-btn${campaignData.budgetType === 'total' ? ' active' : ''}`}
                    onClick={() => setCampaignData(d => ({ ...d, budgetType: 'total', budgetAmount: Math.max(d.budgetAmount, 100) }))}
                  >
                    Total Campaign
                  </button>
                </div>
              </div>
              <div className="form-group">
                <label className="form-label" htmlFor="budget">
                  {campaignData.budgetType === 'daily' ? 'Daily budget' : 'Total campaign budget'} (min ${campaignData.budgetType === 'daily' ? '25' : '100'})
                </label>
                <div className="budget-input-wrapper">
                  <span className="budget-currency">$</span>
                  <input
                    className="form-input budget-input"
                    id="budget"
                    type="number"
                    required
                    min={campaignData.budgetType === 'daily' ? 25 : 100}
                    step="1"
                    value={campaignData.budgetAmount}
                    onChange={e => setCampaignData({ ...campaignData, budgetAmount: Number(e.target.value) })}
                  />
                </div>
              </div>

              {/* Dates */}
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label" htmlFor="startDate">Start date</label>
                  <input
                    className="form-input"
                    id="startDate"
                    type="date"
                    required
                    min={todayStr}
                    value={campaignData.startDate}
                    onChange={e => setCampaignData({ ...campaignData, startDate: e.target.value })}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label" htmlFor="endDate">End date</label>
                  <input
                    className="form-input"
                    id="endDate"
                    type="date"
                    required
                    min={campaignData.startDate || todayStr}
                    value={campaignData.endDate}
                    onChange={e => setCampaignData({ ...campaignData, endDate: e.target.value })}
                  />
                </div>
              </div>

              {/* Creative */}
              <div className="form-group">
                <label className="form-label" htmlFor="creativeName">Product / brand name</label>
                <input
                  className="form-input"
                  id="creativeName"
                  type="text"
                  required
                  value={campaignData.creativeName}
                  onChange={e => setCampaignData({ ...campaignData, creativeName: e.target.value })}
                  placeholder="e.g. Organic Valley Whole Milk"
                />
              </div>
              <div className="form-group">
                <label className="form-label" htmlFor="creativeDesc">Short description (max 140 characters)</label>
                <textarea
                  className="form-input form-textarea"
                  id="creativeDesc"
                  required
                  maxLength={140}
                  rows={3}
                  value={campaignData.creativeDescription}
                  onChange={e => setCampaignData({ ...campaignData, creativeDescription: e.target.value })}
                  placeholder="Farm-fresh organic milk from pasture-raised cows"
                />
                <span className="char-count">{campaignData.creativeDescription.length}/140</span>
              </div>

              <div className="signup-actions">
                <button type="button" className="btn-secondary" onClick={() => { setCurrentStep(1); window.scrollTo({ top: 0, behavior: 'instant' as ScrollBehavior }) }}>
                  ← Back
                </button>
                <button type="submit" className="btn-primary">
                  Continue to Payment →
                </button>
              </div>
            </form>
          </div>
        )}

        {/* Step 3: Payment */}
        {currentStep === 3 && (
          <div className="signup-step">
            <h2>Review & submit</h2>
            <p className="signup-step-desc">Review your campaign details and submit for review.</p>

            {/* Campaign Summary */}
            <div className="campaign-summary">
              <h3>Campaign Summary</h3>
              <div className="summary-row">
                <span>Company</span>
                <span>{accountData.companyName}</span>
              </div>
              <div className="summary-row">
                <span>Campaign type</span>
                <span>{AD_TYPES.find(t => t.value === campaignData.adType)?.label}</span>
              </div>
              <div className="summary-row">
                <span>Categories</span>
                <span>{campaignData.categories.join(', ')}</span>
              </div>
              <div className="summary-row">
                <span>Geography</span>
                <span style={{ textTransform: 'capitalize' }}>{campaignData.geography}</span>
              </div>
              <div className="summary-row">
                <span>Budget</span>
                <span>${campaignData.budgetAmount}/{campaignData.budgetType === 'daily' ? 'day' : 'total'}</span>
              </div>
              <div className="summary-row">
                <span>Dates</span>
                <span>{campaignData.startDate} — {campaignData.endDate}</span>
              </div>
              <div className="summary-row">
                <span>Product</span>
                <span>{campaignData.creativeName}</span>
              </div>
            </div>

            {/* Terms Agreement */}
            <div className="form-group" style={{ margin: '1.5rem 0' }}>
              <label className="checkbox-option" style={{ cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={agreedToTerms}
                  onChange={e => setAgreedToTerms(e.target.checked)}
                  style={{ display: 'inline', width: 'auto', marginRight: '0.5rem' }}
                />
                <span>I agree to Thriftly's <Link to="/terms" target="_blank">Terms of Service</Link> and <Link to="/privacy" target="_blank">Privacy Policy</Link></span>
              </label>
            </div>

            {/* Payment Form */}
            {agreedToTerms && (
              stripePromise ? (
                <Elements stripe={stripePromise} options={{
                  appearance: {
                    theme: 'stripe',
                    variables: {
                      colorPrimary: '#35644F',
                      fontFamily: 'Inter, system-ui, sans-serif',
                      borderRadius: '8px',
                    },
                  },
                }}>
                  <PaymentForm onSuccess={handlePaymentSuccess} amount={campaignData.budgetAmount} />
                </Elements>
              ) : (
                <MockPaymentForm onSuccess={handlePaymentSuccess} />
              )
            )}

            <button type="button" className="btn-secondary" style={{ width: '100%', justifyContent: 'center', marginTop: '1rem' }} onClick={() => { setCurrentStep(2); window.scrollTo({ top: 0, behavior: 'instant' as ScrollBehavior }) }}>
              ← Back to Campaign Details
            </button>
          </div>
        )}

        {/* Step 4: Confirmation */}
        {currentStep === 4 && (
          <div className="signup-step confirmation-step">
            <img src="/logo.svg" alt="thriftly" className="confirmation-logo" />
            <div className="confirmation-icon">✓</div>
            <h2>Campaign submitted for review</h2>
            <p className="signup-step-desc">
              Your campaign for <strong>{campaignData.creativeName}</strong> has been submitted! We review every campaign to make sure it aligns with our values and is genuinely useful to shoppers. You'll receive a confirmation email at <strong>{accountData.email}</strong> when your campaign goes live.
            </p>

            <div className="campaign-summary">
              <h3>Campaign Details</h3>
              <div className="summary-row">
                <span>Campaign</span>
                <span>{campaignData.creativeName}</span>
              </div>
              <div className="summary-row">
                <span>Format</span>
                <span>{AD_TYPES.find(t => t.value === campaignData.adType)?.label}</span>
              </div>
              <div className="summary-row">
                <span>Budget</span>
                <span>${campaignData.budgetAmount}/{campaignData.budgetType === 'daily' ? 'day' : 'total'}</span>
              </div>
              <div className="summary-row">
                <span>Status</span>
                <span className="status-badge">Under Review</span>
              </div>
            </div>

            <div className="confirmation-actions">
              <Link to="/" className="btn-primary" style={{ justifyContent: 'center' }}>
                Return to Home →
              </Link>
              <Link to="/for-brands" className="btn-secondary" style={{ justifyContent: 'center' }}>
                Learn More About Ads
              </Link>
            </div>
          </div>
        )}
      </div>
    </section>
  )
}
