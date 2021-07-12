package auth

import (
	"net/http"

	H "github.com/rclancey/httpserver"
)

func (a *Authenticator) MakeMiddleware() H.Middleware {
	mw := func(handler http.Handler) http.Handler {
		fnc := func(w http.ResponseWriter, r *http.Request) {
			j := a.JWT
			claims, err := j.GetClaimsFromRequest(r)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Login Required"))
				return
			}
			if claims == nil {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Login Required"))
				return
			}
			if !claims.TwoFactor {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Login Required"))
				return
			}
			xr := r.WithContext(withUserContext(r.Context(), claims))
			handler.ServeHTTP(w, xr)
		}
		return http.HandlerFunc(fnc)
	}
	return H.Middleware(mw)
}

