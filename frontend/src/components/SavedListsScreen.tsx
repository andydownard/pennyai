import { useState, useEffect, useRef, useCallback } from 'react'
import { FONT, FONT_BASE, FONT_SM, FONT_XS, FONT_2XS, FONT_LG, pageStyles } from '../styles'
import { PennyAvatar } from './PennyAvatar'
import { ConfirmModal } from './ConfirmModal'
import { ScreenTransition } from './ScreenTransition'

interface SavedList {
  id: number
  name: string
  store_name?: string
  item_count: number
  is_master: boolean
  created_at: string
  updated_at: string
}

interface SavedListsScreenProps {
  renderHeader: () => React.ReactNode
  renderToast: () => React.ReactNode
  transitionStyle: React.CSSProperties
  onShowToast: (msg: string, type?: 'success' | 'error') => void
  onNavigateHome: () => void
}

function timeAgo(dateStr: string): string {
  const now = Date.now()
  const then = new Date(dateStr).getTime()
  const diffMs = now - then
  const diffMin = Math.floor(diffMs / 60000)
  if (diffMin < 1) return 'just now'
  if (diffMin < 60) return `${diffMin}m ago`
  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr}h ago`
  const diffDay = Math.floor(diffHr / 24)
  if (diffDay < 30) return `${diffDay}d ago`
  const diffMonth = Math.floor(diffDay / 30)
  return `${diffMonth}mo ago`
}

export function SavedListsScreen({
  renderHeader,
  renderToast,
  transitionStyle,
  onShowToast,
  onNavigateHome,
}: SavedListsScreenProps) {
  const [lists, setLists] = useState<SavedList[]>([])
  const [loading, setLoading] = useState(true)
  const [loadConfirm, setLoadConfirm] = useState<SavedList | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState<SavedList | null>(null)

  // Long press detection
  const longPressTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const longPressTriggeredRef = useRef(false)

  const fetchLists = useCallback(async () => {
    try {
      const res = await fetch('/api/saved-lists')
      if (res.ok) {
        const data = await res.json()
        setLists(data || [])
      }
    } catch {
      // silently fail
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchLists()
  }, [fetchLists])

  const handleLoad = async (list: SavedList) => {
    try {
      const res = await fetch(`/api/saved-lists/${list.id}/load`, { method: 'POST' })
      if (res.ok) {
        onShowToast(`Loaded ${list.name}!`)
        onNavigateHome()
      } else {
        const data = await res.json().catch(() => null)
        onShowToast(data?.error || 'Could not load list', 'error')
      }
    } catch {
      onShowToast('Connection error', 'error')
    }
    setLoadConfirm(null)
  }

  const handleDelete = async (list: SavedList) => {
    try {
      const res = await fetch(`/api/saved-lists/${list.id}`, { method: 'DELETE' })
      if (res.ok) {
        setLists(prev => prev.filter(l => l.id !== list.id))
        onShowToast(`Deleted ${list.name}`)
      } else {
        onShowToast('Could not delete list', 'error')
      }
    } catch {
      onShowToast('Connection error', 'error')
    }
    setDeleteConfirm(null)
  }

  const handleTouchStart = (list: SavedList) => {
    longPressTriggeredRef.current = false
    longPressTimerRef.current = setTimeout(() => {
      longPressTriggeredRef.current = true
      setDeleteConfirm(list)
    }, 500)
  }

  const handleTouchEnd = () => {
    if (longPressTimerRef.current) {
      clearTimeout(longPressTimerRef.current)
      longPressTimerRef.current = null
    }
  }

  const handleTap = (list: SavedList) => {
    if (longPressTriggeredRef.current) return
    setLoadConfirm(list)
  }

  return (
    <ScreenTransition style={transitionStyle}>
    <div style={pageStyles.page}>
      {/* Confirm modals */}
      <ConfirmModal
        open={!!loadConfirm}
        title={`Load ${loadConfirm?.name || ''}?`}
        message="Load this list into your shopping list?"
        confirmLabel="Load"
        onConfirm={() => loadConfirm && handleLoad(loadConfirm)}
        onCancel={() => setLoadConfirm(null)}
      />
      <ConfirmModal
        open={!!deleteConfirm}
        title={`Delete ${deleteConfirm?.name || ''}?`}
        message="This can't be undone."
        confirmLabel="Delete"
        destructive
        onConfirm={() => deleteConfirm && handleDelete(deleteConfirm)}
        onCancel={() => setDeleteConfirm(null)}
      />

      {renderHeader()}

      <div style={pageStyles.listArea}>
        {loading ? (
          // Skeleton loading state
          <div style={{ padding: '16px 20px' }}>
            {[1, 2, 3].map(i => (
              <div key={i} style={{
                padding: '16px 0',
                borderBottom: '1px solid var(--border)',
                display: 'flex',
                alignItems: 'center',
                gap: '12px',
              }}>
                <div style={{
                  width: '36px', height: '36px', borderRadius: '8px',
                  backgroundColor: 'var(--skeleton)',
                }} />
                <div style={{ flex: 1 }}>
                  <div style={{
                    width: '60%', height: '14px', borderRadius: '4px',
                    backgroundColor: 'var(--skeleton)', marginBottom: '6px',
                  }} />
                  <div style={{
                    width: '40%', height: '10px', borderRadius: '4px',
                    backgroundColor: 'var(--skeleton)',
                  }} />
                </div>
              </div>
            ))}
          </div>
        ) : lists.length === 0 ? (
          // Empty state
          <div style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            padding: '60px 32px',
            textAlign: 'center',
            gap: '16px',
          }}>
            <PennyAvatar size={80} />
            <p style={{
              fontFamily: FONT,
              fontSize: FONT_LG,
              fontWeight: 600,
              color: 'var(--text)',
              margin: '8px 0 0',
            }}>No saved lists yet</p>
            <p style={{
              fontFamily: FONT,
              fontSize: FONT_SM,
              color: 'var(--text-secondary)',
              margin: 0,
              lineHeight: 1.5,
            }}>
              Tell Penny to save your current list — just say "save my list as Weekly Groceries"
            </p>
          </div>
        ) : (
          // List of saved lists
          <div style={{ padding: '0 20px' }}>
            {lists.map(list => (
              <button
                key={list.id}
                onClick={() => handleTap(list)}
                onTouchStart={() => handleTouchStart(list)}
                onTouchEnd={handleTouchEnd}
                onTouchCancel={handleTouchEnd}
                onContextMenu={(e) => {
                  e.preventDefault()
                  setDeleteConfirm(list)
                }}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '12px',
                  width: '100%',
                  padding: '16px 0',
                  borderBottom: '1px solid var(--border)',
                  background: 'none',
                  border: 'none',
                  borderBottomWidth: '1px',
                  borderBottomStyle: 'solid',
                  borderBottomColor: 'var(--border)',
                  cursor: 'pointer',
                  textAlign: 'left',
                  fontFamily: FONT,
                }}
              >
                {/* List icon */}
                <span style={{ fontSize: '1.5rem', lineHeight: 1, flexShrink: 0 }}>
                  {'\uD83D\uDCCB'}
                </span>

                {/* List info */}
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{
                    fontSize: FONT_BASE,
                    fontWeight: 600,
                    color: 'var(--text)',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}>
                    {list.name}
                  </div>
                  <div style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    marginTop: '2px',
                  }}>
                    {list.store_name && (
                      <span style={{
                        fontSize: FONT_2XS,
                        color: 'var(--brand)',
                        fontWeight: 500,
                      }}>{list.store_name}</span>
                    )}
                    <span style={{
                      fontSize: FONT_2XS,
                      color: 'var(--text-secondary)',
                    }}>
                      {list.item_count} {list.item_count === 1 ? 'item' : 'items'}
                    </span>
                  </div>
                </div>

                {/* Timestamp */}
                <span style={{
                  fontSize: FONT_2XS,
                  color: 'var(--text-secondary)',
                  flexShrink: 0,
                  whiteSpace: 'nowrap',
                }}>
                  {timeAgo(list.updated_at)}
                </span>

                {/* Chevron */}
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0, opacity: 0.5 }}>
                  <polyline points="9 18 15 12 9 6"/>
                </svg>
              </button>
            ))}

            {/* Hint at bottom */}
            <p style={{
              fontFamily: FONT,
              fontSize: FONT_XS,
              color: 'var(--text-secondary)',
              textAlign: 'center',
              margin: '24px 0',
              lineHeight: 1.5,
            }}>
              Long-press a list to delete it
            </p>
          </div>
        )}
      </div>

      {renderToast()}
    </div>
    </ScreenTransition>
  )
}
