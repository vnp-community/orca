import { createElement, type ReactNode } from 'react'

// Why: mobile has no react-native renderer in its node test env, so view tests
// mock RN primitives as DOM tags and render with react-dom/server; pressables are
// recorded by accessibilityLabel (or their text) so tests can invoke onPress.
export type PressRegistry = Map<string, () => void>

type HostProps = { children?: ReactNode; accessibilityLabel?: string; style?: unknown }

function isRow(style: unknown): boolean {
  if (Array.isArray(style)) {
    return style.some(isRow)
  }
  return (style as { flexDirection?: string } | null)?.flexDirection === 'row'
}

function textOf(node: ReactNode): string {
  if (node === null || node === undefined || typeof node === 'boolean') {
    return ''
  }
  if (typeof node === 'string' || typeof node === 'number') {
    return String(node)
  }
  if (Array.isArray(node)) {
    return node.map(textOf).join('')
  }
  const props = (node as { props?: { children?: ReactNode } }).props
  return props ? textOf(props.children) : ''
}

export function createReactNativeStaticMock(
  presses: PressRegistry,
  window: { width: number; height: number }
) {
  const host =
    (tag: string) =>
    ({ children, accessibilityLabel, style }: HostProps) =>
      createElement(
        tag,
        { 'aria-label': accessibilityLabel, 'data-row': isRow(style) ? '' : undefined },
        children
      )
  return {
    View: host('div'),
    Text: host('span'),
    SafeAreaView: host('main'),
    ScrollView: ({ children, refreshControl }: HostProps & { refreshControl?: ReactNode }) =>
      createElement('section', null, refreshControl, children),
    Pressable: ({
      children,
      onPress,
      accessibilityLabel
    }: HostProps & { onPress?: () => void; children?: ReactNode | ((s: object) => ReactNode) }) => {
      const content = typeof children === 'function' ? children({ pressed: false }) : children
      const key = accessibilityLabel ?? textOf(content)
      if (onPress && !presses.has(key)) {
        presses.set(key, onPress)
      }
      return createElement('button', { 'aria-label': key }, content)
    },
    ActivityIndicator: () => createElement('progress'),
    RefreshControl: ({ refreshing }: { refreshing: boolean }) =>
      createElement('i', { 'data-refreshing': String(refreshing) }),
    StyleSheet: { create: <T>(styles: T) => styles, hairlineWidth: 1 },
    useWindowDimensions: () => window
  }
}
