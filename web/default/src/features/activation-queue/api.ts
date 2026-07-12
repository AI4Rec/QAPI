import { api } from '@/lib/api'

import type {
  ActivationActionResponse,
  ActivationQueueResponse,
} from './types'

export async function getActivationQueue(): Promise<ActivationQueueResponse> {
  const response = await api.get<ActivationQueueResponse>(
    '/api/activation-queue/overview',
    { params: { limit: 120 } }
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
