package types

// TokenResponse is returned by the registry on successful authentication
type TokenResponse struct {
	Token string `json:"token"`
}
