package httpserver

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/divyeshkakadiya/saas-backend/internal/response"
)

// healthHandlers exposes the two endpoints any container orchestrator
// (Kubernetes, ECS, Fly.io) or load balancer needs:
//   - /healthz: "is the process alive" — never checks dependencies, so a
//     slow database doesn't cause the orchestrator to kill a healthy pod.
//   - /readyz: "is the process ready to serve traffic" — checks DB and
//     Redis connectivity, so the load balancer stops routing to an
//     instance that's up but can't actually do its job.
type healthHandlers struct {
	db  *sql.DB
	rdb *redis.Client
}

func (h *healthHandlers) liveness(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (h *healthHandlers) readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := map[string]string{}
	ready := true

	if err := h.db.PingContext(ctx); err != nil {
		checks["database"] = "unavailable"
		ready = false
	} else {
		checks["database"] = "ok"
	}

	if err := h.rdb.Ping(ctx).Err(); err != nil {
		checks["redis"] = "unavailable"
		ready = false
	} else {
		checks["redis"] = "ok"
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	response.JSON(w, status, checks)
}
