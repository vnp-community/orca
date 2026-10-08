-- Every writer of solutions now assigns seq (insertSolutionRow), so the column can stop being optional.
ALTER TABLE request.solutions NO FORCE ROW LEVEL SECURITY;
UPDATE request.solutions s SET seq = n.rn
FROM (SELECT id, ROW_NUMBER() OVER (PARTITION BY tenant_id, request_id ORDER BY created_at, id) AS rn FROM request.solutions s2 WHERE seq IS NULL) n
WHERE s.id = n.id AND s.seq IS NULL;
ALTER TABLE request.solutions FORCE ROW LEVEL SECURITY;
ALTER TABLE request.solutions ALTER COLUMN seq SET NOT NULL;
