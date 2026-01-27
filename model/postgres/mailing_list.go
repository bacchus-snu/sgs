package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bacchus-snu/sgs/model"
)

type mailingListRepository struct {
	pool *pgxpool.Pool
}

func (r *mailingListRepository) Subscribe(ctx context.Context, username, email string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO mailing_list (username, email)
		VALUES ($1, $2)
		ON CONFLICT (username) DO UPDATE SET email = $2
	`, username, email)
	return err
}

func (r *mailingListRepository) Unsubscribe(ctx context.Context, username string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM mailing_list WHERE username = $1
	`, username)
	return err
}

func (r *mailingListRepository) IsSubscribed(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM mailing_list WHERE username = $1)
	`, username).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *mailingListRepository) ListSubscribers(ctx context.Context) ([]model.Subscriber, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT username, email FROM mailing_list ORDER BY username
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subscribers, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Subscriber])
	if err != nil {
		return nil, err
	}
	return subscribers, nil
}
