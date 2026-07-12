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
import { AlertCircle, CheckCircle2, Loader2, MonitorUp } from 'lucide-react'
import {
  type ClipboardEvent,
  type KeyboardEvent,
  type MouseEvent,
  type WheelEvent,
  useEffect,
  useRef,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

import {
  cancelCPAOAuth,
  getCPAOAuthBrowserFrame,
  getCPAOAuthStatus,
  sendCPAOAuthBrowserInput,
  startCPACodexOAuth,
  type CPARemoteBrowserInput,
} from '../../api'

type CPAOfficialLoginDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type LoginPhase = 'idle' | 'starting' | 'waiting' | 'success' | 'error'

const remoteBrowserWidth = 1440
const remoteBrowserHeight = 900
const specialKeys: Record<string, { code: string; keyCode: number }> = {
  ArrowDown: { code: 'ArrowDown', keyCode: 40 },
  ArrowLeft: { code: 'ArrowLeft', keyCode: 37 },
  ArrowRight: { code: 'ArrowRight', keyCode: 39 },
  ArrowUp: { code: 'ArrowUp', keyCode: 38 },
  Backspace: { code: 'Backspace', keyCode: 8 },
  Delete: { code: 'Delete', keyCode: 46 },
  Enter: { code: 'Enter', keyCode: 13 },
  Escape: { code: 'Escape', keyCode: 27 },
  Tab: { code: 'Tab', keyCode: 9 },
}

export function CPAOfficialLoginDialog(props: CPAOfficialLoginDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [phase, setPhase] = useState<LoginPhase>('idle')
  const [state, setState] = useState('')
  const [frameUrl, setFrameUrl] = useState('')
  const [errorMessage, setErrorMessage] = useState('')
  const statusTimerRef = useRef<number | undefined>(undefined)
  const frameTimerRef = useRef<number | undefined>(undefined)
  const inputQueueRef = useRef<Promise<void>>(Promise.resolve())

  useEffect(() => {
    if (!props.open || !state || phase !== 'waiting') return

    let cancelled = false
    const poll = async () => {
      try {
        const result = await getCPAOAuthStatus(state)
        if (cancelled) return
        if (!result.success || !result.data) {
          statusTimerRef.current = window.setTimeout(poll, 1500)
          return
        }
        if (result.data.status === 'ok') {
          setPhase('success')
          toast.success(t('Official account login succeeded'))
          await queryClient.invalidateQueries({ queryKey: ['cpa-accounts'] })
          return
        }
        if (result.data.status === 'error') {
          setErrorMessage(
            result.data.error || t('Official account login failed')
          )
          setPhase('error')
          return
        }
        statusTimerRef.current = window.setTimeout(poll, 1500)
      } catch {
        if (!cancelled) {
          statusTimerRef.current = window.setTimeout(poll, 2500)
        }
      }
    }

    void poll()
    return () => {
      cancelled = true
      if (statusTimerRef.current !== undefined) {
        window.clearTimeout(statusTimerRef.current)
      }
    }
  }, [phase, props.open, queryClient, state, t])

  useEffect(() => {
    if (!props.open || !state || phase !== 'waiting') return

    let cancelled = false
    let currentFrameUrl = ''
    const refreshFrame = async () => {
      try {
        const nextFrameUrl = await getCPAOAuthBrowserFrame(state)
        if (cancelled) {
          URL.revokeObjectURL(nextFrameUrl)
          return
        }
        if (currentFrameUrl) URL.revokeObjectURL(currentFrameUrl)
        currentFrameUrl = nextFrameUrl
        setFrameUrl(nextFrameUrl)
        frameTimerRef.current = window.setTimeout(refreshFrame, 400)
      } catch {
        if (!cancelled) {
          frameTimerRef.current = window.setTimeout(refreshFrame, 1000)
        }
      }
    }

    void refreshFrame()
    return () => {
      cancelled = true
      if (frameTimerRef.current !== undefined) {
        window.clearTimeout(frameTimerRef.current)
      }
      if (currentFrameUrl) URL.revokeObjectURL(currentFrameUrl)
      setFrameUrl('')
    }
  }, [phase, props.open, state])

  const reset = () => {
    setPhase('idle')
    setState('')
    setFrameUrl('')
    setErrorMessage('')
  }

  const handleOpenChange = (open: boolean) => {
    if (!open && state && phase === 'waiting') {
      void cancelCPAOAuth(state)
    }
    if (!open) reset()
    props.onOpenChange(open)
  }

  const startLogin = async () => {
    if (state) await cancelCPAOAuth(state).catch(() => undefined)
    setPhase('starting')
    setState('')
    setFrameUrl('')
    setErrorMessage('')
    try {
      const result = await startCPACodexOAuth()
      if (!result.success || !result.data) {
        setErrorMessage(
          result.message || t('Failed to start official account login')
        )
        setPhase('error')
        return
      }
      setState(result.data.state)
      setPhase('waiting')
    } catch (error) {
      setErrorMessage(
        error instanceof Error
          ? error.message
          : t('Failed to start official account login')
      )
      setPhase('error')
    }
  }

  const queueInput = (input: CPARemoteBrowserInput) => {
    if (!state || phase !== 'waiting') return
    inputQueueRef.current = inputQueueRef.current
      .then(() => sendCPAOAuthBrowserInput(state, input))
      .catch(() => undefined)
  }

  const handleBrowserClick = (event: MouseEvent<HTMLDivElement>) => {
    const rect = event.currentTarget.getBoundingClientRect()
    event.currentTarget.focus()
    queueInput({
      type: 'click',
      x: ((event.clientX - rect.left) / rect.width) * remoteBrowserWidth,
      y: ((event.clientY - rect.top) / rect.height) * remoteBrowserHeight,
    })
  }

  const handleBrowserWheel = (event: WheelEvent<HTMLDivElement>) => {
    const rect = event.currentTarget.getBoundingClientRect()
    event.preventDefault()
    queueInput({
      type: 'scroll',
      x: ((event.clientX - rect.left) / rect.width) * remoteBrowserWidth,
      y: ((event.clientY - rect.top) / rect.height) * remoteBrowserHeight,
      delta_x: event.deltaX,
      delta_y: event.deltaY,
    })
  }

  const handleBrowserKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.metaKey || event.ctrlKey || event.altKey) return
    const special = specialKeys[event.key]
    if (special) {
      event.preventDefault()
      queueInput({
        type: 'key',
        key: event.key,
        code: special.code,
        key_code: special.keyCode,
      })
      return
    }
    if (event.key.length === 1) {
      event.preventDefault()
      queueInput({ type: 'text', text: event.key })
    }
  }

  const handleBrowserPaste = (event: ClipboardEvent<HTMLDivElement>) => {
    const text = event.clipboardData.getData('text')
    if (!text) return
    event.preventDefault()
    queueInput({ type: 'text', text })
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Add official ChatGPT account')}
      description={t(
        'The official login browser runs entirely on the server. This page only displays and controls the remote browser.'
      )}
      contentClassName='sm:max-w-5xl'
      bodyClassName='overflow-hidden'
      footer={
        <>
          <Button variant='outline' onClick={() => handleOpenChange(false)}>
            {t('Close')}
          </Button>
          {phase === 'idle' || phase === 'error' ? (
            <Button onClick={startLogin}>
              {t('Start remote official login')}
            </Button>
          ) : null}
        </>
      }
    >
      <div className='space-y-4'>
        {phase === 'idle' ? (
          <Alert>
            <MonitorUp className='size-4' />
            <AlertTitle>{t('Server-side browser login')}</AlertTitle>
            <AlertDescription>
              {t(
                'After starting, click inside the remote browser and type normally. Clipboard paste is supported.'
              )}
            </AlertDescription>
          </Alert>
        ) : null}

        {phase === 'starting' ? (
          <div className='flex min-h-96 items-center justify-center'>
            <Loader2 className='text-muted-foreground size-8 animate-spin' />
          </div>
        ) : null}

        {phase === 'waiting' ? (
          <div
            role='application'
            tabIndex={0}
            aria-label={t('Remote official login browser')}
            className='bg-muted focus-visible:ring-ring relative aspect-[8/5] w-full cursor-default overflow-hidden rounded-lg border outline-none focus-visible:ring-2'
            onClick={handleBrowserClick}
            onKeyDown={handleBrowserKeyDown}
            onPaste={handleBrowserPaste}
            onWheel={handleBrowserWheel}
            onContextMenu={(event) => event.preventDefault()}
          >
            {frameUrl ? (
              <img
                src={frameUrl}
                alt={t('Remote official login browser')}
                className='size-full object-contain select-none'
                draggable={false}
              />
            ) : (
              <div className='flex size-full items-center justify-center'>
                <Loader2 className='text-muted-foreground size-8 animate-spin' />
              </div>
            )}
          </div>
        ) : null}

        {phase === 'success' ? (
          <Alert>
            <CheckCircle2 className='size-4 text-emerald-600' />
            <AlertTitle>{t('Official account added')}</AlertTitle>
            <AlertDescription>
              {t('The account is now available in the CPA account pool.')}
            </AlertDescription>
          </Alert>
        ) : null}

        {phase === 'error' && errorMessage ? (
          <Alert variant='destructive'>
            <AlertCircle className='size-4' />
            <AlertTitle>{t('Official account login failed')}</AlertTitle>
            <AlertDescription>{errorMessage}</AlertDescription>
          </Alert>
        ) : null}
      </div>
    </Dialog>
  )
}
