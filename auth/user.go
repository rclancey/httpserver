package auth

type User struct {
	ID        string `json:"id,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	FullName  string `json:"full_name,omitempty"`
	Email     string `json:"email,omitempty"`
	Avatar    string `json:"avatar,omitempty"`
	Provider  string `json:"provider,omitempty"`
}
