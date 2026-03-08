import { useState } from 'react'
import { PennyAvatar } from './PennyAvatar'
import { FloatingLabelField } from './FloatingLabelField'
import { LegalLinks } from './LegalLinks'
import {
  FONT,
  authStyles, pageStyles,
} from '../styles'

interface EmailLoginScreenProps {
  emailInput: string
  passwordInput: string
  loading: boolean
  error: string
  magicLinkSent: boolean
  loginPennyText: string
  googleClientId: string
  onEmailChange: (v: string) => void
  onPasswordChange: (v: string) => void
  onLogin: () => void
  onMagicLink: () => void
  onNavigate: (screen: string) => void
  setError: (e: string) => void
  setMagicLinkSent: (v: boolean) => void
}

export function EmailLoginScreen({
  emailInput, passwordInput, loading, error, magicLinkSent, loginPennyText, googleClientId,
  onEmailChange, onPasswordChange, onLogin, onMagicLink, onNavigate, setError, setMagicLinkSent,
}: EmailLoginScreenProps) {
  const [showPassword, setShowPassword] = useState(false)

  return (
    <div style={authStyles.pageCenter}>
      <div style={authStyles.inner}>
        {/* Penny hero */}
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', marginBottom: '24px', width: '100%' }}>
          <PennyAvatar size={80} />
          <div style={{
            fontFamily: FONT, fontSize: '15px', fontWeight: 500,
            color: 'var(--text)', lineHeight: 1.5, fontStyle: 'italic',
            textAlign: 'left', marginTop: '12px',
            minHeight: '44px', maxWidth: '320px', width: '100%',
          }}>{loginPennyText}<span style={{ opacity: 0.4 }} className="cursor-blink">|</span></div>
        </div>

        <div style={{ width: '100%', marginBottom: '12px' }}>
          <FloatingLabelField label="Email">
            <input
              type="email"
              autoComplete="email"
              value={emailInput}
              onChange={e => onEmailChange(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') onLogin() }}
              style={pageStyles.floatingInput}
              autoFocus
            />
          </FloatingLabelField>
        </div>

        <div style={{ width: '100%', marginBottom: '12px', position: 'relative' }}>
          <FloatingLabelField label="Password">
            <input
              type={showPassword ? 'text' : 'password'}
              autoComplete="current-password"
              value={passwordInput}
              onChange={e => onPasswordChange(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') onLogin() }}
              style={{ ...pageStyles.floatingInput, paddingRight: '44px' }}
            />
          </FloatingLabelField>
          <button type="button" onClick={() => setShowPassword(p => !p)} aria-label={showPassword ? 'Hide password' : 'Show password'} style={{ position: 'absolute', right: '12px', bottom: '12px', background: 'none', border: 'none', cursor: 'pointer', padding: '4px', color: 'var(--text-secondary)' }}>
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">{showPassword ? <><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></> : <><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></>}</svg>
          </button>
        </div>

        {error && <p role="alert" style={{ ...authStyles.error, width: '100%', textAlign: 'left' as const }}>{error}</p>}
        {magicLinkSent && <p style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--brand)', fontWeight: 500, width: '100%', textAlign: 'left' as const, marginBottom: '12px' }}>We sent a sign-in link to your email — check your inbox!</p>}

        <button
          onClick={onLogin}
          disabled={loading}
          style={{
            ...authStyles.primaryButton,
            cursor: loading ? 'default' : 'pointer',
            opacity: loading ? 0.5 : 1,
          }}
        >
          {loading ? 'Signing in…' : 'Sign in'}
        </button>

        <div style={{ display: 'flex', gap: '16px', marginBottom: '16px' }}>
          <button onClick={() => { setError(''); setMagicLinkSent(false); onNavigate('forgot-password') }} style={authStyles.subtleLink}>Forgot password?</button>
          <button onClick={onMagicLink} disabled={loading} style={{ ...authStyles.subtleLink, opacity: loading ? 0.5 : 1 }}>Email me a sign-in link</button>
          <button onClick={() => { setError(''); setMagicLinkSent(false); onNavigate('email-register') }} style={authStyles.subtleLink}>Create an account</button>
        </div>

        {/* Divider + alternative auth */}
        <div style={{ ...authStyles.dividerRow, margin: '8px 0 16px' }}>
          <div style={authStyles.dividerLine} />
          <span style={authStyles.dividerText}>or</span>
          <div style={authStyles.dividerLine} />
        </div>

        {googleClientId && (
          <div id="google-signin-btn" style={{ width: '100%', display: 'flex', justifyContent: 'center', marginBottom: '12px' }} />
        )}

        <button onClick={() => { setError(''); onNavigate('phone') }} style={authStyles.subtleLink}>Use phone number instead</button>

        <div style={{ marginTop: '12px' }}><LegalLinks /></div>
      </div>
    </div>
  )
}
