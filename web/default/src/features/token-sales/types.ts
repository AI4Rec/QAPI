/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export type TokenSaleStatus = 'active' | 'closed'

export type TokenSale = {
  id: number
  token_id: number
  name: string
  key: string
  buyer: string
  order_no: string
  amount_minor: number
  currency: string
  granted_quota: number
  used_quota: number
  remaining_quota: number
  expired_time: number
  accessed_time: number
  token_status: number
  sale_status: TokenSaleStatus
  max_concurrency: number
  rpm_rate_limit: number
  model_limits_enabled: boolean
  model_limits: string
  allow_ips: string
  group: string
  cross_group_retry: boolean
  note: string
  delivered_at: number
  created_at: number
  today_request_count: number
  today_quota: number
  seven_day_request_count: number
  seven_day_quota: number
  last_used_at: number
}

export type TokenSaleSummary = {
  total_keys: number
  active_keys: number
  revenue_minor: number
  granted_quota: number
  used_quota: number
  remaining_quota: number
}

export type TokenSaleModelUsage = {
  model_name: string
  request_count: number
  quota: number
  prompt_tokens: number
  completion_tokens: number
  model_ratio: number
  group_ratio: number
  billing_mode: string
  matched_tier: string
}

export type TokenSaleBasePayload = {
  name: string
  buyer: string
  order_no: string
  amount_minor: number
  granted_quota: number
  max_concurrency: number
  rpm_rate_limit: number
  model_limits_enabled: boolean
  model_limits: string
  allow_ips: string
  group: string
  cross_group_retry: boolean
  note: string
}

export type TokenSaleCreatePayload = TokenSaleBasePayload & {
  validity_days: number
}

export type TokenSaleUpdatePayload = TokenSaleBasePayload & {
  expired_time: number
  status: TokenSaleStatus
}

export type TokenSaleListResponse = {
  success: boolean
  message?: string
  data?: {
    items: TokenSale[]
    total: number
    page: number
    page_size: number
  }
}

export type TokenSaleOverviewResponse = {
  success: boolean
  message?: string
  data?: TokenSaleSummary
}

export type TokenSaleUsageResponse = {
  success: boolean
  message?: string
  data?: TokenSaleModelUsage[]
}

export type TokenSaleKeyResponse = {
  success: boolean
  message?: string
  data?: { key: string }
}
