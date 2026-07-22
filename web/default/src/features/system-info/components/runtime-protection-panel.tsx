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
import {
  Database02Icon,
  RefreshIcon,
  Shield01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'

import { getRuntimeProtectionStats } from '../api'

const RUNTIME_POLL_INTERVAL_MS = 10_000

function formatBytes(bytes?: number) {
  if (typeof bytes !== 'number' || Number.isNaN(bytes)) return '-'
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(
    Math.floor(Math.log(Math.abs(bytes)) / Math.log(1024)),
    units.length - 1
  )
  const value = bytes / 1024 ** index
  return `${new Intl.NumberFormat(undefined, {
    maximumFractionDigits: index === 0 ? 0 : 1,
  }).format(value)} ${units[index]}`
}

function formatNumber(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  return new Intl.NumberFormat().format(value)
}

function formatPercent(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  return `${new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 1,
  }).format(value)}%`
}

type MetricProps = {
  label: string
  value: string
}

function Metric(props: MetricProps) {
  return (
    <div className='bg-muted/40 rounded-lg p-3'>
      <p className='text-muted-foreground text-xs'>{props.label}</p>
      <p className='mt-1 font-mono text-sm font-medium tabular-nums'>
        {props.value}
      </p>
    </div>
  )
}

function RuntimeProtectionSkeleton() {
  return (
    <div className='grid gap-4 xl:grid-cols-2'>
      {['responses-protection-skeleton', 'redis-skeleton'].map((key) => (
        <Card key={key}>
          <CardHeader>
            <Skeleton className='h-5 w-44' />
            <Skeleton className='h-4 w-72 max-w-full' />
          </CardHeader>
          <CardContent className='grid grid-cols-2 gap-3'>
            <Skeleton className='h-16' />
            <Skeleton className='h-16' />
            <Skeleton className='h-16' />
            <Skeleton className='h-16' />
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

export function RuntimeProtectionPanel() {
  const { t } = useTranslation()
  const statsQuery = useQuery({
    queryKey: ['system-info', 'runtime-protection'],
    queryFn: getRuntimeProtectionStats,
    refetchInterval: RUNTIME_POLL_INTERVAL_MS,
    staleTime: 5000,
  })

  if (statsQuery.isPending) return <RuntimeProtectionSkeleton />

  const stats = statsQuery.data?.data
  if (!stats) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t('Runtime protection')}</CardTitle>
          <CardDescription>
            {t('Runtime protection metrics are temporarily unavailable.')}
          </CardDescription>
          <CardAction>
            <Button
              variant='outline'
              size='sm'
              onClick={() => statsQuery.refetch()}
            >
              <HugeiconsIcon icon={RefreshIcon} data-icon='inline-start' />
              {t('Retry')}
            </Button>
          </CardAction>
        </CardHeader>
      </Card>
    )
  }

  const protection = stats.responses_protection
  const redis = stats.redis_stats
  const memoryPercent = Math.max(
    0,
    Math.min(100, redis.memory_usage_percent || 0)
  )
  let redisStatusLabel = t('Disabled')
  if (redis.healthy) {
    redisStatusLabel = t('Healthy')
  } else if (redis.enabled) {
    redisStatusLabel = t('Unavailable')
  }

  return (
    <section aria-label={t('Runtime protection')}>
      <div className='mb-3 flex flex-wrap items-center justify-between gap-2'>
        <div>
          <h2 className='text-base font-medium'>{t('Runtime protection')}</h2>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Live status of large request protection and Redis coordination.'
            )}
          </p>
        </div>
        <Button
          variant='outline'
          size='sm'
          disabled={statsQuery.isFetching}
          onClick={() => statsQuery.refetch()}
        >
          {statsQuery.isFetching ? (
            <Spinner data-icon='inline-start' />
          ) : (
            <HugeiconsIcon icon={RefreshIcon} data-icon='inline-start' />
          )}
          {t('Refresh metrics')}
        </Button>
      </div>

      <div className='grid gap-4 xl:grid-cols-2'>
        <Card>
          <CardHeader>
            <CardTitle className='flex items-center gap-2'>
              <HugeiconsIcon
                icon={Shield01Icon}
                className='size-5'
                aria-hidden='true'
              />
              {t('Responses memory protection')}
            </CardTitle>
            <CardDescription>
              {t(
                'Large request bodies are spooled to disk before relay processing.'
              )}
            </CardDescription>
            <CardAction>
              <Badge variant={protection.enabled ? 'default' : 'destructive'}>
                {protection.enabled ? t('Enabled') : t('Disabled')}
              </Badge>
            </CardAction>
          </CardHeader>
          <CardContent className='flex flex-col gap-4'>
            <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
              <Metric
                label={t('Request body limit')}
                value={`${protection.max_request_body_mb} MB`}
              />
              <Metric
                label={t('Disk spool threshold')}
                value={`${protection.spool_threshold_mb} MB`}
              />
              <Metric
                label={t('Active disk bodies')}
                value={`${formatNumber(stats.cache_stats?.active_disk_files)} / ${formatBytes(stats.cache_stats?.current_disk_usage_bytes)}`}
              />
              <Metric
                label={t('Disk spool hits')}
                value={formatNumber(stats.cache_stats?.disk_cache_hits)}
              />
            </div>
            <div>
              <p className='mb-2 text-sm font-medium'>
                {t('Concurrency limits')}
              </p>
              <div className='flex flex-wrap gap-2'>
                <Badge variant='outline'>
                  {t('Small requests')}: {protection.small_concurrency}
                </Badge>
                <Badge variant='outline'>
                  {t('Medium requests')}: {protection.medium_concurrency}
                </Badge>
                <Badge variant='outline'>
                  {t('Large requests')}: {protection.large_concurrency}
                </Badge>
                <Badge variant='outline'>
                  {t('Huge requests')}: {protection.huge_concurrency}
                </Badge>
                <Badge variant='secondary'>
                  {t('Total')}: {protection.total_concurrency}
                </Badge>
                <Badge variant='secondary'>
                  {t('Admission wait')}: {protection.admission_wait_ms} ms
                </Badge>
              </div>
            </div>
          </CardContent>
          <CardFooter className='text-muted-foreground text-xs'>
            {t(
              'Requests above the limit are rejected before entering application memory.'
            )}
          </CardFooter>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className='flex items-center gap-2'>
              <HugeiconsIcon
                icon={Database02Icon}
                className='size-5'
                aria-hidden='true'
              />
              {t('Redis coordination')}
            </CardTitle>
            <CardDescription>
              {t('Redis coordinates rate limits and active request slots.')}
            </CardDescription>
            <CardAction>
              <Badge variant={redis.healthy ? 'default' : 'destructive'}>
                {redisStatusLabel}
              </Badge>
            </CardAction>
          </CardHeader>
          <CardContent className='flex flex-col gap-4'>
            <div>
              <div className='mb-2 flex items-center justify-between gap-3 text-sm'>
                <span className='font-medium'>{t('Memory usage')}</span>
                <span className='font-mono text-xs tabular-nums'>
                  {formatBytes(redis.used_memory_bytes)} /{' '}
                  {formatBytes(redis.max_memory_bytes)}
                </span>
              </div>
              <Progress value={memoryPercent} />
            </div>
            <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
              <Metric
                label={t('Connected clients')}
                value={formatNumber(redis.connected_clients)}
              />
              <Metric
                label={t('Operations per second')}
                value={formatNumber(redis.instantaneous_ops_per_sec)}
              />
              <Metric
                label={t('Cache hit rate')}
                value={formatPercent(redis.hit_rate_percent)}
              />
              <Metric
                label={t('Stored keys')}
                value={formatNumber(redis.db_size)}
              />
            </div>
            <div className='flex flex-wrap gap-2'>
              <Badge variant='secondary'>
                {t('Active requests')}: {redis.admission.total}
              </Badge>
              <Badge variant='outline'>
                {t('Small requests')}: {redis.admission.small}
              </Badge>
              <Badge variant='outline'>
                {t('Medium requests')}: {redis.admission.medium}
              </Badge>
              <Badge variant='outline'>
                {t('Large requests')}: {redis.admission.large}
              </Badge>
              <Badge variant='outline'>
                {t('Huge requests')}: {redis.admission.huge}
              </Badge>
            </div>
          </CardContent>
          <CardFooter className='text-muted-foreground flex flex-wrap justify-between gap-2 text-xs'>
            <span>
              Redis {redis.version || '-'} · {t('Policy')}:{' '}
              {redis.max_memory_policy || '-'}
            </span>
            <span>
              {t('Latency')}: {redis.latency_ms.toFixed(1)} ms ·{' '}
              {t('Connection pool')}: {redis.pool.idle_conns}/
              {redis.pool.total_conns}
            </span>
          </CardFooter>
        </Card>
      </div>
      <p className='text-muted-foreground mt-2 text-right text-xs'>
        {t('Auto-refreshes every 10 seconds.')}
      </p>
    </section>
  )
}
