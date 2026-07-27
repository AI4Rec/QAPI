import { CheckCheck, ListChecks, Loader2, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'

type AccountSelectionToolbarProps = {
  selectedCount: number
  totalCount: number
  selectingAll: boolean
  selectAllLabel?: string
  onSelectAll: () => void
  onClear: () => void
  onManage: () => void
}

export function AccountSelectionToolbar(props: AccountSelectionToolbarProps) {
  const { t } = useTranslation()
  if (props.selectedCount === 0) return null

  return (
    <div className='bg-primary/5 flex flex-wrap items-center gap-2 border-b px-4 py-2.5'>
      <Badge variant='secondary'>
        {t('{{count}} selected', { count: props.selectedCount })}
      </Badge>
      {props.selectedCount < props.totalCount && (
        <Button
          variant='ghost'
          size='sm'
          onClick={props.onSelectAll}
          disabled={props.selectingAll}
        >
          {props.selectingAll ? (
            <Loader2 data-icon='inline-start' className='animate-spin' />
          ) : (
            <CheckCheck data-icon='inline-start' />
          )}
          {props.selectAllLabel ??
            t('Select all {{count}} accounts in this pool', {
              count: props.totalCount,
            })}
        </Button>
      )}
      <Separator orientation='vertical' className='mx-1 hidden h-5 sm:block' />
      <Button variant='ghost' size='sm' onClick={props.onClear}>
        <X data-icon='inline-start' />
        {t('Clear selection')}
      </Button>
      <Button size='sm' className='sm:ml-auto' onClick={props.onManage}>
        <ListChecks data-icon='inline-start' />
        {t('Manage selected accounts')}
      </Button>
    </div>
  )
}
