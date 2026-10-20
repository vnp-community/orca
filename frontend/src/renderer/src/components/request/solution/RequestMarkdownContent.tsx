/**
 * RequestMarkdownContent — CR-REQ-020-02
 *
 * Safe Markdown for AI-generated text: no raw HTML (react-markdown drops it),
 * links limited to http(s), no HTML injection.
 *
 * @module components/request/solution/RequestMarkdownContent
 */

import React, { useState } from 'react'
import ReactMarkdown, { type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle
} from '@/components/ui/sheet'

export const MARKDOWN_COLLAPSE_LENGTH = 4000

export function isSafeMarkdownHref(href: string | undefined): boolean {
  if (!href) {
    return false
  }
  try {
    const protocol = new URL(href).protocol
    return protocol === 'http:' || protocol === 'https:'
  } catch {
    return false
  }
}

// Why: GFM adds tables and task lists; the checkboxes only report a state, so they never accept a click.
const GFM_COMPONENTS: Components = {
  table: ({ children }) => (
    <div className="max-w-full overflow-x-auto scrollbar-sleek">
      <table className="w-full border-collapse text-xs">{children}</table>
    </div>
  ),
  th: ({ children }) => (
    <th className="border border-border bg-muted px-2 py-1 text-left font-medium">{children}</th>
  ),
  td: ({ children }) => <td className="border border-border px-2 py-1 align-top">{children}</td>,
  input: ({ checked }) => (
    <input
      type="checkbox"
      checked={checked === true}
      disabled
      readOnly
      className="mr-1.5 align-middle"
      aria-label={
        checked === true
          ? translate('auto.components.request.RequestMarkdownContent.taskDone', 'Done')
          : translate('auto.components.request.RequestMarkdownContent.taskOpen', 'Not done')
      }
    />
  )
}

function SafeMarkdown({ text, gfm }: { text: string; gfm: boolean }): React.JSX.Element {
  return (
    <div className="prose-sm max-w-none space-y-2 break-words text-sm text-foreground">
      <ReactMarkdown
        remarkPlugins={gfm ? [remarkGfm] : undefined}
        // Why: an authored document carries machine markers such as <!-- orca:begin plan -->; raw HTML is otherwise shown as text.
        skipHtml={gfm}
        components={{
          ...(gfm ? GFM_COMPONENTS : {}),
          a: ({ href, children }) =>
            isSafeMarkdownHref(href) ? (
              <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                className="text-primary underline"
              >
                {children}
              </a>
            ) : (
              <span>{children}</span>
            ),
          img: ({ alt }) => <span>{alt}</span>
        }}
      >
        {text}
      </ReactMarkdown>
    </div>
  )
}

export function RequestMarkdownContent({
  content,
  title,
  gfm = false
}: {
  content: string
  title?: string
  /** Renders an authored document: tables, task lists (`- [ ]`) and no raw HTML at all. Off by default, so Solution text is unchanged. */
  gfm?: boolean
}): React.JSX.Element {
  const [sheetOpen, setSheetOpen] = useState(false)
  const long = content.length > MARKDOWN_COLLAPSE_LENGTH
  // Why: very long AI output slows rendering; show the head and load the rest on demand.
  const shown = long ? content.slice(0, MARKDOWN_COLLAPSE_LENGTH) : content

  return (
    <div data-testid="request-markdown-content">
      <SafeMarkdown text={shown} gfm={gfm} />
      {long && (
        <>
          <Button variant="link" size="sm" className="px-0" onClick={() => setSheetOpen(true)}>
            {translate('auto.components.request.RequestMarkdownContent.viewFull', 'View full')}
          </Button>
          <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
            <SheetContent className="w-[640px] max-w-full overflow-y-auto scrollbar-sleek">
              <SheetHeader>
                <SheetTitle>
                  {title ??
                    translate(
                      'auto.components.request.RequestMarkdownContent.fullTitle',
                      'Full content'
                    )}
                </SheetTitle>
                <SheetDescription className="sr-only">
                  {translate(
                    'auto.components.request.RequestMarkdownContent.fullTitle',
                    'Full content'
                  )}
                </SheetDescription>
              </SheetHeader>
              <div className="px-4 pb-4">
                <SafeMarkdown text={content} gfm={gfm} />
              </div>
            </SheetContent>
          </Sheet>
        </>
      )}
    </div>
  )
}
