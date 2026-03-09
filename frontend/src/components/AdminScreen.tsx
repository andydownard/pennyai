import { useState, useEffect } from 'react'
import { ScreenTransition } from './ScreenTransition'
import { BrandDealDeepDive } from './BrandDealDeepDive'
import { fetchAPI } from '../api'
import {
  FONT,
  pageStyles,
} from '../styles'

interface AdminScreenProps {
  showToast: (msg: string, type?: 'success' | 'error') => void
  renderHeader: () => React.ReactNode
  renderToast: () => React.ReactNode
  renderBottomTabs: () => React.ReactNode
  transitionStyle: React.CSSProperties
}

interface AdminData {
  active_users: number; total_users: number; new_users: number;
  k_factor: number; leaderboard: { phone: string; name: string; points: number; referrals: number }[];
  nps_responses: { score: number; comment: string; user_name: string; user_phone: string; created_at: string }[];
  nps_score: number; nps_total: number; nps_promoters: number; nps_detractors: number;
  swap_offered?: number; swap_accepted?: number; swap_declined?: number; swap_accept_rate?: number;
}

interface Deal {
  id: number; competitor_product: string; offer_brand: string; offer_product: string;
  discount_cents: number; daily_budget_cents: number; budget_spent_cents: number;
  status: string; starts_at: string; ends_at: string; created_at: string;
  offer_price_cents: number; competitor_price_cents: number;
  offer_size: string; competitor_size: string;
  offer_unit_price: string; competitor_unit_price: string;
  why_bullets: string; allergen_notes: string;
  deal_type: string; quantity_offered: number;
  trial_product_name: string; trial_size: string; campaign_tag: string;
}

interface Payout {
  user_id: number; name: string; email: string; phone: string;
  total_cents: number; swap_count: number; oldest_swap: string;
}

interface AnalyticsData {
  items_created: number;
  items_by_source: Record<string, number>;
  items_by_department: Record<string, number>;
  events_by_action: Record<string, number>;
  events_by_day: { date: string; count: number }[];
}

// --- L5 Business Learning interfaces ---

interface BusinessMetric {
  date: string;
  dau: number;
  wau: number;
  mau: number;
  acceptance_rate?: number;
  gross_revenue_cents?: number;
  active_deals?: number;
  active_brands?: number;
}

interface BusinessMetricsData {
  days: number;
  metrics: BusinessMetric[];
}

interface ChurnRiskUser {
  user_id: number;
  name: string;
  composite_risk: number;
  signals: { type: string; value: number; days_since_active?: number }[];
}

interface ChurnRiskData {
  at_risk_users: ChurnRiskUser[];
  count: number;
}

interface CohortRow {
  cohort_week: string;
  user_count: number;
  retained_count: number;
  avg_swaps: number;
}

interface CohortData {
  cohorts: CohortRow[];
}

interface MarketplaceHealthEntry {
  date: string;
  supply_score: number;
  demand_score: number;
  balance_score: number;
  health_index: number;
}

interface MarketplaceHealthData {
  health: MarketplaceHealthEntry[];
}

interface FeedbackItem {
  id: number; user_id: number; body: string; type: string; urgency: string;
  status: string; admin_notes: string; github_issue_url: string;
  created_at: string; updated_at: string; user_name: string; user_phone: string;
}

type AdminTab = 'overview' | 'business' | 'churn' | 'cohorts' | 'marketplace' | 'feedback'

const EMPTY_FORM = {
  competitor_product: '', offer_brand: '', offer_product: '',
  discount_cents: 0, daily_budget_cents: 0, status: 'active', ends_at: '',
  offer_price_cents: 0, competitor_price_cents: 0,
  offer_size: '', competitor_size: '',
  offer_unit_price: '', competitor_unit_price: '',
  why_bullets: '[]', allergen_notes: '',
  deal_type: 'sku_swap', quantity_offered: 1,
  trial_product_name: '', trial_size: '', campaign_tag: '',
}

// --- SVG Sparkline component ---
function Sparkline({ values, width = 280, height = 60, color = 'var(--brand)' }: { values: number[]; width?: number; height?: number; color?: string }) {
  if (values.length < 2) return null
  const max = Math.max(...values, 1)
  const min = Math.min(...values, 0)
  const range = max - min || 1
  const padding = 4
  const chartW = width - padding * 2
  const chartH = height - padding * 2

  const points = values.map((v, i) => {
    const x = padding + (i / (values.length - 1)) * chartW
    const y = padding + chartH - ((v - min) / range) * chartH
    return `${x},${y}`
  })

  const firstX = padding
  const lastX = padding + chartW
  const bottomY = padding + chartH
  const areaPath = `M${firstX},${bottomY} L${points.join(' L')} L${lastX},${bottomY} Z`

  return (
    <svg width={width} height={height} style={{ display: 'block', maxWidth: '100%' }}>
      <path d={areaPath} fill={`color-mix(in srgb, ${color} 12%, transparent)`} />
      <polyline
        points={points.join(' ')}
        fill="none"
        stroke={color}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {(() => {
        const lastVal = values[values.length - 1]
        const cx = padding + ((values.length - 1) / (values.length - 1)) * chartW
        const cy = padding + chartH - ((lastVal - min) / range) * chartH
        return <circle cx={cx} cy={cy} r="3" fill={color} />
      })()}
    </svg>
  )
}

// --- SVG Bar Chart component ---
function BarChart({ values, labels, width = 280, height = 80, color = 'var(--brand)' }: { values: number[]; labels?: string[]; width?: number; height?: number; color?: string }) {
  if (values.length === 0) return null
  const max = Math.max(...values, 1)
  const barWidth = Math.max(2, Math.min(12, (width - 20) / values.length - 2))
  const gap = 2
  const totalBarWidth = (barWidth + gap) * values.length - gap
  const offsetX = (width - totalBarWidth) / 2
  const chartH = height - 20

  return (
    <svg width={width} height={height} style={{ display: 'block', maxWidth: '100%' }}>
      {values.map((v, i) => {
        const barH = Math.max(1, (v / max) * (chartH - 4))
        const x = offsetX + i * (barWidth + gap)
        const y = chartH - barH
        return (
          <g key={i}>
            <rect x={x} y={y} width={barWidth} height={barH} rx={1} fill={color} opacity={0.8} />
            {labels && labels[i] && i % Math.ceil(values.length / 6) === 0 && (
              <text x={x + barWidth / 2} y={height - 2} textAnchor="middle" fontSize="8" fill="var(--text-secondary)" fontFamily={FONT}>
                {labels[i]}
              </text>
            )}
          </g>
        )
      })}
    </svg>
  )
}

export function AdminScreen({
  showToast, renderHeader, renderToast, renderBottomTabs, transitionStyle,
}: AdminScreenProps) {
  const [adminDays, setAdminDays] = useState(30)
  const [adminData, setAdminData] = useState<AdminData | null>(null)
  const [analyticsData, setAnalyticsData] = useState<AnalyticsData | null>(null)
  const [adminDeals, setAdminDeals] = useState<Deal[]>([])
  const [adminPayouts, setAdminPayouts] = useState<Payout[]>([])
  const [showDealForm, setShowDealForm] = useState(false)
  const [dealForm, setDealForm] = useState(EMPTY_FORM)
  const [editingDealId, setEditingDealId] = useState<number | null>(null)
  const [deepDiveDealId, setDeepDiveDealId] = useState<number | null>(null)

  // Tab navigation
  const [activeTab, setActiveTab] = useState<AdminTab>('overview')

  // L5 Business Learning state
  const [businessMetrics, setBusinessMetrics] = useState<BusinessMetricsData | null>(null)
  const [churnRisk, setChurnRisk] = useState<ChurnRiskData | null>(null)
  const [cohortData, setCohortData] = useState<CohortData | null>(null)
  const [marketplaceHealth, setMarketplaceHealth] = useState<MarketplaceHealthData | null>(null)
  const [businessLoading, setBusinessLoading] = useState(false)
  const [churnLoading, setChurnLoading] = useState(false)
  const [cohortLoading, setCohortLoading] = useState(false)
  const [marketplaceLoading, setMarketplaceLoading] = useState(false)
  const [feedbackItems, setFeedbackItems] = useState<FeedbackItem[] | null>(null)
  const [feedbackLoading, setFeedbackLoading] = useState(false)
  const [feedbackFilter, setFeedbackFilter] = useState<string>('all')
  const [unresolvedFeedbackCount, setUnresolvedFeedbackCount] = useState(0)

  // Original data fetching
  useEffect(() => {
    fetchAPI(`/api/admin/dashboard?days=${adminDays}`)
      .then(data => { if (data) setAdminData(data as unknown as AdminData) })
      .catch(e => console.warn('[load] admin dashboard failed:', e))
    fetchAPI(`/api/admin/analytics?days=${adminDays}`)
      .then(data => { if (data) setAnalyticsData(data as unknown as AnalyticsData) })
      .catch(e => console.warn('[load] admin analytics failed:', e))
    fetch('/api/admin/deals').then(r => r.ok ? r.json() : []).then(setAdminDeals).catch(() => {})
    fetch('/api/admin/payouts').then(r => r.ok ? r.json() : []).then(setAdminPayouts).catch(() => {})
  }, [adminDays])

  // Eagerly load unresolved feedback count for badge
  useEffect(() => {
    fetchAPI('/api/admin/feedback')
      .then(data => {
        if (Array.isArray(data)) {
          const items = data as unknown as FeedbackItem[]
          setUnresolvedFeedbackCount(items.filter(f => f.status === 'new').length)
        }
      })
      .catch(() => {})
  }, [])

  // Lazy-load L5 data when tab is selected
  useEffect(() => {
    if (activeTab === 'business' && !businessMetrics && !businessLoading) {
      setBusinessLoading(true)
      fetchAPI(`/api/admin/business-metrics?days=${adminDays}`)
        .then(data => { if (data) setBusinessMetrics(data as unknown as BusinessMetricsData) })
        .catch(e => console.warn('[load] business metrics failed:', e))
        .finally(() => setBusinessLoading(false))
    }
  }, [activeTab, adminDays, businessMetrics, businessLoading])

  useEffect(() => {
    if (activeTab === 'churn' && !churnRisk && !churnLoading) {
      setChurnLoading(true)
      fetchAPI('/api/admin/churn-risk')
        .then(data => { if (data) setChurnRisk(data as unknown as ChurnRiskData) })
        .catch(e => console.warn('[load] churn risk failed:', e))
        .finally(() => setChurnLoading(false))
    }
  }, [activeTab, churnRisk, churnLoading])

  useEffect(() => {
    if (activeTab === 'cohorts' && !cohortData && !cohortLoading) {
      setCohortLoading(true)
      fetchAPI('/api/admin/cohort-analysis')
        .then(data => { if (data) setCohortData(data as unknown as CohortData) })
        .catch(e => console.warn('[load] cohort analysis failed:', e))
        .finally(() => setCohortLoading(false))
    }
  }, [activeTab, cohortData, cohortLoading])

  useEffect(() => {
    if (activeTab === 'marketplace' && !marketplaceHealth && !marketplaceLoading) {
      setMarketplaceLoading(true)
      fetchAPI('/api/admin/marketplace-health')
        .then(data => { if (data) setMarketplaceHealth(data as unknown as MarketplaceHealthData) })
        .catch(e => console.warn('[load] marketplace health failed:', e))
        .finally(() => setMarketplaceLoading(false))
    }
  }, [activeTab, marketplaceHealth, marketplaceLoading])

  // Lazy-load feedback
  useEffect(() => {
    if (activeTab === 'feedback' && !feedbackItems && !feedbackLoading) {
      setFeedbackLoading(true)
      fetchAPI('/api/admin/feedback')
        .then(data => {
          if (data) {
            const items = data as unknown as FeedbackItem[]
            setFeedbackItems(items)
            setUnresolvedFeedbackCount(items.filter(f => f.status === 'new').length)
          }
        })
        .catch(e => console.warn('[load] feedback failed:', e))
        .finally(() => setFeedbackLoading(false))
    }
  }, [activeTab, feedbackItems, feedbackLoading])

  // Reset lazy-loaded data when time period changes
  useEffect(() => {
    setBusinessMetrics(null)
    setChurnRisk(null)
    setCohortData(null)
    setMarketplaceHealth(null)
  }, [adminDays])

  const dayOptions = [7, 30, 90]

  const inputStyle = { width: '100%', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', fontFamily: FONT, fontSize: '14px' }
  const labelStyle = { fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', display: 'block' as const, marginBottom: '2px' }
  const metricCard: React.CSSProperties = { backgroundColor: 'var(--surface)', borderRadius: '12px', padding: '16px', border: '1px solid var(--border)' }

  const TABS: { key: AdminTab; label: string }[] = [
    { key: 'overview', label: 'Overview' },
    { key: 'business', label: 'Metrics' },
    { key: 'churn', label: 'Churn' },
    { key: 'cohorts', label: 'Cohorts' },
    { key: 'marketplace', label: 'Health' },
    { key: 'feedback', label: 'Feedback' },
  ]

  // Color helpers
  const riskColor = (score: number) => score > 0.7 ? 'var(--error)' : score > 0.5 ? 'var(--warning)' : 'var(--brand)'
  const retentionColor = (pct: number) => pct > 50 ? 'var(--brand)' : pct > 25 ? 'var(--warning)' : 'var(--error)'
  const healthColor = (idx: number) => idx > 70 ? 'var(--brand)' : idx > 40 ? 'var(--warning)' : 'var(--error)'

  // =====================================================================
  // RENDER: Business Metrics tab
  // =====================================================================
  const renderBusinessMetrics = () => {
    if (businessLoading) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>Loading metrics...</div>
    if (!businessMetrics || businessMetrics.metrics.length === 0) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>No business metrics data available.</div>

    const metrics = businessMetrics.metrics
    const latest = metrics[metrics.length - 1]
    const dauValues = metrics.map(m => m.dau)
    const wauValues = metrics.map(m => m.wau)
    const mauValues = metrics.map(m => m.mau)
    const dateLabels = metrics.map(m => {
      const d = new Date(m.date)
      return `${d.getMonth() + 1}/${d.getDate()}`
    })

    return (
      <>
        {/* Key metrics cards */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '20px' }}>
          <div style={metricCard}>
            <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginBottom: '4px' }}>Acceptance Rate</div>
            <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: 'var(--text)' }}>
              {latest.acceptance_rate != null ? `${(latest.acceptance_rate * 100).toFixed(1)}%` : 'N/A'}
            </div>
          </div>
          <div style={metricCard}>
            <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginBottom: '4px' }}>Gross Revenue</div>
            <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: 'var(--text)' }}>
              {latest.gross_revenue_cents != null ? `$${(latest.gross_revenue_cents / 100).toFixed(2)}` : 'N/A'}
            </div>
          </div>
          <div style={metricCard}>
            <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginBottom: '4px' }}>Active Deals</div>
            <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: 'var(--text)' }}>
              {latest.active_deals ?? 'N/A'}
            </div>
          </div>
          <div style={metricCard}>
            <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginBottom: '4px' }}>Active Brands</div>
            <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: 'var(--text)' }}>
              {latest.active_brands ?? 'N/A'}
            </div>
          </div>
        </div>

        {/* DAU bar chart */}
        <div style={{ ...metricCard, marginBottom: '12px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)' }}>DAU (Daily Active Users)</div>
            <div style={{ fontFamily: FONT, fontSize: '20px', fontWeight: 700, color: 'var(--brand)' }}>{latest.dau}</div>
          </div>
          <BarChart values={dauValues} labels={dateLabels} width={280} height={70} color="var(--brand)" />
        </div>

        {/* WAU sparkline */}
        <div style={{ ...metricCard, marginBottom: '12px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)' }}>WAU (Weekly Active Users)</div>
            <div style={{ fontFamily: FONT, fontSize: '20px', fontWeight: 700, color: 'var(--brand)' }}>{latest.wau}</div>
          </div>
          <Sparkline values={wauValues} width={280} height={60} color="var(--brand)" />
        </div>

        {/* MAU sparkline */}
        <div style={{ ...metricCard, marginBottom: '12px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)' }}>MAU (Monthly Active Users)</div>
            <div style={{ fontFamily: FONT, fontSize: '20px', fontWeight: 700, color: 'var(--brand)' }}>{latest.mau}</div>
          </div>
          <Sparkline values={mauValues} width={280} height={60} color="var(--brand)" />
        </div>
      </>
    )
  }

  // =====================================================================
  // RENDER: Churn Risk tab
  // =====================================================================
  const renderChurnRisk = () => {
    if (churnLoading) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>Loading churn data...</div>
    if (!churnRisk || churnRisk.at_risk_users.length === 0) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>No at-risk users detected.</div>

    return (
      <>
        <div style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text-secondary)', marginBottom: '16px' }}>
          {churnRisk.count} user{churnRisk.count !== 1 ? 's' : ''} at risk of churning
        </div>

        {/* Table header */}
        <div style={{
          display: 'grid', gridTemplateColumns: '1fr 70px 80px 70px',
          gap: '8px', padding: '8px 0', borderBottom: '2px solid var(--border)',
          fontFamily: FONT, fontSize: '11px', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase' as const,
        }}>
          <div>Name</div>
          <div style={{ textAlign: 'center' }}>Risk</div>
          <div style={{ textAlign: 'center' }}>Top Signal</div>
          <div style={{ textAlign: 'right' }}>Inactive</div>
        </div>

        {churnRisk.at_risk_users.map(user => {
          const topSignal = user.signals && user.signals.length > 0
            ? user.signals.reduce((a, b) => a.value > b.value ? a : b)
            : null
          const daysSinceActive = topSignal?.days_since_active ??
            (user.signals?.find(s => s.days_since_active != null)?.days_since_active ?? null)

          return (
            <div key={user.user_id} style={{
              display: 'grid', gridTemplateColumns: '1fr 70px 80px 70px',
              gap: '8px', padding: '10px 0', borderBottom: '1px solid var(--border)',
              alignItems: 'center',
            }}>
              <div style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text)', fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const }}>
                {user.name || `User #${user.user_id}`}
              </div>
              <div style={{ textAlign: 'center' }}>
                <span style={{
                  fontFamily: FONT, fontSize: '13px', fontWeight: 700,
                  color: '#fff', backgroundColor: riskColor(user.composite_risk),
                  padding: '2px 8px', borderRadius: '10px', display: 'inline-block',
                }}>
                  {(user.composite_risk * 100).toFixed(0)}%
                </span>
              </div>
              <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', textAlign: 'center', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const }}>
                {topSignal?.type || '-'}
              </div>
              <div style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text)', textAlign: 'right', fontWeight: 500 }}>
                {daysSinceActive != null ? `${daysSinceActive}d` : '-'}
              </div>
            </div>
          )
        })}
      </>
    )
  }

  // =====================================================================
  // RENDER: Cohort Retention tab
  // =====================================================================
  const renderCohorts = () => {
    if (cohortLoading) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>Loading cohort data...</div>
    if (!cohortData || cohortData.cohorts.length === 0) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>No cohort data available.</div>

    return (
      <>
        {/* Table header */}
        <div style={{
          display: 'grid', gridTemplateColumns: '1fr 55px 60px 65px 55px',
          gap: '6px', padding: '8px 0', borderBottom: '2px solid var(--border)',
          fontFamily: FONT, fontSize: '10px', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase' as const,
        }}>
          <div>Cohort</div>
          <div style={{ textAlign: 'center' }}>Users</div>
          <div style={{ textAlign: 'center' }}>Retained</div>
          <div style={{ textAlign: 'center' }}>Rate</div>
          <div style={{ textAlign: 'right' }}>Swaps</div>
        </div>

        {cohortData.cohorts.map(cohort => {
          const retentionRate = cohort.user_count > 0
            ? (cohort.retained_count / cohort.user_count) * 100
            : 0
          const weekLabel = (() => {
            const d = new Date(cohort.cohort_week)
            return `${d.getMonth() + 1}/${d.getDate()}`
          })()

          return (
            <div key={cohort.cohort_week} style={{
              display: 'grid', gridTemplateColumns: '1fr 55px 60px 65px 55px',
              gap: '6px', padding: '10px 0', borderBottom: '1px solid var(--border)',
              alignItems: 'center',
            }}>
              <div style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text)', fontWeight: 500 }}>
                Wk {weekLabel}
              </div>
              <div style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text)', textAlign: 'center' }}>
                {cohort.user_count}
              </div>
              <div style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text)', textAlign: 'center' }}>
                {cohort.retained_count}
              </div>
              <div style={{ textAlign: 'center' }}>
                <span style={{
                  fontFamily: FONT, fontSize: '12px', fontWeight: 700,
                  color: '#fff', backgroundColor: retentionColor(retentionRate),
                  padding: '2px 8px', borderRadius: '10px', display: 'inline-block',
                }}>
                  {retentionRate.toFixed(0)}%
                </span>
              </div>
              <div style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text)', textAlign: 'right', fontWeight: 500 }}>
                {cohort.avg_swaps.toFixed(1)}
              </div>
            </div>
          )
        })}
      </>
    )
  }

  // =====================================================================
  // RENDER: Marketplace Health tab
  // =====================================================================
  const renderMarketplaceHealth = () => {
    if (marketplaceLoading) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>Loading marketplace health...</div>
    if (!marketplaceHealth || marketplaceHealth.health.length === 0) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>No marketplace health data available.</div>

    const latest = marketplaceHealth.health[marketplaceHealth.health.length - 1]
    const healthValues = marketplaceHealth.health.map(h => h.health_index)

    return (
      <>
        {/* Big health index */}
        <div style={{
          ...metricCard, textAlign: 'center' as const, marginBottom: '20px',
          padding: '24px 16px',
        }}>
          <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginBottom: '8px', textTransform: 'uppercase' as const, letterSpacing: '1px' }}>
            Marketplace Health Index
          </div>
          <div style={{
            fontFamily: FONT, fontSize: '56px', fontWeight: 800,
            color: healthColor(latest.health_index),
            lineHeight: 1,
          }}>
            {latest.health_index.toFixed(0)}
          </div>
          <div style={{
            fontFamily: FONT, fontSize: '13px', fontWeight: 600, marginTop: '8px',
            color: healthColor(latest.health_index),
          }}>
            {latest.health_index > 70 ? 'Healthy' : latest.health_index > 40 ? 'Fair' : 'Needs Attention'}
          </div>
        </div>

        {/* Sub-scores */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '10px', marginBottom: '20px' }}>
          {[
            { label: 'Supply', value: latest.supply_score },
            { label: 'Demand', value: latest.demand_score },
            { label: 'Balance', value: latest.balance_score },
          ].map(s => (
            <div key={s.label} style={{ ...metricCard, textAlign: 'center' as const, padding: '12px 8px' }}>
              <div style={{ fontFamily: FONT, fontSize: '11px', color: 'var(--text-secondary)', marginBottom: '4px' }}>{s.label}</div>
              <div style={{ fontFamily: FONT, fontSize: '22px', fontWeight: 700, color: healthColor(s.value) }}>
                {s.value.toFixed(0)}
              </div>
            </div>
          ))}
        </div>

        {/* Health index trend sparkline */}
        <div style={{ ...metricCard, marginBottom: '20px' }}>
          <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>Health Trend</div>
          <Sparkline values={healthValues} width={280} height={60} color={healthColor(latest.health_index)} />
        </div>

        {/* Daily breakdown table */}
        {marketplaceHealth.health.length > 1 && (
          <>
            <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>Daily Breakdown</div>
            <div style={{
              display: 'grid', gridTemplateColumns: '1fr 55px 55px 55px 55px',
              gap: '4px', padding: '8px 0', borderBottom: '2px solid var(--border)',
              fontFamily: FONT, fontSize: '10px', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase' as const,
            }}>
              <div>Date</div>
              <div style={{ textAlign: 'center' }}>Supply</div>
              <div style={{ textAlign: 'center' }}>Demand</div>
              <div style={{ textAlign: 'center' }}>Balance</div>
              <div style={{ textAlign: 'right' }}>Index</div>
            </div>
            {[...marketplaceHealth.health].reverse().slice(0, 14).map(h => (
              <div key={h.date} style={{
                display: 'grid', gridTemplateColumns: '1fr 55px 55px 55px 55px',
                gap: '4px', padding: '6px 0', borderBottom: '1px solid var(--border)',
                fontFamily: FONT, fontSize: '12px', color: 'var(--text)',
              }}>
                <div style={{ color: 'var(--text-secondary)' }}>
                  {(() => { const d = new Date(h.date); return `${d.getMonth() + 1}/${d.getDate()}` })()}
                </div>
                <div style={{ textAlign: 'center' }}>{h.supply_score.toFixed(0)}</div>
                <div style={{ textAlign: 'center' }}>{h.demand_score.toFixed(0)}</div>
                <div style={{ textAlign: 'center' }}>{h.balance_score.toFixed(0)}</div>
                <div style={{ textAlign: 'right', fontWeight: 600, color: healthColor(h.health_index) }}>
                  {h.health_index.toFixed(0)}
                </div>
              </div>
            ))}
          </>
        )}
      </>
    )
  }

  // --- Feedback triage helpers ---
  const statusBadge = (status: string) => {
    const colors: Record<string, { bg: string; fg: string }> = {
      new: { bg: 'var(--info-bg)', fg: 'var(--info)' },
      reviewed: { bg: 'var(--warning-bg)', fg: 'var(--warning)' },
      roadmapped: { bg: 'var(--info-bg)', fg: 'var(--info)' },
      done: { bg: 'var(--success-bg)', fg: 'var(--success)' },
    }
    const c = colors[status] || colors.new
    return (
      <span style={{
        display: 'inline-block', padding: '2px 8px', borderRadius: '10px',
        backgroundColor: c.bg, color: c.fg,
        fontFamily: FONT, fontSize: '11px', fontWeight: 600,
        textTransform: 'capitalize' as const,
      }}>{status}</span>
    )
  }

  const urgencyLabel = (urg: string) => {
    const colors: Record<string, string> = { critical: 'var(--error)', high: 'var(--warning)', normal: 'var(--text-secondary)', low: 'var(--text-secondary)' }
    return <span style={{ fontFamily: FONT, fontSize: '11px', fontWeight: urg === 'critical' || urg === 'high' ? 700 : 400, color: colors[urg] || 'var(--text-secondary)' }}>{urg}</span>
  }

  const handleTriageFeedback = async (id: number, status: string) => {
    const res = await fetchAPI(`/api/admin/feedback/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status }),
    })
    if (res) {
      setFeedbackItems(prev => {
        if (!prev) return prev
        const updated = prev.map(f => f.id === id ? { ...f, status, updated_at: new Date().toISOString() } : f)
        setUnresolvedFeedbackCount(updated.filter(f => f.status === 'new').length)
        return updated
      })
      showToast(`Feedback #${id} → ${status}`)
    }
  }

  const renderFeedback = () => {
    if (feedbackLoading) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>Loading feedback...</div>
    if (!feedbackItems || feedbackItems.length === 0) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>No feedback yet.</div>

    const filtered = feedbackFilter === 'all' ? feedbackItems : feedbackItems.filter(f => f.status === feedbackFilter)

    return (
      <>
        {/* Filter chips */}
        <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' as const, marginBottom: '16px' }}>
          {['all', 'new', 'reviewed', 'roadmapped', 'done'].map(f => (
            <button key={f} onClick={() => setFeedbackFilter(f)} style={{
              padding: '4px 10px', borderRadius: '12px', border: 'none',
              backgroundColor: feedbackFilter === f ? 'var(--brand)' : 'color-mix(in srgb, var(--border) 50%, transparent)',
              color: feedbackFilter === f ? '#fff' : 'var(--text)',
              fontFamily: FONT, fontSize: '12px', fontWeight: 600, cursor: 'pointer',
              textTransform: 'capitalize' as const,
            }}>{f} {f !== 'all' ? `(${feedbackItems.filter(i => i.status === f).length})` : `(${feedbackItems.length})`}</button>
          ))}
        </div>

        {/* Feedback items */}
        {filtered.map(f => (
          <div key={f.id} style={{
            ...metricCard, marginBottom: '10px', padding: '14px 16px',
            borderLeft: `3px solid ${f.urgency === 'critical' ? 'var(--error)' : f.urgency === 'high' ? 'var(--warning)' : 'var(--border)'}`,
          }}>
            {/* Header: type badge + urgency + status */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px', flexWrap: 'wrap' as const }}>
              <span style={{
                padding: '1px 6px', borderRadius: '4px', backgroundColor: 'color-mix(in srgb, var(--brand) 8%, transparent)',
                fontFamily: FONT, fontSize: '10px', fontWeight: 700, color: 'var(--brand)',
                textTransform: 'uppercase' as const,
              }}>{f.type}</span>
              {urgencyLabel(f.urgency)}
              {statusBadge(f.status)}
              <span style={{ marginLeft: 'auto', fontFamily: FONT, fontSize: '11px', color: 'var(--text-secondary)' }}>
                #{f.id}
              </span>
            </div>

            {/* Body */}
            <div style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text)', lineHeight: 1.5, marginBottom: '8px', whiteSpace: 'pre-wrap' as const }}>
              {f.body.length > 300 ? f.body.slice(0, 300) + '...' : f.body}
            </div>

            {/* Meta */}
            <div style={{ fontFamily: FONT, fontSize: '11px', color: 'var(--text-secondary)', marginBottom: '8px' }}>
              {f.user_name || `User #${f.user_id}`}
              {f.user_phone ? ` · ${f.user_phone}` : ''}
              {' · '}
              {new Date(f.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })}
              {f.github_issue_url && (
                <> · <a href={f.github_issue_url} target="_blank" rel="noopener noreferrer" style={{ color: 'var(--brand)' }}>GitHub</a></>
              )}
            </div>

            {/* Triage actions */}
            <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' as const }}>
              {f.status !== 'reviewed' && (
                <button onClick={() => handleTriageFeedback(f.id, 'reviewed')} style={{
                  padding: '4px 10px', borderRadius: '8px', border: '1px solid var(--border)',
                  backgroundColor: 'var(--surface)', fontFamily: FONT, fontSize: '12px', color: 'var(--text)', cursor: 'pointer',
                }}>Mark Reviewed</button>
              )}
              {f.status !== 'roadmapped' && (
                <button onClick={() => handleTriageFeedback(f.id, 'roadmapped')} style={{
                  padding: '4px 10px', borderRadius: '8px', border: '1px solid var(--brand)',
                  backgroundColor: 'color-mix(in srgb, var(--brand) 6%, transparent)', fontFamily: FONT, fontSize: '12px', color: 'var(--brand)', cursor: 'pointer',
                }}>Roadmap</button>
              )}
              {f.status !== 'done' && (
                <button onClick={() => handleTriageFeedback(f.id, 'done')} style={{
                  padding: '4px 10px', borderRadius: '8px', border: '1px solid var(--brand)',
                  backgroundColor: 'var(--brand)', fontFamily: FONT, fontSize: '12px', color: '#fff', cursor: 'pointer',
                }}>Done</button>
              )}
            </div>
          </div>
        ))}
      </>
    )
  }

  // =====================================================================
  // MAIN RENDER
  // =====================================================================
  return (
    <ScreenTransition style={transitionStyle}>
    <div style={pageStyles.page}>
      {renderHeader()}
      <div style={{ ...pageStyles.listArea, padding: '20px' }}>
        {/* Tab navigation */}
        <div style={{
          display: 'flex', gap: '4px', marginBottom: '16px',
          overflowX: 'auto', WebkitOverflowScrolling: 'touch',
          msOverflowStyle: 'none' as React.CSSProperties['msOverflowStyle'],
        }}>
          {TABS.map(tab => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              style={{
                padding: '6px 12px', borderRadius: '16px', border: 'none',
                backgroundColor: activeTab === tab.key ? 'var(--brand)' : 'color-mix(in srgb, var(--border) 50%, transparent)',
                color: activeTab === tab.key ? '#fff' : 'var(--text)',
                fontFamily: FONT, fontSize: '13px', fontWeight: 600,
                cursor: 'pointer', whiteSpace: 'nowrap' as const,
                flexShrink: 0,
                transition: 'background-color 0.2s, color 0.2s',
              }}
            >
              {tab.label}
              {tab.key === 'feedback' && unresolvedFeedbackCount > 0 && (
                <span style={{
                  marginLeft: '4px',
                  minWidth: '16px',
                  height: '16px',
                  borderRadius: '8px',
                  backgroundColor: activeTab === 'feedback' ? '#fff' : 'var(--error)',
                  color: activeTab === 'feedback' ? 'var(--brand)' : '#fff',
                  fontSize: '10px',
                  fontWeight: 700,
                  display: 'inline-flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  padding: '0 4px',
                }}>{unresolvedFeedbackCount}</span>
              )}
            </button>
          ))}
        </div>

        {/* Time period selector */}
        <div style={{ display: 'flex', gap: '8px', marginBottom: '20px' }}>
          {dayOptions.map(d => (
            <button key={d} onClick={() => setAdminDays(d)} style={{
              flex: 1, padding: '8px', borderRadius: '8px', border: '1px solid var(--border)',
              backgroundColor: adminDays === d ? 'var(--brand)' : 'var(--surface)', color: adminDays === d ? '#fff' : 'var(--text)',
              fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer',
            }}>{d}d</button>
          ))}
        </div>

        {/* --- L5 tabs --- */}
        {activeTab === 'business' && renderBusinessMetrics()}
        {activeTab === 'churn' && renderChurnRisk()}
        {activeTab === 'cohorts' && renderCohorts()}
        {activeTab === 'marketplace' && renderMarketplaceHealth()}
        {activeTab === 'feedback' && renderFeedback()}

        {/* --- Overview tab (original content) --- */}
        {activeTab === 'overview' && (
          <>
            {adminData ? (
              <>
                {/* Metric cards */}
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '32px' }}>
                  {[
                    { label: 'Active Users', value: adminData.active_users },
                    { label: 'Total Users', value: adminData.total_users },
                    { label: 'New Users', value: adminData.new_users },
                    { label: 'K-Factor', value: adminData.k_factor.toFixed(2) },
                  ].map(m => (
                    <div key={m.label} style={metricCard}>
                      <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginBottom: '4px' }}>{m.label}</div>
                      <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: 'var(--text)' }}>{m.value}</div>
                    </div>
                  ))}
                </div>

                {/* Leaderboard */}
                <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: 'var(--text)', marginBottom: '12px' }}>Referral Leaderboard</div>
                {adminData.leaderboard.length === 0 ? (
                  <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)' }}>No referrals yet.</div>
                ) : (
                  adminData.leaderboard.map((entry, i) => (
                    <div key={i} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 0', borderBottom: '1px solid var(--border)' }}>
                      <div>
                        <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text)' }}>{entry.name || entry.phone}</div>
                        {entry.name && <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)' }}>{entry.phone}</div>}
                      </div>
                      <div style={{ textAlign: 'right' }}>
                        <div style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: 'var(--brand)' }}>{entry.points} pts</div>
                        <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)' }}>{entry.referrals} referrals</div>
                      </div>
                    </div>
                  ))
                )}

                {/* NPS Responses */}
                <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: 'var(--text)', marginTop: '28px', marginBottom: '12px' }}>
                  NPS: {adminData.nps_score.toFixed(0)} ({adminData.nps_total} responses, {adminData.nps_promoters} promoters, {adminData.nps_detractors} detractors)
                </div>
                {adminData.nps_responses.length === 0 ? (
                  <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)' }}>No NPS responses yet.</div>
                ) : (
                  adminData.nps_responses.map((resp, i) => (
                    <div key={i} style={{ padding: '12px 0', borderBottom: '1px solid var(--border)' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <div style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: 'var(--text)' }}>Score: {resp.score}</div>
                        <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)' }}>{new Date(resp.created_at).toLocaleDateString()}</div>
                      </div>
                      {resp.comment && <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text)', marginTop: '4px' }}>{resp.comment}</div>}
                      <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginTop: '2px' }}>{resp.user_name || resp.user_phone}</div>
                    </div>
                  ))
                )}

                {/* Swap Stats */}
                {(adminData.swap_offered != null && adminData.swap_offered > 0) && (
                  <>
                    <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: 'var(--text)', marginTop: '28px', marginBottom: '12px' }}>Swap Deals</div>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '20px' }}>
                      {[
                        { label: 'Offered', value: adminData.swap_offered },
                        { label: 'Accepted', value: adminData.swap_accepted },
                        { label: 'Declined', value: adminData.swap_declined },
                        { label: 'Accept Rate', value: `${(adminData.swap_accept_rate || 0).toFixed(1)}%` },
                      ].map(m => (
                        <div key={m.label} style={metricCard}>
                          <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginBottom: '4px' }}>{m.label}</div>
                          <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: 'var(--text)' }}>{m.value}</div>
                        </div>
                      ))}
                    </div>
                  </>
                )}

                {/* Payout Checklist */}
                {adminPayouts.length > 0 && (
                  <>
                    <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: 'var(--text)', marginTop: '28px', marginBottom: '12px' }}>Pending Payouts</div>
                    {adminPayouts.map(p => (
                      <div key={p.user_id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 0', borderBottom: '1px solid var(--border)' }}>
                        <div style={{ flex: 1, minWidth: 0 }}>
                          <div style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text)', fontWeight: 500 }}>{p.name || 'No name'} — ${(p.total_cents / 100).toFixed(2)}</div>
                          <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)' }}>{p.phone || p.email || 'No contact'} · {p.swap_count} swap{p.swap_count !== 1 ? 's' : ''} · since {new Date(p.oldest_swap).toLocaleDateString()}</div>
                        </div>
                        <button
                          onClick={async () => {
                            const res = await fetch('/api/admin/payouts', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ user_id: p.user_id }) })
                            if (res.ok) {
                              setAdminPayouts(prev => prev.filter(x => x.user_id !== p.user_id))
                              showToast(`Marked $${(p.total_cents / 100).toFixed(2)} as paid for ${p.name || 'user'}`)
                            } else { showToast('Failed to mark as paid', 'error') }
                          }}
                          style={{ fontSize: '12px', padding: '6px 12px', borderRadius: '6px', border: '1px solid var(--brand)', backgroundColor: 'var(--surface)', cursor: 'pointer', fontFamily: FONT, color: 'var(--brand)', fontWeight: 600, flexShrink: 0 }}
                        >Mark Paid</button>
                      </div>
                    ))}
                  </>
                )}

                {/* Deal Management */}
                <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: 'var(--text)', marginTop: '28px', marginBottom: '12px' }}>
                  Manage Deals
                  <button
                    onClick={() => { setShowDealForm(true); setEditingDealId(null); setDealForm(EMPTY_FORM) }}
                    style={{ marginLeft: '12px', backgroundColor: 'var(--brand)', color: '#fff', border: 'none', borderRadius: '6px', padding: '4px 12px', fontFamily: FONT, fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}
                  >+ Add</button>
                </div>

                {showDealForm && (
                  <div style={{ backgroundColor: 'var(--surface)', borderRadius: '12px', padding: '16px', border: '1px solid var(--border)', marginBottom: '16px' }}>
                    <div style={{ marginBottom: '8px' }}>
                      <label style={labelStyle}>Deal Type</label>
                      <select
                        value={dealForm.deal_type}
                        onChange={e => setDealForm(prev => ({ ...prev, deal_type: e.target.value, quantity_offered: e.target.value === 'bogo' ? 2 : 1 }))}
                        style={inputStyle}
                      >
                        <option value="sku_swap">SKU Swap</option>
                        <option value="bogo">BOGO (Buy One Get One)</option>
                        <option value="trial">Trial (Free Sample)</option>
                        <option value="seasonal">Seasonal / Limited-Time</option>
                      </select>
                    </div>
                    {([
                      { label: 'Competitor Product', key: 'competitor_product' as const, type: 'text' },
                      { label: 'Offer Brand', key: 'offer_brand' as const, type: 'text' },
                      { label: 'Offer Product', key: 'offer_product' as const, type: 'text' },
                      { label: 'Discount (cents)', key: 'discount_cents' as const, type: 'number' },
                      { label: 'Total Budget (cents)', key: 'daily_budget_cents' as const, type: 'number' },
                      { label: 'Offer Price (cents)', key: 'offer_price_cents' as const, type: 'number' },
                      { label: 'Competitor Price (cents)', key: 'competitor_price_cents' as const, type: 'number' },
                      { label: 'Offer Size', key: 'offer_size' as const, type: 'text' },
                      { label: 'Competitor Size', key: 'competitor_size' as const, type: 'text' },
                      { label: 'Offer Unit Price', key: 'offer_unit_price' as const, type: 'text' },
                      { label: 'Competitor Unit Price', key: 'competitor_unit_price' as const, type: 'text' },
                      { label: 'Why Bullets (JSON array)', key: 'why_bullets' as const, type: 'text' },
                      { label: 'Allergen Notes', key: 'allergen_notes' as const, type: 'text' },
                    ] as const).map(f => (
                      <div key={f.key} style={{ marginBottom: '8px' }}>
                        <label style={labelStyle}>{f.label}</label>
                        <input
                          type={f.type}
                          value={dealForm[f.key]}
                          onChange={e => setDealForm(prev => ({ ...prev, [f.key]: f.type === 'number' ? parseInt(e.target.value) || 0 : e.target.value }))}
                          style={inputStyle}
                        />
                      </div>
                    ))}
                    {dealForm.deal_type === 'bogo' && (
                      <div style={{ marginBottom: '8px' }}>
                        <label style={labelStyle}>Quantity Offered</label>
                        <input type="number" value={dealForm.quantity_offered} onChange={e => setDealForm(prev => ({ ...prev, quantity_offered: parseInt(e.target.value) || 2 }))} style={inputStyle} />
                      </div>
                    )}
                    {dealForm.deal_type === 'trial' && (
                      <>
                        <div style={{ marginBottom: '8px' }}>
                          <label style={labelStyle}>Trial Product Name</label>
                          <input type="text" value={dealForm.trial_product_name} onChange={e => setDealForm(prev => ({ ...prev, trial_product_name: e.target.value }))} style={inputStyle} />
                        </div>
                        <div style={{ marginBottom: '8px' }}>
                          <label style={labelStyle}>Trial Size</label>
                          <input type="text" value={dealForm.trial_size} onChange={e => setDealForm(prev => ({ ...prev, trial_size: e.target.value }))} style={inputStyle} />
                        </div>
                      </>
                    )}
                    {dealForm.deal_type === 'seasonal' && (
                      <div style={{ marginBottom: '8px' }}>
                        <label style={labelStyle}>Campaign Tag</label>
                        <input type="text" value={dealForm.campaign_tag} onChange={e => setDealForm(prev => ({ ...prev, campaign_tag: e.target.value }))} placeholder="e.g. holiday_2026" style={inputStyle} />
                      </div>
                    )}
                    <div style={{ display: 'flex', gap: '8px', marginTop: '12px' }}>
                      <button
                        onClick={async () => {
                          const method = editingDealId ? 'PUT' : 'POST'
                          const body = editingDealId ? { ...dealForm, id: editingDealId } : dealForm
                          const res = await fetch('/api/admin/deals', { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
                          if (res.ok) {
                            const dealsRes = await fetch('/api/admin/deals')
                            if (dealsRes.ok) setAdminDeals(await dealsRes.json())
                            setShowDealForm(false)
                            showToast(editingDealId ? 'Deal updated' : 'Deal created')
                          } else { showToast('Failed to save deal', 'error') }
                        }}
                        style={{ flex: 1, backgroundColor: 'var(--brand)', color: '#fff', border: 'none', borderRadius: '8px', padding: '10px', fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer' }}
                      >{editingDealId ? 'Update' : 'Create'}</button>
                      <button onClick={() => setShowDealForm(false)} style={{ padding: '10px 16px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--surface)', fontFamily: FONT, fontSize: '15px', cursor: 'pointer', color: 'var(--text)' }}>Cancel</button>
                    </div>
                  </div>
                )}

                {adminDeals.map(deal => (
                  <div key={deal.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 0', borderBottom: '1px solid var(--border)' }}>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text)', fontWeight: 500, display: 'flex', alignItems: 'center', gap: '6px' }}>
                        {deal.competitor_product} → {deal.offer_brand} {deal.offer_product}
                        {deal.deal_type && deal.deal_type !== 'sku_swap' && (
                          <span style={{ fontSize: '10px', fontWeight: 700, color: '#fff', backgroundColor: deal.deal_type === 'bogo' ? 'var(--warning)' : deal.deal_type === 'trial' ? 'var(--success)' : 'var(--error)', padding: '1px 5px', borderRadius: '3px', textTransform: 'uppercase' as const }}>{deal.deal_type}</span>
                        )}
                      </div>
                      <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)' }}>
                        ${(deal.discount_cents / 100).toFixed(2)} off · Budget: ${(deal.daily_budget_cents / 100).toFixed(2)} · Spent: ${(deal.budget_spent_cents / 100).toFixed(2)} · {deal.status}
                      </div>
                    </div>
                    <div style={{ display: 'flex', gap: '6px', flexShrink: 0 }}>
                      <button
                        onClick={() => setDeepDiveDealId(deal.id)}
                        style={{ fontSize: '12px', padding: '4px 8px', borderRadius: '6px', border: '1px solid var(--brand)', backgroundColor: 'var(--surface)', cursor: 'pointer', fontFamily: FONT, color: 'var(--brand)', fontWeight: 600 }}
                      >Deep Dive</button>
                      <button
                        onClick={async () => {
                          const newStatus = deal.status === 'active' ? 'paused' : 'active'
                          await fetch('/api/admin/deals', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...deal, status: newStatus }) })
                          const res = await fetch('/api/admin/deals')
                          if (res.ok) setAdminDeals(await res.json())
                        }}
                        style={{ fontSize: '12px', padding: '4px 8px', borderRadius: '6px', border: '1px solid var(--border)', backgroundColor: 'var(--surface)', cursor: 'pointer', fontFamily: FONT, color: 'var(--text)' }}
                      >{deal.status === 'active' ? 'Pause' : 'Resume'}</button>
                      <button
                        onClick={() => {
                          setEditingDealId(deal.id)
                          setDealForm({
                            competitor_product: deal.competitor_product, offer_brand: deal.offer_brand, offer_product: deal.offer_product,
                            discount_cents: deal.discount_cents, daily_budget_cents: deal.daily_budget_cents, status: deal.status, ends_at: deal.ends_at,
                            offer_price_cents: deal.offer_price_cents, competitor_price_cents: deal.competitor_price_cents,
                            offer_size: deal.offer_size, competitor_size: deal.competitor_size,
                            offer_unit_price: deal.offer_unit_price, competitor_unit_price: deal.competitor_unit_price,
                            why_bullets: deal.why_bullets, allergen_notes: deal.allergen_notes,
                            deal_type: deal.deal_type || 'sku_swap', quantity_offered: deal.quantity_offered || 1,
                            trial_product_name: deal.trial_product_name || '', trial_size: deal.trial_size || '', campaign_tag: deal.campaign_tag || '',
                          })
                          setShowDealForm(true)
                        }}
                        style={{ fontSize: '12px', padding: '4px 8px', borderRadius: '6px', border: '1px solid var(--border)', backgroundColor: 'var(--surface)', cursor: 'pointer', fontFamily: FONT, color: 'var(--text)' }}
                      >Edit</button>
                      <button
                        onClick={async () => {
                          await fetch('/api/admin/deals', { method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id: deal.id }) })
                          const res = await fetch('/api/admin/deals')
                          if (res.ok) setAdminDeals(await res.json())
                        }}
                        style={{ fontSize: '12px', padding: '4px 8px', borderRadius: '6px', border: '1px solid color-mix(in srgb, var(--error) 25%, transparent)', backgroundColor: 'var(--error-bg)', cursor: 'pointer', fontFamily: FONT, color: 'var(--error)' }}
                      >Del</button>
                    </div>
                  </div>
                ))}

                {/* Item Analytics */}
                {analyticsData && (
                  <>
                    <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: 'var(--text)', marginTop: '28px', marginBottom: '12px' }}>Item Analytics</div>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '20px' }}>
                      <div style={metricCard}>
                        <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginBottom: '4px' }}>Items Created</div>
                        <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: 'var(--text)' }}>{analyticsData.items_created}</div>
                      </div>
                      <div style={metricCard}>
                        <div style={{ fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', marginBottom: '4px' }}>Total Events</div>
                        <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: 'var(--text)' }}>{Object.values(analyticsData.events_by_action).reduce((a, b) => a + b, 0)}</div>
                      </div>
                    </div>

                    {Object.keys(analyticsData.items_by_source).length > 0 && (
                      <>
                        <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>By Source</div>
                        {Object.entries(analyticsData.items_by_source).sort((a, b) => b[1] - a[1]).map(([src, cnt]) => (
                          <div key={src} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: '1px solid var(--border)' }}>
                            <span style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text)' }}>{src}</span>
                            <span style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)' }}>{cnt}</span>
                          </div>
                        ))}
                      </>
                    )}

                    {Object.keys(analyticsData.items_by_department).length > 0 && (
                      <>
                        <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)', marginTop: '16px', marginBottom: '8px' }}>Top Departments</div>
                        {Object.entries(analyticsData.items_by_department).sort((a, b) => b[1] - a[1]).slice(0, 10).map(([dept, cnt]) => (
                          <div key={dept} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: '1px solid var(--border)' }}>
                            <span style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text)' }}>{dept}</span>
                            <span style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)' }}>{cnt}</span>
                          </div>
                        ))}
                      </>
                    )}

                    {Object.keys(analyticsData.events_by_action).length > 0 && (
                      <>
                        <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)', marginTop: '16px', marginBottom: '8px' }}>Events by Action</div>
                        {Object.entries(analyticsData.events_by_action).sort((a, b) => b[1] - a[1]).map(([action, cnt]) => (
                          <div key={action} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: '1px solid var(--border)' }}>
                            <span style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text)' }}>{action}</span>
                            <span style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)' }}>{cnt}</span>
                          </div>
                        ))}
                      </>
                    )}

                    {analyticsData.events_by_day.length > 0 && (
                      <>
                        <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: 'var(--text)', marginTop: '16px', marginBottom: '8px' }}>Daily Activity</div>
                        {analyticsData.events_by_day.slice(-14).map(d => (
                          <div key={d.date} style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0' }}>
                            <span style={{ fontFamily: FONT, fontSize: '13px', color: 'var(--text-secondary)' }}>{d.date}</span>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                              <div style={{ width: `${Math.min(120, Math.max(4, d.count * 3))}px`, height: '8px', backgroundColor: 'var(--brand)', borderRadius: '4px' }} />
                              <span style={{ fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: 'var(--text)', minWidth: '24px', textAlign: 'right' }}>{d.count}</span>
                            </div>
                          </div>
                        ))}
                      </>
                    )}
                  </>
                )}
              </>
            ) : (
              <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>Loading...</div>
            )}
          </>
        )}
      </div>
      {renderToast()}
      {renderBottomTabs()}
      {deepDiveDealId !== null && (
        <BrandDealDeepDive dealId={deepDiveDealId} onClose={() => setDeepDiveDealId(null)} />
      )}
    </div>
    </ScreenTransition>
  )
}
