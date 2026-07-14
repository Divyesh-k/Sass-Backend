package auth

import "context"

type contextKey string

const userIDKey contextKey = "user_id"
const userEmailKey contextKey = "user_email"

func ContextWithUser(ctx context.Context, userID, email string) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	ctx = context.WithValue(ctx, userEmailKey, email)
	return ctx
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)
	return v, ok
}

func UserEmailFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userEmailKey).(string)
	return v, ok
}
