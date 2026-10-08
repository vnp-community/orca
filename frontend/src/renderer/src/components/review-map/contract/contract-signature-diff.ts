/**
 * contract-signature-diff.ts — FE-CV-TASK-059-01
 *
 * Token diff for a before/after signature pair (no Monaco). Whitespace is normalised so
 * formatting-only edits do not show as changes.
 *
 * @module components/review-map/contract/contract-signature-diff
 */

export type SignatureTokenState = 'same' | 'added' | 'removed'
export type SignatureToken = { text: string; state: SignatureTokenState }

export type SignatureDiff = { before: SignatureToken[]; after: SignatureToken[] }

function tokenize(signature: string | undefined): string[] {
  if (!signature) {
    return []
  }
  const normalized = signature.replace(/\s+/g, ' ').trim()
  return normalized.match(/\w+|\W/g)?.filter((t) => t !== ' ') ?? []
}

/** Longest-common-subsequence over tokens; signatures are short so O(n*m) is fine. */
export function diffSignatureTokens(
  before: string | undefined,
  after: string | undefined
): SignatureDiff {
  const a = tokenize(before)
  const b = tokenize(after)
  const lcs: number[][] = Array.from({ length: a.length + 1 }, () => Array.from({ length: b.length + 1 }, () => 0))
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      lcs[i][j] = a[i] === b[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1])
    }
  }
  const outBefore: SignatureToken[] = []
  const outAfter: SignatureToken[] = []
  let i = 0
  let j = 0
  while (i < a.length || j < b.length) {
    if (i < a.length && j < b.length && a[i] === b[j]) {
      outBefore.push({ text: a[i], state: 'same' })
      outAfter.push({ text: b[j], state: 'same' })
      i++
      j++
    } else if (j >= b.length || (i < a.length && lcs[i + 1][j] >= lcs[i][j + 1])) {
      outBefore.push({ text: a[i], state: 'removed' })
      i++
    } else {
      outAfter.push({ text: b[j], state: 'added' })
      j++
    }
  }
  return { before: outBefore, after: outAfter }
}
