import { useState, useRef, useEffect } from 'react'
import { FONT } from '../styles'

interface FeedbackWidgetProps {
  onSubmit: (body: string, type: string, urgency: string) => Promise<boolean>
}

const TYPES = [
  { key: 'bug', label: 'Bug', icon: '🐛' },
  { key: 'feature', label: 'Idea', icon: '💡' },
  { key: 'other', label: 'Other', icon: '💬' },
]

const URGENCIES = [
  { key: 'normal', label: 'Normal' },
  { key: 'high', label: 'Important' },
  { key: 'critical', label: 'Urgent' },
]

export function FeedbackWidget({ onSubmit }: FeedbackWidgetProps) {
  const [open, setOpen] = useState(false)
  const [body, setBody] = useState('')
  const [type, setType] = useState('feature')
  const [urgency, setUrgency] = useState('normal')
  const [sending, setSending] = useState(false)
  const [sent, setSent] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  // Auto-focus textarea when sheet opens
  useEffect(() => {
    if (open && textareaRef.current) {
      setTimeout(() => textareaRef.current?.focus(), 200)
    }
  }, [open])

  // Reset after successful send
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
      {/* FAB — small branded pill, bottom-right above the tab bar + PennyBar area */}
      {!open && (
        <button
          onClick={() => setOpen(true)}
          aria-label="Send beta feedback"
          style={{
            position: 'fixed',
            bottom: '200px',
            right: '16px',
            zIndex: 50,
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            backgroundColor: 'var(--brand)',
            color: '#fff',
            border: 'none',
            borderRadius: '20px',
            padding: '10px 14px',
            fontFamily: FONT,
            fontSize: '13px',
            fontWeight: 600,
            cursor: 'pointer',
            boxShadow: '0 4px 16px rgba(0,0,0,0.18)',
            transition: 'transform 0.15s, box-shadow 0.15s',
          }}
        >
          {/* Lightbulb icon */}
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M9 18h6"/>
            <path d="M10 22h4"/>
            <path d="M12 2a7 7 0 0 0-4 12.7V17h8v-2.3A7 7 0 0 0 12 2z"/>
          </svg>
          <span style={{
            position: 'absolute',
            top: '-6px',
            right: '-4px',
            backgroundColor: '#E8B931',
            color: '#1a1a1a',
            fontSize: '9px',
            fontWeight: 800,
            letterSpacing: '0.5px',
            padding: '1px 5px',
            borderRadius: '6px',
            lineHeight: '14px',
          }}>BETA</span>
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
            transition: 'opacity 0.2s',
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
            /* Success state */
            <div style={{ textAlign: 'center', padding: '24px 0' }}>
              <div style={{ fontSize: '32px', marginBottom: '12px' }}>&#10003;</div>
              <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: 'var(--text)' }}>
                Got it!
              </div>
              <div style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text-secondary)', marginTop: '4px' }}>
                Your voice shapes what we build next.
              </div>
            </div>
          ) : (
            <>
              {/* Header */}
              <div style={{ marginBottom: '16px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
                  <span style={{ fontFamily: FONT, fontSize: '17px', fontWeight: 700, color: 'var(--text)' }}>
                    Help shape Penny
                  </span>
                  <span style={{
                    backgroundColor: '#E8B931',
                    color: '#1a1a1a',
                    fontSize: '9px',
                    fontWeight: 800,
                    letterSpacing: '0.5px',
                    padding: '2px 6px',
                    borderRadius: '6px',
                    lineHeight: '14px',
                  }}>BETA</span>
                </div>
                <p style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text-secondary)', margin: 0, lineHeight: 1.4 }}>
                  You're one of Penny's first users. Your feedback goes directly to our product team.
                </p>
              </div>

              {/* Category chips */}
              <div style={{ display: 'flex', gap: '8px', marginBottom: '14px' }}>
                {TYPES.map(t => (
                  <button
                    key={t.key}
                    onClick={() => setType(t.key)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '4px',
                      padding: '7px 12px',
                      borderRadius: '16px',
                      fontFamily: FONT,
                      fontSize: '13px',
                      fontWeight: 600,
                      cursor: 'pointer',
                      border: type === t.key ? '1.5px solid var(--brand)' : '1.5px solid var(--border)',
                      backgroundColor: type === t.key ? 'color-mix(in srgb, var(--brand) 10%, transparent)' : 'transparent',
                      color: type === t.key ? 'var(--brand)' : 'var(--text-secondary)',
                      transition: 'all 0.15s',
                    }}
                  >
                    <span style={{ fontSize: '14px' }}>{t.icon}</span>
                    {t.label}
                  </button>
                ))}
              </div>

              {/* Textarea */}
              <textarea
                ref={textareaRef}
                value={body}
                onChange={e => setBody(e.target.value)}
                placeholder={
                  type === 'bug' ? "What went wrong? Steps to reproduce help a lot..."
                    : type === 'feature' ? "What would make Penny better for you?"
                    : "What's on your mind?"
                }
                maxLength={10000}
                rows={4}
                style={{
                  width: '100%',
                  padding: '12px 14px',
                  fontFamily: FONT,
                  fontSize: '15px',
                  color: 'var(--text)',
                  backgroundColor: 'var(--bg)',
                  border: '1.5px solid var(--border)',
                  borderRadius: '14px',
                  outline: 'none',
                  resize: 'vertical',
                  minHeight: '100px',
                  maxHeight: '200px',
                  boxSizing: 'border-box',
                }}
              />

              {/* Urgency row */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '12px', marginBottom: '16px' }}>
                <span style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', fontWeight: 500 }}>Priority:</span>
                {URGENCIES.map(u => (
                  <button
                    key={u.key}
                    onClick={() => setUrgency(u.key)}
                    style={{
                      padding: '4px 10px',
                      borderRadius: '12px',
                      fontFamily: FONT,
                      fontSize: '12px',
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

              {/* Submit */}
              <button
                onClick={handleSubmit}
                disabled={!canSend}
                style={{
                  width: '100%',
                  padding: '14px 0',
                  backgroundColor: canSend ? 'var(--brand)' : 'var(--border)',
                  color: canSend ? '#fff' : 'var(--text-secondary)',
                  border: 'none',
                  borderRadius: '12px',
                  fontFamily: FONT,
                  fontSize: '15px',
                  fontWeight: 600,
                  cursor: canSend ? 'pointer' : 'default',
                  transition: 'background-color 0.15s, color 0.15s',
                }}
              >
                {sending ? 'Sending...' : 'Send feedback'}
              </button>
            </>
          )}
        </div>
      )}
    </>
  )
}
