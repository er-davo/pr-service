//go:build integration
// +build integration

package repository_test

import (
	"context"
	"fmt"
	"pr-service/internal/models"
	"pr-service/internal/repository"
	"testing"

	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUserRepository(t *testing.T) {
	ctx := t.Context()
	trManager := manager.Must(trmpgx.NewDefaultFactory(db))

	repo := repository.NewUserRepository(
		db,
		trmpgx.DefaultCtxGetter,
		retrier,
	)

	_ = trManager.Do(ctx, func(ctx context.Context) error {
		team := &models.Team{Name: "team1"}
		teamRepo := repository.NewTeamRepository(db, trmpgx.DefaultCtxGetter, retrier)
		err := teamRepo.Create(ctx, team)
		require.NoError(t, err)

		user := &models.User{
			Name:     "user1",
			TeamID:   &team.ID,
			IsActive: true,
		}

		t.Run("Create user", func(t *testing.T) {
			err := repo.Create(ctx, user)
			require.NoError(t, err)
			require.NotEqual(t, uuid.Nil, user.ID)
		})

		t.Run("GetUserByID", func(t *testing.T) {
			actual, err := repo.GetUserByID(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, user.ID, actual.ID)
			require.Equal(t, user.TeamID, actual.TeamID)
			require.Equal(t, user.Name, actual.Name)
			require.Equal(t, user.IsActive, actual.IsActive)
		})

		t.Run("GetActiveByTeam", func(t *testing.T) {
			users, err := repo.GetActiveByTeam(ctx, team.ID)
			require.NoError(t, err)
			require.Len(t, users, 1)
			require.Equal(t, user.ID, users[0].ID)
		})

		t.Run("UpdateActive", func(t *testing.T) {
			err := repo.UpdateActive(ctx, user.ID, false)
			require.NoError(t, err)

			u, err := repo.GetUserByID(ctx, user.ID)
			require.NoError(t, err)
			require.False(t, u.IsActive)
		})

		t.Run("GetUserByID NotFound", func(t *testing.T) {
			_, err := repo.GetUserByID(ctx, uuid.New())
			require.ErrorIs(t, err, repository.ErrNotFound)
		})

		return fmt.Errorf("rollback transaction")
	})
}
