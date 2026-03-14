import type { ReactNode } from 'react'
import { BottomSheet } from './BottomSheet'

// Re-export SectionHeader from its standalone file for backward compatibility
export { SectionHeader } from './SectionHeader'

// Re-export useFocusTrap from BottomSheet for backward compatibility
// (ConfirmModal and other components import it from here)
export { useFocusTrap } from './BottomSheet'

export function ActionSheet({ isOpen, onClose, title, children, maxHeight }: {
  isOpen: boolean
  onClose: () => void
  title?: string
  children: ReactNode
  maxHeight?: string
}) {
  return (
    <BottomSheet
      open={isOpen}
      onClose={onClose}
      title={title}
      maxHeight={maxHeight}
    >
      {children}
    </BottomSheet>
  )
}
