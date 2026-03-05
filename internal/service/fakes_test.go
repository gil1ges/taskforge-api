package service

import (
	"context"
	"time"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"github.com/gil1ges/taskforge-api/internal/repo/mysql"
)

type fakeUsersRepo struct {
	byEmail map[string]domain.User
	byID    map[uint64]domain.User

	findByEmailErr error
	getByIDErr     error
	createErr      error
	nextID         uint64

	createdEmail string
	createdHash  []byte
}

func (f *fakeUsersRepo) FindByEmail(ctx context.Context, email string) (domain.User, bool, error) {
	if f.findByEmailErr != nil {
		return domain.User{}, false, f.findByEmailErr
	}
	u, ok := f.byEmail[email]
	return u, ok, nil
}

func (f *fakeUsersRepo) Create(ctx context.Context, email string, passwordHash []byte) (uint64, error) {
	if f.createErr != nil {
		return 0, f.createErr
	}
	f.createdEmail = email
	f.createdHash = append([]byte(nil), passwordHash...)
	if f.nextID == 0 {
		f.nextID = 1
	}
	return f.nextID, nil
}

func (f *fakeUsersRepo) GetByID(ctx context.Context, id uint64) (domain.User, bool, error) {
	if f.getByIDErr != nil {
		return domain.User{}, false, f.getByIDErr
	}
	u, ok := f.byID[id]
	return u, ok, nil
}

type fakeTeamsRepo struct {
	createTeamID  uint64
	createTeamErr error

	listTeamsOut []domain.Team
	listTeamsErr error

	memberMap map[[2]uint64]bool
	roleMap   map[[2]uint64]domain.Role

	isMemberErr  error
	getRoleErr   error
	addMemberErr error

	addedMembers []struct {
		TeamID uint64
		UserID uint64
		Role   domain.Role
	}
}

func (f *fakeTeamsRepo) CreateTeamWithOwner(ctx context.Context, name string, creatorID uint64) (uint64, error) {
	if f.createTeamErr != nil {
		return 0, f.createTeamErr
	}
	if f.createTeamID == 0 {
		f.createTeamID = 1
	}
	return f.createTeamID, nil
}

func (f *fakeTeamsRepo) ListTeamsForUser(ctx context.Context, userID uint64) ([]domain.Team, error) {
	if f.listTeamsErr != nil {
		return nil, f.listTeamsErr
	}
	return append([]domain.Team(nil), f.listTeamsOut...), nil
}

func (f *fakeTeamsRepo) GetUserRole(ctx context.Context, teamID, userID uint64) (domain.Role, bool, error) {
	if f.getRoleErr != nil {
		return "", false, f.getRoleErr
	}
	if f.roleMap == nil {
		return "", false, nil
	}
	role, ok := f.roleMap[[2]uint64{teamID, userID}]
	return role, ok, nil
}

func (f *fakeTeamsRepo) IsMember(ctx context.Context, teamID, userID uint64) (bool, error) {
	if f.isMemberErr != nil {
		return false, f.isMemberErr
	}
	if f.memberMap == nil {
		return false, nil
	}
	return f.memberMap[[2]uint64{teamID, userID}], nil
}

func (f *fakeTeamsRepo) AddMember(ctx context.Context, teamID, userID uint64, role domain.Role) error {
	if f.addMemberErr != nil {
		return f.addMemberErr
	}
	f.addedMembers = append(f.addedMembers, struct {
		TeamID uint64
		UserID uint64
		Role   domain.Role
	}{
		TeamID: teamID,
		UserID: userID,
		Role:   role,
	})
	return nil
}

type fakeInvitesRepo struct {
	createErr error
	createID  uint64

	findInvite mysql.Invite
	findFound  bool
	findErr    error
	findEmail  string
	findTeamID uint64
	findHash   []byte

	deleteErr error
	deleted   []uint64

	lastCreate struct {
		TeamID    uint64
		Email     string
		Role      domain.Role
		InvitedBy uint64
		CodeHash  []byte
		ExpiresAt time.Time
	}
}

func (f *fakeInvitesRepo) Create(ctx context.Context, teamID uint64, email string, role domain.Role, invitedBy uint64, codeHash []byte, expiresAt time.Time) (uint64, error) {
	if f.createErr != nil {
		return 0, f.createErr
	}
	f.lastCreate.TeamID = teamID
	f.lastCreate.Email = email
	f.lastCreate.Role = role
	f.lastCreate.InvitedBy = invitedBy
	f.lastCreate.CodeHash = append([]byte(nil), codeHash...)
	f.lastCreate.ExpiresAt = expiresAt
	if f.createID == 0 {
		f.createID = 1
	}
	return f.createID, nil
}

func (f *fakeInvitesRepo) FindValidByTeamEmailCodeHash(ctx context.Context, teamID uint64, email string, codeHash []byte) (mysql.Invite, bool, error) {
	if f.findErr != nil {
		return mysql.Invite{}, false, f.findErr
	}
	if f.findEmail != "" && f.findEmail != email {
		return mysql.Invite{}, false, nil
	}
	if f.findTeamID != 0 && f.findTeamID != teamID {
		return mysql.Invite{}, false, nil
	}
	if len(f.findHash) > 0 {
		if len(codeHash) != len(f.findHash) {
			return mysql.Invite{}, false, nil
		}
		for i := range codeHash {
			if codeHash[i] != f.findHash[i] {
				return mysql.Invite{}, false, nil
			}
		}
	}
	if !f.findFound {
		return mysql.Invite{}, false, nil
	}
	return f.findInvite, true, nil
}

func (f *fakeInvitesRepo) Delete(ctx context.Context, inviteID uint64) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, inviteID)
	return nil
}

type fakeTasksRepo struct {
	createID  uint64
	createErr error
	created   []domain.Task

	listOut []domain.Task
	listErr error

	getTask  domain.Task
	getFound bool
	getErr   error

	updateOld domain.Task
	updateNew domain.Task
	updateErr error

	historyOut []domain.TaskHistory
	historyErr error

	appendedHistory []domain.TaskHistory
	appendErr       error
}

func (f *fakeTasksRepo) Create(ctx context.Context, t domain.Task) (uint64, error) {
	if f.createErr != nil {
		return 0, f.createErr
	}
	f.created = append(f.created, t)
	if f.createID == 0 {
		f.createID = 1
	}
	return f.createID, nil
}

func (f *fakeTasksRepo) List(ctx context.Context, filter mysql.ListTasksFilter) ([]domain.Task, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]domain.Task(nil), f.listOut...), nil
}

func (f *fakeTasksRepo) GetByID(ctx context.Context, id uint64) (domain.Task, bool, error) {
	if f.getErr != nil {
		return domain.Task{}, false, f.getErr
	}
	return f.getTask, f.getFound, nil
}

func (f *fakeTasksRepo) Update(ctx context.Context, id uint64, upd mysql.TaskUpdate) (domain.Task, domain.Task, error) {
	if f.updateErr != nil {
		return domain.Task{}, domain.Task{}, f.updateErr
	}
	return f.updateOld, f.updateNew, nil
}

func (f *fakeTasksRepo) AppendHistory(ctx context.Context, h domain.TaskHistory) error {
	if f.appendErr != nil {
		return f.appendErr
	}
	f.appendedHistory = append(f.appendedHistory, h)
	return nil
}

func (f *fakeTasksRepo) History(ctx context.Context, taskID uint64) ([]domain.TaskHistory, error) {
	if f.historyErr != nil {
		return nil, f.historyErr
	}
	return append([]domain.TaskHistory(nil), f.historyOut...), nil
}

type fakeTasksCache struct {
	getHit bool
	getErr error
	getOut []domain.Task

	setErr   error
	setCalls int

	invalidateErr   error
	invalidatedTeam []uint64
}

func (f *fakeTasksCache) GetTasks(ctx context.Context, teamID uint64, status string, assigneeID *uint64, page, size int, out any) (bool, error) {
	if f.getErr != nil {
		return false, f.getErr
	}
	if !f.getHit {
		return false, nil
	}
	dst, ok := out.(*[]domain.Task)
	if !ok {
		return false, nil
	}
	*dst = append((*dst)[:0], f.getOut...)
	return true, nil
}

func (f *fakeTasksCache) SetTasks(ctx context.Context, teamID uint64, status string, assigneeID *uint64, page, size int, v any) error {
	f.setCalls++
	return f.setErr
}

func (f *fakeTasksCache) InvalidateTeamTasks(ctx context.Context, teamID uint64) error {
	f.invalidatedTeam = append(f.invalidatedTeam, teamID)
	return f.invalidateErr
}
