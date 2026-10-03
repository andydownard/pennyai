// RootProviders decides whether Clerk is loaded at all.
//
// The important assertion: with no publishable key the lazy import is never
// evaluated. That is what keeps @clerk/clerk-react out of the entry bundle for
// every user while Rally sign-in is dark.

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
})
