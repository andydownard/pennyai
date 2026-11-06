import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { WebcamCapture } from './WebcamCapture'

function fakeStream() {
  const track = { stop: vi.fn() }
  return { stream: { getTracks: () => [track] } as unknown as MediaStream, track }
}

function mockCamera(impl: () => Promise<MediaStream>) {
  const getUserMedia = vi.fn(impl)
  Object.defineProperty(navigator, 'mediaDevices', { value: { getUserMedia }, configurable: true })
  return getUserMedia
}

async function ready(container: HTMLElement) {
  const video = container.querySelector('video')!
  await waitFor(() => expect(video.srcObject).toBeTruthy())
  act(() => { fireEvent(video, new Event('loadedmetadata')) })
  return video
}

describe('WebcamCapture', () => {
  beforeEach(() => {
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(() => Promise.resolve())
  })
  afterEach(() => { vi.restoreAllMocks() })

  it('asks for the rear camera, video only, and shows a starting state', async () => {
    const { stream } = fakeStream()
    const gum = mockCamera(() => Promise.resolve(stream))
    const { container } = render(<WebcamCapture onCapture={vi.fn()} onClose={vi.fn()} />)
    expect(screen.getByText('Starting camera...')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Capture' })).toBeDisabled()
    await waitFor(() => expect(gum).toHaveBeenCalledTimes(1))
    expect(gum.mock.calls[0]).toEqual([expect.objectContaining({ audio: false, video: expect.objectContaining({ facingMode: 'environment' }) })])
    await ready(container)
    expect(screen.queryByText('Starting camera...')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Capture' })).toBeEnabled()
  })

  it('captures a JPEG frame, stops the camera, and calls onCapture once', async () => {
    const { stream, track } = fakeStream()
    mockCamera(() => Promise.resolve(stream))
    const drawImage = vi.fn()
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({ drawImage } as never)
    const toDataURL = vi.spyOn(HTMLCanvasElement.prototype, 'toDataURL').mockReturnValue('data:image/jpeg;base64,AAA')
    const onCapture = vi.fn()
    const { container } = render(<WebcamCapture onCapture={onCapture} onClose={vi.fn()} />)
    await ready(container)
    fireEvent.click(screen.getByRole('button', { name: 'Capture' }))
    expect(drawImage).toHaveBeenCalledTimes(1)
    expect(toDataURL).toHaveBeenCalledWith('image/jpeg', 0.85)
    expect(track.stop).toHaveBeenCalled()
    expect(onCapture).toHaveBeenCalledTimes(1)
    expect(onCapture).toHaveBeenCalledWith('data:image/jpeg;base64,AAA')
  })

  it('does not call onCapture when no canvas context is available', async () => {
    const { stream } = fakeStream()
    mockCamera(() => Promise.resolve(stream))
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null)
    const onCapture = vi.fn()
    const { container } = render(<WebcamCapture onCapture={onCapture} onClose={vi.fn()} />)
    await ready(container)
    fireEvent.click(screen.getByRole('button', { name: 'Capture' }))
    expect(onCapture).not.toHaveBeenCalled()
  })

  it('does not capture before the camera is ready', async () => {
    const { stream } = fakeStream()
    mockCamera(() => Promise.resolve(stream))
    const onCapture = vi.fn()
    render(<WebcamCapture onCapture={onCapture} onClose={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Capture' }))
    expect(onCapture).not.toHaveBeenCalled()
  })

  it('Cancel stops the camera and closes once', async () => {
    const { stream, track } = fakeStream()
    mockCamera(() => Promise.resolve(stream))
    const onClose = vi.fn()
    const { container } = render(<WebcamCapture onCapture={vi.fn()} onClose={onClose} />)
    await ready(container)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(track.stop).toHaveBeenCalled()
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('backdrop click closes; clicking the panel does not', async () => {
    const { stream } = fakeStream()
    mockCamera(() => Promise.resolve(stream))
    const onClose = vi.fn()
    const { container } = render(<WebcamCapture onCapture={vi.fn()} onClose={onClose} />)
    await ready(container)
    fireEvent.click(container.querySelector('video')!)
    expect(onClose).not.toHaveBeenCalled()
    fireEvent.click(container.firstElementChild!)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('stops the camera when unmounted', async () => {
    const { stream, track } = fakeStream()
    mockCamera(() => Promise.resolve(stream))
    const { container, unmount } = render(<WebcamCapture onCapture={vi.fn()} onClose={vi.fn()} />)
    await ready(container)
    unmount()
    expect(track.stop).toHaveBeenCalled()
  })

  it('stops a stream that arrives after unmount', async () => {
    const { stream, track } = fakeStream()
    let resolve: (s: MediaStream) => void = () => {}
    const gum = mockCamera(() => new Promise<MediaStream>(r => { resolve = r }))
    const { unmount } = render(<WebcamCapture onCapture={vi.fn()} onClose={vi.fn()} />)
    await waitFor(() => expect(gum).toHaveBeenCalled())
    unmount()
    await act(async () => { resolve(stream) })
    expect(track.stop).toHaveBeenCalledTimes(1)
  })

  describe('errors', () => {
    const fail = (name: string) => mockCamera(() => Promise.reject(new DOMException('x', name)))

    it('permission denied', async () => {
      fail('NotAllowedError')
      render(<WebcamCapture onCapture={vi.fn()} onClose={vi.fn()} />)
      expect(await screen.findByText(/Camera access denied/)).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Capture' })).not.toBeInTheDocument()
    })

    it('no camera', async () => {
      fail('NotFoundError')
      render(<WebcamCapture onCapture={vi.fn()} onClose={vi.fn()} />)
      expect(await screen.findByText('No camera found on this device.')).toBeInTheDocument()
    })

    it('anything else', async () => {
      mockCamera(() => Promise.reject(new Error('weird')))
      render(<WebcamCapture onCapture={vi.fn()} onClose={vi.fn()} />)
      expect(await screen.findByText('Could not start camera. Please try again.')).toBeInTheDocument()
    })

    it('the error view closes once via its Close button', async () => {
      fail('NotAllowedError')
      const onClose = vi.fn()
      render(<WebcamCapture onCapture={vi.fn()} onClose={onClose} />)
      fireEvent.click(await screen.findByRole('button', { name: 'Close' }))
      expect(onClose).toHaveBeenCalledTimes(1)
    })

    it('ignores a failure that arrives after unmount', async () => {
      let reject: (e: unknown) => void = () => {}
      const gum = mockCamera(() => new Promise<MediaStream>((_, r) => { reject = r }))
      const err = vi.spyOn(console, 'error').mockImplementation(() => {})
      const { unmount } = render(<WebcamCapture onCapture={vi.fn()} onClose={vi.fn()} />)
      await waitFor(() => expect(gum).toHaveBeenCalled())
      unmount()
      await act(async () => { reject(new DOMException('x', 'NotAllowedError')) })
      expect(err).not.toHaveBeenCalled()
    })
  })
})
