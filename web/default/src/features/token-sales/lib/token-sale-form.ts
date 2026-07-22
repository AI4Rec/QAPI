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
import type { TFunction } from 'i18next'
import { z } from 'zod'

import { getCurrencyDisplay } from '@/lib/currency'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'

import type {
  TokenSale,
  TokenSaleBasePayload,
  TokenSaleCreatePayload,
  TokenSaleUpdatePayload,
} from '../types'

export function getTokenSaleFormSchema(t: TFunction) {
  return z.object({
    name: z.string().trim().min(1, t('Please enter a name')).max(50),
    buyer: z.string().trim().max(128),
    order_no: z.string().trim().max(191),
    amount_cny: z.number().min(0),
    quota_amount: z.number().positive(t('Quota must be greater than zero')),
    validity_days: z.number().int().min(1).max(3650),
    expired_time: z.date(),
    max_concurrency: z.number().int().min(0).max(1000),
    rpm_rate_limit: z.number().int().min(0).max(100000),
    model_limits: z.array(z.string()),
    allow_ips: z.string(),
    group: z.string(),
    cross_group_retry: z.boolean(),
    note: z.string().max(500),
    status: z.enum(['active', 'closed']),
  })
}

export type TokenSaleFormValues = z.infer<
  ReturnType<typeof getTokenSaleFormSchema>
>

export function getTokenSaleFormDefaults(
  sale?: TokenSale
): TokenSaleFormValues {
  if (sale) {
    return {
      name: sale.name,
      buyer: sale.buyer,
      order_no: sale.order_no,
      amount_cny: sale.amount_minor / 100,
      quota_amount: quotaUnitsToDollars(sale.granted_quota),
      validity_days: 30,
      expired_time:
        sale.expired_time > 0
          ? new Date(sale.expired_time * 1000)
          : new Date(Date.now() + 30 * 24 * 60 * 60 * 1000),
      max_concurrency: sale.max_concurrency,
      rpm_rate_limit: sale.rpm_rate_limit,
      model_limits: sale.model_limits
        ? sale.model_limits.split(',').filter(Boolean)
        : [],
      allow_ips: sale.allow_ips,
      group: sale.group || 'default',
      cross_group_retry: sale.cross_group_retry,
      note: sale.note,
      status: sale.sale_status,
    }
  }

  return {
    name: '',
    buyer: '',
    order_no: '',
    amount_cny: 5,
    quota_amount: quotaUnitsToDollars(
      getCurrencyDisplay().config.quotaPerUnit * 100
    ),
    validity_days: 30,
    expired_time: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000),
    max_concurrency: 5,
    rpm_rate_limit: 60,
    model_limits: [],
    allow_ips: '',
    group: 'default',
    cross_group_retry: false,
    note: '',
    status: 'active',
  }
}

function tokenSaleFormToBasePayload(
  values: TokenSaleFormValues
): TokenSaleBasePayload {
  return {
    name: values.name.trim(),
    buyer: values.buyer.trim(),
    order_no: values.order_no.trim(),
    amount_minor: Math.round(values.amount_cny * 100),
    granted_quota: parseQuotaFromDollars(values.quota_amount),
    max_concurrency: values.max_concurrency,
    rpm_rate_limit: values.rpm_rate_limit,
    model_limits_enabled: values.model_limits.length > 0,
    model_limits: values.model_limits.join(','),
    allow_ips: values.allow_ips.trim(),
    group: values.group,
    cross_group_retry:
      values.group === 'auto' ? values.cross_group_retry : false,
    note: values.note.trim(),
  }
}

export function tokenSaleFormToCreatePayload(
  values: TokenSaleFormValues
): TokenSaleCreatePayload {
  return {
    ...tokenSaleFormToBasePayload(values),
    validity_days: values.validity_days,
  }
}

export function tokenSaleFormToUpdatePayload(
  values: TokenSaleFormValues
): TokenSaleUpdatePayload {
  return {
    ...tokenSaleFormToBasePayload(values),
    expired_time: Math.floor(values.expired_time.getTime() / 1000),
    status: values.status,
  }
}
