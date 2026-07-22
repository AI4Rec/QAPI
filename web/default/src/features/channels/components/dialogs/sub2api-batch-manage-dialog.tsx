import { useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { AlertTriangle, Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Alert, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

import { batchManageSub2APIAccounts, type Sub2APIBatchAction } from '../../api'

type Sub2APIBatchManageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  accountIDs: number[]
  onComplete: (failedIDs: number[]) => void
}

export function Sub2APIBatchManageDialog(props: Sub2APIBatchManageDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [action, setAction] = useState<Sub2APIBatchAction>('disable')
  const [cost, setCost] = useState('0.00')
  const [costDate, setCostDate] = useState(dayjs().format('YYYY-MM-DD'))
  const [costNote, setCostNote] = useState('')
  const [archiveReason, setArchiveReason] = useState('')
  const [deleteConfirmation, setDeleteConfirmation] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const expectedDeleteConfirmation = `DELETE ${props.accountIDs.length}`
  const deleteConfirmed =
    action !== 'delete' ||
    deleteConfirmation.trim() === expectedDeleteConfirmation
  const actionItems = [
    { value: 'disable' as const, label: t('Freeze selected accounts') },
    { value: 'enable' as const, label: t('Unfreeze selected accounts') },
    { value: 'cost' as const, label: t('Set cost for selected accounts') },
    { value: 'archive' as const, label: t('Archive selected accounts') },
    { value: 'delete' as const, label: t('Delete selected accounts') },
  ]

  useEffect(() => {
    if (!props.open) return
    setAction('disable')
    setCost('0.00')
    setCostDate(dayjs().format('YYYY-MM-DD'))
    setCostNote('')
    setArchiveReason('')
    setDeleteConfirmation('')
  }, [props.open])

  const handleOpenChange = (open: boolean) => {
    if (!submitting) props.onOpenChange(open)
  }

  const submit = async () => {
    setSubmitting(true)
    try {
      let succeeded: number[] = []
      let failed: number[] = []
      const parsedCost = Number(cost)
      if (
        action === 'cost' &&
        (!Number.isFinite(parsedCost) || parsedCost < 0)
      ) {
        throw new Error(t('Enter a valid non-negative cost'))
      }
      const result = await batchManageSub2APIAccounts({
        account_ids: props.accountIDs,
        action,
        cost_minor:
          action === 'cost' ? Math.round(parsedCost * 100) : undefined,
        cost_date:
          action === 'cost' ? dayjs(costDate).startOf('day').unix() : undefined,
        cost_note: action === 'cost' ? costNote.trim() : undefined,
        archive_reason: action === 'archive' ? archiveReason.trim() : undefined,
      })
      if (!result.success || !result.data) {
        throw new Error(result.message || t('Batch account management failed'))
      }
      succeeded = result.data.succeeded
      failed = result.data.failed.map((item) => item.id)

      if (succeeded.length > 0) {
        toast.success(
          t('{{count}} account(s) managed successfully', {
            count: succeeded.length,
          })
        )
      }
      if (failed.length > 0) {
        toast.error(
          t('{{count}} account(s) failed and remain selected', {
            count: failed.length,
          })
        )
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['sub2api-accounts'] }),
        queryClient.invalidateQueries({ queryKey: ['operational-costs'] }),
      ])
      props.onComplete(failed)
      if (failed.length === 0) handleOpenChange(false)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Batch account management failed')
      )
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Sub2API account pool')}
      description={t('Manage {{count}} selected account(s).', {
        count: props.accountIDs.length,
      })}
      contentClassName='sm:max-w-xl'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => handleOpenChange(false)}
            disabled={submitting}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            variant={action === 'delete' ? 'destructive' : 'default'}
            disabled={
              submitting || props.accountIDs.length === 0 || !deleteConfirmed
            }
            onClick={submit}
          >
            {submitting && (
              <Loader2 data-icon='inline-start' className='animate-spin' />
            )}
            {t('Apply to selected accounts')}
          </Button>
        </>
      }
    >
      <FieldGroup>
        <Field>
          <FieldLabel>{t('Batch action')}</FieldLabel>
          <Select<Sub2APIBatchAction>
            items={actionItems}
            value={action}
            onValueChange={(value) => value !== null && setAction(value)}
          >
            <SelectTrigger className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {actionItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <FieldDescription>
            {t('Only the selected accounts in this pool will be changed.')}
          </FieldDescription>
        </Field>

        {action === 'cost' && (
          <div className='grid gap-4 sm:grid-cols-2'>
            <Field>
              <FieldLabel htmlFor='sub2api-batch-cost'>{t('Cost')}</FieldLabel>
              <Input
                id='sub2api-batch-cost'
                type='number'
                min='0'
                step='0.01'
                inputMode='decimal'
                value={cost}
                onChange={(event) => setCost(event.target.value)}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor='sub2api-batch-cost-date'>
                {t('Cost date')}
              </FieldLabel>
              <Input
                id='sub2api-batch-cost-date'
                type='date'
                value={costDate}
                onChange={(event) => setCostDate(event.target.value)}
              />
            </Field>
            <Field className='sm:col-span-2'>
              <FieldLabel htmlFor='sub2api-batch-cost-note'>
                {t('Cost note')}
              </FieldLabel>
              <Textarea
                id='sub2api-batch-cost-note'
                maxLength={500}
                value={costNote}
                onChange={(event) => setCostNote(event.target.value)}
              />
            </Field>
          </div>
        )}
        {action === 'archive' && (
          <Field>
            <FieldLabel>{t('Archive reason')}</FieldLabel>
            <Textarea
              value={archiveReason}
              onChange={(event) => setArchiveReason(event.target.value)}
              placeholder={t('Optional archive reason')}
              rows={3}
            />
          </Field>
        )}

        {action === 'delete' && (
          <>
            <Alert variant='destructive'>
              <AlertTriangle aria-hidden='true' />
              <AlertTitle>{t('This action cannot be undone')}</AlertTitle>
            </Alert>
            <Field>
              <FieldLabel htmlFor='sub2api-batch-delete-confirmation'>
                {t('Permanent delete confirmation')}
              </FieldLabel>
              <Input
                id='sub2api-batch-delete-confirmation'
                autoComplete='off'
                spellCheck={false}
                placeholder={expectedDeleteConfirmation}
                value={deleteConfirmation}
                onChange={(event) => setDeleteConfirmation(event.target.value)}
              />
              <FieldDescription>
                {t(
                  'Type {{confirmation}} to permanently delete {{count}} account(s).',
                  {
                    confirmation: expectedDeleteConfirmation,
                    count: props.accountIDs.length,
                  }
                )}
              </FieldDescription>
            </Field>
          </>
        )}
      </FieldGroup>
    </Dialog>
  )
}
