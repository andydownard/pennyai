import { useState } from 'react'
import { useOnScreen } from '../hooks/useOnScreen'
import { useDocTitle } from '../hooks/useDocTitle'
import '../styles/About.css'

export default function About() {
  useDocTitle('About', 'Thriftly is the AI shopping list for families. We connect shoppers with free deals from brands through a shared, intelligent grocery list.')
  const story = useOnScreen()
  const values = useOnScreen()
  const contact = useOnScreen()

  const [formData, setFormData] = useState({
    name: '',
    email: '',
    subject: 'general',
    message: ''
  })
  const [submitted, setSubmitted] = useState(false)

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    // For now, open mailto with form data
    const mailto = `mailto:hello@thrift.ly?subject=${encodeURIComponent(formData.subject === 'general' ? 'General Inquiry' : formData.subject === 'brands' ? 'Brand Partnership Inquiry' : formData.subject === 'press' ? 'Press Inquiry' : formData.subject === 'support' ? 'Support Request' : 'Careers Inquiry')}&body=${encodeURIComponent(`From: ${formData.name} (${formData.email})\n\n${formData.message}`)}`
    window.open(mailto, '_blank')
    setSubmitted(true)
  }

  return (
    <>
      {/* ===== HERO ===== */}
      <section className="about-hero">
        <div className="section-inner">
          <img src="/logo.svg" alt="thriftly" className="about-hero-logo" />
          <p className="section-label">About Thriftly</p>
          <h1 className="section-title">Making grocery shopping effortless</h1>
          <p className="section-subtitle" style={{ marginBottom: 0 }}>
            We believe grocery shopping should be simple, smart, and collaborative. That's why we built Thriftly.
          </p>
        </div>
      </section>

      {/* ===== STORY ===== */}
      <section className={`about-story fade-section${story.visible ? ' visible' : ''}`} ref={story.ref}>
        <div className="section-inner">
          <div className="story-grid">
            <div>
              <h2 className="section-title">Why we built Thriftly</h2>
              <p>
                Grocery shopping is one of those tasks everyone does but nobody enjoys managing. Shared lists get messy. You forget what brands you like. You end up wandering aisles or buying duplicates.
              </p>
              <p>
                We built Thriftly to fix that. Penny, our AI assistant, understands natural language, learns your preferences over time, and keeps everyone in your household in sync — automatically.
              </p>
              <p>
                For brands, we built a marketplace that puts products directly on shopping lists through free trials and targeted swaps. Brands pay to reach shoppers at the moment of decision. Families get free products and deals. Everyone wins.
              </p>
            </div>
            <div className="story-stats">
              <div className="story-stat">
                <div className="story-stat-number">Web-based</div>
                <div className="story-stat-label">No download needed — open Thriftly in your browser</div>
              </div>
              <div className="story-stat">
                <div className="story-stat-number">Shared</div>
                <div className="story-stat-label">Real-time lists for the whole household</div>
              </div>
              <div className="story-stat">
                <div className="story-stat-number">$100/mo</div>
                <div className="story-stat-label">Average savings from free products and deals</div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== VALUES ===== */}
      <section className={`about-values fade-section${values.visible ? ' visible' : ''}`} ref={values.ref}>
        <div className="section-inner">
          <h2 className="section-title" style={{ textAlign: 'center' }}>What we believe</h2>
          <div className="values-grid">
            <div className="value-card">
              <h3>Simplicity first</h3>
              <p>The best tools don't need a manual. Thriftly works the way you already communicate — just text what you need.</p>
            </div>
            <div className="value-card">
              <h3>Built for real life</h3>
              <p>Shared households, busy schedules, last-minute additions. We design for the messy reality of how people actually shop.</p>
            </div>
            <div className="value-card">
              <h3>Helpful, not intrusive</h3>
              <p>Brand deals only appear when they're relevant to what you're buying. Free trials and swaps are always optional — if it's not useful to you, it won't be on your list.</p>
            </div>
          </div>
        </div>
      </section>

      {/* ===== TEAM ===== */}
      <section className={`about-team fade-section${values.visible ? ' visible' : ''}`}>
        <div className="section-inner">
          <h2 className="section-title" style={{ textAlign: 'center' }}>The team behind Thriftly</h2>
          <p className="section-subtitle" style={{ textAlign: 'center', margin: '0 auto 3rem' }}>
            We're a small, focused team of engineers and designers who believe grocery shopping can be better. Based in the US, building for families everywhere.
          </p>
          <div className="team-details">
            <div className="team-detail-item">
              <h4>Founded</h4>
              <p>2024</p>
            </div>
            <div className="team-detail-item">
              <h4>Mission</h4>
              <p>Make grocery shopping effortless for every household</p>
            </div>
            <div className="team-detail-item">
              <h4>Approach</h4>
              <p>AI-first, privacy-conscious, built for real people</p>
            </div>
          </div>
        </div>
      </section>

      {/* ===== CONTACT ===== */}
      <section className={`about-contact fade-section${contact.visible ? ' visible' : ''}`} ref={contact.ref}>
        <div className="section-inner">
          <h2 className="section-title" style={{ textAlign: 'center' }}>Get in touch</h2>
          <p className="section-subtitle" style={{ textAlign: 'center', margin: '0 auto 3rem' }}>
            Have a question, partnership idea, or just want to say hello? We'd love to hear from you.
          </p>
          <div className="contact-grid">
            <div className="contact-form-wrapper">
              {submitted ? (
                <div className="contact-success">
                  <div className="contact-success-icon">✓</div>
                  <h3>Almost there!</h3>
                  <p>Your email client should have opened with a pre-filled message. Please click Send to complete your inquiry. If it didn't open, email us directly at <a href="mailto:hello@thrift.ly">hello@thrift.ly</a>.</p>
                  <button type="button" className="btn-secondary" style={{ marginTop: '1rem' }} onClick={() => setSubmitted(false)}>Send another message</button>
                </div>
              ) : (
                <form className="contact-form" onSubmit={handleSubmit}>
                  <div className="form-group">
                    <label className="form-label" htmlFor="name">Name</label>
                    <input
                      className="form-input"
                      id="name"
                      type="text"
                      required
                      value={formData.name}
                      onChange={e => setFormData({ ...formData, name: e.target.value })}
                      placeholder="Your name"
                    />
                  </div>
                  <div className="form-group">
                    <label className="form-label" htmlFor="email">Email</label>
                    <input
                      className="form-input"
                      id="email"
                      type="email"
                      required
                      value={formData.email}
                      onChange={e => setFormData({ ...formData, email: e.target.value })}
                      placeholder="you@company.com"
                    />
                  </div>
                  <div className="form-group">
                    <label className="form-label" htmlFor="subject">Subject</label>
                    <select
                      className="form-input"
                      id="subject"
                      value={formData.subject}
                      onChange={e => setFormData({ ...formData, subject: e.target.value })}
                    >
                      <option value="general">General Inquiry</option>
                      <option value="brands">Brand Partnerships</option>
                      <option value="press">Press</option>
                      <option value="support">Support</option>
                      <option value="careers">Careers</option>
                    </select>
                  </div>
                  <div className="form-group">
                    <label className="form-label" htmlFor="message">Message</label>
                    <textarea
                      className="form-input form-textarea"
                      id="message"
                      required
                      rows={5}
                      value={formData.message}
                      onChange={e => setFormData({ ...formData, message: e.target.value })}
                      placeholder="How can we help?"
                    />
                  </div>
                  <button type="submit" className="btn-primary" style={{ width: '100%', justifyContent: 'center' }}>
                    Send Message →
                  </button>
                </form>
              )}
            </div>
            <div className="contact-info">
              <div className="contact-info-item">
                <h4>General</h4>
                <a href="mailto:hello@thrift.ly">hello@thrift.ly</a>
              </div>
              <div className="contact-info-item">
                <h4>Brand Partnerships</h4>
                <a href="mailto:brands@thrift.ly">brands@thrift.ly</a>
              </div>
              <div className="contact-info-item">
                <h4>Press</h4>
                <a href="mailto:press@thrift.ly">press@thrift.ly</a>
              </div>
              <div className="contact-info-item">
                <h4>Support</h4>
                <a href="mailto:support@thrift.ly">support@thrift.ly</a>
              </div>
              <div className="contact-info-item">
                <h4>Careers</h4>
                <a href="mailto:careers@thrift.ly">careers@thrift.ly</a>
              </div>
            </div>
          </div>
        </div>
      </section>
    </>
  )
}
