package handler

import (
	"errors"
	"net/http"
	"pr-service/internal/api"
	"pr-service/internal/models"
	"pr-service/internal/repository"
	"pr-service/internal/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type PRHandler struct {
	prService *service.PRService
	log       *zap.Logger
}

var _ api.ServerInterface = (*PRHandler)(nil)

func NewPRHandler(prService *service.PRService, log *zap.Logger) *PRHandler {
	return &PRHandler{
		prService: prService,
		log:       log,
	}
}

func (h *PRHandler) PostPullRequestCreate(c echo.Context) error {
	prcBody := &api.PostPullRequestCreateJSONBody{}
	if err := c.Bind(prcBody); err != nil {
		return c.JSON(http.StatusBadRequest, "bad request")
	}

	authorID, err := uuid.Parse(prcBody.AuthorId)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "invalid id")
	}

	prID, err := uuid.Parse(prcBody.PullRequestId)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "invalid id")
	}

	pr := &models.PullRequest{
		ID:       prID,
		AuthorID: authorID,
		Name:     prcBody.PullRequestName,
		Status:   string(models.PRStatusOpen),
	}

	if err := h.prService.CreatePR(c.Request().Context(), pr); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			errResponse := api.ErrorResponse{}
			errResponse.Error.Code = "not_found"
			errResponse.Error.Message = "Автор/команда не найдены"
			return c.JSON(http.StatusNotFound, errResponse)
		}

		if errors.Is(err, repository.ErrDuplicate) ||
			errors.Is(err, repository.ErrForeignKeyViolation) {
			errResponse := api.ErrorResponse{}
			errResponse.Error.Code = api.PREXISTS
			errResponse.Error.Message = "PR уже существует"
			return c.JSON(http.StatusConflict, errResponse)
		}

		return c.JSON(http.StatusInternalServerError, "")
	}

	prResponse := api.PullRequest{
		PullRequestId:     pr.ID.String(),
		AuthorId:          pr.AuthorID.String(),
		PullRequestName:   pr.Name,
		Status:            api.PullRequestStatus(pr.Status),
		CreatedAt:         &pr.CreatedAt,
		AssignedReviewers: []string{},
	}

	for _, reviewer := range pr.Reviewers {
		prResponse.AssignedReviewers = append(prResponse.AssignedReviewers, reviewer.ID.String())
	}

	return c.JSON(http.StatusCreated, prResponse)
}

func (h *PRHandler) PostPullRequestMerge(c echo.Context) error {
	body := api.PostPullRequestMergeJSONBody{}

	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, "bad request")
	}

	prID, err := uuid.Parse(body.PullRequestId)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "invalid id")
	}

	pr, err := h.prService.PRMerge(c.Request().Context(), prID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			errResponse := api.ErrorResponse{}
			errResponse.Error.Code = "not_found"
			errResponse.Error.Message = "PR не найден"
			return c.JSON(http.StatusNotFound, errResponse)
		}
		return c.JSON(http.StatusInternalServerError, "")
	}

	prResponse := api.PullRequest{
		PullRequestId:     pr.ID.String(),
		PullRequestName:   pr.Name,
		AuthorId:          pr.AuthorID.String(),
		Status:            api.PullRequestStatus(pr.Status),
		CreatedAt:         &pr.CreatedAt,
		MergedAt:          pr.MergedAt,
		AssignedReviewers: []string{},
	}

	for _, reviewer := range pr.Reviewers {
		prResponse.AssignedReviewers = append(prResponse.AssignedReviewers, reviewer.ID.String())
	}

	return c.JSON(http.StatusOK, prResponse)
}

func (h *PRHandler) PostPullRequestReassign(c echo.Context) error {
	body := api.PostPullRequestReassignJSONBody{}

	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, "bad request")
	}

	prID, err := uuid.Parse(body.PullRequestId)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "invalid pull_request_id")
	}

	oldUserID, err := uuid.Parse(body.OldUserId)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "invalid old_user_id")
	}

	pr, err := h.prService.PRReassign(c.Request().Context(), prID, oldUserID)
	if err != nil {
		errResponse := api.ErrorResponse{}
		switch {
		case errors.Is(err, service.ErrCanNotReassing):
			errResponse.Error.Code = api.PRMERGED
			errResponse.Error.Message = "cannot reassign on merged PR"
			return c.JSON(http.StatusConflict, errResponse)
		case errors.Is(err, service.ErrNotAssinged):
			errResponse.Error.Code = api.NOTASSIGNED
			errResponse.Error.Message = "old reviewer not assigned to PR"
			return c.JSON(http.StatusNotFound, errResponse)
		case errors.Is(err, service.ErrNoAvailableReviewer):
			errResponse.Error.Code = api.NOCANDIDATE
			errResponse.Error.Message = "no available reviewer found"
			return c.JSON(http.StatusConflict, errResponse)
		case errors.Is(err, repository.ErrNotFound):
			errResponse.Error.Code = "not_found"
			errResponse.Error.Message = "PR не найден"
			return c.JSON(http.StatusNotFound, errResponse)
		default:
			return c.JSON(http.StatusInternalServerError, "")
		}
	}

	// new one allways last
	replacedBy := pr.Reviewers[len(pr.Reviewers)-1].ID.String()

	prResponse := api.PullRequest{
		PullRequestId:     pr.ID.String(),
		AuthorId:          pr.AuthorID.String(),
		PullRequestName:   pr.Name,
		Status:            api.PullRequestStatus(pr.Status),
		CreatedAt:         &pr.CreatedAt,
		AssignedReviewers: []string{},
	}

	for _, reviewer := range pr.Reviewers {
		prResponse.AssignedReviewers = append(prResponse.AssignedReviewers, reviewer.ID.String())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"pr":          prResponse,
		"replaced_by": replacedBy,
	})
}

func (h *PRHandler) PostTeamAdd(c echo.Context) error {
	body := &api.Team{}
	if err := c.Bind(body); err != nil {
		return c.JSON(http.StatusBadRequest, "bad request")
	}

	team := &models.Team{
		Name:    body.TeamName,
		Members: make([]*models.User, len(body.Members)),
	}

	for i, m := range body.Members {
		id, err := uuid.Parse(m.UserId)
		if err != nil {
			return c.JSON(http.StatusBadRequest, "invalid user_id")
		}
		team.Members[i] = &models.User{
			ID:       id,
			Name:     m.Username,
			IsActive: m.IsActive,
		}
	}

	if err := h.prService.TeamAdd(c.Request().Context(), team); err != nil {
		errResp := api.ErrorResponse{}
		if errors.Is(err, service.ErrTeamAlreadyExists) {
			errResp.Error.Code = api.TEAMEXISTS
			errResp.Error.Message = "team_name already exists"
			return c.JSON(http.StatusBadRequest, errResp)
		}
		return c.JSON(http.StatusInternalServerError, "")
	}

	resp := api.Team{
		TeamName: team.Name,
		Members:  make([]api.TeamMember, len(team.Members)),
	}

	for i, u := range team.Members {
		resp.Members[i] = api.TeamMember{
			UserId:   u.ID.String(),
			Username: u.Name,
			IsActive: u.IsActive,
		}
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"team": resp,
	})
}

func (h *PRHandler) GetTeamGet(c echo.Context, params api.GetTeamGetParams) error {
	return nil
}

func (h *PRHandler) GetUsersGetReview(c echo.Context, params api.GetUsersGetReviewParams) error {
	return nil
}

func (h *PRHandler) PostUsersSetIsActive(c echo.Context) error {
	return nil
}

func (h *PRHandler) PostUsersSetReview(c echo.Context) error {
	return nil
}
