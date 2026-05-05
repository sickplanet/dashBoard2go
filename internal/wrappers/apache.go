package wrappers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

// ApacheWrapper implements the WebServer interface for Apache2 on Debian/Ubuntu
type ApacheWrapper struct {
	SitesAvailablePath string
}

// NewApacheWrapper creates a new Apache2 manager
func NewApacheWrapper() *ApacheWrapper {
	return &ApacheWrapper{
		SitesAvailablePath: "/etc/apache2/sites-available",
	}
}

const apacheVhostTemplate = `<VirtualHost *:80>
    ServerName {{.Domain}}
    ServerAlias www.{{.Domain}}
    ServerAdmin {{.AdminEmail}}
    DocumentRoot {{.DocumentRoot}}

    # Let's Encrypt / certbot webroot Challenge Path
    Alias /.well-known/acme-challenge/ /var/www/acme-challenge/
    <Directory "/var/www/acme-challenge/">
        Options None
        AllowOverride None
        ForceType text/plain
        Require all granted
    </Directory>

    # dashBoard2go UI Proxy mappings
    ProxyPreserveHost On
    ProxyPass /dashBoard2go http://127.0.0.1:8080/
    ProxyPassReverse /dashBoard2go http://127.0.0.1:8080/

    <Directory {{.DocumentRoot}}>
        Options -Indexes +FollowSymLinks
        AllowOverride All
        Require all granted
    </Directory>

    ErrorLog ${APACHE_LOG_DIR}/{{.Domain}}-error.log
    CustomLog ${APACHE_LOG_DIR}/{{.Domain}}-access.log combined
</VirtualHost>

{{if .EnableSSL}}
<VirtualHost *:443>
    ServerName {{.Domain}}
    ServerAlias www.{{.Domain}}
    ServerAdmin {{.AdminEmail}}
    DocumentRoot {{.DocumentRoot}}

    # SSL Config natively supplied by certbot
    SSLEngine on
    SSLCertificateFile {{.CertPath}}
    SSLCertificateKeyFile {{.KeyPath}}

    # Let's Encrypt renewal pass-through
    Alias /.well-known/acme-challenge/ /var/www/acme-challenge/

    # dashBoard2go secure UI Proxy mapping
    ProxyPreserveHost On
    ProxyPass /dashBoard2go http://127.0.0.1:8080/
    ProxyPassReverse /dashBoard2go http://127.0.0.1:8080/

    <Directory {{.DocumentRoot}}>
        Options -Indexes +FollowSymLinks
        AllowOverride All
        Require all granted
    </Directory>

    ErrorLog ${APACHE_LOG_DIR}/{{.Domain}}-error.log
    CustomLog ${APACHE_LOG_DIR}/{{.Domain}}-access.log combined
</VirtualHost>
{{end}}
`

// CreateVhost parses the template and writes it to the sites-available directory
func (a *ApacheWrapper) CreateVhost(config VhostConfig) error {
	tmpl, err := template.New("vhost").Parse(apacheVhostTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse apache template: %w", err)
	}

	confPath := filepath.Join(a.SitesAvailablePath, fmt.Sprintf("%s.conf", config.Domain))

	// Create directory if mock/testing, otherwise it should exist in Debian
	_ = os.MkdirAll(a.SitesAvailablePath, 0755)

	f, err := os.Create(confPath)
	if err != nil {
		return fmt.Errorf("failed to create config file %s: %w", confPath, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, config); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// DeleteVhost removes the configuration file
func (a *ApacheWrapper) DeleteVhost(domain string) error {
	confPath := filepath.Join(a.SitesAvailablePath, fmt.Sprintf("%s.conf", domain))
	return os.Remove(confPath)
}

// EnableVhost runs the native Debian a2ensite command
func (a *ApacheWrapper) EnableVhost(domain string) error {
	cmd := exec.Command("a2ensite", fmt.Sprintf("%s.conf", domain))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("a2ensite failed: %v, output: %s", err, string(out))
	}
	return nil
}

// DisableVhost runs the native Debian a2dissite command
func (a *ApacheWrapper) DisableVhost(domain string) error {
	cmd := exec.Command("a2dissite", fmt.Sprintf("%s.conf", domain))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("a2dissite failed: %v, output: %s", err, string(out))
	}
	return nil
}

// Reload reloads the Apache2 service using systemctl
func (a *ApacheWrapper) Reload() error {
	cmd := exec.Command("systemctl", "reload", "apache2")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apache reload failed: %v, output: %s", err, string(out))
	}
	return nil
}
