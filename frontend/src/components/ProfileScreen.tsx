import { useState, useRef } from 'react'
import { FloatingLabelField } from './FloatingLabelField'
import { ToggleSwitch } from './ToggleSwitch'
import { ScreenTransition } from './ScreenTransition'
import {
  FONT, FONT_2XS, FONT_XS, FONT_SM, FONT_BASE, pageStyles, formatPhone,
} from '../styles'

interface ProfileScreenProps {
  // Settings values
  firstName: string
  lastName: string
  email: string
  phone: string
  address: string
  // Settings setters
  onFirstNameChange: (v: string) => void
  onLastNameChange: (v: string) => void
  onEmailChange: (v: string) => void
  onPhoneChange: (v: string) => void
  onAddressChange: (v: string) => void
  onAutoSave: () => void
  // Auth state
  authMethods: string[]
  onAuthMethodAdded: (method: string) => void
  currentUserId: number | null
  isAdmin: boolean
  // Shared callbacks
  onNavigate: (screen: string) => void
  onLogout: () => void
  showToast: (msg: string, type?: 'success' | 'error') => void
  onSessionExpired: () => void
  // Address autocomplete
  fetchAPI: (url: string, options?: RequestInit) => Promise<Record<string, unknown> | unknown[] | null>
  placesReady: boolean
  // Theme
  darkMode: boolean
  onToggleDarkMode: () => void
  // Header render prop
  renderHeader: () => React.ReactNode
  // Bottom tabs render prop
  renderBottomTabs: () => React.ReactNode
  // Transition
  transitionStyle: React.CSSProperties
}

export function ProfileScreen({
  firstName, lastName, email, phone, address,
  onFirstNameChange, onLastNameChange, onEmailChange, onPhoneChange, onAddressChange,
  onAutoSave,
  authMethods, onAuthMethodAdded, currentUserId, isAdmin,
  onNavigate, onLogout, showToast, onSessionExpired,
  fetchAPI, placesReady,
  darkMode, onToggleDarkMode,
  renderHeader, renderBottomTabs, transitionStyle,
}: ProfileScreenProps) {
  // Avatar state (profile-only)
  const [avatarVersion, setAvatarVersion] = useState(0)
  const [avatarUploading, setAvatarUploading] = useState(false)
  const [avatarError, setAvatarError] = useState(false)

  // Auth linking state (profile-only)
  const [linkingProvider, setLinkingProvider] = useState<'email' | 'phone' | null>(null)
  const [linkEmail, setLinkEmail] = useState('')
  const [linkPhone, setLinkPhone] = useState('')
  const [linkCode, setLinkCode] = useState('')
  const [linkStep, setLinkStep] = useState<'input' | 'verify'>('input')
  const [linkLoading, setLinkLoading] = useState(false)
  const [linkError, setLinkError] = useState('')

  // Address autocomplete state (profile-only)
  const [addressSuggestions, setAddressSuggestions] = useState<{ description: string; placeId: string }[]>([])
  const [showAddressSuggestions, setShowAddressSuggestions] = useState(false)
  const addressSuggestTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const initials = [firstName, lastName].map(n => (n || '')[0]?.toUpperCase() || '').filter(Boolean).join('')
  const avatarUrl = currentUserId ? `/api/avatar/${currentUserId}?v=${avatarVersion}` : null

  const handleAvatarPick = () => {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = 'image/*'
    input.onchange = async () => {
      const file = input.files?.[0]
      if (!file) return
      setAvatarUploading(true)
      try {
        const bitmap = await createImageBitmap(file)
        const canvas = document.createElement('canvas')
        canvas.width = 256
        canvas.height = 256
        const ctx = canvas.getContext('2d')!
        const scale = Math.max(256 / bitmap.width, 256 / bitmap.height)
        const sw = 256 / scale, sh = 256 / scale
        const sx = (bitmap.width - sw) / 2, sy = (bitmap.height - sh) / 2
        ctx.drawImage(bitmap, sx, sy, sw, sh, 0, 0, 256, 256)
        const blob = await new Promise<Blob>((resolve) => canvas.toBlob(b => resolve(b!), 'image/jpeg', 0.85))
        const buf = await blob.arrayBuffer()
        const base64 = btoa(String.fromCharCode(...new Uint8Array(buf)))
        const res = await fetch('/api/avatar', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ photo: base64 }) })
        if (res.status === 401) { onSessionExpired(); return }
        if (res.ok) {
          setAvatarVersion(v => v + 1)
          showToast('Photo updated!')
        } else {
          const data = await res.json().catch(() => ({}))
          showToast(data?.error || 'Upload failed', 'error')
        }
      } catch {
        showToast('Could not process image', 'error')
      } finally {
        setAvatarUploading(false)
      }
    }
    input.click()
  }

  const fetchAddressSuggestions = async (input: string) => {
    if (!placesReady || input.length < 3) {
      setAddressSuggestions([])
      setShowAddressSuggestions(false)
      return
    }
    try {
      const data = await fetchAPI(`/api/places/autocomplete?input=${encodeURIComponent(input)}`)
      const items = (data as { description: string; placeId: string }[]) || []
      setAddressSuggestions(items)
      setShowAddressSuggestions(items.length > 0)
    } catch {
      setAddressSuggestions([])
      setShowAddressSuggestions(false)
    }
  }

  const selectAddressSuggestion = async (placeId: string) => {
    setShowAddressSuggestions(false)
    try {
      const data = await fetchAPI(`/api/places/detail?place_id=${encodeURIComponent(placeId)}`)
      const addr = (data as { address: string })?.address
      if (addr) {
        onAddressChange(addr)
        onAutoSave()
      }
    } catch { /* silent */ }
  }

  const handleLinkEmail = async () => {
    setLinkLoading(true); setLinkError('')
    try {
      const res = await fetch('/api/auth/link', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ provider: 'email', email: linkEmail.trim() }) })
      const data = await res.json().catch(() => ({}))
      if (res.ok) {
        onAuthMethodAdded('email')
        setLinkingProvider(null)
        if (!email) onEmailChange(linkEmail.trim())
        showToast('Email linked!')
      } else {
        setLinkError(data.error || 'Failed to link email')
      }
    } catch { setLinkError('Connection error') }
    finally { setLinkLoading(false) }
  }

  const handleSendPhoneCode = async () => {
    setLinkLoading(true); setLinkError('')
    try {
      const res = await fetch('/auth/send-code', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ phone_number: `+1${linkPhone}` }) })
      if (res.ok) { setLinkStep('verify') }
      else { const d = await res.json().catch(() => ({})); setLinkError(d.error || 'Failed to send code') }
    } catch { setLinkError('Connection error') }
    finally { setLinkLoading(false) }
  }

  const handleVerifyPhoneLink = async () => {
    setLinkLoading(true); setLinkError('')
    try {
      const res = await fetch('/api/auth/link', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ provider: 'phone', phone: `+1${linkPhone}`, code: linkCode }) })
      const data = await res.json().catch(() => ({}))
      if (res.ok) {
        onAuthMethodAdded('phone')
        setLinkingProvider(null)
        if (!phone) onPhoneChange(`+1${linkPhone}`)
        showToast('Phone linked!')
      } else {
        setLinkError(data.error || 'Invalid code')
      }
    } catch { setLinkError('Connection error') }
    finally { setLinkLoading(false) }
  }

  return (
    <ScreenTransition style={transitionStyle}>
    <div style={pageStyles.page}>
      {renderHeader()}
      <div style={pageStyles.listArea}>
        <div style={{ padding: '24px 20px', display: 'flex', flexDirection: 'column', gap: '24px' }}>

          {/* Avatar + display info */}
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '8px' }}>
            <div
              onClick={handleAvatarPick}
              role="button"
              tabIndex={0}
              onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleAvatarPick() } }}
              style={{
                width: 80, height: 80, borderRadius: '50%',
                backgroundColor: 'var(--brand)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                fontFamily: FONT, fontSize: '1.75rem', fontWeight: 700, color: '#fff',
                cursor: 'pointer', position: 'relative', overflow: 'hidden',
              }}
              aria-label="Change profile photo"
              title="Tap to change photo"
            >
              {avatarUrl && !avatarError ? (
                <img
                  key={avatarVersion}
                  src={avatarUrl}
                  alt="Profile"
                  onError={() => setAvatarError(true)}
                  style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                />
              ) : (
                initials || '?'
              )}
              {avatarUploading && (
                <div style={{
                  position: 'absolute', inset: 0, backgroundColor: 'rgba(0,0,0,0.4)',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>
                  <div style={{ width: 24, height: 24, border: '3px solid rgba(255,255,255,0.3)', borderTopColor: '#fff', borderRadius: '50%', animation: 'spin 0.8s linear infinite' }} />
                </div>
              )}
              {!avatarUploading && (
                <div style={{
                  position: 'absolute', bottom: 0, left: 0, right: 0,
                  backgroundColor: 'rgba(0,0,0,0.5)', padding: '2px 0',
                  display: 'flex', justifyContent: 'center',
                }}>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
                    <circle cx="12" cy="13" r="4"/>
                  </svg>
                </div>
              )}
            </div>
          </div>

          {/* Profile fields */}
          <div style={{ display: 'flex', gap: '12px' }}>
            <div style={{ flex: 1 }}>
              <FloatingLabelField label="First name">
                <input type="text" value={firstName} onChange={e => onFirstNameChange(e.target.value)} onBlur={onAutoSave} placeholder="First" style={pageStyles.floatingInput} />
              </FloatingLabelField>
            </div>
            <div style={{ flex: 1 }}>
              <FloatingLabelField label="Last name">
                <input type="text" value={lastName} onChange={e => onLastNameChange(e.target.value)} onBlur={onAutoSave} placeholder="Last" style={pageStyles.floatingInput} />
              </FloatingLabelField>
            </div>
          </div>
          <div>
            <FloatingLabelField label="Email">
              <input type="email" value={email} onChange={e => onEmailChange(e.target.value)} onBlur={onAutoSave} placeholder="your@email.com" style={pageStyles.floatingInput} />
            </FloatingLabelField>
            <p style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', marginTop: '4px', paddingLeft: '4px' }}>
              Once set, forward any recipe or list to penny@list.thrift.ly and Penny will add it for you.
            </p>
          </div>
          <div>
            <FloatingLabelField label="Cell phone">
              <input
                type="tel"
                value={phone ? formatPhone(phone.replace('+1', '')) : ''}
                onChange={e => {
                  const digits = e.target.value.replace(/\D/g, '').slice(0, 10)
                  onPhoneChange(digits ? `+1${digits}` : '')
                }}
                onBlur={onAutoSave}
                placeholder="(555) 123-4567"
                style={pageStyles.floatingInput}
              />
            </FloatingLabelField>
            <p style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', marginTop: '4px', paddingLeft: '4px' }}>
              Text Penny your grocery list anytime.
            </p>
          </div>
          <div style={{ position: 'relative' }}>
            <FloatingLabelField label="Address">
              <input
                type="text" value={address}
                onChange={e => {
                  onAddressChange(e.target.value)
                  if (addressSuggestTimer.current) clearTimeout(addressSuggestTimer.current)
                  addressSuggestTimer.current = setTimeout(() => fetchAddressSuggestions(e.target.value), 300)
                }}
                onBlur={() => { setTimeout(() => setShowAddressSuggestions(false), 200); onAutoSave() }}
                onFocus={() => { if (addressSuggestions.length > 0) setShowAddressSuggestions(true) }}
                placeholder="Start typing your address\u2026" style={pageStyles.floatingInput} autoComplete="off"
              />
            </FloatingLabelField>
            {showAddressSuggestions && addressSuggestions.length > 0 && (
              <div style={{ position: 'absolute', top: '100%', left: 0, right: 0, zIndex: 1000, backgroundColor: 'var(--surface)', borderRadius: '10px', boxShadow: '0 4px 20px rgba(0,0,0,0.15)', border: '1px solid var(--border)', marginTop: '4px', overflow: 'hidden' }}>
                {addressSuggestions.map((s, i) => (
                  <button key={s.placeId} onMouseDown={e => { e.preventDefault(); selectAddressSuggestion(s.placeId) }}
                    style={{ display: 'block', width: '100%', padding: '12px 16px', fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text)', backgroundColor: 'transparent', border: 'none', borderTop: i > 0 ? '1px solid var(--border)' : 'none', textAlign: 'left', cursor: 'pointer', lineHeight: 1.4 }}
                  >{s.description}</button>
                ))}
              </div>
            )}
            <p style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', marginTop: '4px', paddingLeft: '4px' }}>
              Used to find nearby stores
            </p>
          </div>

          {/* Sign-in methods */}
          {authMethods.length > 0 && (
            <div>
              <div style={{ fontFamily: FONT, fontSize: FONT_XS, fontWeight: 600, color: 'var(--text-secondary)', textTransform: 'uppercase' as const, letterSpacing: '0.5px', marginBottom: '12px' }}>Sign-in methods</div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
                {/* Google */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '10px 12px', backgroundColor: 'var(--surface)', borderRadius: '10px', border: '1px solid var(--border)' }}>
                  <svg width="18" height="18" viewBox="0 0 24 24"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/></svg>
                  <span style={{ fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text)', flex: 1 }}>Google</span>
                  {authMethods.includes('google') ? (
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--success)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
                  ) : (
                    <span style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)' }}>Not linked</span>
                  )}
                </div>
                {/* Email */}
                <div style={{ backgroundColor: 'var(--surface)', borderRadius: '10px', border: '1px solid var(--border)', overflow: 'hidden' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '10px 12px' }}>
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/><polyline points="22,6 12,13 2,6"/></svg>
                    <span style={{ fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text)', flex: 1 }}>Email</span>
                    {authMethods.includes('email') ? (
                      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--success)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
                    ) : (
                      <button onClick={() => { setLinkingProvider('email'); setLinkEmail(''); setLinkError('') }} style={{ fontFamily: FONT, fontSize: FONT_2XS, fontWeight: 600, color: 'var(--brand)', background: 'none', border: 'none', cursor: 'pointer', padding: '4px 8px' }}>Add</button>
                    )}
                  </div>
                  {linkingProvider === 'email' && (
                    <div style={{ padding: '0 12px 12px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
                      <input type="email" value={linkEmail} onChange={e => setLinkEmail(e.target.value)} placeholder="your@email.com" autoFocus style={{ ...pageStyles.floatingInput, fontSize: FONT_SM }} />
                      {linkError && <p style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--error)', margin: 0 }}>{linkError}</p>}
                      <div style={{ display: 'flex', gap: '8px' }}>
                        <button onClick={() => setLinkingProvider(null)} style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', background: 'none', border: 'none', cursor: 'pointer', padding: '6px 12px' }}>Cancel</button>
                        <button
                          disabled={!linkEmail.trim() || linkLoading}
                          onClick={handleLinkEmail}
                          style={{ fontFamily: FONT, fontSize: FONT_XS, fontWeight: 600, color: '#fff', backgroundColor: 'var(--brand)', border: 'none', borderRadius: '8px', cursor: 'pointer', padding: '6px 16px', opacity: !linkEmail.trim() || linkLoading ? 0.4 : 1 }}
                        >{linkLoading ? '...' : 'Link'}</button>
                      </div>
                    </div>
                  )}
                </div>
                {/* Phone */}
                <div style={{ backgroundColor: 'var(--surface)', borderRadius: '10px', border: '1px solid var(--border)', overflow: 'hidden' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '10px 12px' }}>
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72c.127.96.361 1.903.7 2.81a2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45c.907.339 1.85.573 2.81.7A2 2 0 0 1 22 16.92z"/></svg>
                    <span style={{ fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text)', flex: 1 }}>Phone / SMS</span>
                    {authMethods.includes('phone') ? (
                      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--success)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
                    ) : (
                      <button onClick={() => { setLinkingProvider('phone'); setLinkPhone(''); setLinkCode(''); setLinkStep('input'); setLinkError('') }} style={{ fontFamily: FONT, fontSize: FONT_2XS, fontWeight: 600, color: 'var(--brand)', background: 'none', border: 'none', cursor: 'pointer', padding: '4px 8px' }}>Add</button>
                    )}
                  </div>
                  {linkingProvider === 'phone' && (
                    <div style={{ padding: '0 12px 12px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
                      {linkStep === 'input' ? (
                        <>
                          <input type="tel" value={formatPhone(linkPhone)} onChange={e => setLinkPhone(e.target.value.replace(/\D/g, '').slice(0, 10))} placeholder="(555) 123-4567" autoFocus style={{ ...pageStyles.floatingInput, fontSize: FONT_SM }} />
                          {linkError && <p style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--error)', margin: 0 }}>{linkError}</p>}
                          <div style={{ display: 'flex', gap: '8px' }}>
                            <button onClick={() => setLinkingProvider(null)} style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', background: 'none', border: 'none', cursor: 'pointer', padding: '6px 12px' }}>Cancel</button>
                            <button
                              disabled={linkPhone.length !== 10 || linkLoading}
                              onClick={handleSendPhoneCode}
                              style={{ fontFamily: FONT, fontSize: FONT_XS, fontWeight: 600, color: '#fff', backgroundColor: 'var(--brand)', border: 'none', borderRadius: '8px', cursor: 'pointer', padding: '6px 16px', opacity: linkPhone.length !== 10 || linkLoading ? 0.4 : 1 }}
                            >{linkLoading ? '...' : 'Send Code'}</button>
                          </div>
                        </>
                      ) : (
                        <>
                          <p style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', margin: 0 }}>Enter the code sent to {formatPhone(linkPhone)}</p>
                          <input type="text" inputMode="numeric" value={linkCode} onChange={e => setLinkCode(e.target.value.replace(/\D/g, '').slice(0, 6))} placeholder="000000" autoFocus maxLength={6} style={{ ...pageStyles.floatingInput, fontSize: FONT_SM, letterSpacing: '4px', textAlign: 'center' as const }} />
                          {linkError && <p style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--error)', margin: 0 }}>{linkError}</p>}
                          <div style={{ display: 'flex', gap: '8px' }}>
                            <button onClick={() => { setLinkStep('input'); setLinkCode(''); setLinkError('') }} style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', background: 'none', border: 'none', cursor: 'pointer', padding: '6px 12px' }}>Back</button>
                            <button
                              disabled={linkCode.length !== 6 || linkLoading}
                              onClick={handleVerifyPhoneLink}
                              style={{ fontFamily: FONT, fontSize: FONT_XS, fontWeight: 600, color: '#fff', backgroundColor: 'var(--brand)', border: 'none', borderRadius: '8px', cursor: 'pointer', padding: '6px 16px', opacity: linkCode.length !== 6 || linkLoading ? 0.4 : 1 }}
                            >{linkLoading ? '...' : 'Verify & Link'}</button>
                          </div>
                        </>
                      )}
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* Divider */}
          <div style={pageStyles.divider} />

          {/* Need Help? */}
          <button onClick={() => onNavigate('help')} style={{
            background: 'none', border: 'none', cursor: 'pointer', padding: 0,
            fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 500, color: 'var(--brand)',
            textAlign: 'left' as const,
            display: 'flex', alignItems: 'center', gap: '8px',
          }}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="10"/>
              <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/>
              <line x1="12" y1="17" x2="12.01" y2="17"/>
            </svg>
            Need Help?
          </button>

          {/* Admin dashboard — admin only */}
          {isAdmin && (
            <button onClick={() => onNavigate('admin')} style={{
              background: 'none', border: 'none', cursor: 'pointer', padding: 0,
              fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 500, color: 'var(--brand)',
              textAlign: 'left' as const,
              display: 'flex', alignItems: 'center', gap: '8px',
            }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <rect x="18" y="3" width="4" height="18"/><rect x="10" y="8" width="4" height="13"/><rect x="2" y="13" width="4" height="8"/>
              </svg>
              Dashboard
            </button>
          )}

          {/* Dark mode toggle */}
          <div style={{
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            padding: '4px 0',
          }}>
            <div>
              <div style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 500, color: 'var(--text)' }}>Dark mode</div>
              <div style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)' }}>Easier on the eyes at night</div>
            </div>
            <ToggleSwitch value={darkMode} onChange={onToggleDarkMode} />
          </div>

          <div style={{ height: '1px', backgroundColor: 'var(--border)', margin: '8px 0' }} />

          {/* Sign out */}
          <button onClick={onLogout} style={{
            background: 'none', border: 'none', cursor: 'pointer', padding: 0,
            fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 500, color: 'var(--error)',
            textAlign: 'left' as const,
          }}>Sign Out</button>

        </div>
      </div>

      {renderBottomTabs()}
    </div>
    </ScreenTransition>
  )
}
