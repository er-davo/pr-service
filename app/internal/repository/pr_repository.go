package repository

import (
	"context"
	"pr-service/internal/models"
	"pr-service/internal/retry"
	"time"

	sq "github.com/Masterminds/squirrel"
	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PRRepository struct {
	db      *pgxpool.Pool
	getter  *trmpgx.CtxGetter
	psql    sq.StatementBuilderType
	retrier retry.Retrier
}

func NewPRRepository(db *pgxpool.Pool, c *trmpgx.CtxGetter, r retry.Retrier) *PRRepository {
	return &PRRepository{
		db:      db,
		getter:  c,
		psql:    sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
		retrier: r,
	}
}

func (r *PRRepository) Create(ctx context.Context, pr *models.PullRequest) error {
	query := r.psql.Insert("pull_requests").
		Columns("id", "name", "author_id", "status", "created_at").
		Values(pr.ID, pr.Name, pr.AuthorID, pr.Status, pr.CreatedAt)
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

func (r *PRRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PullRequest, error) {
	query := r.psql.Select(
		"pr.id",
		"pr.name",
		"pr.author_id",
		"pr.status",
		"pr.created_at",
		"pr.merged_at",
		"r.id",
		"r.assigned_at",
	).From("pull_requests pr").
		LeftJoin("pr_reviewers r ON r.pull_request_id = pr.id").
		Where(sq.Eq{"pr.id": id})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	pr := &models.PullRequest{
		Reviewers: make([]*models.PRReviewer, 0),
	}

	err = r.retrier.Do(ctx, func() error {
		rows, err := conn.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var reviewerID *uuid.UUID
			var assignedAt *time.Time
			err := rows.Scan(
				&pr.ID,
				&pr.Name,
				&pr.AuthorID,
				&pr.Status,
				&pr.CreatedAt,
				&pr.MergedAt,
				&reviewerID,
				&assignedAt,
			)
			if err != nil {
				return err
			}

			if reviewerID != nil && assignedAt != nil {
				pr.Reviewers = append(pr.Reviewers, &models.PRReviewer{
					ID:         *reviewerID,
					PRID:       pr.ID,
					AssignedAt: *assignedAt,
				})
			}
		}
		return nil
	})

	return pr, wrapDBError(err)
}

func (r *PRRepository) AssignReviewers(ctx context.Context, prID uuid.UUID, reviewers []uuid.UUID) error {
	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	now := time.Now()

	err := r.retrier.Do(ctx, func() error {
		delSQL, delArgs, err := r.psql.
			Delete("pr_reviewers").
			Where(sq.Eq{"pull_request_id": prID}).
			ToSql()
		if err != nil {
			return err
		}

		if _, err := conn.Exec(ctx, delSQL, delArgs...); err != nil {
			return err
		}

		batch := &pgx.Batch{}
		for _, reviewerID := range reviewers {
			sql, args, err := r.psql.
				Insert("pr_reviewers").
				Columns("id", "pull_request_id", "assigned_at").
				Values(reviewerID, prID, now).
				ToSql()
			if err != nil {
				return err
			}

			batch.Queue(sql, args...)
		}

		br := conn.SendBatch(ctx, batch)

		return br.Close()
	})

	return wrapDBError(err)
}

func (r *PRRepository) ReplaceReviewer(ctx context.Context, prID, oldID, newID uuid.UUID) error {
	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	now := time.Now()

	err := r.retrier.Do(ctx, func() error {
		delSQL, delArgs, err := r.psql.
			Delete("pr_reviewers").
			Where(sq.Eq{
				"id":              oldID,
				"pull_request_id": prID,
			}).
			ToSql()
		if err != nil {
			return err
		}

		tag, err := conn.Exec(ctx, delSQL, delArgs...)
		if err != nil {
			return err
		}

		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}

		insertSQL, insertArgs, err := r.psql.
			Insert("pr_reviewers").
			Columns("id", "pull_request_id", "assigned_at").
			Values(newID, prID, now).
			Suffix("ON CONFLICT DO NOTHING").
			ToSql()
		if err != nil {
			return err
		}

		_, err = conn.Exec(ctx, insertSQL, insertArgs...)
		return err
	})

	return wrapDBError(err)
}

func (r *PRRepository) Merge(ctx context.Context, id uuid.UUID) error {
	query := r.psql.Update("pull_requests").
		Set("status", string(models.PRStatusMerged)).
		Set("merged_at", time.Now()).
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

func (r *PRRepository) ListByReviewer(ctx context.Context, id uuid.UUID) ([]*models.PullRequest, error) {
	query := r.psql.Select(
		"pr.id", "pr.name", "pr.author_id",
		"pr.status", "pr.created_at", "pr.merged_at",
	).From("pull_requests pr").
		Join("pr_reviewers r ON r.pull_request_id = pr.id").
		Where(sq.Eq{"r.id": id})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	prs := make([]*models.PullRequest, 0)

	err = r.retrier.Do(ctx, func() error {
		rows, err := conn.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		pr := &models.PullRequest{}
		for rows.Next() {
			if err := rows.Scan(
				&pr.ID,
				&pr.Name,
				&pr.AuthorID,
				&pr.Status,
				&pr.CreatedAt,
				&pr.MergedAt,
			); err != nil {
				return err
			}
			prs = append(prs, pr)
			pr = &models.PullRequest{}
		}

		return rows.Err()
	})

	return prs, wrapDBError(err)
}
