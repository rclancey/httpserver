package auth

import (
	"net/http"
	"time"

	"github.com/pkg/errors"
	H "github.com/rclancey/httpserver/v2"
	"github.com/rclancey/logging"
)

func (a *Authenticator) MakeChangePasswordHandler() H.HandlerFunc {
	fnc := func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		j := a.JWT
		src := a.UserSource
		params := &LoginParams{}
		err := H.ReadJSON(r, params)
		if err != nil {
			return nil, err
		}
		if params.NewPassword == nil {
			return nil, H.BadRequest
		}
		claims, err := j.GetClaimsFromRequest(r)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		var username string
		if claims == nil {
			if params.Username == nil {
				return nil, H.Unauthorized
			}
			username = *params.Username
		} else {
			if !claims.TwoFactor {
				return nil, H.Unauthorized
			}
			username = claims.GetUsername()
		}
		user, err := src.GetUser(username)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		auth, err := user.GetAuth()
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		pwauth, isa := auth.(PasswordAuth)
		if !isa {
			return nil, H.Unauthorized
		}
		if params.ResetCode == nil {
			if claims == nil || !claims.TwoFactor || params.Password == nil {
				return nil, H.Unauthorized
			}
			err = auth.Authenticate(*params.Password)
			if err != nil {
				return nil, H.Unauthorized.Wrap(err, "")
			}
		} else {
			err = auth.Authenticate(*params.ResetCode)
			if err != nil {
				return nil, H.Unauthorized.Wrap(err, "")
			}
		}
		err = pwauth.SetPassword(*params.NewPassword)
		if err != nil {
			return nil, H.BadRequest.Wrap(err, "")
		}
		if pwauth.IsDirty() {
			err = user.SetAuth(auth)
			if err != nil {
				return nil, errors.WithStack(err)
			}
		}
		j.SetCookie(w, nil)
		return map[string]string{"status": "OK"}, nil
	}
	return H.HandlerFunc(fnc)
}

func (a *Authenticator) MakeResetPasswordHandler() H.HandlerFunc {
	fnc := func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		src := a.UserSource
		params := &LoginParams{}
		err := H.ReadJSON(r, params)
		if err != nil {
			return nil, err
		}
		if params.Username == nil {
			return nil, H.BadRequest
		}
		resp := map[string]string{"status": "OK"}
		user, err := src.GetUser(*params.Username)
		if err != nil {
			logging.Errorln(r.Context(), "error getting user:", err)
			return resp, nil
		}
		auth, err := user.GetAuth()
		if err != nil {
			logging.Errorln(r.Context(), "error getting user auth:", err)
			return resp, nil
		}
		pwauth, isa := auth.(PasswordAuth)
		if !isa {
			return resp, nil
		}
		code, err := pwauth.ResetPassword(a.ResetTTL)
		if err != nil {
			logging.Errorln(r.Context(), "error generating reset code:", err)
			return resp, nil
		}
		if pwauth.IsDirty() {
			err = user.SetAuth(auth)
			if err != nil {
				return nil, errors.WithStack(err)
			}
		}
		data := &PasswordResetData{
			Scheme: H.ExternalScheme(r),
			Hostname: H.ExternalHostname(r),
			Code: code,
			Username: user.GetUsername(),
			Expires: time.Now().Add(a.ResetTTL),
		}
		err = a.sendMessage(user, "Reset your password", data, a.ResetSMSTemplate, a.ResetTextTemplate, a.ResetHTMLTemplate)
		if err != nil {
			logging.Errorln(r.Context(), err)
		}
		return resp, nil
	}
	return H.HandlerFunc(fnc)
}
