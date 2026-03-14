import { FONT } from '../../styles'
import { Sparkline } from './Sparkline'
import { MarketplaceHealthData, metricCard, healthColor } from './types'

interface MarketplaceHealthTabProps {
  loading: boolean
  data: MarketplaceHealthData | null
}

export function MarketplaceHealthTab({ loading, data }: MarketplaceHealthTabProps) {
  if (loading) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>Loading marketplace health...</div>
  if (!data || data.health.length === 0) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>No marketplace health data available.</div>

  const latest = data.health[data.health.length - 1]
  const healthValues = data.health.map(h => h.health_index)

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
      {data.health.length > 1 && (
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
          {[...data.health].reverse().slice(0, 14).map(h => (
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
