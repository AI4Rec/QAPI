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
import { Activity, CheckCircle2, Loader2, Play, XCircle } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

import {
  getAccountInspectionTargets,
  inspectAccount,
  type AccountInspectionOperation,
  type AccountInspectionProvider,
  type AccountInspectionResult,
  type AccountInspectionTarget,
} from '../../api'

type AccountInspectorDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  initialProvider?: Exclude<AccountInspectionProvider, 'auto'>
  initialIdentifier?: string
  initialOperation?: AccountInspectionOperation
}

export function AccountInspectorDialog(props: AccountInspectorDialogProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<'pool' | 'json'>('pool')
  const [provider, setProvider] = useState<AccountInspectionProvider>('auto')
  const [scope, setScope] = useState<'attention' | 'all'>('attention')
  const [operation, setOperation] =
    useState<AccountInspectionOperation>('verify')
  const [identifier, setIdentifier] = useState('')
  const [content, setContent] = useState('')
  const [results, setResults] = useState<AccountInspectionResult[]>([])
  const [running, setRunning] = useState(false)

  useEffect(() => {
    if (!props.open) return
    setMode('pool')
    setProvider(props.initialProvider ?? 'auto')
    setIdentifier(props.initialIdentifier ?? '')
    setScope(props.initialIdentifier ? 'all' : 'attention')
    setOperation(props.initialOperation ?? 'verify')
    setResults([])
  }, [
    props.initialIdentifier,
    props.initialOperation,
    props.initialProvider,
    props.open,
  ])

  const targetsQuery = useQuery({
    queryKey: ['account-inspection-targets', provider, scope],
    queryFn: () =>
      getAccountInspectionTargets({
        provider: provider === 'auto' ? 'all' : provider,
        scope,
      }),
    enabled: props.open && mode === 'pool',
    staleTime: 15_000,
  })
  const targets = targetsQuery.data?.data?.items ?? []
  const providerItems = [
    { value: 'auto' as const, label: t('Auto detect') },
    { value: 'cpa' as const, label: 'CPA' },
    { value: 'sub2api' as const, label: 'Sub2API' },
  ]
  const scopeItems = [
    {
      value: 'attention' as const,
      label: t('Frozen, unavailable and archived'),
    },
    { value: 'all' as const, label: t('All accounts') },
  ]
  const targetItems = targets.map((target) => ({
    value: target.identifier,
    label: `${target.email || target.identifier} · ${target.provider} · ${t(target.state)}`,
  }))
  const selectedTarget = targets.find(
    (target) => target.identifier === identifier
  )

  const runTarget = async (target: AccountInspectionTarget) => {
    const response = await inspectAccount({
      provider: target.provider,
      operation,
      identifier: target.identifier,
    })
    if (!response.success || !response.data) {
      throw new Error(response.message || t('Account inspection failed'))
    }
    return response.data
  }

  const runSelected = async () => {
    setRunning(true)
    try {
      if (mode === 'json') {
        const response = await inspectAccount({
          provider,
          operation,
          content: content.trim(),
        })
        if (!response.success || !response.data) {
          throw new Error(response.message || t('Account inspection failed'))
        }
        setResults([response.data])
        return
      }
      if (!selectedTarget) throw new Error(t('Select an account'))
      setResults([await runTarget(selectedTarget)])
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Account inspection failed')
      )
    } finally {
      setRunning(false)
    }
  }

  const runAll = async () => {
    setRunning(true)
    setResults([])
    try {
      const nextResults: AccountInspectionResult[] = []
      for (let index = 0; index < targets.length; index += 4) {
        const batch = targets.slice(index, index + 4)
        const settled = await Promise.allSettled(batch.map(runTarget))
        settled.forEach((item, itemIndex) => {
          if (item.status === 'fulfilled') {
            nextResults.push(item.value)
            return
          }
          const target = batch[itemIndex]
          nextResults.push({
            ...target,
            operation,
            success: false,
            latency_ms: 0,
            message:
              item.reason instanceof Error
                ? item.reason.message
                : t('Account inspection failed'),
          })
        })
        setResults([...nextResults])
      }
    } finally {
      setRunning(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Account inspector')}
      description={t(
        'Verify reads account status and quota. Ping sends one minimal model request.'
      )}
      contentClassName='sm:max-w-5xl'
      contentHeight='min(68vh, 680px)'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={running}
          >
            {t('Close')}
          </Button>
          {mode === 'pool' && (
            <Button
              variant='outline'
              onClick={runAll}
              disabled={running || targets.length === 0}
            >
              <Activity data-icon='inline-start' />
              {t('Run all shown')}
            </Button>
          )}
          <Button
            onClick={runSelected}
            disabled={
              running || (mode === 'json' ? !content.trim() : !selectedTarget)
            }
          >
            {running ? (
              <Loader2 data-icon='inline-start' className='animate-spin' />
            ) : (
              <Play data-icon='inline-start' />
            )}
            {operation === 'verify' ? t('Verify account') : t('Ping account')}
          </Button>
        </>
      }
    >
      <div className='space-y-4'>
        <div className='flex flex-wrap items-center gap-3'>
          <Tabs
            value={mode}
            onValueChange={(value) => setMode(value as typeof mode)}
          >
            <TabsList>
              <TabsTrigger value='pool'>{t('Account pool')}</TabsTrigger>
              <TabsTrigger value='json'>{t('JSON')}</TabsTrigger>
            </TabsList>
          </Tabs>
          <Tabs
            value={operation}
            onValueChange={(value) =>
              setOperation(value as AccountInspectionOperation)
            }
          >
            <TabsList>
              <TabsTrigger value='verify'>{t('Verify status')}</TabsTrigger>
              <TabsTrigger value='ping'>{t('Ping')}</TabsTrigger>
            </TabsList>
          </Tabs>
        </div>

        <div className='grid gap-3 sm:grid-cols-2'>
          <Select<AccountInspectionProvider>
            items={providerItems}
            value={provider}
            onValueChange={(value) => value && setProvider(value)}
          >
            <SelectTrigger>
              <SelectValue placeholder={t('Provider')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='auto'>{t('Auto detect')}</SelectItem>
                <SelectItem value='cpa'>CPA</SelectItem>
                <SelectItem value='sub2api'>Sub2API</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
          {mode === 'pool' && (
            <Select<'attention' | 'all'>
              items={scopeItems}
              value={scope}
              onValueChange={(value) => value && setScope(value)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value='attention'>
                    {t('Frozen, unavailable and archived')}
                  </SelectItem>
                  <SelectItem value='all'>{t('All accounts')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          )}
        </div>

        {mode === 'pool' ? (
          <Select<string>
            items={targetItems}
            value={identifier || null}
            onValueChange={(value) => setIdentifier(value ?? '')}
          >
            <SelectTrigger>
              <SelectValue
                placeholder={
                  targetsQuery.isFetching
                    ? t('Loading accounts...')
                    : t('Select an account')
                }
              />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {targets.map((target) => (
                  <SelectItem
                    key={`${target.provider}:${target.identifier}`}
                    value={target.identifier}
                  >
                    {target.email || target.identifier} · {target.provider} ·{' '}
                    {t(target.state)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        ) : (
          <Textarea
            value={content}
            onChange={(event) => setContent(event.target.value)}
            rows={10}
            className='font-mono text-xs'
            placeholder={t('Paste one CPA or Sub2API account JSON')}
          />
        )}

        {results.length > 0 && (
          <div className='overflow-hidden rounded-lg border'>
            <div className='overflow-x-auto'>
              <table className='w-full min-w-[760px] text-left text-sm'>
                <thead className='bg-muted/50 text-muted-foreground text-xs'>
                  <tr>
                    <th className='px-3 py-2'>{t('Account')}</th>
                    <th className='px-3 py-2'>{t('Provider')}</th>
                    <th className='px-3 py-2'>{t('State')}</th>
                    <th className='px-3 py-2'>{t('HTTP Status')}</th>
                    <th className='px-3 py-2'>{t('Latency')}</th>
                    <th className='px-3 py-2'>{t('Result')}</th>
                  </tr>
                </thead>
                <tbody className='divide-y'>
                  {results.map((result) => (
                    <tr key={`${result.provider}:${result.identifier}`}>
                      <td className='max-w-64 px-3 py-2'>
                        <div className='truncate font-medium'>
                          {result.email || result.identifier}
                        </div>
                      </td>
                      <td className='px-3 py-2'>{result.provider}</td>
                      <td className='px-3 py-2'>
                        <Badge
                          variant={result.success ? 'outline' : 'destructive'}
                        >
                          {t(result.state)}
                        </Badge>
                      </td>
                      <td className='px-3 py-2 tabular-nums'>
                        {result.upstream_status || '-'}
                      </td>
                      <td className='px-3 py-2 tabular-nums'>
                        {result.latency_ms} ms
                      </td>
                      <td className='max-w-80 px-3 py-2'>
                        <div className='flex items-center gap-2'>
                          {result.success ? (
                            <CheckCircle2 className='size-4 shrink-0 text-emerald-600' />
                          ) : (
                            <XCircle className='text-destructive size-4 shrink-0' />
                          )}
                          <span className='truncate'>
                            {result.message ||
                              (result.success ? t('Available') : t('Failed'))}
                          </span>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  )
}
