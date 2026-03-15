import { useState, useRef, useEffect, useCallback } from 'react'
import { FONT, FONT_2XS, FONT_XS, FONT_SM, FONT_BASE, FONT_MD, FONT_LG } from '../styles'

interface FeedbackWidgetProps {
  onSubmit: (body: string, type: string, urgency: string) => Promise<boolean>
}

interface FeedbackItem {
  id: number
  body: string
  type: string
  urgency: string
  status: string
  admin_notes: string
  github_issue_url: string
  created_at: string
  updated_at: string
}

const TYPES = [
  { key: 'bug', label: 'Bug' },
  { key: 'feature', label: 'Idea' },
  { key: 'other', label: 'Other' },
]

const URGENCY_COLORS: Record<string, string> = {
  low: 'var(--text-secondary)',
  normal: 'var(--brand)',
  high: '#E6A817',
  critical: '#C53030',
}

const STATUS_CONFIG: Record<string, { label: string; bg: string; color: string }> = {
  new:        { label: 'New',        bg: 'color-mix(in srgb, var(--brand) 12%, transparent)', color: 'var(--brand)' },
  reviewed:   { label: 'Reviewed',   bg: 'color-mix(in srgb, var(--brand) 8%, transparent)',  color: 'var(--text-secondary)' },
  roadmapped: { label: 'Roadmapped', bg: 'color-mix(in srgb, #E6A817 12%, transparent)',     color: '#E6A817' },
  done:       { label: 'Done',       bg: 'color-mix(in srgb, var(--success) 12%, transparent)', color: 'var(--success)' },
}

function formatDate(d: string): string {
  try {
    const dt = new Date(d.includes('Z') || d.includes('+') ? d : d + 'Z')
    return dt.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    })
  } catch {
    return d
  }
}

const IS_DESKTOP = typeof window !== 'undefined' && window.innerWidth >= 640

export function FeedbackWidget({ onSubmit }: FeedbackWidgetProps) {
  const [open, setOpen] = useState(false)
  const [tab, setTab] = useState<'write' | 'history'>('write')
  const [body, setBody] = useState('')
  const [type, setType] = useState('feature')
  const [urgency, setUrgency] = useState('normal')
  const [sending, setSending] = useState(false)
  const [sent, setSent] = useState(false)
  const [history, setHistory] = useState<FeedbackItem[]>([])
  const [loadingHistory, setLoadingHistory] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const fetchHistory = useCallback(async () => {
    setLoadingHistory(true)
    try {
      const res = await fetch('/api/feedback')
      if (res.ok) {
        const data = await res.json()
        setHistory(Array.isArray(data) ? data : [])
      }
    } catch { /* ignore */ }
    setLoadingHistory(false)
  }, [])

  useEffect(() => {
    if (open && tab === 'write' && textareaRef.current) {
      setTimeout(() => textareaRef.current?.focus(), 200)
    }
    if (open && tab === 'history') {
      fetchHistory()
    }
  }, [open, tab, fetchHistory])

  // Auto-transition to history after successful submit
  useEffect(() => {
    if (sent) {
      const t = setTimeout(() => {
        setSent(false)
        setTab('history')
        fetchHistory()
      }, 2000)
      return () => clearTimeout(t)
    }
  }, [sent, fetchHistory])

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

  const handleOpen = () => {
    setOpen(true)
    setTab('write')
    setSent(false)
  }

  const handleClose = () => {
    if (!sending) setOpen(false)
  }

  const canSend = !!body.trim() && !sending

  // Responsive: desktop = centered modal, mobile = bottom sheet
  const isDesktop = IS_DESKTOP

  const modalStyle: React.CSSProperties = isDesktop
    ? {
        position: 'fixed',
        top: '50%',
        left: '50%',
        transform: 'translate(-50%, -50%)',
        zIndex: 9999,
        backgroundColor: 'var(--surface)',
        borderRadius: '16px',
        boxShadow: '0 8px 40px rgba(0,0,0,0.2)',
        width: '460px',
        maxWidth: '90vw',
        maxHeight: '80vh',
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
        animation: 'fbFadeIn 0.2s ease-out',
      }
    : {
        position: 'fixed',
        bottom: 0,
        left: 0,
        right: 0,
        zIndex: 9999,
        backgroundColor: 'var(--surface)',
        borderTopLeftRadius: '20px',
        borderTopRightRadius: '20px',
        boxShadow: '0 -8px 32px rgba(0,0,0,0.12)',
        maxHeight: '85vh',
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
        paddingBottom: 'max(0px, env(safe-area-inset-bottom, 0px))',
        animation: 'fbSlideUp 0.25s ease-out',
      }

  return (
    <>
      {/* FAB */}
      {!open && (
        <button
          onClick={handleOpen}
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
          onClick={handleClose}
          style={{
            position: 'fixed',
            inset: 0,
            zIndex: 9998,
            backgroundColor: 'rgba(0,0,0,0.4)',
          }}
        />
      )}

      {/* Modal / Bottom Sheet */}
      {open && (
        <div role="dialog" aria-modal="true" aria-label="Product feedback" style={modalStyle}>
          {/* Drag handle (mobile only) */}
          {!isDesktop && (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '10px 0 0' }}>
              <div style={{ width: '36px', height: '4px', borderRadius: '2px', backgroundColor: 'var(--border)' }} />
            </div>
          )}

          {/* Header */}
          <div style={{
            display: 'flex', alignItems: 'center', padding: '12px 16px',
            borderBottom: '1px solid var(--border)',
          }}>
            <span style={{
              flex: 1, fontFamily: FONT, fontSize: FONT_LG, fontWeight: 700,
              color: 'var(--text)',
            }}>
              Help shape Penny
            </span>
            <button
              onClick={handleClose}
              style={{
                background: 'none', border: 'none', cursor: 'pointer',
                fontSize: '20px', color: 'var(--text-secondary)', padding: '0 4px', lineHeight: 1,
              }}
              aria-label="Close"
            >
              &times;
            </button>
          </div>

          {/* Tabs */}
          <div style={{ display: 'flex', borderBottom: '1px solid var(--border)' }}>
            {(['write', 'history'] as const).map(t => (
              <button
                key={t}
                onClick={() => setTab(t)}
                style={{
                  flex: 1,
                  padding: '10px 8px',
                  fontFamily: FONT,
                  fontSize: FONT_SM,
                  fontWeight: 600,
                  cursor: 'pointer',
                  border: 'none',
                  borderBottom: tab === t ? '2px solid var(--brand)' : '2px solid transparent',
                  backgroundColor: tab === t ? 'color-mix(in srgb, var(--brand) 8%, transparent)' : 'transparent',
                  color: tab === t ? 'var(--brand)' : 'var(--text-secondary)',
                  transition: 'all 0.15s',
                }}
              >
                {t === 'write' ? 'New Feedback' : 'History'}
              </button>
            ))}
          </div>

          {/* Tab content */}
          <div style={{ flex: 1, overflowY: 'auto', maxHeight: '60vh' }}>
            {tab === 'write' && (
              sent ? (
                <div style={{ textAlign: 'center', padding: '40px 24px' }}>
                  <div style={{
                    width: '48px', height: '48px', borderRadius: '50%',
                    backgroundColor: 'color-mix(in srgb, var(--brand) 15%, transparent)',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    margin: '0 auto 12px',
                  }}>
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                      <polyline points="20 6 9 17 4 12"/>
                    </svg>
                  </div>
                  <div style={{ fontFamily: FONT, fontSize: FONT_MD, fontWeight: 600, color: 'var(--text)' }}>
                    Got it!
                  </div>
                  <div style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', marginTop: '4px' }}>
                    Your voice shapes what we build next.
                  </div>
                </div>
              ) : (
                <div style={{ padding: '16px' }}>
                  <p style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', margin: '0 0 14px', lineHeight: 1.4 }}>
                    Your feedback goes directly to the product team.
                  </p>

                  {/* Category chips */}
                  <div style={{ display: 'flex', gap: '6px', marginBottom: '14px' }}>
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
                    onKeyDown={e => {
                      if (e.key === 'Enter' && (e.metaKey || e.ctrlKey) && canSend) {
                        e.preventDefault()
                        handleSubmit()
                      }
                    }}
                    placeholder={
                      type === 'bug' ? 'What went wrong?'
                        : type === 'feature' ? 'What would make Penny better?'
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
                      lineHeight: 1.5,
                    }}
                  />

                  {/* Urgency chips */}
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

                  {/* Hint + Send */}
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <span style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-tertiary)' }}>
                      {isDesktop ? 'Cmd+Enter to send' : ''}
                    </span>
                    <button
                      onClick={handleSubmit}
                      disabled={!canSend}
                      style={{
                        padding: '10px 24px',
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
                </div>
              )
            )}

            {tab === 'history' && (
              <div style={{ padding: '8px 16px' }}>
                {loadingHistory ? (
                  <div style={{ textAlign: 'center', padding: '24px', fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text-secondary)' }}>
                    Loading...
                  </div>
                ) : history.length === 0 ? (
                  <div style={{ textAlign: 'center', padding: '32px 16px' }}>
                    <p style={{ fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text-secondary)', margin: '0 0 12px' }}>
                      No feedback yet
                    </p>
                    <button
                      onClick={() => setTab('write')}
                      style={{
                        padding: '8px 20px',
                        backgroundColor: 'var(--brand)',
                        color: '#fff',
                        border: 'none',
                        borderRadius: '10px',
                        fontFamily: FONT,
                        fontSize: FONT_SM,
                        fontWeight: 600,
                        cursor: 'pointer',
                      }}
                    >
                      Submit your first
                    </button>
                  </div>
                ) : (
                  history.map(item => {
                    const statusCfg = STATUS_CONFIG[item.status] || STATUS_CONFIG.new
                    const urgColor = URGENCY_COLORS[item.urgency] || URGENCY_COLORS.normal
                    return (
                      <div
                        key={item.id}
                        style={{
                          padding: '10px 12px',
                          marginBottom: '8px',
                          borderRadius: '10px',
                          backgroundColor: 'var(--bg)',
                          border: '1px solid var(--border)',
                          borderLeft: `3px solid ${urgColor}`,
                        }}
                      >
                        {/* Header: type + status badge */}
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
                          <span style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', textTransform: 'capitalize' }}>
                            {item.type}
                          </span>
                          <span style={{
                            fontFamily: FONT,
                            fontSize: '0.625rem',
                            fontWeight: 600,
                            padding: '2px 8px',
                            borderRadius: '100px',
                            backgroundColor: statusCfg.bg,
                            color: statusCfg.color,
                          }}>
                            {statusCfg.label}
                          </span>
                        </div>

                        {/* Body */}
                        <p style={{
                          fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text)',
                          margin: '0 0 6px', lineHeight: 1.5,
                          // Strip the device context line
                          whiteSpace: 'pre-wrap', wordBreak: 'break-word',
                        }}>
                          {item.body.split('\n\n---\n')[0]}
                        </p>

                        {/* Admin notes */}
                        {item.admin_notes && (
                          <div style={{
                            marginTop: '6px',
                            padding: '6px 8px',
                            borderRadius: '6px',
                            backgroundColor: 'color-mix(in srgb, var(--brand) 6%, transparent)',
                            borderLeft: '2px solid var(--brand)',
                          }}>
                            <span style={{ fontFamily: FONT, fontSize: FONT_2XS, fontWeight: 600, color: 'var(--brand)' }}>
                              Team:{' '}
                            </span>
                            <span style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)' }}>
                              {item.admin_notes}
                            </span>
                          </div>
                        )}

                        {/* Footer: date + github link */}
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '6px' }}>
                          <span style={{ fontFamily: FONT, fontSize: '0.625rem', color: 'var(--text-tertiary)' }}>
                            {formatDate(item.created_at)}
                          </span>
                          {item.github_issue_url && (
                            <a
                              href={item.github_issue_url}
                              target="_blank"
                              rel="noopener noreferrer"
                              style={{
                                fontFamily: FONT, fontSize: '0.625rem',
                                color: 'var(--brand)', textDecoration: 'none',
                              }}
                            >
                              View issue &rarr;
                            </a>
                          )}
                        </div>
                      </div>
                    )
                  })
                )}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Animations */}
      <style>{`
        @keyframes fbSlideUp {
          from { transform: translateY(100%); }
          to { transform: translateY(0); }
        }
        @keyframes fbFadeIn {
          from { opacity: 0; transform: translate(-50%, -50%) scale(0.96); }
          to { opacity: 1; transform: translate(-50%, -50%) scale(1); }
        }
      `}</style>
    </>
  )
}
