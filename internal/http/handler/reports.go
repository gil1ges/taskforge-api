package handler

import (
	"net/http"

	"github.com/gil1ges/taskforge-api/internal/repo/mysql"
)

type ReportsHandler struct {
	reports *mysql.ReportsRepo
}

func NewReportsHandler(reports *mysql.ReportsRepo) *ReportsHandler {
	return &ReportsHandler{reports: reports}
}

func (h *ReportsHandler) TeamSummaries(w http.ResponseWriter, r *http.Request) {
	out, err := h.reports.TeamSummaries(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ReportsHandler) TopCreators(w http.ResponseWriter, r *http.Request) {
	out, err := h.reports.TopCreatorsPerTeam(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ReportsHandler) InvalidAssignees(w http.ResponseWriter, r *http.Request) {
	out, err := h.reports.TasksWithInvalidAssignee(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
