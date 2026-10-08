// ClerkAuthProvider chooses, at build time, how this app relates to the Rally
// primary: first-party (same apex, no VITE_CLERK_DOMAIN) or satellite. A build
// missing the key or the sign-in URL renders the app without Clerk — the login
// screen then says sign-in is unavailable — and never crashes.

import { useContext } from 'react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'

const providerProps = vi.fn()

vi.mock('@clerk/clerk-react', () => ({
  ClerkProvider: ({ children, ...props }: { children: React.ReactNode } & Record<string, unknown>) => {
    providerProps(props)
    return <div data-testid="clerk-provider">{children}</div>
  },
}))
vi.mock('./ClerkTokenBridge', () => ({ ClerkTokenBridge: () => <span data-testid="bridge" /> }))
vi.mock('./ClerkSessionExchange', () => ({ ClerkSessionExchange: () => <span data-testid="exchange" /> }))

import { ClerkAuthProvider } from './ClerkAuthProvider'
import { RallyReadyContext } from './RallyReadyContext'

function Probe() {
  const ready = useContext(RallyReadyContext)
  return <div data-testid="app">{ready ? 'rally-ready' : 'rally-unavailable'}</div>
}

function env(vars: { key?: string; signInUrl?: string; domain?: string; origins?: string }) {
  vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', vars.key ?? '')
  vi.stubEnv('VITE_CLERK_SIGN_IN_URL', vars.signInUrl ?? '')
  vi.stubEnv('VITE_CLERK_DOMAIN', vars.domain ?? '')
  vi.stubEnv('VITE_ALLOWED_REDIRECT_ORIGINS', vars.origins ?? '')
}

const KEY = 'pk_test_abc'
const SIGN_IN = 'https://rallytasks.com/sign-in'

describe('ClerkAuthProvider', () => {
  beforeEach(() => {
    providerProps.mockClear()
    vi.spyOn(console, 'warn').mockImplementation(() => {})
  })
  afterEach(() => {
    vi.unstubAllEnvs()
    vi.restoreAllMocks()
  })

  describe('first-party mode (no VITE_CLERK_DOMAIN — the launch configuration)', () => {
    it('passes no isSatellite and no domain', () => {
      env({ key: KEY, signInUrl: SIGN_IN })
      render(<ClerkAuthProvider><Probe /></ClerkAuthProvider>)

      expect(providerProps).toHaveBeenCalledTimes(1)
      const props = providerProps.mock.calls[0][0]
      expect(props).not.toHaveProperty('isSatellite')
      expect(props).not.toHaveProperty('domain')
      expect(props.publishableKey).toBe(KEY)
      expect(props.signInUrl).toBe(SIGN_IN)
    })

    it('does not force the post-sign-in destination, so the page the user left wins', () => {
      env({ key: KEY, signInUrl: SIGN_IN })
      render(<ClerkAuthProvider><Probe /></ClerkAuthProvider>)

      const props = providerProps.mock.calls[0][0]
      expect(props).not.toHaveProperty('signInForceRedirectUrl')
      expect(props.signInFallbackRedirectUrl).toBe(window.location.origin)
      expect(props.signUpFallbackRedirectUrl).toBe(window.location.origin)
    })

    it('defaults allowedRedirectOrigins to this origin', () => {
      env({ key: KEY, signInUrl: SIGN_IN })
      render(<ClerkAuthProvider><Probe /></ClerkAuthProvider>)
      expect(providerProps.mock.calls[0][0].allowedRedirectOrigins).toEqual([window.location.origin])
    })

    it('reads allowedRedirectOrigins from VITE_ALLOWED_REDIRECT_ORIGINS', () => {
      env({ key: KEY, signInUrl: SIGN_IN, origins: 'https://groflo.rallytasks.com, https://rallytasks.com' })
      render(<ClerkAuthProvider><Probe /></ClerkAuthProvider>)
      expect(providerProps.mock.calls[0][0].allowedRedirectOrigins).toEqual([
        'https://groflo.rallytasks.com',
        'https://rallytasks.com',
      ])
    })

    it('mounts the token bridge and the exchange, and marks Rally ready', () => {
      env({ key: KEY, signInUrl: SIGN_IN })
      render(<ClerkAuthProvider><Probe /></ClerkAuthProvider>)
      expect(screen.getByTestId('bridge')).toBeInTheDocument()
      expect(screen.getByTestId('exchange')).toBeInTheDocument()
      expect(screen.getByTestId('app')).toHaveTextContent('rally-ready')
    })
  })

  describe('satellite mode (VITE_CLERK_DOMAIN set)', () => {
    it('passes isSatellite and the domain', () => {
      env({ key: KEY, signInUrl: SIGN_IN, domain: 'app.groflo.io' })
      render(<ClerkAuthProvider><Probe /></ClerkAuthProvider>)

      const props = providerProps.mock.calls[0][0]
      expect(props.isSatellite).toBe(true)
      expect(props.domain).toBe('app.groflo.io')
      expect(props.publishableKey).toBe(KEY)
      expect(props.signInUrl).toBe(SIGN_IN)
      expect(props.allowedRedirectOrigins).toEqual([window.location.origin])
      expect(screen.getByTestId('app')).toHaveTextContent('rally-ready')
    })
  })

  describe('misconfigured builds render the app without Clerk', () => {
    it('missing sign-in URL', () => {
      env({ key: KEY })
      render(<ClerkAuthProvider><Probe /></ClerkAuthProvider>)
      expect(providerProps).not.toHaveBeenCalled()
      expect(screen.queryByTestId('clerk-provider')).not.toBeInTheDocument()
      expect(screen.getByTestId('app')).toHaveTextContent('rally-unavailable')
    })

    it('missing sign-in URL, even with a domain set', () => {
      env({ key: KEY, domain: 'app.groflo.io' })
      render(<ClerkAuthProvider><Probe /></ClerkAuthProvider>)
      expect(providerProps).not.toHaveBeenCalled()
      expect(screen.getByTestId('app')).toHaveTextContent('rally-unavailable')
    })

    it('missing publishable key', () => {
      env({ signInUrl: SIGN_IN })
      render(<ClerkAuthProvider><Probe /></ClerkAuthProvider>)
      expect(providerProps).not.toHaveBeenCalled()
      expect(screen.getByTestId('app')).toHaveTextContent('rally-unavailable')
    })
  })
})
