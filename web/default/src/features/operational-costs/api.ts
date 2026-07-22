import { api } from '@/lib/api'

import type {
  AssetCostPayload,
  CostEntryPayload,
  OperationalAsset,
  OperationalCostEntry,
  OperationalCostOverviewResponse,
  OperationalCostPageResponse,
  OperationalCostSummaryResponse,
} from './types'

const actionConfig = {
  skipBusinessError: true,
  skipErrorHandler: true,
}

export async function getOperationalCostOverview(): Promise<OperationalCostOverviewResponse> {
  const response = await api.get('/api/operational-costs/overview')
  return response.data
}

export async function getOperationalCostSummary(): Promise<OperationalCostSummaryResponse> {
  const response = await api.get('/api/operational-costs/summary')
  return response.data
}

export async function listOperationalCostEntries(params: {
  page: number
  pageSize: number
}): Promise<OperationalCostPageResponse<OperationalCostEntry>> {
  const response = await api.get('/api/operational-costs/entries', {
    params: { p: params.page, page_size: params.pageSize },
  })
  return response.data
}

export async function listOperationalCostArchives(params: {
  page: number
  pageSize: number
  sourceType?: OperationalAsset['source_type']
  search?: string
}): Promise<OperationalCostPageResponse<OperationalAsset>> {
  const response = await api.get('/api/operational-costs/archives', {
    params: {
      p: params.page,
      page_size: params.pageSize,
      source_type: params.sourceType,
      search: params.search,
    },
  })
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
