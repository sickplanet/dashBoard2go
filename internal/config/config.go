package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// PanelConfig represents the hardcoded settings preventing disruptive state changes
type PanelConfig struct {
	Installed          bool   `json:"installed"`
	PanelVersion       string `json:"panel_version"`
	FQDN               string `json:"fqdn"`
	UseLetsEncryptFQDN bool   `json:"use_letsencrypt_fqdn"`
	PanelPortHTTP      int    `json:"panel_port_http"`  // Typically 8080
	PanelPortHTTPS     int    `json:"panel_port_https"` // Typically 8443
	WebEngine          string `json:"web_engine"`       // e.g., "nginx" or "apache"
	HasPostgres        bool   `json:"has_postgres"`     // MariaDB is implicitly always true
	MariaDBRootPass    string `json:"mariadb_root_pass"`
	PostgresRootPass   string `json:"postgres_root_pass,omitempty"`
	SQLitePath         string `json:"sqlite_path"`
	UpdaterEndpoint    string `json:"updater_endpoint"` // e.g. "https://api.github.com/repos/yourname/dashBoard2go/releases/latest"
}

// LoadConfig reads the config.json located next to the executable
func LoadConfig(path string) (*PanelConfig, error) {
	if path == "" {
		path = "config.json"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config.json not found - setup has not been run")
		}
		return nil, err
	}

	var conf PanelConfig
	if err := json.Unmarshal(data, &conf); err != nil {
		return nil, fmt.Errorf("failed to parse config.json: %v", err)
	}

	return &conf, nil
}

// SaveConfig writes the PanelConfig to disk. This is only called ONCE at the end of setup,
// or by the `dashboard2go-updater` during major version migrations.
func SaveConfig(path string, conf *PanelConfig) error {
	if path == "" {
		path = "config.json"
	}

	data, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}

	// 0600 so only root can read/write it
	return os.WriteFile(path, data, 0600)
}
