import { useState } from 'react'
import { PennyAvatar } from './PennyAvatar'
import { FONT, FONT_XS, FONT_SM, FONT_BASE } from '../styles'

interface SharedItem {
  name: string
  quantity: number
  checked: boolean
}

interface SharedData {
  items: SharedItem[]
  sharer_name: string
  expires_at: string
  invite_code?: string
}

interface SharedListScreenProps {
  sharedData: SharedData | null
  sharedLoading: boolean
  sharedError: string | null
  onSignUp: () => void
  onSignUpClear: () => void
  onJoined?: () => void
}

type JoinStep = 'view' | 'phone' | 'code' | 'joining' | 'done'

export function SharedListScreen({
  sharedData, sharedLoading, sharedError, onSignUp, onSignUpClear, onJoined,
}: SharedListScreenProps) {
  const [step, setStep] = useState<JoinStep>('view')
  const [phone, setPhone] = useState('')
  const [code, setCode] = useState('')
  const [error, setError] = useState('')
  const [sending, setSending] = useState(false)

  const timeLeft = sharedData ? (() => {
    const ms = new Date(sharedData.expires_at).getTime() - Date.now()
    if (ms <= 0) return 'expired'
    const hours = Math.floor(ms / 3600000)
    if (hours > 0) return `${hours} hour${hours !== 1 ? 's' : ''}`
    const mins = Math.floor(ms / 60000)
    return `${mins} minute${mins !== 1 ? 's' : ''}`
  })() : null

  const formatPhone = (raw: string) => {
    const digits = raw.replace(/\D/g, '').slice(0, 10)
    if (digits.length <= 3) return digits
    if (digits.length <= 6) return `(${digits.slice(0, 3)}) ${digits.slice(3)}`
    return `(${digits.slice(0, 3)}) ${digits.slice(3, 6)}-${digits.slice(6)}`
  }

  const handleSendCode = async () => {
    const digits = phone.replace(/\D/g, '')
    if (digits.length !== 10) {
      setError('Enter a 10-digit phone number')
      return
    }
    setError('')
    setSending(true)
    try {
      const res = await fetch('/auth/send-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ phone_number: digits }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        setError(body.error || 'Failed to send code')
        return
      }
      setStep('code')
    } catch {
      setError('Network error — please try again')
    } finally {
      setSending(false)
    }
  }

  const handleVerifyCode = async () => {
    const digits = phone.replace(/\D/g, '')
    if (code.length < 6) {
      setError('Enter the 6-digit code')
      return
    }
    setError('')
    setSending(true)
    try {
      const res = await fetch('/auth/verify-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ phone_number: digits, code }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        setError(body.error || 'Invalid code')
        return
      }
      // Verified — now join the household if invite_code exists
      if (sharedData?.invite_code) {
        setStep('joining')
        const joinRes = await fetch('/api/join', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'include',
          body: JSON.stringify({ code: sharedData.invite_code }),
        })
        if (joinRes.ok) {
          setStep('done')
          setTimeout(() => {
            if (onJoined) onJoined()
            else window.location.href = '/'
          }, 1500)
          return
        }
      }
      // No invite code or join failed — just redirect to app
      window.location.href = '/'
    } catch {
      setError('Network error — please try again')
    } finally {
      setSending(false)
    }
  }

  return (
    <div style={{ minHeight: '100vh', backgroundColor: 'var(--bg)', display: 'flex', flexDirection: 'column' }}>
      {/* Header */}
      <div style={{ backgroundColor: 'var(--surface)', borderBottom: '1px solid var(--border)', padding: '20px', textAlign: 'center' }}>
        <div style={{ display: 'flex', justifyContent: 'center', marginBottom: '12px' }}>
          <PennyAvatar size={48} />
        </div>
        <div style={{ fontFamily: FONT, fontSize: '1.25rem', fontWeight: 700, color: 'var(--text)' }}>
          {sharedData ? (sharedData.sharer_name ? `${sharedData.sharer_name}'s Shopping List` : 'Shopping List') : 'Shopping List'}
        </div>
        <div style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--brand)', fontWeight: 600, letterSpacing: '-0.3px', marginTop: '4px' }}>thriftly</div>
      </div>

      {/* Expiry banner */}
      {sharedData && timeLeft && timeLeft !== 'expired' && (
        <div style={{ backgroundColor: 'var(--warning-bg)', padding: '10px 20px', fontFamily: FONT, fontSize: FONT_XS, color: 'var(--warning)', textAlign: 'center' }}>
          This link expires in {timeLeft}
        </div>
      )}

      {/* Content */}
      <div style={{ flex: 1, padding: '16px 20px', overflowY: 'auto' }}>
        {sharedLoading && (
          <div style={{ textAlign: 'center', padding: '40px 0' }}>
            <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text-secondary)' }}>Loading list...</p>
          </div>
        )}

        {sharedError === 'expired' && (
          <div style={{ textAlign: 'center', padding: '40px 20px' }}>
            <div style={{ fontSize: '48px', marginBottom: '16px' }}>⏰</div>
            <h2 style={{ fontFamily: FONT, fontSize: '1.25rem', color: 'var(--text)', margin: '0 0 8px' }}>This link has expired</h2>
            <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text-secondary)', margin: '0 0 24px' }}>
              Share links are valid for 48 hours. Ask the sender to share again.
            </p>
            <button
              onClick={onSignUp}
              style={{ backgroundColor: 'var(--brand)', color: '#fff', border: 'none', borderRadius: '10px', padding: '14px 28px', fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, cursor: 'pointer' }}
            >
              Sign up for Thriftly
            </button>
          </div>
        )}

        {sharedError === 'not_found' && (
          <div style={{ textAlign: 'center', padding: '40px 20px' }}>
            <div style={{ fontSize: '48px', marginBottom: '16px' }}>🔗</div>
            <h2 style={{ fontFamily: FONT, fontSize: '1.25rem', color: 'var(--text)', margin: '0 0 8px' }}>Link not found</h2>
            <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text-secondary)', margin: '0 0 24px' }}>
              This share link doesn't exist. Check the URL and try again.
            </p>
            <button
              onClick={onSignUp}
              style={{ backgroundColor: 'var(--brand)', color: '#fff', border: 'none', borderRadius: '10px', padding: '14px 28px', fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, cursor: 'pointer' }}
            >
              Sign up for Thriftly
            </button>
          </div>
        )}

        {sharedData && !sharedError && (
          <>
            {sharedData.items.length === 0 ? (
              <div style={{ textAlign: 'center', padding: '40px 20px' }}>
                <div style={{ fontSize: '48px', marginBottom: '16px' }}>📝</div>
                <h2 style={{ fontFamily: FONT, fontSize: '1.125rem', color: 'var(--text)', margin: '0 0 8px' }}>
                  Nothing on this list yet
                </h2>
                <p style={{ fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text-secondary)', margin: 0, lineHeight: '1.5' }}>
                  {sharedData.sharer_name || 'The owner'} hasn't added any items yet. Check back soon!
                </p>
              </div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                {sharedData.items.map((item, i) => (
                  <div key={i} style={{
                    display: 'flex', alignItems: 'center', gap: '12px',
                    padding: '14px 16px', backgroundColor: 'var(--surface)', borderRadius: '10px',
                    opacity: item.checked ? 0.5 : 1,
                  }}>
                    <div style={{
                      width: '22px', height: '22px', borderRadius: '6px', flexShrink: 0,
                      border: item.checked ? 'none' : '2px solid var(--border)',
                      backgroundColor: item.checked ? 'var(--brand)' : 'transparent',
                      display: 'flex', alignItems: 'center', justifyContent: 'center',
                    }}>
                      {item.checked && <span style={{ color: '#fff', fontSize: '14px', fontWeight: 700 }}>✓</span>}
                    </div>
                    <span style={{
                      fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text)', flex: 1,
                      textDecoration: item.checked ? 'line-through' : 'none',
                    }}>
                      {item.quantity > 1 ? `${item.quantity}x ` : ''}{item.name}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </div>

      {/* Join CTA — inline phone auth flow */}
      <div style={{ padding: '20px', backgroundColor: 'var(--surface)', borderTop: '1px solid var(--border)', textAlign: 'center' }}>
        {step === 'done' ? (
          <div style={{ padding: '8px 0' }}>
            <div style={{ fontSize: '24px', color: 'var(--brand)', marginBottom: '4px' }}>&#10003;</div>
            <p style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, color: 'var(--text)', margin: 0 }}>
              You're in! Loading your shared list...
            </p>
          </div>
        ) : step === 'joining' ? (
          <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text-secondary)', margin: 0 }}>
            Joining {sharedData?.sharer_name ? `${sharedData.sharer_name}'s` : 'the'} list...
          </p>
        ) : step === 'code' ? (
          <div>
            <p style={{ fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text-secondary)', margin: '0 0 10px' }}>
              Enter the 6-digit code sent to your phone
            </p>
            {error && <p style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--error)', margin: '0 0 8px' }}>{error}</p>}
            <div style={{ display: 'flex', gap: '8px', justifyContent: 'center', maxWidth: '320px', margin: '0 auto' }}>
              <input
                type="tel"
                inputMode="numeric"
                autoComplete="one-time-code"
                value={code}
                onChange={e => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                placeholder="000000"
                style={{
                  flex: 1, padding: '12px 14px', fontFamily: FONT, fontSize: FONT_BASE,
                  color: 'var(--text)', backgroundColor: 'var(--bg)',
                  border: '1.5px solid var(--border)', borderRadius: '10px',
                  outline: 'none', textAlign: 'center', letterSpacing: '4px',
                }}
                onKeyDown={e => { if (e.key === 'Enter') handleVerifyCode() }}
                autoFocus
              />
              <button
                onClick={handleVerifyCode}
                disabled={sending || code.length < 6}
                style={{
                  backgroundColor: 'var(--brand)', color: '#fff', border: 'none',
                  borderRadius: '10px', padding: '12px 20px', fontFamily: FONT,
                  fontSize: FONT_BASE, fontWeight: 600, cursor: 'pointer',
                  opacity: (sending || code.length < 6) ? 0.5 : 1,
                }}
              >
                {sending ? '...' : 'Verify'}
              </button>
            </div>
            <button
              onClick={() => { setStep('phone'); setCode(''); setError('') }}
              style={{ background: 'none', border: 'none', fontFamily: FONT, fontSize: FONT_XS, color: 'var(--brand)', cursor: 'pointer', marginTop: '10px' }}
            >
              Change number
            </button>
          </div>
        ) : step === 'phone' ? (
          <div>
            <p style={{ fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text-secondary)', margin: '0 0 10px' }}>
              Enter your phone number to {sharedData?.invite_code ? 'join this list' : 'get started'}
            </p>
            {error && <p style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--error)', margin: '0 0 8px' }}>{error}</p>}
            <div style={{ display: 'flex', gap: '8px', justifyContent: 'center', maxWidth: '320px', margin: '0 auto' }}>
              <input
                type="tel"
                inputMode="tel"
                autoComplete="tel"
                value={formatPhone(phone)}
                onChange={e => setPhone(e.target.value)}
                placeholder="(555) 555-5555"
                style={{
                  flex: 1, padding: '12px 14px', fontFamily: FONT, fontSize: FONT_BASE,
                  color: 'var(--text)', backgroundColor: 'var(--bg)',
                  border: '1.5px solid var(--border)', borderRadius: '10px',
                  outline: 'none',
                }}
                onKeyDown={e => { if (e.key === 'Enter') handleSendCode() }}
                autoFocus
              />
              <button
                onClick={handleSendCode}
                disabled={sending || phone.replace(/\D/g, '').length !== 10}
                style={{
                  backgroundColor: 'var(--brand)', color: '#fff', border: 'none',
                  borderRadius: '10px', padding: '12px 20px', fontFamily: FONT,
                  fontSize: FONT_BASE, fontWeight: 600, cursor: 'pointer',
                  opacity: (sending || phone.replace(/\D/g, '').length !== 10) ? 0.5 : 1,
                }}
              >
                {sending ? '...' : 'Send Code'}
              </button>
            </div>
            <button
              onClick={() => { setStep('view'); setError('') }}
              style={{ background: 'none', border: 'none', fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', cursor: 'pointer', marginTop: '10px' }}
            >
              Cancel
            </button>
          </div>
        ) : (
          <>
            <p style={{ fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text-secondary)', margin: '0 0 12px' }}>
              {sharedData?.invite_code
                ? `Join ${sharedData?.sharer_name ? sharedData.sharer_name + "'s" : 'this'} grocery list — enter your phone to get started`
                : 'Sign up for Thriftly to make your own grocery list'}
            </p>
            <button
              onClick={() => sharedData?.invite_code ? setStep('phone') : onSignUpClear()}
              style={{ backgroundColor: 'var(--brand)', color: '#fff', border: 'none', borderRadius: '10px', padding: '14px 28px', fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, cursor: 'pointer', width: '100%', maxWidth: '320px' }}
            >
              {sharedData?.invite_code ? 'Join This List' : 'Get Started'}
            </button>
          </>
        )}
      </div>
    </div>
  )
}
