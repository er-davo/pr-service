//go:generate mockgen -source=pr_service.go -destination=../mocks/pr_service.go -package=mocks .

package service

import (
	"context"
	"errors"

	"pr-service/internal/models"
	"pr-service/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TeamRepository interface {
	// Создать новую команду
	Create(ctx context.Context, team *models.Team) error

	// Получить команду по ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.Team, error)

	// Получить команду по имени
	GetByName(ctx context.Context, name string) (*models.Team, error)
}

type UserRepository interface {
	// Получить пользователя по ID
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)

	// Получить всех активных пользователей команды
	GetActiveByTeam(ctx context.Context, teamID uuid.UUID) ([]*models.User, error)

	// Получить всех пользователей команды
	GetByTeam(ctx context.Context, teamID uuid.UUID) ([]*models.User, error)

	// Создать нового пользователя
	Create(ctx context.Context, user *models.User) error

	// Обновить активность пользователя
	UpdateActive(ctx context.Context, id uuid.UUID, active bool) error
}

type PRRepository interface {
	// Создать пулл-реквест
	Create(ctx context.Context, pr *models.PullRequest) error

	// Получить пулл-реквест по ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.PullRequest, error)

	// Назначить ревьюеров
	AssignReviewers(ctx context.Context, prID uuid.UUID, reviewers []uuid.UUID) error

	// Заменить одного ревьюера другим
	ReplaceReviewer(ctx context.Context, prID, oldID, newID uuid.UUID) error

	// Замерджить пулл-реквест
	Merge(ctx context.Context, id uuid.UUID) error

	// Получить список PR, где пользователь является ревьюером
	ListByReviewer(ctx context.Context, id uuid.UUID) ([]*models.PullRequest, error)
}

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type PRService struct {
	teamRepo TeamRepository
	userRepo UserRepository
	prRepo   PRRepository

	trManager TxManager

	log *zap.Logger
}

type TxManagerStub struct{}

func (TxManagerStub) Do(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func NewPRService(
	teamRepo TeamRepository,
	userRepo UserRepository,
	prRepo PRRepository,
	trManager TxManager,
	log *zap.Logger,
) *PRService {
	return &PRService{
		teamRepo:  teamRepo,
		userRepo:  userRepo,
		prRepo:    prRepo,
		trManager: trManager,
		log:       log,
	}
}

func (s *PRService) CreatePR(ctx context.Context, pr *models.PullRequest) error {
	return s.trManager.Do(ctx, func(ctx context.Context) error {
		err := s.prRepo.Create(ctx, pr)
		if err != nil {
			s.log.Error("failed to create PR",
				zap.Error(err),
				zap.String("pr_id", pr.ID.String()),
			)
			return err
		}

		author, err := s.userRepo.GetUserByID(ctx, pr.AuthorID)
		if err != nil {
			s.log.Error("failed to get author",
				zap.Error(err),
				zap.String("pr_id", pr.ID.String()),
			)
			return err
		}

		activeUsers, err := s.userRepo.GetActiveByTeam(ctx, *author.TeamID)
		if err != nil {
			s.log.Error("failed to get active users",
				zap.Error(err),
				zap.String("pr_id", pr.ID.String()),
			)
			return err
		}

		// slice to 2
		if len(activeUsers) > 2 {
			activeUsers = activeUsers[:2]
		}

		uuids := make([]uuid.UUID, len(activeUsers))

		for i, reviewer := range activeUsers {
			uuids[i] = reviewer.ID
		}

		err = s.prRepo.AssignReviewers(ctx, pr.ID, uuids)
		if err != nil {
			s.log.Error("failed to assign reviewers",
				zap.Error(err),
				zap.String("pr_id", pr.ID.String()),
			)
			return err
		}

		s.log.Info("PR created, reviewers assigned",
			zap.String("pr_id", pr.ID.String()),
		)

		return nil
	})
}

func (s *PRService) PRMerge(ctx context.Context, id uuid.UUID) (*models.PullRequest, error) {
	pr := &models.PullRequest{}
	txErr := s.trManager.Do(ctx, func(ctx context.Context) error {
		var err error
		pr, err = s.prRepo.GetByID(ctx, id)
		if err != nil {
			s.log.Error("failed to get PR",
				zap.Error(err),
				zap.String("pr_id", id.String()),
			)
			return err
		}

		if pr.Status == string(models.PRStatusMerged) {
			s.log.Info("PR already merged",
				zap.String("pr_id", id.String()),
			)
			return nil
		}

		err = s.prRepo.Merge(ctx, id)
		if err != nil {
			s.log.Error("failed to merge PR",
				zap.Error(err),
				zap.String("pr_id", id.String()),
			)
			return err
		}

		s.log.Info("PR merged",
			zap.String("pr_id", id.String()),
		)

		return nil
	})

	if txErr != nil {
		return nil, txErr
	}
	return pr, nil
}

func (s *PRService) PRReassign(ctx context.Context, prID uuid.UUID, oldUserID uuid.UUID) (*models.PullRequest, error) {
	pr := &models.PullRequest{}
	trErr := s.trManager.Do(ctx, func(ctx context.Context) error {
		var err error
		pr, err = s.prRepo.GetByID(ctx, prID)
		if err != nil {
			s.log.Error("failed to get PR",
				zap.Error(err),
				zap.String("pr_id", prID.String()),
			)
			return err
		}

		if pr.Status == string(models.PRStatusMerged) {
			s.log.Info("can not reassing, pr is merged",
				zap.String("pr_id", prID.String()),
			)
			return ErrCanNotReassing
		}

		// Check if old reviewer is assigned to PR
		found := false
		for _, r := range pr.Reviewers {
			if r.ID == oldUserID {
				found = true
				break
			}
		}
		if !found {
			s.log.Warn("old reviewer not assigned to PR",
				zap.String("pr_id", prID.String()),
				zap.String("user_id", oldUserID.String()),
			)
			return ErrNotAssinged
		}

		author, err := s.userRepo.GetUserByID(ctx, pr.AuthorID)
		if err != nil {
			s.log.Error("failed to get author",
				zap.Error(err),
				zap.String("pr_id", prID.String()),
			)
			return err
		}

		users, err := s.userRepo.GetActiveByTeam(ctx, *author.TeamID)
		if err != nil {
			s.log.Error("failed to get active users",
				zap.Error(err),
				zap.String("pr_id", prID.String()),
			)
			return err
		}

		// Search for another active user
		var newUserID uuid.UUID
		for _, u := range users {
			if u.ID != oldUserID {
				alreadyReviewerinPR := false

				for _, r := range pr.Reviewers {
					if r.ID == u.ID {
						alreadyReviewerinPR = true
						break
					}
				}

				if alreadyReviewerinPR {
					continue
				}

				newUserID = u.ID
				break
			}
		}

		if newUserID == uuid.Nil {
			s.log.Warn("no replacement reviewer found",
				zap.String("pr_id", prID.String()),
			)
			return ErrNoAvailableReviewer
		}

		err = s.prRepo.ReplaceReviewer(ctx, prID, oldUserID, newUserID)
		if err != nil {
			s.log.Error("failed to replace reviewer",
				zap.Error(err),
				zap.String("pr_id", prID.String()),
				zap.String("old_user_id", oldUserID.String()),
				zap.String("new_user_id", newUserID.String()),
			)
			return err
		}

		newReviewers := make([]*models.PRReviewer, 0, len(pr.Reviewers))
		for _, r := range pr.Reviewers {
			if r.ID != oldUserID {
				newReviewers = append(newReviewers, r)
			}
		}

		newReviewers = append(newReviewers, &models.PRReviewer{
			ID:   newUserID,
			PRID: pr.ID,
		})
		pr.Reviewers = newReviewers

		s.log.Info("reviewer replaced successfully",
			zap.String("pr_id", prID.String()),
			zap.String("old_user_id", oldUserID.String()),
			zap.String("new_user_id", newUserID.String()),
		)

		return nil
	})

	if trErr != nil {
		return nil, trErr
	}

	return pr, nil
}

func (s *PRService) TeamAdd(ctx context.Context, team *models.Team) error {
	return s.trManager.Do(ctx, func(ctx context.Context) error {
		err := s.teamRepo.Create(ctx, team)
		if err != nil {
			if errors.Is(err, repository.ErrDuplicate) {
				s.log.Warn("team already exists",
					zap.String("team_id", team.ID.String()),
				)
				return ErrTeamAlreadyExists
			}
			s.log.Error("failed to create team",
				zap.Error(err),
				zap.String("team_id", team.ID.String()),
			)
			return err
		}

		for i := range team.Members {
			team.Members[i].TeamID = &team.ID
			if err := s.userRepo.Create(ctx, team.Members[i]); err != nil {
				s.log.Error("failed to create user",
					zap.Error(err),
					zap.String("user_id", team.Members[i].ID.String()),
				)
				return err
			}
		}

		s.log.Info("team created, members added",
			zap.String("team_id", team.ID.String()),
			zap.String("team_name", team.Name),
			zap.Int("members_count", len(team.Members)),
		)

		return nil
	})
}

func (s *PRService) TeamGet(ctx context.Context, teamName string) (*models.Team, error) {
	team := &models.Team{}
	var err error
	team, err = s.teamRepo.GetByName(ctx, teamName)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.log.Warn("team not found",
				zap.String("team_name", teamName),
			)
			return nil, ErrNotFound
		}
		s.log.Error("failed to get team",
			zap.Error(err),
			zap.String("team_name", teamName),
		)
		return nil, err
	}

	members, err := s.userRepo.GetByTeam(ctx, team.ID)
	if err != nil {
		s.log.Error("failed to get team members",
			zap.Error(err),
			zap.String("team_name", teamName),
			zap.String("team_id", team.ID.String()),
		)
		return nil, err
	}

	team.Members = members

	s.log.Info("team found",
		zap.String("team_name", teamName),
		zap.String("team_id", team.ID.String()),
	)

	return team, nil
}

func (s *PRService) TeamGetByID(ctx context.Context, teamID uuid.UUID) (*models.Team, error) {
	return s.teamRepo.GetByID(ctx, teamID)
}

func (s *PRService) UsersGetReview(ctx context.Context, userID uuid.UUID) ([]*models.PullRequest, error) {
	prs, err := s.prRepo.ListByReviewer(ctx, userID)
	if err != nil {
		s.log.Error("failed to get PRs for review",
			zap.Error(err),
			zap.String("user_id", userID.String()),
		)
		return nil, err
	}

	s.log.Info("PRs found for review",
		zap.String("user_id", userID.String()),
		zap.Int("pr_count", len(prs)),
	)

	return prs, nil
}

func (s *PRService) UsersSetIsActive(ctx context.Context, userID uuid.UUID, active bool) (*models.User, error) {
	user := &models.User{}

	err := s.userRepo.UpdateActive(ctx, userID, active)
	if err != nil {
		s.log.Error("failed to update user active status",
			zap.Error(err),
			zap.String("user_id", userID.String()),
			zap.Bool("active", active),
		)
		return nil, err
	}

	user, err = s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		s.log.Error("failed to get user",
			zap.Error(err),
			zap.String("user_id", userID.String()),
		)
		return nil, err
	}

	return user, nil
}
