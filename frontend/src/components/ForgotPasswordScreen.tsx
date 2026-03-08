import { FloatingLabelField } from './FloatingLabelField'
import {
  TEXT, TEXT_SECONDARY, FONT,
  authStyles, pageStyles,
} from '../styles'

interface ForgotPasswordScreenProps {
  emailInput: string
  loading: boolean
  error: string
  resetSent: boolean
  onEmailChange: (v: string) => void
  onSubmit: () => void
  onNavigate: (screen: string) => void
  setError: (e: string) => void
  setResetSent: (v: boolean) => void
}

export function ForgotPasswordScreen({
  emailInput, loading, error, resetSent,
  onEmailChange, onSubmit, onNavigate, setError, setResetSent,
}: ForgotPasswordScreenProps) {
  return (
    <div style={authStyles.pageCenter}>
      <div style={authStyles.inner}>
        <div style={{ width: '100%', marginBottom: '24px', textAlign: 'center' }}>
          <div style={{ fontFamily: FONT, fontSize: '22px', fontWeight: 700, color: TEXT, letterSpacing: '-0.5px', marginBottom: '6px' }}>
            Reset your password
          </div>
          <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY }}>
            {resetSent ? "If an account exists with that email, we've sent a reset link." : "Enter your email and we'll send you a reset link."}
          </div>
        </div>

        {!resetSent && (
          <>
            <div style={{ width: '100%', marginBottom: '12px' }}>
              <FloatingLabelField label="Email">
                <input
                  type="email"
                  autoComplete="email"
                  value={emailInput}
                  onChange={e => onEmailChange(e.target.value)}
                  onKeyDown={e => { if (e.key === 'Enter') onSubmit() }}
                  style={pageStyles.floatingInput}
                  autoFocus
                />
              </FloatingLabelField>
            </div>

            {error && <p role="alert" style={{ ...authStyles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}

            <button
              onClick={onSubmit}
              disabled={loading}
              style={{
                ...authStyles.primaryButton,
                cursor: loading ? 'default' : 'pointer',
                opacity: loading ? 0.5 : 1,
              }}
            >
              {loading ? 'Sending…' : 'Send reset link'}
            </button>
          </>
        )}

        <button onClick={() => { setError(''); setResetSent(false); onNavigate('email-login') }} style={authStyles.subtleLink}>Back to sign in</button>
      </div>
    </div>
  )
}
