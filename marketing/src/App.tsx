import { lazy, Suspense } from 'react'
import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import ErrorBoundary from './components/ErrorBoundary'
import Home from './pages/Home'
import './App.css'

const ForBrands = lazy(() => import('./pages/ForBrands'))
const Pricing = lazy(() => import('./pages/Pricing'))
const CampaignSignup = lazy(() => import('./pages/CampaignSignup'))
const About = lazy(() => import('./pages/About'))
const Privacy = lazy(() => import('./pages/Privacy'))
const Terms = lazy(() => import('./pages/Terms'))
const NotFound = lazy(() => import('./pages/NotFound'))

function PageLoader() {
  return (
    <div className="page-loader">
      <img src="/logo.svg" alt="Groflo" className="page-loader-logo" />
    </div>
  )
}

function App() {
  return (
    <ErrorBoundary>
      <Suspense fallback={<PageLoader />}>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<Home />} />
            <Route path="/for-brands" element={<ForBrands />} />
            <Route path="/pricing" element={<Pricing />} />
            <Route path="/campaign/signup" element={<CampaignSignup />} />
            <Route path="/about" element={<About />} />
            <Route path="/privacy" element={<Privacy />} />
            <Route path="/terms" element={<Terms />} />
            <Route path="*" element={<NotFound />} />
          </Route>
        </Routes>
      </Suspense>
    </ErrorBoundary>
  )
}

export default App
