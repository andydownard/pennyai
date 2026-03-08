import { useEffect, useRef } from 'react'
import type { ReactNode } from 'react'
import { listStyles } from '../styles'

// Re-export SectionHeader from its standalone file for backward compatibility
export { SectionHeader } from './SectionHeader'

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

export function ActionSheet({ isOpen, onClose, title, children, maxHeight }: {
  isOpen: boolean
  onClose: () => void
  title?: string
  children: ReactNode
  maxHeight?: string
}) {
  const trapRef = useFocusTrap(isOpen)
  useEffect(() => {
    if (!isOpen) return
    const handle = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', handle)
    return () => document.removeEventListener('keydown', handle)
  }, [isOpen, onClose])
  if (!isOpen) return null
  return (
    <div style={listStyles.overlay} onClick={onClose} role="presentation">
      <div ref={trapRef} role="dialog" aria-modal="true" aria-label={title || 'Dialog'} style={{ ...listStyles.sheet, ...(maxHeight ? { maxHeight } : {}) }} onClick={e => e.stopPropagation()}>
        <div style={{ width: '36px', height: '4px', borderRadius: '2px', backgroundColor: 'var(--border)', margin: '0 auto 12px', flexShrink: 0 }} />
        {title && <p style={listStyles.sheetTitle}>{title}</p>}
        {children}
      </div>
    </div>
  )
}

