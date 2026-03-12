// Uses CSS custom properties for dark mode reactivity

export type PennyMood = 'default' | 'thinking' | 'happy' | 'excited'

const moodStyles: Record<PennyMood, string> = {
  default: '',
  thinking: 'penny-thinking',
  happy: 'penny-happy',
  excited: 'penny-excited',
}

// Inject keyframes once
let stylesInjected = false
function injectMoodStyles() {
  if (stylesInjected) return
  stylesInjected = true
  const style = document.createElement('style')
  style.textContent = `
    @keyframes penny-pulse {
      0%, 100% { transform: scale(1); opacity: 1; }
      50% { transform: scale(0.92); opacity: 0.7; }
    }
    @keyframes penny-glow {
      0% { box-shadow: 0 0 0 0 rgba(53,100,79,0.3); transform: scale(1); }
      50% { box-shadow: 0 0 12px 4px rgba(53,100,79,0.15); transform: scale(1.05); }
      100% { box-shadow: 0 0 0 0 rgba(53,100,79,0); transform: scale(1); }
    }
    @keyframes penny-bounce {
      0%, 100% { transform: translateY(0); }
      25% { transform: translateY(-3px); }
      50% { transform: translateY(0); }
      75% { transform: translateY(-2px); }
    }
    .penny-thinking { animation: penny-pulse 1.2s ease-in-out infinite; }
    .penny-happy { animation: penny-glow 0.8s ease-out forwards; }
    .penny-excited { animation: penny-bounce 0.5s ease-in-out 2; }
    @media (prefers-reduced-motion: reduce) {
      .penny-thinking, .penny-happy, .penny-excited { animation: none; }
    }
  `
  document.head.appendChild(style)
}

export function PennyAvatar({ size = 36, mood = 'default' as PennyMood }: { size?: number; mood?: PennyMood }) {
  injectMoodStyles()
  const isLarge = size > 80
  return (
    <div
      className={moodStyles[mood] || ''}
      style={{
        width: size,
        height: size,
        borderRadius: '50%',
        overflow: 'hidden',
        flexShrink: 0,
        backgroundColor: 'var(--skeleton)',
        border: isLarge ? '2px solid var(--border)' : '1px solid var(--border)',
        boxShadow: isLarge ? '0 4px 20px rgba(0,0,0,0.08)' : 'none',
      }}>
      <img
        src="/penny.png?v=2"
        alt="Penny"
        width={size}
        height={size}
        style={{ objectFit: 'cover', objectPosition: 'center 20%', display: 'block' }}
      />
    </div>
  )
}
