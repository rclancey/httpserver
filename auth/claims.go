package auth

import (
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gofrs/uuid"
)

func NewStandardClaims(issuer string, dur time.Duration) *StandardClaims {
	now := time.Now()
	return &StandardClaims{
		StandardClaims: jwt.StandardClaims{
			IssuedAt: now.Unix(),
			Issuer: issuer,
			ExpiresAt: now.Add(dur).Unix(),
			NotBefore: now.Unix(),
		},
		AuthTime: now.Unix(),
		TTL: dur.Milliseconds(),
	}
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

func (c *StandardClaims) Extend() {
	now := time.Now()
	c.StandardClaims.IssuedAt = now.Unix()
	c.StandardClaims.ExpiresAt = now.Add(time.Duration(c.TTL) * time.Millisecond).Unix()
}

func (c *StandardClaims) SetUser(user User) {
	c.Username = user.GetUsername()
	idu, ok := user.(IntIDUser)
	if ok {
		c.UserID = idu.GetUserID()
	}
	uu, ok := user.(UUIDUser)
	if ok {
		c.UserUUID = uu.GetUUID()
	}
	fnu, ok := user.(FullNameUser)
	if ok {
		c.SetFullName(fnu.GetFullName())
	}
	flnu, ok := user.(FirstLastNameUser)
	if ok {
		c.FirstName = flnu.GetFirstName()
		c.LastName = flnu.GetLastName()
	}
	eu, ok := user.(EmailUser)
	if ok {
		c.Email = eu.GetEmailAddress()
	}
	pu, ok := user.(PhoneUser)
	if ok {
		c.Phone = pu.GetPhoneNumber()
	}
	zu, ok := user.(TimeZoneUser)
	if ok {
		c.TimeZone = zu.GetTimeZone()
	}
	lu, ok := user.(LocaleUser)
	if ok {
		c.Locale = lu.GetLocale()
	}
	au, ok := user.(AvatarUser)
	if ok {
		c.Avatar = au.GetAvatar()
	}
}

func (c *StandardClaims) SetFullName(name string) {
	c.FullName = name
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

func (c *StandardClaims) GetUsername() string {
	return c.Username
}

func (c *StandardClaims) GetUserID() int64 {
	return c.UserID
}

func (c *StandardClaims) GetUUID() uuid.UUID {
	return c.UserUUID
}

func (c *StandardClaims) GetFirstName() string {
	return c.FirstName
}

func (c *StandardClaims) GetLastName() string {
	return c.LastName
}

func (c *StandardClaims) GetFullName() string {
	if c.FullName != "" {
		return c.FullName
	}
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

func (c *StandardClaims) GetTimeZone() string {
	return c.TimeZone
}

func (c *StandardClaims) GetLocale() string {
	return c.Locale
}

func (c *StandardClaims) GetAvatar() string {
	return c.Avatar
}
