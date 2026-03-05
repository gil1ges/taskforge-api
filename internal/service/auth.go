package service

import (
	"context"
	"net/mail"
	"strings"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users authUsersRepo
}

type authUsersRepo interface {
	FindByEmail(ctx context.Context, email string) (domain.User, bool, error)
	Create(ctx context.Context, email string, passwordHash []byte) (uint64, error)
}

func NewAuthService(users authUsersRepo) *AuthService { return &AuthService{users: users} }

func (s *AuthService) Register(ctx context.Context, email, password string) (uint64, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if err := validateCredentials(email, password); err != nil {
		return 0, domain.ErrBadRequest
	}

	_, exists, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, domain.ErrConflict
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	return s.users.Create(ctx, email, hash)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (domain.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if err := validateCredentials(email, password); err != nil {
		return domain.User{}, domain.ErrBadRequest
	}

	u, ok, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return domain.User{}, err
	}
	if !ok {
		return domain.User{}, domain.ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(password)); err != nil {
		return domain.User{}, domain.ErrUnauthorized
	}
	return u, nil
}

func validateCredentials(email, password string) error {
	if !isValidEmail(email) {
		return domain.ErrBadRequest
	}
	if len(password) < 8 {
		return domain.ErrBadRequest
	}
	return nil
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}
