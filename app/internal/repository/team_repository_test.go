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

func TestTeamRepository(t *testing.T) {
	ctx := t.Context()

	trManager := manager.Must(trmpgx.NewDefaultFactory(db))

	repo := repository.NewTeamRepository(
		db,
		trmpgx.DefaultCtxGetter,
		retrier,
	)

	_ = trManager.Do(ctx, func(ctx context.Context) error {
		team := &models.Team{
			Name: "team",
		}
		t.Run("Create", func(t *testing.T) {
			err := repo.Create(ctx, team)
			require.NoError(t, err)
			require.NotEqual(t, team.ID, uuid.Nil)
		})

		t.Run("GetByID", func(t *testing.T) {
			actual, err := repo.GetByID(ctx, team.ID)
			require.NoError(t, err)
			require.Equal(t, team, actual)
		})

		t.Run("GetByName", func(t *testing.T) {
			actual, err := repo.GetByName(ctx, team.Name)
			require.NoError(t, err)
			require.Equal(t, team, actual)
		})

		t.Run("Not found", func(t *testing.T) {
			_, err := repo.GetByID(ctx, uuid.New())
			require.ErrorIs(t, err, repository.ErrNotFound)
		})

		return fmt.Errorf("error for rollback")
	})
}
