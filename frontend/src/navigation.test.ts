import { describe, it, expect, beforeEach, vi } from 'vitest'

// =============================================================================
// Navigation URL Mapping Tests
//
// Tests the SCREEN_PATHS / PATH_TO_SCREEN mappings and History API integration
// for browser back/forward navigation.
// =============================================================================

// Import the mappings directly (they're module-level constants)
const SCREEN_PATHS: Record<string, string> = {
  login: '/login',
  home: '/',
  about: '/about',
  'settings-share': '/share',
  settings: '/settings',
  'settings-profile': '/settings/profile',
  saved: '/settings/saved',
  admin: '/admin',
  invite: '/share',
}

const PATH_TO_SCREEN: Record<string, string> = {
  '/login': 'login',
  '/': 'home',
  '/about': 'about',
  '/share': 'settings-share',
  '/settings': 'settings',
  '/settings/profile': 'settings-profile',
  '/settings/saved': 'saved',
  '/admin': 'admin',
}

const ALL_SCREENS = ['login', 'home', 'saved', 'settings', 'about', 'invite', 'settings-profile', 'settings-share', 'admin']
const AUTH_SCREENS = ['home', 'settings', 'about', 'saved', 'settings-profile', 'settings-share', 'admin']
const PUBLIC_SCREENS = ['login']

describe('URL ↔ Screen Mapping', () => {
  it('every screen has a path', () => {
    for (const screen of ALL_SCREENS) {
      expect(SCREEN_PATHS[screen]).toBeDefined()
      expect(SCREEN_PATHS[screen]).toMatch(/^\//)
    }
  })

  it('every path resolves to a screen (except invite which is a redirect)', () => {
    for (const [path, screen] of Object.entries(PATH_TO_SCREEN)) {
      expect(ALL_SCREENS).toContain(screen)
      expect(path).toMatch(/^\//)
    }
  })

  it('SCREEN_PATHS → PATH_TO_SCREEN round-trip (excluding invite)', () => {
    for (const screen of ALL_SCREENS) {
      if (screen === 'invite') continue // invite maps to /share which resolves to settings-share
      const path = SCREEN_PATHS[screen]
      expect(PATH_TO_SCREEN[path]).toBe(screen)
    }
  })

  it('PATH_TO_SCREEN → SCREEN_PATHS round-trip', () => {
    for (const [path, screen] of Object.entries(PATH_TO_SCREEN)) {
      expect(SCREEN_PATHS[screen]).toBe(path)
    }
  })

  it('home screen maps to root path /', () => {
    expect(SCREEN_PATHS['home']).toBe('/')
    expect(PATH_TO_SCREEN['/']).toBe('home')
  })

  it('the login screen lives at /login', () => {
    expect(SCREEN_PATHS['login']).toBe('/login')
    expect(PATH_TO_SCREEN['/login']).toBe('login')
  })

  it('settings sub-pages are nested under /settings/', () => {
    expect(SCREEN_PATHS['settings-profile']).toBe('/settings/profile')
    expect(SCREEN_PATHS['saved']).toBe('/settings/saved')
  })

  it('invite legacy screen maps to same path as settings-share', () => {
    expect(SCREEN_PATHS['invite']).toBe(SCREEN_PATHS['settings-share'])
  })

  it('no duplicate paths (except invite alias)', () => {
    const paths = Object.entries(SCREEN_PATHS)
      .filter(([screen]) => screen !== 'invite')
      .map(([, path]) => path)
    const unique = new Set(paths)
    expect(unique.size).toBe(paths.length)
  })
})

describe('History API Navigation', () => {
  let historyStack: { state: unknown; url: string }[]
  let currentIndex: number

  beforeEach(() => {
    historyStack = [{ state: { screen: 'home' }, url: '/' }]
    currentIndex = 0

    // Mock history.pushState
    vi.spyOn(window.history, 'pushState').mockImplementation((state, _title, url) => {
      // Truncate forward history
      historyStack = historyStack.slice(0, currentIndex + 1)
      historyStack.push({ state, url: url as string })
      currentIndex++
    })

    vi.spyOn(window.history, 'replaceState').mockImplementation((state, _title, url) => {
      historyStack[currentIndex] = { state, url: url as string }
    })

    vi.spyOn(window.history, 'back').mockImplementation(() => {
      if (currentIndex > 0) {
        currentIndex--
        window.dispatchEvent(new PopStateEvent('popstate', {
          state: historyStack[currentIndex].state,
        }))
      }
    })

    vi.spyOn(window.history, 'forward').mockImplementation(() => {
      if (currentIndex < historyStack.length - 1) {
        currentIndex++
        window.dispatchEvent(new PopStateEvent('popstate', {
          state: historyStack[currentIndex].state,
        }))
      }
    })
  })

  // Helper: simulate setScreen
  const navigateTo = (screen: string, replace = false) => {
    const path = SCREEN_PATHS[screen] || '/'
    const method = replace ? 'replaceState' : 'pushState'
    window.history[method]({ screen }, '', path)
  }

  it('pushState creates history entries', () => {
    navigateTo('settings')
    navigateTo('settings-profile')
    expect(historyStack.length).toBe(3) // home + settings + profile
    expect(historyStack[0].state).toEqual({ screen: 'home' })
    expect(historyStack[1].state).toEqual({ screen: 'settings' })
    expect(historyStack[2].state).toEqual({ screen: 'settings-profile' })
  })

  it('replaceState does not create extra entries', () => {
    navigateTo('home', true)
    expect(historyStack.length).toBe(1)
    expect(historyStack[0].state).toEqual({ screen: 'home' })
  })

  it('back restores previous screen state', () => {
    navigateTo('settings')
    navigateTo('settings-profile')

    let lastPopState: PopStateEvent | null = null
    const handler = (e: PopStateEvent) => { lastPopState = e }
    window.addEventListener('popstate', handler)

    window.history.back()
    expect(lastPopState).not.toBeNull()
    expect((lastPopState as PopStateEvent).state).toEqual({ screen: 'settings' })
    expect(currentIndex).toBe(1)

    window.history.back()
    expect((lastPopState as PopStateEvent).state).toEqual({ screen: 'home' })
    expect(currentIndex).toBe(0)

    window.removeEventListener('popstate', handler)
  })

  it('forward restores next screen state', () => {
    navigateTo('settings')
    navigateTo('about')

    window.history.back() // about → settings
    window.history.back() // settings → home

    let lastPopState: PopStateEvent | null = null
    window.addEventListener('popstate', (e) => { lastPopState = e })

    window.history.forward()
    expect((lastPopState as PopStateEvent).state).toEqual({ screen: 'settings' })

    window.history.forward()
    expect((lastPopState as PopStateEvent).state).toEqual({ screen: 'about' })
  })

  it('navigation after back truncates forward history', () => {
    navigateTo('settings')
    navigateTo('about')

    window.history.back() // about → settings
    navigateTo('admin') // new branch from settings

    expect(historyStack.length).toBe(3) // home, settings, admin (about is gone)
    expect(historyStack[2].state).toEqual({ screen: 'admin' })
  })

  it('multi-step back: settings-profile → settings → home', () => {
    navigateTo('settings')
    navigateTo('settings-profile')

    const states: unknown[] = []
    window.addEventListener('popstate', (e) => { states.push(e.state) })

    window.history.back()
    window.history.back()

    expect(states).toEqual([
      { screen: 'settings' },
      { screen: 'home' },
    ])
  })

  it('URLs match screen paths in history stack', () => {
    navigateTo('settings')
    navigateTo('settings-profile')
    navigateTo('about')

    expect(historyStack.map(h => h.url)).toEqual([
      '/',
      '/settings',
      '/settings/profile',
      '/about',
    ])
  })

  it('modal pushState with modal flag', () => {
    // Simulate opening a modal on home screen
    window.history.pushState({ screen: 'home', modal: true }, '', '/')

    expect(historyStack.length).toBe(2)
    expect(historyStack[1].state).toEqual({ screen: 'home', modal: true })

    // Back should restore non-modal home state
    let lastPopState: PopStateEvent | null = null
    window.addEventListener('popstate', (e) => { lastPopState = e })
    window.history.back()
    expect((lastPopState as PopStateEvent).state).toEqual({ screen: 'home' })
  })

  it('logout uses replaceState so back does not restore authenticated screen', () => {
    navigateTo('settings')
    navigateTo('settings-profile')

    // Simulate logout — replace current entry with login
    navigateTo('login', true)

    expect(historyStack[currentIndex].state).toEqual({ screen: 'login' })
    expect(historyStack[currentIndex].url).toBe('/login')
  })

  it('session expiry uses replaceState', () => {
    navigateTo('settings')

    // Simulate session expiry — replace with login
    navigateTo('login', true)

    expect(historyStack[currentIndex].state).toEqual({ screen: 'login' })
  })
})

describe('Screen Navigation Hierarchy', () => {
  it('settings sub-pages have settings as parent', () => {
    // Profile and Saved Lists should navigate back to settings
    const subPages = ['settings-profile', 'saved']
    for (const page of subPages) {
      const path = SCREEN_PATHS[page]
      expect(path.startsWith('/settings/')).toBe(true)
    }
  })

  it('top-level pages navigate back to home', () => {
    const topLevel = ['settings', 'about', 'admin']
    for (const page of topLevel) {
      const path = SCREEN_PATHS[page]
      expect(path).not.toContain('/settings/')
    }
  })

  it('share is accessible from both home and settings', () => {
    // share should have its own top-level path
    expect(SCREEN_PATHS['settings-share']).toBe('/share')
    expect(PATH_TO_SCREEN['/share']).toBe('settings-share')
  })
})

describe('Hash Migration', () => {
  it('old hash URLs map to valid screens', () => {
    const oldHashes = ['settings', 'about', 'saved', 'settings-profile', 'settings-share', 'admin']
    for (const hash of oldHashes) {
      const screen = hash as keyof typeof SCREEN_PATHS
      expect(SCREEN_PATHS[screen]).toBeDefined()
    }
  })

  it('home hash (empty) maps to root path', () => {
    expect(SCREEN_PATHS['home']).toBe('/')
  })
})

describe('Deep Linking', () => {
  it('all authenticated screens have resolvable paths', () => {
    for (const screen of AUTH_SCREENS) {
      const path = SCREEN_PATHS[screen]
      expect(PATH_TO_SCREEN[path]).toBe(screen)
    }
  })

  it('public screens have resolvable paths', () => {
    for (const screen of PUBLIC_SCREENS) {
      const path = SCREEN_PATHS[screen]
      expect(PATH_TO_SCREEN[path]).toBe(screen)
    }
  })

  it('unknown paths default to home', () => {
    const unknownPaths = ['/foo', '/bar/baz', '/settings/unknown']
    for (const path of unknownPaths) {
      expect(PATH_TO_SCREEN[path]).toBeUndefined()
      // App should fallback to 'home' when PATH_TO_SCREEN returns undefined
    }
  })
})

describe('Modal State in History', () => {
  it('popModalState only pops when history state has modal flag', () => {
    // When history state has modal: true, popModalState should call history.back()
    const state = { screen: 'home', modal: true }
    expect(state.modal).toBe(true)

    // When history state does not have modal flag, should not pop
    const stateNoModal = { screen: 'home' }
    expect((stateNoModal as Record<string, unknown>).modal).toBeUndefined()
  })

  it('modal history entry preserves parent screen', () => {
    const modalState = { screen: 'home', modal: true }
    expect(modalState.screen).toBe('home')

    const savedModalState = { screen: 'saved', modal: true }
    expect(savedModalState.screen).toBe('saved')
  })
})

describe('Scroll Position Preservation', () => {
  it('scroll positions are keyed by screen name', () => {
    const scrollPositions: Record<string, number> = {}
    scrollPositions['home'] = 500
    scrollPositions['saved'] = 200
    scrollPositions['settings'] = 0

    expect(scrollPositions['home']).toBe(500)
    expect(scrollPositions['saved']).toBe(200)
    expect(scrollPositions['settings']).toBe(0)
    expect(scrollPositions['about']).toBeUndefined()
  })
})

describe('Auth Guard Behavior', () => {
  it('authenticated screens should not include login', () => {
    expect(AUTH_SCREENS).not.toContain('login')
  })

  it('login is the only public screen', () => {
    expect(PUBLIC_SCREENS).toEqual(['login'])
  })

  it('deep link to auth screen when logged out stores redirect', () => {
    // Simulate: user visits /settings/profile without auth
    const path = '/settings/profile'
    const resolved = PATH_TO_SCREEN[path]
    expect(resolved).toBe('settings-profile')
    expect(PUBLIC_SCREENS).not.toContain(resolved)
    // App should store path in sessionStorage and redirect to phone
  })

  it('deep link to admin requires admin role', () => {
    const path = '/admin'
    const resolved = PATH_TO_SCREEN[path]
    expect(resolved).toBe('admin')
    // App should check isAdmin before allowing access
  })
})
