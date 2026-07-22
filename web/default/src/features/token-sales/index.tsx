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
import { useQueryClient } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { TokenSaleDrawer } from './components/token-sale-drawer'
import { TokenSalesOverview } from './components/token-sales-overview'
import { TokenSalesTable } from './components/token-sales-table'
import { TokenSalesTransactions } from './components/token-sales-transactions'
import {
  TOKEN_SALES_DEFAULT_SECTION,
  TOKEN_SALES_SECTION_IDS,
  type TokenSalesSectionId,
} from './section-registry'

const route = getRouteApi('/_authenticated/token-sales/$section')

const SECTION_META: Record<TokenSalesSectionId, string> = {
  keys: 'Sales keys',
  transactions: 'Transactions',
  overview: 'Business overview',
}

export function TokenSales() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const params = route.useParams()
  const [createOpen, setCreateOpen] = useState(false)
  const activeSection =
    (params.section as TokenSalesSectionId) ?? TOKEN_SALES_DEFAULT_SECTION

  const changeSection = useCallback(
    (section: string) => {
      void navigate({
        to: '/token-sales/$section',
        params: { section: section as TokenSalesSectionId },
      })
    },
    [navigate]
  )

  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['token-sales'] }),
      queryClient.invalidateQueries({ queryKey: ['token-sales-overview'] }),
    ])
  }

  let content = <TokenSalesTable />
  if (activeSection === 'transactions') {
    content = <TokenSalesTransactions />
  } else if (activeSection === 'overview') {
    content = <TokenSalesOverview />
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t(SECTION_META[activeSection])}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {activeSection === 'keys' && (
            <Button size='sm' onClick={() => setCreateOpen(true)}>
              <Plus className='size-4' />
              {t('Create sales key')}
            </Button>
          )}
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-4'>
            <Tabs value={activeSection} onValueChange={changeSection}>
              <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
                {TOKEN_SALES_SECTION_IDS.map((section) => (
                  <TabsTrigger key={section} value={section}>
                    {t(SECTION_META[section])}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
            <div className='min-h-0 flex-1 overflow-auto'>{content}</div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <TokenSaleDrawer
        open={createOpen}
        onOpenChange={setCreateOpen}
        onSaved={refresh}
      />
    </>
  )
}
