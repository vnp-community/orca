/**
 * useTestScriptText — the whole text of one output of a Request's test-script job: the agent's report (`request.testScriptReport`)
 * or a generated file / the manifest (`request.testScriptFile`). The channels page by characters; this follows `next` until the end
 * or MAX_PAGES, so a viewer gets one string. Docs: docs/integrate/TDD-REQ-TESTSCRIPT-001.
 *
 * @module hooks/useTestScriptText
 */

import { useEffect, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseTestScriptFilePage } from '../../../shared/request-testscript-parsers'

/** 20 pages of 20 000 characters: far more than a generated test file; past it the viewer says the text was cut. */
export const TEST_SCRIPT_TEXT_MAX_PAGES = 20

export type TestScriptTextSource = { kind: 'report' } | { kind: 'file'; path: string }

export type TestScriptTextState = {
  text: string
  loading: boolean
  error: string | null
  /** True when MAX_PAGES was reached before the end. */
  truncated: boolean
}

export async function loadTestScriptText(
  requestId: string,
  source: TestScriptTextSource
): Promise<{ text: string; truncated: boolean } | { error: string }> {
  let text = ''
  let offset = 0
  for (let page = 0; page < TEST_SCRIPT_TEXT_MAX_PAGES; page++) {
    const res =
      source.kind === 'report'
        ? await callRequestRpc<unknown>(REQUEST_RPC_METHODS.TEST_SCRIPT_REPORT, {
            id: requestId,
            offset
          })
        : await callRequestRpc<unknown>(REQUEST_RPC_METHODS.TEST_SCRIPT_FILE, {
            id: requestId,
            path: source.path,
            offset
          })
    if (!res.ok) {
      return { error: res.error.kind }
    }
    const p = parseTestScriptFilePage(res.value)
    text += p.text
    if (!p.hasNext) {
      return { text, truncated: false }
    }
    offset = p.next
  }
  return { text, truncated: true }
}

export function useTestScriptText(
  requestId: string,
  source: TestScriptTextSource | null
): TestScriptTextState {
  const [state, setState] = useState<TestScriptTextState>({
    text: '',
    loading: false,
    error: null,
    truncated: false
  })
  const key = source ? (source.kind === 'report' ? 'report' : `file:${source.path}`) : ''

  useEffect(() => {
    if (!source) {
      return undefined
    }
    let live = true
    setState({ text: '', loading: true, error: null, truncated: false })
    void loadTestScriptText(requestId, source).then((res) => {
      if (!live) {
        return
      }
      if ('error' in res) {
        setState({ text: '', loading: false, error: res.error, truncated: false })
      } else {
        setState({ text: res.text, loading: false, error: null, truncated: res.truncated })
      }
    })
    return () => {
      live = false
    }
    // key names the source; the object itself is rebuilt on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [requestId, key])

  return state
}
