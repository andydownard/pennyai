import { useEffect, useRef, useState } from 'react'
import type { ReactNode, CSSProperties } from 'react'
import { listStyles } from '../styles'

/**
 * Generic bottom sheet primitive.
 *
 * Renders a dark overlay with a slide-up panel. Supports:
 * - Click overlay to close
 * - Escape key to close
 * - Focus trapping while open
 * - Optional title and drag handle
 * - CSS slide-up/down animation
 * - maxHeight override
 */

// Focus trap hook: constrains Tab/Shift+Tab within a container, restores focus on unmount.
export function useFocusTrap(isActive: boolean) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!isActive || !ref.current) return
    const prev = document.activeElement as HTMLElement | null
    const focusable = () => ref.current?.querySelectorAll<HTMLElement>('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])') || []
    const first = focusable()[0]
    first?.focus()
    const handleTab = (e: KeyboardEvent) => {
      if (e.key !== 'Tab') return
      const els = focusable()
      if (els.length === 0) return
      const firstEl = els[0], lastEl = els[els.length - 1]
      if (e.shiftKey && document.activeElement === firstEl) { e.preventDefault(); lastEl.focus() }
      else if (!e.shiftKey && document.activeElement === lastEl) { e.preventDefault(); firstEl.focus() }
    }
    document.addEventListener('keydown', handleTab)
    return () => { document.removeEventListener('keydown', handleTab); prev?.focus() }
  }, [isActive])
  return ref
}

export interface BottomSheetProps {
  open: boolean
  onClose: () => void
  title?: string
  children: ReactNode
  maxHeight?: string
  /** Show the drag handle bar at top of sheet. Defaults to true. */
  showHandle?: boolean
  /** Additional inline styles for the sheet panel */
  sheetStyle?: CSSProperties
  /** aria-label override (defaults to title or 'Dialog') */
  ariaLabel?: string
}

export function BottomSheet({
  open,
  onClose,
  title,
  children,
  maxHeight,
  showHandle = true,
  sheetStyle,
  ariaLabel,
}: BottomSheetProps) {
  const trapRef = useFocusTrap(open)
  const [visible, setVisible] = useState(false)
  const [animateIn, setAnimateIn] = useState(false)

  // Escape key handler
  useEffect(() => {
    if (!open) return
    const handle = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', handle)
    return () => document.removeEventListener('keydown', handle)
  }, [open, onClose])

  // Animation lifecycle: mount -> animate in, close -> animate out -> unmount
  useEffect(() => {
    if (open) {
      setVisible(true)
      // Trigger slide-up on next frame so the transition fires
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          setAnimateIn(true)
        })
      })
    } else {
      setAnimateIn(false)
      // Wait for slide-down transition to finish before unmounting
      const timer = setTimeout(() => setVisible(false), 250)
      return () => clearTimeout(timer)
    }
  }, [open])

  if (!visible) return null

  const overlayStyle: CSSProperties = {
    ...listStyles.overlay,
    opacity: animateIn ? 1 : 0,
    transition: 'opacity 0.25s ease',
  }

  const panelStyle: CSSProperties = {
    ...listStyles.sheet,
    ...(maxHeight ? { maxHeight } : {}),
    ...sheetStyle,
    transform: animateIn ? 'translateY(0)' : 'translateY(100%)',
    transition: 'transform 0.25s ease',
  }

  return (
    <div style={overlayStyle} onClick={onClose} role="presentation">
      <div
        ref={trapRef}
        role="dialog"
        aria-modal="true"
        aria-label={ariaLabel || title || 'Dialog'}
        style={panelStyle}
        onClick={e => e.stopPropagation()}
      >
        {showHandle && (
          <div style={{
            width: '36px',
            height: '4px',
            borderRadius: '2px',
            backgroundColor: 'var(--border)',
            margin: '0 auto 12px',
            flexShrink: 0,
          }} />
        )}
        {title && <p style={listStyles.sheetTitle}>{title}</p>}
        {children}
      </div>
    </div>
  )
}
