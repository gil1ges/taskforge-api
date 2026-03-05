package mysql

import (
	"context"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"github.com/jmoiron/sqlx"
)

type UsersRepo struct{ db *sqlx.DB }

func NewUsersRepo(db *sqlx.DB) *UsersRepo { return &UsersRepo{db: db} }

func (r *UsersRepo) Create(ctx context.Context, email string, passwordHash []byte) (uint64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO users(email, password_hash) VALUES(?, ?)`, email, passwordHash)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}

func (r *UsersRepo) FindByEmail(ctx context.Context, email string) (domain.User, bool, error) {
	var u domain.User
	err := r.db.GetContext(ctx, &u, `SELECT * FROM users WHERE email=? LIMIT 1`, email)
	if err != nil {
		if isNotFound(err) {
			return domain.User{}, false, nil
		}
		return domain.User{}, false, err
	}
	return u, true, nil
}

func (r *UsersRepo) GetByID(ctx context.Context, id uint64) (domain.User, bool, error) {
	var u domain.User
	err := r.db.GetContext(ctx, &u, `SELECT * FROM users WHERE id=?`, id)
	if err != nil {
		if isNotFound(err) {
			return domain.User{}, false, nil
		}
		return domain.User{}, false, err
	}
	return u, true, nil
}
