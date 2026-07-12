import { useQuery } from '@tanstack/react-query'
import {
  AlertTriangle,
  Archive,
  Clock3,
  Loader2,
  LogIn,
  Play,
  RefreshCw,
  Snowflake,
  Trash2,
  UsersRound,
} from 'lucide-react'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { archiveCPAAccount } from '@/features/operational-costs/api'
import { AssetCostInput } from '@/features/operational-costs/components/asset-cost-input'
import { cn } from '@/lib/utils'

import {
  deleteCPAAccount,
  getCPAAccounts,
  setCPAAccountStatus,
  type CPAAccount,
  type CPAUsageWindow,
} from '../api'

function formatReset(window?: CPAUsageWindow) {
  if (!window?.reset_at) return '—'
  return new Date(window.reset_at * 1000).toLocaleString()
}

function quotaRefreshMinutes(window: CPAUsageWindow | undefined, now: number) {
  if (!window?.reset_at) return null
  return Math.max(0, Math.ceil((window.reset_at * 1000 - now) / 60_000))
}

function UsageWindow(props: { label: string; value?: CPAUsageWindow }) {
  const percent = Math.max(0, Math.min(100, props.value?.used_percent ?? 0))
  return (
    <div className='min-w-[150px] space-y-1'>
      <div className='flex justify-between text-[11px]'>
        <span>{props.label}</span>
        <span className={percent >= 90 ? 'text-destructive font-semibold' : ''}>
          {percent}%
        </span>
      </div>
      <Progress value={percent} className='h-1.5' />
      <div className='text-muted-foreground text-[10px]'>
        重置：{formatReset(props.value)}
      </div>
    </div>
  )
}

type AccountPoolSectionProps = {
  accounts: CPAAccount[]
  title: string
  description: string
  emptyMessage: string
  icon: ReactNode
  now: number
  workingName: string | null
  isFetching: boolean
  onRefresh: () => void
  onToggle: (account: CPAAccount) => Promise<void>
  onArchive: (account: CPAAccount) => Promise<void>
  onRemove: (account: CPAAccount) => Promise<void>
}

function AccountPoolSection(props: AccountPoolSectionProps) {
  const { t } = useTranslation()
  const summary = useMemo(() => {
    let active = 0
    let disabled = 0
    let duplicates = 0
    for (const account of props.accounts) {
      if (account.disabled) disabled++
      else active++
      if (account.duplicate) duplicates++
    }
    return { active, disabled, duplicates }
  }, [props.accounts])

  return (
    <section className='bg-card overflow-hidden rounded-lg border'>
      <div className='flex flex-wrap items-center justify-between gap-3 border-b p-4'>
        <div>
          <div className='flex items-center gap-2 font-semibold'>
            {props.icon}
            {props.title}
          </div>
          <div className='text-muted-foreground mt-1 text-xs'>
            {props.description}
          </div>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <Badge variant='outline'>
            {t('Total')} {props.accounts.length}
          </Badge>
          <Badge variant='outline'>
            {t('Enabled')} {summary.active}
          </Badge>
          <Badge variant='outline'>
            {t('Disabled')} {summary.disabled}
          </Badge>
          {summary.duplicates > 0 && (
            <Badge variant='destructive'>
              {t('Duplicate')} {summary.duplicates}
            </Badge>
          )}
          <Button
            variant='outline'
            size='sm'
            onClick={props.onRefresh}
            disabled={props.isFetching}
          >
            <RefreshCw
              className={cn('size-4', props.isFetching && 'animate-spin')}
            />
            {t('Refresh')}
          </Button>
        </div>
      </div>

      {props.accounts.length === 0 ? (
        <div className='text-muted-foreground p-8 text-center text-sm'>
          {props.emptyMessage}
        </div>
      ) : (
        <div className='overflow-x-auto'>
          <table className='w-full min-w-[1050px] text-left text-sm'>
            <thead className='bg-muted/50 text-muted-foreground text-xs'>
              <tr>
                <th className='px-4 py-3'>账号</th>
                <th className='px-4 py-3'>{t('Cost')}</th>
                <th className='px-4 py-3'>状态</th>
                <th className='px-4 py-3'>5 小时额度</th>
                <th className='px-4 py-3'>7 天额度</th>
                <th className='px-4 py-3'>调用</th>
                <th className='px-4 py-3 text-right'>管理</th>
              </tr>
            </thead>
            <tbody className='divide-y'>
              {props.accounts.map((account) => {
                const usage = account.usage
                const rateLimit = usage?.rate_limit
                const working = props.workingName === account.name
                const refreshMinutes = quotaRefreshMinutes(
                  rateLimit?.primary_window,
                  props.now
                )
                let statusLabel = '运行中'
                if (account.disabled) {
                  statusLabel = '已冻结'
                } else if (account.unavailable) {
                  statusLabel = '不可用'
                }
                let toggleIcon = <Snowflake className='size-4' />
                if (working) {
                  toggleIcon = <Loader2 className='size-4 animate-spin' />
                } else if (account.disabled) {
                  toggleIcon = <Play className='size-4' />
                }
                return (
                  <tr
                    key={account.name}
                    className={account.disabled ? 'opacity-60' : ''}
                  >
                    <td className='px-4 py-3'>
                      <div className='flex items-center gap-2 font-medium'>
                        {account.email || account.name}
                        {account.duplicate && (
                          <Badge variant='destructive' className='text-[10px]'>
                            重复 ×{account.duplicate_count}
                          </Badge>
                        )}
                      </div>
                      <div className='text-muted-foreground mt-1 text-xs'>
                        套餐：{account.plan_type || '未知'}
                      </div>
                      <div className='text-muted-foreground mt-0.5 max-w-[320px] truncate font-mono text-[10px]'>
                        文件：{account.name}
                      </div>
                    </td>
                    <td className='px-4 py-3'>
                      <AssetCostInput
                        source_type='cpa_account'
                        source_key={account.asset_key}
                        display_name={account.email || account.name}
                        cost_date={account.cost_date}
                        note={account.cost_note}
                        costMinor={account.cost_minor || 0}
                        disabled={working}
                      />
                    </td>
                    <td className='px-4 py-3'>
                      <Badge
                        variant={account.disabled ? 'secondary' : 'outline'}
                      >
                        {statusLabel}
                      </Badge>
                      {rateLimit?.limit_reached && (
                        <div className='text-destructive mt-1 flex items-center gap-1 text-xs'>
                          <AlertTriangle className='size-3' />
                          额度已达上限
                        </div>
                      )}
                      {refreshMinutes !== null && !usage?.error && (
                        <div className='text-primary mt-1 flex items-center gap-1 text-xs font-semibold tabular-nums'>
                          <Clock3 className='size-3' />
                          {t('Quota refresh in {{minutes}} minutes', {
                            minutes: refreshMinutes,
                          })}
                        </div>
                      )}
                      {account.status_message && (
                        <div className='text-muted-foreground mt-1 max-w-[180px] truncate text-xs'>
                          {account.status_message}
                        </div>
                      )}
                    </td>
                    <td className='px-4 py-3'>
                      {usage?.error ? (
                        <span className='text-muted-foreground text-xs'>
                          查询失败
                        </span>
                      ) : (
                        <UsageWindow
                          label='已使用'
                          value={rateLimit?.primary_window}
                        />
                      )}
                    </td>
                    <td className='px-4 py-3'>
                      {usage?.error ? (
                        <span className='text-muted-foreground text-xs'>
                          查询失败
                        </span>
                      ) : (
                        <UsageWindow
                          label='已使用'
                          value={rateLimit?.secondary_window}
                        />
                      )}
                    </td>
                    <td className='px-4 py-3 tabular-nums'>
                      <span className='text-emerald-600'>
                        {account.success || 0} 成功
                      </span>
                      <span className='text-muted-foreground mx-1'>/</span>
                      <span
                        className={
                          (account.failed || 0) > 0 ? 'text-destructive' : ''
                        }
                      >
                        {account.failed || 0} 失败
                      </span>
                    </td>
                    <td className='px-4 py-3'>
                      <div className='flex justify-end gap-1'>
                        <Button
                          variant='outline'
                          size='sm'
                          disabled={working}
                          onClick={() => props.onToggle(account)}
                        >
                          {toggleIcon}
                          {account.disabled ? '解冻' : '冻结'}
                        </Button>
                        <Button
                          variant='outline'
                          size='sm'
                          disabled={working}
                          onClick={() => props.onArchive(account)}
                        >
                          <Archive className='size-4' />
                          {t('Archive')}
                        </Button>
                        <Button
                          variant='destructive'
                          size='sm'
                          disabled={working}
                          onClick={() => props.onRemove(account)}
                        >
                          <Trash2 className='size-4' />
                          删除
                        </Button>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}

export function CPAAccountsPanel() {
  const { t } = useTranslation()
  const [workingName, setWorkingName] = useState<string | null>(null)
  const [now, setNow] = useState(() => Date.now())
  const query = useQuery({
    queryKey: ['cpa-accounts'],
    queryFn: getCPAAccounts,
    staleTime: 30_000,
    refetchInterval: 120_000,
  })
  const accounts = query.data?.data?.accounts
  const pools = useMemo(() => {
    const cpaImport: CPAAccount[] = []
    const officialLogin: CPAAccount[] = []
    for (const account of accounts ?? []) {
      if (account.pool_type === 'cpa_import') cpaImport.push(account)
      else officialLogin.push(account)
    }
    return { cpaImport, officialLogin }
  }, [accounts])

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 60_000)
    return () => window.clearInterval(timer)
  }, [])

  const toggle = async (account: CPAAccount) => {
    setWorkingName(account.name)
    try {
      const result = await setCPAAccountStatus(account.name, !account.disabled)
      if (!result.success) throw new Error(result.message || '更新失败')
      toast.success(account.disabled ? '账号已解冻' : '账号已冻结')
      await query.refetch()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '更新失败')
    } finally {
      setWorkingName(null)
    }
  }

  const remove = async (account: CPAAccount) => {
    if (
      !window.confirm(`确定删除 CPA 账号 ${account.email || account.name}？`)
    ) {
      return
    }
    setWorkingName(account.name)
    try {
      const result = await deleteCPAAccount(account.name)
      if (!result.success) throw new Error(result.message || '删除失败')
      toast.success('账号已删除')
      await query.refetch()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '删除失败')
    } finally {
      setWorkingName(null)
    }
  }

  const archive = async (account: CPAAccount) => {
    const reason = window.prompt(t('Archive reason'), '')
    if (reason === null) return
    setWorkingName(account.name)
    try {
      const result = await archiveCPAAccount(account.name, reason)
      if (!result.success) {
        throw new Error(result.message || t('Failed to archive asset'))
      }
      toast.success(t('Asset archived'))
      await query.refetch()
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to archive asset')
      )
    } finally {
      setWorkingName(null)
    }
  }

  if (query.isLoading) {
    return (
      <div className='bg-card mb-4 flex items-center justify-center gap-2 rounded-lg border p-8 text-sm'>
        <Loader2 className='size-4 animate-spin' />
        正在读取 CPA 账号与额度…
      </div>
    )
  }

  if (query.data?.success === false) {
    return (
      <div className='bg-card text-destructive mb-4 rounded-lg border p-6 text-sm'>
        {query.data.message || 'CPA 账号读取失败'}
      </div>
    )
  }

  return (
    <div className='mb-4 flex flex-col gap-4'>
      <AccountPoolSection
        accounts={pools.cpaImport}
        title={t('CPA account pool')}
        description={t('Accounts imported through CPA stock upload.')}
        emptyMessage={t('No imported CPA accounts.')}
        icon={<UsersRound className='size-4' />}
        now={now}
        workingName={workingName}
        isFetching={query.isFetching}
        onRefresh={() => query.refetch()}
        onToggle={toggle}
        onArchive={archive}
        onRemove={remove}
      />
      <AccountPoolSection
        accounts={pools.officialLogin}
        title={t('Official login account pool')}
        description={t('Accounts added through server-side official login.')}
        emptyMessage={t('No official login accounts.')}
        icon={<LogIn className='size-4' />}
        now={now}
        workingName={workingName}
        isFetching={query.isFetching}
        onRefresh={() => query.refetch()}
        onToggle={toggle}
        onArchive={archive}
        onRemove={remove}
      />
    </div>
  )
}
