import { FONT } from '../../styles'
import { FeedbackItem, metricCard } from './types'
import { fetchAPI } from '../../api'

interface FeedbackTabProps {
  loading: boolean
  items: FeedbackItem[] | null
  filter: string
  onFilterChange: (filter: string) => void
  onItemsChange: (updater: (prev: FeedbackItem[] | null) => FeedbackItem[] | null) => void
  onUnresolvedCountChange: (count: number) => void
  showToast: (msg: string, type?: 'success' | 'error') => void
}

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

export function FeedbackTab({ loading, items, filter, onFilterChange, onItemsChange, onUnresolvedCountChange, showToast }: FeedbackTabProps) {
  if (loading) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>Loading feedback...</div>
  if (!items || items.length === 0) return <div style={{ fontFamily: FONT, fontSize: '15px', color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0' }}>No feedback yet.</div>

  const filtered = filter === 'all' ? items : items.filter(f => f.status === filter)

  const handleTriageFeedback = async (id: number, status: string) => {
    const res = await fetchAPI(`/api/admin/feedback/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status }),
    })
    if (res) {
      onItemsChange(prev => {
        if (!prev) return prev
        const updated = prev.map(f => f.id === id ? { ...f, status, updated_at: new Date().toISOString() } : f)
        onUnresolvedCountChange(updated.filter(f => f.status === 'new').length)
        return updated
      })
      showToast(`Feedback #${id} → ${status}`)
    }
  }

  return (
    <>
      {/* Filter chips */}
      <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' as const, marginBottom: '16px' }}>
        {['all', 'new', 'reviewed', 'roadmapped', 'done'].map(f => (
          <button key={f} onClick={() => onFilterChange(f)} style={{
            padding: '4px 10px', borderRadius: '12px', border: 'none',
            backgroundColor: filter === f ? 'var(--brand)' : 'color-mix(in srgb, var(--border) 50%, transparent)',
            color: filter === f ? '#fff' : 'var(--text)',
            fontFamily: FONT, fontSize: '12px', fontWeight: 600, cursor: 'pointer',
            textTransform: 'capitalize' as const,
          }}>{f} {f !== 'all' ? `(${items.filter(i => i.status === f).length})` : `(${items.length})`}</button>
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
