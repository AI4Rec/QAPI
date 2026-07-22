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
import { Loader2 } from 'lucide-react'
import { useDeferredValue, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Combobox,
  ComboboxCollection,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from '@/components/ui/combobox'

import { getVendorsByIds, searchVendors } from '../api'
import { vendorsQueryKeys } from '../lib'

type VendorComboboxProps = {
  value?: number
  onValueChange: (value: number | undefined) => void
  disabled?: boolean
}

export function VendorCombobox(props: VendorComboboxProps) {
  const { t } = useTranslation()
  const [inputValue, setInputValue] = useState('')
  const deferredInput = useDeferredValue(inputValue)
  const searchQuery = useQuery({
    queryKey: vendorsQueryKeys.list({ keyword: deferredInput }),
    queryFn: () =>
      searchVendors({ keyword: deferredInput.trim(), p: 1, page_size: 20 }),
  })
  const selectedQuery = useQuery({
    queryKey: vendorsQueryKeys.list({ selectedId: props.value }),
    queryFn: () => getVendorsByIds(props.value ? [props.value] : []),
    enabled: Boolean(props.value),
  })
  const vendors = useMemo(() => {
    const byId = new Map<number, { id: number; name: string }>()
    for (const vendor of selectedQuery.data?.data ?? []) {
      byId.set(vendor.id, vendor)
    }
    for (const vendor of searchQuery.data?.data?.items ?? []) {
      byId.set(vendor.id, vendor)
    }
    return [...byId.values()]
  }, [searchQuery.data?.data?.items, selectedQuery.data?.data])
  const labelByID = useMemo(
    () => new Map(vendors.map((vendor) => [String(vendor.id), vendor.name])),
    [vendors]
  )
  const items = useMemo(
    () => vendors.map((vendor) => String(vendor.id)),
    [vendors]
  )

  return (
    <Combobox
      items={items}
      filteredItems={items}
      value={props.value ? String(props.value) : null}
      onValueChange={(value) =>
        props.onValueChange(value ? Number(value) : undefined)
      }
      inputValue={inputValue}
      onInputValueChange={setInputValue}
      itemToStringLabel={(value) => labelByID.get(value) ?? value}
      itemToStringValue={(value) => labelByID.get(value) ?? value}
      disabled={props.disabled}
    >
      <ComboboxInput
        placeholder={t('Select vendor')}
        aria-label={t('Select vendor')}
        showClear
        className='w-full'
      />
      <ComboboxContent>
        <ComboboxList>
          <ComboboxCollection>
            {(item: string) => (
              <ComboboxItem key={item} value={item}>
                {labelByID.get(item) ?? item}
              </ComboboxItem>
            )}
          </ComboboxCollection>
        </ComboboxList>
        <ComboboxEmpty>
          {searchQuery.isFetching ? (
            <Loader2 className='mx-auto size-4 animate-spin' />
          ) : (
            t('No results found.')
          )}
        </ComboboxEmpty>
      </ComboboxContent>
    </Combobox>
  )
}
