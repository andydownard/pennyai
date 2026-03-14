import { useRef, useCallback } from 'react'
import { QRCodeCanvas } from 'qrcode.react'
import { ScreenTransition } from './ScreenTransition'
import { ConfirmModal } from './ConfirmModal'
import { FONT, FONT_2XS, FONT_XS, FONT_BASE, FONT_SM, formatPhone, pageStyles, listStyles, BRAND } from '../styles'

interface SettingsMember {
  name: string
  phone: string
}

interface ShareScreenProps {
  referralCode: string
  referralFriends: { name: string; created_at: string }[]
  settingsMembers: SettingsMember[]
  inviteLoading: boolean
  confirmModal: { title: string; message?: string; confirmLabel?: string; destructive?: boolean } | null
  onInvite: () => void
  onLeaveHousehold: () => void
  onShareList: () => void
  onShareReferral: () => void
  onCopyReferral: () => void
  onShowToast: (msg: string, type?: 'success' | 'error') => void
  runConfirmAction: () => void
  closeConfirmModal: () => void
  renderHeader: () => React.ReactNode
  renderToast: () => React.ReactNode
  renderBottomTabs: () => React.ReactNode
  transitionStyle: React.CSSProperties
}

const avatarCircle = (size = 32): React.CSSProperties => ({
  width: `${size}px`, height: `${size}px`, borderRadius: '50%', backgroundColor: 'var(--brand)', color: '#fff',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 600, flexShrink: 0,
})

export function ShareScreen({
  referralCode, referralFriends, settingsMembers, inviteLoading,
  confirmModal, onInvite, onLeaveHousehold, onShareList, onShareReferral, onCopyReferral,
  onShowToast, runConfirmAction, closeConfirmModal,
  renderHeader, renderToast, renderBottomTabs, transitionStyle,
}: ShareScreenProps) {
  const qrCanvasRef = useRef<HTMLDivElement>(null)

  const referralUrl = `https://app.thrift.ly/r/${referralCode}?via=qr`

  const handleSaveQR = useCallback(async () => {
    // Build a branded 1000x1240 PNG with QR code centered
    const canvas = document.createElement('canvas')
    const w = 1000, h = 1240
    canvas.width = w
    canvas.height = h
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    // Background
    ctx.fillStyle = '#F7F6F3'
    ctx.fillRect(0, 0, w, h)

    // Brand header bar
    ctx.fillStyle = BRAND
    ctx.fillRect(0, 0, w, 100)

    // Header text
    ctx.fillStyle = '#FFFFFF'
    ctx.font = 'bold 36px -apple-system, "Segoe UI", sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText('Thriftly', w / 2, 62)

    // "Scan to try Penny AI" subtitle
    ctx.fillStyle = '#2A2A2A'
    ctx.font = 'bold 32px -apple-system, "Segoe UI", sans-serif'
    ctx.fillText('Scan to try Penny AI', w / 2, 170)

    ctx.fillStyle = '#525252'
    ctx.font = '22px -apple-system, "Segoe UI", sans-serif'
    ctx.fillText('Your AI-powered grocery list assistant', w / 2, 210)

    // Draw QR code from the on-screen canvas
    const qrCanvas = qrCanvasRef.current?.querySelector('canvas')
    if (qrCanvas) {
      const qrSize = 500
      const qrX = (w - qrSize) / 2
      const qrY = 260
      // White background behind QR
      ctx.fillStyle = '#FFFFFF'
      const pad = 40
      ctx.beginPath()
      ctx.roundRect(qrX - pad, qrY - pad, qrSize + pad * 2, qrSize + pad * 2, 20)
      ctx.fill()
      ctx.drawImage(qrCanvas, qrX, qrY, qrSize, qrSize)
    }

    // Referral URL text below QR
    ctx.fillStyle = '#525252'
    ctx.font = '20px -apple-system, "Segoe UI", sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText(`app.thrift.ly/r/${referralCode}`, w / 2, 860)

    // Earn message
    ctx.fillStyle = BRAND
    ctx.font = 'bold 26px -apple-system, "Segoe UI", sans-serif'
    ctx.fillText('Earn points for every friend who joins!', w / 2, 920)

    // Footer
    ctx.fillStyle = BRAND
    ctx.fillRect(0, h - 80, w, 80)
    ctx.fillStyle = '#FFFFFF'
    ctx.font = '20px -apple-system, "Segoe UI", sans-serif'
    ctx.fillText('thrift.ly', w / 2, h - 35)

    // Export
    const blob = await new Promise<Blob | null>(resolve => canvas.toBlob(resolve, 'image/png'))
    if (!blob) return

    // Try native share (mobile), fall back to download
    if (navigator.share && navigator.canShare?.({ files: [new File([blob], 'penny-referral.png', { type: 'image/png' })] })) {
      const file = new File([blob], 'penny-referral.png', { type: 'image/png' })
      navigator.share({ files: [file], title: 'Try Penny AI' }).catch(() => {})
    } else {
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'penny-referral-qr.png'
      a.click()
      URL.revokeObjectURL(url)
      onShowToast('QR code saved!')
    }
  }, [referralCode, onShowToast])

  const handleEmailInvite = useCallback(() => {
    const subject = encodeURIComponent('Try Penny AI — your AI grocery list assistant')
    const body = encodeURIComponent(
      `Hey!\n\nI've been using Penny AI to manage my grocery list — she organizes everything, suggests brands, and keeps the whole family in sync.\n\nTry it out: https://app.thrift.ly/r/${referralCode}\n\nJust text, email, or type your list — Penny handles the rest!`
    )
    window.open(`mailto:?subject=${subject}&body=${body}`, '_self')
  }, [referralCode])

  return (
    <ScreenTransition style={transitionStyle}>
    <div style={pageStyles.page}>
      <ConfirmModal open={!!confirmModal} title={confirmModal?.title || ''} message={confirmModal?.message} confirmLabel={confirmModal?.confirmLabel} destructive={confirmModal?.destructive} onConfirm={runConfirmAction} onCancel={closeConfirmModal} />
      {renderHeader()}
      <div style={pageStyles.listArea}>
        <div style={{ padding: '20px', display: 'flex', flexDirection: 'column', gap: '40px' }}>

          {/* Refer a Friend */}
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/></svg>
              <span style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 700, color: 'var(--text)', textTransform: 'uppercase' as const, letterSpacing: '0.5px' }}>Refer a Friend</span>
            </div>

            <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text)', margin: '0 0 4px 0', lineHeight: 1.6 }}>
              Earn points for every friend who joins Penny.
            </p>
            <p style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', margin: '0 0 20px 0', lineHeight: 1.5 }}>
              Share your code below — friends scan with their phone camera to sign up.
            </p>

            {/* QR Code */}
            <div style={{
              display: 'flex', flexDirection: 'column', alignItems: 'center',
              backgroundColor: 'var(--surface)', borderRadius: '16px',
              border: '1px solid var(--border)', padding: '24px 20px', marginBottom: '16px',
            }}>
              <div ref={qrCanvasRef}>
                <QRCodeCanvas
                  value={referralUrl}
                  size={200}
                  level="M"
                  marginSize={2}
                  fgColor={BRAND}
                  bgColor="transparent"
                />
              </div>
              <p style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', marginTop: '12px', textAlign: 'center' }}>
                Friends scan this with their phone camera to join.
              </p>
            </div>

            {/* Link + copy */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '10px 14px', backgroundColor: 'var(--bg)', borderRadius: '10px', border: '1px solid var(--border)', marginBottom: '12px' }}>
              <span style={{ flex: 1, fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text)', wordBreak: 'break-all' as const }}>{`https://app.thrift.ly/r/${referralCode}`}</span>
              <button
                onClick={onCopyReferral}
                style={{ padding: '6px 12px', fontFamily: FONT, fontSize: FONT_2XS, fontWeight: 600, color: 'var(--brand)', backgroundColor: 'transparent', border: '1px solid var(--brand)', borderRadius: '8px', cursor: 'pointer', whiteSpace: 'nowrap' as const }}
              >Copy</button>
            </div>

            {/* Action buttons */}
            <div style={{ display: 'flex', gap: '8px', marginBottom: '8px' }}>
              <button onClick={onShareReferral} style={{ ...listStyles.sheetButton, flex: 1 }}>
                Share link
              </button>
              <button onClick={handleEmailInvite} style={{
                ...listStyles.sheetButton, flex: 1,
                backgroundColor: 'transparent', color: 'var(--brand)',
                border: '1.5px solid var(--brand)',
              }}>
                Email a friend
              </button>
            </div>

            <button onClick={handleSaveQR} style={{
              width: '100%', padding: '12px', fontFamily: FONT, fontSize: FONT_SM,
              fontWeight: 600, color: 'var(--text-secondary)', backgroundColor: 'transparent',
              border: '1px solid var(--border)', borderRadius: '12px', cursor: 'pointer',
              display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '6px',
            }}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
              Save QR as image
            </button>

            {referralFriends.length > 0 && (
              <p style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', marginTop: '12px' }}>
                {referralFriends.length} {referralFriends.length === 1 ? 'friend' : 'friends'} joined
              </p>
            )}
          </div>

          {/* Shared List section */}
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
              <span style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 700, color: 'var(--text)', textTransform: 'uppercase' as const, letterSpacing: '0.5px' }}>Shared List</span>
            </div>
            {settingsMembers.length > 0 && settingsMembers.map((m, i) => (
              <div key={i} style={{
                display: 'flex', alignItems: 'center', padding: '10px 0',
                borderBottom: '1px solid var(--border)', gap: '10px',
              }}>
                <div style={avatarCircle()}>
                  {(m.name || m.phone || '?')[0].toUpperCase()}
                </div>
                <div>
                  <div style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text)' }}>
                    {m.name || formatPhone(m.phone)}
                  </div>
                  {m.name && <div style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)' }}>{formatPhone(m.phone)}</div>}
                </div>
              </div>
            ))}
            <button
              onClick={onInvite}
              disabled={inviteLoading}
              style={{
                ...listStyles.sheetButton,
                marginTop: '12px',
                opacity: inviteLoading ? 0.5 : 1,
              }}
            >
              {inviteLoading ? 'Creating link…' : 'Invite someone to your shared grocery list'}
            </button>
            <p style={{ fontFamily: FONT, fontSize: FONT_XS, color: 'var(--text-secondary)', marginTop: '8px', lineHeight: 1.5 }}>
              Everyone on your list shops together in real time
            </p>
            {settingsMembers.length > 0 && (<>
              <button
                onClick={onLeaveHousehold}
                style={{
                  marginTop: '8px', padding: 0, fontFamily: FONT, fontSize: FONT_XS, fontWeight: 500,
                  color: 'var(--error)', backgroundColor: 'transparent', border: 'none', cursor: 'pointer',
                  textAlign: 'left' as const,
                }}
              >Leave shared list</button>
              <div style={{ fontFamily: FONT, fontSize: FONT_2XS, color: 'var(--text-secondary)', marginTop: '4px' }}>You'll still have your own list — you'll just stop sharing with this group.</div>
            </>)}
          </div>

          {/* Share a List (read-only link) */}
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8"/><polyline points="16 6 12 2 8 6"/><line x1="12" y1="2" x2="12" y2="15"/></svg>
              <span style={{ fontFamily: FONT, fontSize: FONT_BASE, fontWeight: 700, color: 'var(--text)', textTransform: 'uppercase' as const, letterSpacing: '0.5px' }}>Share a List</span>
            </div>
            <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text)', margin: '0 0 16px 0', lineHeight: 1.6 }}>
              Send a read-only snapshot of your list to anyone — no account needed. Link expires in 7 days.
            </p>
            <button onClick={onShareList} style={listStyles.sheetButton}>
              Share my list for 7 days
            </button>
          </div>

        </div>
      </div>
      {renderToast()}
      {renderBottomTabs()}
    </div>
    </ScreenTransition>
  )
}
