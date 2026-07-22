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
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { formatQuota } from '@/lib/format'

import { getTokenSaleOverview } from '../api'

function formatCNY(minor: number) {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'CNY',
  }).format(minor / 100)
}

export function TokenSalesOverview() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['token-sales-overview'],
    queryFn: getTokenSaleOverview,
  })
  const summary = query.data?.data
  const fulfillment =
    summary && summary.granted_quota > 0
      ? (summary.used_quota / summary.granted_quota) * 100
      : 0
  const averageOrder =
    summary && summary.total_keys > 0
      ? summary.revenue_minor / summary.total_keys
      : 0

  return (
    <div className='space-y-4'>
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        {[
          [t('Revenue'), formatCNY(summary?.revenue_minor ?? 0)],
          [t('Average order value'), formatCNY(averageOrder)],
          [t('Delivered quota'), formatQuota(summary?.granted_quota ?? 0)],
          [t('Consumed quota'), formatQuota(summary?.used_quota ?? 0)],
        ].map(([label, value]) => (
          <Card key={String(label)} size='sm'>
            <CardHeader>
              <CardDescription>{label}</CardDescription>
              <CardTitle className='text-xl tabular-nums'>{value}</CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t('Quota fulfillment')}</CardTitle>
          <CardDescription>
            {t(
              'Consumed quota is recognized delivery; remaining quota is an outstanding service obligation.'
            )}
          </CardDescription>
          <div className='space-y-2 pt-2'>
            <div className='flex justify-between text-sm'>
              <span>{formatQuota(summary?.used_quota ?? 0)}</span>
              <span className='text-muted-foreground'>
                {fulfillment.toFixed(1)}%
              </span>
            </div>
            <Progress value={fulfillment} />
            <div className='text-muted-foreground text-xs'>
              {t('Outstanding quota: {{quota}}', {
                quota: formatQuota(summary?.remaining_quota ?? 0),
              })}
            </div>
          </div>
        </CardHeader>
      </Card>
    </div>
  )
}
