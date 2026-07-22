import {
  Cancel01Icon,
  Database02Icon,
  RefreshIcon,
  SearchIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { PaginationControls } from '@/components/pagination-controls'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useDebounce } from '@/hooks'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'

import { listOperationalCostArchives, restoreOperationalAsset } from '../api'
import type { OperationalAsset } from '../types'

type ArchiveSourceFilter = 'all' | OperationalAsset['source_type']

function formatMoney(minor: number) {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'CNY',
  }).format(minor / 100)
}

function formatDateTime(timestamp: number) {
  if (!timestamp) return '—'
  return new Date(timestamp * 1000).toLocaleString()
}

function ArchiveSourceBadge(props: {
  sourceType: OperationalAsset['source_type']
}) {
  const { t } = useTranslation()
  let label = t('Channel')
  if (props.sourceType === 'cpa_account') label = 'CPA'
  if (props.sourceType === 'sub2api_account') label = 'Sub2API'
  return <Badge variant='outline'>{label}</Badge>
}

function RestoreButton(props: {
  asset: OperationalAsset
  working: boolean
  onRestore: (asset: OperationalAsset) => void
}) {
  const { t } = useTranslation()
  return (
    <Button
      variant='outline'
      size='sm'
      disabled={props.working}
      onClick={() => props.onRestore(props.asset)}
    >
      <HugeiconsIcon icon={RefreshIcon} data-icon='inline-start' />
      {t('Restore')}
    </Button>
  )
}

function ArchiveLoadingRows() {
  return Array.from({ length: 5 }, (_, index) => (
    <TableRow key={index}>
      <TableCell>
        <Skeleton className='h-8 w-52' />
      </TableCell>
      <TableCell>
        <Skeleton className='h-6 w-16' />
      </TableCell>
      <TableCell>
        <Skeleton className='ml-auto h-5 w-20' />
      </TableCell>
      <TableCell>
        <Skeleton className='ml-auto h-5 w-20' />
      </TableCell>
      <TableCell>
        <Skeleton className='h-5 w-36' />
      </TableCell>
      <TableCell>
        <Skeleton className='h-5 w-44' />
      </TableCell>
      <TableCell>
        <Skeleton className='ml-auto h-8 w-20' />
      </TableCell>
    </TableRow>
  ))
}

export function ArchivedAssetsPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [sourceType, setSourceType] = useState<ArchiveSourceFilter>('all')
  const [pageIndex, setPageIndex] = useState(0)
  const [pageSize, setPageSize] = useState(20)
  const [workingId, setWorkingId] = useState<number | null>(null)
  const debouncedSearch = useDebounce(search.trim(), 300)

  const archivesQuery = useQuery({
    queryKey: [
      'operational-costs',
      'archives',
      pageIndex,
      pageSize,
      sourceType,
      debouncedSearch,
    ],
    queryFn: () =>
      listOperationalCostArchives({
        page: pageIndex + 1,
        pageSize,
        sourceType: sourceType === 'all' ? undefined : sourceType,
        search: debouncedSearch || undefined,
      }),
    placeholderData: (previous) => previous,
    staleTime: 15_000,
  })

  const archives = archivesQuery.data?.data?.items ?? []
  const total = archivesQuery.data?.data?.total ?? 0
  const hasFilters = sourceType !== 'all' || search.trim() !== ''
  const showEmpty =
    !archivesQuery.isError && !archivesQuery.isLoading && archives.length === 0
  const showTable = !archivesQuery.isError && !showEmpty
  const sourceItems = [
    { value: 'all', label: t('All asset types') },
    { value: 'cpa_account', label: 'CPA' },
    { value: 'sub2api_account', label: 'Sub2API' },
    { value: 'channel', label: t('Channel') },
  ]

  const clearFilters = () => {
    setSearch('')
    setSourceType('all')
    setPageIndex(0)
  }

  const restore = async (asset: OperationalAsset) => {
    setWorkingId(asset.id)
    try {
      const result = await restoreOperationalAsset(asset.id)
      if (!result.success) {
        throw new Error(result.message || t('Failed to restore asset'))
      }
      if (archives.length === 1 && pageIndex > 0) {
        setPageIndex(pageIndex - 1)
      }
      await queryClient.invalidateQueries({ queryKey: ['operational-costs'] })
      toast.success(t('Asset restored in disabled state'))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to restore asset')
      )
    } finally {
      setWorkingId(null)
    }
  }

  return (
    <section
      className='flex min-w-0 flex-col gap-4'
      aria-labelledby='archives-title'
    >
      <div className='flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between'>
        <div className='flex min-w-0 items-center gap-2'>
          <h2 id='archives-title' className='text-lg font-semibold'>
            {t('Archived assets')}
          </h2>
          <Badge variant='secondary'>{total.toLocaleString()}</Badge>
        </div>
        <div className='flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center'>
          <InputGroup className='sm:w-72'>
            <InputGroupAddon>
              <HugeiconsIcon icon={SearchIcon} />
            </InputGroupAddon>
            <InputGroupInput
              value={search}
              maxLength={100}
              placeholder={t('Search archived assets')}
              aria-label={t('Search archived assets')}
              onChange={(event) => {
                setSearch(event.target.value)
                setPageIndex(0)
              }}
            />
            {search && (
              <InputGroupAddon align='inline-end'>
                <InputGroupButton
                  size='icon-xs'
                  aria-label={t('Clear search')}
                  onClick={() => {
                    setSearch('')
                    setPageIndex(0)
                  }}
                >
                  <HugeiconsIcon icon={Cancel01Icon} />
                </InputGroupButton>
              </InputGroupAddon>
            )}
          </InputGroup>
          <Select
            items={sourceItems}
            value={sourceType}
            onValueChange={(value) => {
              setSourceType(value as ArchiveSourceFilter)
              setPageIndex(0)
            }}
          >
            <SelectTrigger className='w-full sm:w-40'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {sourceItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <Button
            variant='outline'
            size='icon-sm'
            aria-label={t('Refresh')}
            title={t('Refresh')}
            disabled={archivesQuery.isFetching}
            onClick={() => archivesQuery.refetch()}
          >
            <HugeiconsIcon icon={RefreshIcon} />
          </Button>
        </div>
      </div>

      {archivesQuery.isError && (
        <Empty className='min-h-52 border'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <HugeiconsIcon icon={Database02Icon} />
            </EmptyMedia>
            <EmptyTitle>{t('Failed to load archived assets')}</EmptyTitle>
          </EmptyHeader>
          <EmptyContent>
            <Button variant='outline' onClick={() => archivesQuery.refetch()}>
              <HugeiconsIcon icon={RefreshIcon} data-icon='inline-start' />
              {t('Retry')}
            </Button>
          </EmptyContent>
        </Empty>
      )}
      {showEmpty && (
        <Empty className='min-h-52 border'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <HugeiconsIcon icon={Database02Icon} />
            </EmptyMedia>
            <EmptyTitle>
              {hasFilters
                ? t('No archived assets match your filters')
                : t('No archived assets')}
            </EmptyTitle>
            {hasFilters && (
              <EmptyDescription>
                {t('Try changing or clearing the filters.')}
              </EmptyDescription>
            )}
          </EmptyHeader>
          {hasFilters && (
            <EmptyContent>
              <Button variant='outline' onClick={clearFilters}>
                {t('Clear filters')}
              </Button>
            </EmptyContent>
          )}
        </Empty>
      )}
      {showTable && (
        <div className='overflow-hidden rounded-lg border'>
          <Table className='min-w-[980px]'>
            <TableHeader className='bg-muted/40'>
              <TableRow>
                <TableHead className='pl-4'>{t('Asset')}</TableHead>
                <TableHead>{t('Type')}</TableHead>
                <TableHead className='text-right'>
                  {t('Acquisition cost')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('Cumulative output')}
                </TableHead>
                <TableHead>{t('Archived at')}</TableHead>
                <TableHead>{t('Archive reason')}</TableHead>
                <TableHead className='pr-4 text-right'>
                  {t('Actions')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {archivesQuery.isLoading ? (
                <ArchiveLoadingRows />
              ) : (
                archives.map((asset) => (
                  <TableRow key={asset.id}>
                    <TableCell className='max-w-72 pl-4'>
                      <div className='truncate font-medium'>
                        {asset.display_name}
                      </div>
                      {asset.cost_note && (
                        <div className='text-muted-foreground mt-0.5 truncate text-xs'>
                          {asset.cost_note}
                        </div>
                      )}
                    </TableCell>
                    <TableCell>
                      <ArchiveSourceBadge sourceType={asset.source_type} />
                    </TableCell>
                    <TableCell className='text-right font-medium'>
                      {formatMoney(asset.cost_minor)}
                    </TableCell>
                    <TableCell className='text-right font-medium'>
                      {asset.cumulative_output_usd == null
                        ? '—'
                        : formatBillingCurrencyFromUSD(
                            asset.cumulative_output_usd
                          )}
                    </TableCell>
                    <TableCell>{formatDateTime(asset.archived_at)}</TableCell>
                    <TableCell className='max-w-64'>
                      <span className='block truncate'>
                        {asset.archive_reason || '—'}
                      </span>
                    </TableCell>
                    <TableCell className='pr-4 text-right'>
                      <RestoreButton
                        asset={asset}
                        working={workingId === asset.id}
                        onRestore={restore}
                      />
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      )}

      {total > 0 && (
        <PaginationControls
          pageIndex={pageIndex}
          pageSize={pageSize}
          totalCount={total}
          pageCount={Math.ceil(total / pageSize)}
          onPageIndexChange={setPageIndex}
          onPageSizeChange={(nextPageSize) => {
            setPageSize(nextPageSize)
            setPageIndex(0)
          }}
        />
      )}
    </section>
  )
}
