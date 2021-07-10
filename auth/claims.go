package auth

import (
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
)

type StandardClaims struct {
	jwt.StandardClaims
	TTL time.Duration `json:"ttl,omitemtpy"`
	TwoFactor bool `json:"2fa,omitempty"`
	Provider string `json:"prv,omitempty"`
	FirstName string `json:"fnm,omitempty"`
	LastName string `json:"lnm,omitempty"`
	Email string `json:"eml,omitempty"`
	Phone string `json:"phn,omitempty"`
	Avatar string `json:"avr,omitempty"`
	Extra map[string]interface{} `json:"xtr,omitempty"`
}

func NewStandardClaims(issuer string, dur time.Duration) *StandardClaims {
	now := time.Now()
	return &StandardClaims{
		StandardClaims: jwt.StandardClaims{
			IssuedAt: now.Unix(),
			Issuer: issuer,
			ExpiresAt: now.Add(dur).Unix(),
			NotBefore: now.Unix(),
		},
		TTL: dur,
	}
}

func (c *StandardClaims) Extend() {
	now := time.Now()
	c.StandardClaims.ExpiresAt = now.Add(c.TTL).Unix()
}

func (c *StandardClaims) Valid() error {
	now := time.Now().Unix()
	if c.StandardClaims.IssuedAt == 0 {
		return ErrTokenExpired
	}
	if c.StandardClaims.ExpiresAt < now {
		return ErrTokenExpired
	}
	if c.StandardClaims.NotBefore > now {
		return ErrTokenExpired
	}
	return nil
}

func (c *StandardClaims) SetFullName(name string) {
	parts := strings.Fields(name)
	switch len(parts) {
	case 0:
	case 1:
		c.FirstName = parts[0]
	case 2:
		c.FirstName = parts[0]
		c.LastName = parts[1]
	default:
		c.FirstName = parts[0]
		c.LastName = strings.Join(parts[1:], " ")
	}
}

func (c *StandardClaims) SetUser(user User) {
	c.StandardClaims.Id = user.GetUsername()
	flnu, ok := user.(FirstLastNameUser)
	if ok {
		c.FirstName = flnu.GetFirstName()
		c.LastName = flnu.GetLastName()
	} else {
		fnu, ok := user.(FullNameUser)
		if ok {
			c.SetFullName(fnu.GetFullName())
		}
	}
	eu, ok := user.(EmailUser)
	if ok {
		c.Email = eu.GetEmailAddress()
	}
	pu, ok := user.(PhoneUser)
	if ok {
		c.Phone = pu.GetPhoneNumber()
	}
	au, ok := user.(AvatarUser)
	if ok {
		c.Avatar = au.GetAvatar()
	}
}

func (c *StandardClaims) GetUsername() string {
	return c.StandardClaims.Id
}

func (c *StandardClaims) GetFirstName() string {
	return c.FirstName
}

func (c *StandardClaims) GetLastName() string {
	return c.LastName
}

func (c *StandardClaims) GetFullName() string {
	if c.FirstName == "" {
		if c.LastName == "" {
			return ""
		}
		return c.LastName
	}
	if c.LastName == "" {
		return c.FirstName
	}
	return c.FirstName + " " + c.LastName
}

func (c *StandardClaims) GetEmailAddress() string {
	return c.Email
}

func (c *StandardClaims) GetPhoneNumber() string {
	return c.Phone
}

func (c *StandardClaims) GetAvatar() string {
	return c.Avatar
}
