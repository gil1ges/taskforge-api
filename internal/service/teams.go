package service

import (
	"context"

	"github.com/gil1ges/taskforge-api/internal/domain"
)

type TeamsService struct {
	teams teamsRepo
}

type teamsRepo interface {
	CreateTeamWithOwner(ctx context.Context, name string, creatorID uint64) (uint64, error)
	ListTeamsForUser(ctx context.Context, userID uint64) ([]domain.Team, error)
	GetUserRole(ctx context.Context, teamID, userID uint64) (domain.Role, bool, error)
	IsMember(ctx context.Context, teamID, userID uint64) (bool, error)
	AddMember(ctx context.Context, teamID, userID uint64, role domain.Role) error
}

func NewTeamsService(teams teamsRepo) *TeamsService {
	return &TeamsService{teams: teams}
}

func (s *TeamsService) CreateTeam(ctx context.Context, name string, creatorID uint64) (uint64, error) {
	if name == "" {
		return 0, domain.ErrBadRequest
	}
	return s.teams.CreateTeamWithOwner(ctx, name, creatorID)
}

func (s *TeamsService) ListTeams(ctx context.Context, userID uint64) ([]domain.Team, error) {
	return s.teams.ListTeamsForUser(ctx, userID)
}

func (s *TeamsService) CanInvite(ctx context.Context, teamID, userID uint64) error {
	role, ok, err := s.teams.GetUserRole(ctx, teamID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	if role != domain.RoleOwner && role != domain.RoleAdmin {
		return domain.ErrForbidden
	}
	return nil
}
