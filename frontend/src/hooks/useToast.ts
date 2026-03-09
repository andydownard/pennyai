import { useState, useRef, useCallback, useEffect } from 'react'

export interface ToastData {
  message: string
  type: 'success' | 'error'
  action?: { label: string; onClick: () => void }
}

export function useToast() {
  const [toast, setToast] = useState<ToastData | null>(null)
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const showToast = useCallback((message: string, type: 'success' | 'error' = 'success', action?: { label: string; onClick: () => void }) => {
    if (toastTimer.current) clearTimeout(toastTimer.current)
    setToast({ message, type, action })
    toastTimer.current = setTimeout(() => setToast(null), action ? 10000 : 6250)
  }, [])

  const clearToast = useCallback(() => {
    setToast(null)
    if (toastTimer.current) { clearTimeout(toastTimer.current); toastTimer.current = null }
  }, [])

  useEffect(() => {
    return () => { if (toastTimer.current) clearTimeout(toastTimer.current) }
  }, [])

  return { toast, showToast, clearToast } as const
}
