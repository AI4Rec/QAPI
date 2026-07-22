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
export type SystemInstanceStatus = 'online' | 'stale'

export type SystemInstanceInfo = {
  schema_version?: number
  node?: {
    name?: string
    source?: string
    manually_configured?: boolean
    should_configure_manually?: boolean
    [key: string]: unknown
  }
  role?: {
    is_master?: boolean
    [key: string]: unknown
  }
  runtime?: {
    version?: string
    goos?: string
    goarch?: string
    started_at?: number
    [key: string]: unknown
  }
  host?: {
    hostname?: string
    [key: string]: unknown
  }
  resources?: {
    cpu?: {
      usage_percent?: number
      [key: string]: unknown
    }
    memory?: {
      usage_percent?: number
      [key: string]: unknown
    }
    storage?: {
      total_bytes?: number
      used_bytes?: number
      free_bytes?: number
      used_percent?: number
      [key: string]: unknown
    }
    [key: string]: unknown
  }
  [key: string]: unknown
}

export type SystemInstance = {
  node_name: string
  status: SystemInstanceStatus
  stale_after_seconds: number
  started_at: number
  last_seen_at: number
  info?: SystemInstanceInfo
}

export type SystemInstanceListResponse = {
  success: boolean
  message: string
  data?: SystemInstance[]
}

export type SystemInstanceDeleteResponse = {
  success: boolean
  message: string
  data?: {
    deleted_count: number
  }
}

export type RedisRuntimeStats = {
  enabled: boolean
  healthy: boolean
  latency_ms: number
  version: string
  uptime_seconds: number
  used_memory_bytes: number
  peak_memory_bytes: number
  max_memory_bytes: number
  memory_usage_percent: number
  max_memory_policy: string
  connected_clients: number
  blocked_clients: number
  total_commands_processed: number
  instantaneous_ops_per_sec: number
  keyspace_hits: number
  keyspace_misses: number
  hit_rate_percent: number
  db_size: number
  pool: {
    hits: number
    misses: number
    timeouts: number
    total_conns: number
    idle_conns: number
    stale_conns: number
  }
  admission: {
    total: number
    small: number
    medium: number
    large: number
    huge: number
  }
  error?: string
}

export type ResponsesProtectionStats = {
  enabled: boolean
  max_request_body_mb: number
  spool_threshold_mb: number
  small_concurrency: number
  medium_concurrency: number
  large_concurrency: number
  huge_concurrency: number
  total_concurrency: number
  admission_wait_ms: number
}

export type RuntimeProtectionStats = {
  cache_stats?: {
    active_disk_files: number
    current_disk_usage_bytes: number
    disk_cache_hits: number
    active_memory_buffers: number
    current_memory_usage_bytes: number
  }
  redis_stats: RedisRuntimeStats
  responses_protection: ResponsesProtectionStats
}

export type RuntimeProtectionStatsResponse = {
  success: boolean
  message?: string
  data?: RuntimeProtectionStats
}
