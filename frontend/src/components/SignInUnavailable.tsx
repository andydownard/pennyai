import { FONT, FONT_BASE, FONT_SM } from '../styles'

/**
 * Shown in place of the Rally button when sign-in cannot be offered: the build
 * has no Rally configuration, the server has it switched off, or Rally itself
 * did not load. Calm and plain — nothing here is the visitor's fault.
 */
export function SignInUnavailable() {
  return (
    <div
      role="status"
      style={{
        width: '100%', boxSizing: 'border-box', padding: '16px',
        backgroundColor: 'var(--surface)', border: '1px solid var(--border)', borderRadius: '12px',
        fontFamily: FONT, textAlign: 'center',
      }}
    >
      <p style={{ fontSize: FONT_BASE, fontWeight: 600, color: 'var(--text)', margin: '0 0 4px' }}>
        Sign-in is briefly unavailable.
      </p>
      <p style={{ fontSize: FONT_SM, color: 'var(--text-secondary)', margin: 0, lineHeight: 1.5 }}>
        Your list is safe. Please try again in a few minutes.
      </p>
    </div>
  )
}

export default SignInUnavailable
