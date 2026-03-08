import React from 'react'
import { PennyAvatar } from './PennyAvatar'
import {
  BRAND, BG, TEXT, TEXT_SECONDARY, BORDER, WHITE, FONT,
  ERROR_RED_LIGHT, ERROR_RED_DARK,
} from '../styles'

// Brand disambiguation item from the parse response
export interface DisambigItem {
  product_category: string
  current_brand: string
  alternatives: string[]
}

interface PennyBarProps {
  // Penny text display
  pennyText: string
  pennyTyping: boolean
  pennyStatus: boolean
  buttons: Array<{ label: string; onClick: () => void }>
  // Toast state (affects background color)
  toastError: boolean
  // Keyboard
  keyboardOpen: boolean
  // Brand disambiguation
  disambig: DisambigItem[]
  disambigIndex: number
  onBrandPick: (brand: string) => void
  onSkipDisambig: () => void
  // Photo thumbnails
  pendingImages: string[]
  onRemoveImage: (index: number) => void
  // Chat input
  chatText: string
  onChatTextChange: (text: string) => void
  chatLoading: boolean
  onSend: () => void
  onPaste: (e: React.ClipboardEvent) => void
  onAddPhoto: () => void
  // Refs (forwarded)
  chatInputRef: React.RefObject<HTMLTextAreaElement | null>
}

const btnStyle: React.CSSProperties = {
  background: BRAND, border: 'none', color: WHITE, fontFamily: FONT,
  fontSize: '12px', fontWeight: 600, cursor: 'pointer',
  padding: '5px 12px', borderRadius: '12px', flexShrink: 0,
}

const chatBarStyle: React.CSSProperties = {
  flexShrink: 0,
  padding: '8px 8px 8px 4px',
  paddingBottom: '8px',
  backgroundColor: WHITE,
  borderTop: '1.5px solid #D4CFC7',
  boxShadow: '0 -2px 12px rgba(0,0,0,0.06)',
}

const chatPillStyle: React.CSSProperties = {
  flex: 1, display: 'flex', alignItems: 'flex-end',
  backgroundColor: BG, border: '1.5px solid #C5C0B8',
  borderRadius: '24px', overflow: 'hidden', minWidth: 0,
}

const chatInputStyle: React.CSSProperties = {
  flex: 1, padding: '10px 14px', fontSize: '16px', fontFamily: FONT,
  color: TEXT, backgroundColor: 'transparent', border: 'none', outline: 'none', minWidth: 0,
}

const chatSendStyle = (active: boolean): React.CSSProperties => ({
  flexShrink: 0, width: '40px', height: '40px', borderRadius: '50%',
  backgroundColor: 'transparent', color: active ? BRAND : TEXT_SECONDARY,
  border: 'none', cursor: active ? 'pointer' : 'default',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  transition: 'color 0.15s',
})

export function PennyBar({
  pennyText, pennyTyping, pennyStatus, buttons, toastError,
  keyboardOpen, disambig, disambigIndex, onBrandPick, onSkipDisambig,
  pendingImages, onRemoveImage,
  chatText, onChatTextChange, chatLoading, onSend, onPaste, onAddPhoto,
  chatInputRef,
}: PennyBarProps) {
  const hasDisambig = disambig.length > 0 && disambigIndex < disambig.length
  const bgColor = toastError ? ERROR_RED_LIGHT : '#F5F3EF'
  const textColor = toastError ? ERROR_RED_DARK : TEXT
  const canSend = (!!chatText.trim() || pendingImages.length > 0) && !chatLoading

  return (
    <>
      {/* Penny text + brand disambiguation */}
      <div style={{ flexShrink: 0, backgroundColor: bgColor, borderTop: `1px solid ${BORDER}`, overflow: 'hidden' }}>
        <div style={{ padding: keyboardOpen ? '6px 12px' : '12px 16px', transition: 'padding 200ms ease' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: keyboardOpen ? '8px' : '10px' }}>
            <PennyAvatar size={keyboardOpen ? 28 : 36} />
            <span style={{
              flex: 1, fontFamily: FONT, fontSize: keyboardOpen ? '13px' : '14px',
              color: textColor, fontWeight: 500,
              display: '-webkit-box' as const, WebkitLineClamp: 3,
              WebkitBoxOrient: 'vertical' as const, overflow: 'hidden',
              minHeight: keyboardOpen ? undefined : '20px', transition: 'font-size 200ms ease',
            }}>
              {hasDisambig
                ? (() => { const d = disambig[disambigIndex]; return d.current_brand ? `${d.product_category} (${d.current_brand}) — which brand?` : `${d.product_category} — which brand?` })()
                : pennyText}
              {!hasDisambig && pennyTyping && <span className="cursor-blink" style={{ fontWeight: 300, marginLeft: '1px' }}>|</span>}
            </span>
            {disambig.length > 1 && hasDisambig && keyboardOpen && (
              <span style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY, flexShrink: 0 }}>{disambigIndex + 1}/{disambig.length}</span>
            )}
            {!hasDisambig && pennyStatus && <span className="penny-pulse" style={{ width: '6px', height: '6px', borderRadius: '50%', backgroundColor: BRAND, flexShrink: 0 }} />}
          </div>
          {/* Screen reader gets full text after typing completes */}
          {!hasDisambig && !pennyTyping && pennyText && (
            <span className="sr-only" aria-live="polite">{pennyText}</span>
          )}
          {!hasDisambig && !keyboardOpen && !pennyTyping && buttons.length > 0 && (
            <div style={{ display: 'flex', gap: '8px', marginTop: '8px', paddingLeft: keyboardOpen ? '36px' : '46px' }}>
              {buttons.map((btn, i) => (
                <button key={i} onClick={btn.onClick} style={btnStyle}>{btn.label}</button>
              ))}
            </div>
          )}
        </div>
        {hasDisambig && (() => {
          const d = disambig[disambigIndex]
          return (
            <div style={{ position: 'relative' }}>
              <div style={{ display: 'flex', flexWrap: 'nowrap', overflowX: 'auto', gap: '6px', padding: keyboardOpen ? '4px 12px 6px 42px' : '4px 16px 10px 56px', WebkitOverflowScrolling: 'touch' as any, scrollbarWidth: 'none' as any, msOverflowStyle: 'none' as any }}>
                {d.current_brand && !d.alternatives.includes(d.current_brand) && (
                  <button onClick={() => onBrandPick(d.current_brand)} style={{ ...btnStyle, padding: keyboardOpen ? '4px 10px' : '5px 12px', flexShrink: 0, boxShadow: `0 0 0 2px ${BRAND}` }}>{d.current_brand}</button>
                )}
                {d.alternatives.filter(a => a !== '').map((alt, i) => (
                  <button key={i} onClick={() => onBrandPick(alt)} style={{
                    ...btnStyle,
                    padding: keyboardOpen ? '4px 10px' : '5px 12px',
                    flexShrink: 0,
                    ...(alt === d.current_brand ? { boxShadow: `0 0 0 2px ${BRAND}` } : { background: 'transparent', color: TEXT, border: `1px solid ${BORDER}` }),
                  }}>{alt}</button>
                ))}
                <button onClick={onSkipDisambig} aria-label="Skip brand selection" style={{ ...btnStyle, background: 'transparent', color: TEXT_SECONDARY, border: `1px solid ${BORDER}`, padding: keyboardOpen ? '4px 10px' : '5px 12px', flexShrink: 0 }}>Skip</button>
                {disambig.length > 1 && !keyboardOpen && (
                  <span style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY, alignSelf: 'center', paddingLeft: '4px', flexShrink: 0 }}>{disambigIndex + 1}/{disambig.length}</span>
                )}
              </div>
              <div style={{ position: 'absolute', right: 0, top: 0, bottom: 0, width: '28px', background: `linear-gradient(to right, transparent, ${bgColor})`, pointerEvents: 'none' }} />
            </div>
          )
        })()}
      </div>

      {/* Photo thumbnail strip */}
      {pendingImages.length > 0 && (
        <div style={{ display: 'flex', gap: '8px', padding: '8px 12px', overflowX: 'auto', backgroundColor: WHITE, borderTop: `1px solid ${BORDER}`, flexShrink: 0 }}>
          {pendingImages.map((img, idx) => (
            <div key={idx} style={{ position: 'relative', flexShrink: 0 }}>
              <img src={img} alt={`Preview ${idx + 1}`} style={{ width: '56px', height: '56px', borderRadius: '8px', objectFit: 'cover' }} />
              <button
                aria-label={`Remove image ${idx + 1}`}
                onClick={() => onRemoveImage(idx)}
                style={{
                  position: 'absolute', top: '-6px', right: '-6px',
                  width: '18px', height: '18px', borderRadius: '50%',
                  backgroundColor: TEXT, color: WHITE, border: 'none',
                  fontSize: '12px', lineHeight: '18px', textAlign: 'center',
                  cursor: 'pointer', padding: 0,
                }}
              >×</button>
            </div>
          ))}
        </div>
      )}

      {/* Chat input bar */}
      <div style={chatBarStyle}>
        <div style={{ display: 'flex', alignItems: 'flex-end', gap: '4px' }}>
          <button
            title="Add photo"
            aria-label="Add photo"
            onMouseDown={e => { e.preventDefault(); onAddPhoto() }}
            onTouchEnd={e => { e.preventDefault(); onAddPhoto() }}
            style={{ flexShrink: 0, width: '44px', height: '44px', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke={BRAND} strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          </button>
          <div style={chatPillStyle}>
            <textarea
              ref={chatInputRef as React.RefObject<HTMLTextAreaElement>}
              rows={1}
              enterKeyHint="send"
              aria-label={hasDisambig ? "Type a brand name" : "Tell Penny what you need"}
              placeholder={hasDisambig ? "Or type a brand name\u2026" : "Tell Penny what you need\u2026"}
              value={chatText}
              onChange={e => {
                onChatTextChange(e.target.value)
                e.target.style.height = 'auto'
                e.target.style.height = Math.min(e.target.scrollHeight, 120) + 'px'
              }}
              onKeyDown={e => {
                if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); onSend() }
              }}
              onPaste={onPaste}
              disabled={chatLoading}
              style={{ ...chatInputStyle, resize: 'none' as const, overflowY: 'hidden' as const, maxHeight: '120px' }}
            />
            <button
              onClick={onSend}
              disabled={!canSend}
              aria-label="Send message"
              style={chatSendStyle(canSend)}
            >
              {chatLoading
                ? <span style={{ fontSize: '12px', fontFamily: FONT }}>…</span>
                : <svg width="18" height="18" viewBox="0 0 20 20" fill="none">
                    <path d="M18 2L9 11M18 2l-6 16-3-7-7-3 16-6z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
              }
            </button>
          </div>
        </div>
      </div>
    </>
  )
}
