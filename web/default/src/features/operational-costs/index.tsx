import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArchiveRestore, Loader2, Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'

import {
  createCostEntry,
  deleteCostEntry,
  getOperationalCostOverview,
  restoreOperationalAsset,
  updateCostEntry,
} from './api'
import type { OperationalCostEntry } from './types'

function formatMoney(minor: number) {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'CNY',
  }).format(minor / 100)
}

function formatDate(timestamp: number) {
  if (!timestamp) return '—'
  return new Date(timestamp * 1000).toLocaleDateString()
}

function dateInputValue(timestamp?: number) {
  const date = timestamp ? new Date(timestamp * 1000) : new Date()
  return date.toISOString().slice(0, 10)
}

export function OperationalCosts() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState<OperationalCostEntry | null>(null)
  const [title, setTitle] = useState('')
  const [category, setCategory] = useState('')
  const [amount, setAmount] = useState('')
  const [occurredAt, setOccurredAt] = useState(dateInputValue())
  const [note, setNote] = useState('')
  const [saving, setSaving] = useState(false)
  const [workingId, setWorkingId] = useState<number | null>(null)
  const query = useQuery({
    queryKey: ['operational-costs'],
    queryFn: getOperationalCostOverview,
    staleTime: 15_000,
  })

  const resetForm = () => {
    setEditing(null)
    setTitle('')
    setCategory('')
    setAmount('')
    setOccurredAt(dateInputValue())
    setNote('')
  }

  const submit = async () => {
    const parsedAmount = Number(amount)
    if (!title.trim() || !Number.isFinite(parsedAmount)) {
      toast.error(t('Complete the title and amount'))
      return
    }
    setSaving(true)
    try {
      const payload = {
        title: title.trim(),
        category: category.trim() || t('Other'),
        amount_minor: Math.round(parsedAmount * 100),
        occurred_at: Math.floor(
          new Date(`${occurredAt}T00:00:00`).getTime() / 1000
        ),
        note: note.trim(),
      }
      const result = editing
        ? await updateCostEntry(editing.id, payload)
        : await createCostEntry(payload)
      if (!result.success) {
        throw new Error(result.message || t('Failed to save ledger entry'))
      }
      resetForm()
      await queryClient.invalidateQueries({ queryKey: ['operational-costs'] })
      toast.success(t('Ledger entry saved'))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to save ledger entry')
      )
    } finally {
      setSaving(false)
    }
  }

  const startEdit = (entry: OperationalCostEntry) => {
    setEditing(entry)
    setTitle(entry.title)
    setCategory(entry.category)
    setAmount((entry.amount_minor / 100).toFixed(2))
    setOccurredAt(dateInputValue(entry.occurred_at))
    setNote(entry.note)
  }

  const overview = query.data?.data
  const summary = overview?.summary

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Operational Costs')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {query.isLoading ? (
          <div className='flex items-center justify-center gap-2 py-16'>
            <Loader2 className='size-5 animate-spin' />
            {t('Loading operational costs')}
          </div>
        ) : (
          <div className='space-y-4'>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Manage asset acquisition costs, archived assets, and custom operating expenses.'
              )}
            </p>
            <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
              {[
                [t('Total operating cost'), summary?.total_cost || 0],
                [t('Current asset cost'), summary?.current_asset_cost || 0],
                [t('Archived asset cost'), summary?.archived_asset_cost || 0],
                [t('Custom ledger cost'), summary?.custom_cost || 0],
              ].map(([label, value]) => (
                <Card key={String(label)} size='sm'>
                  <CardHeader>
                    <CardDescription>{label}</CardDescription>
                    <CardTitle className='text-xl tabular-nums'>
                      {formatMoney(Number(value))}
                    </CardTitle>
                  </CardHeader>
                </Card>
              ))}
            </div>

            <div className='grid gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(320px,1fr)]'>
              <Card>
                <CardHeader>
                  <CardTitle>{t('Operating ledger')}</CardTitle>
                  <CardDescription>
                    {t(
                      'Custom expenses and adjustments are recorded here. Negative amounts can be used for refunds.'
                    )}
                  </CardDescription>
                </CardHeader>
                <CardContent className='space-y-4'>
                  <div className='grid gap-2 md:grid-cols-5'>
                    <Input
                      value={title}
                      onChange={(event) => setTitle(event.target.value)}
                      placeholder={t('Entry title')}
                    />
                    <Input
                      value={category}
                      onChange={(event) => setCategory(event.target.value)}
                      placeholder={t('Category')}
                    />
                    <Input
                      value={amount}
                      onChange={(event) => setAmount(event.target.value)}
                      inputMode='decimal'
                      placeholder={t('Amount in CNY')}
                    />
                    <Input
                      value={occurredAt}
                      onChange={(event) => setOccurredAt(event.target.value)}
                      type='date'
                    />
                    <div className='flex gap-2'>
                      <Button
                        onClick={submit}
                        disabled={saving}
                        className='flex-1'
                      >
                        {saving ? (
                          <Loader2 className='size-4 animate-spin' />
                        ) : (
                          <Plus className='size-4' />
                        )}
                        {editing ? t('Save changes') : t('Add entry')}
                      </Button>
                      {editing && (
                        <Button variant='outline' onClick={resetForm}>
                          {t('Cancel')}
                        </Button>
                      )}
                    </div>
                    <Input
                      value={note}
                      onChange={(event) => setNote(event.target.value)}
                      placeholder={t('Note')}
                      className='md:col-span-5'
                    />
                  </div>

                  <div className='overflow-x-auto rounded-lg border'>
                    <table className='w-full min-w-[680px] text-sm'>
                      <thead className='bg-muted/50 text-muted-foreground text-left text-xs'>
                        <tr>
                          <th className='p-3'>{t('Date')}</th>
                          <th className='p-3'>{t('Entry')}</th>
                          <th className='p-3'>{t('Category')}</th>
                          <th className='p-3 text-right'>{t('Amount')}</th>
                          <th className='p-3 text-right'>{t('Actions')}</th>
                        </tr>
                      </thead>
                      <tbody className='divide-y'>
                        {(overview?.entries || []).map((entry) => (
                          <tr key={entry.id}>
                            <td className='p-3'>
                              {formatDate(entry.occurred_at)}
                            </td>
                            <td className='p-3'>
                              <div className='font-medium'>{entry.title}</div>
                              {entry.note && (
                                <div className='text-muted-foreground text-xs'>
                                  {entry.note}
                                </div>
                              )}
                            </td>
                            <td className='p-3'>
                              <Badge variant='outline'>{entry.category}</Badge>
                            </td>
                            <td className='p-3 text-right font-medium tabular-nums'>
                              {formatMoney(entry.amount_minor)}
                            </td>
                            <td className='p-3'>
                              <div className='flex justify-end gap-1'>
                                <Button
                                  variant='ghost'
                                  size='icon-sm'
                                  aria-label={t('Edit')}
                                  onClick={() => startEdit(entry)}
                                >
                                  <Pencil className='size-4' />
                                </Button>
                                <Button
                                  variant='ghost'
                                  size='icon-sm'
                                  aria-label={t('Void entry')}
                                  disabled={workingId === entry.id}
                                  onClick={async () => {
                                    if (
                                      !window.confirm(
                                        t('Void this ledger entry?')
                                      )
                                    ) {
                                      return
                                    }
                                    setWorkingId(entry.id)
                                    try {
                                      const result = await deleteCostEntry(
                                        entry.id
                                      )
                                      if (!result.success) {
                                        throw new Error(
                                          result.message ||
                                            t('Failed to void entry')
                                        )
                                      }
                                      await queryClient.invalidateQueries({
                                        queryKey: ['operational-costs'],
                                      })
                                    } catch (error) {
                                      toast.error(
                                        error instanceof Error
                                          ? error.message
                                          : t('Failed to void entry')
                                      )
                                    } finally {
                                      setWorkingId(null)
                                    }
                                  }}
                                >
                                  <Trash2 className='size-4' />
                                </Button>
                              </div>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>{t('Archived assets')}</CardTitle>
                  <CardDescription>
                    {t(
                      'Archived assets no longer participate in polling, routing, or activation tasks.'
                    )}
                  </CardDescription>
                  <CardAction>
                    <Badge variant='secondary'>
                      {summary?.archived_assets || 0}
                    </Badge>
                  </CardAction>
                </CardHeader>
                <CardContent className='space-y-2'>
                  {(overview?.archives || []).length === 0 ? (
                    <div className='text-muted-foreground py-8 text-center text-sm'>
                      {t('No archived assets')}
                    </div>
                  ) : (
                    (overview?.archives || []).map((asset) => (
                      <div
                        key={asset.id}
                        className='flex items-center justify-between gap-3 rounded-lg border p-3'
                      >
                        <div className='min-w-0'>
                          <div className='truncate font-medium'>
                            {asset.display_name}
                          </div>
                          <div className='text-muted-foreground mt-1 text-xs'>
                            {asset.source_type === 'cpa_account'
                              ? 'CPA'
                              : t('Channel')}{' '}
                            · {formatMoney(asset.cost_minor)} ·{' '}
                            {formatDate(asset.archived_at)}
                          </div>
                          {asset.archive_reason && (
                            <div className='text-muted-foreground mt-1 truncate text-xs'>
                              {asset.archive_reason}
                            </div>
                          )}
                        </div>
                        <Button
                          variant='outline'
                          size='sm'
                          disabled={workingId === asset.id}
                          onClick={async () => {
                            setWorkingId(asset.id)
                            try {
                              const result = await restoreOperationalAsset(
                                asset.id
                              )
                              if (!result.success) {
                                throw new Error(
                                  result.message || t('Failed to restore asset')
                                )
                              }
                              await queryClient.invalidateQueries({
                                queryKey: ['operational-costs'],
                              })
                              toast.success(
                                t('Asset restored in disabled state')
                              )
                            } catch (error) {
                              toast.error(
                                error instanceof Error
                                  ? error.message
                                  : t('Failed to restore asset')
                              )
                            } finally {
                              setWorkingId(null)
                            }
                          }}
                        >
                          <ArchiveRestore className='size-4' />
                          {t('Restore')}
                        </Button>
                      </div>
                    ))
                  )}
                </CardContent>
              </Card>
            </div>
          </div>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
