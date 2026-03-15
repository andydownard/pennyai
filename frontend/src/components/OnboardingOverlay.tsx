import { useState } from 'react'
import { FONT, FONT_BASE, FONT_SM, FONT_LG, FONT_XS, BRAND } from '../styles'

interface OnboardingOverlayProps {
  onDismiss: () => void
}

const steps = [
  {
    emoji: '\u{1F4DD}',
    title: 'Just type what you need',
    desc: 'Tell Penny your grocery items \u2014 "milk, eggs, bread" \u2014 and she\'ll add them to your list.',
  },
  {
    emoji: '\u{1F3EA}',
    title: 'Penny organizes by aisle',
    desc: 'She sorts everything by department so you can shop without backtracking.',
  },
  {
    emoji: '\u{1F468}\u200D\u{1F469}\u200D\u{1F467}\u200D\u{1F466}',
    title: 'Shop together',
    desc: 'Invite your family so everyone stays in sync \u2014 no more "did you get milk?"',
  },
]

export function OnboardingOverlay({ onDismiss }: OnboardingOverlayProps) {
  const [step, setStep] = useState(0)

  const handleDismiss = () => {
    localStorage.setItem('onboarding_complete', '1')
    onDismiss()
  }

  const handleNext = () => {
    if (step < steps.length - 1) {
      setStep(step + 1)
    } else {
      handleDismiss()
    }
  }

  const current = steps[step]

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 9999,
        backgroundColor: 'rgba(0,0,0,0.5)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        animation: 'onboardingFadeIn 0.3s ease',
        fontFamily: FONT,
      }}
    >
      <style>{`
        @keyframes onboardingFadeIn {
          from { opacity: 0; }
          to { opacity: 1; }
        }
      `}</style>
      <div
        style={{
          maxWidth: '320px',
          width: 'calc(100% - 48px)',
          padding: '32px 24px',
          borderRadius: '20px',
          backgroundColor: 'var(--surface)',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          textAlign: 'center',
        }}
      >
        {/* Step indicator dots */}
        <div style={{ display: 'flex', gap: '8px', marginBottom: '24px' }}>
          {steps.map((_, i) => (
            <div
              key={i}
              style={{
                width: '8px',
                height: '8px',
                borderRadius: '50%',
                backgroundColor: i === step ? 'var(--brand)' : 'var(--border)',
                transition: 'background-color 0.2s ease',
              }}
            />
          ))}
        </div>

        {/* Emoji */}
        <div style={{ fontSize: '48px', marginBottom: '16px', lineHeight: 1 }}>
          {current.emoji}
        </div>

        {/* Title */}
        <h2
          style={{
            fontFamily: FONT,
            fontSize: FONT_LG,
            fontWeight: 700,
            color: 'var(--text)',
            margin: '0 0 8px 0',
          }}
        >
          {current.title}
        </h2>

        {/* Description */}
        <p
          style={{
            fontFamily: FONT,
            fontSize: FONT_SM,
            color: 'var(--text-secondary)',
            margin: '0 0 28px 0',
            lineHeight: 1.5,
          }}
        >
          {current.desc}
        </p>

        {/* Next / Get started button */}
        <button
          onClick={handleNext}
          style={{
            width: '100%',
            padding: '14px',
            fontFamily: FONT,
            fontSize: FONT_BASE,
            fontWeight: 600,
            color: '#FFFFFF',
            backgroundColor: BRAND,
            border: 'none',
            borderRadius: '12px',
            cursor: 'pointer',
          }}
        >
          {step < steps.length - 1 ? 'Next' : 'Get started'}
        </button>

        {/* Skip link */}
        <button
          onClick={handleDismiss}
          style={{
            fontFamily: FONT,
            fontSize: FONT_XS,
            color: 'var(--text-secondary)',
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            padding: '12px 0 0 0',
          }}
        >
          Skip
        </button>
      </div>
    </div>
  )
}
