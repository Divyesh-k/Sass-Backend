// Package worker implements a deliberately small background job queue on
// top of Redis lists (LPUSH/BRPOP). For a project at this scale that's a
// better trade-off than pulling in a full job-queue library: it's ~100
// lines, has zero extra dependencies beyond the Redis client we already
// use for rate limiting, and the enqueue/handle contract is simple enough
// that swapping it for asynq, river, or SQS later is a contained change
// (only this package and its constructor call in main.go move).
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const queueKey = "jobs:default"
const maxRetries = 3

type Job struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
	Attempt int             `json:"attempt"`
}

type Handler func(ctx context.Context, payload json.RawMessage) error

type Queue struct {
	rdb      *redis.Client
	log      *slog.Logger
	handlers map[string]Handler
}

func NewQueue(rdb *redis.Client, log *slog.Logger) *Queue {
	return &Queue{rdb: rdb, log: log, handlers: make(map[string]Handler)}
}

// Register binds a job type (e.g. "send_welcome_email") to the function
// that processes it. Call this during startup before Run.
func (q *Queue) Register(jobType string, h Handler) {
	q.handlers[jobType] = h
}

// Enqueue pushes a job onto the queue. This is designed to be called
// from request handlers so slow work (sending email, calling a
// third-party API) never blocks the HTTP response.
func (q *Queue) Enqueue(ctx context.Context, jobType string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	job := Job{Type: jobType, Payload: body, Attempt: 0}
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.rdb.LPush(ctx, queueKey, data).Err()
}

// Run blocks, pulling jobs one at a time with BRPOP and dispatching them
// to the registered handler. Call it in its own goroutine from main.go.
// Failed jobs are retried up to maxRetries times with a short backoff
// before being dropped to a dead-letter list for manual inspection —
// the same "don't lose failures silently" principle applied to the DLQ
// in rustmq.
func (q *Queue) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result, err := q.rdb.BRPop(ctx, 5*time.Second, queueKey).Result()
		if errors.Is(err, redis.Nil) {
			continue // timed out waiting, loop and check ctx again
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			q.log.Error("queue: brpop failed", "error", err)
			time.Sleep(time.Second)
			continue
		}

		// result[0] is the key name, result[1] is the payload.
		var job Job
		if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
			q.log.Error("queue: malformed job, dropping", "error", err)
			continue
		}

		q.process(ctx, job)
	}
}

func (q *Queue) process(ctx context.Context, job Job) {
	handler, ok := q.handlers[job.Type]
	if !ok {
		q.log.Error("queue: no handler registered", "job_type", job.Type)
		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := handler(jobCtx, job.Payload); err != nil {
		job.Attempt++
		q.log.Warn("queue: job failed", "job_type", job.Type, "attempt", job.Attempt, "error", err)

		if job.Attempt >= maxRetries {
			q.deadLetter(ctx, job, err)
			return
		}

		// Simple linear backoff before requeueing.
		time.Sleep(time.Duration(job.Attempt) * time.Second)
		data, _ := json.Marshal(job)
		q.rdb.LPush(ctx, queueKey, data)
		return
	}
}

func (q *Queue) deadLetter(ctx context.Context, job Job, cause error) {
	q.log.Error("queue: job moved to dead letter queue", "job_type", job.Type, "error", cause)
	data, _ := json.Marshal(job)
	q.rdb.LPush(ctx, "jobs:dead_letter", data)
}
