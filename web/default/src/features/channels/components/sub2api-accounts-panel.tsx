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
import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  AlertTriangle,
  Archive,
  ListChecks,
  Loader2,
  Play,
  RefreshCw,
  Snowflake,
  Trash2,
  Zap,
} from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { PaginationControls } from '@/components/pagination-controls'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { AssetCostInput } from '@/features/operational-costs/components/asset-cost-input'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { cn } from '@/lib/utils'

import {
  archiveSub2APIAccount,
  deleteSub2APIAccount,
  getSub2APIAccounts,
  getSub2APIAccountSelection,
  setSub2APIAccountSchedulable,
  type AccountInspectionOperation,
  type Sub2APIAccount,
} from '../api'
import {
  ACCOUNT_POOL_FILTER_OPTIONS,
  type AccountPoolFilter,
} from '../lib/account-pool-filters'
import { AccountSelectionToolbar } from './account-selection-toolbar'
import { AccountUsageWindow } from './account-usage-window'
import { ComponentVersionBar } from './component-version-bar'
import { AccountInspectorDialog } from './dialogs/account-inspector-dialog'
import { Sub2APIBatchManageDialog } from './dialogs/sub2api-batch-manage-dialog'

export function Sub2APIAccountsPanel() {
  const { t } = useTranslation()
  const [pageIndex, setPageIndex] = useState(0)
  const [pageSize, setPageSize] = useState(20)
  const [workingID, setWorkingID] = useState<number | null>(null)
  const [selectedIDs, setSelectedIDs] = useState<Set<number>>(new Set())
  const [selectingAll, setSelectingAll] = useState(false)
  const [batchOpen, setBatchOpen] = useState(false)
  const [accountFilter, setAccountFilter] = useState<AccountPoolFilter>('all')
  const [inspection, setInspection] = useState<{
    id: number
    operation: AccountInspectionOperation
  } | null>(null)
  const query = useQuery({
    queryKey: ['sub2api-accounts', pageIndex, pageSize, accountFilter],
    queryFn: () =>
      getSub2APIAccounts({
        page: pageIndex + 1,
        pageSize,
        accountFilter,
      }),
    staleTime: 30_000,
    refetchInterval: 300_000,
  })
  const data = query.data?.data
  const accounts = data?.items ?? []
  const visibleAccounts = accounts
  const loadFailed = query.data?.success === false || query.isError
  const pageIDs = visibleAccounts.map((account) => account.id)
  const selectedPageCount = pageIDs.reduce(
    (count, id) => count + (selectedIDs.has(id) ? 1 : 0),
    0
  )
  const allPageSelected =
    pageIDs.length > 0 && selectedPageCount === pageIDs.length
  const somePageSelected = selectedPageCount > 0 && !allPageSelected

  useEffect(() => {
    if (pageIndex > 0 && (data?.total ?? 0) <= pageIndex * pageSize) {
      setPageIndex(pageIndex - 1)
    }
  }, [data?.total, pageIndex, pageSize])

  const toggleSelection = useCallback((id: number, selected: boolean) => {
    setSelectedIDs((current) => {
      const next = new Set(current)
      if (selected) next.add(id)
      else next.delete(id)
      return next
    })
  }, [])

  const togglePageSelection = (selected: boolean) => {
    setSelectedIDs((current) => {
      const next = new Set(current)
      for (const id of pageIDs) {
        if (selected) next.add(id)
        else next.delete(id)
      }
      return next
    })
  }

  const selectAll = async () => {
    setSelectingAll(true)
    try {
      const result = await getSub2APIAccountSelection(accountFilter)
      if (!result.success || !result.data) {
        throw new Error(result.message || t('Failed to select all accounts'))
      }
      setSelectedIDs(new Set(result.data.ids))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to select all accounts')
      )
    } finally {
      setSelectingAll(false)
    }
  }

  const toggle = async (account: Sub2APIAccount) => {
    setWorkingID(account.id)
    try {
      const result = await setSub2APIAccountSchedulable(
        account.id,
        !account.schedulable
      )
      if (!result.success) throw new Error(result.message)
      toast.success(t('Account status updated'))
      await query.refetch()
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to update account')
      )
    } finally {
      setWorkingID(null)
    }
  }

  const remove = async (account: Sub2APIAccount) => {
    if (
      !window.confirm(
        t('Delete Sub2API account {{name}}?', {
          name: account.email || account.name,
        })
      )
    ) {
      return
    }
    setWorkingID(account.id)
    try {
      const result = await deleteSub2APIAccount(account.id)
      if (!result.success) throw new Error(result.message)
      toast.success(t('Account deleted'))
      toggleSelection(account.id, false)
      await query.refetch()
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to delete account')
      )
    } finally {
      setWorkingID(null)
    }
  }

  const archive = async (account: Sub2APIAccount) => {
    if (
      !window.confirm(
        t('Archive account {{name}}?', { name: account.email || account.name })
      )
    ) {
      return
    }
    setWorkingID(account.id)
    try {
      const result = await archiveSub2APIAccount(account.id)
      if (!result.success) throw new Error(result.message)
      toast.success(t('Account archived'))
      toggleSelection(account.id, false)
      await query.refetch()
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to archive account')
      )
    } finally {
      setWorkingID(null)
    }
  }

  return (
    <section className='flex flex-col gap-3'>
      <ComponentVersionBar component='sub2api' />
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <Badge variant='outline'>
          {accountFilter === 'all' ? t('Total') : t('Matched')}{' '}
          {data?.total ?? 0}
        </Badge>
        <div className='flex flex-wrap items-center gap-2'>
          <Select<AccountPoolFilter>
            items={ACCOUNT_POOL_FILTER_OPTIONS.map((option) => ({
              value: option.value,
              label: t(option.label),
            }))}
            value={accountFilter}
            onValueChange={(value) => {
              if (value !== null) {
                setAccountFilter(value)
                setPageIndex(0)
                setSelectedIDs(new Set())
              }
            }}
          >
            <SelectTrigger
              size='sm'
              className='min-w-[150px]'
              aria-label={t('Filter')}
              title={t('Filter')}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {ACCOUNT_POOL_FILTER_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {t(option.label)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setBatchOpen(true)}
            disabled={selectedIDs.size === 0}
          >
            <ListChecks data-icon='inline-start' />
            {t('Batch manage')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            onClick={() => query.refetch()}
            disabled={query.isFetching}
          >
            <RefreshCw
              data-icon='inline-start'
              className={cn(query.isFetching && 'animate-spin')}
            />
            {t('Refresh')}
          </Button>
        </div>
      </div>

      <div className='overflow-hidden rounded-lg border'>
        <AccountSelectionToolbar
          selectedCount={selectedIDs.size}
          totalCount={data?.total ?? 0}
          selectingAll={selectingAll}
          selectAllLabel={
            accountFilter === 'all' ? undefined : t('Select all (filtered)')
          }
          onSelectAll={selectAll}
          onClear={() => setSelectedIDs(new Set())}
          onManage={() => setBatchOpen(true)}
        />

        {query.isLoading && (
          <div className='text-muted-foreground flex items-center justify-center gap-2 p-8 text-sm'>
            <Loader2 className='animate-spin' />
            {t('Loading accounts...')}
          </div>
        )}
        {!query.isLoading && loadFailed && (
          <div className='text-destructive p-5 text-sm'>
            {query.data?.message || t('Failed to load Sub2API accounts')}
          </div>
        )}
        {!query.isLoading &&
          !loadFailed &&
          accounts.length === 0 &&
          accountFilter === 'all' && (
            <div className='text-muted-foreground p-8 text-center text-sm'>
              {t('No Sub2API accounts.')}
            </div>
          )}
        {!query.isLoading &&
          !loadFailed &&
          accounts.length === 0 &&
          accountFilter !== 'all' &&
          visibleAccounts.length === 0 && (
            <div className='text-muted-foreground p-8 text-center text-sm'>
              {t('No records found. Try adjusting your filters.')}
            </div>
          )}
        {!query.isLoading && !loadFailed && visibleAccounts.length > 0 && (
          <div className='overflow-x-auto'>
            <table className='w-full min-w-[1210px] text-left text-sm'>
              <thead className='bg-muted/50 text-muted-foreground text-xs'>
                <tr>
                  <th className='w-10 px-4 py-3'>
                    <Checkbox
                      checked={allPageSelected}
                      indeterminate={somePageSelected}
                      onCheckedChange={(value) =>
                        togglePageSelection(Boolean(value))
                      }
                      aria-label={t('Select all accounts on this page')}
                    />
                  </th>
                  <th className='px-4 py-3'>{t('Account')}</th>
                  <th className='px-4 py-3'>{t('Cost')}</th>
                  <th className='px-4 py-3'>{t('Cumulative output')}</th>
                  <th className='px-4 py-3'>{t('Status')}</th>
                  <th className='px-4 py-3'>{t('5-Hour Window')}</th>
                  <th className='px-4 py-3'>{t('7 Days')}</th>
                  <th className='px-4 py-3'>{t('API Requests')}</th>
                  <th className='px-4 py-3 text-right'>{t('Manage')}</th>
                </tr>
              </thead>
              <tbody className='divide-y'>
                {visibleAccounts.map((account) => {
                  const working = workingID === account.id
                  const rateLimit = account.usage?.rate_limit
                  let toggleIcon = <Snowflake data-icon='inline-start' />
                  if (working) {
                    toggleIcon = (
                      <Loader2
                        data-icon='inline-start'
                        className='animate-spin'
                      />
                    )
                  } else if (!account.schedulable) {
                    toggleIcon = <Play data-icon='inline-start' />
                  }
                  return (
                    <tr
                      key={account.id}
                      className={!account.schedulable ? 'opacity-60' : ''}
                    >
                      <td className='px-4 py-3'>
                        <Checkbox
                          checked={selectedIDs.has(account.id)}
                          onCheckedChange={(value) =>
                            toggleSelection(account.id, Boolean(value))
                          }
                          aria-label={t('Select account {{name}}', {
                            name: account.email || account.name,
                          })}
                        />
                      </td>
                      <td className='px-4 py-3'>
                        <div className='font-medium'>
                          {account.email || account.name}
                        </div>
                        <div className='text-muted-foreground mt-1 text-xs'>
                          {t('Plan')}: {account.plan_type || t('Unknown')}
                        </div>
                        <div className='text-muted-foreground mt-0.5 text-[10px]'>
                          {account.platform} / {account.type}
                        </div>
                      </td>
                      <td className='px-4 py-3'>
                        <AssetCostInput
                          source_type='sub2api_account'
                          source_key={account.asset_key}
                          source_id={account.id}
                          source_ref={String(account.id)}
                          display_name={account.email || account.name}
                          cost_date={account.cost_date}
                          note={account.cost_note}
                          costMinor={account.cost_minor}
                          disabled={working}
                        />
                      </td>
                      <td className='px-4 py-3 font-medium tabular-nums'>
                        {formatBillingCurrencyFromUSD(
                          account.cumulative_output_usd ?? 0
                        )}
                      </td>
                      <td className='px-4 py-3'>
                        <Badge
                          variant={
                            account.schedulable ? 'outline' : 'secondary'
                          }
                        >
                          {account.schedulable ? t('Schedulable') : t('Paused')}
                        </Badge>
                        {rateLimit?.limit_reached && (
                          <div className='text-destructive mt-1 flex items-center gap-1 text-xs'>
                            <AlertTriangle className='size-3' aria-hidden />
                            {t('Insufficient quota')}
                          </div>
                        )}
                        {(account.error_message || account.usage?.error) && (
                          <div className='text-muted-foreground mt-1 max-w-48 truncate text-xs'>
                            {account.error_message || account.usage?.error}
                          </div>
                        )}
                      </td>
                      <td className='px-4 py-3'>
                        <AccountUsageWindow value={rateLimit?.primary_window} />
                      </td>
                      <td className='px-4 py-3'>
                        <AccountUsageWindow
                          value={rateLimit?.secondary_window}
                        />
                      </td>
                      <td className='px-4 py-3 tabular-nums'>
                        <div>
                          {account.today_stats?.requests ?? 0} {t('Requests')}
                        </div>
                        <div className='text-muted-foreground mt-1 text-xs'>
                          {(account.today_stats?.tokens ?? 0).toLocaleString()}{' '}
                          {t('Tokens')}
                        </div>
                      </td>
                      <td className='px-4 py-3'>
                        <div className='flex justify-end gap-1'>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            aria-label={t('Verify account')}
                            title={t('Verify account')}
                            onClick={() =>
                              setInspection({
                                id: account.id,
                                operation: 'verify',
                              })
                            }
                          >
                            <Activity />
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            aria-label={t('Ping account')}
                            title={t('Ping account')}
                            onClick={() =>
                              setInspection({
                                id: account.id,
                                operation: 'ping',
                              })
                            }
                          >
                            <Zap />
                          </Button>
                          <Button
                            variant='outline'
                            size='sm'
                            disabled={working}
                            onClick={() => toggle(account)}
                          >
                            {toggleIcon}
                            {account.schedulable ? t('Pause') : t('Enable')}
                          </Button>
                          <Button
                            variant='outline'
                            size='sm'
                            disabled={working}
                            onClick={() => archive(account)}
                          >
                            <Archive data-icon='inline-start' />
                            {t('Archive')}
                          </Button>
                          <Button
                            variant='destructive'
                            size='sm'
                            disabled={working}
                            onClick={() => remove(account)}
                          >
                            <Trash2 data-icon='inline-start' />
                            {t('Delete')}
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
        {!query.isLoading && !loadFailed && accounts.length > 0 && (
          <div className='border-t p-4'>
            <PaginationControls
              pageIndex={pageIndex}
              pageSize={pageSize}
              totalCount={data?.total ?? 0}
              pageCount={data?.pages ?? 1}
              compact
              onPageIndexChange={setPageIndex}
              onPageSizeChange={(nextPageSize) => {
                setPageSize(nextPageSize)
                setPageIndex(0)
              }}
            />
          </div>
        )}
      </div>

      <Sub2APIBatchManageDialog
        open={batchOpen}
        onOpenChange={setBatchOpen}
        accountIDs={[...selectedIDs]}
        onComplete={(failedIDs) => setSelectedIDs(new Set(failedIDs))}
      />
      <AccountInspectorDialog
        open={inspection !== null}
        onOpenChange={(open) => !open && setInspection(null)}
        initialProvider='sub2api'
        initialIdentifier={inspection ? String(inspection.id) : undefined}
        initialOperation={inspection?.operation}
      />
    </section>
  )
}
