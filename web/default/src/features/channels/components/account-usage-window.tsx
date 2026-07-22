import { useTranslation } from 'react-i18next'

import { Progress } from '@/components/ui/progress'

type AccountUsageWindowValue = {
  used_percent?: number
  reset_at?: number
}

type AccountUsageWindowProps = {
  value?: AccountUsageWindowValue
}

export function AccountUsageWindow(props: AccountUsageWindowProps) {
  const { t } = useTranslation()
  const percent = Math.max(0, Math.min(100, props.value?.used_percent ?? 0))
  const resetAt = props.value?.reset_at

  return (
    <div className='min-w-[150px] space-y-1'>
      <div className='flex justify-between text-[11px]'>
        <span>{t('Used')}</span>
        <span className={percent >= 90 ? 'text-destructive font-semibold' : ''}>
          {percent}%
        </span>
      </div>
      <Progress value={percent} className='h-1.5' />
      <div className='text-muted-foreground text-[10px]'>
        {t('Reset')}:{' '}
        {resetAt ? new Date(resetAt * 1000).toLocaleString() : '-'}
      </div>
    </div>
  )
}
