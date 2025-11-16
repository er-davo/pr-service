package service

import (
	"errors"

	"pr-service/internal/repository"
)

var (
	ErrNoAvailableReviewer = errors.New("no available reviewer found")
	ErrCanNotReassing      = errors.New("can not reassign reviewer, pr is merged")
	ErrTeamAlreadyExists   = errors.New("team already exists")
	ErrNotAssinged         = errors.New("not assigned")
	ErrNotFound            = repository.ErrNotFound
)
