package auth

import (
	"net/http"
	"path"

	"github.com/danilopolani/gocialite"
	H "github.com/rclancey/httpserver/v2"
	"github.com/rclancey/logging"
)

func (a *Authenticator) MakeSocialLoginHandlers(router H.Router) {
	config := a.SocialConfig
	if config == nil {
		return
	}
	dispatcher := gocialite.NewDispatcher()
	redirect := func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		driver := H.ContextRequestVars(r.Context())["driver"]
		if driver == "" {
			return nil, H.NotFound
		}
		cfg, ok := config[driver]
		if !ok {
			return nil, H.NotFound
		}
		u := H.ExternalURL(r)
		u.Path = path.Join(u.Path, "callback")
		gdriver := dispatcher.New().Driver(driver)
		if gdriver == nil {
			return nil, H.NotFound
		}
		authUrl, err := gdriver.Redirect(
			cfg.ClientID,
			cfg.ClientSecret,
			u.String(),
		)
		if err != nil {
			return nil, H.NotFound.Wrap(err, "")
		}
		return H.Redirect(authUrl), nil
	}
	callback := func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		j := a.JWT
		src := a.UserSource
		driver := H.ContextRequestVars(r.Context())["driver"]
		if driver == "" {
			return nil, H.NotFound
		}
		q := r.URL.Query()
		code := q.Get("code")
		state := q.Get("state")
		suser, _, err := dispatcher.Handle(state, code)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		var user AuthUser
		socialSrc, ok := src.(SocialUserSource)
		if ok {
			user, err = socialSrc.GetSocialUser(driver, suser.ID, suser.Username)
			if err != nil {
				return nil, H.Unauthorized.Wrap(err, "")
			}
		}
		if user == nil && suser.Email != "" {
			user, err = src.GetUserByEmail(suser.Email)
			if err != nil {
				return nil, H.Unauthorized.Wrap(err, "")
			}
			if user != nil {
				var id string
				if suser.ID != "" {
					id = suser.ID
				} else if suser.Username != "" {
					id = suser.Username
				}
				if id != "" {
					xuser, ok := user.(SocialUser)
					if ok {
						err = xuser.SetSocialID(driver, id)
						if err != nil {
							logging.Errorf(r.Context(), "error updating user's %s id: %s", driver, err.Error())
						}
					}
				}
			}
		}
		if user == nil {
			return nil, H.Unauthorized
		}
		auth, err := user.GetAuth()
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		claims := j.NewClaims()
		claims.SetUser(user)
		claims.Extra = suser.Raw
		j.SetCookie(w, claims)
		if !auth.Has2FA() {
			claims.TwoFactor = true
		}
		err = j.SetCookie(w, claims)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		return H.Redirect("/"), nil
	}
	router.GET("/:driver", H.HandlerFunc(redirect))
	router.GET("/:driver/callback", H.HandlerFunc(callback))
}

