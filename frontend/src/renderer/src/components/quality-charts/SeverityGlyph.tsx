import type { EncodingShape } from './severity-encoding'

type SeverityGlyphProps = {
  shape: EncodingShape
  className?: string
  size?: number
}

export function SeverityGlyph({
  shape,
  className,
  size = 14
}: SeverityGlyphProps): React.JSX.Element {
  return (
    <svg
      viewBox="0 0 16 16"
      width={size}
      height={size}
      aria-hidden="true"
      focusable="false"
      data-shape={shape}
      className={className}
    >
      {shape === 'octagon' ? (
        <polygon
          points="5,1.5 11,1.5 14.5,5 14.5,11 11,14.5 5,14.5 1.5,11 1.5,5"
          fill="currentColor"
        />
      ) : null}
      {shape === 'triangle' ? (
        <polygon points="8,2 14.5,13.5 1.5,13.5" fill="currentColor" />
      ) : null}
      {shape === 'circle-open' ? (
        <circle cx="8" cy="8" r="6" fill="none" stroke="currentColor" strokeWidth="1.75" />
      ) : null}
      {shape === 'circle-check' ? (
        <>
          <circle cx="8" cy="8" r="6" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <polyline
            points="5,8.3 7.2,10.5 11,5.8"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
          />
        </>
      ) : null}
      {shape === 'circle-dashed' ? (
        <circle
          cx="8"
          cy="8"
          r="6"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeDasharray="2 3"
        />
      ) : null}
    </svg>
  )
}
