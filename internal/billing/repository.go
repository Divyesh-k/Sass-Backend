package billing

import (
	"context"
	"database/sql"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) SetStripeCustomer(ctx context.Context, orgID, customerID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE organizations SET stripe_customer_id = $2 WHERE id = $1
	`, orgID, customerID)
	return err
}

func (r *Repository) UpdateSubscription(ctx context.Context, customerID, subscriptionID, planID, status string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE organizations
		SET stripe_subscription_id = $2, plan_id = $3, subscription_status = $4
		WHERE stripe_customer_id = $1
	`, customerID, subscriptionID, planID, status)
	return err
}

func (r *Repository) GetStripeCustomerID(ctx context.Context, orgID string) (string, error) {
	var customerID sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT stripe_customer_id FROM organizations WHERE id = $1
	`, orgID).Scan(&customerID)
	if err != nil {
		return "", err
	}
	return customerID.String, nil
}
