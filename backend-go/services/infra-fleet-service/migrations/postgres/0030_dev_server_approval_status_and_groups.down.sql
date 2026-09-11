ALTER TABLE infra.dev_servers
    DROP COLUMN approval_status,
    DROP COLUMN group_id;

DROP TABLE infra.dev_server_groups;
