package auth

func Has2FA(user User) bool {
	tfuser, isa := user.(TwoFactorUser)
	if !isa {
		return false
	}
	tfa, err := tfuser.GetTwoFactorAuth()
	if err == nil && tfa == nil {
		return false
	}
	return true
}
