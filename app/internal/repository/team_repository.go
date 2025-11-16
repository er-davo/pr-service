package repository

import (
	"context"
	"pr-service/internal/models"
	"pr-service/internal/retry"

	sq "github.com/Masterminds/squirrel"
	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TeamRepository struct {
	db      *pgxpool.Pool
	getter  *trmpgx.CtxGetter
	psql    sq.StatementBuilderType
	retrier retry.Retrier
}

func NewTeamRepository(db *pgxpool.Pool, c *trmpgx.CtxGetter, r retry.Retrier) *TeamRepository {
	return &TeamRepository{
		db:      db,
		getter:  c,
		psql:    sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
		retrier: r,
	}
}

func (r *TeamRepository) Create(ctx context.Context, t *models.Team) error {
	query := r.psql.Insert("teams").
		Columns("name").
		Values(t.Name).
		Suffix("RETURNING id")

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)

	err = r.retrier.Do(ctx, func() error {
		return conn.QueryRow(ctx, sql, args...).Scan(&t.ID)
	})

	return wrapDBError(err)
}

func (r *TeamRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	return r.getBy(ctx, sq.Eq{"id": id})
}

func (r *TeamRepository) GetByName(ctx context.Context, name string) (*models.Team, error) {
	return r.getBy(ctx, sq.Eq{"name": name})
}

func (r *TeamRepository) getBy(ctx context.Context, where sq.Eq) (*models.Team, error) {
	query := r.psql.Select("id", "name").
		From("teams").
		Where(where)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	t := &models.Team{}

	err = r.retrier.Do(ctx, func() error {
		return conn.QueryRow(ctx, sql, args...).Scan(&t.ID, &t.Name)
	})

	return t, wrapDBError(err)
}
