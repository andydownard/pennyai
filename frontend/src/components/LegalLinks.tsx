import { FONT, FONT_2XS } from '../styles'

export function LegalLinks() {
  return (
    <>
      <a
        href="https://groflo.io"
        target="_blank"
        rel="noopener noreferrer"
        style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', textDecoration: 'none', marginTop: '48px', opacity: 0.7 }}
      >
        Penny by <span style={{ fontWeight: 600 }}>Groflo</span>
      </a>
      <div style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', marginTop: '12px', opacity: 0.6 }}>
        <a href="/privacy" target="_blank" rel="noopener noreferrer" style={{ color: 'var(--text-secondary)', textDecoration: 'none' }}>Privacy</a>
        {' · '}
        <a href="/terms" target="_blank" rel="noopener noreferrer" style={{ color: 'var(--text-secondary)', textDecoration: 'none' }}>Terms</a>
      </div>
    </>
  )
}
