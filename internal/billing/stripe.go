// Package billing integrates Stripe subscription billing. Rather than
// pulling in the full stripe-go SDK (a large dependency surface for what
// we need), this talks to Stripe's REST API directly over net/http —
// Stripe's API is stable, well-documented form-encoded REST, and three
// endpoints (checkout session, billing portal, webhook verify) covers
// the entire subscription lifecycle a typical SaaS needs.
package billing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const stripeAPIBase = "https://api.stripe.com/v1"

type Client struct {
	secretKey string
	http      *http.Client
}

func NewClient(secretKey string) *Client {
	return &Client{
		secretKey: secretKey,
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

type CheckoutSession struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// CreateCheckoutSession starts a Stripe-hosted checkout flow for a
// subscription. successURL/cancelURL are where Stripe redirects the
// browser after payment; clientReferenceID ties the session back to our
// internal org ID so the webhook handler knows which tenant to upgrade.
func (c *Client) CreateCheckoutSession(priceID, customerEmail, clientReferenceID, successURL, cancelURL string) (*CheckoutSession, error) {
	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("line_items[0][price]", priceID)
	form.Set("line_items[0][quantity]", "1")
	form.Set("customer_email", customerEmail)
	form.Set("client_reference_id", clientReferenceID)
	form.Set("success_url", successURL)
	form.Set("cancel_url", cancelURL)

	var session CheckoutSession
	if err := c.post("/checkout/sessions", form, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

type PortalSession struct {
	URL string `json:"url"`
}

// CreatePortalSession returns a link to Stripe's self-serve billing
// portal so customers can update payment methods or cancel without your
// team building that UI from scratch.
func (c *Client) CreatePortalSession(customerID, returnURL string) (*PortalSession, error) {
	form := url.Values{}
	form.Set("customer", customerID)
	form.Set("return_url", returnURL)

	var session PortalSession
	if err := c.post("/billing_portal/sessions", form, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (c *Client) post(path string, form url.Values, out any) error {
	req, err := http.NewRequest(http.MethodPost, stripeAPIBase+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.secretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("billing: stripe request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var stripeErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&stripeErr)
		return fmt.Errorf("billing: stripe error (%d): %s", resp.StatusCode, stripeErr.Error.Message)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
