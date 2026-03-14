import { useState, useRef, useCallback } from 'react'
import { fetchAPI, fetchAction } from '../api'
import type { ListItem } from '../components/ItemRow'
import type { SwapOffer } from '../components/SwapCard'
import type { ItemModification } from '../types'
import type { SkippedItem } from './useSwapState'

// Haptic feedback (matches App.tsx)
const haptic = {
  tap:     () => navigator.vibrate?.([10]),
  heavy:   () => navigator.vibrate?.([20]),
  success: () => navigator.vibrate?.([10, 50, 5, 50, 15]),
  double:  () => navigator.vibrate?.([5, 30, 5]),
}

interface UseListStateOptions {
  currentUserId: number | null
  // Callbacks
  showToast: (msg: string, type?: 'success' | 'error') => void
  showChimeIn: (msg: string, duration?: number) => void
  typePennyHome: (msg: string, btns?: { label: string; onClick: () => void }[]) => void
  // Swap state (from useSwapState)
  setSwapsByItemId: React.Dispatch<React.SetStateAction<Record<number, SwapOffer>>>
  // Review state (from useSwapState)
  lastAdded: ListItem[]
  setLastAdded: React.Dispatch<React.SetStateAction<ListItem[]>>
  lastSkipped: SkippedItem[]
  setLastSkipped: React.Dispatch<React.SetStateAction<SkippedItem[]>>
  setShowReviewSheet: React.Dispatch<React.SetStateAction<boolean>>
  // Done celebration state
  setShowDoneToast: React.Dispatch<React.SetStateAction<boolean>>
  setShowFireworks: React.Dispatch<React.SetStateAction<boolean>>
  setShowDoneOverlay: React.Dispatch<React.SetStateAction<boolean>>
  setPennyChimeIn: React.Dispatch<React.SetStateAction<string | null>>
  // Modal navigation
  pushModalState: () => void
  popModalState: () => void
}

export function useListState(options: UseListStateOptions) {
  // Keep stable refs for callbacks to avoid stale closures
  const optRef = useRef(options)
  optRef.current = options

  // --- Core list state ---
  const [items, setItems] = useState<ListItem[]>([])
  const sortItems = (arr: ListItem[]) => [...arr].sort((a, b) => a.name.localeCompare(b.name))
  const [listLoading, setListLoading] = useState(true)
  const [expandedDepts, setExpandedDepts] = useState<Set<string>>(new Set())

  // --- Selected item / edit state ---
  const [selectedItem, setSelectedItem] = useState<ListItem | null>(null)
  const [editForm, setEditForm] = useState({ name: '', brand: '', dept: '', qty: 1, size: '', notes: '' })
  const [displayBrand, setDisplayBrand] = useState('')
  const [editLoading] = useState(false)
  const [itemHistory, setItemHistory] = useState<ItemModification[]>([])

  // --- New item highlight ---
  const [newItemIds, setNewItemIds] = useState<Set<number>>(new Set())
  const newItemTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // --- Review return navigation ---
  const [returnToReview, setReturnToReview] = useState(false)

  // --- Dept scroll refs ---
  const deptRefs = useRef<Record<string, HTMLDivElement | null>>({})

  // --- Utility: patch a single item ---
  const updateItem = (id: number, updates: Partial<ListItem>) =>
    setItems(prev => prev.map(item => item.id === id ? { ...item, ...updates } : item))

  // --- Check if all items are done ---
  const checkAllDone = (next: ListItem[]) => {
    if (next.length > 0 && next.every(i => i.checked)) {
      setTimeout(() => {
        optRef.current.setShowDoneToast(true)
        optRef.current.setShowFireworks(true)
        optRef.current.setShowDoneOverlay(true)
      }, 300)
    }
  }

  // --- Reload list from server ---
  const reloadList = async () => {
    const data = await fetchAPI('/api/list')
    if (data) {
      const loaded = sortItems(Array.isArray(data) ? data as ListItem[] : [])
      setItems(loaded)
      setExpandedDepts(prev => {
        if (prev.size > 0) return prev // preserve user's expand/collapse choices
        const depts = new Set(loaded.map(i => i.department))
        return depts
      })
    }
    // Restore pending swaps from server
    try {
      const swaps = await fetchAPI('/api/list/swaps')
      if (Array.isArray(swaps) && swaps.length > 0) {
        const byItem: Record<number, SwapOffer> = {}
        for (const s of swaps) byItem[(s as SwapOffer).item_id] = s as SwapOffer
        optRef.current.setSwapsByItemId(prev => ({ ...prev, ...byItem }))
      }
    } catch { /* swaps are optional */ }
    setListLoading(false)
  }

  // --- Open item detail sheet ---
  const openDetail = (item: ListItem) => {
    optRef.current.pushModalState()
    setSelectedItem(item)
    setEditForm({ name: item.name, brand: item.brand, dept: item.department, qty: item.quantity || 1, size: item.size, notes: item.notes || '' })
    setDisplayBrand(item.brand)
    setItemHistory([])
    fetchAPI(`/api/list/${item.id}/history`)
      .then(data => setItemHistory(Array.isArray(data) ? data as ItemModification[] : []))
      .catch(e => console.warn('[load] item history failed:', e))
  }

  const openDetailFromReview = (item: ListItem) => {
    optRef.current.setShowReviewSheet(false)
    setReturnToReview(true)
    openDetail(item)
  }

  // --- Close item detail sheet ---
  const closeEditSheet = (deletedId?: number, updatedItem?: ListItem) => {
    optRef.current.popModalState()
    setSelectedItem(null)
    if (returnToReview) {
      setReturnToReview(false)
      if (updatedItem) {
        optRef.current.setLastAdded(prev => prev.map(i => i.id === updatedItem.id ? updatedItem : i))
      }
      if (deletedId) {
        optRef.current.setLastAdded(prev => {
          const remaining = prev.filter(i => i.id !== deletedId)
          if (remaining.length === 0 && optRef.current.lastSkipped.length === 0) return remaining
          return remaining
        })
        optRef.current.setLastSkipped(prev => prev.filter(s => s.existing_id !== deletedId))
      }
      // Reopen review if there are still items to review
      const hasAdded = deletedId ? optRef.current.lastAdded.some(i => i.id !== deletedId) : optRef.current.lastAdded.length > 0
      const hasSkipped = deletedId ? optRef.current.lastSkipped.some(s => s.existing_id !== deletedId) : optRef.current.lastSkipped.length > 0
      if (hasAdded || hasSkipped) {
        setTimeout(() => optRef.current.setShowReviewSheet(true), 100)
      }
    }
  }

  // --- Save edited item ---
  const handleEditSave = async () => {
    if (!selectedItem || !editForm.name.trim() || editLoading) return
    setDisplayBrand(editForm.brand)
    const optimistic: Partial<ListItem> = { name: editForm.name.trim(), brand: editForm.brand.trim(), department: editForm.dept, quantity: editForm.qty, size: editForm.size.trim(), notes: editForm.notes.trim() }
    const oldItem = { ...selectedItem }
    // Optimistic: update UI and close sheet immediately
    setItems(prev => sortItems(prev.map(item => item.id === selectedItem.id ? { ...item, ...optimistic } : item)))
    closeEditSheet(undefined, { ...selectedItem, ...optimistic })
    try {
      const data = await fetchAPI(`/api/list/${selectedItem.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(optimistic),
      })
      if (data) {
        const updated = data as ListItem
        setItems(prev => sortItems(prev.map(item => item.id === updated.id ? updated : item)))
      } else {
        // Rollback
        setItems(prev => sortItems(prev.map(item => item.id === oldItem.id ? oldItem : item)))
        optRef.current.showToast('Couldn\'t save changes — please try again', 'error')
      }
    } catch {
      setItems(prev => sortItems(prev.map(item => item.id === oldItem.id ? oldItem : item)))
      optRef.current.showToast('Couldn\'t save changes — please try again', 'error')
    }
  }

  // --- Delete item from edit sheet ---
  const handleEditDelete = async () => {
    if (!selectedItem || editLoading) return
    const deletingId = selectedItem.id
    const oldItems = items
    // Optimistic: remove from UI and close sheet immediately
    setItems(prev => prev.filter(item => item.id !== deletingId))
    closeEditSheet(deletingId)
    try {
      const result = await fetchAction(`/api/list/${deletingId}`, { method: 'DELETE' })
      if (!result.ok && result.status !== 401) {
        setItems(oldItems) // Rollback
        optRef.current.showToast('Couldn\'t delete that item — please try again', 'error')
      }
    } catch {
      setItems(oldItems) // Rollback
      optRef.current.showToast('Couldn\'t delete that item — please try again', 'error')
    }
  }

  // --- Toggle item checked ---
  const handleToggle = async (id: number) => {
    haptic.tap()
    const oldChecked = items.find(i => i.id === id)?.checked
    setItems(prev => prev.map(item => item.id === id ? { ...item, checked: !item.checked } : item))
    try {
      const data = await fetchAPI(`/api/list/${id}/toggle`, { method: 'PATCH' })
      if (data) {
        const updated = data as ListItem
        setItems(prev => { const next = prev.map(item => item.id === updated.id ? updated : item); checkAllDone(next); return next })
      } else {
        updateItem(id, { checked: oldChecked })
        optRef.current.showToast('Couldn\'t update that item — please try again', 'error')
      }
    } catch {
      updateItem(id, { checked: oldChecked })
      optRef.current.showToast("Hmm, that change didn't stick. Try again?", 'error')
    }
  }

  // --- Toggle urgent ---
  const handleUrgentToggle = async (id: number) => {
    haptic.double()
    const was = items.find(i => i.id === id)?.urgent
    updateItem(id, { urgent: !was })
    try {
      const data = await fetchAPI(`/api/list/${id}/urgent`, { method: 'PATCH' })
      if (data) {
        const updated = data as ListItem
        updateItem(updated.id, updated)
        optRef.current.showChimeIn(updated.urgent ? `Marked ${updated.name} as urgent!` : `${updated.name} back to normal`, 3000)
      } else {
        updateItem(id, { urgent: was })
        optRef.current.showToast('Couldn\'t update that item — please try again', 'error')
      }
    } catch {
      updateItem(id, { urgent: was })
      optRef.current.showToast("Hmm, that change didn't stick. Try again?", 'error')
    }
  }

  // --- Claim item ---
  const handleClaim = async (id: number) => {
    const oldItem = items.find(i => i.id === id)
    const wasClaimed = oldItem?.claimed_by_id === optRef.current.currentUserId
    updateItem(id, { claimed_by_id: wasClaimed ? null : optRef.current.currentUserId, claimed_by_name: wasClaimed ? '' : 'Me' })
    try {
      const data = await fetchAPI(`/api/list/${id}/claim`, { method: 'PATCH' })
      if (data) {
        const updated = data as ListItem
        updateItem(updated.id, updated)
        optRef.current.showChimeIn(updated.claimed_by_id ? `You claimed ${updated.name}!` : `Unclaimed ${updated.name}`, 3000)
      } else {
        if (oldItem) updateItem(id, oldItem)
        optRef.current.showToast('Couldn\'t claim that item — please try again', 'error')
      }
    } catch {
      if (oldItem) updateItem(id, oldItem)
      optRef.current.showToast("Couldn't claim that item. Try again?", 'error')
    }
  }

  // --- Swipe delete ---
  const handleSwipeDelete = useCallback(async (id: number) => {
    haptic.heavy()
    const oldItems = items
    setItems(prev => prev.filter(item => item.id !== id))
    try {
      const result = await fetchAction(`/api/list/${id}`, { method: 'DELETE' })
      if (!result.ok && result.status !== 401) {
        setItems(oldItems)
        optRef.current.showToast('Couldn\'t delete that item — please try again', 'error')
      }
    } catch {
      setItems(oldItems)
      optRef.current.showToast('Couldn\'t delete that item — please try again', 'error')
    }
  }, [items]) // eslint-disable-line react-hooks/exhaustive-deps

  // --- Clear list ---
  const handleClearList = async (all?: boolean) => {
    haptic.heavy()
    const oldItems = items
    if (all) {
      setItems([]); setExpandedDepts(new Set()); optRef.current.setShowDoneToast(false); optRef.current.setShowFireworks(false); optRef.current.setPennyChimeIn(null)
    } else {
      setItems(prev => prev.filter(i => !i.checked)); setExpandedDepts(new Set()); optRef.current.setShowDoneToast(false); optRef.current.setShowFireworks(false); optRef.current.setPennyChimeIn(null)
    }
    optRef.current.typePennyHome('List cleared — ready for a fresh start!')
    const url = all ? '/api/list/clear?all=true' : '/api/list/clear'
    const result = await fetchAction(url, { method: 'POST' })
    if (!result.ok && result.status !== 401) {
      setItems(oldItems) // Rollback
      optRef.current.showToast('Couldn\'t clear your list — please try again', 'error')
    }
  }

  // --- Toggle department expand/collapse ---
  const handleDeptClick = (dept: string) => {
    haptic.tap()
    setExpandedDepts(prev => {
      const next = new Set(prev)
      if (next.has(dept)) {
        next.delete(dept)
      } else {
        next.add(dept)
        setTimeout(() => {
          deptRefs.current[dept]?.scrollIntoView({ behavior: 'smooth', block: 'start' })
        }, 0)
      }
      return next
    })
  }

  return {
    // State
    items, setItems,
    listLoading, setListLoading,
    expandedDepts, setExpandedDepts,
    selectedItem, setSelectedItem,
    editForm, setEditForm,
    displayBrand, setDisplayBrand,
    editLoading,
    itemHistory, setItemHistory,
    newItemIds, setNewItemIds,
    newItemTimerRef,
    returnToReview, setReturnToReview,
    deptRefs,
    // Utilities
    sortItems,
    updateItem,
    checkAllDone,
    // Handlers
    reloadList,
    openDetail,
    openDetailFromReview,
    closeEditSheet,
    handleEditSave,
    handleEditDelete,
    handleToggle,
    handleUrgentToggle,
    handleClaim,
    handleSwipeDelete,
    handleClearList,
    handleDeptClick,
  }
}
