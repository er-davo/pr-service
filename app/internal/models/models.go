package models

import (
	"pr-service/internal/api"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID       uuid.UUID
	TeamID   *uuid.UUID
	Name     string
	IsActive bool
}

type Team struct {
	ID      uuid.UUID
	Name    string
	Members []*User
}

type PullRequest struct {
	ID        uuid.UUID
	Name      string
	AuthorID  uuid.UUID
	Status    string
	CreatedAt time.Time
	MergedAt  *time.Time
	Reviewers []*PRReviewer
}

type PRReviewer struct {
	ID         uuid.UUID
	PRID       uuid.UUID
	AssignedAt time.Time
}

type PRStatus api.PullRequestStatus

const (
	PRStatusOpen   PRStatus = PRStatus(api.PullRequestShortStatusOPEN)
	PRStatusMerged PRStatus = PRStatus(api.PullRequestShortStatusMERGED)
)
