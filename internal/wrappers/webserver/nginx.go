package webserver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

// NginxWrapper implements the WebServer interface for Nginx on Debian/Ubuntu
type NginxWrapper struct {
	SitesAvailablePath string
	SitesEnabledPath   string
}

// NewNginxWrapper creates a new Nginx manager
func NewNginxWrapper() *NginxWrapper {
	return &NginxWrapper{
		SitesAvailablePath: "/etc/nginx/sites-available",
		SitesEnabledPath:   "/etc/nginx/sites-enabled",
	}
}

// Basic Nginx Server Block Template structure
const nginxVhostTemplate = `server {
    listen 80;
    server_name {{.Domain}} www.{{.Domain}};
    
{{if .EnableSSL}}
    listen 443 ssl;
    ssl_certificate {{.CertPath}};
    ssl_certificate_key {{.KeyPath}};
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
{{end}}

    root {{.DocumentRoot}};
    index index.html index.htm index.php;

    access_log /var/log/nginx/{{.Domain}}-access.log;
    error_log /var/log/nginx/{{.Domain}}-error.log;

    # Webroot path for Let's Encrypt AutoSSL
    location ^~ /.well-known/acme-challenge/ {
        default_type "text/plain";
        root /var/www/acme-challenge;
    }

    # Proxy internal dashboard route mapping
    location /dashBoard2go {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location / {
        try_files $uri $uri/ /index.php?$args;
    }

{{if .PHPVersion}}
    # Pass PHP scripts to FastCGI server
    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/run/php/php{{.PHPVersion}}-fpm.sock;
    }
{{end}}

    location ~ /\.ht {
        deny all;
    }
}
`

// CreateVhost parses the template and writes it to the sites-available directory
func (n *NginxWrapper) CreateVhost(config VhostConfig) error {
	tmpl, err := template.New("vhost").Parse(nginxVhostTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse nginx template: %w", err)
	}

	confPath := filepath.Join(n.SitesAvailablePath, config.Domain)

	// Create directory if mock/testing, otherwise it should exist in Debian
	_ = os.MkdirAll(n.SitesAvailablePath, 0755)

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
func (n *NginxWrapper) DeleteVhost(domain string) error {
	confPath := filepath.Join(n.SitesAvailablePath, domain)
	return os.Remove(confPath)
}

// EnableVhost creates a symlink from sites-available to sites-enabled
func (n *NginxWrapper) EnableVhost(domain string) error {
	// Create enabled directory if it doesn't exist
	_ = os.MkdirAll(n.SitesEnabledPath, 0755)

	src := filepath.Join(n.SitesAvailablePath, domain)
	dst := filepath.Join(n.SitesEnabledPath, domain)

	// Check if already enabled string
	if _, err := os.Stat(dst); err == nil {
		return nil // already exists
	}

	err := os.Symlink(src, dst)
	if err != nil {
		return fmt.Errorf("failed to enable nginx vhost (symlink): %w", err)
	}
	return nil
}

// DisableVhost removes the symlink from sites-enabled
func (n *NginxWrapper) DisableVhost(domain string) error {
	dst := filepath.Join(n.SitesEnabledPath, domain)

	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return nil // already disabled
	}

	err := os.Remove(dst)
	if err != nil {
		return fmt.Errorf("failed to disable nginx vhost (remove symlink): %w", err)
	}
	return nil
}

// Reload reloads the Nginx service using systemctl
func (n *NginxWrapper) Reload() error {
	cmd := exec.Command("systemctl", "reload", "nginx")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx reload failed: %v, output: %s", err, string(out))
	}
	return nil
}

// NginxWrapper logic
func (w *NginxWrapper) CreateVhostWithPHP(config VhostConfig, phpSocket string) error {
	vhostPath := fmt.Sprintf("/etc/nginx/sites-available/%s.vhost", config.Domain)

	// Modified Nginx template integrating purely with FPM proxy
	vhostData := fmt.Sprintf(`server {
    listen 80;
    server_name %s;
    root %s;
    index index.php index.html index.htm;
    
    access_log /home/dashboard2go/users/%s/web/%s/logs/access.log;
    error_log /home/dashboard2go/users/%s/web/%s/logs/error.log;

    location / {
        try_files $uri $uri/ /index.php?$args;
    }

    # Pass PHP scripts to FastCGI server
    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        # Map to the specific user's FPM Socket
        fastcgi_pass unix:%s;
        
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }

    # Deny access to .htaccess files
    location ~ /\.ht {
        deny all;
    }
}
`, config.Domain, config.DocumentRoot, config.Username, config.Domain, config.Username, config.Domain, phpSocket)

	return os.WriteFile(vhostPath, []byte(vhostData), 0644)
}
