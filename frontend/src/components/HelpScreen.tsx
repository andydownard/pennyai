import { useState } from 'react'
import { ScreenTransition } from './ScreenTransition'
import {
  BRAND, TEXT, TEXT_SECONDARY, BORDER, WHITE, FONT,
  pageStyles,
} from '../styles'

interface HelpScreenProps {
  showToast: (msg: string, type?: 'success' | 'error') => void
  onSessionExpired: () => void
  renderHeader: () => React.ReactNode
  renderBottomTabs: () => React.ReactNode
  transitionStyle: React.CSSProperties
}

const faqs = [
  { q: 'How do I share my list with someone?', a: 'Go to Share & Earn from the menu. Tap "Invite to Household" to generate an invite link. Anyone who opens it can join your shared list.' },
  { q: 'How do Penny\'s swaps work?', a: 'When you add an item, Penny may suggest a swap from a brand partner. If you accept, the brand covers part of the cost. You can always decline — no pressure!' },
  { q: 'How do I add items via text message?', a: 'Text your grocery items to Penny\'s number (you can find it in the Penny settings screen). She\'ll parse your message and add them to your list automatically.' },
  { q: 'How do I save a list for later?', a: 'Just ask Penny! Say something like "save this as my weekly groceries" in the Penny chat. You can load saved lists later by asking Penny "what lists do I have?"' },
  { q: 'How do I delete my account?', a: 'Email us at support@thrift.ly and we\'ll take care of it within 48 hours. We\'ll delete all your data permanently.' },
]

export function HelpScreen({
  showToast, onSessionExpired,
  renderHeader, renderBottomTabs, transitionStyle,
}: HelpScreenProps) {
  const [openFaq, setOpenFaq] = useState<number | null>(null)
  const [bugDescription, setBugDescription] = useState('')
  const [bugSubmitting, setBugSubmitting] = useState(false)
  const [bugSubmitted, setBugSubmitted] = useState(false)

  const handleBugSubmit = async () => {
    if (!bugDescription.trim() || bugSubmitting) return
    setBugSubmitting(true)
    try {
      const res = await fetch('/api/bug-report', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ description: bugDescription.trim() }),
      })
      if (res.status === 401) { onSessionExpired(); return }
      if (res.ok) {
        setBugSubmitted(true)
        setBugDescription('')
        showToast('Bug report submitted — thank you!')
      } else {
        const data = await res.json().catch(() => ({}))
        showToast(data?.error || 'Failed to submit report', 'error')
      }
    } catch {
      showToast('Something went wrong', 'error')
    } finally {
      setBugSubmitting(false)
    }
  }

  return (
    <ScreenTransition style={transitionStyle}>
    <div style={pageStyles.page}>
      {renderHeader()}
      <div style={pageStyles.listArea}>
        <div style={{ padding: '24px 20px', display: 'flex', flexDirection: 'column', gap: '24px' }}>

          {/* FAQ Section */}
          <div>
            <h2 style={{ fontFamily: FONT, fontSize: '18px', fontWeight: 700, color: TEXT, marginBottom: '16px' }}>
              Frequently Asked Questions
            </h2>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0px', borderRadius: '12px', overflow: 'hidden', border: `1px solid ${BORDER}` }}>
              {faqs.map((faq, i) => (
                <div key={i}>
                  {i > 0 && <div style={{ height: '1px', backgroundColor: BORDER }} />}
                  <button
                    onClick={() => setOpenFaq(openFaq === i ? null : i)}
                    style={{
                      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                      width: '100%', padding: '14px 16px',
                      backgroundColor: openFaq === i ? `${BRAND}08` : WHITE,
                      border: 'none', cursor: 'pointer', textAlign: 'left',
                      fontFamily: FONT, fontSize: '15px', fontWeight: 500, color: TEXT,
                      gap: '12px',
                    }}
                  >
                    <span style={{ flex: 1 }}>{faq.q}</span>
                    <span style={{
                      fontSize: '18px', color: TEXT_SECONDARY, flexShrink: 0,
                      transform: openFaq === i ? 'rotate(180deg)' : 'none',
                      transition: 'transform 0.2s ease',
                    }}>
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={TEXT_SECONDARY} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <polyline points="6 9 12 15 18 9"/>
                      </svg>
                    </span>
                  </button>
                  <div style={{
                    maxHeight: openFaq === i ? '200px' : '0px',
                    opacity: openFaq === i ? 1 : 0,
                    overflow: 'hidden',
                    transition: 'max-height 0.3s ease, opacity 0.25s ease',
                  }}>
                    <div style={{
                      padding: '0 16px 14px 16px',
                      fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, lineHeight: 1.5,
                      backgroundColor: openFaq === i ? `${BRAND}08` : WHITE,
                    }}>
                      {faq.a}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Divider */}
          <div style={pageStyles.divider} />

          {/* Bug Report Section */}
          <div>
            <h2 style={{ fontFamily: FONT, fontSize: '18px', fontWeight: 700, color: TEXT, marginBottom: '4px' }}>
              Report a Bug
            </h2>
            <p style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, marginBottom: '16px' }}>
              Found something that doesn't look right? Let us know and we'll fix it.
            </p>
            {bugSubmitted ? (
              <div style={{
                padding: '20px', borderRadius: '12px',
                backgroundColor: `${BRAND}10`, textAlign: 'center',
              }}>
                <p style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 500, color: BRAND }}>
                  Thanks for your report! We'll look into it.
                </p>
                <button
                  onClick={() => setBugSubmitted(false)}
                  style={{
                    marginTop: '12px', background: 'none', border: 'none', cursor: 'pointer',
                    fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY, textDecoration: 'underline',
                  }}
                >
                  Submit another report
                </button>
              </div>
            ) : (
              <>
                <textarea
                  value={bugDescription}
                  onChange={e => setBugDescription(e.target.value)}
                  placeholder="Describe the issue you encountered..."
                  maxLength={2000}
                  rows={5}
                  style={{
                    width: '100%', padding: '13px 16px',
                    fontSize: '16px', fontFamily: FONT, color: TEXT,
                    backgroundColor: WHITE, border: `1.5px solid ${BORDER}`,
                    borderRadius: '12px', outline: 'none',
                    boxSizing: 'border-box', resize: 'vertical',
                    lineHeight: 1.5,
                  }}
                  onFocus={e => { e.target.style.borderColor = BRAND }}
                  onBlur={e => { e.target.style.borderColor = BORDER }}
                />
                <button
                  onClick={handleBugSubmit}
                  disabled={!bugDescription.trim() || bugSubmitting}
                  style={{
                    marginTop: '12px', width: '100%', padding: '14px',
                    backgroundColor: (!bugDescription.trim() || bugSubmitting) ? `${BRAND}60` : BRAND,
                    color: WHITE, border: 'none', borderRadius: '12px',
                    fontFamily: FONT, fontSize: '16px', fontWeight: 600,
                    cursor: (!bugDescription.trim() || bugSubmitting) ? 'not-allowed' : 'pointer',
                    transition: 'background-color 0.2s',
                  }}
                >
                  {bugSubmitting ? 'Submitting...' : 'Submit Report'}
                </button>
              </>
            )}
          </div>

        </div>
      </div>
      {renderBottomTabs()}
    </div>
    </ScreenTransition>
  )
}
