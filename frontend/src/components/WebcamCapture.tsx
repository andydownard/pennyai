import { useRef, useEffect, useState, useCallback } from 'react'
import { FONT, FONT_BASE, FONT_SM, buttons } from '../styles'

interface WebcamCaptureProps {
  onCapture: (dataUrl: string) => void
  onClose: () => void
}

export function WebcamCapture({ onCapture, onClose }: WebcamCaptureProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    let cancelled = false
    async function startCamera() {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: 'environment', width: { ideal: 1280 }, height: { ideal: 960 } },
          audio: false,
        })
        if (cancelled) { stream.getTracks().forEach(t => t.stop()); return }
        streamRef.current = stream
        if (videoRef.current) {
          videoRef.current.srcObject = stream
          videoRef.current.onloadedmetadata = () => { setReady(true) }
        }
      } catch (err: unknown) {
        if (cancelled) return
        if (err instanceof DOMException && err.name === 'NotAllowedError') {
          setError('Camera access denied. Please allow camera access in your browser settings.')
        } else if (err instanceof DOMException && err.name === 'NotFoundError') {
          setError('No camera found on this device.')
        } else {
          setError('Could not start camera. Please try again.')
        }
      }
    }
    startCamera()
    return () => {
      cancelled = true
      streamRef.current?.getTracks().forEach(t => t.stop())
    }
  }, [])

  const capture = useCallback(() => {
    const video = videoRef.current
    const canvas = canvasRef.current
    if (!video || !canvas) return
    canvas.width = video.videoWidth
    canvas.height = video.videoHeight
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    ctx.drawImage(video, 0, 0)
    const dataUrl = canvas.toDataURL('image/jpeg', 0.85)
    // Stop the stream before calling back
    streamRef.current?.getTracks().forEach(t => t.stop())
    onCapture(dataUrl)
  }, [onCapture])

  const handleClose = useCallback(() => {
    streamRef.current?.getTracks().forEach(t => t.stop())
    onClose()
  }, [onClose])

  return (
    <div
      style={{
        position: 'fixed', inset: 0, zIndex: 10000,
        backgroundColor: 'rgba(0,0,0,0.85)',
        display: 'flex', flexDirection: 'column',
        alignItems: 'center', justifyContent: 'center',
      }}
      onClick={handleClose}
    >
      <div
        style={{
          backgroundColor: 'var(--surface)',
          borderRadius: '16px',
          overflow: 'hidden',
          maxWidth: '640px',
          width: '90vw',
          boxShadow: '0 8px 32px rgba(0,0,0,0.3)',
        }}
        onClick={e => e.stopPropagation()}
      >
        {error ? (
          <div style={{
            padding: '40px 24px',
            textAlign: 'center',
            display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '16px',
          }}>
            <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
              <line x1="1" y1="1" x2="23" y2="23"/>
            </svg>
            <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text)', margin: 0 }}>
              {error}
            </p>
            <button
              onClick={handleClose}
              style={{
                ...buttons.primary,
                backgroundColor: 'var(--brand)', color: '#fff',
                border: 'none', borderRadius: '10px',
                padding: '10px 24px', fontFamily: FONT, fontSize: FONT_SM,
                fontWeight: 600, cursor: 'pointer',
              }}
            >
              Close
            </button>
          </div>
        ) : (
          <>
            <div style={{ position: 'relative', backgroundColor: '#000' }}>
              <video
                ref={videoRef}
                autoPlay
                playsInline
                muted
                style={{
                  width: '100%',
                  display: 'block',
                  maxHeight: '60vh',
                  objectFit: 'cover',
                }}
              />
              {!ready && (
                <div style={{
                  position: 'absolute', inset: 0,
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  color: '#fff', fontFamily: FONT, fontSize: FONT_SM,
                }}>
                  Starting camera...
                </div>
              )}
            </div>
            <div style={{
              padding: '16px 24px',
              display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '16px',
            }}>
              <button
                onClick={handleClose}
                style={{
                  background: 'none', border: '1px solid var(--border)',
                  borderRadius: '10px', padding: '10px 20px',
                  fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text)',
                  cursor: 'pointer',
                }}
              >
                Cancel
              </button>
              <button
                onClick={capture}
                disabled={!ready}
                style={{
                  backgroundColor: 'var(--brand)', color: '#fff',
                  border: 'none', borderRadius: '10px',
                  padding: '10px 24px', fontFamily: FONT, fontSize: FONT_SM,
                  fontWeight: 600, cursor: 'pointer',
                  opacity: ready ? 1 : 0.5,
                  display: 'flex', alignItems: 'center', gap: '8px',
                }}
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
                  <circle cx="12" cy="13" r="4"/>
                </svg>
                Capture
              </button>
            </div>
          </>
        )}
      </div>
      <canvas ref={canvasRef} style={{ display: 'none' }} />
    </div>
  )
}
