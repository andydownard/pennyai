import { FONT, TEXT_SECONDARY } from '../styles'

export function LegalLinks() {
  return (
    <>
      <a
        href="https://thrift.ly"
        target="_blank"
        rel="noopener noreferrer"
        style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, textDecoration: 'none', marginTop: '48px', opacity: 0.7 }}
      >
        powered by <span style={{ fontWeight: 600 }}>thriftly</span>
      </a>
      <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '12px', opacity: 0.6 }}>
        <a href="/privacy" target="_blank" rel="noopener noreferrer" style={{ color: TEXT_SECONDARY, textDecoration: 'none' }}>Privacy</a>
        {' · '}
        <a href="/terms" target="_blank" rel="noopener noreferrer" style={{ color: TEXT_SECONDARY, textDecoration: 'none' }}>Terms</a>
      </div>
    </>
  )
}
