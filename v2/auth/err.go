package auth

import (
	"errors"
)

var ErrTokenExpired = errors.New("token expired")
var ErrInvalidToken = errors.New("invalid token")
var ErrUnknownUser  = errors.New("unknown user")
