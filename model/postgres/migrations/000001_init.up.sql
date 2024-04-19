CREATE TABLE IF NOT EXISTS workspaces (
	id BIGSERIAL PRIMARY KEY,
	created BOOLEAN NOT NULL DEFAULT FALSE,
	enabled BOOLEAN NOT NULL DEFAULT FALSE,
	nodegroup TEXT NOT NULL,
	userdata TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workspaces_quotas (
	workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	resource TEXT NOT NULL,
	quantity BIGINT NOT NULL,
	PRIMARY KEY (workspace_id, resource)
);

CREATE TABLE IF NOT EXISTS workspaces_users (
	workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	username TEXT NOT NULL,
	PRIMARY KEY (workspace_id, username)
);

CREATE TABLE IF NOT EXISTS workspaces_updaterequests (
	workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	by_user TEXT NOT NULL,
	data JSONB NOT NULL,
	PRIMARY KEY (workspace_id)
);
