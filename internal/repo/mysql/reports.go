package mysql

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type ReportsRepo struct{ db *sqlx.DB }

func NewReportsRepo(db *sqlx.DB) *ReportsRepo { return &ReportsRepo{db: db} }

type TeamSummary struct {
	TeamID       uint64 `db:"team_id" json:"team_id"`
	TeamName     string `db:"team_name" json:"team_name"`
	MembersCount int    `db:"members_count" json:"members_count"`
	Done7dCount  int    `db:"done_7d_count" json:"done_7d_count"`
}

func (r *ReportsRepo) TeamSummaries(ctx context.Context) ([]TeamSummary, error) {
	var out []TeamSummary
	err := r.db.SelectContext(ctx, &out, `
SELECT
  t.id AS team_id,
  t.name AS team_name,
  COUNT(DISTINCT tm.user_id) AS members_count,
  SUM(CASE WHEN tk.status='done' AND tk.updated_at >= (NOW() - INTERVAL 7 DAY) THEN 1 ELSE 0 END) AS done_7d_count
FROM teams t
LEFT JOIN team_members tm ON tm.team_id = t.id
LEFT JOIN tasks tk ON tk.team_id = t.id
GROUP BY t.id, t.name
ORDER BY t.id`)
	return out, err
}

type TopCreator struct {
	TeamID     uint64 `db:"team_id" json:"team_id"`
	UserID     uint64 `db:"user_id" json:"user_id"`
	Created    int    `db:"created_count" json:"created_count"`
	RankInTeam int    `db:"rk" json:"rk"`
}

func (r *ReportsRepo) TopCreatorsPerTeam(ctx context.Context) ([]TopCreator, error) {
	var out []TopCreator
	err := r.db.SelectContext(ctx, &out, `
WITH per_user AS (
  SELECT team_id, created_by AS user_id, COUNT(*) AS created_count
  FROM tasks
  WHERE created_at >= (DATE_SUB(CURDATE(), INTERVAL 1 MONTH))
  GROUP BY team_id, created_by
),
ranked AS (
  SELECT *,
    DENSE_RANK() OVER (PARTITION BY team_id ORDER BY created_count DESC) AS rk
  FROM per_user
)
SELECT team_id, user_id, created_count, rk
FROM ranked
WHERE rk <= 3
ORDER BY team_id, rk, user_id`)
	return out, err
}

type IntegrityIssue struct {
	TaskID     uint64  `db:"task_id" json:"task_id"`
	TeamID     uint64  `db:"team_id" json:"team_id"`
	AssigneeID *uint64 `db:"assignee_id" json:"assignee_id"`
}

func (r *ReportsRepo) TasksWithInvalidAssignee(ctx context.Context) ([]IntegrityIssue, error) {
	var out []IntegrityIssue
	err := r.db.SelectContext(ctx, &out, `
SELECT tk.id AS task_id, tk.team_id, tk.assignee_id
FROM tasks tk
LEFT JOIN team_members tm
  ON tm.team_id = tk.team_id AND tm.user_id = tk.assignee_id
WHERE tk.assignee_id IS NOT NULL
  AND tm.user_id IS NULL
ORDER BY tk.id`)
	return out, err
}
