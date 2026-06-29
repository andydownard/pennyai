import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)

// Defer observability init until the browser is idle so it never blocks first
// paint / time-to-interactive. track()/identify() in analytics.ts no-op until
// posthog finishes loading (they guard on posthog.__loaded), and ErrorBoundary
// imports its own Sentry for capture, so early errors are still recorded.
function initObservability() {
  // Sentry error monitoring (DSN is public/safe — only allows sending errors)
  const sentryDsn = 'https://c462ececfb7290bede91712b2d58e30a@o4511042213773312.ingest.us.sentry.io/4511042223407104'
  import('@sentry/react').then(Sentry => {
    Sentry.init({
      dsn: sentryDsn,
      integrations: [Sentry.browserTracingIntegration()],
      tracesSampleRate: 0.1,
    })
  }).catch(() => {})

  // PostHog analytics — gracefully skipped if key not set
  const posthogKey = (window as any).__POSTHOG_KEY__ || import.meta.env.VITE_POSTHOG_KEY
  if (posthogKey) {
    import('posthog-js').then(({ default: posthog }) => {
      posthog.init(posthogKey, {
        api_host: 'https://us.i.posthog.com',
        autocapture: false,
        capture_pageview: false,
      })
    }).catch(() => {})
  }
}

if (typeof requestIdleCallback !== 'undefined') {
  requestIdleCallback(initObservability, { timeout: 3000 })
} else {
  setTimeout(initObservability, 1500)
}

// Register service worker for offline caching and fast repeat loads
if ('serviceWorker' in navigator && import.meta.env.PROD) {
  navigator.serviceWorker.register('/sw.js').catch(() => {})

  // Safety net: if a JS chunk fails to load (stale SW serving old HTML with
  // new asset hashes), unregister the SW, clear caches, and hard-refresh.
  window.addEventListener('error', (event) => {
    const target = event.target as HTMLElement | null
    if (target?.tagName === 'SCRIPT' && (target as HTMLScriptElement).src?.includes('/assets/')) {
      navigator.serviceWorker.getRegistrations().then(regs => {
        regs.forEach(r => r.unregister())
      })
      caches.keys().then(keys => {
        Promise.all(keys.map(k => caches.delete(k))).then(() => {
          window.location.reload()
        })
      })
    }
  }, true) // capture phase to catch module load errors
}
