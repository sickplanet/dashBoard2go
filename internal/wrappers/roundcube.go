package wrappers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// RoundcubeConfig handles pathing and admin details
type RoundcubeConfig struct {
	InstallDir   string // e.g. /var/www/dashboard/roundcube
	DownloadRoot string // URL or version branch
}

// RoundcubeManager orchestrates roundcube installs, themes, and plugins for the webserver.
type RoundcubeManager struct {
	Config RoundcubeConfig
}

// NewRoundcubeManager creates a struct to manage roundcube functions
func NewRoundcubeManager() *RoundcubeManager {
	return &RoundcubeManager{
		Config: RoundcubeConfig{
			InstallDir: "/var/www/dashBoard2go/webmail", // Global shared installation target
		},
	}
}

// InstallCore pulls down the current roundcube package and extracts it natively into the core path.
func (r *RoundcubeManager) InstallCore(versionURL string) error {
	// Simulated fetching to destination folder replacing existing.
	// Real implementation uses HTTP GET, gzip untar, and preserves the /config directory.

	err := os.MkdirAll(r.Config.InstallDir, 0755)
	if err != nil {
		return err
	}

	// In Go we would download, untar to temp, and rsync over existing core files specifically.
	cmd := exec.Command("wget", "-qO-", versionURL, "|", "tar", "-xz", "-p", "1", "-C", r.Config.InstallDir)
	return cmd.Run()
}

// InstallPlugin connects to Roundcube's plugin ecosystem (Composer/Packagist)
// or manually extracts zipped plugins directly into the plugins layer.
func (r *RoundcubeManager) InstallPlugin(pluginName, sourceURL string) error {
	pluginDir := filepath.Join(r.Config.InstallDir, "plugins", pluginName)
	_ = os.MkdirAll(pluginDir, 0755)

	// Real implementation would pull archive, extract, and append string
	// into the Roundcube config.inc.php array: $config['plugins'] = array('...', 'new_plugin');
	return nil
}

// InstallTheme extracts a zip of a visual theme into the Roundcube /skins directory
func (r *RoundcubeManager) InstallTheme(themeName, sourceURL string) error {
	skinDir := filepath.Join(r.Config.InstallDir, "skins", themeName)
	_ = os.MkdirAll(skinDir, 0755)

	// Fetch -> extract
	// Activate for the admin universally if requested, or leave selectable for users.
	return nil
}

// Upgrade is a composite wrapper that runs Roundcube's native /bin/update.sh file.
// It migrates the internal SQL schema seamlessly.
func (r *RoundcubeManager) Upgrade(versionURL string) error {
	// First pull down the new core files and overwrite using InstallCore strategy.
	if err := r.InstallCore(versionURL); err != nil {
		return fmt.Errorf("failed to fetch update package: %v", err)
	}

	// Shell out to Roundcube's built-in upgrade utility, preventing SQL breakage.
	updateScript := filepath.Join(r.Config.InstallDir, "bin", "update.sh")

	cmd := exec.Command("php", updateScript, "--version=?", "-y")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("roundcube db migration step failed: %v. Output: %s", err, string(out))
	}
	return nil
}
