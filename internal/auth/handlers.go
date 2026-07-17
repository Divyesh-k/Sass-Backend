package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/divyeshkakadiya/saas-backend/internal/response"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}

	user, err := h.svc.Signup(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserExists):
			response.Error(w, http.StatusConflict, "user_exists", "an account with this email already exists")
		case errors.Is(err, ErrWeakPassword):
			response.Error(w, http.StatusBadRequest, "weak_password", "password must be at least 8 characters")
		default:
			h.log.Error("signup failed", "error", err)
			response.Error(w, http.StatusBadRequest, "invalid_request", err.Error())
		}
		return
	}

	response.JSON(w, http.StatusCreated, user)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}

	tokens, err := h.svc.Login(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			response.Error(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
			return
		}
		h.log.Error("login failed", "error", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "something went wrong")
		return
	}

	response.JSON(w, http.StatusOK, tokens)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}

	tokens, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "invalid_refresh_token", "refresh token is invalid or expired")
		return
	}

	response.JSON(w, http.StatusOK, tokens)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}

	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		h.log.Error("logout failed", "error", err)
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// Me returns the authenticated user's own profile. It relies entirely on
// the claims injected by middleware.RequireAuth — no extra DB lookup is
// strictly required, but we fetch the fresh row so a just-changed name
// or verified flag is reflected immediately.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthenticated", "no valid session")
		return
	}

	user, err := h.svc.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "user_not_found", "user not found")
		return
	}

	response.JSON(w, http.StatusOK, user)
}
