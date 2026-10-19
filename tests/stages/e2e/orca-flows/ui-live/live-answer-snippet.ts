/**
 * A stretch of the answer that survives markdown rendering unchanged: the longest line that is plain text once emphasis marks,
 * code ticks and heading/list markers are removed, cut to 40 characters. The screen must contain it (whitespace collapsed).
 */
export function plainSnippetOf(markdown: string): string {
  const plain = markdown
    .split('\n')
    .map((l) =>
      l
        .replace(/^\s*(?:#{1,6}\s+|>\s*|[-*+]\s+|\d+[.)]\s+)/, '')
        .replace(/[`*]/g, '')
        .trim()
    )
    .filter((l) => /^[\p{L}\p{N} _,.:'()/-]{25,}$/u.test(l))
    .sort((a, b) => b.length - a.length)
  return (plain[0] ?? markdown.replace(/[^\p{L}\p{N} ]+/gu, ' ').trim())
    .replace(/\s+/g, ' ')
    .slice(0, 40)
    .trim()
}
