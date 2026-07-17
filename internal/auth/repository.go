package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
)

var ErrUserExists = errors.New("auth: user already exists")
var ErrUserNotFound = errors.New("auth: user not found")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash, name string) (*User, error) {
	u := &User{}
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, name)
		VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, name, verified, created_at
	`, email, passwordHash, name).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Verified, &u.CreatedAt)

	if isUniqueViolation(err) {
		return nil, ErrUserExists
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, name, verified, created_at
		FROM users WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Verified, &u.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*User, error) {
	u := &User{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, name, verified, created_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Verified, &u.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) StoreRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	return err
}

// GetValidRefreshToken looks up a refresh token by its hash and returns it
// only if it hasn't been revoked or expired. Rotation logic in the
// service layer revokes it immediately after a successful use.
func (r *Repository) GetValidRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	rt := &RefreshToken{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, revoked, created_at
		FROM refresh_tokens
		WHERE token_hash = $1 AND revoked = false AND expires_at > now()
	`, tokenHash).Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.Revoked, &rt.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, err
	}
	return rt, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked = true WHERE id = $1`, id)
	return err
}

// RevokeAllUserTokens is used on password change / suspected compromise
// to kill every active session for a user in one statement.
func (r *Repository) RevokeAllUserTokens(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked = true WHERE user_id = $1 AND revoked = false`, userID)
	return err
}

// isUniqueViolation checks for Postgres error code 23505 (unique_violation)
// via lib/pq's typed error, rather than fragile string matching on the
// driver's error message.
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	return false
}
