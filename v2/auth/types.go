package auth

import (
	"io"
	"time"

	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt/v4"
	"github.com/rclancey/authenticator"
)

type User interface {
	GetUsername() string
}

type IntIDUser interface {
	User
	GetUserID() int64
}

type UUIDUser interface {
	User
	GetUUID() uuid.UUID
}

type FullNameUser interface {
	User
	GetFullName() string
}

type FirstLastNameUser interface {
	User
	GetFirstName() string
	GetLastName() string
}

type EmailUser interface {
	User
	GetEmailAddress() string
}

type PhoneUser interface {
	User
	GetPhoneNumber() string
}

type TimeZoneUser interface {
	User
	GetTimeZone() string
}

type LocaleUser interface {
	User
	GetLocale() string
}

type AvatarUser interface {
	User
	GetAvatar() string
}

type AuthUser interface {
	User
	GetAuth() (authenticator.Authenticator, error)
	SetAuth(authenticator.Authenticator) error
}

type PasswordAuth interface {
	authenticator.Authenticator
	SetPassword(password string, inputs ...string) error
	ResetPassword(dur time.Duration) (string, error)
	CheckResetCode(code string) error
	IsDirty() bool
}

type TwoFactorUser interface {
	AuthUser
	GetTwoFactorAuth() (authenticator.Authenticator, error)
	SetTwoFactorAuth(authenticator.Authenticator) error
	InitTwoFactorAuth() (authenticator.Authenticator, error)
	CompleteTwoFactorAuth(code string) error
}

type TwoFactorAuth interface {
	authenticator.Authenticator
	GenCode() string
	Configure() (*authenticator.TwoFactorConfig, error)
	IsDirty() bool
}

type SocialUser interface {
	AuthUser
	SetSocialID(driver, id string) error
}

type Claims interface {
	jwt.Claims
	User
	Extend()
	SetUser(user User)
	SetProvider(string)
	GetProvider() string
	SetTwoFactor(bool)
	GetTwoFactor() bool
}

type UserSource interface {
	GetUser(username string) (AuthUser, error)
	GetUserByEmail(email string) (AuthUser, error)
}

type SocialUserSource interface {
	UserSource
	GetSocialUser(driver, id, username string) (AuthUser, error)
}

type EmailClient interface {
	Send(from, to, subject, textContent string, htmlContent *string) error
}

type SMSClient interface {
	Send(phoneNumber, content string) error
}

type Template interface {
	Execute(io.Writer, interface{}) error
}

type StandardClaims struct {
	jwt.StandardClaims
	AuthTime  int64     `json:"auth_time,omitempty"`
	Provider  string    `json:"x-provider,omitempty"`
	TTL       int64     `json:"x-ttl,omitempty"`
	TwoFactor bool      `json:"x-2fa,omitempty"`
	UserID    int64     `json:"x-userid,omitempty"`
	UserUUID  uuid.UUID `json:"x-useruuid,omitempty"`
	FirstName string    `json:"given_name,omitempty"`
	LastName  string    `json:"family_name,omitempty"`
	FullName  string    `json:"name,omitempty"`
	Username  string    `json:"preferred_username,omitempty"`
	Email     string    `json:"email,omitempty"`
	Phone     string    `json:"phone_number,omitempty"`
	TimeZone  string    `json:"zoneinfo,omitempty"`
	Locale    string    `json:"locale,omitempty"`
	Avatar    string    `json:"picture,omitempty"`
	Extra     map[string]interface{} `json:"x-extra,omitempty"`
}

type SocialLoginConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type TemplateConfig struct {
	Text string `json:"text" arg:"text"`
	HTML string `json:"html" arg:"html"`
	SMS  string `json:"sms"  arg:"sms"`
}

type AuthConfig struct {
	AuthKey           string         `json:"key"                 arg:"key"`
	TTL               int            `json:"ttl"                 arg:"ttl"`
	Issuer            string         `json:"issuer"              arg:"issuer"`
	Cookie            string         `json:"cookie"              arg:"cookie"`
	Header            string         `json:"header"              arg:"header"`
	EmailSender       string         `json:"email_sender"        arg:"email-sender"`
	ResetTTL          int            `json:"reset_ttl"           arg:"reset-ttl"`
	ResetTemplate     TemplateConfig `json:"reset_template"      arg:"reset-template"`
	TwoFactorTemplate TemplateConfig `json:"two_factor_template" arg:"two-factor-template"`
	SocialLogin       map[string]*SocialLoginConfig `json:"social"`
}

type Authenticator struct {
	UserSource            UserSource
	EmailClient           EmailClient
	SMSClient             SMSClient
	Domain                string
	JWT                   *JWT
	SocialConfig          map[string]*SocialLoginConfig
	EmailSender           string
	ResetTTL              time.Duration
	ResetTextTemplate     Template
	ResetHTMLTemplate     Template
	ResetSMSTemplate      Template
	TwoFactorTextTemplate Template
	TwoFactorHTMLTemplate Template
	TwoFactorSMSTemplate  Template
}

type LoginParams struct {
	Username    *string `json:"username"`
	Password    *string `json:"password"`
	TwoFactor   *string `json:"two_factor_code"`
	ResetCode   *string `json:"reset_code"`
	NewPassword *string `json:"new_password"`
}

type LoginResponse struct {
	Username string          `json:"username"`
	Claims   *StandardClaims `json:"claims"`
	Token    string          `json:"token"`
	Needs2FA bool            `json:"needs_two_factor,omitempty"`
}

type Init2FAResponse struct {
	URI          string   `json:"uri"`
	QRCode       string   `json:"qr_code"`
	RecoveryKeys []string `json:"recovery_keys"`
}

type TwoFactorData struct {
	Scheme   string
	Hostname string
	Code     string
	Username string
}

type PasswordResetData struct {
	Scheme   string
	Hostname string
	Code     string
	Username string
	Expires  time.Time
}
