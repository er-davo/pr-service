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

type UserRepository struct {
	db      *pgxpool.Pool
	getter  *trmpgx.CtxGetter
	psql    sq.StatementBuilderType
	retrier retry.Retrier
}

func NewUserRepository(db *pgxpool.Pool, c *trmpgx.CtxGetter, r retry.Retrier) *UserRepository {
	return &UserRepository{
		db:      db,
		getter:  c,
		psql:    sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
		retrier: r,
	}
}

func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := r.psql.Select(
		"id", "team_id", "name", "is_active",
	).From("users").
		Where(sq.Eq{"id": id})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	u := &models.User{}

	err = r.retrier.Do(ctx, func() error {
		return conn.QueryRow(ctx, sql, args...).
			Scan(&u.ID, &u.TeamID, &u.Name, &u.IsActive)
	})

	return u, wrapDBError(err)
}

func (r *UserRepository) GetActiveByTeam(ctx context.Context, teamID uuid.UUID) ([]*models.User, error) {
	return r.getUsersBy(ctx, sq.Eq{
		"team_id":   teamID,
		"is_active": true,
	})
}

func (r *UserRepository) GetByTeam(ctx context.Context, teamID uuid.UUID) ([]*models.User, error) {
	return r.getUsersBy(ctx, sq.Eq{"team_id": teamID})

}

func (r *UserRepository) getUsersBy(ctx context.Context, where sq.Eq) ([]*models.User, error) {
	query := r.psql.Select(
		"id", "team_id", "name", "is_active",
	).From("users").
		Where(where)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	users := make([]*models.User, 0)

	err = r.retrier.Do(ctx, func() error {
		rows, err := conn.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		u := &models.User{}

		for rows.Next() {
			if err := rows.Scan(
				&u.ID, &u.TeamID, &u.Name, &u.IsActive,
			); err != nil {
				return err
			}

			users = append(users, u)
			u = &models.User{}
		}

		return nil
	})

	return users, wrapDBError(err)
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	query := r.psql.Insert("users").
		Columns("team_id", "name", "is_active").
		Values(user.TeamID, user.Name, user.IsActive).
		Suffix("RETURNING id")

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)

	err = r.retrier.Do(ctx, func() error {
		return conn.QueryRow(ctx, sql, args...).Scan(&user.ID)
	})

	return wrapDBError(err)
}

func (r *UserRepository) UpdateActive(ctx context.Context, id uuid.UUID, active bool) error {
	query := r.psql.Update("users").
		Set("is_active", active).
		Where(sq.Eq{"id": id})

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)

	err = r.retrier.Do(ctx, func() error {
		_, retryErr := conn.Exec(ctx, sql, args...)
		return retryErr
	})

	return wrapDBError(err)
}
