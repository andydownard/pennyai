import React, { useState, useEffect, useRef, useCallback } from 'react'
import { PennyAvatar } from './PennyAvatar'
import { FONT, FONT_2XS, FONT_XS, FONT_MD, layout, buttons as btnUtils } from '../styles'

// Auto-submit debounce delay in ms
const AUTO_SUBMIT_DELAY = 1500

// SVG circle circumference for the progress ring (r=15)
const RING_CIRCUMFERENCE = 2 * Math.PI * 15 // ~94.25

// Words that indicate a command or question — do NOT auto-submit these
const COMMAND_PREFIXES = [
  'remove', 'delete', 'check', 'uncheck', 'save', 'load', 'what', 'how',
  'why', 'where', 'when', 'who', 'show', 'help', 'clear', 'undo',
  'rename', 'edit', 'change', 'update', 'set', 'create', 'list',
  'invite', 'share', 'join', 'leave', 'find', 'search', 'tell',
  'can', 'could', 'would', 'should', 'is', 'are', 'do', 'does',
]

// Inject keyframes for auto-submit ring once at module load
if (typeof document !== 'undefined' && !document.getElementById('penny-autosubmit-css')) {
  const style = document.createElement('style')
  style.id = 'penny-autosubmit-css'
  style.textContent = `
    @keyframes pennyAutoSubmitRing {
      from { stroke-dashoffset: ${RING_CIRCUMFERENCE}; }
      to   { stroke-dashoffset: 0; }
    }
  `
  document.head.appendChild(style)
}

/**
 * Returns true if the input looks like a simple grocery item that can be auto-submitted.
 * Heuristic: 1-4 words, no question marks, doesn't start with a command verb.
 */
function looksLikeSimpleItem(text: string): boolean {
  const trimmed = text.trim()
  if (!trimmed) return false
  // No question marks anywhere
  if (trimmed.includes('?')) return false
  // Must be 1-4 words
  const words = trimmed.split(/\s+/)
  if (words.length < 1 || words.length > 4) return false
  // First word must not be a command/question word
  const firstWord = words[0].toLowerCase().replace(/[^a-z]/g, '')
  if (COMMAND_PREFIXES.includes(firstWord)) return false
  // No commas (likely multiple items — let user submit manually or keep typing)
  if (trimmed.includes(',')) return false
  return true
}

// Brand disambiguation item from the parse response
export interface DisambigItem {
  product_category: string
  current_brand: string
  alternatives: string[]
  auto_assigned?: boolean
}

interface PennyBarProps {
  // Penny text display
  pennyText: string
  pennyTyping: boolean
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
  onOpenCamera: () => void
  // Auto-submit: enabled when not in special input modes (disambiguation, name input, etc.)
  autoSubmitEnabled?: boolean
  // Refs (forwarded)
  chatInputRef: React.RefObject<HTMLTextAreaElement | null>
}

const btnStyle: React.CSSProperties = {
  background: 'var(--brand)', border: 'none', color: '#fff', fontFamily: FONT,
  fontSize: FONT_2XS, fontWeight: 600, cursor: 'pointer',
  padding: '5px 12px', borderRadius: '12px', flexShrink: 0,
}

const chatBarStyle: React.CSSProperties = {
  flexShrink: 0,
  padding: '6px 6px 6px 2px',
  paddingBottom: '6px',
  backgroundColor: 'var(--surface)',
  borderTop: '1.5px solid var(--border)',
  boxShadow: '0 -2px 12px rgba(0,0,0,0.06)',
}

const chatPillStyle: React.CSSProperties = {
  flex: 1, display: 'flex', alignItems: 'flex-end',
  backgroundColor: 'var(--bg)', border: '1.5px solid var(--border)',
  borderRadius: '24px', overflow: 'hidden', minWidth: 0,
}

const chatInputStyle: React.CSSProperties = {
  flex: 1, padding: '8px 12px', fontSize: FONT_MD, fontFamily: FONT,
  color: 'var(--text)', backgroundColor: 'transparent', border: 'none', outline: 'none', minWidth: 0,
}

const chatSendStyle = (active: boolean): React.CSSProperties => ({
  ...layout.rowCenter,
  flexShrink: 0, width: '40px', height: '40px', borderRadius: '50%',
  backgroundColor: 'transparent', color: active ? 'var(--brand)' : 'var(--text-secondary)',
  border: 'none', cursor: active ? 'pointer' : 'default',
  transition: 'color 0.15s',
})

export function PennyBar({
  pennyText, pennyTyping, buttons, toastError,
  keyboardOpen, disambig, disambigIndex, onBrandPick, onSkipDisambig,
  pendingImages, onRemoveImage,
  chatText, onChatTextChange, chatLoading, onSend, onPaste, onAddPhoto, onOpenCamera,
  autoSubmitEnabled = true,
  chatInputRef,
}: PennyBarProps) {
  const [showAltsForAuto, setShowAltsForAuto] = useState(false)
  const hasDisambig = disambig.length > 0 && disambigIndex < disambig.length
  const bgColor = toastError ? 'var(--error-bg, #FEE2E2)' : 'var(--penny-bg, #F5F3EF)'
  const textColor = toastError ? 'var(--error, #DC2626)' : 'var(--text)'
  const canSend = (!!chatText.trim() || pendingImages.length > 0) && !chatLoading

  // --- Auto-submit debounce logic ---
  const autoSubmitTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [autoSubmitProgress, setAutoSubmitProgress] = useState(false)
  // Stable ref for onSend so the timer callback always calls the latest version
  const onSendRef = useRef(onSend)
  onSendRef.current = onSend

  const clearAutoSubmit = useCallback(() => {
    if (autoSubmitTimer.current) {
      clearTimeout(autoSubmitTimer.current)
      autoSubmitTimer.current = null
    }
    setAutoSubmitProgress(false)
  }, [])

  useEffect(() => {
    // Clear any pending timer whenever input changes, loading state changes, or feature is disabled
    clearAutoSubmit()

    const shouldAutoSubmit =
      autoSubmitEnabled &&
      !chatLoading &&
      !hasDisambig &&
      pendingImages.length === 0 &&
      looksLikeSimpleItem(chatText)

    if (shouldAutoSubmit) {
      setAutoSubmitProgress(true)
      autoSubmitTimer.current = setTimeout(() => {
        autoSubmitTimer.current = null
        setAutoSubmitProgress(false)
        onSendRef.current()
      }, AUTO_SUBMIT_DELAY)
    }

    return () => {
      if (autoSubmitTimer.current) {
        clearTimeout(autoSubmitTimer.current)
        autoSubmitTimer.current = null
      }
    }
  }, [chatText, chatLoading, autoSubmitEnabled, hasDisambig, pendingImages.length, clearAutoSubmit])

  // Hide Penny text when keyboard is open and no disambiguation active
  const hidePennyText = keyboardOpen && !hasDisambig && !pennyTyping

  return (
    <>
      {/* Penny text + brand disambiguation */}
      <div style={{ flexShrink: 0, backgroundColor: bgColor, borderTop: '1px solid var(--border)', overflow: 'hidden' }}>
        {!hidePennyText && (
        <div style={{ padding: keyboardOpen ? '4px 12px' : '8px 14px', transition: 'padding 200ms ease' }}>
          <div style={layout.rowGap}>
            <PennyAvatar size={keyboardOpen ? 24 : 28} mood={pennyTyping ? 'thinking' : 'default'} />
            <span style={{
              flex: 1, fontFamily: FONT, fontSize: keyboardOpen ? FONT_2XS : FONT_XS,
              color: textColor, fontWeight: 500,
              display: '-webkit-box' as const, WebkitLineClamp: 2,
              WebkitBoxOrient: 'vertical' as const, overflow: 'hidden',
              transition: 'font-size 200ms ease',
            }}>
              {hasDisambig
                ? (() => { const d = disambig[disambigIndex]; return d.auto_assigned ? `I picked ${d.current_brand} for ${d.product_category}` : d.current_brand ? `${d.product_category} (${d.current_brand}) — which brand?` : `${d.product_category} — which brand?` })()
                : pennyText}
              {!hasDisambig && pennyTyping && <span className="cursor-blink" style={{ fontWeight: 300, marginLeft: '1px' }}>|</span>}
            </span>
            {disambig.length > 1 && hasDisambig && keyboardOpen && (
              <span style={{ fontFamily: FONT, fontSize: '0.6875rem', color: 'var(--text-secondary)', flexShrink: 0 }}>{disambigIndex + 1}/{disambig.length}</span>
            )}
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
        )}
        {hasDisambig && (() => {
          const d = disambig[disambigIndex]
          const isAuto = d.auto_assigned && !showAltsForAuto
          return (
            <div style={{ position: 'relative' }}>
              <div style={{ display: 'flex', flexWrap: 'nowrap', overflowX: 'auto', gap: '6px', padding: keyboardOpen ? '4px 12px 6px 42px' : '4px 16px 10px 56px', WebkitOverflowScrolling: 'touch' as any, scrollbarWidth: 'none' as any, msOverflowStyle: 'none' as any }}>
                {isAuto ? (
                  <>
                    <button onClick={() => { setShowAltsForAuto(false); onBrandPick(d.current_brand) }} style={{ ...btnStyle, padding: keyboardOpen ? '4px 10px' : '5px 12px', flexShrink: 0 }}>OK</button>
                    <button onClick={() => setShowAltsForAuto(true)} style={{ ...btnStyle, background: 'transparent', color: 'var(--text)', border: '1px solid var(--border)', padding: keyboardOpen ? '4px 10px' : '5px 12px', flexShrink: 0 }}>Change</button>
                    {disambig.length > 1 && (
                      <span style={{ fontFamily: FONT, fontSize: '0.6875rem', color: 'var(--text-secondary)', alignSelf: 'center', paddingLeft: '4px', flexShrink: 0 }}>{disambigIndex + 1}/{disambig.length}</span>
                    )}
                  </>
                ) : (
                  <>
                    {d.current_brand && !d.alternatives.includes(d.current_brand) && (
                      <button onClick={() => { setShowAltsForAuto(false); onBrandPick(d.current_brand) }} style={{ ...btnStyle, padding: keyboardOpen ? '4px 10px' : '5px 12px', flexShrink: 0, boxShadow: '0 0 0 2px var(--brand)' }}>{d.current_brand}</button>
                    )}
                    {d.alternatives.filter(a => a !== '').map((alt, i) => (
                      <button key={i} onClick={() => { setShowAltsForAuto(false); onBrandPick(alt) }} style={{
                        ...btnStyle,
                        padding: keyboardOpen ? '4px 10px' : '5px 12px',
                        flexShrink: 0,
                        ...(alt === d.current_brand ? { boxShadow: '0 0 0 2px var(--brand)' } : { background: 'transparent', color: 'var(--text)', border: '1px solid var(--border)' }),
                      }}>{alt}</button>
                    ))}
                    <button onClick={() => { setShowAltsForAuto(false); onSkipDisambig() }} aria-label="Skip brand selection" style={{ ...btnStyle, background: 'transparent', color: 'var(--text-secondary)', border: '1px solid var(--border)', padding: keyboardOpen ? '4px 10px' : '5px 12px', flexShrink: 0 }}>Skip</button>
                    {disambig.length > 1 && !keyboardOpen && (
                      <span style={{ fontFamily: FONT, fontSize: '0.6875rem', color: 'var(--text-secondary)', alignSelf: 'center', paddingLeft: '4px', flexShrink: 0 }}>{disambigIndex + 1}/{disambig.length}</span>
                    )}
                  </>
                )}
              </div>
              <div style={{ position: 'absolute', right: 0, top: 0, bottom: 0, width: '28px', background: `linear-gradient(to right, transparent, ${bgColor})`, pointerEvents: 'none' }} />
            </div>
          )
        })()}
      </div>

      {/* Photo thumbnail strip */}
      {pendingImages.length > 0 && (
        <div style={{ display: 'flex', gap: '8px', padding: '8px 12px', overflowX: 'auto', WebkitOverflowScrolling: 'touch', backgroundColor: 'var(--surface)', borderTop: '1px solid var(--border)', flexShrink: 0 }}>
          {pendingImages.map((img, idx) => (
            <div key={idx} style={{ position: 'relative', flexShrink: 0 }}>
              <img src={img} alt={`Preview ${idx + 1}`} style={{ width: '56px', height: '56px', borderRadius: '8px', objectFit: 'cover' }} />
              <button
                aria-label={`Remove image ${idx + 1}`}
                onClick={() => onRemoveImage(idx)}
                style={{
                  position: 'absolute', top: '-6px', right: '-6px',
                  width: '18px', height: '18px', borderRadius: '50%',
                  backgroundColor: 'var(--text)', color: '#fff', border: 'none',
                  fontSize: FONT_2XS, lineHeight: '18px', textAlign: 'center',
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
            title="Take photo"
            aria-label="Take photo with camera"
            onClick={() => onOpenCamera()}
            style={{ ...btnUtils.icon, flexShrink: 0, width: '40px', height: '44px', minWidth: 'auto', minHeight: 'auto' }}
          >
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
              <circle cx="12" cy="13" r="4"/>
            </svg>
          </button>
          <button
            title="Add from photo library"
            aria-label="Add from photo library"
            onClick={() => onAddPhoto()}
            style={{ ...btnUtils.icon, flexShrink: 0, width: '36px', height: '44px', minWidth: 'auto', minHeight: 'auto' }}
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
              <circle cx="8.5" cy="8.5" r="1.5"/>
              <polyline points="21 15 16 10 5 21"/>
            </svg>
          </button>
          <div style={chatPillStyle}>
            <textarea
              ref={chatInputRef as React.RefObject<HTMLTextAreaElement>}
              rows={1}
              enterKeyHint="send"
              aria-label={hasDisambig ? "Type a brand name" : "Add items or tell Penny anything"}
              placeholder={hasDisambig ? "Or type a brand name\u2026" : "Add, edit, or ask Penny\u2026"}
              value={chatText}
              onChange={e => {
                onChatTextChange(e.target.value)
                e.target.style.height = 'auto'
                e.target.style.height = Math.min(e.target.scrollHeight, 120) + 'px'
              }}
              onKeyDown={e => {
                if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); clearAutoSubmit(); onSend() }
              }}
              onPaste={onPaste}
              disabled={chatLoading}
              style={{ ...chatInputStyle, resize: 'none' as const, overflowY: 'hidden' as const, maxHeight: '120px' }}
            />
            <button
              onClick={() => { clearAutoSubmit(); onSend() }}
              disabled={!canSend}
              aria-label="Send message"
              style={{ ...chatSendStyle(canSend), position: 'relative' }}
            >
              {/* Circular progress ring for auto-submit countdown */}
              {autoSubmitProgress && (
                <svg
                  width="36" height="36"
                  viewBox="0 0 36 36"
                  style={{ position: 'absolute', top: '2px', left: '2px', pointerEvents: 'none' }}
                >
                  <circle
                    cx="18" cy="18" r="15"
                    fill="none"
                    stroke="var(--brand)"
                    strokeWidth="2.5"
                    strokeDasharray={RING_CIRCUMFERENCE}
                    strokeDashoffset={RING_CIRCUMFERENCE}
                    strokeLinecap="round"
                    style={{
                      animation: `pennyAutoSubmitRing ${AUTO_SUBMIT_DELAY}ms linear forwards`,
                      transformOrigin: 'center',
                      transform: 'rotate(-90deg)',
                      opacity: 0.5,
                    }}
                  />
                </svg>
              )}
              {chatLoading
                ? <span style={{ fontSize: FONT_2XS, fontFamily: FONT }}>…</span>
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
