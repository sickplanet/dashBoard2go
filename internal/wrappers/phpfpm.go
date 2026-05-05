package wrappers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// PHPFpmConfig holds values for generating a user-specific PHP-FPM pool
type PHPFpmConfig struct {
	Username     string
	Domain       string
	PHPVersion   string // e.g. "8.3", default if empty
	SocketPath   string
	DocumentRoot string
	MaxChildren  int // Memory ceiling controls per user
}

// PHPFpmWrapper handles configuration generation for PHP FPM
type PHPFpmWrapper struct {
	BaseDir string // e.g., /etc/php
}

func NewPHPFpmWrapper() *PHPFpmWrapper {
	return &PHPFpmWrapper{
		BaseDir: "/etc/php",
	}
}

func (w *PHPFpmWrapper) CreatePool(config PHPFpmConfig) error {
	version := config.PHPVersion
	if version == "" {
		version = "8.3" // Default version installed via setup
	}

	// Standard FPM Pool Path
	poolDir := filepath.Join(w.BaseDir, version, "fpm/pool.d")
	os.MkdirAll(poolDir, 0755)

	poolFile := filepath.Join(poolDir, fmt.Sprintf("%s_%s.conf", config.Domain, config.Username))

	if config.SocketPath == "" {
		config.SocketPath = fmt.Sprintf("/run/php/php%s-fpm-%s-%s.sock", version, config.Domain, config.Username)
	}

	if config.MaxChildren == 0 {
		config.MaxChildren = 10
	}

	poolConfig := fmt.Sprintf(`[%s]
user = %s
group = %s
listen = %s
listen.owner = www-data
listen.group = www-data
listen.mode = 0660

pm = ondemand
pm.max_children = %d
pm.process_idle_timeout = 10s
pm.max_requests = 500

; Hardening constraints - mimic ISPConfig protections
php_admin_value[open_basedir] = %s:/tmp:/usr/share/php
php_admin_value[session.save_path] = /home/dashboard2go/users/%s/tmp
php_admin_value[upload_tmp_dir] = /home/dashboard2go/users/%s/tmp
`, config.Domain, config.Username, config.Username, config.SocketPath, config.MaxChildren, config.DocumentRoot, config.Username, config.Username)

	// Ensure the user's tmp dir exists to prevent session lockups
	os.MkdirAll(fmt.Sprintf("/home/dashboard2go/users/%s/tmp", config.Username), 0755)

	err := os.WriteFile(poolFile, []byte(poolConfig), 0644)
	if err != nil {
		return fmt.Errorf("failed to write fpm pool: %v", err)
	}

	return nil
}

func (w *PHPFpmWrapper) DeletePool(domain, username, version string) error {
	if version == "" {
		version = "8.3"
	}
	poolFile := filepath.Join(w.BaseDir, version, "fpm/pool.d", fmt.Sprintf("%s_%s.conf", domain, username))
	return os.Remove(poolFile)
}

func (w *PHPFpmWrapper) Reload(version string) error {
	if version == "" {
		version = "8.3"
	}
	service := fmt.Sprintf("php%s-fpm", version)
	return exec.Command("systemctl", "reload", service).Run()
}
