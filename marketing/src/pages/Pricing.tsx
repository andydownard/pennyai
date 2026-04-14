import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useOnScreen } from '../hooks/useOnScreen'
import { useDocTitle } from '../hooks/useDocTitle'
import '../styles/Pricing.css'

export default function Pricing() {
  useDocTitle('Pricing', 'Pay for trials, not impressions. Free product trials, targeted swaps, and traditional ad placements starting at $25/day.')
  const [modelsRef, modelsVisible] = useOnScreen()
  const [adModelsRef, adModelsVisible] = useOnScreen()
  const [detailsRef, detailsVisible] = useOnScreen()
  const [faqSectionRef, faqSectionVisible] = useOnScreen()
  const [ctaSectionRef, ctaSectionVisible] = useOnScreen()

  const [openFaq, setOpenFaq] = useState<number | null>(null)

  const faqs = [
    {
      q: "What's the minimum budget?",
      a: "Daily budgets start at $25/day. For total campaign budgets, the minimum is $100. You can increase or decrease your budget at any time."
    },
    {
      q: "How does billing work?",
      a: "Submitting your brief here doesn't collect a card. Once we set up your account at brands.groflo.io, you fund a balance and your campaign runs against it. You're billed when your spend reaches $500, or at the end of each month — whichever comes first. View your running balance and invoices from your brand dashboard."
    },
    {
      q: "Can I pause my campaign?",
      a: "Yes, you can pause, resume, or stop your campaign at any time from your dashboard. You're only charged for impressions or clicks that have already been delivered."
    },
    {
      q: "What payment methods do you accept?",
      a: "From your brand dashboard at brands.groflo.io, we accept all major credit and debit cards (Visa, Mastercard, American Express, Discover) through our secure Stripe-powered payment system."
    },
    {
      q: "Are there any contracts or commitments?",
      a: "No. Groflo advertising is completely self-serve with no contracts, no minimum commitments, and no cancellation fees. You're in full control."
    },
    {
      q: "What analytics do I get?",
      a: "Track impressions, clicks, list adds, and budget spend in real time from your campaign dashboard. See which categories and regions perform best, and adjust your campaigns accordingly."
    },
    {
      q: "How do free trial deals work?",
      a: "You set the retail value of your product. When a shopper adds a competing item, Penny offers yours for free. If they accept, you're charged the product value you set. The shopper picks it up in-store at no cost to them."
    },
    {
      q: "What's a targeted swap?",
      a: "A targeted swap lets you offer a discount when a shopper adds a competitor's product to their list. You set the discount amount. If they accept the swap, you're charged the discount and they buy your product instead."
    }
  ]

  return (
    <>
      {/* ===== HERO ===== */}
      <section className="pricing-hero">
        <div className="section-inner">
          <p className="section-label">Pricing</p>
          <h1 className="section-title">Pay for trials, not impressions</h1>
          <p className="section-subtitle" style={{ marginBottom: 0 }}>
            Choose how you want to reach shoppers — free trials, targeted swaps, or traditional ad placements. Start with $25/day.
          </p>
        </div>
      </section>

      {/* ===== DEAL PRICING ===== */}
      <section className={`pricing-models fade-section${modelsVisible ? ' visible' : ''}`} ref={modelsRef}>
        <div className="section-inner">
          <p className="section-label" style={{ textAlign: 'center' }}>Deal Campaigns</p>
          <h2 className="section-title" style={{ textAlign: 'center' }}>Get your product in their hands</h2>
          <p className="section-subtitle" style={{ textAlign: 'center', margin: '0 auto 3rem' }}>
            Pay when shoppers accept your deal — not when they scroll past it.
          </p>
          <div className="pricing-cards">
            <div className="pricing-card pricing-card-featured">
              <div className="pricing-card-badge">Most Popular</div>
              <h3>Free Trial</h3>
              <p className="pricing-card-desc">Cost Per Accepted Trial</p>
              <div className="pricing-card-price">
                <span className="price-amount">You set</span>
                <span className="price-unit">the product value</span>
              </div>
              <ul className="pricing-card-features">
                <li>Cover the full cost of your product for the shopper</li>
                <li>Pay only when a shopper accepts your free trial</li>
                <li>Best for emerging brands seeking first-time customers</li>
                <li>Track repeat purchases after the free trial</li>
              </ul>
              <Link to="/campaign/signup" className="btn-primary" style={{ width: '100%', justifyContent: 'center' }}>
                Create a Free Deal →
              </Link>
            </div>
            <div className="pricing-card">
              <div className="pricing-card-badge">Challenger</div>
              <h3>Targeted Swap</h3>
              <p className="pricing-card-desc">Cost Per Accepted Swap</p>
              <div className="pricing-card-price">
                <span className="price-amount">You set</span>
                <span className="price-unit">the discount amount</span>
              </div>
              <ul className="pricing-card-features">
                <li>Offer a discount when someone adds a competitor's product</li>
                <li>Pay only when a shopper swaps to your brand</li>
                <li>Best for brands competing in established categories</li>
                <li>Precision targeting at the moment of purchase decision</li>
              </ul>
              <Link to="/campaign/signup" className="btn-primary" style={{ width: '100%', justifyContent: 'center' }}>
                Create a Swap →
              </Link>
            </div>
          </div>
        </div>
      </section>

      {/* ===== AD PRICING ===== */}
      <section className={`pricing-ads fade-section${adModelsVisible ? ' visible' : ''}`} ref={adModelsRef}>
        <div className="section-inner">
          <p className="section-label" style={{ textAlign: 'center' }}>Ad Campaigns</p>
          <h2 className="section-title" style={{ textAlign: 'center' }}>Traditional ad placements</h2>
          <p className="section-subtitle" style={{ textAlign: 'center', margin: '0 auto 3rem' }}>
            Promote your brand to shoppers building their lists. Pay per impression or click.
          </p>
          <div className="pricing-cards">
            <div className="pricing-card">
              <div className="pricing-card-badge">Awareness</div>
              <h3>CPM</h3>
              <p className="pricing-card-desc">Cost Per Thousand Impressions</p>
              <div className="pricing-card-price">
                <span className="price-amount">$6.00</span>
                <span className="price-unit">starting CPM</span>
              </div>
              <ul className="pricing-card-features">
                <li>Your product shown to shoppers building lists</li>
                <li>Pay per 1,000 impressions delivered</li>
                <li>Best for brand awareness campaigns</li>
                <li>Reach shoppers across all categories</li>
              </ul>
              <Link to="/campaign/signup" className="btn-primary" style={{ width: '100%', justifyContent: 'center' }}>
                Get Started →
              </Link>
            </div>
            <div className="pricing-card">
              <div className="pricing-card-badge">Performance</div>
              <h3>CPC</h3>
              <p className="pricing-card-desc">Cost Per Click</p>
              <div className="pricing-card-price">
                <span className="price-amount">$0.50</span>
                <span className="price-unit">starting CPC</span>
              </div>
              <ul className="pricing-card-features">
                <li>Pay only when shoppers click on your product</li>
                <li>Pay only for engaged shoppers who tap to view your product</li>
                <li>Best for driving product consideration</li>
                <li>Optimize bids based on real-time performance</li>
              </ul>
              <Link to="/campaign/signup" className="btn-primary" style={{ width: '100%', justifyContent: 'center' }}>
                Get Started →
              </Link>
            </div>
          </div>
        </div>
      </section>

      {/* ===== BUDGET DETAILS ===== */}
      <section className={`pricing-details fade-section${detailsVisible ? ' visible' : ''}`} ref={detailsRef}>
        <div className="section-inner">
          <h2 className="section-title" style={{ textAlign: 'center' }}>How billing works</h2>
          <p className="section-subtitle" style={{ textAlign: 'center', margin: '0 auto 4rem' }}>
            Simple, transparent, and completely in your control.
          </p>
          <div className="billing-steps">
            <div className="billing-step">
              <div className="billing-step-number">1</div>
              <h3>Set a budget</h3>
              <p>Choose a daily budget (min $25/day) or a total campaign budget (min $100). You decide how much to spend.</p>
            </div>
            <div className="billing-step">
              <div className="billing-step-number">2</div>
              <h3>Your ads run</h3>
              <p>Your products appear as suggestions to shoppers. You're charged per impression (CPM) or per click (CPC), depending on your model.</p>
            </div>
            <div className="billing-step">
              <div className="billing-step-number">3</div>
              <h3>Adjust anytime</h3>
              <p>Increase your budget, pause your campaign, or stop entirely — no penalties, no contracts, no commitment.</p>
            </div>
          </div>
        </div>
      </section>

      {/* ===== FAQ ===== */}
      <section className={`pricing-faq fade-section${faqSectionVisible ? ' visible' : ''}`} ref={faqSectionRef}>
        <div className="section-inner">
          <h2 className="section-title" style={{ textAlign: 'center' }}>Frequently asked questions</h2>
          <div className="faq-list">
            {faqs.map((faq, i) => (
              <div key={faq.q} className={`faq-item${openFaq === i ? ' open' : ''}`}>
                <button
                  className="faq-question"
                  onClick={() => setOpenFaq(openFaq === i ? null : i)}
                  aria-expanded={openFaq === i}
                  aria-controls={`faq-answer-${i}`}
                >
                  <span>{faq.q}</span>
                  <span className="faq-toggle" aria-hidden="true">{openFaq === i ? '−' : '+'}</span>
                </button>
                {openFaq === i && (
                  <div className="faq-answer" id={`faq-answer-${i}`} role="region">
                    <p>{faq.a}</p>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ===== CTA ===== */}
      <section className={`cta-section fade-section${ctaSectionVisible ? ' visible' : ''}`} ref={ctaSectionRef}>
        <div className="cta-inner">
          <img src="/logo-white.svg" alt="Groflo" className="cta-logo" loading="lazy" />
          <h2>Ready to get your product on their list?</h2>
          <p>
            Create a free deal or targeted swap today. No contracts, no minimums beyond $25/day. Pay only when shoppers accept.
          </p>
          <div className="cta-actions">
            <Link to="/campaign/signup" className="btn-cta-primary">Create a Free Deal →</Link>
            <Link to="/for-brands" className="btn-cta-secondary">Learn More</Link>
          </div>
        </div>
      </section>
    </>
  )
}
