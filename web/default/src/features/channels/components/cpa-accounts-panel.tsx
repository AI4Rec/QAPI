import { useQuery } from '@tanstack/react-query'
import {
  AlertTriangle,
  Activity,
  Archive,
  ArrowDown,
  ArrowUp,
  Clock3,
  ListChecks,
  Loader2,
  LogIn,
  Play,
  RefreshCw,
  Snowflake,
  TimerReset,
  Trash2,
  UsersRound,
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
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { archiveCPAAccount } from '@/features/operational-costs/api'
import { AssetCostInput } from '@/features/operational-costs/components/asset-cost-input'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { cn } from '@/lib/utils'

import {
  deleteCPAAccount,
  getCPAAccounts,
  getCPAAccountSelection,
  setCPAAccountStatus,
  type CPAAccount,
  type CPAAccountPoolType,
  type CPAAccountSortBy,
  type CPAAccountSortOrder,
  type CPAUsageWindow,
  type AccountInspectionOperation,
} from '../api'
import { AccountSelectionToolbar } from './account-selection-toolbar'
import { AccountUsageWindow } from './account-usage-window'
import { ComponentVersionBar } from './component-version-bar'
import { AccountInspectorDialog } from './dialogs/account-inspector-dialog'
import { CPABatchManageDialog } from './dialogs/cpa-batch-manage-dialog'

const CPA_ACCOUNT_SORT_OPTIONS: Array<{
  value: CPAAccountSortBy
  label: string
}> = [
  { value: 'name', label: 'Sort by account' },
  { value: 'cost', label: 'Sort by cost' },
  { value: 'status', label: 'Sort by status' },
  { value: 'success', label: 'Sort by successful calls' },
  { value: 'failed', label: 'Sort by failed calls' },
  { value: 'created_at', label: 'Sort by created time' },
  { value: 'updated_at', label: 'Sort by updated time' },
]

const CPA_ACCOUNT_DEFAULT_SORT_ORDER: Record<
  CPAAccountSortBy,
  CPAAccountSortOrder
> = {
  name: 'asc',
  cost: 'desc',
  status: 'asc',
  success: 'desc',
  failed: 'desc',
  created_at: 'desc',
  updated_at: 'desc',
}

function quotaRefreshMinutes(window: CPAUsageWindow | undefined, now: number) {
  if (!window?.reset_at) return null
  return Math.max(0, Math.ceil((window.reset_at * 1000 - now) / 60_000))
}

type AccountPoolSectionProps = {
  accounts: CPAAccount[]
  emptyMessage: string
  now: number
  workingName: string | null
  isLoading: boolean
  isFetching: boolean
  summary: {
    total: number
    unique: number
    active: number
    disabled: number
    duplicates: number
  }
  pageIndex: number
  pageSize: number
  sortBy: CPAAccountSortBy
  sortOrder: CPAAccountSortOrder
  selectedNames: Set<string>
  selectingAll: boolean
  onPageIndexChange: (pageIndex: number) => void
  onPageSizeChange: (pageSize: number) => void
  onSortByChange: (sortBy: CPAAccountSortBy) => void
  onSortOrderChange: (sortOrder: CPAAccountSortOrder) => void
  onToggleSelection: (name: string, selected: boolean) => void
  onTogglePageSelection: (names: string[], selected: boolean) => void
  onSelectAll: () => void
  onClearSelection: () => void
  onManageSelection: () => void
  onRefresh: () => void
  onToggle: (account: CPAAccount) => Promise<void>
  onInspect: (
    account: CPAAccount,
    operation: AccountInspectionOperation
  ) => void
  onArchive: (account: CPAAccount) => Promise<void>
  onRemove: (account: CPAAccount) => Promise<void>
}

function useCPAAccountPool(poolType: CPAAccountPoolType, enabled: boolean) {
  const [pageIndex, setPageIndex] = useState(0)
  const [pageSize, setPageSize] = useState(20)
  const [sort, setSort] = useState<{
    by: CPAAccountSortBy
    order: CPAAccountSortOrder
  }>({ by: 'name', order: 'asc' })
  const query = useQuery({
    queryKey: [
      'cpa-accounts',
      poolType,
      pageIndex,
      pageSize,
      sort.by,
      sort.order,
    ],
    queryFn: () =>
      getCPAAccounts({
        poolType,
        page: pageIndex + 1,
        pageSize,
        sortBy: sort.by,
        sortOrder: sort.order,
      }),
    enabled,
    staleTime: 30_000,
    refetchInterval: 300_000,
  })

  useEffect(() => {
    const total = query.data?.data?.total ?? 0
    if (pageIndex > 0 && total <= pageIndex * pageSize) {
      setPageIndex(pageIndex - 1)
    }
  }, [pageIndex, pageSize, query.data?.data?.total])

  const setSortBy = useCallback((sortBy: CPAAccountSortBy) => {
    setSort({ by: sortBy, order: CPA_ACCOUNT_DEFAULT_SORT_ORDER[sortBy] })
    setPageIndex(0)
  }, [])

  const setSortOrder = useCallback((sortOrder: CPAAccountSortOrder) => {
    setSort((current) => ({ ...current, order: sortOrder }))
    setPageIndex(0)
  }, [])

  return {
    pageIndex,
    pageSize,
    query,
    setPageIndex,
    setPageSize,
    setSortBy,
    setSortOrder,
    sortBy: sort.by,
    sortOrder: sort.order,
  }
}

function useCPAAccountSelection(poolType: CPAAccountPoolType) {
  const { t } = useTranslation()
  const [selectedNames, setSelectedNames] = useState<Set<string>>(new Set())
  const [selectingAll, setSelectingAll] = useState(false)

  const toggleSelection = useCallback((name: string, selected: boolean) => {
    setSelectedNames((current) => {
      const next = new Set(current)
      if (selected) next.add(name)
      else next.delete(name)
      return next
    })
  }, [])

  const togglePageSelection = useCallback(
    (names: string[], selected: boolean) => {
      setSelectedNames((current) => {
        const next = new Set(current)
        for (const name of names) {
          if (selected) next.add(name)
          else next.delete(name)
        }
        return next
      })
    },
    []
  )

  const selectAll = useCallback(async () => {
    setSelectingAll(true)
    try {
      const result = await getCPAAccountSelection(poolType)
      if (!result.success || !result.data) {
        throw new Error(result.message || t('Failed to select all accounts'))
      }
      setSelectedNames(new Set(result.data.names))
    } finally {
      setSelectingAll(false)
    }
  }, [poolType, t])

  const clearSelection = useCallback(() => setSelectedNames(new Set()), [])
  const replaceSelection = useCallback(
    (names: string[]) => setSelectedNames(new Set(names)),
    []
  )

  return {
    clearSelection,
    replaceSelection,
    selectAll,
    selectedNames,
    selectingAll,
    togglePageSelection,
    toggleSelection,
  }
}

function AccountPoolSection(props: AccountPoolSectionProps) {
  const { t } = useTranslation()
  const pageNames = props.accounts.map((account) => account.name)
  const selectedPageCount = pageNames.reduce(
    (count, name) => count + (props.selectedNames.has(name) ? 1 : 0),
    0
  )
  const allPageSelected =
    pageNames.length > 0 && selectedPageCount === pageNames.length
  const somePageSelected = selectedPageCount > 0 && !allPageSelected

  return (
    <section className='flex flex-col gap-3'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex flex-wrap items-center gap-2'>
          <Badge variant='outline'>
            {t('Total')} {props.summary.total}
          </Badge>
          <Badge variant='outline'>
            {t('Enabled')} {props.summary.active}
          </Badge>
          <Badge variant='secondary'>
            {t('Disabled')} {props.summary.disabled}
          </Badge>
          {props.summary.duplicates > 0 && (
            <Badge variant='destructive'>
              {t('Duplicate')} {props.summary.duplicates}
            </Badge>
          )}
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <Select<CPAAccountSortBy>
            items={CPA_ACCOUNT_SORT_OPTIONS.map((option) => ({
              value: option.value,
              label: t(option.label),
            }))}
            value={props.sortBy}
            onValueChange={(value) => {
              if (value !== null) props.onSortByChange(value)
            }}
          >
            <SelectTrigger size='sm' className='min-w-[160px]'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {CPA_ACCOUNT_SORT_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {t(option.label)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <Button
            variant='outline'
            size='icon-sm'
            aria-label={
              props.sortOrder === 'asc' ? t('Ascending') : t('Descending')
            }
            title={props.sortOrder === 'asc' ? t('Ascending') : t('Descending')}
            onClick={() =>
              props.onSortOrderChange(
                props.sortOrder === 'asc' ? 'desc' : 'asc'
              )
            }
          >
            {props.sortOrder === 'asc' ? <ArrowUp /> : <ArrowDown />}
          </Button>
          <Button
            variant='outline'
            size='sm'
            onClick={props.onManageSelection}
            disabled={props.selectedNames.size === 0}
          >
            <ListChecks data-icon='inline-start' />
            {t('Batch manage')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            onClick={props.onRefresh}
            disabled={props.isFetching}
          >
            <RefreshCw
              data-icon='inline-start'
              className={cn(props.isFetching && 'animate-spin')}
            />
            {t('Refresh')}
          </Button>
        </div>
      </div>

      <div className='overflow-hidden rounded-lg border'>
        <AccountSelectionToolbar
          selectedCount={props.selectedNames.size}
          totalCount={props.summary.total}
          selectingAll={props.selectingAll}
          onSelectAll={props.onSelectAll}
          onClear={props.onClearSelection}
          onManage={props.onManageSelection}
        />

        {props.isLoading && (
          <div className='text-muted-foreground flex items-center justify-center gap-2 p-8 text-sm'>
            <Loader2 className='animate-spin' />
            {t('Loading accounts...')}
          </div>
        )}
        {!props.isLoading && props.accounts.length === 0 && (
          <div className='text-muted-foreground p-8 text-center text-sm'>
            {props.emptyMessage}
          </div>
        )}
        {!props.isLoading && props.accounts.length > 0 && (
          <>
            <div className='overflow-x-auto'>
              <table className='w-full min-w-[1210px] text-left text-sm'>
                <thead className='bg-muted/50 text-muted-foreground text-xs'>
                  <tr>
                    <th className='w-10 px-4 py-3'>
                      <Checkbox
                        checked={allPageSelected}
                        indeterminate={somePageSelected}
                        onCheckedChange={(value) =>
                          props.onTogglePageSelection(pageNames, !!value)
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
                  {props.accounts.map((account) => {
                    const usage = account.usage
                    const rateLimit = usage?.rate_limit
                    const working = props.workingName === account.name
                    const refreshMinutes = quotaRefreshMinutes(
                      rateLimit?.primary_window,
                      props.now
                    )
                    let statusLabel = t('Running')
                    if (account.disabled) {
                      statusLabel = t('Paused')
                    } else if (account.unavailable) {
                      statusLabel = t('Unavailable')
                    }
                    let toggleIcon = <Snowflake data-icon='inline-start' />
                    if (working) {
                      toggleIcon = (
                        <Loader2
                          data-icon='inline-start'
                          className='animate-spin'
                        />
                      )
                    } else if (account.disabled) {
                      toggleIcon = <Play data-icon='inline-start' />
                    }
                    return (
                      <tr
                        key={account.name}
                        className={account.disabled ? 'opacity-60' : ''}
                      >
                        <td className='px-4 py-3'>
                          <Checkbox
                            checked={props.selectedNames.has(account.name)}
                            onCheckedChange={(value) =>
                              props.onToggleSelection(account.name, !!value)
                            }
                            aria-label={t('Select account {{name}}', {
                              name: account.email || account.name,
                            })}
                          />
                        </td>
                        <td className='px-4 py-3'>
                          <div className='flex items-center gap-2 font-medium'>
                            {account.email || account.name}
                            {account.duplicate && (
                              <Badge
                                variant='destructive'
                                className='text-[10px]'
                              >
                                {t('Duplicate')} ×{account.duplicate_count}
                              </Badge>
                            )}
                          </div>
                          <div className='text-muted-foreground mt-1 text-xs'>
                            {t('Plan')}: {account.plan_type || t('Unknown')}
                          </div>
                          <div className='text-muted-foreground mt-0.5 max-w-[320px] truncate font-mono text-[10px]'>
                            {t('File')}: {account.name}
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
                        <td className='px-4 py-3 font-medium tabular-nums'>
                          {formatBillingCurrencyFromUSD(
                            account.cumulative_output_usd ?? 0
                          )}
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
                              {t('Insufficient quota')}
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
                              {t('Failed to load')}
                            </span>
                          ) : (
                            <AccountUsageWindow
                              value={rateLimit?.primary_window}
                            />
                          )}
                        </td>
                        <td className='px-4 py-3'>
                          {usage?.error ? (
                            <span className='text-muted-foreground text-xs'>
                              {t('Failed to load')}
                            </span>
                          ) : (
                            <AccountUsageWindow
                              value={rateLimit?.secondary_window}
                            />
                          )}
                        </td>
                        <td className='px-4 py-3'>
                          <div className='flex flex-wrap gap-1.5 tabular-nums'>
                            <Badge variant='outline'>
                              {account.success || 0} {t('Success')}
                            </Badge>
                            <Badge
                              variant={
                                (account.failed || 0) > 0
                                  ? 'destructive'
                                  : 'secondary'
                              }
                            >
                              {account.failed || 0} {t('Failed')}
                            </Badge>
                          </div>
                        </td>
                        <td className='px-4 py-3'>
                          <div className='flex justify-end gap-1'>
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              aria-label={t('Verify account')}
                              title={t('Verify account')}
                              onClick={() => props.onInspect(account, 'verify')}
                            >
                              <Activity />
                            </Button>
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              aria-label={t('Ping account')}
                              title={t('Ping account')}
                              onClick={() => props.onInspect(account, 'ping')}
                            >
                              <Zap />
                            </Button>
                            <Button
                              variant='outline'
                              size='sm'
                              disabled={working}
                              onClick={() => props.onToggle(account)}
                            >
                              {toggleIcon}
                              {account.disabled ? t('Enable') : t('Pause')}
                            </Button>
                            <Button
                              variant='outline'
                              size='sm'
                              disabled={working}
                              onClick={() => props.onArchive(account)}
                            >
                              <Archive data-icon='inline-start' />
                              {t('Archive')}
                            </Button>
                            <Button
                              variant='destructive'
                              size='sm'
                              disabled={working}
                              onClick={() => props.onRemove(account)}
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
            <div className='border-t p-4'>
              <PaginationControls
                pageIndex={props.pageIndex}
                pageSize={props.pageSize}
                totalCount={props.summary.total}
                pageCount={Math.ceil(props.summary.total / props.pageSize)}
                onPageIndexChange={props.onPageIndexChange}
                onPageSizeChange={props.onPageSizeChange}
              />
            </div>
          </>
        )}
      </div>
    </section>
  )
}

export function CPAAccountsPanel() {
  const { t } = useTranslation()
  const [workingName, setWorkingName] = useState<string | null>(null)
  const [now, setNow] = useState(() => Date.now())
  const [activePoolType, setActivePoolType] =
    useState<CPAAccountPoolType>('cpa_import')
  const importedPool = useCPAAccountPool(
    'cpa_import',
    activePoolType === 'cpa_import'
  )
  const temporaryPool = useCPAAccountPool(
    'temporary',
    activePoolType === 'temporary'
  )
  const officialPool = useCPAAccountPool(
    'official_login',
    activePoolType === 'official_login'
  )
  const importedSelection = useCPAAccountSelection('cpa_import')
  const temporarySelection = useCPAAccountSelection('temporary')
  const officialSelection = useCPAAccountSelection('official_login')
  const [batchPoolType, setBatchPoolType] = useState<CPAAccountPoolType | null>(
    null
  )
  const [inspection, setInspection] = useState<{
    name: string
    operation: AccountInspectionOperation
  } | null>(null)
  const emptySummary = {
    total: 0,
    unique: 0,
    active: 0,
    disabled: 0,
    duplicates: 0,
  }

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 60_000)
    return () => window.clearInterval(timer)
  }, [])

  let activePool = importedPool
  let activeSelection = importedSelection
  let emptyMessage = t('No imported CPA accounts.')
  if (activePoolType === 'temporary') {
    activePool = temporaryPool
    activeSelection = temporarySelection
    emptyMessage = t('No temporary CPA accounts.')
  } else if (activePoolType === 'official_login') {
    activePool = officialPool
    activeSelection = officialSelection
    emptyMessage = t('No official login accounts.')
  }

  const removeFromSelections = (name: string) => {
    importedSelection.toggleSelection(name, false)
    temporarySelection.toggleSelection(name, false)
    officialSelection.toggleSelection(name, false)
  }

  const selectAllAccounts = async (
    selection: ReturnType<typeof useCPAAccountSelection>
  ) => {
    try {
      await selection.selectAll()
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to select all accounts')
      )
    }
  }

  const toggle = async (account: CPAAccount) => {
    setWorkingName(account.name)
    try {
      const result = await setCPAAccountStatus(account.name, !account.disabled)
      if (!result.success) {
        throw new Error(result.message || t('Failed to update account'))
      }
      toast.success(t('Account status updated'))
      await activePool.query.refetch()
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to update account')
      )
    } finally {
      setWorkingName(null)
    }
  }

  const remove = async (account: CPAAccount) => {
    if (
      !window.confirm(
        t('Delete CPA account {{name}}?', {
          name: account.email || account.name,
        })
      )
    ) {
      return
    }
    setWorkingName(account.name)
    try {
      const result = await deleteCPAAccount(account.name)
      if (!result.success) {
        throw new Error(result.message || t('Failed to delete account'))
      }
      toast.success(t('Account deleted'))
      removeFromSelections(account.name)
      await activePool.query.refetch()
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to delete account')
      )
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
      removeFromSelections(account.name)
      await activePool.query.refetch()
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to archive asset')
      )
    } finally {
      setWorkingName(null)
    }
  }

  let batchNames: string[] = []
  if (batchPoolType === 'cpa_import') {
    batchNames = [...importedSelection.selectedNames]
  } else if (batchPoolType === 'temporary') {
    batchNames = [...temporarySelection.selectedNames]
  } else if (batchPoolType === 'official_login') {
    batchNames = [...officialSelection.selectedNames]
  }

  const loadFailed =
    activePool.query.isError || activePool.query.data?.success === false

  return (
    <div className='flex flex-col gap-4'>
      <ComponentVersionBar component='cliproxyapi' />
      <Tabs
        value={activePoolType}
        onValueChange={(value) =>
          setActivePoolType(value as CPAAccountPoolType)
        }
      >
        <TabsList
          variant='line'
          className='max-w-full justify-start overflow-x-auto'
        >
          <TabsTrigger value='cpa_import' className='shrink-0'>
            <UsersRound data-icon='inline-start' />
            {t('CPA account pool')}
          </TabsTrigger>
          <TabsTrigger value='temporary' className='shrink-0'>
            <TimerReset data-icon='inline-start' />
            {t('Temporary account pool')}
          </TabsTrigger>
          <TabsTrigger value='official_login' className='shrink-0'>
            <LogIn data-icon='inline-start' />
            {t('Official login account pool')}
          </TabsTrigger>
        </TabsList>
      </Tabs>

      {loadFailed ? (
        <div className='text-destructive rounded-lg border p-6 text-sm'>
          {activePool.query.data?.message || t('Failed to load')}
        </div>
      ) : (
        <AccountPoolSection
          accounts={activePool.query.data?.data?.accounts ?? []}
          emptyMessage={emptyMessage}
          now={now}
          workingName={workingName}
          isLoading={activePool.query.isLoading}
          isFetching={activePool.query.isFetching}
          summary={activePool.query.data?.data?.summary ?? emptySummary}
          pageIndex={activePool.pageIndex}
          pageSize={activePool.pageSize}
          sortBy={activePool.sortBy}
          sortOrder={activePool.sortOrder}
          selectedNames={activeSelection.selectedNames}
          selectingAll={activeSelection.selectingAll}
          onPageIndexChange={activePool.setPageIndex}
          onPageSizeChange={(pageSize) => {
            activePool.setPageSize(pageSize)
            activePool.setPageIndex(0)
          }}
          onSortByChange={activePool.setSortBy}
          onSortOrderChange={activePool.setSortOrder}
          onToggleSelection={activeSelection.toggleSelection}
          onTogglePageSelection={activeSelection.togglePageSelection}
          onSelectAll={() => selectAllAccounts(activeSelection)}
          onClearSelection={activeSelection.clearSelection}
          onManageSelection={() => setBatchPoolType(activePoolType)}
          onRefresh={() => activePool.query.refetch()}
          onToggle={toggle}
          onInspect={(account, operation) =>
            setInspection({ name: account.name, operation })
          }
          onArchive={archive}
          onRemove={remove}
        />
      )}
      <CPABatchManageDialog
        open={batchPoolType !== null}
        onOpenChange={(open) => {
          if (!open) setBatchPoolType(null)
        }}
        poolType={batchPoolType ?? 'cpa_import'}
        names={batchNames}
        onComplete={(failedNames) => {
          if (batchPoolType === 'cpa_import') {
            importedSelection.replaceSelection(failedNames)
          } else if (batchPoolType === 'temporary') {
            temporarySelection.replaceSelection(failedNames)
          } else if (batchPoolType === 'official_login') {
            officialSelection.replaceSelection(failedNames)
          }
        }}
      />
      <AccountInspectorDialog
        open={inspection !== null}
        onOpenChange={(open) => !open && setInspection(null)}
        initialProvider='cpa'
        initialIdentifier={inspection?.name}
        initialOperation={inspection?.operation}
      />
    </div>
  )
}
