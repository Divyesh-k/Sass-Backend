package worker

import (
	"context"
	"encoding/json"
	"log/slog"
)

type WelcomeEmailPayload struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// RegisterEmailHandlers wires up the email-sending job types. In this
// starter kit the "send" is a logged no-op — swap sendWelcomeEmail's
// body for a real provider call (SES, Postmark, Resend) when you plug
// this into a real client project. The point being demonstrated is the
// pattern: HTTP handler enqueues, worker sends, and a slow/flaky email
// provider can never make a signup request hang.
func RegisterEmailHandlers(q *Queue, log *slog.Logger) {
	q.Register("send_welcome_email", func(ctx context.Context, payload json.RawMessage) error {
		var p WelcomeEmailPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		log.Info("sending welcome email", "to", p.Email, "name", p.Name)
		// TODO: call your transactional email provider here.
		return nil
	})
}
