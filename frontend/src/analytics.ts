import posthog from 'posthog-js'

export function track(event: string, properties?: Record<string, unknown>) {
  try {
    if (posthog.__loaded) posthog.capture(event, properties)
  } catch { /* analytics should never break the app */ }
}

export function identify(userId: number) {
  try {
    if (posthog.__loaded) posthog.identify(String(userId))
  } catch { /* analytics should never break the app */ }
}
