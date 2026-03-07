import { Component } from 'react'
import type { ReactNode, ErrorInfo } from 'react'
import { FONT, BG, TEXT, TEXT_SECONDARY, BRAND, WHITE } from '../styles'

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
          fontFamily: FONT, backgroundColor: BG, color: TEXT, textAlign: 'center',
        }}>
          <div style={{ fontSize: '20px', fontWeight: 700, marginBottom: '8px' }}>Something went wrong</div>
          <div style={{ fontSize: '15px', color: TEXT_SECONDARY, marginBottom: '20px' }}>Please reload to continue.</div>
          <button
            onClick={() => window.location.reload()}
            style={{
              padding: '10px 24px', borderRadius: '12px', border: 'none',
              backgroundColor: BRAND, color: WHITE, fontFamily: FONT,
              fontSize: '15px', fontWeight: 600, cursor: 'pointer',
            }}
          >Reload</button>
        </div>
      )
    }
    return this.props.children
  }
}
