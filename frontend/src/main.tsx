import React from 'react'
import ReactDOM from 'react-dom/client'
import * as Sentry from '@sentry/react'
import posthog from 'posthog-js'
import App from './App'

// Sentry error monitoring (DSN is public/safe — only allows sending errors)
const sentryDsn = 'https://c462ececfb7290bede91712b2d58e30a@o4511042213773312.ingest.us.sentry.io/4511042223407104'
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

// Register service worker for offline caching and fast repeat loads
if ('serviceWorker' in navigator && import.meta.env.PROD) {
  navigator.serviceWorker.register('/sw.js').catch(() => {})
}
