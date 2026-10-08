/**
 * RequestMarkdownContent — CR-REQ-020-02
 *
 * Safe Markdown for AI-generated text: no raw HTML (react-markdown drops it),
 * links limited to http(s), no HTML injection.
 *
 * @module components/request/solution/RequestMarkdownContent
 */

import React, { useState } from 'react'
import ReactMarkdown from 'react-markdown'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'

export const MARKDOWN_COLLAPSE_LENGTH = 4000

export function isSafeMarkdownHref(href: string | undefined): boolean {
  if (!href) {return false}
  try {
    const protocol = new URL(href).protocol
    return protocol === 'http:' || protocol === 'https:'
  } catch {
    return false
  }
}

function SafeMarkdown({ text }: { text: string }): React.JSX.Element {
  return (
    <div className="prose-sm max-w-none space-y-2 break-words text-sm text-foreground">
      <ReactMarkdown
        components={{
          a: ({ href, children }) =>
            isSafeMarkdownHref(href) ? (
              <a href={href} target="_blank" rel="noopener noreferrer" className="text-primary underline">
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
  title
}: {
  content: string
  title?: string
}): React.JSX.Element {
  const [sheetOpen, setSheetOpen] = useState(false)
  const long = content.length > MARKDOWN_COLLAPSE_LENGTH
  // Why: very long AI output slows rendering; show the head and load the rest on demand.
  const shown = long ? content.slice(0, MARKDOWN_COLLAPSE_LENGTH) : content

  return (
    <div data-testid="request-markdown-content">
      <SafeMarkdown text={shown} />
      {long && (
        <>
          <Button variant="link" size="sm" className="px-0" onClick={() => setSheetOpen(true)}>
            {translate('auto.components.request.RequestMarkdownContent.viewFull', 'View full')}
          </Button>
          <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
            <SheetContent className="w-[640px] max-w-full overflow-y-auto scrollbar-sleek">
              <SheetHeader>
                <SheetTitle>
                  {title ?? translate('auto.components.request.RequestMarkdownContent.fullTitle', 'Full content')}
                </SheetTitle>
                <SheetDescription className="sr-only">
                  {translate('auto.components.request.RequestMarkdownContent.fullTitle', 'Full content')}
                </SheetDescription>
              </SheetHeader>
              <div className="px-4 pb-4">
                <SafeMarkdown text={content} />
              </div>
            </SheetContent>
          </Sheet>
        </>
      )}
    </div>
  )
}
