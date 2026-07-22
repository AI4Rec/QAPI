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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import {
  ChartNoAxesColumn,
  Copy,
  KeyRound,
  ReceiptText,
  Settings2,
  WalletCards,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DateTimePicker } from '@/components/datetime-picker'
import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { MultiSelect } from '@/components/multi-select'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { getUserGroups, getUserModels } from '@/lib/api'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { getCurrencyLabel } from '@/lib/currency'
import { formatDateTimeStr, formatQuota } from '@/lib/format'

import { createTokenSale, getTokenSaleUsage, updateTokenSale } from '../api'
import {
  getTokenSaleFormDefaults,
  getTokenSaleFormSchema,
  tokenSaleFormToCreatePayload,
  tokenSaleFormToUpdatePayload,
  type TokenSaleFormValues,
} from '../lib/token-sale-form'
import type { TokenSale } from '../types'

type TokenSaleDrawerProps = {
  open: boolean
  sale?: TokenSale
  onOpenChange: (open: boolean) => void
  onSaved: () => Promise<void>
}

export function TokenSaleDrawer(props: TokenSaleDrawerProps) {
  const { t } = useTranslation()
  const [submitting, setSubmitting] = useState(false)
  const [generatedKey, setGeneratedKey] = useState('')
  const isEditing = Boolean(props.sale)
  const schema = getTokenSaleFormSchema(t)
  const form = useForm<TokenSaleFormValues>({
    resolver: zodResolver(schema),
    defaultValues: getTokenSaleFormDefaults(props.sale),
  })

  const modelsQuery = useQuery({
    queryKey: ['user-models'],
    queryFn: getUserModels,
    enabled: props.open,
    staleTime: 30_000,
  })
  const groupsQuery = useQuery({
    queryKey: ['user-groups'],
    queryFn: getUserGroups,
    enabled: props.open,
    staleTime: 30_000,
  })
  const usageQuery = useQuery({
    queryKey: ['token-sale-usage', props.sale?.id],
    queryFn: () => getTokenSaleUsage(props.sale?.id ?? 0),
    enabled: props.open && Boolean(props.sale),
  })

  useEffect(() => {
    if (!props.open) return
    form.reset(getTokenSaleFormDefaults(props.sale))
    setGeneratedKey('')
  }, [form, props.open, props.sale])

  const models = (modelsQuery.data?.data ?? []).map((model: string) => ({
    label: model,
    value: model,
  }))
  const groups = Object.keys(groupsQuery.data?.data ?? {})
  const selectedGroup = form.watch('group')
  const modelUsage = usageQuery.data?.data ?? []
  const currencyLabel = getCurrencyLabel()

  const submit = async (values: TokenSaleFormValues) => {
    setSubmitting(true)
    try {
      if (isEditing && props.sale) {
        const result = await updateTokenSale(
          props.sale.id,
          tokenSaleFormToUpdatePayload(values)
        )
        if (!result.success) {
          throw new Error(result.message || t('Failed to save sales key'))
        }
        await props.onSaved()
        toast.success(t('Sales key updated'))
        props.onOpenChange(false)
        return
      }

      const result = await createTokenSale(tokenSaleFormToCreatePayload(values))
      if (!result.success) {
        throw new Error(result.message || t('Failed to save sales key'))
      }
      await props.onSaved()
      setGeneratedKey(result.data?.key ?? '')
      if (result.data?.expired_time) {
        form.setValue('expired_time', new Date(result.data.expired_time * 1000))
      }
      toast.success(t('Sales key created'))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to save sales key')
      )
    } finally {
      setSubmitting(false)
    }
  }

  let submitLabel = t('Generate sales key')
  if (submitting) {
    submitLabel = t('Saving...')
  } else if (isEditing) {
    submitLabel = t('Save changes')
  }

  const copyDelivery = async () => {
    if (!generatedKey) return
    const baseUrl = `${window.location.origin}/v1`
    const concurrency = form.getValues('max_concurrency')
    const rpm = form.getValues('rpm_rate_limit')
    const copied = await copyToClipboard(
      [
        `${t('API URL')}: ${baseUrl}`,
        `${t('API Key')}: ${generatedKey}`,
        `${t('Quota')}: ${form.getValues('quota_amount')} ${currencyLabel}`,
        `${t('Expiration Time')}: ${formatDateTimeStr(form.getValues('expired_time'))}`,
        `${t('Maximum concurrency')}: ${concurrency || t('Unlimited')}`,
        `${t('Requests per minute')}: ${rpm || t('Unlimited')}`,
      ].join('\n')
    )
    toast[copied ? 'success' : 'error'](
      copied ? t('Delivery details copied') : t('Copy failed')
    )
  }

  const copyGeneratedKey = async () => {
    if (!generatedKey) return
    const copied = await copyToClipboard(generatedKey)
    toast[copied ? 'success' : 'error'](copied ? t('Copied') : t('Copy failed'))
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent
        className={sideDrawerContentClassName('max-w-none sm:!max-w-[640px]')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isEditing ? t('Edit sales key') : t('Create sales key')}
          </SheetTitle>
          <SheetDescription>
            {t(
              'Configure the buyer, quota, expiration, and request limits before delivery.'
            )}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='token-sale-form'
            className={sideDrawerFormClassName('gap-5')}
            onSubmit={form.handleSubmit(submit)}
          >
            {generatedKey && (
              <Alert>
                <KeyRound className='size-4' />
                <AlertTitle>{t('Key ready for delivery')}</AlertTitle>
                <AlertDescription className='space-y-3'>
                  <InputGroup>
                    <InputGroupInput
                      value={generatedKey}
                      readOnly
                      className='font-mono'
                    />
                    <InputGroupAddon align='inline-end'>
                      <InputGroupButton
                        size='icon-xs'
                        aria-label={t('Copy Key')}
                        onClick={copyGeneratedKey}
                      >
                        <Copy />
                      </InputGroupButton>
                    </InputGroupAddon>
                  </InputGroup>
                  <Button type='button' size='sm' onClick={copyDelivery}>
                    <Copy className='size-4' />
                    {t('Copy delivery details')}
                  </Button>
                </AlertDescription>
              </Alert>
            )}

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Order information')}
                description={t('Record the buyer and payment for this key.')}
                icon={<ReceiptText className='size-4' />}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='name'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Key name')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder={t('Enter a key name')} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='buyer'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Buyer')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder={t('Buyer alias')} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='order_no'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('External order number')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder={t('Optional order number')}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='amount_cny'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Revenue in CNY')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={0}
                          step='0.01'
                          value={field.value}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value))
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Quota and expiration')}
                description={t(
                  'Set the total deliverable quota and validity period.'
                )}
                icon={<WalletCards className='size-4' />}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='quota_amount'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t('Quota ({{currency}})', {
                          currency: currencyLabel,
                        })}
                      </FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={0}
                          step='0.01'
                          value={field.value}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value))
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                {isEditing ? (
                  <FormField
                    control={form.control}
                    name='expired_time'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Expiration Time')}</FormLabel>
                        <FormControl>
                          <DateTimePicker
                            value={field.value}
                            onChange={field.onChange}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                ) : (
                  <FormField
                    control={form.control}
                    name='validity_days'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Validity Period')}</FormLabel>
                        <FormControl>
                          <InputGroup>
                            <InputGroupInput
                              type='number'
                              min={1}
                              max={3650}
                              value={field.value}
                              onChange={(event) =>
                                field.onChange(Number(event.target.value))
                              }
                            />
                            <InputGroupAddon align='inline-end'>
                              {t('days')}
                            </InputGroupAddon>
                          </InputGroup>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
              </div>
            </SideDrawerSection>

            {isEditing && (
              <SideDrawerSection>
                <SideDrawerSectionHeader
                  title={t('Usage')}
                  icon={<ChartNoAxesColumn className='size-4' />}
                />
                <div className='space-y-2'>
                  {modelUsage.map((usage) => (
                    <div
                      key={usage.model_name}
                      className='rounded-lg border p-3 text-xs'
                    >
                      <div className='flex items-center justify-between gap-3'>
                        <span className='truncate font-medium'>
                          {usage.model_name}
                        </span>
                        <span className='tabular-nums'>
                          {formatQuota(usage.quota)}
                        </span>
                      </div>
                      <div className='text-muted-foreground mt-2 grid grid-cols-2 gap-1 sm:grid-cols-4'>
                        <span>
                          {t('Requests')}: {usage.request_count}
                        </span>
                        <span>
                          {t('Model ratio')}: {usage.model_ratio || '—'}
                        </span>
                        <span>
                          {t('Group')}: {usage.group_ratio || '—'}
                        </span>
                        <span>
                          {t('Tier')}: {usage.matched_tier || '—'}
                        </span>
                      </div>
                    </div>
                  ))}
                  {!usageQuery.isLoading && modelUsage.length === 0 && (
                    <div className='text-muted-foreground rounded-lg border p-4 text-center text-xs'>
                      {t('No data')}
                    </div>
                  )}
                </div>
              </SideDrawerSection>
            )}

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Request limits')}
                description={t(
                  'Protect pooled capacity from a single heavy user.'
                )}
                icon={<Settings2 className='size-4' />}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='max_concurrency'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Maximum concurrency')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={0}
                          max={1000}
                          value={field.value}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value))
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='rpm_rate_limit'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Requests per minute')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={0}
                          max={100000}
                          value={field.value}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value))
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='group'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Group')}</FormLabel>
                      <FormControl>
                        <NativeSelect
                          value={field.value}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          className='w-full'
                        >
                          {groups.map((group) => (
                            <NativeSelectOption key={group} value={group}>
                              {group}
                            </NativeSelectOption>
                          ))}
                        </NativeSelect>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                {isEditing && (
                  <FormField
                    control={form.control}
                    name='status'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Status')}</FormLabel>
                        <FormControl>
                          <NativeSelect
                            value={field.value}
                            onChange={(event) =>
                              field.onChange(event.target.value)
                            }
                            className='w-full'
                          >
                            <NativeSelectOption value='active'>
                              {t('Active')}
                            </NativeSelectOption>
                            <NativeSelectOption value='closed'>
                              {t('Closed')}
                            </NativeSelectOption>
                          </NativeSelect>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
              </div>

              {selectedGroup === 'auto' && (
                <FormField
                  control={form.control}
                  name='cross_group_retry'
                  render={({ field }) => (
                    <FormItem className='flex items-center justify-between rounded-lg border p-3'>
                      <FormLabel>{t('Cross-group retry')}</FormLabel>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              )}

              <FormField
                control={form.control}
                name='model_limits'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Allowed models')}</FormLabel>
                    <FormControl>
                      <MultiSelect
                        options={models}
                        selected={field.value}
                        onChange={field.onChange}
                        placeholder={t('All models')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='allow_ips'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('IP whitelist')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        placeholder={t('Optional, one IP or CIDR per line')}
                        rows={3}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='note'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Internal note')}</FormLabel>
                    <FormControl>
                      <Textarea {...field} rows={3} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          {generatedKey && !isEditing ? (
            <Button type='button' onClick={() => props.onOpenChange(false)}>
              {t('Done')}
            </Button>
          ) : (
            <Button type='submit' form='token-sale-form' disabled={submitting}>
              {submitLabel}
            </Button>
          )}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
