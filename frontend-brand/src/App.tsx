import { useState, useEffect, useCallback } from 'react'

// =============================================================================
// DESIGN TOKENS
// =============================================================================

const BRAND = '#35644F'
const BRAND_LIGHT = '#EEF7F2'
const TEXT = '#1A1A1A'
const TEXT_SEC = '#666'
const BORDER = '#E0E0E0'
const WHITE = '#FFF'
const BG = '#FAFAFA'
const ERROR = '#D32F2F'
const FONT = "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"

// =============================================================================
// TYPES
// =============================================================================

interface BrandUser {
  brand_user_id: number
  email: string
  company_name: string
  verified: boolean
  badge: string
  balance_cents: number
  created_at: string
  can_submit_feedback?: boolean
}

interface Deal {
  id: number
  competitor_product: string
  offer_brand: string
  offer_product: string
  discount_cents: number
  daily_budget_cents: number
  budget_spent_cents: number
  status: string
  starts_at: string
  ends_at: string
  deal_type: string
  offered: number
  accepted: number
  declined: number
  accept_rate: number
  created_at: string
}

interface DashboardData {
  total_deals: number
  active_deals: number
  total_swaps: number
  total_accepted: number
  total_declined: number
  accept_rate: number
  total_spent_cents: number
  balance_cents: number
  daily: { date: string; offered: number; accepted: number }[]
  verified_purchases: number
  pending_claims: number
  purchase_rate: number
}

interface Transaction {
  id: number
  type: string
  amount_cents: number
  description: string
  stripe_id: string
  created_at: string
}

interface DealPerformanceDay {
  date: string
  impressions: number
  accepts: number
  declines: number
  revenue_cents: number
  verified_count: number
}

interface OptimizationRec {
  title: string
  description: string
  priority: string
  action: string
}

interface DeclineReason {
  reason: string
  count: number
}

type Screen = 'landing' | 'login' | 'register' | 'dashboard' | 'deals' | 'create-deal' | 'deal-detail' | 'intel' | 'transactions' | 'deposit'

// =============================================================================
// API HELPERS
// =============================================================================

async function api<T>(path: string, opts?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  })
  if (res.status === 401) throw new Error('UNAUTHORIZED')
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `Request failed (${res.status})`)
  }
  return res.json()
}

function formatCents(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`
}

// =============================================================================
// APP
// =============================================================================

export function App() {
  const [screen, setScreen] = useState<Screen>('landing')
  const [user, setUser] = useState<BrandUser | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Dashboard data
  const [dashboard, setDashboard] = useState<DashboardData | null>(null)
  const [deals, setDeals] = useState<Deal[]>([])
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [dashDays, setDashDays] = useState(30)

  // Deal detail
  const [selectedDealId, setSelectedDealId] = useState<number | null>(null)
  const [dealPerf, setDealPerf] = useState<DealPerformanceDay[]>([])
  const [dealRecs, setDealRecs] = useState<OptimizationRec[]>([])
  const [dealDeclineReasons, setDealDeclineReasons] = useState<DeclineReason[]>([])
  const [dealDetailLoading, setDealDetailLoading] = useState(false)

  // Mobile nav
  const [navOpen, setNavOpen] = useState(false)

  // Form state
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [companyName, setCompanyName] = useState('')

  // Auth check on mount
  useEffect(() => {
    api<BrandUser>('/brand/me')
      .then(u => { setUser(u); setScreen('dashboard') })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  // Load dashboard data when screen changes
  const loadDashboard = useCallback(() => {
    if (!user) return
    api<DashboardData>(`/brand/dashboard?days=${dashDays}`).then(setDashboard).catch(() => {})
    api<Deal[]>('/brand/deals').then(setDeals).catch(() => {})
  }, [user, dashDays])

  useEffect(() => {
    if (screen === 'dashboard' || screen === 'deals') loadDashboard()
    if (screen === 'transactions') {
      api<Transaction[]>('/brand/transactions').then(setTransactions).catch(() => {})
    }
  }, [screen, loadDashboard])

  // Load deal detail data
  const openDealDetail = useCallback((dealId: number) => {
    setSelectedDealId(dealId)
    setDealDetailLoading(true)
    setScreen('deal-detail')
    Promise.all([
      api<{ performance: DealPerformanceDay[] }>(`/brand/deal-performance/${dealId}`).catch(() => ({ performance: [] })),
      api<{ recommendations: OptimizationRec[]; decline_reasons: DeclineReason[] }>(`/brand/optimize/${dealId}`).catch(() => ({ recommendations: [], decline_reasons: [] })),
    ]).then(([perf, opt]) => {
      setDealPerf(perf.performance || [])
      setDealRecs(opt.recommendations || [])
      setDealDeclineReasons(opt.decline_reasons || [])
    }).finally(() => setDealDetailLoading(false))
  }, [])

  // Auth handlers
  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      await api('/brand/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      })
      const u = await api<BrandUser>('/brand/me')
      setUser(u)
      setScreen('dashboard')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    }
  }

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      await api('/brand/register', {
        method: 'POST',
        body: JSON.stringify({ email, password, company_name: companyName }),
      })
      const u = await api<BrandUser>('/brand/me')
      setUser(u)
      setScreen('dashboard')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed')
    }
  }

  const handleLogout = async () => {
    await api('/brand/logout', { method: 'POST' }).catch(() => {})
    setUser(null)
    setScreen('landing')
  }

  if (loading) {
    return <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh', fontFamily: FONT, color: TEXT_SEC }}>Loading...</div>
  }

  // ==========================================================================
  // LANDING PAGE
  // ==========================================================================
  if (screen === 'landing' || screen === 'login' || screen === 'register') {
    return (
      <div style={{ minHeight: '100vh', backgroundColor: BG }}>
        {/* Nav */}
        <nav style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '16px 24px', borderBottom: `1px solid ${BORDER}`, backgroundColor: WHITE }}>
          <div style={{ fontFamily: FONT, fontSize: '20px', fontWeight: 700, color: BRAND }}>Thriftly <span style={{ fontWeight: 400, color: TEXT_SEC }}>for Brands</span></div>
          <div style={{ display: 'flex', gap: '12px' }}>
            <button onClick={() => { setScreen('login'); setError('') }} style={linkBtn}>Sign In</button>
            <button onClick={() => { setScreen('register'); setError('') }} style={primaryBtn}>Get Started</button>
          </div>
        </nav>

        {screen === 'landing' && (
          <div style={{ maxWidth: '800px', margin: '0 auto', padding: '80px 24px', textAlign: 'center' }}>
            <h1 style={{ fontFamily: FONT, fontSize: '48px', fontWeight: 700, color: TEXT, lineHeight: 1.2 }}>
              Put your product on<br />every grocery list
            </h1>
            <p style={{ fontFamily: FONT, fontSize: '20px', color: TEXT_SEC, marginTop: '16px', lineHeight: 1.6 }}>
              Thriftly shoppers see your product as a smart swap when they add a competitor item.
              Pay only when they accept. $1.50 per swap.
            </p>
            <button onClick={() => { setScreen('register'); setError('') }} style={{ ...primaryBtn, fontSize: '18px', padding: '14px 32px', marginTop: '32px' }}>
              Create Your First Deal
            </button>

            {/* Value props */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '24px', marginTop: '64px', textAlign: 'left' }}>
              {[
                { title: 'Pay per swap', desc: 'Only pay when a shopper accepts your product. No impressions fees.' },
                { title: 'Real-time analytics', desc: 'See acceptance rates, competitive intel, and ROI in your dashboard.' },
                { title: 'Self-serve', desc: 'Create deals in minutes. No sales calls, no contracts, no minimums.' },
              ].map(v => (
                <div key={v.title} style={{ padding: '24px', backgroundColor: WHITE, borderRadius: '12px', border: `1px solid ${BORDER}` }}>
                  <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT }}>{v.title}</div>
                  <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SEC, marginTop: '8px', lineHeight: 1.5 }}>{v.desc}</div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Login / Register forms */}
        {(screen === 'login' || screen === 'register') && (
          <div style={{ maxWidth: '400px', margin: '0 auto', padding: '48px 24px' }}>
            <h2 style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 600, color: TEXT, marginBottom: '24px' }}>
              {screen === 'login' ? 'Sign in to your account' : 'Create your brand account'}
            </h2>
            {error && <div style={{ fontFamily: FONT, fontSize: '14px', color: ERROR, padding: '12px', backgroundColor: '#FFEBEE', borderRadius: '8px', marginBottom: '16px' }}>{error}</div>}
            <form onSubmit={screen === 'login' ? handleLogin : handleRegister}>
              {screen === 'register' && (
                <div style={{ marginBottom: '16px' }}>
                  <label style={labelStyle}>Company Name</label>
                  <input type="text" value={companyName} onChange={e => setCompanyName(e.target.value)} required style={inputStyle} placeholder="Your brand or company" />
                </div>
              )}
              <div style={{ marginBottom: '16px' }}>
                <label style={labelStyle}>Email</label>
                <input type="email" value={email} onChange={e => setEmail(e.target.value)} required style={inputStyle} placeholder="you@company.com" />
              </div>
              <div style={{ marginBottom: '24px' }}>
                <label style={labelStyle}>Password</label>
                <input type="password" value={password} onChange={e => setPassword(e.target.value)} required minLength={8} style={inputStyle} placeholder="8+ characters" />
              </div>
              <button type="submit" style={{ ...primaryBtn, width: '100%', padding: '12px' }}>
                {screen === 'login' ? 'Sign In' : 'Create Account'}
              </button>
            </form>
            <p style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SEC, marginTop: '16px', textAlign: 'center' }}>
              {screen === 'login' ? (
                <>Don't have an account? <button onClick={() => { setScreen('register'); setError('') }} style={{ ...linkBtn, fontSize: '14px' }}>Sign up</button></>
              ) : (
                <>Already have an account? <button onClick={() => { setScreen('login'); setError('') }} style={{ ...linkBtn, fontSize: '14px' }}>Sign in</button></>
              )}
            </p>
          </div>
        )}
      </div>
    )
  }

  // ==========================================================================
  // AUTHENTICATED VIEWS
  // ==========================================================================
  return (
    <div style={{ minHeight: '100vh', backgroundColor: BG }}>
      {/* Nav */}
      <nav style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 24px', borderBottom: `1px solid ${BORDER}`, backgroundColor: WHITE, position: 'sticky', top: 0, zIndex: 10 }}>
        <div style={{ fontFamily: FONT, fontSize: '18px', fontWeight: 700, color: BRAND, cursor: 'pointer' }} onClick={() => { setScreen('dashboard'); setNavOpen(false) }}>
          Thriftly <span style={{ fontWeight: 400, color: TEXT_SEC }}>for Brands</span>
        </div>
        {/* Desktop nav */}
        <div style={{ display: 'flex', gap: '16px', alignItems: 'center' }} className="brand-nav-desktop">
          <NavLink active={screen === 'dashboard'} onClick={() => setScreen('dashboard')}>Dashboard</NavLink>
          <NavLink active={screen === 'deals' || screen === 'create-deal' || screen === 'deal-detail'} onClick={() => setScreen('deals')}>Deals</NavLink>
          <NavLink active={screen === 'intel'} onClick={() => setScreen('intel')}>Intel</NavLink>
          <NavLink active={screen === 'transactions' || screen === 'deposit'} onClick={() => setScreen('transactions')}>Billing</NavLink>
          <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SEC }}>{user?.company_name}</div>
          {user?.badge === 'founding' && <span style={{ fontFamily: FONT, fontSize: '10px', fontWeight: 700, color: WHITE, backgroundColor: '#FF9800', padding: '2px 6px', borderRadius: '4px' }}>FOUNDING BRAND</span>}
          <button onClick={handleLogout} style={linkBtn}>Sign Out</button>
        </div>
        {/* Mobile hamburger */}
        <button onClick={() => setNavOpen(!navOpen)} className="brand-nav-mobile-btn" style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '8px', display: 'none' }}>
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={TEXT} strokeWidth="2" strokeLinecap="round"><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="18" x2="21" y2="18"/></svg>
        </button>
      </nav>
      {/* Mobile nav dropdown */}
      {navOpen && (
        <div style={{ position: 'sticky', top: '49px', zIndex: 9, backgroundColor: WHITE, borderBottom: `1px solid ${BORDER}`, padding: '8px 24px', display: 'flex', flexDirection: 'column', gap: '4px' }} className="brand-nav-mobile-menu">
          {[
            { label: 'Dashboard', s: 'dashboard' as Screen, match: screen === 'dashboard' },
            { label: 'Deals', s: 'deals' as Screen, match: screen === 'deals' || screen === 'create-deal' || screen === 'deal-detail' },
            { label: 'Intel', s: 'intel' as Screen, match: screen === 'intel' },
            { label: 'Billing', s: 'transactions' as Screen, match: screen === 'transactions' || screen === 'deposit' },
          ].map(n => (
            <button key={n.label} onClick={() => { setScreen(n.s); setNavOpen(false) }} style={{
              background: 'none', border: 'none', textAlign: 'left', padding: '10px 0',
              fontFamily: FONT, fontSize: '15px', fontWeight: n.match ? 600 : 400,
              color: n.match ? BRAND : TEXT, cursor: 'pointer', borderBottom: `1px solid ${BORDER}`,
            }}>{n.label}</button>
          ))}
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '10px 0' }}>
            <span style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SEC }}>{user?.company_name}</span>
            <button onClick={() => { handleLogout(); setNavOpen(false) }} style={{ ...linkBtn, fontSize: '13px' }}>Sign Out</button>
          </div>
        </div>
      )}

      <div style={{ maxWidth: '1000px', margin: '0 auto', padding: '24px' }}>
        {/* ================================================================ */}
        {/* DASHBOARD */}
        {/* ================================================================ */}
        {screen === 'dashboard' && dashboard && (
          <>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '8px' }}>
              <h1 style={h1Style}>Dashboard</h1>
              <div style={{ display: 'flex', gap: '6px', alignItems: 'center' }}>
                {[7, 14, 30, 90].map(d => (
                  <button key={d} onClick={() => setDashDays(d)} style={{
                    padding: '4px 10px', borderRadius: '6px', fontFamily: FONT, fontSize: '12px', fontWeight: 600,
                    cursor: 'pointer', border: `1px solid ${dashDays === d ? BRAND : BORDER}`,
                    backgroundColor: dashDays === d ? BRAND_LIGHT : 'transparent', color: dashDays === d ? BRAND : TEXT_SEC,
                  }}>{d}d</button>
                ))}
                <button onClick={() => window.open('/brand/export?type=swaps', '_blank')} style={{ ...smallBtn, marginLeft: '8px' }} title="Export CSV">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ verticalAlign: 'middle', marginRight: '4px' }}><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                  Export
                </button>
              </div>
            </div>

            {/* Competitive Intel Preview — above fold, FOMO hook */}
            <div style={{ ...cardStyle, marginTop: '16px', cursor: 'pointer', borderLeft: `4px solid ${BRAND}` }} onClick={() => setScreen('intel')}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div>
                  <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT }}>Competitive Swap Intel</div>
                  <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SEC, marginTop: '4px' }}>See which products shoppers are swapping in your category. Free for all brands.</div>
                </div>
                <span style={{ fontFamily: FONT, fontSize: '24px', color: BRAND }}>→</span>
              </div>
            </div>

            {/* Purchase Funnel */}
            <div style={{ ...cardStyle, marginTop: '16px' }}>
              <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginBottom: '16px' }}>Purchase Funnel</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                {[
                  { label: 'Offered', value: dashboard.total_swaps, color: '#E3F2FD' },
                  { label: 'Accepted', value: dashboard.total_accepted, color: BRAND_LIGHT },
                  { label: 'Purchased', value: dashboard.verified_purchases, color: BRAND },
                ].map((step, i) => {
                  const maxVal = Math.max(dashboard.total_swaps, 1)
                  const width = Math.max(((step.value / maxVal) * 100), 8)
                  return (
                    <div key={step.label} style={{ flex: 1, textAlign: 'center' }}>
                      <div style={{
                        height: '48px', backgroundColor: step.color, borderRadius: '6px',
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        width: `${width}%`, minWidth: '60px', margin: '0 auto',
                        color: i === 2 ? WHITE : TEXT, fontFamily: FONT, fontSize: '18px', fontWeight: 700,
                      }}>{step.value}</div>
                      <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SEC, marginTop: '4px' }}>{step.label}</div>
                    </div>
                  )
                })}
              </div>
              {dashboard.total_accepted > 0 && (
                <div style={{ display: 'flex', gap: '24px', marginTop: '12px', fontFamily: FONT, fontSize: '13px', color: TEXT_SEC, justifyContent: 'center' }}>
                  <span>Accept rate: <b style={{ color: dashboard.accept_rate >= 8 ? BRAND : TEXT }}>{dashboard.accept_rate}%</b></span>
                  <span>Purchase rate: <b style={{ color: dashboard.purchase_rate >= 30 ? BRAND : TEXT }}>{dashboard.purchase_rate}%</b></span>
                  <span>Pending claims: {dashboard.pending_claims}</span>
                </div>
              )}
            </div>

            {/* Key Metrics */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: '12px', marginTop: '16px' }}>
              <MetricCard label="Active Deals" value={String(dashboard.active_deals)} />
              <MetricCard label="Total Spent" value={formatCents(dashboard.total_spent_cents)} />
              <MetricCard label="Balance" value={formatCents(dashboard.balance_cents)} />
              <MetricCard label="Accept Rate" value={`${dashboard.accept_rate}%`} highlight={dashboard.accept_rate >= 8} />
            </div>

            {/* Daily chart */}
            {dashboard.daily.length > 0 && (
              <div style={{ ...cardStyle, marginTop: '16px' }}>
                <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginBottom: '16px' }}>Swaps — Last {dashDays} Days</div>
                <div style={{ display: 'flex', gap: '4px', alignItems: 'flex-end', height: '120px' }}>
                  {dashboard.daily.sort((a, b) => a.date.localeCompare(b.date)).map(d => {
                    const maxVal = Math.max(...dashboard.daily.map(x => x.offered), 1)
                    return (
                      <div key={d.date} style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '2px' }}>
                        <div style={{ width: '100%', backgroundColor: BRAND_LIGHT, borderRadius: '2px', height: `${(d.offered / maxVal) * 100}px`, position: 'relative' }}>
                          <div style={{ position: 'absolute', bottom: 0, width: '100%', backgroundColor: BRAND, borderRadius: '2px', height: `${(d.accepted / maxVal) * 100}px` }} />
                        </div>
                      </div>
                    )
                  })}
                </div>
                <div style={{ display: 'flex', gap: '16px', marginTop: '8px', fontFamily: FONT, fontSize: '12px', color: TEXT_SEC }}>
                  <span><span style={{ display: 'inline-block', width: '8px', height: '8px', backgroundColor: BRAND_LIGHT, borderRadius: '2px', marginRight: '4px' }}></span>Offered</span>
                  <span><span style={{ display: 'inline-block', width: '8px', height: '8px', backgroundColor: BRAND, borderRadius: '2px', marginRight: '4px' }}></span>Accepted</span>
                </div>
              </div>
            )}

            {/* Quick actions */}
            <div style={{ display: 'flex', gap: '12px', marginTop: '16px' }}>
              <button onClick={() => setScreen('create-deal')} style={primaryBtn}>Create New Deal</button>
              <button onClick={() => setScreen('deposit')} style={secondaryBtn}>Add Funds</button>
            </div>
          </>
        )}

        {/* ================================================================ */}
        {/* DEALS LIST */}
        {/* ================================================================ */}
        {screen === 'deals' && (
          <>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <h1 style={h1Style}>Your Deals</h1>
              <button onClick={() => setScreen('create-deal')} style={primaryBtn}>New Deal</button>
            </div>
            {deals.length === 0 && (
              <div style={{ ...cardStyle, marginTop: '16px', textAlign: 'center', padding: '48px 24px' }}>
                <div style={{ fontFamily: FONT, fontSize: '16px', color: TEXT_SEC }}>No deals yet. Create your first deal to start reaching shoppers.</div>
                <button onClick={() => setScreen('create-deal')} style={{ ...primaryBtn, marginTop: '16px' }}>Create First Deal</button>
              </div>
            )}
            {deals.map(deal => (
              <div key={deal.id} style={{ ...cardStyle, marginTop: '12px', cursor: 'pointer', transition: 'box-shadow 0.15s' }} onClick={() => openDealDetail(deal.id)}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                  <div>
                    <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT }}>{deal.offer_brand} {deal.offer_product}</div>
                    <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SEC, marginTop: '2px' }}>replaces: {deal.competitor_product}</div>
                    <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SEC }}>Discount: {formatCents(deal.discount_cents)} | Budget: {formatCents(deal.daily_budget_cents)} | Spent: {formatCents(deal.budget_spent_cents)}</div>
                  </div>
                  <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                    <span style={{ fontFamily: FONT, fontSize: '12px', fontWeight: 600, padding: '4px 8px', borderRadius: '4px', backgroundColor: deal.status === 'active' ? BRAND_LIGHT : '#FFF3E0', color: deal.status === 'active' ? BRAND : '#E65100' }}>{deal.status}</span>
                    {deal.deal_type !== 'sku_swap' && (
                      <span style={{ fontFamily: FONT, fontSize: '11px', fontWeight: 600, padding: '2px 6px', borderRadius: '4px', backgroundColor: '#E3F2FD', color: '#1565C0' }}>{deal.deal_type}</span>
                    )}
                  </div>
                </div>
                <div style={{ display: 'flex', gap: '24px', marginTop: '12px', fontFamily: FONT, fontSize: '13px', color: TEXT_SEC }}>
                  <span>Offered: {deal.offered}</span>
                  <span>Accepted: {deal.accepted}</span>
                  <span>Declined: {deal.declined}</span>
                  <span style={{ color: deal.accept_rate >= 8 ? BRAND : TEXT_SEC, fontWeight: deal.accept_rate >= 8 ? 600 : 400 }}>Rate: {deal.accept_rate}%</span>
                </div>
                <div style={{ display: 'flex', gap: '8px', marginTop: '12px' }} onClick={e => e.stopPropagation()}>
                  {deal.status === 'active' && (
                    <button onClick={async () => { await api(`/brand/deals/${deal.id}`, { method: 'PUT', body: JSON.stringify({ status: 'paused' }) }); loadDashboard() }} style={smallBtn}>Pause</button>
                  )}
                  {deal.status === 'paused' && (
                    <button onClick={async () => { await api(`/brand/deals/${deal.id}`, { method: 'PUT', body: JSON.stringify({ status: 'active' }) }); loadDashboard() }} style={{ ...smallBtn, color: BRAND }}>Resume</button>
                  )}
                  <button onClick={async () => { if (confirm('Delete this deal?')) { await api(`/brand/deals/${deal.id}`, { method: 'DELETE' }); loadDashboard() } }} style={{ ...smallBtn, color: ERROR }}>Delete</button>
                  <button onClick={() => openDealDetail(deal.id)} style={{ ...smallBtn, color: BRAND }}>Details</button>
                </div>
              </div>
            ))}
          </>
        )}

        {/* ================================================================ */}
        {/* DEAL DETAIL */}
        {/* ================================================================ */}
        {screen === 'deal-detail' && (() => {
          const deal = deals.find(d => d.id === selectedDealId)
          if (!deal) return <div style={{ fontFamily: FONT, color: TEXT_SEC, textAlign: 'center', padding: '48px' }}>Deal not found. <button onClick={() => setScreen('deals')} style={linkBtn}>Back to deals</button></div>
          return (
            <>
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '16px' }}>
                <button onClick={() => setScreen('deals')} style={{ ...linkBtn, fontSize: '14px' }}>&larr; Back</button>
                <h1 style={{ ...h1Style, flex: 1 }}>{deal.offer_brand} {deal.offer_product}</h1>
                <span style={{ fontFamily: FONT, fontSize: '12px', fontWeight: 600, padding: '4px 8px', borderRadius: '4px', backgroundColor: deal.status === 'active' ? BRAND_LIGHT : '#FFF3E0', color: deal.status === 'active' ? BRAND : '#E65100' }}>{deal.status}</span>
              </div>

              {/* Summary metrics */}
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: '12px' }}>
                <MetricCard label="Offered" value={String(deal.offered)} />
                <MetricCard label="Accepted" value={String(deal.accepted)} />
                <MetricCard label="Declined" value={String(deal.declined)} />
                <MetricCard label="Accept Rate" value={`${deal.accept_rate}%`} highlight={deal.accept_rate >= 8} />
                <MetricCard label="Budget" value={formatCents(deal.daily_budget_cents)} />
                <MetricCard label="Spent" value={formatCents(deal.budget_spent_cents)} />
              </div>

              {dealDetailLoading ? (
                <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SEC, textAlign: 'center', padding: '32px' }}>Loading performance data...</div>
              ) : (
                <>
                  {/* Performance chart */}
                  {dealPerf.length > 0 && (
                    <div style={{ ...cardStyle, marginTop: '16px' }}>
                      <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginBottom: '16px' }}>Daily Performance — Last 30 Days</div>
                      <div style={{ display: 'flex', gap: '3px', alignItems: 'flex-end', height: '120px' }}>
                        {[...dealPerf].sort((a, b) => a.date.localeCompare(b.date)).map(d => {
                          const maxVal = Math.max(...dealPerf.map(x => x.impressions), 1)
                          return (
                            <div key={d.date} title={`${d.date}: ${d.impressions} offered, ${d.accepts} accepted`} style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                              <div style={{ width: '100%', backgroundColor: BRAND_LIGHT, borderRadius: '2px', height: `${(d.impressions / maxVal) * 100}px`, position: 'relative', minHeight: d.impressions > 0 ? '4px' : '0px' }}>
                                <div style={{ position: 'absolute', bottom: 0, width: '100%', backgroundColor: BRAND, borderRadius: '2px', height: `${(d.accepts / maxVal) * 100}px`, minHeight: d.accepts > 0 ? '4px' : '0px' }} />
                              </div>
                            </div>
                          )
                        })}
                      </div>
                      <div style={{ display: 'flex', gap: '16px', marginTop: '8px', fontFamily: FONT, fontSize: '12px', color: TEXT_SEC }}>
                        <span><span style={{ display: 'inline-block', width: '8px', height: '8px', backgroundColor: BRAND_LIGHT, borderRadius: '2px', marginRight: '4px' }}></span>Offered</span>
                        <span><span style={{ display: 'inline-block', width: '8px', height: '8px', backgroundColor: BRAND, borderRadius: '2px', marginRight: '4px' }}></span>Accepted</span>
                      </div>
                    </div>
                  )}

                  {/* Decline reasons + Optimization in two columns */}
                  <div style={{ display: 'grid', gridTemplateColumns: dealDeclineReasons.length > 0 ? '1fr 1fr' : '1fr', gap: '16px', marginTop: '16px' }}>
                    {/* Decline reasons */}
                    {dealDeclineReasons.length > 0 && (
                      <div style={cardStyle}>
                        <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginBottom: '12px' }}>Top Decline Reasons</div>
                        {dealDeclineReasons.map(dr => {
                          const maxCount = Math.max(...dealDeclineReasons.map(x => x.count), 1)
                          return (
                            <div key={dr.reason} style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
                              <div style={{ flex: 1 }}>
                                <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT, textTransform: 'capitalize' }}>{dr.reason.replace(/_/g, ' ')}</div>
                                <div style={{ height: '6px', backgroundColor: BORDER, borderRadius: '3px', marginTop: '4px', overflow: 'hidden' }}>
                                  <div style={{ height: '100%', backgroundColor: '#E57373', borderRadius: '3px', width: `${(dr.count / maxCount) * 100}%` }} />
                                </div>
                              </div>
                              <span style={{ fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: TEXT_SEC, minWidth: '32px', textAlign: 'right' }}>{dr.count}</span>
                            </div>
                          )
                        })}
                      </div>
                    )}

                    {/* Optimization recommendations */}
                    {dealRecs.length > 0 && (
                      <div style={cardStyle}>
                        <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginBottom: '12px' }}>Optimization Recommendations</div>
                        {dealRecs.map((rec, i) => (
                          <div key={i} style={{ marginBottom: '12px', padding: '10px', backgroundColor: rec.priority === 'high' ? '#FFF3E0' : BG, borderRadius: '8px', border: `1px solid ${rec.priority === 'high' ? '#FFE0B2' : BORDER}` }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                              <span style={{ fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: rec.priority === 'high' ? '#E65100' : TEXT }}>{rec.title}</span>
                              {rec.priority === 'high' && <span style={{ fontFamily: FONT, fontSize: '10px', fontWeight: 700, padding: '1px 5px', borderRadius: '3px', backgroundColor: '#FF9800', color: WHITE }}>HIGH</span>}
                            </div>
                            <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SEC, marginTop: '4px', lineHeight: 1.4 }}>{rec.description}</div>
                            {rec.action && <div style={{ fontFamily: FONT, fontSize: '12px', color: BRAND, fontWeight: 600, marginTop: '6px' }}>{rec.action}</div>}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Deal info */}
                  <div style={{ ...cardStyle, marginTop: '16px' }}>
                    <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginBottom: '12px' }}>Deal Details</div>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px', fontFamily: FONT, fontSize: '13px' }}>
                      <div><span style={{ color: TEXT_SEC }}>Competitor:</span> <span style={{ color: TEXT }}>{deal.competitor_product}</span></div>
                      <div><span style={{ color: TEXT_SEC }}>Discount:</span> <span style={{ color: TEXT }}>{formatCents(deal.discount_cents)}</span></div>
                      <div><span style={{ color: TEXT_SEC }}>Type:</span> <span style={{ color: TEXT, textTransform: 'capitalize' }}>{deal.deal_type.replace(/_/g, ' ')}</span></div>
                      <div><span style={{ color: TEXT_SEC }}>Created:</span> <span style={{ color: TEXT }}>{new Date(deal.created_at).toLocaleDateString()}</span></div>
                    </div>
                  </div>

                  {/* Actions */}
                  <div style={{ display: 'flex', gap: '8px', marginTop: '16px' }}>
                    {deal.status === 'active' && (
                      <button onClick={async () => { await api(`/brand/deals/${deal.id}`, { method: 'PUT', body: JSON.stringify({ status: 'paused' }) }); loadDashboard() }} style={secondaryBtn}>Pause Deal</button>
                    )}
                    {deal.status === 'paused' && (
                      <button onClick={async () => { await api(`/brand/deals/${deal.id}`, { method: 'PUT', body: JSON.stringify({ status: 'active' }) }); loadDashboard() }} style={primaryBtn}>Resume Deal</button>
                    )}
                    <button onClick={() => window.open(`/brand/export?type=swaps&deal_id=${deal.id}`, '_blank')} style={secondaryBtn}>Export CSV</button>
                    <button onClick={async () => { if (confirm('Delete this deal?')) { await api(`/brand/deals/${deal.id}`, { method: 'DELETE' }); setScreen('deals'); loadDashboard() } }} style={{ ...secondaryBtn, color: ERROR, borderColor: ERROR }}>Delete</button>
                  </div>
                </>
              )}
            </>
          )
        })()}

        {/* ================================================================ */}
        {/* CREATE DEAL */}
        {/* ================================================================ */}
        {screen === 'create-deal' && <CreateDealForm onCreated={() => { setScreen('deals'); loadDashboard() }} onCancel={() => setScreen('deals')} />}

        {/* ================================================================ */}
        {/* COMPETITIVE INTEL */}
        {/* ================================================================ */}
        {screen === 'intel' && <IntelPanel />}

        {/* ================================================================ */}
        {/* TRANSACTIONS / BILLING */}
        {/* ================================================================ */}
        {screen === 'transactions' && (
          <>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <h1 style={h1Style}>Billing</h1>
              <button onClick={() => setScreen('deposit')} style={primaryBtn}>Add Funds</button>
            </div>
            <div style={{ ...cardStyle, marginTop: '16px' }}>
              <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SEC }}>Current Balance</div>
              <div style={{ fontFamily: FONT, fontSize: '28px', fontWeight: 700, color: TEXT }}>{formatCents(user?.balance_cents || 0)}</div>
            </div>
            {transactions.length === 0 ? (
              <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SEC, marginTop: '16px', textAlign: 'center' }}>No transactions yet</div>
            ) : (
              <table style={{ width: '100%', marginTop: '16px', borderCollapse: 'collapse', fontFamily: FONT, fontSize: '14px' }}>
                <thead>
                  <tr style={{ borderBottom: `2px solid ${BORDER}` }}>
                    <th style={thStyle}>Date</th>
                    <th style={thStyle}>Type</th>
                    <th style={thStyle}>Amount</th>
                    <th style={thStyle}>Description</th>
                  </tr>
                </thead>
                <tbody>
                  {transactions.map(t => (
                    <tr key={t.id} style={{ borderBottom: `1px solid ${BORDER}` }}>
                      <td style={tdStyle}>{new Date(t.created_at).toLocaleDateString()}</td>
                      <td style={tdStyle}><span style={{ fontWeight: 600, color: t.type === 'deposit' ? BRAND : TEXT }}>{t.type}</span></td>
                      <td style={tdStyle}>{t.type === 'deposit' ? '+' : '-'}{formatCents(t.amount_cents)}</td>
                      <td style={tdStyle}>{t.description}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </>
        )}

        {/* ================================================================ */}
        {/* DEPOSIT */}
        {/* ================================================================ */}
        {screen === 'deposit' && <DepositForm user={user} onDeposited={() => { setScreen('transactions'); api<BrandUser>('/brand/me').then(setUser) }} onCancel={() => setScreen('transactions')} />}
      </div>

      {/* Feedback Widget */}
      {user?.can_submit_feedback && (
        <BrandFeedbackWidget />
      )}
    </div>
  )
}

// =============================================================================
// SUB-COMPONENTS
// =============================================================================

function NavLink({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button onClick={onClick} style={{
      background: 'none', border: 'none', cursor: 'pointer',
      fontFamily: FONT, fontSize: '14px', fontWeight: active ? 600 : 400,
      color: active ? BRAND : TEXT_SEC,
      borderBottom: active ? `2px solid ${BRAND}` : '2px solid transparent',
      padding: '4px 0',
    }}>{children}</button>
  )
}

function MetricCard({ label, value, highlight }: { label: string; value: string; highlight?: boolean }) {
  return (
    <div style={{ ...cardStyle, padding: '16px' }}>
      <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SEC }}>{label}</div>
      <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: highlight ? BRAND : TEXT, marginTop: '4px' }}>{value}</div>
    </div>
  )
}

function CreateDealForm({ onCreated, onCancel }: { onCreated: () => void; onCancel: () => void }) {
  const [form, setForm] = useState({
    competitor_product: '', offer_brand: '', offer_product: '',
    discount_cents: 150, daily_budget_cents: 10000,
    offer_price_cents: 0, competitor_price_cents: 0,
    offer_size: '', competitor_size: '',
    offer_unit_price: '', competitor_unit_price: '',
    unit_label: '', why_bullets: '[]', allergen_notes: '',
    deal_type: 'sku_swap', quantity_offered: 1,
    trial_product_name: '', trial_size: '', campaign_tag: '',
  })
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const set = (key: string, val: string | number) => setForm(f => ({ ...f, [key]: val }))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      await api('/brand/deals', { method: 'POST', body: JSON.stringify(form) })
      onCreated()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create deal')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h1 style={h1Style}>Create New Deal</h1>
        <button onClick={onCancel} style={linkBtn}>Cancel</button>
      </div>
      {error && <div style={{ fontFamily: FONT, fontSize: '14px', color: ERROR, padding: '12px', backgroundColor: '#FFEBEE', borderRadius: '8px', marginTop: '12px' }}>{error}</div>}
      <form onSubmit={handleSubmit} style={{ ...cardStyle, marginTop: '16px' }}>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
          <div>
            <label style={labelStyle}>Competitor Product</label>
            <input value={form.competitor_product} onChange={e => set('competitor_product', e.target.value)} required style={inputStyle} placeholder="e.g. Heinz Ketchup" />
          </div>
          <div>
            <label style={labelStyle}>Deal Type</label>
            <select value={form.deal_type} onChange={e => set('deal_type', e.target.value)} style={inputStyle}>
              <option value="sku_swap">SKU Swap</option>
              <option value="bogo">BOGO</option>
              <option value="trial">Free Trial</option>
              <option value="seasonal">Seasonal</option>
            </select>
          </div>
          <div>
            <label style={labelStyle}>Your Brand</label>
            <input value={form.offer_brand} onChange={e => set('offer_brand', e.target.value)} required style={inputStyle} placeholder="e.g. Sir Kensington's" />
          </div>
          <div>
            <label style={labelStyle}>Your Product</label>
            <input value={form.offer_product} onChange={e => set('offer_product', e.target.value)} required style={inputStyle} placeholder="e.g. Classic Ketchup" />
          </div>
          <div>
            <label style={labelStyle}>Discount (cents)</label>
            <input type="number" value={form.discount_cents} onChange={e => set('discount_cents', parseInt(e.target.value) || 0)} style={inputStyle} />
          </div>
          <div>
            <label style={labelStyle}>Total Budget (cents)</label>
            <input type="number" value={form.daily_budget_cents} onChange={e => set('daily_budget_cents', parseInt(e.target.value) || 0)} style={inputStyle} />
          </div>
          <div>
            <label style={labelStyle}>Your Price (cents)</label>
            <input type="number" value={form.offer_price_cents} onChange={e => set('offer_price_cents', parseInt(e.target.value) || 0)} style={inputStyle} />
          </div>
          <div>
            <label style={labelStyle}>Competitor Price (cents)</label>
            <input type="number" value={form.competitor_price_cents} onChange={e => set('competitor_price_cents', parseInt(e.target.value) || 0)} style={inputStyle} />
          </div>
          <div>
            <label style={labelStyle}>Your Size</label>
            <input value={form.offer_size} onChange={e => set('offer_size', e.target.value)} style={inputStyle} placeholder="e.g. 20 oz" />
          </div>
          <div>
            <label style={labelStyle}>Competitor Size</label>
            <input value={form.competitor_size} onChange={e => set('competitor_size', e.target.value)} style={inputStyle} />
          </div>
        </div>

        {/* Why bullets */}
        <div style={{ marginTop: '16px' }}>
          <label style={labelStyle}>Why Bullets (JSON array)</label>
          <textarea value={form.why_bullets} onChange={e => set('why_bullets', e.target.value)} style={{ ...inputStyle, height: '60px', resize: 'vertical' as const }} placeholder='["Better ingredients","Higher ratings"]' />
        </div>

        {/* Conditional fields */}
        {form.deal_type === 'bogo' && (
          <div style={{ marginTop: '16px' }}>
            <label style={labelStyle}>Quantity Offered</label>
            <input type="number" value={form.quantity_offered} onChange={e => set('quantity_offered', parseInt(e.target.value) || 2)} style={inputStyle} />
          </div>
        )}
        {form.deal_type === 'trial' && (
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginTop: '16px' }}>
            <div>
              <label style={labelStyle}>Trial Product Name</label>
              <input value={form.trial_product_name} onChange={e => set('trial_product_name', e.target.value)} style={inputStyle} />
            </div>
            <div>
              <label style={labelStyle}>Trial Size</label>
              <input value={form.trial_size} onChange={e => set('trial_size', e.target.value)} style={inputStyle} />
            </div>
          </div>
        )}
        {form.deal_type === 'seasonal' && (
          <div style={{ marginTop: '16px' }}>
            <label style={labelStyle}>Campaign Tag</label>
            <input value={form.campaign_tag} onChange={e => set('campaign_tag', e.target.value)} style={inputStyle} placeholder="e.g. thanksgiving_2026" />
          </div>
        )}

        <button type="submit" disabled={submitting} style={{ ...primaryBtn, marginTop: '24px', width: '100%', padding: '12px', opacity: submitting ? 0.7 : 1 }}>
          {submitting ? 'Creating...' : 'Create Deal'}
        </button>
      </form>
    </div>
  )
}

function IntelPanel() {
  const [category, setCategory] = useState('')
  const [data, setData] = useState<{ category: string; total_swap_outs: number; top_brands: { brand: string; count: number }[]; details: { competitor_product: string; offer_brand: string; accepted: number; offered: number }[] } | null>(null)
  const [searching, setSearching] = useState(false)

  const search = async () => {
    if (!category.trim()) return
    setSearching(true)
    try {
      const result = await api<typeof data>(`/brand/category-intel/${encodeURIComponent(category)}`)
      setData(result)
    } catch { setData(null) }
    finally { setSearching(false) }
  }

  return (
    <div>
      <h1 style={h1Style}>Competitive Swap Intel</h1>
      <p style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SEC, marginTop: '4px' }}>See which products are being swapped in any category. Anonymized, aggregated data.</p>
      <div style={{ display: 'flex', gap: '8px', marginTop: '16px' }}>
        <input value={category} onChange={e => setCategory(e.target.value)} onKeyDown={e => e.key === 'Enter' && search()} placeholder="Enter category (e.g. ketchup, cereal, yogurt)" style={{ ...inputStyle, flex: 1 }} />
        <button onClick={search} disabled={searching} style={primaryBtn}>{searching ? '...' : 'Search'}</button>
      </div>
      {data && (
        <div style={{ ...cardStyle, marginTop: '16px' }}>
          <div style={{ fontFamily: FONT, fontSize: '18px', fontWeight: 600, color: TEXT }}>
            {data.category} — {data.total_swap_outs} total swap-outs
          </div>
          {data.top_brands.length > 0 && (
            <div style={{ marginTop: '16px' }}>
              <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT_SEC, marginBottom: '8px' }}>Top Replacement Brands</div>
              {data.top_brands.sort((a, b) => b.count - a.count).map(b => (
                <div key={b.brand} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: `1px solid ${BORDER}`, fontFamily: FONT, fontSize: '14px' }}>
                  <span>{b.brand}</span>
                  <span style={{ fontWeight: 600 }}>{b.count} swaps</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function DepositForm({ user, onDeposited, onCancel }: { user: BrandUser | null; onDeposited: () => void; onCancel: () => void }) {
  const [amount, setAmount] = useState(100)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const handleDeposit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      await api('/brand/deposit', {
        method: 'POST',
        body: JSON.stringify({ amount_cents: amount * 100 }),
      })
      onDeposited()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Deposit failed')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h1 style={h1Style}>Add Funds</h1>
        <button onClick={onCancel} style={linkBtn}>Cancel</button>
      </div>
      <div style={{ ...cardStyle, marginTop: '16px' }}>
        <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SEC }}>Current Balance</div>
        <div style={{ fontFamily: FONT, fontSize: '28px', fontWeight: 700, color: TEXT }}>{formatCents(user?.balance_cents || 0)}</div>
      </div>
      {error && <div style={{ fontFamily: FONT, fontSize: '14px', color: ERROR, padding: '12px', backgroundColor: '#FFEBEE', borderRadius: '8px', marginTop: '12px' }}>{error}</div>}
      <form onSubmit={handleDeposit} style={{ ...cardStyle, marginTop: '16px' }}>
        <label style={labelStyle}>Deposit Amount ($)</label>
        <div style={{ display: 'flex', gap: '8px', marginTop: '8px' }}>
          {[50, 100, 250, 500].map(v => (
            <button key={v} type="button" onClick={() => setAmount(v)} style={{
              ...secondaryBtn,
              backgroundColor: amount === v ? BRAND_LIGHT : WHITE,
              borderColor: amount === v ? BRAND : BORDER,
              fontWeight: amount === v ? 600 : 400,
            }}>${v}</button>
          ))}
        </div>
        <input type="number" value={amount} onChange={e => setAmount(parseInt(e.target.value) || 0)} min={50} style={{ ...inputStyle, marginTop: '12px' }} />
        <p style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SEC, marginTop: '8px' }}>Minimum deposit: $50. Funds are used to pay for accepted swaps at {formatCents(150)}/swap.</p>
        <button type="submit" disabled={submitting || amount < 50} style={{ ...primaryBtn, marginTop: '16px', width: '100%', padding: '12px', opacity: (submitting || amount < 50) ? 0.7 : 1 }}>
          {submitting ? 'Processing...' : `Deposit ${formatCents(amount * 100)}`}
        </button>
      </form>
    </div>
  )
}

function BrandFeedbackWidget() {
  const [open, setOpen] = useState(false)
  const [body, setBody] = useState('')
  const [type, setType] = useState('feature')
  const [urgency, setUrgency] = useState('normal')
  const [sending, setSending] = useState(false)
  const [sent, setSent] = useState(false)

  useEffect(() => {
    if (sent) {
      const t = setTimeout(() => { setSent(false); setOpen(false) }, 1800)
      return () => clearTimeout(t)
    }
  }, [sent])

  const handleSubmit = async () => {
    if (!body.trim() || sending) return
    setSending(true)
    try {
      await api('/brand/feedback', {
        method: 'POST',
        body: JSON.stringify({ body: body.trim(), type, urgency }),
      })
      setBody('')
      setType('feature')
      setUrgency('normal')
      setSent(true)
    } catch {
      // silently fail
    } finally {
      setSending(false)
    }
  }

  const TYPES = [
    { key: 'bug', label: 'Bug' },
    { key: 'feature', label: 'Idea' },
    { key: 'other', label: 'Other' },
  ]

  const URGENCIES = [
    { key: 'low', label: 'Low' },
    { key: 'normal', label: 'Normal' },
    { key: 'critical', label: 'Urgent' },
  ]

  const canSend = !!body.trim() && !sending

  return (
    <>
      {/* FAB — bottom-right corner */}
      {!open && (
        <button
          onClick={() => setOpen(true)}
          aria-label="Send feedback"
          style={{
            position: 'fixed', bottom: '24px', right: '24px', zIndex: 50,
            width: '44px', height: '44px', display: 'flex', alignItems: 'center', justifyContent: 'center',
            backgroundColor: BRAND, color: WHITE, border: 'none', borderRadius: '50%',
            cursor: 'pointer', boxShadow: '0 2px 12px rgba(0,0,0,0.15)',
          }}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M9 18h6"/><path d="M10 22h4"/><path d="M12 2a7 7 0 0 0-4 12.7V17h8v-2.3A7 7 0 0 0 12 2z"/>
          </svg>
        </button>
      )}

      {/* Modal backdrop */}
      {open && (
        <div onClick={() => { if (!sending) setOpen(false) }} style={{ position: 'fixed', inset: 0, zIndex: 9998, backgroundColor: 'rgba(0,0,0,0.3)' }} />
      )}

      {/* Feedback panel */}
      {open && (
        <div style={{
          position: 'fixed', bottom: '24px', right: '24px', zIndex: 9999,
          width: '380px', maxWidth: 'calc(100vw - 48px)',
          backgroundColor: WHITE, borderRadius: '16px', border: `1px solid ${BORDER}`,
          boxShadow: '0 8px 32px rgba(0,0,0,0.12)', padding: '24px',
        }}>
          {sent ? (
            <div style={{ textAlign: 'center', padding: '16px 0' }}>
              <div style={{ fontSize: '28px', color: BRAND, marginBottom: '8px' }}>&#10003;</div>
              <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT }}>Got it!</div>
              <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SEC, marginTop: '4px' }}>Your feedback shapes what we build next.</div>
            </div>
          ) : (
            <>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
                <div>
                  <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 700, color: TEXT }}>Product Feedback</div>
                  <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SEC, marginTop: '2px' }}>Goes directly to the product team.</div>
                </div>
                <button onClick={() => setOpen(false)} style={{ background: 'none', border: 'none', cursor: 'pointer', color: TEXT_SEC, fontSize: '18px', padding: '4px' }}>&times;</button>
              </div>

              {/* Type chips */}
              <div style={{ display: 'flex', gap: '8px', marginBottom: '12px' }}>
                {TYPES.map(t => (
                  <button key={t.key} onClick={() => setType(t.key)} style={{
                    padding: '6px 14px', borderRadius: '16px', fontFamily: FONT, fontSize: '12px', fontWeight: 600,
                    cursor: 'pointer',
                    border: type === t.key ? `1.5px solid ${BRAND}` : `1.5px solid ${BORDER}`,
                    backgroundColor: type === t.key ? BRAND_LIGHT : 'transparent',
                    color: type === t.key ? BRAND : TEXT_SEC,
                  }}>{t.label}</button>
                ))}
              </div>

              <textarea
                value={body}
                onChange={e => setBody(e.target.value)}
                placeholder={type === 'bug' ? 'What went wrong?' : type === 'feature' ? 'What would make the brand portal better?' : "What's on your mind?"}
                maxLength={10000}
                rows={3}
                style={{
                  width: '100%', padding: '10px 12px', fontFamily: FONT, fontSize: '14px', color: TEXT,
                  backgroundColor: BG, border: `1px solid ${BORDER}`, borderRadius: '10px',
                  outline: 'none', resize: 'none', minHeight: '80px', boxSizing: 'border-box',
                }}
              />

              {/* Urgency chips */}
              <div style={{ display: 'flex', gap: '8px', marginTop: '10px', marginBottom: '14px' }}>
                {URGENCIES.map(u => (
                  <button key={u.key} onClick={() => setUrgency(u.key)} style={{
                    padding: '6px 14px', borderRadius: '16px', fontFamily: FONT, fontSize: '12px', fontWeight: 600,
                    cursor: 'pointer',
                    border: urgency === u.key ? `1.5px solid ${BRAND}` : `1.5px solid ${BORDER}`,
                    backgroundColor: urgency === u.key ? BRAND_LIGHT : 'transparent',
                    color: urgency === u.key ? BRAND : TEXT_SEC,
                  }}>{u.label}</button>
                ))}
              </div>

              <button
                onClick={handleSubmit}
                disabled={!canSend}
                style={{
                  ...primaryBtn, width: '100%', padding: '10px',
                  opacity: canSend ? 1 : 0.5,
                  cursor: canSend ? 'pointer' : 'default',
                }}
              >
                {sending ? 'Sending...' : 'Send Feedback'}
              </button>
            </>
          )}
        </div>
      )}
    </>
  )
}

// =============================================================================
// SHARED STYLES
// =============================================================================

const primaryBtn: React.CSSProperties = {
  backgroundColor: BRAND, color: WHITE, border: 'none', borderRadius: '8px',
  fontFamily: FONT, fontSize: '14px', fontWeight: 600, cursor: 'pointer', padding: '8px 16px',
}

const secondaryBtn: React.CSSProperties = {
  backgroundColor: WHITE, color: TEXT, border: `1px solid ${BORDER}`, borderRadius: '8px',
  fontFamily: FONT, fontSize: '14px', fontWeight: 400, cursor: 'pointer', padding: '8px 16px',
}

const linkBtn: React.CSSProperties = {
  background: 'none', border: 'none', color: BRAND,
  fontFamily: FONT, fontSize: '14px', fontWeight: 600, cursor: 'pointer', padding: 0,
}

const smallBtn: React.CSSProperties = {
  background: 'none', border: `1px solid ${BORDER}`, borderRadius: '6px',
  fontFamily: FONT, fontSize: '12px', fontWeight: 500, cursor: 'pointer', padding: '4px 10px', color: TEXT_SEC,
}

const cardStyle: React.CSSProperties = {
  backgroundColor: WHITE, border: `1px solid ${BORDER}`, borderRadius: '12px', padding: '20px',
}

const inputStyle: React.CSSProperties = {
  width: '100%', padding: '10px 12px', border: `1px solid ${BORDER}`, borderRadius: '8px',
  fontFamily: FONT, fontSize: '14px', color: TEXT, outline: 'none',
}

const labelStyle: React.CSSProperties = {
  fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: TEXT_SEC, display: 'block', marginBottom: '4px',
}

const h1Style: React.CSSProperties = {
  fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: TEXT,
}

const thStyle: React.CSSProperties = {
  textAlign: 'left' as const, padding: '8px 12px', fontWeight: 600, color: TEXT_SEC,
}

const tdStyle: React.CSSProperties = {
  padding: '10px 12px', color: TEXT,
}
