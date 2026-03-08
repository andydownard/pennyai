import { useRef, useEffect, useState, type ReactNode, type CSSProperties } from 'react'

// =============================================================================
// SCREEN TRANSITION — smooth fade / slide animations between screens
// =============================================================================

type Screen = 'phone' | 'code' | 'home' | 'penny' | 'profile' | 'settings-share'
  | 'admin' | 'email-login' | 'email-register' | 'forgot-password'
  | 'reset-password' | 'shared-list' | 'help' | 'earnings' | 'receipts'

// Screens that are "children" of a parent tab screen
const SUB_SCREEN_PARENTS: Partial<Record<Screen, Screen>> = {
  earnings: 'home',
  receipts: 'home',
  help: 'profile',
  admin: 'profile',
}

// The four main tab screens
const TAB_SCREENS: Set<Screen> = new Set(['home', 'penny', 'settings-share', 'profile'])

type TransitionType = 'fade' | 'slide-in' | 'slide-out' | 'none'

function getTransitionType(prev: Screen | null, next: Screen): TransitionType {
  if (!prev || prev === next) return 'none'

  const prevParent = SUB_SCREEN_PARENTS[prev]
  const nextParent = SUB_SCREEN_PARENTS[next]

  // Drill down: tab → its sub-screen (e.g. home → earnings)
  if (nextParent === prev) return 'slide-in'

  // Back: sub-screen → its parent tab (e.g. earnings → home)
  if (prevParent === next) return 'slide-out'

  // Drill deeper within same parent: treat as slide-in (e.g. earnings → receipts)
  if (nextParent && prevParent && nextParent === prevParent) return 'slide-in'

  // Tab-to-tab navigation
  if (TAB_SCREENS.has(prev) && TAB_SCREENS.has(next)) return 'fade'

  // Sub-screen back to a different tab (e.g. help → home): fade
  if (prevParent && TAB_SCREENS.has(next)) return 'fade'

  // Any other navigation: fade
  return 'fade'
}

// Duration in ms for each transition type
const DURATIONS: Record<TransitionType, number> = {
  fade: 200,
  'slide-in': 250,
  'slide-out': 250,
  none: 0,
}

interface ScreenTransitionProps {
  screen: Screen
  children: ReactNode
}

export function ScreenTransition({ screen, children }: ScreenTransitionProps) {
  const prevScreenRef = useRef<Screen | null>(null)
  const [phase, setPhase] = useState<'idle' | 'entering'>('idle')
  const [transitionType, setTransitionType] = useState<TransitionType>('none')
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Check reduced-motion preference
  const prefersReducedMotion = useRef(false)
  useEffect(() => {
    const mql = window.matchMedia('(prefers-reduced-motion: reduce)')
    prefersReducedMotion.current = mql.matches
    const handler = (e: MediaQueryListEvent) => { prefersReducedMotion.current = e.matches }
    mql.addEventListener('change', handler)
    return () => mql.removeEventListener('change', handler)
  }, [])

  useEffect(() => {
    const prev = prevScreenRef.current
    prevScreenRef.current = screen

    if (prev === null || prev === screen) return

    // Clean up any pending timer
    if (timerRef.current) clearTimeout(timerRef.current)

    if (prefersReducedMotion.current) {
      // Skip animation entirely
      setTransitionType('none')
      setPhase('idle')
      return
    }

    const type = getTransitionType(prev, screen)
    setTransitionType(type)

    if (type === 'none') {
      setPhase('idle')
      return
    }

    // Start at the "from" position, then transition to "to" position on next frame
    setPhase('entering')

    // Use rAF to ensure the browser renders the "from" state first
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        setPhase('idle')
        // Clean up transition type after animation completes
        timerRef.current = setTimeout(() => {
          setTransitionType('none')
        }, DURATIONS[type])
      })
    })

    return () => {
      if (timerRef.current) clearTimeout(timerRef.current)
    }
  }, [screen])

  const getStyles = (): CSSProperties => {
    const duration = DURATIONS[transitionType]

    if (transitionType === 'none') {
      return { opacity: 1, transform: 'translate3d(0, 0, 0)' }
    }

    const isEntering = phase === 'entering'

    if (transitionType === 'fade') {
      return {
        opacity: isEntering ? 0 : 1,
        transform: 'translate3d(0, 0, 0)',
        transition: `opacity ${duration}ms ease-out`,
        willChange: 'opacity',
      }
    }

    if (transitionType === 'slide-in') {
      return {
        opacity: isEntering ? 0 : 1,
        transform: isEntering ? 'translate3d(30%, 0, 0)' : 'translate3d(0, 0, 0)',
        transition: `transform ${duration}ms ease-out, opacity ${duration}ms ease-out`,
        willChange: 'transform, opacity',
      }
    }

    // slide-out: content slides out to right, but since we're showing the NEW screen,
    // we animate the new content sliding in from the left
    if (transitionType === 'slide-out') {
      return {
        opacity: isEntering ? 0 : 1,
        transform: isEntering ? 'translate3d(-30%, 0, 0)' : 'translate3d(0, 0, 0)',
        transition: `transform ${duration}ms ease-out, opacity ${duration}ms ease-out`,
        willChange: 'transform, opacity',
      }
    }

    return {}
  }

  return (
    <div
      style={{
        ...getStyles(),
        height: '100%',
        width: '100%',
      }}
    >
      {children}
    </div>
  )
}
