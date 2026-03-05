package mysql

import (
	"context"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"github.com/jmoiron/sqlx"
)

type TeamsRepo struct{ db *sqlx.DB }

func NewTeamsRepo(db *sqlx.DB) *TeamsRepo { return &TeamsRepo{db: db} }

func (r *TeamsRepo) CreateTeamWithOwner(ctx context.Context, name string, creatorID uint64) (uint64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `INSERT INTO teams(name, created_by) VALUES(?, ?)`, name, creatorID)
	if err != nil {
		return 0, err
	}
	teamID64, _ := res.LastInsertId()
	teamID := uint64(teamID64)

	_, err = tx.ExecContext(ctx,
		`INSERT INTO team_members(team_id, user_id, role) VALUES(?, ?, 'owner')`,
		teamID, creatorID)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return teamID, nil
}

func (r *TeamsRepo) ListTeamsForUser(ctx context.Context, userID uint64) ([]domain.Team, error) {
	var teams []domain.Team
	err := r.db.SelectContext(ctx, &teams, `
SELECT t.*
FROM teams t
JOIN team_members tm ON tm.team_id = t.id
WHERE tm.user_id = ?
ORDER BY t.created_at DESC`, userID)
	return teams, err
}

func (r *TeamsRepo) GetUserRole(ctx context.Context, teamID, userID uint64) (domain.Role, bool, error) {
	var role string
	err := r.db.GetContext(ctx, &role, `SELECT role FROM team_members WHERE team_id=? AND user_id=?`, teamID, userID)
	if err != nil {
		if isNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return domain.Role(role), true, nil
}

func (r *TeamsRepo) IsMember(ctx context.Context, teamID, userID uint64) (bool, error) {
	var v int
	err := r.db.GetContext(ctx, &v, `SELECT 1 FROM team_members WHERE team_id=? AND user_id=?`, teamID, userID)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *TeamsRepo) AddMember(ctx context.Context, teamID, userID uint64, role domain.Role) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO team_members(team_id, user_id, role) VALUES(?, ?, ?)`,
		teamID, userID, string(role))
	return err
}
