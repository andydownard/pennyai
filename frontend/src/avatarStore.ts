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

// Up to two initials: first letter of the first word and of the last word,
// across first and last name. "Andy Downard" → "AD"; "Mary Ann Smith" → "MS".
export function initialsOf(firstName?: string, lastName?: string): string {
  const words = `${firstName || ''} ${lastName || ''}`.trim().split(/\s+/).filter(Boolean)
  if (words.length === 0) return ''
  const first = words[0][0]
  const last = words.length > 1 ? words[words.length - 1][0] : ''
  return (first + last).toUpperCase()
}
