package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"github.com/gil1ges/taskforge-api/internal/service"
	"github.com/go-chi/jwtauth/v5"
)

type AuthHandler struct {
	svc *service.AuthService
	jwt *jwtauth.JWTAuth
}

func NewAuthHandler(svc *service.AuthService, jwt *jwtauth.JWTAuth) *AuthHandler {
	return &AuthHandler{svc: svc, jwt: jwt}
}

type authReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, domain.ErrBadRequest, http.StatusBadRequest)
		return
	}
	id, err := h.svc.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		writeDomainErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user_id": id})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, domain.ErrBadRequest, http.StatusBadRequest)
		return
	}
	u, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeDomainErr(w, err)
		return
	}

	_, tokenString, err := h.jwt.Encode(map[string]any{"user_id": u.ID})
	if err != nil {
		http.Error(w, "token encode error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": tokenString})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error, code int) {
	http.Error(w, err.Error(), code)
}

func writeDomainErr(w http.ResponseWriter, err error) {
	switch err {
	case domain.ErrBadRequest:
		writeErr(w, err, http.StatusBadRequest)
	case domain.ErrUnauthorized:
		writeErr(w, err, http.StatusUnauthorized)
	case domain.ErrForbidden:
		writeErr(w, err, http.StatusForbidden)
	case domain.ErrNotFound:
		writeErr(w, err, http.StatusNotFound)
	case domain.ErrConflict:
		writeErr(w, err, http.StatusConflict)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
