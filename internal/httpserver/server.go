package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Serve starts the HTTP server and blocks until ctx is canceled, at
// which point it drains in-flight requests (up to 15s) before returning.
// This is what prevents a rolling deploy or SIGTERM from cutting off a
// request mid-response.
func Serve(ctx context.Context, port string, handler http.Handler, log *slog.Logger) error {
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("httpserver: %w", err)
	case <-ctx.Done():
		log.Info("shutting down http server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
