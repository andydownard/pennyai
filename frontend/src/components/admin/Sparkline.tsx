import { FONT } from '../../styles'

export function Sparkline({ values, width = 280, height = 60, color = 'var(--brand)' }: { values: number[]; width?: number; height?: number; color?: string }) {
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

export function BarChart({ values, labels, width = 280, height = 80, color = 'var(--brand)' }: { values: number[]; labels?: string[]; width?: number; height?: number; color?: string }) {
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
