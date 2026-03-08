export type ItemModification = {
  action: string
  detail: string
  source: string
  created_at: string
  user_name: string
}

export type DealType = 'sku_swap' | 'bogo' | 'trial' | 'seasonal'
