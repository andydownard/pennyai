import { useEffect, useRef, useState, useCallback } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import Header from './Header'
import Footer from './Footer'
import FeedbackWidget from './FeedbackWidget'

function ScrollToHash() {
  const { pathname, hash } = useLocation()
  const lastHash = useRef('')

  useEffect(() => {
    // Send GA4 page_view on route change (SPA navigation)
    if (typeof window.gtag === 'function') {
      window.gtag('event', 'page_view', {
        page_path: pathname + (hash || ''),
        page_title: document.title,
      })
    }

    if (hash) {
      // Validate hash is a safe CSS selector
      if (/^#[a-zA-Z][\w-]*$/.test(hash)) {
        // Small delay to let DOM render before scrolling
        setTimeout(() => {
          const el = document.querySelector(hash)
          if (el) {
            el.scrollIntoView({ behavior: 'smooth' })
          }
        }, 100)
      }
      lastHash.current = hash
    } else {
      window.scrollTo(0, 0)
    }
  }, [pathname, hash])

  return null
}

function ScrollProgress() {
  const [progress, setProgress] = useState(0)
  const [showBackToTop, setShowBackToTop] = useState(false)

  const handleScroll = useCallback(() => {
    const scrollTop = window.scrollY
    const docHeight = document.documentElement.scrollHeight - window.innerHeight
    setProgress(docHeight > 0 ? (scrollTop / docHeight) * 100 : 0)
    setShowBackToTop(scrollTop > 600)
  }, [])

  useEffect(() => {
    window.addEventListener('scroll', handleScroll, { passive: true })
    return () => window.removeEventListener('scroll', handleScroll)
  }, [handleScroll])

  return (
    <>
      <div className="scroll-progress" role="progressbar" aria-valuenow={Math.round(progress)} aria-valuemin={0} aria-valuemax={100}>
        <div className="scroll-progress-bar" style={{ width: `${progress}%` }} />
      </div>
      <button
        className={`back-to-top${showBackToTop ? ' visible' : ''}`}
        onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}
        aria-label="Back to top"
      >
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path d="M12 4l-1.41 1.41L16.17 11H4v2h12.17l-5.58 5.59L12 20l8-8-8-8z" fill="white" transform="rotate(-90 12 12)" />
        </svg>
      </button>
    </>
  )
}

export default function Layout() {
  return (
    <>
      <a href="#main-content" className="skip-to-content">Skip to content</a>
      <ScrollToHash />
      <ScrollProgress />
      <Header />
      <main id="main-content" role="main">
        <Outlet />
      </main>
      <Footer />
      <FeedbackWidget />
    </>
  )
}
