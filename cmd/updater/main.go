package main

import (
"database/sql"
"log"
"time"

"dashBoard2go/internal/config"
"dashBoard2go/internal/updater"

_ "github.com/mattn/go-sqlite3"
)

func main() {
log.Println("Starting dashBoard2go Updater Service...")

conf, err := config.LoadConfig("config.json")
if err != nil {
log.Fatalf("[Updater] Failed to load config.json: %v", err)
}

if conf.UpdaterEndpoint == "" {
log.Println("[Updater] UpdaterEndpoint is empty. Updates are disabled. Terminating.")
return
}

log.Printf("[Updater] Tracking Current Version: %s\n", conf.PanelVersion)

// In a complete implementation, this poll would run daily (24h) instead of every minute
ticker := time.NewTicker(24 * time.Hour)
defer ticker.Stop()

// Optionally do an immediate check on startup
checkForUpdates(conf)

for {
select {
case <-ticker.C:
checkForUpdates(conf)
}
}
}

func checkForUpdates(conf *config.PanelConfig) {
log.Println("[Updater] Checking GitHub for new releases...")

dbURI := "config.db?_journal_mode=WAL" // Hardcoded generic path
db, err := sql.Open("sqlite3", dbURI)
if err != nil {
log.Printf("[Updater] Failed to open DB: %v\n", err)
return
}
defer db.Close()

err = updater.CheckForUpdates(db, conf.PanelVersion)
if err != nil {
log.Printf("[Updater] Check for updates failed: %v\n", err)
}

updateAvailable := false
newVersion := "v1.1.0" // Mocked response

if updateAvailable && newVersion != conf.PanelVersion {
log.Printf("[Updater] Update found: %s. Initiating upgrade sequence...\n", newVersion)
performUpgrade(conf, newVersion)
} else {
log.Println("[Updater] System is up to date.")
}
}

func performUpgrade(conf *config.PanelConfig, newVersion string) {
log.Println("[Updater] PHASE 1: Downloading new binary payload from GitHub...")
// ... download logic ...

log.Println("[Updater] PHASE 2: Shutting down internal daemons (Core, Worker, Watchdog)...")
// oswrap.StopService("dashboard2go-core")
// oswrap.StopService("dashboard2go-worker")
// oswrap.StopService("dashboard2go-watchdog")

log.Println("[Updater] PHASE 3: Running SQL and JSON Migrations...")
// Handle column additions to sqlite, or new config.json fields

log.Println("[Updater] PHASE 4: Swapping Binaries...")
// e.g. mv /tmp/new_core /usr/local/bin/dashboard2go-core

log.Println("[Updater] PHASE 5: Updating config.json with new target version...")
conf.PanelVersion = newVersion
config.SaveConfig("config.json", conf)

log.Println("[Updater] PHASE 6: Restarting Panel Daemons...")
// oswrap.StartService("dashboard2go-core")
// oswrap.StartService("dashboard2go-worker")
// oswrap.StartService("dashboard2go-watchdog")

log.Println("[Updater] Upgrade complete! System is now on", newVersion)

// Note: If the updater binary itself changed, it might need to `exec.Command` itself natively and safely `os.Exit(0)` here to reload its own memory footprint.
}
