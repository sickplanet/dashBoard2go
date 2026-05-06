package webserver

// VhostConfig defines the configuration parameters for a web virtual host
type VhostConfig struct {
	Username        string
	Domain          string
	DocumentRoot    string
	AdminEmail      string
	PHPVersion      string // e.g., "8.2", leave empty for static/default
	EnableSSL       bool
	CertPath        string
	KeyPath         string
	IsProxy         bool
	ProxyTargetPort int
}

// WebServer is the strategy interface that all web server implementations must satisfy.
// This allows the core application to be completely agnostic to whether it is running Apache or Nginx.
type WebServer interface {
	// CreateVhost generates the configuration file for a new virtual host
	CreateVhost(config VhostConfig) error

	// DeleteVhost removes the configuration file for a virtual host
	DeleteVhost(domain string) error

	// EnableVhost activates a virtual host (e.g., a2ensite or symlink)
	EnableVhost(domain string) error

	// DisableVhost deactivates a virtual host (e.g., a2dissite or remove symlink)
	DisableVhost(domain string) error

	// Reload signals the web server to reload its configuration gracefully
	Reload() error
}
