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
import {
  Plus,
  MoreHorizontal,
  Settings2,
  Trash2,
  Tags,
  TestTube,
  DollarSign,
  ListChecks,
  SortAsc,
  RefreshCw,
  ArrowUpFromLine,
  PackagePlus,
  LogIn,
  Database,
  Activity,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuCheckboxItem,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ActivationQueueWidget } from '@/features/activation-queue/activation-queue-widget'
import {
  ADMIN_PERMISSION_ACTIONS,
  ADMIN_PERMISSION_RESOURCES,
  hasPermission,
} from '@/lib/admin-permissions'
import { useAuthStore } from '@/stores/auth-store'

import {
  handleDeleteAllDisabled,
  handleFixAbilities,
  handleTestAllChannels,
  handleUpdateAllBalances,
} from '../lib'
import type { ChannelWorkspaceView } from '../types'
import { useChannels } from './channels-provider'
import { AccountInspectorDialog } from './dialogs/account-inspector-dialog'
import { CPAImportDialog } from './dialogs/cpa-import-dialog'
import { CPAOfficialLoginDialog } from './dialogs/cpa-official-login-dialog'
import { Sub2APIImportDialog } from './dialogs/sub2api-import-dialog'

type ChannelsPrimaryButtonsProps = {
  view: ChannelWorkspaceView
}

export function ChannelsPrimaryButtons(props: ChannelsPrimaryButtonsProps) {
  const { t } = useTranslation()
  const {
    setOpen,
    setCurrentRow,
    enableTagMode,
    setEnableTagMode,
    idSort,
    setIdSort,
    batchMode,
    setBatchMode,
    upstream,
  } = useChannels()
  const queryClient = useQueryClient()
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const [showConsistencyDialog, setShowConsistencyDialog] = useState(false)
  const [isRepairingConsistency, setIsRepairingConsistency] = useState(false)
  const [showCPAImportDialog, setShowCPAImportDialog] = useState(false)
  const [showSub2APIImportDialog, setShowSub2APIImportDialog] = useState(false)
  const [showAccountInspector, setShowAccountInspector] = useState(false)
  const [showCPAOfficialLoginDialog, setShowCPAOfficialLoginDialog] =
    useState(false)
  const currentUser = useAuthStore((s) => s.auth.user)
  const canEditSensitive = hasPermission(
    currentUser,
    ADMIN_PERMISSION_RESOURCES.CHANNEL,
    ADMIN_PERMISSION_ACTIONS.SENSITIVE_WRITE
  )

  const handleTagModeToggle = (checked: boolean) => {
    localStorage.setItem('enable-tag-mode', String(checked))
    setEnableTagMode(checked)
  }

  const handleIdSortToggle = (checked: boolean) => {
    localStorage.setItem('channels-id-sort', String(checked))
    setIdSort(checked)
  }

  const handleBatchModeToggle = (checked: boolean) => {
    setBatchMode(checked)
  }

  return (
    <>
      <div className='flex items-center gap-2'>
        {(props.view === 'cpa' || props.view === 'sub2api') && (
          <Button
            variant='outline'
            size='sm'
            onClick={() => setShowAccountInspector(true)}
          >
            <Activity data-icon='inline-start' />
            <span className='max-sm:hidden'>{t('Account inspector')}</span>
          </Button>
        )}
        {props.view === 'channels' && (
          <>
            <Button
              variant={batchMode ? 'secondary' : 'outline'}
              size='sm'
              onClick={() => handleBatchModeToggle(!batchMode)}
              aria-label={t('Batch Operations')}
              aria-pressed={batchMode}
            >
              <ListChecks data-icon='inline-start' />
              <span className='max-sm:hidden'>{t('Batch Operations')}</span>
            </Button>

            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    size='sm'
                    disabled={!canEditSensitive}
                    aria-label={t('Create Channel')}
                    onClick={() => {
                      if (!canEditSensitive) return
                      setCurrentRow(null)
                      setOpen('create-channel')
                    }}
                  />
                }
              >
                <Plus data-icon='inline-start' />
                <span className='max-sm:hidden'>{t('Create Channel')}</span>
                <span className='sm:hidden'>{t('Create')}</span>
              </TooltipTrigger>
              {!canEditSensitive && (
                <TooltipContent>
                  {t('No permission to perform this action')}
                </TooltipContent>
              )}
            </Tooltip>

            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button
                    variant='outline'
                    size='icon-sm'
                    aria-label={t('More')}
                  />
                }
              >
                <MoreHorizontal />
              </DropdownMenuTrigger>
              <DropdownMenuContent align='end' className='w-64'>
                <DropdownMenuGroup>
                  <DropdownMenuCheckboxItem
                    checked={enableTagMode}
                    onCheckedChange={handleTagModeToggle}
                  >
                    <Tags />
                    {t('Tag Mode')}
                  </DropdownMenuCheckboxItem>
                  <DropdownMenuCheckboxItem
                    checked={idSort}
                    onCheckedChange={handleIdSortToggle}
                  >
                    <SortAsc />
                    {t('Sort by ID')}
                  </DropdownMenuCheckboxItem>
                </DropdownMenuGroup>

                <DropdownMenuSeparator />

                <DropdownMenuGroup>
                  <DropdownMenuItem
                    onClick={() => handleTestAllChannels(queryClient)}
                  >
                    {t('Test All Channels')}
                    <DropdownMenuShortcut>
                      <TestTube />
                    </DropdownMenuShortcut>
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={() => handleUpdateAllBalances(queryClient)}
                  >
                    {t('Update All Balances')}
                    <DropdownMenuShortcut>
                      <DollarSign />
                    </DropdownMenuShortcut>
                  </DropdownMenuItem>
                </DropdownMenuGroup>

                <DropdownMenuSeparator />

                <DropdownMenuGroup>
                  <DropdownMenuItem
                    onClick={() => upstream.detectAllUpdates()}
                    disabled={upstream.detectAllLoading}
                  >
                    {t('Detect All Upstream Updates')}
                    <DropdownMenuShortcut>
                      <RefreshCw />
                    </DropdownMenuShortcut>
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={() => upstream.applyAllUpdates()}
                    disabled={upstream.applyAllLoading}
                  >
                    {t('Apply All Upstream Updates')}
                    <DropdownMenuShortcut>
                      <ArrowUpFromLine />
                    </DropdownMenuShortcut>
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onSelect={(event) => {
                      event.preventDefault()
                      setShowConsistencyDialog(true)
                    }}
                  >
                    {t('Repair Channel Consistency')}
                    <DropdownMenuShortcut>
                      <Settings2 />
                    </DropdownMenuShortcut>
                  </DropdownMenuItem>
                </DropdownMenuGroup>

                <DropdownMenuSeparator />

                <DropdownMenuGroup>
                  <DropdownMenuItem
                    onSelect={(event) => {
                      event.preventDefault()
                      if (!canEditSensitive) return
                      setShowDeleteDialog(true)
                    }}
                    disabled={!canEditSensitive}
                    className='text-destructive focus:text-destructive'
                  >
                    {t('Delete All Disabled')}
                    <DropdownMenuShortcut>
                      <Trash2 />
                    </DropdownMenuShortcut>
                  </DropdownMenuItem>
                </DropdownMenuGroup>
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        )}

        {props.view === 'sub2api' && (
          <Button
            size='sm'
            onClick={() => setShowSub2APIImportDialog(true)}
            disabled={!canEditSensitive}
          >
            <Database data-icon='inline-start' />
            {t('Sub2API stock upload')}
          </Button>
        )}

        {props.view === 'cpa' && (
          <>
            <Button
              size='sm'
              onClick={() => setShowCPAImportDialog(true)}
              disabled={!canEditSensitive}
            >
              <PackagePlus data-icon='inline-start' />
              <span className='max-sm:hidden'>{t('CPA stock upload')}</span>
            </Button>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setShowCPAOfficialLoginDialog(true)}
              disabled={!canEditSensitive}
            >
              <LogIn data-icon='inline-start' />
              <span className='max-sm:hidden'>{t('Official Login')}</span>
            </Button>
            <ActivationQueueWidget />
          </>
        )}
      </div>

      <ConfirmDialog
        open={showDeleteDialog}
        onOpenChange={setShowDeleteDialog}
        title={t('Delete All Disabled Channels?')}
        desc={t(
          'This will permanently delete all manually and automatically disabled channels. This action cannot be undone.'
        )}
        destructive
        handleConfirm={() => {
          if (!canEditSensitive) return
          handleDeleteAllDisabled(queryClient, (_count) => {
            // eslint-disable-next-line no-console
            console.log(`Deleted ${_count} channels`)
          })
          setShowDeleteDialog(false)
        }}
      />

      <CPAImportDialog
        open={showCPAImportDialog}
        onOpenChange={setShowCPAImportDialog}
      />

      <CPAOfficialLoginDialog
        open={showCPAOfficialLoginDialog}
        onOpenChange={setShowCPAOfficialLoginDialog}
      />

      <Sub2APIImportDialog
        open={showSub2APIImportDialog}
        onOpenChange={setShowSub2APIImportDialog}
      />

      <AccountInspectorDialog
        open={showAccountInspector}
        onOpenChange={setShowAccountInspector}
        initialProvider={props.view === 'cpa' ? 'cpa' : 'sub2api'}
      />

      <ConfirmDialog
        open={showConsistencyDialog}
        onOpenChange={setShowConsistencyDialog}
        title={t('Repair channel consistency?')}
        desc={t(
          'This will rebuild the channel routing index from every channel configuration, including supported models, groups, priorities, and weights. Routing may be briefly incomplete while the rebuild is running. Continue?'
        )}
        confirmText={t('Repair')}
        isLoading={isRepairingConsistency}
        handleConfirm={async () => {
          setIsRepairingConsistency(true)
          try {
            await handleFixAbilities(queryClient, (_result) => {
              // eslint-disable-next-line no-console
              console.log('Repair channel consistency result:', _result)
            })
            setShowConsistencyDialog(false)
          } finally {
            setIsRepairingConsistency(false)
          }
        }}
      />
    </>
  )
}
