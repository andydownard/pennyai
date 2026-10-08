// RootProviders wraps the app in the Rally (Clerk) sign-in provider.
//
// The provider is lazy, so Clerk stays out of the entry bundle. And nothing that
// goes wrong up here may blank the page: with no key, or a provider that fails
// to load, the app still renders and the login screen says sign-in is
// unavailable.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'

const providerMounts = vi.fn()

vi.mock('./auth/ClerkAuthProvider', () => ({
  default: ({ children }: { children: React.ReactNode }) => {
    providerMounts()
    return <div data-testid="clerk-provider">{children}</div>
  },
}))

describe('RootProviders', () => {
  beforeEach(() => {
    providerMounts.mockClear()
    vi.resetModules()
  })
  afterEach(() => vi.unstubAllEnvs())

  it('renders children and never loads Clerk when no key is configured', async () => {
    vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', '')
    const { default: RootProviders } = await import('./RootProviders')

    render(
      <RootProviders>
        <div data-testid="app">app</div>
      </RootProviders>,
    )

    expect(screen.getByTestId('app')).toBeInTheDocument()
    expect(screen.queryByTestId('clerk-provider')).not.toBeInTheDocument()
    // Give any stray lazy import a chance to resolve before asserting.
    await new Promise(r => setTimeout(r, 0))
    expect(providerMounts).not.toHaveBeenCalled()
  })

  it('mounts the Clerk provider around children when a key is configured', async () => {
    vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', 'pk_test_abc')
    const { default: RootProviders } = await import('./RootProviders')

    render(
      <RootProviders>
        <div data-testid="app">app</div>
      </RootProviders>,
    )

    await waitFor(() => expect(screen.getByTestId('clerk-provider')).toBeInTheDocument())
    expect(screen.getByTestId('app')).toBeInTheDocument()
    expect(providerMounts).toHaveBeenCalled()
  })

  it('still renders the app when the provider fails to load', async () => {
    vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', 'pk_test_abc')
    vi.spyOn(console, 'error').mockImplementation(() => {})
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    providerMounts.mockImplementation(() => {
      throw new Error('chunk failed')
    })
    const { default: RootProviders } = await import('./RootProviders')

    render(
      <RootProviders>
        <div data-testid="app">app</div>
      </RootProviders>,
    )

    await waitFor(() => expect(screen.getByTestId('app')).toBeInTheDocument())
    expect(screen.queryByTestId('clerk-provider')).not.toBeInTheDocument()
    providerMounts.mockReset()
    vi.restoreAllMocks()
  })
})
