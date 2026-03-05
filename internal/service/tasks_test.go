package service

import (
	"context"
	"testing"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"github.com/gil1ges/taskforge-api/internal/repo/mysql"
)

func TestIsValidTaskStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status domain.TaskStatus
		valid  bool
	}{
		{status: domain.StatusTodo, valid: true},
		{status: domain.StatusInProgress, valid: true},
		{status: domain.StatusDone, valid: true},
		{status: domain.TaskStatus("blocked"), valid: false},
		{status: domain.TaskStatus(""), valid: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.status), func(t *testing.T) {
			t.Parallel()
			if got := isValidTaskStatus(tc.status); got != tc.valid {
				t.Fatalf("isValidTaskStatus(%q) = %v, want %v", tc.status, got, tc.valid)
			}
		})
	}
}

func TestCanEditTask(t *testing.T) {
	t.Parallel()

	assigneeID := uint64(42)
	task := domain.Task{
		CreatedBy:  7,
		AssigneeID: &assigneeID,
	}

	tests := []struct {
		name   string
		role   domain.Role
		userID uint64
		want   bool
	}{
		{name: "owner can edit", role: domain.RoleOwner, userID: 1, want: true},
		{name: "admin can edit", role: domain.RoleAdmin, userID: 2, want: true},
		{name: "creator can edit", role: domain.RoleMember, userID: 7, want: true},
		{name: "assignee can edit", role: domain.RoleMember, userID: 42, want: true},
		{name: "other member cannot edit", role: domain.RoleMember, userID: 100, want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := canEditTask(tc.role, task, tc.userID); got != tc.want {
				t.Fatalf("canEditTask() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTasksServiceCreate(t *testing.T) {
	t.Parallel()

	memberMap := map[[2]uint64]bool{
		{1, 10}: true,
		{1, 11}: true,
	}
	teams := &fakeTeamsRepo{memberMap: memberMap}
	repo := &fakeTasksRepo{createID: 55}
	cache := &fakeTasksCache{}

	svc := NewTasksService(teams, repo, cache)

	_, err := svc.Create(context.Background(), domain.Task{TeamID: 1, CreatedBy: 10, Title: " ", Status: domain.StatusTodo})
	if err != domain.ErrBadRequest {
		t.Fatalf("Create() empty title error = %v, want %v", err, domain.ErrBadRequest)
	}

	_, err = svc.Create(context.Background(), domain.Task{TeamID: 1, CreatedBy: 99, Title: "task", Status: domain.StatusTodo})
	if err != domain.ErrForbidden {
		t.Fatalf("Create() non-member error = %v, want %v", err, domain.ErrForbidden)
	}

	_, err = svc.Create(context.Background(), domain.Task{TeamID: 1, CreatedBy: 10, Title: "task", Status: "invalid"})
	if err != domain.ErrBadRequest {
		t.Fatalf("Create() invalid status error = %v, want %v", err, domain.ErrBadRequest)
	}

	assigneeID := uint64(999)
	_, err = svc.Create(context.Background(), domain.Task{
		TeamID:     1,
		CreatedBy:  10,
		Title:      "task",
		Status:     domain.StatusTodo,
		AssigneeID: &assigneeID,
	})
	if err != domain.ErrBadRequest {
		t.Fatalf("Create() invalid assignee error = %v, want %v", err, domain.ErrBadRequest)
	}

	id, err := svc.Create(context.Background(), domain.Task{
		TeamID:     1,
		CreatedBy:  10,
		Title:      "  task  ",
		AssigneeID: ptrUint64(11),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if id != 55 {
		t.Fatalf("Create() id = %d, want %d", id, 55)
	}
	if len(repo.created) == 0 || repo.created[len(repo.created)-1].Status != domain.StatusTodo {
		t.Fatalf("Create() should set default status todo, got %+v", repo.created)
	}
	if len(cache.invalidatedTeam) == 0 || cache.invalidatedTeam[len(cache.invalidatedTeam)-1] != 1 {
		t.Fatalf("Create() should invalidate team cache, got %+v", cache.invalidatedTeam)
	}
}

func TestTasksServiceList(t *testing.T) {
	t.Parallel()

	teams := &fakeTeamsRepo{
		memberMap: map[[2]uint64]bool{{1, 10}: true},
	}
	repo := &fakeTasksRepo{
		listOut: []domain.Task{{ID: 1, TeamID: 1, Title: "t1", Status: domain.StatusTodo}},
	}
	cache := &fakeTasksCache{}
	svc := NewTasksService(teams, repo, cache)

	_, err := svc.List(context.Background(), ListParams{TeamID: 0, UserID: 10, Page: 1, Size: 20})
	if err != domain.ErrBadRequest {
		t.Fatalf("List() team=0 error = %v, want %v", err, domain.ErrBadRequest)
	}

	invalid := domain.TaskStatus("blocked")
	_, err = svc.List(context.Background(), ListParams{TeamID: 1, UserID: 10, Status: &invalid, Page: 1, Size: 20})
	if err != domain.ErrBadRequest {
		t.Fatalf("List() invalid status error = %v, want %v", err, domain.ErrBadRequest)
	}

	_, err = svc.List(context.Background(), ListParams{TeamID: 1, UserID: 99, Page: 1, Size: 20})
	if err != domain.ErrForbidden {
		t.Fatalf("List() non-member error = %v, want %v", err, domain.ErrForbidden)
	}

	cache.getHit = true
	cache.getOut = []domain.Task{{ID: 999, TeamID: 1, Title: "cached", Status: domain.StatusDone}}
	out, err := svc.List(context.Background(), ListParams{TeamID: 1, UserID: 10, Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("List() cache hit error = %v", err)
	}
	if len(out) != 1 || out[0].ID != 999 {
		t.Fatalf("List() cache hit output = %+v", out)
	}

	cache.getHit = false
	cache.getErr = context.DeadlineExceeded
	out, err = svc.List(context.Background(), ListParams{TeamID: 1, UserID: 10, Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("List() fallback error = %v", err)
	}
	if len(out) != 1 || out[0].ID != 1 {
		t.Fatalf("List() fallback output = %+v", out)
	}
	if cache.setCalls == 0 {
		t.Fatal("List() should attempt to write cache after DB read")
	}
}

func TestTasksServiceUpdate(t *testing.T) {
	t.Parallel()

	oldTask := domain.Task{
		ID:          100,
		TeamID:      1,
		Title:       "old title",
		Description: ptrString("old"),
		Status:      domain.StatusTodo,
		AssigneeID:  ptrUint64(22),
		CreatedBy:   10,
	}
	newTask := domain.Task{
		ID:          100,
		TeamID:      1,
		Title:       "new title",
		Description: ptrString("new"),
		Status:      domain.StatusDone,
		AssigneeID:  ptrUint64(33),
		CreatedBy:   10,
	}

	teams := &fakeTeamsRepo{
		memberMap: map[[2]uint64]bool{
			{1, 10}: true,
			{1, 33}: true,
		},
		roleMap: map[[2]uint64]domain.Role{
			{1, 10}: domain.RoleOwner,
		},
	}
	repo := &fakeTasksRepo{
		getTask:    oldTask,
		getFound:   true,
		updateOld:  oldTask,
		updateNew:  newTask,
		historyOut: nil,
	}
	cache := &fakeTasksCache{}
	svc := NewTasksService(teams, repo, cache)

	_, err := svc.Update(context.Background(), 100, 10, mysql.TaskUpdate{Status: ptrStatus("blocked")})
	if err != domain.ErrBadRequest {
		t.Fatalf("Update() invalid status error = %v, want %v", err, domain.ErrBadRequest)
	}

	_, err = svc.Update(context.Background(), 100, 10, mysql.TaskUpdate{
		AssigneeID: ptrPtrUint64(999),
	})
	if err != domain.ErrBadRequest {
		t.Fatalf("Update() invalid assignee error = %v, want %v", err, domain.ErrBadRequest)
	}

	updated, err := svc.Update(context.Background(), 100, 10, mysql.TaskUpdate{
		Title:       ptrString("new title"),
		Description: ptrPtrString("new"),
		Status:      ptrStatus(string(domain.StatusDone)),
		AssigneeID:  ptrPtrUint64(33),
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Status != domain.StatusDone {
		t.Fatalf("Update() status = %v, want %v", updated.Status, domain.StatusDone)
	}
	if len(repo.appendedHistory) < 4 {
		t.Fatalf("Update() expected history records for changed fields, got %d", len(repo.appendedHistory))
	}
	if len(cache.invalidatedTeam) == 0 || cache.invalidatedTeam[0] != 1 {
		t.Fatalf("Update() should invalidate team cache, got %+v", cache.invalidatedTeam)
	}
}

func TestTasksServiceUpdatePermissionAndNotFound(t *testing.T) {
	t.Parallel()

	repo := &fakeTasksRepo{getFound: false}
	teams := &fakeTeamsRepo{}
	svc := NewTasksService(teams, repo, &fakeTasksCache{})

	_, err := svc.Update(context.Background(), 1, 1, mysql.TaskUpdate{})
	if err != domain.ErrNotFound {
		t.Fatalf("Update() not found error = %v, want %v", err, domain.ErrNotFound)
	}

	repo.getFound = true
	repo.getTask = domain.Task{ID: 1, TeamID: 1, CreatedBy: 5, Status: domain.StatusTodo}

	_, err = svc.Update(context.Background(), 1, 1, mysql.TaskUpdate{})
	if err != domain.ErrForbidden {
		t.Fatalf("Update() non-member error = %v, want %v", err, domain.ErrForbidden)
	}
}

func TestTasksServiceHistory(t *testing.T) {
	t.Parallel()

	task := domain.Task{ID: 1, TeamID: 1, CreatedBy: 10, Status: domain.StatusTodo}
	repo := &fakeTasksRepo{
		getTask:    task,
		getFound:   true,
		historyOut: []domain.TaskHistory{{ID: 1, TaskID: 1, FieldName: "status"}},
	}
	teams := &fakeTeamsRepo{
		memberMap: map[[2]uint64]bool{{1, 10}: true},
	}
	svc := NewTasksService(teams, repo, &fakeTasksCache{})

	out, err := svc.History(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if len(out) != 1 || out[0].FieldName != "status" {
		t.Fatalf("History() output = %+v", out)
	}

	_, err = svc.History(context.Background(), 1, 999)
	if err != domain.ErrForbidden {
		t.Fatalf("History() non-member error = %v, want %v", err, domain.ErrForbidden)
	}

	repo.getFound = false
	_, err = svc.History(context.Background(), 1, 10)
	if err != domain.ErrNotFound {
		t.Fatalf("History() not found error = %v, want %v", err, domain.ErrNotFound)
	}
}

func ptrString(v string) *string { return &v }

func ptrPtrString(v string) **string {
	p := &v
	return &p
}

func ptrUint64(v uint64) *uint64 { return &v }

func ptrPtrUint64(v uint64) **uint64 {
	p := &v
	return &p
}

func ptrStatus(v string) *domain.TaskStatus {
	s := domain.TaskStatus(v)
	return &s
}
