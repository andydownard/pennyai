import { useState, useEffect, useRef, type RefObject } from 'react'

// Returns [ref, visible] tuple. Default visible=true so SEO crawlers,
// fullPage screenshots, and slow IntersectionObservers always see content.
// We flip to hidden only when the element starts below the viewport on
// mount — those sections genuinely benefit from the fade-in animation.
export function useOnScreen(threshold = 0.05): readonly [RefObject<HTMLElement | null>, boolean] {
  const ref = useRef<HTMLElement | null>(null)
  const [visible, setVisible] = useState(true)
  useEffect(() => {
    const el = ref.current
    if (!el || typeof IntersectionObserver === 'undefined') return
    const rect = el.getBoundingClientRect()
    if (rect.top <= window.innerHeight) return
    setVisible(false)
    const obs = new IntersectionObserver(
      ([e]) => { if (e.isIntersecting) { setVisible(true); obs.disconnect() } },
      { threshold, rootMargin: '0px 0px -40px 0px' }
    )
    obs.observe(el)
    return () => obs.disconnect()
  }, [threshold])
  return [ref, visible] as const
}
