import { useState, useEffect } from 'react'
import {
  BRAND, TEXT, TEXT_SECONDARY, BORDER, WHITE, FONT, BG,
  RADIUS_LG, SHADOW_LG, Z_MODAL, ERROR_RED,
} from '../styles'

// --- API response types ---

interface PerformanceDay {
  date: string
  impressions: number
  accepts: number
  declines: number
  revenue_cents: number
  verified_count: number
}

interface OptimizeMetrics {
  offered: number
  accepted: number
  declined: number
  accept_rate: number
  budget_utilization: number
  category_avg_rate: number
}

interface DeclineReason {
  reason: string
  count: number
}

interface Recommendation {
  priority: 'high' | 'medium' | 'low'
  category: string
  title: string
  description: string
}

interface PerformanceData {
  deal_id: number
  performance: PerformanceDay[]
}

interface OptimizeData {
  deal_id: number
  metrics: OptimizeMetrics
  decline_reasons: DeclineReason[]
  recommendations: Recommendation[]
}

// --- Component props ---

interface BrandDealDeepDiveProps {
  dealId: number
  onClose: () => void
}

// --- Priority badge colors ---

const PRIORITY_COLORS: Record<string, { bg: string; text: string }> = {
  high:   { bg: '#FEE2E2', text: '#C53030' },
  medium: { bg: '#FEF3C7', text: '#92400E' },
  low:    { bg: '#DBEAFE', text: '#1E40AF' },
}

export function BrandDealDeepDive({ dealId, onClose }: BrandDealDeepDiveProps) {
  const [perfData, setPerfData] = useState<PerformanceData | null>(null)
  const [optData, setOptData] = useState<OptimizeData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)

    Promise.all([
      fetch(`/brand/deal-performance/${dealId}`).then(r => {
        if (!r.ok) throw new Error(`Performance API returned ${r.status}`)
        return r.json() as Promise<PerformanceData>
      }),
      fetch(`/brand/optimize/${dealId}`).then(r => {
        if (!r.ok) throw new Error(`Optimize API returned ${r.status}`)
        return r.json() as Promise<OptimizeData>
      }),
    ])
      .then(([perf, opt]) => {
        if (cancelled) return
        setPerfData(perf)
        setOptData(opt)
        setLoading(false)
      })
      .catch(err => {
        if (cancelled) return
        setError(err.message || 'Failed to load deal data')
        setLoading(false)
      })

    return () => { cancelled = true }
  }, [dealId])

  // --- Shared styles ---

  const card: React.CSSProperties = {
    backgroundColor: WHITE,
    borderRadius: RADIUS_LG,
    border: `1px solid ${BORDER}`,
    padding: '16px',
    marginBottom: '16px',
  }

  const sectionTitle: React.CSSProperties = {
    fontFamily: FONT,
    fontSize: '15px',
    fontWeight: 700,
    color: TEXT,
    marginBottom: '12px',
  }

  // --- Render helpers ---

  function renderPerformanceChart(days: PerformanceDay[]) {
    // Show last 30 days (or fewer if less data)
    const data = days.slice(-30)
    if (data.length === 0) {
      return <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY }}>No performance data yet.</div>
    }

    const maxImpressions = Math.max(...data.map(d => d.impressions), 1)
    const barWidth = Math.max(4, Math.floor((100 / data.length) * 0.7))
    const gapWidth = Math.max(1, Math.floor((100 / data.length) * 0.3))
    const chartHeight = 140
    const svgWidth = data.length * (barWidth + gapWidth)

    return (
      <div style={{ overflowX: 'auto', paddingBottom: '4px' }}>
        <svg
          width={Math.max(svgWidth, 280)}
          height={chartHeight + 28}
          viewBox={`0 0 ${Math.max(svgWidth, 280)} ${chartHeight + 28}`}
          style={{ display: 'block' }}
        >
          {data.map((day, i) => {
            const x = i * (barWidth + gapWidth)
            const impressionH = (day.impressions / maxImpressions) * chartHeight
            const acceptH = (day.accepts / maxImpressions) * chartHeight

            return (
              <g key={day.date}>
                {/* Impressions bar (full, lighter) */}
                <rect
                  x={x}
                  y={chartHeight - impressionH}
                  width={barWidth}
                  height={impressionH}
                  rx={2}
                  fill={`${BRAND}30`}
                />
                {/* Accepts bar (inner, darker) */}
                <rect
                  x={x}
                  y={chartHeight - acceptH}
                  width={barWidth}
                  height={acceptH}
                  rx={2}
                  fill={BRAND}
                />
                {/* Date label every ~5 bars */}
                {(i % Math.max(1, Math.floor(data.length / 6)) === 0) && (
                  <text
                    x={x + barWidth / 2}
                    y={chartHeight + 16}
                    textAnchor="middle"
                    fontSize="9"
                    fontFamily={FONT}
                    fill={TEXT_SECONDARY}
                  >
                    {day.date.slice(5)}
                  </text>
                )}
              </g>
            )
          })}
        </svg>
        <div style={{ display: 'flex', gap: '16px', marginTop: '8px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <div style={{ width: '12px', height: '12px', borderRadius: '2px', backgroundColor: `${BRAND}30` }} />
            <span style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>Impressions</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <div style={{ width: '12px', height: '12px', borderRadius: '2px', backgroundColor: BRAND }} />
            <span style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>Accepts</span>
          </div>
        </div>
      </div>
    )
  }

  function renderDeclineReasons(reasons: DeclineReason[]) {
    if (reasons.length === 0) {
      return <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY }}>No decline data recorded.</div>
    }

    const maxCount = Math.max(...reasons.map(r => r.count), 1)

    return (
      <div>
        {reasons.map((r, i) => (
          <div key={i} style={{ marginBottom: '10px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '3px' }}>
              <span style={{ fontFamily: FONT, fontSize: '13px', color: TEXT }}>{r.reason}</span>
              <span style={{ fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: TEXT }}>{r.count}</span>
            </div>
            <div style={{ width: '100%', height: '8px', backgroundColor: `${BORDER}`, borderRadius: '4px', overflow: 'hidden' }}>
              <div style={{
                width: `${(r.count / maxCount) * 100}%`,
                height: '100%',
                backgroundColor: ERROR_RED,
                borderRadius: '4px',
                transition: 'width 0.3s ease',
              }} />
            </div>
          </div>
        ))}
      </div>
    )
  }

  function renderRecommendations(recs: Recommendation[]) {
    if (recs.length === 0) {
      return <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT_SECONDARY }}>No recommendations at this time.</div>
    }

    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
        {recs.map((rec, i) => {
          const priorityColor = PRIORITY_COLORS[rec.priority] || PRIORITY_COLORS.low
          return (
            <div key={i} style={{
              backgroundColor: BG,
              borderRadius: RADIUS_LG,
              padding: '14px',
              border: `1px solid ${BORDER}`,
            }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px', flexWrap: 'wrap' }}>
                {/* Priority badge */}
                <span style={{
                  fontFamily: FONT,
                  fontSize: '10px',
                  fontWeight: 700,
                  textTransform: 'uppercase',
                  color: priorityColor.text,
                  backgroundColor: priorityColor.bg,
                  padding: '2px 8px',
                  borderRadius: '999px',
                }}>
                  {rec.priority}
                </span>
                {/* Category tag */}
                <span style={{
                  fontFamily: FONT,
                  fontSize: '10px',
                  fontWeight: 600,
                  color: TEXT_SECONDARY,
                  backgroundColor: WHITE,
                  border: `1px solid ${BORDER}`,
                  padding: '2px 8px',
                  borderRadius: '999px',
                }}>
                  {rec.category}
                </span>
              </div>
              <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT, marginBottom: '4px' }}>
                {rec.title}
              </div>
              <div style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY, lineHeight: '1.4' }}>
                {rec.description}
              </div>
            </div>
          )
        })}
      </div>
    )
  }

  function renderMetrics(metrics: OptimizeMetrics) {
    const stats = [
      {
        label: 'Accept Rate',
        value: `${metrics.accept_rate.toFixed(1)}%`,
        detail: `${metrics.accepted} of ${metrics.offered}`,
      },
      {
        label: 'Budget Used',
        value: `${metrics.budget_utilization.toFixed(0)}%`,
        detail: null,
      },
      {
        label: 'Category Avg',
        value: `${metrics.category_avg_rate.toFixed(1)}%`,
        detail: 'accept rate',
      },
      {
        label: 'Declined',
        value: `${metrics.declined}`,
        detail: null,
      },
    ]

    return (
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px' }}>
        {stats.map(s => {
          // Highlight accept rate vs category avg
          let valueColor = TEXT
          if (s.label === 'Accept Rate' && metrics.accept_rate >= metrics.category_avg_rate) {
            valueColor = '#276749' // success green
          } else if (s.label === 'Accept Rate' && metrics.accept_rate < metrics.category_avg_rate) {
            valueColor = ERROR_RED
          }

          return (
            <div key={s.label} style={{
              backgroundColor: WHITE,
              borderRadius: RADIUS_LG,
              padding: '14px',
              border: `1px solid ${BORDER}`,
            }}>
              <div style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY, marginBottom: '4px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                {s.label}
              </div>
              <div style={{ fontFamily: FONT, fontSize: '22px', fontWeight: 700, color: valueColor }}>
                {s.value}
              </div>
              {s.detail && (
                <div style={{ fontFamily: FONT, fontSize: '11px', color: TEXT_SECONDARY, marginTop: '2px' }}>
                  {s.detail}
                </div>
              )}
            </div>
          )
        })}
      </div>
    )
  }

  // --- Main render ---

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: Z_MODAL,
        backgroundColor: 'rgba(0,0,0,0.5)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '16px',
      }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div style={{
        width: '100%',
        maxWidth: '520px',
        maxHeight: '90vh',
        backgroundColor: BG,
        borderRadius: '16px',
        boxShadow: SHADOW_LG,
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
      }}>
        {/* Header */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '16px 20px',
          backgroundColor: WHITE,
          borderBottom: `1px solid ${BORDER}`,
          flexShrink: 0,
        }}>
          <div style={{ fontFamily: FONT, fontSize: '17px', fontWeight: 700, color: TEXT }}>
            Deal Deep Dive
          </div>
          <button
            onClick={onClose}
            style={{
              width: '32px',
              height: '32px',
              borderRadius: '999px',
              border: `1px solid ${BORDER}`,
              backgroundColor: BG,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              cursor: 'pointer',
              fontFamily: FONT,
              fontSize: '16px',
              color: TEXT_SECONDARY,
              lineHeight: 1,
              padding: 0,
            }}
            aria-label="Close deep dive"
          >
            &times;
          </button>
        </div>

        {/* Scrollable body */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '16px 20px 24px' }}>
          {loading && (
            <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY, textAlign: 'center', padding: '40px 0' }}>
              Loading deal analytics...
            </div>
          )}

          {error && (
            <div style={{ fontFamily: FONT, fontSize: '15px', color: ERROR_RED, textAlign: 'center', padding: '40px 0' }}>
              {error}
            </div>
          )}

          {!loading && !error && optData && perfData && (
            <>
              {/* Key Metrics Row */}
              <div style={{ ...card, padding: '0', border: 'none', backgroundColor: 'transparent' }}>
                <div style={sectionTitle}>Key Metrics</div>
                {renderMetrics(optData.metrics)}
              </div>

              {/* Performance Chart */}
              <div style={card}>
                <div style={sectionTitle}>Daily Performance (last 30 days)</div>
                {renderPerformanceChart(perfData.performance)}
              </div>

              {/* Decline Reasons */}
              <div style={card}>
                <div style={sectionTitle}>Decline Reasons</div>
                {renderDeclineReasons(optData.decline_reasons)}
              </div>

              {/* Recommendations */}
              <div style={{ ...card, border: 'none', backgroundColor: 'transparent', padding: '0' }}>
                <div style={sectionTitle}>Recommendations</div>
                {renderRecommendations(optData.recommendations)}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
