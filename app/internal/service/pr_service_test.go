package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"pr-service/internal/mocks"
	"pr-service/internal/models"
	"pr-service/internal/repository"
	"pr-service/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestPRService_CreatePR(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	teamRepo := mocks.NewMockTeamRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	prRepo := mocks.NewMockPRRepository(ctrl)
	tx := service.TxManagerStub{}

	svc := service.NewPRService(
		teamRepo,
		userRepo,
		prRepo,
		tx,
		zap.NewNop(),
	)

	ctx := t.Context()
	prID := uuid.New()
	authorID := uuid.New()
	teamID := uuid.New()
	newPR := &models.PullRequest{
		ID:        prID,
		AuthorID:  authorID,
		Reviewers: []*models.PRReviewer{},
	}

	t.Run("PR creation fails", func(t *testing.T) {
		prRepo.EXPECT().
			Create(ctx, newPR).
			Return(errors.New("db error"))

		err := svc.CreatePR(ctx, newPR)
		require.Error(t, err)
	})

	t.Run("author not found", func(t *testing.T) {
		prRepo.EXPECT().
			Create(ctx, newPR).
			Return(nil)
		userRepo.EXPECT().
			GetUserByID(ctx, authorID).
			Return(nil, repository.ErrNotFound)

		err := svc.CreatePR(ctx, newPR)
		require.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("active users fetch fails", func(t *testing.T) {
		prRepo.EXPECT().
			Create(ctx, newPR).
			Return(nil)
		userRepo.EXPECT().
			GetUserByID(ctx, authorID).
			Return(&models.User{ID: authorID, TeamID: &teamID}, nil)
		userRepo.EXPECT().
			GetActiveByTeam(ctx, teamID).
			Return(nil, errors.New("db error"))

		err := svc.CreatePR(ctx, newPR)
		require.Error(t, err)
	})

	t.Run("assign reviewers fails", func(t *testing.T) {
		prRepo.EXPECT().
			Create(ctx, newPR).
			Return(nil)
		userRepo.EXPECT().
			GetUserByID(ctx, authorID).
			Return(&models.User{ID: authorID, TeamID: &teamID}, nil)
		userRepo.EXPECT().
			GetActiveByTeam(ctx, teamID).
			Return([]*models.User{
				{ID: uuid.New(), TeamID: &teamID, IsActive: true},
				{ID: uuid.New(), TeamID: &teamID, IsActive: true},
			}, nil)
		prRepo.EXPECT().
			AssignReviewers(ctx, prID, gomock.Any()).
			Return(errors.New("assign error"))

		err := svc.CreatePR(ctx, newPR)
		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		activeUsers := []*models.User{
			{ID: uuid.New(), TeamID: &teamID, IsActive: true},
			{ID: uuid.New(), TeamID: &teamID, IsActive: true},
			{ID: uuid.New(), TeamID: &teamID, IsActive: true},
		}
		prRepo.EXPECT().
			Create(ctx, newPR).
			Return(nil)
		userRepo.EXPECT().
			GetUserByID(ctx, authorID).
			Return(&models.User{ID: authorID, TeamID: &teamID}, nil)
		userRepo.EXPECT().
			GetActiveByTeam(ctx, teamID).
			Return(activeUsers, nil)
		prRepo.EXPECT().
			AssignReviewers(ctx, prID, gomock.Any()).
			Return(nil)

		err := svc.CreatePR(ctx, newPR)
		require.NoError(t, err)
	})
}

func TestPRService_PRMerge(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	prRepo := mocks.NewMockPRRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	teamRepo := mocks.NewMockTeamRepository(ctrl)
	tx := service.TxManagerStub{}

	svc := service.NewPRService(
		teamRepo,
		userRepo,
		prRepo,
		tx,
		zap.NewNop(),
	)
	ctx := t.Context()
	prID := uuid.New()

	t.Run("GetByID error", func(t *testing.T) {
		prRepo.EXPECT().
			GetByID(ctx, prID).
			Return(nil, errors.New("db error"))
		_, err := svc.PRMerge(ctx, prID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "db error")
	})

	t.Run("already merged", func(t *testing.T) {
		pr := &models.PullRequest{ID: prID, Status: string(models.PRStatusMerged)}
		prRepo.EXPECT().
			GetByID(ctx, prID).
			Return(pr, nil)
		result, err := svc.PRMerge(ctx, prID)
		require.NoError(t, err)
		require.Equal(t, pr, result)
	})

	t.Run("merge fails", func(t *testing.T) {
		pr := &models.PullRequest{ID: prID, Status: string(models.PRStatusOpen)}
		prRepo.EXPECT().
			GetByID(ctx, prID).
			Return(pr, nil)
		prRepo.EXPECT().
			Merge(ctx, prID).
			Return(errors.New("merge failed"))
		result, err := svc.PRMerge(ctx, prID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "merge failed")
		require.Nil(t, result)
	})

	t.Run("success merge", func(t *testing.T) {
		pr := &models.PullRequest{ID: prID, Status: string(models.PRStatusOpen)}
		prRepo.EXPECT().
			GetByID(ctx, prID).
			Return(pr, nil)
		prRepo.EXPECT().
			Merge(ctx, prID).
			Return(nil)
		result, err := svc.PRMerge(ctx, prID)
		require.NoError(t, err)
		require.Equal(t, pr, result)
	})
}

func TestPRService_PRReassign(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	teamRepo := mocks.NewMockTeamRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	prRepo := mocks.NewMockPRRepository(ctrl)
	tx := service.TxManagerStub{}

	svc := service.NewPRService(
		teamRepo,
		userRepo,
		prRepo,
		tx,
		zap.NewNop(),
	)

	ctx := context.Background()
	prID := uuid.New()
	oldUserID := uuid.New()
	authorID := uuid.New()
	newUserID := uuid.New()
	teamID := uuid.New()

	basePR := &models.PullRequest{
		ID:       prID,
		AuthorID: authorID,
		Status:   string(models.PRStatusOpen),
		Reviewers: []*models.PRReviewer{
			{ID: oldUserID, PRID: prID, AssignedAt: time.Now()},
		},
	}

	t.Run("PR not found", func(t *testing.T) {
		prRepo.EXPECT().GetByID(ctx, prID).Return(nil, repository.ErrNotFound)

		pr, err := svc.PRReassign(ctx, prID, oldUserID)
		require.Nil(t, pr)
		require.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("PR already merged", func(t *testing.T) {
		mergedPR := *basePR
		mergedPR.Status = string(models.PRStatusMerged)
		prRepo.EXPECT().GetByID(ctx, prID).Return(&mergedPR, nil)

		pr, err := svc.PRReassign(ctx, prID, oldUserID)
		require.Nil(t, pr)
		require.ErrorIs(t, err, service.ErrCanNotReassing)
	})

	t.Run("old reviewer not assigned", func(t *testing.T) {
		prWithoutOld := *basePR
		prWithoutOld.Reviewers = []*models.PRReviewer{}
		prRepo.EXPECT().GetByID(ctx, prID).Return(&prWithoutOld, nil)

		pr, err := svc.PRReassign(ctx, prID, oldUserID)
		require.Nil(t, pr)
		require.ErrorIs(t, err, service.ErrNotAssinged)
	})

	t.Run("no replacement reviewer available", func(t *testing.T) {
		prRepo.EXPECT().GetByID(ctx, prID).Return(basePR, nil)
		userRepo.EXPECT().GetUserByID(ctx, authorID).Return(&models.User{ID: authorID, TeamID: &teamID}, nil)
		userRepo.EXPECT().GetActiveByTeam(ctx, teamID).Return([]*models.User{
			{ID: oldUserID, TeamID: &teamID, IsActive: true},
		}, nil)

		pr, err := svc.PRReassign(ctx, prID, oldUserID)
		require.Nil(t, pr)
		require.ErrorIs(t, err, service.ErrNoAvailableReviewer)
	})

	t.Run("success", func(t *testing.T) {
		prRepo.EXPECT().GetByID(ctx, prID).Return(basePR, nil)
		userRepo.EXPECT().GetUserByID(ctx, authorID).Return(&models.User{ID: authorID, TeamID: &teamID}, nil)
		userRepo.EXPECT().GetActiveByTeam(ctx, teamID).Return([]*models.User{
			{ID: oldUserID, TeamID: &teamID, IsActive: true},
			{ID: newUserID, TeamID: &teamID, IsActive: true},
		}, nil)
		prRepo.EXPECT().ReplaceReviewer(ctx, prID, oldUserID, newUserID).Return(nil)

		result, err := svc.PRReassign(ctx, prID, oldUserID)
		require.NoError(t, err)
		require.Equal(t, basePR.ID, result.ID)
		require.Contains(t, func() []uuid.UUID {
			ids := make([]uuid.UUID, 0, len(result.Reviewers))
			for _, r := range result.Reviewers {
				ids = append(ids, r.ID)
			}
			return ids
		}(), newUserID)
	})
}

func TestPRService_TeamAdd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	teamRepo := mocks.NewMockTeamRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	prRepo := mocks.NewMockPRRepository(ctrl)

	tx := service.TxManagerStub{}
	logger := zap.NewNop()

	svc := service.NewPRService(
		teamRepo,
		userRepo,
		prRepo,
		tx,
		logger,
	)
	ctx := t.Context()

	teamID := uuid.New()
	userID1 := uuid.New()
	userID2 := uuid.New()

	team := &models.Team{
		ID:   teamID,
		Name: "team1",
		Members: []*models.User{
			{ID: userID1},
			{ID: userID2},
		},
	}

	t.Run("success", func(t *testing.T) {
		teamRepo.EXPECT().
			Create(ctx, team).
			Return(nil)
		userRepo.EXPECT().
			Create(ctx, team.Members[0]).
			Return(nil)
		userRepo.EXPECT().
			Create(ctx, team.Members[1]).
			Return(nil)

		err := svc.TeamAdd(ctx, team)
		require.NoError(t, err)
		require.Equal(t, &teamID, team.Members[0].TeamID)
		require.Equal(t, &teamID, team.Members[1].TeamID)
	})

	t.Run("duplicate team", func(t *testing.T) {
		teamRepo.EXPECT().
			Create(ctx, team).
			Return(repository.ErrDuplicate)

		err := svc.TeamAdd(ctx, team)
		require.ErrorIs(t, err, service.ErrTeamAlreadyExists)
	})

	t.Run("user create fails", func(t *testing.T) {
		teamRepo.EXPECT().
			Create(ctx, team).
			Return(nil)
		userRepo.EXPECT().
			Create(ctx, team.Members[0]).
			Return(errors.New("db error"))

		err := svc.TeamAdd(ctx, team)
		require.Error(t, err)
		require.Contains(t, err.Error(), "db error")
	})
}

func TestPRService_TeamGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	teamRepo := mocks.NewMockTeamRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	prRepo := mocks.NewMockPRRepository(ctrl)

	tx := service.TxManagerStub{}
	logger := zap.NewNop()

	svc := service.NewPRService(
		teamRepo,
		userRepo,
		prRepo,
		tx,
		logger,
	)
	ctx := t.Context()

	teamID := uuid.New()
	teamName := "team1"
	userID1 := uuid.New()
	userID2 := uuid.New()

	t.Run("success", func(t *testing.T) {
		team := &models.Team{ID: teamID, Name: teamName}
		members := []*models.User{
			{ID: userID1, TeamID: &teamID},
			{ID: userID2, TeamID: &teamID},
		}

		teamRepo.EXPECT().
			GetByName(ctx, teamName).
			Return(team, nil)
		userRepo.EXPECT().
			GetByTeam(ctx, teamID).
			Return(members, nil)

		result, err := svc.TeamGet(ctx, teamName)
		require.NoError(t, err)
		require.Equal(t, team.ID, result.ID)
		require.Equal(t, team.Name, result.Name)
		require.Len(t, result.Members, 2)
	})

	t.Run("team not found", func(t *testing.T) {
		teamRepo.EXPECT().
			GetByName(ctx, teamName).
			Return(nil, repository.ErrNotFound)

		result, err := svc.TeamGet(ctx, teamName)
		require.ErrorIs(t, err, repository.ErrNotFound)
		require.Nil(t, result)
	})

	t.Run("team repo error", func(t *testing.T) {
		repoErr := errors.New("db error")
		teamRepo.EXPECT().
			GetByName(ctx, teamName).
			Return(nil, repoErr)

		result, err := svc.TeamGet(ctx, teamName)
		require.Error(t, err)
		require.Contains(t, err.Error(), "db error")
		require.Nil(t, result)
	})

	t.Run("user repo error", func(t *testing.T) {
		team := &models.Team{ID: teamID, Name: teamName}
		userRepoErr := errors.New("user db error")

		teamRepo.EXPECT().
			GetByName(ctx, teamName).
			Return(team, nil)
		userRepo.EXPECT().
			GetByTeam(ctx, teamID).
			Return(nil, userRepoErr)

		result, err := svc.TeamGet(ctx, teamName)
		require.Error(t, err)
		require.Contains(t, err.Error(), "user db error")
		require.Nil(t, result)
	})
}

func TestPRService_UsersGetReview(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	prRepo := mocks.NewMockPRRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	teamRepo := mocks.NewMockTeamRepository(ctrl)
	tx := service.TxManagerStub{}
	logger := zap.NewNop()

	svc := service.NewPRService(
		teamRepo,
		userRepo,
		prRepo,
		tx,
		logger,
	)
	ctx := t.Context()
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		pr1 := &models.PullRequest{ID: uuid.New(), Name: "PR1"}
		pr2 := &models.PullRequest{ID: uuid.New(), Name: "PR2"}
		prList := []*models.PullRequest{pr1, pr2}

		prRepo.EXPECT().
			ListByReviewer(ctx, userID).
			Return(prList, nil)

		result, err := svc.UsersGetReview(ctx, userID)
		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Equal(t, prList, result)
	})

	t.Run("repo error", func(t *testing.T) {
		repoErr := errors.New("db error")
		prRepo.EXPECT().
			ListByReviewer(ctx, userID).
			Return(nil, repoErr)

		result, err := svc.UsersGetReview(ctx, userID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "db error")
		require.Nil(t, result)
	})
}

func TestPRService_UsersSetIsActive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	prRepo := mocks.NewMockPRRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	teamRepo := mocks.NewMockTeamRepository(ctrl)
	tx := service.TxManagerStub{}
	logger := zap.NewNop()

	svc := service.NewPRService(
		teamRepo,
		userRepo,
		prRepo,
		tx,
		logger,
	)
	ctx := t.Context()
	userID := uuid.New()
	active := true

	t.Run("success", func(t *testing.T) {
		user := &models.User{ID: userID, Name: "Alice", IsActive: active}

		userRepo.EXPECT().
			UpdateActive(ctx, userID, active).
			Return(nil)
		userRepo.EXPECT().
			GetUserByID(ctx, userID).
			Return(user, nil)

		result, err := svc.UsersSetIsActive(ctx, userID, active)
		require.NoError(t, err)
		require.Equal(t, user, result)
	})

	t.Run("update fails", func(t *testing.T) {
		updateErr := errors.New("update failed")
		userRepo.EXPECT().
			UpdateActive(ctx, userID, active).
			Return(updateErr)

		result, err := svc.UsersSetIsActive(ctx, userID, active)
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "update failed")
	})

	t.Run("get user fails", func(t *testing.T) {
		userRepo.EXPECT().
			UpdateActive(ctx, userID, active).
			Return(nil)
		getErr := errors.New("get failed")
		userRepo.EXPECT().
			GetUserByID(ctx, userID).
			Return(nil, getErr)

		result, err := svc.UsersSetIsActive(ctx, userID, active)
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "get failed")
	})
}
