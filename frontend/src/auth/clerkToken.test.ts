// clerkToken: the registry between Clerk's React context and plain modules.
// Nothing here may throw — a Clerk failure must degrade to "no token", never
// break the request that was asking for one.

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import {
  clerkActive,
  clerkSignOut,
  getClerkToken,
  notifyClerkExchanged,
  setClerkExchangeHandler,
  setClerkSignOut,
  setClerkTokenGetter,
  _resetClerkTokenRegistry,
} from './clerkToken'

describe('clerkToken', () => {
  beforeEach(() => _resetClerkTokenRegistry())
  afterEach(() => vi.unstubAllEnvs())

  describe('clerkActive', () => {
    it('is false when no publishable key is built in', () => {
      vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', '')
      expect(clerkActive()).toBe(false)
    })

    it('is true when a publishable key is built in', () => {
      vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', 'pk_test_abc')
      expect(clerkActive()).toBe(true)
    })
  })

  describe('getClerkToken', () => {
    it('returns null when no getter is registered', async () => {
      await expect(getClerkToken()).resolves.toBeNull()
    })

    it('returns the token the getter yields', async () => {
      setClerkTokenGetter(async () => 'tok_123')
      await expect(getClerkToken()).resolves.toBe('tok_123')
    })

    it('returns null — never rejects — when the getter throws', async () => {
      setClerkTokenGetter(async () => {
        throw new Error('clerk is unhappy')
      })
      await expect(getClerkToken()).resolves.toBeNull()
    })

    it('normalizes undefined to null', async () => {
      setClerkTokenGetter(async () => undefined as unknown as string)
      await expect(getClerkToken()).resolves.toBeNull()
    })

    it('stops using the getter once it is unregistered', async () => {
      setClerkTokenGetter(async () => 'tok_123')
      setClerkTokenGetter(null)
      await expect(getClerkToken()).resolves.toBeNull()
    })
  })

  describe('clerkSignOut', () => {
    it('is a no-op when nothing is registered', async () => {
      await expect(clerkSignOut()).resolves.toBeUndefined()
    })

    it('calls the registered sign-out', async () => {
      const signOut = vi.fn(async () => {})
      setClerkSignOut(signOut)
      await clerkSignOut()
      expect(signOut).toHaveBeenCalledTimes(1)
    })

    it('swallows a failing sign-out so logout still completes', async () => {
      setClerkSignOut(async () => {
        throw new Error('network')
      })
      await expect(clerkSignOut()).resolves.toBeUndefined()
    })
  })

  describe('exchange handler', () => {
    it('does nothing when no handler is registered', () => {
      expect(() => notifyClerkExchanged()).not.toThrow()
    })

    it('forwards success and failure', () => {
      const handler = vi.fn()
      setClerkExchangeHandler(handler)
      notifyClerkExchanged()
      notifyClerkExchanged('boom')
      expect(handler).toHaveBeenNthCalledWith(1, undefined)
      expect(handler).toHaveBeenNthCalledWith(2, 'boom')
    })

    it('survives a handler that throws', () => {
      setClerkExchangeHandler(() => {
        throw new Error('listener bug')
      })
      expect(() => notifyClerkExchanged()).not.toThrow()
    })
  })
})
