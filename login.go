package httpserver

import (
	"log"
	"net/http"
	"net/url"
	"path"

	"github.com/danilopolani/gocialite"
	"github.com/danilopolani/gocialite/structs"
	"github.com/rclancey/httpserver/auth"
	"github.com/rclancey/logging"
)

type AccountMatcher interface {
	GetUserByEmail(string) (*auth.User, error)
}

func userFromGocialite(guser *structs.User, provider string) *auth.User {
	return &auth.User{
		ID: guser.ID,
		Username: guser.Username,
		FirstName: guser.FirstName,
		LastName: guser.LastName,
		FullName: guser.FullName,
		Email: guser.Email,
		Avatar: guser.Avatar,
		Provider: provider,
	}
}

var gocial = gocialite.NewDispatcher()

func (srv *Server) SocialLogin(prefix string, matcher AccountMatcher) {
	redirectHandler := func(w http.ResponseWriter, r *http.Request) {
		driver := ContextRequestVars(r.Context())["driver"]
		if driver == "" {
			sendError(w, r, NotFound)
			return
		}
		cfg, ok := srv.cfg.Auth.SocialLogin[driver]
		if !ok {
			sendError(w, r, NotFound)
			return
		}
		u := ExternalURL(r)
		u.Path = path.Join(u.Path, "callback")
		log.Println(driver, "callback =", u.String())
		authUrl, err := gocial.New().
			Driver(driver).
			Redirect(
				cfg.ClientID,
				cfg.ClientSecret,
				u.String(),
			)
		if err != nil {
			sendError(w, r, NotFound.Wrap(err, ""))
			return
		}
		w.Header().Set("Location", authUrl)
		w.WriteHeader(http.StatusFound)
	}
	callbackHandler := func(w http.ResponseWriter, r *http.Request) {
		driver := ContextRequestVars(r.Context())["driver"]
		if driver == "" {
			sendError(w, r, NotFound)
			return
		}
		q := r.URL.Query()
		code := q.Get("code")
		state := q.Get("state")
		user, _, err := gocial.Handle(state, code)
		if err != nil {
			sendError(w, r, Unauthorized.Wrap(err, ""))
			return
		}
		u := userFromGocialite(user, driver)
		l := logging.FromContext(r.Context())
		l.Warnf("user = %#v :: %#v", u, user)
		u, err = matcher.GetUserByEmail(u.Email)
		if err != nil {
			sendError(w, r, err)
			return
		}
		if u == nil {
			sendError(w, r, Unauthorized)
			return
		}
		srv.cfg.Auth.SetCookie(w, u)
		loc := r.URL.Query().Get("path")
		if loc == "" {
			loc = "/"
		} else {
			u, err := url.Parse(loc)
			if err != nil {
				loc = "/"
			} else {
				u.Scheme = ""
				u.Host = ""
				u.Opaque = ""
				u.User = nil
				loc = u.String()
			}
		}
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusFound)
	}
	srv.Prefix(prefix).GET("/:driver", http.HandlerFunc(redirectHandler))
	srv.Prefix(prefix).GET("/:driver/callback", http.HandlerFunc(callbackHandler))
}
