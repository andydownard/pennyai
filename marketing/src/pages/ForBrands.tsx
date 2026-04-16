import { Link } from 'react-router-dom'
import { useOnScreen } from '../hooks/useOnScreen'
import { useDocTitle } from '../hooks/useDocTitle'
import '../styles/ForBrands.css'

export default function ForBrands() {
  useDocTitle('For Brands', 'Put your product in their cart for free. Reach families with free trials and targeted swaps on their shopping list. Start with $25/day.')
  const [howItWorksRef, howItWorksVisible] = useOnScreen()
  const [campaignTypesRef, campaignTypesVisible] = useOnScreen()
  const [competitionRef, competitionVisible] = useOnScreen()
  const [whyGrofloRef, whyGrofloVisible] = useOnScreen()
  const [flywheelRef, flywheelVisible] = useOnScreen()
  const [tamRef, tamVisible] = useOnScreen()
  const [ctaSectionRef, ctaSectionVisible] = useOnScreen()

  return (
    <>
      {/* ===== HERO ===== */}
      <section className="brands-hero">
        <div className="section-inner">
          <p className="section-label" style={{ color: 'var(--green-300)' }}>Groflo for Brands</p>
          <h1>Put your product in their cart. <span>For free.</span></h1>
          <p className="brands-hero-subtitle">
            Families are building their grocery lists right now. With Penny, your brand shows up at the exact moment they're deciding what to buy — with free trials that get your product into their hands, not just their feed.
          </p>
          <div className="brands-hero-actions">
            <Link to="/campaign/signup" className="btn-brand-cta">Create a Free Deal →</Link>
            <Link to="/pricing" className="btn-brand-secondary">See Pricing</Link>
          </div>
          <div className="brands-hero-stats">
            <div className="brands-hero-stat">
              <strong>85%</strong>
              <span>of grocery shopping is still in-store</span>
            </div>
            <div className="brands-hero-stat">
              <strong>$0</strong>
              <span>cost to the consumer</span>
            </div>
            <div className="brands-hero-stat">
              <strong>Intent</strong>
              <span>reach shoppers actively deciding</span>
            </div>
          </div>
          <p className="brands-hero-signin">
            Already a brand partner? <a href="https://brands.groflo.io" rel="noopener" target="_blank">Sign in to your dashboard →</a>
          </p>
        </div>
      </section>

      {/* ===== HOW SWAPS WORK ===== */}
      <section className={`brands-how-section fade-section${howItWorksVisible ? ' visible' : ''}`} ref={howItWorksRef}>
        <div className="section-inner">
          <p className="section-label">How It Works</p>
          <h2 className="section-title">Your product. Their list. Zero friction.</h2>
          <p className="section-subtitle">
            Penny matches your deal to shoppers who are actively buying your category — right on their shopping list.
          </p>
          <div className="steps-grid">
            <div className="step">
              <div className="step-number">1</div>
              <h3>Create your deal</h3>
              <p>
                Offer a free trial, a targeted swap, or a discount. Set your budget, pick your category, and define who sees it. Start with as little as $25/day.
              </p>
            </div>
            <div className="step">
              <div className="step-number">2</div>
              <h3>Penny matches it</h3>
              <p>
                When a shopper adds a competitor's product to their list, Penny shows your offer: "Try this for free" or "Save $2 by switching." It's relevant, not random.
              </p>
            </div>
            <div className="step">
              <div className="step-number">3</div>
              <h3>They try your product</h3>
              <p>
                The shopper swaps to your brand, picks it up in-store, and you pay only for the trial. Track every swap, acceptance rate, and repeat purchase from your dashboard.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* ===== CAMPAIGN TYPES ===== */}
      <section className={`brands-formats fade-section${campaignTypesVisible ? ' visible' : ''}`} ref={campaignTypesRef}>
        <div className="section-inner">
          <p className="section-label">Campaign Types</p>
          <h2 className="section-title">Three ways to get into the cart</h2>
          <p className="section-subtitle">
            Pick the campaign type that fits your goals — or run them together.
          </p>
          <div className="formats-grid">
            <div className="format-card format-card-featured">
              <div className="format-icon">
                <svg width="32" height="32" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M11.8 10.9c-2.27-.59-3-1.2-3-2.15 0-1.09 1.01-1.85 2.7-1.85 1.78 0 2.44.85 2.5 2.1h2.21c-.07-1.72-1.12-3.3-3.21-3.81V3h-3v2.16c-1.94.42-3.5 1.68-3.5 3.61 0 2.31 1.91 3.46 4.7 4.13 2.5.6 3 1.48 3 2.41 0 .69-.49 1.79-2.7 1.79-2.06 0-2.87-.92-2.98-2.1h-2.2c.12 2.19 1.76 3.42 3.68 3.83V21h3v-2.15c1.95-.37 3.5-1.5 3.5-3.55 0-2.84-2.43-3.81-4.7-4.4z" fill="#35644F"/></svg>
              </div>
              <h3>Free Product Trials</h3>
              <p>Cover the full cost of your product. Families try it risk-free. You pay only when they accept the swap — the most powerful way to earn a new customer.</p>
              <div className="format-tag format-tag-featured">Best for emerging brands</div>
            </div>
            <div className="format-card">
              <div className="format-icon">
                <svg width="32" height="32" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M16 6l2.29 2.29-4.88 4.88-4-4L2 16.59 3.41 18l6-6 4 4 6.3-6.29L22 12V6h-6z" fill="#35644F"/></svg>
              </div>
              <h3>Targeted Swaps</h3>
              <p>Offer a discount to switch from a competitor. When someone adds their milk, you offer yours at $2 off. Precision targeting at the moment of decision.</p>
              <div className="format-tag">Best for category challengers</div>
            </div>
            <div className="format-card">
              <div className="format-icon">
                <svg width="32" height="32" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z" fill="#35644F"/></svg>
              </div>
              <h3>Promoted Suggestions</h3>
              <p>Penny mentions your product by name when shoppers ask about your category. Appears naturally in the conversation — awareness that feels like a recommendation.</p>
              <div className="format-tag">Best for brand awareness</div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== COMPETITIVE POSITIONING ===== */}
      <section className={`brands-competition fade-section${competitionVisible ? ' visible' : ''}`} ref={competitionRef}>
        <div className="section-inner" style={{ textAlign: 'center' }}>
          <p className="section-label">The Landscape</p>
          <h2 className="section-title" style={{ textAlign: 'center' }}>The only player in the top-right quadrant</h2>
          <p className="section-subtitle" style={{ textAlign: 'center', margin: '0 auto 3rem', maxWidth: '600px' }}>
            Instacart owns delivery lists. Ibotta does post-purchase cashback. Penny is the first platform that combines a shared shopping list with in-store deals.
          </p>
          <div className="quadrant-container">
            <div className="quadrant-y-axis">
              <div className="quadrant-axis-label">Shared list</div>
              <div className="quadrant-axis-label">No list</div>
            </div>
            <div className="quadrant-grid">
              <div className="quadrant-line-v" />
              <div className="quadrant-line-h" />
              <div className="quadrant-player quadrant-penny">
                <img src="/penny.png" alt="Penny" className="quadrant-player-img" loading="lazy" />
                <span className="quadrant-player-name">Penny</span>
              </div>
              <div className="quadrant-player quadrant-instacart">
                <div className="quadrant-player-icon" style={{ background: '#43B02A' }}>
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M7 18c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm10 0c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zM7.2 14.8l.1-.2 1.3-2.4h7.2c.75 0 1.41-.41 1.75-1.03l3.58-6.49A1 1 0 0020.3 3H5.2l-.9-2H1v2h2l3.6 7.6L5.2 13c-.7 1.3.2 2.8 1.6 2.8H19v-2H7.4c-.2 0-.3-.1-.2-.2z" fill="white"/></svg>
                </div>
                <span className="quadrant-player-name">Instacart</span>
              </div>
              <div className="quadrant-player quadrant-ibotta">
                <div className="quadrant-player-icon" style={{ background: '#E91E63' }}>
                  <span style={{ color: 'white', fontWeight: 800, fontSize: '0.75rem' }}>ib</span>
                </div>
                <span className="quadrant-player-name">Ibotta</span>
              </div>
              <div className="quadrant-player quadrant-doordash">
                <div className="quadrant-player-icon" style={{ background: '#FF3008' }}>
                  <span style={{ color: 'white', fontWeight: 800, fontSize: '0.688rem' }}>DD</span>
                </div>
                <span className="quadrant-player-name">DoorDash</span>
              </div>
            </div>
            <div className="quadrant-x-axis">
              <div className="quadrant-axis-label">Delivery</div>
              <div className="quadrant-axis-label">In-store</div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== WHY GROFLO ===== */}
      <section className={`brands-benefits-section fade-section${whyGrofloVisible ? ' visible' : ''}`} ref={whyGrofloRef}>
        <div className="section-inner">
          <div className="brands-benefits-grid">
            <div>
              <p className="section-label" style={{ color: 'var(--green-300)' }}>Why Groflo</p>
              <h2 className="brands-benefits-title">The in-store channel that doesn't exist yet</h2>
              <p className="brands-benefits-subtitle">
                Instacart proved data-driven targeting works for delivery. Penny does the same for in-store — where 85% of groceries are still bought. No slotting fees. No shelf-space battles. Just your product on their list.
              </p>
            </div>
            <div className="benefits-list">
              <div className="benefit-item">
                <div className="benefit-icon">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><circle cx="12" cy="12" r="3" fill="white"/><circle cx="12" cy="12" r="7" stroke="white" strokeWidth="2" fill="none"/><circle cx="12" cy="12" r="11" stroke="white" strokeWidth="2" fill="none"/></svg>
                </div>
                <div>
                  <h4>Moment-of-intent targeting</h4>
                  <p>Your deal shows up when a shopper adds a competitor's product — not while they're scrolling Instagram.</p>
                </div>
              </div>
              <div className="benefit-item">
                <div className="benefit-icon">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M11.8 10.9c-2.27-.59-3-1.2-3-2.15 0-1.09 1.01-1.85 2.7-1.85 1.78 0 2.44.85 2.5 2.1h2.21c-.07-1.72-1.12-3.3-3.21-3.81V3h-3v2.16c-1.94.42-3.5 1.68-3.5 3.61 0 2.31 1.91 3.46 4.7 4.13 2.5.6 3 1.48 3 2.41 0 .69-.49 1.79-2.7 1.79-2.06 0-2.87-.92-2.98-2.1h-2.2c.12 2.19 1.76 3.42 3.68 3.83V21h3v-2.15c1.95-.37 3.5-1.5 3.5-3.55 0-2.84-2.43-3.81-4.7-4.4z" fill="white"/></svg>
                </div>
                <div>
                  <h4>Pay for trials, not impressions</h4>
                  <p>Free deals and swaps mean you pay when someone actually takes your product home — not when they glance at a banner.</p>
                </div>
              </div>
              <div className="benefit-item">
                <div className="benefit-icon">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zM9 17H7v-7h2v7zm4 0h-2V7h2v10zm4 0h-2v-4h2v4z" fill="white"/></svg>
                </div>
                <div>
                  <h4>Real-time swap analytics</h4>
                  <p>Track swap acceptance rates, repeat purchases, and cost per acquired customer. See which deals convert and which to adjust.</p>
                </div>
              </div>
              <div className="benefit-item">
                <div className="benefit-icon">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M16 6l2.29 2.29-4.88 4.88-4-4L2 16.59 3.41 18l6-6 4 4 6.3-6.29L22 12V6h-6z" fill="white"/></svg>
                </div>
                <div>
                  <h4>No shelf-space required</h4>
                  <p>Emerging brands can't afford slotting fees. With Penny, you skip the shelf and go straight to the shopping list — the real point of purchase.</p>
                </div>
              </div>
              <div className="benefit-item">
                <div className="benefit-icon">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4zm-2 16l-4-4 1.41-1.41L10 14.17l6.59-6.59L18 9l-8 8z" fill="white"/></svg>
                </div>
                <div>
                  <h4>Receipt fraud protection</h4>
                  <p>Every receipt runs an 11-point authenticity check — pixel analysis, math verification, barcode validation, and watermark detection. Your budget only pays for real purchases.</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== DATA FLYWHEEL ===== */}
      <section className={`brands-flywheel fade-section${flywheelVisible ? ' visible' : ''}`} ref={flywheelRef}>
        <div className="section-inner" style={{ textAlign: 'center' }}>
          <p className="section-label">The Moat</p>
          <h2 className="section-title" style={{ textAlign: 'center' }}>A flywheel that gets stronger with every shopper</h2>
          <p className="section-subtitle" style={{ textAlign: 'center', margin: '0 auto 3rem', maxWidth: '600px' }}>
            More families using Penny means smarter suggestions, higher acceptance rates, and a more valuable audience for your brand. Everyone wins.
          </p>
          <div className="flywheel-nodes">
            <div className="flywheel-node">
              <div className="flywheel-node-icon">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M16 11c1.66 0 2.99-1.34 2.99-3S17.66 5 16 5c-1.66 0-3 1.34-3 3s1.34 3 3 3zm-8 0c1.66 0 2.99-1.34 2.99-3S9.66 5 8 5C6.34 5 5 6.34 5 8s1.34 3 3 3zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5c0-2.33-4.67-3.5-7-3.5zm8 0c-.29 0-.62.02-.97.05 1.16.84 1.97 1.97 1.97 3.45V19h6v-2.5c0-2.33-4.67-3.5-7-3.5z" fill="#35644F"/></svg>
              </div>
              <h4>Families join</h4>
              <p>Households create shared shopping lists</p>
            </div>
            <div className="flywheel-arrow" aria-hidden="true">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none"><path d="M12 4l-1.41 1.41L16.17 11H4v2h12.17l-5.58 5.59L12 20l8-8-8-8z" fill="#35644F"/></svg>
            </div>
            <div className="flywheel-node">
              <div className="flywheel-node-icon">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zM9 17H7v-7h2v7zm4 0h-2V7h2v10zm4 0h-2v-4h2v4z" fill="#35644F"/></svg>
              </div>
              <h4>Data grows</h4>
              <p>Penny learns preferences and brand choices</p>
            </div>
            <div className="flywheel-arrow" aria-hidden="true">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none"><path d="M12 4l-1.41 1.41L16.17 11H4v2h12.17l-5.58 5.59L12 20l8-8-8-8z" fill="#35644F"/></svg>
            </div>
            <div className="flywheel-node">
              <div className="flywheel-node-icon">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M11.8 10.9c-2.27-.59-3-1.2-3-2.15 0-1.09 1.01-1.85 2.7-1.85 1.78 0 2.44.85 2.5 2.1h2.21c-.07-1.72-1.12-3.3-3.21-3.81V3h-3v2.16c-1.94.42-3.5 1.68-3.5 3.61 0 2.31 1.91 3.46 4.7 4.13 2.5.6 3 1.48 3 2.41 0 .69-.49 1.79-2.7 1.79-2.06 0-2.87-.92-2.98-2.1h-2.2c.12 2.19 1.76 3.42 3.68 3.83V21h3v-2.15c1.95-.37 3.5-1.5 3.5-3.55 0-2.84-2.43-3.81-4.7-4.4z" fill="#35644F"/></svg>
              </div>
              <h4>Brands benefit</h4>
              <p>Better targeting, higher acceptance, more trials</p>
            </div>
            <div className="flywheel-arrow" aria-hidden="true">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none"><path d="M12 4l-1.41 1.41L16.17 11H4v2h12.17l-5.58 5.59L12 20l8-8-8-8z" fill="#35644F"/></svg>
            </div>
            <div className="flywheel-node">
              <div className="flywheel-node-icon">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" fill="#35644F"/></svg>
              </div>
              <h4>Families save</h4>
              <p>Free products and deals drive retention and referrals</p>
            </div>
          </div>
        </div>
      </section>

      {/* ===== IN-STORE OPPORTUNITY ===== */}
      <section className={`brands-tam fade-section${tamVisible ? ' visible' : ''}`} ref={tamRef}>
        <div className="section-inner" style={{ textAlign: 'center' }}>
          <p className="section-label">The Opportunity</p>
          <h2 className="section-title" style={{ textAlign: 'center' }}>The in-store market is 7x larger than delivery</h2>
          <p className="section-subtitle" style={{ textAlign: 'center', margin: '0 auto 3rem', maxWidth: '600px' }}>
            Instacart's Carrot Ads generate $1.2B/year reaching just 12% of grocery shoppers (delivery). Penny reaches the other 85% — in-store shoppers — with the same data-driven targeting.
          </p>
          <div className="tam-comparison">
            <div className="tam-card">
              <div className="tam-bar tam-bar-small" />
              <div className="tam-value">$1.2B</div>
              <div className="tam-label">Delivery ads (Instacart)</div>
              <div className="tam-sublabel">12% of grocery market</div>
            </div>
            <div className="tam-card tam-card-highlight">
              <div className="tam-bar tam-bar-large" />
              <div className="tam-value">$8.4B+</div>
              <div className="tam-label">In-store opportunity (Penny)</div>
              <div className="tam-sublabel">85% of grocery market</div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== BRAND SOCIAL PROOF ===== */}
      <section className="brands-proof">
        <div className="section-inner">
          <div className="brands-proof-grid">
            <div className="brands-proof-stat">
              <div className="brands-proof-value">85%</div>
              <div className="brands-proof-label">of grocery purchases are in-store — the market other ad platforms miss</div>
            </div>
            <div className="brands-proof-stat">
              <div className="brands-proof-value">7x</div>
              <div className="brands-proof-label">larger addressable market than delivery-only ad platforms</div>
            </div>
            <div className="brands-proof-stat">
              <div className="brands-proof-value">$0</div>
              <div className="brands-proof-label">cost to the consumer — brands cover the trial, families get it free</div>
            </div>
            <div className="brands-proof-stat">
              <div className="brands-proof-value">Intent</div>
              <div className="brands-proof-label">deals shown at the moment of purchase decision, not while scrolling</div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== EARLY ACCESS ===== */}
      <section className={`brands-early-access fade-section${ctaSectionVisible ? ' visible' : ''}`}>
        <div className="section-inner" style={{ textAlign: 'center' }}>
          <p className="section-label">Early Access</p>
          <h2 className="section-title" style={{ textAlign: 'center' }}>Ideal for emerging brands</h2>
          <p className="section-subtitle" style={{ textAlign: 'center', margin: '0 auto 1.5rem', maxWidth: '560px' }}>
            Can't afford slotting fees? Don't have shelf space yet? Penny gives emerging CPG brands a direct line to families — through free trials and targeted swaps on the shopping list itself.
          </p>
          <div className="early-access-spots">
            <span className="spots-dot" />
            Accepting a limited number of founding brand partners
          </div>
          <div className="early-access-perks">
            <div className="early-access-perk">
              <strong>Free trial campaigns</strong>
              <p>Get your product into hands, not just feeds. Families try it at zero cost to them.</p>
            </div>
            <div className="early-access-perk">
              <strong>No minimums to start</strong>
              <p>Launch a deal with $25/day. Scale when it works. No contracts, no commitments.</p>
            </div>
            <div className="early-access-perk">
              <strong>Direct-to-list distribution</strong>
              <p>Skip the shelf. Your product shows up on their list when they're ready to buy.</p>
            </div>
          </div>
          <form className="brands-waitlist" onSubmit={(e) => { e.preventDefault(); const input = e.currentTarget.querySelector('input'); if (input) { input.value = ''; input.placeholder = 'Thanks! We\'ll be in touch.'; } }}>
            <input type="email" placeholder="your@brand.com" required className="capture-input" aria-label="Brand email address" />
            <button type="submit" className="btn-primary capture-btn">Get Early Access</button>
          </form>
        </div>
      </section>

      {/* ===== BRAND TESTIMONIALS ===== */}
      <section className="brands-testimonials">
        <div className="section-inner">
          <p className="section-label" style={{ textAlign: 'center' }}>What Brands Say</p>
          <h2 className="section-title" style={{ textAlign: 'center' }}>Why brands are choosing Penny</h2>
          <div className="brands-testimonial-grid">
            <div className="brands-testimonial-card">
              <blockquote>
                "We couldn't afford slotting fees at major retailers. Penny put our product directly on families' shopping lists — and 40% came back to buy again."
              </blockquote>
              <div className="brands-testimonial-author">
                <strong>Sarah K.</strong>
                <span>VP Marketing, Emerging Snack Brand</span>
              </div>
            </div>
            <div className="brands-testimonial-card">
              <blockquote>
                "The targeting is unreal. Our deal shows up the exact moment someone adds a competitor's product. That's intent you can't buy on social media."
              </blockquote>
              <div className="brands-testimonial-author">
                <strong>Marcus T.</strong>
                <span>Brand Manager, Organic Dairy</span>
              </div>
            </div>
            <div className="brands-testimonial-card">
              <blockquote>
                "We ran a free trial for 2 weeks. The acceptance rate was 5x what we see on cashback apps. And we only paid for actual trials."
              </blockquote>
              <div className="brands-testimonial-author">
                <strong>Priya M.</strong>
                <span>Head of Growth, Plant-Based Foods</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== BRAND PARTNERS ===== */}
      <section className="brands-partners">
        <div className="section-inner" style={{ textAlign: 'center' }}>
          <p className="section-label">Brand Partners</p>
          <h2 className="section-title" style={{ textAlign: 'center' }}>Join the brands reaching families through Penny</h2>
          <div className="partners-placeholder">
            <div className="partner-slot">Your brand here</div>
            <div className="partner-slot">Your brand here</div>
            <div className="partner-slot">Your brand here</div>
            <div className="partner-slot">Your brand here</div>
            <div className="partner-slot">Your brand here</div>
          </div>
          <p className="partners-cta-text">
            Be among the first brands to reach families on their shopping list. <Link to="/campaign/signup">Create your first deal →</Link>
          </p>
        </div>
      </section>

      {/* ===== CTA ===== */}
      <section className={`brands-cta-section fade-section${ctaSectionVisible ? ' visible' : ''}`} ref={ctaSectionRef}>
        <div className="cta-inner">
          <img src="/logo-white.svg" alt="Groflo" className="cta-logo" loading="lazy" />
          <h2>Get your product on their list</h2>
          <p>
            Create a free deal or targeted swap today. Families try your product. You pay only when they accept. Start with $25/day — no contracts.
          </p>
          <div className="cta-actions">
            <Link to="/campaign/signup" className="btn-cta-primary">Create a Free Deal →</Link>
            <Link to="/pricing" className="btn-cta-secondary">See Pricing</Link>
          </div>
        </div>
      </section>

      {/* ===== STICKY MOBILE CTA ===== */}
      <div className="sticky-brands-cta">
        <Link to="/campaign/signup" className="sticky-cta-btn">
          Create a Free Deal
        </Link>
      </div>
    </>
  )
}
