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
import { getRouteApi, Link } from '@tanstack/react-router'
import { Database, Route, Settings2, UsersRound } from 'lucide-react'
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { getChannelOps } from './api'
import { ChannelsDialogs } from './components/channels-dialogs'
import { ChannelsPrimaryButtons } from './components/channels-primary-buttons'
import { ChannelsProvider } from './components/channels-provider'
import { ChannelsTable } from './components/channels-table'
import { CPAAccountsPanel } from './components/cpa-accounts-panel'
import { Sub2APIAccountsPanel } from './components/sub2api-accounts-panel'
import type { ChannelWorkspaceView } from './types'

const route = getRouteApi('/_authenticated/channels/')

export function Channels() {
  const { t } = useTranslation()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const activeView = search.view ?? 'channels'
  const isRoot = useAuthStore(
    (state) => state.auth.user?.role === ROLE.SUPER_ADMIN
  )
  const channelOpsQuery = useQuery({
    queryKey: ['channel-ops'],
    queryFn: getChannelOps,
    enabled: activeView === 'channels',
    retry: false,
    staleTime: 5 * 60 * 1000,
  })
  const retryTimes = channelOpsQuery.data?.data?.retry_times
  const retryLabel =
    typeof retryTimes === 'number' ? `${t('Max Retries')}: ${retryTimes}` : null
  let retryBadge = null
  if (retryLabel) {
    retryBadge = isRoot ? (
      <Tooltip>
        <TooltipTrigger
          render={
            <Badge
              variant='outline'
              className='shrink-0 cursor-pointer'
              aria-label={t('Retry Settings')}
              render={
                <Link
                  to='/system-settings/models/$section'
                  params={{ section: 'routing-reliability' }}
                />
              }
            />
          }
        >
          <span>{retryLabel}</span>
          <Settings2 data-icon='inline-end' />
        </TooltipTrigger>
        <TooltipContent>
          <p>{t('Retry Settings')}</p>
        </TooltipContent>
      </Tooltip>
    ) : (
      <Badge variant='outline' className='shrink-0'>
        {retryLabel}
      </Badge>
    )
  }

  const handleViewChange = useCallback(
    (value: string) => {
      void navigate({
        search: (previous) => ({
          ...previous,
          view: value as ChannelWorkspaceView,
        }),
      })
    },
    [navigate]
  )

  let title = t('Channels')
  if (activeView === 'sub2api') {
    title = t('Sub2API account pool')
  } else if (activeView === 'cpa') {
    title = t('CPA account pool')
  }

  return (
    <ChannelsProvider>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          <span className='flex min-w-0 items-center gap-2'>
            <span className='truncate'>{title}</span>
            {activeView === 'channels' && retryBadge}
          </span>
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <ChannelsPrimaryButtons view={activeView} />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex min-h-full flex-col gap-4'>
            <Tabs value={activeView} onValueChange={handleViewChange}>
              <TabsList
                variant='line'
                className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'
              >
                <TabsTrigger value='channels'>
                  <Route data-icon='inline-start' />
                  {t('Channels')}
                </TabsTrigger>
                <TabsTrigger value='sub2api'>
                  <Database data-icon='inline-start' />
                  {t('Sub2API account pool')}
                </TabsTrigger>
                <TabsTrigger value='cpa'>
                  <UsersRound data-icon='inline-start' />
                  {t('CPA account pool')}
                </TabsTrigger>
              </TabsList>
            </Tabs>

            <div className='min-h-0 flex-1'>
              {activeView === 'channels' && <ChannelsTable />}
              {activeView === 'sub2api' && <Sub2APIAccountsPanel />}
              {activeView === 'cpa' && <CPAAccountsPanel />}
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ChannelsDialogs />
    </ChannelsProvider>
  )
}
