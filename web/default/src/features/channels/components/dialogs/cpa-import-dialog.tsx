import { Loader2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

import { importCPAAccounts } from '../../api'

type CPAImportDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function CPAImportDialog({ open, onOpenChange }: CPAImportDialogProps) {
  const [content, setContent] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const submit = async () => {
    if (!content.trim()) {
      toast.error('请粘贴账号 JSON')
      return
    }
    setSubmitting(true)
    try {
      const result = await importCPAAccounts(content)
      if (result.success) {
        toast.success(result.message || 'CPA 上货成功')
        setContent('')
        onOpenChange(false)
      } else {
        toast.error(result.message || 'CPA 上货失败')
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'CPA 上货失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title='CPA 上货'
      description='粘贴单个账号 JSON、JSON 数组，或连续多个 JSON 对象。系统会自动拆分并补齐 CPA 字段。'
      contentClassName='sm:max-w-3xl'
      footer={
        <>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button onClick={submit} disabled={submitting || !content.trim()}>
            {submitting && <Loader2 className='size-4 animate-spin' />}
            开始上货
          </Button>
        </>
      }
    >
      <div className='space-y-2'>
        <Label htmlFor='cpa-import-json'>账号 JSON</Label>
        <Textarea
          id='cpa-import-json'
          value={content}
          onChange={(event) => setContent(event.target.value)}
          placeholder='粘贴包含 access_token 的账号 JSON；多账号可使用 JSON 数组'
          className='min-h-[360px] font-mono text-xs'
          spellCheck={false}
        />
        <p className='text-muted-foreground text-xs'>
          必填字段只有 access_token；email、account_id、过期时间和 id_token
          会自动补齐。凭据只发送到本机 CPA，不写入 New API 数据库。
        </p>
      </div>
    </Dialog>
  )
}
