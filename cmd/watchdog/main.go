package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"dashBoard2go/internal/config"
	"dashBoard2go/internal/oswrap"
	"dashBoard2go/internal/wrappers/firewall"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	log.Println("Starting dashBoard2go Watchdog Service...")

	conf, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("[Watchdog] Failed to load config: %v", err)
	}

	dbURI := fmt.Sprintf("%s?_journal_mode=WAL", conf.SQLitePath)
	db, err := sql.Open("sqlite3", dbURI)
	if err != nil {
		log.Fatalf("[Watchdog] Failed to open SQLite: %v", err)
	}
	defer db.Close()

	criticalServices := []string{
		"nginx", "apache2", "mariadb", "postgresql", "bind9", "postfix", "dovecot", "amavis", "pure-ftpd", "ufw",
	}

	ufwWrapper := firewall.NewUFWWrapper(nil)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial synchronous start for loop
	for {
		select {
		case <-ticker.C:
			// 1. Service Health Checks
			for _, service := range criticalServices {
				if !oswrap.IsActive(service) {
					log.Printf("[Watchdog] ALERT: %s is DOWN. Attempting recovery...\n", service)
					err := oswrap.RestartService(service)
					if err != nil {
						log.Printf("[Watchdog] ERROR: Failed to recover %s: %v\n", service, err)
					} else {
						log.Printf("[Watchdog] SUCCESS: %s recovered successfully.\n", service)
					}
				}
			}

			// 2. Firewall Rule Integrity
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			reconfigured, err := ufwWrapper.Sync(ctx)
			cancel()

			if err != nil {
				log.Printf("[Watchdog] ERROR: Firewall Sync check failed: %v\n", err)
			} else if reconfigured {
				log.Println("[Watchdog] WARNING: UFW Rules diverged. Auto-reconfigured to match DB state.")
			}

			// 3. User Log Analysis mapped dynamically from SQLite routing!
			analyzeUserLogs(db)
		}
	}
}
