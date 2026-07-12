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
import { Copy, Terminal } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'
import { copyToClipboard } from '@/lib/copy-to-clipboard'

import { ApiKeysDialogs } from './components/api-keys-dialogs'
import { ApiKeysPrimaryButtons } from './components/api-keys-primary-buttons'
import { ApiKeysProvider } from './components/api-keys-provider'
import { ApiKeysTable } from './components/api-keys-table'

function normalizeApiBaseUrl(serverAddress?: string) {
  const origin = serverAddress?.trim() || window.location.origin
  const normalized = origin.replace(/\/+$/, '')
  return normalized.endsWith('/v1') ? normalized : `${normalized}/v1`
}

function ApiConnectionGuide() {
  const { status } = useStatus()
  const serverAddress =
    (status?.server_address as string | undefined) ??
    (status?.data?.server_address as string | undefined)
  const apiBaseUrl = useMemo(
    () => normalizeApiBaseUrl(serverAddress),
    [serverAddress]
  )
  const curlExample = `curl ${apiBaseUrl}/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"gpt-5.5","messages":[{"role":"user","content":"Hello"}]}'`

  const copy = async (value: string) => {
    if (await copyToClipboard(value)) toast.success('已复制')
  }

  return (
    <div className='bg-card mb-4 space-y-3 rounded-lg border p-4'>
      <div>
        <div className='text-sm font-semibold'>API 接入信息</div>
        <div className='text-muted-foreground mt-1 text-xs'>
          所有用户使用同一个 Base URL，并在下方使用各自的 API Key。
        </div>
      </div>
      <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
        <code className='bg-muted min-w-0 flex-1 overflow-x-auto rounded-md px-3 py-2 text-xs'>
          {apiBaseUrl}
        </code>
        <Button variant='outline' size='sm' onClick={() => copy(apiBaseUrl)}>
          <Copy className='size-4' />
          复制 Base URL
        </Button>
        <Button variant='outline' size='sm' onClick={() => copy(curlExample)}>
          <Terminal className='size-4' />
          复制 curl 示例
        </Button>
      </div>
      <div className='text-muted-foreground text-xs'>
        点击下方被遮罩的 Key 可查看完整密钥；每行右侧菜单可复制包含真实 Key
        的连接信息或 curl 示例。
      </div>
    </div>
  )
}

export function ApiKeys() {
  const { t } = useTranslation()
  return (
    <ApiKeysProvider>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('API Keys')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <ApiKeysPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <ApiConnectionGuide />
          <ApiKeysTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ApiKeysDialogs />
    </ApiKeysProvider>
  )
}
