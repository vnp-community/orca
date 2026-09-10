-- Only removes the row this migration itself would have inserted — never
-- touches a company that was later renamed/adopted for real use (name no
-- longer matches the placeholder this migration always writes).
DELETE FROM tenant.companies
WHERE id = '00000000-0000-0000-0000-000000000001'
  AND name = 'Legacy Bootstrap Company';
