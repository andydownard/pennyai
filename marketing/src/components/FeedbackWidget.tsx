import { useState, useRef, useEffect } from 'react'

const ALLOWED_PHONES = ['3104861833', '6264043580']
const API_BASE = '/.netlify/functions/feedback'

function normalizePhone(input: string): string {
  return input.replace(/\D/g, '').replace(/^1(\d{10})$/, '$1')
}

interface FeedbackItem {
  id: number
  body: string
  urgency: string
  status: string
  admin_notes: string
  created_at: string
}

const STATUS_LABELS: Record<string, string> = {
  new: 'Submitted',
  reviewed: 'Under Review',
  roadmapped: 'In Progress',
  done: 'Shipped',
  dismissed: 'Dismissed',
}

function formatDate(dateStr: string): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

export default function FeedbackWidget() {
  const [open, setOpen] = useState(false)
  const [phone, setPhone] = useState(() => localStorage.getItem('fb_phone') || '')
  const [authorized, setAuthorized] = useState(() => {
    const stored = localStorage.getItem('fb_phone')
    return stored ? ALLOWED_PHONES.includes(normalizePhone(stored)) : false
  })
  const [body, setBody] = useState('')
  const [isInsist, setIsInsist] = useState(true)
  const [sending, setSending] = useState(false)
  const [sent, setSent] = useState(false)
  const [error, setError] = useState('')
  const [showHistory, setShowHistory] = useState(false)
  const [history, setHistory] = useState<FeedbackItem[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const phoneRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (open && authorized && !showHistory && textareaRef.current) {
      setTimeout(() => textareaRef.current?.focus(), 200)
    }
    if (open && !authorized && phoneRef.current) {
      setTimeout(() => phoneRef.current?.focus(), 200)
    }
  }, [open, authorized, showHistory])

  useEffect(() => {
    if (sent) {
      const t = setTimeout(() => {
        setSent(false)
        setOpen(false)
      }, 2500)
      return () => clearTimeout(t)
    }
  }, [sent])

  const handlePhoneSubmit = () => {
    const normalized = normalizePhone(phone)
    if (ALLOWED_PHONES.includes(normalized)) {
      localStorage.setItem('fb_phone', normalized)
      setAuthorized(true)
      setError('')
    } else {
      setError("That number isn't authorized for feedback.")
    }
  }

  const handleSubmit = async () => {
    const text = body.trim()
    if (!text || sending) return
    setError('')
    setSending(true)
    try {
      const res = await fetch(API_BASE, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          phone: localStorage.getItem('fb_phone') || '',
          body: text,
          urgency: isInsist ? 'critical' : 'normal',
        }),
      })
      const data = await res.json()
      if (data.success) {
        setBody('')
        setIsInsist(true)
        setSent(true)
      } else {
        setError(data.error || 'Something went wrong.')
      }
    } catch {
      setError('Network error. Try again.')
    } finally {
      setSending(false)
    }
  }

  const loadHistory = async () => {
    setShowHistory(true)
    setHistoryLoading(true)
    try {
      const storedPhone = localStorage.getItem('fb_phone') || ''
      const res = await fetch(`${API_BASE}?phone=${encodeURIComponent(storedPhone)}`)
      const items = await res.json()
      setHistory(Array.isArray(items) ? items : [])
    } catch {
      setHistory([])
    } finally {
      setHistoryLoading(false)
    }
  }

  const canSend = body.trim().length > 0 && !sending
  const isDesktop = typeof window !== 'undefined' && window.innerWidth >= 640

  return (
    <>
      {!open && (
        <button
          onClick={() => setOpen(true)}
          aria-label="Product feedback"
          title="Product feedback"
          className="fb-fab"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M9 18h6"/>
            <path d="M10 22h4"/>
            <path d="M12 2a7 7 0 0 0-4 12.7V17h8v-2.3A7 7 0 0 0 12 2z"/>
          </svg>
        </button>
      )}

      {open && (
        <div className="fb-backdrop" onClick={() => { if (!sending) setOpen(false) }} />
      )}

      {open && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Product feedback"
          className={isDesktop ? 'fb-modal' : 'fb-sheet'}
        >
          {/* Header */}
          <div className="fb-header">
            <button
              onClick={() => { if (!sending) setOpen(false) }}
              aria-label="Close"
              className="fb-close"
            >
              &times;
            </button>
            <span className="fb-title">
              {!authorized ? 'Verify Your Number' : showHistory ? 'My Submissions' : sent ? 'Sent' : 'Product Feedback'}
            </span>
            {authorized && !sent && !showHistory && (
              <button
                onClick={loadHistory}
                aria-label="View history"
                title="Past submissions"
                className="fb-close"
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="10"/>
                  <polyline points="12 6 12 12 16 14"/>
                </svg>
              </button>
            )}
            {showHistory && (
              <button
                onClick={() => setShowHistory(false)}
                className="fb-history-new"
              >
                New
              </button>
            )}
            {authorized && !sent && !showHistory && (
              <button
                onClick={handleSubmit}
                disabled={!canSend}
                className="fb-send"
                style={{
                  backgroundColor: canSend ? 'var(--green-600)' : 'var(--gray-300)',
                  cursor: canSend ? 'pointer' : 'not-allowed',
                }}
              >
                {sending ? '...' : 'Send'}
              </button>
            )}
          </div>

          {/* Content */}
          {!authorized ? (
            <div className="fb-phone-gate">
              <p>Enter your phone number to submit feedback.</p>
              <input
                ref={phoneRef}
                type="tel"
                value={phone}
                onChange={e => { setPhone(e.target.value); setError('') }}
                onKeyDown={e => { if (e.key === 'Enter') handlePhoneSubmit() }}
                placeholder="(310) 486-1833"
                className="fb-phone-input"
              />
              {error && <div className="fb-error">{error}</div>}
              <button onClick={handlePhoneSubmit} className="fb-verify-btn">
                Verify
              </button>
            </div>
          ) : showHistory ? (
            <div className="fb-history">
              {historyLoading ? (
                <div className="fb-history-empty">Loading...</div>
              ) : history.length === 0 ? (
                <div className="fb-history-empty">No submissions yet.</div>
              ) : (
                history.map(item => (
                  <div key={item.id} className="fb-history-item">
                    <div className="fb-history-body">{item.body}</div>
                    <div className="fb-history-meta">
                      {item.urgency === 'critical' && (
                        <span className="fb-badge fb-badge-urgent">I insist</span>
                      )}
                      <span className={`fb-badge fb-badge-${item.status}`}>
                        {STATUS_LABELS[item.status] || item.status}
                      </span>
                      <span className="fb-history-date">{formatDate(item.created_at)}</span>
                    </div>
                    {item.admin_notes && (
                      <div className="fb-history-notes">
                        <strong>Product team:</strong> {item.admin_notes}
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>
          ) : sent ? (
            <div className="fb-sent">
              <div className="fb-sent-icon">
                <svg width="28" height="28" viewBox="0 0 24 24" fill="#059669">
                  <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/>
                </svg>
              </div>
              <div className="fb-sent-title">
                {isInsist ? 'Escalated to the CEO' : 'Submitted to the product team'}
              </div>
              <div className="fb-sent-sub">
                Your voice shapes what we build next.
              </div>
            </div>
          ) : (
            <div className="fb-body">
              {/* Priority toggle */}
              <div className="fb-toggle-row">
                <div
                  onClick={() => setIsInsist(!isInsist)}
                  className="fb-toggle"
                  style={{ backgroundColor: isInsist ? 'var(--green-600)' : 'var(--gray-300)' }}
                >
                  <div className="fb-toggle-knob" style={{ left: isInsist ? '18px' : '2px' }} />
                </div>
                <span
                  onClick={() => setIsInsist(!isInsist)}
                  className="fb-toggle-label"
                  style={{ color: isInsist ? 'var(--green-600)' : 'var(--gray-500)' }}
                >
                  {isInsist ? 'I insist' : 'Normal'}
                </span>
              </div>

              {/* Textarea */}
              <div className="fb-textarea-wrap">
                <textarea
                  ref={textareaRef}
                  value={body}
                  onChange={e => { setBody(e.target.value); setError('') }}
                  onKeyDown={e => {
                    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                      e.preventDefault()
                      handleSubmit()
                    }
                  }}
                  placeholder="What's on your mind? Bug, feature idea, or anything else..."
                  maxLength={10000}
                  className="fb-textarea"
                />
                {error && <div className="fb-error">{error}</div>}
              </div>
            </div>
          )}
        </div>
      )}

      <style>{`
        .fb-fab {
          position: fixed;
          top: 80px;
          right: 16px;
          z-index: 50;
          width: 36px;
          height: 36px;
          display: flex;
          align-items: center;
          justify-content: center;
          background-color: var(--green-600);
          color: var(--white);
          border: none;
          border-radius: 50%;
          cursor: pointer;
          box-shadow: 0 2px 8px rgba(0,0,0,0.15);
          opacity: 0.8;
          transition: opacity 0.15s, transform 0.15s;
        }
        .fb-fab:hover {
          opacity: 1;
          transform: scale(1.08);
        }
        .fb-backdrop {
          position: fixed;
          inset: 0;
          z-index: 9998;
          background-color: rgba(0,0,0,0.4);
        }
        .fb-modal {
          position: fixed;
          top: 50%;
          left: 50%;
          transform: translate(-50%, -50%);
          z-index: 9999;
          background: var(--white);
          border-radius: 16px;
          box-shadow: 0 8px 40px rgba(0,0,0,0.18);
          width: 480px;
          max-width: calc(100vw - 48px);
          max-height: 80vh;
          display: flex;
          flex-direction: column;
          overflow: hidden;
          animation: fbFadeIn 0.2s ease-out;
        }
        .fb-sheet {
          position: fixed;
          bottom: 0;
          left: 0;
          right: 0;
          z-index: 9999;
          background: var(--white);
          border-top-left-radius: 20px;
          border-top-right-radius: 20px;
          box-shadow: 0 -8px 32px rgba(0,0,0,0.12);
          max-height: 85vh;
          display: flex;
          flex-direction: column;
          overflow: hidden;
          padding-bottom: max(16px, env(safe-area-inset-bottom, 0px));
          animation: fbSlideUp 0.25s ease-out;
        }
        .fb-header {
          display: flex;
          align-items: center;
          gap: 8px;
          padding: 14px 16px;
          border-bottom: 1px solid var(--border);
          flex-shrink: 0;
        }
        .fb-close {
          border: none;
          background: none;
          cursor: pointer;
          font-size: 22px;
          color: var(--gray-500);
          padding: 0 4px;
          line-height: 1;
          flex-shrink: 0;
        }
        .fb-title {
          flex: 1;
          font-family: var(--font);
          font-size: 16px;
          font-weight: 600;
          color: var(--gray-800);
          text-align: center;
        }
        .fb-send {
          color: var(--white);
          border: none;
          border-radius: 8px;
          padding: 6px 16px;
          font-family: var(--font);
          font-size: 14px;
          font-weight: 600;
          flex-shrink: 0;
          transition: background 0.12s;
        }
        .fb-phone-gate {
          padding: 32px 24px;
          text-align: center;
        }
        .fb-phone-gate p {
          font-size: 15px;
          color: var(--gray-500);
          margin-bottom: 16px;
        }
        .fb-phone-input {
          width: 100%;
          padding: 12px;
          font-family: var(--font);
          font-size: 16px;
          border: 1.5px solid var(--border);
          border-radius: 12px;
          outline: none;
          text-align: center;
          margin-bottom: 12px;
          box-sizing: border-box;
        }
        .fb-phone-input:focus {
          border-color: var(--green-600);
        }
        .fb-verify-btn {
          background: var(--green-600);
          color: var(--white);
          border: none;
          border-radius: 10px;
          padding: 10px 32px;
          font-family: var(--font);
          font-size: 15px;
          font-weight: 600;
          cursor: pointer;
          transition: background 0.15s;
        }
        .fb-verify-btn:hover {
          background: var(--green-700);
        }
        .fb-error {
          color: #dc2626;
          font-size: 13px;
          margin-top: 6px;
          margin-bottom: 6px;
        }
        .fb-sent {
          text-align: center;
          padding: 48px 24px;
          display: flex;
          flex-direction: column;
          align-items: center;
        }
        .fb-sent-icon {
          width: 56px;
          height: 56px;
          border-radius: 50%;
          background: var(--green-50);
          display: flex;
          align-items: center;
          justify-content: center;
          margin-bottom: 16px;
        }
        .fb-sent-title {
          font-family: var(--font);
          font-size: 16px;
          font-weight: 600;
          color: var(--gray-800);
        }
        .fb-sent-sub {
          font-family: var(--font);
          font-size: 13px;
          color: var(--gray-500);
          margin-top: 6px;
        }
        .fb-body {
          display: flex;
          flex-direction: column;
          flex: 1;
          overflow: hidden;
        }
        .fb-toggle-row {
          display: flex;
          align-items: center;
          gap: 10px;
          padding: 10px 16px;
          border-bottom: 1px solid var(--border);
          flex-shrink: 0;
        }
        .fb-toggle {
          width: 36px;
          height: 20px;
          border-radius: 10px;
          position: relative;
          cursor: pointer;
          transition: background-color 0.2s;
          flex-shrink: 0;
        }
        .fb-toggle-knob {
          width: 16px;
          height: 16px;
          border-radius: 50%;
          background: var(--white);
          position: absolute;
          top: 2px;
          transition: left 0.2s;
          box-shadow: 0 1px 3px rgba(0,0,0,0.2);
        }
        .fb-toggle-label {
          font-family: var(--font);
          font-size: 13px;
          font-weight: 600;
          cursor: pointer;
          user-select: none;
        }
        .fb-textarea-wrap {
          flex: 1;
          padding: 12px 16px;
          display: flex;
          flex-direction: column;
        }
        .fb-textarea {
          flex: 1;
          width: 100%;
          padding: 12px;
          font-family: var(--font);
          font-size: 16px;
          color: var(--gray-800);
          background: var(--white);
          border: 1.5px solid var(--border);
          border-radius: 12px;
          outline: none;
          resize: none;
          min-height: 120px;
          box-sizing: border-box;
          line-height: 1.5;
          transition: border-color 0.15s;
        }
        .fb-textarea:focus {
          border-color: var(--green-600);
        }
        .fb-history-new {
          border: none;
          background: none;
          cursor: pointer;
          color: var(--green-600);
          font-family: var(--font);
          font-size: 13px;
          font-weight: 600;
          flex-shrink: 0;
          padding: 0 4px;
        }
        .fb-history {
          flex: 1;
          overflow-y: auto;
          padding: 12px 16px;
        }
        .fb-history-empty {
          text-align: center;
          color: var(--gray-500);
          padding: 32px;
          font-size: 15px;
        }
        .fb-history-item {
          border: 1px solid var(--border);
          border-radius: 12px;
          padding: 14px;
          margin-bottom: 12px;
        }
        .fb-history-body {
          font-size: 15px;
          color: var(--gray-800);
          white-space: pre-wrap;
          word-break: break-word;
          line-height: 1.5;
        }
        .fb-history-meta {
          display: flex;
          gap: 8px;
          flex-wrap: wrap;
          align-items: center;
          margin-top: 8px;
          font-size: 12px;
        }
        .fb-badge {
          padding: 2px 8px;
          border-radius: 8px;
          font-size: 11px;
          font-weight: 600;
        }
        .fb-badge-urgent {
          color: var(--white);
          background: #dc2626;
        }
        .fb-badge-new {
          color: var(--green-700);
          background: var(--green-50);
        }
        .fb-badge-reviewed {
          color: #92400e;
          background: #fef3c7;
        }
        .fb-badge-roadmapped, .fb-badge-done {
          color: #065f46;
          background: #d1fae5;
        }
        .fb-badge-dismissed {
          color: var(--gray-600);
          background: var(--gray-100);
        }
        .fb-history-date {
          color: var(--gray-500);
        }
        .fb-history-notes {
          margin-top: 8px;
          padding: 8px 10px;
          background: var(--cream);
          border-radius: 8px;
          font-size: 13px;
          color: var(--gray-800);
          line-height: 1.4;
        }
        @keyframes fbSlideUp {
          from { transform: translateY(100%); }
          to { transform: translateY(0); }
        }
        @keyframes fbFadeIn {
          from { opacity: 0; transform: translate(-50%, -48%); }
          to { opacity: 1; transform: translate(-50%, -50%); }
        }
      `}</style>
    </>
  )
}
