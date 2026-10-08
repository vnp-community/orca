import type { SymbolRef } from '../../../shared/code-intel-types'

// ---------------------------------------------------------------------------
// Symbols
// ---------------------------------------------------------------------------

export const SYMBOL_REF_SERVICE: SymbolRef = {
  key: 'go:svc/order.Create',
  kind: 'function',
  name: 'Create',
  qualifiedName: 'order.Create',
  filePath: 'services/order/create.go',
  startLine: 10,
  endLine: 42,
  language: 'go',
  isExported: true
}

export const SYMBOL_REF_TEST: SymbolRef = {
  key: 'go:svc/order.TestCreate',
  kind: 'function',
  name: 'TestCreate',
  filePath: 'services/order/create_test.go',
  startLine: 5,
  endLine: 30,
  language: 'go'
}

