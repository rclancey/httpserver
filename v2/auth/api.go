package auth

import (
	"encoding/hex"
	"math/rand"
	"time"

	H "github.com/rclancey/httpserver/v2"
)

func NewAuthenticator(cfg AuthConfig, src UserSource) (*Authenticator, error) {
	keyBytes, err := hex.DecodeString(cfg.AuthKey)
	if err != nil {
		return nil, err
	}
	if len(keyBytes) < 16 {
		pad := make([]byte, 16 - len(keyBytes))
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		rng.Read(pad)
		keyBytes = append(keyBytes, pad...)
	}
	j := NewJWT(keyBytes[:16], time.Duration(cfg.TTL) * time.Second, cfg.Issuer, cfg.Cookie, cfg.Header)
	resetText, resetHtml, resetSms, err := cfg.ResetTemplate.GetTemplates()
	if err != nil {
		return nil, err
	}
	twoFactorText, twoFactorHtml, twoFactorSms, err := cfg.TwoFactorTemplate.GetTemplates()
	if err != nil {
		return nil, err
	}
	return &Authenticator{
		UserSource:            src,
		JWT:                   j,
		SocialConfig:          cfg.SocialLogin,
		EmailSender:           cfg.EmailSender,
		ResetTTL:              time.Duration(cfg.ResetTTL) * time.Second,
		ResetTextTemplate:     resetText,
		ResetHTMLTemplate:     resetHtml,
		ResetSMSTemplate:      resetSms,
		TwoFactorTextTemplate: twoFactorText,
		TwoFactorHTMLTemplate: twoFactorHtml,
		TwoFactorSMSTemplate:  twoFactorSms,
	}, nil
}

func (a *Authenticator) LoginAPI(router H.Router) {
	router.POST("/login", a.MakeLoginHandler())
	router.POST("/login/twofactor", a.MakeLogin2FAHandler())
	router.POST("/password", a.MakeChangePasswordHandler())
	router.POST("/password/reset", a.MakeResetPasswordHandler())
	router.POST("/twofactor", a.MakeInit2FAHandler())
	router.PUT("/twofactor", a.MakeComplete2FAHandler())
	router.POST("/logout", a.MakeLogoutHandler())
	router.DELETE("/login", a.MakeLogoutHandler())
	a.MakeSocialLoginHandlers(router.Prefix("/login/social"))
}
