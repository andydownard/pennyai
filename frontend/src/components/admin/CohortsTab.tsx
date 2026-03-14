import { FONT } from '../../styles'
import { CohortData, retentionColor } from './types'

interface CohortsTabProps {
  loading: boolean
  data: CohortData | null
}

export function CohortsTab({ loading, data }: CohortsTabProps) {
  if (loading) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>Loading cohort data...</div>
  if (!data || data.cohorts.length === 0) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>No cohort data available.</div>

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

      {data.cohorts.map(cohort => {
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
