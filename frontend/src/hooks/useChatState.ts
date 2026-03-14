import { useState, useRef } from 'react'
import { fetchAPI, fetchAction } from '../api'
import { track } from '../analytics'
import { trackEvent } from '../ab'
import type { ListItem } from '../components/ItemRow'
import type { SwapOffer } from '../components/SwapCard'
import type { DisambigItem, SkippedItem } from './useSwapState'
import { DEPARTMENTS } from '../styles'

// Commodity items that should skip brand disambiguation when auto-assigned
const COMMODITY_KEYWORDS = new Set([
  'milk', 'bread', 'eggs', 'butter', 'cheese', 'yogurt', 'water', 'rice',
  'flour', 'sugar', 'salt', 'pepper', 'oil', 'pasta', 'cereal', 'juice',
  'coffee', 'tea', 'chicken', 'beef', 'pork', 'fish', 'onion', 'potato',
  'tomato', 'lettuce', 'apple', 'banana', 'orange',
])

function isCommodityItem(productCategory: string): boolean {
  const lower = productCategory.toLowerCase()
  for (const kw of COMMODITY_KEYWORDS) {
    if (lower.includes(kw)) return true
  }
  return false
}

interface UseChatStateOptions {
  // List state (from useListState)
  items: ListItem[]
  setItems: React.Dispatch<React.SetStateAction<ListItem[]>>
  sortItems: (arr: ListItem[]) => ListItem[]
  setExpandedDepts: React.Dispatch<React.SetStateAction<Set<string>>>
  setNewItemIds: React.Dispatch<React.SetStateAction<Set<number>>>
  newItemTimerRef: React.MutableRefObject<ReturnType<typeof setTimeout> | null>
  deptRefs: React.MutableRefObject<Record<string, HTMLDivElement | null>>
  reloadList: () => Promise<void>
  handleClearList: (all?: boolean) => Promise<void>
  // Swap state (from useSwapState)
  setLastAdded: React.Dispatch<React.SetStateAction<ListItem[]>>
  setLastSkipped: React.Dispatch<React.SetStateAction<SkippedItem[]>>
  hasReviewedRef: React.MutableRefObject<boolean>
  pennyDisambig: DisambigItem[]
  setPennyDisambig: React.Dispatch<React.SetStateAction<DisambigItem[]>>
  pennyDisambigIndex: number
  setPennyDisambigIndex: React.Dispatch<React.SetStateAction<number>>
  setSwapsByItemId: React.Dispatch<React.SetStateAction<Record<number, SwapOffer>>>
  advanceDisambig: () => void
  handleBrandPick: (brand: string) => Promise<void>
  setShowReviewSheet: React.Dispatch<React.SetStateAction<boolean>>
  // Settings state (from useSettingsState)
  settingsFirstName: string
  setSettingsFirstName: React.Dispatch<React.SetStateAction<string>>
  settingsLastName: string
  setSettingsLastName: React.Dispatch<React.SetStateAction<string>>
  settingsEmail: string
  settingsUserPhone: string
  handleOpenInviteSheet: () => void
  // App state
  loginCount: number
  authMethods: string[]
  handleSessionExpired: () => void
  handleLoadSavedList: (id: number, mode: 'merge' | 'overwrite') => Promise<void>
  showToast: (msg: string, type?: 'success' | 'error') => void
  setScreen: (s: string) => void
  // UI state
  pendingImages: string[]
  setPendingImages: React.Dispatch<React.SetStateAction<string[]>>
  setFloatingLabels: React.Dispatch<React.SetStateAction<Array<{ text: string; key: number }>>>
  setShowDoneToast: React.Dispatch<React.SetStateAction<boolean>>
  setShowFireworks: React.Dispatch<React.SetStateAction<boolean>>
  setPendingReceiptMode: React.Dispatch<React.SetStateAction<boolean>>
  receiptInputRef: React.RefObject<HTMLInputElement | null>
  setEarningsTotalCents: React.Dispatch<React.SetStateAction<number>>
}

export function useChatState(options: UseChatStateOptions) {
  // Keep stable refs for callbacks to avoid stale closures
  const optRef = useRef(options)
  optRef.current = options

  // --- Chat input state ---
  const [chatText, setChatText] = useState('')
  const [chatLoading, setChatLoading] = useState(false)
  const chatInputRef = useRef<HTMLTextAreaElement>(null)

  // --- Penny home bar state ---
  const [homePennyText, setHomePennyText] = useState('')
  const [homePennyTyping, setHomePennyTyping] = useState(false)
  const [homePennyBtns, setHomePennyBtns] = useState<{ label: string; onClick: () => void }[]>([])
  const homePennySource = useRef('')
  const homePennyInterval = useRef<ReturnType<typeof setInterval> | null>(null)
  const idleTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const hasGreetedEmpty = useRef(false)
  const hasGreetedWithItems = useRef(false)

  // --- Penny status ---
  const [pennyStatus, setPennyStatus] = useState<string | null>(null)

  // --- Onboarding state (persisted via localStorage + server) ---
  const onboardingNameAsked = useRef(localStorage.getItem('ob_name') === '1')
  const onboardingShareMentioned = useRef(localStorage.getItem('ob_share') === '1')
  const onboardingTextMentioned = useRef(localStorage.getItem('ob_text') === '1')
  const onboardingSwapsMentioned = useRef(localStorage.getItem('ob_swaps') === '1')
  const onboardingLinkPhoneMentioned = useRef(localStorage.getItem('ob_link_phone') === '1')
  const onboardingLinkEmailMentioned = useRef(localStorage.getItem('ob_link_email') === '1')
  const nameInputMode = useRef(false)

  const syncOnboardingFlags = () => {
    const flags: Record<string, string> = {}
    if (onboardingNameAsked.current) flags.ob_name = '1'
    if (onboardingShareMentioned.current) flags.ob_share = '1'
    if (onboardingTextMentioned.current) flags.ob_text = '1'
    if (onboardingSwapsMentioned.current) flags.ob_swaps = '1'
    if (onboardingLinkPhoneMentioned.current) flags.ob_link_phone = '1'
    if (onboardingLinkEmailMentioned.current) flags.ob_link_email = '1'
    fetchAPI('/api/settings', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ onboarding_flags: JSON.stringify(flags) }),
    }).catch(() => {})
  }

  // --- Type out a message in Penny's home bar, letter by letter ---
  const typePennyHome = (msg: string, btns: { label: string; onClick: () => void }[] = []) => {
    if (homePennyInterval.current) clearInterval(homePennyInterval.current)
    if (msg === homePennySource.current && !btns.length) return // don't re-type same message
    homePennySource.current = msg
    setHomePennyText('')
    setHomePennyBtns(btns)
    setHomePennyTyping(true)
    let i = 0
    homePennyInterval.current = setInterval(() => {
      i++
      setHomePennyText(msg.slice(0, i))
      if (i >= msg.length) {
        if (homePennyInterval.current) clearInterval(homePennyInterval.current)
        homePennyInterval.current = null
        setHomePennyTyping(false)
      }
    }, 25)
  }

  // --- Main chat send handler ---
  const handleSendChat = async () => {
    const { pendingImages } = optRef.current
    if ((!chatText.trim() && pendingImages.length === 0) || chatLoading) return

    // Name input mode: user typed their name in response to "what should I call you?"
    if (nameInputMode.current && chatText.trim() && pendingImages.length === 0) {
      nameInputMode.current = false
      const input = chatText.trim()
      setChatText('')
      // Detect if user typed grocery items instead of their name
      const groceryNouns = new Set(['milk','eggs','bread','butter','cheese','chicken','beef','pork','rice','pasta',
        'flour','sugar','salt','pepper','oil','water','juice','coffee','tea','cereal','yogurt','cream',
        'banana','bananas','apple','apples','orange','oranges','tomato','tomatoes','onion','onions',
        'potato','potatoes','lettuce','spinach','broccoli','carrot','carrots','avocado','avocados'])
      // Modifiers that look like groceries only when paired with a noun
      const groceryModifiers = new Set([
        'large','small','medium','organic','frozen','fresh','whole','skim','low','fat','free','diet',
        'pack','box','bag','can','bottle','gallon','dozen','lb','oz','lbs'])
      const words = input.toLowerCase().split(/\s+/)
      // Treat as groceries if: 3+ words, starts with digit, or contains at least one grocery noun
      const hasGroceryNoun = words.some(w => groceryNouns.has(w))
      const allGroceryRelated = words.every(w => groceryNouns.has(w) || groceryModifiers.has(w))
      const looksLikeGroceries = words.length > 2 || (hasGroceryNoun && allGroceryRelated) || /^\d/.test(input)
      if (looksLikeGroceries) {
        // User typed groceries, not their name — route to normal parsing
        setChatText(input)
        // Fall through to normal send flow below (nameInputMode already false)
      } else {
        // Save name to backend
        const parts = input.split(' ')
        const firstName = parts[0] || ''
        const lastName = parts.slice(1).join(' ')
        optRef.current.setSettingsFirstName(firstName)
        optRef.current.setSettingsLastName(lastName)
        fetch('/api/settings', {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: input }),
        }).catch(() => {})
        typePennyHome(`Nice to meet you, ${firstName}! Keep adding — I'll sort everything by aisle.`)
        return
      }
    }

    // Disambiguation-aware routing: handle brand picks via chat input
    if (optRef.current.pennyDisambig.length > 0 && chatText.trim() && optRef.current.pendingImages.length === 0) {
      const text = chatText.trim().toLowerCase()
      // Simple phrases to skip/dismiss
      if (['skip', 'next'].includes(text)) {
        setChatText('')
        optRef.current.advanceDisambig()
        return
      }
      if (['skip all', 'just pick for me', 'just pick', 'you decide'].includes(text)) {
        setChatText('')
        optRef.current.setPennyDisambig([])
        optRef.current.setPennyDisambigIndex(0)
        typePennyHome("All set! I'll remember those for next time.")
        return
      }
      // Check if text matches an alternative for the current item
      const d = optRef.current.pennyDisambig[optRef.current.pennyDisambigIndex]
      if (d) {
        const matchedAlt = d.alternatives.find(a => a.toLowerCase() === text)
        if (matchedAlt !== undefined) {
          setChatText('')
          optRef.current.handleBrandPick(matchedAlt || d.current_brand)
          return
        }
        // Treat as a custom brand name for the current item
        if (text.length < 40 && !text.includes(' and ') && !text.includes(',')) {
          setChatText('')
          optRef.current.handleBrandPick(chatText.trim())
          return
        }
      }
      // Otherwise fall through to normal parsing (e.g. "also get bread")
    }

    const chatPayloadText = chatText.trim()
    const chatPayloadImages = [...optRef.current.pendingImages]
    setChatText('')
    optRef.current.setPendingImages([])
    if (chatInputRef.current) chatInputRef.current.style.height = 'auto'
    setChatLoading(true)
    optRef.current.setLastAdded([])
    optRef.current.setLastSkipped([])
    optRef.current.hasReviewedRef.current = false
    optRef.current.setPennyDisambig([])
    optRef.current.setPennyDisambigIndex(0)
    try {
      const payload = JSON.stringify({ text: chatPayloadText, images: chatPayloadImages })

      const res = await fetch('/api/list/parse', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload,
      })

      if (res.status === 401) { optRef.current.handleSessionExpired(); return }
      if (res.ok) {
        const data = await res.json()
        const action: string = data.action || 'add'
        const addedItems: ListItem[] = data.items || []
        const skippedItems = data.skipped || []
        const removedItems: ListItem[] = data.removed || []
        const updatedItems: ListItem[] = data.updated || []
        const checkedItems: ListItem[] = data.checked || []
        const pennyMsg: string = data.message || ''

        if (action === 'remove' && removedItems.length > 0) {
          const removedIds = new Set(removedItems.map(i => i.id))
          optRef.current.setItems(prev => optRef.current.sortItems(prev.filter(i => !removedIds.has(i.id))))
          const now = Date.now()
          optRef.current.setFloatingLabels(removedItems.slice(0, 5).map((item, i) => ({
            text: `−${item.name}`,
            key: now + i,
          })))
        } else if (action === 'check' || action === 'uncheck') {
          const checkedIds = new Map(checkedItems.map(i => [i.id, i.checked]))
          optRef.current.setItems(prev => optRef.current.sortItems(prev.map(i => checkedIds.has(i.id) ? { ...i, checked: checkedIds.get(i.id)! } : i)))
        } else if (action === 'update' && updatedItems.length > 0) {
          optRef.current.setItems(prev => {
            const map = new Map(prev.map(i => [i.id, i]))
            for (const item of updatedItems) map.set(item.id, item)
            return optRef.current.sortItems(Array.from(map.values()))
          })
        } else if (action === 'share') {
          if (pennyMsg) {
            typePennyHome(pennyMsg, [{
              label: 'Invite',
              onClick: () => optRef.current.handleOpenInviteSheet(),
            }])
          }
        } else if (action === 'save') {
          // Saved — just show message
        } else if (action === 'load_saved_list') {
          const savedListsData: { id: number; name: string; item_count: number }[] = data.saved_lists || []
          if (savedListsData.length === 0) {
            // No saved lists — Penny's message already says so
          } else if (data.added > 0) {
            optRef.current.reloadList()
          } else {
            const btns = savedListsData.map(sl => ({
              label: `${sl.name} (${sl.item_count})`,
              onClick: () => {
                optRef.current.handleLoadSavedList(sl.id, 'merge')
              },
            }))
            typePennyHome(pennyMsg, btns)
            return
          }
        } else if (action === 'delete_saved_list') {
          // Deleted — just show Penny's message
        } else if (action === 'clear') {
          const checkedCount = optRef.current.items.filter(i => i.checked).length
          const totalCount = optRef.current.items.length
          if (totalCount === 0) {
            typePennyHome('Your list is already empty!')
          } else if (checkedCount > 0) {
            const btns: { label: string; onClick: () => void }[] = [
              { label: `Clear ${checkedCount} checked item${checkedCount !== 1 ? 's' : ''}`, onClick: () => { optRef.current.handleClearList(false); typePennyHome(`Cleared ${checkedCount} checked item${checkedCount !== 1 ? 's' : ''}!`) } },
            ]
            if (totalCount > checkedCount) {
              btns.push({ label: `Clear entire list (${totalCount} items)`, onClick: () => { optRef.current.handleClearList(true); typePennyHome('List cleared!') } })
            }
            btns.push({ label: 'Cancel', onClick: () => typePennyHome('No problem, your list is safe!') })
            typePennyHome(pennyMsg || `You have ${checkedCount} checked and ${totalCount - checkedCount} unchecked item${totalCount - checkedCount !== 1 ? 's' : ''}. What would you like to clear?`, btns)
          } else {
            typePennyHome(pennyMsg || `Clear all ${totalCount} item${totalCount !== 1 ? 's' : ''} from your list?`, [
              { label: `Yes, clear all ${totalCount} items`, onClick: () => { optRef.current.handleClearList(true); typePennyHome('List cleared!') } },
              { label: 'Cancel', onClick: () => typePennyHome('No problem, your list is safe!') },
            ])
          }
          return
        } else if (action === 'search') {
          typePennyHome(pennyMsg || 'Here\'s what I found!')
          if (addedItems.length > 0) {
            optRef.current.reloadList()
          }
          return
        } else if (action === 'done_shopping') {
          typePennyHome(pennyMsg || 'Nice trip! Snap your receipt and I\'ll check off what you got.', [
            {
              label: 'Snap receipt',
              onClick: () => {
                optRef.current.setPendingReceiptMode(true)
                optRef.current.receiptInputRef.current?.click()
              },
            },
            { label: 'Skip', onClick: () => typePennyHome('All good! Happy shopping!') },
          ])
          return
        } else if (action === 'receipt_processed') {
          if (checkedItems.length > 0) {
            const checkedIds = new Set(checkedItems.map(i => i.id))
            optRef.current.setItems(prev => optRef.current.sortItems(prev.map(i => checkedIds.has(i.id) ? { ...i, checked: true } : i)))
          }
          optRef.current.setPendingReceiptMode(false)
        } else if (action === 'receipt_duplicate') {
          optRef.current.setPendingReceiptMode(false)
        } else if (action === 'swap_accepted') {
          track('swap_accepted', { items: updatedItems.length })
          if (updatedItems.length > 0) {
            optRef.current.setItems(prev => {
              const map = new Map(prev.map(i => [i.id, i]))
              for (const item of updatedItems) map.set(item.id, item)
              return optRef.current.sortItems(Array.from(map.values()))
            })
          }
          optRef.current.setSwapsByItemId(prev => {
            const next = { ...prev }
            for (const item of updatedItems) delete next[item.id]
            return next
          })
        } else {
          // Default "add" flow
          optRef.current.setItems(prev => {
            const map = new Map(prev.map(i => [i.id, i]))
            for (const item of addedItems) map.set(item.id, item)
            return optRef.current.sortItems(Array.from(map.values()))
          })
          optRef.current.setLastAdded(addedItems)
          optRef.current.setLastSkipped(skippedItems)
          if (addedItems.length > 0) {
            track('items_added', { count: addedItems.length })
            trackEvent('onboarding_v2', 'items_added_day1', addedItems.length)
            const now = Date.now()
            optRef.current.setFloatingLabels(addedItems.slice(0, 5).map((item, i) => ({
              text: `+${item.quantity || 1} ${item.name}`,
              key: now + i,
            })))
          }
          if (addedItems.length > 0) {
            const newDepts = new Set(addedItems.map(i => i.department))
            optRef.current.setExpandedDepts(prev => {
              const next = new Set(prev)
              newDepts.forEach(d => next.add(d))
              return next
            })
            const ids = new Set(addedItems.map(i => i.id))
            optRef.current.setNewItemIds(ids)
            if (optRef.current.newItemTimerRef.current) clearTimeout(optRef.current.newItemTimerRef.current)
            optRef.current.newItemTimerRef.current = setTimeout(() => optRef.current.setNewItemIds(new Set()), 1500)
            const firstDept = DEPARTMENTS.find(d => newDepts.has(d))
            if (firstDept) {
              setTimeout(() => {
                optRef.current.deptRefs.current[firstDept]?.scrollIntoView({ behavior: 'smooth', block: 'start' })
              }, 100)
            }
          }

          // Store any pending swap offer
          if (data.pending_swap) {
            const swap = data.pending_swap
            optRef.current.setSwapsByItemId(prev => ({ ...prev, [swap.item_id]: swap }))
            track('swap_offered', { deal_id: swap.deal_id, offer_brand: swap.offer_brand, discount_cents: swap.discount_cents })
          }

          // Show household duplicate warnings
          const dupeWarnings = (data.duplicate_warnings || []) as Array<{ item_name: string; added_by: string; hours_ago: number }>
          if (dupeWarnings.length > 0) {
            const msg = dupeWarnings.map(d =>
              `${d.added_by} already added ${d.item_name}${d.hours_ago > 0 ? ` ${d.hours_ago}h ago` : ' just now'}`
            ).join('. ')
            typePennyHome(`Heads up — ${msg}`)
            track('household_duplicate_warned', { count: dupeWarnings.length })
          }
        }
        setTimeout(() => chatInputRef.current?.focus(), 150)

        // Handle brand disambiguations — fast-track commodity items
        const disambiguations = (data.brand_disambiguations || []) as { item_id: number; product_category: string; current_brand: string; alternatives: string[]; auto_assigned?: boolean }[]
        if (disambiguations.length > 0) {
          const commodityAutoAssigned: typeof disambiguations = []
          const remaining: typeof disambiguations = []
          for (const d of disambiguations) {
            if (d.auto_assigned && d.current_brand && isCommodityItem(d.product_category)) {
              commodityAutoAssigned.push(d)
            } else {
              remaining.push(d)
            }
          }
          // Instantly confirm commodity auto-assigned items (no UI)
          for (const d of commodityAutoAssigned) {
            fetchAction('/api/list/brand-resolve', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ item_id: d.item_id, brand: d.current_brand, product_category: d.product_category }),
            }).then(result => {
              if (result.ok) {
                optRef.current.setItems(prev => prev.map(i => i.id === d.item_id ? { ...i, brand: d.current_brand } : i))
                optRef.current.setLastAdded(prev => prev.map(i => i.id === d.item_id ? { ...i, brand: d.current_brand } : i))
                trackEvent('onboarding_v2', 'brand_default_confirmed')
              }
            })
          }
          // Show a brief toast for commodity items
          if (commodityAutoAssigned.length > 0 && remaining.length === 0) {
            const names = commodityAutoAssigned.map(d => `${d.product_category} (${d.current_brand})`)
            optRef.current.showToast(`Added ${names.join(', ')}`, 'success')
          } else if (commodityAutoAssigned.length > 0) {
            const names = commodityAutoAssigned.map(d => d.product_category)
            optRef.current.showToast(`Auto-picked brands for ${names.join(', ')}`, 'success')
          }
          // Only show disambiguation UI for non-commodity items
          if (remaining.length > 0) {
            optRef.current.setPennyDisambig(remaining)
            optRef.current.setPennyDisambigIndex(0)
          }
        }

        // Show Penny's message or fallback summary
        if (action === 'share') {
          // Already handled via typePennyHome above
        } else if (pennyMsg) {
          typePennyHome(pennyMsg)
        } else {
          const parts: string[] = []
          if (data.added > 0) parts.push(`Added ${data.added} item${data.added === 1 ? '' : 's'}`)
          if (skippedItems.length > 0) parts.push(`${skippedItems.length} already on your list`)
          if (removedItems.length > 0) parts.push(`Removed ${removedItems.length} item${removedItems.length === 1 ? '' : 's'}`)
          if (updatedItems.length > 0) parts.push(`Updated ${updatedItems.length} item${updatedItems.length === 1 ? '' : 's'}`)
          if (checkedItems.length > 0) parts.push(`${action === 'check' ? 'Checked' : 'Unchecked'} ${checkedItems.length} item${checkedItems.length === 1 ? '' : 's'}`)
          if (parts.length > 0) typePennyHome(parts.join(' · '))
          else if (action === 'add' && addedItems.length === 0 && skippedItems.length === 0) typePennyHome("I couldn't find any items in that — try rewording it?")
        }

        // --- Onboarding nudges (spaced, non-intrusive) ---
        if (action === 'add' && addedItems.length > 0) {
          const { loginCount, settingsFirstName, settingsEmail, settingsUserPhone, authMethods, items } = optRef.current
          // Session 1: After first add, casually ask for name (once per session)
          if (loginCount <= 1 && !settingsFirstName && !onboardingNameAsked.current) {
            onboardingNameAsked.current = true
            localStorage.setItem('ob_name', '1')
            syncOnboardingFlags()
            setTimeout(() => {
              typePennyHome("By the way, what should I call you?", [
                { label: 'Skip', onClick: () => { nameInputMode.current = false; typePennyHome("No worries! Just keep adding items.") } },
              ])
              nameInputMode.current = true
            }, 2500)
          }
          // After 3rd item on list: mention sharing
          else if (!onboardingShareMentioned.current && items.length + addedItems.length >= 3) {
            onboardingShareMentioned.current = true
            localStorage.setItem('ob_share', '1')
            syncOnboardingFlags()
            setTimeout(() => {
              typePennyHome("Shopping for the family? You can invite them to share this list.", [
                { label: 'Invite someone', onClick: () => optRef.current.handleOpenInviteSheet() },
                { label: 'Just me', onClick: () => typePennyHome("Got it — solo mission!") },
              ])
            }, 3000)
          }
          // Session 3: Mention texting Penny
          else if (loginCount === 3 && !onboardingTextMentioned.current) {
            onboardingTextMentioned.current = true
            localStorage.setItem('ob_text', '1')
            syncOnboardingFlags()
            setTimeout(() => {
              typePennyHome("Quick tip — you can text me your list anytime. Just save my number!", [
                { label: 'Save contact', onClick: () => { window.open('/api/penny.vcf', '_blank') } },
                { label: 'Maybe later', onClick: () => typePennyHome("I'll be here!") },
              ])
            }, 3000)
          }
          // After 5+ items on list and haven't mentioned swaps yet
          else if (!onboardingSwapsMentioned.current && items.length + addedItems.length >= 5) {
            onboardingSwapsMentioned.current = true
            localStorage.setItem('ob_swaps', '1')
            syncOnboardingFlags()
            setTimeout(() => {
              typePennyHome("By the way — I keep an eye out for deals on your items. If I spot a swap that saves you money, I'll let you know!")
            }, 3500)
          }
          // Session 2+: Prompt to add phone if signed up via Google/email
          else if (loginCount >= 2 && !onboardingLinkPhoneMentioned.current && !settingsUserPhone && settingsEmail && (authMethods.includes('google') || authMethods.includes('email'))) {
            onboardingLinkPhoneMentioned.current = true
            localStorage.setItem('ob_link_phone', '1')
            syncOnboardingFlags()
            setTimeout(() => {
              typePennyHome("Want to text me your grocery list on the go? Add your phone number and I can help over SMS.", [
                { label: 'Add phone', onClick: () => optRef.current.setScreen('profile') },
                { label: 'Not now', onClick: () => typePennyHome("No problem — I'm always here in the app!") },
              ])
            }, 3500)
          }
          // Session 2+: Prompt to add email if signed up via phone only
          else if (loginCount >= 2 && !onboardingLinkEmailMentioned.current && settingsUserPhone && !settingsEmail && authMethods.includes('phone')) {
            onboardingLinkEmailMentioned.current = true
            localStorage.setItem('ob_link_email', '1')
            syncOnboardingFlags()
            setTimeout(() => {
              typePennyHome("Add your email so you can log in from any device and I can send you receipts.", [
                { label: 'Add email', onClick: () => optRef.current.setScreen('profile') },
                { label: 'Maybe later', onClick: () => typePennyHome("Just let me know when you're ready!") },
              ])
            }, 3500)
          }
        }
      } else if (res.status === 429) {
        optRef.current.showToast('Hold on — give it a minute and try again', 'error')
      } else {
        const errData = await res.json().catch(() => null)
        const errMsg = errData?.error || `Error ${res.status}`
        console.error('[penny] parse failed:', res.status, errMsg)
        optRef.current.showToast(errMsg, 'error')
      }
    } catch (err) {
      console.error('[penny] parse error:', err)
      optRef.current.showToast('Couldn\'t connect — check your connection and try again', 'error')
    }
    finally { setChatLoading(false) }
  }

  return {
    // Chat state
    chatText, setChatText,
    chatLoading, setChatLoading,
    chatInputRef,
    // Penny home bar
    homePennyText, setHomePennyText,
    homePennyTyping, setHomePennyTyping,
    homePennyBtns, setHomePennyBtns,
    homePennySource, homePennyInterval,
    idleTimer,
    hasGreetedEmpty, hasGreetedWithItems,
    // Penny status
    pennyStatus, setPennyStatus,
    // Onboarding
    onboardingNameAsked, onboardingShareMentioned,
    onboardingTextMentioned, onboardingSwapsMentioned,
    onboardingLinkPhoneMentioned, onboardingLinkEmailMentioned,
    nameInputMode, syncOnboardingFlags,
    // Functions
    typePennyHome,
    handleSendChat,
  }
}
