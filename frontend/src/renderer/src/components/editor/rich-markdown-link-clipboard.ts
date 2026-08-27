import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'

import { uiWriteClipboardText } from '@/runtime/runtime-ui-client'
export async function copyRichMarkdownLink(href: string): Promise<void> {
  try {
    await uiWriteClipboardText(href)
    toast.success(
      translate('auto.components.editor.richMarkdownLinkClipboard.copiedLink', 'Copied link')
    )
  } catch {
    toast.error(
      translate(
        'auto.components.editor.richMarkdownLinkClipboard.copyLinkFailed',
        'Failed to copy link'
      )
    )
  }
}
