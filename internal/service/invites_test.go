package service

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"github.com/gil1ges/taskforge-api/internal/repo/mysql"
)

func TestInvitesServiceInvite(t *testing.T) {
	t.Parallel()

	teams := &fakeTeamsRepo{
		roleMap: map[[2]uint64]domain.Role{
			{1, 10}: domain.RoleOwner,
			{1, 20}: domain.RoleMember,
		},
	}
	users := &fakeUsersRepo{
		byEmail: map[string]domain.User{
			"user@example.com": {ID: 7, Email: "user@example.com"},
		},
	}
	inv := &fakeInvitesRepo{}
	svc := NewInvitesService(teams, users, inv, time.Hour)

	_, err := svc.Invite(context.Background(), 1, 10, "", domain.RoleMember)
	if err != domain.ErrBadRequest {
		t.Fatalf("Invite() empty email error = %v, want %v", err, domain.ErrBadRequest)
	}

	_, err = svc.Invite(context.Background(), 1, 10, "user@example.com", domain.Role("invalid"))
	if err != domain.ErrBadRequest {
		t.Fatalf("Invite() invalid role error = %v, want %v", err, domain.ErrBadRequest)
	}

	_, err = svc.Invite(context.Background(), 1, 20, "user@example.com", domain.RoleMember)
	if err != domain.ErrForbidden {
		t.Fatalf("Invite() member cannot invite error = %v, want %v", err, domain.ErrForbidden)
	}

	_, err = svc.Invite(context.Background(), 1, 10, "missing@example.com", domain.RoleMember)
	if err != domain.ErrNotFound {
		t.Fatalf("Invite() missing user error = %v, want %v", err, domain.ErrNotFound)
	}

	code, err := svc.Invite(context.Background(), 1, 10, "user@example.com", "")
	if err != nil {
		t.Fatalf("Invite() success error = %v", err)
	}
	if len(code) != 12 {
		t.Fatalf("Invite() code len = %d, want %d", len(code), 12)
	}
	if inv.lastCreate.Role != domain.RoleMember {
		t.Fatalf("Invite() default role = %q, want %q", inv.lastCreate.Role, domain.RoleMember)
	}
	if len(inv.lastCreate.CodeHash) != 32 {
		t.Fatalf("Invite() code hash len = %d, want %d", len(inv.lastCreate.CodeHash), 32)
	}
}

func TestInvitesServiceAccept(t *testing.T) {
	t.Parallel()

	code := "a1b2c3d4e5f6"
	hash := sha256.Sum256([]byte(code))

	teams := &fakeTeamsRepo{
		memberMap: map[[2]uint64]bool{
			{1, 10}: false,
		},
	}
	users := &fakeUsersRepo{
		byID: map[uint64]domain.User{
			10: {ID: 10, Email: "user@example.com"},
		},
	}
	inv := &fakeInvitesRepo{
		findFound:  true,
		findEmail:  "user@example.com",
		findTeamID: 1,
		findHash:   hash[:],
		findInvite: mysql.Invite{
			ID:   77,
			Role: domain.RoleMember,
		},
	}
	svc := NewInvitesService(teams, users, inv, time.Hour)

	err := svc.Accept(context.Background(), 1, 10, "")
	if err != domain.ErrBadRequest {
		t.Fatalf("Accept() empty code error = %v, want %v", err, domain.ErrBadRequest)
	}

	err = svc.Accept(context.Background(), 1, 999, code)
	if err != domain.ErrUnauthorized {
		t.Fatalf("Accept() unknown user error = %v, want %v", err, domain.ErrUnauthorized)
	}

	err = svc.Accept(context.Background(), 1, 10, "wrongcode1111")
	if err != domain.ErrForbidden {
		t.Fatalf("Accept() wrong code error = %v, want %v", err, domain.ErrForbidden)
	}

	teams.memberMap[[2]uint64{1, 10}] = true
	err = svc.Accept(context.Background(), 1, 10, code)
	if err != nil {
		t.Fatalf("Accept() existing member error = %v", err)
	}
	if len(inv.deleted) == 0 || inv.deleted[len(inv.deleted)-1] != 77 {
		t.Fatalf("Accept() should delete invite when already member, deleted=%+v", inv.deleted)
	}

	teams.memberMap[[2]uint64{1, 10}] = false
	inv.deleted = nil
	err = svc.Accept(context.Background(), 1, 10, code)
	if err != nil {
		t.Fatalf("Accept() success error = %v", err)
	}
	if len(teams.addedMembers) != 1 {
		t.Fatalf("Accept() expected AddMember call, got %+v", teams.addedMembers)
	}
	if teams.addedMembers[0].Role != domain.RoleMember {
		t.Fatalf("Accept() added role = %q, want %q", teams.addedMembers[0].Role, domain.RoleMember)
	}
	if len(inv.deleted) != 1 || inv.deleted[0] != 77 {
		t.Fatalf("Accept() expected invite delete, deleted=%+v", inv.deleted)
	}
}

func TestGenerateInviteCode(t *testing.T) {
	t.Parallel()

	code, hash, err := generateInviteCode()
	if err != nil {
		t.Fatalf("generateInviteCode() error = %v", err)
	}
	if len(code) != 12 {
		t.Fatalf("generateInviteCode() code len = %d, want %d", len(code), 12)
	}
	if len(hash) != 32 {
		t.Fatalf("generateInviteCode() hash len = %d, want %d", len(hash), 32)
	}
}

func TestNewInvitesServiceWithNilNotifier(t *testing.T) {
	t.Parallel()

	teams := &fakeTeamsRepo{
		roleMap: map[[2]uint64]domain.Role{
			{1, 10}: domain.RoleOwner,
		},
	}
	users := &fakeUsersRepo{
		byEmail: map[string]domain.User{
			"user@example.com": {ID: 1, Email: "user@example.com"},
		},
	}
	inv := &fakeInvitesRepo{}

	svc := NewInvitesServiceWithNotifier(teams, users, inv, nil, time.Hour)
	code, err := svc.Invite(context.Background(), 1, 10, "user@example.com", domain.RoleMember)
	if err != nil {
		t.Fatalf("Invite() with nil notifier error = %v", err)
	}
	if len(code) != 12 {
		t.Fatalf("Invite() code len = %d, want 12", len(code))
	}
}
