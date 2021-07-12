package auth

import (
	"net/http"
	H "github.com/rclancey/httpserver"
	"github.com/rclancey/logging"
)

func (a *Authenticator) MakeLoginHandler() H.HandlerFunc {
	fnc := func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		j := a.JWT
		src := a.UserSource
		params := &LoginParams{}
		err := H.ReadJSON(r, params)
		if err != nil {
			return nil, err
		}
		if params.Username == nil || params.Password == nil {
			return nil, H.Unauthorized
		}
		user, err := src.GetUser(*params.Username)
		if err != nil {
			logging.Errorf(r.Context(), "error getting user: %s", err.Error())
			return nil, H.Unauthorized
		}
		auth, err := user.GetAuth()
		if err != nil {
			logging.Errorf(r.Context(), "error getting user auth info: %s", err.Error())
			return nil, H.Unauthorized
		}
		err = auth.CheckPassword(*params.Password)
		if err != nil {
			logging.Errorf(r.Context(), "error checking user password: %s", err.Error())
			return nil, H.Unauthorized
		}
		claims := j.NewClaims()
		claims.SetUser(user)
		if !auth.Has2FA() {
			claims.TwoFactor = true
		}
		token, err := j.MakeToken(claims)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		err = j.SetCookie(w, claims)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		return &LoginResponse{
			Username: *params.Username,
			Claims: claims,
			Token: token,
			Needs2FA: auth.Has2FA(),
		}, nil
	}
	return H.HandlerFunc(fnc)
}
