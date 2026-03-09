import { PennyAvatar } from './PennyAvatar'
import { FONT, FONT_BASE, text, buttons } from '../styles'

interface JoinPromptProps {
  name: string
  code: string
  onJoined: (message: string) => void
  onError: (message: string) => void
  onClose: () => void
}

export function JoinPrompt({ name, code, onJoined, onError, onClose }: JoinPromptProps) {
  return (
    <div role="presentation" style={{ position: 'fixed', inset: 0, zIndex: 9999, backgroundColor: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '20px' }}>
      <div role="dialog" aria-modal="true" aria-label="Join shared list" style={{ backgroundColor: 'var(--surface)', borderRadius: '16px', padding: '32px 24px', maxWidth: '360px', width: '100%', textAlign: 'center' }}>
        <PennyAvatar size={64} />
        <h2 style={{ fontFamily: FONT, fontSize: '1.25rem', fontWeight: 700, color: 'var(--text)', margin: '16px 0 8px' }}>
          {name ? `${name} wants to shop together!` : 'Shop together!'}
        </h2>
        <p style={{ ...text.secondary, margin: '0 0 24px', lineHeight: 1.5 }}>
          You'll share one shopping list — items either of you add will show up for both of you.
        </p>
        <button
          onClick={async () => {
            try {
              const res = await fetch('/api/join', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ code }),
              })
              const data = await res.json().catch(() => null)
              if (res.ok) {
                onJoined(data?.penny_message || `You joined ${name ? name + "'s" : 'the shared'} list!`)
              } else {
                onError(data?.error || 'Could not join — the invite may have expired')
              }
            } catch {
              onError('Could not join right now')
            }
            onClose()
          }}
          style={{ backgroundColor: 'var(--brand)', color: '#fff', border: 'none', borderRadius: '10px', padding: '14px 0', fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, cursor: 'pointer', width: '100%', marginBottom: '12px' }}
        >
          Shop together{name ? ` with ${name}` : ''}
        </button>
        <button
          onClick={onClose}
          style={{ ...buttons.reset, ...text.secondary, padding: '8px' }}
        >
          Start my own list
        </button>
      </div>
    </div>
  )
}
