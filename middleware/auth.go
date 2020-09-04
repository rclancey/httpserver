package middleware

import (
	"encoding/hex"
	"math/rand"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/rclancey/httpserver"
)

type Authenticator interface {
	Authenticate(username, password string) (bool, error)
}

type Authorizer interface {
	Authorize(req *http.Request) (bool, error)
}

type authKey string

type Auth struct {
	cfg *AuthConfig
	authen Authenticator
	authz Authorizer
}

func NewAuth(cfg *AuthConfig, authen Authenticator, authz Authorizer) *Auth {
	return &Auth{
		cfg: cfg,
		authen: authen,
		authz: authz,
	}
}

func (a *Auth) Authenticate(username, password string) (bool, error) {
	if a.authen == nil {
		return true, nil
	}
	return a.authen.Authenticate(username, password)
}

func (a *Auth) Authorize(r *http.Request) (bool, error) {
	if a.authz == nil {
		return true, nil
	}
	return a.authz.Authorize(r)
}

func (a *Auth) Middleware(handler http.Handler) http.Handler {
	return &AuthMiddleware{a, handler}
}

type AuthMiddleware struct {
	auth *Auth
	handler http.Handler
}

func (mw *AuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r, err := mw.Authenticate(w, r)
	if err != nil {
		sendError(w, r, err)
		return
	}
	err = mw.Authorize(w, r)
	if err != nil {
		sendError(w, r, err)
	}
	a.handler.ServeHTTP(w, r)
}

func GetRequestUser(r *http.Request) *auth.User {
	u, isa := r.Context().Value(authKey("user")).(*auth.User)
	if isa {
		return u
	}
	return nil
}

func (mw *AuthMiddleware) Authenticate(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	username, password, ok := r.BasicAuth()
	if ok {
		user, _ := mw.auth.Authenticate(username, password)
		if user == nil {
			return r, httpserver.Unauthorized
		}
		mw.auth.cfg.SetCookie(w, user)
		ctx := context.WithValue(r.Context(), authKey("user"), user)
		return r.Clone(ctx), nil
	}
	user, ok = mw.auth.cfg.CheckCookie(r)
	if !ok {
		return r, httpserver.Unauthorized
	}
	mw.auth.cfg.SetCookie(w, user)
	ctx := context.WithValue(r.Context(), authKey("user"), user)
	return r.Clone(ctx), nil
}

func (mw *AuthMiddleware) Authorize(w http.ResponseWriter, r *http.Request) error {
	ok, err := mw.auth.Authorize(r)
	if err != nil {
		return err
	}
	if !ok {
		return httpserver.Forbidden
	}
	return nil
}
