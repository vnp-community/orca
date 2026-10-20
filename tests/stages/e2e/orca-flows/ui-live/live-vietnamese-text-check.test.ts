/**
 * Pure checks of the Vietnamese-text heuristic; no stack, no browser.
 *   node --test tests/stages/e2e/orca-flows/ui-live/live-vietnamese-text-check.test.ts
 */
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { vietnameseShare } from './live-vietnamese-text-check.ts'

test('a Vietnamese answer passes the three thresholds of the real-model spec', () => {
  const md =
    'Hàm `max_of` trong `src/calc.py` trả về số lớn nhất của một danh sách và báo lỗi khi danh sách không có phần tử nào. ' +
    'Bài kiểm thử `test_max_of` trong tệp test_calc.py phủ các trường hợp này, kể cả danh sách chỉ có một số.'
  const share = vietnameseShare(md)
  assert.ok(share.accentedLetterRatio >= 0.15, `accented ${share.accentedLetterRatio}`)
  assert.ok(share.vietnameseStopWords >= 3, `stop words ${share.vietnameseStopWords}`)
  assert.ok(share.englishStopWordRatio < 0.1)
})

test('an English answer fails them', () => {
  const share = vietnameseShare(
    'The max_of function returns the largest value of the list and raises an error when it is empty. The test covers this.'
  )
  assert.ok(share.accentedLetterRatio < 0.01)
  assert.equal(share.vietnameseStopWords, 0)
  assert.ok(share.englishStopWordRatio >= 0.1)
})

test('code is ignored, so identifiers cannot make a Vietnamese answer look English', () => {
  const md =
    'Đoạn mã dưới đây là ví dụ:\n```go\nfunc The(of, and, is, to, in, that string) {}\n```\nHàm này được dùng cho các bài kiểm thử và không có tác dụng phụ.'
  const share = vietnameseShare(md)
  assert.ok(share.englishStopWordRatio < 0.1)
  assert.ok(share.vietnameseStopWords >= 3)
})

test('an empty answer is neither Vietnamese nor English', () => {
  assert.deepEqual(vietnameseShare(''), {
    accentedLetterRatio: 0,
    vietnameseStopWords: 0,
    englishStopWordRatio: 0
  })
})
