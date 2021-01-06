package types

// RegistryCredentials is a credential pair used for basic auth
type RegistryCredentials struct {
	Username string
	Password string // usually a token rather than an actual password
}
