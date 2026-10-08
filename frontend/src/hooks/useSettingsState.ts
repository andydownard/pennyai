// =============================================================================
// useSettingsState — Custom hook for settings-related state, handlers, and effects
//
// Extracted from App.tsx to reduce file size. Contains:
//   - Settings form state (name, email, address, etc.)
//   - Penny configuration state (slider, autotune, store order)
//   - Household member & invite state
//   - Referral state
//   - Settings load/save handlers
//   - Invite handler (share link)
//   - Toggle handlers (autotune, store order)
//   - Effects for loading settings/referral data on screen change
// =============================================================================

import { useState, useRef, useEffect } from 'react'
import { fetchAPI } from '../api'
import { track } from '../analytics'

export interface UseSettingsStateOptions {
  screen: string
  currentUserId: number | null
  showToast: (msg: string, type?: 'success' | 'error') => void
  /** Called when loadSettings receives swap_credits_cents from the server */
  onEarningsTotalCents?: (cents: number) => void
  /** Called when loadSettings receives onboarding_flags from the server */
  onOnboardingFlags?: (flags: Record<string, string>) => void
}

export function useSettingsState(options: UseSettingsStateOptions) {
  const { screen, showToast, onEarningsTotalCents, onOnboardingFlags } = options

  // Stable refs for callbacks to avoid stale closures
  const showToastRef = useRef(showToast)
  showToastRef.current = showToast
  const onEarningsTotalCentsRef = useRef(onEarningsTotalCents)
  onEarningsTotalCentsRef.current = onEarningsTotalCents
  const onOnboardingFlagsRef = useRef(onOnboardingFlags)
  onOnboardingFlagsRef.current = onOnboardingFlags

  // --- Settings state ---
  const [settingsFirstName, setSettingsFirstName] = useState('')
  const [settingsLastName, setSettingsLastName] = useState('')
  const [settingsEmail, setSettingsEmail] = useState('')
  const [settingsAddress, setSettingsAddress] = useState('')
  const [placesReady, setPlacesReady] = useState(false)
  const [settingsMembers, setSettingsMembers] = useState<{ name: string }[]>([])
  const [listName, setListName] = useState('')
  const [editingListName, setEditingListName] = useState(false)
  const [listNameDraft, setListNameDraft] = useState('')
  const [inviteLoading, setInviteLoading] = useState(false)
  const [showInviteSheet, setShowInviteSheet] = useState(false)

  // --- Referral state ---
  const [referralCode, setReferralCode] = useState('')
  const [referralFriends, setReferralFriends] = useState<{ name: string; created_at: string }[]>([])

  // --- Penny configuration state ---
  const [pennySlider, setPennySlider] = useState(35)
  const [pennyAutotune, setPennyAutotune] = useState(true)
  const [storeOrderEnabled, setStoreOrderEnabled] = useState(true)

  // --- Auto-save refs ---
  const settingsSaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const settingsFirstNameRef = useRef(settingsFirstName)
  const settingsLastNameRef = useRef(settingsLastName)
  const settingsEmailRef = useRef(settingsEmail)
  const settingsAddressRef = useRef(settingsAddress)
  settingsFirstNameRef.current = settingsFirstName
  settingsLastNameRef.current = settingsLastName
  settingsEmailRef.current = settingsEmail
  settingsAddressRef.current = settingsAddress
  const settingsSaving = useRef(false)
  const settingsQueued = useRef(false)

  // =========================================================================
  // LOAD SETTINGS
  // =========================================================================

  const loadSettings = () => {
    fetchAPI('/api/settings')
      .then(data => {
        if (data && !Array.isArray(data)) {
          const parts = ((data.name as string) || '').split(' ')
          setSettingsFirstName(parts[0] || '')
          setSettingsLastName(parts.slice(1).join(' '))
          setSettingsEmail((data.email as string) || '')
          setSettingsAddress((data.address as string) || '')
          if (data.google_places_ready) setPlacesReady(true)
          setSettingsMembers((data.household_members as { name: string }[]) || [])
          if (data.penny_slider != null) setPennySlider(data.penny_slider as number)
          if (data.penny_autotune != null) setPennyAutotune(data.penny_autotune as boolean)
          if (data.store_order_enabled != null) setStoreOrderEnabled(data.store_order_enabled as boolean)
          if (data.swap_credits_cents != null) {
            onEarningsTotalCentsRef.current?.(data.swap_credits_cents as number)
          }
          if (data.list_name != null) setListName(data.list_name as string)
          // Hydrate onboarding flags from server (server is source of truth)
          if (data.onboarding_flags && typeof data.onboarding_flags === 'string') {
            try {
              const flags = JSON.parse(data.onboarding_flags as string) as Record<string, string>
              onOnboardingFlagsRef.current?.(flags)
            } catch { /* ignore parse errors */ }
          }
        }
      })
      .catch(e => console.warn('[load] settings failed:', e))
  }

  // =========================================================================
  // LOAD REFERRAL DATA
  // =========================================================================

  const loadReferralData = () => {
    fetchAPI('/api/referral')
      .then(data => {
        if (data && !Array.isArray(data)) {
          setReferralCode((data.code as string) || '')
          setReferralFriends((data.friends as { name: string; created_at: string }[]) || [])
        }
      })
      .catch(e => console.warn('[load] referral failed:', e))
  }

  // =========================================================================
  // SETTINGS AUTO-SAVE
  // =========================================================================

  const doSettingsSave = async () => {
    if (settingsSaving.current) { settingsQueued.current = true; return }
    settingsSaving.current = true
    try {
      await fetchAPI('/api/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: [settingsFirstNameRef.current.trim(), settingsLastNameRef.current.trim()].filter(Boolean).join(' '), address: settingsAddressRef.current.trim() }),
      })
    } catch { showToastRef.current('Something went wrong. Try again.', 'error') }
    settingsSaving.current = false
    if (settingsQueued.current) { settingsQueued.current = false; doSettingsSave() }
  }

  const handleSettingsAutoSave = () => {
    if (settingsSaveTimer.current) clearTimeout(settingsSaveTimer.current)
    settingsSaveTimer.current = setTimeout(doSettingsSave, 500)
  }

  // =========================================================================
  // INVITE HANDLERS
  // =========================================================================

  const handleOpenInviteSheet = () => {
    setShowInviteSheet(true)
  }

  const handleShareInviteLink = async () => {
    setInviteLoading(true)
    try {
      const data = await fetchAPI('/api/settings/invite', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
      })
      if (!data || Array.isArray(data) || !data.link) {
        showToastRef.current('Couldn\'t create invite link — please try again', 'error')
        return
      }
      const link = data.link as string
      const message = `Join our household shopping list on Groflo! ${link}`
      if (navigator.share) {
        track('invite_shared', { method: 'native_share' })
        try { await navigator.share({ title: 'Join my Groflo list', text: message, url: link }); setShowInviteSheet(false); return } catch { /* fall through */ }
      }
      track('invite_shared', { method: 'clipboard' })
      try {
        await navigator.clipboard.writeText(link)
        showToastRef.current('Invite link copied!')
      } catch {
        window.prompt('Copy this invite link', link)
      }
      setShowInviteSheet(false)
    } catch { showToastRef.current('Couldn\'t connect — check your connection and try again', 'error') }
    finally { setInviteLoading(false) }
  }

  // =========================================================================
  // TOGGLE HANDLERS
  // =========================================================================

  const toggleAutotune = () => {
    const next = !pennyAutotune
    setPennyAutotune(next)
    fetchAPI('/api/settings', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ penny_autotune: next }),
    }).catch(() => { setPennyAutotune(!next); showToastRef.current('Something went wrong. Try again.', 'error') })
  }

  const toggleStoreOrder = () => {
    const next = !storeOrderEnabled
    setStoreOrderEnabled(next)
    fetchAPI('/api/settings', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ store_order_enabled: next }),
    }).catch(() => { setStoreOrderEnabled(!next); showToastRef.current('Something went wrong. Try again.', 'error') })
  }

  // =========================================================================
  // EFFECTS
  // =========================================================================

  // Load settings when entering penny, profile, or share screens
  useEffect(() => {
    if (screen !== 'penny' && screen !== 'profile' && screen !== 'settings-share') return
    loadSettings()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen])

  // Load referral data when entering share screen
  useEffect(() => {
    if (screen !== 'settings-share') return
    loadReferralData()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen])

  return {
    // Settings state
    settingsFirstName, setSettingsFirstName,
    settingsLastName, setSettingsLastName,
    settingsEmail, setSettingsEmail,
    settingsAddress, setSettingsAddress,
    placesReady,
    settingsMembers,
    listName, setListName,
    editingListName, setEditingListName,
    listNameDraft, setListNameDraft,

    // Invite state
    inviteLoading,
    showInviteSheet, setShowInviteSheet,

    // Referral state
    referralCode,
    referralFriends,

    // Penny configuration state
    pennySlider, setPennySlider,
    pennyAutotune,
    storeOrderEnabled,

    // Handlers
    loadSettings,
    loadReferralData,
    doSettingsSave,
    handleSettingsAutoSave,
    handleOpenInviteSheet,
    handleShareInviteLink,
    toggleAutotune,
    toggleStoreOrder,
  }
}
