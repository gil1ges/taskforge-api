package domain

import "time"

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
)

type User struct {
	ID           uint64    `db:"id" json:"id"`
	Email        string    `db:"email" json:"email"`
	PasswordHash []byte    `db:"password_hash" json:"-"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

type Team struct {
	ID        uint64    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	CreatedBy uint64    `db:"created_by" json:"created_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Task struct {
	ID          uint64     `db:"id" json:"id"`
	TeamID      uint64     `db:"team_id" json:"team_id"`
	Title       string     `db:"title" json:"title"`
	Description *string    `db:"description" json:"description"`
	Status      TaskStatus `db:"status" json:"status"`
	AssigneeID  *uint64    `db:"assignee_id" json:"assignee_id"`
	CreatedBy   uint64     `db:"created_by" json:"created_by"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}

type TaskHistory struct {
	ID        uint64    `db:"id" json:"id"`
	TaskID    uint64    `db:"task_id" json:"task_id"`
	ChangedBy uint64    `db:"changed_by" json:"changed_by"`
	ChangedAt time.Time `db:"changed_at" json:"changed_at"`
	FieldName string    `db:"field_name" json:"field_name"`
	OldValue  *string   `db:"old_value" json:"old_value"`
	NewValue  *string   `db:"new_value" json:"new_value"`
}

type Comment struct {
	ID        uint64    `db:"id" json:"id"`
	TaskID    uint64    `db:"task_id" json:"task_id"`
	UserID    uint64    `db:"user_id" json:"user_id"`
	Body      string    `db:"body" json:"body"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
