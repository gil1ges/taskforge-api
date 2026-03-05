package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"github.com/gil1ges/taskforge-api/internal/repo/mysql"
)

type InvitesService struct {
	teams invitesTeamsRepo
	users invitesUsersRepo
	inv   invitesRepo
	notif InviteNotifier
	ttl   time.Duration
}

type invitesTeamsRepo interface {
	GetUserRole(ctx context.Context, teamID, userID uint64) (domain.Role, bool, error)
	IsMember(ctx context.Context, teamID, userID uint64) (bool, error)
	AddMember(ctx context.Context, teamID, userID uint64, role domain.Role) error
}

type invitesUsersRepo interface {
	FindByEmail(ctx context.Context, email string) (domain.User, bool, error)
	GetByID(ctx context.Context, id uint64) (domain.User, bool, error)
}

type invitesRepo interface {
	Create(ctx context.Context, teamID uint64, email string, role domain.Role, invitedBy uint64, codeHash []byte, expiresAt time.Time) (uint64, error)
	FindValidByTeamEmailCodeHash(ctx context.Context, teamID uint64, email string, codeHash []byte) (mysql.Invite, bool, error)
	Delete(ctx context.Context, inviteID uint64) error
}

func NewInvitesService(teams invitesTeamsRepo, users invitesUsersRepo, inv invitesRepo, ttl time.Duration) *InvitesService {
	return NewInvitesServiceWithNotifier(teams, users, inv, NewCircuitBreakerNotifier(NoopInviteNotifier{}, 3, 30*time.Second), ttl)
}

func NewInvitesServiceWithNotifier(teams invitesTeamsRepo, users invitesUsersRepo, inv invitesRepo, notifier InviteNotifier, ttl time.Duration) *InvitesService {
	if notifier == nil {
		notifier = NewCircuitBreakerNotifier(NoopInviteNotifier{}, 3, 30*time.Second)
	}
	return &InvitesService{teams: teams, users: users, inv: inv, notif: notifier, ttl: ttl}
}

func (s *InvitesService) Invite(ctx context.Context, teamID, inviterID uint64, email string, role domain.Role) (string, error) {
	if email == "" {
		return "", domain.ErrBadRequest
	}
	if role == "" {
		role = domain.RoleMember
	}
	if role != domain.RoleAdmin && role != domain.RoleMember {
		return "", domain.ErrBadRequest
	}

	r, ok, err := s.teams.GetUserRole(ctx, teamID, inviterID)
	if err != nil {
		return "", err
	}
	if !ok || (r != domain.RoleOwner && r != domain.RoleAdmin) {
		return "", domain.ErrForbidden
	}

	_, found, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if !found {
		return "", domain.ErrNotFound
	}

	code, codeHash, err := generateInviteCode()
	if err != nil {
		return "", err
	}

	expires := time.Now().Add(s.ttl)
	_, err = s.inv.Create(ctx, teamID, email, role, inviterID, codeHash, expires)
	if err != nil {
		return "", err
	}

	_ = s.notif.NotifyInvite(ctx, email, teamID, code)
	return code, nil
}

func (s *InvitesService) Accept(ctx context.Context, teamID, userID uint64, code string) error {
	if code == "" {
		return domain.ErrBadRequest
	}
	u, ok, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrUnauthorized
	}

	codeHash := sha256.Sum256([]byte(code))
	inv, found, err := s.inv.FindValidByTeamEmailCodeHash(ctx, teamID, u.Email, codeHash[:])
	if err != nil {
		return err
	}
	if !found {
		return domain.ErrForbidden
	}

	isMember, err := s.teams.IsMember(ctx, teamID, userID)
	if err != nil {
		return err
	}
	if isMember {
		_ = s.inv.Delete(ctx, inv.ID)
		return nil
	}

	if err := s.teams.AddMember(ctx, teamID, userID, inv.Role); err != nil {
		return err
	}

	return s.inv.Delete(ctx, inv.ID)
}

func generateInviteCode() (string, []byte, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	code := hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(code))
	return code, hash[:], nil
}
