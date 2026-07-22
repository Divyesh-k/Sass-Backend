package httpserver

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"

	"github.com/divyeshkakadiya/saas-backend/internal/auth"
	"github.com/divyeshkakadiya/saas-backend/internal/billing"
	"github.com/divyeshkakadiya/saas-backend/internal/config"
	appmw "github.com/divyeshkakadiya/saas-backend/internal/middleware"
	"github.com/divyeshkakadiya/saas-backend/internal/tenant"
)

type Deps struct {
	Config         *config.Config
	DB             *sql.DB
	Redis          *redis.Client
	Logger         *slog.Logger
	AuthHandler    *auth.Handler
	Tokens         *auth.TokenManager
	TenantHandler  *tenant.Handler
	TenantRepo     *tenant.Repository
	BillingHandler *billing.Handler
}

// NewRouter wires every route in the service. Reading this function
// top-to-bottom should tell a new engineer (or a client reviewing the
// code) exactly what the API surface is, without needing to trace
// through a dozen files first.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	// --- Global middleware, applied to every request ---
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer) // convert panics into 500s instead of crashing the process
	r.Use(appmw.AccessLog(d.Logger))
	r.Use(chimw.Timeout(30 * time.Second))

	m := newMetrics()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			rec := &statusCapture{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, req)
			m.RecordRequest(req.Method, routePattern(req), rec.status, time.Since(start))
		})
	})

	// --- Operational endpoints (no auth, no rate limit) ---
	health := &healthHandlers{db: d.DB, rdb: d.Redis}
	r.Get("/healthz", health.liveness)
	r.Get("/readyz", health.readiness)
	r.Get("/metrics", m.handler)

	// --- Public API ---
	r.Route("/api/v1", func(api chi.Router) {
		api.Use(appmw.RateLimit(d.Redis, d.Config.RateLimitRPS, time.Minute))

		// Billing webhooks must NOT be behind the idempotency middleware's
		// JSON assumptions or auth — Stripe calls this directly and signs
		// the raw body itself.
		api.Post("/billing/webhook", d.BillingHandler.Webhook)

		api.Route("/auth", func(ar chi.Router) {
			ar.Post("/signup", d.AuthHandler.Signup)
			ar.Post("/login", d.AuthHandler.Login)
			ar.Post("/refresh", d.AuthHandler.Refresh)
			ar.Post("/logout", d.AuthHandler.Logout)
		})

		// --- Authenticated routes ---
		api.Group(func(pr chi.Router) {
			pr.Use(appmw.RequireAuth(d.Tokens))
			pr.Use(appmw.Idempotency(d.Redis))

			pr.Get("/me", d.AuthHandler.Me)

			pr.Route("/orgs", func(or chi.Router) {
				or.Post("/", d.TenantHandler.Create)
				or.Get("/", d.TenantHandler.ListMine)

				or.Route("/{orgID}", func(sr chi.Router) {
					sr.Use(tenant.RequireRole(d.TenantRepo, tenant.RoleMember))
					sr.Get("/members", d.TenantHandler.ListMembers)
					sr.Post("/billing/portal", d.BillingHandler.CreatePortalSession)

					sr.Group(func(ar chi.Router) {
						ar.Use(tenant.RequireRole(d.TenantRepo, tenant.RoleAdmin))
						ar.Post("/members", d.TenantHandler.Invite)
						ar.Delete("/members/{userID}", d.TenantHandler.RemoveMember)
						ar.Post("/billing/checkout", d.BillingHandler.CreateCheckoutSession)
					})
				})
			})
		})
	})

	return r
}

type statusCapture struct {
	http.ResponseWriter
	status int
}

func (s *statusCapture) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
		return rc.RoutePattern()
	}
	return r.URL.Path
}
