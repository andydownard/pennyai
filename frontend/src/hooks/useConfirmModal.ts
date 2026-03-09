import { useState, useRef, useCallback } from 'react'

export interface ConfirmModalData {
  title: string
  message?: string
  confirmLabel?: string
  destructive?: boolean
}

export function useConfirmModal() {
  const [confirmModal, setConfirmModal] = useState<ConfirmModalData | null>(null)
  const confirmActionRef = useRef<(() => void) | null>(null)

  const openConfirmModal = useCallback((opts: ConfirmModalData & { onConfirm: () => void }) => {
    confirmActionRef.current = opts.onConfirm
    setConfirmModal({ title: opts.title, message: opts.message, confirmLabel: opts.confirmLabel, destructive: opts.destructive })
  }, [])

  const closeConfirmModal = useCallback(() => {
    setConfirmModal(null)
    confirmActionRef.current = null
  }, [])

  const runConfirmAction = useCallback(() => {
    const action = confirmActionRef.current
    closeConfirmModal()
    if (action) action()
  }, [closeConfirmModal])

  return { confirmModal, openConfirmModal, closeConfirmModal, runConfirmAction } as const
}
