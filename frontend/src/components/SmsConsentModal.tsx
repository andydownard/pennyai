import { useState } from 'react'
import { fetchAPI } from '../api'
import { FONT, FONT_SM, FONT_BASE, FONT_MD, FONT_2XS } from '../styles'
import { PennyAvatar } from './PennyAvatar'

interface SmsConsentModalProps {
  /** Called when the user has resolved the modal (consented or declined). */
  onResolved: (optedIn: boolean) => void
}

/**
 * One-time modal shown to existing pre-rebrand users so they can re-consent
 * to SMS messaging under the new Groflo brand.
 *
 * Twilio A2P 10DLC vetting requires a provable consent record per user. Users
 * who signed up while the brand was "Thriftly" have NULL sms_consent_at; this
 * modal captures fresh consent under "Groflo" with timestamp + IP + UA.
 *
 * - "Keep texting Penny" → POST /api/me/sms-consent { opted_in: true }
 * - "Not now"            → POST /api/me/sms-consent { opted_in: false }
 *
 * Either resolution removes the modal for this session. If the user picks
 * "Not now" the server-side canSendSMS() gate will skip outbound SMS to them
 * until they consent.
 */
export function SmsConsentModal({ onResolved }: SmsConsentModalProps) {
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const submit = async (optedIn: boolean) => {
    if (submitting) return
    setSubmitting(true)
    setError('')
    try {
      const res = await fetchAPI('/api/me/sms-consent', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ opted_in: optedIn }),
      })
      if (!res) {
        setError('Network error. Please try again.')
        setSubmitting(false)
        return
      }
      onResolved(optedIn)
    } catch {
      setError('Network error. Please try again.')
      setSubmitting(false)
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="sms-consent-title"
      style={{
        position: 'fixed', inset: 0, zIndex: 9500,
        backgroundColor: 'rgba(0,0,0,0.55)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        padding: '20px',
      }}
    >
      <div style={{
        backgroundColor: 'var(--surface)', borderRadius: '20px',
        maxWidth: '420px', width: '100%', padding: '28px 24px 24px',
        boxShadow: '0 20px 60px rgba(0,0,0,0.25)',
      }}>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', marginBottom: '16px' }}>
          <PennyAvatar size={64} />
          <h2 id="sms-consent-title" style={{
            fontFamily: FONT, fontSize: FONT_MD, fontWeight: 700,
            color: 'var(--text)', margin: '12px 0 4px', textAlign: 'center',
          }}>
            Penny is now part of Groflo
          </h2>
          <p style={{
            fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text-secondary)',
            margin: 0, textAlign: 'center', lineHeight: 1.5,
          }}>
            Same Penny, same list. We're updating our SMS terms — please confirm you want to keep texting with her.
          </p>
        </div>

        <div style={{
          backgroundColor: 'var(--bg)', borderRadius: '12px',
          padding: '14px 16px', marginBottom: '16px',
          fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)',
          lineHeight: 1.55,
        }}>
          You'll get list updates from your household, replies from Penny, and the occasional reminder. <strong>Message frequency varies.</strong> Message and data rates may apply. Reply STOP to opt out, HELP for help. See our <a href="/privacy" target="_blank" rel="noopener noreferrer" style={{ color: 'var(--brand)' }}>Privacy Policy</a>.
        </div>

        {error && (
          <p role="alert" style={{
            fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--error)',
            margin: '0 0 12px', textAlign: 'center',
          }}>{error}</p>
        )}

        <button
          onClick={() => submit(true)}
          disabled={submitting}
          style={{
            width: '100%', padding: '14px',
            fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600,
            color: '#fff', backgroundColor: 'var(--brand)',
            border: 'none', borderRadius: '12px',
            cursor: submitting ? 'default' : 'pointer',
            opacity: submitting ? 0.6 : 1, marginBottom: '8px',
          }}
        >
          {submitting ? 'Saving…' : 'Keep texting Penny'}
        </button>
        <button
          onClick={() => submit(false)}
          disabled={submitting}
          style={{
            width: '100%', padding: '12px',
            fontFamily: FONT, fontSize: FONT_SM, fontWeight: 500,
            color: 'var(--text-secondary)', backgroundColor: 'transparent',
            border: 'none', cursor: submitting ? 'default' : 'pointer',
          }}
        >
          Not now
        </button>
      </div>
    </div>
  )
}
