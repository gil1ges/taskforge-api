package service

import (
	"context"
	"testing"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

func TestValidateCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  error
	}{
		{name: "valid", email: "user@example.com", password: "password123", wantErr: nil},
		{name: "invalid email", email: "user-at-example.com", password: "password123", wantErr: domain.ErrBadRequest},
		{name: "short password", email: "user@example.com", password: "short", wantErr: domain.ErrBadRequest},
		{name: "empty fields", email: "", password: "", wantErr: domain.ErrBadRequest},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateCredentials(tc.email, tc.password)
			if err != tc.wantErr {
				t.Fatalf("validateCredentials() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestAuthServiceLoginRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	svc := &AuthService{}
	_, err := svc.Login(context.Background(), "broken-email", "123")
	if err != domain.ErrBadRequest {
		t.Fatalf("Login() error = %v, want %v", err, domain.ErrBadRequest)
	}
}

func TestAuthServiceRegister(t *testing.T) {
	t.Parallel()

	t.Run("bad payload", func(t *testing.T) {
		t.Parallel()
		repo := &fakeUsersRepo{byEmail: map[string]domain.User{}}
		svc := NewAuthService(repo)

		_, err := svc.Register(context.Background(), "bad-email", "123")
		if err != domain.ErrBadRequest {
			t.Fatalf("Register() error = %v, want %v", err, domain.ErrBadRequest)
		}
	})

	t.Run("conflict", func(t *testing.T) {
		t.Parallel()
		repo := &fakeUsersRepo{
			byEmail: map[string]domain.User{
				"user@example.com": {ID: 1, Email: "user@example.com"},
			},
		}
		svc := NewAuthService(repo)

		_, err := svc.Register(context.Background(), "user@example.com", "password123")
		if err != domain.ErrConflict {
			t.Fatalf("Register() error = %v, want %v", err, domain.ErrConflict)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		repo := &fakeUsersRepo{byEmail: map[string]domain.User{}, nextID: 7}
		svc := NewAuthService(repo)

		id, err := svc.Register(context.Background(), "User@Example.com", "password123")
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if id != 7 {
			t.Fatalf("Register() id = %d, want %d", id, 7)
		}
		if repo.createdEmail != "user@example.com" {
			t.Fatalf("created email = %q, want normalized", repo.createdEmail)
		}
		if err := bcrypt.CompareHashAndPassword(repo.createdHash, []byte("password123")); err != nil {
			t.Fatalf("stored hash does not match password: %v", err)
		}
	})
}

func TestAuthServiceLogin(t *testing.T) {
	t.Parallel()

	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	repo := &fakeUsersRepo{
		byEmail: map[string]domain.User{
			"user@example.com": {
				ID:           11,
				Email:        "user@example.com",
				PasswordHash: hash,
			},
		},
	}
	svc := NewAuthService(repo)

	_, err = svc.Login(context.Background(), "missing@example.com", "password123")
	if err != domain.ErrUnauthorized {
		t.Fatalf("Login() missing user error = %v, want %v", err, domain.ErrUnauthorized)
	}

	_, err = svc.Login(context.Background(), "user@example.com", "wrong-password")
	if err != domain.ErrUnauthorized {
		t.Fatalf("Login() wrong password error = %v, want %v", err, domain.ErrUnauthorized)
	}

	u, err := svc.Login(context.Background(), " User@Example.com ", "password123")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if u.ID != 11 {
		t.Fatalf("Login() user id = %d, want %d", u.ID, 11)
	}
}
