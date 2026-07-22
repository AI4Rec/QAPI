import { useQueryClient } from '@tanstack/react-query'
import { Loader2, TimerReset, UsersRound } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldTitle,
} from '@/components/ui/field'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

import { importCPAAccounts, type CPAAccountPoolType } from '../../api'

type CPAImportDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function CPAImportDialog(props: CPAImportDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [content, setContent] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [poolType, setPoolType] =
    useState<Extract<CPAAccountPoolType, 'cpa_import' | 'temporary'>>(
      'cpa_import'
    )
  const isTemporary = poolType === 'temporary'

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      setContent('')
      setPoolType('cpa_import')
    }
    props.onOpenChange(open)
  }

  const submit = async () => {
    if (!content.trim()) {
      toast.error(t('Paste account JSON'))
      return
    }
    setSubmitting(true)
    try {
      const result = await importCPAAccounts(content, poolType)
      if (result.success) {
        toast.success(result.message || t('CPA stock upload succeeded'))
        setContent('')
        handleOpenChange(false)
        await queryClient.invalidateQueries({ queryKey: ['cpa-accounts'] })
      } else {
        toast.error(result.message || t('CPA stock upload failed'))
      }
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('CPA stock upload failed')
      )
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('CPA stock upload')}
      description={t(
        'Paste one account JSON, a JSON array, or multiple consecutive JSON objects. The system automatically splits them and completes CPA fields.'
      )}
      contentClassName='sm:max-w-3xl'
      footer={
        <>
          <Button variant='outline' onClick={() => handleOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={submit} disabled={submitting || !content.trim()}>
            {submitting && (
              <Loader2 data-icon='inline-start' className='animate-spin' />
            )}
            {t('Start upload')}
          </Button>
        </>
      }
    >
      <FieldGroup>
        <Field>
          <FieldTitle>{t('Destination account pool')}</FieldTitle>
          <ToggleGroup
            value={[poolType]}
            onValueChange={(value) => {
              const nextValue = value.find((item) => item !== poolType)
              if (nextValue === 'cpa_import' || nextValue === 'temporary') {
                setPoolType(nextValue)
              }
            }}
            variant='outline'
            spacing={2}
            className='grid w-full grid-cols-2 gap-2'
            aria-label={t('Destination account pool')}
          >
            <ToggleGroupItem
              value='cpa_import'
              className='h-auto min-h-20 w-full flex-col items-start gap-1 px-3 py-3 text-left'
            >
              <span className='flex items-center gap-2 font-medium'>
                <UsersRound aria-hidden='true' />
                {t('CPA account pool')}
              </span>
              <span className='text-muted-foreground text-xs font-normal'>
                {t('Accounts imported through CPA stock upload.')}
              </span>
            </ToggleGroupItem>
            <ToggleGroupItem
              value='temporary'
              className='h-auto min-h-20 w-full flex-col items-start gap-1 px-3 py-3 text-left'
            >
              <span className='flex items-center gap-2 font-medium'>
                <TimerReset aria-hidden='true' />
                {t('Temporary account pool')}
              </span>
              <span className='text-muted-foreground text-xs font-normal'>
                {t('Accounts uploaded for temporary use.')}
              </span>
            </ToggleGroupItem>
          </ToggleGroup>
          <FieldDescription>
            {isTemporary
              ? t(
                  'Uploaded accounts will be assigned to the temporary account pool.'
                )
              : t(
                  'Uploaded accounts will be assigned to the CPA account pool.'
                )}
          </FieldDescription>
        </Field>
        <Field>
          <FieldTitle>{t('Account JSON')}</FieldTitle>
          <Textarea
            id='cpa-import-json'
            value={content}
            onChange={(event) => setContent(event.target.value)}
            placeholder={t(
              'Paste account JSON containing access_token; use a JSON array for multiple accounts'
            )}
            className='min-h-[320px] font-mono text-xs'
            spellCheck={false}
            aria-label={t('Account JSON')}
          />
          <FieldDescription>
            {t(
              'Only access_token is required. email, account_id, expiration time, and id_token are completed automatically. Credentials are sent only to local CPA and are not stored in the database.'
            )}
          </FieldDescription>
        </Field>
      </FieldGroup>
    </Dialog>
  )
}
