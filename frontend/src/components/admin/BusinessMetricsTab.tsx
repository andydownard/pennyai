import { FONT } from '../../styles'
import { Sparkline, BarChart } from './Sparkline'
import { BusinessMetricsData, metricCard } from './types'

interface BusinessMetricsTabProps {
  loading: boolean
  data: BusinessMetricsData | null
}

export function BusinessMetricsTab({ loading, data }: BusinessMetricsTabProps) {
  if (loading) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>Loading metrics...</div>
  if (!data || data.metrics.length === 0) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>No business metrics data available.</div>

  const metrics = data.metrics
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
