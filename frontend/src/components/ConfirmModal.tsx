import { useEffect } from 'react'
import { useFocusTrap } from './ActionSheet'
import { FONT, TEXT, ERROR_RED } from '../styles'

export function ConfirmModal({ open, title, message, confirmLabel, onConfirm, onCancel, destructive }: {
  open: boolean
  title: string
  message?: string
  confirmLabel?: string
  onConfirm: () => void
  onCancel: () => void
  destructive?: boolean
}) {
  const trapRef = useFocusTrap(open)
  useEffect(() => {
    if (!open) return
    const handle = (e: KeyboardEvent) => { if (e.key === 'Escape') onCancel() }
    document.addEventListener('keydown', handle)
    return () => document.removeEventListener('keydown', handle)
  }, [open, onCancel])
  if (!open) return null
  return (
    <div onClick={onCancel} role="presentation" style={{
      position: 'fixed', inset: 0, zIndex: 9999,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      backgroundColor: 'rgba(0,0,0,0.4)',
    }}>
      <div ref={trapRef} role="alertdialog" aria-modal="true" aria-label={title} onClick={e => e.stopPropagation()} style={{
        backgroundColor: '#fff', borderRadius: '14px', width: '270px',
        overflow: 'hidden', textAlign: 'center', fontFamily: FONT,
      }}>
        <div style={{ padding: '20px 16px 0' }}>
          <div style={{ fontSize: '17px', fontWeight: 600, color: TEXT }}>{title}</div>
          {message && <div style={{ fontSize: '14px', color: TEXT, marginTop: '6px', lineHeight: 1.4 }}>{message}</div>}
        </div>
        <div style={{ display: 'flex', borderTop: '1px solid #e5e5e5', marginTop: '16px' }}>
          <button onClick={onCancel} style={{
            flex: 1, padding: '12px', border: 'none', background: 'none',
            fontSize: '17px', fontFamily: FONT, color: '#007AFF', cursor: 'pointer',
            borderRight: '1px solid #e5e5e5',
          }}>Cancel</button>
          <button onClick={onConfirm} style={{
            flex: 1, padding: '12px', border: 'none', background: 'none',
            fontSize: '17px', fontWeight: 600, fontFamily: FONT, cursor: 'pointer',
            color: destructive ? ERROR_RED : '#007AFF',
          }}>{confirmLabel || 'Confirm'}</button>
        </div>
      </div>
    </div>
  )
}
