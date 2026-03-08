import { useState, useEffect } from 'react'
import { PennyAvatar } from './PennyAvatar'
import {
  BRAND, BG, TEXT, TEXT_SECONDARY, BORDER, WHITE, FONT,
} from '../styles'

interface NPSSurveyModalProps {
  onClose: () => void
  onSubmit: (score: number, comment: string) => void
}

export function NPSSurveyModal({ onClose, onSubmit }: NPSSurveyModalProps) {
  const [score, setScore] = useState<number | null>(null)
  const [step, setStep] = useState<1 | 2>(1)
  const [comment, setComment] = useState('')
  const [typedText, setTypedText] = useState('')
  const [typingDone, setTypingDone] = useState(false)

  // Typewriter effect for NPS questions
  useEffect(() => {
    const question = step === 1
      ? 'How likely are you to recommend Thriftly to a friend?'
      : `You picked ${score}! What's the biggest reason for your score?`
    setTypedText('')
    setTypingDone(false)
    let i = 0
    const timer = setInterval(() => {
      i++
      setTypedText(question.slice(0, i))
      if (i >= question.length) {
        clearInterval(timer)
        setTypingDone(true)
      }
    }, 25)
    return () => clearInterval(timer)
  }, [step, score])

  return (
    <div onClick={onClose} role="presentation" style={{
      position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
      backgroundColor: 'rgba(0,0,0,0.5)', zIndex: 1000,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
    }}>
      <div onClick={e => e.stopPropagation()} role="dialog" aria-modal="true" aria-label="NPS Survey" style={{
        backgroundColor: WHITE, borderRadius: '16px', padding: '24px',
        width: '90%', maxWidth: '360px', position: 'relative',
      }}>
        <button onClick={onClose} aria-label="Close survey" style={{
          position: 'absolute', top: '12px', right: '12px',
          background: 'none', border: 'none', fontSize: '20px', color: TEXT_SECONDARY, cursor: 'pointer', zIndex: 1,
        }}>×</button>

        {/* Penny speech bubble */}
        <div style={{ display: 'flex', gap: '12px', marginBottom: '20px', paddingRight: '24px' }}>
          <PennyAvatar />
          <div style={{
            flex: 1, backgroundColor: BG, borderRadius: '14px', padding: '12px 14px',
            fontFamily: FONT, fontSize: '15px', color: TEXT, lineHeight: '1.4', position: 'relative',
          }}>
            {typedText}{!typingDone && <span style={{ opacity: 0.4 }}>|</span>}
          </div>
        </div>

        {/* Controls appear after typing finishes */}
        {typingDone && (
          step === 1 ? (
            <>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(11, 1fr)', gap: '4px', marginBottom: '8px' }}>
                {Array.from({ length: 11 }, (_, i) => (
                  <button key={i} onClick={() => { setScore(i); setStep(2) }} aria-label={`Score ${i} out of 10`} style={{
                    height: '40px', borderRadius: '10px',
                    border: `1.5px solid ${BRAND}30`, backgroundColor: WHITE,
                    fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: BRAND,
                    cursor: 'pointer', padding: 0,
                  }}>{i}</button>
                ))}
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>
                <span>Not likely</span><span>Very likely</span>
              </div>
            </>
          ) : (
            <>
              <textarea
                value={comment}
                onChange={e => setComment(e.target.value)}
                aria-label="What's behind your score? (optional)"
                placeholder={`What's behind your score of ${score}? (optional)`}
                rows={3}
                autoFocus
                style={{
                  width: '100%', padding: '10px', borderRadius: '8px',
                  border: `1px solid ${BORDER}`, fontFamily: FONT, fontSize: '15px',
                  resize: 'vertical', marginBottom: '16px', boxSizing: 'border-box',
                }}
              />
              <button onClick={() => { if (score !== null) onSubmit(score, comment) }} style={{
                width: '100%', padding: '12px', borderRadius: '10px',
                backgroundColor: BRAND, color: WHITE, border: 'none',
                fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer',
              }}>Submit</button>
            </>
          )
        )}
      </div>
    </div>
  )
}
