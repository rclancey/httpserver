package auth

import (
	"github.com/rclancey/twofactor"
)

type FullNameUser interface {
	GetFullName() string
}

type FirstLastNameUser interface {
	GetFirstName() string
	GetLastName() string
}

type EmailUser interface {
	GetEmailAddress() string
}

type PhoneUser interface {
	GetPhoneNumber() string
}

type AvatarUser interface {
	GetAvatar() string
}

type User interface {
	GetUsername() string
}

type AuthUser interface {
	User
	GetAuth() (*twofactor.Auth, error)
	SetAuth(*twofactor.Auth) error
}

type SocialUser interface {
	AuthUser
	SetSocialID(driver, id string) error
}

type UserSource interface {
	GetUser(username string) (AuthUser, error)
	GetUserByEmail(email string) (AuthUser, error)
}

type SocialUserSource interface {
	UserSource
	GetSocialUser(driver, id, username string) (AuthUser, error)
}
