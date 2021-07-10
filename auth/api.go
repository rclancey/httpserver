package auth

import (
	"io"
	"time"

	H "github.com/rclancey/httpserver"
)

type EmailClient interface {
	Send(from, to, subject, textContent string, htmlContent *string) error
}

type SMSClient interface {
	Send(phoneNumber, content string) error
}

type Template interface {
	Execute(io.Writer, interface{}) error
}

type SocialConfig interface {
	GetClientID() string
	GetClientSecret() string
}

type Authenticator struct {
	Domain string
	UserSource UserSource
	JWT *JWT
	SocialConfig map[string]SocialConfig
	EmailClient EmailClient
	EmailSender string
	SMSClient SMSClient
	ResetTTL time.Duration
	ResetTextTemplate Template
	ResetHTMLTemplate Template
	ResetSMSTemplate Template
	TwoFactorTextTemplate Template
	TwoFactorHTMLTemplate Template
	TwoFactorSMSTemplate Template
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

type LoginParams struct {
	Username    *string `json:"username"`
	Password    *string `json:"password"`
	TwoFactor   *string `json:"two_factor_code"`
	ResetCode   *string `json:"reset_code"`
	NewPassword *string `json:"new_password"`
}

type LoginResponse struct {
	Username string `json:"username"`
	Claims *StandardClaims `json:"claims"`
	Token string `json:"token"`
	Needs2FA bool `json:"needs_two_factor,omitempty"`
}

type TwoFactorData struct {
	Code string
	Username string
}

type Init2FAResponse struct {
	URI string `json:"uri"`
	QRCode string `json:"qr_code"`
	RecoveryKeys []string `json:"recovery_keys"`
}

type PasswordResetData struct {
	Code string
	Username string
	Expires time.Time
}
