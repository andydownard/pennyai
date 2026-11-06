import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

vi.mock('qrcode.react', () => ({
  QRCodeCanvas: ({ value }: { value: string }) => <canvas data-testid="qr" data-value={value} />,
}))

import { ShareScreen } from './ShareScreen'

function setup(over: Partial<React.ComponentProps<typeof ShareScreen>> = {}) {
  const props: React.ComponentProps<typeof ShareScreen> = {
    referralCode: 'ABC123',
    referralFriends: [],
    settingsMembers: [],
    inviteLoading: false,
    confirmModal: null,
    onInvite: vi.fn(),
    onLeaveHousehold: vi.fn(),
    onShareList: vi.fn(),
    onShareReferral: vi.fn(),
    onCopyReferral: vi.fn(),
    onShowToast: vi.fn(),
    runConfirmAction: vi.fn(),
    closeConfirmModal: vi.fn(),
    renderHeader: () => <header>HEADER</header>,
    renderToast: () => <div>TOAST</div>,
    transitionStyle: {},
    ...over,
  }
  render(<ShareScreen {...props} />)
  return props
}

describe('ShareScreen', () => {
  it('renders the three sections, header and toast slot', () => {
    setup()
    expect(screen.getByText('HEADER')).toBeInTheDocument()
    expect(screen.getByText('TOAST')).toBeInTheDocument()
    expect(screen.getByText('Refer a Friend')).toBeInTheDocument()
    expect(screen.getByText('Shared List')).toBeInTheDocument()
    expect(screen.getByText('Share a List')).toBeInTheDocument()
  })

  it('shows the referral link and encodes it in the QR code', () => {
    setup()
    expect(screen.getByText(new RegExp(`${window.location.origin}.*ABC123`))).toBeInTheDocument()
    const qr = screen.getByTestId('qr')
    expect(qr.getAttribute('data-value')).toContain('ABC123')
    expect(qr.getAttribute('data-value')).toContain('qr')
  })

  it('Copy, Share link and Share my list each fire their callback once', () => {
    const p = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Copy' }))
    fireEvent.click(screen.getByRole('button', { name: 'Share link' }))
    fireEvent.click(screen.getByRole('button', { name: 'Share my list for 7 days' }))
    expect(p.onCopyReferral).toHaveBeenCalledTimes(1)
    expect(p.onShareReferral).toHaveBeenCalledTimes(1)
    expect(p.onShareList).toHaveBeenCalledTimes(1)
    expect(p.onInvite).not.toHaveBeenCalled()
  })

  it('invite fires once; while loading it is disabled and relabelled', () => {
    const p = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Invite your family to this list' }))
    expect(p.onInvite).toHaveBeenCalledTimes(1)
  })

  it('does not fire invite while a link is being created', () => {
    const p = setup({ inviteLoading: true })
    const btn = screen.getByRole('button', { name: 'Creating link…' })
    expect(btn).toBeDisabled()
    fireEvent.click(btn)
    expect(p.onInvite).not.toHaveBeenCalled()
  })

  it('hides leave and friend count when there are no members or friends', () => {
    setup()
    expect(screen.queryByRole('button', { name: 'Leave shared list' })).not.toBeInTheDocument()
    expect(screen.queryByText(/joined$/)).not.toBeInTheDocument()
  })

  it('lists household members with initials and a fallback name, and leaves once', () => {
    const p = setup({ settingsMembers: [{ name: 'dana' }, { name: '' }] })
    expect(screen.getByText('dana')).toBeInTheDocument()
    expect(screen.getByText('D')).toBeInTheDocument()
    expect(screen.getByText('Household member')).toBeInTheDocument()
    expect(screen.getByText('?')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Leave shared list' }))
    expect(p.onLeaveHousehold).toHaveBeenCalledTimes(1)
  })

  it('pluralises the friend count', () => {
    const { unmount } = render(<div />)
    unmount()
    setup({ referralFriends: [{ name: 'A', created_at: '' }] })
    expect(screen.getByText('1 friend joined')).toBeInTheDocument()
  })

  it('pluralises the friend count for several', () => {
    setup({ referralFriends: [{ name: 'A', created_at: '' }, { name: 'B', created_at: '' }] })
    expect(screen.getByText('2 friends joined')).toBeInTheDocument()
  })

  describe('confirm modal', () => {
    it('is absent when confirmModal is null', () => {
      setup()
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    })

    it('shows the supplied copy; confirm and cancel each fire once', () => {
      const p = setup({ confirmModal: { title: 'Leave list?', message: 'Sure?', confirmLabel: 'Leave', destructive: true } })
      expect(screen.getByRole('alertdialog', { name: 'Leave list?' })).toBeInTheDocument()
      expect(screen.getByText('Sure?')).toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Leave' }))
      expect(p.runConfirmAction).toHaveBeenCalledTimes(1)
      fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
      expect(p.closeConfirmModal).toHaveBeenCalledTimes(1)
    })

    it('Escape closes it', () => {
      const p = setup({ confirmModal: { title: 'Leave list?' } })
      fireEvent.keyDown(document, { key: 'Escape' })
      expect(p.closeConfirmModal).toHaveBeenCalledTimes(1)
    })
  })

  describe('Email a friend', () => {
    it('opens a mailto link carrying the referral link', () => {
      setup()
      const open = vi.spyOn(window, 'open').mockImplementation(() => null)
      fireEvent.click(screen.getByRole('button', { name: 'Email a friend' }))
      expect(open).toHaveBeenCalledTimes(1)
      const [url, target] = open.mock.calls[0] as [string, string]
      expect(url.startsWith('mailto:?subject=')).toBe(true)
      expect(decodeURIComponent(url)).toContain('ABC123')
      expect(target).toBe('_self')
      open.mockRestore()
    })
  })

  describe('Save QR as image', () => {
    const ctx = new Proxy({}, { get: () => () => {}, set: () => true })
    beforeEach(() => {
      vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(ctx as never)
      vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation(cb => cb(new Blob(['x'], { type: 'image/png' })))
      URL.createObjectURL = vi.fn(() => 'blob:x')
      URL.revokeObjectURL = vi.fn()
    })
    afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

    it('downloads the PNG once and toasts when native share is unavailable', async () => {
      const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
      const p = setup()
      fireEvent.click(screen.getByRole('button', { name: 'Save QR as image' }))
      await waitFor(() => expect(p.onShowToast).toHaveBeenCalledWith('QR code saved!'))
      expect(click).toHaveBeenCalledTimes(1)
      expect(p.onShowToast).toHaveBeenCalledTimes(1)
    })

    it('uses native share with the file when available and does not toast', async () => {
      const share = vi.fn().mockResolvedValue(undefined)
      vi.stubGlobal('navigator', { ...navigator, share, canShare: () => true })
      const p = setup()
      fireEvent.click(screen.getByRole('button', { name: 'Save QR as image' }))
      await waitFor(() => expect(share).toHaveBeenCalledTimes(1))
      expect(share.mock.calls[0][0].files[0].name).toBe('penny-referral.png')
      expect(p.onShowToast).not.toHaveBeenCalled()
    })

    it('does nothing when the canvas yields no image', async () => {
      vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation(cb => cb(null))
      const p = setup()
      fireEvent.click(screen.getByRole('button', { name: 'Save QR as image' }))
      await new Promise(r => setTimeout(r, 20))
      expect(p.onShowToast).not.toHaveBeenCalled()
      expect(URL.createObjectURL).not.toHaveBeenCalled()
    })

    it('does nothing when no 2d context exists', async () => {
      vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null)
      const p = setup()
      fireEvent.click(screen.getByRole('button', { name: 'Save QR as image' }))
      await new Promise(r => setTimeout(r, 20))
      expect(p.onShowToast).not.toHaveBeenCalled()
    })
  })

  it('visible copy never says bot, AI or assistant', () => {
    setup()
    expect(document.body.textContent).not.toMatch(/\b(bot|AI|assistant)\b/i)
  })
})
