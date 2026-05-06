package ssl

import (
	"context"
)

// SSLConfig represents the SSL preferences for a domain or subdomain
type SSLConfig struct {
	Domain   string `json:"domain"`
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`  // "letsencrypt" or "custom"
	CertPath string `json:"cert_path"` // If custom or generated
	KeyPath  string `json:"key_path"`  // If custom or generated
}

// SSLManager defines interface for managing SSL certificates
type SSLManager interface {
	ConfigureDomain(ctx context.Context, config *SSLConfig) error
	RevokeDomain(ctx context.Context, domain string) error
	GetStatus(ctx context.Context, domain string) (*SSLConfig, error)
}
