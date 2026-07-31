// ProviderList.tsx — AI provider accounts table with filters (TASK-V5-08)
import { useState, lazy, Suspense } from 'react'
import { useAIProviders } from '../../hooks/useAIProviders'
import { HealthStatusBadge } from './HealthStatusBadge'
import { UsageChart } from './UsageChart'
import { Button } from '../ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import { Plus, RefreshCw, TestTube } from 'lucide-react'
import type { AIProviderAccount } from '../../types/ai-provider-types'

const ProviderForm = lazy(() =>
  import('./ProviderForm').then(m => ({ default: m.ProviderForm }))
)

type Filters = {
  devServerId: string
  scope:       string
  status:      string
}

export function ProviderList() {
  const { accounts, isLoading, refresh, testConnection } = useAIProviders()
  const [editingAccount, setEditingAccount] = useState<AIProviderAccount | null>(null)
  const [showForm, setShowForm]             = useState(false)
  const [filters, setFilters]               = useState<Filters>({ devServerId: 'all', scope: 'all', status: 'all' })

  const filtered = accounts.filter(a => {
    if (filters.devServerId !== 'all' && a.devServerId !== filters.devServerId) return false
    if (filters.scope       !== 'all' && a.scope       !== filters.scope)       return false
    if (filters.status      !== 'all' && a.status      !== filters.status)      return false
    return true
  })

  return (
    <div className="provider-list p-4 space-y-4" data-testid="provider-list">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold">AI Provider Accounts</h2>
        <div className="flex gap-2">
          <Button size="sm" variant="outline" onClick={refresh} disabled={isLoading}>
            <RefreshCw size={14} className={isLoading ? 'animate-spin' : ''} />
          </Button>
          <Button
            size="sm"
            onClick={() => { setEditingAccount(null); setShowForm(true) }}
            data-testid="add-account-btn"
          >
            <Plus size={14} className="mr-1" /> Add Account
          </Button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex gap-3">
        <Select value={filters.scope} onValueChange={v => setFilters(f => ({ ...f, scope: v }))}>
          <SelectTrigger className="w-36" data-testid="filter-scope">
            <SelectValue placeholder="All Scopes" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Scopes</SelectItem>
            <SelectItem value="server">Server</SelectItem>
            <SelectItem value="project">Project</SelectItem>
            <SelectItem value="user">User</SelectItem>
          </SelectContent>
        </Select>
        <Select value={filters.status} onValueChange={v => setFilters(f => ({ ...f, status: v }))}>
          <SelectTrigger className="w-40" data-testid="filter-status">
            <SelectValue placeholder="All Statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Statuses</SelectItem>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="invalid">Invalid</SelectItem>
            <SelectItem value="quota_exceeded">Quota Exceeded</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Table */}
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Provider</TableHead>
            <TableHead>Label</TableHead>
            <TableHead>Server</TableHead>
            <TableHead>Scope</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Today's Usage</TableHead>
            <TableHead></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {filtered.map(account => (
            <TableRow key={account.id} data-testid={`account-row-${account.id}`}>
              <TableCell className="font-medium capitalize">{account.provider}</TableCell>
              <TableCell>{account.label}</TableCell>
              <TableCell className="text-sm text-muted-foreground">{account.devServerId}</TableCell>
              <TableCell className="text-sm capitalize">{account.scope}</TableCell>
              <TableCell><HealthStatusBadge status={account.status} /></TableCell>
              <TableCell><UsageChart accountId={account.id} /></TableCell>
              <TableCell>
                <div className="flex gap-1">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => testConnection(account.id)}
                    data-testid={`test-btn-${account.id}`}
                  >
                    <TestTube size={12} />
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => { setEditingAccount(account); setShowForm(true) }}
                    data-testid={`edit-btn-${account.id}`}
                  >
                    Edit
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
          {filtered.length === 0 && (
            <TableRow>
              <TableCell colSpan={7} className="text-center text-muted-foreground py-8">
                No accounts found
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>

      {/* Form dialog */}
      {showForm && (
        <Suspense>
          <ProviderForm
            account={editingAccount ?? undefined}
            onClose={() => { setShowForm(false); refresh() }}
          />
        </Suspense>
      )}
    </div>
  )
}
