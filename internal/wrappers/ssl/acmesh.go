package ssl

import (
	"context"
	"fmt"
)

// AcmeShManager implements Let's Encrypt / ZeroSSL integration via acme.sh
type AcmeShManager struct {
	WebServerPlugin string
}

func NewAcmeShManager(plugin string) *AcmeShManager {
	return &AcmeShManager{WebServerPlugin: plugin}
}

func (a *AcmeShManager) ObtainCertStandalone(ctx context.Context, domain string) error {
	// TODO: implement acme.sh standalone procurement
	return fmt.Errorf("Not implemented")
}

func (a *AcmeShManager) ObtainCertWebroot(ctx context.Context, domain string) error {
	// TODO: implement acme.sh webroot procurement
	return fmt.Errorf("Not implemented")
}

func (a *AcmeShManager) ConfigureDomain(ctx context.Context, config *SSLConfig) error {
	// TODO: wrapper for acme.sh --issue and --install-cert
	return fmt.Errorf("Not implemented")
}

func (a *AcmeShManager) RevokeDomain(ctx context.Context, domain string) error {
	// TODO: implement acme.sh --revoke
	return fmt.Errorf("Not implemented")
}

func (a *AcmeShManager) GetStatus(ctx context.Context, domain string) (*SSLConfig, error) {
	// TODO: implement acme.sh --list
	return nil, fmt.Errorf("Not implemented")
}
