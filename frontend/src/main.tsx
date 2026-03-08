import React from 'react'
import ReactDOM from 'react-dom/client'
import * as Sentry from '@sentry/react'
import posthog from 'posthog-js'
import App from './App'

// Sentry error monitoring — gracefully skipped if DSN not set
const sentryDsn = (window as any).__SENTRY_DSN__ || import.meta.env.VITE_SENTRY_DSN
if (sentryDsn) {
  Sentry.init({
    dsn: sentryDsn,
    integrations: [Sentry.browserTracingIntegration()],
    tracesSampleRate: 0.1,
  })
}

// PostHog analytics — gracefully skipped if key not set
const posthogKey = (window as any).__POSTHOG_KEY__ || import.meta.env.VITE_POSTHOG_KEY
if (posthogKey) {
  posthog.init(posthogKey, {
    api_host: 'https://us.i.posthog.com',
    autocapture: false,
    capture_pageview: false,
  })
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
