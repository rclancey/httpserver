package auth

import (
	"net/http"

	H "github.com/rclancey/httpserver/v2"
)

func (a *Authenticator) MakeLogoutHandler() H.HandlerFunc {
	fnc := func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		j := a.JWT
		err := j.SetCookie(w, nil)
		if err != nil {
			return nil, err
		}
		return map[string]string{"status": "OK"}, nil
	}
	return H.HandlerFunc(fnc)
}

