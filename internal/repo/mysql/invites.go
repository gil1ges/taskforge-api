package mysql

import (
	"context"
	"time"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"github.com/jmoiron/sqlx"
)

type InvitesRepo struct{ db *sqlx.DB }

func NewInvitesRepo(db *sqlx.DB) *InvitesRepo { return &InvitesRepo{db: db} }

type Invite struct {
	ID        uint64      `db:"id"`
	TeamID    uint64      `db:"team_id"`
	Email     string      `db:"email"`
	Role      domain.Role `db:"role"`
	InvitedBy uint64      `db:"invited_by"`
	CodeHash  []byte      `db:"code_hash"`
	ExpiresAt time.Time   `db:"expires_at"`
	CreatedAt time.Time   `db:"created_at"`
}

func (r *InvitesRepo) Create(ctx context.Context, teamID uint64, email string, role domain.Role, invitedBy uint64, codeHash []byte, expiresAt time.Time) (uint64, error) {
	res, err := r.db.ExecContext(ctx, `
INSERT INTO team_invites(team_id, email, role, invited_by, code_hash, expires_at)
VALUES(?, ?, ?, ?, ?, ?)`,
		teamID, email, string(role), invitedBy, codeHash, expiresAt)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}

func (r *InvitesRepo) FindValidByTeamEmailCodeHash(ctx context.Context, teamID uint64, email string, codeHash []byte) (Invite, bool, error) {
	var inv Invite
	err := r.db.GetContext(ctx, &inv, `
SELECT * FROM team_invites
WHERE team_id=? AND email=? AND code_hash=? AND expires_at > NOW()
ORDER BY id DESC
LIMIT 1`, teamID, email, codeHash)
	if err != nil {
		if isNotFound(err) {
			return Invite{}, false, nil
		}
		return Invite{}, false, err
	}
	return inv, true, nil
}

func (r *InvitesRepo) Delete(ctx context.Context, inviteID uint64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM team_invites WHERE id=?`, inviteID)
	return err
}
