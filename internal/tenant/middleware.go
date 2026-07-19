package tenant

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/divyeshkakadiya/saas-backend/internal/auth"
	"github.com/divyeshkakadiya/saas-backend/internal/response"
)

// RequireRole checks that the authenticated user (already validated by
// auth.RequireAuth upstream) is a member of the :orgID path parameter's
// organization with at least the given role. It lives in the tenant
// package rather than internal/middleware to avoid an import cycle with
// the Repository it depends on — this is a deliberate structural
// decision, not an oversight: cross-cutting concerns that need
// domain-specific data live next to that domain.
func RequireRole(repo *Repository, min Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := auth.UserIDFromContext(r.Context())
			if !ok {
				response.Error(w, http.StatusUnauthorized, "unauthenticated", "no valid session")
				return
			}

			orgID := chi.URLParam(r, "orgID")
			if orgID == "" {
				response.Error(w, http.StatusBadRequest, "missing_org_id", "organization ID is required in the URL")
				return
			}

			role, err := repo.GetMemberRole(r.Context(), orgID, userID)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					response.Error(w, http.StatusForbidden, "not_a_member", "you are not a member of this organization")
					return
				}
				response.Error(w, http.StatusInternalServerError, "internal_error", "something went wrong")
				return
			}

			if !role.AtLeast(min) {
				response.Error(w, http.StatusForbidden, "insufficient_role", "your role does not permit this action")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
