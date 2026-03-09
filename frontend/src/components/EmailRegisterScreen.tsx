import { useState } from 'react'
import { PennyAvatar } from './PennyAvatar'
import { FloatingLabelField } from './FloatingLabelField'
import { LegalLinks } from './LegalLinks'
import {
  FONT, FONT_2XS, FONT_BASE,
  authStyles, pageStyles,
} from '../styles'

interface EmailRegisterScreenProps {
  nameInput: string
  lastNameInput: string
  emailInput: string
  passwordInput: string
  confirmPassword: string
  loading: boolean
  error: string
  loginPennyText: string
  onNameChange: (v: string) => void
  onLastNameChange: (v: string) => void
  onEmailChange: (v: string) => void
  onPasswordChange: (v: string) => void
  onConfirmPasswordChange: (v: string) => void
  onRegister: () => void
  onNavigate: (screen: string) => void
  setError: (e: string) => void
}

export function EmailRegisterScreen({
  nameInput, lastNameInput, emailInput, passwordInput, confirmPassword,
  loading, error, loginPennyText,
  onNameChange, onLastNameChange, onEmailChange, onPasswordChange, onConfirmPasswordChange,
  onRegister, onNavigate, setError,
}: EmailRegisterScreenProps) {
  const [showPassword, setShowPassword] = useState(false)

  return (
    <div style={authStyles.pageCenter}>
      <div style={authStyles.inner}>
        {/* Penny hero */}
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', marginBottom: '24px', width: '100%' }}>
          <PennyAvatar size={80} />
          <div style={{
            fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 500,
            color: 'var(--text)', lineHeight: 1.5, fontStyle: 'italic',
            textAlign: 'left', marginTop: '12px',
            minHeight: '44px', maxWidth: '320px', width: '100%',
          }}>{loginPennyText}<span style={{ opacity: 0.4 }} className="cursor-blink">|</span></div>
        </div>

        <div style={{ display: 'flex', gap: '12px', width: '100%', marginBottom: '12px' }}>
          <div style={{ flex: 1 }}>
            <FloatingLabelField label="First name">
              <input type="text" autoComplete="given-name" value={nameInput} onChange={e => onNameChange(e.target.value)} style={pageStyles.floatingInput} autoFocus />
            </FloatingLabelField>
          </div>
          <div style={{ flex: 1 }}>
            <FloatingLabelField label="Last name">
              <input type="text" autoComplete="family-name" value={lastNameInput} onChange={e => onLastNameChange(e.target.value)} style={pageStyles.floatingInput} />
            </FloatingLabelField>
          </div>
        </div>

        <div style={{ width: '100%', marginBottom: '12px' }}>
          <FloatingLabelField label="Email">
            <input type="email" autoComplete="email" value={emailInput} onChange={e => onEmailChange(e.target.value)} style={pageStyles.floatingInput} />
          </FloatingLabelField>
        </div>

        <div style={{ width: '100%', marginBottom: '4px', position: 'relative' }}>
          <FloatingLabelField label="Password">
            <input type={showPassword ? 'text' : 'password'} autoComplete="new-password" value={passwordInput} onChange={e => onPasswordChange(e.target.value)} style={{ ...pageStyles.floatingInput, paddingRight: '44px' }} />
          </FloatingLabelField>
          <button type="button" onClick={() => setShowPassword(p => !p)} aria-label={showPassword ? 'Hide password' : 'Show password'} style={{ position: 'absolute', right: '12px', bottom: '12px', background: 'none', border: 'none', cursor: 'pointer', padding: '4px', color: 'var(--text-secondary)' }}>
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">{showPassword ? <><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></> : <><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></>}</svg>
          </button>
        </div>
        <p style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', marginBottom: '12px', paddingLeft: '4px' }}>At least 8 characters</p>

        <div style={{ width: '100%', marginBottom: '12px', position: 'relative' }}>
          <FloatingLabelField label="Confirm password">
            <input
              type={showPassword ? 'text' : 'password'}
              autoComplete="new-password"
              value={confirmPassword}
              onChange={e => onConfirmPasswordChange(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') onRegister() }}
              style={{ ...pageStyles.floatingInput, paddingRight: '44px' }}
            />
          </FloatingLabelField>
        </div>

        {error && <p role="alert" style={{ ...authStyles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}

        <button
          onClick={onRegister}
          disabled={loading}
          style={{
            ...authStyles.primaryButton,
            cursor: loading ? 'default' : 'pointer',
            opacity: loading ? 0.5 : 1,
          }}
        >
          {loading ? 'Creating account…' : 'Create account'}
        </button>

        <button onClick={() => { setError(''); onNavigate('email-login') }} style={authStyles.subtleLink}>Already have an account? Sign in</button>

        <div style={{ marginTop: '12px' }}><LegalLinks /></div>
      </div>
    </div>
  )
}
