import { useState } from 'react'

export default function EmailCapture() {
  const [submitted, setSubmitted] = useState(false)

  return (
    <section className="early-access-capture">
      <div className="section-inner" style={{ textAlign: 'center' }}>
        <img src="/logo.svg" alt="Groflo" className="capture-logo" loading="lazy" />
        <h2 className="capture-title">Get notified when Penny launches in your area</h2>
        <p className="capture-subtitle">Enter your email and we'll let you know when free deals are available near you.</p>
        {submitted ? (
          <div className="capture-success">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" fill="#35644F"/></svg>
            <p>You're on the list! We'll email you when free deals launch near you.</p>
          </div>
        ) : (
          <form className="capture-form" onSubmit={(e) => { e.preventDefault(); setSubmitted(true); }}>
            <input type="email" placeholder="your@email.com" required className="capture-input" aria-label="Email address" />
            <button type="submit" className="btn-primary capture-btn">Notify Me</button>
          </form>
        )}
        <p className="capture-note">No spam. Just a heads-up when Penny is ready for you.</p>
      </div>
    </section>
  )
}
