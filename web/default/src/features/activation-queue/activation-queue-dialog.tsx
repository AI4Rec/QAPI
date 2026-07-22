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

import { Dialog } from '@/components/dialog'
import { PaginationControls } from '@/components/pagination-controls'
import { Badge } from '@/components/ui/badge'

import { listActivationQueueTargets } from './api'

type ActivationQueueDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ActivationQueueDialog(props: ActivationQueueDialogProps) {
  const { t } = useTranslation()
  const [targetPageIndex, setTargetPageIndex] = useState(0)
  const [targetPageSize, setTargetPageSize] = useState(20)
  const targetsQuery = useQuery({
    queryKey: ['activation-queue', 'targets', targetPageIndex, targetPageSize],
    queryFn: () =>
      listActivationQueueTargets({
        page: targetPageIndex + 1,
        pageSize: targetPageSize,
      }),
    enabled: props.open,
    placeholderData: (previous) => previous,
  })
  const targets = targetsQuery.data?.data?.items ?? []

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Automatic activation queue')}
      description={t('Targets')}
      contentClassName='flex max-h-[90vh] max-w-4xl flex-col'
      contentHeight='min(72vh, 760px)'
    >
      <div className='min-h-0 space-y-3 overflow-auto'>
        {targets.map((target) => (
          <div
            key={target.id}
            className='flex items-start justify-between gap-3 rounded-lg border p-3 text-sm'
          >
            <div className='min-w-0'>
              <div className='truncate font-medium'>{target.display_name}</div>
              <div className='text-muted-foreground mt-1 text-xs'>
                {target.source_type} · {target.plan_type || t('Unknown plan')}
              </div>
            </div>
            <Badge variant={target.enabled ? 'outline' : 'secondary'}>
              {target.enabled ? t('Enabled') : t('Disabled')}
            </Badge>
          </div>
        ))}
        {targets.length > 0 && (
          <PaginationControls
            pageIndex={targetPageIndex}
            pageSize={targetPageSize}
            totalCount={targetsQuery.data?.data?.total ?? 0}
            pageCount={Math.ceil(
              (targetsQuery.data?.data?.total ?? 0) / targetPageSize
            )}
            onPageIndexChange={setTargetPageIndex}
            onPageSizeChange={(pageSize) => {
              setTargetPageSize(pageSize)
              setTargetPageIndex(0)
            }}
          />
        )}
      </div>
    </Dialog>
  )
}
