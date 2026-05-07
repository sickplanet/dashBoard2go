package main

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"dashBoard2go/internal/api"
	"dashBoard2go/internal/config"
	"dashBoard2go/internal/db/migrations"
	"dashBoard2go/internal/queue"
	"dashBoard2go/internal/updater"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	log.Println("Starting dashBoard2go Core API Server...")

	conf, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("FAILED: Could not load config.json. Have you run the setup? Error: %v", err)
	}

	if !conf.Installed {
		log.Fatalf("FAILED: Panel is not fully installed. Please run dashboard2go-setup.")
	}

	dbURI := fmt.Sprintf("%s?_journal_mode=WAL", conf.SQLitePath)
	db, err := sql.Open("sqlite3", dbURI)
	if err != nil {
		log.Fatalf("FAILED to open SQLite database: %v", err)
	}
	defer db.Close()

	// Apply database migrations on start
	if err := migrations.Migrate(db); err != nil {
		log.Fatalf("FAILED to apply database migrations: %v", err)
	}

	go func() {
		for {
			versionBytes, _ := os.ReadFile("VERSION")
			currentVer := strings.TrimSpace(string(versionBytes))
			updater.CheckForUpdates(db, currentVer, conf.UpdaterEndpoint)
			time.Sleep(1 * time.Hour)
		}
	}()

	log.Println("SQLite Database connected and migrated successfully (WAL Mode).")

	q, err := queue.NewSQLiteQueue(db)
	if err != nil {
		log.Fatalf("FAILED to init queue: %v", err)
	}

	// Initialize the HTTP Router before setting up routes
	r := gin.Default()

	api.SetupRoutes(r, db, q, conf)

	r.Static("/admin", "./web/admin")
	r.Static("/user", "./web/user")
	r.Static("/shared", "./web/shared")
	r.Static("/js", "./web/js")

	httpAddr := fmt.Sprintf(":%d", conf.PanelPortHTTP)
	httpsAddr := fmt.Sprintf(":%d", conf.PanelPortHTTPS)

	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", conf.FQDN)
	keyPath := fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", conf.FQDN)

	if conf.UseLetsEncryptFQDN {
		go func() {
			log.Printf("Starting HTTP Server on %s (Redirecting to HTTPS)", httpAddr)
			err := http.ListenAndServe(httpAddr, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				target := "https://" + req.Host + req.URL.Path
				if len(req.URL.RawQuery) > 0 {
					target += "?" + req.URL.RawQuery
				}
				http.Redirect(w, req, target, http.StatusTemporaryRedirect)
			}))
			if err != nil {
				log.Printf("Warning: HTTP redirect server failed: %v", err)
			}
		}()

		log.Printf("Starting HTTPS Server on %s", httpsAddr)
		server := &http.Server{
			Addr:    httpsAddr,
			Handler: r,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		}

		err = server.ListenAndServeTLS(certPath, keyPath)
		if err != nil {
			log.Fatalf("FAILED to start HTTPS Server: %v", err)
		}
	} else {
		log.Printf("Starting HTTP Server on %s (No SSL)", httpAddr)
		err = r.Run(httpAddr)
		if err != nil {
			log.Fatalf("FAILED to start HTTP Server: %v", err)
		}
	}
}
