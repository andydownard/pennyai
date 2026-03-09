import { useState, useRef, useEffect } from 'react'
import { FONT, FONT_XS, FONT_SM, FONT_BASE, FONT_MD } from '../styles'

interface BugReportFABProps {
  onSessionExpired: () => void
  showToast: (msg: string, type?: 'success' | 'error') => void
}

export function BugReportFAB({ onSessionExpired, showToast }: BugReportFABProps) {
  const [open, setOpen] = useState(false)
  const [body, setBody] = useState('')
  const [sending, setSending] = useState(false)
  const [sent, setSent] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    if (open && textareaRef.current) {
      setTimeout(() => textareaRef.current?.focus(), 200)
    }
  }, [open])

  useEffect(() => {
    if (sent) {
      const t = setTimeout(() => { setSent(false); setOpen(false) }, 1800)
      return () => clearTimeout(t)
    }
  }, [sent])

  const handleSubmit = async () => {
    if (!body.trim() || sending) return
    setSending(true)
    try {
      const res = await fetch('/api/bug-report', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ description: body.trim() }),
      })
      if (res.status === 401) { onSessionExpired(); setOpen(false); return }
      if (res.ok) {
        setBody('')
        setSent(true)
        showToast('Bug report submitted — thank you!')
      } else {
        const data = await res.json().catch(() => ({}))
        showToast(data?.error || 'Failed to submit report', 'error')
      }
    } catch {
      showToast('Something went wrong', 'error')
    } finally {
      setSending(false)
    }
  }

  const canSend = !!body.trim() && !sending

  return (
    <>
      {/* Small bug icon — bottom-left, above tab bar */}
      {!open && (
        <button
          onClick={() => setOpen(true)}
          aria-label="Report a bug"
          style={{
            position: 'fixed',
            bottom: '68px',
            left: '16px',
            zIndex: 40,
            width: '32px',
            height: '32px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            backgroundColor: 'var(--surface)',
            color: 'var(--text-secondary)',
            border: '1px solid var(--border)',
            borderRadius: '50%',
            cursor: 'pointer',
            opacity: 0.6,
            transition: 'opacity 0.15s',
          }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M8 2l1.88 1.88"/>
            <path d="M14.12 3.88L16 2"/>
            <path d="M9 7.13v-1a3.003 3.003 0 1 1 6 0v1"/>
            <path d="M12 20c-3.3 0-6-2.7-6-6v-3a4 4 0 0 1 4-4h4a4 4 0 0 1 4 4v3c0 3.3-2.7 6-6 6"/>
            <path d="M12 20v-9"/>
            <path d="M6.53 9C4.6 8.8 3 7.1 3 5"/>
            <path d="M6 13H2"/>
            <path d="M3 21c0-2.1 1.7-3.9 3.8-4"/>
            <path d="M20.97 5c0 2.1-1.6 3.8-3.5 4"/>
            <path d="M22 13h-4"/>
            <path d="M17.2 17c2.1.1 3.8 1.9 3.8 4"/>
          </svg>
        </button>
      )}

      {/* Backdrop */}
      {open && (
        <div
          onClick={() => { if (!sending) setOpen(false) }}
          style={{
            position: 'fixed',
            inset: 0,
            zIndex: 9998,
            backgroundColor: 'rgba(0,0,0,0.4)',
          }}
        />
      )}

      {/* Bottom sheet */}
      {open && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Report a bug"
          style={{
            position: 'fixed',
            bottom: 0,
            left: 0,
            right: 0,
            zIndex: 9999,
            backgroundColor: 'var(--surface)',
            borderTopLeftRadius: '20px',
            borderTopRightRadius: '20px',
            boxShadow: '0 -8px 32px rgba(0,0,0,0.12)',
            padding: '20px 20px 32px',
            paddingBottom: 'max(32px, env(safe-area-inset-bottom, 0px))',
            maxHeight: '70vh',
            overflowY: 'auto',
          }}
        >
          {/* Drag handle */}
          <div style={{ display: 'flex', justifyContent: 'center', marginBottom: '12px' }}>
            <div style={{ width: '36px', height: '4px', borderRadius: '2px', backgroundColor: 'var(--border)' }} />
          </div>

          {sent ? (
            <div style={{ textAlign: 'center', padding: '24px 0' }}>
              <div style={{ fontSize: '32px', marginBottom: '12px', color: 'var(--brand)' }}>&#10003;</div>
              <div style={{ fontFamily: FONT, fontSize: FONT_MD, fontWeight: 600, color: 'var(--text)' }}>
                Thanks for the report!
              </div>
              <div style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', marginTop: '4px' }}>
                We'll look into it.
              </div>
            </div>
          ) : (
            <>
              <div style={{ marginBottom: '14px' }}>
                <div style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 700, color: 'var(--text)', marginBottom: '4px' }}>
                  Report a Bug
                </div>
                <p style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', margin: 0, lineHeight: 1.4 }}>
                  Found something broken? We'll fix it fast.
                </p>
              </div>

              <textarea
                ref={textareaRef}
                value={body}
                onChange={e => setBody(e.target.value)}
                placeholder="What went wrong?"
                maxLength={5000}
                rows={3}
                style={{
                  width: '100%',
                  padding: '12px 14px',
                  fontFamily: FONT,
                  fontSize: FONT_SM,
                  color: 'var(--text)',
                  backgroundColor: 'var(--bg)',
                  border: '1.5px solid var(--border)',
                  borderRadius: '14px',
                  outline: 'none',
                  resize: 'none',
                  minHeight: '80px',
                  boxSizing: 'border-box',
                }}
              />

              <button
                onClick={handleSubmit}
                disabled={!canSend}
                style={{
                  marginTop: '12px',
                  width: '100%',
                  padding: '12px 0',
                  backgroundColor: canSend ? 'var(--brand)' : 'var(--border)',
                  color: canSend ? '#fff' : 'var(--text-secondary)',
                  border: 'none',
                  borderRadius: '12px',
                  fontFamily: FONT,
                  fontSize: FONT_BASE,
                  fontWeight: 600,
                  cursor: canSend ? 'pointer' : 'default',
                  transition: 'background-color 0.15s, color 0.15s',
                }}
              >
                {sending ? 'Sending...' : 'Submit Report'}
              </button>
            </>
          )}
        </div>
      )}
    </>
  )
}
