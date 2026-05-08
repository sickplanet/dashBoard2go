package updater

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type applyGitHubRelease struct {
	Assets []struct {
		BrowserDownloadURL string `json:"browser_download_url"`
		Name               string `json:"name"`
	} `json:"assets"`
}

// ApplyUpdate writes a decoupled script and runs it in the background
func ApplyUpdate(targetVersion string, endpoint string) error {
	if endpoint == "" {
		endpoint = "https://api.github.com/repos/sickplanet/dashBoard2go/releases/latest"
	}
	if endpoint == "" {
		endpoint = "https://api.github.com/repos/sickplanet/dashBoard2go/releases/latest"
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get cwd: %v", err)
	}

	// Fetch dynamic asset URL
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return fmt.Errorf("failed checking latest release: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var release applyGitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return err
	}

	if len(release.Assets) == 0 {
		return fmt.Errorf("no release assets found")
	}

	downloadURL := release.Assets[0].BrowserDownloadURL
	assetName := release.Assets[0].Name // e.g. dashboard2go-linux-amd64-v1.0.14.zip

	scriptPath := "/tmp/dashboard2go-apply-update.sh"

	// Check if it's a zip or tar
	extractCmd := ""
	if len(assetName) > 4 && assetName[len(assetName)-4:] == ".zip" {
		extractCmd = fmt.Sprintf("unzip -q /tmp/dashboard2go_payload -d /tmp/dashboard2go_extract/ || log \"Warning: Unzip extract failed\"")
	} else {
		extractCmd = fmt.Sprintf("tar -xzf /tmp/dashboard2go_payload -C /tmp/dashboard2go_extract/ || log \"Warning: Tar extract failed\"")
	}

	scriptContent := fmt.Sprintf(`#!/bin/bash
set -e

LOG_PUBLIC="/var/www/html/dashboard2go_update.log"
LOG_TMP="/tmp/dashboard2go-update.log"
> $LOG_TMP || true
> $LOG_PUBLIC || true
chmod 644 $LOG_PUBLIC || true

log() {
    echo "[$(date +'%%H:%%M:%%S')] $1" | tee -a $LOG_TMP
    echo "[$(date +'%%H:%%M:%%S')] $1" >> $LOG_PUBLIC || true
}

log "--- dashBoard2go Updater Triggered for %s ---"
log "1. Stopping all dashboard2go systemd services..."
systemctl stop dashboard2go-core || true
systemctl stop dashboard2go-worker || true
systemctl stop dashboard2go-watchdog || true
systemctl stop dashboard2go-updater || true

sleep 2

log "2. Removing old locked binaries..."
rm -f %s/dashboard2go-*

log "3. Extracting and staging payload..."
log "Fetching payload from Github..."
wget -q "%s" -O /tmp/dashboard2go_payload || log "Warning: Wget failed"
mkdir -p /tmp/dashboard2go_extract
%s

log "4. Swapping target executables and web dir..."
if [ -d "/tmp/dashboard2go_extract/dashBoard2go" ]; then
    cp -R /tmp/dashboard2go_extract/dashBoard2go/* %s/ || true
else
    cp -R /tmp/dashboard2go_extract/* %s/ || true
fi

log "5. Binding permissions..."
chmod +x %s/dashboard2go-* || true

log "6. Restarting ecosystem daemons..."
systemctl start dashboard2go-core
systemctl start dashboard2go-worker
systemctl start dashboard2go-watchdog
systemctl start dashboard2go-updater || true

log "Update completed successfully. Target payload active."
rm -f /tmp/dashboard2go-apply-update.sh
rm -rf /tmp/dashboard2go_extract /tmp/dashboard2go_payload
`, targetVersion, cwd, downloadURL, extractCmd, cwd, cwd, cwd)

	err = os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		return fmt.Errorf("failed writing decoupled update script: %w", err)
	}

	log.Printf("[Updater] Launching detached update script: %s\n", scriptPath)
	cmd := exec.Command("systemd-run", "--unit=dashboard2go-update-task", "/bin/bash", scriptPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed starting decoupled update script: %w", err)
	}

	return nil
}
