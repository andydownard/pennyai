import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
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

const BRAND_PORTAL_URL = (import.meta.env.VITE_BRAND_PORTAL_URL as string | undefined) || 'https://brands.groflo.io'
const LEAD_ENDPOINT = '/.netlify/functions/campaign-lead'

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

function newIdempotencyKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  // Fallback for environments without crypto.randomUUID (very old browsers / test stubs)
  const bytes = new Uint8Array(16)
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    crypto.getRandomValues(bytes)
  } else {
    for (let i = 0; i < 16; i++) bytes[i] = Math.floor(Math.random() * 256)
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

interface SubmitFormProps {
  account: AccountData
  campaign: CampaignData
  agreedToTerms: boolean
  onAgreedChange: (v: boolean) => void
  onSuccess: (leadId: number, redirectUrl: string) => void
  idempotencyKey: string
}

function SubmitForm({ account, campaign, agreedToTerms, onAgreedChange, onSuccess, idempotencyKey }: SubmitFormProps) {
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (submitting) return
    if (!agreedToTerms) {
      setError('Please agree to the Terms of Service and Privacy Policy.')
      return
    }
    setSubmitting(true)
    setError(null)

    try {
      const res = await fetch(LEAD_ENDPOINT, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Idempotency-Key': idempotencyKey,
        },
        body: JSON.stringify({ ...account, ...campaign }),
      })
      const json = await res.json().catch(() => ({}))
      if (!res.ok) {
        setError(json.error || 'Submission failed. Please try again.')
        setSubmitting(false)
        return
      }
      onSuccess(json.leadId, json.redirectUrl)
    } catch (err) {
      console.error('Campaign lead submit failed:', err)
      setError('Network error. Please check your connection and try again.')
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} aria-label="Submit campaign brief">
      <div className="form-group" style={{ margin: '1.5rem 0' }}>
        <label className="checkbox-option" style={{ cursor: 'pointer' }}>
          <input
            type="checkbox"
            checked={agreedToTerms}
            onChange={e => onAgreedChange(e.target.checked)}
            style={{ display: 'inline', width: 'auto', marginRight: '0.5rem' }}
          />
          <span>I agree to Groflo's <Link to="/terms" target="_blank">Terms of Service</Link> and <Link to="/privacy" target="_blank">Privacy Policy</Link></span>
        </label>
      </div>
      {error && <div className="payment-error" role="alert">{error}</div>}
      <button type="submit" disabled={submitting || !agreedToTerms} className="btn-primary signup-submit-btn">
        {submitting ? 'Submitting...' : 'Submit Campaign Brief'}
      </button>
      <p className="payment-note">
        We'll review your brief and email you within one business day at <strong>{account.email}</strong> with a link to finish setup at brands.groflo.io. No card required to submit.
      </p>
    </form>
  )
}

export default function CampaignSignup() {
  useDocTitle('Create Campaign', 'Create a free deal or targeted swap campaign on Groflo. Reach families at the moment they build their grocery list.')
  const [currentStep, setCurrentStep] = useState(1)
  const [validationError, setValidationError] = useState<string | null>(null)
  const [agreedToTerms, setAgreedToTerms] = useState(false)
  const [submittedLeadId, setSubmittedLeadId] = useState<number | null>(null)
  const [submittedRedirectUrl, setSubmittedRedirectUrl] = useState<string>(BRAND_PORTAL_URL)
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

  // Stable idempotency key for this signup session — same key across retries
  // means the backend deduplicates instead of creating duplicate leads.
  const idempotencyKey = useMemo(() => newIdempotencyKey(), [])

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

    const minBudget = campaignData.budgetType === 'daily' ? 25 : 100
    if (campaignData.budgetAmount < minBudget || isNaN(campaignData.budgetAmount)) {
      setValidationError(`Budget must be at least $${minBudget}.`)
      return
    }

    if (campaignData.startDate < todayStr) {
      setValidationError('Start date must be today or later.')
      return
    }
    if (campaignData.endDate <= campaignData.startDate) {
      setValidationError('End date must be after the start date.')
      return
    }

    if (campaignData.categories.length === 0) {
      setValidationError('Please select at least one target category.')
      return
    }

    setCurrentStep(3)
    window.scrollTo({ top: 0, behavior: 'instant' as ScrollBehavior })
  }

  const handleSubmitSuccess = (leadId: number, redirectUrl: string) => {
    setSubmittedLeadId(leadId)
    if (redirectUrl) setSubmittedRedirectUrl(redirectUrl)
    setCurrentStep(4)
    window.scrollTo({ top: 0, behavior: 'instant' as ScrollBehavior })
  }

  const steps = ['Account', 'Campaign', 'Review', 'Confirmation']

  return (
    <section className="signup-section">
      <div className="signup-container">
        {/* Step indicator */}
        <div className="step-indicator">
          {steps.map((label, i) => (
            <div key={label} className={`step-dot-wrapper${currentStep > i + 1 ? ' completed' : ''}${currentStep === i + 1 ? ' active' : ''}`}>
              <div className="step-dot">
                {currentStep > i + 1 ? '\u2713' : i + 1}
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
                  Continue to Review →
                </button>
              </div>
            </form>
          </div>
        )}

        {/* Step 3: Review & submit */}
        {currentStep === 3 && (
          <div className="signup-step">
            <h2>Review & submit</h2>
            <p className="signup-step-desc">Review your campaign details and submit your brief. No payment is collected here — the brand portal at brands.groflo.io handles deposits and goes-live.</p>

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

            <SubmitForm
              account={accountData}
              campaign={campaignData}
              agreedToTerms={agreedToTerms}
              onAgreedChange={setAgreedToTerms}
              onSuccess={handleSubmitSuccess}
              idempotencyKey={idempotencyKey}
            />

            <button type="button" className="btn-secondary" style={{ width: '100%', justifyContent: 'center', marginTop: '1rem' }} onClick={() => { setCurrentStep(2); window.scrollTo({ top: 0, behavior: 'instant' as ScrollBehavior }) }}>
              ← Back to Campaign Details
            </button>
          </div>
        )}

        {/* Step 4: Confirmation */}
        {currentStep === 4 && (
          <div className="signup-step confirmation-step">
            <img src="/logo.svg" alt="Groflo" className="confirmation-logo" />
            <div className="confirmation-icon">{'\u2713'}</div>
            <h2>Campaign brief submitted</h2>
            <p className="signup-step-desc">
              Thanks! Your brief for <strong>{campaignData.creativeName}</strong> is in. We'll email <strong>{accountData.email}</strong> within one business day with a link to set up your brand account at <strong>brands.groflo.io</strong>, fund your balance, and approve your campaign to go live.
              {submittedLeadId !== null && (
                <> Reference: <code>#{submittedLeadId}</code>.</>
              )}
            </p>

            <div className="campaign-summary">
              <h3>Brief Details</h3>
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
                <span className="status-badge">Brief Received</span>
              </div>
            </div>

            <div className="confirmation-actions">
              <a href={submittedRedirectUrl} className="btn-primary" style={{ justifyContent: 'center' }} rel="noopener">
                Visit Brand Portal →
              </a>
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
