package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
)

func TestIdempotencyReplaysDuplicateRequest(t *testing.T) {
	rdb := newTestRedis(t)
	var callCount int64

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&callCount, 1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"resource-1"}`))
	})
	handler := Idempotency(rdb)(next)

	makeRequest := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/orgs", nil)
		req.Header.Set("Idempotency-Key", "fixed-key-123")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	rec1 := makeRequest()
	rec2 := makeRequest()

	if rec1.Code != http.StatusCreated || rec2.Code != http.StatusCreated {
		t.Errorf("expected both responses to be 201, got %d and %d", rec1.Code, rec2.Code)
	}
	if rec1.Body.String() != rec2.Body.String() {
		t.Errorf("expected replayed body to match original: %q vs %q", rec1.Body.String(), rec2.Body.String())
	}
	if got := atomic.LoadInt64(&callCount); got != 1 {
		t.Errorf("expected handler to execute exactly once, got %d calls", got)
	}
	if rec2.Header().Get("Idempotent-Replay") != "true" {
		t.Error("expected second response to be marked as a replay")
	}
}

func TestIdempotencyDifferentKeysExecuteIndependently(t *testing.T) {
	rdb := newTestRedis(t)
	var callCount int64

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&callCount, 1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"call":` + strconv.FormatInt(n, 10) + `}`))
	})
	handler := Idempotency(rdb)(next)

	for i, key := range []string{"key-a", "key-b"} {
		req := httptest.NewRequest(http.MethodPost, "/orgs", nil)
		req.Header.Set("Idempotency-Key", key)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Errorf("request %d: expected 201, got %d", i, rec.Code)
		}
	}

	if got := atomic.LoadInt64(&callCount); got != 2 {
		t.Errorf("expected handler to execute twice for two distinct keys, got %d", got)
	}
}

func TestIdempotencySkipsGetRequests(t *testing.T) {
	rdb := newTestRedis(t)
	var callCount int64
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	})
	handler := Idempotency(rdb)(next)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/orgs", nil)
		req.Header.Set("Idempotency-Key", "irrelevant-for-get")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	if got := atomic.LoadInt64(&callCount); got != 3 {
		t.Errorf("expected GET requests to bypass idempotency caching, got %d calls (want 3)", got)
	}
}
