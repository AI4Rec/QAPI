import { useQuery } from '@tanstack/react-query'
import {
  AlertTriangle,
  Loader2,
  Play,
  RefreshCw,
  Snowflake,
  Trash2,
  UsersRound,
  Archive,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { archiveCPAAccount } from '@/features/operational-costs/api'
import { AssetCostInput } from '@/features/operational-costs/components/asset-cost-input'

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

function UsageWindow({
  label,
  value,
}: {
  label: string
  value?: CPAUsageWindow
}) {
  const percent = Math.max(0, Math.min(100, value?.used_percent ?? 0))
  return (
    <div className='min-w-[150px] space-y-1'>
      <div className='flex justify-between text-[11px]'>
        <span>{label}</span>
        <span className={percent >= 90 ? 'text-destructive font-semibold' : ''}>
          {percent}%
        </span>
      </div>
      <Progress value={percent} className='h-1.5' />
      <div className='text-muted-foreground text-[10px]'>
        重置：{formatReset(value)}
      </div>
    </div>
  )
}

export function CPAAccountsPanel() {
  const { t } = useTranslation()
  const [workingName, setWorkingName] = useState<string | null>(null)
  const query = useQuery({
    queryKey: ['cpa-accounts'],
    queryFn: getCPAAccounts,
    staleTime: 30_000,
    refetchInterval: 120_000,
  })
  const accounts = query.data?.data?.accounts || []
  const summary = query.data?.data?.summary

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
    if (reason === null) {
      return
    }
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

  return (
    <div className='bg-card mb-4 overflow-hidden rounded-lg border'>
      <div className='flex flex-wrap items-center justify-between gap-3 border-b p-4'>
        <div>
          <div className='flex items-center gap-2 font-semibold'>
            <UsersRound className='size-4' />
            CPA 账号池
          </div>
          <div className='text-muted-foreground mt-1 text-xs'>
            直接监控 CPA 内的 Codex 账号、额度窗口和运行状态。
          </div>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <Badge variant='outline'>认证条目 {summary?.total ?? '—'}</Badge>
          <Badge variant='outline'>唯一账号 {summary?.unique ?? '—'}</Badge>
          <Badge variant='outline'>启用 {summary?.active ?? '—'}</Badge>
          <Badge variant='outline'>冻结 {summary?.disabled ?? '—'}</Badge>
          {(summary?.duplicates || 0) > 0 && (
            <Badge variant='destructive'>重复 {summary?.duplicates}</Badge>
          )}
          <Button
            variant='outline'
            size='sm'
            onClick={() => query.refetch()}
            disabled={query.isFetching}
          >
            <RefreshCw
              className={query.isFetching ? 'size-4 animate-spin' : 'size-4'}
            />
            刷新
          </Button>
        </div>
      </div>

      {query.isLoading && (
        <div className='flex items-center justify-center gap-2 p-8 text-sm'>
          <Loader2 className='size-4 animate-spin' />
          正在读取 CPA 账号与额度…
        </div>
      )}
      {!query.isLoading && query.data?.success === false && (
        <div className='text-destructive p-6 text-sm'>
          {query.data.message || 'CPA 账号读取失败'}
        </div>
      )}
      {!query.isLoading &&
        query.data?.success !== false &&
        accounts.length === 0 && (
          <div className='text-muted-foreground p-8 text-center text-sm'>
            暂无 CPA 账号，请点击页面上方“CPA 上货”。
          </div>
        )}
      {!query.isLoading &&
        query.data?.success !== false &&
        accounts.length > 0 && (
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
                {accounts.map((account) => {
                  const usage = account.usage
                  const rateLimit = usage?.rate_limit
                  const working = workingName === account.name
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
                            <Badge
                              variant='destructive'
                              className='text-[10px]'
                            >
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
                            onClick={() => toggle(account)}
                          >
                            {toggleIcon}
                            {account.disabled ? '解冻' : '冻结'}
                          </Button>
                          <Button
                            variant='outline'
                            size='sm'
                            disabled={working}
                            onClick={() => archive(account)}
                          >
                            <Archive className='size-4' />
                            {t('Archive')}
                          </Button>
                          <Button
                            variant='destructive'
                            size='sm'
                            disabled={working}
                            onClick={() => remove(account)}
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
    </div>
  )
}
