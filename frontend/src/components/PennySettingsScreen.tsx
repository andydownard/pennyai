import { useRef, useState } from 'react'
import { PennyAvatar } from './PennyAvatar'
import { PennyInsightsCard } from './PennyInsightsCard'
import { ToggleSwitch } from './ToggleSwitch'
import { ScreenTransition } from './ScreenTransition'
import { FONT, pageStyles, formatPhone } from '../styles'

interface PennySettingsScreenProps {
  pennyPhone: string
  pennySlider: number
  pennyAutotune: boolean
  pennyDeals: boolean
  onSliderChange: (val: number) => void
  onAutotuneToggle: () => void
  onDealsToggle: () => void
  fetchAPI: (url: string, options?: RequestInit) => Promise<Record<string, unknown> | unknown[] | null>
  track: (event: string, props?: Record<string, unknown>) => void
  renderHeader: () => React.ReactNode
  renderToast: () => React.ReactNode
  renderBottomTabs: () => React.ReactNode
  transitionStyle: React.CSSProperties
}

export function PennySettingsScreen({
  pennyPhone, pennySlider, pennyAutotune, pennyDeals,
  onSliderChange, onAutotuneToggle, onDealsToggle,
  fetchAPI, track,
  renderHeader, renderToast, renderBottomTabs, transitionStyle,
}: PennySettingsScreenProps) {
  const sliderSaveTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [showInsights, setShowInsights] = useState(false)

  const saveSlider = (val: number) => {
    onSliderChange(val)
    if (sliderSaveTimeout.current) clearTimeout(sliderSaveTimeout.current)
    sliderSaveTimeout.current = setTimeout(() => {
      fetchAPI('/api/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ penny_slider: val }),
      })
    }, 400)
  }

  const pennyPhoneFormatted = pennyPhone ? formatPhone(pennyPhone.replace('+1', '')) : ''

  return (
    <ScreenTransition style={transitionStyle}>
    <div style={pageStyles.page}>
      {renderHeader()}
      <div style={pageStyles.listArea}>
        <div style={{ padding: '24px 20px', display: 'flex', flexDirection: 'column', gap: '20px' }}>

          {/* Contact Penny (top) */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '14px' }}>
            <PennyAvatar size={52} />
            <div>
              <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: 'var(--text)' }}>Penny</div>
              <div style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text-secondary)', marginTop: '2px' }}>Your household's AI shopping assistant</div>
            </div>
            <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '16px' }}>
              {pennyPhoneFormatted && (
                <a href={`sms:${pennyPhone}`} style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: '44px', height: '44px', borderRadius: '50%', backgroundColor: 'var(--bg)', textDecoration: 'none' }} aria-label="Text Penny">
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
                </a>
              )}
              <a href="mailto:penny@list.thrift.ly" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: '44px', height: '44px', borderRadius: '50%', backgroundColor: 'var(--bg)', textDecoration: 'none' }} aria-label="Email Penny">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/><polyline points="22,6 12,13 2,6"/></svg>
              </a>
              <a href="/api/penny.vcf" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: '44px', height: '44px', borderRadius: '50%', backgroundColor: 'var(--bg)', textDecoration: 'none' }} aria-label="Add Penny to contacts">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/></svg>
              </a>
            </div>
          </div>

          {/* Divider */}
          <div style={{ height: '1px', backgroundColor: 'var(--border)', margin: '4px 0' }} />

          {/* Deals toggle — only show if user has messaging (phone) */}
          {pennyPhone && (
            <div onClick={onDealsToggle} style={pageStyles.settingsCard}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/>
              </svg>
              <div style={{ flex: 1 }}>
                <div style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text)' }}>I want to SMS with Penny</div>
                <div style={{ fontFamily: FONT, fontSize: '11px', color: 'var(--text-secondary)', marginTop: '1px' }}>Penny texts you about deals and savings she finds</div>
              </div>
              <ToggleSwitch value={pennyDeals} onChange={onDealsToggle} />
            </div>
          )}

          {/* Learn from my choices toggle */}
          <div onClick={onAutotuneToggle} style={pageStyles.settingsCard}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12 2a7 7 0 0 1 7 7c0 2.38-1.19 4.47-3 5.74V17a1 1 0 0 1-1 1h-6a1 1 0 0 1-1-1v-2.26C6.19 13.47 5 11.38 5 9a7 7 0 0 1 7-7z"/><path d="M9 21h6"/><path d="M10 17v-2.5"/><path d="M14 17v-2.5"/>
            </svg>
            <div style={{ flex: 1 }}>
              <div style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text)' }}>Learn from my choices</div>
              <div style={{ fontFamily: FONT, fontSize: '11px', color: 'var(--text-secondary)', marginTop: '1px' }}>Penny remembers your brand preferences over time</div>
            </div>
            <ToggleSwitch value={pennyAutotune} onChange={onAutotuneToggle} />
          </div>

          {/* "What Penny Knows" — only visible when autotune is ON */}
          {pennyAutotune && (
            <>
              {!showInsights ? (
                <button
                  onClick={() => setShowInsights(true)}
                  style={{
                    width: '100%', padding: '10px 16px',
                    background: 'none', border: '1px solid var(--border)',
                    borderRadius: '10px', cursor: 'pointer',
                    display: 'flex', alignItems: 'center', gap: '8px',
                    fontFamily: FONT, fontSize: '13px', color: 'var(--brand)',
                  }}
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/>
                  </svg>
                  What Penny knows about you
                </button>
              ) : (
                <PennyInsightsCard
                  fetchAPI={fetchAPI}
                  onClose={() => setShowInsights(false)}
                  track={track}
                />
              )}
            </>
          )}

          {/* Slider — only visible when autotune is OFF */}
          <div style={{
            maxHeight: pennyAutotune ? 0 : '200px',
            opacity: pennyAutotune ? 0 : 1,
            overflow: pennyAutotune ? 'hidden' : 'visible',
            transition: 'max-height 0.3s ease, opacity 0.25s ease',
            pointerEvents: pennyAutotune ? 'none' : 'auto',
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '12px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/>
                </svg>
                <div>
                  <div style={{ fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>Convenience</div>
                  <div style={{ fontFamily: FONT, fontSize: '11px', color: 'var(--text-secondary)' }}>Just grab my usual</div>
                </div>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                <div style={{ textAlign: 'right' }}>
                  <div style={{ fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>Savings</div>
                  <div style={{ fontFamily: FONT, fontSize: '11px', color: 'var(--text-secondary)' }}>Help me save</div>
                </div>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/><text x="13" y="11" fontSize="7" fontWeight="bold" fill="var(--text)" stroke="none" fontFamily="sans-serif">%</text>
                </svg>
              </div>
            </div>
            <input
              type="range"
              min={0}
              max={100}
              value={pennySlider}
              aria-label="Convenience versus savings preference"
              onChange={e => saveSlider(Number(e.target.value))}
              style={{
                width: '100%', height: '6px', appearance: 'none', WebkitAppearance: 'none',
                background: `linear-gradient(to right, var(--brand) 0%, var(--brand) ${pennySlider}%, var(--border) ${pennySlider}%, var(--border) 100%)`,
                borderRadius: '3px', outline: 'none', cursor: 'pointer',
                padding: '10px 0', margin: 0,
              }}
            />
            <p style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginTop: '8px', textAlign: 'center' as const }}>
              {pennySlider <= 30
                ? 'Penny sticks to your usual picks'
                : pennySlider <= 60
                  ? 'Penny suggests a mix of favorites and new finds'
                  : 'Penny loves trying new brands and products'}
            </p>
          </div>

        </div>
      </div>
      {renderToast()}
      {renderBottomTabs()}
    </div>
    </ScreenTransition>
  )
}
