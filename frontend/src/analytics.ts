// posthog-js is loaded lazily so it stays out of the initial render bundle.
// It is initialized in main.tsx (on idle); the same module singleton is shared
// here, so __loaded reflects that init. track()/identify() no-op until then.
let posthogPromise: Promise<typeof import('posthog-js').default> | null = null
function loadPosthog() {
  if (!posthogPromise) posthogPromise = import('posthog-js').then(m => m.default)
  return posthogPromise
}

export function track(event: string, properties?: Record<string, unknown>) {
  loadPosthog().then(posthog => {
    if (posthog.__loaded) posthog.capture(event, properties)
  }).catch(() => { /* analytics should never break the app */ })
}

export function identify(userId: number) {
  loadPosthog().then(posthog => {
    if (posthog.__loaded) posthog.identify(String(userId))
  }).catch(() => { /* analytics should never break the app */ })
}
