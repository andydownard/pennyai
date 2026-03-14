import { FONT } from '../../styles'
import { ChurnRiskData, riskColor } from './types'

interface ChurnRiskTabProps {
  loading: boolean
  data: ChurnRiskData | null
}

export function ChurnRiskTab({ loading, data }: ChurnRiskTabProps) {
  if (loading) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>Loading churn data...</div>
  if (!data || data.at_risk_users.length === 0) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>No at-risk users detected.</div>

  return (
    <>
      <div style={{ fontFamily: FONT, fontSize: '14px', color: 'var(--text-secondary)', marginBottom: '16px' }}>
        {data.count} user{data.count !== 1 ? 's' : ''} at risk of churning
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

      {data.at_risk_users.map(user => {
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
