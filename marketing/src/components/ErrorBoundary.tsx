import { Component, type ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  render() {
    if (this.state.hasError) {
      return (
        <div style={{ textAlign: 'center', padding: '10rem 2rem 6rem', maxWidth: 480, margin: '0 auto' }}>
          <img src="/logo.svg" alt="Groflo" style={{ height: 28, margin: '0 auto 2rem' }} />
          <h1 style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--green-600)', marginBottom: '1rem' }}>
            Something went wrong
          </h1>
          <p style={{ fontSize: '1.125rem', color: 'var(--gray-600)', marginBottom: '0.5rem' }}>
            We hit an unexpected error. Please try refreshing the page.
          </p>
          <p style={{ fontSize: '0.875rem', color: 'var(--gray-400)', marginBottom: '2rem' }}>
            If this keeps happening, email us at support@groflo.io
          </p>
          <button
            className="btn-primary"
            onClick={() => window.location.reload()}
          >
            Refresh Page
          </button>
        </div>
      )
    }

    return this.props.children
  }
}
