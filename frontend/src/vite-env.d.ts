/// <reference types="vite/client" />

// Build-time environment. Every Clerk variable is OPTIONAL: with
// VITE_CLERK_PUBLISHABLE_KEY unset, @clerk/clerk-react is never imported, the
// Rally sign-in button never renders, and the bundle is the one we ship today.
interface ImportMetaEnv {
  /** Empty or absent = Rally sign-in off. Must match the backend's CLERK_PUBLISHABLE_KEY. */
  readonly VITE_CLERK_PUBLISHABLE_KEY?: string
  /** This satellite's domain. */
  readonly VITE_CLERK_DOMAIN?: string
  /** Absolute sign-in URL on the Rally primary. */
  readonly VITE_CLERK_SIGN_IN_URL?: string
  /** CSV of origins Clerk may redirect back to. Defaults to this origin. */
  readonly VITE_ALLOWED_REDIRECT_ORIGINS?: string
  readonly VITE_POSTHOG_KEY?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
