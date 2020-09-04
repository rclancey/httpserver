package httpserver

import (
	//"bufio"
	"encoding/hex"
	"encoding/json"
	//"io"
	"math/rand"
	"net/http"
	//"os"
	//"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"
	//"github.com/rclancey/file-lock"
	"github.com/rclancey/httpserver/auth"
	//"golang.org/x/crypto/bcrypt"
)

/*
type Authenticator interface {
	Authenticate(context.Context, http.ResponseWriter, *http.Request) error
}

type Authorizer interface {
	Authorize(context.Context, http.ResponseWriter, *http.Request) error
}

type User interface {
	UserID() string
}
*/

type AuthCfg interface {
	GetAuthKey() []byte
	GetTTL() time.Duration
	Authenticate(http.ResponseWriter, *http.Request) (string, error)
	Authorize(req *http.Request, username string) bool
}

type SocialLoginConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type AuthConfig struct {
	PasswordFile string `json:"htpasswd" arg:"--htpasswd"`
	AuthKey      string `json:"key"      arg:"key"`
	TTL          int    `json:"ttl"      arg:"ttl"`
	SocialLogin  map[string]*SocialLoginConfig `json:"social"`
	keyBytes     []byte
	authorizer   func(*http.Request, string) bool
}

func (cfg *AuthConfig) Init(serverRoot string) error {
	fn, err := makeRootAbs(serverRoot, cfg.PasswordFile)
	if err != nil {
		return errors.Wrap(err, "can't get abs path for " + cfg.PasswordFile)
	}
	cfg.PasswordFile = fn
	return nil
}

func (cfg *AuthConfig) GetAuthKey() []byte {
	if cfg.keyBytes == nil || len(cfg.keyBytes) != 16 {
		kb, _ := hex.DecodeString(cfg.AuthKey)
		if kb == nil {
			kb = []byte{}
		}
		if len(kb) < 16 {
			pad := make([]byte, 16 - len(kb))
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			rng.Read(pad)
			kb = append(kb, pad...)
		}
		cfg.keyBytes = kb[:16]
	}
	return cfg.keyBytes
}

func (cfg *AuthConfig) GetTTL() time.Duration {
	return time.Duration(cfg.TTL) * time.Second
}

func (cfg *AuthConfig) ReadCookie(req *http.Request) *auth.User {
	cookie, err := req.Cookie("auth")
	if err != nil {
		return nil
	}
	claims := &jwt.StandardClaims{}
	token, err := jwt.ParseWithClaims(cookie.Value, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return cfg.GetAuthKey(), nil
	})
	if err != nil {
		return nil
	}
	if token.Valid && token.Claims.Valid() == nil {
		claims, isa := token.Claims.(*jwt.StandardClaims)
		if isa {
			user := &auth.User{}
			err := json.Unmarshal([]byte(claims.Id), user)
			if err != nil {
				return nil
			}
			return user
		}
	}
	return nil
}

func (cfg *AuthConfig) SetCookie(w http.ResponseWriter, user *auth.User) {
	data, err := json.Marshal(user)
	if err != nil {
		return
	}
	now := jwt.TimeFunc().Unix()
	exp := now + int64(cfg.GetTTL().Seconds())
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.StandardClaims{
		ExpiresAt: exp,
		Id: string(data),
		IssuedAt: now,
		NotBefore: now - 10,
		Subject: "",
	})
	tokenString, err := token.SignedString(cfg.GetAuthKey())
	if err == nil {
		cookie := &http.Cookie{
			Name: "auth",
			Value: tokenString,
			Path: "/",
			Expires: time.Unix(exp, 0),
		}
		w.Header().Add("Set-Cookie", cookie.String())
	}
}
