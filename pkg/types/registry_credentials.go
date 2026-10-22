package types

// RegistryCredentials holds registry authentication material.
//
// Username/Password cover classic Basic auth. IdentityToken covers cloud
// helpers (for example ECR) that store a short-lived token without a password.
type RegistryCredentials struct {
	Username      string `json:"username"`                // Registry username.
	Password      string `json:"password"`                // Registry token or password.
	IdentityToken string `json:"identitytoken,omitempty"` // OAuth/identity token from a credential helper.
}
