package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gil1ges/taskforge-api/internal/domain"
	hmw "github.com/gil1ges/taskforge-api/internal/http/middleware"
	"github.com/gil1ges/taskforge-api/internal/service"
	"github.com/go-chi/chi/v5"
)

type TeamsHandler struct {
	teams   *service.TeamsService
	invites *service.InvitesService
}

func NewTeamsHandler(teams *service.TeamsService, invites *service.InvitesService) *TeamsHandler {
	return &TeamsHandler{teams: teams, invites: invites}
}

type createTeamReq struct {
	Name string `json:"name"`
}

func (h *TeamsHandler) Create(w http.ResponseWriter, r *http.Request) {
	uid, ok := hmw.UserIDFromContext(r.Context())
	if !ok {
		writeDomainErr(w, domain.ErrUnauthorized)
		return
	}
	var req createTeamReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainErr(w, domain.ErrBadRequest)
		return
	}

	id, err := h.teams.CreateTeam(r.Context(), req.Name, uid)
	if err != nil {
		writeDomainErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"team_id": id})
}

func (h *TeamsHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := hmw.UserIDFromContext(r.Context())
	if !ok {
		writeDomainErr(w, domain.ErrUnauthorized)
		return
	}
	teams, err := h.teams.ListTeams(r.Context(), uid)
	if err != nil {
		writeDomainErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, teams)
}

type inviteReq struct {
	Email string      `json:"email"`
	Role  domain.Role `json:"role"`
}

func (h *TeamsHandler) Invite(w http.ResponseWriter, r *http.Request) {
	uid, ok := hmw.UserIDFromContext(r.Context())
	if !ok {
		writeDomainErr(w, domain.ErrUnauthorized)
		return
	}

	teamID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeDomainErr(w, domain.ErrBadRequest)
		return
	}

	var req inviteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainErr(w, domain.ErrBadRequest)
		return
	}

	code, err := h.invites.Invite(r.Context(), uint64(teamID), uid, req.Email, req.Role)
	if err != nil {
		writeDomainErr(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "created", "code": code})
}

type acceptInviteReq struct {
	Code string `json:"code"`
}

func (h *TeamsHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	uid, ok := hmw.UserIDFromContext(r.Context())
	if !ok {
		writeDomainErr(w, domain.ErrUnauthorized)
		return
	}

	teamID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeDomainErr(w, domain.ErrBadRequest)
		return
	}

	var req acceptInviteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainErr(w, domain.ErrBadRequest)
		return
	}

	if err := h.invites.Accept(r.Context(), uint64(teamID), uid, req.Code); err != nil {
		writeDomainErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "accepted"})
}
