// CompanyTab + its NewCompanySection/CompanyListSection — split out of
// AdminOrgConsole.tsx (AGENTS.md max-lines budget).
//
// Real multi-company support (user-requested): "Create company" mints a
// brand-new, fully isolated tenant_id (a Company row's id IS the tenant_id
// in this domain) — see NewCompanySection's own doc comment for the caveat
// that still applies: there is no company-switcher UI anywhere, so a new
// company is only reachable by immediately creating its first admin user
// and logging in as them separately. This deployment's own originally-
// missing company row was backfilled by a migration
// (0002_backfill_legacy_bootstrap_company); renaming it is the Company
// tab's main form below.
import { useCallback, useEffect, useState } from 'react'
import { Building2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'sonner'
import type { TenantCompany } from '../../../../shared/tenant-user-profile-types'
import { CreateUserForm } from './admin-org-console-users-tab'

// NewCompanySection: creates an additional, fully isolated company
// (tenant_id) — genuinely useful once ListDevServersForUser/AdminUser flows
// support real multi-tenancy, but with a real caveat the UI states plainly:
// there is still no company-switcher anywhere, so the ONLY way to ever use
// a newly created company is to immediately create its first admin user
// here (below) and log in as them separately — this admin's own account
// stays on their current company. onCreated fires after the inline
// first-admin form is dismissed so the company list below can refresh —
// without this, a created company was only ever visible for the rest of
// that one session (nothing else ever listed tenant.companies).
function NewCompanySection({ onCreated }: { onCreated: () => void }): React.JSX.Element {
  const [name, setName] = useState('')
  const [creating, setCreating] = useState(false)
  const [created, setCreated] = useState<TenantCompany | null>(null)

  const handleCreate = useCallback(() => {
    if (!name.trim() || creating) {
      return
    }
    setCreating(true)
    window.api.tenantProfile
      .createCompany({ name: name.trim() })
      .then((company) => {
        setCreated(company)
        setName('')
        toast.success('Company created')
        onCreated()
      })
      .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
      .finally(() => setCreating(false))
  }, [name, creating, onCreated])

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-dashed border-border p-4">
      <div>
        <h3 className="text-sm font-semibold">New company</h3>
        <p className="text-xs text-muted-foreground">
          Creates a fully separate, isolated company (own departments, dev servers, users). There is
          no company switcher yet — your own account stays on your current company. Create the new
          company&apos;s first admin below immediately, or use the list below to find it again
          later.
        </p>
      </div>
      <div className="flex items-end gap-2">
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="New company name"
          className="max-w-xs"
        />
        <Button disabled={!name.trim() || creating} onClick={handleCreate}>
          {creating ? 'Creating…' : 'Create company'}
        </Button>
      </div>
      {created ? (
        <div className="flex flex-col gap-2 rounded-md bg-muted/50 p-3">
          <p className="text-sm">
            Created <span className="font-medium">{created.name}</span> — now create its first
            admin:
          </p>
          <CreateUserForm fixedTenantId={created.id} onCreated={() => setCreated(null)} />
        </div>
      ) : null}
    </div>
  )
}

// CompanyListSection: the browsable list of every company (profile.listCompanies)
// that was previously entirely missing — a company created via
// NewCompanySection had nowhere to be found again once its creation toast
// and inline admin-creation form were dismissed. Selecting a row loads it
// into the editable form above via onSelect.
function CompanyListSection({
  reloadToken,
  selectedId,
  onSelect
}: {
  reloadToken: number
  selectedId: string
  onSelect: (id: string) => void
}): React.JSX.Element {
  const [companies, setCompanies] = useState<TenantCompany[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    window.api.tenantProfile
      .listCompanies()
      .then(setCompanies)
      .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
      .finally(() => setLoading(false))
  }, [reloadToken])

  if (loading && companies.length === 0) {
    return <p className="text-sm text-muted-foreground">Loading companies…</p>
  }

  return (
    <div className="flex flex-col gap-2">
      <h3 className="text-sm font-semibold">All companies</h3>
      <div className="flex flex-col gap-1.5">
        {companies.map((c) => (
          <button
            key={c.id}
            type="button"
            onClick={() => onSelect(c.id)}
            className={`flex items-center justify-between rounded-md border px-3 py-2 text-left text-sm transition-colors ${
              c.id === selectedId
                ? 'border-primary bg-primary/10'
                : 'border-border hover:bg-muted/50'
            }`}
          >
            <span className="flex items-center gap-2">
              <Building2 className="size-3.5 text-muted-foreground" />
              {c.name}
            </span>
            <span className="font-mono text-[11px] text-muted-foreground">{c.id}</span>
          </button>
        ))}
      </div>
    </div>
  )
}

export function CompanyTab(): React.JSX.Element {
  const [selectedCompanyId, setSelectedCompanyId] = useState('')
  const [company, setCompany] = useState<TenantCompany | null>(null)
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [listReloadToken, setListReloadToken] = useState(0)

  useEffect(() => {
    setLoading(true)
    window.api.tenantProfile
      .getCompany(selectedCompanyId ? { id: selectedCompanyId } : undefined)
      .then((c) => {
        setCompany(c)
        setName(c.name)
        setSelectedCompanyId(c.id)
      })
      .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
      .finally(() => setLoading(false))
  }, [selectedCompanyId])

  const handleSave = useCallback(() => {
    if (!company || !name.trim() || saving) {
      return
    }
    setSaving(true)
    window.api.tenantProfile
      .updateCompany({ id: company.id, name: name.trim(), settingsJson: company.settingsJson })
      .then((updated) => {
        setCompany(updated)
        setListReloadToken((n) => n + 1)
        toast.success('Company updated')
      })
      .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
      .finally(() => setSaving(false))
  }, [company, name, saving])

  if (loading && !company) {
    return <p className="text-sm text-muted-foreground">Loading…</p>
  }

  if (!company) {
    return <p className="text-sm text-muted-foreground">No company found.</p>
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex max-w-sm flex-col gap-3">
        <div className="space-y-1">
          <label htmlFor="company-name" className="text-sm font-medium text-foreground">
            Company name
          </label>
          <Input id="company-name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <Button
          className="self-start"
          disabled={!name.trim() || name === company.name || saving}
          onClick={handleSave}
        >
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </div>
      <CompanyListSection
        reloadToken={listReloadToken}
        selectedId={company.id}
        onSelect={setSelectedCompanyId}
      />
      <NewCompanySection onCreated={() => setListReloadToken((n) => n + 1)} />
    </div>
  )
}
