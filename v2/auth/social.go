package auth

import (
	"errors"
	"log"
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
		log.Println("social start")
		j := a.JWT
		src := a.UserSource
		driver := H.ContextRequestVars(r.Context())["driver"]
		log.Println("social 1")
		if driver == "" {
			log.Println("driver route not found")
			return nil, H.NotFound
		}
		log.Println("social 2")
		q := r.URL.Query()
		code := q.Get("code")
		state := q.Get("state")
		suser, _, err := dispatcher.Handle(state, code)
		log.Println("social 3")
		if err != nil {
			log.Println("dispatcher error:", code, state, suser, err)
			return nil, H.Unauthorized.Wrap(err, "")
		}
		log.Println("social 4")
		var user AuthUser
		socialSrc, ok := src.(SocialUserSource)
		log.Println("social 5")
		if ok {
			log.Println("social 6")
			user, err = socialSrc.GetSocialUser(driver, suser.ID, suser.Username)
			log.Println("social 7")
			if err != nil && !errors.Is(err, ErrUnknownUser) {
				log.Printf("error getting social user for %s/%s: %s", suser.ID, suser.Username, err)
				return nil, H.Unauthorized.Wrap(err, "")
			}
		}
		log.Println("social 8")
		if user == nil && suser.Email != "" {
			log.Println("social 9")
			user, err = src.GetUserByEmail(suser.Email)
			if err != nil {
				log.Printf("error getting social user by email %s: %s", suser.Email, err)
				return nil, H.Unauthorized.Wrap(err, "")
			}
			log.Println("social 10")
			if user != nil {
				log.Println("social 11")
				var id string
				if suser.ID != "" {
					id = suser.ID
				} else if suser.Username != "" {
					id = suser.Username
				}
				if id != "" {
					xuser, ok := user.(SocialUser)
					if ok {
						log.Println("social 12")
						err = xuser.SetSocialID(driver, id)
						if err != nil {
							logging.Errorf(r.Context(), "error updating user's %s id: %s", driver, err.Error())
						}
					}
				}
			}
		}
		log.Println("social 13")
		if user == nil {
			log.Println("no user for social user", suser)
			return nil, H.Unauthorized
		}
		log.Println("social 14")
		auth, err := user.GetAuth()
		if err != nil {
			log.Println("error getting auth:", err)
			return nil, H.Unauthorized.Wrap(err, "")
		}
		log.Println("social 15")
		claims := j.NewClaims()
		claims.SetUser(user)
		claims.Extra = suser.Raw
		j.SetCookie(w, claims)
		if !auth.Has2FA() {
			claims.TwoFactor = true
		}
		err = j.SetCookie(w, claims)
		log.Println("social 16")
		if err != nil {
			log.Println("error setting cookie:", err)
			return nil, H.Unauthorized.Wrap(err, "")
		}
		log.Println("social success")
		return H.Redirect("/"), nil
	}
	router.GET("/:driver", H.HandlerFunc(redirect))
	router.GET("/:driver/callback", H.HandlerFunc(callback))
}

