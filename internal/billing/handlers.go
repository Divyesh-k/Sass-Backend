package billing

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/divyeshkakadiya/saas-backend/internal/auth"
	"github.com/divyeshkakadiya/saas-backend/internal/response"
)

type Handler struct {
	client        *Client
	repo          *Repository
	webhookSecret string
	log           *slog.Logger
}

func NewHandler(client *Client, repo *Repository, webhookSecret string, log *slog.Logger) *Handler {
	return &Handler{client: client, repo: repo, webhookSecret: webhookSecret, log: log}
}

func (h *Handler) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	email, _ := auth.UserEmailFromContext(r.Context())

	var req struct {
		PriceID    string `json:"price_id"`
		SuccessURL string `json:"success_url"`
		CancelURL  string `json:"cancel_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}

	session, err := h.client.CreateCheckoutSession(req.PriceID, email, orgID, req.SuccessURL, req.CancelURL)
	if err != nil {
		h.log.Error("checkout session creation failed", "error", err)
		response.Error(w, http.StatusBadGateway, "stripe_error", "could not start checkout session")
		return
	}

	response.JSON(w, http.StatusOK, session)
}

func (h *Handler) CreatePortalSession(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")

	customerID, err := h.repo.GetStripeCustomerID(r.Context(), orgID)
	if err != nil || customerID == "" {
		response.Error(w, http.StatusNotFound, "no_subscription", "this organization has no billing account yet")
		return
	}

	var req struct {
		ReturnURL string `json:"return_url"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	session, err := h.client.CreatePortalSession(customerID, req.ReturnURL)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "stripe_error", "could not open billing portal")
		return
	}

	response.JSON(w, http.StatusOK, session)
}

// Webhook receives Stripe events. It MUST be mounted with the raw request
// body available (no JSON-parsing middleware ahead of it) because
// signature verification is computed over the exact bytes Stripe sent.
func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_body", "could not read request body")
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	if err := VerifyWebhookSignature(payload, sigHeader, h.webhookSecret); err != nil {
		h.log.Warn("webhook signature verification failed", "error", err)
		response.Error(w, http.StatusBadRequest, "invalid_signature", "webhook signature verification failed")
		return
	}

	var event struct {
		Type string `json:"type"`
		Data struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json", "malformed event payload")
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		h.handleCheckoutCompleted(r.Context(), event.Data.Object)
	case "customer.subscription.updated", "customer.subscription.deleted":
		h.handleSubscriptionUpdated(r.Context(), event.Data.Object)
	default:
		h.log.Info("unhandled webhook event", "type", event.Type)
	}

	// Stripe only cares about the 2xx; body content is ignored.
	response.JSON(w, http.StatusOK, map[string]bool{"received": true})
}

// handleCheckoutCompleted fires once, right after a customer finishes
// Stripe Checkout. We use client_reference_id (the org ID we passed in
// when creating the session) to link the newly created Stripe customer
// back to the correct tenant — this is the join point between "a person
// paid" and "which org gets upgraded."
func (h *Handler) handleCheckoutCompleted(ctx context.Context, raw json.RawMessage) {
	var session struct {
		Customer          string `json:"customer"`
		ClientReferenceID string `json:"client_reference_id"`
	}
	if err := json.Unmarshal(raw, &session); err != nil {
		h.log.Error("failed to parse checkout.session.completed", "error", err)
		return
	}
	if session.ClientReferenceID == "" || session.Customer == "" {
		h.log.Warn("checkout.session.completed missing org linkage", "raw", string(raw))
		return
	}
	if err := h.repo.SetStripeCustomer(ctx, session.ClientReferenceID, session.Customer); err != nil {
		h.log.Error("failed to persist stripe customer", "error", err, "org_id", session.ClientReferenceID)
	}
}

// handleSubscriptionUpdated keeps our local plan_id/subscription_status
// mirror in sync with Stripe's source of truth. Handling both "updated"
// and "deleted" the same way means a canceled subscription just becomes
// another status value — no special-cased downgrade path.
func (h *Handler) handleSubscriptionUpdated(ctx context.Context, raw json.RawMessage) {
	var sub struct {
		ID       string `json:"id"`
		Customer string `json:"customer"`
		Status   string `json:"status"`
		Items    struct {
			Data []struct {
				Price struct {
					ID string `json:"id"`
				} `json:"price"`
			} `json:"data"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &sub); err != nil {
		h.log.Error("failed to parse subscription webhook", "error", err)
		return
	}

	planID := ""
	if len(sub.Items.Data) > 0 {
		planID = sub.Items.Data[0].Price.ID
	}

	if err := h.repo.UpdateSubscription(ctx, sub.Customer, sub.ID, planID, sub.Status); err != nil {
		h.log.Error("failed to update subscription", "error", err, "customer", sub.Customer)
	}
}
