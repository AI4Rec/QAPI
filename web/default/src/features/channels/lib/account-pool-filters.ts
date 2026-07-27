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
import type { CPAAccount, Sub2APIAccount } from '../api'

export type AccountPoolFilter =
  | 'all'
  | 'insufficient_quota'
  | 'forbidden'
  | 'paused'

export const ACCOUNT_POOL_FILTER_OPTIONS: Array<{
  value: AccountPoolFilter
  label: string
}> = [
  { value: 'all', label: 'All accounts' },
  { value: 'insufficient_quota', label: 'Insufficient quota' },
  { value: 'forbidden', label: 'Access Forbidden' },
  { value: 'paused', label: 'Paused' },
]

const INSUFFICIENT_QUOTA_MARKERS = [
  'insufficient quota',
  'quota insufficient',
  'limit reached',
  '额度不足',
]

const FORBIDDEN_MARKERS = ['forbidden', '403', '禁止访问']
const PAUSED_MARKERS = ['paused', '暂停', '已暂停']

function textIncludesAnyMarker(
  values: Array<string | undefined>,
  markers: string[]
) {
  for (const value of values) {
    const normalized = value?.toLowerCase()
    if (!normalized) continue
    for (const marker of markers) {
      if (normalized.includes(marker)) return true
    }
  }
  return false
}

export function cpaAccountMatchesPoolFilter(
  account: CPAAccount,
  filter: AccountPoolFilter
) {
  switch (filter) {
    case 'insufficient_quota':
      return (
        account.usage?.rate_limit?.limit_reached === true ||
        textIncludesAnyMarker(
          [account.status_message, account.usage?.error],
          INSUFFICIENT_QUOTA_MARKERS
        )
      )
    case 'forbidden':
      return textIncludesAnyMarker(
        [account.status, account.status_message, account.usage?.error],
        FORBIDDEN_MARKERS
      )
    case 'paused':
      return (
        account.disabled ||
        textIncludesAnyMarker(
          [account.status, account.status_message],
          PAUSED_MARKERS
        )
      )
    default:
      return true
  }
}

export function sub2APIAccountMatchesPoolFilter(
  account: Sub2APIAccount,
  filter: AccountPoolFilter
) {
  switch (filter) {
    case 'insufficient_quota':
      return (
        account.usage?.rate_limit?.limit_reached === true ||
        textIncludesAnyMarker(
          [account.error_message, account.usage?.error],
          INSUFFICIENT_QUOTA_MARKERS
        )
      )
    case 'forbidden':
      return textIncludesAnyMarker(
        [account.status, account.error_message, account.usage?.error],
        FORBIDDEN_MARKERS
      )
    case 'paused':
      return (
        !account.schedulable ||
        textIncludesAnyMarker(
          [account.status, account.error_message],
          PAUSED_MARKERS
        )
      )
    default:
      return true
  }
}
