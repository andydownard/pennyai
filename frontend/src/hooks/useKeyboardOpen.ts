import { useState, useEffect } from 'react'

export function useKeyboardOpen() {
  const [keyboardOpen, setKeyboardOpen] = useState(false)

  useEffect(() => {
    if (!window.visualViewport) return
    const handler = () => {
      const shrunk = window.innerHeight - window.visualViewport!.height
      setKeyboardOpen(shrunk > 100)
    }
    window.visualViewport.addEventListener('resize', handler)
    return () => window.visualViewport!.removeEventListener('resize', handler)
  }, [])

  return keyboardOpen
}
