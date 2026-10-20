/**
 * Is a piece of Markdown prose written in Vietnamese? A heuristic for the real-model spec: it measures, it does not translate.
 * Code (fences, inline code) is dropped first, since identifiers and commands stay in English by design.
 */

const VIETNAMESE_STOP_WORDS = new Set([
  'và',
  'của',
  'không',
  'là',
  'các',
  'một',
  'được',
  'cho',
  'trong'
])
const ENGLISH_STOP_WORDS = new Set([
  'the',
  'of',
  'and',
  'is',
  'to',
  'in',
  'that',
  'it',
  'for',
  'this',
  'with',
  'as',
  'are',
  'by',
  'an',
  'be',
  'which',
  'if',
  'when',
  'returns',
  'from',
  'then',
  'also'
])

export type VietnameseShare = {
  /** Letters carrying a diacritic or đ, over all letters. Vietnamese prose is roughly a quarter to a third; English is near 0. */
  accentedLetterRatio: number
  /** How many distinct common Vietnamese words appear. */
  vietnameseStopWords: number
  /** English function words over all words. */
  englishStopWordRatio: number
}

export function proseOf(markdown: string): string {
  return markdown.replace(/```[\s\S]*?```/g, ' ').replace(/`[^`\n]*`/g, ' ')
}

export function vietnameseShare(markdown: string): VietnameseShare {
  const prose = proseOf(markdown)
  const letters = [...prose].filter((c) => /\p{L}/u.test(c))
  const accented = letters.filter(
    (c) => c === 'đ' || c === 'Đ' || /[̀-ͯ]/.test(c.normalize('NFD').slice(1))
  )
  const words = prose.toLowerCase().match(/\p{L}+/gu) ?? []
  const vi = new Set(words.filter((w) => VIETNAMESE_STOP_WORDS.has(w)))
  const en = words.filter((w) => ENGLISH_STOP_WORDS.has(w))
  return {
    accentedLetterRatio: letters.length === 0 ? 0 : accented.length / letters.length,
    vietnameseStopWords: vi.size,
    englishStopWordRatio: words.length === 0 ? 0 : en.length / words.length
  }
}
