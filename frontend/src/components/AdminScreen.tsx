import { useState, useEffect } from 'react'
import { ScreenTransition } from './ScreenTransition'
import { BrandDealDeepDive } from './BrandDealDeepDive'
import { fetchAPI } from '../api'
import {
  FONT,
  pageStyles,
} from '../styles'
import {
  BusinessMetricsTab,
  ChurnRiskTab,
  CohortsTab,
  MarketplaceHealthTab,
  FeedbackTab,
  OverviewTab,
} from './admin'
import type {
  AdminData, AnalyticsData, Deal, Payout,
  BusinessMetricsData, ChurnRiskData, CohortData,
  MarketplaceHealthData, FeedbackItem, AdminTab,
} from './admin'

interface AdminScreenProps {
  showToast: (msg: string, type?: 'success' | 'error') => void
  renderHeader: () => React.ReactNode
  renderToast: () => React.ReactNode
  renderBottomTabs: () => React.ReactNode
  transitionStyle: React.CSSProperties
}

const TABS: { key: AdminTab; label: string }[] = [
  { key: 'overview', label: 'Overview' },
  { key: 'business', label: 'Metrics' },
  { key: 'churn', label: 'Churn' },
  { key: 'cohorts', label: 'Cohorts' },
  { key: 'marketplace', label: 'Health' },
  { key: 'feedback', label: 'Feedback' },
]

export function AdminScreen({
  showToast, renderHeader, renderToast, renderBottomTabs, transitionStyle,
}: AdminScreenProps) {
  const [adminDays, setAdminDays] = useState(30)
  const [adminData, setAdminData] = useState<AdminData | null>(null)
  const [analyticsData, setAnalyticsData] = useState<AnalyticsData | null>(null)
  const [adminDeals, setAdminDeals] = useState<Deal[]>([])
  const [adminPayouts, setAdminPayouts] = useState<Payout[]>([])
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

        {/* Tab content */}
        {activeTab === 'business' && <BusinessMetricsTab loading={businessLoading} data={businessMetrics} />}
        {activeTab === 'churn' && <ChurnRiskTab loading={churnLoading} data={churnRisk} />}
        {activeTab === 'cohorts' && <CohortsTab loading={cohortLoading} data={cohortData} />}
        {activeTab === 'marketplace' && <MarketplaceHealthTab loading={marketplaceLoading} data={marketplaceHealth} />}
        {activeTab === 'feedback' && (
          <FeedbackTab
            loading={feedbackLoading}
            items={feedbackItems}
            filter={feedbackFilter}
            onFilterChange={setFeedbackFilter}
            onItemsChange={setFeedbackItems}
            onUnresolvedCountChange={setUnresolvedFeedbackCount}
            showToast={showToast}
          />
        )}
        {activeTab === 'overview' && (
          <OverviewTab
            adminData={adminData}
            analyticsData={analyticsData}
            deals={adminDeals}
            payouts={adminPayouts}
            onDealsChange={setAdminDeals}
            onPayoutsChange={setAdminPayouts}
            onDeepDive={setDeepDiveDealId}
            showToast={showToast}
          />
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
