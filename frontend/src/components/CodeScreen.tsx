import { useRef, useCallback } from 'react'
import { LegalLinks } from './LegalLinks'
import { FONT, FONT_BASE, FONT_XL, formatPhone, authStyles, layout } from '../styles'

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
  const hiddenInputRef = useRef<HTMLInputElement>(null)

  // When the hidden input receives the full OTP (e.g. via iOS autofill),
  // distribute it across the visual digit boxes via onCodeChange
  const handleHiddenInput = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const digits = e.target.value.replace(/\D/g, '')
    if (digits.length >= 1) {
      // Feed the full string into the first code change handler,
      // which already handles multi-digit distribution
      onCodeChange(0, digits.slice(0, 6))
    }
  }, [onCodeChange])

  // Tapping a visual box focuses the hidden input so iOS keyboard/autofill works
  const focusHiddenInput = useCallback(() => {
    hiddenInputRef.current?.focus()
  }, [])

  const fullCode = code.join('')

  return (
    <div style={authStyles.page}>
      <div style={authStyles.innerFlex}>

        {/* Headline */}
        <div style={{ width: '100%', marginBottom: '24px', textAlign: 'center' }}>
          <div style={{
            fontFamily: FONT, fontSize: FONT_XL, fontWeight: 700,
            color: 'var(--text)', letterSpacing: '-0.5px', lineHeight: 1.2,
            marginBottom: '6px',
          }}>Check your texts</div>
          <div style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text-secondary)', lineHeight: 1.5 }}>
            Enter the code sent to {formatPhone(phone)}
          </div>
        </div>

        {/* Hidden single input for iOS OTP autofill */}
        <input
          ref={hiddenInputRef}
          type="text"
          inputMode="numeric"
          autoComplete="one-time-code"
          pattern="[0-9]*"
          maxLength={6}
          value={fullCode}
          onChange={handleHiddenInput}
          onPaste={onCodePaste}
          disabled={loading}
          aria-label="Verification code"
          style={{
            position: 'absolute',
            opacity: 0,
            width: '1px',
            height: '1px',
            overflow: 'hidden',
            pointerEvents: 'none',
          }}
        />

        {/* Visual code boxes */}
        <div
          style={{ ...layout.rowCenter, gap: '8px', marginBottom: '16px', width: '100%', cursor: 'text' }}
          onClick={focusHiddenInput}
        >
          {code.map((digit, i) => (
            <input
              key={i}
              ref={el => { codeInputRefs.current[i] = el }}
              type="text"
              inputMode="numeric"
              pattern="[0-9]*"
              autoComplete="off"
              value={digit}
              aria-label={`Verification code digit ${i + 1} of 6`}
              onChange={e => onCodeChange(i, e.target.value)}
              onKeyDown={e => onCodeKeyDown(i, e)}
              onPaste={onCodePaste}
              onFocus={() => {
                // On iOS, also ensure the hidden input is focused for autofill
                // Only redirect focus if user tapped (not programmatic focus from handleCodeChange)
                if (!digit && fullCode.length <= i) {
                  hiddenInputRef.current?.focus()
                }
              }}
              disabled={loading}
              style={{
                width: '48px', height: '60px',
                fontSize: '1.625rem', fontFamily: FONT, fontWeight: 700,
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
          <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--brand)', fontWeight: 500, margin: '0 0 12px 0' }}>
            Signing you in…
          </p>
        )}

        {error && <p role="alert" style={{ ...authStyles.error, width: '100%', textAlign: 'center' as const }}>{error}</p>}

        {/* Helper links */}
        <div style={{ ...layout.row, gap: '16px', marginTop: '8px' }}>
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

        <div style={{ ...layout.columnCenter, marginTop: 'auto', paddingTop: '16px' }}>
          <LegalLinks />
        </div>
      </div>
    </div>
  )
}
