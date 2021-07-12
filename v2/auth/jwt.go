package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"
)

type JWT struct {
	key []byte
	keyFunc jwt.Keyfunc
	ttl time.Duration
	issuer string
	cookieName string
	headerName string
}

func NewJWT(key []byte, ttl time.Duration, issuer, cookieName, headerName string) *JWT {
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return key, nil
	}
	return &JWT{
		key: key,
		keyFunc: keyFunc,
		ttl: ttl,
		issuer: issuer,
		cookieName: cookieName,
		headerName: headerName,
	}
}

func (j *JWT) NewClaims() *StandardClaims {
	return NewStandardClaims(j.issuer, j.ttl)
}

func (j *JWT) GetClaims(token string) (*StandardClaims, error) {
	claims := &StandardClaims{}
	t, err := jwt.ParseWithClaims(token, claims, j.keyFunc)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if t.Valid && t.Claims.Valid() == nil {
		claims, isa := t.Claims.(*StandardClaims)
		if isa {
			return claims, nil
		}
	}
	return nil, errors.WithStack(ErrInvalidToken)
}

func (j *JWT) GetClaimsFromCookie(r *http.Request, name string) (*StandardClaims, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "can't get cookie %s", name)
	}
	return j.GetClaims(cookie.Value)
}

func (j *JWT) GetClaimsFromHeader(r *http.Request, name string) (*StandardClaims, error) {
	header := r.Header.Get(name)
	if header == "" {
		return nil, nil
	}
	header = strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	return j.GetClaims(header)
}

func (j *JWT) GetClaimsFromRequest(r *http.Request) (*StandardClaims, error) {
	if j.headerName != "" {
		claims, err := j.GetClaimsFromHeader(r, j.headerName)
		if err != nil {
			return nil, err
		}
		if claims != nil {
			return claims, nil
		}
	}
	if j.cookieName != "" {
		claims, err := j.GetClaimsFromCookie(r, j.cookieName)
		if err != nil {
			return nil, err
		}
		if claims != nil {
			return claims, nil
		}
	}
	return nil, nil
}

func (j *JWT) MakeToken(claims *StandardClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.key)
}

func (j *JWT) SetCookie(w http.ResponseWriter, claims *StandardClaims) error {
	cookie := &http.Cookie{
		Name: j.cookieName,
		Path: "/",
	}
	if claims == nil {
		cookie.Value = ""
		cookie.Expires = time.Now().Add(-1 * time.Second)
	} else {
		token, err := j.MakeToken(claims)
		if err != nil {
			return err
		}
		cookie.Value = token
		cookie.Expires = time.Unix(claims.StandardClaims.ExpiresAt, 0)
	}
	w.Header().Add("Set-Cookie", cookie.String())
	return nil
}
