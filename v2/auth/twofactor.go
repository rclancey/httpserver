package auth

import (
	"net/http"

	"github.com/rclancey/authenticator"
	H "github.com/rclancey/httpserver/v2"
	"github.com/rclancey/logging"
)

func (a *Authenticator) MakeLogin2FAHandler() H.HandlerFunc {
	fnc := func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		j := a.JWT
		src := a.UserSource
		claims, err := j.GetClaimsFromRequest(r)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		if claims == nil {
			return nil, H.Unauthorized
		}
		params := &LoginParams{}
		err = H.ReadJSON(r, params)
		if err != nil {
			return nil, err
		}
		if params.TwoFactor == nil {
			return nil, H.Unauthorized
		}
		user, err := src.GetUser(claims.GetUsername())
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		tfuser, isa := user.(TwoFactorUser)
		if !isa {
			return nil, H.Unauthorized
		}
		auth, err := tfuser.GetTwoFactorAuth()
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		if auth == nil {
			return nil, H.Unauthorized
		}
		tfa, isa := auth.(TwoFactorAuth)
		if !isa {
			return nil, H.Unauthorized
		}
		err = tfa.Authenticate(*params.TwoFactor)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		if tfa.IsDirty() {
			err = tfuser.SetTwoFactorAuth(tfa)
			if err != nil {
				return nil, H.Unauthorized.Wrap(err, "")
			}
		}
		claims.TwoFactor = true
		claims.Extend()
		token, err := j.MakeToken(claims)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		err = j.SetCookie(w, claims)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		return &LoginResponse{
			Username: claims.GetUsername(),
			Claims: claims,
			Token: token,
			Needs2FA: true,
		}, nil
	}
	return H.HandlerFunc(fnc)
}

func (a *Authenticator) MakeSend2FACodeHandler() H.HandlerFunc {
	fnc := func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		j := a.JWT
		src := a.UserSource
		claims, err := j.GetClaimsFromRequest(r)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		if claims == nil {
			return nil, H.Unauthorized
		}
		user, err := src.GetUser(claims.GetUsername())
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		tfuser, isa := user.(TwoFactorUser)
		if !isa {
			return nil, H.Unauthorized
		}
		auth, err := tfuser.GetTwoFactorAuth()
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		if auth == nil {
			return nil, H.Unauthorized
		}
		tfa, isa := auth.(TwoFactorAuth)
		if !isa {
			return nil, H.Unauthorized
		}
		resp := map[string]string{"status": "OK"}
		code := tfa.GenCode()
		if code == "" {
			return resp, nil
		}
		data := &TwoFactorData{
			Scheme: H.ExternalScheme(r),
			Hostname: H.ExternalHostname(r),
			Code: code,
			Username: user.GetUsername(),
		}
		err = a.sendMessage(user, "Two Factor Authentication Code", data, a.TwoFactorSMSTemplate, a.TwoFactorTextTemplate, a.TwoFactorHTMLTemplate)
		if err != nil {
			logging.Errorln(r.Context(), err)
		}
		return resp, nil
	}
	return H.HandlerFunc(fnc)
}

func (a *Authenticator) MakeInit2FAHandler() H.HandlerFunc {
	fnc := func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		j := a.JWT
		src := a.UserSource
		claims, err := j.GetClaimsFromRequest(r)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		if claims == nil {
			return nil, H.Unauthorized
		}
		if !claims.TwoFactor {
			return nil, H.Unauthorized
		}
		user, err := src.GetUser(claims.GetUsername())
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		tfuser, isa := user.(TwoFactorUser)
		if !isa {
			return nil, H.Unauthorized
		}
		auth, err := tfuser.InitTwoFactorAuth()
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		tfa, isa := auth.(*authenticator.TwoFactorAuthenticator)
		if !isa {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		if tfa.Domain == "" {
			tfa.Domain = H.ExternalHostname(r)
		}
		if tfa.Username == "" {
			tfa.Username = user.GetUsername()
		}
		return tfa.Configure()
	}
	return H.HandlerFunc(fnc)
}

func (a *Authenticator) MakeComplete2FAHandler() H.HandlerFunc {
	fnc := func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		j := a.JWT
		src := a.UserSource
		claims, err := j.GetClaimsFromRequest(r)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		if claims == nil {
			return nil, H.Unauthorized
		}
		if !claims.TwoFactor {
			return nil, H.Unauthorized
		}
		user, err := src.GetUser(claims.GetUsername())
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		tfuser, isa := user.(TwoFactorUser)
		if !isa {
			return nil, H.Unauthorized
		}
		params := &LoginParams{}
		err = H.ReadJSON(r, params)
		if err != nil {
			return nil, err
		}
		if params.TwoFactor == nil {
			return nil, H.BadRequest
		}
		err = tfuser.CompleteTwoFactorAuth(*params.TwoFactor)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		return map[string]string{"status": "OK"}, nil
	}
	return H.HandlerFunc(fnc)
}
