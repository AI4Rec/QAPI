import { api } from '@/lib/api'

import type {
  AssetCostPayload,
  CostEntryPayload,
  OperationalCostOverviewResponse,
} from './types'

const actionConfig = {
  skipBusinessError: true,
  skipErrorHandler: true,
}

export async function getOperationalCostOverview(): Promise<OperationalCostOverviewResponse> {
  const response = await api.get('/api/operational-costs/overview')
  return response.data
}

export async function saveAssetCost(payload: AssetCostPayload) {
  const response = await api.patch(
    '/api/operational-costs/assets/cost',
    payload,
    actionConfig
  )
  return response.data as { success: boolean; message?: string }
}

export async function archiveChannel(channelId: number, reason: string) {
  const response = await api.post(
    `/api/operational-costs/assets/channel/${channelId}/archive`,
    { reason },
    actionConfig
  )
  return response.data as { success: boolean; message?: string }
}

export async function archiveCPAAccount(name: string, reason: string) {
  const response = await api.post(
    '/api/cpa/accounts/archive',
    { name, reason },
    actionConfig
  )
  return response.data as { success: boolean; message?: string }
}

export async function restoreOperationalAsset(assetId: number) {
  const response = await api.post(
    `/api/operational-costs/archives/${assetId}/restore`,
    undefined,
    actionConfig
  )
  return response.data as { success: boolean; message?: string }
}

export async function createCostEntry(payload: CostEntryPayload) {
  const response = await api.post(
    '/api/operational-costs/entries',
    payload,
    actionConfig
  )
  return response.data as { success: boolean; message?: string }
}

export async function updateCostEntry(id: number, payload: CostEntryPayload) {
  const response = await api.put(
    `/api/operational-costs/entries/${id}`,
    payload,
    actionConfig
  )
  return response.data as { success: boolean; message?: string }
}

export async function deleteCostEntry(id: number) {
  const response = await api.delete(
    `/api/operational-costs/entries/${id}`,
    actionConfig
  )
  return response.data as { success: boolean; message?: string }
}
