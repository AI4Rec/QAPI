import { useQuery } from '@tanstack/react-query'
import {
  AlarmClock,
  CheckCircle2,
  CirclePause,
  CirclePlay,
  Clock3,
  Loader2,
  Play,
  RefreshCw,
  SkipForward,
  TimerReset,
  XCircle,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { ScrollArea } from '@/components/ui/scroll-area'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import {
  getActivationQueue,
  reconcileActivationQueue,
  runActivationJobNow,
  setActivationQueuePaused,
  setActivationTargetEnabled,
  skipActivationJob,
  snoozeActivationJob,
} from './api'
import type { ActivationJob, ActivationJobStatus } from './types'

const ACTIVE_STATUSES: ActivationJobStatus[] = ['pending', 'running']

function formatTime(timestamp: number): string {
  if (!timestamp) return '—'
  return new Date(timestamp * 1000).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatDateTime(timestamp: number): string {
  if (!timestamp) return '—'
  return new Date(timestamp * 1000).toLocaleString()
}

function sourceLabel(sourceType: string): string {
  if (sourceType === 'cpa_account') return 'CPA'
  if (sourceType === 'codex_channel') return 'Codex'
  return sourceType
}

function statusIcon(status: ActivationJobStatus) {
  if (status === 'running') return <Loader2 className='size-3.5 animate-spin' />
  if (status === 'succeeded') return <CheckCircle2 className='size-3.5 text-emerald-500' />
  if (status === 'failed') return <XCircle className='text-destructive size-3.5' />
  if (status === 'skipped') return <SkipForward className='text-muted-foreground size-3.5' />
  return <Clock3 className='size-3.5 text-amber-500' />
}

type QueueJobRowProps = {
  job: ActivationJob
  workingKey: string | null
  onAction: (key: string, action: () => Promise<unknown>) => Promise<void>
}

function QueueJobRow(props: QueueJobRowProps) {
  const { t } = useTranslation()
  const active = ACTIVE_STATUSES.includes(props.job.status)
  const working = props.workingKey?.endsWith(`:${props.job.id}`) ?? false

  return (
    <div className='space-y-2 rounded-md border p-2.5'>
      <div className='flex items-start justify-between gap-2'>
        <div className='min-w-0'>
          <div className='flex items-center gap-1.5 text-sm font-medium'>
            {statusIcon(props.job.status)}
            <span className='tabular-nums'>{formatTime(props.job.scheduled_at)}</span>
            <span className='truncate'>{props.job.target.display_name}</span>
          </div>
          <div className='text-muted-foreground mt-1 flex flex-wrap gap-1 text-[11px]'>
            <span>{sourceLabel(props.job.target.source_type)}</span>
            <span>·</span>
            <span>{props.job.target.plan_type || t('Unknown plan')}</span>
            <span>·</span>
            <span>{Math.round(props.job.target.window_seconds / 3600)}h</span>
          </div>
        </div>
        <Badge variant={props.job.status === 'failed' ? 'destructive' : 'outline'}>
          {t(`Activation status: ${props.job.status}`)}
        </Badge>
      </div>

      {(props.job.error || props.job.result) && (
        <div
          className={cn(
            'rounded bg-muted px-2 py-1 text-[11px]',
            props.job.error && 'text-destructive'
          )}
        >
          {props.job.error || props.job.result}
        </div>
      )}

      {active && (
        <div className='flex flex-wrap justify-end gap-1'>
          <Button
            size='xs'
            variant='outline'
            disabled={working || props.job.status === 'running'}
            onClick={() =>
              props.onAction(`run:${props.job.id}`, () =>
                runActivationJobNow(props.job.id)
              )
            }
          >
            <Play className='size-3' />
            {t('Run now')}
          </Button>
          <Button
            size='xs'
            variant='outline'
            disabled={working || props.job.status === 'running'}
            onClick={() =>
              props.onAction(`snooze:${props.job.id}`, () =>
                snoozeActivationJob(props.job.id, 30 * 60)
              )
            }
          >
            <TimerReset className='size-3' />
            {t('Delay 30 minutes')}
          </Button>
          <Button
            size='xs'
            variant='ghost'
            disabled={working || props.job.status === 'running'}
            onClick={() =>
              props.onAction(`skip:${props.job.id}`, () =>
                skipActivationJob(props.job.id)
              )
            }
          >
            <SkipForward className='size-3' />
            {t('Skip cycle')}
          </Button>
          <Button
            size='xs'
            variant='ghost'
            disabled={working}
            onClick={() =>
              props.onAction(`target:${props.job.id}`, () =>
                setActivationTargetEnabled(
                  props.job.target.id,
                  !props.job.target.enabled
                )
              )
            }
          >
            {props.job.target.enabled ? (
              <CirclePause className='size-3' />
            ) : (
              <CirclePlay className='size-3' />
            )}
            {props.job.target.enabled ? t('Pause target') : t('Resume target')}
          </Button>
        </div>
      )}
    </div>
  )
}

export function ActivationQueueWidget() {
  const { t } = useTranslation()
  const userRole = useAuthStore((state) => state.auth.user?.role ?? 0)
  const [workingKey, setWorkingKey] = useState<string | null>(null)
  const query = useQuery({
    queryKey: ['activation-queue'],
    queryFn: getActivationQueue,
    enabled: userRole >= ROLE.ADMIN,
    refetchInterval: 30_000,
    staleTime: 15_000,
  })

  if (userRole < ROLE.ADMIN) return null

  const overview = query.data?.data
  const jobs = overview?.jobs ?? []
  const activeJobs = jobs.filter((job) => ACTIVE_STATUSES.includes(job.status))
  const historyJobs = jobs
    .filter((job) => !ACTIVE_STATUSES.includes(job.status))
    .slice(-10)
    .reverse()
  const nextJob = activeJobs.find((job) => job.status === 'pending')

  const runAction = async (key: string, action: () => Promise<unknown>) => {
    setWorkingKey(key)
    try {
      const result = (await action()) as { success?: boolean; message?: string }
      if (result.success === false) throw new Error(result.message || t('Operation failed'))
      await query.refetch()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Operation failed'))
    } finally {
      setWorkingKey(null)
    }
  }

  return (
    <Popover>
      <PopoverTrigger
        render={<Button variant='outline' size='sm' className='max-w-[320px]' />}
      >
        <AlarmClock className='size-4' />
        <span className='max-w-[220px] truncate max-sm:hidden'>
          {nextJob
            ? `${formatTime(nextJob.scheduled_at)} ${t('Ping')} ${nextJob.target.display_name}`
            : t('Activation queue idle')}
        </span>
        {activeJobs.length > 0 && (
          <Badge variant='secondary'>{activeJobs.length}</Badge>
        )}
      </PopoverTrigger>
      <PopoverContent
        side='bottom'
        align='end'
        sideOffset={10}
        className='w-[min(430px,calc(100vw-2rem))] gap-3 p-3'
      >
          <div className='flex items-start justify-between gap-2'>
            <div>
              <div className='flex items-center gap-2 font-semibold'>
                <AlarmClock className='size-4' />
                {t('Automatic activation queue')}
                <Badge variant={overview?.paused ? 'secondary' : 'outline'}>
                  {overview?.paused ? t('Paused') : t('Running')}
                </Badge>
              </div>
              <div className='text-muted-foreground mt-1 text-xs'>
                {t('Last discovery')}: {formatDateTime(overview?.last_reconcile_at ?? 0)}
              </div>
            </div>
            <div className='flex gap-1'>
              <Button
                size='icon-sm'
                variant='outline'
                disabled={workingKey !== null}
                onClick={() =>
                  runAction('reconcile:all', reconcileActivationQueue)
                }
                aria-label={t('Rediscover')}
              >
                <RefreshCw className={cn('size-4', query.isFetching && 'animate-spin')} />
              </Button>
              <Button
                size='icon-sm'
                variant={overview?.paused ? 'default' : 'outline'}
                disabled={workingKey !== null}
                onClick={() =>
                  runAction('pause:all', () =>
                    setActivationQueuePaused(!overview?.paused)
                  )
                }
                aria-label={overview?.paused ? t('Resume queue') : t('Pause queue')}
              >
                {overview?.paused ? (
                  <CirclePlay className='size-4' />
                ) : (
                  <CirclePause className='size-4' />
                )}
              </Button>
            </div>
          </div>

          <div className='flex gap-2 text-xs'>
            <Badge variant='outline'>
              {t('Pending')} {activeJobs.length}
            </Badge>
            <Badge variant='outline'>
              {t('Targets')} {overview?.targets.length ?? 0}
            </Badge>
            <Badge variant='outline'>
              {t('Failed')} {jobs.filter((job) => job.status === 'failed').length}
            </Badge>
          </div>

          <ScrollArea className='max-h-[55vh] pr-2'>
            <div className='space-y-2'>
              {query.isLoading ? (
                <div className='flex items-center justify-center gap-2 py-8 text-sm'>
                  <Loader2 className='size-4 animate-spin' />
                  {t('Loading activation queue')}
                </div>
              ) : activeJobs.length === 0 ? (
                <div className='text-muted-foreground py-6 text-center text-sm'>
                  {t('No pending activation jobs')}
                </div>
              ) : (
                activeJobs.map((job) => (
                  <QueueJobRow
                    key={job.id}
                    job={job}
                    workingKey={workingKey}
                    onAction={runAction}
                  />
                ))
              )}

              {historyJobs.length > 0 && (
                <div className='pt-2'>
                  <div className='text-muted-foreground mb-2 text-xs font-medium'>
                    {t('Recent executions')}
                  </div>
                  <div className='space-y-2 opacity-80'>
                    {historyJobs.map((job) => (
                      <QueueJobRow
                        key={job.id}
                        job={job}
                        workingKey={workingKey}
                        onAction={runAction}
                      />
                    ))}
                  </div>
                </div>
              )}
            </div>
          </ScrollArea>
      </PopoverContent>
    </Popover>
  )
}
