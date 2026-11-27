package auth

import "context"

type userIDKey struct{}

// WithUserID возвращает контекст с идентификатором пользователя.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// UserID возвращает идентификатор пользователя из контекста или пустую строку.
func UserID(ctx context.Context) string {
	userID, _ := ctx.Value(userIDKey{}).(string)
	return userID
}
