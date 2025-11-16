//go:build integration
// +build integration

package repository_test

import (
	"context"
	"fmt"
	"pr-service/internal/models"
	"pr-service/internal/repository"
	"testing"
	"time"

	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPRRepository(t *testing.T) {
	ctx := t.Context()
	trManager := manager.Must(trmpgx.NewDefaultFactory(db))

	prRepo := repository.NewPRRepository(
		db,
		trmpgx.DefaultCtxGetter,
		retrier,
	)

	userRepo := repository.NewUserRepository(db, trmpgx.DefaultCtxGetter, retrier)
	teamRepo := repository.NewTeamRepository(db, trmpgx.DefaultCtxGetter, retrier)

	_ = trManager.Do(ctx, func(ctx context.Context) error {
		team := &models.Team{Name: "team"}
		err := teamRepo.Create(ctx, team)
		require.NoError(t, err)

		author := &models.User{
			Name:     "author",
			TeamID:   &team.ID,
			IsActive: true,
		}
		err = userRepo.Create(ctx, author)
		require.NoError(t, err)

		pr := &models.PullRequest{
			ID:        uuid.New(),
			Name:      "PR-1",
			AuthorID:  author.ID,
			Status:    string(models.PRStatusOpen),
			CreatedAt: time.Now(),
		}

		t.Run("Create PR", func(t *testing.T) {
			err := prRepo.Create(ctx, pr)
			require.NoError(t, err)
			require.NotEqual(t, uuid.Nil, pr.ID)
		})

		t.Run("GetByID", func(t *testing.T) {
			actual, err := prRepo.GetByID(ctx, pr.ID)
			require.NoError(t, err)
			require.Equal(t, pr.ID, actual.ID)
			require.Equal(t, pr.Name, actual.Name)
			require.Equal(t, pr.AuthorID, actual.AuthorID)
			require.Equal(t, pr.Status, actual.Status)
		})

		t.Run("AssignReviewers", func(t *testing.T) {
			r1 := &models.User{Name: "rev1", TeamID: &team.ID, IsActive: true}
			r2 := &models.User{Name: "rev2", TeamID: &team.ID, IsActive: true}
			require.NoError(t, userRepo.Create(ctx, r1))
			require.NoError(t, userRepo.Create(ctx, r2))

			err := prRepo.AssignReviewers(ctx, pr.ID, []uuid.UUID{r1.ID, r2.ID})
			require.NoError(t, err)

			fetched, err := prRepo.GetByID(ctx, pr.ID)
			require.NoError(t, err)
			require.Len(t, fetched.Reviewers, 2)
		})

		t.Run("ReplaceReviewer", func(t *testing.T) {
			r3 := &models.User{Name: "rev3", TeamID: &team.ID, IsActive: true}
			require.NoError(t, userRepo.Create(ctx, r3))

			fetchedPR, err := prRepo.GetByID(ctx, pr.ID)
			require.NoError(t, err)
			require.NotEmpty(t, fetchedPR.Reviewers)

			oldID := fetchedPR.Reviewers[0].ID

			err = prRepo.ReplaceReviewer(ctx, pr.ID, oldID, r3.ID)
			require.NoError(t, err)

			updated, err := prRepo.GetByID(ctx, pr.ID)
			require.NoError(t, err)

			found := false
			for _, rev := range updated.Reviewers {
				if rev.ID == r3.ID {
					found = true
					break
				}
			}
			require.True(t, found)
		})

		t.Run("Merge PR", func(t *testing.T) {
			err := prRepo.Merge(ctx, pr.ID)
			require.NoError(t, err)

			fetched, err := prRepo.GetByID(ctx, pr.ID)
			require.NoError(t, err)
			require.Equal(t, string(models.PRStatusMerged), fetched.Status)
			require.NotNil(t, fetched.MergedAt)
		})

		t.Run("ListByReviewer", func(t *testing.T) {
			fetchedPR, err := prRepo.GetByID(ctx, pr.ID)
			require.NoError(t, err)
			require.Len(t, fetchedPR.Reviewers, 2)

			reviewerID := fetchedPR.Reviewers[1].ID

			prs, err := prRepo.ListByReviewer(ctx, reviewerID)
			require.NoError(t, err)
			require.NotEmpty(t, prs)

			found := false
			for _, p := range prs {
				if p.ID == pr.ID {
					found = true
					break
				}
			}
			require.True(t, found)
		})

		return fmt.Errorf("rollback transaction")
	})
}
