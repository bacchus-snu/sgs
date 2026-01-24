-- Restore nodegroup column
ALTER TABLE workspaces ADD COLUMN nodegroup TEXT;

-- Restore data from first access type (best effort)
UPDATE workspaces w
SET nodegroup = (
    SELECT access_type
    FROM workspaces_access
    WHERE workspace_id = w.id
    ORDER BY access_type
    LIMIT 1
);

-- Drop new table
DROP TABLE IF EXISTS workspaces_access;
