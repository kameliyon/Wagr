package auth

import "context"

type contextKey string

const claimsKey contextKey = "auth_claims"

// SetClaimsInContext adds claims to the context
func SetClaimsInContext(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// GetClaimsFromContext retrieves claims from the context
func GetClaimsFromContext(ctx context.Context) *Claims {
	claims, _ := ctx.Value(claimsKey).(*Claims)
	return claims
}
