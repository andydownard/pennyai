import { useSyncExternalStore } from 'react'

// Cache-buster for the signed-in user's photo. Profile bumps it after an upload so every
// avatar on screen (header, menu, profile) reloads without a page refresh.
let version = 0
const listeners = new Set<() => void>()

export function bumpAvatarVersion(): void {
  version += 1
  listeners.forEach(l => l())
}

export function useAvatarVersion(): number {
  return useSyncExternalStore(
    cb => { listeners.add(cb); return () => { listeners.delete(cb) } },
    () => version,
    () => version,
  )
}

export function initialsOf(firstName?: string, lastName?: string): string {
  return [firstName, lastName].map(n => (n || '')[0]?.toUpperCase() || '').filter(Boolean).join('')
}
