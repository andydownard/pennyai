export type ItemModification = {
  action: string
  detail: string
  source: string
  created_at: string
  user_name: string
}

export type DealType = 'sku_swap' | 'bogo' | 'trial' | 'seasonal'

export interface ReceiptSummary {
  id: number
  image_hash: string
  items_found: number
  items_matched: number
  store_name: string
  transaction_date: string
  total_cents: number
  status: string
  swaps_verified: number
  created_at: string
}

export interface ReceiptLineItem {
  id: number
  raw_text: string
  price_cents: number
  quantity: number
  matched_item_id?: number
  matched_name?: string
  confidence: 'low' | 'medium' | 'high' | 'rejected'
}
