import { Component } from 'react'
import type { ReactNode, ErrorInfo } from 'react'
import { FONT, FONT_BASE } from '../styles'

export class ErrorBoundary extends Component<{ children: ReactNode }, { hasError: boolean }> {
  constructor(props: { children: ReactNode }) {
    super(props)
    this.state = { hasError: false }
  }
  static getDerivedStateFromError() { return { hasError: true } }
  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[ErrorBoundary] Uncaught error:', error, info.componentStack)
  }
  render() {
    if (this.state.hasError) {
      return (
        <div style={{
          minHeight: '100vh', display: 'flex', flexDirection: 'column',
          alignItems: 'center', justifyContent: 'center', padding: '32px',
          fontFamily: FONT, backgroundColor: 'var(--bg)', color: 'var(--text)', textAlign: 'center',
        }}>
          <div style={{ fontSize: '1.25rem', fontWeight: 700, marginBottom: '8px' }}>Something went wrong</div>
          <div style={{ fontSize: FONT_BASE, color: 'var(--text-secondary)', marginBottom: '20px' }}>Please reload to continue.</div>
          <button
            onClick={() => window.location.reload()}
            style={{
              padding: '10px 24px', borderRadius: '12px', border: 'none',
              backgroundColor: 'var(--brand)', color: '#fff', fontFamily: FONT,
              fontSize: FONT_BASE, fontWeight: 600, cursor: 'pointer',
            }}
          >Reload</button>
        </div>
      )
    }
    return this.props.children
  }
}
