import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

// The tail is the last path segment (a remote ref `origin/release/1.4` keeps `/1.4`), or the last characters when the
// ref has no slash, so the part that tells two long refs apart survives the ellipsis.
const MAX_TAIL_CHARS = 24
const FALLBACK_TAIL_CHARS = 10

export function splitRefForMiddleTruncation(ref: string): { head: string; tail: string } {
  const lastSlash = ref.lastIndexOf('/')
  const segmentLength = lastSlash >= 0 ? ref.length - lastSlash : FALLBACK_TAIL_CHARS
  const tailLength = Math.min(
    Math.max(segmentLength, FALLBACK_TAIL_CHARS),
    MAX_TAIL_CHARS,
    ref.length
  )
  return { head: ref.slice(0, ref.length - tailLength), tail: ref.slice(ref.length - tailLength) }
}

/** A ref shown in full when it fits and cut in the middle when it does not; the tooltip always carries the whole name. */
export function MiddleTruncatedRef({ value }: { value: string }): React.JSX.Element {
  const { head, tail } = splitRefForMiddleTruncation(value)
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="flex min-w-0 flex-1 text-left" data-slot="middle-truncated-ref">
          <span className="min-w-0 truncate">{head}</span>
          <span className="shrink-0 whitespace-pre">{tail}</span>
        </span>
      </TooltipTrigger>
      <TooltipContent side="top" sideOffset={4} className="max-w-[min(32rem,90vw)] break-all">
        {value}
      </TooltipContent>
    </Tooltip>
  )
}
