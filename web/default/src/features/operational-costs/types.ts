export type OperationalAsset = {
  id: number
  source_type: 'cpa_account' | 'channel'
  source_key: string
  source_id: number
  display_name: string
  state: 'active' | 'archived'
  cost_minor: number
  currency: string
  cost_date: number
  cost_note: string
  archived_at: number
  archive_reason: string
  last_seen_at: number
}

export type OperationalCostEntry = {
  id: number
  title: string
  category: string
  amount_minor: number
  currency: string
  occurred_at: number
  note: string
}

export type OperationalCostSummary = {
  current_asset_cost: number
  archived_asset_cost: number
  custom_cost: number
  total_cost: number
  current_assets: number
  archived_assets: number
}

export type OperationalCostOverviewResponse = {
  success: boolean
  message?: string
  data?: {
    summary: OperationalCostSummary
    entries: OperationalCostEntry[]
    archives: OperationalAsset[]
    currency: string
  }
}

export type AssetCostPayload = {
  source_type: 'cpa_account' | 'channel'
  source_key: string
  source_id?: number
  source_ref?: string
  display_name: string
  cost_minor: number
  cost_date?: number
  note?: string
}

export type CostEntryPayload = {
  title: string
  category: string
  amount_minor: number
  occurred_at: number
  note: string
}
