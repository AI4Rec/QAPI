import { useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Input } from '@/components/ui/input'

import { saveAssetCost } from '../api'
import type { AssetCostPayload } from '../types'

type AssetCostInputProps = Omit<AssetCostPayload, 'cost_minor'> & {
  costMinor: number
  disabled?: boolean
}

export function AssetCostInput(props: AssetCostInputProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [value, setValue] = useState((props.costMinor / 100).toFixed(2))
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setValue((props.costMinor / 100).toFixed(2))
  }, [props.costMinor])

  const save = async () => {
    const parsed = Number(value)
    if (!Number.isFinite(parsed) || parsed < 0) {
      setValue((props.costMinor / 100).toFixed(2))
      toast.error(t('Enter a valid non-negative cost'))
      return
    }
    const nextMinor = Math.round(parsed * 100)
    if (nextMinor === props.costMinor) return
    setSaving(true)
    try {
      const result = await saveAssetCost({
        source_type: props.source_type,
        source_key: props.source_key,
        source_id: props.source_id,
        source_ref: props.source_ref,
        display_name: props.display_name,
        cost_minor: nextMinor,
        cost_date: props.cost_date,
        note: props.note,
      })
      if (!result.success) {
        throw new Error(result.message || t('Failed to save cost'))
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['operational-costs'] }),
        queryClient.invalidateQueries({ queryKey: ['channels'] }),
        queryClient.invalidateQueries({ queryKey: ['cpa-accounts'] }),
      ])
      toast.success(t('Cost saved'))
    } catch (error) {
      setValue((props.costMinor / 100).toFixed(2))
      toast.error(
        error instanceof Error ? error.message : t('Failed to save cost')
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className='relative w-28'>
      <span className='text-muted-foreground pointer-events-none absolute top-1/2 left-2 -translate-y-1/2 text-xs'>
        ¥
      </span>
      <Input
        value={value}
        inputMode='decimal'
        className='pr-7 pl-6 text-right tabular-nums'
        disabled={props.disabled || saving}
        aria-label={t('Operational cost')}
        onChange={(event) => setValue(event.target.value)}
        onBlur={save}
        onKeyDown={(event) => {
          if (event.key === 'Enter') event.currentTarget.blur()
          if (event.key === 'Escape') {
            setValue((props.costMinor / 100).toFixed(2))
            event.currentTarget.blur()
          }
        }}
      />
      {saving && (
        <Loader2 className='text-muted-foreground absolute top-1/2 right-2 size-3 -translate-y-1/2 animate-spin' />
      )}
    </div>
  )
}
