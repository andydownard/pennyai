import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { useOnScreen } from '../hooks/useOnScreen'
import { useDocTitle } from '../hooks/useDocTitle'
import SavingsCalculator from '../components/SavingsCalculator'
import EmailCapture from '../components/EmailCapture'

const APP_URL = import.meta.env.VITE_APP_URL || 'https://app.groflo.io'

const HERO_QUOTES = [
  '"I never ask my partner about the list anymore." — Molly N.',
  '"Got free pasta just for trying a new brand." — Karen R.',
  '"Saved $80 last month from swaps alone." — David T.',
  '"We stopped buying duplicates instantly." — Jada M.',
]

export default function Home() {
  useDocTitle()
  const [statsRef, statsVisible] = useOnScreen()
  const [howItWorksRef, howItWorksVisible] = useOnScreen()
  const [referralRef, referralVisible] = useOnScreen()
  const [featuresRef, featuresVisible] = useOnScreen()
  const [dealsRef, dealsVisible] = useOnScreen()
  const [howYouEarnRef, howYouEarnVisible] = useOnScreen()
  const [tripPlannerRef, tripPlannerVisible] = useOnScreen()
  const [pennyRef, pennyVisible] = useOnScreen()
  const [brandsRef, brandsVisible] = useOnScreen()
  const [testimonialsRef, testimonialsVisible] = useOnScreen()
  const [wrappedRef, wrappedVisible] = useOnScreen()
  const [trustRef, trustVisible] = useOnScreen()
  const [faqSectionRef, faqSectionVisible] = useOnScreen()
  const [ctaRef, ctaVisible] = useOnScreen()
  const [openFaq, setOpenFaq] = useState<number | null>(null)
  const [quoteIndex, setQuoteIndex] = useState(0)

  useEffect(() => {
    const interval = setInterval(() => {
      setQuoteIndex(i => (i + 1) % HERO_QUOTES.length)
    }, 4000)
    return () => clearInterval(interval)
  }, [])

  const faqs = [
    { q: "Is Penny really free?", a: "Yes. Penny is completely free for shoppers. We make money when brands offer you deals and free products — you never pay anything." },
    { q: "How does Penny pick my brands?", a: "Penny learns from what you actually buy. When you choose Tillamook over store brand, she remembers. You can also tell her your preferences directly, or turn auto-picking off entirely." },
    { q: "What are Penny Swaps?", a: "When you add an item to your list, Penny checks if a brand is offering a free trial or a better deal on a similar product. You can accept or ignore — it's always your choice." },
    { q: "Do I need to download an app?", a: "No. Groflo works in your phone's browser. You can also text or email Penny directly to add items — no app needed." },
    { q: "Can my whole family use it?", a: "Absolutely. Invite your partner, roommates, or family to share the same list. Everyone sees live updates, and you can claim items so nobody buys duplicates." },
    { q: "Is my data sold to advertisers?", a: "Never. Your shopping data stays yours. Brand suggestions are based on what's popular across all users — not your personal history. Sponsored deals are always labeled." },
  ]

  return (
    <>
      {/* ===== HERO ===== */}
      <section className="hero">
        <div className="hero-inner">
          <div className="hero-text">
            <div className="hero-badge">
              <span className="hero-badge-dot" />
              The shopping list for families
            </div>
            <h1>
              No more <span>"Did you get milk?"</span>
            </h1>
            <p className="hero-subtitle">
              Text Penny what you need. She builds a sorted list your whole household shares — everyone sees updates, no one buys duplicates. Plus, brands send you free products to try.
            </p>
            <div className="hero-actions">
              <a href={APP_URL} rel="noopener" target="_blank" className="btn-primary">
                Get Penny — It's Free →
              </a>
              <a href="#how-it-works" className="btn-secondary">
                See How It Works
              </a>
            </div>
            <div className="hero-social-proof">
              <div className="hero-quote-rotator" aria-live="polite">
                <p className="hero-quote" key={quoteIndex}>{HERO_QUOTES[quoteIndex]}</p>
              </div>
              <div className="hero-proof-badges">
                <span className="hero-proof-badge">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" fill="#35644F"/></svg>
                  Free forever
                </span>
                <span className="hero-proof-badge">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" fill="#35644F"/></svg>
                  No app download
                </span>
                <span className="hero-proof-badge">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" fill="#35644F"/></svg>
                  Free deals from brands
                </span>
              </div>
            </div>
          </div>

          <div className="hero-visual">
            <div className="hero-phone">
              <div className="hero-phone-screen">
                <div className="phone-header">
                  <img src="/penny.png" alt="Penny" className="phone-penny-avatar" />
                  <div className="phone-header-text">
                    <h3>Penny</h3>
                    <p>Online</p>
                  </div>
                </div>
                <div className="phone-messages">
                  <div className="phone-msg phone-msg-user" style={{ animationDelay: '0.2s' }}>
                    Here's what the fridge looks like
                  </div>
                  <div className="phone-msg phone-msg-penny" style={{ animationDelay: '0.6s' }}>
                    You're low on a few things. I picked all your favorite brands and added taco night too.
                  </div>
                  <div className="phone-list" style={{ animation: 'fadeInUp 0.6s ease 1s both' }}>
                    <div className="phone-list-item">
                      <div className="phone-list-check" /><span>Organic Valley milk</span>
                    </div>
                    <div className="phone-list-item">
                      <div className="phone-list-check" /><span>Pete &amp; Gerry's eggs</span>
                    </div>
                    <div className="phone-list-item">
                      <div className="phone-list-check" /><span>Kerrygold butter</span>
                    </div>
                    <div className="phone-list-more">+ 4 more items</div>
                  </div>
                  <div className="phone-swap-card" style={{ animation: 'fadeInUp 0.6s ease 1.4s both' }}>
                    <div className="phone-swap-header">
                      <span className="phone-swap-badge">Free trial</span>
                    </div>
                    <div className="phone-swap-body">
                      <strong>Try Horizon Organic milk — free</strong>
                      <span>Horizon wants you to try theirs. Swap and it's on them.</span>
                    </div>
                    <div className="phone-swap-actions">
                      <span className="phone-swap-accept">Try it free</span>
                      <span className="phone-swap-skip">Keep mine</span>
                    </div>
                  </div>
                  <div className="phone-shared-update" style={{ animation: 'fadeInUp 0.6s ease 1.8s both' }}>
                    <span className="phone-shared-dot" />
                    Alex added: paper towels, dish soap
                  </div>
                </div>
              </div>
            </div>

            <div className="hero-float-card float-card-1">
              <div className="float-card-icon">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M21 19V5c0-1.1-.9-2-2-2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2zM8.5 13.5l2.5 3.01L14.5 12l4.5 6H5l3.5-4.5z" fill="#35644F"/></svg>
              </div>
              <div className="float-card-label">Snap your fridge</div>
              <div className="float-card-value">Penny sees what's low</div>
            </div>
            <div className="hero-float-card float-card-2">
              <div className="float-card-icon">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M16 11c1.66 0 2.99-1.34 2.99-3S17.66 5 16 5c-1.66 0-3 1.34-3 3s1.34 3 3 3zm-8 0c1.66 0 2.99-1.34 2.99-3S9.66 5 8 5C6.34 5 5 6.34 5 8s1.34 3 3 3zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5c0-2.33-4.67-3.5-7-3.5zm8 0c-.29 0-.62.02-.97.05 1.16.84 1.97 1.97 1.97 3.45V19h6v-2.5c0-2.33-4.67-3.5-7-3.5z" fill="#35644F"/></svg>
              </div>
              <div className="float-card-label">Shared with 3 people</div>
              <div className="float-card-value">Everyone adds, everyone sees</div>
            </div>
            <div className="hero-float-card float-card-3">
              <div className="float-card-icon">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M11.8 10.9c-2.27-.59-3-1.2-3-2.15 0-1.09 1.01-1.85 2.7-1.85 1.78 0 2.44.85 2.5 2.1h2.21c-.07-1.72-1.12-3.3-3.21-3.81V3h-3v2.16c-1.94.42-3.5 1.68-3.5 3.61 0 2.31 1.91 3.46 4.7 4.13 2.5.6 3 1.48 3 2.41 0 .69-.49 1.79-2.7 1.79-2.06 0-2.87-.92-2.98-2.1h-2.2c.12 2.19 1.76 3.42 3.68 3.83V21h3v-2.15c1.95-.37 3.5-1.5 3.5-3.55 0-2.84-2.43-3.81-4.7-4.4z" fill="#35644F"/></svg>
              </div>
              <div className="float-card-label">Free trial available</div>
              <div className="float-card-value">Horizon milk — on them</div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== LOGOS BAR ===== */}
      <section className="logos-bar">
        <div className="logos-inner">
          <p className="logos-label">Penny organizes your list by department — for any store</p>
          <div className="logos-row">
            <span className="store-badge">Kroger</span>
            <span className="store-badge">Whole Foods</span>
            <span className="store-badge">Costco</span>
            <span className="store-badge">Trader Joe's</span>
            <span className="store-badge">Target</span>
            <span className="store-badge">Walmart</span>
            <span className="store-badge">Your local market</span>
          </div>
        </div>
      </section>

      {/* ===== ONBOARDING PREVIEW ===== */}
      <section className="onboarding-preview">
        <div className="section-inner">
          <div className="onboarding-header">
            <img src="/logo.svg" alt="Groflo" className="onboarding-logo" />
            <span className="onboarding-tagline">Get started in 10 seconds</span>
          </div>
          <div className="onboarding-strip">
            <div className="onboarding-screen">
              <div className="onboarding-screen-number">1</div>
              <div className="onboarding-screen-content">
                <strong>Enter your phone</strong>
                <span>Sign up in 10 seconds — no app download</span>
              </div>
            </div>
            <div className="onboarding-connector" aria-hidden="true" />
            <div className="onboarding-screen">
              <div className="onboarding-screen-number">2</div>
              <div className="onboarding-screen-content">
                <strong>Add your first items</strong>
                <span>Type naturally — Penny parses everything</span>
              </div>
            </div>
            <div className="onboarding-connector" aria-hidden="true" />
            <div className="onboarding-screen">
              <div className="onboarding-screen-number">3</div>
              <div className="onboarding-screen-content">
                <strong>Invite your household</strong>
                <span>Share the list — everyone stays in sync</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== STATS ===== */}
      <section className={`stats fade-section${statsVisible ? ' visible' : ''}`} ref={statsRef}>
        <div className="stats-inner">
          <div>
            <div className="stat-number">Fewer trips</div>
            <div className="stat-label">Shared lists mean one trip covers everyone</div>
          </div>
          <div>
            <div className="stat-number">$100/mo</div>
            <div className="stat-label">Average savings from deals and free products</div>
          </div>
          <div>
            <div className="stat-number">Organized</div>
            <div className="stat-label">Sorted by department so you don't backtrack</div>
          </div>
          <div>
            <div className="stat-number">Free deals</div>
            <div className="stat-label">Brands send you free products to try</div>
          </div>
        </div>
      </section>

      {/* ===== HOW IT WORKS ===== */}
      <section className={`how-it-works fade-section${howItWorksVisible ? ' visible' : ''}`} id="how-it-works" ref={howItWorksRef}>
        <div className="section-inner">
          <p className="section-label">How It Works</p>
          <h2 className="section-title">Three steps. That's it.</h2>
          <p className="section-subtitle">
            No app to download. Just open Groflo in your browser and tell Penny what you need.
          </p>
          <div className="steps-grid">
            <div className="step">
              <div className="step-number">1</div>
              <h3>Text Penny</h3>
              <p>
                Type naturally — "need stuff for pasta tonight" or snap a photo of a recipe. Penny understands context, quantities, and meal ideas.
              </p>
              <div className="step-channels">
                <span className="step-channel">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2z" fill="#35644F"/></svg>
                  SMS
                </span>
                <span className="step-channel">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4l-8 5-8-5V6l8 5 8-5v2z" fill="#35644F"/></svg>
                  Email
                </span>
                <span className="step-channel">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M21 19V5c0-1.1-.9-2-2-2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2zM8.5 13.5l2.5 3.01L14.5 12l4.5 6H5l3.5-4.5z" fill="#35644F"/></svg>
                  Photo
                </span>
                <span className="step-channel">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M11 21h-1l1-7H7.5c-.58 0-.57-.32-.38-.66l.17-.34C8.18 11.17 9.26 9.12 10.37 7H14l-1 5h3.5c.49 0 .56.33.47.51l-.07.15C12.96 19.45 11 21 11 21z" fill="#35644F"/></svg>
                  Web app
                </span>
              </div>
            </div>
            <div className="step">
              <div className="step-number">2</div>
              <h3>She builds your list</h3>
              <p>
                Penny organizes everything by department, picks your preferred brands, and adds the right quantities. Sorted for an efficient trip.
              </p>
              <div className="penny-tip">
                <img src="/penny.png" alt="" className="penny-tip-avatar" loading="lazy" />
                <span>"I picked Organic Valley — tap to change if you prefer something else."</span>
              </div>
            </div>
            <div className="step">
              <div className="step-number">3</div>
              <h3>Shop together</h3>
              <p>
                Share your list with anyone — roommates, a partner, your family. Everyone sees live updates. Check items off as you go.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* ===== SHARE & EARN ===== */}
      <section className={`share-earn fade-section${referralVisible ? ' visible' : ''}`} id="share" ref={referralRef}>
        <div className="section-inner">
          <div className="share-earn-grid">
            <div className="share-earn-content">
              <p className="section-label">Share &amp; Earn</p>
              <h2>Friends who shop together, stay organized together.</h2>
              <p>
                Want to invite your family or roommates to share the list? You all see the same list, everyone adds items, and Penny keeps you in sync. Both of you earn rewards when they join.
              </p>
              <div className="share-earn-actions">
                <a href={`${APP_URL}/invite`} rel="noopener" target="_blank" className="btn-primary">
                  Create invite link
                </a>
                <a href={`${APP_URL}/rewards`} rel="noopener" target="_blank" className="btn-secondary">
                  See referral rewards
                </a>
              </div>
            </div>
            <div className="share-earn-visual">
              <div className="share-earn-card">
                <div className="share-earn-card-icon">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" fill="#35644F"/></svg>
                </div>
                <h4>You invite a friend</h4>
                <p>Share your personal invite link with anyone</p>
              </div>
              <div className="share-earn-card">
                <div className="share-earn-card-icon">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M16 11c1.66 0 2.99-1.34 2.99-3S17.66 5 16 5c-1.66 0-3 1.34-3 3s1.34 3 3 3zm-8 0c1.66 0 2.99-1.34 2.99-3S9.66 5 8 5C6.34 5 5 6.34 5 8s1.34 3 3 3zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5c0-2.33-4.67-3.5-7-3.5zm8 0c-.29 0-.62.02-.97.05 1.16.84 1.97 1.97 1.97 3.45V19h6v-2.5c0-2.33-4.67-3.5-7-3.5z" fill="#35644F"/></svg>
                </div>
                <h4>They join your list</h4>
                <p>Everyone sees the same list, adds items, stays in sync</p>
              </div>
              <div className="share-earn-card">
                <div className="share-earn-card-icon">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M11.8 10.9c-2.27-.59-3-1.2-3-2.15 0-1.09 1.01-1.85 2.7-1.85 1.78 0 2.44.85 2.5 2.1h2.21c-.07-1.72-1.12-3.3-3.21-3.81V3h-3v2.16c-1.94.42-3.5 1.68-3.5 3.61 0 2.31 1.91 3.46 4.7 4.13 2.5.6 3 1.48 3 2.41 0 .69-.49 1.79-2.7 1.79-2.06 0-2.87-.92-2.98-2.1h-2.2c.12 2.19 1.76 3.42 3.68 3.83V21h3v-2.15c1.95-.37 3.5-1.5 3.5-3.55 0-2.84-2.43-3.81-4.7-4.4z" fill="#35644F"/></svg>
                </div>
                <h4>You both earn rewards</h4>
                <p>Real rewards for keeping your household organized</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== FEATURES ===== */}
      <section className={`features fade-section${featuresVisible ? ' visible' : ''}`} id="features" ref={featuresRef}>
        <div className="section-inner">
          <p className="section-label">Features</p>
          <h2 className="section-title">Everything you need, nothing you don't</h2>
          <p className="section-subtitle">
            Built for the way you actually shop — in the store, with your people.
          </p>
          <div className="features-grid">
            <div className="feature-card">
              <div className="feature-icon">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z" fill="#35644F"/></svg>
              </div>
              <h3>Natural language input</h3>
              <p>Just type like you'd text a friend. Penny handles abbreviations, slang, and meal descriptions.</p>
            </div>
            <div className="feature-card">
              <div className="feature-icon">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M21 19V5c0-1.1-.9-2-2-2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2zM8.5 13.5l2.5 3.01L14.5 12l4.5 6H5l3.5-4.5z" fill="#35644F"/></svg>
              </div>
              <h3>Photo to list</h3>
              <p>Snap a pic of a recipe, handwritten note, or your nearly-empty fridge. Penny extracts the items.</p>
            </div>
            <div className="feature-card">
              <div className="feature-icon">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" fill="#35644F"/></svg>
              </div>
              <h3>Brand memory</h3>
              <p>Penny learns which brands you prefer. Choose to have her ask each time, auto-pick your favorites, or suggest the best deal.</p>
            </div>
            <div className="feature-card">
              <div className="feature-icon">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M16 11c1.66 0 2.99-1.34 2.99-3S17.66 5 16 5c-1.66 0-3 1.34-3 3s1.34 3 3 3zm-8 0c1.66 0 2.99-1.34 2.99-3S9.66 5 8 5C6.34 5 5 6.34 5 8s1.34 3 3 3zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5c0-2.33-4.67-3.5-7-3.5zm8 0c-.29 0-.62.02-.97.05 1.16.84 1.97 1.97 1.97 3.45V19h6v-2.5c0-2.33-4.67-3.5-7-3.5z" fill="#35644F"/></svg>
              </div>
              <h3>No more "Did you get milk?"</h3>
              <p>Penny picks your brands based on household preferences. Claim items as you add them. Your roommate knows milk is covered. Your partner doesn't buy duplicates. One shared list, clear ownership, less chaos.</p>
            </div>
            <div className="feature-card">
              <div className="feature-icon">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M3 13h2v-2H3v2zm0 4h2v-2H3v2zm0-8h2V7H3v2zm4 4h14v-2H7v2zm0 4h14v-2H7v2zM7 7v2h14V7H7z" fill="#35644F"/></svg>
              </div>
              <h3>Smart sorting</h3>
              <p>Items are organized by store department — produce, dairy, frozen — so you spend less time backtracking through aisles.</p>
            </div>
            <div className="feature-card">
              <div className="feature-icon">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M17 3H7c-1.1 0-2 .9-2 2v16l7-3 7 3V5c0-1.1-.9-2-2-2z" fill="#35644F"/></svg>
              </div>
              <h3>Saved lists</h3>
              <p>Save your go-to lists as templates. Weekly staples, party prep, back-to-school — one tap to reload.</p>
            </div>
            <div className="feature-card">
              <div className="feature-icon">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M19.35 10.04C18.67 6.59 15.64 4 12 4 9.11 4 6.6 5.64 5.35 8.04 2.34 8.36 0 10.91 0 14c0 3.31 2.69 6 6 6h13c2.76 0 5-2.24 5-5 0-2.64-2.05-4.78-4.65-4.96zM14 13v4h-4v-4H7l5-5 5 5h-3z" fill="#35644F"/></svg>
              </div>
              <h3>Receipt scanning</h3>
              <p>Snap a photo of your receipt. Penny reads it, logs your purchases, and verifies your deal earnings automatically.</p>
            </div>
            <div className="feature-card">
              <div className="feature-icon">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 3a9 9 0 109 9c0-.46-.04-.92-.1-1.36a5.389 5.389 0 01-4.4 2.26 5.403 5.403 0 01-3.14-9.8c-.44-.06-.9-.1-1.36-.1z" fill="#35644F"/></svg>
              </div>
              <h3>Dark mode included</h3>
              <p>Shop late at night without straining your eyes. Available in the app.</p>
            </div>
          </div>
        </div>
      </section>

      {/* ===== COMING SOON ===== */}
      <section className="coming-soon">
        <div className="section-inner" style={{ textAlign: 'center' }}>
          <p className="section-label">Coming Soon</p>
          <div className="coming-soon-grid">
            <div className="coming-soon-item">
              <span className="coming-soon-badge">Soon</span>
              <h4>Trip planner with store maps</h4>
              <p>Penny learns your store's layout and reorders your list by aisle</p>
            </div>
            <div className="coming-soon-item">
              <span className="coming-soon-badge">Soon</span>
              <h4>Budget tracking</h4>
              <p>Set a weekly grocery budget and Penny helps you stay on track</p>
            </div>
            <div className="coming-soon-item">
              <span className="coming-soon-badge">Soon</span>
              <h4>Meal planning</h4>
              <p>Tell Penny what you want to cook this week — she builds the full list</p>
            </div>
          </div>
        </div>
      </section>

      {/* ===== WHY PENNY ===== */}
      <section className="why-penny">
        <div className="section-inner" style={{ textAlign: 'center' }}>
          <p className="section-label">Why Penny</p>
          <h2 className="section-title" style={{ textAlign: 'center' }}>The only shopping list that pays you back</h2>
          <div className="comparison-table">
            <div className="comparison-row comparison-header">
              <div className="comparison-feature" />
              <div className="comparison-them">Other apps</div>
              <div className="comparison-us">Penny</div>
            </div>
            <div className="comparison-row">
              <div className="comparison-feature">Shared household list</div>
              <div className="comparison-them">Some</div>
              <div className="comparison-us comparison-yes">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" fill="#35644F"/></svg>
              </div>
            </div>
            <div className="comparison-row">
              <div className="comparison-feature">AI that learns your brands</div>
              <div className="comparison-them">No</div>
              <div className="comparison-us comparison-yes">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" fill="#35644F"/></svg>
              </div>
            </div>
            <div className="comparison-row">
              <div className="comparison-feature">Free products from brands</div>
              <div className="comparison-them">No</div>
              <div className="comparison-us comparison-yes">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" fill="#35644F"/></svg>
              </div>
            </div>
            <div className="comparison-row">
              <div className="comparison-feature">Targeted swaps and deals</div>
              <div className="comparison-them">Post-purchase only</div>
              <div className="comparison-us comparison-yes">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" fill="#35644F"/></svg>
              </div>
            </div>
            <div className="comparison-row">
              <div className="comparison-feature">SMS, email, and photo input</div>
              <div className="comparison-them">No</div>
              <div className="comparison-us comparison-yes">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" fill="#35644F"/></svg>
              </div>
            </div>
          </div>
          <a href={APP_URL} rel="noopener" target="_blank" className="btn-primary" style={{ marginTop: '2rem' }}>
            Try Penny Free →
          </a>
        </div>
      </section>

      {/* ===== DEAL EXAMPLES ===== */}
      <section className="deal-examples">
        <div className="section-inner" style={{ textAlign: 'center' }}>
          <p className="section-label">Real Deals</p>
          <h2 className="section-title" style={{ textAlign: 'center' }}>Deals Penny finds for you</h2>
          <p className="section-subtitle" style={{ textAlign: 'center', margin: '0 auto 3rem', maxWidth: '560px' }}>
            These are the kinds of offers brands create on Groflo. You choose what to try.
          </p>
          <div className="deal-examples-grid">
            <div className="deal-example-card">
              <div className="deal-example-badge deal-example-free">Free</div>
              <h4>Horizon Organic Milk</h4>
              <p className="deal-example-desc">Try a half gallon on us — no catch, no coupon</p>
              <div className="deal-example-savings">You save $4.99</div>
            </div>
            <div className="deal-example-card">
              <div className="deal-example-badge deal-example-swap">Swap</div>
              <h4>Canyon Bakehouse Bread</h4>
              <p className="deal-example-desc">Switch from Dave's and save $2 this week</p>
              <div className="deal-example-savings">You save $2.00</div>
            </div>
            <div className="deal-example-card">
              <div className="deal-example-badge deal-example-free">Free</div>
              <h4>Banza Chickpea Pasta</h4>
              <p className="deal-example-desc">High protein, gluten-free — free box to try</p>
              <div className="deal-example-savings">You save $3.49</div>
            </div>
            <div className="deal-example-card">
              <div className="deal-example-badge deal-example-swap">Swap</div>
              <h4>Chobani Oat Milk</h4>
              <p className="deal-example-desc">Swap your usual oat milk and save $1.50</p>
              <div className="deal-example-savings">You save $1.50</div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== PENNY DEALS ===== */}
      <section className={`penny-deals fade-section${dealsVisible ? ' visible' : ''}`} ref={dealsRef}>
        <div className="section-inner">
          <div className="penny-deals-grid">
            <div className="penny-deals-visual">
              <div className="deal-demo-card">
                <div className="deal-demo-label">
                  <span className="deal-demo-badge">Free</span>
                  Penny found a deal for you
                </div>
                <div className="deal-demo-product">
                  <div className="deal-demo-swap">
                    <div className="deal-demo-from">
                      <span className="deal-demo-from-label">On your list</span>
                      <strong>Organic Valley Milk</strong>
                    </div>
                    <div className="deal-demo-arrow">
                      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 4l-1.41 1.41L16.17 11H4v2h12.17l-5.58 5.59L12 20l8-8-8-8z" fill="#35644F"/></svg>
                    </div>
                    <div className="deal-demo-to">
                      <span className="deal-demo-to-label">Try for free</span>
                      <strong>Horizon Organic Milk</strong>
                    </div>
                  </div>
                  <p className="deal-demo-why">Horizon wants families to try their milk. Swap and this one's on them — $0 for you.</p>
                </div>
                <div className="deal-demo-actions">
                  <span className="deal-demo-accept">Try it free</span>
                  <span className="deal-demo-decline">No thanks</span>
                </div>
                <div className="demo-watermark">
                  <img src="/logo.svg" alt="" className="demo-watermark-logo" loading="lazy" />
                </div>
              </div>
              <div className="deal-demo-card deal-demo-card-stacked">
                <div className="deal-demo-label">
                  <span className="deal-demo-badge deal-demo-badge-save">Save $2</span>
                  Swap available
                </div>
                <div className="deal-demo-product">
                  <strong>Dave's Killer Bread → Canyon Bakehouse</strong>
                  <p className="deal-demo-why">Gluten-free and $2 cheaper this week.</p>
                </div>
              </div>
            </div>
            <div className="penny-deals-content">
              <p className="section-label">Penny Deals</p>
              <h2>Get free products from brands that want you to try them</h2>
              <p>
                When you add an item to your list, Penny checks if a brand is offering a free trial or a better deal. You decide — keep your usual pick or try something new at no cost. Brands pay for the trial. You just shop.
              </p>
              <div className="penny-deals-highlights">
                <div className="deals-highlight">
                  <div className="deals-highlight-icon">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M11.8 10.9c-2.27-.59-3-1.2-3-2.15 0-1.09 1.01-1.85 2.7-1.85 1.78 0 2.44.85 2.5 2.1h2.21c-.07-1.72-1.12-3.3-3.21-3.81V3h-3v2.16c-1.94.42-3.5 1.68-3.5 3.61 0 2.31 1.91 3.46 4.7 4.13 2.5.6 3 1.48 3 2.41 0 .69-.49 1.79-2.7 1.79-2.06 0-2.87-.92-2.98-2.1h-2.2c.12 2.19 1.76 3.42 3.68 3.83V21h3v-2.15c1.95-.37 3.5-1.5 3.5-3.55 0-2.84-2.43-3.81-4.7-4.4z" fill="#35644F"/></svg>
                  </div>
                  <div>
                    <h4>Free product trials</h4>
                    <p>Brands cover the full cost. You try it, risk-free.</p>
                  </div>
                </div>
                <div className="deals-highlight">
                  <div className="deals-highlight-icon">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M16 6l2.29 2.29-4.88 4.88-4-4L2 16.59 3.41 18l6-6 4 4 6.3-6.29L22 12V6h-6z" fill="#35644F"/></svg>
                  </div>
                  <div>
                    <h4>Targeted swaps</h4>
                    <p>Penny matches deals to what's already on your list — never random.</p>
                  </div>
                </div>
                <div className="deals-highlight">
                  <div className="deals-highlight-icon">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2a10 10 0 100 20 10 10 0 000-20zm-2 15l-5-5 1.4-1.4L10 14.2l7.6-7.6L19 8l-9 9z" fill="#35644F"/></svg>
                  </div>
                  <div>
                    <h4>Always your choice</h4>
                    <p>Keep your usual brand or try something new. No pressure, ever.</p>
                  </div>
                </div>
              </div>
              <a href={APP_URL} rel="noopener" target="_blank" className="btn-primary">
                Start getting deals →
              </a>
            </div>
          </div>
        </div>
      </section>

      {/* ===== HOW YOU EARN ===== */}
      <section className={`how-you-earn fade-section${howYouEarnVisible ? ' visible' : ''}`} ref={howYouEarnRef}>
        <div className="section-inner" style={{ textAlign: 'center' }}>
          <p className="section-label">How You Save</p>
          <h2 className="section-title" style={{ textAlign: 'center' }}>Brands pay. You save.</h2>
          <p className="section-subtitle" style={{ textAlign: 'center', margin: '0 auto 3rem', maxWidth: '560px' }}>
            Brands spend money to reach grocery shoppers. Penny puts that money back in your pocket through free products and deals.
          </p>
          <div className="earn-flow">
            <div className="earn-step">
              <div className="earn-amount">$140/mo</div>
              <div className="earn-label">Brands spend to reach your household</div>
            </div>
            <div className="earn-arrow" aria-hidden="true">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none"><path d="M12 4l-1.41 1.41L16.17 11H4v2h12.17l-5.58 5.59L12 20l8-8-8-8z" fill="#35644F"/></svg>
            </div>
            <div className="earn-step earn-step-highlight">
              <div className="earn-amount">$100/mo</div>
              <div className="earn-label">Goes back to you as free products and deals</div>
            </div>
            <div className="earn-arrow" aria-hidden="true">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none"><path d="M12 4l-1.41 1.41L16.17 11H4v2h12.17l-5.58 5.59L12 20l8-8-8-8z" fill="#35644F"/></svg>
            </div>
            <div className="earn-step">
              <div className="earn-amount">30%</div>
              <div className="earn-label">Extra earnings when you refer friends</div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== SAVINGS CALCULATOR ===== */}
      <SavingsCalculator />

      {/* ===== TRIP PLANNER ===== */}
      <section className={`trip-planner fade-section${tripPlannerVisible ? ' visible' : ''}`} ref={tripPlannerRef}>
        <div className="section-inner">
          <div className="trip-planner-grid">
            <div className="trip-planner-content">
              <p className="section-label">Trip Planner</p>
              <h2>Get in, get out. Fast.</h2>
              <p>
                Penny knows the store layout. She shows you the best order to hit each aisle — dairy, then produce, then frozen. Instead of zigzagging for 30 minutes, you're done in 15.
              </p>
              <a href={APP_URL} rel="noopener" target="_blank" className="btn-primary">
                See it in action →
              </a>
            </div>
            <div className="trip-planner-visual">
              <div className="trip-planner-demo">
                <div className="trip-demo-header">
                  <span className="trip-demo-dot" />
                  Your optimized route
                </div>
                <div className="trip-demo-aisle">
                  <span className="trip-demo-num">1</span>
                  <div>
                    <strong>Produce</strong>
                    <span>Bananas, spinach, avocados</span>
                  </div>
                </div>
                <div className="trip-demo-aisle">
                  <span className="trip-demo-num">2</span>
                  <div>
                    <strong>Dairy</strong>
                    <span>Organic Valley milk, Kerrygold butter</span>
                    <span className="trip-demo-deal">Free trial: Horizon milk</span>
                  </div>
                </div>
                <div className="trip-demo-aisle">
                  <span className="trip-demo-num">3</span>
                  <div>
                    <strong>Bakery</strong>
                    <span>Dave's Killer Bread</span>
                  </div>
                </div>
                <div className="trip-demo-aisle">
                  <span className="trip-demo-num">4</span>
                  <div>
                    <strong>Frozen</strong>
                    <span>Amy's burritos, ice cream</span>
                  </div>
                </div>
                <div className="trip-demo-footer">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" fill="#35644F"/></svg>
                  Estimated trip: 15 min
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== MEET PENNY ===== */}
      <section className={`penny-section fade-section${pennyVisible ? ' visible' : ''}`} id="penny" ref={pennyRef}>
        <div className="section-inner">
          <div className="penny-grid">
            <div className="penny-avatar-wrapper">
              <div className="penny-avatar-circle">
                <img src="/penny.png" alt="Penny" className="penny-avatar-img" loading="lazy" />
              </div>
            </div>
            <div className="penny-content">
              <p className="section-label">Meet Penny</p>
              <h2>The <span>shopping list</span> that knows your household</h2>
              <p>
                Text Penny what you need. She remembers what you like, infers brands and quantities, and keeps the whole household on the same list.
              </p>
              <div className="penny-features">
                <div className="penny-feature">
                  <div className="penny-feature-icon">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2a10 10 0 100 20 10 10 0 000-20zm-2 15l-5-5 1.4-1.4L10 14.2l7.6-7.6L19 8l-9 9z" fill="#35644F"/></svg>
                  </div>
                  <div>
                    <h4>Learns your preferences</h4>
                    <p>Penny tracks which brands you choose and adapts over time. No setup required.</p>
                  </div>
                </div>
                <div className="penny-feature">
                  <div className="penny-feature-icon">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4l-8 5-8-5V6l8 5 8-5v2z" fill="#35644F"/></svg>
                  </div>
                  <div>
                    <h4>Email or text to add</h4>
                    <p>Forward a recipe email or text your list — Penny handles it all, wherever you are.</p>
                  </div>
                </div>
                <div className="penny-feature">
                  <div className="penny-feature-icon">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M11 21h-1l1-7H7.5c-.58 0-.57-.32-.38-.66l.17-.34C8.18 11.17 9.26 9.12 10.37 7H14l-1 5h3.5c.49 0 .56.33.47.51l-.07.15C12.96 19.45 11 21 11 21z" fill="#35644F"/></svg>
                  </div>
                  <div>
                    <h4>Instant organization</h4>
                    <p>No matter how messy your input, Penny returns a clean, sorted, ready-to-shop list in seconds.</p>
                  </div>
                </div>
              </div>
              <div className="penny-timeline">
                <h4>How Penny gets smarter</h4>
                <div className="penny-timeline-steps">
                  <div className="penny-timeline-step">
                    <div className="penny-timeline-dot" />
                    <div>
                      <strong>Day 1</strong>
                      <span>You add items — Penny picks popular brands</span>
                    </div>
                  </div>
                  <div className="penny-timeline-step">
                    <div className="penny-timeline-dot" />
                    <div>
                      <strong>Week 1</strong>
                      <span>Penny learns your favorites from corrections</span>
                    </div>
                  </div>
                  <div className="penny-timeline-step">
                    <div className="penny-timeline-dot" />
                    <div>
                      <strong>Month 1</strong>
                      <span>Auto-picks your brands, suggests deals you'll like</span>
                    </div>
                  </div>
                </div>
              </div>
              <div className="penny-contact-actions">
                <a href={`${APP_URL}/api/penny.vcf`} rel="noopener" target="_blank" className="btn-primary penny-contact-btn">
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm-2 10h-4v4h-2v-4H7v-2h4V7h2v4h4v2z" fill="white"/></svg>
                  Save Penny as a contact
                </a>
                <p className="penny-contact-hint">Add Penny to your phone. Text her anytime to add items to your list.</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== FOR BRANDS (teaser) ===== */}
      <section className={`brands-section fade-section${brandsVisible ? ' visible' : ''}`} id="brands" ref={brandsRef}>
        <div className="section-inner">
          <div className="brands-grid">
            <div className="brands-content">
              <p className="section-label">For Brands</p>
              <h2>Put your product on their list. For free.</h2>
              <p>
                Penny matches your deals to families who are actively building their shopping list. Offer free trials or targeted swaps — they try your product, you pay only when they accept.
              </p>
              <div className="brand-benefits">
                <div className="brand-benefit">
                  <div className="brand-benefit-icon">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M11.8 10.9c-2.27-.59-3-1.2-3-2.15 0-1.09 1.01-1.85 2.7-1.85 1.78 0 2.44.85 2.5 2.1h2.21c-.07-1.72-1.12-3.3-3.21-3.81V3h-3v2.16c-1.94.42-3.5 1.68-3.5 3.61 0 2.31 1.91 3.46 4.7 4.13 2.5.6 3 1.48 3 2.41 0 .69-.49 1.79-2.7 1.79-2.06 0-2.87-.92-2.98-2.1h-2.2c.12 2.19 1.76 3.42 3.68 3.83V21h3v-2.15c1.95-.37 3.5-1.5 3.5-3.55 0-2.84-2.43-3.81-4.7-4.4z" fill="white"/></svg>
                  </div>
                  <div>
                    <h4>Free product trials</h4>
                    <p>Cover the cost. Families try your product risk-free — the most powerful way to earn a new customer.</p>
                  </div>
                </div>
                <div className="brand-benefit">
                  <div className="brand-benefit-icon">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><circle cx="12" cy="12" r="3" fill="white"/><circle cx="12" cy="12" r="7" stroke="white" strokeWidth="2" fill="none"/><circle cx="12" cy="12" r="11" stroke="white" strokeWidth="2" fill="none"/></svg>
                  </div>
                  <div>
                    <h4>Targeted swaps</h4>
                    <p>When someone adds a competitor's product, Penny offers yours at a discount. Precision at the moment of decision.</p>
                  </div>
                </div>
                <div className="brand-benefit">
                  <div className="brand-benefit-icon">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M16 6l2.29 2.29-4.88 4.88-4-4L2 16.59 3.41 18l6-6 4 4 6.3-6.29L22 12V6h-6z" fill="white"/></svg>
                  </div>
                  <div>
                    <h4>No shelf space needed</h4>
                    <p>Skip slotting fees. Go straight to the shopping list — where purchase decisions actually happen.</p>
                  </div>
                </div>
              </div>
              <Link to="/for-brands" className="btn-brand">
                Create a Free Deal →
              </Link>
            </div>
            <div className="brands-visual">
              <div className="brand-metric-card">
                <h3>85%</h3>
                <p>of groceries bought in-store — the market Penny reaches</p>
              </div>
              <div className="brand-metric-card">
                <h3>$0</h3>
                <p>cost to the consumer — brands cover the trial</p>
              </div>
              <div className="brand-metric-card">
                <h3>Intent</h3>
                <p>deals matched to what's already on their list</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== TESTIMONIALS ===== */}
      <section className={`testimonials fade-section${testimonialsVisible ? ' visible' : ''}`} ref={testimonialsRef}>
        <div className="section-inner">
          <p className="section-label">Real People, Real Relief</p>
          <h2 className="section-title">No more grocery chaos</h2>
          <p className="section-subtitle">
            Families, roommates, and couples who stopped fighting about the shopping list.
          </p>
          <div className="testimonials-grid">
            <div className="testimonial-card">
              <div className="testimonial-stars">★★★★★</div>
              <blockquote>
                I just text what we need for dinner and Penny figures out the rest. It's saved me so much time at the store — I don't wander anymore.
              </blockquote>
              <div className="testimonial-author">
                <div className="testimonial-avatar">MN</div>
                <div>
                  <div className="testimonial-name">Molly N.</div>
                  <div className="testimonial-role">Raleigh, NC</div>
                </div>
              </div>
            </div>
            <div className="testimonial-card">
              <div className="testimonial-stars">★★★★★</div>
              <blockquote>
                My roommates and I share a list now. No more buying duplicates. I added "stuff for stir fry" and it just... worked.
              </blockquote>
              <div className="testimonial-author">
                <div className="testimonial-avatar">JM</div>
                <div>
                  <div className="testimonial-name">Jada M.</div>
                  <div className="testimonial-role">Brooklyn, NY</div>
                </div>
              </div>
            </div>
            <div className="testimonial-card">
              <div className="testimonial-stars">★★★★★</div>
              <blockquote>
                Penny offered me a free box of pasta from a new brand I'd never tried. Turns out I love it — and it cost me nothing. Now I get deals like that every week.
              </blockquote>
              <div className="testimonial-author">
                <div className="testimonial-avatar">KR</div>
                <div>
                  <div className="testimonial-name">Karen R.</div>
                  <div className="testimonial-role">Austin, TX</div>
                </div>
              </div>
            </div>
            <div className="testimonial-card">
              <div className="testimonial-stars">★★★★★</div>
              <blockquote>
                I saved over $80 last month just from swaps Penny suggested. I didn't even have to look for coupons — she just showed me better deals on things I was already buying.
              </blockquote>
              <div className="testimonial-author">
                <div className="testimonial-avatar">DT</div>
                <div>
                  <div className="testimonial-name">David T.</div>
                  <div className="testimonial-role">Portland, OR</div>
                </div>
              </div>
            </div>
          </div>
          <div style={{ textAlign: 'center', marginTop: '2.5rem' }}>
            <a href={APP_URL} rel="noopener" target="_blank" className="btn-primary">
              Join them — it's free →
            </a>
          </div>
        </div>
      </section>

      {/* ===== TRUST / PRIVACY ===== */}
      <section className={`trust-section fade-section${trustVisible ? ' visible' : ''}`} ref={trustRef}>
        <div className="section-inner">
          <p className="section-label">Your Privacy</p>
          <h2 className="section-title">How does Penny know what I like?</h2>
          <p className="section-subtitle">
            Great question. Here's exactly how it works — no fine print.
          </p>
          <div className="trust-grid">
            <div className="trust-card">
              <h4>Penny learns from your choices</h4>
              <p>When you pick Tillamook over store brand, Penny remembers. She only learns from what you actually buy — never from browsing or guessing.</p>
            </div>
            <div className="trust-card">
              <h4>Your data stays yours</h4>
              <p>We never sell your shopping data to advertisers. Your list, your preferences, your business. Period.</p>
            </div>
            <div className="trust-card">
              <h4>You're always in control</h4>
              <p>Don't want Penny to auto-pick brands? Turn it off. Want to clear what she's learned? One tap. You decide how smart she gets.</p>
            </div>
            <div className="trust-card">
              <h4>Brand suggestions are labeled</h4>
              <p>If a brand pays to appear on your list, you'll always know. Sponsored suggestions are clearly marked — Penny never sneaks one in.</p>
            </div>
          </div>
          <div className="trust-cta">
            <Link to="/privacy" className="btn-secondary">
              Read our full privacy policy →
            </Link>
          </div>
        </div>
      </section>

      {/* ===== GROFLO WRAPPED ===== */}
      <section className={`wrapped-section fade-section${wrappedVisible ? ' visible' : ''}`} ref={wrappedRef}>
        <div className="section-inner">
          <div className="wrapped-grid">
            <div className="wrapped-content">
              <p className="section-label">Groflo Wrapped</p>
              <h2>See what you saved every month</h2>
              <p>
                At the end of each month, Penny sends you a summary of everything you saved — free products tried, deals accepted, and total savings. Share it with friends to show them what they're missing.
              </p>
              <a href={APP_URL} rel="noopener" target="_blank" className="btn-primary">
                Start tracking your savings →
              </a>
            </div>
            <div className="wrapped-preview">
              <div className="wrapped-card">
                <div className="wrapped-card-header">
                  <img src="/penny.png" alt="Penny" className="wrapped-penny-img" loading="lazy" />
                  <div>
                    <strong>Your March Wrapped</strong>
                    <span>Here's what you saved this month</span>
                  </div>
                </div>
                <div className="wrapped-card-stats">
                  <div className="wrapped-stat">
                    <div className="wrapped-stat-value">$94</div>
                    <div className="wrapped-stat-label">Total saved</div>
                  </div>
                  <div className="wrapped-stat">
                    <div className="wrapped-stat-value">7</div>
                    <div className="wrapped-stat-label">Free products</div>
                  </div>
                  <div className="wrapped-stat">
                    <div className="wrapped-stat-value">12</div>
                    <div className="wrapped-stat-label">Deals accepted</div>
                  </div>
                </div>
                <div className="wrapped-card-footer">
                  <img src="/logo.svg" alt="Groflo" className="wrapped-logo" loading="lazy" />
                  <span>Top brand: Horizon Organic</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== FAQ ===== */}
      <section className={`home-faq fade-section${faqSectionVisible ? ' visible' : ''}`} ref={faqSectionRef}>
        <div className="section-inner">
          <p className="section-label">FAQ</p>
          <h2 className="section-title" style={{ textAlign: 'center' }}>Common questions</h2>
          <div className="faq-list">
            {faqs.map((faq, i) => (
              <div key={faq.q} className={`faq-item${openFaq === i ? ' open' : ''}`}>
                <button
                  className="faq-question"
                  onClick={() => setOpenFaq(openFaq === i ? null : i)}
                  aria-expanded={openFaq === i}
                  aria-controls={`home-faq-answer-${i}`}
                >
                  <span>{faq.q}</span>
                  <span className="faq-toggle" aria-hidden="true">{openFaq === i ? '−' : '+'}</span>
                </button>
                {openFaq === i && (
                  <div className="faq-answer" id={`home-faq-answer-${i}`} role="region">
                    <p>{faq.a}</p>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ===== EARLY ACCESS CAPTURE ===== */}
      <EmailCapture />

      {/* ===== CTA ===== */}
      <section className={`cta-section fade-section${ctaVisible ? ' visible' : ''}`} ref={ctaRef}>
        <div className="cta-inner">
          <img src="/logo-white.svg" alt="Groflo" className="cta-logo" loading="lazy" />
          <h2>Start saving on groceries today</h2>
          <p>
            Get organized, get free products from brands, and never ask "did you get milk?" again. It's free to start — no credit card needed.
          </p>
          <div className="cta-actions">
            <a href={APP_URL} rel="noopener" target="_blank" className="btn-cta-primary">
              Try Groflo Free →
            </a>
            <Link to="/about" className="btn-cta-secondary">
              Learn More About Groflo
            </Link>
          </div>
        </div>
      </section>

      {/* ===== STICKY MOBILE CTA ===== */}
      <div className="sticky-mobile-cta">
        <a href={APP_URL} rel="noopener" target="_blank" className="sticky-cta-btn">
          Try Groflo Free
        </a>
      </div>
    </>
  )
}
