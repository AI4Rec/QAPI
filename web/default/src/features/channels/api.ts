import type { SystemTask } from '@/features/system-settings/types'
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
import { getGroups as getUserGroups } from '@/features/users/api'
import { api, type ApiRequestConfig } from '@/lib/api'

import type {
  AddChannelRequest,
  BatchDeleteParams,
  BatchSetTagParams,
  Channel,
  ChannelBalanceResponse,
  ChannelOpsResponse,
  ChannelTestResponse,
  CopyChannelParams,
  CopyChannelResponse,
  FetchModelsResponse,
  GetChannelResponse,
  GetChannelsParams,
  GetChannelsResponse,
  MultiKeyManageParams,
  MultiKeyStatusResponse,
  SearchChannelsParams,
  SearchChannelsResponse,
  TagOperationParams,
} from './types'

const channelActionConfig = (
  config: ApiRequestConfig = {}
): ApiRequestConfig => ({
  ...config,
  skipBusinessError: true,
  skipErrorHandler: true,
})

export type CodexUsageResponse = {
  success: boolean
  message?: string
  upstream_status?: number
  data?: Record<string, unknown>
}

export type CodexResetCreditsResponse = CodexUsageResponse

export type CodexUsageResetResponse = CodexUsageResponse

export type CodexCredentialRefreshResponse = {
  success: boolean
  message?: string
  data?: {
    expires_at?: string
    last_refresh?: string
    account_id?: string
    email?: string
    channel_id?: number
    channel_type?: number
    channel_name?: string
  }
}

export type CPAImportResponse = {
  success: boolean
  message?: string
  data?: {
    total: number
    uploaded: number
    failed: Array<{ index: number; error: string }>
    files: string[]
  }
}

export type Sub2APIImportResponse = {
  success: boolean
  message?: string
  data?: Record<string, unknown>
}

export type ManagedComponent = 'cliproxyapi' | 'sub2api'

export type ComponentReleaseStatus = {
  component: ManagedComponent
  repository: string
  current_version: string
  latest_version: string
  release_name: string
  release_notes: string
  release_url: string
  published_at: string
  update_available: boolean
  rollback_available: boolean
  updater_available: boolean
  checked_at: number
}

export type ComponentUpdateTask = SystemTask<
  {
    action: 'update' | 'rollback'
    component: ManagedComponent
    target_version?: string
  },
  {
    component: ManagedComponent
    stage: 'preparing' | 'installing' | 'completed'
    progress: number
  },
  {
    action: 'update' | 'rollback'
    component: ManagedComponent
    target_version?: string
    output?: string
  }
>

export type ComponentUpdateStatusResponse = {
  success: boolean
  message?: string
  data?: {
    status: ComponentReleaseStatus
    active_task?: ComponentUpdateTask | null
  }
}

export type StartComponentUpdateResponse = {
  success: boolean
  message?: string
  data?: {
    task: ComponentUpdateTask
    created: boolean
  }
}

export async function getComponentUpdateStatus(
  component: ManagedComponent,
  refresh = false
): Promise<ComponentUpdateStatusResponse> {
  const res = await api.get(
    `/api/component-update/${component}`,
    channelActionConfig({ params: refresh ? { refresh: true } : undefined })
  )
  return res.data
}

export async function startComponentUpdate(
  component: ManagedComponent
): Promise<StartComponentUpdateResponse> {
  const res = await api.post(
    `/api/component-update/${component}`,
    null,
    channelActionConfig()
  )
  return res.data
}

export async function startComponentRollback(
  component: ManagedComponent
): Promise<StartComponentUpdateResponse> {
  const res = await api.post(
    `/api/component-update/${component}/rollback`,
    null,
    channelActionConfig()
  )
  return res.data
}

export type Sub2APIAccount = {
  id: number
  name: string
  email?: string
  platform: string
  type: string
  concurrency: number
  current_concurrency: number
  priority: number
  status: string
  schedulable: boolean
  error_message?: string
  expires_at?: number | null
  plan_type?: string
  usage?: CPAAccountUsage
  today_stats?: {
    requests: number
    tokens: number
    cost: number
    standard_cost: number
    user_cost: number
  }
  asset_key: string
  cost_minor: number
  cost_date: number
  cost_note: string
  cumulative_output_usd?: number
}

export type Sub2APIAccountSelectionResponse = {
  success: boolean
  message?: string
  data?: {
    ids: number[]
    total: number
  }
}

export type Sub2APIBatchAction =
  | 'enable'
  | 'disable'
  | 'cost'
  | 'archive'
  | 'delete'

export type AccountInspectionProvider = 'auto' | 'cpa' | 'sub2api'
export type AccountInspectionOperation = 'verify' | 'ping'
export type AccountInspectionTarget = {
  provider: Exclude<AccountInspectionProvider, 'auto'>
  identifier: string
  email?: string
  source: string
  state: string
}
export type AccountInspectionResult = AccountInspectionTarget & {
  operation: AccountInspectionOperation
  success: boolean
  latency_ms: number
  upstream_status?: number
  message?: string
  usage?: CPAAccountUsage
}
export type AccountInspectionTargetsResponse = {
  success: boolean
  message?: string
  data?: { items: AccountInspectionTarget[] }
}
export type AccountInspectionResponse = {
  success: boolean
  message?: string
  data?: AccountInspectionResult
}

export type Sub2APIBatchResponse = {
  success: boolean
  message?: string
  data?: {
    succeeded: number[]
    failed: Array<{ id: number; error: string }>
  }
}

export type Sub2APIAccountsResponse = {
  success: boolean
  message?: string
  data?: {
    items: Sub2APIAccount[]
    total: number
    page: number
    page_size: number
    pages: number
  }
}

export type CPAAccountPoolType = 'cpa_import' | 'temporary' | 'official_login'

export type CPAAccountSortBy =
  | 'name'
  | 'cost'
  | 'status'
  | 'success'
  | 'failed'
  | 'created_at'
  | 'updated_at'

export type CPAAccountSortOrder = 'asc' | 'desc'

export type CPAAccountBatchAction =
  | 'disable'
  | 'enable'
  | 'move'
  | 'cost'
  | 'archive'
  | 'delete'

export type CPAUsageWindow = {
  used_percent?: number
  reset_at?: number
  reset_after_seconds?: number
  limit_window_seconds?: number
}

export type CPAAccountUsage = {
  error?: string
  plan_type?: string
  rate_limit?: {
    allowed?: boolean
    limit_reached?: boolean
    primary_window?: CPAUsageWindow
    secondary_window?: CPAUsageWindow
  }
  credits?: {
    has_credits?: boolean
    unlimited?: boolean
    balance?: string
  }
}

export type CPAAccount = {
  name: string
  email?: string
  status?: string
  status_message?: string
  disabled: boolean
  unavailable?: boolean
  success?: number
  failed?: number
  created_at?: string
  updated_at?: string
  last_refresh?: string
  next_retry_after?: string
  plan_type?: string
  account_id?: string
  pool_type: CPAAccountPoolType
  duplicate?: boolean
  duplicate_count?: number
  usage?: CPAAccountUsage
  asset_key: string
  cost_minor?: number
  cost_date?: number
  cost_note?: string
  cumulative_output_usd?: number
}

export type CPAAccountsResponse = {
  success: boolean
  message?: string
  data?: {
    accounts: CPAAccount[]
    page: number
    page_size: number
    total: number
    sort_by: CPAAccountSortBy
    sort_order: CPAAccountSortOrder
    summary: {
      total: number
      unique: number
      active: number
      disabled: number
      duplicates: number
    }
  }
}

export type CPAAccountSelectionResponse = {
  success: boolean
  message?: string
  data?: {
    names: string[]
    total: number
  }
}

export type CPAAccountBatchRequest = {
  names: string[]
  action: CPAAccountBatchAction
  pool_type?: Extract<CPAAccountPoolType, 'cpa_import' | 'temporary'>
  cost_minor?: number
  cost_date?: number
  cost_note?: string
  archive_reason?: string
  delete_confirmation?: string
}

export type CPAAccountBatchResponse = {
  success: boolean
  message?: string
  data?: {
    total: number
    succeeded: string[]
    failed: Array<{ name: string; error: string }>
  }
}

export type CPAOAuthStartResponse = {
  success: boolean
  message?: string
  data?: {
    provider: string
    url: string
    state: string
    expires_at: number
  }
}

export type CPAOAuthStatusResponse = {
  success: boolean
  message?: string
  data?: {
    status: 'wait' | 'ok' | 'error'
    error?: string
  }
}

export type CPARemoteBrowserInput =
  | { type: 'click'; x: number; y: number }
  | { type: 'scroll'; x: number; y: number; delta_x: number; delta_y: number }
  | { type: 'text'; text: string }
  | { type: 'key'; key: string; code: string; key_code: number }

export async function importCPAAccounts(
  content: string,
  poolType: Extract<CPAAccountPoolType, 'cpa_import' | 'temporary'>
): Promise<CPAImportResponse> {
  const res = await api.post(
    '/api/cpa/import',
    { content, pool_type: poolType },
    channelActionConfig()
  )
  return res.data
}

export async function importSub2APIAccounts(
  content: string
): Promise<Sub2APIImportResponse> {
  const res = await api.post(
    '/api/sub2api/import',
    { content },
    channelActionConfig()
  )
  return res.data
}

export async function getAccountInspectionTargets(params: {
  provider: 'all' | Exclude<AccountInspectionProvider, 'auto'>
  scope: 'all' | 'attention'
}): Promise<AccountInspectionTargetsResponse> {
  const res = await api.get('/api/account-inspection/targets', {
    ...channelActionConfig(),
    params,
  })
  return res.data
}

export async function inspectAccount(request: {
  provider: AccountInspectionProvider
  operation: AccountInspectionOperation
  identifier?: string
  content?: string
}): Promise<AccountInspectionResponse> {
  const res = await api.post(
    '/api/account-inspection/run',
    request,
    channelActionConfig()
  )
  return res.data
}

export async function getSub2APIAccounts(params?: {
  page?: number
  pageSize?: number
}): Promise<Sub2APIAccountsResponse> {
  const res = await api.get('/api/sub2api/accounts', {
    ...channelActionConfig(),
    params: {
      page: params?.page ?? 1,
      page_size: params?.pageSize ?? 20,
      sort_by: 'created_at',
      sort_order: 'desc',
    },
  })
  return res.data
}

export async function setSub2APIAccountSchedulable(
  id: number,
  schedulable: boolean
): Promise<Sub2APIImportResponse> {
  const res = await api.post(
    `/api/sub2api/accounts/${id}/schedulable`,
    { schedulable },
    channelActionConfig()
  )
  return res.data
}

export async function deleteSub2APIAccount(
  id: number
): Promise<Sub2APIImportResponse> {
  const res = await api.delete(`/api/sub2api/accounts/${id}`, {
    ...channelActionConfig(),
  })
  return res.data
}

export async function archiveSub2APIAccount(
  id: number,
  reason = ''
): Promise<Sub2APIImportResponse> {
  const res = await api.post(
    `/api/sub2api/accounts/${id}/archive`,
    { reason },
    channelActionConfig()
  )
  return res.data
}

export async function getSub2APIAccountSelection(): Promise<Sub2APIAccountSelectionResponse> {
  const res = await api.get(
    '/api/sub2api/accounts/selection',
    channelActionConfig()
  )
  return res.data
}

export async function batchManageSub2APIAccounts(request: {
  account_ids: number[]
  action: Sub2APIBatchAction
  cost_minor?: number
  cost_date?: number
  cost_note?: string
  archive_reason?: string
}): Promise<Sub2APIBatchResponse> {
  const res = await api.post(
    '/api/sub2api/accounts/batch',
    request,
    channelActionConfig()
  )
  return res.data
}

export async function getCPAAccounts(params?: {
  poolType?: CPAAccountPoolType
  page?: number
  pageSize?: number
  sortBy?: CPAAccountSortBy
  sortOrder?: CPAAccountSortOrder
}): Promise<CPAAccountsResponse> {
  const res = await api.get('/api/cpa/accounts', {
    ...channelActionConfig(),
    params: {
      pool_type: params?.poolType,
      p: params?.page ?? 1,
      page_size: params?.pageSize ?? 20,
      sort_by: params?.sortBy,
      sort_order: params?.sortOrder,
    },
  })
  return res.data
}

export async function getCPAAccountSelection(
  poolType: CPAAccountPoolType
): Promise<CPAAccountSelectionResponse> {
  const res = await api.get('/api/cpa/accounts/selection', {
    ...channelActionConfig(),
    params: { pool_type: poolType },
  })
  return res.data
}

export async function batchManageCPAAccounts(
  request: CPAAccountBatchRequest
): Promise<CPAAccountBatchResponse> {
  const res = await api.post(
    '/api/cpa/accounts/batch',
    request,
    channelActionConfig()
  )
  return res.data
}

export async function setCPAAccountStatus(
  name: string,
  disabled: boolean
): Promise<{ success: boolean; message?: string }> {
  const res = await api.patch(
    '/api/cpa/accounts/status',
    { name, disabled },
    channelActionConfig()
  )
  return res.data
}

export async function deleteCPAAccount(
  name: string
): Promise<{ success: boolean; message?: string }> {
  const res = await api.delete('/api/cpa/accounts', {
    ...channelActionConfig(),
    params: { name },
  })
  return res.data
}

export async function startCPACodexOAuth(): Promise<CPAOAuthStartResponse> {
  const res = await api.post(
    '/api/cpa/oauth/codex/start',
    undefined,
    channelActionConfig()
  )
  return res.data
}

export async function submitCPAOAuthCallback(
  redirectUrl: string
): Promise<{ success: boolean; message?: string }> {
  const res = await api.post(
    '/api/cpa/oauth/callback',
    { redirect_url: redirectUrl },
    channelActionConfig()
  )
  return res.data
}

export async function getCPAOAuthStatus(
  state: string
): Promise<CPAOAuthStatusResponse> {
  const res = await api.get('/api/cpa/oauth/status', {
    ...channelActionConfig(),
    params: { state },
  })
  return res.data
}

export async function cancelCPAOAuth(
  state: string
): Promise<{ success: boolean; message?: string }> {
  const res = await api.delete('/api/cpa/oauth/session', {
    ...channelActionConfig(),
    params: { state },
  })
  return res.data
}

export async function getCPAOAuthBrowserFrame(state: string): Promise<string> {
  const res = await api.get('/api/cpa/oauth/browser/frame', {
    ...channelActionConfig(),
    params: { state, timestamp: Date.now() },
    responseType: 'blob',
    disableDuplicate: true,
  })
  return URL.createObjectURL(res.data)
}

export async function sendCPAOAuthBrowserInput(
  state: string,
  input: CPARemoteBrowserInput
): Promise<void> {
  await api.post('/api/cpa/oauth/browser/input', input, {
    ...channelActionConfig(),
    params: { state },
  })
}

// ============================================================================
// Base Channel CRUD Operations
// ============================================================================

/**
 * Get paginated list of channels
 */
export async function getChannels(
  params: GetChannelsParams = {}
): Promise<GetChannelsResponse> {
  const res = await api.get('/api/channel', { params })
  return res.data
}

/**
 * Search channels with filters
 */
export async function searchChannels(
  params: SearchChannelsParams
): Promise<SearchChannelsResponse> {
  const res = await api.get('/api/channel/search', { params })
  return res.data
}

/**
 * Get single channel by ID
 */
export async function getChannel(id: number): Promise<GetChannelResponse> {
  const res = await api.get(`/api/channel/${id}`)
  return res.data
}

/**
 * Get channel operations summary for administrators
 */
export async function getChannelOps(): Promise<ChannelOpsResponse> {
  const res = await api.get('/api/channel/ops', channelActionConfig())
  return res.data
}

/**
 * Create new channel(s)
 * Supports single, batch, and multi-key modes
 */
export async function createChannel(
  data: AddChannelRequest
): Promise<{ success: boolean; message?: string }> {
  const res = await api.post('/api/channel', data, channelActionConfig())
  return res.data
}

/**
 * Update existing channel
 */
export async function updateChannel(
  id: number,
  data: Partial<Channel>
): Promise<{ success: boolean; message?: string; data?: Channel }> {
  const res = await api.put(
    '/api/channel/',
    { id, ...data },
    channelActionConfig()
  )
  return res.data
}

/**
 * Update channel enabled/disabled status.
 */
export async function updateChannelStatus(
  id: number,
  status: number
): Promise<{ success: boolean; message?: string; data?: boolean }> {
  const res = await api.post(
    `/api/channel/${id}/status`,
    { status },
    channelActionConfig()
  )
  return res.data
}

/**
 * Batch update channel enabled/disabled status.
 */
export async function batchUpdateChannelStatus(
  ids: number[],
  status: number
): Promise<{ success: boolean; message?: string; data?: number }> {
  const res = await api.post(
    '/api/channel/status/batch',
    { ids, status },
    channelActionConfig()
  )
  return res.data
}

/**
 * Delete single channel
 */
export async function deleteChannel(
  id: number
): Promise<{ success: boolean; message?: string }> {
  const res = await api.delete(`/api/channel/${id}`, channelActionConfig())
  return res.data
}

/**
 * Batch delete channels
 */
export async function batchDeleteChannels(
  data: BatchDeleteParams
): Promise<{ success: boolean; message?: string; data?: number }> {
  const res = await api.post('/api/channel/batch', data, channelActionConfig())
  return res.data
}

/**
 * Batch set tag for channels
 */
export async function batchSetChannelTag(
  data: BatchSetTagParams
): Promise<{ success: boolean; message?: string; data?: number }> {
  const res = await api.post(
    '/api/channel/batch/tag',
    data,
    channelActionConfig()
  )
  return res.data
}

// ============================================================================
// Channel Operations
// ============================================================================

/**
 * Test channel connectivity
 */
export async function testChannel(
  id: number,
  params?: { model?: string; endpoint_type?: string; stream?: boolean }
): Promise<ChannelTestResponse> {
  const res = await api.get(
    `/api/channel/test/${id}`,
    channelActionConfig({ params })
  )
  return res.data
}

/**
 * Update channel balance
 */
export async function updateChannelBalance(
  id: number
): Promise<ChannelBalanceResponse> {
  const res = await api.get(
    `/api/channel/update_balance/${id}`,
    channelActionConfig()
  )
  return res.data
}

/**
 * Fetch available models from upstream provider
 */
export async function fetchUpstreamModels(
  id: number
): Promise<FetchModelsResponse> {
  const res = await api.get(
    `/api/channel/fetch_models/${id}`,
    channelActionConfig()
  )
  return res.data
}

/**
 * Copy/clone a channel
 */
export async function copyChannel(
  id: number,
  params: CopyChannelParams = {}
): Promise<CopyChannelResponse> {
  const res = await api.post(
    `/api/channel/copy/${id}`,
    null,
    channelActionConfig({ params })
  )
  return res.data
}

/**
 * Fix channel abilities
 */
export async function fixChannelAbilities(): Promise<{
  success: boolean
  message?: string
  data?: { success: number; fails: number }
}> {
  const res = await api.post(
    '/api/channel/fix',
    undefined,
    channelActionConfig()
  )
  return res.data
}

/**
 * Delete all disabled channels
 */
export async function deleteDisabledChannels(): Promise<{
  success: boolean
  message?: string
  data?: number
}> {
  const res = await api.delete('/api/channel/disabled', channelActionConfig())
  return res.data
}

/**
 * Get channel key (requires 2FA verification)
 */
export async function getChannelKey(
  id: number,
  code?: string
): Promise<{ success: boolean; message?: string; data?: { key: string } }> {
  const payload = code ? { code } : undefined
  const res = await api.post(
    `/api/channel/${id}/key`,
    payload,
    channelActionConfig()
  )
  return res.data
}

// ============================================================================
// Codex Channel Operations
// ============================================================================

export async function refreshCodexCredential(
  channelId: number
): Promise<CodexCredentialRefreshResponse> {
  const res = await api.post(
    `/api/channel/${channelId}/codex/refresh`,
    {},
    channelActionConfig()
  )
  return res.data
}

export async function getCodexUsage(
  channelId: number
): Promise<CodexUsageResponse> {
  const res = await api.get(
    `/api/channel/${channelId}/codex/usage`,
    channelActionConfig({ disableDuplicate: true })
  )
  return res.data
}

export async function getCodexResetCredits(
  channelId: number
): Promise<CodexResetCreditsResponse> {
  const res = await api.get(
    `/api/channel/${channelId}/codex/usage/reset-credits`,
    channelActionConfig({ disableDuplicate: true })
  )
  return res.data
}

export async function resetCodexUsage(
  channelId: number
): Promise<CodexUsageResetResponse> {
  const res = await api.post(
    `/api/channel/${channelId}/codex/usage/reset`,
    {},
    channelActionConfig({ disableDuplicate: true })
  )
  return res.data
}

// ============================================================================
// Multi-Key Management
// ============================================================================

/**
 * Manage multi-key channel operations
 */
export async function manageMultiKeys(
  params: MultiKeyManageParams
): Promise<MultiKeyStatusResponse | { success: boolean; message?: string }> {
  const res = await api.post(
    '/api/channel/multi_key/manage',
    params,
    channelActionConfig()
  )
  return res.data
}

/**
 * Get key status for multi-key channel
 */
export async function getMultiKeyStatus(
  channelId: number,
  page = 1,
  pageSize = 50,
  status?: number
): Promise<MultiKeyStatusResponse> {
  return manageMultiKeys({
    channel_id: channelId,
    action: 'get_key_status',
    page,
    page_size: pageSize,
    status,
  }) as Promise<MultiKeyStatusResponse>
}

/**
 * Enable a specific key in multi-key channel
 */
export async function enableMultiKey(
  channelId: number,
  keyIndex: number
): Promise<{ success: boolean; message?: string }> {
  return manageMultiKeys({
    channel_id: channelId,
    action: 'enable_key',
    key_index: keyIndex,
  }) as Promise<{ success: boolean; message?: string }>
}

/**
 * Disable a specific key in multi-key channel
 */
export async function disableMultiKey(
  channelId: number,
  keyIndex: number
): Promise<{ success: boolean; message?: string }> {
  return manageMultiKeys({
    channel_id: channelId,
    action: 'disable_key',
    key_index: keyIndex,
  }) as Promise<{ success: boolean; message?: string }>
}

/**
 * Delete a specific key in multi-key channel
 */
export async function deleteMultiKey(
  channelId: number,
  keyIndex: number
): Promise<{ success: boolean; message?: string }> {
  return manageMultiKeys({
    channel_id: channelId,
    action: 'delete_key',
    key_index: keyIndex,
  }) as Promise<{ success: boolean; message?: string }>
}

/**
 * Enable all keys in multi-key channel
 */
export async function enableAllMultiKeys(
  channelId: number
): Promise<{ success: boolean; message?: string }> {
  return manageMultiKeys({
    channel_id: channelId,
    action: 'enable_all_keys',
  }) as Promise<{ success: boolean; message?: string }>
}

/**
 * Disable all keys in multi-key channel
 */
export async function disableAllMultiKeys(
  channelId: number
): Promise<{ success: boolean; message?: string }> {
  return manageMultiKeys({
    channel_id: channelId,
    action: 'disable_all_keys',
  }) as Promise<{ success: boolean; message?: string }>
}

/**
 * Delete all disabled keys in multi-key channel
 */
export async function deleteDisabledMultiKeys(
  channelId: number
): Promise<{ success: boolean; message?: string; data?: number }> {
  return manageMultiKeys({
    channel_id: channelId,
    action: 'delete_disabled_keys',
  }) as Promise<{ success: boolean; message?: string; data?: number }>
}

// ============================================================================
// Tag Operations
// ============================================================================

/**
 * Enable all channels with a specific tag
 */
export async function enableTagChannels(
  tag: string
): Promise<{ success: boolean; message?: string }> {
  const res = await api.post(
    '/api/channel/tag/enabled',
    { tag },
    channelActionConfig()
  )
  return res.data
}

/**
 * Disable all channels with a specific tag
 */
export async function disableTagChannels(
  tag: string
): Promise<{ success: boolean; message?: string }> {
  const res = await api.post(
    '/api/channel/tag/disabled',
    { tag },
    channelActionConfig()
  )
  return res.data
}

/**
 * Edit all channels with a specific tag
 */
export async function editTagChannels(
  params: TagOperationParams
): Promise<{ success: boolean; message?: string }> {
  const res = await api.put('/api/channel/tag', params, channelActionConfig())
  return res.data
}

/**
 * Get models for a specific tag
 */
export async function getTagModels(
  tag: string
): Promise<{ success: boolean; message?: string; data?: string }> {
  const res = await api.get('/api/channel/tag/models', { params: { tag } })
  return res.data
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Fetch models from a custom endpoint (for testing before creating channel)
 */
export async function fetchModels(data: {
  base_url: string
  type: number
  key: string
}): Promise<FetchModelsResponse> {
  const res = await api.post(
    '/api/channel/fetch_models',
    data,
    channelActionConfig()
  )
  return res.data
}

/**
 * Delete an Ollama model from a channel
 */
export async function deleteOllamaModel(params: {
  channel_id: number
  model_name: string
}): Promise<{ success: boolean; message?: string }> {
  const res = await api.delete(
    '/api/channel/ollama/delete',
    channelActionConfig({ data: params })
  )
  return res.data
}

/**
 * Test all enabled channels
 */
export async function testAllChannels(): Promise<{
  success: boolean
  message?: string
}> {
  const res = await api.get('/api/channel/test', channelActionConfig())
  return res.data
}

/**
 * Update balance for all enabled channels
 */
export async function updateAllChannelsBalance(): Promise<{
  success: boolean
  message?: string
}> {
  const res = await api.get(
    '/api/channel/update_balance',
    channelActionConfig()
  )
  return res.data
}

/**
 * Get all available models
 */
export async function getAllModels(): Promise<{
  success: boolean
  message?: string
  data?: Array<{ id: string; [key: string]: unknown }>
}> {
  const res = await api.get('/api/channel/models')
  return res.data
}

/**
 * Get all enabled models
 */
export async function getEnabledModels(): Promise<{
  success: boolean
  message?: string
  data?: string[]
}> {
  const res = await api.get('/api/channel/models_enabled')
  return res.data
}

// ============================================================================
// Ollama Utilities
// ============================================================================

/**
 * Check Ollama version for a given channel
 */
export async function getOllamaVersion(
  channelId: number
): Promise<{ success: boolean; message?: string; data?: { version: string } }> {
  const res = await api.get(`/api/channel/ollama/version/${channelId}`)
  return res.data
}

// ============================================================================
// Group Management
// ============================================================================

/**
 * Get all available groups (re-exported from users API for convenience)
 */
export const getGroups = getUserGroups

// ============================================================================
// Prefill Groups (Model Groups)
// ============================================================================

/**
 * Get prefill groups for quick model selection
 */
export async function getPrefillGroups(
  type: 'model' | 'group' = 'model'
): Promise<{
  success: boolean
  message?: string
  data?: Array<{ id: number; name: string; items: string | string[] }>
}> {
  const res = await api.get('/api/prefill_group', { params: { type } })
  return res.data
}
