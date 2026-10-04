/// <reference types="vite/client" />

// Build-time environment. Rally sign-in is the only way into Groflo, so a
// production build needs VITE_CLERK_PUBLISHABLE_KEY and VITE_CLERK_SIGN_IN_URL.
// If either is missing the app still renders: the login screen says sign-in is
// briefly unavailable.
interface ImportMetaEnv {
  /** Required. Must match the backend's CLERK_PUBLISHABLE_KEY. */
  readonly VITE_CLERK_PUBLISHABLE_KEY?: string
  /**
   * Leave unset when the app shares an apex with the Rally primary (first-party,
   * the launch configuration). Set to this app's domain only when it runs as a
   * cross-apex satellite.
   */
  readonly VITE_CLERK_DOMAIN?: string
  /** Required. Absolute sign-in URL on the Rally primary. */
  readonly VITE_CLERK_SIGN_IN_URL?: string
  /** CSV of origins Clerk may redirect back to. Defaults to this origin. */
  readonly VITE_ALLOWED_REDIRECT_ORIGINS?: string
  readonly VITE_POSTHOG_KEY?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
