package auth

import (
	"encoding/base64"
	"net/http"

	H "github.com/rclancey/httpserver"
	"github.com/rclancey/logging"
	"github.com/skip2/go-qrcode"
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
		auth, err := user.GetAuth()
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		err = auth.Check2FA(*params.TwoFactor)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		if auth.IsDirty() {
			err = user.SetAuth(auth)
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
			Needs2FA: auth.Has2FA(),
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
		auth, err := user.GetAuth()
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		resp := map[string]string{"status": "OK"}
		code := auth.Get2FACode()
		if code == "" {
			return resp, nil
		}
		data := &TwoFactorData{
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
		auth, err := user.GetAuth()
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		uri, recoveryKeys, err := auth.Configure2FA(user.GetUsername(), a.Domain)
		if err != nil {
			return nil, err
		}
		pngdata, err := qrcode.Encode(uri, qrcode.Medium, 256)
		if err != nil {
			return nil, err
		}
		b64data := base64.StdEncoding.EncodeToString(pngdata)
		if auth.IsDirty() {
			err = user.SetAuth(auth)
			if err != nil {
				return nil, err
			}
		}
		return &Init2FAResponse{
			URI: uri,
			RecoveryKeys: recoveryKeys,
			QRCode: "data:image/png;base64," + b64data,
		}, nil
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
		auth, err := user.GetAuth()
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		params := &LoginParams{}
		err = H.ReadJSON(r, params)
		if err != nil {
			return nil, err
		}
		if params.TwoFactor == nil {
			return nil, H.BadRequest
		}
		err = auth.Complete2FA(*params.TwoFactor)
		if err != nil {
			return nil, H.Unauthorized.Wrap(err, "")
		}
		if auth.IsDirty() {
			err = user.SetAuth(auth)
			if err != nil {
				return nil, err
			}
		}
		return map[string]string{"status": "OK"}, nil
	}
	return H.HandlerFunc(fnc)
}
