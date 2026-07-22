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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Copy, Pencil, Search, Trash2 } from 'lucide-react'
import { useDeferredValue, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { PaginationControls } from '@/components/pagination-controls'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Progress } from '@/components/ui/progress'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { formatQuota, formatTimestampRelative } from '@/lib/format'

import {
  deleteTokenSale,
  getTokenSaleKey,
  getTokenSaleOverview,
  listTokenSales,
} from '../api'
import type { TokenSale } from '../types'
import { TokenSaleDrawer } from './token-sale-drawer'

function formatCNY(minor: number) {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'CNY',
  }).format(minor / 100)
}

function QuotaProgress(props: { sale: TokenSale }) {
  const total = props.sale.granted_quota
  const used = props.sale.used_quota
  const percentage = total > 0 ? Math.min(100, (used / total) * 100) : 0

  return (
    <div className='w-40 space-y-1'>
      <div className='flex justify-between text-xs'>
        <span>{formatQuota(used)}</span>
        <span className='text-muted-foreground'>{formatQuota(total)}</span>
      </div>
      <Progress value={percentage} />
    </div>
  )
}

function SalesKeyCards(props: {
  sales: TokenSale[]
  onEdit: (sale: TokenSale) => void
  onCopy: (sale: TokenSale) => Promise<void>
  onDelete: (sale: TokenSale) => void
  copyingSaleId?: number
  deletingSaleId?: number
}) {
  const { t } = useTranslation()
  return (
    <div className='space-y-3 md:hidden'>
      {props.sales.map((sale) => (
        <Card key={sale.id} size='sm'>
          <CardHeader className='flex-row items-start justify-between'>
            <div>
              <CardTitle className='text-sm'>{sale.name}</CardTitle>
              <div className='text-muted-foreground mt-1 font-mono text-xs'>
                sk-{sale.key}
              </div>
            </div>
            <StatusBadge
              label={t(sale.sale_status === 'active' ? 'Active' : 'Closed')}
              variant={sale.sale_status === 'active' ? 'success' : 'neutral'}
              copyable={false}
            />
          </CardHeader>
          <CardContent className='space-y-3'>
            <QuotaProgress sale={sale} />
            <div className='grid grid-cols-2 gap-2 text-xs'>
              <span className='text-muted-foreground'>{t('Buyer')}</span>
              <span>{sale.buyer || '—'}</span>
              <span className='text-muted-foreground'>
                {t('Maximum concurrency')}
              </span>
              <span>{sale.max_concurrency || t('Unlimited')}</span>
              <span className='text-muted-foreground'>{t('Revenue')}</span>
              <span>{formatCNY(sale.amount_minor)}</span>
            </div>
            <div className='grid grid-cols-2 gap-2'>
              <Button
                variant='outline'
                size='sm'
                disabled={props.copyingSaleId === sale.id}
                onClick={() => void props.onCopy(sale)}
              >
                <Copy className='size-4' />
                {t('Copy Key')}
              </Button>
              <Button
                variant='outline'
                size='sm'
                onClick={() => props.onEdit(sale)}
              >
                <Pencil className='size-4' />
                {t('Edit')}
              </Button>
              <Button
                variant='destructive'
                size='sm'
                className='col-span-2'
                disabled={props.deletingSaleId === sale.id}
                onClick={() => props.onDelete(sale)}
              >
                <Trash2 data-icon='inline-start' />
                {t('Delete')}
              </Button>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

export function TokenSalesTable() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState('')
  const [editingSale, setEditingSale] = useState<TokenSale>()
  const [deleteTarget, setDeleteTarget] = useState<TokenSale>()
  const [copyingSaleId, setCopyingSaleId] = useState<number>()
  const [deletingSaleId, setDeletingSaleId] = useState<number>()
  const [pageIndex, setPageIndex] = useState(0)
  const [pageSize, setPageSize] = useState(20)
  const deferredKeyword = useDeferredValue(keyword)

  const salesQuery = useQuery({
    queryKey: ['token-sales', deferredKeyword, status, pageIndex, pageSize],
    queryFn: () =>
      listTokenSales({
        keyword: deferredKeyword.trim(),
        status,
        page: pageIndex + 1,
        pageSize,
      }),
    placeholderData: (previous) => previous,
  })
  const overviewQuery = useQuery({
    queryKey: ['token-sales-overview'],
    queryFn: getTokenSaleOverview,
  })

  const sales = salesQuery.data?.data?.items ?? []
  const summary = overviewQuery.data?.data

  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['token-sales'] }),
      queryClient.invalidateQueries({ queryKey: ['token-sales-overview'] }),
    ])
  }

  const copySaleKey = async (sale: TokenSale) => {
    setCopyingSaleId(sale.id)
    try {
      const result = await getTokenSaleKey(sale.id)
      if (!result.success || !result.data?.key) {
        throw new Error(result.message || t('Copy failed'))
      }
      const copied = await copyToClipboard(result.data.key)
      toast[copied ? 'success' : 'error'](
        copied ? t('Copied') : t('Copy failed')
      )
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Copy failed'))
    } finally {
      setCopyingSaleId(undefined)
    }
  }

  const removeSale = async () => {
    if (!deleteTarget) return
    setDeletingSaleId(deleteTarget.id)
    try {
      const result = await deleteTokenSale(deleteTarget.id)
      if (!result.success) {
        throw new Error(result.message || t('Failed to delete sales key'))
      }
      toast.success(t('Sales key deleted'))
      setDeleteTarget(undefined)
      if (pageIndex > 0 && sales.length === 1) {
        setPageIndex((current) => Math.max(0, current - 1))
      }
      await refresh()
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to delete sales key')
      )
    } finally {
      setDeletingSaleId(undefined)
    }
  }

  return (
    <div className='space-y-4'>
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        {[
          [t('Active sales keys'), summary?.active_keys ?? 0],
          [t('Total sales keys'), summary?.total_keys ?? 0],
          [t('Revenue'), formatCNY(summary?.revenue_minor ?? 0)],
          [t('Remaining quota'), formatQuota(summary?.remaining_quota ?? 0)],
        ].map(([label, value]) => (
          <Card key={String(label)} size='sm'>
            <CardHeader>
              <div className='text-muted-foreground text-xs'>{label}</div>
              <CardTitle className='text-xl tabular-nums'>{value}</CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>

      <div className='flex flex-col gap-2 sm:flex-row'>
        <div className='relative flex-1'>
          <Search className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2' />
          <Input
            value={keyword}
            onChange={(event) => {
              setKeyword(event.target.value)
              setPageIndex(0)
            }}
            placeholder={t('Search buyer, order number, or key name')}
            className='pl-9'
          />
        </div>
        <NativeSelect
          value={status}
          onChange={(event) => {
            setStatus(event.target.value)
            setPageIndex(0)
          }}
          className='w-full sm:w-44'
        >
          <NativeSelectOption value=''>{t('All statuses')}</NativeSelectOption>
          <NativeSelectOption value='active'>{t('Active')}</NativeSelectOption>
          <NativeSelectOption value='closed'>{t('Closed')}</NativeSelectOption>
        </NativeSelect>
      </div>

      {salesQuery.isLoading && (
        <div className='text-muted-foreground py-16 text-center'>
          {t('Loading sales keys')}
        </div>
      )}
      {!salesQuery.isLoading && sales.length === 0 && (
        <div className='rounded-lg border py-16 text-center'>
          <p className='font-medium'>{t('No sales keys found')}</p>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t('Create a sales key to start manual API delivery.')}
          </p>
        </div>
      )}
      {!salesQuery.isLoading && sales.length > 0 && (
        <>
          <div className='hidden rounded-lg border md:block'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Buyer / Key')}</TableHead>
                  <TableHead>{t('Quota progress')}</TableHead>
                  <TableHead>{t('Expiration')}</TableHead>
                  <TableHead>{t('Request limits')}</TableHead>
                  <TableHead>{t('Revenue')}</TableHead>
                  <TableHead>{t('Last used')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead className='w-32' />
                </TableRow>
              </TableHeader>
              <TableBody>
                {sales.map((sale) => (
                  <TableRow key={sale.id}>
                    <TableCell>
                      <div className='font-medium'>
                        {sale.buyer || sale.name}
                      </div>
                      <div className='text-muted-foreground font-mono text-xs'>
                        sk-{sale.key}
                      </div>
                    </TableCell>
                    <TableCell>
                      <QuotaProgress sale={sale} />
                    </TableCell>
                    <TableCell>
                      {sale.expired_time > 0
                        ? formatTimestampRelative(sale.expired_time)
                        : t('Never')}
                    </TableCell>
                    <TableCell>
                      <div>
                        {t('{{count}} concurrent', {
                          count: sale.max_concurrency || '∞',
                        })}
                      </div>
                      <div className='text-muted-foreground text-xs'>
                        {t('{{count}} RPM', {
                          count: sale.rpm_rate_limit || '∞',
                        })}
                      </div>
                    </TableCell>
                    <TableCell>{formatCNY(sale.amount_minor)}</TableCell>
                    <TableCell>
                      {formatTimestampRelative(sale.last_used_at)}
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        label={t(
                          sale.sale_status === 'active' ? 'Active' : 'Closed'
                        )}
                        variant={
                          sale.sale_status === 'active' ? 'success' : 'neutral'
                        }
                        copyable={false}
                      />
                    </TableCell>
                    <TableCell>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        aria-label={t('Copy Key')}
                        disabled={copyingSaleId === sale.id}
                        onClick={() => void copySaleKey(sale)}
                      >
                        <Copy className='size-4' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        aria-label={t('Edit sales key')}
                        onClick={() => setEditingSale(sale)}
                      >
                        <Pencil className='size-4' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        className='text-destructive hover:text-destructive'
                        aria-label={t('Delete sales key')}
                        title={t('Delete sales key')}
                        disabled={deletingSaleId === sale.id}
                        onClick={() => setDeleteTarget(sale)}
                      >
                        <Trash2 />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <SalesKeyCards
            sales={sales}
            onEdit={setEditingSale}
            onCopy={copySaleKey}
            onDelete={setDeleteTarget}
            copyingSaleId={copyingSaleId}
            deletingSaleId={deletingSaleId}
          />
          <PaginationControls
            pageIndex={pageIndex}
            pageSize={pageSize}
            totalCount={salesQuery.data?.data?.total ?? 0}
            pageCount={Math.ceil(
              (salesQuery.data?.data?.total ?? 0) / pageSize
            )}
            onPageIndexChange={setPageIndex}
            onPageSizeChange={(nextPageSize) => {
              setPageSize(nextPageSize)
              setPageIndex(0)
            }}
          />
        </>
      )}

      <TokenSaleDrawer
        open={Boolean(editingSale)}
        sale={editingSale}
        onOpenChange={(open) => {
          if (!open) setEditingSale(undefined)
        }}
        onSaved={refresh}
      />
      <ConfirmDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open && deletingSaleId === undefined) setDeleteTarget(undefined)
        }}
        title={t('Delete sales key {{name}}?', {
          name: deleteTarget?.name ?? '',
        })}
        desc={t(
          'This permanently deletes the sales record and disables its API key. This action cannot be undone.'
        )}
        destructive
        isLoading={deletingSaleId !== undefined}
        confirmText={
          deletingSaleId !== undefined ? t('Deleting...') : t('Delete')
        }
        handleConfirm={() => void removeSale()}
      />
    </div>
  )
}
