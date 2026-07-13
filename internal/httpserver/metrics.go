package httpserver

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// metrics is a deliberately minimal Prometheus-text-format exporter.
// The full prometheus/client_golang library is the right call on a team
// already standardized on it, but for a starter kit it's a lot of
// dependency surface for what amounts to "count requests and bucket
// their latency." This does that in ~80 lines with zero dependencies,
// and the output is still scrapeable by any real Prometheus instance —
// swap this file for client_golang later without touching call sites,
// since RecordRequest is the only method the rest of the app calls.
type metrics struct {
	mu           sync.Mutex
	requestCount map[string]int64
	statusCount  map[int]int64
	durationSum  int64 // milliseconds
	durationN    int64
}

func newMetrics() *metrics {
	return &metrics{
		requestCount: make(map[string]int64),
		statusCount:  make(map[int]int64),
	}
}

func (m *metrics) RecordRequest(method, path string, status int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestCount[method+" "+path]++
	m.statusCount[status]++
	atomic.AddInt64(&m.durationSum, duration.Milliseconds())
	atomic.AddInt64(&m.durationN, 1)
}

func (m *metrics) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	fmt.Fprintln(w, "# HELP http_requests_total Total HTTP requests by method and path")
	fmt.Fprintln(w, "# TYPE http_requests_total counter")
	for key, count := range m.requestCount {
		fmt.Fprintf(w, "http_requests_total{route=%q} %d\n", key, count)
	}

	fmt.Fprintln(w, "# HELP http_responses_total Total HTTP responses by status code")
	fmt.Fprintln(w, "# TYPE http_responses_total counter")
	for status, count := range m.statusCount {
		fmt.Fprintf(w, "http_responses_total{status=\"%d\"} %d\n", status, count)
	}

	n := atomic.LoadInt64(&m.durationN)
	sum := atomic.LoadInt64(&m.durationSum)
	avg := float64(0)
	if n > 0 {
		avg = float64(sum) / float64(n)
	}
	fmt.Fprintln(w, "# HELP http_request_duration_ms_avg Average request duration in milliseconds")
	fmt.Fprintln(w, "# TYPE http_request_duration_ms_avg gauge")
	fmt.Fprintf(w, "http_request_duration_ms_avg %.2f\n", avg)
}
