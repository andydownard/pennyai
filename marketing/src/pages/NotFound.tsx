import { Link } from 'react-router-dom'
import { useDocTitle } from '../hooks/useDocTitle'

export default function NotFound() {
  useDocTitle('Page Not Found')
  return (
    <section style={{ textAlign: 'center', padding: '10rem 2rem 6rem' }}>
      <img src="/logo.svg" alt="Groflo" style={{ height: 28, margin: '0 auto 2rem' }} />
      <h1 style={{ fontSize: '4rem', fontWeight: 800, color: 'var(--green-600)', marginBottom: '1rem' }}>404</h1>
      <p style={{ fontSize: '1.25rem', color: 'var(--gray-600)', marginBottom: '0.5rem' }}>
        This page doesn't exist. Let's get you back on track.
      </p>
      <p style={{ fontSize: '1rem', color: 'var(--gray-500)', marginBottom: '2rem' }}>
        Looking for free grocery deals? Start with Penny — it's free.
      </p>
      <div style={{ display: 'flex', gap: '1rem', justifyContent: 'center', flexWrap: 'wrap' }}>
        <Link to="/" className="btn-primary">Get Penny Free →</Link>
        <Link to="/for-brands" className="btn-secondary">For Brands</Link>
      </div>
    </section>
  )
}
