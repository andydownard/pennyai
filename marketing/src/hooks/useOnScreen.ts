import { useState, useEffect, useRef, type RefObject } from 'react'

// Returns [ref, visible] tuple. Tuple form (not object) so callers must
// destructure — that satisfies eslint-plugin-react-hooks `react-hooks/refs`,
// which forbids reading `.current`-bearing properties off an object during
// render. The boolean half is plain state and is safe to read in JSX.
export function useOnScreen(threshold = 0.05): readonly [RefObject<HTMLElement | null>, boolean] {
  const ref = useRef<HTMLElement | null>(null)
  const [visible, setVisible] = useState(false)
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const obs = new IntersectionObserver(
      ([e]) => { if (e.isIntersecting) { setVisible(true); obs.disconnect() } },
      { threshold, rootMargin: '0px 0px -40px 0px' }
    )
    obs.observe(el)
    return () => obs.disconnect()
  }, [threshold])
  return [ref, visible] as const
}
