-- Backfills a tenant.companies row for a legacy bootstrap admin account
-- whose auth.users.tenant_id predates auth-service's current
-- Bootstrap.EnsureAdmin saga (which correctly originates a fresh tenant_id
-- via tenant-service.CreateCompany before creating the admin user — see
-- auth-service/internal/usecase/bootstrap.go's doc comment). This specific
-- deployment's admin account was seeded with the well-known sentinel
-- tenant_id 00000000-0000-0000-0000-000000000001 through an older/manual
-- path that never created a matching company row, leaving every
-- department/company operation for that tenant with nothing to attach to
-- (live-verified: 0 rows in tenant.companies, 0 departments possible,
-- CR-DS-007's "Grant a department access" picker permanently empty).
--
-- ON CONFLICT DO NOTHING makes this a no-op everywhere else: a fresh
-- deployment's Bootstrap.EnsureAdmin already creates its own company with a
-- randomly generated tenant_id, so this sentinel id essentially never
-- collides with anything real outside this specific legacy case.
INSERT INTO tenant.companies (id, name)
VALUES ('00000000-0000-0000-0000-000000000001', 'Legacy Bootstrap Company')
ON CONFLICT (id) DO NOTHING;
