package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidCredentials = errors.New("auth: invalid email or password")
var ErrWeakPassword = errors.New("auth: password does not meet minimum requirements")

type Service struct {
	repo       *Repository
	tokens     *TokenManager
	refreshTTL time.Duration
}

func NewService(repo *Repository, tokens *TokenManager, refreshTTL time.Duration) *Service {
	return &Service{repo: repo, tokens: tokens, refreshTTL: refreshTTL}
}

func (s *Service) Signup(ctx context.Context, req SignupRequest) (*User, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !strings.Contains(email, "@") {
		return nil, fmt.Errorf("auth: invalid email address")
	}
	if len(req.Password) < 8 {
		return nil, ErrWeakPassword
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	return s.repo.CreateUser(ctx, email, hash, strings.TrimSpace(req.Name))
}

// Login verifies credentials and issues a fresh access/refresh pair. The
// error returned for "no such user" and "wrong password" is identical
// (ErrInvalidCredentials) so the API never leaks which part was wrong —
// that distinction is what makes email enumeration attacks possible.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*TokenPair, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !VerifyPassword(user.PasswordHash, req.Password) {
		return nil, ErrInvalidCredentials
	}

	return s.issueTokenPair(ctx, user)
}

// Refresh rotates a refresh token: the presented token is revoked and a
// brand new pair is issued. Rotation means a stolen-and-replayed refresh
// token is only usable once before the legitimate client's next refresh
// invalidates it, surfacing the theft.
func (s *Service) Refresh(ctx context.Context, rawToken string) (*TokenPair, error) {
	hash := hashToken(rawToken)

	stored, err := s.repo.GetValidRefreshToken(ctx, hash)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if err := s.repo.RevokeRefreshToken(ctx, stored.ID); err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, stored.UserID)
	if err != nil {
		return nil, err
	}

	return s.issueTokenPair(ctx, user)
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	hash := hashToken(rawToken)
	stored, err := s.repo.GetValidRefreshToken(ctx, hash)
	if err != nil {
		// Already invalid/expired — logout is idempotent from the
		// client's perspective.
		return nil
	}
	return s.repo.RevokeRefreshToken(ctx, stored.ID)
}

func (s *Service) issueTokenPair(ctx context.Context, user *User) (*TokenPair, error) {
	access, expiresIn, err := s.tokens.IssueAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	rawRefresh, err := generateOpaqueToken()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(s.refreshTTL)
	if err := s.repo.StoreRefreshToken(ctx, user.ID, hashToken(rawRefresh), expiresAt); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: rawRefresh,
		ExpiresIn:    expiresIn,
	}, nil
}
