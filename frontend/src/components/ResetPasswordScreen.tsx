import { useState } from 'react'
import { FloatingLabelField } from './FloatingLabelField'
import {
  TEXT, TEXT_SECONDARY, FONT,
  authStyles, pageStyles,
} from '../styles'

interface ResetPasswordScreenProps {
  passwordInput: string
  confirmPassword: string
  loading: boolean
  error: string
  onPasswordChange: (v: string) => void
  onConfirmPasswordChange: (v: string) => void
  onSubmit: () => void
  onNavigate: (screen: string) => void
  setError: (e: string) => void
}

export function ResetPasswordScreen({
  passwordInput, confirmPassword, loading, error,
  onPasswordChange, onConfirmPasswordChange, onSubmit, onNavigate, setError,
}: ResetPasswordScreenProps) {
  const [showPassword, setShowPassword] = useState(false)

  return (
    <div style={authStyles.pageCenter}>
      <div style={authStyles.inner}>
        <div style={{ width: '100%', marginBottom: '24px', textAlign: 'center' }}>
          <div style={{ fontFamily: FONT, fontSize: '22px', fontWeight: 700, color: TEXT, letterSpacing: '-0.5px', marginBottom: '6px' }}>
            Set a new password
          </div>
        </div>

        <div style={{ width: '100%', marginBottom: '12px', position: 'relative' }}>
          <FloatingLabelField label="New password">
            <input type={showPassword ? 'text' : 'password'} autoComplete="new-password" value={passwordInput} onChange={e => onPasswordChange(e.target.value)} style={{ ...pageStyles.floatingInput, paddingRight: '44px' }} autoFocus />
          </FloatingLabelField>
          <button type="button" onClick={() => setShowPassword(p => !p)} aria-label={showPassword ? 'Hide password' : 'Show password'} style={{ position: 'absolute', right: '12px', bottom: '12px', background: 'none', border: 'none', cursor: 'pointer', padding: '4px', color: TEXT_SECONDARY }}>
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">{showPassword ? <><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></> : <><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></>}</svg>
          </button>
        </div>

        <div style={{ width: '100%', marginBottom: '12px' }}>
          <FloatingLabelField label="Confirm password">
            <input
              type={showPassword ? 'text' : 'password'}
              autoComplete="new-password"
              value={confirmPassword}
              onChange={e => onConfirmPasswordChange(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') onSubmit() }}
              style={pageStyles.floatingInput}
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
          {loading ? 'Resetting…' : 'Reset password'}
        </button>

        <button onClick={() => { setError(''); onNavigate('email-login') }} style={authStyles.subtleLink}>Back to sign in</button>
      </div>
    </div>
  )
}
