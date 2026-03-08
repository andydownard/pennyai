import { useState, useEffect } from 'react'
import { ScreenTransition } from './ScreenTransition'
import { BrandDealDeepDive } from './BrandDealDeepDive'
import {
  BRAND, TEXT, TEXT_SECONDARY, BORDER, ERROR_RED, ERROR_RED_BG, WHITE, FONT,
  pageStyles,
} from '../styles'

interface AdminScreenProps {
  fetchAPI: (url: string, options?: RequestInit) => Promise<Record<string, unknown> | unknown[] | null>
  showToast: (msg: string, type?: 'success' | 'error') => void
  renderHeader: () => React.ReactNode
  renderToast: () => React.ReactNode
  renderBottomTabs: () => React.ReactNode
  transitionStyle: React.CSSProperties
}

interface AdminData {
  active_users: number; total_users: number; new_users: number;
  k_factor: number; leaderboard: { phone: string; name: string; points: number; referrals: number }[];
  nps_responses: { score: number; comment: string; user_name: string; user_phone: string; created_at: string }[];
  nps_score: number; nps_total: number; nps_promoters: number; nps_detractors: number;
  swap_offered?: number; swap_accepted?: number; swap_declined?: number; swap_accept_rate?: number;
}

interface Deal {
  id: number; competitor_product: string; offer_brand: string; offer_product: string;
  discount_cents: number; daily_budget_cents: number; budget_spent_cents: number;
  status: string; starts_at: string; ends_at: string; created_at: string;
  offer_price_cents: number; competitor_price_cents: number;
  offer_size: string; competitor_size: string;
  offer_unit_price: string; competitor_unit_price: string;
  why_bullets: string; allergen_notes: string;
  deal_type: string; quantity_offered: number;
  trial_product_name: string; trial_size: string; campaign_tag: string;
}

interface Payout {
  user_id: number; name: string; email: string; phone: string;
  total_cents: number; swap_count: number; oldest_swap: string;
}

interface AnalyticsData {
  items_created: number;
  items_by_source: Record<string, number>;
  items_by_department: Record<string, number>;
  events_by_action: Record<string, number>;
  events_by_day: { date: string; count: number }[];
}

const EMPTY_FORM = {
  competitor_product: '', offer_brand: '', offer_product: '',
  discount_cents: 0, daily_budget_cents: 0, status: 'active', ends_at: '',
  offer_price_cents: 0, competitor_price_cents: 0,
  offer_size: '', competitor_size: '',
  offer_unit_price: '', competitor_unit_price: '',
  why_bullets: '[]', allergen_notes: '',
  deal_type: 'sku_swap', quantity_offered: 1,
  trial_product_name: '', trial_size: '', campaign_tag: '',
}

export function AdminScreen({
  fetchAPI, showToast, renderHeader, renderToast, renderBottomTabs, transitionStyle,
}: AdminScreenProps) {
  const [adminDays, setAdminDays] = useState(30)
  const [adminData, setAdminData] = useState<AdminData | null>(null)
  const [analyticsData, setAnalyticsData] = useState<AnalyticsData | null>(null)
  const [adminDeals, setAdminDeals] = useState<Deal[]>([])
  const [adminPayouts, setAdminPayouts] = useState<Payout[]>([])
  const [showDealForm, setShowDealForm] = useState(false)
  const [dealForm, setDealForm] = useState(EMPTY_FORM)
  const [editingDealId, setEditingDealId] = useState<number | null>(null)
  const [deepDiveDealId, setDeepDiveDealId] = useState<number | null>(null)

  useEffect(() => {
    fetchAPI(`/api/admin/dashboard?days=${adminDays}`)
      .then(data => { if (data) setAdminData(data as unknown as AdminData) })
      .catch(e => console.warn('[load] admin dashboard failed:', e))
    fetchAPI(`/api/admin/analytics?days=${adminDays}`)
      .then(data => { if (data) setAnalyticsData(data as unknown as AnalyticsData) })
      .catch(e => console.warn('[load] admin analytics failed:', e))
    fetch('/api/admin/deals').then(r => r.ok ? r.json() : []).then(setAdminDeals).catch(() => {})
    fetch('/api/admin/payouts').then(r => r.ok ? r.json() : []).then(setAdminPayouts).catch(() => {})
  }, [adminDays])

  const dayOptions = [7, 30, 90]

  const inputStyle = { width: '100%', padding: '8px', borderRadius: '6px', border: `1px solid ${BORDER}`, fontFamily: FONT, fontSize: '14px' }
  const labelStyle = { fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, display: 'block' as const, marginBottom: '2px' }
  const metricCard = { backgroundColor: WHITE, borderRadius: '12px', padding: '16px', border: `1px solid ${BORDER}` }

  return (
    <ScreenTransition style={transitionStyle}>
    <div style={pageStyles.page}>
      {renderHeader()}
      <div style={{ ...pageStyles.listArea, padding: '20px' }}>
        {/* Time period selector */}
        <div style={{ display: 'flex', gap: '8px', marginBottom: '20px' }}>
          {dayOptions.map(d => (
            <button key={d} onClick={() => setAdminDays(d)} style={{
              flex: 1, padding: '8px', borderRadius: '8px', border: `1px solid ${BORDER}`,
              backgroundColor: adminDays === d ? BRAND : WHITE, color: adminDays === d ? WHITE : TEXT,
              fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer',
            }}>{d}d</button>
          ))}
        </div>

        {adminData ? (
          <>
            {/* Metric cards */}
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '32px' }}>
              {[
                { label: 'Active Users', value: adminData.active_users },
                { label: 'Total Users', value: adminData.total_users },
                { label: 'New Users', value: adminData.new_users },
                { label: 'K-Factor', value: adminData.k_factor.toFixed(2) },
              ].map(m => (
                <div key={m.label} style={metricCard}>
                  <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginBottom: '4px' }}>{m.label}</div>
                  <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: TEXT }}>{m.value}</div>
                </div>
              ))}
            </div>

            {/* Leaderboard */}
            <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginBottom: '12px' }}>Referral Leaderboard</div>
            {adminData.leaderboard.length === 0 ? (
              <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY }}>No referrals yet.</div>
            ) : (
              adminData.leaderboard.map((entry, i) => (
                <div key={i} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 0', borderBottom: `1px solid ${BORDER}` }}>
                  <div>
                    <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT }}>{entry.name || entry.phone}</div>
                    {entry.name && <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>{entry.phone}</div>}
                  </div>
                  <div style={{ textAlign: 'right' }}>
                    <div style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: BRAND }}>{entry.points} pts</div>
                    <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>{entry.referrals} referrals</div>
                  </div>
                </div>
              ))
            )}

            {/* NPS Responses */}
            <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginTop: '28px', marginBottom: '12px' }}>
              NPS: {adminData.nps_score.toFixed(0)} ({adminData.nps_total} responses, {adminData.nps_promoters} promoters, {adminData.nps_detractors} detractors)
            </div>
            {adminData.nps_responses.length === 0 ? (
              <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY }}>No NPS responses yet.</div>
            ) : (
              adminData.nps_responses.map((resp, i) => (
                <div key={i} style={{ padding: '12px 0', borderBottom: `1px solid ${BORDER}` }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div style={{ fontFamily: FONT, fontSize: '15px', fontWeight: 600, color: TEXT }}>Score: {resp.score}</div>
                    <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>{new Date(resp.created_at).toLocaleDateString()}</div>
                  </div>
                  {resp.comment && <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT, marginTop: '4px' }}>{resp.comment}</div>}
                  <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginTop: '2px' }}>{resp.user_name || resp.user_phone}</div>
                </div>
              ))
            )}

            {/* Swap Stats */}
            {(adminData.swap_offered != null && adminData.swap_offered > 0) && (
              <>
                <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginTop: '28px', marginBottom: '12px' }}>Swap Deals</div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '20px' }}>
                  {[
                    { label: 'Offered', value: adminData.swap_offered },
                    { label: 'Accepted', value: adminData.swap_accepted },
                    { label: 'Declined', value: adminData.swap_declined },
                    { label: 'Accept Rate', value: `${(adminData.swap_accept_rate || 0).toFixed(1)}%` },
                  ].map(m => (
                    <div key={m.label} style={metricCard}>
                      <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginBottom: '4px' }}>{m.label}</div>
                      <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: TEXT }}>{m.value}</div>
                    </div>
                  ))}
                </div>
              </>
            )}

            {/* Payout Checklist */}
            {adminPayouts.length > 0 && (
              <>
                <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginTop: '28px', marginBottom: '12px' }}>Pending Payouts</div>
                {adminPayouts.map(p => (
                  <div key={p.user_id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 0', borderBottom: `1px solid ${BORDER}` }}>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT, fontWeight: 500 }}>{p.name || 'No name'} — ${(p.total_cents / 100).toFixed(2)}</div>
                      <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>{p.phone || p.email || 'No contact'} · {p.swap_count} swap{p.swap_count !== 1 ? 's' : ''} · since {new Date(p.oldest_swap).toLocaleDateString()}</div>
                    </div>
                    <button
                      onClick={async () => {
                        const res = await fetch('/api/admin/payouts', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ user_id: p.user_id }) })
                        if (res.ok) {
                          setAdminPayouts(prev => prev.filter(x => x.user_id !== p.user_id))
                          showToast(`Marked $${(p.total_cents / 100).toFixed(2)} as paid for ${p.name || 'user'}`)
                        } else { showToast('Failed to mark as paid', 'error') }
                      }}
                      style={{ fontSize: '12px', padding: '6px 12px', borderRadius: '6px', border: `1px solid ${BRAND}`, backgroundColor: WHITE, cursor: 'pointer', fontFamily: FONT, color: BRAND, fontWeight: 600, flexShrink: 0 }}
                    >Mark Paid</button>
                  </div>
                ))}
              </>
            )}

            {/* Deal Management */}
            <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginTop: '28px', marginBottom: '12px' }}>
              Manage Deals
              <button
                onClick={() => { setShowDealForm(true); setEditingDealId(null); setDealForm(EMPTY_FORM) }}
                style={{ marginLeft: '12px', backgroundColor: BRAND, color: WHITE, border: 'none', borderRadius: '6px', padding: '4px 12px', fontFamily: FONT, fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}
              >+ Add</button>
            </div>

            {showDealForm && (
              <div style={{ backgroundColor: WHITE, borderRadius: '12px', padding: '16px', border: `1px solid ${BORDER}`, marginBottom: '16px' }}>
                <div style={{ marginBottom: '8px' }}>
                  <label style={labelStyle}>Deal Type</label>
                  <select
                    value={dealForm.deal_type}
                    onChange={e => setDealForm(prev => ({ ...prev, deal_type: e.target.value, quantity_offered: e.target.value === 'bogo' ? 2 : 1 }))}
                    style={inputStyle}
                  >
                    <option value="sku_swap">SKU Swap</option>
                    <option value="bogo">BOGO (Buy One Get One)</option>
                    <option value="trial">Trial (Free Sample)</option>
                    <option value="seasonal">Seasonal / Limited-Time</option>
                  </select>
                </div>
                {([
                  { label: 'Competitor Product', key: 'competitor_product' as const, type: 'text' },
                  { label: 'Offer Brand', key: 'offer_brand' as const, type: 'text' },
                  { label: 'Offer Product', key: 'offer_product' as const, type: 'text' },
                  { label: 'Discount (cents)', key: 'discount_cents' as const, type: 'number' },
                  { label: 'Total Budget (cents)', key: 'daily_budget_cents' as const, type: 'number' },
                  { label: 'Offer Price (cents)', key: 'offer_price_cents' as const, type: 'number' },
                  { label: 'Competitor Price (cents)', key: 'competitor_price_cents' as const, type: 'number' },
                  { label: 'Offer Size', key: 'offer_size' as const, type: 'text' },
                  { label: 'Competitor Size', key: 'competitor_size' as const, type: 'text' },
                  { label: 'Offer Unit Price', key: 'offer_unit_price' as const, type: 'text' },
                  { label: 'Competitor Unit Price', key: 'competitor_unit_price' as const, type: 'text' },
                  { label: 'Why Bullets (JSON array)', key: 'why_bullets' as const, type: 'text' },
                  { label: 'Allergen Notes', key: 'allergen_notes' as const, type: 'text' },
                ] as const).map(f => (
                  <div key={f.key} style={{ marginBottom: '8px' }}>
                    <label style={labelStyle}>{f.label}</label>
                    <input
                      type={f.type}
                      value={dealForm[f.key]}
                      onChange={e => setDealForm(prev => ({ ...prev, [f.key]: f.type === 'number' ? parseInt(e.target.value) || 0 : e.target.value }))}
                      style={inputStyle}
                    />
                  </div>
                ))}
                {dealForm.deal_type === 'bogo' && (
                  <div style={{ marginBottom: '8px' }}>
                    <label style={labelStyle}>Quantity Offered</label>
                    <input type="number" value={dealForm.quantity_offered} onChange={e => setDealForm(prev => ({ ...prev, quantity_offered: parseInt(e.target.value) || 2 }))} style={inputStyle} />
                  </div>
                )}
                {dealForm.deal_type === 'trial' && (
                  <>
                    <div style={{ marginBottom: '8px' }}>
                      <label style={labelStyle}>Trial Product Name</label>
                      <input type="text" value={dealForm.trial_product_name} onChange={e => setDealForm(prev => ({ ...prev, trial_product_name: e.target.value }))} style={inputStyle} />
                    </div>
                    <div style={{ marginBottom: '8px' }}>
                      <label style={labelStyle}>Trial Size</label>
                      <input type="text" value={dealForm.trial_size} onChange={e => setDealForm(prev => ({ ...prev, trial_size: e.target.value }))} style={inputStyle} />
                    </div>
                  </>
                )}
                {dealForm.deal_type === 'seasonal' && (
                  <div style={{ marginBottom: '8px' }}>
                    <label style={labelStyle}>Campaign Tag</label>
                    <input type="text" value={dealForm.campaign_tag} onChange={e => setDealForm(prev => ({ ...prev, campaign_tag: e.target.value }))} placeholder="e.g. holiday_2026" style={inputStyle} />
                  </div>
                )}
                <div style={{ display: 'flex', gap: '8px', marginTop: '12px' }}>
                  <button
                    onClick={async () => {
                      const method = editingDealId ? 'PUT' : 'POST'
                      const body = editingDealId ? { ...dealForm, id: editingDealId } : dealForm
                      const res = await fetch('/api/admin/deals', { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
                      if (res.ok) {
                        const dealsRes = await fetch('/api/admin/deals')
                        if (dealsRes.ok) setAdminDeals(await dealsRes.json())
                        setShowDealForm(false)
                        showToast(editingDealId ? 'Deal updated' : 'Deal created')
                      } else { showToast('Failed to save deal', 'error') }
                    }}
                    style={{ flex: 1, backgroundColor: BRAND, color: WHITE, border: 'none', borderRadius: '8px', padding: '10px', fontFamily: FONT, fontSize: '15px', fontWeight: 600, cursor: 'pointer' }}
                  >{editingDealId ? 'Update' : 'Create'}</button>
                  <button onClick={() => setShowDealForm(false)} style={{ padding: '10px 16px', borderRadius: '8px', border: `1px solid ${BORDER}`, backgroundColor: WHITE, fontFamily: FONT, fontSize: '15px', cursor: 'pointer', color: TEXT }}>Cancel</button>
                </div>
              </div>
            )}

            {adminDeals.map(deal => (
              <div key={deal.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 0', borderBottom: `1px solid ${BORDER}` }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontFamily: FONT, fontSize: '14px', color: TEXT, fontWeight: 500, display: 'flex', alignItems: 'center', gap: '6px' }}>
                    {deal.competitor_product} → {deal.offer_brand} {deal.offer_product}
                    {deal.deal_type && deal.deal_type !== 'sku_swap' && (
                      <span style={{ fontSize: '10px', fontWeight: 700, color: WHITE, backgroundColor: deal.deal_type === 'bogo' ? '#E67E22' : deal.deal_type === 'trial' ? '#27AE60' : '#E74C3C', padding: '1px 5px', borderRadius: '3px', textTransform: 'uppercase' as const }}>{deal.deal_type}</span>
                    )}
                  </div>
                  <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY }}>
                    ${(deal.discount_cents / 100).toFixed(2)} off · Budget: ${(deal.daily_budget_cents / 100).toFixed(2)} · Spent: ${(deal.budget_spent_cents / 100).toFixed(2)} · {deal.status}
                  </div>
                </div>
                <div style={{ display: 'flex', gap: '6px', flexShrink: 0 }}>
                  <button
                    onClick={() => setDeepDiveDealId(deal.id)}
                    style={{ fontSize: '12px', padding: '4px 8px', borderRadius: '6px', border: `1px solid ${BRAND}`, backgroundColor: WHITE, cursor: 'pointer', fontFamily: FONT, color: BRAND, fontWeight: 600 }}
                  >Deep Dive</button>
                  <button
                    onClick={async () => {
                      const newStatus = deal.status === 'active' ? 'paused' : 'active'
                      await fetch('/api/admin/deals', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...deal, status: newStatus }) })
                      const res = await fetch('/api/admin/deals')
                      if (res.ok) setAdminDeals(await res.json())
                    }}
                    style={{ fontSize: '12px', padding: '4px 8px', borderRadius: '6px', border: `1px solid ${BORDER}`, backgroundColor: WHITE, cursor: 'pointer', fontFamily: FONT, color: TEXT }}
                  >{deal.status === 'active' ? 'Pause' : 'Resume'}</button>
                  <button
                    onClick={() => {
                      setEditingDealId(deal.id)
                      setDealForm({
                        competitor_product: deal.competitor_product, offer_brand: deal.offer_brand, offer_product: deal.offer_product,
                        discount_cents: deal.discount_cents, daily_budget_cents: deal.daily_budget_cents, status: deal.status, ends_at: deal.ends_at,
                        offer_price_cents: deal.offer_price_cents, competitor_price_cents: deal.competitor_price_cents,
                        offer_size: deal.offer_size, competitor_size: deal.competitor_size,
                        offer_unit_price: deal.offer_unit_price, competitor_unit_price: deal.competitor_unit_price,
                        why_bullets: deal.why_bullets, allergen_notes: deal.allergen_notes,
                        deal_type: deal.deal_type || 'sku_swap', quantity_offered: deal.quantity_offered || 1,
                        trial_product_name: deal.trial_product_name || '', trial_size: deal.trial_size || '', campaign_tag: deal.campaign_tag || '',
                      })
                      setShowDealForm(true)
                    }}
                    style={{ fontSize: '12px', padding: '4px 8px', borderRadius: '6px', border: `1px solid ${BORDER}`, backgroundColor: WHITE, cursor: 'pointer', fontFamily: FONT, color: TEXT }}
                  >Edit</button>
                  <button
                    onClick={async () => {
                      await fetch('/api/admin/deals', { method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id: deal.id }) })
                      const res = await fetch('/api/admin/deals')
                      if (res.ok) setAdminDeals(await res.json())
                    }}
                    style={{ fontSize: '12px', padding: '4px 8px', borderRadius: '6px', border: `1px solid ${ERROR_RED}40`, backgroundColor: ERROR_RED_BG, cursor: 'pointer', fontFamily: FONT, color: ERROR_RED }}
                  >Del</button>
                </div>
              </div>
            ))}

            {/* Item Analytics */}
            {analyticsData && (
              <>
                <div style={{ fontFamily: FONT, fontSize: '16px', fontWeight: 600, color: TEXT, marginTop: '28px', marginBottom: '12px' }}>Item Analytics</div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '20px' }}>
                  <div style={metricCard}>
                    <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginBottom: '4px' }}>Items Created</div>
                    <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: TEXT }}>{analyticsData.items_created}</div>
                  </div>
                  <div style={metricCard}>
                    <div style={{ fontFamily: FONT, fontSize: '12px', color: TEXT_SECONDARY, marginBottom: '4px' }}>Total Events</div>
                    <div style={{ fontFamily: FONT, fontSize: '24px', fontWeight: 700, color: TEXT }}>{Object.values(analyticsData.events_by_action).reduce((a, b) => a + b, 0)}</div>
                  </div>
                </div>

                {Object.keys(analyticsData.items_by_source).length > 0 && (
                  <>
                    <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT, marginBottom: '8px' }}>By Source</div>
                    {Object.entries(analyticsData.items_by_source).sort((a, b) => b[1] - a[1]).map(([src, cnt]) => (
                      <div key={src} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: `1px solid ${BORDER}` }}>
                        <span style={{ fontFamily: FONT, fontSize: '14px', color: TEXT }}>{src}</span>
                        <span style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT }}>{cnt}</span>
                      </div>
                    ))}
                  </>
                )}

                {Object.keys(analyticsData.items_by_department).length > 0 && (
                  <>
                    <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT, marginTop: '16px', marginBottom: '8px' }}>Top Departments</div>
                    {Object.entries(analyticsData.items_by_department).sort((a, b) => b[1] - a[1]).slice(0, 10).map(([dept, cnt]) => (
                      <div key={dept} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: `1px solid ${BORDER}` }}>
                        <span style={{ fontFamily: FONT, fontSize: '14px', color: TEXT }}>{dept}</span>
                        <span style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT }}>{cnt}</span>
                      </div>
                    ))}
                  </>
                )}

                {Object.keys(analyticsData.events_by_action).length > 0 && (
                  <>
                    <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT, marginTop: '16px', marginBottom: '8px' }}>Events by Action</div>
                    {Object.entries(analyticsData.events_by_action).sort((a, b) => b[1] - a[1]).map(([action, cnt]) => (
                      <div key={action} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: `1px solid ${BORDER}` }}>
                        <span style={{ fontFamily: FONT, fontSize: '14px', color: TEXT }}>{action}</span>
                        <span style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT }}>{cnt}</span>
                      </div>
                    ))}
                  </>
                )}

                {analyticsData.events_by_day.length > 0 && (
                  <>
                    <div style={{ fontFamily: FONT, fontSize: '14px', fontWeight: 600, color: TEXT, marginTop: '16px', marginBottom: '8px' }}>Daily Activity</div>
                    {analyticsData.events_by_day.slice(-14).map(d => (
                      <div key={d.date} style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0' }}>
                        <span style={{ fontFamily: FONT, fontSize: '13px', color: TEXT_SECONDARY }}>{d.date}</span>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <div style={{ width: `${Math.min(120, Math.max(4, d.count * 3))}px`, height: '8px', backgroundColor: BRAND, borderRadius: '4px' }} />
                          <span style={{ fontFamily: FONT, fontSize: '13px', fontWeight: 600, color: TEXT, minWidth: '24px', textAlign: 'right' }}>{d.count}</span>
                        </div>
                      </div>
                    ))}
                  </>
                )}
              </>
            )}
          </>
        ) : (
          <div style={{ fontFamily: FONT, fontSize: '15px', color: TEXT_SECONDARY, textAlign: 'center', padding: '40px 0' }}>Loading...</div>
        )}
      </div>
      {renderToast()}
      {renderBottomTabs()}
      {deepDiveDealId !== null && (
        <BrandDealDeepDive dealId={deepDiveDealId} onClose={() => setDeepDiveDealId(null)} />
      )}
    </div>
    </ScreenTransition>
  )
}
