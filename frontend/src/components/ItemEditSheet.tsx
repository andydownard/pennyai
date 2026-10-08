import { useEffect, useRef } from 'react'
import { BrandLogo } from './BrandLogo'
import { FloatingLabelField } from './FloatingLabelField'
import { SectionHeader } from './ActionSheet'
import { type ListItem } from './ItemRow'
import { DEPARTMENTS, FONT, FONT_SM, layout, buttons, text } from '../styles'
import { type ItemModification } from '../types'

type PriorityLevel = 'low' | 'normal' | 'urgent'

interface EditForm {
  name: string
  brand: string
  dept: string
  size: string
  qty: number
  notes: string
}

interface ItemEditSheetProps {
  item: ListItem
  editForm: EditForm
  setEditForm: React.Dispatch<React.SetStateAction<EditForm>>
  displayBrand: string
  setDisplayBrand: (b: string) => void
  editLoading: boolean
  currentUserId: number | null
  itemHistory: ItemModification[]
  onSave: () => void
  onDelete: () => void
  onClose: () => void
  onUrgentToggle: (id: number) => void
  onClaim: (id: number) => void
  setSelectedItem: React.Dispatch<React.SetStateAction<ListItem | null>>
  openConfirmModal: (opts: { title: string; message?: string; confirmLabel?: string; destructive?: boolean; onConfirm: () => void }) => void
  listStyles: {
    overlay: React.CSSProperties
    floatingInput: React.CSSProperties
    floatingSelect: React.CSSProperties
    sheetRow: React.CSSProperties
    sheetButton: React.CSSProperties
    sheetDeleteBtn: React.CSSProperties
  }
  ToggleSwitch: React.ComponentType<{ value: boolean; onChange: () => void; label?: string }>
  QuantityControl: React.ComponentType<{ value: number; onChange: (v: number) => void }>
}

export function ItemEditSheet({
  item, editForm, setEditForm, displayBrand, setDisplayBrand,
  editLoading, currentUserId, itemHistory,
  onSave, onDelete, onClose, onUrgentToggle, onClaim,
  setSelectedItem, openConfirmModal, listStyles,
  ToggleSwitch, QuantityControl,
}: ItemEditSheetProps) {
  // Focus-restore: capture trigger element on mount, restore on unmount
  const triggerRef = useRef<Element | null>(null)
  useEffect(() => {
    triggerRef.current = document.activeElement
    return () => {
      if (triggerRef.current && triggerRef.current instanceof HTMLElement) {
        triggerRef.current.focus()
      }
    }
  }, [])

  return (
    <div style={listStyles.overlay} onClick={() => onClose()} role="presentation">
      <div role="dialog" aria-modal="true" aria-label="Edit item" style={{
        ...layout.column, width: '100%', height: '100dvh', backgroundColor: 'var(--surface)',
      }} onClick={e => e.stopPropagation()}>
        {/* Header */}
        <div style={{ ...layout.row, padding: '16px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
          <button onClick={() => onClose()} aria-label="Close edit item" style={{ ...buttons.reset, padding: '4px', display: 'flex', color: 'var(--text)' }}>
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="M15 5L5 15M5 5l10 10" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round"/></svg>
          </button>
          <span style={{ ...text.heading, flex: 1, textAlign: 'center' }}>Edit item</span>
          <div style={{ width: '28px' }} />
        </div>
        {/* Scrollable content */}
        <div style={{ ...layout.column, flex: 1, minHeight: 0, overflowY: 'auto' as const, overscrollBehavior: 'contain' as const, WebkitOverflowScrolling: 'touch' as any, padding: '20px', gap: '14px' }}>
        {/* Brand field with logo */}
        <div style={{ display: 'flex', alignItems: 'flex-end', gap: '12px' }}>
          <div style={{ flex: 1 }}>
            <FloatingLabelField label="Brand">
              <input
                type="text"
                value={editForm.brand}
                onChange={e => setEditForm(f => ({ ...f, brand: e.target.value }))}
                onBlur={() => setDisplayBrand(editForm.brand)}
                placeholder="Optional"
                style={listStyles.floatingInput}
              />
            </FloatingLabelField>
          </div>
          <BrandLogo brand={displayBrand || undefined} size={52} itemName={editForm.name} department={editForm.dept} emoji={item.emoji} />
        </div>

        <FloatingLabelField label="Name">
          <input
            type="text"
            value={editForm.name}
            onChange={e => setEditForm(f => ({ ...f, name: e.target.value }))}
            placeholder="Item name"
            style={listStyles.floatingInput}
            autoFocus
          />
        </FloatingLabelField>

        <FloatingLabelField label="Department">
          <select
            value={editForm.dept}
            onChange={e => setEditForm(f => ({ ...f, dept: e.target.value }))}
            style={listStyles.floatingSelect}
          >
            {DEPARTMENTS.map(d => <option key={d} value={d}>{d}</option>)}
          </select>
        </FloatingLabelField>

        <div style={listStyles.sheetRow}>
          <div style={{ flex: 1 }}>
            <FloatingLabelField label="Size">
              <input
                type="text"
                list="size-suggestions"
                value={editForm.size}
                onChange={e => setEditForm(f => ({ ...f, size: e.target.value }))}
                placeholder="e.g. 16 oz, 1 lb"
                style={listStyles.floatingInput}
              />
            </FloatingLabelField>
            <datalist id="size-suggestions">
              {['4 oz','8 oz','12 oz','16 oz','24 oz','32 oz','1 lb','2 lb','5 lb','½ gal','1 gal','Small','Medium','Large','XL'].map(s => (
                <option key={s} value={s} />
              ))}
            </datalist>
          </div>
          <div>
            <FloatingLabelField label="Qty">
              <QuantityControl value={editForm.qty} onChange={(v: number) => setEditForm(f => ({ ...f, qty: v }))} />
            </FloatingLabelField>
          </div>
        </div>

        <FloatingLabelField label="Notes">
          <input
            type="text"
            value={editForm.notes}
            onChange={e => setEditForm(f => ({ ...f, notes: e.target.value }))}
            placeholder="e.g. Tangerine flavor, organic"
            style={listStyles.floatingInput}
          />
        </FloatingLabelField>

        {/* Priority selector: low / normal / urgent */}
        <div style={{ padding: '12px 0', borderTop: '1px solid var(--border)', marginTop: '4px' }}>
          <span style={{ ...text.body, display: 'block', marginBottom: '8px' }}>Priority</span>
          <div style={{ display: 'flex', gap: '0px', borderRadius: '8px', overflow: 'hidden', border: '1px solid var(--border)' }}>
            {(['low', 'normal', 'urgent'] as PriorityLevel[]).map((level) => {
              const currentPriority: PriorityLevel = item.urgent ? 'urgent' : 'normal'
              const isActive = currentPriority === level
              const labelMap: Record<PriorityLevel, string> = { low: 'Low', normal: 'Normal', urgent: 'Urgent' }
              const emojiMap: Record<PriorityLevel, string> = { low: '↓', normal: '—', urgent: '🔥' }
              const activeBgMap: Record<PriorityLevel, string> = {
                low: 'var(--info-bg)',
                normal: 'var(--brand)',
                urgent: 'var(--error)',
              }
              const activeColorMap: Record<PriorityLevel, string> = {
                low: 'var(--info)',
                normal: '#fff',
                urgent: '#fff',
              }
              return (
                <button
                  key={level}
                  onClick={() => {
                    // Map to backend boolean: only 'urgent' = true
                    const shouldBeUrgent = level === 'urgent'
                    if (item.urgent !== shouldBeUrgent) {
                      onUrgentToggle(item.id)
                      setSelectedItem(prev => prev ? { ...prev, urgent: shouldBeUrgent } : prev)
                    }
                  }}
                  style={{
                    flex: 1,
                    padding: '8px 4px',
                    fontFamily: FONT,
                    fontSize: FONT_SM,
                    fontWeight: isActive ? 600 : 400,
                    color: isActive ? activeColorMap[level] : 'var(--text-secondary)',
                    backgroundColor: isActive ? activeBgMap[level] : 'var(--surface)',
                    border: 'none',
                    borderRight: level !== 'urgent' ? '1px solid var(--border)' : 'none',
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    gap: '4px',
                    transition: 'background-color 0.15s, color 0.15s',
                  }}
                  aria-label={`Set priority to ${level}`}
                  aria-pressed={isActive}
                >
                  <span>{emojiMap[level]}</span>
                  <span>{labelMap[level]}</span>
                </button>
              )
            })}
          </div>
        </div>

        {/* Claim UI */}
        <div style={{ ...layout.rowBetween, padding: '12px 0', borderBottom: '1px solid var(--border)' }}>
          <div style={layout.rowGap}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={item.claimed_by_id ? 'var(--brand)' : 'var(--text)'} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="9" cy="21" r="1"/><circle cx="20" cy="21" r="1"/>
              <path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"/>
            </svg>
            <span style={text.body}>
              {!item.claimed_by_id ? "I'll get it" : item.claimed_by_id === currentUserId ? "You're getting this" : `${item.claimed_by_name} is getting this`}
            </span>
          </div>
          {(!item.claimed_by_id || item.claimed_by_id === currentUserId) && (
            <ToggleSwitch
              value={item.claimed_by_id === currentUserId}
              onChange={() => {
                const wasClaimed = item.claimed_by_id === currentUserId
                onClaim(item.id)
                setSelectedItem(prev => prev ? { ...prev, claimed_by_id: wasClaimed ? null : currentUserId, claimed_by_name: wasClaimed ? '' : 'Me' } : prev)
              }}
            />
          )}
        </div>

        {itemHistory.length > 0 && (
          <div style={{ marginTop: '12px', paddingTop: '12px', borderTop: '1px solid var(--border)' }}>
            <SectionHeader label="History" />
            {itemHistory.map((mod, i) => {
              const name = mod.user_name || 'Someone'
              const via = mod.source === 'email' ? ' via email' : ''
              const ago = mod.created_at ? (() => {
                const diff = Date.now() - new Date(mod.created_at).getTime()
                const mins = Math.floor(diff / 60000)
                if (mins < 1) return 'just now'
                if (mins < 60) return `${mins}m ago`
                const hrs = Math.floor(mins / 60)
                if (hrs < 24) return `${hrs}h ago`
                const days = Math.floor(hrs / 24)
                if (days < 7) return `${days}d ago`
                return new Date(mod.created_at).toLocaleDateString()
              })() : ''
              let description: string
              switch (mod.action) {
                case 'added': description = `${name} added this${via}`; break
                case 'checked': description = `${name} checked this off${via}`; break
                case 'unchecked': description = `${name} unchecked this${via}`; break
                case 'updated': description = mod.detail ? `${name} changed ${mod.detail}${via}` : `${name} updated this${via}`; break
                case 'claimed': description = `${name} claimed this`; break
                case 'unclaimed': description = `${name} unclaimed this`; break
                default: description = `${name} ${mod.action}${mod.detail ? ` ${mod.detail}` : ''}${via}`
              }
              return (
                <div key={i} style={{
                  ...text.caption,
                  padding: '6px 0',
                  borderBottom: i < itemHistory.length - 1 ? '1px solid var(--border)' : 'none',
                }}>
                  {description}
                  {ago && <span> · {ago}</span>}
                </div>
              )
            })}
          </div>
        )}
        </div>
        {/* Anchored bottom buttons */}
        <div style={{ ...layout.columnGap, flexShrink: 0, padding: '16px 20px', paddingBottom: 'max(16px, env(safe-area-inset-bottom))', borderTop: '1px solid var(--border)' }}>
          <button
            onClick={onSave}
            disabled={!editForm.name.trim() || editLoading}
            style={{
              ...listStyles.sheetButton,
              opacity: editForm.name.trim() && !editLoading ? 1 : 0.4,
            }}
          >
            {editLoading ? 'Saving…' : 'Save'}
          </button>
          <button onClick={() => openConfirmModal({ title: 'Delete Item', message: 'Remove this item from your list?', confirmLabel: 'Delete', destructive: true, onConfirm: onDelete })} disabled={editLoading} style={listStyles.sheetDeleteBtn}>
            Delete item
          </button>
        </div>
      </div>
    </div>
  )
}
