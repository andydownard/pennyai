// Sign-in happens on the Rally primary: the browser leaves Groflo and returns.
// These tests prove that what a signed-out visitor was on their way to — a
// household invite, a Share & Earn link, a shared list, a deep link — is still
// there after that round trip, and is used exactly once.

import { describe, it, expect, beforeEach } from 'vitest'
import {
  JOIN_CODE_KEY, REFERRAL_CODE_KEY, SHARE_INVITE_KEY, SHARE_NAME_KEY, REDIRECT_KEY,
  consumeLanding, rememberDestination, stashInviteLink, stashShareInvite,
} from './pendingLinks'

type Screen = 'login' | 'home' | 'profile' | 'settings-share'
const PATHS: Record<string, Screen> = {
  '/login': 'login',
  '/verify': 'login',
  '/': 'home',
  '/profile': 'profile',
  '/share': 'settings-share',
}
const SIGNED_OUT: Screen[] = ['login']
const land = (path: string) => consumeLanding<Screen>(path, PATHS, SIGNED_OUT, 'home')

describe('pendingLinks', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
  })

  describe('stashInviteLink', () => {
    it('keeps a /join/<code> household invite in localStorage', () => {
      expect(stashInviteLink('/join/ABC123')).toBe('join')
      expect(localStorage.getItem(JOIN_CODE_KEY)).toBe('ABC123')
    })

    it('keeps a /r/<code> Share & Earn link in localStorage', () => {
      expect(stashInviteLink('/r/XYZ')).toBe('referral')
      expect(localStorage.getItem(REFERRAL_CODE_KEY)).toBe('XYZ')
    })

    it('ignores every other path', () => {
      for (const path of ['/', '/login', '/profile', '/shared/abc', '/join/', '/r/']) {
        expect(stashInviteLink(path), path).toBeNull()
      }
      expect(localStorage.length).toBe(0)
    })
  })

  describe('the round trip through Rally sign-in', () => {
    // A full page navigation away and back keeps web storage and nothing else.
    // Reading the keys back with no in-memory state is that round trip.

    it('a household invite is still pending on return', () => {
      stashInviteLink('/join/ABC123')
      // …browser goes to Rally, comes back to /login, session is exchanged…
      expect(land('/login').screen).toBe('home')
      // consumeLanding leaves the join code for tryJoinCode to redeem.
      expect(localStorage.getItem(JOIN_CODE_KEY)).toBe('ABC123')
    })

    it('a Share & Earn code is still pending on return', () => {
      stashInviteLink('/r/XYZ')
      land('/login')
      expect(localStorage.getItem(REFERRAL_CODE_KEY)).toBe('XYZ')
    })

    it('a shared-list invite becomes the join prompt on home', () => {
      stashShareInvite('INV42', 'Tamika')
      expect(land('/login')).toEqual({ screen: 'home', joinPrompt: { code: 'INV42', name: 'Tamika' } })
    })

    it('a shared-list invite is offered once, not on every load', () => {
      stashShareInvite('INV42', 'Tamika')
      land('/login')
      expect(localStorage.getItem(SHARE_INVITE_KEY)).toBeNull()
      expect(localStorage.getItem(SHARE_NAME_KEY)).toBeNull()
      expect(land('/login')).toEqual({ screen: 'home' })
    })

    it('a deep link is remembered signed-out and honoured after sign-in', () => {
      rememberDestination<Screen>('/profile', PATHS, SIGNED_OUT)
      expect(sessionStorage.getItem(REDIRECT_KEY)).toBe('/profile')
      expect(land('/login')).toEqual({ screen: 'profile' })
      // Used once.
      expect(land('/login')).toEqual({ screen: 'home' })
    })

    it('a shared-list invite outranks a remembered deep link', () => {
      rememberDestination<Screen>('/profile', PATHS, SIGNED_OUT)
      stashShareInvite('INV42', '')
      expect(land('/login')).toEqual({ screen: 'home', joinPrompt: { code: 'INV42', name: '' } })
    })
  })

  describe('rememberDestination', () => {
    it('does not remember the login screen or its retired aliases', () => {
      rememberDestination<Screen>('/login', PATHS, SIGNED_OUT)
      rememberDestination<Screen>('/verify', PATHS, SIGNED_OUT)
      expect(sessionStorage.getItem(REDIRECT_KEY)).toBeNull()
    })

    it('does not remember unknown paths', () => {
      rememberDestination<Screen>('/nope', PATHS, SIGNED_OUT)
      expect(sessionStorage.getItem(REDIRECT_KEY)).toBeNull()
    })
  })

  describe('consumeLanding', () => {
    it('lands on the screen the URL names when already signed in', () => {
      expect(land('/share')).toEqual({ screen: 'settings-share' })
    })

    it('never lands a signed-in user on the login screen', () => {
      expect(land('/login').screen).toBe('home')
      expect(land('/verify').screen).toBe('home')
    })

    it('falls back to home for a remembered path that no longer exists', () => {
      sessionStorage.setItem(REDIRECT_KEY, '/gone')
      expect(land('/login')).toEqual({ screen: 'home' })
    })

    it('defaults to home', () => {
      expect(land('/')).toEqual({ screen: 'home' })
    })
  })
})
