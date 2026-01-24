-- Create workspaces_access table for many-to-many relationship
CREATE TABLE IF NOT EXISTS workspaces_access (
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    access_type TEXT NOT NULL,
    PRIMARY KEY (workspace_id, access_type)
);

-- Migrate existing nodegroup data to new table
INSERT INTO workspaces_access (workspace_id, access_type)
SELECT id, nodegroup FROM workspaces WHERE nodegroup IS NOT NULL;

-- Drop old nodegroup column (clean break, no compatibility)
ALTER TABLE workspaces DROP COLUMN nodegroup;
