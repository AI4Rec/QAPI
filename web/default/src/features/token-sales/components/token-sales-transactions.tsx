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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { PaginationControls } from '@/components/pagination-controls'
import { StatusBadge } from '@/components/status-badge'
import { Card, CardContent } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatQuota, formatTimestampToDate } from '@/lib/format'

import { listTokenSales } from '../api'

function formatCNY(minor: number) {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'CNY',
  }).format(minor / 100)
}

export function TokenSalesTransactions() {
  const { t } = useTranslation()
  const [pageIndex, setPageIndex] = useState(0)
  const [pageSize, setPageSize] = useState(20)
  const query = useQuery({
    queryKey: ['token-sales', 'transactions', pageIndex, pageSize],
    queryFn: () => listTokenSales({ page: pageIndex + 1, pageSize }),
    placeholderData: (previous) => previous,
  })
  const sales = query.data?.data?.items ?? []

  if (query.isLoading) {
    return (
      <div className='text-muted-foreground py-16 text-center'>
        {t('Loading transactions')}
      </div>
    )
  }

  return (
    <Card>
      <CardContent className='p-0'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Delivery time')}</TableHead>
              <TableHead>{t('Buyer')}</TableHead>
              <TableHead>{t('External order number')}</TableHead>
              <TableHead>{t('Key name')}</TableHead>
              <TableHead>{t('Revenue')}</TableHead>
              <TableHead>{t('Granted quota')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sales.map((sale) => (
              <TableRow key={sale.id}>
                <TableCell>
                  {formatTimestampToDate(sale.delivered_at)}
                </TableCell>
                <TableCell>{sale.buyer || '—'}</TableCell>
                <TableCell>{sale.order_no || '—'}</TableCell>
                <TableCell>{sale.name}</TableCell>
                <TableCell>{formatCNY(sale.amount_minor)}</TableCell>
                <TableCell>{formatQuota(sale.granted_quota)}</TableCell>
                <TableCell>
                  <StatusBadge
                    label={t(
                      sale.sale_status === 'active' ? 'Delivered' : 'Closed'
                    )}
                    variant={
                      sale.sale_status === 'active' ? 'success' : 'neutral'
                    }
                    copyable={false}
                  />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {sales.length === 0 && (
          <div className='text-muted-foreground py-16 text-center'>
            {t('No delivery transactions yet')}
          </div>
        )}
        {sales.length > 0 && (
          <div className='border-t p-4'>
            <PaginationControls
              pageIndex={pageIndex}
              pageSize={pageSize}
              totalCount={query.data?.data?.total ?? 0}
              pageCount={Math.ceil((query.data?.data?.total ?? 0) / pageSize)}
              onPageIndexChange={setPageIndex}
              onPageSizeChange={(nextPageSize) => {
                setPageSize(nextPageSize)
                setPageIndex(0)
              }}
            />
          </div>
        )}
      </CardContent>
    </Card>
  )
}
