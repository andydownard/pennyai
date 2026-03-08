import { PennyAvatar } from './PennyAvatar'
import { FloatingLabelField } from './FloatingLabelField'
import { LegalLinks } from './LegalLinks'
import { FONT, formatPhone, authStyles, pageStyles } from '../styles'

interface PhoneScreenProps {
  phone: string
  smsOptIn: boolean
  loading: boolean
  error: string
  loginPennyText: string
  googleClientId: string
  phoneInputRef: React.Ref<HTMLInputElement>
  onPhoneChange: (e: React.ChangeEvent<HTMLInputElement>) => void
  onSmsOptInChange: (v: boolean) => void
  onSendCode: () => void
  onNavigate: (screen: string) => void
  setError: (e: string) => void
}

export function PhoneScreen({
  phone, smsOptIn, loading, error, loginPennyText, googleClientId,
  phoneInputRef, onPhoneChange, onSmsOptInChange, onSendCode, onNavigate, setError,
}: PhoneScreenProps) {
  const isValid = phone.length === 10

  return (
    <div style={authStyles.page}>
      <div style={authStyles.innerFlex}>

        {/* Penny hero */}
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', marginBottom: '28px', width: '100%' }}>
          <PennyAvatar size={130} />
          <div style={{
            fontFamily: FONT, fontSize: '18px', fontWeight: 500,
            color: 'var(--text)', lineHeight: 1.5, fontStyle: 'italic',
            textAlign: 'left', marginTop: '16px',
            minHeight: '54px', maxWidth: '320px', width: '100%',
          }}>{loginPennyText}<span style={{ opacity: 0.4 }} className="cursor-blink">|</span></div>
        </div>

        {/* Phone input */}
        <div style={{ width: '100%', marginBottom: '12px' }}>
          <FloatingLabelField label="Your mobile number">
            <input
              ref={phoneInputRef}
              type="tel"
              inputMode="numeric"
              autoComplete="tel-national"
              placeholder="(555) 123-4567"
              value={formatPhone(phone)}
              onChange={onPhoneChange}
              onKeyDown={e => { if (e.key === 'Enter' && isValid && smsOptIn) onSendCode() }}
              style={{ ...pageStyles.floatingInput, fontSize: '20px', fontWeight: 500, letterSpacing: '0.5px' }}
              autoFocus
            />
          </FloatingLabelField>
        </div>

        {/* SMS opt-in */}
        <label style={{
          display: 'flex', alignItems: 'flex-start', gap: '8px',
          width: '100%', marginBottom: '20px',
          cursor: 'pointer', userSelect: 'none' as const,
        }}>
          <input
            type="checkbox"
            checked={smsOptIn}
            onChange={e => onSmsOptInChange(e.target.checked)}
            style={{ accentColor: 'var(--brand)', width: '16px', height: '16px', flexShrink: 0, marginTop: '1px' }}
          />
          <span style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', lineHeight: 1.5, whiteSpace: 'nowrap' }}>
            I want to SMS with Penny
          </span>
        </label>

        {error && <p role="alert" style={{ ...authStyles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}

        <button
          onClick={() => onSendCode()}
          disabled={!isValid || !smsOptIn || loading}
          style={{
            ...authStyles.primaryButton,
            cursor: isValid && smsOptIn && !loading ? 'pointer' : 'default',
            transition: 'opacity 0.2s, transform 0.1s',
            letterSpacing: '0.2px',
            opacity: isValid && smsOptIn && !loading ? 1 : 0.35,
            marginBottom: '4px',
          }}
        >
          {loading ? 'Sending…' : 'Continue →'}
        </button>

        {/* Divider + alternative auth */}
        <div style={{ ...authStyles.dividerRow, margin: '20px 0 16px' }}>
          <div style={authStyles.dividerLine} />
          <span style={authStyles.dividerText}>or</span>
          <div style={authStyles.dividerLine} />
        </div>

        {googleClientId && (
          <div id="google-signin-btn" style={{ width: '100%', display: 'flex', justifyContent: 'center', marginBottom: '12px' }} />
        )}

        <button
          onClick={() => { setError(''); onNavigate('email-login') }}
          style={authStyles.subtleLink}
        >Use email instead</button>

        <div style={{ marginTop: 'auto', paddingTop: '16px', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
          <LegalLinks />
        </div>
      </div>
    </div>
  )
}
