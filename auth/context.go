package auth

import (
	"context"
	"net/http"
)

type contextKey string
const userContextKey = contextKey("user")
func withUserContext(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) User {
	obj := ctx.Value(userContextKey)
	if obj != nil {
		user, ok := obj.(User)
		if ok {
			return user
		}
	}
	return nil
}

func UserFromRequest(r *http.Request) User {
	return UserFromContext(r.Context())
}

