import { useState, useRef, useEffect } from 'react'
import { FONT, FONT_XS, FONT_BASE, FONT_MD, FONT_LG } from '../styles'

interface FeedbackWidgetProps {
  onSubmit: (body: string, type: string, urgency: string) => Promise<boolean>
}

const TYPES = [
  { key: 'bug', label: 'Bug' },
  { key: 'feature', label: 'Idea' },
  { key: 'other', label: 'Other' },
]

export function FeedbackWidget({ onSubmit }: FeedbackWidgetProps) {
  const [open, setOpen] = useState(false)
  const [body, setBody] = useState('')
  const [type, setType] = useState('feature')
  const [urgency, setUrgency] = useState('normal')
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
    const ok = await onSubmit(body.trim(), type, urgency)
    setSending(false)
    if (ok) {
      setBody('')
      setType('feature')
      setUrgency('normal')
      setSent(true)
    }
  }

  const canSend = !!body.trim() && !sending

  return (
    <>
      {/* FAB — small brand-colored circle, top-right corner */}
      {!open && (
        <button
          onClick={() => setOpen(true)}
          aria-label="Send feedback"
          style={{
            position: 'fixed',
            top: '72px',
            right: '16px',
            zIndex: 50,
            width: '36px',
            height: '36px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            backgroundColor: 'var(--brand)',
            color: '#fff',
            border: 'none',
            borderRadius: '50%',
            cursor: 'pointer',
            boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
            opacity: 0.8,
          }}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M9 18h6"/>
            <path d="M10 22h4"/>
            <path d="M12 2a7 7 0 0 0-4 12.7V17h8v-2.3A7 7 0 0 0 12 2z"/>
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
          aria-label="Send product feedback"
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
            maxHeight: '85vh',
            overflowY: 'auto',
            animation: 'feedbackSlideUp 0.25s ease-out',
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
                Got it!
              </div>
              <div style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', marginTop: '4px' }}>
                Your voice shapes what we build next.
              </div>
            </div>
          ) : (
            <>
              <div style={{ marginBottom: '16px' }}>
                <div style={{ fontFamily: FONT, fontSize: FONT_LG, fontWeight: 700, color: 'var(--text)', marginBottom: '4px' }}>
                  Help shape Penny
                </div>
                <p style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', margin: 0, lineHeight: 1.4 }}>
                  Your feedback goes directly to our product team.
                </p>
              </div>

              {/* Category chips */}
              <div style={{ display: 'flex', gap: '8px', marginBottom: '14px' }}>
                {TYPES.map(t => (
                  <button
                    key={t.key}
                    onClick={() => setType(t.key)}
                    style={{
                      padding: '7px 14px',
                      borderRadius: '16px',
                      fontFamily: FONT,
                      fontSize: FONT_XS,
                      fontWeight: 600,
                      cursor: 'pointer',
                      border: type === t.key ? '1.5px solid var(--brand)' : '1.5px solid var(--border)',
                      backgroundColor: type === t.key ? 'color-mix(in srgb, var(--brand) 10%, transparent)' : 'transparent',
                      color: type === t.key ? 'var(--brand)' : 'var(--text-secondary)',
                      transition: 'all 0.15s',
                    }}
                  >
                    {t.label}
                  </button>
                ))}
              </div>

              <textarea
                ref={textareaRef}
                value={body}
                onChange={e => setBody(e.target.value)}
                placeholder={
                  type === 'bug' ? "What went wrong?"
                    : type === 'feature' ? "What would make Penny better?"
                    : "What's on your mind?"
                }
                maxLength={10000}
                rows={3}
                style={{
                  width: '100%',
                  padding: '12px 14px',
                  fontFamily: FONT,
                  fontSize: FONT_BASE,
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

              {/* Urgency chips — tappable so keyboard stays up on iOS */}
              <div style={{ display: 'flex', gap: '6px', marginTop: '12px', marginBottom: '12px' }}>
                {([
                  { key: 'low', label: 'Low' },
                  { key: 'normal', label: 'Normal' },
                  { key: 'critical', label: 'Urgent' },
                ] as const).map(u => (
                  <button
                    key={u.key}
                    onClick={() => setUrgency(u.key)}
                    style={{
                      padding: '7px 14px',
                      borderRadius: '16px',
                      fontFamily: FONT,
                      fontSize: FONT_XS,
                      fontWeight: 600,
                      cursor: 'pointer',
                      border: urgency === u.key ? '1.5px solid var(--brand)' : '1.5px solid var(--border)',
                      backgroundColor: urgency === u.key ? 'color-mix(in srgb, var(--brand) 10%, transparent)' : 'transparent',
                      color: urgency === u.key ? 'var(--brand)' : 'var(--text-secondary)',
                      transition: 'all 0.15s',
                    }}
                  >
                    {u.label}
                  </button>
                ))}
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <button
                  onClick={handleSubmit}
                  disabled={!canSend}
                  style={{
                    flex: 1,
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
                  {sending ? 'Sending...' : 'Send'}
                </button>
              </div>
            </>
          )}
        </div>
      )}
    </>
  )
}
