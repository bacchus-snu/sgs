package postgres

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/bacchus-snu/sgs/model"
)

// Common utilities for reading workspaces from the db.

func queryWorkspace(ctx context.Context, tx pgx.Tx, id model.ID) (*model.Workspace, error) {
	wss, err := queryWorkspaces(ctx, tx, []model.ID{id})
	if err != nil {
		return nil, err
	}
	return wss[0], nil
}

func queryWorkspaces(ctx context.Context, tx pgx.Tx, ids []model.ID) ([]*model.Workspace, error) {
	rows, err := tx.Query(ctx, `SELECT id, created, enabled, userdata FROM workspaces WHERE id = ANY($1) ORDER BY id`, ids)
	if err != nil {
		return nil, err
	}

	wss, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[model.Workspace])
	if err != nil {
		return nil, err
	}
	if len(wss) != len(ids) {
		return nil, model.ErrNotFound
	}

	// mapping for quick lookup
	wsind := make(map[model.ID]*model.Workspace, len(wss))
	for _, ws := range wss {
		wsind[ws.ID] = ws
	}

	if err := fillAccessTypes(ctx, tx, ids, wsind); err != nil {
		return nil, err
	}
	if err := fillQuotas(ctx, tx, ids, wsind); err != nil {
		return nil, err
	}
	if err := fillUsers(ctx, tx, ids, wsind); err != nil {
		return nil, err
	}
	if err := fillRequests(ctx, tx, ids, wsind); err != nil {
		return nil, err
	}

	return wss, nil
}

func fillAccessTypes(ctx context.Context, tx pgx.Tx, ids []model.ID, wsind map[model.ID]*model.Workspace) error {
	rows, err := tx.Query(ctx, `SELECT workspace_id, access_type FROM workspaces_access WHERE workspace_id = ANY($1) ORDER BY workspace_id, access_type`, ids)
	if err != nil {
		return err
	}

	var (
		id         model.ID
		accessType model.AccessType
	)
	_, err = pgx.ForEachRow(rows, []any{&id, &accessType}, func() error {
		wsind[id].AccessTypes = append(wsind[id].AccessTypes, accessType)
		return nil
	})
	return err
}

func fillQuotas(ctx context.Context, tx pgx.Tx, ids []model.ID, wsind map[model.ID]*model.Workspace) error {
	rows, err := tx.Query(ctx, `SELECT workspace_id, resource, quantity FROM workspaces_quotas WHERE workspace_id = ANY($1)`, ids)
	if err != nil {
		return err
	}

	var (
		id       model.ID
		resource model.Resource
		quantity uint64
	)
	_, err = pgx.ForEachRow(rows, []any{&id, &resource, &quantity}, func() error {
		if wsind[id].Quotas == nil {
			wsind[id].Quotas = map[model.Resource]uint64{}
		}
		wsind[id].Quotas[resource] = quantity
		return nil
	})
	return err
}

func fillUsers(ctx context.Context, tx pgx.Tx, idx []model.ID, wsind map[model.ID]*model.Workspace) error {
	rows, err := tx.Query(ctx, `SELECT workspace_id, username FROM workspaces_users WHERE workspace_id = ANY($1) ORDER BY username`, idx)
	if err != nil {
		return err
	}

	var (
		id   model.ID
		user string
	)
	_, err = pgx.ForEachRow(rows, []any{&id, &user}, func() error {
		wsind[id].Users = append(wsind[id].Users, user)
		return nil
	})
	return err
}

func fillRequests(ctx context.Context, tx pgx.Tx, idx []model.ID, wsind map[model.ID]*model.Workspace) error {
	rows, err := tx.Query(ctx, `SELECT workspace_id, data FROM workspaces_updaterequests WHERE workspace_id = ANY($1)`, idx)
	if err != nil {
		return err
	}

	var (
		id   model.ID
		data string
	)
	_, err = pgx.ForEachRow(rows, []any{&id, &data}, func() error {
		upd := model.WorkspaceUpdate{}
		if err := json.Unmarshal([]byte(data), &upd); err != nil {
			return err
		}
		slices.Sort(upd.Users)
		wsind[id].Request = &upd
		return nil
	})
	return err
}
