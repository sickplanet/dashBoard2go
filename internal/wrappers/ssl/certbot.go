package ssl

import (
	"context"
	"fmt"
	"os/exec"
)

// CertbotManager implements Let's Encrypt integration via certbot
type CertbotManager struct {
	// e.g. "nginx" or "apache"
	WebServerPlugin string
}

func NewCertbotManager(plugin string) *CertbotManager {
	return &CertbotManager{WebServerPlugin: plugin}
}

func (c *CertbotManager) ConfigureDomain(ctx context.Context, config *SSLConfig) error {
	if !config.Enabled {
		return nil
	}

	if config.Provider == "letsencrypt" {
		// e.g. certbot --nginx -d example.com --non-interactive --agree-tos -m admin@example.com
		// We use standard webroot or the specific plugin.
		// Using --webroot is critical for domains actively reverse-proxying into dashBoard2go
		// (e.g. userdomain.tld/dashboard2go) so we do not have to dynamically drop proxy configs to validate ACME HTTP-01 challenges.
		cmd := exec.CommandContext(ctx, "certbot", "certonly",
			"--webroot", "-w", "/var/www/acme-challenge",
			"-d", config.Domain,
			"--non-interactive",
			"--agree-tos",
			"--register-unsafely-without-email",
		)
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("certbot failed for %s: %v", config.Domain, err)
		}

		// Update paths automatically provisioned by certbot
		config.CertPath = fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", config.Domain)
		config.KeyPath = fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", config.Domain)

		return nil

	} else if config.Provider == "custom" {
		// Validating custom provided paths
		if config.CertPath == "" || config.KeyPath == "" {
			return fmt.Errorf("custom provider selected but cert/key paths missing")
		}
		// Logic to copy to standard location or merely verify existence would go here
		return nil
	}

	return fmt.Errorf("unknown SSL provider: %s", config.Provider)
}

func (c *CertbotManager) RevokeDomain(ctx context.Context, domain string) error {
	cmd := exec.CommandContext(ctx, "certbot", "revoke", "--cert-name", domain, "--non-interactive")
	return cmd.Run()
}

func (c *CertbotManager) GetStatus(ctx context.Context, domain string) (*SSLConfig, error) {
	// Can invoke certbot certificates -d example.com and parse output,
	// returning mocked state for brevity.
	return &SSLConfig{
		Domain:   domain,
		Enabled:  true,
		Provider: "letsencrypt",
		CertPath: fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domain),
		KeyPath:  fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", domain),
	}, nil
}

func (c *CertbotManager) ObtainCertStandalone(ctx context.Context, domain string) error {
	cmd := exec.CommandContext(ctx, "certbot", "certonly",
		"--standalone",
		"-d", domain,
		"--non-interactive",
		"--agree-tos",
		"--register-unsafely-without-email",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certbot standalone failed for %s: %v\nOutput: %s", domain, err, string(out))
	}
	return nil
}

func (c *CertbotManager) ObtainCertWebroot(ctx context.Context, domain string) error {
	cmd := exec.CommandContext(ctx, "certbot", "certonly",
		"--webroot", "-w", "/var/www/acme-challenge",
		"-d", domain,
		"--non-interactive",
		"--agree-tos",
		"--register-unsafely-without-email",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certbot webroot failed for %s: %v\nOutput: %s", domain, err, string(out))
	}
	return nil
}
