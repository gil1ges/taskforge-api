package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gil1ges/taskforge-api/internal/domain"
	hmw "github.com/gil1ges/taskforge-api/internal/http/middleware"
	"github.com/gil1ges/taskforge-api/internal/repo/mysql"
	"github.com/gil1ges/taskforge-api/internal/service"
	"github.com/go-chi/chi/v5"
)

type TasksHandler struct {
	svc *service.TasksService
}

func NewTasksHandler(svc *service.TasksService) *TasksHandler { return &TasksHandler{svc: svc} }

type createTaskReq struct {
	TeamID      uint64  `json:"team_id"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
	AssigneeID  *uint64 `json:"assignee_id"`
}

func (h *TasksHandler) Create(w http.ResponseWriter, r *http.Request) {
	uid, ok := hmw.UserIDFromContext(r.Context())
	if !ok {
		writeDomainErr(w, domain.ErrUnauthorized)
		return
	}

	var req createTaskReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainErr(w, domain.ErrBadRequest)
		return
	}

	t := domain.Task{
		TeamID:      req.TeamID,
		Title:       req.Title,
		Description: req.Description,
		AssigneeID:  req.AssigneeID,
		CreatedBy:   uid,
		Status:      domain.TaskStatus(req.Status),
	}
	id, err := h.svc.Create(r.Context(), t)
	if err != nil {
		writeDomainErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"task_id": id})
}

func (h *TasksHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := hmw.UserIDFromContext(r.Context())
	if !ok {
		writeDomainErr(w, domain.ErrUnauthorized)
		return
	}

	q := r.URL.Query()
	teamID, err := strconv.ParseUint(q.Get("team_id"), 10, 64)
	if err != nil || teamID == 0 {
		writeDomainErr(w, domain.ErrBadRequest)
		return
	}

	var status *domain.TaskStatus
	if s := q.Get("status"); s != "" {
		st := domain.TaskStatus(s)
		status = &st
	}

	var assigneeID *uint64
	if a := q.Get("assignee_id"); a != "" {
		v, err := strconv.ParseUint(a, 10, 64)
		if err != nil {
			writeDomainErr(w, domain.ErrBadRequest)
			return
		}
		tmp := uint64(v)
		assigneeID = &tmp
	}

	page := 1
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	size := 20
	if s := q.Get("size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 100 {
			size = v
		}
	}

	out, err := h.svc.List(r.Context(), service.ListParams{
		TeamID:     uint64(teamID),
		Status:     status,
		AssigneeID: assigneeID,
		Page:       page,
		Size:       size,
		UserID:     uid,
	})
	if err != nil {
		writeDomainErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type updateTaskReq struct {
	Title            *string `json:"title"`
	Description      *string `json:"description"`
	Status           *string `json:"status"`
	AssigneeID       *uint64 `json:"assignee_id"`
	ClearAssignee    bool    `json:"clear_assignee"`
	ClearDescription bool    `json:"clear_description"`
}

func (h *TasksHandler) Update(w http.ResponseWriter, r *http.Request) {
	uid, ok := hmw.UserIDFromContext(r.Context())
	if !ok {
		writeDomainErr(w, domain.ErrUnauthorized)
		return
	}

	taskID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeDomainErr(w, domain.ErrBadRequest)
		return
	}

	var req updateTaskReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainErr(w, domain.ErrBadRequest)
		return
	}

	var upd mysql.TaskUpdate
	upd.Title = req.Title

	if req.ClearDescription {
		var nilStr *string = nil
		upd.Description = &nilStr
	} else if req.Description != nil {
		v := req.Description
		upd.Description = &v
	}

	if req.Status != nil {
		st := domain.TaskStatus(*req.Status)
		upd.Status = &st
	}

	if req.ClearAssignee {
		var nilID *uint64 = nil
		upd.AssigneeID = &nilID
	} else if req.AssigneeID != nil {
		v := req.AssigneeID
		upd.AssigneeID = &v
	}

	newv, err := h.svc.Update(r.Context(), uint64(taskID), uid, upd)
	if err != nil {
		writeDomainErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newv)
}

func (h *TasksHandler) History(w http.ResponseWriter, r *http.Request) {
	uid, ok := hmw.UserIDFromContext(r.Context())
	if !ok {
		writeDomainErr(w, domain.ErrUnauthorized)
		return
	}

	taskID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeDomainErr(w, domain.ErrBadRequest)
		return
	}

	out, err := h.svc.History(r.Context(), uint64(taskID), uid)
	if err != nil {
		writeDomainErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
