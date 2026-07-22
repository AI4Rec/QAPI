export type ActivationJobStatus =
  | 'pending'
  | 'running'
  | 'succeeded'
  | 'failed'
  | 'skipped'
  | 'cancelled'

export type ActivationTarget = {
  id: number
  target_key: string
  source_type: 'cpa_account' | 'codex_channel' | string
  source_id: string
  display_name: string
  plan_type?: string
  enabled: boolean
  auto_managed: boolean
  available: boolean
  window_seconds: number
  reset_at: number
  last_seen_at: number
  last_activated_at: number
  last_status?: string
  last_error?: string
}

export type ActivationJob = {
  id: number
  job_id: string
  target_id: number
  dedupe_key: string
  scheduled_at: number
  status: ActivationJobStatus
  trigger: 'automatic' | 'manual' | string
  attempt: number
  max_attempts: number
  started_at: number
  finished_at: number
  previous_reset_at: number
  new_reset_at: number
  result?: string
  error?: string
  target: ActivationTarget
}

export type ActivationQueueOverview = {
  paused: boolean
  last_reconcile_at: number
  targets: ActivationTarget[]
  jobs: ActivationJob[]
}

export type ActivationQueueResponse = {
  success: boolean
  message?: string
  data?: ActivationQueueOverview
}

export type ActivationQueueSummary = {
  paused: boolean
  last_reconcile_at: number
  targets: number
  enabled_targets: number
  pending_jobs: number
  running_jobs: number
  failed_jobs: number
}

export type ActivationQueueSummaryResponse = {
  success: boolean
  message?: string
  data?: ActivationQueueSummary
}

export type ActivationQueueJobsResponse = {
  success: boolean
  message?: string
  data?: {
    page: number
    page_size: number
    total: number
    items: ActivationJob[]
  }
}

export type ActivationQueueTargetsResponse = {
  success: boolean
  message?: string
  data?: {
    page: number
    page_size: number
    total: number
    items: ActivationTarget[]
  }
}

export type ActivationActionResponse = {
  success: boolean
  message?: string
}
