package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"slices"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"

	"github.com/bacchus-snu/sgs/model"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func openMigrations(connString string) (*migrate.Migrate, error) {
	migSource, err := iofs.New(migrationFS, "migrations")
	if err != nil {
		return nil, err
	}

	migDB, err := (&migratepgx.Postgres{}).Open(connString)
	if err != nil {
		return nil, err
	}

	mig, err := migrate.NewWithInstance("iofs", migSource, "pgx/v5", migDB)
	if err != nil {
		return nil, err
	}

	return mig, nil
}

type Config struct {
	ConnString string `mapstructure:"conn_string"`
}

func (c *Config) Bind() {
	viper.BindEnv("postgres.conn_string", "SGS_POSTGRES_CONN_STRING")
}

func (c *Config) Validate() error {
	if c.ConnString == "" {
		return errors.New("conn_string is required")
	}
	return nil
}

type Repository struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, cfg Config) (*Repository, error) {
	mig, err := openMigrations(cfg.ConnString)
	if err != nil {
		return nil, fmt.Errorf("opening migrations: %w", err)
	}
	defer mig.Close()

	if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, fmt.Errorf("applying migrations: %w", err)
	}

	pool, err := pgxpool.New(ctx, cfg.ConnString)
	if err != nil {
		return nil, fmt.Errorf("opening pool: %w", err)
	}

	// check if the pool is working
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging pool: %w", err)
	}

	return &Repository{pool}, nil
}

func (r *Repository) Close() error {
	r.pool.Close()
	return nil
}

func (r *Repository) Workspaces() *workspacesRepository {
	return &workspacesRepository{r.pool}
}

type workspacesRepository struct {
	pool *pgxpool.Pool
}

func (svc *workspacesRepository) CreateWorkspace(ctx context.Context, ws *model.Workspace) (*model.Workspace, error) {
	if !ws.Valid() {
		return nil, model.ErrInvalid
	}

	var newWs *model.Workspace
	err := pgx.BeginFunc(ctx, svc.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `INSERT INTO workspaces (nodegroup, userdata) VALUES ($1, $2) RETURNING id`,
			ws.Nodegroup, ws.Userdata)
		if err != nil {
			return err
		}
		id, err := pgx.CollectExactlyOneRow(rows, pgx.RowTo[model.ID])
		if err != nil {
			return err
		}

		for _, user := range ws.Users {
			_, err = tx.Exec(ctx, `INSERT INTO workspaces_users (workspace_id, username) VALUES ($1, $2)`,
				id, user)
			if err != nil {
				return err
			}
		}

		for resource, quantity := range ws.Quotas {
			_, err = tx.Exec(ctx, `INSERT INTO workspaces_quotas (workspace_id, resource, quantity) VALUES ($1, $2, $3)`,
				id, resource, quantity)
			if err != nil {
				return err
			}
		}

		upd := ws.InitialRequest()
		upd.WorkspaceID = id
		tx.Exec(ctx, `
			INSERT INTO workspaces_updaterequests  (workspace_id, by_user, data)
			VALUES ($1, $2, $3)
			ON CONFLICT (workspace_id) DO UPDATE
			SET by_user = EXCLUDED.by_user, data = EXCLUDED.data`,
			upd.WorkspaceID, upd.ByUser, upd)

		// we could reconstruct the ws here, but it's easier to just query it
		newWs, err = queryWorkspace(ctx, tx, id)
		return err
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}

	return newWs, nil
}

func (svc *workspacesRepository) ListAllWorkspaces(ctx context.Context) ([]*model.Workspace, error) {
	var wss []*model.Workspace
	err := pgx.BeginFunc(ctx, svc.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id FROM workspaces ORDER BY id DESC`)
		if err != nil {
			return err
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[model.ID])
		if err != nil {
			return err
		}

		wss, err = queryWorkspaces(ctx, tx, ids)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}

	return wss, nil
}

func (svc *workspacesRepository) ListUserWorkspaces(ctx context.Context, user string) ([]*model.Workspace, error) {
	var wss []*model.Workspace
	err := pgx.BeginFunc(ctx, svc.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT workspace_id FROM workspaces_users WHERE username = $1 ORDER BY workspace_id DESC`,
			user)
		if err != nil {
			return err
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[model.ID])
		if err != nil {
			return err
		}

		wss, err = queryWorkspaces(ctx, tx, ids)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}

	return wss, nil
}

func (svc *workspacesRepository) ListCreatedWorkspaces(ctx context.Context) ([]*model.Workspace, error) {
	var wss []*model.Workspace
	err := pgx.BeginFunc(ctx, svc.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id FROM workspaces WHERE created = true ORDER BY id DESC`)
		if err != nil {
			return err
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[model.ID])
		if err != nil {
			return err
		}

		wss, err = queryWorkspaces(ctx, tx, ids)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}

	return wss, nil
}

func (svc *workspacesRepository) GetWorkspace(ctx context.Context, id model.ID) (*model.Workspace, error) {
	var ws *model.Workspace
	err := pgx.BeginFunc(ctx, svc.pool, func(tx pgx.Tx) error {
		var err error
		ws, err = queryWorkspace(ctx, tx, id)
		return err
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}

	return ws, nil
}

func (svc *workspacesRepository) GetUserWorkspace(ctx context.Context, id model.ID, user string) (*model.Workspace, error) {
	var ws *model.Workspace
	err := pgx.BeginFunc(ctx, svc.pool, func(tx pgx.Tx) error {
		var err error

		rows, err := tx.Query(ctx, `SELECT workspace_id FROM workspaces_users WHERE workspace_id = $1 AND username = $2`,
			id, user)
		if err != nil {
			return err
		}
		id, err := pgx.CollectExactlyOneRow(rows, pgx.RowTo[model.ID])
		if err != nil {
			return err
		}

		ws, err = queryWorkspace(ctx, tx, id)
		return err
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}

	return ws, nil
}

func (svc *workspacesRepository) UpdateWorkspace(ctx context.Context, upd *model.WorkspaceUpdate) (*model.Workspace, error) {
	if !upd.Valid() {
		return nil, model.ErrInvalid
	}

	var ws *model.Workspace
	err := pgx.BeginFunc(ctx, svc.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE workspaces SET created = created OR $2, enabled = $2, nodegroup = $3, userdata = $4 WHERE id = $1`,
			upd.WorkspaceID, upd.Enabled, upd.Nodegroup, upd.Userdata)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return model.ErrNotFound
		}

		quotas := make([]model.Resource, 0, len(upd.Quotas))
		for quota := range upd.Quotas {
			quotas = append(quotas, quota)
		}
		_, err = tx.Exec(ctx, `DELETE FROM workspaces_quotas WHERE workspace_id = $1 AND resource != ALL($2)`,
			upd.WorkspaceID, quotas)
		if err != nil {
			return err
		}

		for resource, quantity := range upd.Quotas {
			_, err = tx.Exec(ctx, `
				INSERT INTO workspaces_quotas (workspace_id, resource, quantity)
				VALUES ($1, $2, $3)
				ON CONFLICT (workspace_id, resource) DO UPDATE
				SET quantity = EXCLUDED.quantity`,
				upd.WorkspaceID, resource, quantity)
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(ctx, `DELETE FROM workspaces_users WHERE workspace_id = $1 AND username != ALL($2)`,
			upd.WorkspaceID, upd.Users)
		if err != nil {
			return err
		}

		for _, user := range upd.Users {
			_, err = tx.Exec(ctx, `
				INSERT INTO workspaces_users (workspace_id, username)
				VALUES ($1, $2)
				ON CONFLICT (workspace_id, username) DO NOTHING`,
				upd.WorkspaceID, user)
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(ctx, `DELETE FROM workspaces_updaterequests WHERE workspace_id = $1`, upd.WorkspaceID)
		if err != nil {
			return err
		}

		ws, err = queryWorkspace(ctx, tx, upd.WorkspaceID)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return ws, err
}

func (svc *workspacesRepository) RequestUpdateWorkspace(ctx context.Context, upd *model.WorkspaceUpdate) (*model.Workspace, error) {
	if !upd.Valid() {
		return nil, model.ErrInvalid
	}
	if upd.ByUser == "" || !slices.Contains(upd.Users, upd.ByUser) {
		return nil, model.ErrInvalid
	}

	var ws *model.Workspace
	err := pgx.BeginFunc(ctx, svc.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT 1 FROM workspaces_users WHERE workspace_id = $1 AND username = $2`,
			upd.WorkspaceID, upd.ByUser)
		if err != nil {
			return err
		}
		if _, err := pgx.CollectExactlyOneRow(rows, pgx.RowTo[model.ID]); err != nil {
			return model.ErrNotFound
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO workspaces_updaterequests  (workspace_id, by_user, data)
			VALUES ($1, $2, $3)
			ON CONFLICT (workspace_id) DO UPDATE
			SET by_user = EXCLUDED.by_user, data = EXCLUDED.data`,
			upd.WorkspaceID, upd.ByUser, upd)
		if err != nil {
			if pgerr := (*pgconn.PgError)(nil); errors.As(err, &pgerr) && pgerr.Code == pgerrcode.ForeignKeyViolation {
				return model.ErrNotFound
			}
			return err
		}

		ws, err = queryWorkspace(ctx, tx, upd.WorkspaceID)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return ws, err
}

func (svc *workspacesRepository) DeleteWorkspace(ctx context.Context, id model.ID) error {
	err := pgx.BeginFunc(ctx, svc.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return model.ErrNotFound
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrNotFound
	}
	return err
}
