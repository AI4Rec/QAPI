import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Download,
  ExternalLink,
  Loader2,
  PackageCheck,
  RefreshCw,
  Undo2,
} from 'lucide-react'
import { type ReactNode, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { getSystemTask } from '@/features/system-settings/api'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import {
  getComponentUpdateStatus,
  startComponentRollback,
  startComponentUpdate,
  type ComponentUpdateTask,
  type ManagedComponent,
} from '../api'

const COMPONENT_NAME: Record<ManagedComponent, string> = {
  cliproxyapi: 'CLIProxyAPI',
  sub2api: 'Sub2API',
}

type ComponentVersionBarProps = {
  component: ManagedComponent
}

function isActiveTask(task: ComponentUpdateTask | null | undefined) {
  return task?.status === 'pending' || task?.status === 'running'
}

export function ComponentVersionBar({ component }: ComponentVersionBarProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isRoot = useAuthStore(
    (state) => state.auth.user?.role === ROLE.SUPER_ADMIN
  )
  const [confirmation, setConfirmation] = useState<
    'update' | 'rollback' | null
  >(null)
  const [trackedTaskId, setTrackedTaskId] = useState<string | null>(null)
  const notifiedTaskId = useRef<string | null>(null)
  const statusQueryKey = useMemo(
    () => ['component-update', component] as const,
    [component]
  )

  const statusQuery = useQuery({
    queryKey: statusQueryKey,
    queryFn: async () => {
      const response = await getComponentUpdateStatus(component)
      if (!response.success || !response.data) {
        throw new Error(response.message || t('Failed to check version'))
      }
      return response.data
    },
    staleTime: 5 * 60_000,
    retry: false,
  })

  useEffect(() => {
    const activeTask = statusQuery.data?.active_task
    if (activeTask && isActiveTask(activeTask)) {
      setTrackedTaskId(activeTask.task_id)
    }
  }, [statusQuery.data?.active_task])

  const taskQuery = useQuery({
    queryKey: ['system-task', trackedTaskId],
    queryFn: async () => {
      if (!trackedTaskId) {
        throw new Error(t('Failed to load update task'))
      }
      const response =
        await getSystemTask<ComponentUpdateTask>(trackedTaskId)
      if (!response.success || !response.data) {
        throw new Error(response.message || t('Failed to load update task'))
      }
      return response.data
    },
    enabled: Boolean(trackedTaskId),
    retry: false,
    refetchInterval: (query) =>
      isActiveTask(query.state.data) ? 1_500 : false,
  })

  useEffect(() => {
    const task = taskQuery.data
    if (!task || isActiveTask(task) || notifiedTaskId.current === task.task_id) {
      return
    }
    notifiedTaskId.current = task.task_id
    if (task.status === 'succeeded') {
      toast.success(
        task.payload?.action === 'rollback'
          ? t('{{component}} rollback completed', {
              component: COMPONENT_NAME[component],
            })
          : t('{{component}} update completed', {
              component: COMPONENT_NAME[component],
            })
      )
    } else {
      toast.error(task.error || t('Component update failed'))
    }
    setTrackedTaskId(null)
    void queryClient.invalidateQueries({ queryKey: statusQueryKey })
    void queryClient.invalidateQueries({
      queryKey: ['system-info', 'system-tasks'],
    })
  }, [component, queryClient, statusQueryKey, t, taskQuery.data])

  const refreshMutation = useMutation({
    mutationFn: () => getComponentUpdateStatus(component, true),
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || t('Failed to check version'))
        return
      }
      queryClient.setQueryData(statusQueryKey, response.data)
    },
    onError: (error) =>
      toast.error(
        error instanceof Error ? error.message : t('Failed to check version')
      ),
  })

  const actionMutation = useMutation({
    mutationFn: (action: 'update' | 'rollback') =>
      action === 'update'
        ? startComponentUpdate(component)
        : startComponentRollback(component),
    onSuccess: (response) => {
      if (!response.success || !response.data?.task) {
        toast.error(response.message || t('Failed to start update task'))
        return
      }
      notifiedTaskId.current = null
      setTrackedTaskId(response.data.task.task_id)
      setConfirmation(null)
      void queryClient.invalidateQueries({
        queryKey: ['system-info', 'system-tasks'],
      })
    },
    onError: (error) =>
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to start update task')
      ),
  })

  const status = statusQuery.data?.status
  const activeTask = taskQuery.data ?? statusQuery.data?.active_task
  const isWorking = isActiveTask(activeTask) || actionMutation.isPending
  const stage = activeTask?.state?.stage
  let taskLabel = t('Preparing update')
  if (stage === 'installing') {
    taskLabel = t('Installing update')
  } else if (stage === 'completed') {
    taskLabel = t('Update completed')
  }

  let statusBadge: ReactNode = null
  if (isWorking) {
    statusBadge = (
      <Badge className='bg-sky-50 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300'>
        <Loader2 className='animate-spin' />
        {taskLabel}
      </Badge>
    )
  } else if (status?.update_available) {
    statusBadge = (
      <Badge className='bg-amber-50 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'>
        {t('Update available')}
      </Badge>
    )
  } else if (status) {
    statusBadge = (
      <Badge
        variant='secondary'
        className='bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
      >
        {t('Up to date')}
      </Badge>
    )
  }

  let versionSummary: ReactNode
  if (statusQuery.isLoading) {
    versionSummary = <span>{t('Checking version...')}</span>
  } else if (status) {
    versionSummary = (
      <>
        <span>
          {t('Current version')}: {status.current_version}
        </span>
        <span>
          {t('Latest version')}: {status.latest_version}
        </span>
      </>
    )
  } else {
    const message =
      statusQuery.error instanceof Error
        ? statusQuery.error.message
        : t('Failed to check version')
    versionSummary = <span className='text-destructive'>{message}</span>
  }

  return (
    <>
      <div className='bg-muted/20 flex min-h-14 flex-wrap items-center gap-x-4 gap-y-2 border-y px-3 py-2.5'>
        <div className='flex min-w-0 flex-1 items-center gap-2.5'>
          <PackageCheck className='text-muted-foreground size-4 shrink-0' />
          <div className='min-w-0'>
            <div className='flex flex-wrap items-center gap-2 text-sm'>
              <span className='font-medium'>{COMPONENT_NAME[component]}</span>
              {statusBadge}
            </div>
            <div className='text-muted-foreground mt-0.5 flex flex-wrap gap-x-3 text-xs tabular-nums'>
              {versionSummary}
            </div>
          </div>
        </div>

        <div className='flex h-8 shrink-0 items-center gap-1.5'>
          {status?.release_url && (
            <Button
              variant='ghost'
              size='icon-sm'
              title={t('Open release')}
              render={<a href={status.release_url} target='_blank' rel='noreferrer' />}
            >
              <ExternalLink />
              <span className='sr-only'>{t('Open release')}</span>
            </Button>
          )}
          <Button
            variant='ghost'
            size='icon-sm'
            title={t('Check updates')}
            onClick={() => refreshMutation.mutate()}
            disabled={refreshMutation.isPending || isWorking}
          >
            <RefreshCw
              className={cn(refreshMutation.isPending && 'animate-spin')}
            />
            <span className='sr-only'>{t('Check updates')}</span>
          </Button>
          {isRoot && status?.rollback_available && (
            <Button
              variant='outline'
              size='icon-sm'
              title={t('Rollback {{component}}', {
                component: COMPONENT_NAME[component],
              })}
              onClick={() => setConfirmation('rollback')}
              disabled={isWorking || !status.updater_available}
            >
              <Undo2 />
              <span className='sr-only'>
                {t('Rollback {{component}}', {
                  component: COMPONENT_NAME[component],
                })}
              </span>
            </Button>
          )}
          {isRoot && status?.update_available && (
            <Button
              size='sm'
              onClick={() => setConfirmation('update')}
              disabled={isWorking || !status.updater_available}
              title={
                status.updater_available ? undefined : t('Updater unavailable')
              }
            >
              {isWorking ? (
                <Loader2 data-icon='inline-start' className='animate-spin' />
              ) : (
                <Download data-icon='inline-start' />
              )}
              {t('Update to {{version}}', {
                version: status.latest_version,
              })}
            </Button>
          )}
        </div>
      </div>

      <ConfirmDialog
        open={confirmation !== null}
        onOpenChange={(open) => {
          if (!open) setConfirmation(null)
        }}
        title={
          confirmation === 'rollback'
            ? t('Rollback {{component}}', {
                component: COMPONENT_NAME[component],
              })
            : t('Update {{component}}', {
                component: COMPONENT_NAME[component],
              })
        }
        desc={
          confirmation === 'rollback'
            ? t(
                'The service will restart on the previous installed version. Requests may be briefly interrupted.'
              )
            : t(
                'The official release will be downloaded and verified before the service restarts. Requests may be briefly interrupted.'
              )
        }
        confirmText={
          confirmation === 'rollback' ? t('Start rollback') : t('Start update')
        }
        destructive={confirmation === 'rollback'}
        isLoading={actionMutation.isPending}
        handleConfirm={() => {
          if (confirmation) actionMutation.mutate(confirmation)
        }}
      />
    </>
  )
}
