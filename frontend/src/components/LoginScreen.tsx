import { Suspense, lazy, useContext } from 'react'
import { PennyAvatar } from './PennyAvatar'
import { LegalLinks } from './LegalLinks'
import { SignInUnavailable } from './SignInUnavailable'
import { RallyReadyContext } from '../auth/RallyReadyContext'
import { FONT, authStyles, layout, text } from '../styles'

// Lazy so the screen itself never imports Clerk: the button only loads once
// RallyReadyContext says its provider is mounted.
const ClerkSignInButton = lazy(() => import('../auth/ClerkSignInButton'))

interface LoginScreenProps {
  /** Penny's typewriter line. */
  loginPennyText: string
  /** /api/config's clerk_enabled. null while the answer is still on its way. */
  clerkEnabled: boolean | null
  /** Why the last sign-in attempt failed, in the server's own words. */
  error?: string
}

/**
 * The one sign-in screen: Penny, "Continue with Rally", legal links.
 *
 * It collects no credentials — Rally hosts those.
 */
export function LoginScreen({ loginPennyText, clerkEnabled, error = '' }: LoginScreenProps) {
  const rallyReady = useContext(RallyReadyContext)
  const unavailable = !rallyReady || clerkEnabled === false

  return (
    <div data-testid="login-screen" style={authStyles.page}>
      <div style={authStyles.innerFlex}>

        {/* Penny hero */}
        <div style={{ ...layout.columnCenter, marginTop: 'auto', marginBottom: '28px', width: '100%' }}>
          <PennyAvatar size={130} />
          <div style={{
            fontFamily: FONT, fontSize: '1.125rem', fontWeight: 500,
            color: 'var(--text)', lineHeight: 1.5, fontStyle: 'italic',
            textAlign: 'left', marginTop: '16px',
            minHeight: '54px', maxWidth: '320px', width: '100%',
          }}>{loginPennyText}<span style={{ opacity: 0.4 }} className="cursor-blink">|</span></div>
        </div>

        {error && <p role="alert" style={{ ...authStyles.error, width: '100%', textAlign: 'center' }}>{error}</p>}

        {/* One action, full width, in the thumb's reach. */}
        <div style={{ ...layout.columnCenter, width: '100%', minHeight: '68px' }}>
          {unavailable ? (
            <SignInUnavailable />
          ) : clerkEnabled === null ? null : (
            <Suspense fallback={null}>
              <ClerkSignInButton />
            </Suspense>
          )}
        </div>

        {!unavailable && (
          <p style={{ ...text.caption, fontSize: '0.6875rem', opacity: 0.7, lineHeight: 1.5, margin: '4px 0 0', textAlign: 'center', maxWidth: '320px' }}>
            By continuing you agree to our <a href="/terms" style={{ color: 'var(--text-secondary)', textDecoration: 'underline' }}>Terms</a> and <a href="/privacy" style={{ color: 'var(--text-secondary)', textDecoration: 'underline' }}>Privacy Policy</a>.
          </p>
        )}

        <div style={{ ...layout.columnCenter, marginTop: 'auto', paddingTop: '16px' }}>
          <LegalLinks />
        </div>
      </div>
    </div>
  )
}

export default LoginScreen
