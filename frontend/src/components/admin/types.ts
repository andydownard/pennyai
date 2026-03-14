import { FONT } from '../../styles'

// --- Data interfaces ---

export interface AdminData {
  active_users: number; total_users: number; new_users: number;
  k_factor: number; leaderboard: { phone: string; name: string; points: number; referrals: number }[];
  nps_responses: { score: number; comment: string; user_name: string; user_phone: string; created_at: string }[];
  nps_score: number; nps_total: number; nps_promoters: number; nps_detractors: number;
  swap_offered?: number; swap_accepted?: number; swap_declined?: number; swap_accept_rate?: number;
}

export interface Deal {
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

export interface Payout {
  user_id: number; name: string; email: string; phone: string;
  total_cents: number; swap_count: number; oldest_swap: string;
}

export interface AnalyticsData {
  items_created: number;
  items_by_source: Record<string, number>;
  items_by_department: Record<string, number>;
  events_by_action: Record<string, number>;
  events_by_day: { date: string; count: number }[];
}

export interface BusinessMetric {
  date: string;
  dau: number;
  wau: number;
  mau: number;
  acceptance_rate?: number;
  gross_revenue_cents?: number;
  active_deals?: number;
  active_brands?: number;
}

export interface BusinessMetricsData {
  days: number;
  metrics: BusinessMetric[];
}

export interface ChurnRiskUser {
  user_id: number;
  name: string;
  composite_risk: number;
  signals: { type: string; value: number; days_since_active?: number }[];
}

export interface ChurnRiskData {
  at_risk_users: ChurnRiskUser[];
  count: number;
}

export interface CohortRow {
  cohort_week: string;
  user_count: number;
  retained_count: number;
  avg_swaps: number;
}

export interface CohortData {
  cohorts: CohortRow[];
}

export interface MarketplaceHealthEntry {
  date: string;
  supply_score: number;
  demand_score: number;
  balance_score: number;
  health_index: number;
}

export interface MarketplaceHealthData {
  health: MarketplaceHealthEntry[];
}

export interface FeedbackItem {
  id: number; user_id: number; body: string; type: string; urgency: string;
  status: string; admin_notes: string; github_issue_url: string;
  created_at: string; updated_at: string; user_name: string; user_phone: string;
}

export type AdminTab = 'overview' | 'business' | 'churn' | 'cohorts' | 'marketplace' | 'feedback'

export const EMPTY_FORM = {
  competitor_product: '', offer_brand: '', offer_product: '',
  discount_cents: 0, daily_budget_cents: 0, status: 'active', ends_at: '',
  offer_price_cents: 0, competitor_price_cents: 0,
  offer_size: '', competitor_size: '',
  offer_unit_price: '', competitor_unit_price: '',
  why_bullets: '[]', allergen_notes: '',
  deal_type: 'sku_swap', quantity_offered: 1,
  trial_product_name: '', trial_size: '', campaign_tag: '',
}

// --- Shared style constants ---

export const inputStyle = { width: '100%', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', fontFamily: FONT, fontSize: '14px' }
export const labelStyle = { fontFamily: FONT, fontSize: '12px', color: 'var(--text-secondary)', display: 'block' as const, marginBottom: '2px' }
export const metricCard: React.CSSProperties = { backgroundColor: 'var(--surface)', borderRadius: '12px', padding: '16px', border: '1px solid var(--border)' }

// --- Color helpers ---

export const riskColor = (score: number) => score > 0.7 ? 'var(--error)' : score > 0.5 ? 'var(--warning)' : 'var(--brand)'
export const retentionColor = (pct: number) => pct > 50 ? 'var(--brand)' : pct > 25 ? 'var(--warning)' : 'var(--error)'
export const healthColor = (idx: number) => idx > 70 ? 'var(--brand)' : idx > 40 ? 'var(--warning)' : 'var(--error)'
