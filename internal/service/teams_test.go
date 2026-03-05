package service

import (
	"context"
	"testing"

	"github.com/gil1ges/taskforge-api/internal/domain"
)

func TestTeamsServiceCreateAndList(t *testing.T) {
	t.Parallel()

	repo := &fakeTeamsRepo{
		createTeamID: 99,
		listTeamsOut: []domain.Team{{ID: 99, Name: "core"}},
	}
	svc := NewTeamsService(repo)

	_, err := svc.CreateTeam(context.Background(), "", 1)
	if err != domain.ErrBadRequest {
		t.Fatalf("CreateTeam() empty name error = %v, want %v", err, domain.ErrBadRequest)
	}

	id, err := svc.CreateTeam(context.Background(), "core", 1)
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	if id != 99 {
		t.Fatalf("CreateTeam() id = %d, want %d", id, 99)
	}

	teams, err := svc.ListTeams(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListTeams() error = %v", err)
	}
	if len(teams) != 1 || teams[0].ID != 99 {
		t.Fatalf("ListTeams() = %+v, want one team with id 99", teams)
	}
}

func TestTeamsServiceCanInvite(t *testing.T) {
	t.Parallel()

	repo := &fakeTeamsRepo{
		roleMap: map[[2]uint64]domain.Role{
			{1, 10}: domain.RoleOwner,
			{1, 11}: domain.RoleAdmin,
			{1, 12}: domain.RoleMember,
		},
	}
	svc := NewTeamsService(repo)

	if err := svc.CanInvite(context.Background(), 1, 10); err != nil {
		t.Fatalf("owner should be able to invite, err=%v", err)
	}
	if err := svc.CanInvite(context.Background(), 1, 11); err != nil {
		t.Fatalf("admin should be able to invite, err=%v", err)
	}
	if err := svc.CanInvite(context.Background(), 1, 12); err != domain.ErrForbidden {
		t.Fatalf("member invite error = %v, want %v", err, domain.ErrForbidden)
	}
	if err := svc.CanInvite(context.Background(), 1, 99); err != domain.ErrForbidden {
		t.Fatalf("non-member invite error = %v, want %v", err, domain.ErrForbidden)
	}
}
