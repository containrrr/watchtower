package types

// TokenResponse holds a registry authentication token response.
type TokenResponse struct {
	Token       string `json:"token"`        // Authentication token.
	AccessToken string `json:"access_token"` // Alternative authentication token.
	ExpiresIn   int    `json:"expires_in"`   // Token lifetime in seconds.
	IssuedAt    string `json:"issued_at"`    // Token issuance time in RFC3339 format.
}
