import { api } from '@/lib/api'

import type {
  ActivationActionResponse,
  ActivationQueueJobsResponse,
  ActivationQueueResponse,
  ActivationQueueSummaryResponse,
  ActivationQueueTargetsResponse,
} from './types'

export async function getActivationQueue(): Promise<ActivationQueueResponse> {
  const response = await api.get<ActivationQueueResponse>(
    '/api/activation-queue/overview',
    { params: { limit: 120 } }
  )
  return response.data
}

export async function getActivationQueueSummary(): Promise<ActivationQueueSummaryResponse> {
  const response = await api.get<ActivationQueueSummaryResponse>(
    '/api/activation-queue/summary'
  )
  return response.data
}

export async function listActivationQueueJobs(params: {
  page?: number
  pageSize?: number
}): Promise<ActivationQueueJobsResponse> {
  const response = await api.get<ActivationQueueJobsResponse>(
    '/api/activation-queue/jobs',
    {
      params: { p: params.page ?? 1, page_size: params.pageSize ?? 10 },
    }
  )
  return response.data
}

export async function listActivationQueueTargets(params: {
  page?: number
  pageSize?: number
}): Promise<ActivationQueueTargetsResponse> {
  const response = await api.get<ActivationQueueTargetsResponse>(
    '/api/activation-queue/targets',
    {
      params: { p: params.page ?? 1, page_size: params.pageSize ?? 20 },
    }
  )
  return response.data
}

export async function reconcileActivationQueue(): Promise<ActivationActionResponse> {
  const response = await api.post<ActivationActionResponse>(
    '/api/activation-queue/reconcile'
  )
  return response.data
}

export async function setActivationQueuePaused(
  paused: boolean
): Promise<ActivationActionResponse> {
  const response = await api.post<ActivationActionResponse>(
    '/api/activation-queue/pause',
    { paused }
  )
  return response.data
}

export async function setActivationTargetEnabled(
  targetId: number,
  enabled: boolean
): Promise<ActivationActionResponse> {
  const response = await api.patch<ActivationActionResponse>(
    `/api/activation-queue/targets/${targetId}`,
    { enabled }
  )
  return response.data
}

export async function runActivationJobNow(
  jobId: number
): Promise<ActivationActionResponse> {
  const response = await api.post<ActivationActionResponse>(
    `/api/activation-queue/jobs/${jobId}/run`
  )
  return response.data
}

export async function snoozeActivationJob(
  jobId: number,
  seconds: number
): Promise<ActivationActionResponse> {
  const response = await api.post<ActivationActionResponse>(
    `/api/activation-queue/jobs/${jobId}/snooze`,
    { seconds }
  )
  return response.data
}

export async function skipActivationJob(
  jobId: number
): Promise<ActivationActionResponse> {
  const response = await api.post<ActivationActionResponse>(
    `/api/activation-queue/jobs/${jobId}/skip`
  )
  return response.data
}
