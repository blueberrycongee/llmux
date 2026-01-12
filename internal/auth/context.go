package auth

import "context"

// WithAuthContext stores an AuthContext on the provided context.
func WithAuthContext(ctx context.Context, authCtx *AuthContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, AuthContextKey, authCtx)
}
