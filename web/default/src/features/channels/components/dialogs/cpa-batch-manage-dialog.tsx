import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { AlertTriangle, Loader2 } from 'lucide-react'
import { useEffect } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldError,
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

import {
  batchManageCPAAccounts,
  type CPAAccountBatchAction,
  type CPAAccountPoolType,
} from '../../api'
import {
  cpaBatchFormSchema,
  type CPABatchFormValues,
} from '../../lib/cpa-batch-form'

type CPABatchManageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  poolType: CPAAccountPoolType
  names: string[]
  onComplete: (failedNames: string[]) => void
}

function getCPABatchDefaultValues(
  poolType: CPAAccountPoolType
): CPABatchFormValues {
  return {
    action: 'disable',
    poolType: poolType === 'temporary' ? 'cpa_import' : 'temporary',
    cost: '0.00',
    costDate: dayjs().format('YYYY-MM-DD'),
    costNote: '',
    archiveReason: '',
    deleteConfirmation: '',
  }
}

export function CPABatchManageDialog(props: CPABatchManageDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const form = useForm<CPABatchFormValues>({
    resolver: zodResolver(cpaBatchFormSchema),
    defaultValues: getCPABatchDefaultValues(props.poolType),
  })
  const action = form.watch('action')
  const deleteConfirmation = form.watch('deleteConfirmation')
  const submitting = form.formState.isSubmitting
  const expectedDeleteConfirmation = `DELETE ${props.names.length}`
  const deleteConfirmed =
    action !== 'delete' ||
    deleteConfirmation.trim() === expectedDeleteConfirmation
  const actionItems = [
    { value: 'disable' as const, label: t('Freeze selected accounts') },
    { value: 'enable' as const, label: t('Unfreeze selected accounts') },
    { value: 'move' as const, label: t('Move selected accounts') },
    { value: 'cost' as const, label: t('Set cost for selected accounts') },
    { value: 'archive' as const, label: t('Archive selected accounts') },
    { value: 'delete' as const, label: t('Delete selected accounts') },
  ]
  const poolItems = [
    { value: 'cpa_import' as const, label: t('CPA account pool') },
    { value: 'temporary' as const, label: t('Temporary account pool') },
  ]

  useEffect(() => {
    if (props.open) form.reset(getCPABatchDefaultValues(props.poolType))
  }, [form, props.open, props.poolType])

  const handleOpenChange = (open: boolean) => {
    if (!open && !submitting) {
      form.reset(getCPABatchDefaultValues(props.poolType))
    }
    if (!submitting) props.onOpenChange(open)
  }

  const submit = form.handleSubmit(async (values) => {
    const request: Parameters<typeof batchManageCPAAccounts>[0] = {
      names: props.names,
      action: values.action as CPAAccountBatchAction,
    }
    if (values.action === 'move') {
      request.pool_type = values.poolType
    }
    if (values.action === 'cost') {
      request.cost_minor = Math.round(Number(values.cost) * 100)
      request.cost_date = dayjs(values.costDate).startOf('day').unix()
      request.cost_note = values.costNote.trim()
    }
    if (values.action === 'archive') {
      request.archive_reason = values.archiveReason.trim()
    }
    if (values.action === 'delete') {
      request.delete_confirmation = values.deleteConfirmation.trim()
    }

    try {
      const result = await batchManageCPAAccounts(request)
      const failed = result.data?.failed ?? []
      const succeeded = result.data?.succeeded ?? []
      if (!result.data) {
        throw new Error(result.message || t('Batch account management failed'))
      }
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
        queryClient.invalidateQueries({ queryKey: ['cpa-accounts'] }),
        queryClient.invalidateQueries({ queryKey: ['operational-costs'] }),
      ])
      props.onComplete(failed.map((item) => item.name))
      if (failed.length === 0) handleOpenChange(false)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Batch account management failed')
      )
    }
  })

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Batch manage CPA accounts')}
      description={t('Manage {{count}} selected account(s).', {
        count: props.names.length,
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
            form='cpa-batch-manage-form'
            type='submit'
            variant={action === 'delete' ? 'destructive' : 'default'}
            disabled={
              submitting || props.names.length === 0 || !deleteConfirmed
            }
          >
            {submitting && <Loader2 data-icon='inline-start' />}
            {t('Apply to selected accounts')}
          </Button>
        </>
      }
    >
      <form id='cpa-batch-manage-form' onSubmit={submit}>
        <FieldGroup>
          <Field>
            <FieldLabel>{t('Batch action')}</FieldLabel>
            <Controller
              control={form.control}
              name='action'
              render={({ field }) => (
                <Select<CPAAccountBatchAction>
                  items={actionItems}
                  value={field.value}
                  onValueChange={(value) =>
                    value !== null && field.onChange(value)
                  }
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
              )}
            />
            <FieldDescription>
              {t('Only the selected accounts in this pool will be changed.')}
            </FieldDescription>
          </Field>

          {action === 'move' && (
            <Field>
              <FieldLabel>{t('Destination account pool')}</FieldLabel>
              <Controller
                control={form.control}
                name='poolType'
                render={({ field }) => (
                  <Select
                    items={poolItems}
                    value={field.value}
                    onValueChange={(value) =>
                      value !== null && field.onChange(value)
                    }
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {poolItems.map((item) => (
                          <SelectItem key={item.value} value={item.value}>
                            {item.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                )}
              />
            </Field>
          )}

          {action === 'cost' && (
            <div className='grid gap-4 sm:grid-cols-2'>
              <Field data-invalid={!!form.formState.errors.cost}>
                <FieldLabel htmlFor='cpa-batch-cost'>{t('Cost')}</FieldLabel>
                <Input
                  id='cpa-batch-cost'
                  type='number'
                  min='0'
                  step='0.01'
                  inputMode='decimal'
                  aria-invalid={!!form.formState.errors.cost}
                  {...form.register('cost')}
                />
                <FieldError
                  errors={
                    form.formState.errors.cost
                      ? [{ message: t('Enter a valid non-negative cost') }]
                      : undefined
                  }
                />
              </Field>
              <Field data-invalid={!!form.formState.errors.costDate}>
                <FieldLabel htmlFor='cpa-batch-cost-date'>
                  {t('Cost date')}
                </FieldLabel>
                <Input
                  id='cpa-batch-cost-date'
                  type='date'
                  aria-invalid={!!form.formState.errors.costDate}
                  {...form.register('costDate')}
                />
                <FieldError
                  errors={
                    form.formState.errors.costDate
                      ? [
                          {
                            message: t('Select a cost date'),
                          },
                        ]
                      : undefined
                  }
                />
              </Field>
              <Field className='sm:col-span-2'>
                <FieldLabel htmlFor='cpa-batch-cost-note'>
                  {t('Cost note')}
                </FieldLabel>
                <Textarea
                  id='cpa-batch-cost-note'
                  maxLength={500}
                  {...form.register('costNote')}
                />
              </Field>
            </div>
          )}

          {action === 'archive' && (
            <Field>
              <FieldLabel htmlFor='cpa-batch-archive-reason'>
                {t('Archive reason')}
              </FieldLabel>
              <Textarea
                id='cpa-batch-archive-reason'
                maxLength={500}
                {...form.register('archiveReason')}
              />
              <FieldDescription>
                {t(
                  'Archived accounts are frozen and removed from active pools.'
                )}
              </FieldDescription>
            </Field>
          )}

          {action === 'delete' && (
            <>
              <Alert variant='destructive'>
                <AlertTriangle aria-hidden='true' />
                <AlertTitle>{t('This action cannot be undone')}</AlertTitle>
                <AlertDescription>
                  {t(
                    'The selected CPA credential files will be permanently deleted.'
                  )}
                </AlertDescription>
              </Alert>
              <Field>
                <FieldLabel htmlFor='cpa-batch-delete-confirmation'>
                  {t('Permanent delete confirmation')}
                </FieldLabel>
                <Input
                  id='cpa-batch-delete-confirmation'
                  autoComplete='off'
                  spellCheck={false}
                  placeholder={expectedDeleteConfirmation}
                  {...form.register('deleteConfirmation')}
                />
                <FieldDescription>
                  {t(
                    'Type {{confirmation}} to permanently delete {{count}} account(s).',
                    {
                      confirmation: expectedDeleteConfirmation,
                      count: props.names.length,
                    }
                  )}
                </FieldDescription>
              </Field>
            </>
          )}
        </FieldGroup>
      </form>
    </Dialog>
  )
}
