import { useState, useRef } from 'react'
import { FloatingLabelField } from './FloatingLabelField'
import { ToggleSwitch } from './ToggleSwitch'
import { ScreenTransition } from './ScreenTransition'
import { fetchAPI } from '../api'
import { useAvatarVersion, bumpAvatarVersion, initialsOf } from '../avatarStore'
import {
  FONT, FONT_2XS, FONT_SM, FONT_BASE, pageStyles,
} from '../styles'

interface ProfileScreenProps {
  // Settings values
  firstName: string
  lastName: string
  /** From the Rally account, when Rally has verified one. Read-only here. */
  address: string
  // Settings setters
  onFirstNameChange: (v: string) => void
  onLastNameChange: (v: string) => void
  onAddressChange: (v: string) => void
  onAutoSave: () => void
  // Auth state
  currentUserId: number | null
  isAdmin: boolean
  // Shared callbacks
  onNavigate: (screen: string) => void
  showToast: (msg: string, type?: 'success' | 'error') => void
  onSessionExpired: () => void
  // Address autocomplete
  placesReady: boolean
  // Theme
  darkMode: boolean
  onToggleDarkMode: () => void
  // Header render prop
  renderHeader: () => React.ReactNode
  // Transition
  transitionStyle: React.CSSProperties
}

export function ProfileScreen({
  firstName, lastName, address,
  onFirstNameChange, onLastNameChange, onAddressChange,
  onAutoSave,
  currentUserId, isAdmin,
  onNavigate, showToast, onSessionExpired,
  placesReady,
  darkMode, onToggleDarkMode,
  renderHeader, transitionStyle,
}: ProfileScreenProps) {
  // Avatar state (profile-only)
  const avatarVersion = useAvatarVersion()
  const [avatarUploading, setAvatarUploading] = useState(false)
  const [avatarError, setAvatarError] = useState(false)

  // Address autocomplete state (profile-only)
  const [addressSuggestions, setAddressSuggestions] = useState<{ description: string; placeId: string }[]>([])
  const [showAddressSuggestions, setShowAddressSuggestions] = useState(false)
  const addressSuggestTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const initials = initialsOf(firstName, lastName)
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
          bumpAvatarVersion()
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
      const data = await fetchAPI<{ description: string; placeId: string }[]>(`/api/places/autocomplete?input=${encodeURIComponent(input)}`)
      const items = data || []
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
      const data = await fetchAPI<{ address: string }>(`/api/places/detail?place_id=${encodeURIComponent(placeId)}`)
      const addr = data?.address
      if (addr) {
        onAddressChange(addr)
        onAutoSave()
      }
    } catch { /* silent */ }
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
                placeholder="Start typing your address…" style={pageStyles.floatingInput} autoComplete="off"
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


        </div>
      </div>

    </div>
    </ScreenTransition>
  )
}
