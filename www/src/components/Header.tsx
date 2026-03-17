import { useState, useEffect, useCallback, useRef } from 'react'
import { Link, NavLink, useLocation } from 'react-router-dom'

const prefetchCache = new Set<string>()
function prefetch(loader: () => Promise<unknown>) {
  const key = loader.toString()
  if (!prefetchCache.has(key)) {
    prefetchCache.add(key)
    loader()
  }
}

export default function Header() {
  const [scrolled, setScrolled] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const [headerHidden, setHeaderHidden] = useState(false)
  const lastScrollY = useRef(0)
  const ticking = useRef(false)
  const location = useLocation()

  const handleScroll = useCallback(() => {
    if (ticking.current) return
    ticking.current = true
    requestAnimationFrame(() => {
      const y = window.scrollY
      setScrolled(y > 10)
      if (y > 200) {
        setHeaderHidden(y > lastScrollY.current && y - lastScrollY.current > 5)
      } else {
        setHeaderHidden(false)
      }
      lastScrollY.current = y
      ticking.current = false
    })
  }, [])

  useEffect(() => {
    window.addEventListener('scroll', handleScroll, { passive: true })
    return () => window.removeEventListener('scroll', handleScroll)
  }, [handleScroll])

  // Close mobile nav on route change
  useEffect(() => {
    setMobileOpen(false)
  }, [location.pathname])

  // Lock body scroll when mobile menu is open
  useEffect(() => {
    document.body.style.overflow = mobileOpen ? 'hidden' : ''
    return () => { document.body.style.overflow = '' }
  }, [mobileOpen])

  return (
    <header className={`header${scrolled ? ' scrolled' : ''}${headerHidden ? ' header-hidden' : ''}`} role="banner">
      <div className="header-inner">
        <Link to="/" className="logo">
          <img src="/logo.svg" alt="thriftly" className="logo-img" />
        </Link>
        <nav className="nav" aria-label="Main navigation">
          <Link to="/#how-it-works">How It Works</Link>
          <Link to="/#features">Features</Link>
          <NavLink to="/for-brands" className={({ isActive }) => isActive ? 'active' : ''} onMouseEnter={() => prefetch(() => import('../pages/ForBrands'))}>For Brands</NavLink>
          <NavLink to="/pricing" className={({ isActive }) => isActive ? 'active' : ''} onMouseEnter={() => prefetch(() => import('../pages/Pricing'))}>Pricing</NavLink>
          <NavLink to="/about" className={({ isActive }) => isActive ? 'active' : ''} onMouseEnter={() => prefetch(() => import('../pages/About'))}>About</NavLink>
          <a href="https://app.thrift.ly" rel="noopener" target="_blank" className="nav-cta">Try Free</a>
        </nav>
        <button
          className="mobile-menu-btn"
          onClick={() => setMobileOpen(!mobileOpen)}
          aria-label={mobileOpen ? 'Close menu' : 'Open menu'}
          aria-expanded={mobileOpen}
        >
          <span /><span /><span />
        </button>
      </div>
      <nav className={`mobile-nav${mobileOpen ? ' open' : ''}`} aria-label="Mobile navigation">
        <Link to="/#how-it-works" onClick={() => setMobileOpen(false)}>How It Works</Link>
        <Link to="/#features" onClick={() => setMobileOpen(false)}>Features</Link>
        <Link to="/for-brands" onClick={() => setMobileOpen(false)}>For Brands</Link>
        <Link to="/pricing" onClick={() => setMobileOpen(false)}>Pricing</Link>
        <Link to="/about" onClick={() => setMobileOpen(false)}>About</Link>
        <a href="https://app.thrift.ly" rel="noopener" target="_blank" className="nav-cta" onClick={() => setMobileOpen(false)}>Try Free</a>
      </nav>
    </header>
  )
}
