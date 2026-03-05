package mysql

import (
	"context"
	"fmt"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"github.com/jmoiron/sqlx"
)

type TasksRepo struct{ db *sqlx.DB }

func NewTasksRepo(db *sqlx.DB) *TasksRepo { return &TasksRepo{db: db} }

type ListTasksFilter struct {
	TeamID     uint64
	Status     *domain.TaskStatus
	AssigneeID *uint64
	Page       int
	Size       int
}

func (r *TasksRepo) Create(ctx context.Context, t domain.Task) (uint64, error) {
	res, err := r.db.ExecContext(ctx, `
INSERT INTO tasks(team_id, title, description, status, assignee_id, created_by)
VALUES(?, ?, ?, ?, ?, ?)`,
		t.TeamID, t.Title, t.Description, string(t.Status), t.AssigneeID, t.CreatedBy)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}

func (r *TasksRepo) List(ctx context.Context, f ListTasksFilter) ([]domain.Task, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Size <= 0 || f.Size > 100 {
		f.Size = 20
	}
	offset := (f.Page - 1) * f.Size

	where := "WHERE team_id=?"
	args := []any{f.TeamID}

	if f.Status != nil {
		where += " AND status=?"
		args = append(args, string(*f.Status))
	}
	if f.AssigneeID != nil {
		where += " AND assignee_id=?"
		args = append(args, *f.AssigneeID)
	}

	q := fmt.Sprintf(`SELECT * FROM tasks %s ORDER BY updated_at DESC LIMIT ? OFFSET ?`, where)
	args = append(args, f.Size, offset)

	var tasks []domain.Task
	err := r.db.SelectContext(ctx, &tasks, q, args...)
	return tasks, err
}

func (r *TasksRepo) GetByID(ctx context.Context, id uint64) (domain.Task, bool, error) {
	var t domain.Task
	err := r.db.GetContext(ctx, &t, `SELECT * FROM tasks WHERE id=?`, id)
	if err != nil {
		if isNotFound(err) {
			return domain.Task{}, false, nil
		}
		return domain.Task{}, false, err
	}
	return t, true, nil
}

type TaskUpdate struct {
	Title       *string
	Description **string
	Status      *domain.TaskStatus
	AssigneeID  **uint64
}

func (r *TasksRepo) Update(ctx context.Context, id uint64, u TaskUpdate) (domain.Task, domain.Task, error) {
	old, ok, err := r.GetByID(ctx, id)
	if err != nil {
		return domain.Task{}, domain.Task{}, err
	}
	if !ok {
		return domain.Task{}, domain.Task{}, domain.ErrNotFound
	}

	set := ""
	args := []any{}
	add := func(expr string, val any) {
		if set != "" {
			set += ", "
		}
		set += expr
		args = append(args, val)
	}

	if u.Title != nil {
		add("title=?", *u.Title)
	}
	if u.Description != nil {
		add("description=?", *u.Description)
	}
	if u.Status != nil {
		add("status=?", string(*u.Status))
	}
	if u.AssigneeID != nil {
		add("assignee_id=?", *u.AssigneeID)
	}

	if set == "" {
		return old, old, nil
	}

	args = append(args, id)
	_, err = r.db.ExecContext(ctx, `UPDATE tasks SET `+set+` WHERE id=?`, args...)
	if err != nil {
		return domain.Task{}, domain.Task{}, err
	}

	newv, _, err := r.GetByID(ctx, id)
	if err != nil {
		return domain.Task{}, domain.Task{}, err
	}
	return old, newv, nil
}

func (r *TasksRepo) AppendHistory(ctx context.Context, h domain.TaskHistory) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO task_history(task_id, changed_by, field_name, old_value, new_value)
VALUES(?, ?, ?, ?, ?)`,
		h.TaskID, h.ChangedBy, h.FieldName, h.OldValue, h.NewValue)
	return err
}

func (r *TasksRepo) History(ctx context.Context, taskID uint64) ([]domain.TaskHistory, error) {
	var hs []domain.TaskHistory
	err := r.db.SelectContext(ctx, &hs, `
SELECT * FROM task_history WHERE task_id=? ORDER BY changed_at DESC`, taskID)
	return hs, err
}

func (r *TasksRepo) AddComment(ctx context.Context, taskID, userID uint64, body string) (uint64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO task_comments(task_id, user_id, body) VALUES(?, ?, ?)`,
		taskID, userID, body)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}
