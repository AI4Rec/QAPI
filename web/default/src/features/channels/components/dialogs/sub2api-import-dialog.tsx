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
import { Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldTitle } from '@/components/ui/field'
import { Textarea } from '@/components/ui/textarea'

import { importSub2APIAccounts } from '../../api'

type Sub2APIImportDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function Sub2APIImportDialog(props: Sub2APIImportDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [content, setContent] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleOpenChange = (open: boolean) => {
    if (!open) setContent('')
    props.onOpenChange(open)
  }

  const submit = async () => {
    if (!content.trim()) {
      toast.error(t('Paste account JSON'))
      return
    }
    setSubmitting(true)
    try {
      const result = await importSub2APIAccounts(content)
      if (!result.success) {
        throw new Error(result.message || t('Sub2API stock upload failed'))
      }
      toast.success(result.message || t('Sub2API stock upload succeeded'))
      handleOpenChange(false)
      await queryClient.invalidateQueries({ queryKey: ['sub2api-accounts'] })
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Sub2API stock upload failed')
      )
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Sub2API stock upload')}
      description={t('Sub2API account pool')}
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
          <FieldTitle>{t('Account JSON')}</FieldTitle>
          <Textarea
            id='sub2api-import-json'
            value={content}
            onChange={(event) => setContent(event.target.value)}
            placeholder={t('Paste Sub2API, CPA, or Codex account JSON')}
            className='min-h-[360px] font-mono text-xs'
            spellCheck={false}
            aria-label={t('Account JSON')}
          />
        </Field>
      </FieldGroup>
    </Dialog>
  )
}
