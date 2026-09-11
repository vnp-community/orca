ALTER TABLE dev_servers
    DROP COLUMN approval_status,
    DROP COLUMN group_id;

DROP TABLE dev_server_groups;
