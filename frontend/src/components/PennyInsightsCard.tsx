import React, { useEffect, useState } from 'react'
import { RADIUS_LG, SHADOW_SM, FONT } from '../styles'

interface InsightEntry {
  value: string
  confidence: number
}

interface InsightsData {
  user_learnings: Record<string, InsightEntry>
  household_learnings: Record<string, InsightEntry>
  patterns: {
    active_days_30d: number
    top_categories: string[] | null
    brand_affinities: Array<{ category: string; brand: string; picks: number; streak: number }> | null
  }
  swap_behavior: {
    offered: number
    accepted: number
    declined: number
    accept_rate: number
  }
}

interface PennyInsightsCardProps {
  fetchAPI: (path: string, opts?: RequestInit) => Promise<Record<string, unknown> | unknown[] | null>
  onClose: () => void
  track: (event: string, props?: Record<string, unknown>) => void
}

const INSIGHT_LABELS: Record<string, string> = {
  shopping_frequency: 'Shopping frequency',
  price_sensitivity: 'Price sensitivity',
  preferred_shopping_day: 'Preferred day',
  items_per_month: 'Items per month',
  avg_accepted_discount_cents: 'Avg swap savings',
  coordination_score: 'Household coordination',
  active_members: 'Active members',
}

export function PennyInsightsCard({ fetchAPI, onClose, track }: PennyInsightsCardProps) {
  const [data, setData] = useState<InsightsData | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchAPI('/api/penny/insights')
      .then(d => {
        if (d && !Array.isArray(d)) {
          setData(d as unknown as InsightsData)
          track('penny_insights_viewed', {
            learnings_count: Object.keys((d as Record<string, unknown>).user_learnings as object || {}).length,
          })
        }
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <div style={styles.container}>
        <div style={styles.header}>
          <span style={styles.title}>What Penny Knows</span>
          <button onClick={onClose} style={styles.closeBtn}>&times;</button>
        </div>
        <div style={styles.body}>
          <div style={{ color: 'var(--text-secondary)', fontSize: 14, fontFamily: FONT }}>Loading...</div>
        </div>
      </div>
    )
  }

  if (!data) return null

  const userLearnings = data.user_learnings || {}
  const patterns = data.patterns || { active_days_30d: 0, top_categories: null, brand_affinities: null }
  const swapBehavior = data.swap_behavior || { offered: 0, accepted: 0, declined: 0, accept_rate: 0 }
  const hasLearnings = Object.keys(userLearnings).length > 0 || patterns.active_days_30d > 0 || swapBehavior.offered > 0

  return (
    <div style={styles.container}>
      <div style={styles.header}>
        <span style={styles.title}>What Penny Knows</span>
        <button onClick={onClose} style={styles.closeBtn}>&times;</button>
      </div>
      <div style={styles.body}>
        {!hasLearnings ? (
          <p style={styles.emptyText}>
            Penny is still learning about you! As you use your list and respond to swaps,
            she'll build up insights to help you shop smarter.
          </p>
        ) : (
          <>
            {/* User learnings */}
            {Object.keys(userLearnings).length > 0 && (
              <div style={styles.section}>
                <div style={styles.sectionTitle}>Your Preferences</div>
                {Object.entries(userLearnings).map(([key, entry]) => {
                  const label = key.startsWith('repurchase_cycle:')
                    ? `Buys ${key.replace('repurchase_cycle:', '')} every`
                    : (INSIGHT_LABELS[key] || key.replace(/_/g, ' '))
                  const value = key === 'avg_accepted_discount_cents'
                    ? `$${(Number(entry.value) / 100).toFixed(2)}`
                    : entry.value
                  return (
                    <div key={key} style={styles.insightRow}>
                      <span style={styles.insightLabel}>{label}</span>
                      <span style={styles.insightValue}>{value}</span>
                    </div>
                  )
                })}
              </div>
            )}

            {/* Brand affinities */}
            {patterns.brand_affinities && patterns.brand_affinities.length > 0 && (
              <div style={styles.section}>
                <div style={styles.sectionTitle}>Brand Preferences</div>
                {patterns.brand_affinities.map((b, i) => (
                  <div key={i} style={styles.insightRow}>
                    <span style={styles.insightLabel}>{b.category}</span>
                    <span style={styles.insightValue}>
                      {b.brand} {b.streak >= 3 ? '🔥' : ''}
                    </span>
                  </div>
                ))}
              </div>
            )}

            {/* Top categories */}
            {patterns.top_categories && patterns.top_categories.length > 0 && (
              <div style={styles.section}>
                <div style={styles.sectionTitle}>Top Categories</div>
                <div style={styles.chips}>
                  {patterns.top_categories.map((cat, i) => (
                    <span key={i} style={styles.chip}>{cat}</span>
                  ))}
                </div>
              </div>
            )}

            {/* Swap behavior */}
            {swapBehavior.offered > 0 && (
              <div style={styles.section}>
                <div style={styles.sectionTitle}>Swap Activity</div>
                <div style={styles.insightRow}>
                  <span style={styles.insightLabel}>Swaps seen</span>
                  <span style={styles.insightValue}>{swapBehavior.offered}</span>
                </div>
                <div style={styles.insightRow}>
                  <span style={styles.insightLabel}>Accepted</span>
                  <span style={styles.insightValue}>
                    {swapBehavior.accepted} ({swapBehavior.accept_rate}%)
                  </span>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    background: 'var(--surface)',
    borderRadius: RADIUS_LG,
    boxShadow: SHADOW_SM,
    border: '1px solid var(--border)',
    margin: '12px 16px',
    overflow: 'hidden',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: '12px 16px',
    background: 'color-mix(in srgb, var(--brand) 8%, transparent)',
    borderBottom: '1px solid var(--border)',
  },
  title: {
    fontSize: 15,
    fontWeight: 600,
    color: 'var(--brand)',
    fontFamily: FONT,
  },
  closeBtn: {
    background: 'none',
    border: 'none',
    fontSize: 20,
    color: 'var(--text-secondary)',
    cursor: 'pointer',
    padding: '0 4px',
    lineHeight: 1,
  },
  body: {
    padding: '12px 16px',
  },
  emptyText: {
    color: 'var(--text-secondary)',
    fontSize: 14,
    lineHeight: 1.5,
    fontFamily: FONT,
    margin: 0,
  },
  section: {
    marginBottom: 16,
  },
  sectionTitle: {
    fontSize: 12,
    fontWeight: 600,
    color: 'var(--text-secondary)',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.5px',
    marginBottom: 8,
    fontFamily: FONT,
  },
  insightRow: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: '6px 0',
    borderBottom: '1px solid var(--bg)',
  },
  insightLabel: {
    fontSize: 14,
    color: 'var(--text)',
    fontFamily: FONT,
    textTransform: 'capitalize' as const,
  },
  insightValue: {
    fontSize: 14,
    fontWeight: 500,
    color: 'var(--brand)',
    fontFamily: FONT,
  },
  chips: {
    display: 'flex',
    flexWrap: 'wrap' as const,
    gap: 6,
  },
  chip: {
    fontSize: 12,
    padding: '4px 10px',
    borderRadius: '999px',
    background: 'color-mix(in srgb, var(--brand) 8%, transparent)',
    color: 'var(--brand)',
    fontFamily: FONT,
    fontWeight: 500,
  },
}
