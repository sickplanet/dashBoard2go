package updater

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
)

// ApplyUpdate writes a decoupled script and runs it in the background
func ApplyUpdate(targetVersion string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get cwd: %v", err)
	}

	scriptPath := "/tmp/dashboard2go-apply-update.sh"

	scriptContent := fmt.Sprintf(`#!/bin/bash
set -e

LOG_PUBLIC="/var/www/html/dashboard2go_update.log"
LOG_TMP="/tmp/dashboard2go-update.log"
touch $LOG_TMP || true
touch $LOG_PUBLIC || true
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
log "Fetching %s from Github..."
wget -q "https://github.com/sickplanet/dashBoard2go/releases/download/%s/dashboard2go-linux-amd64.tar.gz" -O /tmp/dashboard2go.tar.gz || log "Warning: Wget failed"
mkdir -p /tmp/dashboard2go_extract
tar -xzf /tmp/dashboard2go.tar.gz -C /tmp/dashboard2go_extract/ || log "Warning: Tar extract failed"

log "4. Swapping target executables and web dir..."
cp -R /tmp/dashboard2go_extract/dashboard2go-* %s/ || true
cp -R /tmp/dashboard2go_extract/web %s/ || true

log "5. Binding permissions..."
chmod +x %s/dashboard2go-* || true

log "6. Restarting ecosystem daemons..."
systemctl start dashboard2go-core
systemctl start dashboard2go-worker
systemctl start dashboard2go-watchdog
systemctl start dashboard2go-updater || true

log "Update completed successfully. Target payload active."
rm -f /tmp/dashboard2go-apply-update.sh
rm -rf /tmp/dashboard2go_extract /tmp/dashboard2go.tar.gz
`, targetVersion, cwd, targetVersion, targetVersion, cwd, cwd, cwd)
	err = ioutil.WriteFile(scriptPath, []byte(scriptContent), 0755)
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
