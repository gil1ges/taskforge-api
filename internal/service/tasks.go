package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"github.com/gil1ges/taskforge-api/internal/repo/mysql"
)

type TasksService struct {
	teams tasksTeamsRepo
	tasks tasksRepo
	cache tasksCache
}

type tasksTeamsRepo interface {
	IsMember(ctx context.Context, teamID, userID uint64) (bool, error)
	GetUserRole(ctx context.Context, teamID, userID uint64) (domain.Role, bool, error)
}

type tasksRepo interface {
	Create(ctx context.Context, t domain.Task) (uint64, error)
	List(ctx context.Context, f mysql.ListTasksFilter) ([]domain.Task, error)
	GetByID(ctx context.Context, id uint64) (domain.Task, bool, error)
	Update(ctx context.Context, id uint64, u mysql.TaskUpdate) (domain.Task, domain.Task, error)
	AppendHistory(ctx context.Context, h domain.TaskHistory) error
	History(ctx context.Context, taskID uint64) ([]domain.TaskHistory, error)
}

type tasksCache interface {
	GetTasks(ctx context.Context, teamID uint64, status string, assigneeID *uint64, page, size int, out any) (bool, error)
	SetTasks(ctx context.Context, teamID uint64, status string, assigneeID *uint64, page, size int, v any) error
	InvalidateTeamTasks(ctx context.Context, teamID uint64) error
}

func NewTasksService(teams tasksTeamsRepo, tasks tasksRepo, cache tasksCache) *TasksService {
	return &TasksService{teams: teams, tasks: tasks, cache: cache}
}

func (s *TasksService) Create(ctx context.Context, t domain.Task) (uint64, error) {
	t.Title = strings.TrimSpace(t.Title)
	if t.TeamID == 0 || t.CreatedBy == 0 || t.Title == "" {
		return 0, domain.ErrBadRequest
	}

	isMember, err := s.teams.IsMember(ctx, t.TeamID, t.CreatedBy)
	if err != nil {
		return 0, err
	}
	if !isMember {
		return 0, domain.ErrForbidden
	}

	if t.Status == "" {
		t.Status = domain.StatusTodo
	}
	if !isValidTaskStatus(t.Status) {
		return 0, domain.ErrBadRequest
	}

	if t.AssigneeID != nil {
		ok, err := s.teams.IsMember(ctx, t.TeamID, *t.AssigneeID)
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, domain.ErrBadRequest
		}
	}

	id, err := s.tasks.Create(ctx, t)
	if err != nil {
		return 0, err
	}
	s.invalidateTeamTasks(ctx, t.TeamID)
	return id, nil
}

type ListParams struct {
	TeamID     uint64
	Status     *domain.TaskStatus
	AssigneeID *uint64
	Page       int
	Size       int
	UserID     uint64
}

func (s *TasksService) List(ctx context.Context, p ListParams) ([]domain.Task, error) {
	if p.TeamID == 0 {
		return nil, domain.ErrBadRequest
	}
	if p.Status != nil && !isValidTaskStatus(*p.Status) {
		return nil, domain.ErrBadRequest
	}
	isMember, err := s.teams.IsMember(ctx, p.TeamID, p.UserID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, domain.ErrForbidden
	}

	statusKey := "any"
	if p.Status != nil {
		statusKey = string(*p.Status)
	}

	if s.cache != nil {
		var cached []domain.Task
		hit, err := s.cache.GetTasks(ctx, p.TeamID, statusKey, p.AssigneeID, p.Page, p.Size, &cached)
		if err == nil && hit {
			return cached, nil
		}
	}

	out, err := s.tasks.List(ctx, mysql.ListTasksFilter{
		TeamID: p.TeamID, Status: p.Status, AssigneeID: p.AssigneeID, Page: p.Page, Size: p.Size,
	})
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		_ = s.cache.SetTasks(ctx, p.TeamID, statusKey, p.AssigneeID, p.Page, p.Size, out)
	}
	return out, nil
}

func (s *TasksService) Update(ctx context.Context, taskID uint64, userID uint64, upd mysql.TaskUpdate) (domain.Task, error) {
	t, ok, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return domain.Task{}, err
	}
	if !ok {
		return domain.Task{}, domain.ErrNotFound
	}

	isMember, err := s.teams.IsMember(ctx, t.TeamID, userID)
	if err != nil {
		return domain.Task{}, err
	}
	if !isMember {
		return domain.Task{}, domain.ErrForbidden
	}

	role, okRole, err := s.teams.GetUserRole(ctx, t.TeamID, userID)
	if err != nil {
		return domain.Task{}, err
	}
	if !okRole {
		return domain.Task{}, domain.ErrForbidden
	}

	if !canEditTask(role, t, userID) {
		return domain.Task{}, domain.ErrForbidden
	}

	if upd.Status != nil && !isValidTaskStatus(*upd.Status) {
		return domain.Task{}, domain.ErrBadRequest
	}

	if upd.AssigneeID != nil && *upd.AssigneeID != nil {
		ok, err := s.teams.IsMember(ctx, t.TeamID, **upd.AssigneeID)
		if err != nil {
			return domain.Task{}, err
		}
		if !ok {
			return domain.Task{}, domain.ErrBadRequest
		}
	}

	old, newv, err := s.tasks.Update(ctx, taskID, upd)
	if err != nil {
		return domain.Task{}, err
	}

	writeChange := func(field string, ov, nv *string) {
		_ = s.tasks.AppendHistory(ctx, domain.TaskHistory{
			TaskID: taskID, ChangedBy: userID, FieldName: field, OldValue: ov, NewValue: nv,
		})
	}

	if old.Title != newv.Title {
		ov := old.Title
		nv := newv.Title
		writeChange("title", &ov, &nv)
	}

	descChanged := (old.Description == nil) != (newv.Description == nil)
	if !descChanged && old.Description != nil && newv.Description != nil && *old.Description != *newv.Description {
		descChanged = true
	}
	if descChanged {
		var ov, nv *string
		if old.Description != nil {
			x := *old.Description
			ov = &x
		}
		if newv.Description != nil {
			x := *newv.Description
			nv = &x
		}
		writeChange("description", ov, nv)
	}

	if old.Status != newv.Status {
		ov := string(old.Status)
		nv := string(newv.Status)
		writeChange("status", &ov, &nv)
	}

	assChanged := (old.AssigneeID == nil) != (newv.AssigneeID == nil)
	if !assChanged && old.AssigneeID != nil && newv.AssigneeID != nil && *old.AssigneeID != *newv.AssigneeID {
		assChanged = true
	}
	if assChanged {
		var ov, nv *string
		if old.AssigneeID != nil {
			x := strconv.FormatUint(*old.AssigneeID, 10)
			ov = &x
		}
		if newv.AssigneeID != nil {
			x := strconv.FormatUint(*newv.AssigneeID, 10)
			nv = &x
		}
		writeChange("assignee_id", ov, nv)
	}

	s.invalidateTeamTasks(ctx, t.TeamID)
	return newv, nil
}

func (s *TasksService) History(ctx context.Context, taskID, userID uint64) ([]domain.TaskHistory, error) {
	t, ok, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrNotFound
	}

	isMember, err := s.teams.IsMember(ctx, t.TeamID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, domain.ErrForbidden
	}
	return s.tasks.History(ctx, taskID)
}

func isValidTaskStatus(s domain.TaskStatus) bool {
	switch s {
	case domain.StatusTodo, domain.StatusInProgress, domain.StatusDone:
		return true
	default:
		return false
	}
}

func canEditTask(role domain.Role, t domain.Task, userID uint64) bool {
	if role == domain.RoleOwner || role == domain.RoleAdmin {
		return true
	}
	if t.CreatedBy == userID {
		return true
	}
	if t.AssigneeID != nil && *t.AssigneeID == userID {
		return true
	}
	return false
}

func (s *TasksService) invalidateTeamTasks(ctx context.Context, teamID uint64) {
	if s.cache == nil {
		return
	}
	_ = s.cache.InvalidateTeamTasks(ctx, teamID)
}
