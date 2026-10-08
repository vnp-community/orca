BEGIN;
UPDATE project.projects 
SET tenant_id = '8cc62317-8421-43b4-b9d8-38e5fe84bacb', 
    created_by = 'f2b7b7b6-fafd-479a-8610-7061fb7e8311' 
WHERE tenant_id = '00000000-0000-0000-0000-000000000001';

DELETE FROM project.project_members WHERE user_id != 'f2b7b7b6-fafd-479a-8610-7061fb7e8311';
INSERT INTO project.project_members (project_id, user_id, role) 
SELECT id, 'f2b7b7b6-fafd-479a-8610-7061fb7e8311', 'owner' 
FROM project.projects 
WHERE tenant_id = '8cc62317-8421-43b4-b9d8-38e5fe84bacb'
ON CONFLICT DO NOTHING;

DELETE FROM project.repo_members WHERE user_id != 'f2b7b7b6-fafd-479a-8610-7061fb7e8311';
INSERT INTO project.repo_members (repo_id, user_id, functional_role) 
SELECT r.id, 'f2b7b7b6-fafd-479a-8610-7061fb7e8311', 'admin' 
FROM project.repos r 
JOIN project.projects p ON p.id = r.project_id
WHERE p.tenant_id = '8cc62317-8421-43b4-b9d8-38e5fe84bacb'
ON CONFLICT DO NOTHING;
COMMIT;
