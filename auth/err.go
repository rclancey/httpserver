package auth

import (
	"errors"
)

var ErrTokenExpired = errors.New("token expired")
var ErrInvalidToken = errors.New("invalid token")
