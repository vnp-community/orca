/**
 * Pure checks of the snippet the add-project spec looks for on screen; no stack, no browser.
 *   node --test tests/stages/e2e/orca-flows/ui-live/live-answer-snippet.test.ts
 */
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { plainSnippetOf } from './live-answer-snippet.ts'

test('skips tables and headings, drops emphasis and code ticks, keeps identifiers', () => {
  const md = [
    '## Tổng quan',
    '',
    '| Hàm | Chức năng |',
    '|---|---|',
    '| `bravo_add` | Trả về tổng |',
    '',
    'Repo **bravo-sandbox** là một fixture rất nhỏ dùng cho test tự động, có hàm `bravo_add` và `bravo_sub`.',
    '- ngắn'
  ].join('\n')
  const snippet = plainSnippetOf(md)
  assert.equal(snippet, 'Repo bravo-sandbox là một fixture rất nh')
  assert.ok(snippet.length <= 40)
})

test('keeps underscores of identifiers and strips list markers', () => {
  assert.equal(
    plainSnippetOf('1. Hàm bravo_add cộng hai số nguyên và trả về kết quả'),
    'Hàm bravo_add cộng hai số nguyên và trả'
  )
})

test('falls back to the plain words when no line is plain enough', () => {
  const fallback = plainSnippetOf('| a | b |\n|---|---|\n| `x` | `y` |')
  assert.match(fallback, /^[a-z x y]+$/i)
})
