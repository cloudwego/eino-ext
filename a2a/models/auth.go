package models

// AuthenticationInfo defines the authentication schemes and credentials for an agent
type AuthenticationInfo struct {
	// Schemes is a list of supported authentication schemes
	Schemes []string `json:"schemes"`
	// Credentials for authentication. Can be a string (e.g., token) or null if not required initially
	Credentials string `json:"credentials,omitempty"`
}
