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
import { api } from '@/lib/api'

import type {
  TokenSaleListResponse,
  TokenSaleCreatePayload,
  TokenSaleKeyResponse,
  TokenSaleOverviewResponse,
  TokenSaleUpdatePayload,
  TokenSaleUsageResponse,
} from './types'

const actionConfig = {
  skipBusinessError: true,
  skipErrorHandler: true,
}

export async function listTokenSales(params?: {
  keyword?: string
  status?: string
  page?: number
  pageSize?: number
}): Promise<TokenSaleListResponse> {
  const query = new URLSearchParams()
  query.set('p', String(params?.page ?? 1))
  query.set('page_size', String(params?.pageSize ?? 20))
  if (params?.keyword) query.set('keyword', params.keyword)
  if (params?.status) query.set('status', params.status)
  const response = await api.get(`/api/token-sales/?${query.toString()}`)
  return response.data
}

export async function getTokenSaleOverview(): Promise<TokenSaleOverviewResponse> {
  const response = await api.get('/api/token-sales/overview')
  return response.data
}

export async function getTokenSaleUsage(
  id: number
): Promise<TokenSaleUsageResponse> {
  const response = await api.get(`/api/token-sales/${id}/usage`)
  return response.data
}

export async function getTokenSaleKey(
  id: number
): Promise<TokenSaleKeyResponse> {
  const response = await api.get(`/api/token-sales/${id}/key`, actionConfig)
  return response.data
}

export async function createTokenSale(payload: TokenSaleCreatePayload) {
  const response = await api.post('/api/token-sales/', payload, actionConfig)
  return response.data as {
    success: boolean
    message?: string
    data?: {
      id: number
      token_id: number
      key: string
      expired_time: number
    }
  }
}

export async function updateTokenSale(
  id: number,
  payload: TokenSaleUpdatePayload
) {
  const response = await api.put(
    `/api/token-sales/${id}`,
    payload,
    actionConfig
  )
  return response.data as { success: boolean; message?: string }
}

export async function deleteTokenSale(id: number) {
  const response = await api.delete(`/api/token-sales/${id}`, actionConfig)
  return response.data as { success: boolean; message?: string }
}
