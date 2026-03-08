import { LegalLinks } from './LegalLinks'
import { FONT, formatPhone, authStyles } from '../styles'

interface CodeScreenProps {
  phone: string
  code: string[]
  loading: boolean
  error: string
  codeInputRefs: React.MutableRefObject<(HTMLInputElement | null)[]>
  onCodeChange: (index: number, value: string) => void
  onCodeKeyDown: (index: number, e: React.KeyboardEvent<HTMLInputElement>) => void
  onCodePaste: (e: React.ClipboardEvent<HTMLInputElement>) => void
  onResendCode: (phone?: string) => void
  onNavigate: (screen: string) => void
  setError: (e: string) => void
}

export function CodeScreen({
  phone, code, loading, error, codeInputRefs,
  onCodeChange, onCodeKeyDown, onCodePaste,
  onResendCode, onNavigate, setError,
}: CodeScreenProps) {
  return (
    <div style={authStyles.page}>
      <div style={authStyles.innerFlex}>

        {/* Headline */}
        <div style={{ width: '100%', marginBottom: '24px', textAlign: 'center' }}>
          <div style={{
            fontFamily: FONT, fontSize: '22px', fontWeight: 700,
            color: 'var(--text)', letterSpacing: '-0.5px', lineHeight: 1.2,
            marginBottom: '6px',
          }}>Check your texts</div>
          <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', lineHeight: 1.5 }}>
            Enter the code sent to {formatPhone(phone)}
          </div>
        </div>

        {/* Code boxes */}
        <div style={{ display: 'flex', gap: '8px', marginBottom: '16px', width: '100%', justifyContent: 'center' }}>
          {code.map((digit, i) => (
            <input
              key={i}
              ref={el => { codeInputRefs.current[i] = el }}
              type="text"
              inputMode="numeric"
              pattern="[0-9]*"
              autoComplete={i === 0 ? 'one-time-code' : 'off'}
              value={digit}
              aria-label={`Verification code digit ${i + 1} of 6`}
              onChange={e => onCodeChange(i, e.target.value)}
              onKeyDown={e => onCodeKeyDown(i, e)}
              onPaste={onCodePaste}
              disabled={loading}
              style={{
                width: '48px', height: '60px',
                fontSize: '26px', fontFamily: FONT, fontWeight: 700,
                color: 'var(--text)', textAlign: 'center' as const,
                backgroundColor: 'var(--surface)',
                border: `2px solid ${digit ? 'var(--brand)' : 'var(--border)'}`,
                borderRadius: '12px', outline: 'none', caretColor: 'var(--brand)',
                transition: 'border-color 0.15s, box-shadow 0.15s',
                boxShadow: digit ? '0 0 0 3px color-mix(in srgb, var(--brand) 12%, transparent)' : 'none',
                WebkitTextSizeAdjust: '100%',
                opacity: loading ? 0.5 : 1,
                boxSizing: 'border-box',
              }}
            />
          ))}
        </div>

        {loading && (
          <p style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--brand)', fontWeight: 500, margin: '0 0 12px 0' }}>
            Signing you in…
          </p>
        )}

        {error && <p role="alert" style={{ ...authStyles.error, width: '100%', textAlign: 'center' as const }}>{error}</p>}

        {/* Helper links */}
        <div style={{ display: 'flex', gap: '16px', marginTop: '8px' }}>
          <button
            onClick={() => { onResendCode(phone) }}
            disabled={loading}
            style={{ ...authStyles.subtleLink, opacity: loading ? 0.5 : 1 }}
          >Resend code</button>
          <button
            onClick={() => { onNavigate('phone'); setError('') }}
            style={authStyles.subtleLink}
          >Use a different number</button>
        </div>

        <div style={{ marginTop: 'auto', paddingTop: '16px', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
          <LegalLinks />
        </div>
      </div>
    </div>
  )
}
